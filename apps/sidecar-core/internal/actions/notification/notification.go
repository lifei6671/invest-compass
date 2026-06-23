package notification

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	service "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/notification"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Store 是 notification action 依赖的业务持久化边界。
type Store interface {
	service.Store
}

// Config 是应用内通知 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type listRequest struct {
	UnreadOnly bool `json:"unread_only"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
}

type markReadRequest struct {
	IDs []int64 `json:"ids"`
}

type emptyRequest struct{}

type okData struct {
	OK bool `json:"ok"`
}

// Routes 返回应用内通知路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/notifications/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/notifications/unread-count", Handler: handleUnreadCount(config)},
		{Method: http.MethodPost, Path: "/api/notifications/mark-read", Handler: handleMarkRead(config)},
		{Method: http.MethodPost, Path: "/api/notifications/mark-all-read", Handler: handleMarkAllRead(config)},
		{Method: http.MethodPost, Path: "/api/notifications/clear-read", Handler: handleClearRead(config)},
	}
}

// handleList 返回分页通知列表，支持未读过滤。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		result, err := service.NewService(config.Store).List(request.Context(), service.ListRequest{
			UnreadOnly: payload.UnreadOnly,
			Limit:      payload.Limit,
			Offset:     payload.Offset,
		})
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, result, context)
	}
}

// handleUnreadCount 返回未读通知数量，供前端角标使用。
func handleUnreadCount(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		var payload emptyRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		result, err := service.NewService(config.Store).UnreadCount(request.Context())
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, result, context)
	}
}

// handleMarkRead 将指定通知标记为已读。
func handleMarkRead(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		var payload markReadRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if err := service.NewService(config.Store).MarkRead(request.Context(), payload.IDs); err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, okData{OK: true}, context)
	}
}

// handleMarkAllRead 将所有通知标记为已读。
func handleMarkAllRead(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		var payload emptyRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if err := service.NewService(config.Store).MarkAllRead(request.Context()); err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, okData{OK: true}, context)
	}
}

// handleClearRead 清理已读通知记录，不影响通知来源业务数据。
func handleClearRead(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		var payload emptyRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if err := service.NewService(config.Store).ClearRead(request.Context()); err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, okData{OK: true}, context)
	}
}

// requireStore 校验 ready/token 和通知 store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50340, "notification_store_unavailable", context)
		return false
	}
	return true
}

// writeServiceError 将通知业务错误映射为稳定 HTTP 响应。
func writeServiceError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	if errors.Is(err, service.ErrInvalidRequest) {
		httpx.WriteError(response, http.StatusBadRequest, 40040, "invalid_notification_request", context)
		return
	}
	slog.Warn(
		"应用内通知操作失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50040, "notification_store_error", context)
}
