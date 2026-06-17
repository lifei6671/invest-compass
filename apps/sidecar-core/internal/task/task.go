package task

import (
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
)

// ErrorCode 是任务模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidTransition 表示任务状态流转不符合状态机规则。
	ErrorInvalidTransition ErrorCode = "invalid_task_transition"
)

// Error 表示任务模块错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把任务 payload 写入错误文本。
func (err *Error) Error() string {
	return string(err.Code)
}

// Status 是首版支持的任务状态。
type Status string

const (
	// StatusPending 表示任务已创建但未开始。
	StatusPending Status = "PENDING"
	// StatusRunning 表示任务正在执行。
	StatusRunning Status = "RUNNING"
	// StatusSuccess 表示任务成功完成。
	StatusSuccess Status = "SUCCESS"
	// StatusFailed 表示任务失败终止。
	StatusFailed Status = "FAILED"
	// StatusCancelled 表示任务被取消。
	StatusCancelled Status = "CANCELLED"
)

// Type 是首版支持的任务类型。
type Type string

const (
	// TypeAnalysis 表示 AI 个股分析任务。
	TypeAnalysis Type = "ANALYSIS"
)

// EventType 是首版支持的任务事件类型。
type EventType string

const (
	// EventCreated 表示任务已创建。
	EventCreated EventType = "TASK_CREATED"
	// EventStarted 表示任务已开始运行。
	EventStarted EventType = "TASK_STARTED"
	// EventProgress 表示任务进度变化。
	EventProgress EventType = "TASK_PROGRESS"
	// EventLog 表示任务日志。
	EventLog EventType = "TASK_LOG"
	// EventChunk 表示流式任务输出片段。
	EventChunk EventType = "TASK_CHUNK"
	// EventSuccess 表示任务成功完成。
	EventSuccess EventType = "TASK_SUCCESS"
	// EventFailed 表示任务失败。
	EventFailed EventType = "TASK_FAILED"
	// EventCancelled 表示任务被取消。
	EventCancelled EventType = "TASK_CANCELLED"
)

// Task 是长任务基础模型。
type Task struct {
	ID         string
	Type       Type
	Status     Status
	Title      string
	Progress   int
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Event 是任务事件回放和 SSE 转发共用的基础模型。
type Event struct {
	ID        int64
	TaskID    string
	Type      EventType
	Payload   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusPending: {
		StatusRunning:   {},
		StatusCancelled: {},
	},
	StatusRunning: {
		StatusSuccess:   {},
		StatusFailed:    {},
		StatusCancelled: {},
	},
}

// ValidateTransition 校验任务状态流转是否合法。
func ValidateTransition(from Status, to Status) error {
	nextStatuses, ok := allowedTransitions[from]
	if !ok {
		return &Error{Code: ErrorInvalidTransition}
	}
	if _, ok := nextStatuses[to]; !ok {
		return &Error{Code: ErrorInvalidTransition}
	}
	return nil
}

// Terminal 判断任务状态是否为终态。
func (status Status) Terminal() bool {
	return status == StatusSuccess || status == StatusFailed || status == StatusCancelled
}

// Status 返回事件对应的任务状态，进度、日志和 chunk 事件不改变状态。
func (event EventType) Status() (Status, bool) {
	switch event {
	case EventCreated:
		return StatusPending, true
	case EventStarted:
		return StatusRunning, true
	case EventSuccess:
		return StatusSuccess, true
	case EventFailed:
		return StatusFailed, true
	case EventCancelled:
		return StatusCancelled, true
	default:
		return "", false
	}
}

// SanitizeEventPayload 在事件 payload 入库或转发前统一脱敏。
func SanitizeEventPayload(payload string) string {
	return logger.RedactText(payload)
}

// ReplayEvents 按事件 ID 递增回放 afterEventID 之后的事件。
func ReplayEvents(events []Event, afterEventID int64) []Event {
	filtered := make([]Event, 0, len(events))
	for _, event := range events {
		if event.ID <= afterEventID {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.SliceStable(filtered, func(left int, right int) bool {
		return filtered[left].ID < filtered[right].ID
	})
	return filtered
}

// RecoverRunningTask 将 RUNNING 任务恢复成终态，避免 sidecar 重启后永久悬挂。
func RecoverRunningTask(task Task) (Task, Event) {
	if task.Status != StatusRunning {
		return task, Event{}
	}

	now := time.Now().UTC()
	task.Status = StatusFailed
	task.Error = "task_interrupted_by_core_restart"
	task.FinishedAt = now
	task.UpdatedAt = now
	return task, Event{
		TaskID:    task.ID,
		Type:      EventFailed,
		Payload:   `{"reason":"task_interrupted_by_core_restart"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
