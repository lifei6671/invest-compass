package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
)

// TestTriggerRunCreatesUserRequestAndEnqueues 验证用户手动抓取会借用调度执行队列。
func TestTriggerRunCreatesUserRequestAndEnqueues(t *testing.T) {
	store := &memoryStore{}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/runs/trigger", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/runs/trigger", strings.NewReader(`{
		"cron_type":"cn_a_share_quote_refresh",
		"scope_key":"CN:SH:600519",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 1 {
		t.Fatalf("expected one scheduler run, got %+v", store.createdRuns)
	}
	run := store.createdRuns[0]
	if run.TriggerType != schedulerservice.TriggerUserRequest || run.Status != schedulerservice.RunStatusQueued || run.Priority != 100 {
		t.Fatalf("unexpected user request run: %+v", run)
	}
	if run.ScopeKey != "CN:SH:600519" || run.TargetDate != "2026-06-19" {
		t.Fatalf("unexpected user request scope/date: %+v", run)
	}
	if len(queue.Snapshot()) != 1 || queue.Snapshot()[0].RunKey != run.RunKey {
		t.Fatalf("expected run to enter shared scheduler queue, got %+v", queue.Snapshot())
	}
}

// TestTriggerRunRejectsUnsupportedCronType 验证手动触发不会持久化 registry 外的调度类型。
func TestTriggerRunRejectsUnsupportedCronType(t *testing.T) {
	store := &memoryStore{}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/runs/trigger", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/runs/trigger", strings.NewReader(`{
		"cron_type":"unknown_refresh",
		"scope_key":"CN:SH:600519",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported cron_type to return 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 || len(queue.Snapshot()) != 0 {
		t.Fatalf("unsupported cron_type must not create or enqueue run: runs=%+v queue=%+v", store.createdRuns, queue.Snapshot())
	}
}

// TestTriggerRunReturnsServiceUnavailableWithoutQueue 验证手动抓取缺少执行队列时不创建悬空 run。
func TestTriggerRunReturnsServiceUnavailableWithoutQueue(t *testing.T) {
	store := &memoryStore{}
	handler := routeHandler(t, "/api/scheduler/runs/trigger", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/runs/trigger", strings.NewReader(`{
		"cron_type":"cn_a_share_quote_refresh",
		"scope_key":"CN:SH:600519",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing scheduler queue to return 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("queue unavailable must not create trigger run: %+v", store.createdRuns)
	}
}

// TestRefreshSymbolCreatesRunsForRequestedDataType 验证用户手动刷新单股会按 data_type 生成并入队。
func TestRefreshSymbolCreatesRunsForRequestedDataType(t *testing.T) {
	store := &memoryStore{}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/refresh-symbol", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/refresh-symbol", strings.NewReader(`{
		"symbol":"CN:SH:600519",
		"data_type":"quote",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 1 {
		t.Fatalf("expected one refresh-symbol run, got %+v", store.createdRuns)
	}
	run := store.createdRuns[0]
	if run.CronType != schedulerservice.CronTypeCNAShareQuoteRefresh || run.DataType != "quote" || run.TriggerType != schedulerservice.TriggerUserRequest {
		t.Fatalf("unexpected refresh-symbol run: %+v", run)
	}
	if len(queue.Snapshot()) != 1 || queue.Snapshot()[0].RunKey != run.RunKey {
		t.Fatalf("expected refresh-symbol run to enter queue, got %+v", queue.Snapshot())
	}
}

// TestRefreshSymbolAllCreatesQuoteKlineAndNewsRuns 验证 all 会为同一股票生成 quote、kline、news 三类 run。
func TestRefreshSymbolAllCreatesQuoteKlineAndNewsRuns(t *testing.T) {
	store := &memoryStore{}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/refresh-symbol", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/refresh-symbol", strings.NewReader(`{
		"symbol":"CN:SH:600519",
		"data_type":"all",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 3 {
		t.Fatalf("expected quote/kline/news runs, got %+v", store.createdRuns)
	}
	gotDataTypes := []string{store.createdRuns[0].DataType, store.createdRuns[1].DataType, store.createdRuns[2].DataType}
	if strings.Join(gotDataTypes, ",") != "quote,kline,news" {
		t.Fatalf("unexpected data types: %+v", gotDataTypes)
	}
}

// TestRefreshSymbolDuplicateActiveScopeReturnsExistingRun 验证重复手动刷新会复用已有 active run。
func TestRefreshSymbolDuplicateActiveScopeReturnsExistingRun(t *testing.T) {
	store := &memoryStore{}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/refresh-symbol", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	body := `{"symbol":"CN:SH:600519","data_type":"quote","target_date":"2026-06-19"}`
	for index := 0; index < 2; index++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/scheduler/refresh-symbol", strings.NewReader(body))
		request.Header.Set(httpx.TokenHeader, "test-token")
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
		}
	}

	if len(queue.Snapshot()) != 1 {
		t.Fatalf("duplicate active scope must only enqueue once, got %+v", queue.Snapshot())
	}
	if len(store.createdRuns) != 1 {
		t.Fatalf("duplicate active scope must only create one run, got %+v", store.createdRuns)
	}
	if len(store.updatedRuns) != 0 {
		t.Fatalf("duplicate active scope must return existing run without skipped update, got %+v", store.updatedRuns)
	}
}

// TestRefreshSymbolReturnsExistingActiveRun 验证重复单股刷新会返回已有 queued/running run，而不是创建 skipped run。
func TestRefreshSymbolReturnsExistingActiveRun(t *testing.T) {
	store := &memoryStore{
		runs: []model.SchedulerRun{{
			ID:          77,
			CronType:    schedulerservice.CronTypeCNAShareQuoteRefresh,
			DataType:    "quote",
			ScopeKey:    "CN:SH:600519",
			TargetDate:  "2026-06-19",
			RunKey:      "user_request:quote:existing",
			TriggerType: schedulerservice.TriggerUserRequest,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    100,
			Source:      "user_request",
		}},
	}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/refresh-symbol", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/refresh-symbol", strings.NewReader(`{
		"symbol":"CN:SH:600519",
		"data_type":"quote",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate refresh-symbol to return existing run, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("duplicate refresh-symbol must not create new run: %+v", store.createdRuns)
	}
	if len(queue.Snapshot()) != 0 {
		t.Fatalf("duplicate existing refresh-symbol run must not enqueue again, got %+v", queue.Snapshot())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object response data, got %T", response.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one existing refresh-symbol item, got %#v", data["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["id"] != float64(77) || item["run_key"] != "user_request:quote:existing" {
		t.Fatalf("unexpected existing refresh-symbol item: %#v", items[0])
	}
}

// TestRefreshSymbolReturnsServiceUnavailableWithoutQueue 验证单股手动刷新缺少执行队列时返回 503。
func TestRefreshSymbolReturnsServiceUnavailableWithoutQueue(t *testing.T) {
	store := &memoryStore{}
	handler := routeHandler(t, "/api/scheduler/refresh-symbol", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/refresh-symbol", strings.NewReader(`{
		"symbol":"CN:SH:600519",
		"data_type":"quote",
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing scheduler queue to return 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("queue unavailable must not create refresh-symbol run: %+v", store.createdRuns)
	}
}

// TestRunNowCreatesUserRequestFromJob 验证长期 job 的立即执行会生成 user_request run 并复用队列。
func TestRunNowCreatesUserRequestFromJob(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:             7,
		CronType:       schedulerservice.CronTypeCNAShareQuoteRefresh,
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON:     `{"provider":"sina_tencent"}`,
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TimeoutSeconds: 45,
	}}}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{
		"id":7,
		"target_date":"2026-06-19",
		"ignore_trade_window":true
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 1 {
		t.Fatalf("expected one run-now run, got %+v", store.createdRuns)
	}
	run := store.createdRuns[0]
	if run.JobID != 7 || run.TriggerType != schedulerservice.TriggerUserRequest || run.ScopeKey != "CN:SH:600519" || run.TargetDate != "2026-06-19" {
		t.Fatalf("unexpected run-now run: %+v", run)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(run.ParamsJSON), &params); err != nil {
		t.Fatalf("run-now params_json should stay valid JSON, got %q: %v", run.ParamsJSON, err)
	}
	if params["timeout_seconds"] != float64(45) {
		t.Fatalf("expected run-now to preserve job timeout_seconds, got params %+v", params)
	}
	if len(queue.Snapshot()) != 1 || queue.Snapshot()[0].RunKey != run.RunKey {
		t.Fatalf("expected run-now run to enter queue, got %+v", queue.Snapshot())
	}
}

// TestRunNowUsesSchedulerServiceWhenConfigured 验证生产注入 service 时 action 只负责读取请求并委托业务编排。
func TestRunNowUsesSchedulerServiceWhenConfigured(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareQuoteRefresh,
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	service := &fakeSchedulerService{
		run: model.SchedulerRun{
			ID:          99,
			JobID:       7,
			CronType:    schedulerservice.CronTypeCNAShareQuoteRefresh,
			DataType:    "quote",
			RunKey:      "user_request:from-service",
			TriggerType: schedulerservice.TriggerUserRequest,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    100,
			Source:      "user_request",
			TargetDate:  "2026-06-19",
			ScopeKey:    "CN:SH:600519",
		},
	}
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{
		"id":7,
		"target_date":"2026-06-19",
		"ignore_trade_window":true
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.job.ID != 7 || service.targetDate != "2026-06-19" || service.requestedAt.IsZero() {
		t.Fatalf("scheduler service was not called with request data: %+v", service)
	}
	if !service.runNowRequest.IgnoreTradeWindow {
		t.Fatalf("scheduler service should receive ignore_trade_window=true, got %+v", service.runNowRequest)
	}
	if len(store.createdRuns) != 0 || len(service.queueSnapshot) != 0 {
		t.Fatalf("action must delegate run creation/enqueue to service, store=%+v queue=%+v", store.createdRuns, service.queueSnapshot)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || data["run_key"] != "user_request:from-service" {
		t.Fatalf("expected action response to return service run, got %#v", response.Data)
	}
}

// TestRunNowReturnsBadRequestWhenTradeWindowClosed 验证默认立即执行被交易窗口拦截时返回 400。
func TestRunNowReturnsBadRequestWhenTradeWindowClosed(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareQuoteRefresh,
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	service := &fakeSchedulerService{runNowErr: schedulerservice.ErrTradeWindowClosed}
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{
		"id":7,
		"target_date":"2026-06-19"
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected closed trade window to return 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.runNowRequest.IgnoreTradeWindow {
		t.Fatalf("scheduler service should receive default ignore_trade_window=false, got %+v", service.runNowRequest)
	}
}

// TestRunNowRejectsUnknownRequestFields 验证 scheduler API 不接受请求体中的未知字段。
func TestRunNowRejectsUnknownRequestFields(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareQuoteRefresh,
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{
		"id":7,
		"target_date":"2026-06-19",
		"unexpected":true
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unknown request field to return 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("unknown request field must not create scheduler run: %+v", store.createdRuns)
	}
}

// TestRunNowReturnsServiceUnavailableWithoutQueue 验证未注入执行队列时立即执行返回 503。
func TestRunNowReturnsServiceUnavailableWithoutQueue(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareQuoteRefresh,
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{
		"id":7,
		"target_date":"2026-06-19",
		"ignore_trade_window":true
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing scheduler queue to return 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("queue unavailable must not create run-now run: %+v", store.createdRuns)
	}
}

// TestBackfillCreatesCatchupRunsWithinDateRange 验证手动补偿按日期范围生成 catchup_gap run。
func TestBackfillCreatesCatchupRunsWithinDateRange(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:         7,
		CronType:   schedulerservice.CronTypeCNAShareKlineRefresh,
		ScopeJSON:  `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON: `{"period":"day","adjust":"none","limit":120}`,
		Market:     "CN",
		Timezone:   "Asia/Shanghai",
	}}}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/jobs/backfill", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/backfill", strings.NewReader(`{
		"id":7,
		"dateFrom":"2026-06-17",
		"dateTo":"2026-06-19",
		"symbols":["CN:SH:600519"]
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 3 {
		t.Fatalf("expected three backfill runs, got %+v", store.createdRuns)
	}
	for _, run := range store.createdRuns {
		if run.TriggerType != schedulerservice.TriggerCatchupGap || run.Priority != 60 || run.ScopeKey != "CN:SH:600519" {
			t.Fatalf("unexpected backfill run: %+v", run)
		}
	}
}

// TestBackfillUsesSchedulerServiceWhenConfigured 验证生产注入 service 时 action 只委托手动补偿编排。
func TestBackfillUsesSchedulerServiceWhenConfigured(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareKlineRefresh,
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	service := &fakeSchedulerService{
		backfillRuns: []model.SchedulerRun{{
			ID:          101,
			JobID:       7,
			CronType:    schedulerservice.CronTypeCNAShareKlineRefresh,
			DataType:    "kline",
			Period:      "day",
			RunKey:      "scheduler_job:7:2026-06-19:catchup_gap",
			TriggerType: schedulerservice.TriggerCatchupGap,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    60,
			Source:      "manual_backfill",
			TargetDate:  "2026-06-19",
			ScopeKey:    "CN:SH:600519",
		}},
	}
	handler := routeHandler(t, "/api/scheduler/jobs/backfill", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/backfill", strings.NewReader(`{
		"id":7,
		"dateFrom":"2026-06-19",
		"dateTo":"2026-06-19",
		"symbols":["CN:SH:600519"]
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.backfillJob.ID != 7 || service.backfillRequest.DateFrom != "2026-06-19" || len(service.backfillRequest.Symbols) != 1 {
		t.Fatalf("scheduler service was not called with backfill request data: %+v", service)
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("action must delegate backfill creation to service, got %+v", store.createdRuns)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object response data, got %T", response.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one service backfill item, got %#v", data["items"])
	}
}

// TestBackfillReturnsExistingRunForDuplicateRunKey 验证重复补偿请求会返回已有 run，避免把幂等重试变成 500。
func TestBackfillReturnsExistingRunForDuplicateRunKey(t *testing.T) {
	store := &memoryStore{
		jobs: []model.SchedulerJob{{
			ID:         7,
			CronType:   schedulerservice.CronTypeCNAShareKlineRefresh,
			ScopeJSON:  `{"symbols":["CN:SH:600519"]}`,
			ParamsJSON: `{"period":"day","adjust":"none","limit":120}`,
			Market:     "CN",
			Timezone:   "Asia/Shanghai",
		}},
		runs: []model.SchedulerRun{{
			ID:          99,
			JobID:       7,
			CronType:    schedulerservice.CronTypeCNAShareKlineRefresh,
			DataType:    "kline",
			Period:      "day",
			ScopeKey:    "CN:SH:600519",
			TargetDate:  "2026-06-19",
			RunKey:      "scheduler_job:7:CN:SH:600519:2026-06-19:catchup_gap",
			TriggerType: schedulerservice.TriggerCatchupGap,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    60,
			Source:      "manual_backfill",
		}},
	}
	queue := schedulerservice.NewExecutionQueue()
	handler := routeHandler(t, "/api/scheduler/jobs/backfill", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    queue,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/backfill", strings.NewReader(`{
		"id":7,
		"dateFrom":"2026-06-19",
		"dateTo":"2026-06-19",
		"symbols":["CN:SH:600519"]
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate backfill to return existing run, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("duplicate backfill must not create new run: %+v", store.createdRuns)
	}
	if len(queue.Snapshot()) != 0 {
		t.Fatalf("duplicate existing run must not enqueue again, got %+v", queue.Snapshot())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object response data, got %T", response.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one existing backfill item, got %#v", data["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["id"] != float64(99) || item["run_key"] != "scheduler_job:7:CN:SH:600519:2026-06-19:catchup_gap" {
		t.Fatalf("unexpected existing backfill item: %#v", items[0])
	}
}

// TestBackfillRejectsTooLargeDateRange 验证手动补偿日期范围不能超过 30 天。
func TestBackfillRejectsTooLargeDateRange(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{ID: 7, CronType: schedulerservice.CronTypeCNAShareKlineRefresh, Market: "CN", Timezone: "Asia/Shanghai"}}}
	handler := routeHandler(t, "/api/scheduler/jobs/backfill", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/backfill", strings.NewReader(`{
		"id":7,
		"dateFrom":"2026-05-01",
		"dateTo":"2026-06-19",
		"symbols":["CN:SH:600519"]
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("invalid backfill must not create runs: %+v", store.createdRuns)
	}
}

// TestBackfillReturnsServiceUnavailableWithoutQueue 验证手动补偿缺少执行队列时返回 503。
func TestBackfillReturnsServiceUnavailableWithoutQueue(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:        7,
		CronType:  schedulerservice.CronTypeCNAShareKlineRefresh,
		Market:    "CN",
		Timezone:  "Asia/Shanghai",
		ScopeJSON: `{"symbols":["CN:SH:600519"]}`,
	}}}
	handler := routeHandler(t, "/api/scheduler/jobs/backfill", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/backfill", strings.NewReader(`{
		"id":7,
		"dateFrom":"2026-06-19",
		"dateTo":"2026-06-19",
		"symbols":["CN:SH:600519"]
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing scheduler queue to return 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("queue unavailable must not create backfill run: %+v", store.createdRuns)
	}
}

// TestRunNowReturnsNotFoundForMissingJob 验证立即执行不存在的长期任务时返回 404。
func TestRunNowReturnsNotFoundForMissingJob(t *testing.T) {
	store := &memoryStore{}
	handler := routeHandler(t, "/api/scheduler/jobs/run-now", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run-now", strings.NewReader(`{"id":404,"target_date":"2026-06-19"}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing scheduler job to return 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("missing scheduler job must not create run: %+v", store.createdRuns)
	}
}

// TestListJobsReturnsCronType 验证管理接口返回 cron_type 而不是旧的 type 字段。
func TestListJobsReturnsCronType(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:       7,
		Name:     "A 股开盘行情刷新",
		CronType: schedulerservice.CronTypeCNAShareQuoteRefresh,
		Enabled:  true,
	}}}
	handler := routeHandler(t, "/api/scheduler/jobs/list", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/list", strings.NewReader(`{}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object response data, got %T", response.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one scheduler job item, got %#v", data["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected scheduler job object, got %T", items[0])
	}
	if item["cron_type"] != schedulerservice.CronTypeCNAShareQuoteRefresh {
		t.Fatalf("expected cron_type field, got %#v", item)
	}
	if _, exists := item["type"]; exists {
		t.Fatalf("scheduler job response must not expose legacy type field: %#v", item)
	}
}

