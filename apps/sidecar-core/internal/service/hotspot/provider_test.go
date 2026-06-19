package hotspot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestXueqiuProviderRequiresCookie 验证雪球渠道缺少用户 Cookie 时不会尝试抓取。
func TestXueqiuProviderRequiresCookie(t *testing.T) {
	provider := newTestXueqiuProvider(t, XueqiuConfig{})

	_, err := provider.FetchHotStocks(context.Background(), HotStockRequest{Size: 10, MarketType: "10"})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "credential" {
		t.Fatalf("expected credential ProviderError, got %T %[1]v", err)
	}
}

// TestXueqiuProviderFetchesHotStocks 验证雪球热股接口使用用户 Cookie 并清洗结果。
func TestXueqiuProviderFetchesHotStocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v5/stock/hot_stock/list.json" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if !strings.Contains(request.Header.Get("Cookie"), "xq_a_token=user-token") {
			t.Fatalf("missing cookie: %q", request.Header.Get("Cookie"))
		}
		_, _ = writer.Write([]byte(`{
			"error_code": 0,
			"data": {
				"items": [
					{"code":"SH600519","name":"贵州茅台","value":99.9}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestXueqiuProvider(t, XueqiuConfig{
		HotStockURL: server.URL + "/v5/stock/hot_stock/list.json",
		Credential:  ChannelCredential{Cookie: "xq_a_token=user-token"},
	})

	items, err := provider.FetchHotStocks(context.Background(), HotStockRequest{Size: 10, MarketType: "10"})
	if err != nil {
		t.Fatalf("FetchHotStocks returned error: %v", err)
	}
	if len(items) != 1 || items[0].Code != "SH600519" || items[0].Name != "贵州茅台" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

// TestXueqiuProviderRejectsInvalidHotStockValue 验证远端热度坏字段不会被静默清洗成 0。
func TestXueqiuProviderRejectsInvalidHotStockValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"error_code": 0,
			"data": {
				"items": [
					{"code":"SH600519","name":"贵州茅台","value":"not-a-number"}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestXueqiuProvider(t, XueqiuConfig{
		HotStockURL: server.URL,
		Credential:  ChannelCredential{Cookie: "xq_a_token=user-token"},
	})

	_, err := provider.FetchHotStocks(context.Background(), HotStockRequest{Size: 10, MarketType: "10"})
	if err == nil {
		t.Fatal("expected invalid value error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "parse_hot_stocks" {
		t.Fatalf("expected parse_hot_stocks ProviderError, got %T %[1]v", err)
	}
}

// newTestXueqiuProvider 创建测试用雪球热点 Provider。
func newTestXueqiuProvider(t *testing.T, config XueqiuConfig) *XueqiuProvider {
	t.Helper()
	provider, err := NewXueqiuProvider(config)
	if err != nil {
		t.Fatalf("NewXueqiuProvider returned error: %v", err)
	}
	return provider
}
