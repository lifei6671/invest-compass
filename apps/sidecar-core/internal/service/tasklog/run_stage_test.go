package tasklog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

type captureStageWriter struct {
	entries []model.TaskLogEntry
	err     error
}

func (writer *captureStageWriter) WriteTaskLog(_ context.Context, entry model.TaskLogEntry) error {
	writer.entries = append(writer.entries, entry)
	return writer.err
}

// TestRunStageSuccess 验证阶段成功时写入 INFO 日志和耗时。
func TestRunStageSuccess(t *testing.T) {
	writer := &captureStageWriter{}
	now := steppingNow(
		time.Date(2025, 5, 20, 15, 28, 40, 0, time.UTC),
		time.Date(2025, 5, 20, 15, 28, 41, 200000000, time.UTC),
		time.Date(2025, 5, 20, 15, 28, 41, 200000000, time.UTC),
	)
	err := RunStage(context.Background(), writer, StageMeta{
		TaskID:    "task-1",
		RequestID: "req-1",
		TraceID:   "trace-1",
		Module:    "analysis",
		Provider:  "DeepSeek",
		Model:     "DeepSeek-V3",
		Symbol:    "CN:SH:600183",
		Now:       now,
	}, StagePromptBuild, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one entry, got %+v", writer.entries)
	}
	entry := writer.entries[0]
	if entry.Level != LevelInfo || entry.Stage != StagePromptBuild || entry.DurationMS != 1200 {
		t.Fatalf("unexpected success entry: %+v", entry)
	}
	if !strings.Contains(entry.PayloadJSON, `"status":"success"`) {
		t.Fatalf("expected success payload, got %s", entry.PayloadJSON)
	}
}

// TestRunStageFailed 验证阶段失败时写入 ERROR 日志并脱敏。
func TestRunStageFailed(t *testing.T) {
	writer := &captureStageWriter{}
	stageErr := &xerr.Error{Code: xerr.AICancelled, Message: "Authorization: Bearer secret-token"}
	err := RunStage(context.Background(), writer, StageMeta{
		TaskID:    "task-1",
		Module:    "analysis",
		Retryable: true,
	}, StageStreamFailed, func(context.Context) error {
		return stageErr
	})
	if !errors.Is(err, stageErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one entry, got %+v", writer.entries)
	}
	entry := writer.entries[0]
	if entry.Level != LevelError || entry.Code != string(xerr.AICancelled) || !entry.Retryable {
		t.Fatalf("unexpected error entry: %+v", entry)
	}
	if strings.Contains(entry.Message, "secret-token") || strings.Contains(entry.PayloadJSON, "secret-token") {
		t.Fatalf("stage log leaked secret: %+v", entry)
	}
	if !strings.Contains(entry.Message, logger.RedactedValue) {
		t.Fatalf("expected redacted message, got %q", entry.Message)
	}
}

// TestRunStageReturnsWriteError 验证日志写入失败不会被静默吞掉。
func TestRunStageReturnsWriteError(t *testing.T) {
	writeErr := errors.New("sqlite locked")
	writer := &captureStageWriter{err: writeErr}
	err := RunStage(context.Background(), writer, StageMeta{TaskID: "task-1", Module: "analysis"}, StageQuoteFetch, func(context.Context) error {
		return nil
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
}

func steppingNow(values ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}
