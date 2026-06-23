package providers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	notificationservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/notification"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Notifier 是 Provider 状态 action 的通知生成边界。
type Notifier interface {
	NotifyProviderError(ctx context.Context, input notificationservice.ProviderErrorInput) (bool, error)
}

// Config 是 Provider 状态 action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	MarketProvider market.MarketProvider
	NewsProvider   news.Provider
	Notifier       Notifier
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
		notifyProviderError(request.Context(), config.Notifier, marketStatus.Name, marketStatus.Source, marketStatus.Available, marketStatus.LastError)
		notifyProviderError(request.Context(), config.Notifier, newsStatus.Name, newsStatus.Source, newsStatus.Available, newsStatus.LastError)
		summary := dashboard.BuildSummary(dashboard.Input{
			ProviderStatuses: []dashboard.ProviderStatus{
				dashboard.ProviderStatusFromMarket(marketStatus),
				dashboard.ProviderStatusFromNews(newsStatus),
			},
		})
		httpx.WriteOK(response, summary.ProviderStatuses, context)
	}
}

// notifyProviderError 只对已配置 Provider 的真实异常生成通知，未配置空态不刷通知。
func notifyProviderError(ctx context.Context, notifier Notifier, name string, source string, available bool, lastError string) {
	if notifier == nil || available || strings.TrimSpace(lastError) == "" || strings.TrimSpace(source) == "unconfigured" {
		return
	}
	if _, err := notifier.NotifyProviderError(ctx, notificationservice.ProviderErrorInput{
		Name:      name,
		Source:    source,
		LastError: logger.RedactText(lastError),
	}); err != nil {
		slog.Warn(
			"Provider 异常通知写入失败",
			"provider", strings.TrimSpace(name),
			"source", strings.TrimSpace(source),
			"error", logger.RedactText(err.Error()),
		)
	}
}
