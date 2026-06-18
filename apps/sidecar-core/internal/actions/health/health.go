package health

import (
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
)

// Config 是健康检查和关闭 action 的运行期配置。
type Config struct {
	Version    string
	DBStatus   string
	Security   httpx.SecurityConfig
	OnShutdown func()
}

type healthData struct {
	Version  string `json:"version"`
	DBStatus string `json:"dbStatus"`
}

// Routes 返回 sidecar 生命周期相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/internal/health", Handler: handleHealth(config)},
		{Method: http.MethodPost, Path: "/internal/shutdown", Handler: handleShutdown(config)},
	}
}

// handleHealth 返回最小健康信息，只在 sidecar 握手完成且 token 正确时可用。
func handleHealth(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		httpx.WriteOK(response, healthData{
			Version:  config.Version,
			DBStatus: config.DBStatus,
		}, context)
	}
}

// handleShutdown 处理 Rust 生命周期管理层发起的关闭请求，并复用 runtime token 安全边界。
func handleShutdown(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		httpx.WriteOK(response, map[string]string{"status": "shutting_down"}, context)
		if config.OnShutdown != nil {
			config.OnShutdown()
		}
	}
}
