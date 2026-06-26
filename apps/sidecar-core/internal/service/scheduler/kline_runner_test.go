package scheduler

import (
	"context"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestKlineRefreshRunnerFetchesAndPersistsBars 验证 K 线刷新 runner 会按 params 抓取并幂等写入 K 线缓存。
func TestKlineRefreshRunnerFetchesAndPersistsBars(t *testing.T) {
	provider := &klineProvider{
		bars: []marketservice.KlineBar{
			{Symbol: mustParseSymbol(t, "CN:SH:600519"), Period: marketservice.PeriodDay, Adjust: marketservice.AdjustNone, TradeDate: "2026-06-18", Close: 100, Provider: "test-provider"},
			{Symbol: mustParseSymbol(t, "CN:SH:600519"), Period: marketservice.PeriodDay, Adjust: marketservice.AdjustNone, TradeDate: "2026-06-19", Close: 101, Provider: "test-provider"},
		},
	}
	store := &klineStore{}
	runner := KlineRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareKlineRefresh,
		DataType:   "kline",
		Period:     "day",
		ScopeKey:   "CN:SH:600519",
		ParamsJSON: `{"period":"day","adjust":"none","limit":120}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run kline refresh: %v", err)
	}

	if result.FetchedCount != 2 || result.WrittenCount != 2 {
		t.Fatalf("unexpected kline refresh result: %+v", result)
	}
	if provider.request.Symbol.String() != "CN:SH:600519" || provider.request.Period != marketservice.PeriodDay || provider.request.Adjust != marketservice.AdjustNone || provider.request.Limit != 120 {
		t.Fatalf("unexpected provider request: %+v", provider.request)
	}
	if len(store.klines) != 2 || store.klines[1].TradeDate != "2026-06-19" || store.klines[1].Close != 101 {
		t.Fatalf("unexpected saved klines: %+v", store.klines)
	}
	if len(store.watermarks) != 1 || store.watermarks[0].DataType != "kline" || store.watermarks[0].Period != "day" {
		t.Fatalf("unexpected watermarks: %+v", store.watermarks)
	}
}

// TestKlineRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket 验证市场范围 K 线任务会读取 active watchlist 中的 CN 标的。
func TestKlineRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket(t *testing.T) {
	provider := &klineProvider{
		bars: []marketservice.KlineBar{
			{Symbol: mustParseSymbol(t, "CN:SH:600519"), Period: marketservice.PeriodDay, Adjust: marketservice.AdjustNone, TradeDate: "2026-06-19", Close: 101, Provider: "test-provider"},
		},
	}
	store := &klineStore{
		watchlists: []model.Watchlist{
			{Symbol: "US:AAPL"},
			{Symbol: "CN:SH:600519"},
		},
	}
	runner := KlineRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareKlineRefresh,
		DataType:   "kline",
		Period:     "day",
		ScopeKey:   "CN",
		ParamsJSON: `{"period":"day","adjust":"none","limit":120}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run kline refresh from watchlist: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected kline refresh result: %+v", result)
	}
	if provider.request.Symbol.String() != "CN:SH:600519" {
		t.Fatalf("expected CN watchlist symbol request, got %+v", provider.request)
	}
	if len(store.klines) != 1 || store.klines[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected only CN watchlist kline to be saved, got %+v", store.klines)
	}
}

