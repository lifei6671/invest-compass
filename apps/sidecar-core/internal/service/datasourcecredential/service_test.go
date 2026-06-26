package datasourcecredential

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

type fakeClock struct {
	now time.Time
}

// Now 返回测试固定时间，避免过期统计和预检时间随系统时间漂移。
func (clock fakeClock) Now() time.Time {
	return clock.now
}

type memoryStore struct {
	items map[string]model.DataSourceCredential
	next  int64
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// Do 让测试用函数直接满足 HTTPDoer，便于断言预检是否真的发起了 HTTP 请求。
func (do roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}

// SaveDataSourceCredential 保存凭据模型，模拟 DAO 的 provider_id upsert 行为。
func (store *memoryStore) SaveDataSourceCredential(_ context.Context, credential *model.DataSourceCredential) error {
	if store.items == nil {
		store.items = map[string]model.DataSourceCredential{}
	}
	item := *credential
	if item.ID == 0 {
		store.next++
		item.ID = store.next
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)
	}
	item.UpdatedAt = item.CreatedAt
	store.items[item.ProviderID] = item
	*credential = item
	return nil
}

// ListDataSourceCredentials 返回内存中保存的全部凭据模型。
func (store *memoryStore) ListDataSourceCredentials(context.Context) ([]model.DataSourceCredential, error) {
	items := make([]model.DataSourceCredential, 0, len(store.items))
	for _, item := range store.items {
		items = append(items, item)
	}
	return items, nil
}

// GetDataSourceCredential 按 provider_id 读取内存凭据。
func (store *memoryStore) GetDataSourceCredential(_ context.Context, providerID string) (model.DataSourceCredential, bool, error) {
	item, ok := store.items[providerID]
	return item, ok, nil
}

// ClearDataSourceCredential 清空内存凭据密文并标记为未配置。
func (store *memoryStore) ClearDataSourceCredential(_ context.Context, providerID string) error {
	item := store.items[providerID]
	item.CredentialStatus = string(StatusNotConfigured)
	item.EncryptedCredential = ""
	item.CredentialNonce = ""
	item.MaskedCredential = ""
	store.items[providerID] = item
	return nil
}

// TestListIncludesTdxProvider 验证凭据管理目录会展示通达信无凭据 Provider。
func TestListIncludesTdxProvider(t *testing.T) {
	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
		Clock:       fakeClock{now: time.Date(2026, 6, 25, 16, 29, 0, 0, time.UTC)},
	}

	view, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}

	config, ok := view.Configs["tdx"]
	if !ok {
		t.Fatalf("tdx provider should be listed, configs=%+v", view.Configs)
	}
	if config.ProviderName != "通达信" || config.AuthType != AuthTypeNone {
		t.Fatalf("unexpected tdx config: %+v", config)
	}
	foundProvider := false
	for _, provider := range view.Providers {
		if provider.ID == "tdx" {
			foundProvider = provider.Name == "通达信" && provider.IconType == "tdx" && provider.AuthType == AuthTypeNone
			break
		}
	}
	if !foundProvider {
		t.Fatalf("tdx provider should be visible in provider list: %+v", view.Providers)
	}
}

