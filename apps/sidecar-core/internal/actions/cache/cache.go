package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Cleaner 是缓存清理 API 的单一执行边界，由实际存储层注入。
type Cleaner interface {
	CleanCache(ctx context.Context, targets []settings.CacheTarget) error
}

// Config 是缓存 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Usages   []settings.CacheUsage
	Cleaner  Cleaner
}

type cleanRequest struct {
	Targets []settings.CacheTarget `json:"targets"`
}

// Routes 返回缓存相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/cache/stats", Handler: handleStats(config)},
		{Method: http.MethodPost, Path: "/api/cache/clean", Handler: handleClean(config)},
	}
}

// handleStats 返回可清理临时缓存统计，不包含报告和配置。
func handleStats(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		httpx.WriteOK(response, settings.BuildCacheStats(config.Usages), context)
	}
}

// handleClean 只清理 settings 允许的临时缓存目标。
func handleClean(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Cleaner == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50302, "cache_cleaner_unavailable", context)
			return
		}

		var payload cleanRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			httpx.WriteError(response, http.StatusBadRequest, 40001, "invalid_json", context)
			return
		}

		targets := settings.FilterCacheCleanupTargets(payload.Targets)
		if err := config.Cleaner.CleanCache(request.Context(), targets); err != nil {
			slog.Warn(
				"缓存清理失败",
				logger.FieldRequestID, context.RequestID,
				logger.FieldTraceID, context.TraceID,
				"error", logger.RedactError(err),
			)
			httpx.WriteError(response, http.StatusInternalServerError, 50001, "cache_clean_failed", context)
			return
		}

		httpx.WriteOK(response, map[string][]settings.CacheTarget{"cleaned_targets": targets}, context)
	}
}
