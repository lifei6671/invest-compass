package tasklog

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

type adapterCaptureStageWriter struct {
	entries []model.TaskLogEntry
	err     error
}

func (writer *adapterCaptureStageWriter) WriteTaskLog(_ context.Context, entry model.TaskLogEntry) error {
	writer.entries = append(writer.entries, entry)
	return writer.err
}

// TestWriterAppenderAdaptsStageWriter 验证单条 writer 可以安全接入 AsyncWriter 批量写入边界。
func TestWriterAppenderAdaptsStageWriter(t *testing.T) {
	writer := &adapterCaptureStageWriter{}
	appender := WriterAppender{Writer: writer}

	if err := appender.Append(context.Background(), []model.TaskLogEntry{
		{TaskID: "task-1", Message: "first"},
		{TaskID: "task-1", Message: "second"},
	}); err != nil {
		t.Fatalf("append entries: %v", err)
	}
	if len(writer.entries) != 2 {
		t.Fatalf("expected two entries to be forwarded, got %d", len(writer.entries))
	}
}

// TestMultiAppenderWritesEveryTarget 验证组合写入器会把同一批日志写入每个后端。
func TestMultiAppenderWritesEveryTarget(t *testing.T) {
	first := &captureBatchAppender{}
	second := &captureBatchAppender{}
	appender := NewMultiAppender(first, second)

	if err := appender.Append(context.Background(), []model.TaskLogEntry{{TaskID: "task-1"}}); err != nil {
		t.Fatalf("append entries: %v", err)
	}
	if first.totalEntries() != 1 || second.totalEntries() != 1 {
		t.Fatalf("expected both appenders to receive one entry, got first=%d second=%d", first.totalEntries(), second.totalEntries())
	}
}

// TestMultiAppenderReturnsJoinedErrors 验证多个后端失败时不会吞掉错误。
func TestMultiAppenderReturnsJoinedErrors(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	appender := NewMultiAppender(
		&captureBatchAppender{err: firstErr},
		&captureBatchAppender{err: secondErr},
	)

	err := appender.Append(context.Background(), []model.TaskLogEntry{{TaskID: "task-1"}})
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("expected joined errors, got %v", err)
	}
}
