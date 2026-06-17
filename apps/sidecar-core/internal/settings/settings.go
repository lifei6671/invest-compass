package settings

import (
	"net/url"
	"strings"
)

// ErrorCode 是设置模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidProxyURL 表示代理 URL 无法解析。
	ErrorInvalidProxyURL ErrorCode = "invalid_proxy_url"
	// ErrorProxyCredentialInURL 表示代理 URL 中包含 username/password。
	ErrorProxyCredentialInURL ErrorCode = "proxy_credential_in_url"
)

// Error 表示设置模块规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串。
func (err *Error) Error() string {
	return string(err.Code)
}

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
		return &Error{Code: ErrorInvalidProxyURL}
	}
	if parsedURL.User != nil {
		return &Error{Code: ErrorProxyCredentialInURL}
	}
	return nil
}

// FilterCacheCleanupTargets 过滤缓存清理目标，避免误删报告和配置。
func FilterCacheCleanupTargets(targets []CacheTarget) []CacheTarget {
	filtered := make([]CacheTarget, 0, len(targets))
	for _, target := range targets {
		if target == CacheTargetReport || target == CacheTargetConfig {
			continue
		}
		filtered = append(filtered, target)
	}
	return filtered
}

// FreeLicenseView 返回首版关于页 FREE 授权占位。
func FreeLicenseView() LicenseView {
	return LicenseView{Status: LicenseStatusFree}
}
