package stocks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestHandleSearchDelegatesToService 验证股票搜索 action 只委托 service，不直接访问 Provider。
func TestHandleSearchDelegatesToService(t *testing.T) {
	service := &fakeStockSearchService{
		results: []searchservice.StockSearchResult{{
			Symbol:   "CN:SH:600519",
			Name:     "贵州茅台",
			Code:     "600519",
			Market:   "CN",
			Exchange: "SH",
		}},
	}
	recorder := performStockSearch(t, Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"keyword":"茅台"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if service.keyword != "茅台" {
		t.Fatalf("expected service keyword, got %q", service.keyword)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	results := response.Data.([]any)
	first := results[0].(map[string]any)
	if first["symbol"] != "CN:SH:600519" || first["name"] != "贵州茅台" {
		t.Fatalf("unexpected response data: %#v", first)
	}
}

// TestHandleSearchMapsUnconfiguredProviderError 验证 service 返回未配置 Provider 时 action 保持既有 503 契约。
func TestHandleSearchMapsUnconfiguredProviderError(t *testing.T) {
	recorder := performStockSearch(t, Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  &fakeStockSearchService{err: &xerr.Error{Code: xerr.MarketProviderUnconfigured}},
	}, `{"keyword":"茅台"}`)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(xerr.MarketProviderUnconfigured)) {
		t.Fatalf("expected unconfigured error body, got %s", recorder.Body.String())
	}
}

// TestHandleSearchRequiresService 验证 service 未注入时不会伪造股票搜索能力。
func TestHandleSearchRequiresService(t *testing.T) {
	recorder := performStockSearch(t, Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
	}, `{"keyword":"茅台"}`)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	}
}

// TestHandleProfileReturnsStoredStockFields 验证个股资料接口只返回 stocks 表已有基础资料，不伪造公司字段。
func TestHandleProfileReturnsStoredStockFields(t *testing.T) {
	store := &fakeStockProfileStore{
		stock: model.Stock{
			Symbol:   "600000.SH",
			Name:     "浦发银行",
			Code:     "600000",
			Market:   "CN",
			Exchange: "SH",
			Industry: "银行",
			Concept:  `["低估值","大金融"]`,
			ListDate: "1999-11-10",
			Status:   "active",
			FullName: "上海浦东发展银行股份有限公司",
		},
	}
	recorder := performStockProfile(t, Config{
		Security:     httpx.SecurityConfig{Token: "test-token", Ready: true},
		ProfileStore: store,
	}, `{"symbol":"600000.SH"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if store.symbol != "600000.SH" {
		t.Fatalf("expected store symbol, got %q", store.symbol)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data := response.Data.(map[string]any)
	if data["name"] != "浦发银行" || data["industry"] != "银行" || data["full_name"] != "上海浦东发展银行股份有限公司" {
		t.Fatalf("unexpected profile response: %#v", data)
	}
	concepts := data["concepts"].([]any)
	if len(concepts) != 2 || concepts[0] != "低估值" || concepts[1] != "大金融" {
		t.Fatalf("unexpected concepts: %#v", concepts)
	}
}

// TestHandleProfileReturnsNotFound 验证本地 stocks 表没有资料时明确返回错误态。
func TestHandleProfileReturnsNotFound(t *testing.T) {
	recorder := performStockProfile(t, Config{
		Security:     httpx.SecurityConfig{Token: "test-token", Ready: true},
		ProfileStore: &fakeStockProfileStore{},
	}, `{"symbol":"600000.SH"}`)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("stock_profile_not_found")) {
		t.Fatalf("expected not found error body, got %s", recorder.Body.String())
	}
}

// performStockSearch 使用固定 token 执行股票搜索 action，复用统一安全校验路径。
func performStockSearch(t *testing.T, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/search", bytes.NewReader([]byte(body)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[0].Handler(recorder, request)
	return recorder
}

// performStockProfile 使用固定 token 执行个股资料 action，复用统一安全校验路径。
func performStockProfile(t *testing.T, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/profile", bytes.NewReader([]byte(body)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[1].Handler(recorder, request)
	return recorder
}

type fakeStockSearchService struct {
	results []searchservice.StockSearchResult
	err     error
	keyword string
	limit   int
}

// Search 记录 action 传入的关键词和 limit，并返回预置结果或错误。
func (service *fakeStockSearchService) Search(_ context.Context, keyword string, limit int) ([]searchservice.StockSearchResult, error) {
	service.keyword = keyword
	service.limit = limit
	if service.err != nil {
		return nil, service.err
	}
	return service.results, nil
}

type fakeStockProfileStore struct {
	stock  model.Stock
	exists bool
	symbol string
}

// GetStockBySymbol 记录 action 查询的 symbol，并返回预置股票资料。
func (store *fakeStockProfileStore) GetStockBySymbol(_ context.Context, symbol string) (model.Stock, bool, error) {
	store.symbol = symbol
	if store.stock.Symbol != "" {
		return store.stock, true, nil
	}
	return model.Stock{}, store.exists, nil
}

var _ = errors.New
