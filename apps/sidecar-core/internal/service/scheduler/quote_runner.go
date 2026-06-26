package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// QuoteRefreshStore 定义行情刷新 runner 需要的最小写库能力。
type QuoteRefreshStore interface {
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	SaveQuote(ctx context.Context, quote *model.Quote) error
	UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error
}

// QuoteRefreshRunner 执行 A 股行情刷新任务，把 provider 返回值写入 quote 缓存和抓取水位。
type QuoteRefreshRunner struct {
	Provider marketservice.MarketProvider
	Store    QuoteRefreshStore
}

// Run 执行一次行情刷新，市场范围任务会读取 active watchlist，显式 scope_key 支持逗号分隔的标准股票代码。
func (runner QuoteRefreshRunner) Run(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
	if runner.Provider == nil {
		return RunResult{}, fmt.Errorf("market provider is required")
	}
	if runner.Store == nil {
		return RunResult{}, fmt.Errorf("quote refresh store is required")
	}
	symbols, err := quoteSymbols(ctx, runner.Store, run.ScopeKey)
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
			return result, fmt.Errorf("parse quote refresh symbol: %w", err)
		}
		if symbol.Market != "CN" {
			continue
		}
		quote, err := runner.Provider.Quote(ctx, symbol)
		if err != nil {
			failures = append(failures, fmt.Sprintf("fetch quote %s: %v", symbol.String(), err))
			continue
		}
		result.FetchedCount++
		modelQuote := modelQuoteFromMarket(quote)
		if err := runner.Store.SaveQuote(ctx, &modelQuote); err != nil {
			return result, fmt.Errorf("save quote %s: %w", symbol.String(), err)
		}
		result.WrittenCount++
		if err := runner.Store.UpsertIngestionWatermark(ctx, quoteWatermark(modelQuote, run.TargetDate)); err != nil {
			return result, fmt.Errorf("upsert quote watermark %s: %w", symbol.String(), err)
		}
	}
	if len(failures) > 0 {
		return result, fmt.Errorf("quote refresh partial failure: %s", strings.Join(failures, "; "))
	}
	return result, nil
}

// quoteSymbols 返回本次行情刷新需要处理的 CN 标的；市场范围任务从 active watchlist 派生。
func quoteSymbols(ctx context.Context, store QuoteRefreshStore, scopeKey string) ([]string, error) {
	trimmedScope := strings.TrimSpace(scopeKey)
	if trimmedScope != "" && !strings.EqualFold(trimmedScope, "CN") {
		return symbolsFromScopeKey(trimmedScope), nil
	}
	watchlists, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active watchlists for quote refresh: %w", err)
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

// symbolsFromScopeKey 从执行记录范围字段中解析股票代码列表。
func symbolsFromScopeKey(scopeKey string) []string {
	parts := strings.Split(scopeKey, ",")
	symbols := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			symbols = append(symbols, trimmed)
		}
	}
	return symbols
}

// modelQuoteFromMarket 将 market service 行情模型转换为持久化模型。
func modelQuoteFromMarket(quote marketservice.Quote) model.Quote {
	quote = marketservice.NormalizeQuote(quote)
	return model.Quote{
		Symbol:         quote.Symbol.String(),
		Price:          quote.Price,
		ChangeAmount:   quote.ChangeAmount,
		ChangePercent:  quote.ChangePercent,
		Open:           quote.Open,
		High:           quote.High,
		Low:            quote.Low,
		PreClose:       quote.PreClose,
		Volume:         quote.Volume,
		Amount:         quote.Amount,
		TurnoverRate:   quote.TurnoverRate,
		PE:             quote.PE,
		PB:             quote.PB,
		TotalMarketCap: quote.TotalMarketCap,
		FloatMarketCap: quote.FloatMarketCap,
		QuoteTime:      quote.QuoteTime,
		Provider:       quote.Provider,
	}
}

// quoteWatermark 根据保存的行情生成抓取水位。
func quoteWatermark(quote model.Quote, targetDate string) *model.IngestionWatermark {
	lastSuccessAt := time.Now().UTC()
	lastTradeDate := strings.TrimSpace(targetDate)
	if lastTradeDate == "" && !quote.QuoteTime.IsZero() {
		lastTradeDate = quote.QuoteTime.Format("2006-01-02")
	}
	return &model.IngestionWatermark{
		DataType:      "quote",
		ScopeKey:      quote.Symbol,
		Provider:      quote.Provider,
		Period:        "",
		LastSuccessAt: &lastSuccessAt,
		LastTradeDate: lastTradeDate,
	}
}
