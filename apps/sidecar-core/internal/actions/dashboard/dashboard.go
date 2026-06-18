package dashboard

import (
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	dashboardservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
)

// Config 是 Dashboard action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Input    dashboardservice.Input
}

// Routes 返回 Dashboard 相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/dashboard/summary", Handler: handleSummary(config)},
	}
}

// handleSummary 返回 Dashboard 首版聚合结果，数据只来自 Config.Input。
func handleSummary(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		httpx.WriteOK(response, dashboardservice.BuildSummary(config.Input), context)
	}
}
