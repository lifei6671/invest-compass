package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestMarketNewsRefreshRunnerFetchesAndPersistsNews 验证市场新闻 runner 会标准化并写入新闻缓存。
func TestMarketNewsRefreshRunnerFetchesAndPersistsNews(t *testing.T) {
	publishedAt := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	provider := &newsProvider{
		marketItems: []newsservice.Item{{
			Source:      "wallstreetcn",
			Title:       "市场新闻",
			URL:         "https://example.com/news",
			Summary:     "summary",
			PublishedAt: publishedAt,
			Symbols:     []stockservice.Symbol{mustParseSymbol(t, "CN:SH:600519")},
			Tags:        []string{"market"},
		}},
	}
	store := &newsStore{}
	runner := NewsRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeMarketNewsRefresh,
		DataType:   "news",
		ScopeKey:   "CN",
		ParamsJSON: `{"limit":20}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run market news refresh: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected news refresh result: %+v", result)
	}
	if provider.marketRequest.Market != "CN" || provider.marketRequest.Limit != 20 {
		t.Fatalf("unexpected market news request: %+v", provider.marketRequest)
	}
	if len(store.items) != 1 || store.items[0].ContentHash == "" || store.items[0].Symbols == "" {
		t.Fatalf("unexpected saved news items: %+v", store.items)
	}
	if len(store.watermarks) != 1 || store.watermarks[0].DataType != "news" || store.watermarks[0].ScopeKey != "CN" {
		t.Fatalf("unexpected watermarks: %+v", store.watermarks)
	}
}

// TestMarketNewsRefreshRunnerRedactsProviderError 验证市场新闻 Provider 错误不会把认证信息透出到 run 错误。
func TestMarketNewsRefreshRunnerRedactsProviderError(t *testing.T) {
	provider := &newsProvider{
		marketErr: errors.New("Authorization: Bearer sk-news-secret api_key=plain-secret"),
	}
	runner := NewsRefreshRunner{Provider: provider, Store: &newsStore{}}

	_, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeMarketNewsRefresh,
		DataType:   "news",
		ScopeKey:   "CN",
		ParamsJSON: `{"limit":20}`,
		TargetDate: "2026-06-19",
	})
	if err == nil {
		t.Fatal("expected market news provider error")
	}
	message := err.Error()
	if strings.Contains(message, "sk-news-secret") || strings.Contains(message, "plain-secret") {
		t.Fatalf("provider error must be redacted, got %q", message)
	}
	if !strings.Contains(message, "[REDACTED]") {
		t.Fatalf("provider error should keep redaction marker, got %q", message)
	}
}

// TestMarketNewsRefreshRunnerFailsWhenProviderUnconfigured 验证未配置新闻 Provider 时快速失败，不写入假新闻或水位。
func TestMarketNewsRefreshRunnerFailsWhenProviderUnconfigured(t *testing.T) {
	store := &newsStore{}
	runner := NewsRefreshRunner{Provider: newsservice.UnconfiguredProvider{}, Store: store}

	_, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeMarketNewsRefresh,
		DataType:   "news",
		ScopeKey:   "CN",
		ParamsJSON: `{"limit":20}`,
		TargetDate: "2026-06-19",
	})
	if err == nil || !strings.Contains(err.Error(), "news_provider_unconfigured") {
		t.Fatalf("expected unconfigured provider error, got %v", err)
	}
	if len(store.items) != 0 || len(store.watermarks) != 0 {
		t.Fatalf("unconfigured provider must not persist news or watermark, items=%+v watermarks=%+v", store.items, store.watermarks)
	}
}

// TestMarketNewsRefreshRunnerSkipsNonCNMarket 验证首版市场新闻任务不会误抓未支持的非 CN 市场。
func TestMarketNewsRefreshRunnerSkipsNonCNMarket(t *testing.T) {
	provider := &newsProvider{}
	runner := NewsRefreshRunner{Provider: provider, Store: &newsStore{}}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeMarketNewsRefresh,
		DataType:   "news",
		ScopeKey:   "US",
		ParamsJSON: `{"limit":20}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run non CN market news refresh: %v", err)
	}
	if result.SkippedReason != skippedReasonNonCNScope {
		t.Fatalf("expected non_cn_scope skipped reason, got %+v", result)
	}
	if provider.marketRequest.Market != "" {
		t.Fatalf("non CN market must not call provider, got %+v", provider.marketRequest)
	}
}