// TestSaveEncryptsCredentialAndReturnsMaskedConfig 验证凭据保存只落密文并只返回脱敏值。
func TestSaveEncryptsCredentialAndReturnsMaskedConfig(t *testing.T) {
	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
		Clock:       fakeClock{now: time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)},
	}

	config, err := service.Save(context.Background(), SaveRequest{
		Config: Config{
			ProviderID:         "cls",
			ProviderName:       "财联社",
			Capability:         "快讯 / 行业事件 / 日历",
			AuthType:           AuthTypeCookie,
			BaseURL:            "https://www.cls.cn",
			CredentialStatus:   StatusNormal,
			ExpiresAt:          "2026-06-30 23:59",
			TimeoutSeconds:     15,
			RateLimitPerMinute: 30,
		},
		Credential: "uid=real-user; token=real-token; session=real-session",
	})
	if err != nil {
		t.Fatalf("save credential: %v", err)
	}

	if config.MaskedCredential != "uid=****; token=****; session=****" {
		t.Fatalf("masked credential = %q", config.MaskedCredential)
	}
	stored := store.items["cls"]
	if stored.EncryptedCredential == "" || stored.CredentialNonce == "" {
		t.Fatalf("credential should be encrypted and nonce should be stored")
	}
	if strings.Contains(stored.EncryptedCredential, "real-token") || strings.Contains(stored.MaskedCredential, "real-token") {
		t.Fatalf("stored credential leaked plaintext: %#v", stored)
	}
	plaintext, err := decryptCredential([]byte("12345678901234567890123456789012"), stored.EncryptedCredential, stored.CredentialNonce)
	if err != nil {
		t.Fatalf("decrypt stored credential: %v", err)
	}
	if plaintext != "uid=real-user; token=real-token; session=real-session" {
		t.Fatalf("decrypted credential = %q", plaintext)
	}
}

// TestClearCredentialRemovesCiphertext 验证清除凭据会移除密文并标记未配置。
func TestClearCredentialRemovesCiphertext(t *testing.T) {
	store := &memoryStore{items: map[string]model.DataSourceCredential{
		"cls": {
			ProviderID:          "cls",
			ProviderName:        "财联社",
			AuthType:            string(AuthTypeCookie),
			BaseURL:             "https://www.cls.cn",
			CredentialStatus:    string(StatusNormal),
			EncryptedCredential: "cipher",
			CredentialNonce:     "nonce",
			MaskedCredential:    "uid=****",
		},
	}}
	service := Service{Store: store, KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012"))}

	config, err := service.Clear(context.Background(), "cls")
	if err != nil {
		t.Fatalf("clear credential: %v", err)
	}

	if config.CredentialStatus != StatusNotConfigured {
		t.Fatalf("config status = %s", config.CredentialStatus)
	}
	stored := store.items["cls"]
	if stored.EncryptedCredential != "" || stored.CredentialNonce != "" || stored.MaskedCredential != "" {
		t.Fatalf("credential should be cleared: %#v", stored)
	}
}

// TestTestCredentialSendsRealRequestWithCookie 验证连接测试会发起真实 HTTP 请求并注入解密后的 Cookie。
func TestTestCredentialSendsRealRequestWithCookie(t *testing.T) {
	var receivedCookie string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/cache" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		receivedCookie = request.Header.Get("Cookie")
		_, _ = writer.Write([]byte(`{"errno":0,"data":{"roll_data":[]}}`))
	}))
	defer server.Close()

	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
		Clock:       fakeClock{now: time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)},
		HTTPClient:  server.Client(),
	}
	_, err := service.Save(context.Background(), SaveRequest{
		Config: Config{
			ProviderID:         "cls",
			ProviderName:       "财联社",
			Capability:         "快讯 / 行业事件 / 日历",
			AuthType:           AuthTypeCookie,
			BaseURL:            server.URL,
			CredentialStatus:   StatusNormal,
			TimeoutSeconds:     15,
			RateLimitPerMinute: 30,
		},
		Credential: "uid=real-user; token=real-token",
	})
	if err != nil {
		t.Fatalf("save credential: %v", err)
	}

	result, err := service.Test(context.Background(), TestRequest{ProviderID: "cls", Target: "flash"})
	if err != nil {
		t.Fatalf("test credential: %v", err)
	}
	if result.Status != testStatusSuccess {
		t.Fatalf("result status = %s", result.Status)
	}
	if receivedCookie != "uid=real-user; token=real-token" {
		t.Fatalf("received cookie = %q", receivedCookie)
	}
	stored := store.items["cls"]
	if stored.LastTestStatus != testStatusSuccess || stored.LastTestedAt == nil {
		t.Fatalf("test result should be persisted: %#v", stored)
	}
}

