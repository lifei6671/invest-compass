package tasklog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

type serviceTestStore struct {
	appended    []model.TaskLogEntry
	logs        []model.TaskLogEntry
	task        model.Task
	taskOK      bool
	report      model.AnalysisReport
	reportOK    bool
	events      []model.TaskEvent
	diagnosis   model.TaskErrorDiagnosis
	diagnosisOK bool
}

// AppendTaskLogs 捕获 service 写入的任务日志。
func (store *serviceTestStore) AppendTaskLogs(_ context.Context, entries []model.TaskLogEntry) error {
	store.appended = append(store.appended, entries...)
	store.logs = append(store.logs, entries...)
	return nil
}

// ListTaskLogs 按测试查询条件过滤内存日志。
func (store *serviceTestStore) ListTaskLogs(_ context.Context, query dao.TaskLogQuery) (dao.TaskLogListResult, error) {
	limit := query.Limit
	if limit == 0 {
		limit = len(store.logs)
	}
	var entries []model.TaskLogEntry
	for _, entry := range store.logs {
		if entry.TaskID != query.TaskID || entry.ID <= query.AfterID {
			continue
		}
		if query.Level != "" && entry.Level != query.Level {
			continue
		}
		if query.Module != "" && entry.Module != query.Module {
			continue
		}
		if query.Stage != "" && entry.Stage != query.Stage {
			continue
		}
		if query.OnlyError && entry.Level != LevelError {
			continue
		}
		if query.Keyword != "" && !strings.Contains(entry.Message+" "+entry.Module+" "+entry.Stage, query.Keyword) {
			continue
		}
		entries = append(entries, entry)
		if len(entries) == limit {
			break
		}
	}
	nextID := int64(0)
	if len(entries) > 0 {
		nextID = entries[len(entries)-1].ID
	}
	return dao.TaskLogListResult{Entries: entries, NextID: nextID}, nil
}

// GetTaskLog 从内存日志中读取指定 ID。
func (store *serviceTestStore) GetTaskLog(_ context.Context, id int64) (model.TaskLogEntry, bool, error) {
	for _, entry := range store.logs {
		if entry.ID == id {
			return entry, true, nil
		}
	}
	return model.TaskLogEntry{}, false, nil
}

// UpsertTaskErrorDiagnosis 保存测试诊断结果。
func (store *serviceTestStore) UpsertTaskErrorDiagnosis(_ context.Context, diagnosis model.TaskErrorDiagnosis) error {
	store.diagnosis = diagnosis
	store.diagnosisOK = true
	return nil
}

// GetTaskErrorDiagnosis 读取测试诊断结果。
func (store *serviceTestStore) GetTaskErrorDiagnosis(_ context.Context, taskID string) (model.TaskErrorDiagnosis, bool, error) {
	if store.diagnosisOK && store.diagnosis.TaskID == taskID {
		return store.diagnosis, true, nil
	}
	return model.TaskErrorDiagnosis{}, false, nil
}

// GetTask 返回测试预设任务。
func (store *serviceTestStore) GetTask(_ context.Context, taskID string) (model.Task, bool, error) {
	if store.taskOK && store.task.ID == taskID {
		return store.task, true, nil
	}
	return model.Task{}, false, nil
}

// ListTaskEventsAfter 返回指定任务的后续事件。
func (store *serviceTestStore) ListTaskEventsAfter(_ context.Context, taskID string, afterID int64) ([]model.TaskEvent, error) {
	var events []model.TaskEvent
	for _, event := range store.events {
		if event.TaskID == taskID && event.ID > afterID {
			events = append(events, event)
		}
	}
	return events, nil
}

// GetAnalysisReportByTaskID 返回指定任务关联的测试报告。
func (store *serviceTestStore) GetAnalysisReportByTaskID(_ context.Context, taskID string) (model.AnalysisReport, bool, error) {
	if store.reportOK && store.report.TaskID == taskID {
		return store.report, true, nil
	}
	return model.AnalysisReport{}, false, nil
}

// TestTaskLogRedactionBeforePersist 验证任务日志在进入 DAO 前已经完成统一脱敏。
func TestTaskLogRedactionBeforePersist(t *testing.T) {
	store := &serviceTestStore{}
	service := Service{Store: store}

	err := service.Append(context.Background(), []model.TaskLogEntry{{
		TaskID:      "task-1",
		Level:       LevelInfo,
		Module:      "ai",
		Stage:       StagePromptBuild,
		Message:     "Authorization: Bearer placeholder-auth-secret",
		PayloadJSON: `{"api_key":"placeholder-provider-secret","proxy_password":"placeholder-proxy-secret"}`,
		RequestID:   "req-placeholder-safe",
		TraceID:     "trace-placeholder-safe",
	}})
	if err != nil {
		t.Fatalf("append task log: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected one appended log, got %+v", store.appended)
	}
	got := store.appended[0].Message + store.appended[0].PayloadJSON
	for _, secret := range []string{"placeholder-auth-secret", "placeholder-provider-secret", "placeholder-proxy-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("persisted log leaked %q: %+v", secret, store.appended[0])
		}
	}
	if !strings.Contains(got, logger.RedactedValue) {
		t.Fatalf("expected redaction marker before persist: %+v", store.appended[0])
	}
}

