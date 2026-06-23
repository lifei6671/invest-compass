package news

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		_, _ = writer.Write([]byte(`{
			"errno": 0,
			"data": {
				"roll_data": [
					{
						"id": 123,
						"ctime": 1781805600,
						"title": "重大事项",
						"content": "A股市场出现重要消息",
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
	if item.Source != "财联社电报" || item.Title != "重大事项" || item.URL != "https://www.cls.cn/telegraph/123" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.PublishedAt.Format(time.RFC3339) != "2026-06-18T18:00:00Z" {
		t.Fatalf("unexpected published time: %s", item.PublishedAt.Format(time.RFC3339))
	}
	if len(item.Tags) != 2 || item.Tags[0] != "半导体" || item.Tags[1] != "AI" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
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
	if len(item.Tags) != 2 || item.Tags[0] != "焦点" || item.Tags[1] != "A股" {
		t.Fatalf("unexpected tags: %+v", item.Tags)
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
