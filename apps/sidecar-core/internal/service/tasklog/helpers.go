package tasklog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// sanitizeJSON 对日志 JSON 详情执行二次脱敏；无法解析的内容按普通文本保存。
func sanitizeJSON(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return logger.RedactText(trimmed)
}

// normalizeOption 将前端“全部”选项归一为空值，避免 DAO 误按字面量过滤。
func normalizeOption(value string, allLabel string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == allLabel {
		return ""
	}
	return trimmed
}

func entryToRow(entry model.TaskLogEntry) Row {
	return Row{
		ID:         entry.ID,
		TaskID:     entry.TaskID,
		Time:       formatClock(entry.Ts),
		Timestamp:  formatTime(entry.Ts),
		Level:      entry.Level,
		Module:     entry.Module,
		Stage:      entry.Stage,
		Message:    logger.RedactText(entry.Message),
		Code:       logger.RedactText(entry.Code),
		Provider:   logger.RedactText(entry.Provider),
		Model:      logger.RedactText(entry.Model),
		Symbol:     logger.RedactText(entry.Symbol),
		DurationMS: entry.DurationMS,
		Retryable:  entry.Retryable,
	}
}

// rawJSONForEntry 生成单条日志详情面板使用的脱敏 JSON。
func rawJSONForEntry(entry model.TaskLogEntry) string {
	payload := map[string]any{}
	if strings.TrimSpace(entry.PayloadJSON) != "" {
		var decoded any
		if err := json.Unmarshal([]byte(entry.PayloadJSON), &decoded); err == nil {
			payload["payload"] = decoded
		} else {
			payload["payload_text"] = logger.RedactText(entry.PayloadJSON)
		}
	}
	payload["id"] = entry.ID
	payload["timestamp"] = formatTime(entry.Ts)
	payload["level"] = entry.Level
	payload["module"] = entry.Module
	payload["stage"] = entry.Stage
	payload["message"] = logger.RedactText(entry.Message)
	payload["code"] = logger.RedactText(entry.Code)
	payload["provider"] = logger.RedactText(entry.Provider)
	payload["model"] = logger.RedactText(entry.Model)
	payload["symbol"] = logger.RedactText(entry.Symbol)
	payload["duration_ms"] = entry.DurationMS
	payload["retryable"] = entry.Retryable
	payload["request_id"] = logger.RedactText(entry.RequestID)
	payload["trace_id"] = logger.RedactText(entry.TraceID)

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{}"
	}
	return logger.RedactText(string(encoded))
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatClock(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("15:04:05.000")
}

func taskDuration(task model.Task) string {
	if task.StartedAt.IsZero() {
		return "—"
	}
	end := task.FinishedAt
	if end.IsZero() {
		end = task.UpdatedAt
	}
	if end.IsZero() || end.Before(task.StartedAt) {
		return "—"
	}
	duration := end.Sub(task.StartedAt).Round(time.Second)
	hours := int(duration / time.Hour)
	minutes := int((duration % time.Hour) / time.Minute)
	seconds := int((duration % time.Minute) / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func marshalStringList(values []string) (string, error) {
	safe := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			safe = append(safe, logger.RedactText(trimmed))
		}
	}
	encoded, err := json.Marshal(safe)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func diagnosisFromModel(value model.TaskErrorDiagnosis) Diagnosis {
	return Diagnosis{
		TaskID:      value.TaskID,
		ErrorCode:   logger.RedactText(value.ErrorCode),
		ErrorStage:  logger.RedactText(value.ErrorStage),
		Summary:     logger.RedactText(value.Summary),
		Causes:      unmarshalStringList(value.CausesJSON),
		Suggestions: unmarshalStringList(value.SuggestionsJSON),
		Retryable:   value.Retryable,
		RequestID:   logger.RedactText(value.RequestID),
		TraceID:     logger.RedactText(value.TraceID),
		SourceLogID: value.SourceLogID,
	}
}

func unmarshalStringList(value string) []string {
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil
	}
	for index, item := range values {
		values[index] = logger.RedactText(item)
	}
	return values
}

