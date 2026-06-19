package updatecheck

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Action 是首版检查更新允许的用户动作。
type Action string

const (
	// ActionPromptOnly 表示首版只展示提示或外链，不下载、不安装、不静默升级。
	ActionPromptOnly Action = "PROMPT_ONLY"
)

// Manifest 是可信更新 JSON 解析后的首版字段。
type Manifest struct {
	Version         string
	DownloadURL     string
	ReleaseNotesURL string
}

type manifestJSON struct {
	Version         string `json:"version"`
	DownloadURL     string `json:"download_url"`
	ReleaseNotesURL string `json:"release_notes_url"`
}

// Result 是检查更新后的首版展示结果。
type Result struct {
	HasNewVersion   bool   `json:"has_new_version"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	Action          Action `json:"action"`
	DownloadURL     string `json:"download_url"`
	ReleaseNotesURL string `json:"release_notes_url"`
}

// ParseManifest 解析更新 JSON，并清理首版支持字段的首尾空白。
func ParseManifest(content []byte) (Manifest, error) {
	var payload manifestJSON
	if err := json.Unmarshal(content, &payload); err != nil {
		return Manifest{}, &xerr.Error{Code: xerr.UpdateInvalidManifest}
	}
	manifest := Manifest{
		Version:         strings.TrimSpace(payload.Version),
		DownloadURL:     strings.TrimSpace(payload.DownloadURL),
		ReleaseNotesURL: strings.TrimSpace(payload.ReleaseNotesURL),
	}
	if manifest.Version == "" {
		return Manifest{}, &xerr.Error{Code: xerr.UpdateInvalidManifest}
	}
	return manifest, nil
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
		return &xerr.Error{Code: xerr.UpdateInvalidURL}
	}
	if parsedURL.Scheme != "https" {
		return &xerr.Error{Code: xerr.UpdateInsecureURL}
	}
	if !hostAllowed(parsedURL.Hostname(), allowedHosts) {
		return &xerr.Error{Code: xerr.UpdateHostNotAllowed}
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
