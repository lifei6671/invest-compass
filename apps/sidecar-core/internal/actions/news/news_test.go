package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
)

// TestItemDataFromServiceAddsSentiment 验证 Provider 新鲜结果会带上本地情绪标签返回前端。
func TestItemDataFromServiceAddsSentiment(t *testing.T) {
	items := itemDataFromService([]newsservice.Item{
		{
			ID:          "news-1",
			Source:      "财联社电报",
			Title:       "光模块订单超预期",
			Summary:     "公司订单增长超预期，行业景气度回升",
			PublishedAt: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
			Tags:        []string{"光模块"},
		},
	}, 10)

	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].Sentiment != "positive" {
		t.Fatalf("expected positive sentiment, got %+v", items[0])
	}
}

// TestItemDataFromModelsAddsSentiment 验证缓存新闻也会在返回时补齐情绪标签。
func TestItemDataFromModelsAddsSentiment(t *testing.T) {
	items := itemDataFromModels([]model.NewsItem{
		{
			Source:      "新浪财经",
			Title:       "汽车板块承压",
			Summary:     "海外需求下滑，公司业绩不及预期",
			ContentHash: "hash-1",
			PublishedAt: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].Sentiment != "negative" {
		t.Fatalf("expected negative sentiment, got %+v", items[0])
	}
}

// TestBuildStatsDataSummarizesSentiment 验证资讯统计按缓存内容输出真实情绪摘要。
func TestBuildStatsDataSummarizesSentiment(t *testing.T) {
	items := []model.NewsItem{
		{Source: "财联社电报", Title: "订单超预期", Summary: "公司订单增长超预期", PublishedAt: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)},
		{Source: "新浪财经", Title: "需求下滑", Summary: "海外需求下滑，业绩不及预期", PublishedAt: time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)},
		{Source: "新浪财经", Title: "常规会议", Summary: "公司召开年度股东大会", PublishedAt: time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)},
	}

	stats := buildStatsData(items)

	if stats.SentimentPositiveCount != 1 || stats.SentimentNegativeCount != 1 || stats.SentimentNeutralCount != 1 {
		t.Fatalf("unexpected sentiment counts: %+v", stats)
	}
	if stats.SentimentSummary != "利好 1 条，中性 1 条，利空 1 条。" {
		t.Fatalf("unexpected sentiment summary: %q", stats.SentimentSummary)
	}
}

// TestMarketFetchIndexesCachedNewsIncrementally 验证市场资讯入库后增量写入 active 搜索索引，不制造完整重建任务。
func TestMarketFetchIndexesCachedNewsIncrementally(t *testing.T) {
	indexed := make(chan model.NewsItem, 1)
	store := &actionNewsStore{}
	handler := handleMarket(Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		NewsProvider: actionNewsProvider{marketItems: []newsservice.Item{{
			Source:      "财联社电报",
			Title:       "光模块订单超预期",
			URL:         "https://example.com/news/1",
			Summary:     "行业景气度回升",
			PublishedAt: time.Date(2026, 6, 28, 9, 0, 0, 0, time.UTC),
		}}},
		Store:         store,
		SearchIndexer: recordingSearchIndexer{indexed: indexed},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/news/market", strings.NewReader(`{"market":"CN","limit":5,"force_refresh":true}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one saved news item, got %+v", store.saved)
	}
	select {
	case item := <-indexed:
		if item.ID == 0 || item.Title != "光模块订单超预期" {
			t.Fatalf("expected saved news item to be indexed, got %+v", item)
		}
	case <-time.After(time.Second):
		t.Fatal("expected saved market news to be indexed incrementally")
	}
}

// TestListFetchIndexesCachedNewsIncrementally 验证个股资讯入库后增量写入 active 搜索索引。
func TestListFetchIndexesCachedNewsIncrementally(t *testing.T) {
	indexed := make(chan model.NewsItem, 1)
	handler := handleList(Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		NewsProvider: actionNewsProvider{listItems: []newsservice.Item{{
			Source:      "新浪财经",
			Title:       "贵州茅台发布经营动态",
			URL:         "https://example.com/news/2",
			Summary:     "经营保持稳定",
			PublishedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
		}}},
		Store:         &actionNewsStore{},
		SearchIndexer: recordingSearchIndexer{indexed: indexed},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/news/list", strings.NewReader(`{"symbol":"CN:SH:600519","limit":5}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	select {
	case item := <-indexed:
		if item.ID == 0 || item.Title != "贵州茅台发布经营动态" {
			t.Fatalf("expected saved stock news to be indexed, got %+v", item)
		}
	case <-time.After(time.Second):
		t.Fatal("expected saved stock news to be indexed incrementally")
	}
}

type actionNewsStore struct {
	saved []model.NewsItem
}

func (store *actionNewsStore) SaveNewsItems(_ context.Context, items []model.NewsItem) error {
	for _, item := range items {
		item.ID = int64(len(store.saved) + 1)
		store.saved = append(store.saved, item)
	}
	return nil
}

func (store *actionNewsStore) ListNewsBySymbol(context.Context, string, int, time.Duration) ([]model.NewsItem, error) {
	return append([]model.NewsItem(nil), store.saved...), nil
}

func (store *actionNewsStore) ListMarketNews(context.Context, string, int, time.Duration) ([]model.NewsItem, error) {
	return append([]model.NewsItem(nil), store.saved...), nil
}

type actionNewsProvider struct {
	listItems   []newsservice.Item
	marketItems []newsservice.Item
}

func (provider actionNewsProvider) Name() string {
	return "action-news-provider"
}

func (provider actionNewsProvider) List(context.Context, newsservice.ListRequest) ([]newsservice.Item, error) {
	return provider.listItems, nil
}

func (provider actionNewsProvider) Market(context.Context, newsservice.MarketRequest) ([]newsservice.Item, error) {
	return provider.marketItems, nil
}

type recordingSearchIndexer struct {
	indexed chan<- model.NewsItem
}

func (indexer recordingSearchIndexer) IndexNews(_ context.Context, item model.NewsItem) error {
	indexer.indexed <- item
	return nil
}
