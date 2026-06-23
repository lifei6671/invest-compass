package settings

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// CacheTarget 是设置中心允许展示的缓存清理目标。
type CacheTarget string

const (
	// CacheTargetQuote 表示行情短缓存。
	CacheTargetQuote CacheTarget = "quote"
	// CacheTargetKline 表示 K 线缓存。
	CacheTargetKline CacheTarget = "kline"
	// CacheTargetNews 表示新闻缓存。
	CacheTargetNews CacheTarget = "news"
	// CacheTargetChartImage 表示图片或图表临时缓存。
	CacheTargetChartImage CacheTarget = "chart_image"
	// CacheTargetTaskLogs 表示任务结构化日志缓存，只允许清理日志明细和诊断摘要。
	CacheTargetTaskLogs CacheTarget = "task_logs"
	// CacheTargetAppLogs 表示本地 NDJSON 应用日志文件缓存。
	CacheTargetAppLogs CacheTarget = "app_logs"
	// CacheTargetReport 表示用户报告，不能被缓存清理删除。
	CacheTargetReport CacheTarget = "report"
	// CacheTargetConfig 表示用户配置，不能被缓存清理删除。
	CacheTargetConfig CacheTarget = "config"
)

// Setting 是 settings 表允许保存的单个键值配置。
type Setting struct {
	Key   string
	Value string
}

// CacheUsage 是某类缓存的体积统计。
type CacheUsage struct {
	Target CacheTarget `json:"target"`
	Bytes  int64       `json:"bytes"`
}

// CacheStats 是设置中心可展示的临时缓存统计。
type CacheStats struct {
	Items      []CacheUsage `json:"items"`
	TotalBytes int64        `json:"total_bytes"`
}

// LicenseStatus 是关于页首版可展示的授权状态。
type LicenseStatus string

const (
	// SettingKeyAppTheme 表示应用主题偏好。
	SettingKeyAppTheme = "app.theme"
	// SettingKeyAppLanguage 表示应用界面语言偏好。
	SettingKeyAppLanguage = "app.language"
	// SettingKeyMarketDefault 表示默认市场。
	SettingKeyMarketDefault = "market.default"
	// SettingKeyQuoteRefreshInterval 表示行情刷新间隔。
	SettingKeyQuoteRefreshInterval = "quote.refresh_interval"
	// SettingKeyKlineDefaultPeriod 表示默认 K 线周期。
	SettingKeyKlineDefaultPeriod = "kline.default_period"
	// SettingKeyKlineDefaultAdjust 表示默认复权方式。
	SettingKeyKlineDefaultAdjust = "kline.default_adjust"
	// SettingKeyNotificationsInAppEnabled 表示应用内通知总开关。
	SettingKeyNotificationsInAppEnabled = "notifications.in_app_enabled"
	// SettingKeyNotificationsSystemEnabled 表示系统级通知总开关。
	SettingKeyNotificationsSystemEnabled = "notifications.system_enabled"
	// SettingKeyNotificationsTaskSuccess 表示任务成功通知开关。
	SettingKeyNotificationsTaskSuccess = "notifications.task_success"
	// SettingKeyNotificationsTaskFailed 表示任务失败通知开关。
	SettingKeyNotificationsTaskFailed = "notifications.task_failed"
	// SettingKeyNotificationsProviderError 表示 Provider 异常通知开关。
	SettingKeyNotificationsProviderError = "notifications.provider_error"
	// SettingKeyWindowCloseToTray 表示关闭窗口时是否最小化到托盘。
	SettingKeyWindowCloseToTray = "window.close_to_tray"
	// SettingKeyUpdateCheckOnStartup 表示启动时检查更新开关。
	SettingKeyUpdateCheckOnStartup = "update.check_on_startup"
	// SettingKeyDataSourceDefaultMarketSource 表示数据源设置中的默认行情源。
	SettingKeyDataSourceDefaultMarketSource = "data_source.default_market_source"

	// DefaultAppTheme 是首版默认浅色主题。
	DefaultAppTheme = "light"
	// DefaultAppLanguage 是首版默认简体中文。
	DefaultAppLanguage = "zh-CN"
	// DefaultMarket 是首版默认 A 股市场。
	DefaultMarket = "CN"
	// DefaultQuoteRefreshInterval 是首版默认行情刷新间隔。
	DefaultQuoteRefreshInterval = "60s"
	// DefaultKlinePeriod 是首版默认日 K。
	DefaultKlinePeriod = "day"
	// DefaultKlineAdjust 是首版默认前复权。
	DefaultKlineAdjust = "qfq"
	// DataSourceMarketSourceAutoFallback 表示按已启用行情源自动降级。
	DataSourceMarketSourceAutoFallback = "auto-fallback"
	// DataSourceMarketSourceSina 表示新浪行情渠道。
	DataSourceMarketSourceSina = "sina"
	// DataSourceMarketSourceTencent 表示腾讯行情渠道。
	DataSourceMarketSourceTencent = "tencent"
	// DataSourceMarketSourceEastMoney 表示东方财富行情渠道。
	DataSourceMarketSourceEastMoney = "eastmoney"
	// DataSourceMarketSourceAkShareEastMoney 表示 AkShare / EastMoney 聚合渠道。
	DataSourceMarketSourceAkShareEastMoney = "akshare-eastmoney"
	// DataSourceMarketSourceCustom 表示自定义行情渠道。
	DataSourceMarketSourceCustom = "custom"

	// LicenseStatusFree 表示首版仅展示 FREE 占位。
	LicenseStatusFree LicenseStatus = "FREE"
	// LocalAIConfigVaultRefPrefix 是 AI Key 本地 vault 引用的唯一合法前缀。
	LocalAIConfigVaultRefPrefix = "local-vault://ai-config/"
	// LocalProxyVaultRefPrefix 是代理密码本地 vault 引用的唯一合法前缀。
	LocalProxyVaultRefPrefix = "local-vault://proxy/"
)