// TestSaveJobReloadsSchedulerServiceWhenConfigured 验证保存长期任务后会同步重载 gocron entry。
func TestSaveJobReloadsSchedulerServiceWhenConfigured(t *testing.T) {
	store := &memoryStore{}
	service := &fakeSchedulerService{}
	handler := routeHandler(t, "/api/scheduler/jobs/save", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/save", strings.NewReader(`{
		"id":7,
		"name":"A 股开盘行情刷新",
		"cron_type":"cn_a_share_quote_refresh",
		"cron_expr":"30 9 * * 1-5",
		"enabled":true,
		"market":"CN",
		"timezone":"Asia/Shanghai",
		"trade_window":"trading_time",
		"scope_json":"{\"symbols\":[\"CN:SH:600519\"]}",
		"params_json":"{}",
		"catchup_max_days":5,
		"timeout_seconds":120
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(service.reloadJobs) != 1 || service.reloadJobs[0].ID != 7 || !service.reloadJobs[0].Enabled {
		t.Fatalf("expected scheduler service to reload saved job, got %+v", service.reloadJobs)
	}
}

// TestSetEnabledReloadsSchedulerServiceWhenConfigured 验证启停长期任务后会按最新状态重载 gocron entry。
func TestSetEnabledReloadsSchedulerServiceWhenConfigured(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:       7,
		Name:     "A 股开盘行情刷新",
		CronType: schedulerservice.CronTypeCNAShareQuoteRefresh,
		Enabled:  true,
	}}}
	service := &fakeSchedulerService{}
	handler := routeHandler(t, "/api/scheduler/jobs/set-enabled", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/set-enabled", strings.NewReader(`{
		"id":7,
		"enabled":false
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(service.reloadJobs) != 1 || service.reloadJobs[0].ID != 7 || service.reloadJobs[0].Enabled {
		t.Fatalf("expected scheduler service to reload disabled job, got %+v", service.reloadJobs)
	}
}

