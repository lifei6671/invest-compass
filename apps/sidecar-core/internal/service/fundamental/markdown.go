package fundamental

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RenderMarkdown 将结构化基本面 Dataset 渲染为适合 AI 上下文使用的 Markdown。
func RenderMarkdown(dataset Dataset) string {
	title := strings.TrimSpace(dataset.Title)
	if title == "" {
		title = string(dataset.Kind)
	}
	if len(dataset.Rows) == 0 {
		return fmt.Sprintf("## %s\n\n暂无数据", title)
	}

	columns := visibleColumns(dataset.Columns)
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("## %s\n\n", title))
	if len(dataset.Rows) == 1 {
		builder.WriteString("| 指标 | 数值 |\n| --- | --- |\n")
		row := dataset.Rows[0]
		for _, column := range columns {
			builder.WriteString(fmt.Sprintf("| %s | %s |\n", markdownTableCell(column.Label), markdownTableCell(formatValue(column, row[column.Key]))))
		}
		return builder.String()
	}

	builder.WriteString("| ")
	for _, column := range columns {
		builder.WriteString(markdownTableCell(column.Label) + " | ")
	}
	builder.WriteString("\n| ")
	for range columns {
		builder.WriteString("--- | ")
	}
	builder.WriteString("\n")
	for _, row := range dataset.Rows {
		builder.WriteString("| ")
		for _, column := range columns {
			builder.WriteString(markdownTableCell(formatValue(column, row[column.Key])) + " | ")
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

// visibleColumns 过滤隐藏字段，并补齐空白显示名。
func visibleColumns(columns []Column) []Column {
	visible := make([]Column, 0, len(columns))
	for _, column := range columns {
		if column.Hidden || strings.TrimSpace(column.Key) == "" {
			continue
		}
		if strings.TrimSpace(column.Label) == "" {
			column.Label = column.Key
		}
		visible = append(visible, column)
	}
	return visible
}

// formatValue 根据列格式生成稳定展示文本。
func formatValue(column Column, value any) string {
	if value == nil {
		return "-"
	}
	switch column.Format {
	case FormatMoney:
		parsed, ok := asFloat(value)
		if !ok {
			return "-"
		}
		return formatMoney(parsed)
	case FormatVolume:
		parsed, ok := asFloat(value)
		if !ok {
			return "-"
		}
		return formatVolume(parsed)
	case FormatPercent:
		parsed, ok := asFloat(value)
		if !ok {
			return "-"
		}
		return fmt.Sprintf("%.2f%%", parsed)
	case FormatPrice:
		parsed, ok := asFloat(value)
		if !ok {
			return "-"
		}
		return fmt.Sprintf("%.2f", parsed)
	case FormatInteger:
		parsed, ok := asFloat(value)
		if !ok {
			return "-"
		}
		return strconv.FormatInt(int64(math.Round(parsed)), 10)
	case FormatDate:
		return formatDate(value)
	default:
		return formatPlain(value)
	}
}

// asFloat 将 JSON 数值和常见字符串数值转换为 float64，失败时返回 ok=false。
func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" || trimmed == "-" || trimmed == "--" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// formatMoney 按中文金融语境压缩金额单位。
func formatMoney(value float64) string {
	absolute := math.Abs(value)
	if absolute >= 1e8 {
		return fmt.Sprintf("%.2f亿", value/1e8)
	}
	if absolute >= 1e4 {
		return fmt.Sprintf("%.2f万", value/1e4)
	}
	return fmt.Sprintf("%.2f", value)
}

// formatVolume 按股数单位压缩数量。
func formatVolume(value float64) string {
	absolute := math.Abs(value)
	if absolute >= 1e8 {
		return fmt.Sprintf("%.2f亿股", value/1e8)
	}
	if absolute >= 1e4 {
		return fmt.Sprintf("%.2f万股", value/1e4)
	}
	return fmt.Sprintf("%.0f股", value)
}

// formatDate 只保留日期部分，兼容远端常见的日期时间字符串。
func formatDate(value any) string {
	text := formatPlain(value)
	if index := strings.Index(text, " "); index > 0 {
		return text[:index]
	}
	return text
}

// formatPlain 生成普通文本展示。
func formatPlain(value any) string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "-"
		}
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return fmt.Sprintf("%.2f", typed)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// markdownTableCell 转义 Markdown 表格单元，避免远端文本改变表格列结构。
func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "\r", "<br>")
	return value
}
