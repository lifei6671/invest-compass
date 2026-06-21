package tasklog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Service 统一封装任务结构化日志的查询、脱敏和导出逻辑。
type Service struct {
	Store Store
	Now   func() time.Time
}

// Append 写入任务结构化日志；入库前会对 message 与 payload_json 做统一脱敏。
func (service Service) Append(ctx context.Context, entries []model.TaskLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if service.Store == nil {
		return fmt.Errorf("task log store is required")
	}
	sanitized := make([]model.TaskLogEntry, 0, len(entries))
	for _, entry := range entries {
		entry.TaskID = strings.TrimSpace(entry.TaskID)
		entry.Level = strings.TrimSpace(entry.Level)
		entry.Module = strings.TrimSpace(entry.Module)
		entry.Stage = strings.TrimSpace(entry.Stage)
		if entry.Ts.IsZero() {
			entry.Ts = service.now()
		}
		entry.Message = logger.RedactText(strings.TrimSpace(entry.Message))
		entry.PayloadJSON = sanitizeJSON(entry.PayloadJSON)
		entry.RequestID = logger.RedactText(entry.RequestID)
		entry.TraceID = logger.RedactText(entry.TraceID)
		entry.Provider = logger.RedactText(entry.Provider)
		entry.Model = logger.RedactText(entry.Model)
		entry.Symbol = logger.RedactText(entry.Symbol)
		sanitized = append(sanitized, entry)
	}
	return service.Store.AppendTaskLogs(ctx, sanitized)
}

// List 返回任务日志列表，按 id 升序用于首屏和运行中增量拉取。
func (service Service) List(ctx context.Context, query Query) (ListResult, error) {
	if service.Store == nil {
		return ListResult{}, fmt.Errorf("task log store is required")
	}
	taskID := strings.TrimSpace(query.TaskID)
	if taskID == "" {
		return ListResult{}, fmt.Errorf("task_id is required")
	}
	limit := query.Limit
	if limit == 0 {
		limit = defaultServiceLimit
	}
	if limit < 0 || limit > maxServiceLimit {
		return ListResult{}, fmt.Errorf("task log limit must be between 1 and %d", maxServiceLimit)
	}
	result, err := service.Store.ListTaskLogs(ctx, dao.TaskLogQuery{
		TaskID:    taskID,
		Level:     normalizeOption(query.Level, "全部级别"),
		Module:    strings.TrimSpace(query.Module),
		Stage:     normalizeOption(query.Stage, "全部阶段"),
		Keyword:   strings.TrimSpace(query.Keyword),
		OnlyError: query.OnlyError,
		AfterID:   query.AfterID,
		Limit:     limit,
	})
	if err != nil {
		return ListResult{}, err
	}
	rows := make([]Row, 0, len(result.Entries))
	for _, entry := range result.Entries {
		rows = append(rows, entryToRow(entry))
	}
	return ListResult{Rows: rows, NextAfterID: result.NextID, HasMore: result.HasMore}, nil
}

// Get 返回单条日志和脱敏 JSON 详情。
func (service Service) Get(ctx context.Context, id int64) (Detail, bool, error) {
	if service.Store == nil {
		return Detail{}, false, fmt.Errorf("task log store is required")
	}
	entry, ok, err := service.Store.GetTaskLog(ctx, id)
	if err != nil || !ok {
		return Detail{}, ok, err
	}
	return Detail{Row: entryToRow(entry), RawJSON: rawJSONForEntry(entry)}, true, nil
}

// Summary 返回日志抽屉头部基础信息。
func (service Service) Summary(ctx context.Context, taskID string) (Summary, bool, error) {
	if service.Store == nil {
		return Summary{}, false, fmt.Errorf("task log store is required")
	}
	task, ok, err := service.Store.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil || !ok {
		return Summary{}, ok, err
	}
	logs, err := service.Store.ListTaskLogs(ctx, dao.TaskLogQuery{TaskID: task.ID, Limit: 1})
	if err != nil {
		return Summary{}, false, err
	}
	requestID, traceID := "", ""
	if len(logs.Entries) > 0 {
		requestID = logs.Entries[0].RequestID
		traceID = logs.Entries[0].TraceID
	}
	return Summary{
		Title:     logger.RedactText(task.Title),
		TaskID:    task.ID,
		TaskType:  task.Type,
		StartedAt: formatTime(task.StartedAt),
		Duration:  taskDuration(task),
		RequestID: requestID,
		TraceID:   traceID,
		Status:    task.Status,
	}, true, nil
}

// UpsertDiagnosis 保存失败任务诊断摘要；数组字段会以 JSON 字符串持久化。
func (service Service) UpsertDiagnosis(ctx context.Context, diagnosis Diagnosis) error {
	if service.Store == nil {
		return fmt.Errorf("task log store is required")
	}
	causesJSON, err := marshalStringList(diagnosis.Causes)
	if err != nil {
		return err
	}
	suggestionsJSON, err := marshalStringList(diagnosis.Suggestions)
	if err != nil {
		return err
	}
	return service.Store.UpsertTaskErrorDiagnosis(ctx, model.TaskErrorDiagnosis{
		TaskID:          strings.TrimSpace(diagnosis.TaskID),
		ErrorCode:       strings.TrimSpace(diagnosis.ErrorCode),
		ErrorStage:      strings.TrimSpace(diagnosis.ErrorStage),
		Summary:         logger.RedactText(diagnosis.Summary),
		CausesJSON:      logger.RedactText(causesJSON),
		SuggestionsJSON: logger.RedactText(suggestionsJSON),
		Retryable:       diagnosis.Retryable,
		RequestID:       logger.RedactText(diagnosis.RequestID),
		TraceID:         logger.RedactText(diagnosis.TraceID),
		SourceLogID:     diagnosis.SourceLogID,
	})
}