// LicenseView 是关于页授权信息展示模型。
type LicenseView struct {
	Status        LicenseStatus
	CanActivate   bool
	ActivationURL string
}

// ValidateProxyURL 校验代理 URL 不得包含用户名或密码。
func ValidateProxyURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return nil
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return &xerr.Error{Code: xerr.SettingsInvalidProxyURL}
	}
	if parsedURL.User != nil {
		return &xerr.Error{Code: xerr.SettingsProxyCredentialInURL}
	}
	return nil
}

// ValidateSetting 校验 settings 不保存 API Key、密码、token 等敏感明文。
func ValidateSetting(setting Setting) error {
	key := strings.ToLower(strings.TrimSpace(setting.Key))
	if key == "" {
		return nil
	}
	if isAllowedCredentialMetadata(key) {
		return validateCredentialMetadata(key, setting.Value)
	}
	if strings.HasSuffix(key, "_ref") {
		return &xerr.Error{Code: xerr.SettingsInvalidCredentialRef}
	}
	if strings.Contains(key, "password") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "authorization") ||
		key == "api_key" ||
		key == "resolved_api_key" {
		return &xerr.Error{Code: xerr.SettingsSensitiveSetting}
	}
	return validateKnownSettingValue(key, setting.Value)
}

// ValidateWorkspacePath 校验工作区路径必须来自系统目录或用户选择后的绝对路径。
func ValidateWorkspacePath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || !filepath.IsAbs(trimmed) {
		return &xerr.Error{Code: xerr.SettingsInvalidWorkspacePath}
	}
	return nil
}

// FilterCacheCleanupTargets 过滤缓存清理目标，避免误删报告和配置。
func FilterCacheCleanupTargets(targets []CacheTarget) []CacheTarget {
	filtered := make([]CacheTarget, 0, len(targets))
	for _, target := range targets {
		if !isTemporaryCacheTarget(target) {
			continue
		}
		filtered = append(filtered, target)
	}
	return filtered
}

// BuildCacheStats 汇总可清理的临时缓存体积，明确排除报告和配置。
func BuildCacheStats(usages []CacheUsage) CacheStats {
	stats := CacheStats{
		Items: make([]CacheUsage, 0, len(usages)),
	}
	for _, usage := range usages {
		if !isTemporaryCacheTarget(usage.Target) {
			continue
		}
		stats.Items = append(stats.Items, usage)
		if usage.Bytes > 0 {
			stats.TotalBytes += usage.Bytes
		}
	}
	return stats
}

