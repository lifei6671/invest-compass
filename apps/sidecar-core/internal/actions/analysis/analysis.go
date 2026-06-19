package analysis

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	analysisservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/analysis"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是分析任务 action 依赖的任务持久化边界。
type Store interface {
	SaveTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, taskID string) (model.Task, bool, error)
	AppendTaskEvent(ctx context.Context, event *model.TaskEvent) error
}

// Executor 是分析任务真实执行器边界，action 只负责触发，不承载业务编排。
type Executor interface {
	Execute(ctx context.Context, taskID string, request analysisservice.ValidatedCreateRequest, resolvedAPIKey string) error
}

// TransactFunc 在同一个数据库事务内执行任务和事件写入。
type TransactFunc func(ctx context.Context, run func(Store) error) error

// Config 是分析任务 action 的运行期依赖。
type Config struct {
	Security     httpx.SecurityConfig
	Store        Store
	Executor     Executor
	Transact     TransactFunc
	RunningTasks *RunningTasks
}

type createRequest struct {
	Symbol           string               `json:"symbol"`
	AnalysisType     string               `json:"analysis_type"`
	AIConfigID       int64                `json:"ai_config_id"`
	PromptTemplateID int64                `json:"prompt_template_id"`
	UserPosition     *userPositionRequest `json:"user_position"`
	ResolvedAPIKey   string               `json:"resolved_api_key"`
}

type userPositionRequest struct {
	CostPrice float64 `json:"cost_price"`
	Shares    float64 `json:"shares"`
	RiskLevel string  `json:"risk_level"`
}

type cancelRequest struct {
	TaskID string `json:"task_id"`
}

type taskCreatedData struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type taskCancelledData struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// Routes 返回分析任务相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	if config.RunningTasks == nil {
		config.RunningTasks = NewRunningTasks()
	}
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/analysis/tasks", Handler: handleCreate(config)},
		{Method: http.MethodPost, Path: "/api/tasks/cancel", Handler: handleCancel(config)},
	}
}

// handleCreate 创建待执行分析任务，并写入 TASK_CREATED 事件。
func handleCreate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload createRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if strings.TrimSpace(payload.ResolvedAPIKey) == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40022, string(xerr.AIInvalidRequest), context)
			return
		}

		validated, err := analysisservice.ValidateCreateRequest(analysisservice.CreateRequest{
			Symbol:           payload.Symbol,
			AnalysisType:     analysisservice.AnalysisType(payload.AnalysisType),
			AIConfigID:       payload.AIConfigID,
			PromptTemplateID: payload.PromptTemplateID,
			UserPosition:     userPositionToService(payload.UserPosition),
		})
		if err != nil {
			writeRuleError(response, err, context)
			return
		}

		taskID := "analysis-" + context.RequestID
		createdTask, createdEvent := analysisservice.CreateTask(validated, taskID, time.Now().UTC())
		modelTask := taskToModel(createdTask)
		modelEvent := taskEventToModel(createdEvent)
		if err := runMutation(request.Context(), config, func(store Store) error {
			if err := store.SaveTask(request.Context(), &modelTask); err != nil {
				return err
			}
			return store.AppendTaskEvent(request.Context(), &modelEvent)
		}); err != nil {
			writeStoreError(response, context, "创建分析任务和事件失败", err)
			return
		}

		startExecutor(config, taskID, validated, payload.ResolvedAPIKey, context)
		httpx.WriteOK(response, taskCreatedData{TaskID: taskID, Status: string(taskservice.StatusPending)}, context)
	}
}

