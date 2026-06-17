package report

import (
	"sort"
	"strings"
	"time"
)

// ErrorCode 是报告模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorReportNotFound 表示报告不存在或已软删除。
	ErrorReportNotFound ErrorCode = "report_not_found"
)

// Error 表示报告模块规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免泄露报告输入快照。
func (err *Error) Error() string {
	return string(err.Code)
}

// Report 是分析报告的业务模型，字段对应 analysis_reports 表的首版核心列。
type Report struct {
	ID               int64
	TaskID           string
	Symbol           string
	Title            string
	AnalysisType     string
	ModelName        string
	PromptTemplateID int64
	InputSnapshot    string
	ContentMarkdown  string
	RiskSummary      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// ExportOptions 控制 Markdown 导出是否包含敏感输入快照。
type ExportOptions struct {
	IncludeInputSnapshot bool
}

// UniqueByTaskID 对报告按 task_id 去重，保留 updated_at 最新的一条。
func UniqueByTaskID(reports []Report) []Report {
	latestByTaskID := make(map[string]Report, len(reports))
	for _, report := range reports {
		existing, ok := latestByTaskID[report.TaskID]
		if !ok || report.UpdatedAt.After(existing.UpdatedAt) {
			latestByTaskID[report.TaskID] = report
		}
	}

	uniqueReports := make([]Report, 0, len(latestByTaskID))
	for _, report := range latestByTaskID {
		uniqueReports = append(uniqueReports, report)
	}
	sort.SliceStable(uniqueReports, func(left int, right int) bool {
		return uniqueReports[left].UpdatedAt.After(uniqueReports[right].UpdatedAt)
	})
	return uniqueReports
}

// VisibleReports 过滤软删除报告，供列表和详情查询复用。
func VisibleReports(reports []Report) []Report {
	visible := make([]Report, 0, len(reports))
	for _, report := range reports {
		if report.DeletedAt != nil {
			continue
		}
		visible = append(visible, report)
	}
	return visible
}

// FindVisibleReport 按报告 ID 查找未软删除报告，供详情 API 复用同一可见性规则。
func FindVisibleReport(reports []Report, id int64) (Report, error) {
	for _, report := range reports {
		if report.ID != id || report.DeletedAt != nil {
			continue
		}
		return report, nil
	}
	return Report{}, &Error{Code: ErrorReportNotFound}
}

// ExportMarkdown 导出报告 Markdown，默认不包含完整 input_snapshot。
func ExportMarkdown(report Report, options ExportOptions) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(report.Title)
	builder.WriteString("\n\n")

	if report.Symbol != "" {
		builder.WriteString("- 股票代码：")
		builder.WriteString(report.Symbol)
		builder.WriteString("\n")
	}
	if report.AnalysisType != "" {
		builder.WriteString("- 分析类型：")
		builder.WriteString(report.AnalysisType)
		builder.WriteString("\n")
	}
	if !report.CreatedAt.IsZero() {
		builder.WriteString("- 生成时间：")
		builder.WriteString(report.CreatedAt.UTC().Format(time.RFC3339))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(strings.TrimSpace(report.ContentMarkdown))
	builder.WriteString("\n")

	if strings.TrimSpace(report.RiskSummary) != "" {
		builder.WriteString("\n## 风险摘要\n\n")
		builder.WriteString(strings.TrimSpace(report.RiskSummary))
		builder.WriteString("\n")
	}

	if options.IncludeInputSnapshot && strings.TrimSpace(report.InputSnapshot) != "" {
		builder.WriteString("\n## 输入快照\n\n")
		builder.WriteString("```json\n")
		builder.WriteString(strings.TrimSpace(report.InputSnapshot))
		builder.WriteString("\n```\n")
	}
	return builder.String()
}
