package logexport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	logexportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Source 定义日志导出 API 获取原始日志行的唯一边界。
type Source interface {
	ExportLogRequest(ctx context.Context) (logexportservice.Request, error)
}

// Config 是日志导出 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Source   Source
}

// Routes 返回日志导出路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/logs/export", Handler: handleExport(config)},
	}
}

// handleExport 生成已二次脱敏的日志导出包，避免 Go core 写入任意用户路径。
func handleExport(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Source == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50306, "log_export_unavailable", context)
			return
		}

		exportRequest, err := config.Source.ExportLogRequest(request.Context())
		if err != nil {
			slog.Warn(
				"读取日志导出内容失败",
				logger.FieldRequestID, context.RequestID,
				logger.FieldTraceID, context.TraceID,
				"error", logger.RedactError(err),
			)
			httpx.WriteError(response, http.StatusInternalServerError, 50042, "log_export_failed", context)
			return
		}
		if err := logexportservice.ValidateRequest(exportRequest); err != nil {
			writeRuleError(response, err, context)
			return
		}

		httpx.WriteOK(response, logexportservice.BuildBundle(exportRequest), context)
	}
}

// writeRuleError 将日志导出规则错误映射为稳定 HTTP 错误响应。
func writeRuleError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) {
		httpx.WriteError(response, http.StatusBadRequest, 40042, string(ruleError.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusInternalServerError, 50043, "log_export_failed", context)
}
