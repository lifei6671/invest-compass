package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestQuoteRefreshRunnerFetchesAndPersistsQuotes 验证 A 股行情刷新 runner 会抓取 quote、写缓存并推进水位。
func TestQuoteRefreshRunnerFetchesAndPersistsQuotes(t *testing.T) {
	provider := &quoteProvider{
		quotes: map[string]marketservice.Quote{
			"CN:SH:600519": {
				Symbol:         mustParseSymbol(t, "CN:SH:600519"),
				Price:          1688.5,
				ChangePercent:  0.73,
				TotalMarketCap: 123456789000,
				FloatMarketCap: 98765432100,
				QuoteTime:      time.Date(2026, 6, 19, 10, 0, 0, 0, time.Local),
				Provider:       "test-provider",
			},
		},
	}
	store := &quoteStore{}
	runner := QuoteRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareQuoteRefresh,
		ScopeKey:   "CN:SH:600519",
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run quote refresh: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected quote refresh result: %+v", result)
	}
	if len(store.quotes) != 1 || store.quotes[0].Symbol != "CN:SH:600519" || store.quotes[0].Provider != "test-provider" {
		t.Fatalf("unexpected saved quotes: %+v", store.quotes)
	}
	if store.quotes[0].TotalMarketCap != 123456789000 || store.quotes[0].FloatMarketCap != 98765432100 {
		t.Fatalf("saved quote lost market caps: %+v", store.quotes[0])
	}
	if len(store.watermarks) != 1 || store.watermarks[0].DataType != "quote" || store.watermarks[0].LastTradeDate != "2026-06-19" {
		t.Fatalf("unexpected watermarks: %+v", store.watermarks)
	}
}

// TestQuoteRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket 验证市场范围任务会读取 active watchlist 中的 CN 标的。
func TestQuoteRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket(t *testing.T) {
	provider := &quoteProvider{
		quotes: map[string]marketservice.Quote{
			"CN:SH:600519": {
				Symbol:    mustParseSymbol(t, "CN:SH:600519"),
				Price:     1688.5,
				QuoteTime: time.Date(2026, 6, 19, 10, 0, 0, 0, time.Local),
				Provider:  "test-provider",
			},
		},
	}
	store := &quoteStore{
		watchlists: []model.Watchlist{
			{Symbol: "US:AAPL"},
			{Symbol: "CN:SH:600519"},
		},
	}
	runner := QuoteRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareQuoteRefresh,
		ScopeKey:   "CN",
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run quote refresh from watchlist: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected quote refresh result: %+v", result)
	}
	if len(store.quotes) != 1 || store.quotes[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected only CN watchlist quote to be saved, got %+v", store.quotes)
	}
}

// TestQuoteRefreshRunnerContinuesAfterSingleSymbolFailure 验证多只自选股中单只失败不会阻断后续成功写入。
func TestQuoteRefreshRunnerContinuesAfterSingleSymbolFailure(t *testing.T) {
	provider := &quoteProvider{
		quotes: map[string]marketservice.Quote{
			"CN:SH:600519": {
				Symbol:    mustParseSymbol(t, "CN:SH:600519"),
				Price:     1688.5,
				QuoteTime: time.Date(2026, 6, 19, 10, 0, 0, 0, time.Local),
				Provider:  "test-provider",
			},
		},
		errors: map[string]error{
			"CN:SZ:000001": errors.New("provider timeout"),
		},
	}
	store := &quoteStore{
		watchlists: []model.Watchlist{
			{Symbol: "CN:SZ:000001"},
			{Symbol: "CN:SH:600519"},
		},
	}
	runner := QuoteRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeCNAShareQuoteRefresh,
		ScopeKey:   "CN",
		TargetDate: "2026-06-19",
	})
	if err == nil || !strings.Contains(err.Error(), "fetch quote CN:SZ:000001") {
		t.Fatalf("expected partial failure error for failed symbol, got %v", err)
	}
	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("partial failure should keep successful counts, got %+v", result)
	}
	if len(store.quotes) != 1 || store.quotes[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected successful later quote to be saved, got %+v", store.quotes)
	}
}

// mustParseSymbol 解析测试用股票代码。
func mustParseSymbol(t *testing.T, raw string) stockservice.Symbol {
	t.Helper()
	symbol, err := stockservice.ParseSymbol(raw)
	if err != nil {
		t.Fatalf("parse symbol %s: %v", raw, err)
	}
	return symbol
}

type quoteProvider struct {
	quotes map[string]marketservice.Quote
	errors map[string]error
}

// Name 返回测试 Provider 名称。
func (provider *quoteProvider) Name() string {
	return "test-provider"
}

// Status 返回测试 Provider 可用状态。
func (provider *quoteProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Available: true}
}

// Search 在 quote runner 测试中不使用。
func (provider *quoteProvider) Search(context.Context, string) ([]marketservice.StockBasic, error) {
	return nil, nil
}

// Quote 返回测试预置行情。
func (provider *quoteProvider) Quote(_ context.Context, symbol stockservice.Symbol) (marketservice.Quote, error) {
	if err, ok := provider.errors[symbol.String()]; ok {
		return marketservice.Quote{}, err
	}
	return provider.quotes[symbol.String()], nil
}

// Kline 在 quote runner 测试中不使用。
func (provider *quoteProvider) Kline(context.Context, marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	return nil, nil
}

type quoteStore struct {
	quotes     []model.Quote
	watermarks []model.IngestionWatermark
	watchlists []model.Watchlist
}

// SaveQuote 记录测试保存的行情快照。
func (store *quoteStore) SaveQuote(_ context.Context, quote *model.Quote) error {
	store.quotes = append(store.quotes, *quote)
	return nil
}

// UpsertIngestionWatermark 记录测试更新的抓取水位。
func (store *quoteStore) UpsertIngestionWatermark(_ context.Context, watermark *model.IngestionWatermark) error {
	store.watermarks = append(store.watermarks, *watermark)
	return nil
}

// ListActiveWatchlists 返回测试预置的 active watchlist。
func (store *quoteStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return append([]model.Watchlist(nil), store.watchlists...), nil
}
