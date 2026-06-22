package dao

import (
	"context"
	"fmt"
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
	if entry.PayloadJSON != `{"code":"provider_timeout"}` {
		t.Fatalf("payload_json should remain stable, got %q", entry.PayloadJSON)
	}

	ordered, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: 20})
	if err != nil {
		t.Fatalf("list ordered logs: %v", err)
	}
	gotLevels := []string{ordered.Entries[0].Level, ordered.Entries[1].Level, ordered.Entries[2].Level}
	wantLevels := []string{"INFO", "WARN", "ERROR"}
	for index := range wantLevels {
		if gotLevels[index] != wantLevels[index] {
			t.Fatalf("unexpected level order: got=%v want=%v", gotLevels, wantLevels)
		}
	}
}

// TestAppendTaskLogsBatch 验证 DAO 支持批量写入任务结构化日志。
func TestAppendTaskLogsBatch(t *testing.T) {
	store, ctx, _ := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: 10})
	if err != nil {
		t.Fatalf("list task logs: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected three task-1 logs after batch append, got %+v", result.Entries)
	}
}

// TestListTaskLogsByTaskID 验证日志列表只返回指定任务的数据。
func TestListTaskLogsByTaskID(t *testing.T) {
	store, ctx, _ := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-2", Limit: 10})
	if err != nil {
		t.Fatalf("list by task id: %v", err)
	}
	if len(result.Entries) != 1 || result.Entries[0].TaskID != "task-2" {
		t.Fatalf("unexpected task id filter result: %+v", result.Entries)
	}
}

// TestListTaskLogsByLevel 验证日志级别筛选只返回匹配级别。
func TestListTaskLogsByLevel(t *testing.T) {
	store, ctx, _ := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Level: "ERROR", Limit: 10})
	if err != nil {
		t.Fatalf("list by level: %v", err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Level != "ERROR" {
		t.Fatalf("unexpected level filter result: %+v", result.Entries)
	}
}

// TestListTaskLogsByModuleStage 验证模块和阶段组合筛选用于日志抽屉阶段过滤。
func TestListTaskLogsByModuleStage(t *testing.T) {
	store, ctx, _ := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Module: "ai", Stage: "stream_timeout", Limit: 10})
	if err != nil {
		t.Fatalf("list by module stage: %v", err)
	}
	if len(result.Entries) != 1 || !result.Entries[0].Retryable {
		t.Fatalf("unexpected module/stage result: %+v", result.Entries)
	}
}

