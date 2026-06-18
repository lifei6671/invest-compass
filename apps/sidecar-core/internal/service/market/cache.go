package market

import (
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	minQuoteCacheTTL = 10 * time.Second
	maxQuoteCacheTTL = 60 * time.Second
)

// QuoteCache 是行情短缓存，只缓存最近一次 quote 快照。
type QuoteCache struct {
	ttl     time.Duration
	entries map[string]quoteCacheEntry
}

type quoteCacheEntry struct {
	quote     Quote
	expiresAt time.Time
}

// NewQuoteCache 创建行情短缓存，TTL 必须处于 10-60 秒。
func NewQuoteCache(ttl time.Duration) (*QuoteCache, error) {
	if ttl < minQuoteCacheTTL || ttl > maxQuoteCacheTTL {
		return nil, &xerr.Error{Code: xerr.MarketInvalidQuoteCacheTTL}
	}
	return &QuoteCache{
		ttl:     ttl,
		entries: make(map[string]quoteCacheEntry),
	}, nil
}

// Put 写入 quote 快照，后续同一 symbol 在 TTL 内直接命中。
func (cache *QuoteCache) Put(quote Quote, now time.Time) {
	cache.entries[quote.Symbol.String()] = quoteCacheEntry{
		quote:     quote,
		expiresAt: now.Add(cache.ttl),
	}
}

// Get 读取 quote 缓存，过期时返回 miss。
func (cache *QuoteCache) Get(symbol stock.Symbol, now time.Time) (Quote, bool) {
	entry, ok := cache.entries[symbol.String()]
	if !ok || now.After(entry.expiresAt) {
		return Quote{}, false
	}
	return entry.quote, true
}

// KlineCache 是按 symbol、period、adjust 隔离的 K 线内存缓存。
type KlineCache struct {
	entries map[klineCacheKey][]KlineBar
}

type klineCacheKey struct {
	symbol string
	period Period
	adjust Adjust
}

// NewKlineCache 创建 K 线缓存。
func NewKlineCache() *KlineCache {
	return &KlineCache{
		entries: make(map[klineCacheKey][]KlineBar),
	}
}

// Put 写入 K 线缓存，并按交易日升序保存。
func (cache *KlineCache) Put(request KlineRequest, bars []KlineBar) {
	copied := append([]KlineBar(nil), bars...)
	sort.SliceStable(copied, func(left int, right int) bool {
		return copied[left].TradeDate < copied[right].TradeDate
	})
	cache.entries[newKlineCacheKey(request)] = copied
}

// Get 读取 K 线缓存，返回副本防止调用方污染缓存。
func (cache *KlineCache) Get(request KlineRequest) ([]KlineBar, bool) {
	bars, ok := cache.entries[newKlineCacheKey(request)]
	if !ok {
		return nil, false
	}
	return append([]KlineBar(nil), bars...), true
}

// newKlineCacheKey 生成 K 线缓存键，保证 period 和 adjust 不串用。
func newKlineCacheKey(request KlineRequest) klineCacheKey {
	return klineCacheKey{
		symbol: request.Symbol.String(),
		period: request.Period,
		adjust: request.Adjust,
	}
}
