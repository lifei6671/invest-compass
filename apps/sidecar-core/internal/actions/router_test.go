package actions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	analysisservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/analysis"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	promptservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/prompt"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/updatecheck"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
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

// TestTaskLogRouteRejectsNonPost 验证任务日志路由同样受全局 POST-only 边界保护。
func TestTaskLogRouteRejectsNonPost(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/logs/list", nil)
	request.Header.Set(httpx.TokenHeader, "test-token")

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

// TestRecoverGinRedactsPanicError 验证 Gin panic recovery 会转成统一错误响应且不会泄露密钥。
func TestRecoverGinRedactsPanicError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := gin.New()
	handler.Use(recoverGin())
	handler.POST("/panic", func(*gin.Context) {
		panic(errors.New("provider failed with Authorization: Bearer demo-sensitive-value"))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/panic", nil)
	request.Header.Set("X-Request-Id", "req-panic")
	request.Header.Set("X-Trace-Id", "trace-panic")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "demo-sensitive-value") {
		t.Fatalf("panic response leaked secret: %s", recorder.Body.String())
	}

	var response httpx.Response
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

	var response httpx.Response
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

	var response httpx.Response
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

// TestStockSearchUpsertsStockCache 验证股票搜索成功后会写入标准股票基础信息缓存。
func TestStockSearchUpsertsStockCache(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	store := &memoryStockStore{}
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
				Industry: "白酒",
				Concept:  "消费",
			}},
		},
		StockStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/search", strings.NewReader(`{"keyword":"茅台"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one cached stock, got %+v", store.saved)
	}
	cached := store.saved[0]
	if cached.Symbol != "CN:SH:600519" ||
		cached.Name != "贵州茅台" ||
		cached.Code != "600519" ||
		cached.Market != "CN" ||
		cached.Exchange != "SH" ||
		cached.Industry != "白酒" ||
		cached.Concept != "消费" {
		t.Fatalf("unexpected cached stock: %+v", cached)
	}
	if len(store.jobs) != 1 ||
		store.jobs[0].DocType != "stock" ||
		store.jobs[0].RefID != "CN:SH:600519" ||
		store.jobs[0].Operation != "upsert" {
		t.Fatalf("expected stock search index job, got %+v", store.jobs)
	}
}

// TestDocumentSearchRoutesAreScoped 验证菜单范围搜索路由已注册，且不存在全局搜索入口。
func TestDocumentSearchRoutesAreScoped(t *testing.T) {
	handler := NewHandler(Config{
		Version:               "0.1.0",
		Token:                 "test-token",
		DBStatus:              "not_configured",
		Ready:                 true,
		DocumentSearchService: &fakeRootDocumentSearchService{},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/search/reports", strings.NewReader(`{"keyword":"茅台","limit":20,"offset":0}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected scoped report search route status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/search/reports", nil)
	getRequest.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected non-POST status %d, got %d", http.StatusMethodNotAllowed, getRecorder.Code)
	}

	globalRecorder := httptest.NewRecorder()
	globalRequest := httptest.NewRequest(http.MethodPost, "/api/search/global", strings.NewReader(`{"keyword":"茅台"}`))
	globalRequest.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(globalRecorder, globalRequest)
	if globalRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected no global search route status %d, got %d", http.StatusNotFound, globalRecorder.Code)
	}
}

