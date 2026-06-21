package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestTaskLogRepositoryPersistsAndFilters 验证任务结构化日志支持写入、筛选、分页和详情读取。
func TestTaskLogRepositoryPersistsAndFilters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	baseTime := time.Date(2025, 5, 20, 15, 28, 40, 123000000, time.UTC)
	entries := []model.TaskLogEntry{
		{
			TaskID:      "task-1",
			RequestID:   "req-1",
			TraceID:     "trace-1",
			Ts:          baseTime,
			Level:       "INFO",
			Module:      "market",
			Stage:       "quote_fetch",
			Message:     "开始拉取行情",
			Provider:    "EastMoney",
			Symbol:      "CN:SH:600183",
			DurationMS:  12,
			PayloadJSON: `{"rows":1}`,
		},
		{
			TaskID:      "task-1",
			RequestID:   "req-1",
			TraceID:     "trace-1",
			Ts:          baseTime.Add(time.Second),
			Level:       "WARN",
			Module:      "ai",
			Stage:       "stream_timeout",
			Message:     "模型流式响应耗时过长",
			Model:       "DeepSeek-V3",
			DurationMS:  53000,
			Retryable:   true,
			PayloadJSON: `{"retryable":true}`,
		},
		{
			TaskID:      "task-1",
			RequestID:   "req-1",
			TraceID:     "trace-1",
			Ts:          baseTime.Add(2 * time.Second),
			Level:       "ERROR",
			Module:      "ai",
			Stage:       "stream_failed",
			Message:     "Provider 响应超时，任务终止",
			Code:        "provider_timeout",
			Model:       "DeepSeek-V3",
			PayloadJSON: `{"code":"provider_timeout"}`,
		},
		{
			TaskID:      "task-2",
			Ts:          baseTime,
			Level:       "INFO",
			Module:      "market",
			Stage:       "quote_fetch",
			Message:     "其他任务日志",
			PayloadJSON: `{}`,
		},
	}
	if err := store.AppendTaskLogs(ctx, entries); err != nil {
		t.Fatalf("append task logs: %v", err)
	}

	levelResult, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Level: "ERROR", Limit: 20})
	if err != nil {
		t.Fatalf("list by level: %v", err)
	}
	if len(levelResult.Entries) != 1 || levelResult.Entries[0].Stage != "stream_failed" {
		t.Fatalf("unexpected level filter result: %+v", levelResult)
	}

	stageResult, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Module: "ai", Stage: "stream_timeout", Limit: 20})
	if err != nil {
		t.Fatalf("list by module stage: %v", err)
	}
	if len(stageResult.Entries) != 1 || !stageResult.Entries[0].Retryable {
		t.Fatalf("unexpected stage filter result: %+v", stageResult)
	}

	keywordResult, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Keyword: "响应超时", Limit: 20})
	if err != nil {
		t.Fatalf("list by keyword: %v", err)
	}
	if len(keywordResult.Entries) != 1 || keywordResult.Entries[0].Code != "provider_timeout" {
		t.Fatalf("unexpected keyword result: %+v", keywordResult)
	}

	page, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page.Entries) != 2 || !page.HasMore || page.NextID == 0 {
		t.Fatalf("unexpected first page: %+v", page)
	}
	nextPage, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", AfterID: page.NextID, Limit: 2})
	if err != nil {
		t.Fatalf("list next page: %v", err)
	}
	if len(nextPage.Entries) != 1 || nextPage.HasMore {
		t.Fatalf("unexpected next page: %+v", nextPage)
	}

	entry, ok, err := store.GetTaskLog(ctx, nextPage.Entries[0].ID)
	if err != nil {
		t.Fatalf("get task log: %v", err)
	}
	if !ok || !strings.Contains(entry.PayloadJSON, "provider_timeout") {
		t.Fatalf("unexpected task log detail: ok=%v entry=%+v", ok, entry)
	}
}

// TestTaskLogRepositoryRejectsInvalidBoundaries 验证 DAO 对无界查询和缺失标识快速失败。
func TestTaskLogRepositoryRejectsInvalidBoundaries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.AppendTaskLogs(ctx, []model.TaskLogEntry{{Level: "INFO", Module: "ai", Stage: "prompt_build", Message: "bad"}}); err == nil {
		t.Fatalf("expected empty task id append to fail")
	}
	if _, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "", Limit: 10}); err == nil {
		t.Fatalf("expected empty task id query to fail")
	}
	if _, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: -1}); err == nil {
		t.Fatalf("expected negative limit to fail")
	}
	if _, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: maxTaskLogLimit + 1}); err == nil {
		t.Fatalf("expected too large limit to fail")
	}
	if _, _, err := store.GetTaskLog(ctx, 0); err == nil {
		t.Fatalf("expected non-positive id to fail")
	}
	_, ok, err := store.GetTaskLog(ctx, 999)
	if err != nil {
		t.Fatalf("get missing log: %v", err)
	}
	if ok {
		t.Fatalf("expected missing log to return ok=false")
	}
}
