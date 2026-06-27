package news

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestCailianpressProviderFetchesTelegraphs 验证财联社快讯会被清洗成统一新闻条目。
func TestCailianpressProviderFetchesTelegraphs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/cache" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if !strings.Contains(request.Header.Get("Referer"), "cls.cn") {
			t.Fatalf("expected cls referer, got %q", request.Header.Get("Referer"))
		}
		if request.URL.Query().Get("name") != "telegraphList" {
			t.Fatalf("expected telegraphList endpoint, got query %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"errno": 0,
			"data": {
				"roll_data": [
					{
						"id": 123,
						"ctime": 1781805600,
						"title": "重大事项",
						"brief": "A股市场出现重要消息",
						"shareurl": "",
						"level": "B",
						"subjects": [{"subject_name": "半导体"}, {"subject_name": "AI"}]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestCailianpressProvider(t, CailianpressConfig{TelegraphURL: server.URL + "/api/cache"})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	item := items[0]
	if item.Source != "财联社电报" || item.Title != "重大事项" || item.URL != "https://www.cls.cn/detail/123" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.Summary != "A股市场出现重要消息" {
		t.Fatalf("unexpected summary: %s", item.Summary)
	}
	if item.PublishedAt.Format(time.RFC3339) != "2026-06-18T18:00:00Z" {
		t.Fatalf("unexpected published time: %s", item.PublishedAt.Format(time.RFC3339))
	}
	if len(item.Tags) != 2 || item.Tags[0] != "半导体" || item.Tags[1] != "AI" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
	}
}

// TestCailianpressProviderPrefersShareURL 验证财联社返回原文链接时优先使用 shareurl。
func TestCailianpressProviderPrefersShareURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"errno": 0,
			"data": {
				"roll_data": [
					{"id": 456, "ctime": 1781805600, "title": "原文链接", "brief": "带 shareurl", "shareurl": "https://www.cls.cn/detail/456"}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestCailianpressProvider(t, CailianpressConfig{TelegraphURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 || items[0].URL != "https://www.cls.cn/detail/456" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

// TestCailianpressProviderInjectsCredentialCookie 验证财联社 Provider 会把本地凭据 Cookie 注入真实请求。
func TestCailianpressProviderInjectsCredentialCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != "uid=real-user; token=real-token" {
			t.Fatalf("unexpected cookie: %q", request.Header.Get("Cookie"))
		}
		_, _ = writer.Write([]byte(`{"errno":0,"data":{"roll_data":[]}}`))
	}))
	defer server.Close()

	provider := newTestCailianpressProvider(t, CailianpressConfig{
		TelegraphURL:       server.URL,
		CredentialResolver: staticCookieResolver("uid=real-user; token=real-token"),
	})

	if _, err := provider.Market(context.Background(), MarketRequest{Limit: 10}); err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
}

// TestCailianpressProviderStatusRequiresCredential 验证启用凭据 resolver 后未配置 Cookie 会显式不可用。
func TestCailianpressProviderStatusRequiresCredential(t *testing.T) {
	provider := newTestCailianpressProvider(t, CailianpressConfig{
		CredentialResolver: failingCookieResolver{},
	})

	status := provider.Status(context.Background())
	if status.Available || status.LastError != "data_source_credential_not_configured" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

// TestMarketNewsProvidersRejectStockNewsList 验证只支持市场快讯的 Provider 不会把个股新闻能力伪装成空成功。
func TestMarketNewsProvidersRejectStockNewsList(t *testing.T) {
	providers := []Provider{
		newTestCailianpressProvider(t, CailianpressConfig{}),
		newTestSinaLiveProvider(t, SinaLiveConfig{}),
		newTestWallstreetcnLiveProvider(t, WallstreetcnLiveConfig{}),
	}

	for _, provider := range providers {
		t.Run(provider.Name(), func(t *testing.T) {
			items, err := provider.List(context.Background(), ListRequest{Limit: 10})
			if err == nil {
				t.Fatalf("expected unsupported list error, got items=%+v", items)
			}
			var providerError *ProviderError
			if !errors.As(err, &providerError) || providerError.Operation != "list_unsupported" {
				t.Fatalf("expected list_unsupported ProviderError, got %T %[1]v", err)
			}
		})
	}
}

// TestCailianpressProviderFallsBackWhenTimestampMissing 验证财联社缺失发布时间时不会输出 1970 时间污染排序和缓存。
func TestCailianpressProviderFallsBackWhenTimestampMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"errno": 0,
			"data": {
				"roll_data": [
					{"id": 123, "ctime": 0, "title": "缺失时间", "content": "发布时间缺失", "shareurl": ""}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestCailianpressProvider(t, CailianpressConfig{TelegraphURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].PublishedAt.IsZero() || items[0].PublishedAt.Year() <= 1970 {
		t.Fatalf("expected fetched-time fallback, got %s", items[0].PublishedAt.Format(time.RFC3339))
	}
}

// TestSinaLiveProviderFetchesFeed 验证新浪财经直播 JSONP 会被清洗成统一新闻条目。
func TestSinaLiveProviderFetchesFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/zhibo/feed" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`try{callback({
			"result": {
				"data": {
					"feed": {
						"list": [
							{
								"rich_text": "【市场】指数午后拉升",
								"create_time": "2026-06-18 14:30:00",
								"docurl": "https://finance.sina.cn/7x24/2026-06-18/detail-test.d.html",
								"tag": [{"name": "焦点"}, {"name": "A股"}]
							}
						]
					}
				}
			}
		});}catch(e){};`))
	}))
	defer server.Close()

	provider := newTestSinaLiveProvider(t, SinaLiveConfig{FeedURL: server.URL + "/api/zhibo/feed"})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	item := items[0]
	if item.Source != "新浪财经" || item.Title != "市场" || item.Summary != "【市场】指数午后拉升" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.URL != "https://finance.sina.cn/7x24/2026-06-18/detail-test.d.html" {
		t.Fatalf("unexpected url: %s", item.URL)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "焦点" || item.Tags[1] != "A股" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
	}
}

// TestSinaLiveProviderUsesExtDocURLWhenTopLevelURLMissing 验证新浪直播缺少顶层 docurl 时会从 ext 中读取原文链接。
func TestSinaLiveProviderUsesExtDocURLWhenTopLevelURLMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`try{callback({
			"result": {
				"data": {
					"feed": {
						"list": [
							{
								"rich_text": "【市场】原文来自 ext",
								"create_time": "2026-06-18 14:30:00",
								"ext": "{\"docurl\":\"https:\\/\\/finance.sina.com.cn\\/7x24\\/2026-06-18\\/doc-test.shtml\"}",
								"tag": []
							}
						]
					}
				}
			}
		});}catch(e){};`))
	}))
	defer server.Close()

	provider := newTestSinaLiveProvider(t, SinaLiveConfig{FeedURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].URL != "https://finance.sina.com.cn/7x24/2026-06-18/doc-test.shtml" {
		t.Fatalf("unexpected ext url: %s", items[0].URL)
	}
}

// TestSinaLiveProviderParsesCreateTimeInChinaTimezone 验证新浪直播时间不受本机时区影响。
func TestSinaLiveProviderParsesCreateTimeInChinaTimezone(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`try{callback({
			"result": {
				"data": {
					"feed": {
						"list": [
							{"rich_text": "【市场】午后拉升", "create_time": "2026-06-18 14:30:00", "tag": []}
						]
					}
				}
			}
		});}catch(e){};`))
	}))
	defer server.Close()

	provider := newTestSinaLiveProvider(t, SinaLiveConfig{FeedURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].PublishedAt.Format(time.RFC3339) != "2026-06-18T06:30:00Z" {
		t.Fatalf("expected China timezone instant, got %s", items[0].PublishedAt.Format(time.RFC3339))
	}
}

// TestSinaLiveProviderFallsBackWhenCreateTimeInvalid 验证新浪直播时间解析失败时不会输出零值时间。
func TestSinaLiveProviderFallsBackWhenCreateTimeInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`try{callback({
			"result": {
				"data": {
					"feed": {
						"list": [
							{"rich_text": "【市场】发布时间格式异常", "create_time": "bad-time", "tag": []}
						]
					}
				}
			}
		});}catch(e){};`))
	}))
	defer server.Close()

	provider := newTestSinaLiveProvider(t, SinaLiveConfig{FeedURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].PublishedAt.IsZero() || items[0].PublishedAt.Year() <= 1970 {
		t.Fatalf("expected fetched-time fallback, got %s", items[0].PublishedAt.Format(time.RFC3339))
	}
}

// TestWallstreetcnLiveProviderFetchesLives 验证华尔街见闻快讯会被清洗成统一市场新闻。
func TestWallstreetcnLiveProviderFetchesLives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/apiv1/content/lives" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("channel") != "global-channel" || query.Get("limit") != "5" || query.Get("first_page") != "true" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		if !strings.Contains(request.Header.Get("Referer"), "wallstreetcn.com") {
			t.Fatalf("expected wallstreetcn referer, got %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"items": [
					{
						"id": 9988,
						"title": "美联储官员讲话",
						"content": "<p><strong>美联储</strong>称将继续关注通胀。</p>",
						"content_text": "",
						"display_time": 1781805600,
						"uri": "https://wallstreetcn.com/livenews/9988",
						"channels": ["global-channel", "forex-channel"],
						"score": 2
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnLiveProvider(t, WallstreetcnLiveConfig{LivesURL: server.URL + "/apiv1/content/lives"})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 5})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	item := items[0]
	if item.Source != "华尔街见闻-全球7x24" || item.Title != "美联储官员讲话" || item.Summary != "美联储称将继续关注通胀。" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.URL != "https://wallstreetcn.com/livenews/9988" {
		t.Fatalf("unexpected url: %s", item.URL)
	}
	if item.PublishedAt.Format(time.RFC3339) != "2026-06-18T18:00:00Z" {
		t.Fatalf("unexpected published time: %s", item.PublishedAt.Format(time.RFC3339))
	}
	if len(item.Tags) != 2 || item.Tags[0] != "全球7x24" || item.Tags[1] != "外汇" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
	}
}

// TestWallstreetcnLiveProviderFallsBackWhenDisplayTimeMissing 验证华尔街见闻缺失发布时间时不会输出 1970 时间。
func TestWallstreetcnLiveProviderFallsBackWhenDisplayTimeMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"items": [
					{
						"id": 9988,
						"title": "缺失展示时间",
						"content_text": "缺失展示时间",
						"display_time": 0,
						"uri": "/livenews/9988",
						"channels": ["global-channel"]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnLiveProvider(t, WallstreetcnLiveConfig{LivesURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].PublishedAt.IsZero() || items[0].PublishedAt.Year() <= 1970 {
		t.Fatalf("expected fetched-time fallback, got %s", items[0].PublishedAt.Format(time.RFC3339))
	}
}

// TestWallstreetcnLiveProviderDoesNotMislabelUnknownChannel 验证未知远端频道不会被误标成全球频道。
func TestWallstreetcnLiveProviderDoesNotMislabelUnknownChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"items": [
					{
						"id": 9988,
						"title": "未知频道消息",
						"content_text": "未知频道消息",
						"display_time": 1781805600,
						"uri": "/livenews/9988",
						"channels": ["new-remote-channel", "forex-channel"]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnLiveProvider(t, WallstreetcnLiveConfig{LivesURL: server.URL})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if len(items[0].Tags) != 1 || items[0].Tags[0] != "外汇" {
		t.Fatalf("unexpected tags: %+v", items[0].Tags)
	}
}

// TestCompositeProviderAggregatesMarketNews 验证市场资讯会聚合多个来源并按发布时间倒序截断。
func TestCompositeProviderAggregatesMarketNews(t *testing.T) {
	newer := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	provider := NewCompositeProvider("multi-news", []Provider{
		staticNewsProvider{name: "source-a", marketItems: []Item{{Source: "A", Title: "较旧新闻", URL: "https://example.com/a", Summary: "旧", PublishedAt: older}}},
		staticNewsProvider{name: "source-b", marketItems: []Item{{Source: "B", Title: "较新新闻", URL: "https://example.com/b", Summary: "新", PublishedAt: newer}}},
	})

	items, err := provider.Market(context.Background(), MarketRequest{Market: "CN", Limit: 2})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 2 || items[0].Title != "较新新闻" || items[1].Title != "较旧新闻" {
		t.Fatalf("unexpected aggregated order: %+v", items)
	}
	status := provider.Status(context.Background())
	if !status.Available || !strings.Contains(status.Source, "source-a") || !strings.Contains(status.Source, "source-b") {
		t.Fatalf("unexpected composite status: %+v", status)
	}
}

// TestCompositeProviderFetchesSourcesConcurrently 验证多个资讯源并发抓取，避免串行超出桌面 API 超时。
func TestCompositeProviderFetchesSourcesConcurrently(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	provider := NewCompositeProvider("multi-news", []Provider{
		staticNewsProvider{name: "source-a", delay: 80 * time.Millisecond, marketItems: []Item{{Source: "A", Title: "来源 A", URL: "https://example.com/a", Summary: "A", PublishedAt: now}}},
		staticNewsProvider{name: "source-b", delay: 80 * time.Millisecond, marketItems: []Item{{Source: "B", Title: "来源 B", URL: "https://example.com/b", Summary: "B", PublishedAt: now.Add(time.Second)}}},
	})

	startedAt := time.Now()
	items, err := provider.Market(context.Background(), MarketRequest{Market: "CN", Limit: 2})
	elapsed := time.Since(startedAt)

	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two aggregated items, got %+v", items)
	}
	if elapsed >= 140*time.Millisecond {
		t.Fatalf("expected concurrent fetch under 140ms, got %s", elapsed)
	}
}

// TestCompositeProviderKeepsEmptySuccessfulList 验证个股新闻聚合中，
// 某个来源正常返回空列表时不应被其它“不支持个股”来源拖成错误。
func TestCompositeProviderKeepsEmptySuccessfulList(t *testing.T) {
	symbol, err := stockservice.ParseSymbol("CN:SH:600522")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	provider := NewCompositeProvider("multi-news", []Provider{
		staticNewsProvider{name: "market-only", listError: unsupportedStockNewsError("market-only")},
		staticNewsProvider{name: "eastmoney-empty", listItems: []Item{}},
	})

	items, err := provider.List(context.Background(), ListRequest{Symbol: symbol, Limit: 10})
	if err != nil {
		t.Fatalf("empty successful list must not be treated as provider failure: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty stock news list, got %+v", items)
	}
}

// TestTradingViewProviderFetchesChineseNews 验证 go-stock 中的 TradingView 中文新闻源可清洗成统一资讯模型。
func TestTradingViewProviderFetchesChineseNews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/news-flow/v2/news":
			if request.URL.Query()["filter"][0] != "lang:zh-Hans" {
				t.Fatalf("unexpected news filter: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{
				"items": [
					{
						"id": "panews:abc:0",
						"title": "外媒市场新闻",
						"published": 1781805600,
						"provider": {"name": "PANews"}
					}
				]
			}`))
		case "/v3/story":
			if request.URL.Query().Get("lang") != "zh-Hans" {
				t.Fatalf("unexpected detail query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"shortDescription":"TradingView 新闻摘要","tags":[{"title":"加密货币"},{"title":"全球市场"}]}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestTradingViewProvider(t, TradingViewConfig{
		NewsURL:   server.URL + "/news-flow/v2/news",
		DetailURL: server.URL + "/v3/story",
	})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	item := items[0]
	if item.Source != "TradingView-PANews" || item.Title != "外媒市场新闻" || item.Summary != "TradingView 新闻摘要" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.URL != "https://cn.tradingview.com/news/panews:abc:0" {
		t.Fatalf("unexpected url: %s", item.URL)
	}
	if item.PublishedAt.Format(time.RFC3339) != "2026-06-18T18:00:00Z" {
		t.Fatalf("unexpected published time: %s", item.PublishedAt.Format(time.RFC3339))
	}
	if len(item.Tags) != 2 || item.Tags[0] != "加密货币" || item.Tags[1] != "全球市场" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
	}
}

