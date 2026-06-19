package news

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestNormalizeItemAcceptsHTTPSAndStandardSymbols 验证新闻条目只接受安全 URL 和标准股票代码。
func TestNormalizeItemAcceptsHTTPSAndStandardSymbols(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	item := Item{
		Source:      "official-feed",
		Title:       "贵州茅台发布经营信息",
		URL:         "HTTPS://example.com/news?id=1",
		Summary:     "公司披露最新经营信息",
		PublishedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
		Symbols:     []stock.Symbol{symbol},
		Tags:        []string{"company"},
	}

	normalized, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem returned error: %v", err)
	}

	if normalized.ID == "" || normalized.ContentHash == "" {
		t.Fatalf("expected generated id and content hash: %+v", normalized)
	}
	if normalized.URL != "https://example.com/news?id=1" {
		t.Fatalf("expected normalized URL, got %q", normalized.URL)
	}
	if normalized.Symbols[0].String() != "CN:SH:600519" {
		t.Fatalf("unexpected symbols: %+v", normalized.Symbols)
	}
}

// TestNormalizeItemRejectsUnsafeURL 验证新闻 URL 禁止 javascript、file 等非 HTTP(S) scheme。
func TestNormalizeItemRejectsUnsafeURL(t *testing.T) {
	_, err := NormalizeItem(Item{
		Source:      "bad-feed",
		Title:       "恶意链接",
		URL:         "javascript:alert(1)",
		PublishedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var newsError *xerr.Error
	if !errors.As(err, &newsError) {
		t.Fatalf("expected news Error, got %T", err)
	}
	if newsError.Code != xerr.NewsUnsafeURL {
		t.Fatalf("expected error code %q, got %q", xerr.NewsUnsafeURL, newsError.Code)
	}
}

// TestDeduplicateKeepsFirstItemByContentHash 验证新闻去重按内容 hash 保留首个条目。
func TestDeduplicateKeepsFirstItemByContentHash(t *testing.T) {
	publishedAt := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	items := []Item{
		{Source: "feed-a", Title: "同一新闻", URL: "https://example.com/a", PublishedAt: publishedAt},
		{Source: "feed-b", Title: "同一新闻", URL: "https://example.com/b", PublishedAt: publishedAt},
		{Source: "feed-c", Title: "另一新闻", URL: "https://example.com/c", PublishedAt: publishedAt},
	}

	normalized, err := NormalizeItems(items)
	if err != nil {
		t.Fatalf("NormalizeItems returned error: %v", err)
	}
	deduped := Deduplicate(normalized)

	if len(deduped) != 2 {
		t.Fatalf("expected 2 items after deduplicate, got %d", len(deduped))
	}
	if deduped[0].Source != "feed-a" || deduped[1].Source != "feed-c" {
		t.Fatalf("unexpected deduplicate order: %+v", deduped)
	}
}

// TestNewsProviderContractSupportsStockAndMarketNews 验证 Provider 契约同时支持个股新闻和市场新闻。
func TestNewsProviderContractSupportsStockAndMarketNews(t *testing.T) {
	provider := fakeProvider{}
	ctx := context.Background()
	symbol, err := stock.ParseSymbol("US:AAPL")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}

	stockNews, err := provider.List(ctx, ListRequest{Symbol: symbol, Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	marketNews, err := provider.Market(ctx, MarketRequest{Market: "US", Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}

	if len(stockNews) != 1 || stockNews[0].Symbols[0].String() != "US:AAPL" {
		t.Fatalf("unexpected stock news: %+v", stockNews)
	}
	if len(marketNews) != 1 || marketNews[0].Source != "fake-news" {
		t.Fatalf("unexpected market news: %+v", marketNews)
	}
}

// TestProviderStatusFromProviderUsesOptionalStatus 验证新闻 Provider 可显式报告不可用状态。
func TestProviderStatusFromProviderUsesOptionalStatus(t *testing.T) {
	status := ProviderStatusFromProvider(context.Background(), fakeProvider{})

	if status.Name != "fake-news" || status.Source != "licensed-feed" || status.Available {
		t.Fatalf("unexpected provider status: %+v", status)
	}
}

// TestProviderStatusFromProviderReturnsUnconfigured 验证未配置新闻 Provider 时状态接口不会伪造可用。
func TestProviderStatusFromProviderReturnsUnconfigured(t *testing.T) {
	status := ProviderStatusFromProvider(context.Background(), nil)

	if status.Source != "unconfigured" || status.Available {
		t.Fatalf("unexpected unconfigured status: %+v", status)
	}
}

// TestProviderErrorRedactsSensitiveRequest 验证新闻 Provider 错误不会泄露授权头或密钥。
func TestProviderErrorRedactsSensitiveRequest(t *testing.T) {
	err := NewProviderError(
		"fake-news",
		"list",
		errors.New("remote failed Authorization: Bearer sk-news-secret api_key=raw-news-secret"),
	)

	message := err.Error()
	if strings.Contains(message, "sk-news-secret") || strings.Contains(message, "raw-news-secret") {
		t.Fatalf("provider error leaked secret: %s", message)
	}
	if !strings.Contains(message, "fake-news") || !strings.Contains(message, "list") {
		t.Fatalf("provider error should keep observable context: %s", message)
	}
}

type fakeProvider struct{}

// Name 返回测试新闻 Provider 名称。
func (fakeProvider) Name() string {
	return "fake-news"
}

// Status 返回固定新闻 Provider 状态，用于验证可选状态能力。
func (fakeProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{Name: "fake-news", Source: "licensed-feed", Available: false}
}

// List 返回固定个股新闻，用于验证 Provider 契约。
func (fakeProvider) List(context.Context, ListRequest) ([]Item, error) {
	symbol, err := stock.ParseSymbol("US:AAPL")
	if err != nil {
		return nil, err
	}
	return []Item{{
		Source:      "fake-news",
		Title:       "Apple releases product update",
		URL:         "https://example.com/aapl",
		PublishedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
		Symbols:     []stock.Symbol{symbol},
	}}, nil
}

// Market 返回固定市场新闻，用于验证 Provider 契约。
func (fakeProvider) Market(context.Context, MarketRequest) ([]Item, error) {
	return []Item{{
		Source:      "fake-news",
		Title:       "US market summary",
		URL:         "https://example.com/market",
		PublishedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
		Tags:        []string{"market"},
	}}, nil
}