// FreeLicenseView 返回首版关于页 FREE 授权占位。
func FreeLicenseView() LicenseView {
	return LicenseView{Status: LicenseStatusFree}
}

// isAllowedCredentialMetadata 判断 key 是否属于允许落库的凭据引用或脱敏状态。
func isAllowedCredentialMetadata(key string) bool {
	return key == "api_key_ref" ||
		key == "proxy_credential_ref" ||
		key == "masked_api_key" ||
		key == "has_api_key"
}

// validateCredentialMetadata 校验可落库的凭据元数据只包含本地 vault 引用和脱敏状态。
func validateCredentialMetadata(key string, value string) error {
	trimmed := strings.TrimSpace(value)
	switch key {
	case "api_key_ref":
		if trimmed == "" || strings.HasPrefix(trimmed, LocalAIConfigVaultRefPrefix) {
			return nil
		}
	case "proxy_credential_ref":
		if trimmed == "" || strings.HasPrefix(trimmed, LocalProxyVaultRefPrefix) {
			return nil
		}
	case "masked_api_key":
		return validateMaskedAPIKey(trimmed)
	case "has_api_key":
		return nil
	}
	return &xerr.Error{Code: xerr.SettingsInvalidCredentialRef}
}

// validateMaskedAPIKey 校验 settings 中的 API Key 展示值必须是脱敏文本。
func validateMaskedAPIKey(maskedAPIKey string) error {
	if maskedAPIKey == "" || strings.Contains(maskedAPIKey, "*") || strings.Contains(maskedAPIKey, "...") {
		return nil
	}
	return &xerr.Error{Code: xerr.SettingsSensitiveSetting}
}

// validateKnownSettingValue 校验设置中心已定义 key 的值域，未知 key 只走敏感词边界。
func validateKnownSettingValue(key string, value string) error {
	trimmed := strings.TrimSpace(value)
	switch key {
	case SettingKeyAppTheme:
		return validateStringEnum(key, trimmed, DefaultAppTheme, "dark", "system")
	case SettingKeyAppLanguage:
		return validateStringEnum(key, trimmed, DefaultAppLanguage, "en-US")
	case SettingKeyMarketDefault:
		return validateStringEnum(key, trimmed, DefaultMarket, "HK", "US")
	case SettingKeyQuoteRefreshInterval:
		return validateStringEnum(key, trimmed, "15s", "30s", DefaultQuoteRefreshInterval, "120s", "manual")
	case SettingKeyKlineDefaultPeriod:
		return validateStringEnum(key, trimmed, "minute", DefaultKlinePeriod, "week", "month")
	case SettingKeyKlineDefaultAdjust:
		return validateStringEnum(key, trimmed, "none", DefaultKlineAdjust, "hfq")
	case SettingKeyDataSourceDefaultMarketSource:
		return validateStringEnum(
			key,
			trimmed,
			DataSourceMarketSourceAutoFallback,
			DataSourceMarketSourceSina,
			DataSourceMarketSourceTencent,
			DataSourceMarketSourceEastMoney,
			DataSourceMarketSourceAkShareEastMoney,
			DataSourceMarketSourceCustom,
		)
	case SettingKeyNotificationsInAppEnabled,
		SettingKeyNotificationsSystemEnabled,
		SettingKeyNotificationsTaskSuccess,
		SettingKeyNotificationsTaskFailed,
		SettingKeyNotificationsProviderError,
		SettingKeyWindowCloseToTray,
		SettingKeyUpdateCheckOnStartup:
		return validateStringEnum(key, strings.ToLower(trimmed), "true", "false")
	default:
		return nil
	}
}

// validateStringEnum 校验字符串枚举值，保持 settings 表只保存可解释的配置。
func validateStringEnum(key string, value string, allowedValues ...string) error {
	for _, allowedValue := range allowedValues {
		if value == allowedValue {
			return nil
		}
	}
	return fmt.Errorf("settings key %s has invalid value", key)
}

// isTemporaryCacheTarget 判断缓存目标是否属于允许统计和清理的临时缓存。
func isTemporaryCacheTarget(target CacheTarget) bool {
	switch target {
	case CacheTargetQuote, CacheTargetKline, CacheTargetNews, CacheTargetChartImage, CacheTargetTaskLogs, CacheTargetAppLogs:
		return true
	default:
		return false
	}
}
