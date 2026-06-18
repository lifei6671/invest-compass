package actions

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/cache"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/health"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/providers"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/stocks"
	dashboardservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Config 是 actions 根路由构建所需的运行期依赖集合。
type Config struct {
	Version        string
	Token          string
	DBStatus       string
	Ready          bool
	MarketProvider market.MarketProvider
	DashboardInput dashboardservice.Input
	CacheUsages    []settings.CacheUsage
	CacheCleaner   cache.Cleaner
	OnShutdown     func()
}

// NewHandler 创建本地 HTTP handler，并集中注册各 action 子包声明的路由。
func NewHandler(config Config) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(recoverGin(), rejectNonPost())
	RegisterRoutes(router, Routes(config))
	router.NoRoute(func(ginContext *gin.Context) {
		context := httpx.ContextFrom(ginContext.Request)
		httpx.WriteError(ginContext.Writer, http.StatusNotFound, 40400, "not_found", context)
	})

	return router
}

// Routes 汇总所有 action 子包拥有的 HTTP 路由定义。
func Routes(config Config) []httpx.Route {
	security := httpx.SecurityConfig{Token: config.Token, Ready: config.Ready}
	routes := make([]httpx.Route, 0, 7)
	routes = append(routes, health.Routes(health.Config{
		Version:    config.Version,
		DBStatus:   config.DBStatus,
		Security:   security,
		OnShutdown: config.OnShutdown,
	})...)
	routes = append(routes, stocks.Routes(stocks.Config{
		Security:       security,
		MarketProvider: config.MarketProvider,
	})...)
	routes = append(routes, providers.Routes(providers.Config{
		Security:       security,
		MarketProvider: config.MarketProvider,
	})...)
	routes = append(routes, dashboard.Routes(dashboard.Config{
		Security: security,
		Input:    config.DashboardInput,
	})...)
	routes = append(routes, cache.Routes(cache.Config{
		Security: security,
		Usages:   config.CacheUsages,
		Cleaner:  config.CacheCleaner,
	})...)
	return routes
}

// RegisterRoutes 将 action 路由定义注册到 Gin，引导子包避免直接依赖 Gin 注册 API。
func RegisterRoutes(router *gin.Engine, routes []httpx.Route) {
	for _, route := range routes {
		router.Handle(route.Method, route.Path, gin.WrapF(route.Handler))
	}
}

// recoverGin 将 Gin handler panic 转为统一错误响应，并确保异常文本先脱敏再进入日志。
func recoverGin() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(ginContext *gin.Context, recovered any) {
		context := httpx.ContextFrom(ginContext.Request)
		slog.Error(
			"本地 HTTP handler panic",
			logger.FieldRequestID, context.RequestID,
			logger.FieldTraceID, context.TraceID,
			"error", logger.RedactText(fmt.Sprint(recovered)),
		)
		httpx.WriteError(ginContext.Writer, http.StatusInternalServerError, 50000, "internal_error", context)
		ginContext.Abort()
	})
}

// rejectNonPost 统一拒绝非 POST 请求，防止本地 core API 被 GET 等方法探测。
func rejectNonPost() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		if ginContext.Request.Method != http.MethodPost {
			context := httpx.ContextFrom(ginContext.Request)
			httpx.WriteError(ginContext.Writer, http.StatusMethodNotAllowed, 40500, "method_not_allowed", context)
			ginContext.Abort()
			return
		}
		ginContext.Next()
	}
}
