package task

import (
	"errors"
	"strings"
	"testing"
	"time"
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
			assertTaskErrorCode(t, err, ErrorInvalidTransition)
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
	payload := `{"Authorization":"Bearer sk-task-secret","api_key":"raw-secret","message":"ok"}`

	sanitized := SanitizeEventPayload(payload)

	if strings.Contains(sanitized, "sk-task-secret") || strings.Contains(sanitized, "raw-secret") {
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

// assertTaskErrorCode 校验任务错误码稳定。
func assertTaskErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var taskError *Error
	if !errors.As(err, &taskError) {
		t.Fatalf("expected task Error, got %T", err)
	}
	if taskError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, taskError.Code)
	}
}