// TestMarketQuoteReturnsAndCachesProviderResult 验证行情快照 API 返回标准字段并写入缓存表边界。
func TestMarketQuoteReturnsAndCachesProviderResult(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	quoteTime := time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC)
	quoteCalls := 0
	store := newMemoryMarketStore()
	handler := NewHandler(Config{
		Version:     "0.1.0",
		Token:       "test-token",
		DBStatus:    "not_configured",
		Ready:       true,
		MarketStore: store,
		MarketProvider: fakeMarketProvider{
			quoteCalls: &quoteCalls,
			quoteResult: market.Quote{
				Symbol:        symbol,
				Price:         1688.5,
				ChangeAmount:  12.3,
				ChangePercent: 0.73,
				QuoteTime:     quoteTime,
				Provider:      "fake",
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/market/quote", strings.NewReader(`{"symbol":"cn:sh:600519"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	if data["symbol"] != "CN:SH:600519" ||
		data["price"] != 1688.5 ||
		data["change_amount"] != 12.3 ||
		data["change_percent"] != 0.73 ||
		data["provider"] != "fake" ||
		data["quote_time"] != quoteTime.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected quote data: %#v", data)
	}
	if quoteCalls != 1 {
		t.Fatalf("expected provider quote called once, got %d", quoteCalls)
	}
	cached, ok, err := store.LatestQuote(context.Background(), "CN:SH:600519", time.Minute)
	if err != nil || !ok {
		t.Fatalf("expected quote cache saved, ok=%v err=%v", ok, err)
	}
	if cached.Price != 1688.5 || cached.Provider != "fake" {
		t.Fatalf("unexpected cached quote: %+v", cached)
	}
}

// TestMarketQuoteUsesFreshCacheBeforeProvider 验证短周期行情缓存命中时不会重复调用 Provider。
func TestMarketQuoteUsesFreshCacheBeforeProvider(t *testing.T) {
	quoteCalls := 0
	store := newMemoryMarketStore()
	store.quotes["CN:SH:600519"] = model.Quote{
		Symbol:        "CN:SH:600519",
		Price:         1700,
		ChangePercent: 1.2,
		QuoteTime:     time.Date(2026, 6, 18, 10, 31, 0, 0, time.UTC),
		Provider:      "cache",
		UpdatedAt:     time.Now().UTC(),
	}
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		MarketStore:    store,
		MarketProvider: fakeMarketProvider{quoteCalls: &quoteCalls},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/market/quote", strings.NewReader(`{"symbol":"CN:SH:600519"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	if data["price"] != float64(1700) || data["provider"] != "cache" {
		t.Fatalf("expected cached quote response, got %#v", data)
	}
	if quoteCalls != 0 {
		t.Fatalf("expected provider quote not called on cache hit, got %d", quoteCalls)
	}
}

// TestMarketKlineReturnsOrderedBarsAndCachesProviderResult 验证 K 线 API 按交易日升序返回并写入缓存。
func TestMarketKlineReturnsOrderedBarsAndCachesProviderResult(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	klineCalls := 0
	store := newMemoryMarketStore()
	handler := NewHandler(Config{
		Version:     "0.1.0",
		Token:       "test-token",
		DBStatus:    "not_configured",
		Ready:       true,
		MarketStore: store,
		MarketProvider: fakeMarketProvider{
			klineCalls: &klineCalls,
			klineResults: []market.KlineBar{
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-18", Close: 101, Provider: "fake"},
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-17", Close: 100, Provider: "fake"},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/market/kline",
		strings.NewReader(`{"symbol":"CN:SH:600519","period":"day","adjust":"none","limit":2}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected two kline items, got %#v", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["trade_date"] != "2026-06-17" || first["close"] != float64(100) {
		t.Fatalf("unexpected first kline item: %#v", items[0])
	}
	second, ok := items[1].(map[string]any)
	if !ok || second["trade_date"] != "2026-06-18" || second["close"] != float64(101) {
		t.Fatalf("unexpected second kline item: %#v", items[1])
	}
	if klineCalls != 1 {
		t.Fatalf("expected provider kline called once, got %d", klineCalls)
	}
	cached, err := store.ListKlines(context.Background(), "CN:SH:600519", "day", "none", 2)
	if err != nil || len(cached) != 2 {
		t.Fatalf("expected cached klines saved, len=%d err=%v", len(cached), err)
	}
}

// TestMarketIndicatorsComputesFromCachedKlines 验证技术指标 API 复用 K 线缓存并由 Go core 计算指标。
func TestMarketIndicatorsComputesFromCachedKlines(t *testing.T) {
	klineCalls := 0
	store := newMemoryMarketStore()
	for index, closePrice := range []float64{100, 101, 102, 103, 104} {
		tradeDate := time.Date(2026, 6, 14+index, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
		if err := store.SaveKlines(context.Background(), []model.Kline{{
			Symbol:    "CN:SH:600519",
			Period:    "day",
			Adjust:    "none",
			TradeDate: tradeDate,
			High:      closePrice + 1,
			Low:       closePrice - 1,
			Close:     closePrice,
			Volume:    1000 + float64(index)*100,
			Provider:  "cache",
		}}); err != nil {
			t.Fatalf("seed kline cache: %v", err)
		}
	}
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		MarketStore:    store,
		MarketProvider: fakeMarketProvider{klineCalls: &klineCalls},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/market/indicators",
		strings.NewReader(`{"symbol":"CN:SH:600519","period":"day","adjust":"none","limit":5,"indicators":["ma","volume_ma","change_percent","max_drawdown"]}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	indicators, ok := data["indicators"].(map[string]any)
	if !ok {
		t.Fatalf("expected indicators object, got %#v", data)
	}
	ma, ok := indicators["ma"].(map[string]any)
	if !ok {
		t.Fatalf("expected ma object, got %#v", indicators["ma"])
	}
	ma5, ok := ma["ma5"].([]any)
	if !ok || len(ma5) != 5 || ma5[4] != float64(102) {
		t.Fatalf("unexpected ma5 series: %#v", ma["ma5"])
	}
	volumeMA, ok := indicators["volume_ma"].(map[string]any)
	if !ok {
		t.Fatalf("expected volume_ma object, got %#v", indicators["volume_ma"])
	}
	volumeMA5, ok := volumeMA["ma5"].([]any)
	if !ok || len(volumeMA5) != 5 || volumeMA5[4] != float64(1200) {
		t.Fatalf("unexpected volume ma5 series: %#v", volumeMA["ma5"])
	}
	if indicators["max_drawdown"] != float64(0) {
		t.Fatalf("unexpected max drawdown: %#v", indicators["max_drawdown"])
	}
	if klineCalls != 0 {
		t.Fatalf("expected provider kline not called when cache is enough, got %d", klineCalls)
	}
}

// TestMarketIndicatorsFetchesKlineWhenCacheIsInsufficient 验证缓存不足时指标 API 拉取 Provider K 线并写回缓存。
func TestMarketIndicatorsFetchesKlineWhenCacheIsInsufficient(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	klineCalls := 0
	store := newMemoryMarketStore()
	handler := NewHandler(Config{
		Version:     "0.1.0",
		Token:       "test-token",
		DBStatus:    "not_configured",
		Ready:       true,
		MarketStore: store,
		MarketProvider: fakeMarketProvider{
			klineCalls: &klineCalls,
			klineResults: []market.KlineBar{
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-14", Close: 100, Volume: 1000, Provider: "fake"},
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-15", Close: 101, Volume: 1100, Provider: "fake"},
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-16", Close: 102, Volume: 1200, Provider: "fake"},
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-17", Close: 103, Volume: 1300, Provider: "fake"},
				{Symbol: symbol, Period: market.PeriodDay, Adjust: market.AdjustNone, TradeDate: "2026-06-18", Close: 104, Volume: 1400, Provider: "fake"},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/market/indicators",
		strings.NewReader(`{"symbol":"CN:SH:600519","period":"day","adjust":"none","limit":5,"indicators":["ma"]}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if klineCalls != 1 {
		t.Fatalf("expected provider kline called once, got %d", klineCalls)
	}
	cached, err := store.ListKlines(context.Background(), "CN:SH:600519", "day", "none", 5)
	if err != nil || len(cached) != 5 {
		t.Fatalf("expected provider klines cached, len=%d err=%v", len(cached), err)
	}
}

// TestNewsListNormalizesDeduplicatesAndCachesProviderItems 验证个股新闻 API 标准化输出并按 content_hash 去重入库。
func TestNewsListNormalizesDeduplicatesAndCachesProviderItems(t *testing.T) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	newsCalls := 0
	publishedAt := time.Date(2026, 6, 18, 9, 30, 0, 0, time.UTC)
	store := newMemoryNewsStore()
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		NewsStore: store,
		NewsProvider: fakeNewsProvider{
			listCalls: &newsCalls,
			listItems: []newsservice.Item{
				{
					Source:      "source-a",
					Title:       " 贵州茅台发布经营动态 ",
					URL:         "HTTPS://Example.COM/a",
					Summary:     "经营保持稳定",
					PublishedAt: publishedAt,
					Symbols:     []stock.Symbol{symbol},
					Tags:        []string{"company"},
				},
				{
					Source:      "source-b",
					Title:       "贵州茅台发布经营动态",
					URL:         "https://example.com/b",
					Summary:     "经营保持稳定",
					PublishedAt: publishedAt,
					Symbols:     []stock.Symbol{symbol},
				},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/news/list",
		strings.NewReader(`{"symbol":"CN:SH:600519","limit":5}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one deduped news item, got %#v", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected news item object, got %#v", items[0])
	}
	if first["title"] != "贵州茅台发布经营动态" ||
		first["url"] != "https://example.com/a" ||
		first["published_at"] != publishedAt.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected news item: %#v", first)
	}
	if newsCalls != 1 {
		t.Fatalf("expected provider list called once, got %d", newsCalls)
	}
	cached, err := store.ListNewsBySymbol(context.Background(), "CN:SH:600519", 10, time.Hour)
	if err != nil || len(cached) != 1 {
		t.Fatalf("expected one cached news item, len=%d err=%v", len(cached), err)
	}
}

// TestNewsMarketReturnsSortedCachedItems 验证市场新闻 API 优先读取缓存并按发布时间倒序返回。
func TestNewsMarketReturnsSortedCachedItems(t *testing.T) {
	marketCalls := 0
	store := newMemoryNewsStore()
	if err := store.SaveNewsItems(context.Background(), []model.NewsItem{
		{Title: "旧新闻", URL: "https://example.com/old", ContentHash: "old", PublishedAt: time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC), Source: "cache"},
		{Title: "新新闻", URL: "https://example.com/new", ContentHash: "new", PublishedAt: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC), Source: "cache"},
	}); err != nil {
		t.Fatalf("seed news cache: %v", err)
	}
	handler := NewHandler(Config{
		Version:      "0.1.0",
		Token:        "test-token",
		DBStatus:     "not_configured",
		Ready:        true,
		NewsStore:    store,
		NewsProvider: fakeNewsProvider{marketCalls: &marketCalls},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/news/market", strings.NewReader(`{"market":"CN","limit":2}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected two market news items, got %#v", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["title"] != "新新闻" {
		t.Fatalf("expected newest news first, got %#v", items[0])
	}
	if marketCalls != 0 {
		t.Fatalf("expected provider market not called when cache is enough, got %d", marketCalls)
	}
}

// TestWatchlistCRUDUsesStoreAndSoftDelete 验证自选股 API 使用真实 store 边界完成增删改查。
func TestWatchlistCRUDUsesStoreAndSoftDelete(t *testing.T) {
	store := newMemoryWatchlistStore()
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		WatchlistStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/watchlist/create",
		strings.NewReader(`{"symbol":"cn:sh:600519","tags":["白酒","核心"],"note":"长期观察","sort_order":2}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	created := decodeResponseData(t, recorder.Body.Bytes())
	if created["symbol"] != "CN:SH:600519" || created["note"] != "长期观察" {
		t.Fatalf("unexpected created watchlist item: %#v", created)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(
		http.MethodPost,
		"/api/watchlist/update",
		strings.NewReader(`{"id":1,"tags":["核心"],"note":"等待回调","sort_order":1}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/watchlist/list", strings.NewReader(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	listData := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := listData["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one active watchlist item, got %#v", listData)
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["sort_order"] != float64(1) || item["note"] != "等待回调" {
		t.Fatalf("unexpected listed item: %#v", items[0])
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/watchlist/delete", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/watchlist/list", strings.NewReader(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	listData = decodeResponseData(t, recorder.Body.Bytes())
	items, ok = listData["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("expected soft deleted item hidden, got %#v", listData)
	}
}

// TestWatchlistCreateRejectsDuplicateActiveSymbol 验证重复添加同一 active symbol 返回稳定错误。
func TestWatchlistCreateRejectsDuplicateActiveSymbol(t *testing.T) {
	store := newMemoryWatchlistStore()
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		WatchlistStore: store,
	})

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/watchlist/create", strings.NewReader(`{"symbol":"US:AAPL"}`))
		request.Header.Set("X-Invest-Compass-Token", "test-token")
		handler.ServeHTTP(recorder, request)

		if attempt == 0 && recorder.Code != http.StatusOK {
			t.Fatalf("expected first create status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
		}
		if attempt == 1 {
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected duplicate status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
			}
			assertErrorEnvelope(t, recorder.Body.String(), "duplicate_active_symbol")
		}
	}
}

// TestPromptTemplateCRUDUsesStoreAndSoftDelete 验证 Prompt 模板 API 复用 service 规则完成增删改查。
func TestPromptTemplateCRUDUsesStoreAndSoftDelete(t *testing.T) {
	store := newMemoryPromptTemplateStore()
	handler := NewHandler(Config{
		Version:             "0.1.0",
		Token:               "test-token",
		DBStatus:            "not_configured",
		Ready:               true,
		PromptTemplateStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/prompt-templates/create",
		strings.NewReader(`{"name":"综合分析","type":"stock_full","description":"首版综合模板","content":"分析 {{ stock_name }} 和 {{ quote }}"}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	created := decodeResponseData(t, recorder.Body.Bytes())
	if created["id"] != float64(1) || created["type"] != "stock_full" || created["is_builtin"] != false {
		t.Fatalf("unexpected created prompt template: %#v", created)
	}
	variables, ok := created["variables"].([]any)
	if !ok || len(variables) != 2 || variables[0] != "stock_name" || variables[1] != "quote" {
		t.Fatalf("unexpected prompt variables: %#v", created["variables"])
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(
		http.MethodPost,
		"/api/prompt-templates/update",
		strings.NewReader(`{"id":1,"name":"技术分析","type":"technical","description":"技术模板","content":"技术指标 {{ indicators }}"}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	updated := decodeResponseData(t, recorder.Body.Bytes())
	if updated["type"] != "technical" || updated["content"] != "技术指标 {{ indicators }}" {
		t.Fatalf("unexpected updated prompt template: %#v", updated)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/prompt-templates/get", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	got := decodeResponseData(t, recorder.Body.Bytes())
	if got["name"] != "技术分析" {
		t.Fatalf("unexpected prompt get payload: %#v", got)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/prompt-templates/delete", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/prompt-templates/list", strings.NewReader(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	listData := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := listData["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("expected soft deleted prompt template hidden, got %#v", listData)
	}
}

// TestPromptTemplateRejectsUnsupportedVariable 验证未列入白名单的模板变量不能保存。
func TestPromptTemplateRejectsUnsupportedVariable(t *testing.T) {
	handler := NewHandler(Config{
		Version:             "0.1.0",
		Token:               "test-token",
		DBStatus:            "not_configured",
		Ready:               true,
		PromptTemplateStore: newMemoryPromptTemplateStore(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/prompt-templates/create",
		strings.NewReader(`{"name":"越界模板","type":"custom","content":"资金流 {{ capital_flow }}"}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported variable status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), "unsupported_prompt_variable")
}

// TestPromptTemplateBuiltinReadonly 验证内置模板不能通过 CRUD API 更新或删除。
func TestPromptTemplateBuiltinReadonly(t *testing.T) {
	store := newMemoryPromptTemplateStore()
	store.items[1] = model.PromptTemplate{
		ID:        1,
		Name:      "内置系统模板",
		Type:      string(promptservice.TemplateSystem),
		Content:   "系统提示 {{ analysis_language }}",
		Variables: `["analysis_language"]`,
		IsBuiltin: true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	handler := NewHandler(Config{
		Version:             "0.1.0",
		Token:               "test-token",
		DBStatus:            "not_configured",
		Ready:               true,
		PromptTemplateStore: store,
	})

	for _, testCase := range []struct {
		name string
		path string
		body string
	}{
		{
			name: "update builtin",
			path: "/api/prompt-templates/update",
			body: `{"id":1,"name":"改名","type":"system","content":"系统提示 {{ analysis_language }}"}`,
		},
		{
			name: "delete builtin",
			path: "/api/prompt-templates/delete",
			body: `{"id":1}`,
		},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body))
		request.Header.Set("X-Invest-Compass-Token", "test-token")
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s expected status %d, got %d, body %s", testCase.name, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		}
		assertErrorEnvelope(t, recorder.Body.String(), "builtin_prompt_template_readonly")
	}
}

// TestTasksAPIListsGetsAndReplaysEvents 验证任务历史 API 使用真实 store 返回任务和事件。
func TestTasksAPIListsGetsAndReplaysEvents(t *testing.T) {
	store := newMemoryTaskStore()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	store.tasks["task-old"] = model.Task{
		ID:        "task-old",
		Type:      "ANALYSIS",
		Status:    "SUCCESS",
		Title:     "旧任务",
		UpdatedAt: base,
		CreatedAt: base,
	}
	store.tasks["task-new"] = model.Task{
		ID:           "task-new",
		Type:         "ANALYSIS",
		Status:       "FAILED",
		Title:        "新任务",
		Progress:     80,
		ErrorMessage: "provider error",
		UpdatedAt:    base.Add(time.Hour),
		CreatedAt:    base,
	}
	store.events = append(store.events,
		model.TaskEvent{ID: 1, TaskID: "task-new", EventType: "TASK_STARTED", Payload: `{"progress":10}`, CreatedAt: base},
		model.TaskEvent{ID: 2, TaskID: "task-new", EventType: "TASK_LOG", Payload: `{"Authorization":"Bearer sk-secret","message":"ok"}`, CreatedAt: base.Add(time.Minute)},
	)

	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/list", strings.NewReader(`{"limit":10}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	listData := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := listData["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected two tasks, got %#v", listData)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != "task-new" || first["status"] != "FAILED" {
		t.Fatalf("expected newest task first, got %#v", items[0])
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/tasks/get", strings.NewReader(`{"task_id":"task-new"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	taskData := decodeResponseData(t, recorder.Body.Bytes())
	if taskData["id"] != "task-new" || taskData["error_message"] != "provider error" {
		t.Fatalf("unexpected task detail: %#v", taskData)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/tasks/events", strings.NewReader(`{"task_id":"task-new","after_event_id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected events status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	eventData := decodeResponseData(t, recorder.Body.Bytes())
	events, ok := eventData["items"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("expected one replay event, got %#v", eventData)
	}
	event, ok := events[0].(map[string]any)
	if !ok || event["id"] != float64(2) || strings.Contains(event["payload"].(string), "sk-secret") {
		t.Fatalf("expected sanitized incremental event, got %#v", events[0])
	}
}

// TestTasksAPIRejectsInvalidListLimit 验证任务历史列表在 HTTP 边界拒绝无界查询参数。
func TestTasksAPIRejectsInvalidListLimit(t *testing.T) {
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: newMemoryTaskStore(),
	})

	for _, body := range []string{`{"limit":0}`, `{"limit":101}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/tasks/list", strings.NewReader(body))
		request.Header.Set("X-Invest-Compass-Token", "test-token")
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid limit status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
		}
		assertErrorEnvelope(t, recorder.Body.String(), "invalid_limit")
	}
}

// TestTasksAPIRejectsNegativeAfterEventID 验证任务事件回放拒绝负数游标。
func TestTasksAPIRejectsNegativeAfterEventID(t *testing.T) {
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: newMemoryTaskStore(),
	})

	for _, path := range []string{"/api/tasks/events", "/api/tasks/events/stream"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"task_id":"task-negative","after_event_id":-1}`))
		request.Header.Set("X-Invest-Compass-Token", "test-token")
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid after_event_id status %d for %s, got %d, body %s", http.StatusBadRequest, path, recorder.Code, recorder.Body.String())
		}
		assertErrorEnvelope(t, recorder.Body.String(), "invalid_after_event_id")
	}
}

// TestTaskEventsStreamReplaysSanitizedSSEFrames 验证任务事件流 API 用 SSE 帧补拉历史事件。
func TestTaskEventsStreamReplaysSanitizedSSEFrames(t *testing.T) {
	store := newMemoryTaskStore()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	store.tasks["task-stream"] = model.Task{
		ID:        "task-stream",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		UpdatedAt: base,
	}
	store.events = append(store.events,
		model.TaskEvent{ID: 1, TaskID: "task-stream", EventType: "TASK_STARTED", Payload: `{"progress":10}`, CreatedAt: base},
		model.TaskEvent{ID: 2, TaskID: "task-stream", EventType: "TASK_CHUNK", Payload: `{"content":"ok","Authorization":"Bearer sk-secret"}`, CreatedAt: base.Add(time.Minute)},
		model.TaskEvent{ID: 3, TaskID: "task-stream", EventType: "TASK_SUCCESS", Payload: `{"progress":100}`, CreatedAt: base.Add(2 * time.Minute)},
	)
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/events/stream", strings.NewReader(`{"task_id":"task-stream","after_event_id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected stream status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "id: 2\n") ||
		!strings.Contains(body, "event: TASK_CHUNK\n") ||
		!strings.Contains(body, "event: TASK_SUCCESS\n") ||
		!strings.Contains(body, "data: ") ||
		strings.Contains(body, "sk-secret") {
		t.Fatalf("unexpected SSE stream body: %q", body)
	}
}

// TestTaskEventsStreamWaitsForNewTerminalEvent 验证任务事件流会等待新增事件并在终态事件后结束。
func TestTaskEventsStreamWaitsForNewTerminalEvent(t *testing.T) {
	store := newMemoryTaskStore()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	store.tasks["task-stream-wait"] = model.Task{
		ID:        "task-stream-wait",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		UpdatedAt: base,
	}
	store.events = append(store.events,
		model.TaskEvent{ID: 1, TaskID: "task-stream-wait", EventType: "TASK_STARTED", Payload: `{"progress":10}`, CreatedAt: base},
	)
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/events/stream", strings.NewReader(`{"task_id":"task-stream-wait","after_event_id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()

	time.Sleep(50 * time.Millisecond)
	if recorder.Body.Len() != 0 {
		t.Fatalf("stream returned before new event arrived: %q", recorder.Body.String())
	}
	store.appendEvent(model.TaskEvent{
		ID:        2,
		TaskID:    "task-stream-wait",
		EventType: "TASK_SUCCESS",
		Payload:   `{"progress":100}`,
		CreatedAt: base.Add(time.Minute),
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not finish after terminal event")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "id: 2\n") || !strings.Contains(body, "event: TASK_SUCCESS\n") {
		t.Fatalf("expected streamed terminal event, got %q", body)
	}
}

// TestTaskEventsStreamKeepsOpenAfterInitialNonTerminalReplay 验证订阅补拉到非终态历史事件后继续等待后续事件。
func TestTaskEventsStreamKeepsOpenAfterInitialNonTerminalReplay(t *testing.T) {
	store := newMemoryTaskStore()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	store.tasks["task-stream-live"] = model.Task{
		ID:        "task-stream-live",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		UpdatedAt: base,
	}
	store.events = append(store.events,
		model.TaskEvent{ID: 1, TaskID: "task-stream-live", EventType: "TASK_CREATED", Payload: `{"progress":0}`, CreatedAt: base},
	)
	handler := NewHandler(Config{
		Version:   "0.1.0",
		Token:     "test-token",
		DBStatus:  "not_configured",
		Ready:     true,
		TaskStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/events/stream", strings.NewReader(`{"task_id":"task-stream-live","after_event_id":0}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatalf("stream closed after non-terminal replay: %q", recorder.Body.String())
	default:
	}
	store.appendEvent(model.TaskEvent{
		ID:        2,
		TaskID:    "task-stream-live",
		EventType: "TASK_SUCCESS",
		Payload:   `{"progress":100}`,
		CreatedAt: base.Add(time.Minute),
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not finish after terminal event")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "id: 1\n") || !strings.Contains(body, "id: 2\n") {
		t.Fatalf("expected replay and terminal events, got %q", body)
	}
}

// TestAnalysisTaskCreatePersistsPendingTaskAndCreatedEvent 验证分析任务创建 API 写入 PENDING 任务和安全创建事件。
func TestAnalysisTaskCreatePersistsPendingTaskAndCreatedEvent(t *testing.T) {
	store := newMemoryTaskStore()
	executor := newRecordingAnalysisExecutor()
	handler := NewHandler(Config{
		Version:          "0.1.0",
		Token:            "test-token",
		DBStatus:         "not_configured",
		Ready:            true,
		AnalysisStore:    store,
		AnalysisExecutor: executor,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/analysis/tasks",
		strings.NewReader(`{
			"symbol":"us:aapl",
			"analysis_type":"stock_full",
			"ai_config_id":7,
			"prompt_template_id":9,
			"user_position":{"cost_price":123.45,"shares":10,"risk_level":"medium"},
			"resolved_api_key":"sk-runtime-secret"
		}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-analysis-create")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	taskID, ok := data["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected task id in response: %#v", data)
	}
	created, ok := store.tasks[taskID]
	if !ok || created.Status != "PENDING" || created.Type != "ANALYSIS" || !strings.Contains(created.Title, "US:AAPL") {
		t.Fatalf("unexpected created task: %+v", created)
	}
	if len(store.events) != 1 || store.events[0].TaskID != taskID || store.events[0].EventType != "TASK_CREATED" {
		t.Fatalf("expected TASK_CREATED event, got %+v", store.events)
	}
	if strings.Contains(store.events[0].Payload, "sk-runtime-secret") ||
		strings.Contains(store.events[0].Payload, "cost_price") {
		t.Fatalf("analysis event leaked sensitive input: %s", store.events[0].Payload)
	}
	call := executor.wait(t)
	if call.taskID != taskID || call.resolvedAPIKey != "sk-runtime-secret" || call.request.Symbol.String() != "US:AAPL" {
		t.Fatalf("unexpected executor call: %+v", call)
	}
}

// TestAnalysisTaskCancelPersistsCancelledTaskAndEvent 验证取消 API 将非终态任务切换为 CANCELLED。
func TestAnalysisTaskCancelPersistsCancelledTaskAndEvent(t *testing.T) {
	store := newMemoryTaskStore()
	store.tasks["task-running"] = model.Task{
		ID:        "task-running",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		Title:     "US:AAPL stock_full",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		AnalysisStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel", strings.NewReader(`{"task_id":"task-running"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	cancelled := store.tasks["task-running"]
	if cancelled.Status != "CANCELLED" || cancelled.FinishedAt.IsZero() {
		t.Fatalf("expected cancelled task, got %+v", cancelled)
	}
	if len(store.events) != 1 || store.events[0].TaskID != "task-running" || store.events[0].EventType != "TASK_CANCELLED" {
		t.Fatalf("expected TASK_CANCELLED event, got %+v", store.events)
	}
}

// TestAnalysisTaskCancelCancelsRunningExecutorContext 验证取消 API 会中断正在运行的分析执行上下文。
func TestAnalysisTaskCancelCancelsRunningExecutorContext(t *testing.T) {
	store := newMemoryTaskStore()
	executor := newBlockingAnalysisExecutor()
	handler := NewHandler(Config{
		Version:          "0.1.0",
		Token:            "test-token",
		DBStatus:         "not_configured",
		Ready:            true,
		AnalysisStore:    store,
		AnalysisExecutor: executor,
	})

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/analysis/tasks",
		strings.NewReader(`{
			"symbol":"US:AAPL",
			"analysis_type":"stock_full",
			"ai_config_id":7,
			"prompt_template_id":9,
			"resolved_api_key":"sk-runtime-secret"
		}`),
	)
	createRequest.Header.Set("X-Invest-Compass-Token", "test-token")
	createRequest.Header.Set("X-Request-Id", "req-running-cancel")

	handler.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d, body %s", http.StatusOK, createRecorder.Code, createRecorder.Body.String())
	}
	taskID := "analysis-req-running-cancel"
	runningContext := executor.waitContext(t)

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel", strings.NewReader(`{"task_id":"analysis-req-running-cancel"}`))
	cancelRequest.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(cancelRecorder, cancelRequest)

	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("expected cancel status %d, got %d, body %s", http.StatusOK, cancelRecorder.Code, cancelRecorder.Body.String())
	}
	select {
	case <-runningContext.Done():
	case <-time.After(time.Second):
		t.Fatalf("expected running executor context for %s to be cancelled", taskID)
	}
}

// TestReportsAPIListsGetsAndSoftDeletes 验证报告 API 隐藏软删除报告且默认不回显输入快照。
func TestReportsAPIListsGetsAndSoftDeletes(t *testing.T) {
	store := newMemoryReportStore()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	store.items[1] = model.AnalysisReport{
		ID:              1,
		TaskID:          "task-visible",
		Symbol:          "US:AAPL",
		Title:           "苹果分析",
		AnalysisType:    "stock_full",
		ModelName:       "gpt-test",
		InputSnapshot:   `{"userPosition":"满仓"}`,
		ContentMarkdown: "正文内容",
		RiskSummary:     "风险摘要",
		CreatedAt:       base,
		UpdatedAt:       base.Add(time.Hour),
	}
	store.items[2] = model.AnalysisReport{
		ID:              2,
		TaskID:          "task-deleted",
		Symbol:          "US:MSFT",
		Title:           "已删报告",
		AnalysisType:    "technical",
		ContentMarkdown: "不应出现",
		CreatedAt:       base,
		UpdatedAt:       base,
	}
	deleteTime := base.Add(2 * time.Hour)
	store.deleted[2] = &deleteTime

	handler := NewHandler(Config{
		Version:     "0.1.0",
		Token:       "test-token",
		DBStatus:    "not_configured",
		Ready:       true,
		ReportStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/reports/list", strings.NewReader(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	listData := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := listData["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one visible report, got %#v", listData)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != float64(1) || first["title"] != "苹果分析" {
		t.Fatalf("unexpected report list item: %#v", items[0])
	}
	if _, exists := first["input_snapshot"]; exists {
		t.Fatalf("report list must not expose input_snapshot: %#v", first)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/reports/get", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	detail := decodeResponseData(t, recorder.Body.Bytes())
	if detail["content_markdown"] != "正文内容" || strings.Contains(recorder.Body.String(), "满仓") {
		t.Fatalf("unexpected report detail payload: %#v", detail)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/reports/delete", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/reports/get", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted report get status %d, got %d, body %s", http.StatusNotFound, recorder.Code, recorder.Body.String())
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

	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal provider status response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected provider status envelope: %+v", response)
	}
	statuses, ok := response.Data.([]any)
	if !ok || len(statuses) != 2 {
		t.Fatalf("expected market and news provider status, got %#v", response.Data)
	}
	statusByName := statusListByName(t, statuses)
	marketStatus := statusByName["demo-provider"]
	if marketStatus["source"] != "demo-source" || marketStatus["available"] != false {
		t.Fatalf("unexpected market provider status data: %#v", marketStatus)
	}
	newsStatus := statusByName["news-provider"]
	if newsStatus["source"] != "unconfigured" || newsStatus["available"] != false {
		t.Fatalf("unexpected news provider status data: %#v", newsStatus)
	}
}

// TestProviderStatusReturnsUnconfiguredStatus 验证未配置真实行情 Provider 时状态接口说真话，而不是返回假数据或 503。
func TestProviderStatusReturnsUnconfiguredStatus(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "ok",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal provider status response: %v", err)
	}
	statuses, ok := response.Data.([]any)
	if !ok || len(statuses) != 2 {
		t.Fatalf("expected market and news unconfigured provider status, got %#v", response.Data)
	}
	statusByName := statusListByName(t, statuses)
	for _, name := range []string{"market-provider", "news-provider"} {
		status := statusByName[name]
		if status["available"] != false || status["source"] != "unconfigured" {
			t.Fatalf("unexpected unconfigured provider status for %s: %#v", name, status)
		}
	}
}

// TestProviderStatusUsesNewsProviderStatus 验证新闻数据源状态来自 Provider 自身，不能因 Provider 非空就假定可用。
func TestProviderStatusUsesNewsProviderStatus(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "ok",
		Ready:    true,
		NewsProvider: fakeNewsProvider{
			status: newsservice.ProviderStatus{
				Name:      "news-demo",
				Source:    "licensed-feed",
				Available: false,
				LastError: "Authorization: Bearer news-sensitive-value",
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/providers/status", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "news-sensitive-value") {
		t.Fatalf("news provider status leaked secret: %s", recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal provider status response: %v", err)
	}
	statuses, ok := response.Data.([]any)
	if !ok || len(statuses) != 2 {
		t.Fatalf("expected market and news provider status, got %#v", response.Data)
	}
	newsStatus := statusListByName(t, statuses)["news-demo"]
	if newsStatus["source"] != "licensed-feed" || newsStatus["available"] != false {
		t.Fatalf("unexpected news provider status data: %#v", newsStatus)
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
			ProviderStatuses: []dashboard.ProviderStatus{{
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

	var response httpx.Response
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

// TestDashboardSummaryReadsRealStoreData 验证 Dashboard summary 从真实 DAO 聚合数据，而不是返回静态空状态。
func TestDashboardSummaryReadsRealStoreData(t *testing.T) {
	store := newActionTestStore(t)
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	if err := store.SaveWatchlist(context.Background(), &model.Watchlist{
		Symbol:    "CN:SH:600519",
		SortOrder: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save watchlist: %v", err)
	}
	if err := store.SaveQuote(context.Background(), &model.Quote{
		Symbol:        "CN:SH:600519",
		Price:         1688,
		ChangePercent: 2.5,
		QuoteTime:     now,
		Provider:      "real-cache",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("save quote: %v", err)
	}
	if err := store.SaveQuote(context.Background(), &model.Quote{
		Symbol:        "CN:SH:000001",
		Price:         12,
		ChangePercent: -3.1,
		QuoteTime:     now,
		Provider:      "real-cache",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("save non-watchlist quote: %v", err)
	}
	if err := store.SaveAnalysisReportByTaskID(context.Background(), &model.AnalysisReport{
		TaskID:          "task-dashboard",
		Symbol:          "CN:SH:600519",
		Title:           "茅台分析",
		AnalysisType:    "stock_full",
		ContentMarkdown: "报告正文",
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save report: %v", err)
	}
	if err := store.SaveTask(context.Background(), &model.Task{
		ID:        "task-dashboard",
		Type:      "ANALYSIS",
		Status:    "SUCCESS",
		Title:     "分析任务",
		Progress:  100,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := store.SaveNewsItems(context.Background(), []model.NewsItem{{
		Source:      "news-provider",
		Title:       "市场新闻",
		URL:         "https://example.com/news",
		Summary:     "摘要",
		ContentHash: "dashboard-news",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}); err != nil {
		t.Fatalf("save news: %v", err)
	}

	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "ok",
		Ready:          true,
		DashboardStore: store,
		MarketProvider: fakeMarketProvider{status: market.ProviderStatus{
			Name:      "real-provider",
			Source:    "licensed-source",
			Available: true,
		}},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/dashboard/summary", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	watchlist, ok := data["watchlist"].(map[string]any)
	if !ok {
		t.Fatalf("expected watchlist summary object, got %#v", data["watchlist"])
	}
	if watchlist["up_count"] != float64(1) || watchlist["down_count"] != float64(0) || watchlist["flat_count"] != float64(0) {
		t.Fatalf("dashboard must summarize only active watchlist quotes, got %#v", watchlist)
	}
	if reports, ok := data["recent_reports"].([]any); !ok || len(reports) != 1 {
		t.Fatalf("expected one recent report from store, got %#v", data["recent_reports"])
	}
	if tasks, ok := data["recent_tasks"].([]any); !ok || len(tasks) != 1 {
		t.Fatalf("expected one recent task from store, got %#v", data["recent_tasks"])
	}
	if newsItems, ok := data["market_news"].([]any); !ok || len(newsItems) != 1 {
		t.Fatalf("expected one market news item from store, got %#v", data["market_news"])
	}
	statuses, ok := data["provider_statuses"].([]any)
	if !ok || len(statuses) != 2 {
		t.Fatalf("expected provider status from market provider, got %#v", data["provider_statuses"])
	}
	statusByName := statusListByName(t, statuses)
	if statusByName["real-provider"]["source"] != "licensed-source" || statusByName["real-provider"]["available"] != true {
		t.Fatalf("expected real market provider status, got %#v", statusByName["real-provider"])
	}
	if statusByName["news-provider"]["source"] != "unconfigured" || statusByName["news-provider"]["available"] != false {
		t.Fatalf("expected unconfigured news provider status, got %#v", statusByName["news-provider"])
	}
}

// TestDashboardSummaryIncludesUnconfiguredProviderStatus 验证 Dashboard 在未配置 Provider 时仍返回明确不可用状态。
func TestDashboardSummaryIncludesUnconfiguredProviderStatus(t *testing.T) {
	store := newActionTestStore(t)
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "ok",
		Ready:          true,
		DashboardStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/dashboard/summary", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	statuses, ok := data["provider_statuses"].([]any)
	if !ok || len(statuses) != 2 {
		t.Fatalf("expected market and news unconfigured provider status, got %#v", data["provider_statuses"])
	}
	statusByName := statusListByName(t, statuses)
	for _, name := range []string{"market-provider", "news-provider"} {
		status := statusByName[name]
		if status["available"] != false || status["source"] != "unconfigured" {
			t.Fatalf("unexpected dashboard provider status for %s: %#v", name, status)
		}
	}
}

// TestCacheStatsOnlyReturnsTemporaryCaches 验证缓存统计 API 不返回报告和配置。
func TestCacheStatsOnlyReturnsTemporaryCaches(t *testing.T) {
	statsProvider := &recordingCacheStatsProvider{
		usages: []settings.CacheUsage{
			{Target: settings.CacheTargetQuote, Bytes: 10},
			{Target: settings.CacheTargetNews, Bytes: 20},
			{Target: settings.CacheTargetReport, Bytes: 300},
			{Target: settings.CacheTargetConfig, Bytes: 400},
		},
	}
	handler := NewHandler(Config{
		Version:            "0.1.0",
		Token:              "test-token",
		DBStatus:           "not_configured",
		Ready:              true,
		CacheStatsProvider: statsProvider,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/cache/stats", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	payload := recorder.Body.String()
	if strings.Contains(payload, string(settings.CacheTargetReport)) || strings.Contains(payload, string(settings.CacheTargetConfig)) {
		t.Fatalf("cache stats exposed protected targets: %s", payload)
	}
	if !strings.Contains(payload, "total_bytes") {
		t.Fatalf("cache stats must use stable snake_case json fields: %s", payload)
	}

	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal cache stats response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected cache stats object, got %#v", response.Data)
	}
	if data["total_bytes"] != float64(30) {
		t.Fatalf("unexpected cache total bytes: %#v", data)
	}
	if statsProvider.calls != 1 {
		t.Fatalf("expected cache stats provider to be called once, got %d", statsProvider.calls)
	}
}

// TestCacheCleanOnlyCleansTemporaryTargets 验证缓存清理 API 只把临时缓存目标交给 cleaner。
func TestCacheCleanOnlyCleansTemporaryTargets(t *testing.T) {
	cleaner := &recordingCacheCleaner{}
	handler := NewHandler(Config{
		Version:      "0.1.0",
		Token:        "test-token",
		DBStatus:     "not_configured",
		Ready:        true,
		CacheCleaner: cleaner,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/cache/clean",
		strings.NewReader(`{"targets":["quote","report","config","news"]}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(cleaner.cleanedTargets) != 2 ||
		cleaner.cleanedTargets[0] != settings.CacheTargetQuote ||
		cleaner.cleanedTargets[1] != settings.CacheTargetNews {
		t.Fatalf("unexpected cleaned targets: %+v", cleaner.cleanedTargets)
	}
}

// TestUpdateCheckRejectsInsecureManifestURL 验证检查更新入口只允许 HTTPS manifest。
func TestUpdateCheckRejectsInsecureManifestURL(t *testing.T) {
	fetcher := &recordingUpdateManifestFetcher{}
	handler := NewHandler(Config{
		Version:               "0.1.0",
		Token:                 "test-token",
		DBStatus:              "not_configured",
		Ready:                 true,
		UpdateManifestURL:     "http://updates.example.com/invest-compass.json",
		UpdateAllowedHosts:    []string{"updates.example.com"},
		UpdateManifestFetcher: fetcher,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/update/check", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), string(xerr.UpdateInsecureURL))
	if fetcher.calls != 0 {
		t.Fatalf("insecure manifest URL must not be fetched, got %d calls", fetcher.calls)
	}
}

// TestUpdateCheckFetchesManifestAndReturnsPromptOnlyResult 验证检查更新只返回首版提示结果，不下载或安装。
func TestUpdateCheckFetchesManifestAndReturnsPromptOnlyResult(t *testing.T) {
	fetcher := &recordingUpdateManifestFetcher{
		content: []byte(`{
			"version":"0.2.0",
			"download_url":"https://updates.example.com/download",
			"release_notes_url":"https://updates.example.com/notes"
		}`),
	}
	handler := NewHandler(Config{
		Version:               "0.1.0",
		Token:                 "test-token",
		DBStatus:              "not_configured",
		Ready:                 true,
		UpdateManifestURL:     "https://updates.example.com/invest-compass.json",
		UpdateAllowedHosts:    []string{"updates.example.com"},
		UpdateManifestFetcher: fetcher,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/update/check", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if fetcher.calls != 1 || fetcher.lastURL != "https://updates.example.com/invest-compass.json" {
		t.Fatalf("unexpected fetcher state: calls=%d url=%q", fetcher.calls, fetcher.lastURL)
	}

	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal update check response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected update result object, got %#v", response.Data)
	}
	if data["has_new_version"] != true ||
		data["current_version"] != "0.1.0" ||
		data["latest_version"] != "0.2.0" ||
		data["action"] != string(updatecheck.ActionPromptOnly) {
		t.Fatalf("unexpected update result: %#v", data)
	}
	if data["download_url"] != "https://updates.example.com/download" ||
		data["release_notes_url"] != "https://updates.example.com/notes" {
		t.Fatalf("unexpected update links: %#v", data)
	}
}

// TestUpdateCheckUsesSettingsConfigSource 验证检查更新可从 settings 表读取 manifest URL 和 allowlist。
func TestUpdateCheckUsesSettingsConfigSource(t *testing.T) {
	fetcher := &recordingUpdateManifestFetcher{
		content: []byte(`{
			"version":"0.2.0",
			"download_url":"https://updates.example.com/download",
			"release_notes_url":"https://updates.example.com/notes"
		}`),
	}
	settingsStore := newMemorySettingsStore()
	if err := settingsStore.UpsertSetting(context.Background(), model.Setting{Key: "update.manifest_url", Value: "https://updates.example.com/invest-compass.json"}); err != nil {
		t.Fatalf("save manifest url setting: %v", err)
	}
	if err := settingsStore.UpsertSetting(context.Background(), model.Setting{Key: "update.allowed_hosts", Value: "updates.example.com,mirror.example.com"}); err != nil {
		t.Fatalf("save allowed host setting: %v", err)
	}
	handler := NewHandler(Config{
		Version:               "0.1.0",
		Token:                 "test-token",
		DBStatus:              "not_configured",
		Ready:                 true,
		SettingsStore:         settingsStore,
		UpdateManifestFetcher: fetcher,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/update/check", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if fetcher.calls != 1 || fetcher.lastURL != "https://updates.example.com/invest-compass.json" {
		t.Fatalf("unexpected fetcher state: calls=%d url=%q", fetcher.calls, fetcher.lastURL)
	}
}

// TestUpdateCheckRejectsManifestLinksOutsideAllowlist 验证 manifest 内链接仍必须命中 allowlist。
func TestUpdateCheckRejectsManifestLinksOutsideAllowlist(t *testing.T) {
	fetcher := &recordingUpdateManifestFetcher{
		content: []byte(`{
			"version":"0.2.0",
			"download_url":"https://evil.example.com/download"
		}`),
	}
	handler := NewHandler(Config{
		Version:               "0.1.0",
		Token:                 "test-token",
		DBStatus:              "not_configured",
		Ready:                 true,
		UpdateManifestURL:     "https://updates.example.com/invest-compass.json",
		UpdateAllowedHosts:    []string{"updates.example.com"},
		UpdateManifestFetcher: fetcher,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/update/check", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), string(xerr.UpdateHostNotAllowed))
}

// TestLogExportReturnsRedactedTroubleshootingBundle 验证日志导出 API 返回二次脱敏后的排障包。
func TestLogExportReturnsRedactedTroubleshootingBundle(t *testing.T) {
	source := &recordingLogExportSource{
		request: logexport.Request{
			Lines: []string{
				`level=INFO request_id=req-1 trace_id=trace-1 task_id=task-1 message=started`,
				`Authorization: Bearer demo-sensitive-value`,
				`user_position=100 shares at private cost`,
			},
			CreatedAt: time.Date(2026, 6, 18, 10, 11, 12, 0, time.UTC),
		},
	}
	handler := NewHandler(Config{
		Version:         "0.1.0",
		Token:           "test-token",
		DBStatus:        "not_configured",
		Ready:           true,
		LogExportSource: source,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/logs/export", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if source.calls != 1 {
		t.Fatalf("expected log export source to be called once, got %d", source.calls)
	}
	payload := recorder.Body.String()
	if strings.Contains(payload, "demo-sensitive-value") || strings.Contains(payload, "private cost") {
		t.Fatalf("log export leaked sensitive content: %s", payload)
	}
	if !strings.Contains(payload, "invest-compass-logs-20260618-101112.txt") ||
		!strings.Contains(payload, logger.RedactedValue) ||
		!strings.Contains(payload, "request_id=req-1") ||
		!strings.Contains(payload, "trace_id=trace-1") ||
		!strings.Contains(payload, "task_id=task-1") {
		t.Fatalf("unexpected log export payload: %s", payload)
	}
}

// TestLogExportRejectsMissingTroubleshootingFields 验证日志导出必须包含全局 request_id 和 trace_id。
func TestLogExportRejectsMissingTroubleshootingFields(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		LogExportSource: &recordingLogExportSource{
			request: logexport.Request{
				Lines: []string{`level=INFO request_id=req-1 message=missing trace`},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/logs/export", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), string(xerr.LogExportMissingTroubleshootingField))
}

// TestAIConfigSaveRejectsRawAPIKey 验证 Go core 拒绝保存真实 API Key。
func TestAIConfigSaveRejectsRawAPIKey(t *testing.T) {
	store := newMemoryAIConfigStore()
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		AIConfigStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/configs/save",
		strings.NewReader(`{
			"name":"OpenAI",
			"provider":"openai-compatible",
			"base_url":"https://api.example.com/v1",
			"model_name":"gpt-4.1",
			"raw_api_key":"sk-real-secret"
		}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), string(xerr.AIRawAPIKeyNotAllowed))
	if len(store.items) != 0 {
		t.Fatalf("raw API key request must not be saved: %+v", store.items)
	}
}

// TestAIConfigSaveListAndDeleteUseSafeMetadata 验证 AI 配置 API 只保存和返回凭据元数据。
func TestAIConfigSaveListAndDeleteUseSafeMetadata(t *testing.T) {
	store := newMemoryAIConfigStore()
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		AIConfigStore: store,
	})

	saveRecorder := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/configs/save",
		strings.NewReader(`{
			"name":"OpenAI",
			"provider":"openai-compatible",
			"base_url":"https://api.example.com/v1/",
			"api_key_ref":"local-vault://ai-config/openai-compatible-1",
			"masked_api_key":"sk-****7890",
			"has_api_key":true,
			"model_name":"gpt-4.1",
			"temperature":0.2,
			"max_tokens":2048,
			"timeout_seconds":60,
			"stream_enabled":true,
			"is_default":true
		}`),
	)
	saveRequest.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(saveRecorder, saveRequest)

	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("expected save status %d, got %d, body %s", http.StatusOK, saveRecorder.Code, saveRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodPost, "/api/ai/configs/list", strings.NewReader("{}"))
	listRequest.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d, body %s", http.StatusOK, listRecorder.Code, listRecorder.Body.String())
	}
	payload := listRecorder.Body.String()
	if strings.Contains(payload, "raw_api_key") ||
		strings.Contains(payload, "resolved_api_key") ||
		strings.Contains(payload, "sk-real-secret") {
		t.Fatalf("AI config list leaked secret fields: %s", payload)
	}
	if !strings.Contains(payload, `"base_url":"https://api.example.com/v1"`) ||
		!strings.Contains(payload, `"has_api_key":true`) ||
		!strings.Contains(payload, `"masked_api_key":"sk-****7890"`) {
		t.Fatalf("AI config list missing safe metadata: %s", payload)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/ai/configs/delete", strings.NewReader(`{"id":1}`))
	deleteRequest.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d, body %s", http.StatusOK, deleteRecorder.Code, deleteRecorder.Body.String())
	}

	afterDeleteRecorder := httptest.NewRecorder()
	afterDeleteRequest := httptest.NewRequest(http.MethodPost, "/api/ai/configs/list", strings.NewReader("{}"))
	afterDeleteRequest.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(afterDeleteRecorder, afterDeleteRequest)

	if afterDeleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected list after delete status %d, got %d", http.StatusOK, afterDeleteRecorder.Code)
	}
	if strings.Contains(afterDeleteRecorder.Body.String(), "OpenAI") {
		t.Fatalf("deleted AI config must be hidden from list: %s", afterDeleteRecorder.Body.String())
	}
}

// TestAIConfigTestUsesResolvedKeyWithoutLeakingSecret 验证模型测试只在运行期使用密钥且响应不泄露明文。
func TestAIConfigTestUsesResolvedKeyWithoutLeakingSecret(t *testing.T) {
	store := newMemoryAIConfigStore()
	tester := &recordingAIConfigTester{}
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		AIConfigStore:  store,
		AIConfigTester: tester,
	})
	config := model.AIConfig{
		ID:             11,
		Name:           "OpenAI",
		Provider:       aiservice.ProviderOpenAICompatible,
		BaseURL:        "https://api.example.com/v1",
		APIKeyRef:      "local-vault://ai-config/openai-compatible-11",
		MaskedAPIKey:   "sk-****3456",
		HasAPIKey:      true,
		ModelName:      "gpt-4.1-mini",
		Temperature:    0.2,
		MaxTokens:      128,
		TimeoutSeconds: 30,
		StreamEnabled:  false,
	}
	if err := store.SaveAIConfig(context.Background(), &config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/configs/test",
		strings.NewReader(`{"id":11,"resolved_api_key":"sk-runtime-secret-123456"}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if tester.calls != 1 || tester.config.ID != 11 || tester.resolvedAPIKey != "sk-runtime-secret-123456" {
		t.Fatalf("unexpected tester call: %+v", tester)
	}
	payload := recorder.Body.String()
	if strings.Contains(payload, "sk-runtime-secret") ||
		strings.Contains(payload, "resolved_api_key") ||
		strings.Contains(payload, "Authorization") {
		t.Fatalf("AI config test leaked secret: %s", payload)
	}
	if !strings.Contains(payload, `"ok":true`) || !strings.Contains(payload, `"provider":"openai-compatible"`) {
		t.Fatalf("AI config test missing safe result fields: %s", payload)
	}
}

// TestAIConfigTestRedactsProviderError 验证模型测试失败时不把 Provider 错误中的密钥写入响应。
func TestAIConfigTestRedactsProviderError(t *testing.T) {
	store := newMemoryAIConfigStore()
	tester := &recordingAIConfigTester{err: errors.New("Authorization: Bearer sk-runtime-secret failed")}
	handler := NewHandler(Config{
		Version:        "0.1.0",
		Token:          "test-token",
		DBStatus:       "not_configured",
		Ready:          true,
		AIConfigStore:  store,
		AIConfigTester: tester,
	})
	if err := store.SaveAIConfig(context.Background(), &model.AIConfig{
		ID:             12,
		Name:           "OpenAI",
		Provider:       aiservice.ProviderOpenAICompatible,
		BaseURL:        "https://api.example.com/v1",
		APIKeyRef:      "local-vault://ai-config/openai-compatible-12",
		MaskedAPIKey:   "sk-****3456",
		HasAPIKey:      true,
		ModelName:      "gpt-4.1-mini",
		TimeoutSeconds: 30,
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/configs/test",
		strings.NewReader(`{"id":12,"resolved_api_key":"sk-runtime-secret"}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadGateway, recorder.Code, recorder.Body.String())
	}
	payload := recorder.Body.String()
	if strings.Contains(payload, "sk-runtime-secret") || strings.Contains(payload, "Authorization") {
		t.Fatalf("AI config test error leaked secret: %s", payload)
	}
	assertErrorEnvelope(t, payload, string(xerr.AIUpstream))
}

// TestSettingsSetAndGetPersistValues 验证 settings API 可以保存并读取非敏感配置。
func TestSettingsSetAndGetPersistValues(t *testing.T) {
	store := newMemorySettingsStore()
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		SettingsStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/settings/set",
		strings.NewReader(`{"items":[{"key":"theme","value":"dark"},{"key":"proxy_url","value":"https://proxy.example:8080"}]}`),
	)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/settings/get", strings.NewReader(`{"keys":["theme","missing"]}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one settings item, got %#v", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["key"] != "theme" || first["value"] != "dark" {
		t.Fatalf("unexpected settings item: %#v", items[0])
	}
}

// TestSettingsSetRejectsSensitivePlaintext 验证 settings API 不允许保存敏感明文。
func TestSettingsSetRejectsSensitivePlaintext(t *testing.T) {
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		SettingsStore: newMemorySettingsStore(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/settings/set", strings.NewReader(`{"items":[{"key":"api_key","value":"sk-sensitive"}]}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), "sensitive_setting")
}

// TestSettingsSetRejectsCredentialsInDocumentedProxyURLKeys 验证实际代理 URL key 不能绕过凭据 vault 边界。
func TestSettingsSetRejectsCredentialsInDocumentedProxyURLKeys(t *testing.T) {
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		SettingsStore: newMemorySettingsStore(),
	})

	for _, key := range []string{"proxy.http_url", "proxy.socks5_url"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/settings/set",
			strings.NewReader(`{"items":[{"key":"`+key+`","value":"http://user:pass@proxy.example:8080"}]}`),
		)
		request.Header.Set("X-Invest-Compass-Token", "test-token")

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for %s, got %d, body %s", http.StatusBadRequest, key, recorder.Code, recorder.Body.String())
		}
		assertErrorEnvelope(t, recorder.Body.String(), string(xerr.SettingsProxyCredentialInURL))
	}
}

// TestWorkspaceSetAndGetValidateAbsolutePath 验证 workspace API 只接受绝对路径并能读回。
func TestWorkspaceSetAndGetValidateAbsolutePath(t *testing.T) {
	store := newMemorySettingsStore()
	handler := NewHandler(Config{
		Version:       "0.1.0",
		Token:         "test-token",
		DBStatus:      "not_configured",
		Ready:         true,
		SettingsStore: store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspace/set", strings.NewReader(`{"path":"relative/workspace"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertErrorEnvelope(t, recorder.Body.String(), "invalid_workspace_path")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/workspace/set", strings.NewReader(`{"path":"/Users/demo/InvestCompass"}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/workspace/get", strings.NewReader(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeResponseData(t, recorder.Body.Bytes())
	if data["path"] != "/Users/demo/InvestCompass" {
		t.Fatalf("unexpected workspace data: %#v", data)
	}
}

// assertErrorEnvelope 校验错误响应必须包含统一 envelope 和追踪 ID。
func assertErrorEnvelope(t *testing.T, body string, message string) {
	t.Helper()

	var response httpx.Response
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

// decodeResponseData 解析统一响应中的 data 对象。
func decodeResponseData(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var response httpx.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data object, got %#v", response.Data)
	}
	return data
}

// statusListByName 将 Provider 状态响应按 name 建索引，方便断言多数据源状态。
func statusListByName(t *testing.T, statuses []any) map[string]map[string]any {
	t.Helper()

	result := make(map[string]map[string]any, len(statuses))
	for _, item := range statuses {
		status, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected provider status object, got %T", item)
		}
		name, ok := status["name"].(string)
		if !ok || name == "" {
			t.Fatalf("provider status missing name: %#v", status)
		}
		result[name] = status
	}
	return result
}

type fakeMarketProvider struct {
	searchResults []market.StockBasic
	status        market.ProviderStatus
	quoteResult   market.Quote
	quoteCalls    *int
	klineResults  []market.KlineBar
	klineCalls    *int
}

type fakeNewsProvider struct {
	listItems   []newsservice.Item
	listCalls   *int
	marketItems []newsservice.Item
	marketCalls *int
	status      newsservice.ProviderStatus
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

// Quote 返回测试预置的行情快照并记录调用次数。
func (provider fakeMarketProvider) Quote(context.Context, stock.Symbol) (market.Quote, error) {
	if provider.quoteCalls != nil {
		(*provider.quoteCalls)++
	}
	return provider.quoteResult, nil
}

// Kline 返回测试预置的 K 线并记录调用次数。
func (provider fakeMarketProvider) Kline(context.Context, market.KlineRequest) ([]market.KlineBar, error) {
	if provider.klineCalls != nil {
		(*provider.klineCalls)++
	}
	return provider.klineResults, nil
}

// Name 返回测试新闻 Provider 名称。
func (provider fakeNewsProvider) Name() string {
	return "fake-news"
}

// Status 返回测试新闻 Provider 状态。
func (provider fakeNewsProvider) Status(context.Context) newsservice.ProviderStatus {
	if provider.status.Name != "" {
		return provider.status
	}
	return newsservice.ProviderStatus{Name: provider.Name(), Source: provider.Name(), Available: true}
}

// List 返回测试预置的个股新闻并记录调用次数。
func (provider fakeNewsProvider) List(context.Context, newsservice.ListRequest) ([]newsservice.Item, error) {
	if provider.listCalls != nil {
		(*provider.listCalls)++
	}
	return provider.listItems, nil
}

// Market 返回测试预置的市场新闻并记录调用次数。
func (provider fakeNewsProvider) Market(context.Context, newsservice.MarketRequest) ([]newsservice.Item, error) {
	if provider.marketCalls != nil {
		(*provider.marketCalls)++
	}
	return provider.marketItems, nil
}

type recordingCacheCleaner struct {
	cleanedTargets []settings.CacheTarget
}

type memoryStockStore struct {
	saved []model.Stock
	jobs  []model.SearchIndexJob
}

type memoryMarketStore struct {
	quotes map[string]model.Quote
	klines map[string]model.Kline
}

type memoryNewsStore struct {
	items map[string]model.NewsItem
}

type memoryWatchlistStore struct {
	nextID int64
	items  map[int64]model.Watchlist
}

type memoryPromptTemplateStore struct {
	nextID int64
	items  map[int64]model.PromptTemplate
}

type memoryAIConfigStore struct {
	nextID int64
	items  map[int64]model.AIConfig
}

type recordingAIConfigTester struct {
	calls          int
	config         aiservice.Config
	resolvedAPIKey string
	result         aiservice.TestResult
	err            error
}

type analysisExecutorCall struct {
	taskID         string
	request        analysisservice.ValidatedCreateRequest
	resolvedAPIKey string
}

type recordingAnalysisExecutor struct {
	calls chan analysisExecutorCall
}

type blockingAnalysisExecutor struct {
	contexts chan context.Context
}

// TestAIConfig 记录运行期密钥注入调用，避免 action 单测访问外部 Provider。
func (tester *recordingAIConfigTester) TestAIConfig(_ context.Context, config aiservice.Config, resolvedAPIKey string) (aiservice.TestResult, error) {
	tester.calls++
	tester.config = config
	tester.resolvedAPIKey = resolvedAPIKey
	if tester.result.Provider == "" {
		tester.result = aiservice.TestResult{
			OK:       true,
			Provider: config.Provider,
			Model:    config.ModelName,
			Message:  "ok",
		}
	}
	return tester.result, tester.err
}

// newRecordingAnalysisExecutor 创建记录分析任务执行触发的测试替身。
func newRecordingAnalysisExecutor() *recordingAnalysisExecutor {
	return &recordingAnalysisExecutor{calls: make(chan analysisExecutorCall, 1)}
}

// newBlockingAnalysisExecutor 创建等待 context 取消的分析执行器替身。
func newBlockingAnalysisExecutor() *blockingAnalysisExecutor {
	return &blockingAnalysisExecutor{contexts: make(chan context.Context, 1)}
}

// Execute 记录异步执行器调用，不访问真实 Provider。
func (executor *recordingAnalysisExecutor) Execute(_ context.Context, taskID string, request analysisservice.ValidatedCreateRequest, resolvedAPIKey string) error {
	executor.calls <- analysisExecutorCall{taskID: taskID, request: request, resolvedAPIKey: resolvedAPIKey}
	return nil
}

// Execute 保持运行直到 context 取消，用于验证取消信号传播到执行器。
func (executor *blockingAnalysisExecutor) Execute(ctx context.Context, _ string, _ analysisservice.ValidatedCreateRequest, _ string) error {
	executor.contexts <- ctx
	<-ctx.Done()
	return ctx.Err()
}

// wait 等待异步执行器触发，避免测试依赖调度时序。
func (executor *recordingAnalysisExecutor) wait(t *testing.T) analysisExecutorCall {
	t.Helper()
	select {
	case call := <-executor.calls:
		return call
	case <-time.After(time.Second):
		t.Fatal("analysis executor was not called")
		return analysisExecutorCall{}
	}
}

// waitContext 等待阻塞执行器启动并返回其运行上下文。
func (executor *blockingAnalysisExecutor) waitContext(t *testing.T) context.Context {
	t.Helper()
	select {
	case ctx := <-executor.contexts:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("analysis executor was not started")
		return context.Background()
	}
}

type memoryTaskStore struct {
	mutex  sync.Mutex
	tasks  map[string]model.Task
	events []model.TaskEvent
}

type memoryReportStore struct {
	items   map[int64]model.AnalysisReport
	deleted map[int64]*time.Time
}

// UpsertStocks 记录搜索后写入的股票基础信息缓存。
func (store *memoryStockStore) UpsertStocks(_ context.Context, stocks []model.Stock) error {
	store.saved = append(store.saved, stocks...)
	return nil
}

// ListStocksForSearch 在路由测试中默认不提供本地股票命中，触发 Provider fallback。
func (store *memoryStockStore) ListStocksForSearch(context.Context, string, []string, int) ([]model.Stock, error) {
	return nil, nil
}

// ListStockAliasesBySymbols 返回空别名集合。
func (store *memoryStockStore) ListStockAliasesBySymbols(_ context.Context, symbols []string) (map[string][]model.StockAlias, error) {
	result := make(map[string][]model.StockAlias, len(symbols))
	for _, symbol := range symbols {
		result[symbol] = nil
	}
	return result, nil
}

// ListActiveWatchlists 返回空自选股集合。
func (store *memoryStockStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return nil, nil
}

// GetSearchIndexState 在路由测试中默认表示尚无 active FTS batch。
func (store *memoryStockStore) GetSearchIndexState(context.Context, string) (string, bool, error) {
	return "", false, nil
}

// SearchStockFTS 在路由测试中默认无 FTS 命中。
func (store *memoryStockStore) SearchStockFTS(context.Context, string, string, int) ([]dao.StockSearchFTSMatch, error) {
	return nil, nil
}

// UpsertSearchIndexJob 记录 Provider fallback 后的股票增量索引任务。
func (store *memoryStockStore) UpsertSearchIndexJob(_ context.Context, job model.SearchIndexJob) error {
	store.jobs = append(store.jobs, job)
	return nil
}

// newMemoryMarketStore 创建 actions 测试使用的内存行情缓存 store。
func newMemoryMarketStore() *memoryMarketStore {
	return &memoryMarketStore{
		quotes: make(map[string]model.Quote),
		klines: make(map[string]model.Kline),
	}
}

// LatestQuote 返回未超过最大缓存年龄的测试 quote。
func (store *memoryMarketStore) LatestQuote(_ context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error) {
	quote, ok := store.quotes[symbol]
	if !ok {
		return model.Quote{}, false, nil
	}
	if quote.UpdatedAt.IsZero() || time.Since(quote.UpdatedAt) > maxAge {
		return model.Quote{}, false, nil
	}
	return quote, true, nil
}

// SaveQuote 保存测试 quote 缓存。
func (store *memoryMarketStore) SaveQuote(_ context.Context, quote *model.Quote) error {
	now := time.Now().UTC()
	if quote.UpdatedAt.IsZero() {
		quote.UpdatedAt = now
	}
	if quote.CreatedAt.IsZero() {
		quote.CreatedAt = now
	}
	store.quotes[quote.Symbol] = *quote
	return nil
}

// ListKlines 按交易日升序返回测试 K 线缓存。
func (store *memoryMarketStore) ListKlines(_ context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error) {
	items := make([]model.Kline, 0, len(store.klines))
	for _, item := range store.klines {
		if item.Symbol == symbol && item.Period == period && item.Adjust == adjust {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].TradeDate < items[right].TradeDate
	})
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

// SaveKlines 保存测试 K 线缓存。
func (store *memoryMarketStore) SaveKlines(_ context.Context, klines []model.Kline) error {
	for _, item := range klines {
		store.klines[item.Symbol+"|"+item.Period+"|"+item.Adjust+"|"+item.TradeDate] = item
	}
	return nil
}

// newMemoryNewsStore 创建 actions 测试使用的内存新闻缓存 store。
func newMemoryNewsStore() *memoryNewsStore {
	return &memoryNewsStore{items: make(map[string]model.NewsItem)}
}

// SaveNewsItems 按 content_hash 保存测试新闻缓存。
func (store *memoryNewsStore) SaveNewsItems(_ context.Context, items []model.NewsItem) error {
	now := time.Now().UTC()
	for _, item := range items {
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = now
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		store.items[item.ContentHash] = item
	}
	return nil
}

// ListNewsBySymbol 返回包含目标 symbol 的测试新闻缓存。
func (store *memoryNewsStore) ListNewsBySymbol(_ context.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error) {
	items := make([]model.NewsItem, 0, len(store.items))
	for _, item := range store.items {
		if maxAge > 0 && time.Since(item.UpdatedAt) > maxAge {
			continue
		}
		if strings.Contains(item.Symbols, symbol) {
			items = append(items, item)
		}
	}
	sortNewsModels(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// ListMarketNews 返回测试市场新闻缓存。
func (store *memoryNewsStore) ListMarketNews(_ context.Context, _ string, limit int, maxAge time.Duration) ([]model.NewsItem, error) {
	items := make([]model.NewsItem, 0, len(store.items))
	for _, item := range store.items {
		if maxAge > 0 && time.Since(item.UpdatedAt) > maxAge {
			continue
		}
		items = append(items, item)
	}
	sortNewsModels(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// sortNewsModels 按发布时间倒序整理测试新闻缓存。
func sortNewsModels(items []model.NewsItem) {
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].PublishedAt.After(items[right].PublishedAt)
	})
}

// newMemoryWatchlistStore 创建 actions 测试使用的内存自选股 store。
func newMemoryWatchlistStore() *memoryWatchlistStore {
	return &memoryWatchlistStore{nextID: 1, items: make(map[int64]model.Watchlist)}
}

// SaveWatchlist 保存或更新测试自选股。
func (store *memoryWatchlistStore) SaveWatchlist(_ context.Context, item *model.Watchlist) error {
	if item.ID == 0 {
		item.ID = store.nextID
		store.nextID++
	}
	store.items[item.ID] = *item
	return nil
}

// ListActiveWatchlists 返回当前测试自选股。
func (store *memoryWatchlistStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	items := make([]model.Watchlist, 0, len(store.items))
	for _, item := range store.items {
		items = append(items, item)
	}
	return items, nil
}

// SoftDeleteWatchlist 从测试 store 中移除自选股，模拟软删除后的 active 列表行为。
func (store *memoryWatchlistStore) SoftDeleteWatchlist(_ context.Context, id int64) error {
	delete(store.items, id)
	return nil
}

// newMemoryPromptTemplateStore 创建 actions 测试使用的内存 Prompt 模板 store。
func newMemoryPromptTemplateStore() *memoryPromptTemplateStore {
	return &memoryPromptTemplateStore{nextID: 1, items: make(map[int64]model.PromptTemplate)}
}

// SavePromptTemplate 保存或更新测试 Prompt 模板。
func (store *memoryPromptTemplateStore) SavePromptTemplate(_ context.Context, item *model.PromptTemplate) error {
	if item.ID == 0 {
		item.ID = store.nextID
		store.nextID++
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	store.items[item.ID] = *item
	return nil
}

// ListPromptTemplates 返回未软删除的测试 Prompt 模板。
func (store *memoryPromptTemplateStore) ListPromptTemplates(context.Context) ([]model.PromptTemplate, error) {
	items := make([]model.PromptTemplate, 0, len(store.items))
	for _, item := range store.items {
		items = append(items, item)
	}
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].UpdatedAt.After(items[right].UpdatedAt)
	})
	return items, nil
}

// SoftDeletePromptTemplate 从测试 store 中移除 Prompt 模板，模拟软删除后的 active 列表行为。
func (store *memoryPromptTemplateStore) SoftDeletePromptTemplate(_ context.Context, id int64) error {
	delete(store.items, id)
	return nil
}

// newMemoryAIConfigStore 创建 actions 测试使用的内存 AI 配置 store。
func newMemoryAIConfigStore() *memoryAIConfigStore {
	return &memoryAIConfigStore{nextID: 1, items: make(map[int64]model.AIConfig)}
}

// SaveAIConfig 保存或更新测试 AI 配置元数据。
func (store *memoryAIConfigStore) SaveAIConfig(_ context.Context, item *model.AIConfig) error {
	if item.ID == 0 {
		item.ID = store.nextID
		store.nextID++
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = time.Now().UTC()
	store.items[item.ID] = *item
	return nil
}

// ListAIConfigs 返回未软删除的测试 AI 配置。
func (store *memoryAIConfigStore) ListAIConfigs(context.Context) ([]model.AIConfig, error) {
	items := make([]model.AIConfig, 0, len(store.items))
	for _, item := range store.items {
		items = append(items, item)
	}
	sort.SliceStable(items, func(left int, right int) bool {
		if items[left].IsDefault != items[right].IsDefault {
			return items[left].IsDefault
		}
		return items[left].UpdatedAt.After(items[right].UpdatedAt)
	})
	return items, nil
}

// GetAIConfig 返回指定 AI 配置，供模型连通性测试读取安全元数据。
func (store *memoryAIConfigStore) GetAIConfig(_ context.Context, id int64) (model.AIConfig, error) {
	item, ok := store.items[id]
	if !ok {
		return model.AIConfig{}, errors.New("ai config not found")
	}
	return item, nil
}

// SoftDeleteAIConfig 从测试 store 中移除 AI 配置，模拟软删除后的列表行为。
func (store *memoryAIConfigStore) SoftDeleteAIConfig(_ context.Context, id int64) error {
	delete(store.items, id)
	return nil
}

// newMemoryTaskStore 创建 actions 测试使用的内存任务 store。
func newMemoryTaskStore() *memoryTaskStore {
	return &memoryTaskStore{tasks: make(map[string]model.Task)}
}

// SaveTask 保存或更新测试任务。
func (store *memoryTaskStore) SaveTask(_ context.Context, task *model.Task) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.tasks[task.ID] = *task
	return nil
}

// ListTasks 按更新时间倒序返回测试任务历史。
func (store *memoryTaskStore) ListTasks(_ context.Context, limit int) ([]model.Task, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	items := make([]model.Task, 0, len(store.tasks))
	for _, item := range store.tasks {
		items = append(items, item)
	}
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].UpdatedAt.After(items[right].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// GetTask 按 ID 返回测试任务详情。
func (store *memoryTaskStore) GetTask(_ context.Context, taskID string) (model.Task, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	item, ok := store.tasks[taskID]
	return item, ok, nil
}

// AppendTaskEvent 追加测试任务事件并模拟自增 ID。
func (store *memoryTaskStore) AppendTaskEvent(_ context.Context, event *model.TaskEvent) error {
	store.appendEvent(*event)
	return nil
}

// appendEvent 追加测试事件，供并发 SSE 测试模拟后台任务写入。
func (store *memoryTaskStore) appendEvent(event model.TaskEvent) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if event.ID == 0 {
		event.ID = int64(len(store.events) + 1)
	}
	store.events = append(store.events, event)
}

// ListTaskEventsAfter 按 ID 增量返回测试任务事件。
func (store *memoryTaskStore) ListTaskEventsAfter(_ context.Context, taskID string, afterID int64) ([]model.TaskEvent, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	items := make([]model.TaskEvent, 0, len(store.events))
	for _, item := range store.events {
		if item.TaskID == taskID && item.ID > afterID {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].ID < items[right].ID
	})
	return items, nil
}

// newMemoryReportStore 创建 actions 测试使用的内存报告 store。
func newMemoryReportStore() *memoryReportStore {
	return &memoryReportStore{
		items:   make(map[int64]model.AnalysisReport),
		deleted: make(map[int64]*time.Time),
	}
}

// ListVisibleAnalysisReports 返回未软删除的测试报告。
func (store *memoryReportStore) ListVisibleAnalysisReports(context.Context) ([]model.AnalysisReport, error) {
	items := make([]model.AnalysisReport, 0, len(store.items))
	for id, item := range store.items {
		if store.deleted[id] != nil {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].UpdatedAt.After(items[right].UpdatedAt)
	})
	return items, nil
}

// SoftDeleteAnalysisReport 对测试报告执行软删除。
func (store *memoryReportStore) SoftDeleteAnalysisReport(_ context.Context, id int64) error {
	now := time.Now().UTC()
	store.deleted[id] = &now
	return nil
}

type recordingCacheStatsProvider struct {
	usages []settings.CacheUsage
	calls  int
}

type recordingUpdateManifestFetcher struct {
	content []byte
	err     error
	calls   int
	lastURL string
}

type recordingLogExportSource struct {
	request logexport.Request
	err     error
	calls   int
}

type fakeRootDocumentSearchService struct{}

// SearchReports 返回空报告搜索结果，供根路由注册测试使用。
func (service *fakeRootDocumentSearchService) SearchReports(context.Context, searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	return nil, nil
}

// SearchNews 返回空资讯搜索结果，供根路由注册测试使用。
func (service *fakeRootDocumentSearchService) SearchNews(context.Context, searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	return nil, nil
}

// SearchWatchlistNotes 返回空自选备注搜索结果，供根路由注册测试使用。
func (service *fakeRootDocumentSearchService) SearchWatchlistNotes(context.Context, searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	return nil, nil
}

// Status 返回空搜索索引状态，供根路由注册测试使用。
func (service *fakeRootDocumentSearchService) Status(context.Context) (searchservice.SearchStatus, error) {
	return searchservice.SearchStatus{FTS5Status: "AVAILABLE", SearchStatus: "READY"}, nil
}

// Rebuild 返回空搜索重建结果，供根路由注册测试使用。
func (service *fakeRootDocumentSearchService) Rebuild(context.Context, searchservice.SearchRebuildRequest) (searchservice.SearchRebuildAccepted, error) {
	return searchservice.SearchRebuildAccepted{}, nil
}

// CacheUsages 返回测试注入的缓存统计，并记录调用次数。
func (provider *recordingCacheStatsProvider) CacheUsages(context.Context) ([]settings.CacheUsage, error) {
	provider.calls++
	return provider.usages, nil
}

// CleanCache 记录被清理的缓存目标，供 server 测试断言。
func (cleaner *recordingCacheCleaner) CleanCache(_ context.Context, targets []settings.CacheTarget) error {
	cleaner.cleanedTargets = append([]settings.CacheTarget(nil), targets...)
	return nil
}

// FetchManifest 记录检查更新远程 JSON 请求，避免测试访问真实网络。
func (fetcher *recordingUpdateManifestFetcher) FetchManifest(_ context.Context, manifestURL string) ([]byte, error) {
	fetcher.calls++
	fetcher.lastURL = manifestURL
	return fetcher.content, fetcher.err
}

// ExportLogRequest 返回测试注入的日志导出输入，避免测试读取真实日志文件。
func (source *recordingLogExportSource) ExportLogRequest(context.Context) (logexport.Request, error) {
	source.calls++
	return source.request, source.err
}

// newActionTestStore 创建 actions 集成测试使用的真实 SQLite Store。
func newActionTestStore(t *testing.T) *dao.Store {
	t.Helper()

	db, err := dao.Open(context.Background(), dao.Config{Path: filepath.Join(t.TempDir(), "actions-test.db")})
	if err != nil {
		t.Fatalf("open actions sqlite: %v", err)
	}
	if err := dao.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate actions sqlite: %v", err)
	}
	store, err := dao.NewStore(db)
	if err != nil {
		t.Fatalf("new actions store: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	return store
}

type memorySettingsStore struct {
	items map[string]model.Setting
}

// newMemorySettingsStore 创建 actions 测试使用的内存 settings store。
func newMemorySettingsStore() *memorySettingsStore {
	return &memorySettingsStore{items: make(map[string]model.Setting)}
}

// UpsertSetting 保存或更新测试 settings。
func (store *memorySettingsStore) UpsertSetting(_ context.Context, item model.Setting) error {
	store.items[item.Key] = item
	return nil
}

// GetSettings 按 key 返回测试 settings。
func (store *memorySettingsStore) GetSettings(_ context.Context, keys []string) ([]model.Setting, error) {
	result := make([]model.Setting, 0, len(keys))
	for _, key := range keys {
		item, ok := store.items[key]
		if ok {
			result = append(result, item)
		}
	}
	return result, nil
}