// handleCancel 取消非终态分析任务，并写入 TASK_CANCELLED 事件。
func handleCancel(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload cancelRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		taskID := strings.TrimSpace(payload.TaskID)
		if taskID == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_task_id", context)
			return
		}

		item, found, err := config.Store.GetTask(request.Context(), taskID)
		if err != nil {
			writeStoreError(response, context, "读取分析任务失败", err)
			return
		}
		if !found {
			httpx.WriteError(response, http.StatusNotFound, 40403, "task_not_found", context)
			return
		}

		cancelledTask, cancelledEvent, err := analysisservice.CancelTask(taskFromModel(item))
		if err != nil {
			writeRuleError(response, err, context)
			return
		}
		modelTask := taskToModel(cancelledTask)
		modelEvent := taskEventToModel(cancelledEvent)
		if err := runMutation(request.Context(), config, func(store Store) error {
			if err := store.SaveTask(request.Context(), &modelTask); err != nil {
				return err
			}
			return store.AppendTaskEvent(request.Context(), &modelEvent)
		}); err != nil {
			writeStoreError(response, context, "取消分析任务和事件失败", err)
			return
		}

		if config.RunningTasks != nil {
			config.RunningTasks.Cancel(taskID)
		}
		httpx.WriteOK(response, taskCancelledData{TaskID: taskID, Status: string(taskservice.StatusCancelled)}, context)
	}
}

// requireStore 校验 ready/token 和分析任务 store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "analysis_store_unavailable", context)
		return false
	}
	return true
}

// runMutation 优先使用生产事务，测试替身未提供事务时退回直接执行。
func runMutation(ctx context.Context, config Config, run func(Store) error) error {
	if config.Transact != nil {
		return config.Transact(ctx, run)
	}
	return run(config.Store)
}

// startExecutor 异步触发真实分析执行，避免 HTTP 请求等待外部 AI Provider。
func startExecutor(config Config, taskID string, request analysisservice.ValidatedCreateRequest, resolvedAPIKey string, requestContext httpx.RequestContext) {
	if config.Executor == nil {
		return
	}
	executeContext, cleanup := contextWithTrace(requestContext)
	if config.RunningTasks != nil {
		executeContext, cleanup = config.RunningTasks.Start(taskID)
	}
	go func() {
		defer cleanup()
		if err := config.Executor.Execute(executeContext, taskID, request, resolvedAPIKey); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn(
				"分析任务执行失败",
				logger.FieldRequestID, requestContext.RequestID,
				logger.FieldTraceID, requestContext.TraceID,
				"task_id", taskID,
				"error", logger.RedactError(err),
			)
		}
	}()
}

// contextWithTrace 返回后台执行上下文；当前 trace 字段由日志显式携带，不写入 context value。
func contextWithTrace(_ httpx.RequestContext) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	return ctx, cancel
}

// userPositionToService 转换一次性持仓输入，保持只用于本次分析上下文。
func userPositionToService(position *userPositionRequest) *analysisservice.UserPosition {
	if position == nil {
		return nil
	}
	return &analysisservice.UserPosition{
		CostPrice: position.CostPrice,
		Shares:    position.Shares,
		RiskLevel: position.RiskLevel,
	}
}

// taskFromModel 转换数据库任务为 service 状态机模型。
func taskFromModel(item model.Task) taskservice.Task {
	return taskservice.Task{
		ID:         item.ID,
		Type:       taskservice.Type(item.Type),
		Status:     taskservice.Status(item.Status),
		Title:      item.Title,
		Progress:   item.Progress,
		Error:      item.ErrorMessage,
		StartedAt:  item.StartedAt,
		FinishedAt: item.FinishedAt,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

// taskToModel 转换 service 任务模型为数据库模型。
func taskToModel(item taskservice.Task) model.Task {
	return model.Task{
		ID:           item.ID,
		Type:         string(item.Type),
		Status:       string(item.Status),
		Title:        item.Title,
		Progress:     item.Progress,
		ErrorMessage: item.Error,
		StartedAt:    item.StartedAt,
		FinishedAt:   item.FinishedAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

// taskEventToModel 转换 service 事件为数据库事件模型。
func taskEventToModel(event taskservice.Event) model.TaskEvent {
	return model.TaskEvent{
		ID:        event.ID,
		TaskID:    event.TaskID,
		EventType: string(event.Type),
		Payload:   event.Payload,
		CreatedAt: event.CreatedAt,
		UpdatedAt: event.UpdatedAt,
	}
}

// writeRuleError 将分析任务规则错误映射为稳定响应。
func writeRuleError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) {
		httpx.WriteError(response, http.StatusBadRequest, 40031, string(ruleError.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusInternalServerError, 50031, "analysis_task_failed", context)
}

// writeStoreError 记录脱敏后的任务存储错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50032, "analysis_store_error", context)
}
