package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	notificationservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/notification"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestHandleStatusNotifiesProviderError 验证 Provider 真实异常状态会生成应用内通知。
func TestHandleStatusNotifiesProviderError(t *testing.T) {
	notifier := &recordingProviderNotifier{}
	config := Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		MarketProvider: fakeMarketProvider{
			status: market.ProviderStatus{
				Name:      "EastMoney",
				Source:    "market",
				Available: false,
				LastError: "Cookie: token=secret unavailable",
			},
		},
		NewsProvider: fakeNewsProvider{
			status: newsservice.ProviderStatus{Name: "news", Source: "news", Available: true},
		},
		Notifier: notifier,
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set(httpx.TokenHeader, "test-token")
	handleStatus(config).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(notifier.inputs) != 1 {
		t.Fatalf("expected one provider notification, got %+v", notifier.inputs)
	}
	input := notifier.inputs[0]
	if input.Name != "EastMoney" || input.Source != "market" || input.LastError == "" {
		t.Fatalf("unexpected provider notification input: %+v", input)
	}
}

// TestHandleStatusSkipsUnconfiguredProviderNotification 验证未配置 Provider 的安全空态不会刷异常通知。
func TestHandleStatusSkipsUnconfiguredProviderNotification(t *testing.T) {
	notifier := &recordingProviderNotifier{}
	config := Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Notifier: notifier,
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set(httpx.TokenHeader, "test-token")
	handleStatus(config).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(notifier.inputs) != 0 {
		t.Fatalf("unconfigured providers must not create notifications: %+v", notifier.inputs)
	}
}

// TestHandleStatusReturnsProviderStatuses 验证状态接口保持原有统一响应结构。
func TestHandleStatusReturnsProviderStatuses(t *testing.T) {
	config := Config{Security: httpx.SecurityConfig{Token: "test-token", Ready: true}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handleStatus(config).ServeHTTP(recorder, request)

	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal provider status response: %v", err)
	}
	if response.Code != 0 || response.Data == nil {
		t.Fatalf("unexpected provider status response: %+v", response)
	}
}

type recordingProviderNotifier struct {
	inputs []notificationservice.ProviderErrorInput
	err    error
}

// NotifyProviderError 记录 Provider 异常通知请求，避免 action 测试触碰真实数据库。
func (notifier *recordingProviderNotifier) NotifyProviderError(_ context.Context, input notificationservice.ProviderErrorInput) (bool, error) {
	notifier.inputs = append(notifier.inputs, input)
	return notifier.err == nil, notifier.err
}

type fakeMarketProvider struct {
	status market.ProviderStatus
}

// Name 返回测试行情 Provider 名称。
func (provider fakeMarketProvider) Name() string {
	return "fake-market"
}

// Status 返回测试行情 Provider 状态。
func (provider fakeMarketProvider) Status(context.Context) market.ProviderStatus {
	return provider.status
}

// Search 返回空搜索结果，状态接口测试不会调用该能力。
func (fakeMarketProvider) Search(context.Context, string) ([]market.StockBasic, error) {
	return nil, nil
}

// Quote 返回空行情，状态接口测试不会调用该能力。
func (fakeMarketProvider) Quote(context.Context, stock.Symbol) (market.Quote, error) {
	return market.Quote{}, nil
}

// Kline 返回空 K 线，状态接口测试不会调用该能力。
func (fakeMarketProvider) Kline(context.Context, market.KlineRequest) ([]market.KlineBar, error) {
	return nil, nil
}

type fakeNewsProvider struct {
	status newsservice.ProviderStatus
}

// Name 返回测试新闻 Provider 名称。
func (provider fakeNewsProvider) Name() string {
	return "fake-news"
}

// Status 返回测试新闻 Provider 状态。
func (provider fakeNewsProvider) Status(context.Context) newsservice.ProviderStatus {
	return provider.status
}

// List 返回空新闻，状态接口测试不会调用该能力。
func (fakeNewsProvider) List(context.Context, newsservice.ListRequest) ([]newsservice.Item, error) {
	return nil, nil
}

// Market 返回空市场新闻，状态接口测试不会调用该能力。
func (fakeNewsProvider) Market(context.Context, newsservice.MarketRequest) ([]newsservice.Item, error) {
	return nil, nil
}
