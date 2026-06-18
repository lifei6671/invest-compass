package market

import (
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestNewQuoteCacheValidatesShortTTL 验证行情短缓存只允许 10-60 秒。
func TestNewQuoteCacheValidatesShortTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		code xerr.Code
	}{
		{name: "too short", ttl: 9 * time.Second, code: xerr.MarketInvalidQuoteCacheTTL},
		{name: "too long", ttl: 61 * time.Second, code: xerr.MarketInvalidQuoteCacheTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewQuoteCache(tt.ttl)
			assertMarketErrorCode(t, err, tt.code)
		})
	}
}

// TestQuoteCacheHitsWithinTTLAndMissesAfterTTL 验证行情短缓存命中窗口。
func TestQuoteCacheHitsWithinTTLAndMissesAfterTTL(t *testing.T) {
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")
	cache, err := NewQuoteCache(30 * time.Second)
	if err != nil {
		t.Fatalf("NewQuoteCache returned error: %v", err)
	}
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	quote := Quote{
		Symbol:        symbol,
		Price:         100,
		ChangePercent: 1.2,
		QuoteTime:     now,
		Provider:      "fake-provider",
	}

	cache.Put(quote, now)

	if cached, ok := cache.Get(symbol, now.Add(29*time.Second)); !ok || cached.Price != 100 {
		t.Fatalf("expected quote cache hit, got ok=%v quote=%+v", ok, cached)
	}
	if _, ok := cache.Get(symbol, now.Add(31*time.Second)); ok {
		t.Fatal("expected quote cache miss after ttl")
	}
}

// TestKlineCacheStoresBySymbolPeriodAdjustAndSorts 验证 K 线按 symbol、period、adjust 隔离并按交易日排序。
func TestKlineCacheStoresBySymbolPeriodAdjustAndSorts(t *testing.T) {
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")
	cache := NewKlineCache()
	bars := []KlineBar{
		{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 102},
		{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-17", Close: 100},
	}

	cache.Put(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward}, bars)

	cached, ok := cache.Get(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward})
	if !ok {
		t.Fatal("expected kline cache hit")
	}
	if len(cached) != 2 || cached[0].TradeDate != "2026-06-17" || cached[1].TradeDate != "2026-06-18" {
		t.Fatalf("expected sorted kline bars, got %+v", cached)
	}
	if _, ok := cache.Get(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustNone}); ok {
		t.Fatal("expected different adjust to miss cache")
	}
}

// TestKlineCacheReturnsCopy 验证调用方不能通过返回切片污染缓存数据。
func TestKlineCacheReturnsCopy(t *testing.T) {
	symbol := mustParseMarketSymbol(t, "US:AAPL")
	cache := NewKlineCache()
	cache.Put(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustNone}, []KlineBar{
		{Symbol: symbol, Period: PeriodDay, Adjust: AdjustNone, TradeDate: "2026-06-17", Close: 100},
	})

	cached, ok := cache.Get(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustNone})
	if !ok {
		t.Fatal("expected kline cache hit")
	}
	cached[0].Close = 0

	again, ok := cache.Get(KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustNone})
	if !ok || again[0].Close != 100 {
		t.Fatalf("expected cached kline copy isolation, got ok=%v bars=%+v", ok, again)
	}
}

// assertMarketErrorCode 校验行情模块错误码稳定。
func assertMarketErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var marketError *xerr.Error
	if !errors.As(err, &marketError) {
		t.Fatalf("expected market Error, got %T", err)
	}
	if marketError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, marketError.Code)
	}
}

// mustParseMarketSymbol 解析测试股票代码。
func mustParseMarketSymbol(t *testing.T, raw string) stock.Symbol {
	t.Helper()
	symbol, err := stock.ParseSymbol(raw)
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	return symbol
}
