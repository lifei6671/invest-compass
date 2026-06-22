package tasklog

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

type captureBatchAppender struct {
	mu      sync.Mutex
	batches [][]model.TaskLogEntry
	err     error
}

// Append 记录异步写入器提交的批次。
func (appender *captureBatchAppender) Append(_ context.Context, entries []model.TaskLogEntry) error {
	appender.mu.Lock()
	defer appender.mu.Unlock()
	appender.batches = append(appender.batches, append([]model.TaskLogEntry(nil), entries...))
	return appender.err
}

// batchCount 返回已捕获的批次数量。
func (appender *captureBatchAppender) batchCount() int {
	appender.mu.Lock()
	defer appender.mu.Unlock()
	return len(appender.batches)
}

// totalEntries 返回所有批次中的日志总数。
func (appender *captureBatchAppender) totalEntries() int {
	appender.mu.Lock()
	defer appender.mu.Unlock()
	total := 0
	for _, batch := range appender.batches {
		total += len(batch)
	}
	return total
}

// TestTaskLogAsyncWriterFlushBySize 验证达到批量阈值后会自动 flush。
func TestTaskLogAsyncWriterFlushBySize(t *testing.T) {
	appender := &captureBatchAppender{}
	writer, err := NewAsyncWriter(AsyncWriterConfig{
		Appender:      appender,
		QueueSize:     4,
		BatchSize:     2,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("new async writer: %v", err)
	}
	defer shutdownAsyncWriter(t, writer)

	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write first log: %v", err)
	}
	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write second log: %v", err)
	}

	waitFor(t, func() bool { return appender.batchCount() == 1 && appender.totalEntries() == 2 })
}

// TestTaskLogAsyncWriterFlushByInterval 验证未达到批量阈值的日志会在定时器触发后落库。
func TestTaskLogAsyncWriterFlushByInterval(t *testing.T) {
	appender := &captureBatchAppender{}
	writer, err := NewAsyncWriter(AsyncWriterConfig{
		Appender:      appender,
		QueueSize:     4,
		BatchSize:     10,
		FlushInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new async writer: %v", err)
	}
	defer shutdownAsyncWriter(t, writer)

	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write interval log: %v", err)
	}

	waitFor(t, func() bool { return appender.batchCount() == 1 && appender.totalEntries() == 1 })
}

// TestTaskLogAsyncWriterFlushOnShutdown 验证关闭时会写出未达到批量阈值的日志。
func TestTaskLogAsyncWriterFlushOnShutdown(t *testing.T) {
	appender := &captureBatchAppender{}
	writer, err := NewAsyncWriter(AsyncWriterConfig{
		Appender:      appender,
		QueueSize:     4,
		BatchSize:     10,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("new async writer: %v", err)
	}
	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write queued log: %v", err)
	}
	if err := writer.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown writer: %v", err)
	}
	if appender.totalEntries() != 1 {
		t.Fatalf("expected shutdown flush to write one log, got %d", appender.totalEntries())
	}
}

// TestAsyncWriterQueueFullFailsFast 验证队列满时快速失败，避免阻塞任务主链路。
func TestAsyncWriterQueueFullFailsFast(t *testing.T) {
	appender := &blockingAppender{release: make(chan struct{}), startedCh: make(chan struct{})}
	writer, err := NewAsyncWriter(AsyncWriterConfig{
		Appender:      appender,
		QueueSize:     1,
		BatchSize:     1,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("new async writer: %v", err)
	}
	defer func() {
		close(appender.release)
		shutdownAsyncWriter(t, writer)
	}()

	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write first log: %v", err)
	}
	waitFor(t, func() bool { return appender.started() })
	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err != nil {
		t.Fatalf("write second log: %v", err)
	}
	if err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{TaskID: "task-1"}); err == nil {
		t.Fatalf("expected queue full error")
	}
}

type blockingAppender struct {
	once      sync.Once
	release   chan struct{}
	startedCh chan struct{}
}

// Append 阻塞首个批次写入，用于制造队列满场景。
func (appender *blockingAppender) Append(_ context.Context, _ []model.TaskLogEntry) error {
	appender.once.Do(func() {
		close(appender.startedCh)
	})
	<-appender.release
	return nil
}

// started 判断阻塞 appender 是否已经开始处理批次。
func (appender *blockingAppender) started() bool {
	select {
	case <-appender.startedCh:
		return true
	default:
		return false
	}
}

// shutdownAsyncWriter 在测试中带超时关闭异步写入器。
func shutdownAsyncWriter(t *testing.T, writer *AsyncWriter) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := writer.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown async writer: %v", err)
	}
}

// waitFor 等待异步断言条件在超时前成立。
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout: %s", fmt.Sprint(condition()))
}
