package reports

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

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
	batchIDs  []int64
	singleIDs []int64
}

// ListVisibleAnalysisReports 返回测试报告列表，模拟 action 校验可见性。
func (store *batchDeleteStore) ListVisibleAnalysisReports(context.Context) ([]model.AnalysisReport, error) {
	return append([]model.AnalysisReport(nil), store.reports...), nil
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
