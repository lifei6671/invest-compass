package datasourcecredential

import (
	"context"
	"errors"
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

// TestTestCredentialUsesLocalPrecheckOnly 验证连接测试只基于本地密文状态判断，不访问外部网络。
func TestTestCredentialUsesLocalPrecheckOnly(t *testing.T) {
	store := &memoryStore{items: map[string]model.DataSourceCredential{}}
	service := Service{Store: store, KeyProvider: StaticKeyProvider([]byte("12345678901234567890123456789012"))}

	result, err := service.Test(context.Background(), TestRequest{ProviderID: "cls", Target: "flash"})
	if err != nil {
		t.Fatalf("test credential: %v", err)
	}
	if result.Status != testStatusUntested {
		t.Fatalf("result status = %s", result.Status)
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