// TestTaskErrorDiagnosisFromXerr 验证没有持久化诊断时会从最后一条 ERROR 日志生成可重试诊断。
func TestTaskErrorDiagnosisFromXerr(t *testing.T) {
	store := &serviceTestStore{logs: []model.TaskLogEntry{{
		ID:        9,
		TaskID:    "task-failed",
		Level:     LevelError,
		Module:    "ai",
		Stage:     StageStreamFailed,
		Code:      "provider_timeout",
		Message:   "Provider timeout",
		RequestID: "req-1",
		TraceID:   "trace-1",
		Retryable: true,
	}}}
	service := Service{Store: store}

	diagnosis, ok, err := service.Diagnosis(context.Background(), "task-failed")
	if err != nil {
		t.Fatalf("diagnosis: %v", err)
	}
	if !ok || !diagnosis.Retryable || diagnosis.SourceLogID != 9 {
		t.Fatalf("unexpected diagnosis: ok=%v diagnosis=%+v", ok, diagnosis)
	}
	if !strings.Contains(diagnosis.Summary, "超时") {
		t.Fatalf("expected timeout diagnosis, got %+v", diagnosis)
	}
}

// TestTaskLogContextSummaryRedacted 验证上下文摘要只回显安全摘要，不暴露持仓和密钥原文。
func TestTaskLogContextSummaryRedacted(t *testing.T) {
	store := &serviceTestStore{
		task:   model.Task{ID: "task-1", Type: "个股综合分析", UpdatedAt: time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)},
		taskOK: true,
		report: model.AnalysisReport{
			TaskID:           "task-1",
			Symbol:           "CN:SH:600183",
			AnalysisType:     "个股综合分析",
			ModelName:        "DeepSeek-V3",
			PromptTemplateID: 7,
			InputSnapshot:    `{"quote":{},"kline":[],"indicator":{},"news":[],"user_position":"100股成本价12.34","api_key":"placeholder-context-key"}`,
		},
		reportOK: true,
	}
	service := Service{Store: store}

	summary, ok, err := service.Context(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("context summary: %v", err)
	}
	if !ok || summary.UserPosition != "已提供（已脱敏）" {
		t.Fatalf("unexpected context summary: ok=%v summary=%+v", ok, summary)
	}
	if strings.Contains(summary.RawSnapshotBrief, "100股成本价12.34") || strings.Contains(summary.RawSnapshotBrief, "placeholder-context-key") {
		t.Fatalf("context summary leaked sensitive input: %+v", summary)
	}
	if summary.QuoteStatus != "已加载" || summary.NewsStatus != "已加载" {
		t.Fatalf("expected loaded context flags, got %+v", summary)
	}
}

// TestTaskLogsExportRedacted 验证导出的日志包包含 raw-json-sample 且导出前二次脱敏。
func TestTaskLogsExportRedacted(t *testing.T) {
	createdAt := time.Date(2025, 5, 20, 15, 29, 46, 0, time.UTC)
	store := &serviceTestStore{
		task:   model.Task{ID: "task-1", Type: "AI 分析", Title: "生益科技 个股综合分析", Status: "FAILED", StartedAt: createdAt.Add(-time.Minute), UpdatedAt: createdAt},
		taskOK: true,
		logs: []model.TaskLogEntry{{
			ID:          1,
			TaskID:      "task-1",
			Ts:          createdAt,
			Level:       LevelError,
			Module:      "ai",
			Stage:       StageStreamFailed,
			Message:     "Authorization: Bearer placeholder-export-secret",
			Code:        "provider_timeout",
			PayloadJSON: `{"Authorization":"Bearer placeholder-json-secret","message":"timeout"}`,
			RequestID:   "req-1",
			TraceID:     "trace-1",
			Retryable:   true,
		}},
		events: []model.TaskEvent{{
			ID:        1,
			TaskID:    "task-1",
			EventType: "TASK_FAILED",
			Payload:   `{"api_key":"placeholder-event-secret"}`,
			CreatedAt: createdAt,
		}},
		report:   model.AnalysisReport{TaskID: "task-1", Symbol: "CN:SH:600183", AnalysisType: "个股综合分析", ModelName: "DeepSeek-V3"},
		reportOK: true,
	}
	service := Service{Store: store, Now: func() time.Time { return createdAt }}

	bundle, err := service.Export(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("export task logs: %v", err)
	}
	if !strings.Contains(bundle.Content, "## 原始 JSON 示例") {
		t.Fatalf("export should include raw-json-sample, got:\n%s", bundle.Content)
	}
	for _, secret := range []string{"placeholder-export-secret", "placeholder-json-secret", "placeholder-event-secret"} {
		if strings.Contains(bundle.Content, secret) {
			t.Fatalf("export leaked %q:\n%s", secret, bundle.Content)
		}
	}
	if !strings.Contains(bundle.Content, logger.RedactedValue) {
		t.Fatalf("expected redaction marker in export:\n%s", bundle.Content)
	}
}
