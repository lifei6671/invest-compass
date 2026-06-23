package datasourcecredential

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	service "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
)

type fakeService struct {
	saveRequest service.SaveRequest
	saveConfig  service.Config
}

// List 返回空白凭据页数据，供 action 测试验证 envelope。
func (fake *fakeService) List(context.Context) (service.ListView, error) {
	return service.ListView{SelectedProvider: "cls"}, nil
}

// Save 记录保存请求并返回脱敏配置，确保 action 不自行处理明文。
func (fake *fakeService) Save(_ context.Context, request service.SaveRequest) (service.Config, error) {
	fake.saveRequest = request
	return fake.saveConfig, nil
}

// Clear 返回指定 Provider 的未配置状态。
func (fake *fakeService) Clear(_ context.Context, providerID string) (service.Config, error) {
	return service.Config{ProviderID: providerID, CredentialStatus: service.StatusNotConfigured}, nil
}

// Test 返回本地预检结果。
func (fake *fakeService) Test(context.Context, service.TestRequest) (service.TestResult, error) {
	return service.TestResult{Status: "success", Messages: []string{"本地预检通过"}}, nil
}

// TestSaveDoesNotEchoPlainCredential 验证保存响应不会回显真实凭据明文。
func TestSaveDoesNotEchoPlainCredential(t *testing.T) {
	fake := &fakeService{saveConfig: service.Config{
		ProviderID:       "cls",
		ProviderName:     "财联社",
		AuthType:         service.AuthTypeCookie,
		BaseURL:          "https://www.cls.cn",
		CredentialStatus: service.StatusNormal,
		MaskedCredential: "uid=****; token=****",
	}}
	recorder := perform(t, handleSave(Config{
		Security: httpx.SecurityConfig{Token: "token", Ready: true},
		Service:  fake,
	}), `{
		"config":{
			"providerId":"cls",
			"providerName":"财联社",
			"capability":"快讯 / 日历",
			"authType":"cookie",
			"baseUrl":"https://www.cls.cn",
			"credentialStatus":"normal",
			"expiresAt":"",
			"timeoutSeconds":15,
			"rateLimitPerMinute":30,
			"maskedCredential":"",
			"note":""
		},
		"credential":"uid=real-user; token=real-token"
	}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(fake.saveRequest.Credential, "real-token") {
		t.Fatalf("service should receive one-time plaintext credential")
	}
	if strings.Contains(recorder.Body.String(), "real-token") || strings.Contains(recorder.Body.String(), "real-user") {
		t.Fatalf("response leaked credential: %s", recorder.Body.String())
	}
}

// TestListRequiresRuntimeToken 验证凭据接口仍受 runtime token 保护。
func TestListRequiresRuntimeToken(t *testing.T) {
	recorder := performWithoutToken(t, handleList(Config{
		Security: httpx.SecurityConfig{Token: "token", Ready: true},
		Service:  &fakeService{},
	}), `{}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

// perform 执行带 token 的 action 请求。
func perform(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	request.Header.Set(httpx.TokenHeader, "token")
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not json: %v", err)
	}
	return recorder
}

// performWithoutToken 执行不带 token 的 action 请求。
func performWithoutToken(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}
