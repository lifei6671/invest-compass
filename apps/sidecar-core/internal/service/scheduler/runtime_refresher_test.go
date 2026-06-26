package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestRuntimeRefresherStartupRefreshesIndexesAndWatchlist 验证进程启动后会异步刷新指数和全部自选股缓存。
func TestRuntimeRefresherStartupRefreshesIndexesAndWatchlist(t *testing.T) {
	store := &runtimeRefreshStore{
		watchlists: []model.Watchlist{{Symbol: "CN:SH:600519"}},
		settings: map[string]string{
			runtimeQuoteIntervalKey: "60s",
		},
	}
	provider := &runtimeMarketProvider{}
	refresher := newTestRuntimeRefresher(t, store, provider, nil, func() time.Time {
		return time.Date(2026, 6, 25, 9, 35, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	})

	refresher.runQuoteCycle(context.Background(), true)

	if len(store.quotes) != 5 {
		t.Fatalf("expected four indexes and one watchlist quote, got %d: %+v", len(store.quotes), store.quotes)
	}
	if len(store.klines) != 5 {
		t.Fatalf("expected minute klines for all runtime symbols, got %d", len(store.klines))
	}
	if provider.quoteCalls["CN:SH:000001"] != 1 || provider.quoteCalls["CN:SH:600519"] != 1 {
		t.Fatalf("unexpected quote calls: %+v", provider.quoteCalls)
	}
	if provider.klineCalls["CN:SH:000001"] != 1 || provider.klineCalls["CN:SH:600519"] != 1 {
		t.Fatalf("unexpected kline calls: %+v", provider.klineCalls)
	}
}

// TestRuntimeRefresherSkipsPeriodicQuotesOutsideTradingTime 验证周期行情刷新只在交易时段执行。
func TestRuntimeRefresherSkipsPeriodicQuotesOutsideTradingTime(t *testing.T) {
	store := &runtimeRefreshStore{}
	provider := &runtimeMarketProvider{}
	refresher := newTestRuntimeRefresher(t, store, provider, nil, func() time.Time {
		return time.Date(2026, 6, 25, 16, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	})

	refresher.runQuoteCycle(context.Background(), false)

	if len(provider.quoteCalls) != 0 || len(provider.klineCalls) != 0 {
		t.Fatalf("expected no market calls after close, got quotes=%+v klines=%+v", provider.quoteCalls, provider.klineCalls)
	}
}

// TestRuntimeRefresherRefreshesNewsOutsideTradingTime 验证新闻刷新不受 A 股交易时段限制。
func TestRuntimeRefresherRefreshesNewsOutsideTradingTime(t *testing.T) {
	store := &runtimeRefreshStore{}
	provider := &runtimeNewsProvider{
		items: []newsservice.Item{{
			Source:      "财联社电报",
			Title:       "周末新闻",
			URL:         "https://www.cls.cn/detail/123",
			Summary:     "周末仍应刷新新闻",
			PublishedAt: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
		}},
	}
	refresher := newTestRuntimeRefresher(t, store, nil, provider, func() time.Time {
		return time.Date(2026, 6, 27, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	})

	refresher.runNewsCycle(context.Background())

	if provider.marketCalls != 1 {
		t.Fatalf("expected one market news refresh, got %d", provider.marketCalls)
	}
	if len(store.newsItems) != 1 {
		t.Fatalf("expected saved news item, got %+v", store.newsItems)
	}
}

// TestRuntimeRefresherReadsIntervalsFromSettings 验证运行期刷新频率优先读取数据源设置。
func TestRuntimeRefresherReadsIntervalsFromSettings(t *testing.T) {
	store := &runtimeRefreshStore{settings: map[string]string{
		runtimeBasicQuoteIntervalKey: "120s",
		runtimeQuoteIntervalKey:      "15s",
		runtimeNewsIntervalKey:       "5m",
	}}
	refresher := newTestRuntimeRefresher(t, store, nil, nil, nil)

	quoteInterval, quoteEnabled := refresher.quoteInterval(context.Background())
	newsInterval, newsEnabled := refresher.newsInterval(context.Background())

	if !quoteEnabled || quoteInterval != 15*time.Second {
		t.Fatalf("unexpected quote interval enabled=%v interval=%s", quoteEnabled, quoteInterval)
	}
	if !newsEnabled || newsInterval != 5*time.Minute {
		t.Fatalf("unexpected news interval enabled=%v interval=%s", newsEnabled, newsInterval)
	}
}

// TestRuntimeRefresherManualQuoteIntervalSkipsStartupRefresh 验证手动刷新模式不会在 sidecar 启动时主动访问外部行情源。
func TestRuntimeRefresherManualQuoteIntervalSkipsStartupRefresh(t *testing.T) {
	store := &runtimeRefreshStore{
		watchlists: []model.Watchlist{{Symbol: "CN:SH:600519"}},
		settings: map[string]string{
			runtimeQuoteIntervalKey: "manual",
		},
	}
	provider := &runtimeMarketProvider{quoteNotified: make(chan string, 1)}
	refresher := newTestRuntimeRefresher(t, store, provider, nil, func() time.Time {
		return time.Date(2026, 6, 25, 9, 35, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		refresher.quoteLoop(ctx)
		close(done)
	}()

	select {
	case symbol := <-provider.quoteNotified:
		cancel()
		t.Fatalf("expected manual mode to skip startup quote refresh, got call for %s", symbol)
	case <-time.After(50 * time.Millisecond):
		cancel()
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("quoteLoop did not stop after context cancellation")
	}
}

func newTestRuntimeRefresher(t *testing.T, store *runtimeRefreshStore, marketProvider marketservice.MarketProvider, newsProvider newsservice.Provider, now func() time.Time) *RuntimeRefresher {
	t.Helper()
	refresher, err := NewRuntimeRefresher(store, marketProvider, newsProvider, now)
	if err != nil {
		t.Fatalf("NewRuntimeRefresher returned error: %v", err)
	}
	return refresher
}

type runtimeRefreshStore struct {
	watchlists []model.Watchlist
	settings   map[string]string
	quotes     []model.Quote
	klines     []model.Kline
	newsItems  []model.NewsItem
	watermarks []model.IngestionWatermark
}

// ListActiveWatchlists 返回测试预置自选股。
func (store *runtimeRefreshStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return append([]model.Watchlist(nil), store.watchlists...), nil
}

// SaveQuote 记录运行期刷新写入的 quote。
func (store *runtimeRefreshStore) SaveQuote(_ context.Context, quote *model.Quote) error {
	store.quotes = append(store.quotes, *quote)
	return nil
}

// SaveKlines 记录运行期刷新写入的分时 K 线。
func (store *runtimeRefreshStore) SaveKlines(_ context.Context, klines []model.Kline) error {
	store.klines = append(store.klines, klines...)
	return nil
}

// SaveNewsItems 记录运行期刷新写入的新闻。
func (store *runtimeRefreshStore) SaveNewsItems(_ context.Context, items []model.NewsItem) error {
	store.newsItems = append(store.newsItems, items...)
	return nil
}

// UpsertIngestionWatermark 记录运行期刷新推进的水位。
func (store *runtimeRefreshStore) UpsertIngestionWatermark(_ context.Context, watermark *model.IngestionWatermark) error {
	store.watermarks = append(store.watermarks, *watermark)
	return nil
}

// GetSettings 返回测试设置项。
func (store *runtimeRefreshStore) GetSettings(_ context.Context, keys []string) ([]model.Setting, error) {
	items := make([]model.Setting, 0, len(keys))
	for _, key := range keys {
		if value, ok := store.settings[key]; ok {
			items = append(items, model.Setting{Key: key, Value: value})
		}
	}
	return items, nil
}

type runtimeMarketProvider struct {
	quoteCalls    map[string]int
	klineCalls    map[string]int
	quoteNotified chan string
}

// Name 返回测试行情 Provider 名称。
func (provider *runtimeMarketProvider) Name() string {
	return "runtime-market-provider"
}

// Status 返回测试行情 Provider 状态。
func (provider *runtimeMarketProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Available: true}
}

// Search 在运行期刷新测试中不使用。
func (provider *runtimeMarketProvider) Search(context.Context, string) ([]marketservice.StockBasic, error) {
	return nil, nil
}

// Quote 返回测试行情并记录调用次数。
func (provider *runtimeMarketProvider) Quote(_ context.Context, symbol stockservice.Symbol) (marketservice.Quote, error) {
	if provider.quoteCalls == nil {
		provider.quoteCalls = map[string]int{}
	}
	provider.quoteCalls[symbol.String()]++
	if provider.quoteNotified != nil {
		select {
		case provider.quoteNotified <- symbol.String():
		default:
		}
	}
	return marketservice.Quote{
		Symbol:        symbol,
		Price:         10,
		ChangeAmount:  1,
		ChangePercent: 10,
		QuoteTime:     time.Date(2026, 6, 25, 9, 35, 0, 0, time.UTC),
		Provider:      provider.Name(),
	}, nil
}

// Kline 返回测试分时数据并记录调用次数。
func (provider *runtimeMarketProvider) Kline(_ context.Context, request marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	if provider.klineCalls == nil {
		provider.klineCalls = map[string]int{}
	}
	provider.klineCalls[request.Symbol.String()]++
	return []marketservice.KlineBar{{
		Symbol:    request.Symbol,
		Period:    request.Period,
		Adjust:    request.Adjust,
		TradeDate: "2026-06-25T09:35:00+08:00",
		Close:     10,
		Provider:  provider.Name(),
	}}, nil
}

type runtimeNewsProvider struct {
	items       []newsservice.Item
	marketCalls int
}

// Name 返回测试新闻 Provider 名称。
func (provider *runtimeNewsProvider) Name() string {
	return "runtime-news-provider"
}

// List 在运行期市场新闻刷新测试中不使用。
func (provider *runtimeNewsProvider) List(context.Context, newsservice.ListRequest) ([]newsservice.Item, error) {
	return nil, nil
}

// Market 返回测试市场新闻并记录调用次数。
func (provider *runtimeNewsProvider) Market(context.Context, newsservice.MarketRequest) ([]newsservice.Item, error) {
	provider.marketCalls++
	return provider.items, nil
}
