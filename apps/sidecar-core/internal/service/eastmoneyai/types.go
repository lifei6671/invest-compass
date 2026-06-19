package eastmoneyai

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Entity 表示东方财富识别出的证券或金融实体。
type Entity struct {
	ClassCode     string
	Code          string
	Exchange      string
	Name          string
	EastMoneyCode string
}

// ReportOption 表示业绩点评可用报告期。
type ReportOption struct {
	ReportDate  string
	PeriodLabel string
}

// EarningsReviewRequest 是业绩点评请求。
type EarningsReviewRequest struct {
	EastMoneyCode string
	ReportDate    string
}

// ArticleResult 表示文章类接口返回的标题、正文和分享链接。
type ArticleResult struct {
	Title    string
	Content  string
	ShareURL string
}

// Markdown 将文章类结果渲染为 Markdown。
func (result ArticleResult) Markdown() string {
	var builder strings.Builder
	if strings.TrimSpace(result.Title) != "" {
		builder.WriteString("### ")
		builder.WriteString(strings.TrimSpace(result.Title))
		builder.WriteString("\n\n")
	}
	if shareURL, ok := articleShareMarkdownDestination(result.ShareURL); ok {
		builder.WriteString("[查看原文](")
		builder.WriteString(shareURL)
		builder.WriteString(")\n\n")
	}
	if strings.TrimSpace(result.Content) != "" {
		builder.WriteString(strings.TrimSpace(result.Content))
	}
	return builder.String()
}

