package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestConfigListViewNeverContainsRawAPIKey 验证配置列表只返回凭据引用和脱敏状态。
func TestConfigListViewNeverContainsRawAPIKey(t *testing.T) {
	config := Config{
		ID:             1,
		Name:           "OpenAI",
		Provider:       ProviderOpenAICompatible,
		BaseURL:        "https://api.openai.com",
		APIKeyRef:      "local-vault://ai-config/openai-compatible-1",
		MaskedAPIKey:   "sk-p...7890",
		HasAPIKey:      true,
		ModelName:      "gpt-4.1-mini",
		Temperature:    0.7,
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		StreamEnabled:  true,
		IsDefault:      true,
		CreatedAt:      time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
	}

	view := config.ListView()

	if view.APIKeyRef != "local-vault://ai-config/openai-compatible-1" || view.MaskedAPIKey != "sk-p...7890" || !view.HasAPIKey {
		t.Fatalf("unexpected safe key metadata: %+v", view)
	}
	if strings.Contains(view.String(), "raw_api_key") || strings.Contains(view.String(), "resolved_api_key") {
		t.Fatalf("list view leaked raw key: %s", view.String())
	}
}

// TestSaveRequestRejectsRawAPIKey 验证 Go core 保存模型配置时不接受真实 API Key。
func TestSaveRequestRejectsRawAPIKey(t *testing.T) {
	_, err := BuildConfigForSave(SaveRequest{
		Name:           "OpenAI",
		Provider:       ProviderOpenAICompatible,
		BaseURL:        "https://api.openai.com",
		APIKeyRef:      "local-vault://ai-config/openai-compatible-1",
		MaskedAPIKey:   "sk-p...7890",
		HasAPIKey:      true,
		RawAPIKey:      "sk-raw-secret",
		ModelName:      "gpt-4.1-mini",
		TimeoutSeconds: 120,
	})

	assertAIErrorCode(t, err, xerr.AIRawAPIKeyNotAllowed)
}

// TestSaveRequestRejectsInvalidAPIKeyRef 验证 Go core 只接受 Rust 本地 vault 生成的 AI Key 引用。
func TestSaveRequestRejectsInvalidAPIKeyRef(t *testing.T) {
	for _, apiKeyRef := range []string{
		"file:///tmp/plain-secret",
		"local-vault://proxy/default",
		"plain-reference",
	} {
		_, err := BuildConfigForSave(SaveRequest{
			Name:         "OpenAI",
			Provider:     ProviderOpenAICompatible,
			BaseURL:      "https://api.openai.com",
			APIKeyRef:    apiKeyRef,
			MaskedAPIKey: "sk-p...7890",
			HasAPIKey:    true,
			ModelName:    "gpt-4.1-mini",
		})

		assertAIErrorCode(t, err, xerr.AIInvalidCredentialRef)
	}
}

// TestSaveRequestRejectsUnmaskedAPIKeyDisplay 验证脱敏展示字段不能被明文 API Key 旁路滥用。
func TestSaveRequestRejectsUnmaskedAPIKeyDisplay(t *testing.T) {
	_, err := BuildConfigForSave(SaveRequest{
		Name:         "OpenAI",
		Provider:     ProviderOpenAICompatible,
		BaseURL:      "https://api.openai.com",
		APIKeyRef:    "local-vault://ai-config/openai-compatible-1",
		MaskedAPIKey: "sk-live-raw-secret-123456",
		HasAPIKey:    true,
		ModelName:    "gpt-4.1-mini",
	})

	assertAIErrorCode(t, err, xerr.AIRawAPIKeyNotAllowed)
}

// TestBuildConfigForSaveKeepsOnlyCredentialMetadata 验证保存模型只保留凭据引用和脱敏状态。
func TestBuildConfigForSaveKeepsOnlyCredentialMetadata(t *testing.T) {
	config, err := BuildConfigForSave(SaveRequest{
		Name:           "OpenAI",
		Provider:       ProviderOpenAICompatible,
		BaseURL:        "https://api.openai.com",
		APIKeyRef:      "local-vault://ai-config/openai-compatible-1",
		MaskedAPIKey:   "sk-p...7890",
		HasAPIKey:      true,
		ModelName:      "gpt-4.1-mini",
		Temperature:    0.7,
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		StreamEnabled:  true,
	})
	if err != nil {
		t.Fatalf("BuildConfigForSave returned error: %v", err)
	}

	if config.APIKeyRef != "local-vault://ai-config/openai-compatible-1" || config.MaskedAPIKey != "sk-p...7890" || !config.HasAPIKey {
		t.Fatalf("unexpected saved key metadata: %+v", config)
	}
}

// TestTestRequestLogSnapshotRedactsResolvedKey 验证测试连接的运行期密钥不会进入日志快照。
func TestTestRequestLogSnapshotRedactsResolvedKey(t *testing.T) {
	request := TestRequest{
		ConfigID:       1,
		ResolvedAPIKey: "sk-runtime-secret",
	}

	snapshot := request.LogSnapshot()

	if strings.Contains(snapshot, "sk-runtime-secret") || strings.Contains(snapshot, "resolved_api_key") {
		t.Fatalf("test request log snapshot leaked runtime key: %s", snapshot)
	}
	if !strings.Contains(snapshot, "has_api_key") {
		t.Fatalf("test request log snapshot should keep key presence: %s", snapshot)
	}
}
