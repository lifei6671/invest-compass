package analysis

import (
	"encoding/json"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/task"
)

// ErrorCode 是分析任务模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidSymbol 表示股票代码非法。
	ErrorInvalidSymbol ErrorCode = "invalid_analysis_symbol"
	// ErrorUnsupportedAnalysisType 表示分析类型不在首版支持范围内。
	ErrorUnsupportedAnalysisType ErrorCode = "unsupported_analysis_type"
	// ErrorMissingAIConfig 表示缺少模型配置 ID。
	ErrorMissingAIConfig ErrorCode = "missing_ai_config"
	// ErrorMissingPromptTemplate 表示缺少 Prompt 模板 ID。
	ErrorMissingPromptTemplate ErrorCode = "missing_prompt_template"
	// ErrorTaskAlreadyTerminal 表示终态任务不能继续取消。
	ErrorTaskAlreadyTerminal ErrorCode = "task_already_terminal"
)

// Error 表示分析任务规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把用户输入写进错误文本。
func (err *Error) Error() string {
	return string(err.Code)
}

// AnalysisType 是首版支持的分析任务类型。
type AnalysisType string

const (
	// AnalysisStockFull 表示个股综合分析。
	AnalysisStockFull AnalysisType = "stock_full"
	// AnalysisTechnical 表示技术面分析。
	AnalysisTechnical AnalysisType = "technical"
)

// UserPosition 是一次性持仓输入，只允许用于本次分析上下文和报告输入快照。
type UserPosition struct {
	CostPrice float64
	Shares    float64
	RiskLevel string
}

// CreateRequest 是前端到 Rust command 的分析任务创建输入。
type CreateRequest struct {
	Symbol           string
	AnalysisType     AnalysisType
	AIConfigID       int64
	PromptTemplateID int64
	UserPosition     *UserPosition
}

// ValidatedCreateRequest 是已完成规则校验的分析任务创建输入。
type ValidatedCreateRequest struct {
	Symbol           stock.Symbol
	AnalysisType     AnalysisType
	AIConfigID       int64
	PromptTemplateID int64
	UserPosition     *UserPosition
}

var supportedAnalysisTypes = map[AnalysisType]struct{}{
	AnalysisStockFull: {},
	AnalysisTechnical: {},
}

// ValidateCreateRequest 校验分析任务创建请求，并标准化股票代码。
func ValidateCreateRequest(request CreateRequest) (ValidatedCreateRequest, error) {
	symbol, err := stock.ParseSymbol(request.Symbol)
	if err != nil {
		return ValidatedCreateRequest{}, &Error{Code: ErrorInvalidSymbol}
	}
	if _, ok := supportedAnalysisTypes[request.AnalysisType]; !ok {
		return ValidatedCreateRequest{}, &Error{Code: ErrorUnsupportedAnalysisType}
	}
	if request.AIConfigID <= 0 {
		return ValidatedCreateRequest{}, &Error{Code: ErrorMissingAIConfig}
	}
	if request.PromptTemplateID <= 0 {
		return ValidatedCreateRequest{}, &Error{Code: ErrorMissingPromptTemplate}
	}

	return ValidatedCreateRequest{
		Symbol:           symbol,
		AnalysisType:     request.AnalysisType,
		AIConfigID:       request.AIConfigID,
		PromptTemplateID: request.PromptTemplateID,
		UserPosition:     request.UserPosition,
	}, nil
}

// InputSnapshotForLog 返回普通日志可记录的输入快照，必须隐藏一次性持仓明细。
func (request ValidatedCreateRequest) InputSnapshotForLog() string {
	payload := map[string]any{
		"symbol":             request.Symbol.String(),
		"analysis_type":      request.AnalysisType,
		"ai_config_id":       request.AIConfigID,
		"prompt_template_id": request.PromptTemplateID,
		"has_user_position":  request.UserPosition != nil,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return logger.RedactText(string(encoded))
}

// CreateTask 将已校验分析请求转换为待执行任务和创建事件。
func CreateTask(request ValidatedCreateRequest, taskID string, now time.Time) (task.Task, task.Event) {
	payload := map[string]any{
		"symbol":             request.Symbol.String(),
		"analysis_type":      request.AnalysisType,
		"ai_config_id":       request.AIConfigID,
		"prompt_template_id": request.PromptTemplateID,
		"has_user_position":  request.UserPosition != nil,
	}
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		panic("failed to build analysis task created event payload")
	}

	createdTask := task.Task{
		ID:        taskID,
		Type:      task.TypeAnalysis,
		Status:    task.StatusPending,
		Title:     request.Symbol.String() + " " + string(request.AnalysisType),
		CreatedAt: now,
		UpdatedAt: now,
	}
	event := task.Event{
		TaskID:    taskID,
		Type:      task.EventCreated,
		Payload:   task.SanitizeEventPayload(string(encodedPayload)),
		CreatedAt: now,
		UpdatedAt: now,
	}

	return createdTask, event
}

// CancelTask 将可取消任务切换为 CANCELLED，并生成对应任务事件。
func CancelTask(existing task.Task) (task.Task, task.Event, error) {
	if existing.Status.Terminal() {
		return task.Task{}, task.Event{}, &Error{Code: ErrorTaskAlreadyTerminal}
	}
	if err := task.ValidateTransition(existing.Status, task.StatusCancelled); err != nil {
		return task.Task{}, task.Event{}, err
	}

	now := time.Now().UTC()
	existing.Status = task.StatusCancelled
	existing.FinishedAt = now
	existing.UpdatedAt = now
	return existing, task.Event{
		TaskID:    existing.ID,
		Type:      task.EventCancelled,
		Payload:   `{"reason":"cancelled_by_user"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
