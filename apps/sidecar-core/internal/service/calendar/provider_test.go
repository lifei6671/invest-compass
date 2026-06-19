package calendar

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCLSProviderFetchesEvents 验证财联社日历接口会被清洗成财经日历事件。
func TestCLSProviderFetchesEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/calendar/web/list" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{
			"data": [
				{"title":"中国5月社融数据公布","date":"2026-06-18","time":"16:00","country":"中国","importance":"高"}
			]
		}`))
	}))
	defer server.Close()

	provider := newTestCLSProvider(t, CLSConfig{URL: server.URL + "/api/calendar/web/list"})

	events, err := provider.Fetch(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", events)
	}
	if events[0].Title != "中国5月社融数据公布" || events[0].Source != "财联社日历" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

// TestCLSProviderRejectsYearMonthUntilQuerySupported 验证财联社日历不会忽略调用方传入的月份条件返回错月数据。
func TestCLSProviderRejectsYearMonthUntilQuerySupported(t *testing.T) {
	provider := newTestCLSProvider(t, CLSConfig{})

	_, err := provider.Fetch(context.Background(), Request{YearMonth: "2026-06"})
	if err == nil {
		t.Fatal("expected unsupported year month error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "request" {
		t.Fatalf("expected request ProviderError, got %T %[1]v", err)
	}
}

// TestCLSProviderRejectsRemoteError 验证财联社远端错误不会被误报为空日历。
func TestCLSProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"errno": 1001,
			"msg": "rate limited",
			"data": []
		}`))
	}))
	defer server.Close()

	provider := newTestCLSProvider(t, CLSConfig{URL: server.URL})

	_, err := provider.Fetch(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected remote error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestJiuyangProviderRequiresCredential 验证需要 Cookie/token 的渠道没有配置时会快速失败。
func TestJiuyangProviderRequiresCredential(t *testing.T) {
	provider := newTestJiuyangProvider(t, JiuyangConfig{})

	_, err := provider.Fetch(context.Background(), Request{YearMonth: "2026-06"})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "credential" {
		t.Fatalf("expected credential ProviderError, got %T %[1]v", err)
	}
}

// TestJiuyangProviderUsesConfiguredCredential 验证九阳日历只使用注入的 Cookie/token。
func TestJiuyangProviderUsesConfiguredCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("token") != "user-token" || !strings.Contains(request.Header.Get("Cookie"), "SESSION=user-session") {
			t.Fatalf("credential was not forwarded correctly")
		}
		_, _ = writer.Write([]byte(`{"data":[{"title":"产业会议","date":"2026-06-20","grade":"3"}]}`))
	}))
	defer server.Close()

	provider := newTestJiuyangProvider(t, JiuyangConfig{
		URL: server.URL,
		Credential: ChannelCredential{
			Token:  "user-token",
			Cookie: "SESSION=user-session",
		},
	})

	events, err := provider.Fetch(context.Background(), Request{YearMonth: "2026-06"})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(events) != 1 || events[0].Title != "产业会议" || events[0].Source != "九阳公社日历" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

// TestJiuyangProviderRejectsRemoteError 验证九阳远端错误不会被误报为空日历。
func TestJiuyangProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":401,"msg":"token expired","data":[]}`))
	}))
	defer server.Close()

	provider := newTestJiuyangProvider(t, JiuyangConfig{
		URL: server.URL,
		Credential: ChannelCredential{
			Token:  "user-token",
			Cookie: "SESSION=user-session",
		},
	})

	_, err := provider.Fetch(context.Background(), Request{YearMonth: "2026-06"})
	if err == nil {
		t.Fatal("expected remote error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestWallstreetcnProviderFetchesEvents 验证华尔街见闻财经日历会清洗重要字段。
func TestWallstreetcnProviderFetchesEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/apiv1/finance/indicator/search" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("start_time") != "1780272000" || query.Get("end_time") != "1782864000" || query.Get("limit") != "50" {
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
						"id": 1,
						"public_date": 1780617600,
						"country": "美国",
						"event": "美国5月非农就业人口变动",
						"importance": 3,
						"previous": "17.5万",
						"forecast": "18.0万",
						"actual": "20.0万"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnProvider(t, WallstreetcnConfig{
		CalendarURL: server.URL + "/apiv1/finance/indicator/search",
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
		},
	})

	events, err := provider.Fetch(context.Background(), Request{YearMonth: "2026-06"})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", events)
	}
	event := events[0]
	if event.Title != "美国5月非农就业人口变动" || event.Country != "美国" || event.Importance != "3" || event.Source != "华尔街见闻日历" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.Date != "2026-06-05" || event.Time != "00:00" {
		t.Fatalf("unexpected event time: %+v", event)
	}
	if event.Raw["actual"] != "20.0万" || event.Raw["forecast"] != "18.0万" || event.Raw["previous"] != "17.5万" {
		t.Fatalf("unexpected raw payload: %+v", event.Raw)
	}
}

// TestWallstreetcnProviderSkipsEventsWithoutPublicDate 验证缺失发布时间的日历行不会被清洗成 1970 年事件。
func TestWallstreetcnProviderSkipsEventsWithoutPublicDate(t *testing.T) {
	events := wallstreetcnCalendarRowsToEvents([]wallstreetcnCalendarItem{
		{
			Country:    "美国",
			Event:      "美国5月非农就业人口变动",
			Importance: 3,
		},
		{
			PublicDate: 1780617600,
			Country:    "美国",
			Event:      "美国5月失业率",
			Importance: 2,
		},
	})

	if len(events) != 1 {
		t.Fatalf("expected one valid event, got %+v", events)
	}
	if events[0].Title != "美国5月失业率" || events[0].Date != "2026-06-05" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

// newTestCLSProvider 创建测试用财联社日历 Provider。
func newTestCLSProvider(t *testing.T, config CLSConfig) *CLSProvider {
	t.Helper()
	provider, err := NewCLSProvider(config)
	if err != nil {
		t.Fatalf("NewCLSProvider returned error: %v", err)
	}
	return provider
}

// newTestWallstreetcnProvider 创建测试用华尔街见闻日历 Provider。
func newTestWallstreetcnProvider(t *testing.T, config WallstreetcnConfig) *WallstreetcnProvider {
	t.Helper()
	provider, err := NewWallstreetcnProvider(config)
	if err != nil {
		t.Fatalf("NewWallstreetcnProvider returned error: %v", err)
	}
	return provider
}

// newTestJiuyangProvider 创建测试用九阳日历 Provider。
func newTestJiuyangProvider(t *testing.T, config JiuyangConfig) *JiuyangProvider {
	t.Helper()
	provider, err := NewJiuyangProvider(config)
	if err != nil {
		t.Fatalf("NewJiuyangProvider returned error: %v", err)
	}
	return provider
}
