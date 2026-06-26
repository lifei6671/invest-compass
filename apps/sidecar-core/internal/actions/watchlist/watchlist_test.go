package watchlist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestHandleListIncludesStockProfileFields 验证自选股列表关联 stocks 表返回展示资料，避免前端只能用 symbol 降级展示。
func TestHandleListIncludesStockProfileFields(t *testing.T) {
	createdAt := time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "600000.SH",
			SortOrder: 10,
			Tags:      `[\"银行\"]`,
			Note:      "低估值观察",
			CreatedAt: createdAt,
			UpdatedAt: createdAt.Add(time.Minute),
		}},
		stocks: map[string]model.Stock{
			"600000.SH": {
				Symbol:   "600000.SH",
				Name:     "浦发银行",
				Code:     "600000",
				Market:   "CN",
				Exchange: "SH",
				Industry: "银行",
				Concept:  `["低估值","大金融"]`,
				ListDate: "1999-11-10",
				Status:   "active",
				FullName: "上海浦东发展银行股份有限公司",
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(store.profileSymbols) != 1 || store.profileSymbols[0] != "600000.SH" {
		t.Fatalf("expected profile lookup for watchlist symbol, got %#v", store.profileSymbols)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	items := response.Data.(map[string]any)["items"].([]any)
	first := items[0].(map[string]any)
	if first["name"] != "浦发银行" || first["industry"] != "银行" || first["full_name"] != "上海浦东发展银行股份有限公司" {
		t.Fatalf("unexpected watchlist item profile fields: %#v", first)
	}
	if first["created_at"] != "2026-06-24T09:30:00Z" || first["updated_at"] != "2026-06-24T09:31:00Z" {
		t.Fatalf("expected watchlist timestamps in response, got %#v", first)
	}
	concepts := first["concepts"].([]any)
	if len(concepts) != 2 || concepts[0] != "低估值" || concepts[1] != "大金融" {
		t.Fatalf("unexpected concepts: %#v", concepts)
	}
}

// TestHandleListIncludesCachedQuoteAndTrend 验证自选股列表直接读取本地行情和分时缓存，避免前端进入页面时逐股打远端接口。
func TestHandleListIncludesCachedQuoteAndTrend(t *testing.T) {
	createdAt := time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)
	quoteTime := time.Date(2026, 6, 25, 10, 8, 0, 0, time.UTC)
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "600000.SH",
			SortOrder: 10,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}},
		quotes: map[string]model.Quote{
			"600000.SH": {
				Symbol:        "600000.SH",
				Price:         9.12,
				ChangeAmount:  0.08,
				ChangePercent: 0.88,
				Amount:        98000000,
				TurnoverRate:  0.6,
				PE:            5.4,
				QuoteTime:     quoteTime,
				UpdatedAt:     quoteTime,
			},
		},
		klines: map[string][]model.Kline{
			"600000.SH|minute|none": {
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-25 09:30", Close: 9.01},
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-25 09:31", Close: 9.12},
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security:    httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:       store,
		MarketStore: store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	first := response.Data.(map[string]any)["items"].([]any)[0].(map[string]any)
	quote := first["quote"].(map[string]any)
	if quote["price"] != 9.12 || quote["turnover_rate"] != 0.6 || quote["pe"] != 5.4 {
		t.Fatalf("expected cached quote fields, got %#v", quote)
	}
	points := first["trend_points"].([]any)
	if len(points) != 2 || points[0] != 9.01 || points[1] != 9.12 {
		t.Fatalf("expected cached trend points, got %#v", points)
	}
}

// TestHandleListPrefersCanonicalCacheForLegacySymbol 验证历史格式自选股优先读取规范化缓存，避免刷新后仍显示旧缓存。
func TestHandleListPrefersCanonicalCacheForLegacySymbol(t *testing.T) {
	createdAt := time.Date(2026, 6, 26, 9, 30, 0, 0, time.UTC)
	staleTime := time.Date(2026, 6, 26, 9, 31, 0, 0, time.UTC)
	freshTime := time.Date(2026, 6, 26, 10, 8, 0, 0, time.UTC)
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "600000.SH",
			SortOrder: 10,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}},
		quotes: map[string]model.Quote{
			"600000.SH": {
				Symbol:    "600000.SH",
				Price:     8.88,
				QuoteTime: staleTime,
				UpdatedAt: staleTime,
			},
			"CN:SH:600000": {
				Symbol:    "CN:SH:600000",
				Price:     9.66,
				QuoteTime: freshTime,
				UpdatedAt: freshTime,
			},
		},
		klines: map[string][]model.Kline{
			"600000.SH|minute|none": {
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-26 09:30", Close: 8.80},
			},
			"CN:SH:600000|minute|none": {
				{Symbol: "CN:SH:600000", Period: "minute", Adjust: "none", TradeDate: "2026-06-26 10:07", Close: 9.60},
				{Symbol: "CN:SH:600000", Period: "minute", Adjust: "none", TradeDate: "2026-06-26 10:08", Close: 9.66},
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security:    httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:       store,
		MarketStore: store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	first := response.Data.(map[string]any)["items"].([]any)[0].(map[string]any)
	quote := first["quote"].(map[string]any)
	if quote["price"] != 9.66 {
		t.Fatalf("expected canonical cached quote, got %#v", quote)
	}
	points := first["trend_points"].([]any)
	if len(points) != 2 || points[0] != 9.60 || points[1] != 9.66 {
		t.Fatalf("expected canonical cached trend points, got %#v", points)
	}
}

// TestHandleListNormalizesCachedZeroQuote 验证自选股列表不会把未开盘缓存中的 0 价展示成 -100%。
func TestHandleListNormalizesCachedZeroQuote(t *testing.T) {
	createdAt := time.Date(2026, 6, 26, 9, 14, 0, 0, time.UTC)
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "603026.SH",
			SortOrder: 10,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}},
		quotes: map[string]model.Quote{
			"603026.SH": {
				Symbol:        "603026.SH",
				Price:         0,
				ChangeAmount:  -93.78,
				ChangePercent: -100,
				PreClose:      93.78,
				QuoteTime:     createdAt,
				UpdatedAt:     createdAt,
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security:    httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:       store,
		MarketStore: store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	first := response.Data.(map[string]any)["items"].([]any)[0].(map[string]any)
	quote := first["quote"].(map[string]any)
	if quote["price"] != 93.78 || quote["change_amount"] != 0.0 || quote["change_percent"] != 0.0 {
		t.Fatalf("expected normalized cached quote, got %#v", quote)
	}
}

// TestHandleListUsesLatestTradingDayTrend 验证自选股迷你走势只取最新交易日分时，避免旧交易日和当天分时混在一条曲线上。
func TestHandleListUsesLatestTradingDayTrend(t *testing.T) {
	createdAt := time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "600000.SH",
			SortOrder: 10,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}},
		klines: map[string][]model.Kline{
			"600000.SH|minute|none": {
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-24 14:59", Close: 8.88},
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-25 09:30", Close: 9.01},
				{Symbol: "600000.SH", Period: "minute", Adjust: "none", TradeDate: "2026-06-25 09:31", Close: 9.12},
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security:    httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:       store,
		MarketStore: store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	first := response.Data.(map[string]any)["items"].([]any)[0].(map[string]any)
	points := first["trend_points"].([]any)
	if len(points) != 2 || points[0] != 9.01 || points[1] != 9.12 {
		t.Fatalf("expected latest trading day trend points, got %#v", points)
	}
}

// TestHandleRefreshDeduplicatesInFlightSymbols 验证自选股刷新不会对同一 symbol 并发启动多条 Provider 请求。
func TestHandleRefreshDeduplicatesInFlightSymbols(t *testing.T) {
	watchlistRefreshInFlight.Delete("600000.SH")
	t.Cleanup(func() { watchlistRefreshInFlight.Delete("600000.SH") })

	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:     1,
			Symbol: "600000.SH",
		}},
	}
	provider := &blockingMarketProvider{
		quoteStarted: make(chan struct{}, 2),
		releaseQuote: make(chan struct{}),
	}
	config := Config{
		Security:       httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:          store,
		MarketStore:    store,
		MarketProvider: provider,
	}

	first := performWatchlistRefresh(t, config, `{"symbols":["600000.SH"]}`)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first refresh status %d, got %d body=%s", http.StatusOK, first.Code, first.Body.String())
	}
	select {
	case <-provider.quoteStarted:
	case <-time.After(time.Second):
		t.Fatal("expected first provider quote call to start")
	}

	second := performWatchlistRefresh(t, config, `{"symbols":["600000.SH"]}`)
	if second.Code != http.StatusOK {
		t.Fatalf("expected second refresh status %d, got %d body=%s", http.StatusOK, second.Code, second.Body.String())
	}
	select {
	case <-provider.quoteStarted:
		t.Fatal("duplicate refresh should not start a second provider quote while first is in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(provider.releaseQuote)
	waitUntilWatchlistRefreshReleased(t, "600000.SH")

	if got := provider.quoteCalls.Load(); got != 1 {
		t.Fatalf("expected one provider quote call, got %d", got)
	}
}

// waitUntilWatchlistRefreshReleased 等待后台刷新清理 in-flight 标记，避免异步尾巴影响后续用例。
func waitUntilWatchlistRefreshReleased(t *testing.T, symbol string) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, ok := watchlistRefreshInFlight.Load(symbol); !ok {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected refresh in-flight marker for %s to be released", symbol)
		case <-ticker.C:
		}
	}
}

