package task

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestStatusMachineAllowsExpectedTransitions 验证任务状态机允许首版合法流转。
func TestStatusMachineAllowsExpectedTransitions(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
	}{
		{StatusPending, StatusRunning},
		{StatusRunning, StatusSuccess},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusCancelled},
		{StatusPending, StatusCancelled},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+" to "+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Fatalf("ValidateTransition returned error: %v", err)
			}
		})
	}
}

// TestStatusMachineRejectsInvalidTransitions 验证终态不可继续流转。
func TestStatusMachineRejectsInvalidTransitions(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
	}{
		{StatusPending, StatusSuccess},
		{StatusSuccess, StatusRunning},
		{StatusFailed, StatusRunning},
		{StatusCancelled, StatusSuccess},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+" to "+string(tt.to), func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			assertTaskErrorCode(t, err, xerr.TaskInvalidTransition)
		})
	}
}

// TestEventTypeMapsToStatus 验证任务事件会映射到预期任务状态。
func TestEventTypeMapsToStatus(t *testing.T) {
	tests := []struct {
		event  EventType
		status Status
		ok     bool
	}{
		{EventCreated, StatusPending, true},
		{EventStarted, StatusRunning, true},
		{EventSuccess, StatusSuccess, true},
		{EventFailed, StatusFailed, true},
		{EventCancelled, StatusCancelled, true},
		{EventProgress, "", false},
		{EventLog, "", false},
		{EventChunk, "", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			status, ok := tt.event.Status()
			if status != tt.status || ok != tt.ok {
				t.Fatalf("expected status=%q ok=%v, got status=%q ok=%v", tt.status, tt.ok, status, ok)
			}
		})
	}
}

// TestSanitizeEventPayloadRedactsSensitiveFields 验证事件 payload 入库前必须脱敏。
func TestSanitizeEventPayloadRedactsSensitiveFields(t *testing.T) {
	payload := `{"Authorization":"Bearer placeholder-sensitive-value","api_key":"placeholder-api-value","message":"ok"}`

	sanitized := SanitizeEventPayload(payload)

	if strings.Contains(sanitized, "placeholder-sensitive-value") || strings.Contains(sanitized, "placeholder-api-value") {
		t.Fatalf("sanitized payload leaked secret: %s", sanitized)
	}
	if !strings.Contains(sanitized, "message") || !strings.Contains(sanitized, "ok") {
		t.Fatalf("sanitized payload should keep non-sensitive content: %s", sanitized)
	}
}

// TestReplayEventsOrdersByID 验证事件回放按 id 递增排序，并支持 afterEventID。
func TestReplayEventsOrdersByID(t *testing.T) {
	events := []Event{
		{ID: 3, Type: EventSuccess},
		{ID: 1, Type: EventCreated},
		{ID: 2, Type: EventStarted},
	}

	replayed := ReplayEvents(events, 1)

	if len(replayed) != 2 || replayed[0].ID != 2 || replayed[1].ID != 3 {
		t.Fatalf("unexpected replay order: %+v", replayed)
	}
}

// TestRecoverRunningTaskReturnsTerminalStatus 验证 RUNNING 任务恢复时不会继续悬挂。
func TestRecoverRunningTaskReturnsTerminalStatus(t *testing.T) {
	recovered, event := RecoverRunningTask(Task{
		ID:        "task-1",
		Type:      TypeAnalysis,
		Status:    StatusRunning,
		CreatedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
	})

	if recovered.Status != StatusFailed {
		t.Fatalf("expected running analysis task to recover as FAILED, got %q", recovered.Status)
	}
	if event.Type != EventFailed || event.TaskID != "task-1" {
		t.Fatalf("unexpected recovery event: %+v", event)
	}
}

// TestListTasksByUpdatedAtDesc 验证任务历史列表按更新时间倒序返回。
func TestListTasksByUpdatedAtDesc(t *testing.T) {
	tasks := []Task{
		{ID: "task-old", UpdatedAt: time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)},
		{ID: "task-new", UpdatedAt: time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)},
		{ID: "task-mid", UpdatedAt: time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)},
	}

	listed := ListTasksByUpdatedAt(tasks)

	if len(listed) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(listed))
	}
	if listed[0].ID != "task-new" || listed[1].ID != "task-mid" || listed[2].ID != "task-old" {
		t.Fatalf("unexpected task order: %+v", listed)
	}
}

