package settings

import (
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
	// LicenseStatusFree 表示首版仅展示 FREE 占位。
	LicenseStatusFree LicenseStatus = "FREE"
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
		return nil
	}
	if strings.Contains(key, "password") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "authorization") ||
		key == "api_key" ||
		key == "resolved_api_key" {
		return &xerr.Error{Code: xerr.SettingsSensitiveSetting}
	}
	return nil
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
	return strings.HasSuffix(key, "_ref") || key == "masked_api_key" || key == "has_api_key"
}

// isTemporaryCacheTarget 判断缓存目标是否属于允许统计和清理的临时缓存。
func isTemporaryCacheTarget(target CacheTarget) bool {
	switch target {
	case CacheTargetQuote, CacheTargetKline, CacheTargetNews, CacheTargetChartImage:
		return true
	default:
		return false
	}
}
