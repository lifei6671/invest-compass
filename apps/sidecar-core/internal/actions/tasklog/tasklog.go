package tasklog

import (
	"context"
	"net/http"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
)

// Service 是任务结构化日志 action 依赖的业务契约。
type Service interface {
	List(ctx context.Context, query tasklogservice.Query) (tasklogservice.ListResult, error)
	Get(ctx context.Context, id int64) (tasklogservice.Detail, bool, error)
	Summary(ctx context.Context, taskID string) (tasklogservice.Summary, bool, error)
	Diagnosis(ctx context.Context, taskID string) (tasklogservice.Diagnosis, bool, error)
	Context(ctx context.Context, taskID string) (tasklogservice.ContextSummary, bool, error)
	Export(ctx context.Context, taskID string) (tasklogservice.ExportBundle, error)
}

// Config 是任务日志 action 的固定依赖集合。
type Config struct {
	Security httpx.SecurityConfig
	Service  Service
}

type listRequest struct {
	TaskID    string `json:"task_id"`
	Level     string `json:"level"`
	Module    string `json:"module"`
	Stage     string `json:"stage"`
	Keyword   string `json:"keyword"`
	OnlyError bool   `json:"only_error"`
	AfterID   int64  `json:"after_id"`
	Limit     int    `json:"limit"`
}

type getRequest struct {
	ID int64 `json:"id"`
}

type taskIDRequest struct {
	TaskID string `json:"task_id"`
}

// Routes 返回任务结构化日志相关固定路由定义。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/tasks/logs/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/tasks/logs/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/tasks/logs/summary", Handler: handleSummary(config)},
		{Method: http.MethodPost, Path: "/api/tasks/logs/diagnosis", Handler: handleDiagnosis(config)},
		{Method: http.MethodPost, Path: "/api/tasks/logs/context", Handler: handleContext(config)},
		{Method: http.MethodPost, Path: "/api/tasks/logs/export", Handler: handleExport(config)},
	}
}

// handleList 处理任务日志列表查询并校验分页参数。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !ready(response, request, config, context) {
			return
		}
		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if strings.TrimSpace(payload.TaskID) == "" || payload.AfterID < 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_task_log_query", context)
			return
		}
		result, err := config.Service.List(request.Context(), tasklogservice.Query{
			TaskID:    payload.TaskID,
			Level:     payload.Level,
			Module:    payload.Module,
			Stage:     payload.Stage,
			Keyword:   payload.Keyword,
			OnlyError: payload.OnlyError,
			AfterID:   payload.AfterID,
			Limit:     payload.Limit,
		})
		if err != nil {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_task_log_query", context)
			return
		}
		httpx.WriteOK(response, result, context)
	}
}

// handleGet 按日志 ID 读取单条任务日志详情。
func handleGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !ready(response, request, config, context) {
			return
		}
		var payload getRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_task_log_id", context)
			return
		}
		detail, ok, err := config.Service.Get(request.Context(), payload.ID)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "internal_error", context)
			return
		}
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40404, "task_log_not_found", context)
			return
		}
		httpx.WriteOK(response, detail, context)
	}
}

// handleSummary 返回指定任务的日志聚合摘要。
func handleSummary(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		taskID, ok := decodeTaskID(response, request, config)
		if !ok {
			return
		}
		context := httpx.ContextFrom(request)
		summary, found, err := config.Service.Summary(request.Context(), taskID)
		writeTaskScopedResponse(response, context, summary, found, err)
	}
}

// handleDiagnosis 返回指定任务的错误诊断结果。
func handleDiagnosis(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		taskID, ok := decodeTaskID(response, request, config)
		if !ok {
			return
		}
		context := httpx.ContextFrom(request)
		diagnosis, _, err := config.Service.Diagnosis(request.Context(), taskID)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "internal_error", context)
			return
		}
		httpx.WriteOK(response, diagnosis, context)
	}
}

// handleContext 返回指定任务的安全上下文摘要。
func handleContext(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		taskID, ok := decodeTaskID(response, request, config)
		if !ok {
			return
		}
		context := httpx.ContextFrom(request)
		summary, found, err := config.Service.Context(request.Context(), taskID)
		writeTaskScopedResponse(response, context, summary, found, err)
	}
}

// handleExport 返回指定任务的脱敏日志导出包。
func handleExport(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		taskID, ok := decodeTaskID(response, request, config)
		if !ok {
			return
		}
		context := httpx.ContextFrom(request)
		bundle, err := config.Service.Export(request.Context(), taskID)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "internal_error", context)
			return
		}
		httpx.WriteOK(response, bundle, context)
	}
}

// decodeTaskID 解码并校验 task_id 请求字段。
func decodeTaskID(response http.ResponseWriter, request *http.Request, config Config) (string, bool) {
	context := httpx.ContextFrom(request)
	if !ready(response, request, config, context) {
		return "", false
	}
	var payload taskIDRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return "", false
	}
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_task_id", context)
		return "", false
	}
	return taskID, true
}

// ready 校验任务日志接口的 ready 状态和 runtime token。
func ready(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Service == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "task_log_service_unavailable", context)
		return false
	}
	return true
}

// writeTaskScopedResponse 写入任务范围查询的统一响应。
func writeTaskScopedResponse(response http.ResponseWriter, context httpx.RequestContext, data any, found bool, err error) {
	if err != nil {
		httpx.WriteError(response, http.StatusInternalServerError, 50000, "internal_error", context)
		return
	}
	if !found {
		httpx.WriteError(response, http.StatusNotFound, 40403, "task_not_found", context)
		return
	}
	httpx.WriteOK(response, data, context)
}
