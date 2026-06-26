package ai

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestResult 是模型连通性测试的安全结果，不包含真实 API Key 或请求头。
type TestResult struct {
	OK         bool   `json:"ok"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Message    string `json:"message"`
	DurationMS int64  `json:"duration_ms"`
}

// ConfigTester 定义 AI 配置连通性测试边界，便于 action 单测替换外部网络。
type ConfigTester interface {
	TestAIConfig(ctx context.Context, config Config, resolvedAPIKey string) (TestResult, error)
}

// OpenAIConfigTester 使用 OpenAI-compatible chat completions 执行真实连通性测试。
type OpenAIConfigTester struct {
	HTTPClient *http.Client
}

// TestAIConfig 根据配置和运行期密钥测试模型是否可访问。
func (tester OpenAIConfigTester) TestAIConfig(ctx context.Context, config Config, resolvedAPIKey string) (TestResult, error) {
	if strings.TrimSpace(resolvedAPIKey) == "" || strings.TrimSpace(config.ModelName) == "" {
		return TestResult{}, &xerr.Error{Code: xerr.AIInvalidRequest}
	}
	if !usesOpenAICompatibleTestProtocol(config.Provider) {
		return TestResult{}, &xerr.Error{Code: xerr.AIInvalidRequest, Message: "unsupported provider"}
	}

	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	client := NewOpenAICompatibleClient(ClientConfig{
		BaseURL: config.BaseURL,
		APIKey:  resolvedAPIKey,
		Timeout: timeout,
		Client:  tester.HTTPClient,
	})
	startedAt := time.Now()
	response, err := client.Chat(ctx, ChatRequest{
		Model:       config.ModelName,
		Temperature: config.Temperature,
		MaxTokens:   16,
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a connectivity probe."},
			{Role: RoleUser, Content: "Reply with ok."},
		},
	})
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{
		OK:         true,
		Provider:   config.Provider,
		Model:      config.ModelName,
		Message:    strings.TrimSpace(response.Content),
		DurationMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

// usesOpenAICompatibleTestProtocol 判断 Provider 是否复用 OpenAI-compatible 连通性测试协议。
func usesOpenAICompatibleTestProtocol(provider string) bool {
	switch provider {
	case ProviderOpenAICompatible, ProviderDeepSeek:
		return true
	default:
		return false
	}
}
