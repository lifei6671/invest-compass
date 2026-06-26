package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestHandleListReturnsBuiltinMetadata 验证列表 API 向前端暴露内置模板的稳定 key、版本、checksum 和锁定状态。
func TestHandleListReturnsBuiltinMetadata(t *testing.T) {
	store := &fakePromptStore{items: []model.PromptTemplate{{
		ID:            1,
		Key:           "builtin_stock_full",
		Name:          "个股综合分析",
		Type:          "stock_full",
		Description:   "内置综合模板",
		Content:       "分析 {{stock_name}}",
		Variables:     `["stock_name"]`,
		IsBuiltin:     true,
		BuiltinLocked: true,
		Version:       3,
		Checksum:      "sha256:abc",
		Source:        "builtin",
		CreatedAt:     time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC),
	}}}

	body := performPromptRoute(t, Routes(Config{Security: readySecurity(), Store: store})[0], `{}`)
	item := body["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
	if item["key"] != "builtin_stock_full" ||
		item["version"].(float64) != 3 ||
		item["checksum"] != "sha256:abc" ||
		item["builtin_locked"] != true ||
		item["source"] != "builtin" {
		t.Fatalf("unexpected prompt template data: %+v", item)
	}
}

// TestHandleUpdateRejectsBuiltinLockedTemplate 验证后端拒绝直接修改 locked 内置模板。
func TestHandleUpdateRejectsBuiltinLockedTemplate(t *testing.T) {
	store := &fakePromptStore{items: []model.PromptTemplate{{
		ID:            2,
		Key:           "builtin_technical",
		Name:          "技术面分析",
		Type:          "technical",
		Content:       "分析 {{stock_name}}",
		IsBuiltin:     true,
		BuiltinLocked: true,
		Source:        "builtin",
	}}}

	body := performPromptRoute(t, Routes(Config{Security: readySecurity(), Store: store})[3], `{"id":2,"name":"修改","type":"technical","description":"","content":"修改 {{stock_name}}"}`)
	if body["code"].(float64) != 40007 || body["message"] != "builtin_prompt_template_readonly" {
		t.Fatalf("expected readonly error, got %+v", body)
	}
	if store.saved {
		t.Fatal("locked builtin template must not be saved")
	}
}

// TestHandleDeleteRejectsBuiltinLockedTemplate 验证后端拒绝删除 locked 内置模板。
func TestHandleDeleteRejectsBuiltinLockedTemplate(t *testing.T) {
	store := &fakePromptStore{items: []model.PromptTemplate{{
		ID:            3,
		Key:           "builtin_news",
		Name:          "消息面分析",
		Type:          "news",
		Content:       "分析 {{stock_name}}",
		IsBuiltin:     true,
		BuiltinLocked: true,
		Source:        "builtin",
	}}}

	body := performPromptRoute(t, Routes(Config{Security: readySecurity(), Store: store})[4], `{"id":3}`)
	if body["code"].(float64) != 40007 || body["message"] != "builtin_prompt_template_readonly" {
		t.Fatalf("expected readonly error, got %+v", body)
	}
	if store.deletedID != 0 {
		t.Fatalf("locked builtin template must not be deleted, got id %d", store.deletedID)
	}
}

// performPromptRoute 执行 Prompt 路由并解析统一响应体，避免测试重复组装鉴权请求。
func performPromptRoute(t *testing.T, route httpx.Route, payload string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(route.Method, route.Path, bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	response := httptest.NewRecorder()

	route.Handler(response, request)

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v body=%s", err, response.Body.String())
	}
	return body
}

// readySecurity 返回已就绪的测试鉴权配置，覆盖 Prompt 路由的 token 门禁。
func readySecurity() httpx.SecurityConfig {
	return httpx.SecurityConfig{Token: "test-token", Ready: true}
}

type fakePromptStore struct {
	items     []model.PromptTemplate
	saved     bool
	deletedID int64
}

// SavePromptTemplate 记录保存调用，用于验证创建路径会把模板交给 store。
func (store *fakePromptStore) SavePromptTemplate(_ context.Context, template *model.PromptTemplate) error {
	store.saved = true
	store.items = append(store.items, *template)
	return nil
}

// ListPromptTemplates 返回测试模板快照，用于验证列表和删除前置校验。
func (store *fakePromptStore) ListPromptTemplates(_ context.Context) ([]model.PromptTemplate, error) {
	return append([]model.PromptTemplate(nil), store.items...), nil
}

// SoftDeletePromptTemplate 记录软删除目标，验证内置锁定模板不会被删除。
func (store *fakePromptStore) SoftDeletePromptTemplate(_ context.Context, id int64) error {
	store.deletedID = id
	return nil
}

var _ http.HandlerFunc
