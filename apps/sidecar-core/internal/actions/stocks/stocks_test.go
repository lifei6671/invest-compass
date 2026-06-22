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

// performStockSearch 使用固定 token 执行股票搜索 action，复用统一安全校验路径。
func performStockSearch(t *testing.T, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/stocks/search", bytes.NewReader([]byte(body)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[0].Handler(recorder, request)
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

var _ = errors.New
