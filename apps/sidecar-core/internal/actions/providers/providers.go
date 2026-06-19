package providers

import (
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
)

// Config 是 Provider 状态 action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	MarketProvider market.MarketProvider
	NewsProvider   news.Provider
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
		marketStatus := market.UnconfiguredProviderStatus()
		if config.MarketProvider != nil {
			marketStatus = config.MarketProvider.Status(request.Context())
		}
		newsStatus := news.ProviderStatusFromProvider(request.Context(), config.NewsProvider)
		summary := dashboard.BuildSummary(dashboard.Input{
			ProviderStatuses: []dashboard.ProviderStatus{
				dashboard.ProviderStatusFromMarket(marketStatus),
				dashboard.ProviderStatusFromNews(newsStatus),
			},
		})
		httpx.WriteOK(response, summary.ProviderStatuses, context)
	}
}
