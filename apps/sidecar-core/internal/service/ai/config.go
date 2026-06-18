package ai

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	// ProviderOpenAICompatible 表示 OpenAI-compatible AI Provider。
	ProviderOpenAICompatible = "openai-compatible"
)

// Config 是 Go core 可持久化的 AI 配置模型，不包含真实 API Key。
type Config struct {
	ID             int64
	Name           string
	Provider       string
	BaseURL        string
	APIKeyRef      string
	MaskedAPIKey   string
	HasAPIKey      bool
	ModelName      string
	Temperature    float64
	MaxTokens      int
	TimeoutSeconds int
	StreamEnabled  bool
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ListView 是返回给前端的 AI 配置安全展示模型。
type ListView struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	BaseURL        string    `json:"base_url"`
	APIKeyRef      string    `json:"api_key_ref"`
	MaskedAPIKey   string    `json:"masked_api_key"`
	HasAPIKey      bool      `json:"has_api_key"`
	ModelName      string    `json:"model_name"`
	Temperature    float64   `json:"temperature"`
	MaxTokens      int       `json:"max_tokens"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	StreamEnabled  bool      `json:"stream_enabled"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// String 返回安全展示模型的 JSON 文本，用于测试和日志排查。
func (view ListView) String() string {
	encoded, err := json.Marshal(view)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ListView 生成不包含真实 API Key 的前端展示模型。
func (config Config) ListView() ListView {
	return ListView{
		ID:             config.ID,
		Name:           config.Name,
		Provider:       config.Provider,
		BaseURL:        config.BaseURL,
		APIKeyRef:      config.APIKeyRef,
		MaskedAPIKey:   config.MaskedAPIKey,
		HasAPIKey:      config.HasAPIKey,
		ModelName:      config.ModelName,
		Temperature:    config.Temperature,
		MaxTokens:      config.MaxTokens,
		TimeoutSeconds: config.TimeoutSeconds,
		StreamEnabled:  config.StreamEnabled,
		IsDefault:      config.IsDefault,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}

// SaveRequest 是 Rust 写入系统凭据后转给 Go core 的配置保存输入。
type SaveRequest struct {
	ID             int64
	Name           string
	Provider       string
	BaseURL        string
	APIKeyRef      string
	MaskedAPIKey   string
	HasAPIKey      bool
	RawAPIKey      string
	ModelName      string
	Temperature    float64
	MaxTokens      int
	TimeoutSeconds int
	StreamEnabled  bool
	IsDefault      bool
}

// BuildConfigForSave 构造 Go core 可保存的配置，禁止真实 API Key 进入模型。
func BuildConfigForSave(request SaveRequest) (Config, error) {
	if strings.TrimSpace(request.RawAPIKey) != "" {
		return Config{}, &xerr.Error{Code: xerr.AIRawAPIKeyNotAllowed}
	}
	return Config{
		ID:             request.ID,
		Name:           request.Name,
		Provider:       request.Provider,
		BaseURL:        strings.TrimRight(strings.TrimSpace(request.BaseURL), "/"),
		APIKeyRef:      request.APIKeyRef,
		MaskedAPIKey:   request.MaskedAPIKey,
		HasAPIKey:      request.HasAPIKey,
		ModelName:      request.ModelName,
		Temperature:    request.Temperature,
		MaxTokens:      request.MaxTokens,
		TimeoutSeconds: request.TimeoutSeconds,
		StreamEnabled:  request.StreamEnabled,
		IsDefault:      request.IsDefault,
	}, nil
}

// TestRequest 是 Rust 注入运行期密钥后发给 Go core 的模型连通性测试输入。
type TestRequest struct {
	ConfigID       int64
	ResolvedAPIKey string
}

// LogSnapshot 返回测试请求的安全日志快照，只保留密钥存在状态。
func (request TestRequest) LogSnapshot() string {
	payload := map[string]any{
		"config_id":   request.ConfigID,
		"has_api_key": strings.TrimSpace(request.ResolvedAPIKey) != "",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return logger.RedactText(string(encoded))
}
