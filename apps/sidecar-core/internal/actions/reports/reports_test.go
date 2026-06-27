package reports

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

// TestHandleGetReturnsStructuredInputSnapshot 验证报告详情返回结构化输入快照，
// 既保留模型和模板审计字段，又不回显一次性持仓明细。
func TestHandleGetReturnsStructuredInputSnapshot(t *testing.T) {
	store := &batchDeleteStore{
		reports: []model.AnalysisReport{
			{
				ID:               3,
				TaskID:           "task-report-3",
				Symbol:           "CN:SH:600519",
				Title:            "贵州茅台 个股综合分析",
				AnalysisType:     "stock_full",
				ModelName:        "gpt-analysis",
				PromptTemplateID: 9,
				InputSnapshot:    `{"symbol":"CN:SH:600519","prompt_template":"综合分析","model":"gpt-analysis","temperature":0.2,"max_tokens":2048,"user_position":{"cost_price":123.45},"raw_prompt":"User:\n请结合 user_position 成本价 123.45 分析"}`,
				ContentMarkdown:  "# 报告",
				CreatedAt:        time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/reports/get", bytes.NewBufferString(`{"id":3}`))
	request.Header.Set("X-Invest-Compass-Token", "token")

	handleGet(Config{
		Security: httpx.SecurityConfig{Token: "token", Ready: true},
		Store:    store,
	})(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data reportData `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	snapshot, ok := envelope.Data.InputSnapshot.(map[string]any)
	if !ok {
		t.Fatalf("input_snapshot must be structured object, got %#v", envelope.Data.InputSnapshot)
	}
	if snapshot["prompt_template"] != "综合分析" ||
		snapshot["model"] != "gpt-analysis" ||
		snapshot["temperature"] != 0.2 ||
		snapshot["max_tokens"] != float64(2048) {
		t.Fatalf("unexpected snapshot fields: %+v", snapshot)
	}
	if _, ok := snapshot["user_position"]; ok {
		t.Fatalf("detail response must not expose one-time position: %+v", snapshot)
	}
	if _, ok := snapshot["raw_prompt"]; ok {
		t.Fatalf("detail response must not expose raw prompt with one-time position: %+v", snapshot)
	}
	for _, forbidden := range [][]byte{[]byte("user_position"), []byte("123.45"), []byte("raw_prompt")} {
		if bytes.Contains(recorder.Body.Bytes(), forbidden) {
			t.Fatalf("detail response leaked private prompt fragment %q: %s", forbidden, recorder.Body.String())
		}
	}
}

// TestHandleListIncludesStockNameWithoutSnapshot 验证报告列表只补充非敏感股票名称，
// 不回显完整输入快照，避免一次性持仓等上下文进入列表响应。
func TestHandleListIncludesStockNameWithoutSnapshot(t *testing.T) {
	store := &batchDeleteStore{
		reports: []model.AnalysisReport{
			{
				ID:              7,
				TaskID:          "task-report-7",
				Symbol:          "CN:SH:600522",
				Title:           "CN:SH:600522 stock_full AI 分析报告",
				AnalysisType:    "stock_full",
				ModelName:       "gpt-analysis",
				InputSnapshot:   `{"stock_name":"中天科技","user_position":{"shares":100}}`,
				ContentMarkdown: "# 报告",
				CreatedAt:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
			},
		},
		stocks: map[string]model.Stock{
			"CN:SH:600522": {Symbol: "CN:SH:600522", Name: "中天科技"},
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/reports/list", bytes.NewBufferString(`{}`))
	request.Header.Set("X-Invest-Compass-Token", "token")

	handleList(Config{
		Security: httpx.SecurityConfig{Token: "token", Ready: true},
		Store:    store,
	})(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data listData `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].StockName != "中天科技" {
		t.Fatalf("list response should include stock name, got %+v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].InputSnapshot != nil || bytes.Contains(recorder.Body.Bytes(), []byte("user_position")) {
		t.Fatalf("list response must not expose input snapshot: %s", recorder.Body.String())
	}
}

// TestHandleBatchDeleteUsesStoreAtomicBatch 验证批量删除只调用 store 的原子批量边界，不逐条删除造成部分成功。
func TestHandleBatchDeleteUsesStoreAtomicBatch(t *testing.T) {
	store := &batchDeleteStore{
		reports: []model.AnalysisReport{
			{ID: 1, TaskID: "task-1", Symbol: "600000.SH", Title: "报告 1", CreatedAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)},
			{ID: 2, TaskID: "task-2", Symbol: "000001.SZ", Title: "报告 2", CreatedAt: time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)},
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/reports/batch-delete", bytes.NewBufferString(`{"ids":[1,2]}`))
	request.Header.Set("X-Invest-Compass-Token", "token")

	handleBatchDelete(Config{
		Security: httpx.SecurityConfig{Token: "token", Ready: true},
		Store:    store,
	})(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if len(store.batchIDs) != 2 || store.batchIDs[0] != 1 || store.batchIDs[1] != 2 {
		t.Fatalf("expected batch delete ids [1 2], got %+v", store.batchIDs)
	}
	if len(store.singleIDs) != 0 {
		t.Fatalf("batch delete must not call single delete path: %+v", store.singleIDs)
	}
}

type batchDeleteStore struct {
	reports   []model.AnalysisReport
	stocks    map[string]model.Stock
	batchIDs  []int64
	singleIDs []int64
}

// ListVisibleAnalysisReports 返回测试报告列表，模拟 action 校验可见性。
func (store *batchDeleteStore) ListVisibleAnalysisReports(context.Context) ([]model.AnalysisReport, error) {
	return append([]model.AnalysisReport(nil), store.reports...), nil
}

// GetStocksBySymbols 返回报告关联股票基础资料，模拟列表接口的名称补齐来源。
func (store *batchDeleteStore) GetStocksBySymbols(_ context.Context, symbols []string) (map[string]model.Stock, error) {
	result := make(map[string]model.Stock, len(symbols))
	for _, symbol := range symbols {
		if stock, ok := store.stocks[symbol]; ok {
			result[symbol] = stock
		}
	}
	return result, nil
}

// SoftDeleteAnalysisReport 记录逐条删除调用；批量删除路径不应调用它。
func (store *batchDeleteStore) SoftDeleteAnalysisReport(_ context.Context, id int64) error {
	store.singleIDs = append(store.singleIDs, id)
	return nil
}

// BatchSoftDeleteAnalysisReports 记录原子批量删除调用。
func (store *batchDeleteStore) BatchSoftDeleteAnalysisReports(_ context.Context, ids []int64) error {
	store.batchIDs = append([]int64(nil), ids...)
	return nil
}

// UpdateAnalysisReportFavorite 在批量删除测试中不使用。
func (store *batchDeleteStore) UpdateAnalysisReportFavorite(context.Context, int64, bool) error {
	return nil
}