// TestTradingViewProviderSkipsDetailFetchByDefault 验证生产默认只抓列表，避免资讯中心首屏逐条补详情导致超时。
func TestTradingViewProviderSkipsDetailFetchByDefault(t *testing.T) {
	detailCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/news-flow/v2/news":
			_, _ = writer.Write([]byte(`{
				"items": [
					{"id": "provider:fast:0", "title": "快速新闻", "published": 1781805600, "provider": {"name": "Fast"}}
				]
			}`))
		case "/v3/story":
			detailCalls++
			_, _ = writer.Write([]byte(`{"shortDescription":"不应请求详情"}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestTradingViewProvider(t, TradingViewConfig{
		NewsURL: server.URL + "/news-flow/v2/news",
	})

	items, err := provider.Market(context.Background(), MarketRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 1 || items[0].Title != "快速新闻" || items[0].Summary != "快速新闻" {
		t.Fatalf("unexpected fast news item: %+v", items)
	}
	if detailCalls != 0 {
		t.Fatalf("expected detail endpoint not called by default, got %d", detailCalls)
	}
}

// TestEastMoneyResearchProviderFetchesMarketResearchFeeds 验证东方财富研报、行业研究和公司公告会进入资讯中心统一列表。
func TestEastMoneyResearchProviderFetchesMarketResearchFeeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/report/list2":
			body, _ := io.ReadAll(request.Body)
			var payload struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("invalid stock research body: %v", err)
			}
			if payload.Code != "" {
				t.Fatalf("market request should not pin stock code, got %q", payload.Code)
			}
			_, _ = writer.Write([]byte(`{
				"data": [
					{
						"infoCode": "AP202606270001",
						"title": "中天科技首次覆盖报告",
						"stockCode": "600522",
						"stockName": "中天科技",
						"indvInduName": "通信设备",
						"emRatingName": "增持",
						"sRatingName": "买入",
						"researcher": "张三",
						"orgSName": "东方证券",
						"publishDate": "2026-06-27 08:00:00"
					}
				]
			}`))
		case "/report/list":
			if request.URL.Query().Get("qType") != "1" {
				t.Fatalf("unexpected industry research query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{
				"data": [
					{
						"infoCode": "IR202606270001",
						"title": "通信行业专题研究",
						"industryName": "通信设备",
						"orgSName": "中信建投",
						"publishDate": "2026-06-27 09:00:00"
					}
				]
			}`))
		case "/api/security/ann":
			_, _ = writer.Write([]byte(`{
				"data": {
					"list": [
						{
							"art_code": "AN202606270001",
							"title": "中天科技关于重大合同的公告",
							"notice_date": "2026-06-27 10:00:00",
							"columns": [{"column_name": "重大事项"}],
							"codes": [{"stock_code": "600522", "short_name": "中天科技"}]
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestEastMoneyResearchProvider(t, EastMoneyResearchConfig{
		StockReportURL:    server.URL + "/report/list2",
		IndustryReportURL: server.URL + "/report/list",
		AnnouncementURL:   server.URL + "/api/security/ann",
	})

	items, err := provider.Market(context.Background(), MarketRequest{Market: "CN", Limit: 10})
	if err != nil {
		t.Fatalf("Market returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected three eastmoney items, got %+v", items)
	}
	if items[0].Source != "东方财富公告" || items[0].Title != "中天科技关于重大合同的公告" {
		t.Fatalf("unexpected announcement item: %+v", items[0])
	}
	if items[0].URL != "https://data.eastmoney.com/notices/detail/600522/AN202606270001.html" {
		t.Fatalf("unexpected announcement url: %s", items[0].URL)
	}
	if len(items[0].Symbols) != 1 || items[0].Symbols[0].String() != "CN:SH:600522" {
		t.Fatalf("unexpected announcement symbols: %+v", items[0].Symbols)
	}
	if items[1].Source != "东方财富行业研究" || items[1].URL != "https://pdf.dfcfw.com/pdf/H3_IR202606270001_1.pdf" {
		t.Fatalf("unexpected industry item: %+v", items[1])
	}
	if items[2].Source != "东方财富研报" || items[2].URL != "https://pdf.dfcfw.com/pdf/H3_AP202606270001_1.pdf" {
		t.Fatalf("unexpected stock research item: %+v", items[2])
	}
	for _, item := range items {
		if len(item.Tags) == 0 {
			t.Fatalf("expected research item tags: %+v", item)
		}
	}
}

// TestEastMoneyResearchProviderListUsesRequestedStock 验证个股资讯查询会用指定股票过滤研报和公告。
func TestEastMoneyResearchProviderListUsesRequestedStock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/report/list2":
			body, _ := io.ReadAll(request.Body)
			var payload struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("invalid stock research body: %v", err)
			}
			if payload.Code != "600522" {
				t.Fatalf("expected stock code 600522, got %q", payload.Code)
			}
			_, _ = writer.Write([]byte(`{"data":[]}`))
		case "/api/security/ann":
			if request.URL.Query().Get("stock_list") != "600522" {
				t.Fatalf("expected stock_list=600522, got query %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":{"list":[]}}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestEastMoneyResearchProvider(t, EastMoneyResearchConfig{
		StockReportURL:  server.URL + "/report/list2",
		AnnouncementURL: server.URL + "/api/security/ann",
	})
	symbol, err := stockservice.ParseSymbol("CN:SH:600522")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}

	if _, err := provider.List(context.Background(), ListRequest{Symbol: symbol, Limit: 10}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

// newTestCailianpressProvider 创建测试用财联社 Provider。
func newTestCailianpressProvider(t *testing.T, config CailianpressConfig) *CailianpressProvider {
	t.Helper()
	provider, err := NewCailianpressProvider(config)
	if err != nil {
		t.Fatalf("NewCailianpressProvider returned error: %v", err)
	}
	return provider
}

// newTestWallstreetcnLiveProvider 创建测试用华尔街见闻快讯 Provider。
func newTestWallstreetcnLiveProvider(t *testing.T, config WallstreetcnLiveConfig) *WallstreetcnLiveProvider {
	t.Helper()
	provider, err := NewWallstreetcnLiveProvider(config)
	if err != nil {
		t.Fatalf("NewWallstreetcnLiveProvider returned error: %v", err)
	}
	return provider
}

// newTestSinaLiveProvider 创建测试用新浪直播 Provider。
func newTestSinaLiveProvider(t *testing.T, config SinaLiveConfig) *SinaLiveProvider {
	t.Helper()
	provider, err := NewSinaLiveProvider(config)
	if err != nil {
		t.Fatalf("NewSinaLiveProvider returned error: %v", err)
	}
	return provider
}

// newTestTradingViewProvider 创建测试用 TradingView 新闻 Provider。
func newTestTradingViewProvider(t *testing.T, config TradingViewConfig) *TradingViewProvider {
	t.Helper()
	provider, err := NewTradingViewProvider(config)
	if err != nil {
		t.Fatalf("NewTradingViewProvider returned error: %v", err)
	}
	return provider
}

// newTestEastMoneyResearchProvider 创建测试用东方财富研报公告 Provider。
func newTestEastMoneyResearchProvider(t *testing.T, config EastMoneyResearchConfig) *EastMoneyResearchProvider {
	t.Helper()
	provider, err := NewEastMoneyResearchProvider(config)
	if err != nil {
		t.Fatalf("NewEastMoneyResearchProvider returned error: %v", err)
	}
	return provider
}

type staticNewsProvider struct {
	name        string
	delay       time.Duration
	listItems   []Item
	listError   error
	marketItems []Item
	status      ProviderStatus
}

// Name 返回测试新闻 Provider 名称。
func (provider staticNewsProvider) Name() string {
	return provider.name
}

// Status 返回测试新闻 Provider 状态。
func (provider staticNewsProvider) Status(context.Context) ProviderStatus {
	if provider.status.Name != "" {
		return provider.status
	}
	return ProviderStatus{Name: provider.name, Source: provider.name, Available: true}
}

// List 返回预置个股新闻，支持模拟不支持个股新闻的市场级来源。
func (provider staticNewsProvider) List(context.Context, ListRequest) ([]Item, error) {
	if provider.listError != nil {
		return nil, provider.listError
	}
	return provider.listItems, nil
}

// Market 返回预置市场新闻。
func (provider staticNewsProvider) Market(context.Context, MarketRequest) ([]Item, error) {
	if provider.delay > 0 {
		time.Sleep(provider.delay)
	}
	return provider.marketItems, nil
}

type staticCookieResolver string

// ResolveCookie 返回测试固定 Cookie，模拟从本地凭据存储解密成功。
func (resolver staticCookieResolver) ResolveCookie(context.Context, string) (string, error) {
	return string(resolver), nil
}

type failingCookieResolver struct{}

// ResolveCookie 返回失败，模拟用户尚未配置数据源凭据。
func (failingCookieResolver) ResolveCookie(context.Context, string) (string, error) {
	return "", errors.New("credential missing")
}
