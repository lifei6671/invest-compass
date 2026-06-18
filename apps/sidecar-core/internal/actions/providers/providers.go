package providers

import (
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
)

// Config 是 Provider 状态 action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	MarketProvider market.MarketProvider
}

// Routes 返回 Provider 状态相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/providers/status", Handler: handleStatus(config)},
	}
}

// handleStatus 返回数据源状态的安全展示数据。
func handleStatus(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.MarketProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
			return
		}

		status := config.MarketProvider.Status(request.Context())
		summary := dashboard.BuildSummary(dashboard.Input{
			ProviderStatuses: []market.ProviderStatus{status},
		})
		httpx.WriteOK(response, summary.ProviderStatuses, context)
	}
}
