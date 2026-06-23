package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	maxSymbolNewsSymbols    = 30
	skippedReasonNonCNScope = "non_cn_scope"
)

// NewsRefreshStore 定义新闻刷新 runner 需要的最小写库能力。
type NewsRefreshStore interface {
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	SaveNewsItems(ctx context.Context, items []model.NewsItem) error
	UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error
}

// NewsRefreshRunner 执行市场新闻或个股新闻刷新任务。
type NewsRefreshRunner struct {
	Provider newsservice.Provider
	Store    NewsRefreshStore
}

// Run 执行一次新闻刷新，根据 cron_type 分流市场新闻或个股新闻。
func (runner NewsRefreshRunner) Run(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
	if runner.Provider == nil {
		return RunResult{}, fmt.Errorf("news provider is required")
	}
	if runner.Store == nil {
		return RunResult{}, fmt.Errorf("news refresh store is required")
	}
	params, err := parseNewsParams(run.ParamsJSON)
	if err != nil {
		return RunResult{}, err
	}

	switch run.CronType {
	case CronTypeMarketNewsRefresh:
		return runner.runMarket(ctx, run, params)
	case CronTypeSymbolNewsRefresh:
		return runner.runSymbols(ctx, run, params)
	default:
		return RunResult{}, fmt.Errorf("news cron_type is unsupported")
	}
}

// runMarket 执行市场新闻刷新。
func (runner NewsRefreshRunner) runMarket(ctx context.Context, run model.SchedulerRun, params newsParams) (RunResult, error) {
	market := strings.ToUpper(strings.TrimSpace(run.ScopeKey))
	if market == "" {
		market = "CN"
	}
	if market != "CN" {
		return RunResult{SkippedReason: skippedReasonNonCNScope}, nil
	}
	items, err := runner.Provider.Market(ctx, newsservice.MarketRequest{Market: market, Limit: params.Limit})
	if err != nil {
		return RunResult{}, fmt.Errorf("fetch market news: %s", logger.RedactError(err))
	}
	return runner.saveNews(ctx, run, market, market, items)
}

// runSymbols 执行个股新闻刷新。
func (runner NewsRefreshRunner) runSymbols(ctx context.Context, run model.SchedulerRun, params newsParams) (RunResult, error) {
	symbols, err := newsSymbols(ctx, runner.Store, run.ScopeKey)
	if err != nil {
		return RunResult{}, err
	}
	if len(symbols) == 0 {
		return RunResult{SkippedReason: "empty_scope"}, nil
	}
	if len(symbols) > maxSymbolNewsSymbols {
		return RunResult{}, fmt.Errorf("symbol news scope exceeds %d symbols", maxSymbolNewsSymbols)
	}
	result := RunResult{}
	cnSymbols := 0
	for _, rawSymbol := range symbols {
		symbol, err := stockservice.ParseSymbol(rawSymbol)
		if err != nil {
			return result, fmt.Errorf("parse news refresh symbol: %w", err)
		}
		if symbol.Market != "CN" {
			continue
		}
		cnSymbols++
		items, err := runner.Provider.List(ctx, newsservice.ListRequest{Symbol: symbol, Limit: params.Limit})
		if err != nil {
			return result, fmt.Errorf("fetch symbol news %s: %s", symbol.String(), logger.RedactError(err))
		}
		nextResult, err := runner.saveNews(ctx, run, symbol.String(), symbol.Market, items)
		if err != nil {
			return result, err
		}
		result.FetchedCount += nextResult.FetchedCount
		result.WrittenCount += nextResult.WrittenCount
	}
	if cnSymbols == 0 {
		return RunResult{SkippedReason: skippedReasonNonCNScope}, nil
	}
	return result, nil
}

