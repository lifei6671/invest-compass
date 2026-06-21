package logger

const (
	// FieldRequestID 是每条请求链路日志必须携带的请求 ID 字段。
	FieldRequestID = "request_id"
	// FieldTraceID 是跨模块关联日志时使用的 trace ID 字段。
	FieldTraceID = "trace_id"
	// FieldTaskID 是分析任务、任务事件和报告链路共用的任务 ID 字段。
	FieldTaskID = "task_id"
	// FieldModule 是任务结构化日志的业务模块字段。
	FieldModule = "module"
	// FieldStage 是任务结构化日志的执行阶段字段。
	FieldStage = "stage"
	// FieldProvider 是外部数据源或 AI Provider 日志使用的提供方字段。
	FieldProvider = "provider"
	// FieldModel 是 AI 或数据处理链路使用的模型字段。
	FieldModel = "model"
	// FieldSymbol 是股票、行情和分析链路使用的统一股票代码字段。
	FieldSymbol = "symbol"
	// FieldCode 是错误日志中的稳定错误码字段。
	FieldCode = "code"
	// FieldDurationMS 是阶段耗时毫秒数字段。
	FieldDurationMS = "duration_ms"
	// FieldRetryable 表示当前错误或阶段是否可重试。
	FieldRetryable = "retryable"
)
