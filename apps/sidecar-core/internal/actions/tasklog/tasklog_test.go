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

func (service *fakeService) List(_ context.Context, query tasklogservice.Query) (tasklogservice.ListResult, error) {
	service.listQuery = query
	if service.listErr != nil {
		return tasklogservice.ListResult{}, service.listErr
	}
	return tasklogservice.ListResult{Rows: []tasklogservice.Row{{ID: 7, TaskID: query.TaskID, Level: "INFO"}}}, nil
}

func (service *fakeService) Get(_ context.Context, _ int64) (tasklogservice.Detail, bool, error) {
	return service.detail, service.detailOK, nil
}

func (service *fakeService) Summary(_ context.Context, _ string) (tasklogservice.Summary, bool, error) {
	return service.summary, service.summaryOK, nil
}

func (service *fakeService) Diagnosis(_ context.Context, taskID string) (tasklogservice.Diagnosis, bool, error) {
	return tasklogservice.Diagnosis{TaskID: taskID, Summary: "暂无错误诊断"}, false, nil
}

func (service *fakeService) Context(_ context.Context, taskID string) (tasklogservice.ContextSummary, bool, error) {
	return tasklogservice.ContextSummary{TaskID: taskID}, true, nil
}

func (service *fakeService) Export(_ context.Context, taskID string) (tasklogservice.ExportBundle, error) {
	return tasklogservice.ExportBundle{FileName: taskID + ".txt", Content: "safe log"}, nil
}

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

func TestGetTaskLogRejectsInvalidID(t *testing.T) {
	recorder := perform(t, handleGet(testConfig(&fakeService{})), map[string]any{"id": 0})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestGetTaskLogReturnsNotFound(t *testing.T) {
	recorder := perform(t, handleGet(testConfig(&fakeService{})), map[string]any{"id": 99})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

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

func TestListTaskLogsRequiresReadyToken(t *testing.T) {
	service := &fakeService{}
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"task_id":"task_1","limit":10}`)))
	recorder := httptest.NewRecorder()

	handleList(testConfig(service))(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", recorder.Code)
	}
}

func TestListTaskLogsRejectsWhenNotReady(t *testing.T) {
	config := testConfig(&fakeService{})
	config.Security.Ready = false
	recorder := perform(t, handleList(config), map[string]any{"task_id": "task_1", "limit": 10})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when core is not ready, got %d", recorder.Code)
	}
}

func TestSummaryRequiresTaskID(t *testing.T) {
	recorder := perform(t, handleSummary(testConfig(&fakeService{})), map[string]any{"task_id": ""})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

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

func testConfig(service Service) Config {
	return Config{Security: httpx.SecurityConfig{Ready: true, Token: "token"}, Service: service}
}

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
