package updatecheck

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	updatecheckservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/updatecheck"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const maxManifestBytes = 256 * 1024
const updateManifestURLKey = "update.manifest_url"
const updateAllowedHostsKey = "update.allowed_hosts"

// Fetcher 定义检查更新 action 读取远程 manifest 的唯一边界。
type Fetcher interface {
	FetchManifest(ctx context.Context, manifestURL string) ([]byte, error)
}

// SettingsStore 定义检查更新读取用户配置的 settings 边界。
type SettingsStore interface {
	GetSettings(ctx context.Context, keys []string) ([]model.Setting, error)
}

// HTTPFetcher 通过标准库 HTTP client 获取远程更新 manifest。
type HTTPFetcher struct {
	Client *http.Client
}

// Config 是检查更新 action 的运行期依赖。
type Config struct {
	Security    httpx.SecurityConfig
	Current     string
	ManifestURL string
	AllowedHost []string
	Settings    SettingsStore
	Fetcher     Fetcher
}

// Routes 返回检查更新相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/update/check", Handler: handleCheck(config)},
	}
}

// FetchManifest 使用 HTTPS GET 拉取远程 manifest，并限制响应体大小。
func (fetcher HTTPFetcher) FetchManifest(ctx context.Context, manifestURL string) ([]byte, error) {
	client := manifestHTTPClient(fetcher.Client)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("update manifest status not ok")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxManifestBytes {
		return nil, errors.New("update manifest body too large")
	}
	return content, nil
}

// manifestHTTPClient 返回专用于更新 manifest 的 HTTP client，禁止自动重定向绕过已校验 URL。
func manifestHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: 5 * time.Second}
	}
	client := *base
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

// handleCheck 校验 allowlist 后拉取 manifest，并只返回首版允许的提示结果。
func handleCheck(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		manifestURL, allowedHosts, ok := resolveUpdateConfig(request.Context(), config, context)
		if !ok {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "update_checker_unavailable", context)
			return
		}
		if config.Fetcher == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "update_checker_unavailable", context)
			return
		}
		if err := updatecheckservice.ValidateManifestURL(manifestURL, allowedHosts); err != nil {
			writeRuleError(response, err, context)
			return
		}

		content, err := config.Fetcher.FetchManifest(request.Context(), manifestURL)
		if err != nil {
			slog.Warn(
				"检查更新 manifest 拉取失败",
				logger.FieldRequestID, context.RequestID,
				logger.FieldTraceID, context.TraceID,
				"error", logger.RedactError(err),
			)
			httpx.WriteError(response, http.StatusBadGateway, 50201, "update_manifest_fetch_failed", context)
			return
		}
		manifest, err := updatecheckservice.ParseManifest(content)
		if err != nil {
			writeRuleError(response, err, context)
			return
		}
		result, err := updatecheckservice.BuildResult(config.Current, manifest, allowedHosts)
		if err != nil {
			writeRuleError(response, err, context)
			return
		}

		httpx.WriteOK(response, result, context)
	}
}

// resolveUpdateConfig 从静态配置或 settings 表读取检查更新配置。
func resolveUpdateConfig(ctx context.Context, config Config, requestContext httpx.RequestContext) (string, []string, bool) {
	manifestURL := strings.TrimSpace(config.ManifestURL)
	allowedHosts := normalizedAllowedHosts(config.AllowedHost)
	if manifestURL != "" && len(allowedHosts) > 0 {
		return manifestURL, allowedHosts, true
	}
	if config.Settings == nil {
		return "", nil, false
	}

	items, err := config.Settings.GetSettings(ctx, []string{updateManifestURLKey, updateAllowedHostsKey})
	if err != nil {
		slog.Warn(
			"读取检查更新配置失败",
			logger.FieldRequestID, requestContext.RequestID,
			logger.FieldTraceID, requestContext.TraceID,
			"error", logger.RedactError(err),
		)
		return "", nil, false
	}
	for _, item := range items {
		switch item.Key {
		case updateManifestURLKey:
			manifestURL = strings.TrimSpace(item.Value)
		case updateAllowedHostsKey:
			allowedHosts = splitAllowedHosts(item.Value)
		}
	}
	if manifestURL == "" || len(allowedHosts) == 0 {
		return "", nil, false
	}
	return manifestURL, allowedHosts, true
}

// splitAllowedHosts 将 settings 中的逗号分隔 allowlist 转为稳定列表。
func splitAllowedHosts(raw string) []string {
	return normalizedAllowedHosts(strings.Split(raw, ","))
}

// normalizedAllowedHosts 清理 allowlist，避免空项影响 host 校验。
func normalizedAllowedHosts(hosts []string) []string {
	normalized := make([]string, 0, len(hosts))
	for _, host := range hosts {
		trimmed := strings.TrimSpace(host)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

// writeRuleError 将 updatecheck 规则错误映射为稳定 HTTP 错误响应。
func writeRuleError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) {
		httpx.WriteError(response, http.StatusBadRequest, 40041, string(ruleError.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusInternalServerError, 50041, "update_check_failed", context)
}
