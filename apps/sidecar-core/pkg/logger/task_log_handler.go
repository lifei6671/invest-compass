package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TaskLogWriter 是任务结构化日志写入边界，通常由 tasklog.Service 或 AsyncWriter 实现。
type TaskLogWriter interface {
	WriteTaskLog(ctx context.Context, entry model.TaskLogEntry) error
}

// TaskLogHandler 将带 task_id 的 slog.Record 转换为 TaskLogEntry。
//
// 普通应用日志不携带 task_id 时只交给 next handler，不写入任务日志表。
type TaskLogHandler struct {
	next   slog.Handler
	writer TaskLogWriter
	attrs  []slog.Attr
	groups []string
}

// NewTaskLogHandler 创建任务日志转换 handler。
func NewTaskLogHandler(next slog.Handler, writer TaskLogWriter) *TaskLogHandler {
	return &TaskLogHandler{next: next, writer: writer}
}

// Enabled 复用 next handler 的级别判断；无 next 时默认开启。
func (handler *TaskLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if handler.next == nil {
		return true
	}
	return handler.next.Enabled(ctx, level)
}

// Handle 处理 slog.Record；只有带 task_id 的记录会写入任务结构化日志。
func (handler *TaskLogHandler) Handle(ctx context.Context, record slog.Record) error {
	var nextErr error
	if handler.next != nil {
		nextErr = handler.next.Handle(ctx, record)
	}
	if handler.writer == nil {
		return nextErr
	}

	values := map[string]any{}
	groupPrefix := strings.Join(handler.groups, ".")
	for _, attr := range handler.attrs {
		flattenSlogAttr(values, groupPrefix, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		flattenSlogAttr(values, groupPrefix, attr)
		return true
	})

	taskID := stringValue(values, FieldTaskID)
	if taskID == "" {
		return nextErr
	}
	module := stringValue(values, FieldModule)
	stage := stringValue(values, FieldStage)
	if module == "" || stage == "" {
		return fmt.Errorf("task log requires module and stage")
	}

	payloadJSON, err := taskLogPayloadJSON(values)
	if err != nil {
		return err
	}
	entry := model.TaskLogEntry{
		TaskID:      taskID,
		RequestID:   stringValue(values, FieldRequestID),
		TraceID:     stringValue(values, FieldTraceID),
		Ts:          record.Time.UTC(),
		Level:       taskLogLevel(record.Level),
		Module:      module,
		Stage:       stage,
		Message:     RedactText(record.Message),
		Code:        stringValue(values, FieldCode),
		Provider:    stringValue(values, FieldProvider),
		Model:       stringValue(values, FieldModel),
		Symbol:      stringValue(values, FieldSymbol),
		DurationMS:  int64Value(values, FieldDurationMS),
		Retryable:   boolValue(values, FieldRetryable),
		PayloadJSON: payloadJSON,
	}
	if entry.Ts.IsZero() {
		entry.Ts = time.Now().UTC()
	}
	if err := handler.writer.WriteTaskLog(ctx, entry); err != nil {
		return err
	}
	return nextErr
}

// WithAttrs 返回携带额外属性的 handler。
func (handler *TaskLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := handler.next
	if next != nil {
		next = next.WithAttrs(attrs)
	}
	cloned := *handler
	cloned.next = next
	cloned.attrs = append(append([]slog.Attr{}, handler.attrs...), attrs...)
	return &cloned
}

// WithGroup 返回带分组的 handler；任务日志 payload 会保留分组前缀。
func (handler *TaskLogHandler) WithGroup(name string) slog.Handler {
	next := handler.next
	if next != nil {
		next = next.WithGroup(name)
	}
	cloned := *handler
	cloned.next = next
	cloned.groups = append(append([]string{}, handler.groups...), name)
	return &cloned
}

func flattenSlogAttr(values map[string]any, prefix string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Key == "" {
		return
	}
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + attr.Key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			flattenSlogAttr(values, key, child)
		}
		return
	}
	values[key] = attr.Value.Any()
}

func taskLogPayloadJSON(values map[string]any) (string, error) {
	payload := map[string]any{}
	for key, value := range values {
		if isTaskLogReservedField(key) {
			continue
		}
		payload[key] = value
	}
	if len(payload) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return RedactText(string(encoded)), nil
}

func isTaskLogReservedField(key string) bool {
	field := key
	if index := strings.LastIndex(key, "."); index >= 0 {
		field = key[index+1:]
	}
	switch key {
	case FieldTaskID, FieldRequestID, FieldTraceID, FieldModule, FieldStage,
		FieldProvider, FieldModel, FieldSymbol, FieldCode, FieldDurationMS, FieldRetryable:
		return true
	default:
		switch field {
		case FieldTaskID, FieldRequestID, FieldTraceID, FieldModule, FieldStage,
			FieldProvider, FieldModel, FieldSymbol, FieldCode, FieldDurationMS, FieldRetryable:
			return true
		default:
			return false
		}
	}
}

func taskLogLevel(level slog.Level) string {
	if level >= slog.LevelError {
		return "ERROR"
	}
	if level >= slog.LevelWarn {
		return "WARN"
	}
	return "INFO"
}

func stringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		value, ok = groupedValue(values, key)
	}
	if !ok || value == nil {
		return ""
	}
	return RedactText(fmt.Sprint(value))
}

func int64Value(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		value, ok = groupedValue(values, key)
	}
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case time.Duration:
		return typed.Milliseconds()
	default:
		return 0
	}
}

func boolValue(values map[string]any, key string) bool {
	value, ok := values[key]
	if !ok {
		value, ok = groupedValue(values, key)
	}
	if !ok || value == nil {
		return false
	}
	typed, ok := value.(bool)
	return ok && typed
}

func groupedValue(values map[string]any, key string) (any, bool) {
	suffix := "." + key
	for candidate, value := range values {
		if strings.HasSuffix(candidate, suffix) {
			return value, true
		}
	}
	return nil, false
}