// TestListTaskLogsByKeyword 验证关键字筛选覆盖消息和错误码。
func TestListTaskLogsByKeyword(t *testing.T) {
	store, ctx, _ := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Keyword: "provider_timeout", Limit: 10})
	if err != nil {
		t.Fatalf("list by keyword: %v", err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Code != "provider_timeout" {
		t.Fatalf("unexpected keyword result: %+v", result.Entries)
	}
}

// TestGetTaskLog 验证单条日志详情读取用于原始 JSON 面板。
func TestGetTaskLog(t *testing.T) {
	store, ctx, entries := seedTaskLogFilterFixture(t)
	result, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Level: "ERROR", Limit: 10})
	if err != nil {
		t.Fatalf("list error log: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected one error log, got %+v", result.Entries)
	}
	entry, ok, err := store.GetTaskLog(ctx, result.Entries[0].ID)
	if err != nil {
		t.Fatalf("get task log: %v", err)
	}
	if !ok || entry.PayloadJSON != entries[2].PayloadJSON {
		t.Fatalf("unexpected detail entry: ok=%v entry=%+v", ok, entry)
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

// TestTaskLogRepositoryPrunesExpiredLogsOnly 验证日志保留策略不删除任务、事件、报告和设置。
func TestTaskLogRepositoryPrunesExpiredLogsOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	if err := store.db.WithContext(ctx).Create(&model.Task{ID: "task-1", Type: "AI 分析", Status: "SUCCESS"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.TaskEvent{TaskID: "task-1", EventType: "TASK_SUCCESS"}).Error; err != nil {
		t.Fatalf("seed task event: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.AnalysisReport{TaskID: "task-1", Symbol: "CN:SH:600183", Title: "report", AnalysisType: "stock_full"}).Error; err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.Setting{Key: "theme", Value: "light"}).Error; err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	if err := store.AppendTaskLogs(ctx, []model.TaskLogEntry{
		{TaskID: "task-1", Ts: now.AddDate(0, 0, -40), Level: "INFO", Module: "ai", Stage: "old", Message: "old"},
		{TaskID: "task-1", Ts: now.AddDate(0, 0, -1), Level: "INFO", Module: "ai", Stage: "new", Message: "new"},
	}); err != nil {
		t.Fatalf("append logs: %v", err)
	}

	result, err := store.PruneTaskLogs(ctx, TaskLogRetentionOptions{Before: now.AddDate(0, 0, -30)})
	if err != nil {
		t.Fatalf("prune task logs: %v", err)
	}
	if result.DeletedExpired != 1 {
		t.Fatalf("expected one expired log deleted, got %+v", result)
	}

	logs, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: 10})
	if err != nil {
		t.Fatalf("list logs after prune: %v", err)
	}
	if len(logs.Entries) != 1 || logs.Entries[0].Stage != "new" {
		t.Fatalf("unexpected remaining logs: %+v", logs.Entries)
	}
	requireCount(t, store, &model.Task{}, 1)
	requireCount(t, store, &model.TaskEvent{}, 1)
	requireCount(t, store, &model.AnalysisReport{}, 1)
	requireCount(t, store, &model.Setting{}, 1)
}

// TestTaskLogRepositoryPrunesPerTaskAndTotalOverflow 验证单任务和全库上限按最旧日志裁剪。
func TestTaskLogRepositoryPrunesPerTaskAndTotalOverflow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	for taskIndex := 1; taskIndex <= 2; taskIndex++ {
		for logIndex := 1; logIndex <= 4; logIndex++ {
			if err := store.AppendTaskLogs(ctx, []model.TaskLogEntry{{
				TaskID:  fmt.Sprintf("task-%d", taskIndex),
				Ts:      now.Add(time.Duration(taskIndex*10+logIndex) * time.Second),
				Level:   "INFO",
				Module:  "ai",
				Stage:   "stage",
				Message: "log",
			}}); err != nil {
				t.Fatalf("append log task=%d index=%d: %v", taskIndex, logIndex, err)
			}
		}
	}

	result, err := store.PruneTaskLogs(ctx, TaskLogRetentionOptions{PerTaskLimit: 3, TotalLimit: 5})
	if err != nil {
		t.Fatalf("prune task logs: %v", err)
	}
	if result.DeletedPerTaskOverflow != 2 || result.DeletedTotalOverflow != 1 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	requireCount(t, store, &model.TaskLogEntry{}, 5)

	taskOne, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-1", Limit: 10})
	if err != nil {
		t.Fatalf("list task-1 logs: %v", err)
	}
	if len(taskOne.Entries) != 2 {
		t.Fatalf("expected task-1 to have 2 logs after total overflow pruning, got %+v", taskOne.Entries)
	}
	taskTwo, err := store.ListTaskLogs(ctx, TaskLogQuery{TaskID: "task-2", Limit: 10})
	if err != nil {
		t.Fatalf("list task-2 logs: %v", err)
	}
	if len(taskTwo.Entries) != 3 {
		t.Fatalf("expected task-2 to have 3 logs after pruning, got %+v", taskTwo.Entries)
	}
}

