package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
)

// TestSearchReportsReturnsFixedScopeResults 验证报告搜索路由只调用 report 固定范围。
func TestSearchReportsReturnsFixedScopeResults(t *testing.T) {
	service := &fakeDocumentSearchActionService{
		results: []searchservice.DocumentSearchResult{{
			DocUID:     "report:1",
			DocType:    searchservice.SearchDocTypeReport,
			RefID:      "1",
			Symbol:     "CN:SH:600519",
			Title:      "贵州茅台报告",
			Summary:    "均线改善",
			Source:     "gpt-analysis",
			SourceTime: time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
			Score:      1,
			Highlights: []string{"均线改善"},
		}},
	}
	recorder := performDocumentSearch(t, "/api/search/reports", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"keyword":"茅台","symbols":["CN:SH:600519"],"limit":10,"offset":0}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if service.called != "reports" {
		t.Fatalf("expected reports scope, got %q", service.called)
	}
	if service.request.Keyword != "茅台" || len(service.request.Symbols) != 1 || service.request.Symbols[0] != "CN:SH:600519" {
		t.Fatalf("unexpected request passed to service: %+v", service.request)
	}
	data := decodeActionResponseData(t, recorder.Body.Bytes())
	items, ok := data.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one result item, got %#v", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["doc_type"] != searchservice.SearchDocTypeReport || first["doc_uid"] != "report:1" {
		t.Fatalf("unexpected report result: %#v", items[0])
	}
}

// TestSearchRejectsDocTypesField 验证前端不能通过 doc_types 传入任意范围组合。
func TestSearchRejectsDocTypesField(t *testing.T) {
	recorder := performDocumentSearch(t, "/api/search/news", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  &fakeDocumentSearchActionService{},
	}, `{"keyword":"茅台","doc_types":["report","news"]}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertActionErrorEnvelope(t, recorder.Body.String(), "invalid_json")
}

// TestSearchValidatesLimitAndOffset 验证菜单范围搜索 action 拒绝越界分页参数。
func TestSearchValidatesLimitAndOffset(t *testing.T) {
	recorder := performDocumentSearch(t, "/api/search/watchlist-notes", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  &fakeDocumentSearchActionService{},
	}, `{"keyword":"息差","limit":101,"offset":0}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertActionErrorEnvelope(t, recorder.Body.String(), "invalid_request")
}

// TestSearchWatchlistNotesUsesFixedScope 验证自选备注路由固定调用 watchlist_note 范围。
func TestSearchWatchlistNotesUsesFixedScope(t *testing.T) {
	service := &fakeDocumentSearchActionService{}
	recorder := performDocumentSearch(t, "/api/search/watchlist-notes", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"keyword":"息差","limit":20,"offset":0}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if service.called != "watchlist_notes" {
		t.Fatalf("expected watchlist notes scope, got %q", service.called)
	}
}

// TestSearchStatusReturnsIndexState 验证搜索状态 API 返回 active batch 和 FTS5 状态。
func TestSearchStatusReturnsIndexState(t *testing.T) {
	service := &fakeDocumentSearchActionService{
		status: searchservice.SearchStatus{
			FTS5Status:            "AVAILABLE",
			SearchStatus:          "READY",
			ActiveStockBatchID:    "stock-ready-1",
			ActiveDocumentBatchID: "doc-ready-1",
		},
	}
	recorder := performDocumentSearch(t, "/api/search/status", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeActionResponseData(t, recorder.Body.Bytes())
	status, ok := data.(map[string]any)
	if !ok || status["active_stock_batch_id"] != "stock-ready-1" || status["fts5_status"] != "AVAILABLE" {
		t.Fatalf("unexpected search status response: %#v", data)
	}
}

// TestSearchRebuildRejectsInvalidScope 验证重建 API 只接受固定 scope 白名单。
func TestSearchRebuildRejectsInvalidScope(t *testing.T) {
	service := &fakeDocumentSearchActionService{}
	recorder := performDocumentSearch(t, "/api/search/rebuild", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"scope":"global"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	assertActionErrorEnvelope(t, recorder.Body.String(), "invalid_scope")
	if service.rebuildCalled {
		t.Fatal("invalid scope must not call rebuild service")
	}
}

// TestSearchRebuildMapsAlreadyRunning 验证同一 scope 正在重建时返回冲突错误。
func TestSearchRebuildMapsAlreadyRunning(t *testing.T) {
	service := &fakeDocumentSearchActionService{rebuildErr: searchservice.ErrSearchRebuildAlreadyRunning}
	recorder := performDocumentSearch(t, "/api/search/rebuild", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"scope":"all"}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusConflict, recorder.Code, recorder.Body.String())
	}
	assertActionErrorEnvelope(t, recorder.Body.String(), "search_rebuild_already_running")
}

