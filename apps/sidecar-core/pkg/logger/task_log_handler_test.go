package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

type captureTaskLogWriter struct {
	entries []model.TaskLogEntry
}

// WriteTaskLog 捕获 slog handler 生成的任务日志。
func (writer *captureTaskLogWriter) WriteTaskLog(_ context.Context, entry model.TaskLogEntry) error {
	writer.entries = append(writer.entries, entry)
	return nil
}

// TestTaskLogHandlerSkipsApplicationLogWithoutTaskID 验证普通应用日志不会进入任务日志表。
func TestTaskLogHandlerSkipsApplicationLogWithoutTaskID(t *testing.T) {
	writer := &captureTaskLogWriter{}
	handler := NewTaskLogHandler(nil, writer)
	record := slog.NewRecord(time.Date(2025, 5, 20, 15, 28, 40, 0, time.UTC), slog.LevelInfo, "app boot", 0)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("handle app log: %v", err)
	}
	if len(writer.entries) != 0 {
		t.Fatalf("expected no task logs, got %+v", writer.entries)
	}
}

// TestTaskLogHandlerWritesRedactedTaskLog 验证 slog.Record 能稳定转换为脱敏任务结构化日志。
func TestTaskLogHandlerWritesRedactedTaskLog(t *testing.T) {
	writer := &captureTaskLogWriter{}
	handler := NewTaskLogHandler(nil, writer)
	record := slog.NewRecord(time.Date(2025, 5, 20, 15, 28, 40, 123000000, time.UTC), slog.LevelError, "failed Authorization: Bearer secret-token", 0)
	record.AddAttrs(
		slog.String(FieldTaskID, "task-1"),
		slog.String(FieldRequestID, "req-1"),
		slog.String(FieldTraceID, "trace-1"),
		slog.String(FieldModule, "ai"),
		slog.String(FieldStage, "stream_failed"),
		slog.String(FieldProvider, "DeepSeek"),
		slog.String(FieldModel, "DeepSeek-V3"),
		slog.String(FieldSymbol, "CN:SH:600183"),
		slog.String(FieldCode, "provider_timeout"),
		slog.Int(FieldDurationMS, 1200),
		slog.Bool(FieldRetryable, true),
		slog.String("api_key", "sk-secret"),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("handle task log: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one task log, got %+v", writer.entries)
	}
	entry := writer.entries[0]
	if entry.Level != "ERROR" || entry.Module != "ai" || entry.Stage != "stream_failed" {
		t.Fatalf("unexpected entry fields: %+v", entry)
	}
	if entry.DurationMS != 1200 || !entry.Retryable {
		t.Fatalf("unexpected duration/retryable: %+v", entry)
	}
	for _, secret := range []string{"secret-token", "sk-secret"} {
		if strings.Contains(entry.Message, secret) || strings.Contains(entry.PayloadJSON, secret) {
			t.Fatalf("task log leaked secret %q: %+v", secret, entry)
		}
	}
	if !strings.Contains(entry.PayloadJSON, RedactedValue) {
		t.Fatalf("expected payload json to contain redaction marker: %s", entry.PayloadJSON)
	}
}

// TestTaskLogHandlerRejectsMissingModuleOrStage 验证 task_id 日志缺少阶段字段时快速失败。
func TestTaskLogHandlerRejectsMissingModuleOrStage(t *testing.T) {
	writer := &captureTaskLogWriter{}
	handler := NewTaskLogHandler(nil, writer)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "missing stage", 0)
	record.AddAttrs(slog.String(FieldTaskID, "task-1"), slog.String(FieldModule, "ai"))

	if err := handler.Handle(context.Background(), record); err == nil {
		t.Fatalf("expected missing stage to fail")
	}
	if len(writer.entries) != 0 {
		t.Fatalf("expected no task logs, got %+v", writer.entries)
	}
}

// TestTaskLogHandlerKeepsGroupedPayload 验证 slog.WithGroup 仍能提取任务字段，且业务字段保留分组前缀。
func TestTaskLogHandlerKeepsGroupedPayload(t *testing.T) {
	writer := &captureTaskLogWriter{}
	handler := NewTaskLogHandler(nil, writer).WithGroup("ai")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "stage ok", 0)
	record.AddAttrs(
		slog.String(FieldTaskID, "task-1"),
		slog.String(FieldModule, "analysis"),
		slog.String(FieldStage, "prompt_build"),
		slog.String("prompt_summary", "Authorization: Bearer secret-token"),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("handle grouped task log: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one task log, got %+v", writer.entries)
	}
	entry := writer.entries[0]
	if entry.TaskID != "task-1" || entry.Module != "analysis" || entry.Stage != "prompt_build" {
		t.Fatalf("unexpected grouped entry fields: %+v", entry)
	}
	if !strings.Contains(entry.PayloadJSON, "ai.prompt_summary") {
		t.Fatalf("expected grouped payload key, got %s", entry.PayloadJSON)
	}
	if strings.Contains(entry.PayloadJSON, "secret-token") {
		t.Fatalf("grouped payload leaked secret: %s", entry.PayloadJSON)
	}
}
