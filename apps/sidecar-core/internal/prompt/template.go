package prompt

import (
	"regexp"
	"strings"
)

// ErrorCode 是 Prompt 模板规则失败时对外稳定的错误码。
type ErrorCode string

const (
	// ErrorMissingTemplateName 表示模板名称为空。
	ErrorMissingTemplateName ErrorCode = "missing_prompt_template_name"
	// ErrorMissingTemplateContent 表示模板内容为空。
	ErrorMissingTemplateContent ErrorCode = "missing_prompt_template_content"
	// ErrorUnsupportedTemplateType 表示模板类型不在首版白名单内。
	ErrorUnsupportedTemplateType ErrorCode = "unsupported_prompt_template_type"
	// ErrorUnsupportedVariable 表示模板使用了首版不支持的变量。
	ErrorUnsupportedVariable ErrorCode = "unsupported_prompt_variable"
	// ErrorMissingPromptData 表示 Prompt 构建缺少核心上下文数据。
	ErrorMissingPromptData ErrorCode = "missing_prompt_data"
)

// Error 表示 Prompt 模板规则校验失败。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把模板内容写进错误文本。
func (err *Error) Error() string {
	return string(err.Code)
}

// TemplateType 是首版支持的 Prompt 模板类型。
type TemplateType string

const (
	// TemplateSystem 表示系统模板。
	TemplateSystem TemplateType = "system"
	// TemplateStockFull 表示个股综合分析模板。
	TemplateStockFull TemplateType = "stock_full"
	// TemplateTechnical 表示技术面分析模板。
	TemplateTechnical TemplateType = "technical"
	// TemplateCustom 表示用户自定义模板。
	TemplateCustom TemplateType = "custom"
)

// Variable 是首版允许在 Prompt 模板中持久化的变量。
type Variable string

const (
	// VariableStockName 表示股票名称。
	VariableStockName Variable = "stock_name"
	// VariableStockCode 表示股票代码。
	VariableStockCode Variable = "stock_code"
	// VariableMarket 表示市场。
	VariableMarket Variable = "market"
	// VariableQuote 表示行情快照。
	VariableQuote Variable = "quote"
	// VariableKlineSummary 表示 K 线摘要。
	VariableKlineSummary Variable = "kline_summary"
	// VariableIndicators 表示技术指标。
	VariableIndicators Variable = "indicators"
	// VariableNews 表示新闻资讯。
	VariableNews Variable = "news"
	// VariableAnalysisLanguage 表示分析输出语言。
	VariableAnalysisLanguage Variable = "analysis_language"
)

var (
	variablePattern        = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)
	supportedTemplateTypes = map[TemplateType]struct{}{
		TemplateSystem:    {},
		TemplateStockFull: {},
		TemplateTechnical: {},
		TemplateCustom:    {},
	}
	supportedVariables = map[Variable]struct{}{
		VariableStockName:        {},
		VariableStockCode:        {},
		VariableMarket:           {},
		VariableQuote:            {},
		VariableKlineSummary:     {},
		VariableIndicators:       {},
		VariableNews:             {},
		VariableAnalysisLanguage: {},
	}
)

// Template 是 Prompt 模板的领域模型。
type Template struct {
	ID          int64
	Name        string
	Type        TemplateType
	Description string
	Content     string
	Variables   []Variable
	IsBuiltin   bool
	Deleted     bool
}

// ValidateTemplate 校验模板类型和变量白名单，防止首版未闭环能力进入模板。
func ValidateTemplate(template Template) error {
	if strings.TrimSpace(template.Name) == "" {
		return &Error{Code: ErrorMissingTemplateName}
	}
	if strings.TrimSpace(template.Content) == "" {
		return &Error{Code: ErrorMissingTemplateContent}
	}
	if _, ok := supportedTemplateTypes[template.Type]; !ok {
		return &Error{Code: ErrorUnsupportedTemplateType}
	}

	for _, variable := range ExtractVariables(template.Content) {
		if _, ok := supportedVariables[variable]; !ok {
			return &Error{Code: ErrorUnsupportedVariable}
		}
	}
	return nil
}

// ExtractVariables 从模板内容中提取变量，并按首次出现顺序去重。
func ExtractVariables(content string) []Variable {
	matches := variablePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[Variable]struct{}, len(matches))
	variables := make([]Variable, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		variable := Variable(strings.TrimSpace(match[1]))
		if _, exists := seen[variable]; exists {
			continue
		}
		seen[variable] = struct{}{}
		variables = append(variables, variable)
	}
	return variables
}

// CanDeleteTemplate 判断模板删除是否符合首版规则：custom 或非内置模板可删除。
func CanDeleteTemplate(template Template) bool {
	return template.Type == TemplateCustom || !template.IsBuiltin
}

// FilterActiveTemplates 过滤已软删除模板，供列表 API 和页面复用。
func FilterActiveTemplates(templates []Template) []Template {
	active := make([]Template, 0, len(templates))
	for _, template := range templates {
		if template.Deleted {
			continue
		}
		active = append(active, template)
	}
	return active
}