// TestRecoverRunningTasksOnlyRecoversRunning 验证 core 启动恢复只处理 RUNNING 任务。
func TestRecoverRunningTasksOnlyRecoversRunning(t *testing.T) {
	tasks := []Task{
		{ID: "task-running-1", Type: TypeAnalysis, Status: StatusRunning},
		{ID: "task-success", Type: TypeAnalysis, Status: StatusSuccess},
		{ID: "task-running-2", Type: TypeAnalysis, Status: StatusRunning},
	}

	recovered, events := RecoverRunningTasks(tasks)

	if len(recovered) != 3 {
		t.Fatalf("expected 3 recovered tasks, got %d", len(recovered))
	}
	if recovered[0].Status != StatusFailed || recovered[1].Status != StatusSuccess || recovered[2].Status != StatusFailed {
		t.Fatalf("unexpected recovered statuses: %+v", recovered)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 recovery events, got %d", len(events))
	}
	if events[0].TaskID != "task-running-1" || events[1].TaskID != "task-running-2" {
		t.Fatalf("unexpected recovery events: %+v", events)
	}
}

// TestFormatSSEEventRedactsPayload 验证 SSE 转发前会保留事件元信息并脱敏 payload。
func TestFormatSSEEventRedactsPayload(t *testing.T) {
	frame := FormatSSEEvent(Event{
		ID:      12,
		TaskID:  "task-1",
		Type:    EventChunk,
		Payload: `{"content":"ok","Authorization":"Bearer placeholder-stream-value"}`,
	})

	if !strings.Contains(frame, "id: 12\n") {
		t.Fatalf("expected SSE id, got %q", frame)
	}
	if !strings.Contains(frame, "event: TASK_CHUNK\n") {
		t.Fatalf("expected SSE event type, got %q", frame)
	}
	if !strings.Contains(frame, `data: {"content":"ok"`) {
		t.Fatalf("expected SSE data payload, got %q", frame)
	}
	if strings.Contains(frame, "placeholder-stream-value") {
		t.Fatalf("SSE frame leaked secret: %q", frame)
	}
	if !strings.HasSuffix(frame, "\n\n") {
		t.Fatalf("expected SSE frame delimiter, got %q", frame)
	}
}

// TestFormatSSEEventPrefixesEveryDataLine 验证多行 payload 会被编码为合法 SSE data 行。
func TestFormatSSEEventPrefixesEveryDataLine(t *testing.T) {
	frame := FormatSSEEvent(Event{
		ID:      13,
		TaskID:  "task-1",
		Type:    EventLog,
		Payload: "第一行\n第二行",
	})

	if !strings.Contains(frame, "data: 第一行\n") || !strings.Contains(frame, "data: 第二行\n") {
		t.Fatalf("expected every payload line to use data prefix, got %q", frame)
	}
	if strings.Contains(frame, "\n第二行\n") {
		t.Fatalf("SSE frame contains unprefixed payload line: %q", frame)
	}
}

// TestFormatSSEReplayFramesUsesAfterEventID 验证 afterEventID 补拉后可直接编码为 SSE 帧。
func TestFormatSSEReplayFramesUsesAfterEventID(t *testing.T) {
	events := []Event{
		{ID: 3, Type: EventSuccess, Payload: `{"status":"ok"}`},
		{ID: 1, Type: EventCreated, Payload: `{"status":"created"}`},
		{ID: 2, Type: EventStarted, Payload: `{"status":"running"}`},
	}

	frames := FormatSSEReplayFrames(events, 1)

	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if !strings.HasPrefix(frames[0], "id: 2\n") || !strings.HasPrefix(frames[1], "id: 3\n") {
		t.Fatalf("unexpected replay frames: %#v", frames)
	}
}

// assertTaskErrorCode 校验任务错误码稳定。
func assertTaskErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var taskError *xerr.Error
	if !errors.As(err, &taskError) {
		t.Fatalf("expected task Error, got %T", err)
	}
	if taskError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, taskError.Code)
	}
}