// TestSearchRebuildMapsFTS5Unavailable 验证 FTS5 不可用时重建 API 返回明确不可用错误。
func TestSearchRebuildMapsFTS5Unavailable(t *testing.T) {
	service := &fakeDocumentSearchActionService{rebuildErr: searchservice.ErrSearchFTS5Unavailable}
	recorder := performDocumentSearch(t, "/api/search/rebuild", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"scope":"all"}`)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	}
	assertActionErrorEnvelope(t, recorder.Body.String(), "search_fts5_unavailable")
}

// TestSearchRebuildReturnsTaskAndBatches 验证合法重建请求返回任务和 batch 指针。
func TestSearchRebuildReturnsTaskAndBatches(t *testing.T) {
	service := &fakeDocumentSearchActionService{
		rebuildResult: searchservice.SearchRebuildAccepted{
			TaskID:          "search-rebuild-1",
			Scope:           "all",
			StockBatchID:    "stock-ready-1",
			DocumentBatchID: "doc-ready-1",
		},
	}
	recorder := performDocumentSearch(t, "/api/search/rebuild", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Service:  service,
	}, `{"scope":"all"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if !service.rebuildCalled || service.rebuildScope != "all" {
		t.Fatalf("expected all scope rebuild, called=%v scope=%q", service.rebuildCalled, service.rebuildScope)
	}
	data := decodeActionResponseData(t, recorder.Body.Bytes())
	result, ok := data.(map[string]any)
	if !ok || result["task_id"] != "search-rebuild-1" || result["scope"] != "all" {
		t.Fatalf("unexpected rebuild response: %#v", data)
	}
}

// performDocumentSearch 执行指定搜索路由并返回响应记录器。
func performDocumentSearch(t *testing.T, path string, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	var handler http.HandlerFunc
	for _, route := range Routes(config) {
		if route.Path == path {
			handler = route.Handler
			break
		}
	}
	if handler == nil {
		t.Fatalf("route %s not found", path)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set(httpx.TokenHeader, "test-token")
	handler.ServeHTTP(recorder, request)
	return recorder
}

type fakeDocumentSearchActionService struct {
	called        string
	request       searchservice.DocumentSearchRequest
	results       []searchservice.DocumentSearchResult
	status        searchservice.SearchStatus
	rebuildCalled bool
	rebuildScope  string
	rebuildResult searchservice.SearchRebuildAccepted
	rebuildErr    error
}

// SearchReports 记录报告搜索请求并返回预置结果。
func (service *fakeDocumentSearchActionService) SearchReports(_ context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	service.called = "reports"
	service.request = request
	return service.results, nil
}

// SearchNews 记录资讯搜索请求并返回预置结果。
func (service *fakeDocumentSearchActionService) SearchNews(_ context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	service.called = "news"
	service.request = request
	return service.results, nil
}

// SearchWatchlistNotes 记录自选备注搜索请求并返回预置结果。
func (service *fakeDocumentSearchActionService) SearchWatchlistNotes(_ context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
	service.called = "watchlist_notes"
	service.request = request
	return service.results, nil
}

// Status 返回预置搜索状态。
func (service *fakeDocumentSearchActionService) Status(context.Context) (searchservice.SearchStatus, error) {
	return service.status, nil
}

// Rebuild 记录重建请求并返回预置结果或错误。
func (service *fakeDocumentSearchActionService) Rebuild(_ context.Context, request searchservice.SearchRebuildRequest) (searchservice.SearchRebuildAccepted, error) {
	service.rebuildCalled = true
	service.rebuildScope = request.Scope
	if service.rebuildErr != nil {
		return searchservice.SearchRebuildAccepted{}, service.rebuildErr
	}
	return service.rebuildResult, nil
}

// decodeActionResponseData 解析统一 envelope 的 data 字段。
func decodeActionResponseData(t *testing.T, body []byte) any {
	t.Helper()
	var response httpx.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Code != 0 {
		t.Fatalf("expected ok envelope, got %+v", response)
	}
	return response.Data
}

// assertActionErrorEnvelope 校验错误响应 message。
func assertActionErrorEnvelope(t *testing.T, body string, message string) {
	t.Helper()
	var response httpx.Response
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if response.Message != message {
		t.Fatalf("expected error %q, got %+v", message, response)
	}
}
