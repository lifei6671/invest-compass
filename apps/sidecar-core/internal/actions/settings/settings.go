package settings

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	netproxyservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/netproxy"
	settingsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const workspacePathKey = "workspace_path"
const proxyTestTimeout = 10 * time.Second

// Store 是 settings action 依赖的数据访问边界。
type Store interface {
	UpsertSetting(ctx context.Context, setting model.Setting) error
	GetSettings(ctx context.Context, keys []string) ([]model.Setting, error)
}

// ProxyTester 是代理连通性测试的运行时边界，便于 action 单测不访问公网。
type ProxyTester interface {
	TestConnection(ctx context.Context, target string) (netproxyservice.ConnectionTestResult, error)
}

// Config 是 settings action 的运行期依赖。
type Config struct {
	Security    httpx.SecurityConfig
	Store       Store
	ProxyTester ProxyTester
}

type getRequest struct {
	Keys []string `json:"keys"`
}

type setRequest struct {
	Items []settingItem `json:"items"`
}

type settingItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type settingsData struct {
	Items []settingItem `json:"items"`
}

type savedKeysData struct {
	SavedKeys []string `json:"saved_keys"`
}

type workspaceRequest struct {
	Path string `json:"path"`
}

type workspaceData struct {
	Path string `json:"path"`
}

type proxyTestRequest struct {
	Target string `json:"target"`
}

type proxyTestData struct {
	Result netproxyservice.ConnectionTestResult `json:"result"`
}

// Routes 返回 settings 和 workspace 相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/settings/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/settings/set", Handler: handleSet(config)},
		{Method: http.MethodPost, Path: "/api/workspace/get", Handler: handleWorkspaceGet(config)},
		{Method: http.MethodPost, Path: "/api/workspace/set", Handler: handleWorkspaceSet(config)},
		{Method: http.MethodPost, Path: "/api/proxy/test", Handler: handleProxyTest(config)},
	}
}

// handleGet 按 key 批量读取 settings 表中的非敏感配置。
func handleGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload getRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		items, err := config.Store.GetSettings(request.Context(), normalizedKeys(payload.Keys))
		if err != nil {
			writeStoreError(response, context, "读取 settings 失败", err)
			return
		}
		httpx.WriteOK(response, settingsData{Items: modelSettingsToItems(items)}, context)
	}
}

// handleSet 校验并保存 settings，禁止敏感明文进入 SQLite。
func handleSet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload setRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		savedKeys := make([]string, 0, len(payload.Items))
		for _, item := range payload.Items {
			setting := settingsservice.Setting{Key: strings.TrimSpace(item.Key), Value: item.Value}
			if err := validateSetting(setting); err != nil {
				writeValidationError(response, context, err)
				return
			}
			if setting.Key == "" {
				continue
			}
			if err := config.Store.UpsertSetting(request.Context(), model.Setting{Key: setting.Key, Value: setting.Value}); err != nil {
				writeStoreError(response, context, "保存 settings 失败", err)
				return
			}
			savedKeys = append(savedKeys, setting.Key)
		}

		httpx.WriteOK(response, savedKeysData{SavedKeys: savedKeys}, context)
	}
}

// handleWorkspaceGet 读取当前工作区路径。
func handleWorkspaceGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		items, err := config.Store.GetSettings(request.Context(), []string{workspacePathKey})
		if err != nil {
			writeStoreError(response, context, "读取工作区路径失败", err)
			return
		}
		path := ""
		if len(items) > 0 {
			path = items[0].Value
		}
		httpx.WriteOK(response, workspaceData{Path: path}, context)
	}
}

// handleWorkspaceSet 校验并保存工作区路径。
func handleWorkspaceSet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload workspaceRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		path := strings.TrimSpace(payload.Path)
		if err := settingsservice.ValidateWorkspacePath(path); err != nil {
			writeValidationError(response, context, err)
			return
		}
		if err := config.Store.UpsertSetting(request.Context(), model.Setting{Key: workspacePathKey, Value: path}); err != nil {
			writeStoreError(response, context, "保存工作区路径失败", err)
			return
		}

		httpx.WriteOK(response, workspaceData{Path: path}, context)
	}
}

// handleProxyTest 使用当前 settings 中的代理模式访问固定白名单目标。
func handleProxyTest(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload proxyTestRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		tester := proxyTester(config)
		result, err := tester.TestConnection(request.Context(), payload.Target)
		if errors.Is(err, netproxyservice.ErrUnsupportedTestTarget) {
			httpx.WriteError(response, http.StatusBadRequest, 40003, "invalid_proxy_test_target", context)
			return
		}
		if err != nil {
			writeStoreError(response, context, "代理连接测试失败", err)
			return
		}
		httpx.WriteOK(response, proxyTestData{Result: result}, context)
	}
}

// requireStore 校验 ready/token 和 settings store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50303, "settings_store_unavailable", context)
		return false
	}
	return true
}

// proxyTester 返回注入 tester 或默认 settings 驱动 tester。
func proxyTester(config Config) ProxyTester {
	if config.ProxyTester != nil {
		return config.ProxyTester
	}
	return settingsProxyTester{store: config.Store}
}

type settingsProxyTester struct {
	store Store
}

// TestConnection 通过 netproxy service 读取 settings 并执行固定目标连通性测试。
func (tester settingsProxyTester) TestConnection(ctx context.Context, target string) (netproxyservice.ConnectionTestResult, error) {
	return netproxyservice.TestConnection(ctx, tester.store, target, proxyTestTimeout)
}

// validateSetting 复用 settings service 的安全规则，并为代理 URL 补充边界校验。
func validateSetting(setting settingsservice.Setting) error {
	if err := settingsservice.ValidateSetting(setting); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(setting.Key)) {
	case "proxy_url", "proxy.http_url", "proxy.socks5_url":
		return settingsservice.ValidateProxyURL(setting.Value)
	}
	return nil
}

// writeValidationError 把 service 层稳定错误码映射为 HTTP 错误消息。
func writeValidationError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	if xerrValue, ok := err.(*xerr.Error); ok {
		httpx.WriteError(response, http.StatusBadRequest, 40003, string(xerrValue.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusBadRequest, 40003, "invalid_setting", context)
}

// writeStoreError 记录脱敏后的数据库错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50002, "settings_store_error", context)
}

// normalizedKeys 清理 settings key，并保留调用方请求顺序。
func normalizedKeys(keys []string) []string {
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

// modelSettingsToItems 转换持久化模型为 API 稳定响应字段。
func modelSettingsToItems(settings []model.Setting) []settingItem {
	items := make([]settingItem, 0, len(settings))
	for _, setting := range settings {
		items = append(items, settingItem{Key: setting.Key, Value: setting.Value})
	}
	return items
}
