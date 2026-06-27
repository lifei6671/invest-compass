package prompt

import (
	"errors"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestBuildStockFullPromptCreatesSeparatedLayers 验证个股综合分析 Prompt 会分离 System、Context、User 三层。
func TestBuildStockFullPromptCreatesSeparatedLayers(t *testing.T) {
	template := Template{
		Key:     "builtin_stock_full",
		Name:    "个股综合分析",
		Type:    TemplateStockFull,
		Content: "请分析 {{stock_name}} {{stock_code}} {{market}} {{quote}} {{kline_summary}} {{daily_klines}} {{indicators}} {{news}} {{user_position}}，截止：{{data_asof}}，质量：{{context_quality}}，模板：{{prompt_key}} v{{prompt_version}}，输出语言：{{analysis_language}}",
		Version: 2,
	}
	input := BuildInput{
		StockName:        "贵州茅台",
		StockCode:        "CN:SH:600519",
		Market:           "A股",
		Quote:            "现价 100，涨跌幅 1%",
		KlineSummary:     "近 20 日震荡上行",
		DailyKlines:      "date=2026-06-26 open=100.00 high=105.00 low=99.00 close=103.00 volume=1000 amount=103000",
		Indicators:       "MA5 上穿 MA20",
		News:             "公司发布经营公告",
		AnalysisLanguage: "简体中文",
		DataAsof:         "2026-06-26T15:00:00Z",
		ContextQuality:   "K线 250 条，指标完整",
		PromptKey:        "builtin_stock_full",
		PromptVersion:    2,
		UserQuestion:     "关注风险",
		UserPosition:     "持仓 100 股，成本 90",
	}

	built, err := BuildStockFullPrompt(template, input)
	if err != nil {
		t.Fatalf("BuildStockFullPrompt returned error: %v", err)
	}

	assertComplianceText(t, built.System)
	if !strings.Contains(built.Context, "贵州茅台") ||
		!strings.Contains(built.Context, "MA5 上穿 MA20") ||
		!strings.Contains(built.Context, "date=2026-06-26 open=100.00") {
		t.Fatalf("context prompt missing market data: %s", built.Context)
	}
	for _, expected := range []string{"2026-06-26T15:00:00Z", "K线 250 条，指标完整", "builtin_stock_full", "v2"} {
		if !strings.Contains(built.Context, expected) {
			t.Fatalf("context prompt missing rendered template value %q: %s", expected, built.Context)
		}
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
		DailyKlines:      "date=2026-06-26 close=200.00",
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

// TestBuildPromptOmitsNewsWhenTemplateDoesNotUseNews 验证技术类模板未声明新闻变量时不会把新闻塞进模型上下文。
func TestBuildPromptOmitsNewsWhenTemplateDoesNotUseNews(t *testing.T) {
	template := Template{
		Name:    "个股综合分析",
		Type:    TemplateStockFull,
		Content: "仅分析 {{quote}} {{kline_summary}} {{indicators}}",
	}
	input := BuildInput{
		StockName:        "贵州茅台",
		StockCode:        "CN:SH:600519",
		Market:           "A股",
		Quote:            "现价 100",
		KlineSummary:     "近 20 日震荡",
		DailyKlines:      "date=2026-06-26 close=100.00",
		Indicators:       "RSI 60",
		News:             "公司发布经营公告",
		AnalysisLanguage: "简体中文",
	}

	built, err := BuildStockFullPrompt(template, input)
	if err != nil {
		t.Fatalf("BuildStockFullPrompt returned error: %v", err)
	}
	if strings.Contains(built.Context, "新闻资讯") || strings.Contains(built.Context, "公司发布经营公告") {
		t.Fatalf("context must omit news when template does not use news: %s", built.Context)
	}
}

// TestBuildPromptOmitsUserPositionWhenTemplateDoesNotUsePosition 验证模板未声明持仓变量时不会把一次性持仓塞进模型输入。
func TestBuildPromptOmitsUserPositionWhenTemplateDoesNotUsePosition(t *testing.T) {
	template := Template{
		Name:    "个股综合分析",
		Type:    TemplateStockFull,
		Content: "仅分析 {{quote}} {{kline_summary}} {{indicators}}",
	}
	input := BuildInput{
		StockName:        "贵州茅台",
		StockCode:        "CN:SH:600519",
		Market:           "A股",
		Quote:            "现价 100",
		KlineSummary:     "近 20 日震荡",
		DailyKlines:      "date=2026-06-26 close=100.00",
		Indicators:       "RSI 60",
		News:             "暂无相关新闻缓存",
		AnalysisLanguage: "简体中文",
		UserPosition:     "cost_price=90 shares=100 risk_level=medium",
	}

	built, err := BuildStockFullPrompt(template, input)
	if err != nil {
		t.Fatalf("BuildStockFullPrompt returned error: %v", err)
	}
	if strings.Contains(built.Joined(), "cost_price=90") || strings.Contains(built.Joined(), "用户一次性持仓输入") {
		t.Fatalf("prompt must omit user position when template does not use user_position: %s", built.Joined())
	}
}

// TestBuildPromptRejectsMissingData 验证缺失核心股票数据时返回稳定错误码。
func TestBuildPromptRejectsMissingData(t *testing.T) {
	template := Template{Name: "个股综合分析", Type: TemplateStockFull, Content: "分析 {{stock_name}}"}

	_, err := BuildStockFullPrompt(template, BuildInput{StockName: "贵州茅台"})

	var promptError *xerr.Error
	if !errors.As(err, &promptError) {
		t.Fatalf("expected prompt Error, got %T", err)
	}
	if promptError.Code != xerr.PromptMissingData {
		t.Fatalf("expected error code %q, got %q", xerr.PromptMissingData, promptError.Code)
	}
}

// assertComplianceText 校验 System Prompt 包含投研合规边界。
func assertComplianceText(t *testing.T, system string) {
	t.Helper()
	for _, required := range []string{
		"不构成投资建议",
		"不承诺收益",
		"买入信号",
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