// TestTaskErrorDiagnosisRepositoryUpsertsAndReads 验证失败任务诊断按 task_id 幂等保存和回放。
func TestTaskErrorDiagnosisRepositoryUpsertsAndReads(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	diagnosis := model.TaskErrorDiagnosis{
		TaskID:          "task-failed",
		ErrorCode:       "provider_timeout",
		ErrorStage:      "stream_failed",
		Summary:         "模型服务响应超时",
		CausesJSON:      `["模型服务响应超时"]`,
		SuggestionsJSON: `["更换模型后重试"]`,
		Retryable:       true,
		RequestID:       "req-1",
		TraceID:         "trace-1",
		SourceLogID:     7,
	}
	if err := store.UpsertTaskErrorDiagnosis(ctx, diagnosis); err != nil {
		t.Fatalf("upsert diagnosis: %v", err)
	}
	diagnosis.Summary = "代理或模型服务响应超时"
	diagnosis.SuggestionsJSON = `["检查代理设置","更换模型后重试"]`
	if err := store.UpsertTaskErrorDiagnosis(ctx, diagnosis); err != nil {
		t.Fatalf("upsert diagnosis second time: %v", err)
	}

	got, ok, err := store.GetTaskErrorDiagnosis(ctx, "task-failed")
	if err != nil {
		t.Fatalf("get diagnosis: %v", err)
	}
	if !ok {
		t.Fatalf("expected diagnosis to exist")
	}
	if got.Summary != "代理或模型服务响应超时" || !strings.Contains(got.SuggestionsJSON, "检查代理设置") {
		t.Fatalf("unexpected diagnosis after upsert: %+v", got)
	}
	if got.ID == 0 {
		t.Fatalf("expected diagnosis id to be populated")
	}
}

// TestTaskErrorDiagnosisRepositoryRejectsInvalidInput 验证诊断 DAO 对缺失核心字段快速失败。
func TestTaskErrorDiagnosisRepositoryRejectsInvalidInput(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertTaskErrorDiagnosis(ctx, model.TaskErrorDiagnosis{}); err == nil {
		t.Fatalf("expected empty diagnosis to fail")
	}
	if _, _, err := store.GetTaskErrorDiagnosis(ctx, ""); err == nil {
		t.Fatalf("expected empty task id to fail")
	}
	_, ok, err := store.GetTaskErrorDiagnosis(ctx, "missing")
	if err != nil {
		t.Fatalf("get missing diagnosis: %v", err)
	}
	if ok {
		t.Fatalf("expected missing diagnosis to return ok=false")
	}
}

// seedTaskLogFilterFixture 写入任务日志筛选测试所需的多级别样本。
func seedTaskLogFilterFixture(t *testing.T) (*Store, context.Context, []model.TaskLogEntry) {
	t.Helper()
	store := newTestStore(t)
	ctx := context.Background()
	baseTime := time.Date(2025, 5, 20, 15, 28, 40, 123000000, time.UTC)
	entries := []model.TaskLogEntry{
		{TaskID: "task-1", Ts: baseTime, Level: "INFO", Module: "market", Stage: "quote_fetch", Message: "开始拉取行情", PayloadJSON: `{"rows":1}`},
		{TaskID: "task-1", Ts: baseTime.Add(time.Second), Level: "WARN", Module: "ai", Stage: "stream_timeout", Message: "模型流式响应耗时过长", Retryable: true, PayloadJSON: `{"retryable":true}`},
		{TaskID: "task-1", Ts: baseTime.Add(2 * time.Second), Level: "ERROR", Module: "ai", Stage: "stream_failed", Message: "Provider 响应超时，任务终止", Code: "provider_timeout", PayloadJSON: `{"code":"provider_timeout"}`},
		{TaskID: "task-2", Ts: baseTime, Level: "INFO", Module: "market", Stage: "quote_fetch", Message: "其他任务日志", PayloadJSON: `{}`},
	}
	if err := store.AppendTaskLogs(ctx, entries); err != nil {
		t.Fatalf("append task log fixture: %v", err)
	}
	return store, ctx, entries
}