// performWatchlistList 使用固定 token 执行自选股列表 action。
func performWatchlistList(t *testing.T, config Config) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/watchlist/list", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[0].Handler(recorder, request)
	return recorder
}

// performWatchlistRefresh 使用固定 token 执行自选股刷新 action。
func performWatchlistRefresh(t *testing.T, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/watchlist/refresh", bytes.NewReader([]byte(body)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[4].Handler(recorder, request)
	return recorder
}

type fakeWatchlistStore struct {
	items          []model.Watchlist
	stocks         map[string]model.Stock
	quotes         map[string]model.Quote
	klines         map[string][]model.Kline
	profileSymbols []string
}

// SaveWatchlist 满足 action Store 接口；列表测试不应调用写入路径。
func (store *fakeWatchlistStore) SaveWatchlist(context.Context, *model.Watchlist) error {
	return nil
}

// ListActiveWatchlists 返回测试预置的 active 自选股。
func (store *fakeWatchlistStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return store.items, nil
}

// SoftDeleteWatchlist 满足 action Store 接口；列表测试不应调用删除路径。
func (store *fakeWatchlistStore) SoftDeleteWatchlist(context.Context, int64) error {
	return nil
}

// GetStocksBySymbols 返回按 symbol 关联的股票基础资料。
func (store *fakeWatchlistStore) GetStocksBySymbols(_ context.Context, symbols []string) (map[string]model.Stock, error) {
	store.profileSymbols = append([]string(nil), symbols...)
	return store.stocks, nil
}

