package news

import (
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	minCacheTTL = 30 * time.Minute
	maxCacheTTL = 120 * time.Minute
)

// Cache 是新闻内存缓存，分别保存个股新闻和市场新闻。
type Cache struct {
	ttl           time.Duration
	stockEntries  map[string]cacheEntry
	marketEntries map[string]cacheEntry
}

type cacheEntry struct {
	items     []Item
	expiresAt time.Time
}

// NewCache 创建新闻缓存，TTL 必须处于 30-120 分钟。
func NewCache(ttl time.Duration) (*Cache, error) {
	if ttl < minCacheTTL || ttl > maxCacheTTL {
		return nil, &xerr.Error{Code: xerr.NewsInvalidCacheTTL}
	}
	return &Cache{
		ttl:           ttl,
		stockEntries:  make(map[string]cacheEntry),
		marketEntries: make(map[string]cacheEntry),
	}, nil
}

// PutStock 写入个股新闻缓存，并只保留绑定该 symbol 的新闻。
func (cache *Cache) PutStock(symbol stock.Symbol, items []Item, now time.Time) {
	cache.stockEntries[symbol.String()] = cacheEntry{
		items:     normalizeCachedItems(filterBySymbol(items, symbol)),
		expiresAt: now.Add(cache.ttl),
	}
}

// GetStock 读取个股新闻缓存，过期时返回 miss。
func (cache *Cache) GetStock(request ListRequest, now time.Time) ([]Item, bool) {
	entry, ok := cache.stockEntries[request.Symbol.String()]
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	return limitItems(copyItems(entry.items), request.Limit), true
}

// PutMarket 写入市场新闻缓存。
func (cache *Cache) PutMarket(market string, items []Item, now time.Time) {
	cache.marketEntries[normalizeMarket(market)] = cacheEntry{
		items:     normalizeCachedItems(items),
		expiresAt: now.Add(cache.ttl),
	}
}

// GetMarket 读取市场新闻缓存，过期时返回 miss。
func (cache *Cache) GetMarket(request MarketRequest, now time.Time) ([]Item, bool) {
	entry, ok := cache.marketEntries[normalizeMarket(request.Market)]
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	return limitItems(copyItems(entry.items), request.Limit), true
}

// normalizeCachedItems 去重并按发布时间倒序整理新闻。
func normalizeCachedItems(items []Item) []Item {
	deduped := Deduplicate(items)
	sort.SliceStable(deduped, func(left int, right int) bool {
		return deduped[left].PublishedAt.After(deduped[right].PublishedAt)
	})
	return copyItems(deduped)
}

// filterBySymbol 只保留绑定目标 symbol 的新闻。
func filterBySymbol(items []Item, target stock.Symbol) []Item {
	filtered := make([]Item, 0, len(items))
	for _, item := range items {
		for _, symbol := range item.Symbols {
			if symbol.String() == target.String() {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

// limitItems 按请求 limit 截断新闻列表，limit 小于等于 0 时不截断。
func limitItems(items []Item, limit int) []Item {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

// copyItems 复制新闻列表，避免调用方污染缓存内容。
func copyItems(items []Item) []Item {
	copied := make([]Item, 0, len(items))
	for _, item := range items {
		item.Symbols = append([]stock.Symbol(nil), item.Symbols...)
		item.Tags = append([]string(nil), item.Tags...)
		copied = append(copied, item)
	}
	return copied
}

// normalizeMarket 统一市场缓存键。
func normalizeMarket(market string) string {
	return strings.ToUpper(strings.TrimSpace(market))
}
