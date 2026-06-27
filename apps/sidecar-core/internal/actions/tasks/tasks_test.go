package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestEventsStreamRejectsMissingTask 验证订阅不存在的任务会快速返回 404，避免 SSE 长连接永久轮询空事件。
func TestEventsStreamRejectsMissingTask(t *testing.T) {
	store := &missingTaskStore{}
	handler := handleEventsStream(Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/tasks/events/stream",
		strings.NewReader(`{"task_id":"missing-task","after_event_id":0}`),
	)
	ctx, cancel := context.WithTimeout(request.Context(), 50*time.Millisecond)
	defer cancel()
	request = request.WithContext(ctx)
	request.Header.Set(httpx.TokenHeader, "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if store.eventPolls != 0 {
		t.Fatalf("missing task should not start event polling, got %d polls", store.eventPolls)
	}
}

// TestEventsStreamClosesWhenTerminalTaskHasNoNewEvents 验证已终态任务从最后事件后订阅会立即结束，
// 避免 Rust SSE 订阅在历史任务详情或断线重连场景中一直等待不存在的新事件。
func TestEventsStreamClosesWhenTerminalTaskHasNoNewEvents(t *testing.T) {
	store := &terminalTaskStore{
		task: model.Task{
			ID:     "task-success",
			Type:   "ANALYSIS",
			Status: "SUCCESS",
		},
		events: []model.TaskEvent{
			{ID: 1, TaskID: "task-success", EventType: "TASK_SUCCESS", Payload: `{"progress":100}`},
		},
	}
	handler := handleEventsStream(Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/tasks/events/stream",
		strings.NewReader(`{"task_id":"task-success","after_event_id":1}`),
	)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	request = request.WithContext(ctx)
	request.Header.Set(httpx.TokenHeader, "test-token")

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()

	select {
	case <-done:
	case <-time.After(150 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("terminal task stream should close when there are no new events")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected no replay frames after the last event, got %q", recorder.Body.String())
	}
}

// TestEventsStreamWaitsForRunningTaskProgress 验证运行中任务订阅会等待后续事件，
// 并把新增进度事件实时写成 SSE 帧，供 Rust 转发给前端。
func TestEventsStreamWaitsForRunningTaskProgress(t *testing.T) {
	store := &liveTaskStore{
		task: model.Task{
			ID:     "task-running",
			Type:   "ANALYSIS",
			Status: "RUNNING",
		},
	}
	handler := handleEventsStream(Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/tasks/events/stream",
		strings.NewReader(`{"task_id":"task-running","after_event_id":0}`),
	)
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	request.Header.Set(httpx.TokenHeader, "test-token")

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()
	time.AfterFunc(40*time.Millisecond, func() {
		store.appendEvent(model.TaskEvent{
			ID:        1,
			TaskID:    "task-running",
			EventType: "TASK_PROGRESS",
			Payload:   `{"stage":"quote_fetch","progress":20,"message":"行情快照已读取"}`,
		})
	})
	time.AfterFunc(180*time.Millisecond, cancel)

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("running task stream did not return after context cancellation")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: TASK_PROGRESS") ||
		!strings.Contains(body, `"stage":"quote_fetch"`) {
		t.Fatalf("expected progress SSE frame, got %q", body)
	}
	if store.eventPolls < 2 {
		t.Fatalf("expected stream to wait and poll for later events, got %d polls", store.eventPolls)
	}
}

// TestClearStreamWriteDeadlineRemovesServerWriteTimeout 验证 SSE 流式响应会清除全局写超时，
// 避免长时间 AI 分析还未产生终态事件时连接被 http.Server 主动切断。
func TestClearStreamWriteDeadlineRemovesServerWriteTimeout(t *testing.T) {
	response := &deadlineRecordingResponseWriter{header: http.Header{}}

	if err := clearStreamWriteDeadline(response); err != nil {
		t.Fatalf("clear stream write deadline: %v", err)
	}

	if !response.called {
		t.Fatal("expected stream response deadline to be updated")
	}
	if !response.deadline.IsZero() {
		t.Fatalf("expected zero deadline for stream response, got %s", response.deadline)
	}
}

type deadlineRecordingResponseWriter struct {
	header   http.Header
	called   bool
	deadline time.Time
}

// Header 实现 http.ResponseWriter，供 ResponseController 测试写 deadline。
func (writer *deadlineRecordingResponseWriter) Header() http.Header {
	return writer.header
}

// Write 实现 http.ResponseWriter，测试中无需真实写出响应体。
func (writer *deadlineRecordingResponseWriter) Write(body []byte) (int, error) {
	return len(body), nil
}

// WriteHeader 实现 http.ResponseWriter，测试中无需记录状态码。
func (writer *deadlineRecordingResponseWriter) WriteHeader(int) {}

// SetWriteDeadline 记录 SSE handler 是否把连接写 deadline 清成零值。
func (writer *deadlineRecordingResponseWriter) SetWriteDeadline(deadline time.Time) error {
	writer.called = true
	writer.deadline = deadline
	return nil
}

type missingTaskStore struct {
	eventPolls int
}

// ListTasks 实现 Store 接口；当前用例不需要任务列表。
func (store *missingTaskStore) ListTasks(context.Context, int) ([]model.Task, error) {
	return nil, nil
}

// GetTask 模拟任务不存在。
func (store *missingTaskStore) GetTask(context.Context, string) (model.Task, bool, error) {
	return model.Task{}, false, nil
}

// ListTaskEventsAfter 记录是否错误进入事件轮询。
func (store *missingTaskStore) ListTaskEventsAfter(context.Context, string, int64) ([]model.TaskEvent, error) {
	store.eventPolls++
	return nil, nil
}

// GetAnalysisReportByTaskID 实现 Store 接口；SSE 缺失任务用例不需要报告上下文。
func (store *missingTaskStore) GetAnalysisReportByTaskID(context.Context, string) (model.AnalysisReport, bool, error) {
	return model.AnalysisReport{}, false, nil
}

type terminalTaskStore struct {
	task   model.Task
	events []model.TaskEvent
}

// ListTasks 实现 Store 接口；当前用例不需要任务列表。
func (store *terminalTaskStore) ListTasks(context.Context, int) ([]model.Task, error) {
	return nil, nil
}

// GetTask 返回已终态任务，用于验证 SSE 空增量订阅结束条件。
func (store *terminalTaskStore) GetTask(context.Context, string) (model.Task, bool, error) {
	return store.task, true, nil
}

// ListTaskEventsAfter 只返回游标之后的事件。
func (store *terminalTaskStore) ListTaskEventsAfter(_ context.Context, taskID string, afterID int64) ([]model.TaskEvent, error) {
	result := make([]model.TaskEvent, 0)
	for _, event := range store.events {
		if event.TaskID == taskID && event.ID > afterID {
			result = append(result, event)
		}
	}
	return result, nil
}

// GetAnalysisReportByTaskID 实现 Store 接口；终态 SSE 空增量用例不需要报告上下文。
func (store *terminalTaskStore) GetAnalysisReportByTaskID(context.Context, string) (model.AnalysisReport, bool, error) {
	return model.AnalysisReport{}, false, nil
}

type liveTaskStore struct {
	mu         sync.Mutex
	task       model.Task
	events     []model.TaskEvent
	eventPolls int
}

// appendEvent 模拟后台分析任务在 SSE 订阅建立后写入新任务事件。
func (store *liveTaskStore) appendEvent(event model.TaskEvent) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.events = append(store.events, event)
}

// ListTasks 实现 Store 接口；当前用例不需要任务列表。
func (store *liveTaskStore) ListTasks(context.Context, int) ([]model.Task, error) {
	return nil, nil
}

// GetTask 返回运行中任务，直到测试取消请求上下文。
func (store *liveTaskStore) GetTask(context.Context, string) (model.Task, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.task, true, nil
}

// ListTaskEventsAfter 返回订阅建立后新写入的事件。
func (store *liveTaskStore) ListTaskEventsAfter(_ context.Context, taskID string, afterID int64) ([]model.TaskEvent, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.eventPolls++
	result := make([]model.TaskEvent, 0)
	for _, event := range store.events {
		if event.TaskID == taskID && event.ID > afterID {
			result = append(result, event)
		}
	}
	return result, nil
}

// GetAnalysisReportByTaskID 实现 Store 接口；运行中 SSE 用例不需要报告上下文。
func (store *liveTaskStore) GetAnalysisReportByTaskID(context.Context, string) (model.AnalysisReport, bool, error) {
	return model.AnalysisReport{}, false, nil
}
