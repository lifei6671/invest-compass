package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/stock"
)

// TestHealthRejectsNonPost 验证本地 core API 默认拒绝非 POST 请求。
func TestHealthRejectsNonPost(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "method_not_allowed")
}

// TestHealthRequiresReadyToken 验证 sidecar 未完成握手前健康检查不可用。
func TestHealthRequiresReadyToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "",
		DBStatus: "not_configured",
		Ready:    false,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "core_not_ready")
}

// TestHealthRejectsInvalidToken 验证错误 runtime token 不能访问健康检查。
func TestHealthRejectsInvalidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)
	request.Header.Set("X-Invest-Compass-Token", "wrong-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "unauthorized")
}

// TestShutdownRequiresValidToken 验证关闭接口必须经过 runtime token 校验。
func TestShutdownRequiresValidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		OnShutdown: func() {
			t.Fatal("shutdown callback must not run for invalid token")
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/shutdown", nil)
	request.Header.Set("X-Invest-Compass-Token", "wrong-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "unauthorized")
}

// TestShutdownCallsConfiguredCallback 验证关闭接口通过后会触发进程生命周期清理回调。
func TestShutdownCallsConfiguredCallback(t *testing.T) {
	called := false
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		OnShutdown: func() {
			called = true
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/shutdown", nil)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !called {
		t.Fatal("expected shutdown callback to run")
	}
}

// TestRecoverHTTPRedactsPanicError 验证 panic 会转成统一错误响应，且不会泄露异常中的密钥。
func TestRecoverHTTPRedactsPanicError(t *testing.T) {
	handler := recoverHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(errors.New("provider failed with Authorization: Bearer demo-sensitive-value"))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)
	request.Header.Set("X-Request-Id", "req-panic")
	request.Header.Set("X-Trace-Id", "trace-panic")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "demo-sensitive-value") {
		t.Fatalf("panic response leaked secret: %s", recorder.Body.String())
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal panic response: %v", err)
	}
	if response.Code != 50000 || response.Message != "internal_error" {
		t.Fatalf("unexpected panic envelope: %+v", response)
	}
	if response.RequestID != "req-panic" || response.TraceID != "trace-panic" {
		t.Fatalf("expected propagated ids, got requestId=%q traceId=%q", response.RequestID, response.TraceID)
	}
}

// TestHealthReturnsVersionAndDBStatusWithValidToken 验证合法 token 可获取最小健康信息。
func TestHealthReturnsVersionAndDBStatusWithValidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-test")
	request.Header.Set("X-Trace-Id", "trace-test")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected response envelope: %+v", response)
	}
	if response.RequestID != "req-test" || response.TraceID != "trace-test" {
		t.Fatalf("expected propagated ids, got requestId=%q traceId=%q", response.RequestID, response.TraceID)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected health data object, got %T", response.Data)
	}
	if data["version"] != "0.1.0" || data["dbStatus"] != "not_configured" {
		t.Fatalf("unexpected health data: %#v", data)
	}
}

// TestStockSearchRejectsEmptyKeyword 验证股票搜索会拒绝空 keyword。
func TestStockSearchRejectsEmptyKeyword(t *testing.T) {
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		MarketProvider: fakeMarketProvider{},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/search", strings.NewReader(`{"keyword":"  "}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "invalid_keyword")
}

// TestStockSearchReturnsStandardSymbol 验证股票搜索通过 Provider 返回标准股票基础信息。
func TestStockSearchReturnsStandardSymbol(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}

	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		MarketProvider: fakeMarketProvider{
			searchResults: []market.StockBasic{{
				Symbol:   symbol,
				Name:     "贵州茅台",
				Code:     "600519",
				Market:   "CN",
				Exchange: "SH",
			}},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/search", strings.NewReader(`{"keyword":"茅台"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-search")
	request.Header.Set("X-Trace-Id", "trace-search")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal stock search response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected stock search envelope: %+v", response)
	}
	if response.RequestID != "req-search" || response.TraceID != "trace-search" {
		t.Fatalf("expected propagated ids, got requestId=%q traceId=%q", response.RequestID, response.TraceID)
	}

	results, ok := response.Data.([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected one stock result, got %#v", response.Data)
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", results[0])
	}
	if first["symbol"] != "CN:SH:600519" ||
		first["name"] != "贵州茅台" ||
		first["code"] != "600519" ||
		first["market"] != "CN" ||
		first["exchange"] != "SH" {
		t.Fatalf("unexpected stock search data: %#v", first)
	}
}

