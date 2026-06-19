package logexport

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const defaultMemorySourceCapacity = 500

// MemorySource 采集 Go core 运行期结构化日志，并作为日志导出的生产数据源。
type MemorySource struct {
	next     slog.Handler
	capacity int
	attrs    []slog.Attr
	groups   []string
	state    *memoryState
}

type memoryState struct {
	mutex sync.Mutex
	lines []string
}

// NewMemorySource 创建固定容量的内存日志源，next 用于保留原有日志输出链路。
func NewMemorySource(next slog.Handler, capacity int) *MemorySource {
	if capacity <= 0 {
		capacity = defaultMemorySourceCapacity
	}
	return &MemorySource{
		next:     next,
		capacity: capacity,
		state:    &memoryState{},
	}
}

// Enabled 判断当前日志级别是否需要处理。
func (source *MemorySource) Enabled(ctx context.Context, level slog.Level) bool {
	if source.next == nil {
		return true
	}
	return source.next.Enabled(ctx, level)
}

// Handle 记录一行已脱敏结构化日志，并继续交给下游 handler 输出。
func (source *MemorySource) Handle(ctx context.Context, record slog.Record) error {
	source.appendLine(record)
	if source.next != nil {
		return source.next.Handle(ctx, redactedRecord(record))
	}
	return nil
}

// WithAttrs 返回携带固定属性的新 handler，符合 slog.Handler 契约。
func (source *MemorySource) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := slog.Handler(nil)
	if source.next != nil {
		next = source.next.WithAttrs(redactedAttrs(attrs))
	}
	return &MemorySource{
		next:     next,
		capacity: source.capacity,
		attrs:    append(append([]slog.Attr(nil), source.attrs...), attrs...),
		groups:   append([]string(nil), source.groups...),
		state:    source.state,
	}
}

// WithGroup 返回携带分组名的新 handler；导出文本保留扁平字段便于排障搜索。
func (source *MemorySource) WithGroup(name string) slog.Handler {
	next := slog.Handler(nil)
	if source.next != nil {
		next = source.next.WithGroup(name)
	}
	groups := append([]string(nil), source.groups...)
	if strings.TrimSpace(name) != "" {
		groups = append(groups, name)
	}
	return &MemorySource{
		next:     next,
		capacity: source.capacity,
		attrs:    append([]slog.Attr(nil), source.attrs...),
		groups:   groups,
		state:    source.state,
	}
}

// ExportLogRequest 返回当前内存日志快照，后续 BuildBundle 会再次执行脱敏。
func (source *MemorySource) ExportLogRequest(context.Context) (Request, error) {
	source.state.mutex.Lock()
	defer source.state.mutex.Unlock()

	return Request{
		Lines:     append([]string(nil), source.state.lines...),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// appendLine 将 slog.Record 转成便于导出和搜索的单行文本。
func (source *MemorySource) appendLine(record slog.Record) {
	parts := []string{
		"time=" + record.Time.UTC().Format(time.RFC3339Nano),
		"level=" + record.Level.String(),
		"message=" + quoteLogValue(record.Message),
	}
	for _, attr := range source.attrs {
		parts = append(parts, formatAttr(attr))
	}
	record.Attrs(func(attr slog.Attr) bool {
		parts = append(parts, formatAttr(attr))
		return true
	})
	if len(source.groups) > 0 {
		parts = append(parts, "groups="+quoteLogValue(strings.Join(source.groups, ".")))
	}

	source.state.mutex.Lock()
	defer source.state.mutex.Unlock()
	source.state.lines = append(source.state.lines, logger.RedactText(strings.Join(parts, " ")))
	if len(source.state.lines) > source.capacity {
		source.state.lines = append([]string(nil), source.state.lines[len(source.state.lines)-source.capacity:]...)
	}
}

// formatAttr 将 slog attr 扁平化为 key=value，便于导出文本保留稳定字段名。
func formatAttr(attr slog.Attr) string {
	attr.Value = attr.Value.Resolve()
	if sensitiveLogKey(attr.Key) {
		return attr.Key + "=" + logger.RedactedValue
	}
	return attr.Key + "=" + quoteLogValue(fmt.Sprint(attr.Value.Any()))
}

// redactedRecord 克隆 slog record 后脱敏结构化字段，再交给下游 handler 输出。
func redactedRecord(record slog.Record) slog.Record {
	redacted := slog.NewRecord(record.Time, record.Level, logger.RedactText(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactedAttr(attr))
		return true
	})
	return redacted
}

// redactedAttrs 脱敏 WithAttrs 固定字段，避免派生 logger 旁路下游脱敏。
func redactedAttrs(attrs []slog.Attr) []slog.Attr {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactedAttr(attr))
	}
	return redacted
}

// redactedAttr 按字段名和字段值双重脱敏，保留非敏感排障字段。
func redactedAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if sensitiveLogKey(attr.Key) {
		return slog.String(attr.Key, logger.RedactedValue)
	}
	if attr.Value.Kind() == slog.KindGroup {
		return slog.Group(attr.Key, attrsToAny(redactedAttrs(attr.Value.Group()))...)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, logger.RedactText(attr.Value.String()))
	}
	if attr.Value.Kind() == slog.KindAny {
		return slog.String(attr.Key, logger.RedactText(fmt.Sprint(attr.Value.Any())))
	}
	return attr
}

// attrsToAny 适配 slog.Group 的 variadic any 入参。
func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		values = append(values, attr)
	}
	return values
}

// sensitiveLogKey 判断结构化日志字段名是否属于首版禁止导出的敏感字段。
func sensitiveLogKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	switch normalized {
	case "authorization",
		"proxy_authorization",
		"api_key",
		"apikey",
		"license_key",
		"licensekey",
		"proxy_password",
		"proxypassword",
		"position_snapshot",
		"position_input",
		"holding_input",
		"user_position",
		"userposition",
		"portfolio":
		return true
	default:
		return false
	}
}

// quoteLogValue 对包含空白的值加引号，保持导出文本可读。
func quoteLogValue(value string) string {
	if strings.ContainsAny(value, " \t\n\r") {
		return fmt.Sprintf("%q", value)
	}
	return value
}
