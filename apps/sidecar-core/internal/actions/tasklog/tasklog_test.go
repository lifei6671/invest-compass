package tasklog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
)

type fakeService struct {
	listQuery tasklogservice.Query
	detail    tasklogservice.Detail
	detailOK  bool
	summary   tasklogservice.Summary
	summaryOK bool
	listErr   error
}

// List 记录列表查询参数并返回可断言的任务日志行。
func (service *fakeService) List(_ context.Context, query tasklogservice.Query) (tasklogservice.ListResult, error) {
	service.listQuery = query
	if service.listErr != nil {
		return tasklogservice.ListResult{}, service.listErr
	}
	return tasklogservice.ListResult{Rows: []tasklogservice.Row{{ID: 7, TaskID: query.TaskID, Level: "INFO"}}}, nil
}

// Get 返回测试预设的任务日志详情。
func (service *fakeService) Get(_ context.Context, _ int64) (tasklogservice.Detail, bool, error) {
	return service.detail, service.detailOK, nil
}

// Summary 返回测试预设的任务日志摘要。
func (service *fakeService) Summary(_ context.Context, _ string) (tasklogservice.Summary, bool, error) {
	return service.summary, service.summaryOK, nil
}

// Diagnosis 返回无需外部依赖的默认诊断内容。
func (service *fakeService) Diagnosis(_ context.Context, taskID string) (tasklogservice.Diagnosis, bool, error) {
	return tasklogservice.Diagnosis{TaskID: taskID, Summary: "暂无错误诊断"}, false, nil
}

// Context 返回指定任务的测试上下文摘要。
func (service *fakeService) Context(_ context.Context, taskID string) (tasklogservice.ContextSummary, bool, error) {
	return tasklogservice.ContextSummary{TaskID: taskID}, true, nil
}

// Export 返回指定任务的安全日志导出包。
func (service *fakeService) Export(_ context.Context, taskID string) (tasklogservice.ExportBundle, error) {
	return tasklogservice.ExportBundle{FileName: taskID + ".txt", Content: "safe log"}, nil
}

// TestListTaskLogsAppliesQuery 验证列表接口会把筛选条件传给 service。
func TestListTaskLogsAppliesQuery(t *testing.T) {
	service := &fakeService{}
	recorder := perform(t, handleList(testConfig(service)), map[string]any{
		"task_id":    "task_1",
		"level":      "ERROR",
		"module":     "ai",
		"stage":      "stream_failed",
		"keyword":    "timeout",
		"only_error": true,
		"after_id":   3,
		"limit":      50,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if service.listQuery.TaskID != "task_1" ||
		service.listQuery.Level != "ERROR" ||
		service.listQuery.Module != "ai" ||
		service.listQuery.Stage != "stream_failed" ||
		service.listQuery.Keyword != "timeout" ||
		!service.listQuery.OnlyError ||
		service.listQuery.AfterID != 3 ||
		service.listQuery.Limit != 50 {
		t.Fatalf("unexpected query: %+v", service.listQuery)
	}
}

// TestGetTaskLogRejectsInvalidID 验证日志详情接口拒绝非法 ID。
func TestGetTaskLogRejectsInvalidID(t *testing.T) {
	recorder := perform(t, handleGet(testConfig(&fakeService{})), map[string]any{"id": 0})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

// TestGetTaskLogReturnsNotFound 验证日志详情缺失时返回 not found。
func TestGetTaskLogReturnsNotFound(t *testing.T) {
	recorder := perform(t, handleGet(testConfig(&fakeService{})), map[string]any{"id": 99})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

// TestListTaskLogsRejectsInvalidQuery 验证列表接口拒绝非法分页条件。
func TestListTaskLogsRejectsInvalidQuery(t *testing.T) {
	recorder := perform(t, handleList(testConfig(&fakeService{})), map[string]any{
		"task_id":  "task_1",
		"after_id": -1,
		"limit":    10,
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid after_id, got %d", recorder.Code)
	}

	recorder = perform(t, handleList(testConfig(&fakeService{listErr: errors.New("invalid limit")})), map[string]any{
		"task_id": "task_1",
		"limit":   501,
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid limit, got %d", recorder.Code)
	}
}

// TestListTaskLogsRequiresReadyToken 验证任务日志查询必须携带 runtime token。
func TestListTaskLogsRequiresReadyToken(t *testing.T) {
	service := &fakeService{}
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"task_id":"task_1","limit":10}`)))
	recorder := httptest.NewRecorder()

	handleList(testConfig(service))(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", recorder.Code)
	}
}

// TestListTaskLogsRejectsWhenNotReady 验证 Go core 未 ready 时拒绝任务日志查询。
func TestListTaskLogsRejectsWhenNotReady(t *testing.T) {
	config := testConfig(&fakeService{})
	config.Security.Ready = false
	recorder := perform(t, handleList(config), map[string]any{"task_id": "task_1", "limit": 10})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when core is not ready, got %d", recorder.Code)
	}
}

// TestSummaryRequiresTaskID 验证任务摘要接口要求有效 task_id。
func TestSummaryRequiresTaskID(t *testing.T) {
	recorder := perform(t, handleSummary(testConfig(&fakeService{})), map[string]any{"task_id": ""})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

// TestExportReturnsSanitizedBundle 验证日志导出接口返回脱敏导出包。
func TestExportReturnsSanitizedBundle(t *testing.T) {
	recorder := perform(t, handleExport(testConfig(&fakeService{})), map[string]any{"task_id": "task_1"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var payload httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := payload.Data.(map[string]any)
	if !ok || data["file_name"] != "task_1.txt" {
		t.Fatalf("unexpected export payload: %#v", payload.Data)
	}
}

// testConfig 创建带 ready token 的任务日志 action 测试配置。
func testConfig(service Service) Config {
	return Config{Security: httpx.SecurityConfig{Ready: true, Token: "token"}, Service: service}
}

// perform 执行带 token 的 POST 请求并返回响应记录器。
func perform(t *testing.T, handler http.HandlerFunc, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set(httpx.TokenHeader, "token")
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}
