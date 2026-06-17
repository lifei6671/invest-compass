package news

import (
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/stock"
)

// TestNewCacheValidatesTTL 验证新闻缓存 TTL 必须处于 30-120 分钟。
func TestNewCacheValidatesTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		code Code
	}{
		{name: "too short", ttl: 29 * time.Minute, code: ErrorInvalidCacheTTL},
		{name: "too long", ttl: 121 * time.Minute, code: ErrorInvalidCacheTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCache(tt.ttl)
			assertNewsErrorCode(t, err, tt.code)
		})
	}
}

// TestCacheListsDedupedAndSortedMarketNews 验证市场新闻缓存会去重并按发布时间倒序返回。
func TestCacheListsDedupedAndSortedMarketNews(t *testing.T) {
	cache, err := NewCache(60 * time.Minute)
	if err != nil {
		t.Fatalf("NewCache returned error: %v", err)
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)

	items, err := NormalizeItems([]Item{
		{Source: "feed-a", Title: "同一市场新闻", URL: "https://example.com/a", PublishedAt: older, Tags: []string{"market"}},
		{Source: "feed-b", Title: "同一市场新闻", URL: "https://example.com/b", PublishedAt: older, Tags: []string{"market"}},
		{Source: "feed-c", Title: "更新市场新闻", URL: "https://example.com/c", PublishedAt: now, Tags: []string{"market"}},
	})
	if err != nil {
		t.Fatalf("NormalizeItems returned error: %v", err)
	}

	cache.PutMarket("CN", items, now)
	listed, ok := cache.GetMarket(MarketRequest{Market: "CN", Limit: 10}, now.Add(30*time.Minute))
	if !ok {
		t.Fatal("expected market news cache hit")
	}

	if len(listed) != 2 {
		t.Fatalf("expected 2 deduped market news, got %d", len(listed))
	}
	if listed[0].Title != "更新市场新闻" || listed[1].Title != "同一市场新闻" {
		t.Fatalf("unexpected market news order: %+v", listed)
	}
}

// TestCacheFiltersStockNewsBySymbolAndExpires 验证个股新闻按 symbol 过滤并遵守缓存过期时间。
func TestCacheFiltersStockNewsBySymbolAndExpires(t *testing.T) {
	cache, err := NewCache(30 * time.Minute)
	if err != nil {
		t.Fatalf("NewCache returned error: %v", err)
	}
	aapl := mustParseNewsSymbol(t, "US:AAPL")
	msft := mustParseNewsSymbol(t, "US:MSFT")
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	items, err := NormalizeItems([]Item{
		{Source: "feed-a", Title: "Apple news", URL: "https://example.com/aapl", PublishedAt: now, Symbols: []stock.Symbol{aapl}},
		{Source: "feed-b", Title: "Microsoft news", URL: "https://example.com/msft", PublishedAt: now, Symbols: []stock.Symbol{msft}},
	})
	if err != nil {
		t.Fatalf("NormalizeItems returned error: %v", err)
	}

	cache.PutStock(aapl, items, now)

	listed, ok := cache.GetStock(ListRequest{Symbol: aapl, Limit: 10}, now.Add(29*time.Minute))
	if !ok {
		t.Fatal("expected stock news cache hit")
	}
	if len(listed) != 1 || listed[0].Title != "Apple news" {
		t.Fatalf("unexpected stock news list: %+v", listed)
	}
	if _, ok := cache.GetStock(ListRequest{Symbol: aapl, Limit: 10}, now.Add(31*time.Minute)); ok {
		t.Fatal("expected stock news cache miss after ttl")
	}
}

// TestCacheReturnsCopy 验证调用方不能通过返回切片污染新闻缓存。
func TestCacheReturnsCopy(t *testing.T) {
	cache, err := NewCache(60 * time.Minute)
	if err != nil {
		t.Fatalf("NewCache returned error: %v", err)
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	items, err := NormalizeItems([]Item{
		{Source: "feed-a", Title: "市场新闻", URL: "https://example.com/market", PublishedAt: now},
	})
	if err != nil {
		t.Fatalf("NormalizeItems returned error: %v", err)
	}

	cache.PutMarket("US", items, now)
	listed, ok := cache.GetMarket(MarketRequest{Market: "US", Limit: 10}, now)
	if !ok {
		t.Fatal("expected market news cache hit")
	}
	listed[0].Title = "污染缓存"

	again, ok := cache.GetMarket(MarketRequest{Market: "US", Limit: 10}, now)
	if !ok || again[0].Title != "市场新闻" {
		t.Fatalf("expected cached news copy isolation, got ok=%v items=%+v", ok, again)
	}
}

// assertNewsErrorCode 校验新闻模块错误码稳定。
func assertNewsErrorCode(t *testing.T, err error, code Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	newsError, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected news Error, got %T", err)
	}
	if newsError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, newsError.Code)
	}
}

// mustParseNewsSymbol 解析测试股票代码。
func mustParseNewsSymbol(t *testing.T, raw string) stock.Symbol {
	t.Helper()
	symbol, err := stock.ParseSymbol(raw)
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	return symbol
}
