package netproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"golang.org/x/net/proxy"
)

const (
	proxyModeKey       = "proxy.mode"
	proxyHTTPURLKey    = "proxy.http_url"
	proxySocks5URLKey  = "proxy.socks5_url"
	proxyNoProxyKey    = "proxy.no_proxy"
	proxyUsernameKey   = "proxy.username"
	proxyCredentialKey = "proxy_credential_ref"
)

var (
	ErrUnsupportedTestTarget         = errors.New("unsupported proxy test target")
	ErrAuthenticatedProxyUnsupported = errors.New("authenticated proxy is not supported")
)

// Store 是代理运行时只读 settings 所需的数据访问边界。
type Store interface {
	GetSettings(ctx context.Context, keys []string) ([]model.Setting, error)
}

// ConnectionTestResult 是代理连通性测试的脱敏结果。
type ConnectionTestResult struct {
	OK         bool   `json:"ok"`
	Target     string `json:"target"`
	StatusCode int    `json:"status_code"`
	DurationMS int64  `json:"duration_ms"`
	CheckedAt  string `json:"checked_at"`
	Message    string `json:"message"`
}

// ClientForSettings 根据 settings 创建外部数据请求使用的 HTTP client。
func ClientForSettings(ctx context.Context, store Store, timeout time.Duration) (*http.Client, error) {
	settings, err := loadSettings(ctx, store)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch normalizedMode(settings[proxyModeKey]) {
	case "none":
		transport.Proxy = nil
	case "custom":
		if hasProxyAuthentication(settings) {
			return nil, ErrAuthenticatedProxyUnsupported
		}
		if strings.TrimSpace(settings[proxySocks5URLKey]) != "" && strings.TrimSpace(settings[proxyHTTPURLKey]) == "" {
			if err := applySocksProxy(transport, settings[proxySocks5URLKey], settings[proxyNoProxyKey]); err != nil {
				return nil, err
			}
		} else if strings.TrimSpace(settings[proxyHTTPURLKey]) != "" {
			proxyURL, err := url.Parse(settings[proxyHTTPURLKey])
			if err != nil {
				return nil, err
			}
			if proxyURL.User != nil {
				return nil, ErrAuthenticatedProxyUnsupported
			}
			transport.Proxy = proxyFuncWithBypass(http.ProxyURL(proxyURL), settings[proxyNoProxyKey])
		} else {
			transport.Proxy = nil
		}
	default:
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// DynamicClientForSettings 创建外部数据请求使用的 HTTP client，每次请求都会读取最新代理 settings。
func DynamicClientForSettings(store Store, timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: settingsAwareTransport{store: store, timeout: timeout}}
}

type settingsAwareTransport struct {
	store   Store
	timeout time.Duration
}

// RoundTrip 在请求发出前重新构造代理 Transport，确保设置页变更能立即影响外部数据请求。
func (transport settingsAwareTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	client, err := ClientForSettings(request.Context(), transport.store, transport.timeout)
	if err != nil {
		return nil, err
	}
	if client.Transport == nil {
		return http.DefaultTransport.RoundTrip(request)
	}
	response, err := client.Transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response != nil && response.Body != nil {
		if httpTransport, ok := client.Transport.(*http.Transport); ok {
			response.Body = closeIdleReadCloser{ReadCloser: response.Body, closeIdle: httpTransport.CloseIdleConnections}
		}
	}
	return response, nil
}

type closeIdleReadCloser struct {
	io.ReadCloser
	closeIdle func()
}

// Close 先关闭响应体，再释放本次请求临时 Transport 的空闲连接。
func (body closeIdleReadCloser) Close() error {
	err := body.ReadCloser.Close()
	if body.closeIdle != nil {
		body.closeIdle()
	}
	return err
}

// TestConnection 使用当前代理 settings 访问固定白名单目标，验证外部网络是否可达。
func TestConnection(ctx context.Context, store Store, target string, timeout time.Duration) (ConnectionTestResult, error) {
	targetURL, ok := proxyTestTargetURL(target)
	if !ok {
		return ConnectionTestResult{}, ErrUnsupportedTestTarget
	}
	client, err := ClientForSettings(ctx, store, timeout)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, targetURL, nil)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	request.Header.Set("User-Agent", "InvestCompass/0.1 proxy-test")

	startedAt := time.Now()
	response, err := client.Do(request)
	durationMS := time.Since(startedAt).Milliseconds()
	result := ConnectionTestResult{
		OK:         false,
		Target:     strings.TrimSpace(target),
		DurationMS: durationMS,
		CheckedAt:  time.Now().Format(time.RFC3339),
	}
	if err != nil {
		result.Message = "connection_failed"
		return result, nil
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	result.StatusCode = response.StatusCode
	result.OK = response.StatusCode > 0 && response.StatusCode < http.StatusInternalServerError
	if result.OK {
		result.Message = "ok"
	} else {
		result.Message = "upstream_error"
	}
	return result, nil
}

// loadSettings 读取代理相关 settings；store 为空时按系统代理处理。
func loadSettings(ctx context.Context, store Store) (map[string]string, error) {
	if store == nil {
		return map[string]string{proxyModeKey: "system"}, nil
	}
	items, err := store.GetSettings(ctx, []string{proxyModeKey, proxyHTTPURLKey, proxySocks5URLKey, proxyNoProxyKey, proxyUsernameKey, proxyCredentialKey})
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.Key] = item.Value
	}
	return values, nil
}