// TestTestCredentialFailsOnRemoteError 验证真实预检会按 HTTP 状态码判断失败。
func TestTestCredentialFailsOnRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
		Clock:       fakeClock{now: time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)},
		HTTPClient:  server.Client(),
	}
	_, err := service.Save(context.Background(), SaveRequest{
		Config: Config{
			ProviderID:         "cls",
			ProviderName:       "财联社",
			Capability:         "快讯 / 行业事件 / 日历",
			AuthType:           AuthTypeCookie,
			BaseURL:            server.URL,
			CredentialStatus:   StatusNormal,
			TimeoutSeconds:     15,
			RateLimitPerMinute: 30,
		},
		Credential: "uid=real-user; token=real-token",
	})
	if err != nil {
		t.Fatalf("save credential: %v", err)
	}

	result, err := service.Test(context.Background(), TestRequest{ProviderID: "cls", Target: "flash"})
	if err != nil {
		t.Fatalf("test credential: %v", err)
	}
	if result.Status != testStatusFailed {
		t.Fatalf("result status = %s", result.Status)
	}
	if !strings.Contains(strings.Join(result.Messages, " "), "403") {
		t.Fatalf("result should include status code: %+v", result.Messages)
	}
}

// TestTestCredentialSendsProviderPreflightHeaders 验证真实预检按 Provider 补充必要来源头。
func TestTestCredentialSendsProviderPreflightHeaders(t *testing.T) {
	var receivedReferer string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedReferer = request.Header.Get("Referer")
		_, _ = writer.Write([]byte(`var hq_str_sh000001="上证指数,1,2";`))
	}))
	defer server.Close()

	service := Service{
		Store:      &memoryStore{items: map[string]model.DataSourceCredential{}},
		HTTPClient: server.Client(),
	}
	_, err := service.Save(context.Background(), SaveRequest{Config: Config{
		ProviderID:         "sina",
		ProviderName:       "新浪财经",
		Capability:         "股票搜索 / 实时行情",
		AuthType:           AuthTypeNone,
		BaseURL:            server.URL,
		CredentialStatus:   StatusNormal,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 60,
	}})
	if err != nil {
		t.Fatalf("save sina test config: %v", err)
	}
	result, err := service.Test(context.Background(), TestRequest{ProviderID: "sina", Target: "quote"})
	if err != nil {
		t.Fatalf("test sina credentialless provider: %v", err)
	}
	if result.Status != testStatusSuccess {
		t.Fatalf("result status = %s", result.Status)
	}
	if receivedReferer != "https://finance.sina.com.cn/" {
		t.Fatalf("received referer = %q", receivedReferer)
	}
}

// TestCredentiallessProviderTestPersistsResultWithoutExistingRow 验证无凭据 Provider 首次真实测试后也会保存测试结果。
func TestCredentiallessProviderTestPersistsResultWithoutExistingRow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`var hq_str_sh000001="上证指数,1,2";`))
	}))
	defer server.Close()

	originalCatalog := providerCatalog
	providerCatalog = append([]Config(nil), providerCatalog...)
	for index := range providerCatalog {
		if providerCatalog[index].ProviderID == "sina" {
			providerCatalog[index].BaseURL = server.URL
		}
	}
	defer func() {
		providerCatalog = originalCatalog
	}()

	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:      store,
		HTTPClient: server.Client(),
		Clock:      fakeClock{now: time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)},
	}

	result, err := service.Test(context.Background(), TestRequest{ProviderID: "sina", Target: "quote"})
	if err != nil {
		t.Fatalf("test credentialless provider: %v", err)
	}
	if result.Status != testStatusSuccess {
		t.Fatalf("result status = %s", result.Status)
	}
	stored, ok := store.items["sina"]
	if !ok || stored.LastTestStatus != testStatusSuccess || stored.LastTestedAt == nil {
		t.Fatalf("test result should be persisted for first-time credentialless provider: ok=%v item=%+v", ok, stored)
	}
}

