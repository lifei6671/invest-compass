package xerr

import "github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"

// Code 是 Go core 跨包共享的稳定错误码类型。
type Code string

// Error 是 service 层通用规则错误，业务上下文包装错误应保留在具体 service 中。
type Error struct {
	Code    Code
	Message string
}

// Error 返回稳定错误码和脱敏消息，避免敏感输入进入错误文本。
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Message == "" {
		return string(err.Code)
	}
	return string(err.Code) + ": " + logger.RedactText(err.Message)
}

const (
	// StockEmptySymbol 表示股票代码输入为空。
	StockEmptySymbol Code = "empty_symbol"
	// StockInvalidSymbolFormat 表示股票代码分段数量不符合目标市场格式。
	StockInvalidSymbolFormat Code = "invalid_symbol_format"
	// StockUnsupportedMarket 表示首版暂不支持该市场。
	StockUnsupportedMarket Code = "unsupported_market"
	// StockUnsupportedExchange 表示 A 股市场下的交易所代码不受支持。
	StockUnsupportedExchange Code = "unsupported_exchange"
	// StockInvalidSymbolCode 表示股票代码主体不符合目标市场规则。
	StockInvalidSymbolCode Code = "invalid_symbol_code"

	// WatchlistInvalidSymbol 表示自选股股票代码无法解析。
	WatchlistInvalidSymbol Code = "invalid_watchlist_symbol"
	// WatchlistDuplicateActiveSymbol 表示同一 active symbol 已存在。
	WatchlistDuplicateActiveSymbol Code = "duplicate_active_symbol"

	// UpdateInsecureURL 表示更新相关链接不是 HTTPS。
	UpdateInsecureURL Code = "insecure_update_url"
	// UpdateHostNotAllowed 表示更新相关链接域名不在 allowlist 中。
	UpdateHostNotAllowed Code = "update_host_not_allowed"
	// UpdateInvalidURL 表示更新相关链接不是合法 URL。
	UpdateInvalidURL Code = "invalid_update_url"
	// UpdateInvalidManifest 表示更新 JSON 结构无效或缺少必填字段。
	UpdateInvalidManifest Code = "invalid_update_manifest"

	// MarketInvalidQuoteCacheTTL 表示行情短缓存 TTL 不在 10-60 秒范围内。
	MarketInvalidQuoteCacheTTL Code = "invalid_quote_cache_ttl"
	// MarketProviderUnconfigured 表示尚未配置真实行情数据源。
	MarketProviderUnconfigured Code = "market_provider_unconfigured"

	// NewsInvalidCacheTTL 表示新闻缓存 TTL 不在 30-120 分钟范围内。
	NewsInvalidCacheTTL Code = "invalid_news_cache_ttl"
	// NewsProviderUnconfigured 表示尚未配置真实新闻数据源。
	NewsProviderUnconfigured Code = "news_provider_unconfigured"
	// NewsMissingTitle 表示新闻标题为空。
	NewsMissingTitle Code = "missing_news_title"
	// NewsUnsafeURL 表示新闻链接使用了不允许的 URL scheme。
	NewsUnsafeURL Code = "unsafe_news_url"

	// LogExportMissingTroubleshootingField 表示导出日志缺少核心排障字段。
	LogExportMissingTroubleshootingField Code = "missing_log_troubleshooting_field"

	// AIRawAPIKeyNotAllowed 表示 Go core 保存配置时收到了真实 API Key。
	AIRawAPIKeyNotAllowed Code = "raw_api_key_not_allowed"
	// AIInvalidCredentialRef 表示 AI 配置携带了非法本地凭据引用。
	AIInvalidCredentialRef Code = "invalid_ai_credential_ref"
	// AIInvalidRequest 表示 AI 请求参数不足。
	AIInvalidRequest Code = "invalid_ai_request"
	// AIUnauthorized 表示 AI Provider 返回认证失败。
	AIUnauthorized Code = "ai_unauthorized"
	// AIRateLimited 表示 AI Provider 返回限流。
	AIRateLimited Code = "ai_rate_limited"
	// AIUpstream 表示 AI Provider 返回 5xx 或不可识别错误。
	AIUpstream Code = "ai_upstream_error"
	// AICancelled 表示 AI 请求上下文取消或超时。
	AICancelled Code = "ai_request_cancelled"

	// IndicatorInvalidPeriod 表示指标周期参数非法。
	IndicatorInvalidPeriod Code = "invalid_indicator_period"
	// IndicatorInsufficientData 表示输入数据不足以计算目标指标。
	IndicatorInsufficientData Code = "insufficient_indicator_data"
	// IndicatorInvalidInput 表示输入价格或参数不符合计算前置条件。
	IndicatorInvalidInput Code = "invalid_indicator_input"

	// SettingsInvalidProxyURL 表示代理 URL 无法解析。
	SettingsInvalidProxyURL Code = "invalid_proxy_url"
	// SettingsProxyCredentialInURL 表示代理 URL 中包含 username/password。
	SettingsProxyCredentialInURL Code = "proxy_credential_in_url"
	// SettingsSensitiveSetting 表示 settings 试图保存敏感明文。
	SettingsSensitiveSetting Code = "sensitive_setting"
	// SettingsInvalidCredentialRef 表示 settings 携带了非法本地凭据引用。
	SettingsInvalidCredentialRef Code = "invalid_setting_credential_ref"
	// SettingsInvalidWorkspacePath 表示工作区路径不是可接受的绝对路径。
	SettingsInvalidWorkspacePath Code = "invalid_workspace_path"

	// AnalysisInvalidSymbol 表示分析任务股票代码非法。
	AnalysisInvalidSymbol Code = "invalid_analysis_symbol"
	// AnalysisUnsupportedType 表示分析类型不在首版支持范围内。
	AnalysisUnsupportedType Code = "unsupported_analysis_type"
	// AnalysisMissingAIConfig 表示缺少模型配置 ID。
	AnalysisMissingAIConfig Code = "missing_ai_config"
	// AnalysisMissingPromptTemplate 表示缺少 Prompt 模板 ID。
	AnalysisMissingPromptTemplate Code = "missing_prompt_template"
	// AnalysisTaskAlreadyTerminal 表示终态任务不能继续取消。
	AnalysisTaskAlreadyTerminal Code = "task_already_terminal"

	// ReportNotFound 表示报告不存在或已软删除。
	ReportNotFound Code = "report_not_found"

	// PromptMissingTemplateName 表示模板名称为空。
	PromptMissingTemplateName Code = "missing_prompt_template_name"
	// PromptMissingTemplateContent 表示模板内容为空。
	PromptMissingTemplateContent Code = "missing_prompt_template_content"
	// PromptUnsupportedTemplateType 表示模板类型不在首版白名单内。
	PromptUnsupportedTemplateType Code = "unsupported_prompt_template_type"
	// PromptUnsupportedVariable 表示模板使用了首版不支持的变量。
	PromptUnsupportedVariable Code = "unsupported_prompt_variable"
	// PromptMissingData 表示 Prompt 构建缺少核心上下文数据。
	PromptMissingData Code = "missing_prompt_data"
	// PromptBuiltinTemplateReadOnly 表示内置模板不允许更新或删除。
	PromptBuiltinTemplateReadOnly Code = "builtin_prompt_template_readonly"

	// TaskInvalidTransition 表示任务状态流转不符合状态机规则。
	TaskInvalidTransition Code = "invalid_task_transition"
)