// TestSymbolNewsRefreshRunnerFetchesBySymbol 验证个股新闻 runner 会按 scope_key 的 symbol 拉取新闻。
func TestSymbolNewsRefreshRunnerFetchesBySymbol(t *testing.T) {
	provider := &newsProvider{
		listItems: []newsservice.Item{{
			Source:      "wallstreetcn",
			Title:       "个股新闻",
			URL:         "https://example.com/symbol-news",
			PublishedAt: time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
		}},
	}
	store := &newsStore{}
	runner := NewsRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeSymbolNewsRefresh,
		DataType:   "news",
		ScopeKey:   "CN:SH:600519",
		ParamsJSON: `{"limit":10}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run symbol news refresh: %v", err)
	}

	if result.FetchedCount != 1 || provider.listRequest.Symbol.String() != "CN:SH:600519" || provider.listRequest.Limit != 10 {
		t.Fatalf("unexpected symbol news result/request: result=%+v request=%+v", result, provider.listRequest)
	}
}

// TestSymbolNewsRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket 验证个股新闻市场范围任务会读取 active watchlist 中的 CN 标的。
func TestSymbolNewsRefreshRunnerReadsActiveWatchlistWhenScopeIsMarket(t *testing.T) {
	provider := &newsProvider{
		listItems: []newsservice.Item{{
			Source:      "wallstreetcn",
			Title:       "自选股新闻",
			URL:         "https://example.com/watchlist-news",
			PublishedAt: time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
		}},
	}
	store := &newsStore{
		watchlists: []model.Watchlist{
			{Symbol: "US:AAPL"},
			{Symbol: "CN:SH:600519"},
		},
	}
	runner := NewsRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeSymbolNewsRefresh,
		DataType:   "news",
		ScopeKey:   "CN",
		ParamsJSON: `{"limit":10}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run symbol news refresh from watchlist: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected watchlist symbol news result: %+v", result)
	}
	if len(provider.listRequests) != 1 || provider.listRequests[0].Symbol.String() != "CN:SH:600519" {
		t.Fatalf("expected only CN watchlist symbol request, got %+v", provider.listRequests)
	}
}

// TestSymbolNewsRefreshRunnerRejectsTooManySymbols 验证个股新闻任务会拒绝超过硬上限的 symbol 列表。
func TestSymbolNewsRefreshRunnerRejectsTooManySymbols(t *testing.T) {
	symbols := make([]string, 0, 31)
	for index := 0; index < 31; index++ {
		symbols = append(symbols, fmt.Sprintf("CN:SH:%06d", 600000+index))
	}
	provider := &newsProvider{}
	runner := NewsRefreshRunner{Provider: provider, Store: &newsStore{}}

	_, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeSymbolNewsRefresh,
		DataType:   "news",
		ScopeKey:   strings.Join(symbols, ","),
		ParamsJSON: `{"limit":10}`,
		TargetDate: "2026-06-19",
	})
	if err == nil || !strings.Contains(err.Error(), "symbol news scope exceeds 30 symbols") {
		t.Fatalf("expected symbol limit error, got %v", err)
	}
	if len(provider.listRequests) != 0 {
		t.Fatalf("symbol limit error must happen before provider calls, got %+v", provider.listRequests)
	}
}

// TestSymbolNewsRefreshRunnerSkipsNonCNSymbol 验证个股新闻任务遇到非 CN 标的时跳过而不调用 Provider。
func TestSymbolNewsRefreshRunnerSkipsNonCNSymbol(t *testing.T) {
	provider := &newsProvider{}
	runner := NewsRefreshRunner{Provider: provider, Store: &newsStore{}}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeSymbolNewsRefresh,
		DataType:   "news",
		ScopeKey:   "US:AAPL",
		ParamsJSON: `{"limit":10}`,
		TargetDate: "2026-06-19",
	})
	if err != nil {
		t.Fatalf("run non CN symbol news refresh: %v", err)
	}
	if result.SkippedReason != "non_cn_scope" {
		t.Fatalf("expected non_cn_scope skipped reason, got %+v", result)
	}
	if len(provider.listRequests) != 0 {
		t.Fatalf("non CN symbol must not call provider, got %+v", provider.listRequests)
	}
}

type newsProvider struct {
	marketRequest newsservice.MarketRequest
	listRequest   newsservice.ListRequest
	listRequests  []newsservice.ListRequest
	marketItems   []newsservice.Item
	listItems     []newsservice.Item
	marketErr     error
	listErr       error
}

// Name 返回测试新闻 Provider 名称。
func (provider *newsProvider) Name() string {
	return "test-news-provider"
}

// Market 记录市场新闻请求并返回预置新闻。
func (provider *newsProvider) Market(_ context.Context, request newsservice.MarketRequest) ([]newsservice.Item, error) {
	provider.marketRequest = request
	if provider.marketErr != nil {
		return nil, provider.marketErr
	}
	return provider.marketItems, nil
}

// List 记录个股新闻请求并返回预置新闻。
func (provider *newsProvider) List(_ context.Context, request newsservice.ListRequest) ([]newsservice.Item, error) {
	provider.listRequest = request
	provider.listRequests = append(provider.listRequests, request)
	if provider.listErr != nil {
		return nil, provider.listErr
	}
	return provider.listItems, nil
}

type newsStore struct {
	items      []model.NewsItem
	watermarks []model.IngestionWatermark
	watchlists []model.Watchlist
}

// SaveNewsItems 记录测试保存的新闻。
func (store *newsStore) SaveNewsItems(_ context.Context, items []model.NewsItem) error {
	store.items = append(store.items, items...)
	return nil
}

// UpsertIngestionWatermark 记录测试更新的抓取水位。
func (store *newsStore) UpsertIngestionWatermark(_ context.Context, watermark *model.IngestionWatermark) error {
	store.watermarks = append(store.watermarks, *watermark)
	return nil
}

// ListActiveWatchlists 返回测试预置的 active watchlist。
func (store *newsStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return append([]model.Watchlist(nil), store.watchlists...), nil
}