// TestTestCredentialMarksTdxAsLimitedWithoutHTTPPreflight 验证 TDX 不走错误的 HTTP 凭据预检路径。
func TestTestCredentialMarksTdxAsLimitedWithoutHTTPPreflight(t *testing.T) {
	httpCalled := false
	service := Service{
		Store: &memoryStore{items: map[string]model.DataSourceCredential{}},
		HTTPClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			httpCalled = true
			return nil, errors.New("tdx should not use HTTP preflight")
		}),
		Clock: fakeClock{now: time.Date(2026, 6, 25, 16, 29, 0, 0, time.UTC)},
	}

	result, err := service.Test(context.Background(), TestRequest{ProviderID: "tdx", Target: "connectivity"})
	if err != nil {
		t.Fatalf("test tdx provider: %v", err)
	}
	if result.Status != string(StatusLimited) {
		t.Fatalf("tdx test status = %s", result.Status)
	}
	if httpCalled {
		t.Fatalf("tdx should not issue HTTP preflight")
	}
	if !strings.Contains(strings.Join(result.Messages, " "), "TDX") {
		t.Fatalf("tdx result should explain the non-HTTP boundary: %+v", result.Messages)
	}
}

// TestResolveReturnsEmptyCredentialForCredentiallessProvider 验证无需凭据的 Provider 可直接进入运行时链路。
func TestResolveReturnsEmptyCredentialForCredentiallessProvider(t *testing.T) {
	service := Service{
		Store:       &memoryStore{items: map[string]model.DataSourceCredential{}},
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	credential, err := service.Resolve(context.Background(), "eastmoney")
	if err != nil {
		t.Fatalf("resolve eastmoney credential: %v", err)
	}

	if credential.ProviderID != "eastmoney" || credential.AuthType != AuthTypeNone {
		t.Fatalf("resolved credential = %#v", credential)
	}
	if credential.Cookie != "" || credential.APIKey != "" || credential.BearerToken != "" || credential.HeaderValue != "" {
		t.Fatalf("credentialless provider should not expose secret fields: %#v", credential)
	}
}

// TestListIncludesSplitSinaAndTencentStockProviders 验证股票行情 Provider 目录拆分展示新浪和腾讯渠道。
func TestListIncludesSplitSinaAndTencentStockProviders(t *testing.T) {
	service := Service{
		Store:       &memoryStore{items: map[string]model.DataSourceCredential{}},
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	view, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}

	expected := map[string]struct {
		name     string
		baseURL  string
		iconType string
	}{
		"sina":    {name: "新浪财经", baseURL: "https://hq.sinajs.cn", iconType: "sina"},
		"tencent": {name: "腾讯财经", baseURL: "https://web.ifzq.gtimg.cn", iconType: "tencent"},
	}
	for providerID, want := range expected {
		config, ok := view.Configs[providerID]
		if !ok {
			t.Fatalf("%s provider config is missing", providerID)
		}
		if config.ProviderName != want.name || config.AuthType != AuthTypeNone || config.BaseURL != want.baseURL {
			t.Fatalf("unexpected %s config: %#v", providerID, config)
		}

		var provider Provider
		for _, item := range view.Providers {
			if item.ID == providerID {
				provider = item
				break
			}
		}
		if provider.Name != want.name || provider.Status != StatusNormal || provider.IconType != want.iconType {
			t.Fatalf("unexpected %s provider: %#v", providerID, provider)
		}
	}
}

// TestListExcludesHiddenCustomHTTPProvider 验证本期隐藏的自定义 HTTP 数据源不会被旧存量配置重新暴露。
func TestListExcludesHiddenCustomHTTPProvider(t *testing.T) {
	service := Service{
		Store: &memoryStore{items: map[string]model.DataSourceCredential{
			"custom-http": {
				ProviderID:          "custom-http",
				ProviderName:        "Custom HTTP",
				Capability:          "自定义接口",
				AuthType:            string(AuthTypeBearerToken),
				BaseURL:             "https://api.example.test",
				CredentialStatus:    string(StatusNormal),
				EncryptedCredential: "cipher",
				CredentialNonce:     "nonce",
				MaskedCredential:    "Bearer ****",
			},
		}},
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	view, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if _, ok := view.Configs["custom-http"]; ok {
		t.Fatalf("custom-http config should be hidden: %#v", view.Configs["custom-http"])
	}
	for _, provider := range view.Providers {
		if provider.ID == "custom-http" {
			t.Fatalf("custom-http provider should be hidden: %#v", provider)
		}
	}
	if got := joinProviderIDs(providerIDs(view.Providers)); strings.Contains(got, "custom-http") {
		t.Fatalf("provider ids should not include custom-http: %s", got)
	}
}

// TestListMarksEastMoneyCapabilitiesAsLimited 验证东财当前只展示行情列表可用，K 线能力仍保持受限说明。
func TestListMarksEastMoneyCapabilitiesAsLimited(t *testing.T) {
	service := Service{
		Store:       &memoryStore{items: map[string]model.DataSourceCredential{}},
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	view, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}

	config := view.Configs["eastmoney"]
	if config.CredentialStatus != StatusLimited {
		t.Fatalf("eastmoney credential status = %s", config.CredentialStatus)
	}
	if config.Capability != "基础证券列表可访问 / K线受限" {
		t.Fatalf("eastmoney capability = %q", config.Capability)
	}
	for _, provider := range view.Providers {
		if provider.ID == "eastmoney" && (provider.Status != StatusLimited || provider.Capability != config.Capability) {
			t.Fatalf("unexpected eastmoney provider: %#v", provider)
		}
	}
}

// providerIDs 提取 Provider ID，便于测试输出可读的失败信息。
func providerIDs(providers []Provider) []string {
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.ID)
	}
	return ids
}

