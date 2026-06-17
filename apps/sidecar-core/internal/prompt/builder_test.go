package prompt

import (
	"errors"
	"strings"
	"testing"
)

// TestBuildStockFullPromptCreatesSeparatedLayers 验证个股综合分析 Prompt 会分离 System、Context、User 三层。
func TestBuildStockFullPromptCreatesSeparatedLayers(t *testing.T) {
	template := Template{
		Name:    "个股综合分析",
		Type:    TemplateStockFull,
		Content: "请分析 {{stock_name}} {{stock_code}} {{market}} {{quote}} {{kline_summary}} {{indicators}} {{news}}，输出语言：{{analysis_language}}",
	}
	input := BuildInput{
		StockName:        "贵州茅台",
		StockCode:        "CN:SH:600519",
		Market:           "A股",
		Quote:            "现价 100，涨跌幅 1%",
		KlineSummary:     "近 20 日震荡上行",
		Indicators:       "MA5 上穿 MA20",
		News:             "公司发布经营公告",
		AnalysisLanguage: "简体中文",
		UserQuestion:     "关注风险",
		UserPosition:     "持仓 100 股，成本 90",
	}

	built, err := BuildStockFullPrompt(template, input)
	if err != nil {
		t.Fatalf("BuildStockFullPrompt returned error: %v", err)
	}

	assertComplianceText(t, built.System)
	if !strings.Contains(built.Context, "贵州茅台") || !strings.Contains(built.Context, "MA5 上穿 MA20") {
		t.Fatalf("context prompt missing market data: %s", built.Context)
	}
	if !strings.Contains(built.User, "关注风险") || !strings.Contains(built.User, "持仓 100 股") {
		t.Fatalf("user prompt missing user context: %s", built.User)
	}
	if strings.Contains(built.System, "持仓 100 股") || strings.Contains(built.Context, "持仓 100 股") {
		t.Fatalf("user position must only appear in user prompt: %+v", built)
	}
	if strings.Contains(built.Joined(), "{{") {
		t.Fatalf("built prompt contains unresolved variables: %s", built.Joined())
	}
}

// TestBuildTechnicalPromptOmitsUserPositionWhenAbsent 验证没有一次性持仓输入时不会生成持仓占位文案。
func TestBuildTechnicalPromptOmitsUserPositionWhenAbsent(t *testing.T) {
	template := Template{
		Name:    "技术分析",
		Type:    TemplateTechnical,
		Content: "技术分析 {{stock_name}} {{stock_code}} {{kline_summary}} {{indicators}}",
	}
	input := BuildInput{
		StockName:        "Apple",
		StockCode:        "US:AAPL",
		Market:           "美股",
		Quote:            "现价 200",
		KlineSummary:     "突破区间高点",
		Indicators:       "RSI 60",
		News:             "无重大新闻",
		AnalysisLanguage: "简体中文",
	}

	built, err := BuildTechnicalPrompt(template, input)
	if err != nil {
		t.Fatalf("BuildTechnicalPrompt returned error: %v", err)
	}

	if strings.Contains(built.User, "持仓") {
		t.Fatalf("user prompt must not mention position when absent: %s", built.User)
	}
	if !strings.Contains(built.Context, "突破区间高点") || !strings.Contains(built.Context, "RSI 60") {
		t.Fatalf("context prompt missing technical data: %s", built.Context)
	}
}

// TestBuildPromptRejectsMissingData 验证缺失核心股票数据时返回稳定错误码。
func TestBuildPromptRejectsMissingData(t *testing.T) {
	template := Template{Name: "个股综合分析", Type: TemplateStockFull, Content: "分析 {{stock_name}}"}

	_, err := BuildStockFullPrompt(template, BuildInput{StockName: "贵州茅台"})

	var promptError *Error
	if !errors.As(err, &promptError) {
		t.Fatalf("expected prompt Error, got %T", err)
	}
	if promptError.Code != ErrorMissingPromptData {
		t.Fatalf("expected error code %q, got %q", ErrorMissingPromptData, promptError.Code)
	}
}

// assertComplianceText 校验 System Prompt 包含投研合规边界。
func assertComplianceText(t *testing.T, system string) {
	t.Helper()
	for _, required := range []string{
		"不构成投资建议",
		"不承诺收益",
		"不直接替用户做买卖决策",
		"风险",
		"数据时效",
		"事实、推断和观点",
		"观察指标",
		"用户自行决策",
	} {
		if !strings.Contains(system, required) {
			t.Fatalf("system prompt missing compliance text %q: %s", required, system)
		}
	}
}
