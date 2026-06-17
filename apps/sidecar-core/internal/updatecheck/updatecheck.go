package updatecheck

import (
	"net/url"
	"strconv"
	"strings"
)

// ErrorCode 是检查更新模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInsecureURL 表示更新相关链接不是 HTTPS。
	ErrorInsecureURL ErrorCode = "insecure_update_url"
	// ErrorHostNotAllowed 表示更新相关链接域名不在 allowlist 中。
	ErrorHostNotAllowed ErrorCode = "update_host_not_allowed"
	// ErrorInvalidURL 表示更新相关链接不是合法 URL。
	ErrorInvalidURL ErrorCode = "invalid_update_url"
)

// Action 是首版检查更新允许的用户动作。
type Action string

const (
	// ActionPromptOnly 表示首版只展示提示或外链，不下载、不安装、不静默升级。
	ActionPromptOnly Action = "PROMPT_ONLY"
)

// Error 表示检查更新规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串。
func (err *Error) Error() string {
	return string(err.Code)
}

// Manifest 是可信更新 JSON 解析后的首版字段。
type Manifest struct {
	Version         string
	DownloadURL     string
	ReleaseNotesURL string
}

// Result 是检查更新后的首版展示结果。
type Result struct {
	HasNewVersion   bool
	CurrentVersion  string
	LatestVersion   string
	Action          Action
	DownloadURL     string
	ReleaseNotesURL string
}

// ValidateManifestURL 校验更新 JSON URL 必须使用 HTTPS 且命中 allowlist。
func ValidateManifestURL(rawURL string, allowedHosts []string) error {
	return validateAllowedHTTPSURL(rawURL, allowedHosts)
}

// ValidateManifest 校验 manifest 内所有可展示链接的信任边界。
func ValidateManifest(manifest Manifest, allowedHosts []string) error {
	for _, rawURL := range []string{manifest.DownloadURL, manifest.ReleaseNotesURL} {
		if strings.TrimSpace(rawURL) == "" {
			continue
		}
		if err := validateAllowedHTTPSURL(rawURL, allowedHosts); err != nil {
			return err
		}
	}
	return nil
}

// BuildResult 根据当前版本和 manifest 生成首版提示结果。
func BuildResult(currentVersion string, manifest Manifest, allowedHosts []string) (Result, error) {
	if err := ValidateManifest(manifest, allowedHosts); err != nil {
		return Result{}, err
	}
	return Result{
		HasNewVersion:   compareVersion(manifest.Version, currentVersion) > 0,
		CurrentVersion:  strings.TrimSpace(currentVersion),
		LatestVersion:   strings.TrimSpace(manifest.Version),
		Action:          ActionPromptOnly,
		DownloadURL:     manifest.DownloadURL,
		ReleaseNotesURL: manifest.ReleaseNotesURL,
	}, nil
}

// validateAllowedHTTPSURL 校验单个 URL 的 scheme 和 host。
func validateAllowedHTTPSURL(rawURL string, allowedHosts []string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Hostname() == "" {
		return &Error{Code: ErrorInvalidURL}
	}
	if parsedURL.Scheme != "https" {
		return &Error{Code: ErrorInsecureURL}
	}
	if !hostAllowed(parsedURL.Hostname(), allowedHosts) {
		return &Error{Code: ErrorHostNotAllowed}
	}
	return nil
}

// hostAllowed 判断 host 是否命中 allowlist。
func hostAllowed(host string, allowedHosts []string) bool {
	for _, allowedHost := range allowedHosts {
		if strings.EqualFold(host, strings.TrimSpace(allowedHost)) {
			return true
		}
	}
	return false
}

// compareVersion 比较点分数字版本，返回 1 表示 left 更新，-1 表示 right 更新。
func compareVersion(left string, right string) int {
	leftParts := strings.Split(strings.TrimPrefix(strings.TrimSpace(left), "v"), ".")
	rightParts := strings.Split(strings.TrimPrefix(strings.TrimSpace(right), "v"), ".")
	maxLength := len(leftParts)
	if len(rightParts) > maxLength {
		maxLength = len(rightParts)
	}

	for index := 0; index < maxLength; index++ {
		leftValue := versionPart(leftParts, index)
		rightValue := versionPart(rightParts, index)
		if leftValue > rightValue {
			return 1
		}
		if leftValue < rightValue {
			return -1
		}
	}
	return 0
}

// versionPart 读取版本片段，缺失或非法片段按 0 处理。
func versionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(parts[index])
	if err != nil {
		return 0
	}
	return value
}