// Diagnosis 返回失败任务诊断；没有持久化诊断时会基于最后一条 ERROR 生成保守诊断。
func (service Service) Diagnosis(ctx context.Context, taskID string) (Diagnosis, bool, error) {
	if service.Store == nil {
		return Diagnosis{}, false, fmt.Errorf("task log store is required")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Diagnosis{}, false, fmt.Errorf("task_id is required")
	}
	persisted, ok, err := service.Store.GetTaskErrorDiagnosis(ctx, taskID)
	if err != nil {
		return Diagnosis{}, false, err
	}
	if ok {
		return diagnosisFromModel(persisted), true, nil
	}
	result, err := service.Store.ListTaskLogs(ctx, dao.TaskLogQuery{TaskID: taskID, OnlyError: true, Limit: maxServiceLimit})
	if err != nil {
		return Diagnosis{}, false, err
	}
	if len(result.Entries) == 0 {
		return Diagnosis{TaskID: taskID, Summary: "当前任务暂无错误诊断。"}, false, nil
	}
	last := result.Entries[len(result.Entries)-1]
	generated := buildDiagnosisFromError(last)
	return generated, true, nil
}

// Context 返回安全的任务上下文摘要，不回显完整 Prompt、完整快照和隐私输入。
func (service Service) Context(ctx context.Context, taskID string) (ContextSummary, bool, error) {
	if service.Store == nil {
		return ContextSummary{}, false, fmt.Errorf("task log store is required")
	}
	task, ok, err := service.Store.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil || !ok {
		return ContextSummary{}, ok, err
	}
	report, reportOK, err := service.Store.GetAnalysisReportByTaskID(ctx, task.ID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	context := ContextSummary{
		TaskID:          task.ID,
		Stock:           "未提供",
		AnalysisType:    task.Type,
		Model:           "未提供",
		PromptTemplate:  "未提供",
		QuoteStatus:     "未记录",
		KlineStatus:     "未记录",
		IndicatorStatus: "未记录",
		NewsStatus:      "未记录",
		UserPosition:    "未提供",
		DataUpdatedAt:   formatTime(task.UpdatedAt),
	}
	if reportOK {
		context.Stock = logger.RedactText(report.Symbol)
		context.AnalysisType = logger.RedactText(report.AnalysisType)
		context.Model = logger.RedactText(report.ModelName)
		context.PromptTemplate = "模板 ID " + strconv.FormatInt(report.PromptTemplateID, 10)
		context.ReportCreatedAt = formatTime(report.CreatedAt)
		context.RawSnapshotBrief = snapshotBrief(report.InputSnapshot)
		context.QuoteStatus = inferLoadedStatus(report.InputSnapshot, "quote")
		context.KlineStatus = inferLoadedStatus(report.InputSnapshot, "kline")
		context.IndicatorStatus = inferLoadedStatus(report.InputSnapshot, "indicator")
		context.NewsStatus = inferLoadedStatus(report.InputSnapshot, "news")
		if strings.Contains(strings.ToLower(report.InputSnapshot), "user_position") {
			context.UserPosition = "已提供（已脱敏）"
		}
	}
	return context, true, nil
}

// Export 生成任务级脱敏日志包，文件写入仍由 Rust 白名单命令完成。
func (service Service) Export(ctx context.Context, taskID string) (ExportBundle, error) {
	if service.Store == nil {
		return ExportBundle{}, fmt.Errorf("task log store is required")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ExportBundle{}, fmt.Errorf("task_id is required")
	}
	summary, _, _ := service.Summary(ctx, taskID)
	diagnosis, _, _ := service.Diagnosis(ctx, taskID)
	contextSummary, _, _ := service.Context(ctx, taskID)
	events, err := service.Store.ListTaskEventsAfter(ctx, taskID, 0)
	if err != nil {
		return ExportBundle{}, err
	}
	logResult, err := service.List(ctx, Query{TaskID: taskID, Limit: maxServiceLimit})
	if err != nil {
		return ExportBundle{}, err
	}
	content := buildExportContent(summary, events, logResult.Rows, diagnosis, contextSummary)
	createdAt := service.now().UTC()
	return ExportBundle{
		FileName:  "invest-compass-task-log-" + safeFilePart(taskID) + "-" + createdAt.Format("20060102-150405") + ".txt",
		Content:   logger.ExportLogText([]string{content}),
		CreatedAt: createdAt,
	}, nil
}

// WriteTaskLog 让 Service 可作为同步 StageWriter 使用。
func (service Service) WriteTaskLog(ctx context.Context, entry model.TaskLogEntry) error {
	return service.Append(ctx, []model.TaskLogEntry{entry})
}

func (service Service) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}
	return time.Now().UTC()
}
