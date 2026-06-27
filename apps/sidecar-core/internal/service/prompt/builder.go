package prompt

import (
	"strconv"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const complianceSystemPrompt = `你是投研罗盘的 AI 投研辅助助手。
你的输出仅用于研究辅助，不构成投资建议。
不承诺收益，禁止使用稳赚、必涨、买入信号等诱导表达。
不直接替用户做买卖决策。
必须列出风险点，必须说明数据时效。
必须区分事实、推断和观点。
必须给出后续观察指标，并提醒用户自行决策。`

// BuildInput 是构建单次分析 Prompt 所需的上下文。
type BuildInput struct {
	StockName        string
	StockCode        string
	Market           string
	Quote            string
	KlineSummary     string
	DailyKlines      string
	Indicators       string
	News             string
	DataAsof         string
	ContextQuality   string
	PromptKey        string
	PromptVersion    int
	AnalysisLanguage string
	UserQuestion     string
	UserPosition     string
}

// BuiltPrompt 表示分层后的 System、Context、User Prompt。
type BuiltPrompt struct {
	System  string
	Context string
	User    string
}

// Joined 返回三层 Prompt 的合并文本，主要用于测试和日志前脱敏检查。
func (prompt BuiltPrompt) Joined() string {
	return strings.Join([]string{prompt.System, prompt.Context, prompt.User}, "\n\n")
}

// BuildStockFullPrompt 构建个股综合分析 Prompt。
func BuildStockFullPrompt(template Template, input BuildInput) (BuiltPrompt, error) {
	if template.Type != TemplateStockFull && template.Type != TemplateCustom {
		return BuiltPrompt{}, &xerr.Error{Code: xerr.PromptUnsupportedTemplateType}
	}
	return buildPrompt(template, input, "个股综合分析")
}

// BuildTechnicalPrompt 构建技术面分析 Prompt。
func BuildTechnicalPrompt(template Template, input BuildInput) (BuiltPrompt, error) {
	if template.Type != TemplateTechnical && template.Type != TemplateCustom {
		return BuiltPrompt{}, &xerr.Error{Code: xerr.PromptUnsupportedTemplateType}
	}
	return buildPrompt(template, input, "技术面分析")
}

// buildPrompt 校验输入并生成 System、Context、User 三层 Prompt。
func buildPrompt(template Template, input BuildInput, analysisType string) (BuiltPrompt, error) {
	if err := ValidateTemplate(template); err != nil {
		return BuiltPrompt{}, err
	}
	if err := validateBuildInput(input); err != nil {
		return BuiltPrompt{}, err
	}
	if templateUsesVariable(template, VariableDailyKlines) && strings.TrimSpace(input.DailyKlines) == "" {
		return BuiltPrompt{}, &xerr.Error{Code: xerr.PromptMissingData}
	}

	renderedTemplate := renderTemplate(template.Content, input)
	contextLines := []string{
		"分析类型：" + analysisType,
		"股票名称：" + input.StockName,
		"股票代码：" + input.StockCode,
		"市场：" + input.Market,
		"行情：" + input.Quote,
		"K线摘要：" + input.KlineSummary,
		"技术指标：" + input.Indicators,
	}
	if templateUsesVariable(template, VariableDailyKlines) {
		contextLines = append(contextLines, "日K线数据："+input.DailyKlines)
	}
	if templateUsesVariable(template, VariableNews) {
		contextLines = append(contextLines, "新闻资讯："+input.News)
	}
	contextLines = append(contextLines,
		"模板内容："+renderedTemplate,
		"输出语言："+input.AnalysisLanguage,
	)
	return BuiltPrompt{
		System:  complianceSystemPrompt,
		Context: strings.Join(contextLines, "\n"),
		User:    buildUserPrompt(template, input),
	}, nil
}

// validateBuildInput 校验构建 Prompt 必需的核心上下文。
func validateBuildInput(input BuildInput) error {
	required := []string{
		input.StockName,
		input.StockCode,
		input.Market,
		input.Quote,
		input.KlineSummary,
		input.Indicators,
		input.News,
		input.AnalysisLanguage,
	}
	for _, value := range required {
		if strings.TrimSpace(value) == "" {
			return &xerr.Error{Code: xerr.PromptMissingData}
		}
	}
	return nil
}

// renderTemplate 替换首版白名单变量，确保构建结果不残留模板占位符。
func renderTemplate(content string, input BuildInput) string {
	replacements := map[Variable]string{
		VariableStockName:          input.StockName,
		VariableStockCode:          input.StockCode,
		VariableMarket:             input.Market,
		VariableQuote:              input.Quote,
		VariableKlineSummary:       input.KlineSummary,
		VariableDailyKlines:        input.DailyKlines,
		VariableIndicators:         input.Indicators,
		VariableNews:               input.News,
		VariableFundamentalSummary: "未接入基本面结构化数据",
		VariableUserPosition:       "见用户层一次性持仓输入",
		VariableDataAsof:           input.DataAsof,
		VariableContextQuality:     input.ContextQuality,
		VariablePromptKey:          input.PromptKey,
		VariablePromptVersion:      strconv.Itoa(input.PromptVersion),
		VariableAnalysisLanguage:   input.AnalysisLanguage,
	}

	rendered := content
	for variable, value := range replacements {
		rendered = replaceVariable(rendered, variable, value)
	}
	return rendered
}

// templateUsesVariable 判断模板正文是否显式引用变量，避免未声明上下文进入模型输入。
func templateUsesVariable(template Template, variable Variable) bool {
	for _, item := range ExtractVariables(template.Content) {
		if item == variable {
			return true
		}
	}
	return false
}

// replaceVariable 替换单个变量，兼容变量名两侧带空格的写法。
func replaceVariable(content string, variable Variable, value string) string {
	return variablePattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := variablePattern.FindStringSubmatch(match)
		if len(parts) < 2 || Variable(strings.TrimSpace(parts[1])) != variable {
			return match
		}
		return value
	})
}

// buildUserPrompt 构建用户层 Prompt，一次性持仓输入只在模板显式声明时放入模型输入。
func buildUserPrompt(template Template, input BuildInput) string {
	lines := []string{"用户问题：" + strings.TrimSpace(input.UserQuestion)}
	if templateUsesVariable(template, VariableUserPosition) && strings.TrimSpace(input.UserPosition) != "" {
		lines = append(lines, "用户一次性持仓输入："+strings.TrimSpace(input.UserPosition))
	}
	return strings.Join(lines, "\n")
}