// TestKlineRefreshRunnerAcceptsLongCyclePeriods 验证 K 线任务支持月线以上长周期缓存刷新。
func TestKlineRefreshRunnerAcceptsLongCyclePeriods(t *testing.T) {
	tests := []marketservice.Period{
		marketservice.PeriodMonth,
		marketservice.PeriodQuarter,
		marketservice.PeriodYear,
	}

	for _, period := range tests {
		provider := &klineProvider{
			bars: []marketservice.KlineBar{
				{Symbol: mustParseSymbol(t, "CN:SH:600519"), Period: period, Adjust: marketservice.AdjustNone, TradeDate: "2026-06-19", Close: 101, Provider: "test-provider"},
			},
		}
		store := &klineStore{}
		runner := KlineRefreshRunner{Provider: provider, Store: store}

		result, err := runner.Run(context.Background(), model.SchedulerRun{
			CronType:   CronTypeCNAShareKlineRefresh,
			DataType:   "kline",
			Period:     string(period),
			ScopeKey:   "CN:SH:600519",
			ParamsJSON: `{"period":"` + string(period) + `","adjust":"none","limit":120}`,
			TargetDate: "2026-06-19",
		})
		if err != nil {
			t.Fatalf("run kline refresh for %q: %v", period, err)
		}
		if result.FetchedCount != 1 || result.WrittenCount != 1 {
			t.Fatalf("unexpected kline refresh result for %q: %+v", period, result)
		}
		if provider.request.Period != period {
			t.Fatalf("expected provider period %q, got %+v", period, provider.request)
		}
		if len(store.watermarks) != 1 || store.watermarks[0].Period != string(period) {
			t.Fatalf("unexpected watermark for %q: %+v", period, store.watermarks)
		}
	}
}

// TestKlineRefreshRunnerFailsOnEmptyBars 验证 Provider 返回空 K 线时不会写入水位，避免把空数据误认为刷新成功。
func TestKlineRefreshRunnerFailsOnEmptyBars(t *testing.T) {
	runner := KlineRefreshRunner{Provider: &klineProvider{}, Store: &klineStore{}}

	_, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareKlineRefresh,
		DataType:   "kline",
		Period:     "day",
		ScopeKey:   "CN:SH:600519",
		ParamsJSON: `{"period":"day","adjust":"none","limit":120}`,
		TargetDate: "2026-06-19",
	})
	if err == nil || !strings.Contains(err.Error(), "returned empty bars") {
		t.Fatalf("expected empty bars error, got %v", err)
	}
}

// TestKlineRefreshRunnerRejectsInvalidPeriod 验证非法周期会在调用 Provider 前快速失败。
func TestKlineRefreshRunnerRejectsInvalidPeriod(t *testing.T) {
	provider := &klineProvider{}
	runner := KlineRefreshRunner{Provider: provider, Store: &klineStore{}}

	_, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareKlineRefresh,
		DataType:   "kline",
		Period:     "halfyear",
		ScopeKey:   "CN:SH:600519",
		ParamsJSON: `{"period":"halfyear","adjust":"none","limit":120}`,
		TargetDate: "2026-06-19",
	})
	if err == nil || !strings.Contains(err.Error(), "kline period is invalid") {
		t.Fatalf("expected invalid period error, got %v", err)
	}
	if provider.request.Symbol.String() != "" {
		t.Fatalf("provider should not be called for invalid period, got %+v", provider.request)
	}
}

type klineProvider struct {
	request marketservice.KlineRequest
	bars    []marketservice.KlineBar
}

// Name 返回测试 Provider 名称。
func (provider *klineProvider) Name() string {
	return "test-provider"
}

// Status 返回测试 Provider 可用状态。
func (provider *klineProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Available: true}
}

// Search 在 K 线 runner 测试中不使用。
func (provider *klineProvider) Search(context.Context, string) ([]marketservice.StockBasic, error) {
	return nil, nil
}

// Quote 在 K 线 runner 测试中不使用。
func (provider *klineProvider) Quote(context.Context, stockservice.Symbol) (marketservice.Quote, error) {
	return marketservice.Quote{}, nil
}

// Kline 记录请求并返回测试预置 K 线。
func (provider *klineProvider) Kline(_ context.Context, request marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	provider.request = request
	return provider.bars, nil
}

type klineStore struct {
	klines     []model.Kline
	watermarks []model.IngestionWatermark
	watchlists []model.Watchlist
}

// SaveKlines 记录测试保存的 K 线。
func (store *klineStore) SaveKlines(_ context.Context, klines []model.Kline) error {
	store.klines = append(store.klines, klines...)
	return nil
}

// UpsertIngestionWatermark 记录测试更新的抓取水位。
func (store *klineStore) UpsertIngestionWatermark(_ context.Context, watermark *model.IngestionWatermark) error {
	store.watermarks = append(store.watermarks, *watermark)
	return nil
}

// ListActiveWatchlists 返回测试预置的 active watchlist。
func (store *klineStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return append([]model.Watchlist(nil), store.watchlists...), nil
}
