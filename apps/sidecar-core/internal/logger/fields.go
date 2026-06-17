package logger

const (
	// FieldRequestID 是每条请求链路日志必须携带的请求 ID 字段。
	FieldRequestID = "request_id"
	// FieldTraceID 是跨模块关联日志时使用的 trace ID 字段。
	FieldTraceID = "trace_id"
	// FieldTaskID 是分析任务、任务事件和报告链路共用的任务 ID 字段。
	FieldTaskID = "task_id"
	// FieldProvider 是外部数据源或 AI Provider 日志使用的提供方字段。
	FieldProvider = "provider"
	// FieldSymbol 是股票、行情和分析链路使用的统一股票代码字段。
	FieldSymbol = "symbol"
)
