package actions

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	aiconfigaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/aiconfig"
	analysisaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/analysis"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/cache"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/dashboard"
	datasourcecredentialaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/datasourcecredential"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/health"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	logexportaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/logexport"
	marketaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/market"
	newsaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/news"
	notificationaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/notification"
	promptaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/prompt"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/providers"
	reportaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/reports"
	scheduleraction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/scheduler"
	searchaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/search"
	settingsaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/stocks"
	tasklogaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/tasklog"
	taskaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/tasks"
	updatecheckaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/updatecheck"
	watchlistaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/watchlist"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	dashboardservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	datasourcecredentialservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Config 是 actions 根路由构建所需的运行期依赖集合。
type Config struct {
	Version               string
	Token                 string
	DBStatus              string
	Ready                 bool
	MarketProvider        market.MarketProvider
	NewsProvider          newsservice.Provider
	StockStore            searchservice.StockSearchStore
	StockSearchService    stocks.Service
	DocumentSearchStore   searchservice.DocumentSearchStore
	DocumentSearchService searchaction.Service
	MarketStore           marketaction.Store
	NewsStore             newsaction.Store
	WatchlistStore        watchlistaction.Store
	PromptTemplateStore   promptaction.Store
	AIConfigStore         aiconfigaction.Store
	AIConfigTester        aiservice.ConfigTester
	ProviderNotifier      providers.Notifier
	AnalysisStore         analysisaction.Store
	AnalysisExecutor      analysisaction.Executor
	AnalysisTransact      analysisaction.TransactFunc
	TaskStore             taskaction.Store
	TaskLogService        tasklogaction.Service
	ReportStore           reportaction.Store
	DashboardInput        dashboardservice.Input
	DashboardStore        dashboard.Store
	CacheStatsProvider    cache.StatsProvider
	CacheCleaner          cache.Cleaner
	SettingsStore         settingsaction.Store
	NotificationStore     notificationaction.Store
	DataSourceCredentials datasourcecredentialservice.Service
	SchedulerStore        scheduleraction.Store
	SchedulerQueue        schedulerservice.Queue
	SchedulerService      scheduleraction.Service
	LogExportSource       logexportaction.Source
	UpdateManifestURL     string
	UpdateAllowedHosts    []string
	UpdateManifestFetcher updatecheckaction.Fetcher
	OnShutdown            func()
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
	routes := make([]httpx.Route, 0, 11)
	routes = append(routes, health.Routes(health.Config{
		Version:    config.Version,
		DBStatus:   config.DBStatus,
		Security:   security,
		OnShutdown: config.OnShutdown,
	})...)
	routes = append(routes, stocks.Routes(stocks.Config{
		Security: security,
		Service:  stockSearchService(config),
	})...)
	routes = append(routes, searchaction.Routes(searchaction.Config{
		Security: security,
		Service:  documentSearchService(config),
	})...)
	routes = append(routes, marketaction.Routes(marketaction.Config{
		Security:       security,
		MarketProvider: config.MarketProvider,
		Store:          config.MarketStore,
	})...)
	routes = append(routes, newsaction.Routes(newsaction.Config{
		Security:     security,
		NewsProvider: config.NewsProvider,
		Store:        config.NewsStore,
	})...)
	routes = append(routes, watchlistaction.Routes(watchlistaction.Config{
		Security: security,
		Store:    config.WatchlistStore,
	})...)
	routes = append(routes, promptaction.Routes(promptaction.Config{
		Security: security,
		Store:    config.PromptTemplateStore,
	})...)
	routes = append(routes, aiconfigaction.Routes(aiconfigaction.Config{
		Security: security,
		Store:    config.AIConfigStore,
		Tester:   config.AIConfigTester,
	})...)
	routes = append(routes, analysisaction.Routes(analysisaction.Config{
		Security: security,
		Store:    config.AnalysisStore,
		Executor: config.AnalysisExecutor,
		Transact: config.AnalysisTransact,
	})...)
	routes = append(routes, taskaction.Routes(taskaction.Config{
		Security: security,
		Store:    config.TaskStore,
	})...)
	routes = append(routes, tasklogaction.Routes(tasklogaction.Config{
		Security: security,
		Service:  config.TaskLogService,
	})...)
	routes = append(routes, reportaction.Routes(reportaction.Config{
		Security: security,
		Store:    config.ReportStore,
	})...)
	routes = append(routes, providers.Routes(providers.Config{
		Security:       security,
		MarketProvider: config.MarketProvider,
		NewsProvider:   config.NewsProvider,
		Notifier:       config.ProviderNotifier,
	})...)
	routes = append(routes, dashboard.Routes(dashboard.Config{
		Security:       security,
		Input:          config.DashboardInput,
		Store:          config.DashboardStore,
		MarketProvider: config.MarketProvider,
		NewsProvider:   config.NewsProvider,
	})...)
	routes = append(routes, cache.Routes(cache.Config{
		Security:      security,
		StatsProvider: config.CacheStatsProvider,
		Cleaner:       config.CacheCleaner,
	})...)
	routes = append(routes, settingsaction.Routes(settingsaction.Config{
		Security: security,
		Store:    config.SettingsStore,
	})...)
	routes = append(routes, notificationaction.Routes(notificationaction.Config{
		Security: security,
		Store:    config.NotificationStore,
	})...)
	routes = append(routes, datasourcecredentialaction.Routes(datasourcecredentialaction.Config{
		Security: security,
		Service:  config.DataSourceCredentials,
	})...)
	routes = append(routes, scheduleraction.Routes(scheduleraction.Config{
		Security: security,
		Store:    config.SchedulerStore,
		Queue:    config.SchedulerQueue,
		Service:  config.SchedulerService,
	})...)
	routes = append(routes, logexportaction.Routes(logexportaction.Config{
		Security: security,
		Source:   config.LogExportSource,
	})...)
	routes = append(routes, updatecheckaction.Routes(updatecheckaction.Config{
		Security:    security,
		Current:     config.Version,
		ManifestURL: config.UpdateManifestURL,
		AllowedHost: config.UpdateAllowedHosts,
		Settings:    config.SettingsStore,
		Fetcher:     config.UpdateManifestFetcher,
	})...)
	return routes
}

// stockSearchService 返回股票搜索 service；测试或特殊场景可显式注入，默认用本地 store 和 Provider 构造。
func stockSearchService(config Config) stocks.Service {
	if config.StockSearchService != nil {
		return config.StockSearchService
	}
	if config.StockStore == nil && config.MarketProvider == nil {
		return nil
	}
	return searchservice.NewStockSearchService(searchservice.StockSearchConfig{
		Store:    config.StockStore,
		Provider: config.MarketProvider,
	})
}

// documentSearchService 返回菜单范围文档搜索 service；默认用本地 store 构造固定范围搜索。
func documentSearchService(config Config) searchaction.Service {
	if config.DocumentSearchService != nil {
		return config.DocumentSearchService
	}
	if config.DocumentSearchStore == nil {
		return nil
	}
	return searchservice.NewScopedDocumentSearchService(searchservice.DocumentSearchConfig{
		Store: config.DocumentSearchStore,
	})
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