// buildDiagnosisFromError 用最后一条 ERROR 日志生成保守诊断，避免前端空白。
func buildDiagnosisFromError(entry model.TaskLogEntry) Diagnosis {
	message := strings.ToLower(entry.Message + " " + entry.Code + " " + entry.Stage)
	timeout := strings.Contains(message, "timeout") || strings.Contains(message, "超时")
	summary := "任务执行失败，请根据错误阶段和日志详情排查。"
	if timeout {
		summary = "模型服务响应超时，可检查代理、API Key 或更换模型后重试。"
	}
	return Diagnosis{
		TaskID:     entry.TaskID,
		ErrorCode:  logger.RedactText(entry.Code),
		ErrorStage: logger.RedactText(entry.Stage),
		Summary:    summary,
		Causes: []string{
			"模型服务响应超时或网络延迟过高",
			"代理配置异常或 Provider 暂时不可用",
			"请求输出内容较长，超过模型响应时间",
		},
		Suggestions: []string{
			"检查代理设置并重新测试连接",
			"更换 AI 模型后重试",
			"降低最大输出 Token 后重新发起分析",
		},
		Retryable:   entry.Retryable || timeout,
		RequestID:   logger.RedactText(entry.RequestID),
		TraceID:     logger.RedactText(entry.TraceID),
		SourceLogID: entry.ID,
	}
}

func snapshotBrief(value string) string {
	redacted := logger.RedactText(strings.TrimSpace(value))
	if len([]rune(redacted)) <= 160 {
		return redacted
	}
	runes := []rune(redacted)
	return string(runes[:160]) + "..."
}

func inferLoadedStatus(snapshot string, key string) string {
	if strings.Contains(strings.ToLower(snapshot), strings.ToLower(key)) {
		return "已加载"
	}
	return "未记录"
}

func buildExportContent(summary Summary, events []model.TaskEvent, rows []Row, diagnosis Diagnosis, contextSummary ContextSummary) string {
	var builder strings.Builder
	builder.WriteString("# 投研罗盘任务脱敏日志\n\n")
	builder.WriteString("## 任务摘要\n")
	builder.WriteString(fmt.Sprintf("- 标题：%s\n", logger.RedactText(summary.Title)))
	builder.WriteString(fmt.Sprintf("- 任务 ID：%s\n", logger.RedactText(summary.TaskID)))
	builder.WriteString(fmt.Sprintf("- 任务类型：%s\n", logger.RedactText(summary.TaskType)))
	builder.WriteString(fmt.Sprintf("- 状态：%s\n", logger.RedactText(summary.Status)))
	builder.WriteString(fmt.Sprintf("- 开始时间：%s\n", logger.RedactText(summary.StartedAt)))
	builder.WriteString(fmt.Sprintf("- 耗时：%s\n\n", logger.RedactText(summary.Duration)))

	builder.WriteString("## 事件流\n")
	for _, event := range events {
		builder.WriteString(fmt.Sprintf("- %s %s %s\n", formatTime(event.CreatedAt), logger.RedactText(event.EventType), snapshotBrief(event.Payload)))
	}

	builder.WriteString("\n## 执行日志\n")
	for _, row := range rows {
		builder.WriteString(fmt.Sprintf("- %s [%s] %s/%s %s\n", row.Time, row.Level, row.Module, row.Stage, logger.RedactText(row.Message)))
	}

	builder.WriteString("\n## 错误诊断\n")
	builder.WriteString(fmt.Sprintf("- 摘要：%s\n", logger.RedactText(diagnosis.Summary)))
	for _, suggestion := range diagnosis.Suggestions {
		builder.WriteString(fmt.Sprintf("- 建议：%s\n", logger.RedactText(suggestion)))
	}

	builder.WriteString("\n## 上下文摘要\n")
	builder.WriteString(fmt.Sprintf("- 股票：%s\n", logger.RedactText(contextSummary.Stock)))
	builder.WriteString(fmt.Sprintf("- 分析类型：%s\n", logger.RedactText(contextSummary.AnalysisType)))
	builder.WriteString(fmt.Sprintf("- 使用模型：%s\n", logger.RedactText(contextSummary.Model)))
	builder.WriteString(fmt.Sprintf("- 数据更新时间：%s\n", logger.RedactText(contextSummary.DataUpdatedAt)))
	return logger.RedactText(builder.String())
}

var unsafeFilePart = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFilePart(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '_'
		}
		return r
	}, trimmed)
	safe := unsafeFilePart.ReplaceAllString(trimmed, "_")
	safe = strings.Trim(safe, "._-")
	if safe == "" {
		return "task"
	}
	return safe
}