// articleShareMarkdownDestination 将远端分享链接收口为安全的 Markdown link 目标。
func articleShareMarkdownDestination(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	destination := strings.ReplaceAll(trimmed, `\`, `\\`)
	destination = strings.ReplaceAll(destination, ")", `\)`)
	return destination, true
}

// QARequest 是金融问答请求。
type QARequest struct {
	Question  string
	DeepThink bool
}

// QAResult 表示金融问答结果。
type QAResult struct {
	Answer     string
	References []Reference
}

// Reference 表示金融问答引用或扩展阅读。
type Reference struct {
	RefID         int
	Type          string
	ReferenceType string
	Markdown      string
	Title         string
	JumpURL       string
	Source        string
}

// TrackingReportResult 表示跟踪报告返回结构。
type TrackingReportResult struct {
	Title      string
	Content    string
	EntityType string
	ShareURL   string
}

// SearchDataResult 表示金融数据查询结果。
type SearchDataResult struct {
	Tables    []DataTable
	Condition string
	Message   string
}

// Markdown 将金融数据查询结果渲染为 Markdown 表格。
func (result SearchDataResult) Markdown() string {
	if len(result.Tables) == 0 {
		return "未查询到相关金融数据，请尝试更具体的查询条件。"
	}
	var builder strings.Builder
	if strings.TrimSpace(result.Message) != "" {
		builder.WriteString("> ")
		builder.WriteString(strings.TrimSpace(result.Message))
		builder.WriteString("\n\n")
	}
	for index, table := range result.Tables {
		if index > 0 {
			builder.WriteString("\n---\n\n")
		}
		if title := strings.TrimSpace(table.Title); title != "" {
			builder.WriteString("### ")
			builder.WriteString(title)
			builder.WriteString("\n\n")
		}
		rendered := table.Markdown()
		if rendered == "" {
			builder.WriteString("暂无数据\n")
		} else {
			builder.WriteString(rendered)
		}
	}
	if strings.TrimSpace(result.Condition) != "" {
		builder.WriteString("\n\n### 查询条件\n\n")
		builder.WriteString(strings.TrimSpace(result.Condition))
	}
	return builder.String()
}

// hasRenderableTable 判断金融数据结果是否至少包含一张可渲染表格。
func (result SearchDataResult) hasRenderableTable() bool {
	for _, table := range result.Tables {
		if strings.TrimSpace(table.Markdown()) != "" {
			return true
		}
	}
	return false
}

// DataTable 表示东财 AI 返回的一个可渲染数据表。
type DataTable struct {
	Title          string
	EntityName     string
	NameMap        map[string]any
	IndicatorOrder []any
	Table          map[string]any
	Condition      string
}

// Markdown 将单个数据表渲染为 Markdown。
func (table DataTable) Markdown() string {
	if len(table.Table) == 0 {
		return ""
	}
	headers, _ := table.Table["headName"].([]any)
	keys := orderedIndicatorKeys(table.Table, table.IndicatorOrder)
	entityName := strings.TrimSpace(table.EntityName)
	if entityName == "" {
		entityName = "指标"
	}
	if len(headers) > 0 && len(keys) > 0 {
		headerTexts := make([]string, len(headers))
		for index, header := range headers {
			headerTexts[index] = markdownTableCell(flattenValue(header))
		}
		var builder strings.Builder
		builder.WriteString("| ")
		builder.WriteString(markdownTableCell(entityName))
		builder.WriteString(" | ")
		builder.WriteString(strings.Join(headerTexts, " | "))
		builder.WriteString(" |\n")
		builder.WriteString("| ")
		builder.WriteString(strings.Repeat("--- | ", len(headerTexts)+1))
		builder.WriteString("\n")
		for _, key := range keys {
			values, _ := table.Table[key].([]any)
			cells := normalizeValues(values, len(headerTexts))
			builder.WriteString("| ")
			builder.WriteString(markdownTableCell(formatIndicatorLabel(key, table.NameMap)))
			builder.WriteString(" | ")
			builder.WriteString(strings.Join(markdownTableCells(cells), " | "))
			builder.WriteString(" |\n")
		}
		return builder.String()
	}
	return genericTableToMarkdown(table.Table, table.NameMap)
}

// SearchNewsResult 表示金融资讯搜索结果。
type SearchNewsResult struct {
	Content string
}

// ComparableCompanyResult 表示可比公司分析结果。
type ComparableCompanyResult struct {
	Sections []DataTable
}

// Markdown 将可比公司分析结果渲染为 Markdown。
func (result ComparableCompanyResult) Markdown() string {
	var builder strings.Builder
	for _, section := range result.Sections {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = strings.TrimSpace(section.EntityName)
		}
		if title != "" {
			builder.WriteString("### ")
			builder.WriteString(title)
			builder.WriteString("\n\n")
		}
		if markdown := section.Markdown(); markdown != "" {
			builder.WriteString(markdown)
			builder.WriteString("\n")
		}
	}
	if builder.Len() == 0 {
		return "可比公司分析暂无数据"
	}
	return strings.TrimSpace(builder.String())
}

// firstEntityMap 从东财多种实体响应结构中提取第一条实体记录。
func firstEntityMap(raw map[string]any) map[string]any {
	data, _ := raw["data"].(map[string]any)
	if data != nil {
		if entityMetricList, ok := data["entityMetricList"].([]any); ok && len(entityMetricList) > 0 {
			if inner, ok := entityMetricList[0].([]any); ok && len(inner) > 0 {
				if item, ok := inner[0].(map[string]any); ok {
					return item
				}
			}
		}
		if entityList, ok := data["entityList"].([]any); ok && len(entityList) > 0 {
			if item, ok := entityList[0].(map[string]any); ok {
				return item
			}
		}
	}
	if values, ok := raw["data"].([]any); ok && len(values) > 0 {
		if item, ok := values[0].(map[string]any); ok {
			return item
		}
	}
	return nil
}

// entityFromMap 将远端实体字段转换为稳定结构。
func entityFromMap(raw map[string]any) (Entity, error) {
	classCode := stringField(raw, "classCode")
	switch classCode {
	case "002001", "002003", "002004":
	default:
		return Entity{}, fmt.Errorf("unsupported classCode %s", classCode)
	}
	code := stringField(raw, "secuCode")
	if code == "" {
		return Entity{}, fmt.Errorf("missing secuCode")
	}
	exchange := strings.TrimPrefix(stringField(raw, "marketChar"), ".")
	name := stringField(raw, "shortName", "fullName", "secuName")
	entity := Entity{
		ClassCode: classCode,
		Code:      code,
		Exchange:  exchange,
		Name:      name,
	}
	if strings.Contains(code, ".") || exchange == "" {
		entity.EastMoneyCode = code
	} else {
		entity.EastMoneyCode = code + "." + exchange
	}
	return entity, nil
}

// reportOptionFromMap 将多种报告期字段名转换为统一结构。
func reportOptionFromMap(raw map[string]any) ReportOption {
	return ReportOption{
		ReportDate:  stringField(raw, "reportDate", "report_date", "date"),
		PeriodLabel: stringField(raw, "period", "periodLabel"),
	}
}

// articleFromRaw 从远端统一响应中抽取文章类数据。
func articleFromRaw(raw map[string]any) ArticleResult {
	data, _ := raw["data"].(map[string]any)
	return ArticleResult{
		Title:    stringField(data, "title"),
		Content:  stringField(data, "content", "displayData"),
		ShareURL: stringField(data, "shareUrl"),
	}
}

// referencesFromAny 解析金融问答引用列表。
func referencesFromAny(value any) []Reference {
	values, _ := value.([]any)
	result := make([]Reference, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		ref := Reference{
			RefID:         intField(raw, "refId"),
			Type:          stringField(raw, "type"),
			ReferenceType: stringField(raw, "referenceType"),
			Markdown:      stringField(raw, "markdown"),
			Title:         stringField(raw, "title"),
			JumpURL:       stringField(raw, "jumpUrl"),
			Source:        stringField(raw, "source"),
		}
		if ref.Source == "" {
			if nested, ok := raw["data"].(map[string]any); ok {
				ref.Source = stringField(nested, "source")
			}
		}
		result = append(result, ref)
	}
	return result
}

// searchDataResultFromRaw 从远端金融数据响应中抽取表格列表和查询条件。
func searchDataResultFromRaw(raw map[string]any) SearchDataResult {
	var values []any
	data, _ := raw["data"].(map[string]any)
	if data != nil {
		if wrapper, ok := data["searchDataResultDTO"].(map[string]any); ok {
			values, _ = wrapper["dataTableDTOList"].([]any)
		}
		if len(values) == 0 {
			values, _ = data["dataTableDTOList"].([]any)
		}
	}
	if len(values) == 0 {
		values, _ = raw["dataTableDTOList"].([]any)
	}
	result := SearchDataResult{}
	if data != nil {
		result.Message = stringField(data, "message")
	}
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			table := dataTableFromMap(item)
			result.Tables = append(result.Tables, table)
			if table.Condition != "" {
				name := table.EntityName
				if name == "" {
					name = table.Title
				}
				result.Condition = appendCondition(result.Condition, name, table.Condition)
			}
		}
	}
	return result
}

// dataTableFromMap 转换东财表格 DTO。
func dataTableFromMap(raw map[string]any) DataTable {
	nameMap, _ := raw["nameMap"].(map[string]any)
	indicatorOrder, _ := raw["indicatorOrder"].([]any)
	table, _ := raw["table"].(map[string]any)
	return DataTable{
		Title:          stringField(raw, "title", "inputTitle"),
		EntityName:     stringField(raw, "entityName"),
		NameMap:        nameMap,
		IndicatorOrder: indicatorOrder,
		Table:          table,
		Condition:      stringField(raw, "condition"),
	}
}

// appendCondition 合并多张表的查询条件说明。
func appendCondition(existing string, name string, condition string) string {
	part := strings.TrimSpace(condition)
	if strings.TrimSpace(name) != "" {
		part = "[" + strings.TrimSpace(name) + "]\n" + part
	}
	if existing == "" {
		return part
	}
	return existing + "\n\n" + part
}

// orderedIndicatorKeys 根据远端建议顺序排序表格指标。
func orderedIndicatorKeys(table map[string]any, order []any) []string {
	keySet := make(map[string]bool)
	for key := range table {
		if key != "headName" {
			keySet[key] = true
		}
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(keySet))
	for _, item := range order {
		key := flattenValue(item)
		if keySet[key] && !seen[key] {
			result = append(result, key)
			seen[key] = true
		}
	}
	remaining := make([]string, 0, len(keySet))
	for key := range keySet {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(result, remaining...)
}

// formatIndicatorLabel 返回指标展示名，缺失时回退原始 key。
func formatIndicatorLabel(key string, nameMap map[string]any) string {
	if nameMap != nil {
		if value, ok := nameMap[key]; ok {
			if label := strings.TrimSpace(flattenValue(value)); label != "" {
				return label
			}
		}
	}
	return key
}

// genericTableToMarkdown 渲染没有 headName 的通用二维表。
func genericTableToMarkdown(table map[string]any, nameMap map[string]any) string {
	keys := make([]string, 0, len(table))
	columns := make([][]string, 0, len(table))
	for key, value := range table {
		values, ok := value.([]any)
		if !ok {
			continue
		}
		keys = append(keys, key)
		column := make([]string, len(values))
		for index, item := range values {
			column[index] = flattenValue(item)
		}
		columns = append(columns, column)
	}
	if len(columns) == 0 {
		return ""
	}
	rowCount := len(columns[0])
	for _, column := range columns {
		if len(column) != rowCount {
			return ""
		}
	}
	headers := make([]string, len(keys))
	for index, key := range keys {
		headers[index] = markdownTableCell(formatIndicatorLabel(key, nameMap))
	}
	var builder strings.Builder
	builder.WriteString("| ")
	builder.WriteString(strings.Join(headers, " | "))
	builder.WriteString(" |\n| ")
	builder.WriteString(strings.Repeat("--- | ", len(headers)))
	builder.WriteString("\n")
	for row := 0; row < rowCount; row++ {
		cells := make([]string, len(columns))
		for column := range columns {
			cells[column] = markdownTableCell(columns[column][row])
		}
		builder.WriteString("| ")
		builder.WriteString(strings.Join(cells, " | "))
		builder.WriteString(" |\n")
	}
	return builder.String()
}

// markdownTableCells 批量转义 Markdown 表格单元。
func markdownTableCells(values []string) []string {
	escaped := make([]string, len(values))
	for index, value := range values {
		escaped[index] = markdownTableCell(value)
	}
	return escaped
}

// normalizeValues 将远端数组补齐到表头长度。
func normalizeValues(values []any, expected int) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = flattenValue(value)
	}
	for len(result) < expected {
		result = append(result, "")
	}
	if len(result) > expected {
		return result[:expected]
	}
	return result
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

// extractSearchContent 从多层搜索响应中提取正文。
func extractSearchContent(raw map[string]any) string {
	for _, key := range []string{"llmSearchResponse", "searchResponse", "content", "answer", "summary"} {
		if value := strings.TrimSpace(stringField(raw, key)); value != "" {
			return value
		}
	}
	if data, ok := raw["data"].(map[string]any); ok {
		return extractSearchContent(data)
	}
	return ""
}

// numericField 读取 JSON 数值字段。
func numericField(raw map[string]any, key string) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	switch value := raw[key].(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// intField 读取 JSON 整数字段。
func intField(raw map[string]any, key string) int {
	value, ok := numericField(raw, key)
	if !ok {
		return 0
	}
	return int(value)
}

// stringField 按顺序读取第一个非空字符串字段。
func stringField(raw map[string]any, keys ...string) string {
	if raw == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		text := flattenValue(value)
		if strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// flattenValue 将 JSON 值转换为 Markdown 友好的字符串。
func flattenValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%v", typed)
	case map[string]any, []any:
		payload, _ := json.Marshal(typed)
		return string(payload)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
