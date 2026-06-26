package datasourcecredential

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	service "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Service 是数据源凭据 action 依赖的业务边界。
type Service interface {
	List(ctx context.Context) (service.ListView, error)
	Save(ctx context.Context, request service.SaveRequest) (service.Config, error)
	Clear(ctx context.Context, providerID string) (service.Config, error)
	Test(ctx context.Context, request service.TestRequest) (service.TestResult, error)
}

// Config 是数据源凭据 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Service  Service
}

type listRequest struct{}

type getRequest struct {
	ProviderID string `json:"providerId"`
}

type clearRequest struct {
	ProviderID string `json:"providerId"`
}

type saveData struct {
	Config service.Config `json:"config"`
}

type clearData struct {
	Config service.Config `json:"config"`
}

type testData struct {
	Result service.TestResult `json:"result"`
}

// Routes 返回数据源凭据管理路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/data-source/credentials/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/data-source/credentials/save", Handler: handleSave(config)},
		{Method: http.MethodPost, Path: "/api/data-source/credentials/clear", Handler: handleClear(config)},
		{Method: http.MethodPost, Path: "/api/data-source/credentials/test", Handler: handleTest(config)},
	}
}

// handleList 返回凭据页安全展示数据。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50330, "data_source_credential_service_unavailable", context)
			return
		}
		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		view, err := config.Service.List(request.Context())
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, view, context)
	}
}

// handleSave 保存数据源凭据配置，响应中只包含脱敏后的配置。
func handleSave(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50330, "data_source_credential_service_unavailable", context)
			return
		}
		var payload service.SaveRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		configView, err := config.Service.Save(request.Context(), payload)
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, saveData{Config: configView}, context)
	}
}

// handleClear 清除指定 Provider 凭据密文。
func handleClear(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50330, "data_source_credential_service_unavailable", context)
			return
		}
		var payload clearRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		configView, err := config.Service.Clear(request.Context(), payload.ProviderID)
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, clearData{Config: configView}, context)
	}
}

// handleTest 执行数据源凭据真实 HTTP 预检，响应只返回脱敏后的状态和说明。
func handleTest(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50330, "data_source_credential_service_unavailable", context)
			return
		}
		var payload service.TestRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		result, err := config.Service.Test(request.Context(), payload)
		if err != nil {
			writeServiceError(response, err, context)
			return
		}
		httpx.WriteOK(response, testData{Result: result}, context)
	}
}

// writeServiceError 将凭据业务错误映射为稳定 HTTP 错误响应。
func writeServiceError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	slog.Warn(
		"数据源凭据操作失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusBadRequest, 40030, "data_source_credential_failed", context)
}
