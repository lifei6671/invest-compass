package model

import "time"

// TaskLogEntry 是任务级结构化日志表，用于任务历史抽屉的排障日志查询。
//
// 该表只保存已经脱敏的短消息和 JSON 详情，不承载完整 AI 输出、密钥、
// 代理密码或用户一次性持仓输入。
type TaskLogEntry struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;index:idx_task_log_entries_task_id_id,priority:2"`
	TaskID      string    `gorm:"column:task_id;not null;index:idx_task_log_entries_task_id_id,priority:1;index:idx_task_log_entries_task_id_level,priority:1;index:idx_task_log_entries_task_id_module_stage,priority:1"`
	RequestID   string    `gorm:"column:request_id;index:idx_task_log_entries_request_id"`
	TraceID     string    `gorm:"column:trace_id;index:idx_task_log_entries_trace_id"`
	Ts          time.Time `gorm:"column:ts;not null;index:idx_task_log_entries_ts"`
	Level       string    `gorm:"column:level;not null;index:idx_task_log_entries_task_id_level,priority:2"`
	Module      string    `gorm:"column:module;not null;index:idx_task_log_entries_task_id_module_stage,priority:2"`
	Stage       string    `gorm:"column:stage;not null;index:idx_task_log_entries_task_id_module_stage,priority:3"`
	Message     string    `gorm:"column:message;not null"`
	Code        string    `gorm:"column:code"`
	Provider    string    `gorm:"column:provider"`
	Model       string    `gorm:"column:model"`
	Symbol      string    `gorm:"column:symbol"`
	DurationMS  int64     `gorm:"column:duration_ms"`
	Retryable   bool      `gorm:"column:retryable;not null;default:false"`
	PayloadJSON string    `gorm:"column:payload_json"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TaskErrorDiagnosis 是失败任务的诊断摘要，供日志抽屉错误诊断 Tab 使用。
type TaskErrorDiagnosis struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	TaskID      string `gorm:"column:task_id;not null;uniqueIndex"`
	ErrorCode   string `gorm:"column:error_code;not null"`
	ErrorStage  string `gorm:"column:error_stage;not null"`
	Summary     string `gorm:"column:summary;not null"`
	Suggestion  string `gorm:"column:suggestion"`
	Retryable   bool   `gorm:"column:retryable;not null;default:false"`
	RequestID   string `gorm:"column:request_id"`
	TraceID     string `gorm:"column:trace_id"`
	SourceLogID int64  `gorm:"column:source_log_id"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
