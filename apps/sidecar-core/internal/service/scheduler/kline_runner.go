package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// KlineRefreshStore 定义 K 线刷新 runner 需要的最小写库能力。
type KlineRefreshStore interface {
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	SaveKlines(ctx context.Context, klines []model.Kline) error
	UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error
}

// KlineRefreshRunner 执行 A 股 K 线刷新任务，把 provider 返回值写入 K 线缓存和抓取水位。
type KlineRefreshRunner struct {
	Provider marketservice.MarketProvider
	Store    KlineRefreshStore
}

// Run 执行一次 K 线刷新，市场范围任务会读取 active watchlist，显式 scope_key 支持逗号分隔的标准股票代码。
func (runner KlineRefreshRunner) Run(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
	if runner.Provider == nil {
		return RunResult{}, fmt.Errorf("market provider is required")
	}
	if runner.Store == nil {
		return RunResult{}, fmt.Errorf("kline refresh store is required")
	}
	params, err := parseKlineParams(run.ParamsJSON)
	if err != nil {
		return RunResult{}, err
	}
	symbols, err := klineSymbols(ctx, runner.Store, run.ScopeKey)
	if err != nil {
		return RunResult{}, err
	}
	if len(symbols) == 0 {
		return RunResult{SkippedReason: "empty_scope"}, nil
	}

	result := RunResult{}
	for _, rawSymbol := range symbols {
		symbol, err := stockservice.ParseSymbol(rawSymbol)
		if err != nil {
			return result, fmt.Errorf("parse kline refresh symbol: %w", err)
		}
		if symbol.Market != "CN" {
			continue
		}
		bars, err := runner.Provider.Kline(ctx, marketservice.KlineRequest{
			Symbol: symbol,
			Period: params.Period,
			Adjust: params.Adjust,
			Limit:  params.Limit,
		})
		if err != nil {
			return result, fmt.Errorf("fetch kline %s: %w", symbol.String(), err)
		}
		if len(bars) == 0 {
			return result, fmt.Errorf("fetch kline %s returned empty bars", symbol.String())
		}
		result.FetchedCount += len(bars)
		modelKlines := modelKlinesFromMarket(bars)
		if err := runner.Store.SaveKlines(ctx, modelKlines); err != nil {
			return result, fmt.Errorf("save kline %s: %w", symbol.String(), err)
		}
		result.WrittenCount += len(modelKlines)
		if err := runner.Store.UpsertIngestionWatermark(ctx, klineWatermark(symbol.String(), string(params.Period), providerFromKlines(modelKlines), run.TargetDate)); err != nil {
			return result, fmt.Errorf("upsert kline watermark %s: %w", symbol.String(), err)
		}
	}
	return result, nil
}

// klineSymbols 返回本次 K 线刷新需要处理的 CN 标的；市场范围任务从 active watchlist 派生。
func klineSymbols(ctx context.Context, store KlineRefreshStore, scopeKey string) ([]string, error) {
	trimmedScope := strings.TrimSpace(scopeKey)
	if trimmedScope != "" && !strings.EqualFold(trimmedScope, "CN") {
		return symbolsFromScopeKey(trimmedScope), nil
	}
	watchlists, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active watchlists for kline refresh: %w", err)
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

type klineParams struct {
	Period marketservice.Period
	Adjust marketservice.Adjust
	Limit  int
}

// parseKlineParams 从 run params_json 解析 K 线参数，并提供首批默认值。
func parseKlineParams(raw string) (klineParams, error) {
	params := struct {
		Period string `json:"period"`
		Adjust string `json:"adjust"`
		Limit  int    `json:"limit"`
	}{
		Period: string(marketservice.PeriodDay),
		Adjust: string(marketservice.AdjustNone),
		Limit:  120,
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return klineParams{}, fmt.Errorf("kline params_json is invalid: %w", err)
		}
	}
	period := marketservice.Period(strings.TrimSpace(params.Period))
	adjust := marketservice.Adjust(strings.TrimSpace(params.Adjust))
	switch period {
	case marketservice.PeriodMinute, marketservice.PeriodDay, marketservice.PeriodWeek, marketservice.PeriodMonth, marketservice.PeriodQuarter, marketservice.PeriodYear:
	default:
		return klineParams{}, fmt.Errorf("kline period is invalid")
	}
	switch adjust {
	case marketservice.AdjustNone, marketservice.AdjustForward, marketservice.AdjustBackward:
	default:
		return klineParams{}, fmt.Errorf("kline adjust is invalid")
	}
	if params.Limit <= 0 || params.Limit > 500 {
		return klineParams{}, fmt.Errorf("kline limit is invalid")
	}
	return klineParams{Period: period, Adjust: adjust, Limit: params.Limit}, nil
}

// modelKlinesFromMarket 将 market service K 线转换为持久化模型。
func modelKlinesFromMarket(bars []marketservice.KlineBar) []model.Kline {
	klines := make([]model.Kline, 0, len(bars))
	for _, bar := range bars {
		klines = append(klines, model.Kline{
			Symbol:    bar.Symbol.String(),
			Period:    string(bar.Period),
			Adjust:    string(bar.Adjust),
			TradeDate: bar.TradeDate,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Amount:    bar.Amount,
			Provider:  bar.Provider,
		})
	}
	return klines
}

// providerFromKlines 返回本次 K 线数据来源，空数据时返回空字符串。
func providerFromKlines(klines []model.Kline) string {
	if len(klines) == 0 {
		return ""
	}
	return klines[0].Provider
}

// klineWatermark 根据 K 线执行结果生成抓取水位。
func klineWatermark(symbol string, period string, provider string, targetDate string) *model.IngestionWatermark {
	lastSuccessAt := time.Now().UTC()
	return &model.IngestionWatermark{
		DataType:      "kline",
		ScopeKey:      symbol,
		Provider:      provider,
		Period:        period,
		LastSuccessAt: &lastSuccessAt,
		LastTradeDate: targetDate,
	}
}
