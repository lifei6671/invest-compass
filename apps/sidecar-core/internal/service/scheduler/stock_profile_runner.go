package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// StockProfileRefreshStore 定义股票基础资料主动刷新需要的最小数据库能力。
type StockProfileRefreshStore interface {
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	UpsertStocks(ctx context.Context, stocks []model.Stock) error
	UpsertSearchIndexJob(ctx context.Context, job model.SearchIndexJob) error
	UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error
}

// StockProfileRefreshRunner 从已配置行情 Provider 主动刷新股票基础资料缓存。
type StockProfileRefreshRunner struct {
	Provider marketservice.MarketProvider
	Store    StockProfileRefreshStore
}

// Run 执行一次股票基础资料主动刷新；市场范围任务读取 active watchlist，显式 scope_key 支持逗号分隔 symbol。
func (runner StockProfileRefreshRunner) Run(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
	if runner.Provider == nil {
		return RunResult{}, fmt.Errorf("market provider is required")
	}
	if runner.Store == nil {
		return RunResult{}, fmt.Errorf("stock profile refresh store is required")
	}
	symbols, err := stockProfileSymbols(ctx, runner.Store, run.ScopeKey)
	if err != nil {
		return RunResult{}, err
	}
	if len(symbols) == 0 {
		return RunResult{SkippedReason: "empty_scope"}, nil
	}

	result := RunResult{}
	failures := make([]string, 0)
	for _, rawSymbol := range symbols {
		symbol, err := stockservice.ParseSymbol(rawSymbol)
		if err != nil {
			return result, fmt.Errorf("parse stock profile refresh symbol: %w", err)
		}
		if symbol.Market != "CN" {
			continue
		}
		stocks, err := runner.Provider.Search(ctx, symbol.Code)
		if err != nil {
			failures = append(failures, fmt.Sprintf("fetch stock profile %s: %v", symbol.String(), err))
			continue
		}
		stock, ok := matchingStockBasic(stocks, symbol)
		if !ok {
			failures = append(failures, fmt.Sprintf("stock profile not found %s", symbol.String()))
			continue
		}
		result.FetchedCount++
		modelStock := modelStockFromMarketBasic(stock)
		if err := runner.Store.UpsertStocks(ctx, []model.Stock{modelStock}); err != nil {
			return result, fmt.Errorf("upsert stock profile %s: %w", symbol.String(), err)
		}
		if err := runner.Store.UpsertSearchIndexJob(ctx, model.SearchIndexJob{
			DocType:   "stock",
			RefID:     modelStock.Symbol,
			Operation: "upsert",
			Status:    dao.SearchIndexJobStatusPending,
		}); err != nil {
			return result, fmt.Errorf("upsert stock search index job %s: %w", symbol.String(), err)
		}
		if err := runner.Store.UpsertIngestionWatermark(ctx, stockProfileWatermark(modelStock, runner.Provider.Name(), run.TargetDate)); err != nil {
			return result, fmt.Errorf("upsert stock profile watermark %s: %w", symbol.String(), err)
		}
		result.WrittenCount++
	}
	if len(failures) > 0 {
		return result, fmt.Errorf("stock profile refresh partial failure: %s", strings.Join(failures, "; "))
	}
	return result, nil
}

// stockProfileSymbols 返回本次基础资料刷新要处理的 CN 标的。
func stockProfileSymbols(ctx context.Context, store StockProfileRefreshStore, scopeKey string) ([]string, error) {
	trimmedScope := strings.TrimSpace(scopeKey)
	if trimmedScope != "" && !strings.EqualFold(trimmedScope, "CN") {
		return symbolsFromScopeKey(trimmedScope), nil
	}
	watchlists, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active watchlists for stock profile refresh: %w", err)
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

// matchingStockBasic 从搜索结果中选择与目标 symbol 精确匹配的基础资料。
func matchingStockBasic(stocks []marketservice.StockBasic, target stockservice.Symbol) (marketservice.StockBasic, bool) {
	for _, stock := range stocks {
		if stock.Symbol.String() == target.String() {
			return stock, true
		}
	}
	return marketservice.StockBasic{}, false
}

// modelStockFromMarketBasic 将 provider 股票基础资料转换为持久化模型。
func modelStockFromMarketBasic(stock marketservice.StockBasic) model.Stock {
	market := strings.TrimSpace(stock.Market)
	if market == "" {
		market = stock.Symbol.Market
	}
	exchange := strings.TrimSpace(stock.Exchange)
	if exchange == "" {
		exchange = stock.Symbol.Exchange
	}
	code := strings.TrimSpace(stock.Code)
	if code == "" {
		code = stock.Symbol.Code
	}
	return model.Stock{
		Symbol:   stock.Symbol.String(),
		Market:   market,
		Code:     code,
		Name:     stock.Name,
		Exchange: exchange,
		Industry: stock.Industry,
		Concept:  stock.Concept,
		Status:   "LISTED",
	}
}

// stockProfileWatermark 根据股票基础资料刷新结果生成抓取水位。
func stockProfileWatermark(stock model.Stock, provider string, targetDate string) *model.IngestionWatermark {
	lastSuccessAt := time.Now().UTC()
	return &model.IngestionWatermark{
		DataType:      "stock_profile",
		ScopeKey:      stock.Symbol,
		Provider:      provider,
		Period:        "",
		LastSuccessAt: &lastSuccessAt,
		LastTradeDate: targetDate,
	}
}