// joinProviderIDs 将 Provider ID 拼成稳定字符串，避免测试失败时输出大对象。
func joinProviderIDs(providerIDs []string) string {
	return strings.Join(providerIDs, ",")
}

// TestPreflightURLSupportsSplitSinaAndTencentTargets 验证股票行情源预检分别命中新浪和腾讯渠道。
func TestPreflightURLSupportsSplitSinaAndTencentTargets(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		target   string
		expected string
	}{
		{
			name:     "sina quote",
			config:   Config{ProviderID: "sina", BaseURL: "https://hq.sinajs.cn"},
			target:   "quote",
			expected: "https://hq.sinajs.cn/list=sh000001",
		},
		{
			name:     "tencent kline",
			config:   Config{ProviderID: "tencent", BaseURL: "https://web.ifzq.gtimg.cn"},
			target:   "kline",
			expected: "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=sh000001,day,,,2,qfq",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetURL, err := preflightURL(tt.config, tt.target)
			if err != nil {
				t.Fatalf("build preflight url: %v", err)
			}
			if targetURL != tt.expected {
				t.Fatalf("preflight url = %q", targetURL)
			}
		})
	}
}

// TestPreflightURLSupportsCredentialedProviderTargets 验证凭据型和补充数据源预检命中各自真实端点。
func TestPreflightURLSupportsCredentialedProviderTargets(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		target   string
		expected string
	}{
		{
			name:     "eastmoney security list",
			config:   Config{ProviderID: "eastmoney", BaseURL: "https://quote.eastmoney.com"},
			target:   "security_list",
			expected: "https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=SECURITY_CODE&sortTypes=1&pageSize=1&pageNumber=1&reportName=RPT_VALUEANALYSIS_DET&columns=SECURITY_CODE,SECURITY_NAME_ABBR",
		},
		{
			name:     "alpha vantage global quote",
			config:   Config{ProviderID: "alpha-vantage", BaseURL: "https://www.alphavantage.co"},
			target:   "quote",
			expected: "https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=IBM",
		},
		{
			name:     "xueqiu hot stock",
			config:   Config{ProviderID: "xueqiu", BaseURL: "https://xueqiu.com"},
			target:   "hot_stock",
			expected: "https://stock.xueqiu.com/v5/stock/hot_stock/list.json?page=1&size=1&_type=10&type=10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetURL, err := preflightURL(tt.config, tt.target)
			if err != nil {
				t.Fatalf("build preflight url: %v", err)
			}
			if targetURL != tt.expected {
				t.Fatalf("preflight url = %q", targetURL)
			}
		})
	}
}