// TestDeleteJobReloadsSchedulerServiceWhenConfigured 验证删除长期任务后会移除 gocron entry。
func TestDeleteJobReloadsSchedulerServiceWhenConfigured(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:       7,
		Name:     "A 股开盘行情刷新",
		CronType: schedulerservice.CronTypeCNAShareQuoteRefresh,
		Enabled:  true,
	}}}
	service := &fakeSchedulerService{}
	handler := routeHandler(t, "/api/scheduler/jobs/delete", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
		Service:  service,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/delete", strings.NewReader(`{"id":7}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(service.reloadJobs) != 1 || service.reloadJobs[0].ID != 7 || service.reloadJobs[0].Enabled {
		t.Fatalf("expected scheduler service to remove deleted job entry, got %+v", service.reloadJobs)
	}
}

// TestGetJobReturnsSchedulerJob 验证管理接口可读取单个调度任务详情。
func TestGetJobReturnsSchedulerJob(t *testing.T) {
	store := &memoryStore{jobs: []model.SchedulerJob{{
		ID:       7,
		Name:     "A 股开盘行情刷新",
		CronType: schedulerservice.CronTypeCNAShareQuoteRefresh,
		Enabled:  true,
	}}}
	handler := routeHandler(t, "/api/scheduler/jobs/get", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/get", strings.NewReader(`{"id":7}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || data["cron_type"] != schedulerservice.CronTypeCNAShareQuoteRefresh {
		t.Fatalf("unexpected scheduler job response: %#v", response.Data)
	}
}

