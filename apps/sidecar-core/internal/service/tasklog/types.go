package tasklog

import (
	"context"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// LevelInfo 表示普通信息日志。
	LevelInfo = "INFO"
	// LevelWarn 表示可恢复或需要关注的警告日志。
	LevelWarn = "WARN"
	// LevelError 表示任务失败或阶段失败日志。
	LevelError = "ERROR"

	defaultServiceLimit = 200
	maxServiceLimit     = 500
)

// Store 是 tasklog service 依赖的数据访问边界。
type Store interface {
	AppendTaskLogs(ctx context.Context, entries []model.TaskLogEntry) error
	ListTaskLogs(ctx context.Context, query dao.TaskLogQuery) (dao.TaskLogListResult, error)
	GetTaskLog(ctx context.Context, id int64) (model.TaskLogEntry, bool, error)
	UpsertTaskErrorDiagnosis(ctx context.Context, diagnosis model.TaskErrorDiagnosis) error
	GetTaskErrorDiagnosis(ctx context.Context, taskID string) (model.TaskErrorDiagnosis, bool, error)
	GetTask(ctx context.Context, taskID string) (model.Task, bool, error)
	ListTaskEventsAfter(ctx context.Context, taskID string, afterID int64) ([]model.TaskEvent, error)
	GetAnalysisReportByTaskID(ctx context.Context, taskID string) (model.AnalysisReport, bool, error)
}

// Query 描述任务日志列表查询条件。
type Query struct {
	TaskID    string
	Level     string
	Module    string
	Stage     string
	Keyword   string
	OnlyError bool
	AfterID   int64
	Limit     int
}

// ListResult 是任务日志列表查询结果。
type ListResult struct {
	Rows        []Row `json:"rows"`
	NextAfterID int64 `json:"next_after_id"`
	HasMore     bool  `json:"has_more"`
}

// Row 是前端表格展示所需的任务结构化日志摘要。
type Row struct {
	ID         int64  `json:"id"`
	TaskID     string `json:"task_id"`
	Time       string `json:"time"`
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Module     string `json:"module"`
	Stage      string `json:"stage"`
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Retryable  bool   `json:"retryable"`
}

// Detail 是单条日志的脱敏 JSON 详情。
type Detail struct {
	Row
	RawJSON string `json:"raw_json"`
}

// Summary 是日志抽屉头部基础信息卡所需数据。
type Summary struct {
	Title     string `json:"title"`
	TaskID    string `json:"task_id"`
	TaskType  string `json:"task_type"`
	Stock     string `json:"stock,omitempty"`
	Model     string `json:"model,omitempty"`
	StartedAt string `json:"started_at"`
	Duration  string `json:"duration"`
	RequestID string `json:"request_id"`
	TraceID   string `json:"trace_id"`
	Status    string `json:"status"`
}

// Diagnosis 是错误诊断 Tab 的可展示结果。
type Diagnosis struct {
	TaskID      string   `json:"task_id"`
	ErrorCode   string   `json:"error_code"`
	ErrorStage  string   `json:"error_stage"`
	Summary     string   `json:"summary"`
	Causes      []string `json:"causes"`
	Suggestions []string `json:"suggestions"`
	Retryable   bool     `json:"retryable"`
	RequestID   string   `json:"request_id,omitempty"`
	TraceID     string   `json:"trace_id,omitempty"`
	SourceLogID int64    `json:"source_log_id,omitempty"`
}

// ContextSummary 是上下文摘要 Tab 的安全展示模型，不包含完整 Prompt 和隐私输入。
type ContextSummary struct {
	TaskID           string `json:"task_id"`
	Stock            string `json:"stock"`
	AnalysisType     string `json:"analysis_type"`
	Model            string `json:"model"`
	PromptTemplate   string `json:"prompt_template"`
	QuoteStatus      string `json:"quote_status"`
	KlineStatus      string `json:"kline_status"`
	IndicatorStatus  string `json:"indicator_status"`
	NewsStatus       string `json:"news_status"`
	UserPosition     string `json:"user_position"`
	DataUpdatedAt    string `json:"data_updated_at"`
	ReportCreatedAt  string `json:"report_created_at"`
	RawSnapshotBrief string `json:"raw_snapshot_brief,omitempty"`
}

// ExportBundle 是交给 Rust 白名单命令写入用户目录的脱敏日志包。
type ExportBundle struct {
	FileName  string    `json:"file_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// StageMeta 描述 RunStage 生成结构化日志时需要的任务上下文。
type StageMeta struct {
	TaskID    string
	RequestID string
	TraceID   string
	Module    string
	Provider  string
	Model     string
	Symbol    string
	Code      string
	Retryable bool
	Now       func() time.Time
}

// StageWriter 是 RunStage 依赖的最小写入边界。
type StageWriter interface {
	WriteTaskLog(ctx context.Context, entry model.TaskLogEntry) error
}
