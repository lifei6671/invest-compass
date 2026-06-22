package search

import (
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestBuildReportSearchSummaryRemovesSensitiveInputs 验证报告搜索摘要不会索引输入快照、持仓、Prompt、Provider 原始响应或凭据。
func TestBuildReportSearchSummaryRemovesSensitiveInputs(t *testing.T) {
	report := model.AnalysisReport{
		InputSnapshot: `{"userPosition":{"cost":123.45,"shares":1000},"api_key":"sk-input-secret"}`,
		ContentMarkdown: `
# 贵州茅台分析

## 结论

均线结构改善，成交量需要继续观察。

## 用户持仓

userPosition: 成本 123.45，仓位 80%

## Prompt

system prompt 和完整用户提示词不得索引

## Provider 原始响应

Authorization: Bearer sk-provider-secret
Proxy-Authorization: Basic abc
api_key: sk-live-secret
proxy_password: proxy-secret
`,
	}

	summary := BuildReportSearchSummary(report)

	assertContainsText(t, summary, "均线结构改善")
	for _, forbidden := range []string{
		"userposition",
		"成本",
		"仓位",
		"prompt",
		"provider",
		"authorization",
		"proxy-authorization",
		"api_key",
		"proxy_password",
		"sk-input-secret",
		"sk-provider-secret",
		"完整用户提示词",
	} {
		assertNotContainsText(t, summary, forbidden)
	}
}

// TestBuildReportSearchSummaryFailsClosedWhenSanitizationLeavesForbiddenText 验证脱敏后仍残留敏感字段时返回空摘要。
func TestBuildReportSearchSummaryFailsClosedWhenSanitizationLeavesForbiddenText(t *testing.T) {
	report := model.AnalysisReport{
		ContentMarkdown: "安全结论。随后出现 userPosition 未结构化残留。",
	}

	if summary := BuildReportSearchSummary(report); summary != "" {
		t.Fatalf("expected fail closed empty summary, got %q", summary)
	}
}

// TestBuildReportSearchSummaryTruncatesLongMarkdown 验证搜索摘要有长度上限，避免把完整报告正文塞进 FTS。
func TestBuildReportSearchSummaryTruncatesLongMarkdown(t *testing.T) {
	report := model.AnalysisReport{
		ContentMarkdown: strings.Repeat("技术面中性，等待量价确认。", 300),
	}

	summary := BuildReportSearchSummary(report)
	if len([]rune(summary)) > MaxReportSearchSummaryRunes {
		t.Fatalf("expected summary length <= %d runes, got %d", MaxReportSearchSummaryRunes, len([]rune(summary)))
	}
}