// TestGetJobReturnsNotFound 验证读取不存在的调度任务时返回 404。
func TestGetJobReturnsNotFound(t *testing.T) {
	handler := routeHandler(t, "/api/scheduler/jobs/get", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    &memoryStore{},
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/get", strings.NewReader(`{"id":404}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing scheduler job to return 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestGetRunReturnsSchedulerRun 验证管理接口可读取单条执行记录详情。
func TestGetRunReturnsSchedulerRun(t *testing.T) {
	startedAt := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 6, 19, 9, 31, 0, 0, time.UTC)
	store := &memoryStore{runs: []model.SchedulerRun{{
		ID:          11,
		RunKey:      "run-11",
		TriggerType: schedulerservice.TriggerMissedToday,
		Status:      schedulerservice.RunStatusQueued,
		StartedAt:   &startedAt,
		FinishedAt:  &finishedAt,
	}}}
	handler := routeHandler(t, "/api/scheduler/runs/get", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/runs/get", strings.NewReader(`{"id":11}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || data["run_key"] != "run-11" || data["started_at"] != "2026-06-19T09:30:00Z" || data["finished_at"] != "2026-06-19T09:31:00Z" {
		t.Fatalf("unexpected scheduler run response: %#v", response.Data)
	}
}

// TestGetRunReturnsNotFound 验证读取不存在的执行记录时返回 404。
func TestGetRunReturnsNotFound(t *testing.T) {
	handler := routeHandler(t, "/api/scheduler/runs/get", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    &memoryStore{},
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/runs/get", strings.NewReader(`{"id":404}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing scheduler run to return 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestStatusReturnsSchedulerCounts 验证调度状态接口返回 job 和运行中 run 计数。
func TestStatusReturnsSchedulerCounts(t *testing.T) {
	store := &memoryStore{
		jobs: []model.SchedulerJob{
			{ID: 1, Enabled: true},
			{ID: 2, Enabled: false},
		},
		runs: []model.SchedulerRun{
			{ID: 1, Status: schedulerservice.RunStatusQueued},
			{ID: 2, Status: schedulerservice.RunStatusRunning},
			{ID: 3, Status: schedulerservice.RunStatusFailed},
		},
	}
	handler := routeHandler(t, "/api/scheduler/status", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/status", strings.NewReader(`{}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object response data, got %T", response.Data)
	}
	if data["jobs_total"] != float64(2) || data["jobs_enabled"] != float64(1) || data["queued_runs"] != float64(1) || data["running_runs"] != float64(1) || data["failed_runs"] != float64(1) {
		t.Fatalf("unexpected scheduler status response: %#v", data)
	}
}

// TestSaveJobRejectsUnknownCronType 验证 API 写入任务前会调用注册表校验 cron_type。
func TestSaveJobRejectsUnknownCronType(t *testing.T) {
	store := &memoryStore{}
	handler := routeHandler(t, "/api/scheduler/jobs/save", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/save", strings.NewReader(`{
		"name":"未知任务",
		"cron_type":"unknown",
		"cron_expr":"30 9 * * 1-5",
		"market":"CN",
		"timezone":"Asia/Shanghai",
		"trade_window":"trading_time",
		"scope_json":"{\"symbols\":[\"CN:SH:600519\"]}",
		"params_json":"{}",
		"catchup_max_days":5,
		"timeout_seconds":120
	}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cron_type to return 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.jobs) != 0 {
		t.Fatalf("invalid scheduler job must not be saved: %+v", store.jobs)
	}
}

// TestJobTypesReturnsRegistryMetadata 验证管理接口能读取注册表中的 cron_type 元数据。
func TestJobTypesReturnsRegistryMetadata(t *testing.T) {
	handler := routeHandler(t, "/api/scheduler/job-types", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    &memoryStore{},
		Queue:    schedulerservice.NewExecutionQueue(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scheduler/job-types", strings.NewReader(`{}`))
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %T", response.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected job type items, got %#v", data["items"])
	}
}

// routeHandler 返回指定 path 对应的 handler，避免测试依赖 Gin 根路由。
func routeHandler(t *testing.T, path string, config Config) http.HandlerFunc {
	t.Helper()
	for _, route := range Routes(config) {
		if route.Path == path {
			return route.Handler
		}
	}
	t.Fatalf("missing route %s", path)
	return nil
}

type fakeSchedulerService struct {
	run             model.SchedulerRun
	runNowErr       error
	job             model.SchedulerJob
	targetDate      string
	requestedAt     time.Time
	runNowRequest   schedulerservice.RunNowRequest
	backfillRuns    []model.SchedulerRun
	backfillJob     model.SchedulerJob
	backfillRequest schedulerservice.BackfillRequest
	reloadJobs      []model.SchedulerJob
	queueSnapshot   []model.SchedulerRun
}

// RunNow 记录 action 委托的立即执行请求，并返回测试预置执行记录。
func (service *fakeSchedulerService) RunNow(_ context.Context, job model.SchedulerJob, request schedulerservice.RunNowRequest) (model.SchedulerRun, error) {
	service.job = job
	service.targetDate = request.TargetDate
	service.requestedAt = request.RequestedAt
	service.runNowRequest = request
	return service.run, service.runNowErr
}

// Backfill 记录 action 委托的手动补偿请求，并返回测试预置执行记录。
func (service *fakeSchedulerService) Backfill(_ context.Context, job model.SchedulerJob, request schedulerservice.BackfillRequest) ([]model.SchedulerRun, error) {
	service.backfillJob = job
	service.backfillRequest = request
	return service.backfillRuns, nil
}

// ReloadJob 记录 action 在任务配置变更后委托的 gocron 重载请求。
func (service *fakeSchedulerService) ReloadJob(_ context.Context, job model.SchedulerJob) error {
	service.reloadJobs = append(service.reloadJobs, job)
	return nil
}

type memoryStore struct {
	jobs        []model.SchedulerJob
	runs        []model.SchedulerRun
	createdRuns []model.SchedulerRun
	updatedRuns []model.SchedulerRun
}

// SaveSchedulerJob 记录保存的调度任务。
func (store *memoryStore) SaveSchedulerJob(_ context.Context, job *model.SchedulerJob) error {
	store.jobs = append(store.jobs, *job)
	return nil
}

// ListSchedulerJobs 返回测试预置的调度任务。
func (store *memoryStore) ListSchedulerJobs(context.Context) ([]model.SchedulerJob, error) {
	return store.jobs, nil
}

// GetSchedulerJob 按 ID 返回测试任务。
func (store *memoryStore) GetSchedulerJob(_ context.Context, id int64) (model.SchedulerJob, bool, error) {
	for _, job := range store.jobs {
		if job.ID == id {
			return job, true, nil
		}
	}
	return model.SchedulerJob{}, false, nil
}

// SetSchedulerJobEnabled 切换测试任务的启用状态。
func (store *memoryStore) SetSchedulerJobEnabled(_ context.Context, id int64, enabled bool) error {
	for index := range store.jobs {
		if store.jobs[index].ID == id {
			store.jobs[index].Enabled = enabled
		}
	}
	return nil
}

// SoftDeleteSchedulerJob 删除测试任务。
func (store *memoryStore) SoftDeleteSchedulerJob(_ context.Context, id int64) error {
	filtered := store.jobs[:0]
	for _, job := range store.jobs {
		if job.ID != id {
			filtered = append(filtered, job)
		}
	}
	store.jobs = filtered
	return nil
}

// CreateSchedulerRun 记录新建的执行记录。
func (store *memoryStore) CreateSchedulerRun(_ context.Context, run *model.SchedulerRun) error {
	store.createdRuns = append(store.createdRuns, *run)
	store.runs = append(store.runs, *run)
	return nil
}

// GetSchedulerRunByRunKey 按 run_key 返回测试执行记录。
func (store *memoryStore) GetSchedulerRunByRunKey(_ context.Context, runKey string) (model.SchedulerRun, bool, error) {
	for _, run := range store.runs {
		if run.RunKey == runKey {
			return run, true, nil
		}
	}
	return model.SchedulerRun{}, false, nil
}

// UpdateSchedulerRun 记录执行状态更新。
func (store *memoryStore) UpdateSchedulerRun(_ context.Context, run *model.SchedulerRun) error {
	store.updatedRuns = append(store.updatedRuns, *run)
	return nil
}

// ListSchedulerRuns 返回测试预置的执行记录。
func (store *memoryStore) ListSchedulerRuns(context.Context, int64, int) ([]model.SchedulerRun, error) {
	return store.runs, nil
}

// GetSchedulerRun 按 ID 返回测试执行记录。
func (store *memoryStore) GetSchedulerRun(_ context.Context, id int64) (model.SchedulerRun, bool, error) {
	for _, run := range store.runs {
		if run.ID == id {
			return run, true, nil
		}
	}
	return model.SchedulerRun{}, false, nil
}

// ListSchedulerRunsByStatuses 返回匹配状态的执行记录。
func (store *memoryStore) ListSchedulerRunsByStatuses(_ context.Context, statuses []string) ([]model.SchedulerRun, error) {
	statusSet := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		statusSet[status] = struct{}{}
	}
	var runs []model.SchedulerRun
	for _, run := range store.runs {
		if _, ok := statusSet[run.Status]; ok {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

var _ Store = (*memoryStore)(nil)