// LatestQuote 返回测试预置的本地行情快照。
func (store *fakeWatchlistStore) LatestQuote(_ context.Context, symbol string, _ time.Duration) (model.Quote, bool, error) {
	quote, ok := store.quotes[symbol]
	return quote, ok, nil
}

// SaveQuote 满足行情缓存写入接口；列表测试不应调用写入路径。
func (store *fakeWatchlistStore) SaveQuote(context.Context, *model.Quote) error {
	return nil
}

// ListKlines 返回测试预置的本地 K 线缓存。
func (store *fakeWatchlistStore) ListKlines(_ context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error) {
	items := append([]model.Kline(nil), store.klines[symbol+"|"+period+"|"+adjust]...)
	if limit > 0 && len(items) > limit {
		return items[len(items)-limit:], nil
	}
	return items, nil
}

// SaveKlines 满足 K 线缓存写入接口；列表测试不应调用写入路径。
func (store *fakeWatchlistStore) SaveKlines(context.Context, []model.Kline) error {
	return nil
}

type blockingMarketProvider struct {
	quoteCalls   atomic.Int64
	quoteStarted chan struct{}
	releaseQuote chan struct{}
}

// Name 返回测试 Provider 名称。
func (provider *blockingMarketProvider) Name() string {
	return "blocking-market"
}

// Status 返回测试 Provider 可用状态。
func (provider *blockingMarketProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Name: "blocking-market", Available: true}
}

// Search 满足 MarketProvider 接口；刷新测试不应调用搜索。
func (provider *blockingMarketProvider) Search(context.Context, string) ([]marketservice.StockBasic, error) {
	return nil, nil
}

// Quote 阻塞到测试释放，用于观察并发刷新是否被去重。
func (provider *blockingMarketProvider) Quote(ctx context.Context, symbol stockservice.Symbol) (marketservice.Quote, error) {
	provider.quoteCalls.Add(1)
	provider.quoteStarted <- struct{}{}
	select {
	case <-provider.releaseQuote:
	case <-ctx.Done():
		return marketservice.Quote{}, ctx.Err()
	}
	return marketservice.Quote{Symbol: symbol, Price: 9.12, QuoteTime: time.Now(), Provider: provider.Name()}, nil
}

// Kline 返回最小分时数据，满足刷新写入趋势缓存。
func (provider *blockingMarketProvider) Kline(_ context.Context, request marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	return []marketservice.KlineBar{{
		Symbol:    request.Symbol,
		Period:    request.Period,
		Adjust:    request.Adjust,
		TradeDate: "2026-06-26 09:30",
		Close:     9.12,
		Provider:  provider.Name(),
	}}, nil
}