// TestProviderStatusReturnsSafeStatus 验证数据源状态 API 返回脱敏后的可展示状态。
func TestProviderStatusReturnsSafeStatus(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		MarketProvider: fakeMarketProvider{
			status: market.ProviderStatus{
				Name:      "demo-provider",
				Source:    "demo-source",
				Available: false,
				LastError: "Authorization: Bearer demo-sensitive-value",
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-provider-status")
	request.Header.Set("X-Trace-Id", "trace-provider-status")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "demo-sensitive-value") {
		t.Fatalf("provider status leaked secret: %s", recorder.Body.String())
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal provider status response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected provider status envelope: %+v", response)
	}
	statuses, ok := response.Data.([]any)
	if !ok || len(statuses) != 1 {
		t.Fatalf("expected one provider status, got %#v", response.Data)
	}
	first, ok := statuses[0].(map[string]any)
	if !ok {
		t.Fatalf("expected status object, got %T", statuses[0])
	}
	if first["name"] != "demo-provider" ||
		first["source"] != "demo-source" ||
		first["available"] != false {
		t.Fatalf("unexpected provider status data: %#v", first)
	}
}

// TestDashboardSummaryReturnsInjectedData 验证 Dashboard summary API 只从注入数据源构建首版允许字段。
func TestDashboardSummaryReturnsInjectedData(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		DashboardInput: dashboard.Input{
			WatchlistQuotes: []market.Quote{
				{ChangePercent: 1},
				{ChangePercent: -1},
				{ChangePercent: 0},
			},
			ProviderStatuses: []market.ProviderStatus{{
				Name:      "demo-provider",
				Source:    "demo-source",
				Available: false,
				LastError: "Authorization: Bearer demo-sensitive-value",
			}},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/dashboard/summary", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-dashboard")
	request.Header.Set("X-Trace-Id", "trace-dashboard")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	payload := recorder.Body.String()
	if strings.Contains(payload, "demo-sensitive-value") {
		t.Fatalf("dashboard summary leaked secret: %s", payload)
	}
	for _, forbidden := range []string{"strategy", "announcement", "research", "fund_flow"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("dashboard summary exposed unsupported field %q: %s", forbidden, payload)
		}
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal dashboard summary response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected dashboard summary envelope: %+v", response)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected dashboard summary object, got %#v", response.Data)
	}
	watchlist, ok := data["watchlist"].(map[string]any)
	if !ok {
		t.Fatalf("expected watchlist summary object, got %#v", data["watchlist"])
	}
	if watchlist["up_count"] != float64(1) ||
		watchlist["down_count"] != float64(1) ||
		watchlist["flat_count"] != float64(1) {
		t.Fatalf("unexpected watchlist summary: %#v", watchlist)
	}
}

// assertErrorEnvelope 校验错误响应必须包含统一 envelope 和追踪 ID。
func assertErrorEnvelope(t *testing.T, body string, message string) {
	t.Helper()

	var response apiResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if response.Code == 0 {
		t.Fatalf("expected non-zero error code, got response %+v", response)
	}
	if response.Message != message {
		t.Fatalf("expected message %q, got %q", message, response.Message)
	}
	if response.RequestID == "" || response.TraceID == "" {
		t.Fatalf("expected requestId and traceId, got response %+v", response)
	}
}

type fakeMarketProvider struct {
	searchResults []market.StockBasic
	status        market.ProviderStatus
}

// Name 返回测试 Provider 名称。
func (provider fakeMarketProvider) Name() string {
	return "fake"
}

// Status 返回测试 Provider 状态。
func (provider fakeMarketProvider) Status(context.Context) market.ProviderStatus {
	if provider.status.Name != "" {
		return provider.status
	}
	return market.ProviderStatus{Name: provider.Name(), Available: true}
}

// Search 返回测试预置的股票搜索结果。
func (provider fakeMarketProvider) Search(context.Context, string) ([]market.StockBasic, error) {
	return provider.searchResults, nil
}

// Quote 在 server 搜索测试中不会被调用。
func (provider fakeMarketProvider) Quote(context.Context, stock.Symbol) (market.Quote, error) {
	return market.Quote{}, nil
}

// Kline 在 server 搜索测试中不会被调用。
func (provider fakeMarketProvider) Kline(context.Context, market.KlineRequest) ([]market.KlineBar, error) {
	return nil, nil
}