// newsSymbols 返回个股新闻刷新需要处理的 CN 标的；市场范围任务从 active watchlist 派生。
func newsSymbols(ctx context.Context, store NewsRefreshStore, scopeKey string) ([]string, error) {
	trimmedScope := strings.TrimSpace(scopeKey)
	if trimmedScope != "" && !strings.EqualFold(trimmedScope, "CN") {
		return symbolsFromScopeKey(trimmedScope), nil
	}
	watchlists, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active watchlists for news refresh: %w", err)
	}
	symbols := make([]string, 0, len(watchlists))
	for _, item := range watchlists {
		symbol, err := stockservice.ParseSymbol(item.Symbol)
		if err != nil || symbol.Market != "CN" {
			continue
		}
		symbols = append(symbols, symbol.String())
	}
	return symbols, nil
}

// saveNews 标准化、去重并写入新闻缓存和水位。
func (runner NewsRefreshRunner) saveNews(ctx context.Context, run model.SchedulerRun, scopeKey string, market string, items []newsservice.Item) (RunResult, error) {
	normalized, err := newsservice.NormalizeItems(items)
	if err != nil {
		return RunResult{}, fmt.Errorf("normalize news items: %w", err)
	}
	deduped := newsservice.Deduplicate(normalized)
	if len(deduped) == 0 {
		return RunResult{SkippedReason: "empty_news"}, nil
	}
	modelItems := modelNewsItemsFromService(deduped, market)
	if err := runner.Store.SaveNewsItems(ctx, modelItems); err != nil {
		return RunResult{}, fmt.Errorf("save news items: %w", err)
	}
	provider := modelItems[0].Source
	if err := runner.Store.UpsertIngestionWatermark(ctx, newsWatermark(scopeKey, provider, run.TargetDate)); err != nil {
		return RunResult{}, fmt.Errorf("upsert news watermark: %w", err)
	}
	return RunResult{FetchedCount: len(items), WrittenCount: len(modelItems)}, nil
}

type newsParams struct {
	Limit int
}

// parseNewsParams 从 params_json 解析新闻刷新参数。
func parseNewsParams(raw string) (newsParams, error) {
	params := newsParams{Limit: 20}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return newsParams{}, fmt.Errorf("news params_json is invalid: %w", err)
		}
	}
	if params.Limit <= 0 || params.Limit > 100 {
		return newsParams{}, fmt.Errorf("news limit is invalid")
	}
	return params, nil
}

// modelNewsItemsFromService 将标准化新闻模型转换为持久化模型。
func modelNewsItemsFromService(items []newsservice.Item, market string) []model.NewsItem {
	normalizedMarket := strings.ToUpper(strings.TrimSpace(market))
	modelItems := make([]model.NewsItem, 0, len(items))
	for _, item := range items {
		modelItems = append(modelItems, model.NewsItem{
			Source:      item.Source,
			Market:      normalizedMarket,
			Title:       item.Title,
			URL:         item.URL,
			Summary:     item.Summary,
			ContentHash: item.ContentHash,
			Symbols:     encodeStringList(symbolStringsFromNews(item.Symbols)),
			Tags:        encodeStringList(item.Tags),
			PublishedAt: item.PublishedAt,
		})
	}
	return modelItems
}

// symbolStringsFromNews 将新闻中的 symbol 列表转换为字符串。
func symbolStringsFromNews(symbols []stockservice.Symbol) []string {
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		result = append(result, symbol.String())
	}
	return result
}

// encodeStringList 将字符串列表序列化为 JSON，保持新闻缓存格式一致。
func encodeStringList(values []string) string {
	payload, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(payload)
}

// newsWatermark 根据新闻刷新结果生成抓取水位。
func newsWatermark(scopeKey string, provider string, targetDate string) *model.IngestionWatermark {
	lastSuccessAt := time.Now().UTC()
	return &model.IngestionWatermark{
		DataType:      "news",
		ScopeKey:      scopeKey,
		Provider:      provider,
		Period:        "",
		LastSuccessAt: &lastSuccessAt,
		LastTradeDate: targetDate,
	}
}