// TestApplyCredentialHeadersUsesAlphaVantageQueryKey 验证 Alpha Vantage 按官方查询参数注入 API Key。
func TestApplyCredentialHeadersUsesAlphaVantageQueryKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=IBM", nil)

	applyCredentialHeaders(request, RuntimeCredential{
		ProviderID: "alpha-vantage",
		AuthType:   AuthTypeAPIKey,
		APIKey:     "alpha-secret",
	})

	if got := request.URL.Query().Get("apikey"); got != "alpha-secret" {
		t.Fatalf("apikey query = %q", got)
	}
	if got := request.Header.Get("X-API-Key"); got != "" {
		t.Fatalf("alpha vantage should not use X-API-Key header, got %q", got)
	}
}

// TestResolveDecryptsCookieCredential 验证运行时解析只在 service 边界解密凭据，供 Provider 注入使用。
func TestResolveDecryptsCookieCredential(t *testing.T) {
	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}
	_, err := service.Save(context.Background(), SaveRequest{
		Config: Config{
			ProviderID:         "xueqiu",
			ProviderName:       "雪球",
			Capability:         "讨论热度",
			AuthType:           AuthTypeCookie,
			BaseURL:            "https://xueqiu.com",
			CredentialStatus:   StatusNormal,
			TimeoutSeconds:     15,
			RateLimitPerMinute: 30,
		},
		Credential: "u=real-user; xq_a_token=real-token",
	})
	if err != nil {
		t.Fatalf("save xueqiu credential: %v", err)
	}

	credential, err := service.Resolve(context.Background(), "xueqiu")
	if err != nil {
		t.Fatalf("resolve xueqiu credential: %v", err)
	}

	if credential.ProviderID != "xueqiu" || credential.AuthType != AuthTypeCookie {
		t.Fatalf("resolved metadata = %#v", credential)
	}
	if credential.Cookie != "u=real-user; xq_a_token=real-token" {
		t.Fatalf("resolved cookie = %q", credential.Cookie)
	}
	if credential.APIKey != "" || credential.BearerToken != "" || credential.HeaderValue != "" {
		t.Fatalf("cookie provider should not fill unrelated secret fields: %#v", credential)
	}
}

// TestResolveMissingCredentialFails 验证需要凭据的 Provider 未配置时不会静默降级或返回空凭据。
func TestResolveMissingCredentialFails(t *testing.T) {
	service := Service{
		Store:       &memoryStore{items: map[string]model.DataSourceCredential{}},
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	_, err := service.Resolve(context.Background(), "xueqiu")
	if !errors.Is(err, ErrCredentialNotConfigured) {
		t.Fatalf("resolve error = %v", err)
	}
}

// TestListRestoresLatestTestResult 验证凭据页重新加载时会展示最近一次真实预检结果。
func TestListRestoresLatestTestResult(t *testing.T) {
	testedAt := time.Date(2026, 6, 22, 15, 28, 41, 0, time.UTC)
	store := &memoryStore{items: map[string]model.DataSourceCredential{
		"cls": {
			ProviderID:           "cls",
			ProviderName:         "财联社",
			Capability:           "快讯 / 行业事件 / 日历",
			AuthType:             string(AuthTypeCookie),
			BaseURL:              "https://www.cls.cn",
			CredentialStatus:     string(StatusNormal),
			MaskedCredential:     "uid=****",
			LastTestStatus:       testStatusSuccess,
			LastTestResponseTime: 186,
			LastTestedAt:         &testedAt,
			LastTestMessages:     `["HTTP 状态码：200","响应数据可读取"]`,
			TimeoutSeconds:       15,
			RateLimitPerMinute:   30,
			EncryptedCredential:  "cipher",
			CredentialNonce:      "nonce",
		},
	}}
	service := Service{
		Store:       store,
		KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012")),
	}

	view, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}

	if view.TestResult.Status != testStatusSuccess || view.TestResult.ResponseTimeMS != 186 {
		t.Fatalf("test result not restored: %+v", view.TestResult)
	}
	if len(view.TestResult.Messages) != 2 || view.TestResult.Messages[0] != "HTTP 状态码：200" {
		t.Fatalf("unexpected restored messages: %+v", view.TestResult.Messages)
	}
}
