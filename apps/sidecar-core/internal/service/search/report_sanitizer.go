package search

import (
	"strings"
	"unicode/utf8"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	// MaxReportSearchSummaryRunes 限制报告搜索摘要长度，避免把完整报告正文写入 FTS。
	MaxReportSearchSummaryRunes = 2000
)

var reportSearchForbiddenFragments = []string{
	"userposition",
	"user_position",
	"position_snapshot",
	"position_input",
	"holding_input",
	"portfolio",
	"持仓",
	"仓位",
	"成本",
	"prompt",
	"provider",
	"原始响应",
	"authorization",
	"proxy-authorization",
	"api_key",
	"apikey",
	"api key",
	"proxy_password",
	"proxypassword",
	"代理密码",
	"sk-",
}

// BuildReportSearchSummary 从报告正文生成可索引摘要，失败时返回空字符串让调用方 fail closed。
func BuildReportSearchSummary(report model.AnalysisReport) string {
	if !utf8.ValidString(report.ContentMarkdown) {
		return ""
	}
	cleaned := removeSensitiveReportSections(report.ContentMarkdown)
	cleaned = logger.RedactText(cleaned)
	if containsReportSearchForbiddenText(cleaned) {
		return ""
	}
	return trimReportSearchSummary(collapseSpaces(cleaned), MaxReportSearchSummaryRunes)
}

// removeSensitiveReportSections 删除 Prompt、Provider 原始响应和持仓等不允许进入搜索的 Markdown 段落。
func removeSensitiveReportSections(markdown string) string {
	lines := strings.Split(markdown, "\n")
	kept := make([]string, 0, len(lines))
	dropSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isMarkdownHeading(trimmed) {
			dropSection = containsReportSearchForbiddenText(trimmed)
			if dropSection {
				continue
			}
		}
		if dropSection || containsReportSearchForbiddenText(trimmed) {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "\n")
}

// isMarkdownHeading 判断一行是否是 Markdown 标题，用于确定敏感段落边界。
func isMarkdownHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	return strings.TrimSpace(strings.TrimLeft(line, "#")) != ""
}

// containsReportSearchForbiddenText 检查文本是否残留禁止进入报告搜索摘要的敏感片段。
func containsReportSearchForbiddenText(text string) bool {
	lower := strings.ToLower(text)
	for _, fragment := range reportSearchForbiddenFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// trimReportSearchSummary 按 rune 长度裁剪摘要，避免截断中文字符。
func trimReportSearchSummary(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
