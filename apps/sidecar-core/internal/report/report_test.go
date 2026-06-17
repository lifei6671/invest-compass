package report

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestUniqueByTaskIDKeepsLatestReport 验证同一 task_id 只保留更新时间最新的报告。
func TestUniqueByTaskIDKeepsLatestReport(t *testing.T) {
	oldReport := Report{
		ID:              1,
		TaskID:          "task-1",
		Title:           "旧报告",
		ContentMarkdown: "old",
		UpdatedAt:       time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
	}
	latestReport := Report{
		ID:              2,
		TaskID:          "task-1",
		Title:           "新报告",
		ContentMarkdown: "latest",
		UpdatedAt:       time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC),
	}

	reports := UniqueByTaskID([]Report{oldReport, latestReport})

	if len(reports) != 1 {
		t.Fatalf("expected one report, got %d", len(reports))
	}
	if reports[0].ID != latestReport.ID || reports[0].ContentMarkdown != "latest" {
		t.Fatalf("expected latest report, got %+v", reports[0])
	}
}

// TestVisibleReportsFiltersSoftDeleted 验证软删除报告不会出现在报告列表。
func TestVisibleReportsFiltersSoftDeleted(t *testing.T) {
	deletedAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	reports := VisibleReports([]Report{
		{ID: 1, TaskID: "task-visible", Title: "可见报告"},
		{ID: 2, TaskID: "task-deleted", Title: "已删报告", DeletedAt: &deletedAt},
	})

	if len(reports) != 1 {
		t.Fatalf("expected one visible report, got %d", len(reports))
	}
	if reports[0].TaskID != "task-visible" {
		t.Fatalf("unexpected visible report: %+v", reports[0])
	}
}

// TestFindVisibleReportRejectsSoftDeleted 验证详情查询不能返回已软删除报告。
func TestFindVisibleReportRejectsSoftDeleted(t *testing.T) {
	deletedAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	_, err := FindVisibleReport([]Report{
		{ID: 1, TaskID: "task-visible", Title: "可见报告"},
		{ID: 2, TaskID: "task-deleted", Title: "已删报告", DeletedAt: &deletedAt},
	}, 2)

	assertReportErrorCode(t, err, ErrorReportNotFound)
}

// TestFindVisibleReportReturnsVisibleReport 验证详情查询只返回未软删除报告。
func TestFindVisibleReportReturnsVisibleReport(t *testing.T) {
	report, err := FindVisibleReport([]Report{
		{ID: 1, TaskID: "task-visible", Title: "可见报告"},
	}, 1)
	if err != nil {
		t.Fatalf("FindVisibleReport returned error: %v", err)
	}
	if report.TaskID != "task-visible" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

// TestExportMarkdownExcludesInputSnapshotByDefault 验证默认 Markdown 导出不包含完整 input_snapshot 和 userPosition。
func TestExportMarkdownExcludesInputSnapshotByDefault(t *testing.T) {
	report := Report{
		Title:           "贵州茅台分析",
		Symbol:          "600519.SH",
		AnalysisType:    "stock_full",
		ContentMarkdown: "## 结论\n仅作研究辅助，不构成投资建议。",
		RiskSummary:     "波动风险",
		InputSnapshot:   `{"userPosition":{"costPrice":100},"note":"private"}`,
		CreatedAt:       time.Date(2026, 6, 17, 12, 30, 0, 0, time.UTC),
	}

	markdown := ExportMarkdown(report, ExportOptions{})

	if !strings.Contains(markdown, "贵州茅台分析") || !strings.Contains(markdown, "600519.SH") {
		t.Fatalf("expected report metadata in markdown: %s", markdown)
	}
	if !strings.Contains(markdown, "仅作研究辅助") || !strings.Contains(markdown, "波动风险") {
		t.Fatalf("expected content and risk summary in markdown: %s", markdown)
	}
	if strings.Contains(markdown, "userPosition") || strings.Contains(markdown, "costPrice") || strings.Contains(markdown, "private") {
		t.Fatalf("markdown leaked input snapshot: %s", markdown)
	}
}

// TestExportMarkdownCanIncludeInputSnapshotExplicitly 验证显式选择时才导出 input_snapshot。
func TestExportMarkdownCanIncludeInputSnapshotExplicitly(t *testing.T) {
	report := Report{
		Title:           "显式快照报告",
		ContentMarkdown: "正文",
		InputSnapshot:   `{"has_user_position":true}`,
	}

	markdown := ExportMarkdown(report, ExportOptions{IncludeInputSnapshot: true})

	if !strings.Contains(markdown, "输入快照") || !strings.Contains(markdown, "has_user_position") {
		t.Fatalf("expected explicit input snapshot export, got %s", markdown)
	}
}

// assertReportErrorCode 校验报告错误码稳定。
func assertReportErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var reportError *Error
	if !errors.As(err, &reportError) {
		t.Fatalf("expected report Error, got %T", err)
	}
	if reportError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, reportError.Code)
	}
}
