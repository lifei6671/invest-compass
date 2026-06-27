package dashboard

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	dashboardservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	reportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/report"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Store 是 Dashboard action 聚合真实本地数据所需的数据访问边界。
type Store interface {
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	LatestQuote(ctx context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error)
	GetStocksBySymbols(ctx context.Context, symbols []string) (map[string]model.Stock, error)
	ListVisibleAnalysisReports(ctx context.Context) ([]model.AnalysisReport, error)
	ListTasks(ctx context.Context, limit int) ([]model.Task, error)
	ListMarketNews(ctx context.Context, market string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
}

// Config 是 Dashboard action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	Input          dashboardservice.Input
	Store          Store
	MarketProvider market.MarketProvider
	NewsProvider   newsservice.Provider
}

// Routes 返回 Dashboard 相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/dashboard/summary", Handler: handleSummary(config)},
	}
}

// handleSummary 返回 Dashboard 首版聚合结果；生产环境优先从 Store 读取真实本地数据。
func handleSummary(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		input := config.Input
		if config.Store != nil {
			loaded, err := loadDashboardInput(request.Context(), config)
			if err != nil {
				writeStoreError(response, context, err)
				return
			}
			input = loaded
		}
		httpx.WriteOK(response, dashboardservice.BuildSummary(input), context)
	}
}

// loadDashboardInput 从 DAO 汇总 Dashboard 所需数据，不在 handler 内生成假数据。
func loadDashboardInput(ctx context.Context, config Config) (dashboardservice.Input, error) {
	watchlists, err := config.Store.ListActiveWatchlists(ctx)
	if err != nil {
		return dashboardservice.Input{}, err
	}
	watchlistQuotes := make([]market.Quote, 0, len(watchlists))
	for _, item := range watchlists {
		quote, ok, err := config.Store.LatestQuote(ctx, item.Symbol, 0)
		if err != nil {
			return dashboardservice.Input{}, err
		}
		if !ok {
			continue
		}
		converted, err := quoteModelToService(quote)
		if err != nil {
			return dashboardservice.Input{}, err
		}
		watchlistQuotes = append(watchlistQuotes, converted)
	}

	reports, err := config.Store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		return dashboardservice.Input{}, err
	}
	reportStocks, err := config.Store.GetStocksBySymbols(ctx, reportSymbols(reports))
	if err != nil {
		return dashboardservice.Input{}, err
	}
	tasks, err := config.Store.ListTasks(ctx, 5)
	if err != nil {
		return dashboardservice.Input{}, err
	}
	newsItems, err := config.Store.ListMarketNews(ctx, "", 5, 0)
	if err != nil {
		return dashboardservice.Input{}, err
	}

	marketStatus := market.UnconfiguredProviderStatus()
	if config.MarketProvider != nil {
		marketStatus = config.MarketProvider.Status(ctx)
	}
	newsStatus := newsservice.ProviderStatusFromProvider(ctx, config.NewsProvider)
	return dashboardservice.Input{
		WatchlistQuotes: watchlistQuotes,
		Reports:         reportModelsToService(reports, reportStocks),
		Tasks:           taskModelsToService(tasks),
		MarketNews:      newsModelsToService(newsItems),
		ProviderStatuses: []dashboardservice.ProviderStatus{
			dashboardservice.ProviderStatusFromMarket(marketStatus),
			dashboardservice.ProviderStatusFromNews(newsStatus),
		},
	}, nil
}

// quoteModelToService 转换行情缓存为 Dashboard service 模型，发现非法 symbol 时直接失败。
func quoteModelToService(item model.Quote) (market.Quote, error) {
	symbol, err := stock.ParseSymbol(item.Symbol)
	if err != nil {
		return market.Quote{}, err
	}
	return market.NormalizeQuote(market.Quote{
		Symbol:        symbol,
		Price:         item.Price,
		ChangeAmount:  item.ChangeAmount,
		ChangePercent: item.ChangePercent,
		Open:          item.Open,
		High:          item.High,
		Low:           item.Low,
		PreClose:      item.PreClose,
		Volume:        item.Volume,
		Amount:        item.Amount,
		TurnoverRate:  item.TurnoverRate,
		PE:            item.PE,
		PB:            item.PB,
		QuoteTime:     item.QuoteTime,
		Provider:      item.Provider,
	}), nil
}

// reportSymbols 收集报告关联股票代码，供 Dashboard 一次性补齐股票名称。
func reportSymbols(items []model.AnalysisReport) []string {
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		if item.Symbol == "" {
			continue
		}
		symbols = append(symbols, item.Symbol)
	}
	return symbols
}

// reportModelsToService 转换报告缓存为 Dashboard 业务模型，并带上股票基础资料名称。
func reportModelsToService(items []model.AnalysisReport, stocks map[string]model.Stock) []reportservice.Report {
	reports := make([]reportservice.Report, 0, len(items))
	for _, item := range items {
		var deletedAt *time.Time
		if item.DeletedAt.Valid {
			value := item.DeletedAt.Time
			deletedAt = &value
		}
		stockName := ""
		if stock, ok := stocks[item.Symbol]; ok {
			stockName = stock.Name
		}
		reports = append(reports, reportservice.Report{
			ID:               item.ID,
			TaskID:           item.TaskID,
			Symbol:           item.Symbol,
			StockName:        stockName,
			Title:            item.Title,
			AnalysisType:     item.AnalysisType,
			ModelName:        item.ModelName,
			PromptTemplateID: item.PromptTemplateID,
			InputSnapshot:    item.InputSnapshot,
			ContentMarkdown:  item.ContentMarkdown,
			RiskSummary:      item.RiskSummary,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
			DeletedAt:        deletedAt,
		})
	}
	return reports
}

// taskModelsToService 转换任务缓存为 Dashboard 业务模型。
func taskModelsToService(items []model.Task) []taskservice.Task {
	tasks := make([]taskservice.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, taskservice.Task{
			ID:         item.ID,
			Type:       taskservice.Type(item.Type),
			Status:     taskservice.Status(item.Status),
			Title:      item.Title,
			Progress:   item.Progress,
			Error:      item.ErrorMessage,
			StartedAt:  item.StartedAt,
			FinishedAt: item.FinishedAt,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return tasks
}

// newsModelsToService 转换市场新闻缓存为 Dashboard 业务模型。
func newsModelsToService(items []model.NewsItem) []newsservice.Item {
	newsItems := make([]newsservice.Item, 0, len(items))
	for _, item := range items {
		newsItems = append(newsItems, newsservice.Item{
			ID:          item.ContentHash,
			Source:      item.Source,
			Title:       item.Title,
			URL:         item.URL,
			Summary:     item.Summary,
			ContentHash: item.ContentHash,
			PublishedAt: item.PublishedAt,
		})
	}
	return newsItems
}

// writeStoreError 记录 Dashboard 聚合失败，响应不泄露数据库或 Provider 细节。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	slog.Warn(
		"读取 Dashboard summary 失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50011, "dashboard_store_error", context)
}
