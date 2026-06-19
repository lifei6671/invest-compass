package tasks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	streamPollInterval = 100 * time.Millisecond
	taskListMaxLimit   = 100
)

// Store 是 tasks action 依赖的数据访问边界。
type Store interface {
	ListTasks(ctx context.Context, limit int) ([]model.Task, error)
	GetTask(ctx context.Context, taskID string) (model.Task, bool, error)
	ListTaskEventsAfter(ctx context.Context, taskID string, afterID int64) ([]model.TaskEvent, error)
}

// Config 是任务历史 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type listRequest struct {
	Limit int `json:"limit"`
}

type getRequest struct {
	TaskID string `json:"task_id"`
}

type eventsRequest struct {
	TaskID       string `json:"task_id"`
	AfterEventID int64  `json:"after_event_id"`
}

type listData struct {
	Items []taskData `json:"items"`
}

type eventsData struct {
	Items []eventData `json:"items"`
}

type taskData struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Title        string `json:"title"`
	Progress     int    `json:"progress"`
	ErrorMessage string `json:"error_message"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type eventData struct {
	ID        int64  `json:"id"`
	TaskID    string `json:"task_id"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"created_at"`
}

// Routes 返回任务历史相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/tasks/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/tasks/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/tasks/events", Handler: handleEvents(config)},
		{Method: http.MethodPost, Path: "/api/tasks/events/stream", Handler: handleEventsStream(config)},
	}
}

// handleList 返回任务历史列表，按 DAO 层更新时间倒序口径展示。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if !validateListLimit(response, payload.Limit, context) {
			return
		}
		tasks, err := config.Store.ListTasks(request.Context(), payload.Limit)
		if err != nil {
			writeStoreError(response, context, "读取任务历史失败", err)
			return
		}
		httpx.WriteOK(response, listData{Items: tasksToData(tasks)}, context)
	}
}

// handleGet 返回单个任务详情。
func handleGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		taskID, ok := decodeTaskID(response, request, context)
		if !ok {
			return
		}
		task, found, err := config.Store.GetTask(request.Context(), taskID)
		if err != nil {
			writeStoreError(response, context, "读取任务详情失败", err)
			return
		}
		if !found {
			httpx.WriteError(response, http.StatusNotFound, 40403, "task_not_found", context)
			return
		}
		httpx.WriteOK(response, taskToData(task), context)
	}
}

// handleEvents 按 after_event_id 增量返回任务事件，供断线恢复和历史详情使用。
func handleEvents(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload eventsRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		taskID := strings.TrimSpace(payload.TaskID)
		if taskID == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_task_id", context)
			return
		}
		if !validateAfterEventID(response, payload.AfterEventID, context) {
			return
		}
		events, err := config.Store.ListTaskEventsAfter(request.Context(), taskID, payload.AfterEventID)
		if err != nil {
			writeStoreError(response, context, "读取任务事件失败", err)
			return
		}
		httpx.WriteOK(response, eventsData{Items: eventsToData(events)}, context)
	}
}

// handleEventsStream 以 SSE 帧补拉任务事件，供 Rust 订阅后转发给前端。
func handleEventsStream(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload eventsRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		taskID := strings.TrimSpace(payload.TaskID)
		if taskID == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_task_id", context)
			return
		}
		if !validateAfterEventID(response, payload.AfterEventID, context) {
			return
		}
		task, found, err := config.Store.GetTask(request.Context(), taskID)
		if err != nil {
			writeStoreError(response, context, "读取任务详情失败", err)
			return
		}
		if !found {
			httpx.WriteError(response, http.StatusNotFound, 40403, "task_not_found", context)
			return
		}
		if err := clearStreamWriteDeadline(response); err != nil {
			slog.Warn(
				"清除任务事件流写超时失败",
				logger.FieldRequestID, context.RequestID,
				logger.FieldTraceID, context.TraceID,
				"error", logger.RedactError(err),
			)
		}
		response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		response.Header().Set("Cache-Control", "no-cache")
		response.Header().Set("Connection", "keep-alive")
		response.WriteHeader(http.StatusOK)

		afterEventID := payload.AfterEventID
		for {
			events, err := config.Store.ListTaskEventsAfter(request.Context(), taskID, afterEventID)
			if err != nil {
				slog.Warn(
					"读取任务事件流失败",
					logger.FieldRequestID, context.RequestID,
					logger.FieldTraceID, context.TraceID,
					"error", logger.RedactError(err),
				)
				return
			}
			if len(events) > 0 {
				terminal := writeSSEEvents(response, events, afterEventID)
				afterEventID = events[len(events)-1].ID
				if terminal {
					return
				}
			} else if isTerminalTaskStatus(task.Status) {
				return
			} else if latestTask, found, err := config.Store.GetTask(request.Context(), taskID); err != nil {
				slog.Warn(
					"读取任务事件流状态失败",
					logger.FieldRequestID, context.RequestID,
					logger.FieldTraceID, context.TraceID,
					"error", logger.RedactError(err),
				)
				return
			} else if !found || isTerminalTaskStatus(latestTask.Status) {
				return
			} else {
				task = latestTask
			}

			select {
			case <-request.Context().Done():
				return
			case <-time.After(streamPollInterval):
			}
		}
	}
}

// clearStreamWriteDeadline 清除当前 SSE 响应的写 deadline，保留普通 API 的全局写超时。
func clearStreamWriteDeadline(response http.ResponseWriter) error {
	err := http.NewResponseController(response).SetWriteDeadline(time.Time{})
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

// writeSSEEvents 写出一批任务事件 SSE 帧，并返回是否包含终态事件。
func writeSSEEvents(response http.ResponseWriter, events []model.TaskEvent, afterEventID int64) bool {
	terminal := false
	for _, frame := range taskservice.FormatSSEReplayFrames(eventsToService(events), afterEventID) {
		if _, err := response.Write([]byte(frame)); err != nil {
			return terminal
		}
		if flusher, ok := response.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	for _, event := range events {
		if isTerminalTaskEvent(event.EventType) {
			terminal = true
		}
	}
	return terminal
}

// isTerminalTaskEvent 判断任务事件是否代表当前任务已经进入终态。
func isTerminalTaskEvent(eventType string) bool {
	switch taskservice.EventType(eventType) {
	case taskservice.EventSuccess, taskservice.EventFailed, taskservice.EventCancelled:
		return true
	default:
		return false
	}
}

// isTerminalTaskStatus 判断任务状态是否已经进入终态；终态且无新事件时 SSE 可以安全结束。
func isTerminalTaskStatus(status string) bool {
	return taskservice.Status(status).Terminal()
}

// requireStore 校验 ready/token 和任务 store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "task_store_unavailable", context)
		return false
	}
	return true
}

// decodeTaskID 解析任务详情请求中的任务 ID。
func decodeTaskID(response http.ResponseWriter, request *http.Request, context httpx.RequestContext) (string, bool) {
	var payload getRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return "", false
	}
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_task_id", context)
		return "", false
	}
	return taskID, true
}

// writeStoreError 记录脱敏后的任务存储错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50007, "task_store_error", context)
}

// validateListLimit 校验任务历史分页边界，避免 HTTP 请求触发 DAO 的无界查询语义。
func validateListLimit(response http.ResponseWriter, limit int, context httpx.RequestContext) bool {
	if limit <= 0 || limit > taskListMaxLimit {
		httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_limit", context)
		return false
	}
	return true
}

// validateAfterEventID 校验事件回放游标，0 表示从头补拉，负数没有合法业务语义。
func validateAfterEventID(response http.ResponseWriter, afterEventID int64, context httpx.RequestContext) bool {
	if afterEventID < 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40009, "invalid_after_event_id", context)
		return false
	}
	return true
}

// tasksToData 转换数据库任务列表为 API 响应模型。
func tasksToData(tasks []model.Task) []taskData {
	items := make([]taskData, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskToData(task))
	}
	return items
}

// taskToData 转换单个数据库任务为 API 响应模型。
func taskToData(task model.Task) taskData {
	return taskData{
		ID:           task.ID,
		Type:         task.Type,
		Status:       task.Status,
		Title:        task.Title,
		Progress:     task.Progress,
		ErrorMessage: logger.RedactText(task.ErrorMessage),
		StartedAt:    formatTime(task.StartedAt),
		FinishedAt:   formatTime(task.FinishedAt),
		CreatedAt:    formatTime(task.CreatedAt),
		UpdatedAt:    formatTime(task.UpdatedAt),
	}
}

// eventsToData 转换数据库事件列表为 API 响应模型，并在输出前再次脱敏。
func eventsToData(events []model.TaskEvent) []eventData {
	replayed := eventsToService(events)

	items := make([]eventData, 0, len(replayed))
	for _, event := range taskservice.ReplayEvents(replayed, 0) {
		items = append(items, eventData{
			ID:        event.ID,
			TaskID:    event.TaskID,
			EventType: string(event.Type),
			Payload:   event.Payload,
			CreatedAt: formatTime(event.CreatedAt),
		})
	}
	return items
}

// eventsToService 转换数据库事件为任务 service 事件，并在输出前再次脱敏。
func eventsToService(events []model.TaskEvent) []taskservice.Event {
	replayed := make([]taskservice.Event, 0, len(events))
	for _, event := range events {
		replayed = append(replayed, taskservice.Event{
			ID:        event.ID,
			TaskID:    event.TaskID,
			Type:      taskservice.EventType(event.EventType),
			Payload:   taskservice.SanitizeEventPayload(event.Payload),
			CreatedAt: event.CreatedAt,
			UpdatedAt: event.UpdatedAt,
		})
	}
	return replayed
}

// formatTime 统一输出 RFC3339 时间；零值保留为空字符串，便于前端区分未开始和未完成。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
