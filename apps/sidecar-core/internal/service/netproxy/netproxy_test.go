package netproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestClientForSettingsDisablesEnvironmentProxyWhenModeNone 验证“不使用代理”会关闭 Go 默认环境代理。
func TestClientForSettingsDisablesEnvironmentProxyWhenModeNone(t *testing.T) {
	client, err := ClientForSettings(context.Background(), fakeStore{
		settings: []model.Setting{{Key: "proxy.mode", Value: "none"}},
	}, time.Second)
	if err != nil {
		t.Fatalf("ClientForSettings returned error: %v", err)
	}
	transport := transportFromClient(t, client)
	if transport.Proxy != nil {
		t.Fatalf("mode none must disable proxy function")
	}
}

// TestClientForSettingsUsesManualHTTPProxy 验证手动 HTTP 代理从 settings 生效。
func TestClientForSettingsUsesManualHTTPProxy(t *testing.T) {
	client, err := ClientForSettings(context.Background(), fakeStore{
		settings: []model.Setting{
			{Key: "proxy.mode", Value: "custom"},
			{Key: "proxy.http_url", Value: "http://127.0.0.1:7890"},
		},
	}, time.Second)
	if err != nil {
		t.Fatalf("ClientForSettings returned error: %v", err)
	}
	proxyURL, err := transportFromClient(t, client).Proxy(&http.Request{URL: mustURL(t, "https://example.com/api")})
	if err != nil {
		t.Fatalf("proxy resolver returned error: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy URL: %v", proxyURL)
	}
}

// TestClientForSettingsRejectsStoredCredentialReference 验证首版不会把保存了但未注入的代理凭据误当成生效配置。
func TestClientForSettingsRejectsStoredCredentialReference(t *testing.T) {
	_, err := ClientForSettings(context.Background(), fakeStore{
		settings: []model.Setting{
			{Key: "proxy.mode", Value: "custom"},
			{Key: "proxy.http_url", Value: "http://127.0.0.1:7890"},
			{Key: "proxy.username", Value: "alice"},
			{Key: "proxy_credential_ref", Value: "local-vault://proxy/default"},
		},
	}, time.Second)
	if !errors.Is(err, ErrAuthenticatedProxyUnsupported) {
		t.Fatalf("expected ErrAuthenticatedProxyUnsupported, got %v", err)
	}
}

// TestClientForSettingsTreatsLegacyHTTPModeAsCustom 验证旧版 proxy.mode=http 不会丢失代理配置。
func TestClientForSettingsTreatsLegacyHTTPModeAsCustom(t *testing.T) {
	client, err := ClientForSettings(context.Background(), fakeStore{
		settings: []model.Setting{
			{Key: "proxy.mode", Value: "http"},
			{Key: "proxy.http_url", Value: "http://127.0.0.1:7890"},
		},
	}, time.Second)
	if err != nil {
		t.Fatalf("ClientForSettings returned error: %v", err)
	}
	if transportFromClient(t, client).Proxy == nil {
		t.Fatalf("legacy http mode must keep proxy resolver")
	}
}

// TestDynamicClientForSettingsReadsLatestModePerRequest 验证生产外部请求 client 每次请求都读取最新代理模式。
func TestDynamicClientForSettingsReadsLatestModePerRequest(t *testing.T) {
	var proxyCalls int32
	var targetCalls int32
	targetServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		atomic.AddInt32(&targetCalls, 1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer targetServer.Close()
	proxyServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		atomic.AddInt32(&proxyCalls, 1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer proxyServer.Close()
	store := &mutableStore{}
	store.Set([]model.Setting{
		{Key: "proxy.mode", Value: "custom"},
		{Key: "proxy.http_url", Value: proxyServer.URL},
	})
	client := DynamicClientForSettings(store, time.Second)

	if _, err := client.Get(targetServer.URL); err != nil {
		t.Fatalf("proxied request returned error: %v", err)
	}
	if atomic.LoadInt32(&proxyCalls) != 1 || atomic.LoadInt32(&targetCalls) != 0 {
		t.Fatalf("expected first request to use proxy, proxy=%d target=%d", proxyCalls, targetCalls)
	}

	store.Set([]model.Setting{{Key: "proxy.mode", Value: "none"}})
	if _, err := client.Get(targetServer.URL); err != nil {
		t.Fatalf("direct request returned error: %v", err)
	}
	if atomic.LoadInt32(&proxyCalls) != 1 || atomic.LoadInt32(&targetCalls) != 1 {
		t.Fatalf("expected mode none to bypass proxy, proxy=%d target=%d", proxyCalls, targetCalls)
	}
}

// TestConnectionRejectsUnsupportedTarget 验证代理测试只允许固定白名单目标。
func TestConnectionRejectsUnsupportedTarget(t *testing.T) {
	_, err := TestConnection(context.Background(), fakeStore{}, "https://example.com", time.Second)
	if !errors.Is(err, ErrUnsupportedTestTarget) {
		t.Fatalf("expected ErrUnsupportedTestTarget, got %v", err)
	}
}

// TestProxyTestTargetURLKeepsKnownTargets 验证 UI 目标被映射到固定 URL，不接受任意 URL。
func TestProxyTestTargetURLKeepsKnownTargets(t *testing.T) {
	for _, target := range []string{"baidu", "google", "openai", "deepseek"} {
		if value, ok := proxyTestTargetURL(target); !ok || value == "" {
			t.Fatalf("target %s should map to a fixed URL", target)
		}
	}
	if _, ok := proxyTestTargetURL("example"); ok {
		t.Fatalf("unknown target must not be accepted")
	}
}

type fakeStore struct {
	settings []model.Setting
}

// GetSettings 返回固定代理设置，验证代理构造不会写入配置。
func (store fakeStore) GetSettings(context.Context, []string) ([]model.Setting, error) {
	return store.settings, nil
}

type mutableStore struct {
	mutex    sync.Mutex
	settings []model.Setting
}

// Set 替换测试 settings，模拟用户在设置页切换代理模式后的持久化状态。
func (store *mutableStore) Set(settings []model.Setting) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.settings = settings
}

// GetSettings 返回当前可变设置快照，模拟运行期代理配置切换。
func (store *mutableStore) GetSettings(context.Context, []string) ([]model.Setting, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return append([]model.Setting(nil), store.settings...), nil
}

// transportFromClient 提取 HTTP transport，验证代理函数是否按设置落入客户端。
func transportFromClient(t *testing.T, client *http.Client) *http.Transport {
	t.Helper()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport must be *http.Transport, got %T", client.Transport)
	}
	return transport
}

// mustURL 解析测试 URL，失败时立即终止当前用例。
func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}
