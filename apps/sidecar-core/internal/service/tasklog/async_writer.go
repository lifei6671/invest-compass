package tasklog

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	defaultAsyncQueueSize     = 512
	defaultAsyncBatchSize     = 50
	defaultAsyncFlushInterval = 2 * time.Second
)

// BatchAppender 是 AsyncWriter 依赖的批量写入边界，通常由 Service 实现。
type BatchAppender interface {
	Append(ctx context.Context, entries []model.TaskLogEntry) error
}

// AsyncWriterConfig 描述任务日志异步写入器的队列和 flush 策略。
type AsyncWriterConfig struct {
	Appender      BatchAppender
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
}

// AsyncWriter 将任务结构化日志先写入内存队列，再批量落库，避免拖慢任务主链路。
type AsyncWriter struct {
	appender      BatchAppender
	queue         chan model.TaskLogEntry
	batchSize     int
	flushInterval time.Duration
	shutdown      chan struct{}
	stopped       chan struct{}
	closeOnce     sync.Once

	mu      sync.Mutex
	closed  bool
	lastErr error
}

// NewAsyncWriter 创建并启动任务日志异步写入器。
func NewAsyncWriter(config AsyncWriterConfig) (*AsyncWriter, error) {
	if config.Appender == nil {
		return nil, fmt.Errorf("task log appender is required")
	}
	queueSize := config.QueueSize
	if queueSize <= 0 {
		queueSize = defaultAsyncQueueSize
	}
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = defaultAsyncBatchSize
	}
	flushInterval := config.FlushInterval
	if flushInterval <= 0 {
		flushInterval = defaultAsyncFlushInterval
	}
	writer := &AsyncWriter{
		appender:      config.Appender,
		queue:         make(chan model.TaskLogEntry, queueSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		shutdown:      make(chan struct{}),
		stopped:       make(chan struct{}),
	}
	go writer.run()
	return writer, nil
}

// WriteTaskLog 将单条任务日志放入队列；队列满时快速失败，避免阻塞任务主流程。
func (writer *AsyncWriter) WriteTaskLog(ctx context.Context, entry model.TaskLogEntry) error {
	if writer.isClosed() {
		return fmt.Errorf("task log async writer is closed")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-writer.shutdown:
		return fmt.Errorf("task log async writer is closed")
	case writer.queue <- entry:
		return nil
	default:
		return fmt.Errorf("task log async queue is full")
	}
}

// Shutdown 停止异步写入器，并 flush 已入队但尚未落库的日志。
func (writer *AsyncWriter) Shutdown(ctx context.Context) error {
	writer.closeOnce.Do(func() {
		writer.mu.Lock()
		writer.closed = true
		writer.mu.Unlock()
		close(writer.shutdown)
	})
	select {
	case <-writer.stopped:
		return writer.LastError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// LastError 返回最近一次后台 flush 错误，供生命周期管理和测试观测。
func (writer *AsyncWriter) LastError() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.lastErr
}

func (writer *AsyncWriter) isClosed() bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.closed
}

func (writer *AsyncWriter) run() {
	defer close(writer.stopped)
	ticker := time.NewTicker(writer.flushInterval)
	defer ticker.Stop()
	batch := make([]model.TaskLogEntry, 0, writer.batchSize)
	for {
		select {
		case entry := <-writer.queue:
			batch = append(batch, entry)
			if len(batch) >= writer.batchSize {
				batch = writer.flush(context.Background(), batch)
			}
		case <-ticker.C:
			batch = writer.flush(context.Background(), batch)
		case <-writer.shutdown:
			batch = writer.drain(batch)
			writer.flush(context.Background(), batch)
			return
		}
	}
}

func (writer *AsyncWriter) drain(batch []model.TaskLogEntry) []model.TaskLogEntry {
	for {
		select {
		case entry := <-writer.queue:
			batch = append(batch, entry)
		default:
			return batch
		}
	}
}

func (writer *AsyncWriter) flush(ctx context.Context, batch []model.TaskLogEntry) []model.TaskLogEntry {
	if len(batch) == 0 {
		return batch[:0]
	}
	entries := append([]model.TaskLogEntry(nil), batch...)
	if err := writer.appender.Append(ctx, entries); err != nil {
		writer.mu.Lock()
		writer.lastErr = err
		writer.mu.Unlock()
		slog.Warn(
			"task log async writer flush failed",
			slog.String(logger.FieldCode, "task_log_flush_failed"),
			slog.String("error", logger.RedactError(err)),
		)
	}
	return batch[:0]
}