// hasProxyAuthentication 判断 settings 是否包含首版尚未支持的代理认证配置。
func hasProxyAuthentication(settings map[string]string) bool {
	return strings.TrimSpace(settings[proxyUsernameKey]) != "" || strings.TrimSpace(settings[proxyCredentialKey]) != ""
}

// proxyTestTargetURL 把 UI 白名单目标映射为固定 URL，避免任意外部访问代理。
func proxyTestTargetURL(target string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "baidu":
		return "https://www.baidu.com/", true
	case "google":
		return "https://www.google.com/generate_204", true
	case "openai":
		return "https://api.openai.com/v1/models", true
	case "deepseek":
		return "https://api.deepseek.com/models", true
	default:
		return "", false
	}
}

// normalizedMode 兼容旧版 http/socks5 模式，并收敛到当前运行时模式。
func normalizedMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return "none"
	case "custom", "http", "socks5":
		return "custom"
	default:
		return "system"
	}
}

// applySocksProxy 为 HTTP transport 注入 SOCKS 代理拨号器。
func applySocksProxy(transport *http.Transport, rawURL string, bypass string) error {
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if proxyURL.User != nil {
		return ErrAuthenticatedProxyUnsupported
	}
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return err
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		contextDialer = contextDialerAdapter{dialer: dialer}
	}
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		if shouldBypass(address, bypass) {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
		return contextDialer.DialContext(ctx, network, address)
	}
	return nil
}

type contextDialerAdapter struct {
	dialer proxy.Dialer
}

// DialContext 把不支持 context 的 SOCKS dialer 包装成 HTTP transport 需要的接口。
func (adapter contextDialerAdapter) DialContext(_ context.Context, network string, address string) (net.Conn, error) {
	return adapter.dialer.Dial(network, address)
}

// proxyFuncWithBypass 将手动 HTTP 代理和 no_proxy 规则组合成 transport.Proxy。
func proxyFuncWithBypass(proxyFunc func(*http.Request) (*url.URL, error), bypass string) func(*http.Request) (*url.URL, error) {
	return func(request *http.Request) (*url.URL, error) {
		if request != nil && request.URL != nil && shouldBypass(request.URL.Host, bypass) {
			return nil, nil
		}
		return proxyFunc(request)
	}
}

// shouldBypass 判断目标 host 是否命中用户配置的直连规则。
func shouldBypass(address string, bypass string) bool {
	host := address
	if parsedHost, _, err := net.SplitHostPort(address); err == nil {
		host = parsedHost
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, rule := range strings.FieldsFunc(bypass, func(r rune) bool { return r == ';' || r == ',' || r == '\n' }) {
		normalized := strings.ToLower(strings.TrimSpace(rule))
		if normalized == "" {
			continue
		}
		if normalized == host {
			return true
		}
		if strings.HasPrefix(normalized, "*.") && strings.HasSuffix(host, strings.TrimPrefix(normalized, "*")) {
			return true
		}
	}
	return false
}
