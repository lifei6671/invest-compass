package aiconfig

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是 AI 配置 action 依赖的数据访问边界。
type Store interface {
	SaveAIConfig(ctx context.Context, config *model.AIConfig) error
	ListAIConfigs(ctx context.Context) ([]model.AIConfig, error)
	GetAIConfig(ctx context.Context, id int64) (model.AIConfig, error)
	SoftDeleteAIConfig(ctx context.Context, id int64) error
}

// Config 是 AI 配置 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
	Tester   aiservice.ConfigTester
}

type listRequest struct{}

type testRequest struct {
	ID             int64  `json:"id"`
	ResolvedAPIKey string `json:"resolved_api_key"`
}

type deleteRequest struct {
	ID int64 `json:"id"`
}

type saveRequest struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Provider       string  `json:"provider"`
	BaseURL        string  `json:"base_url"`
	APIKeyRef      string  `json:"api_key_ref"`
	MaskedAPIKey   string  `json:"masked_api_key"`
	HasAPIKey      bool    `json:"has_api_key"`
	RawAPIKey      string  `json:"raw_api_key"`
	ModelName      string  `json:"model_name"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	StreamEnabled  bool    `json:"stream_enabled"`
	IsDefault      bool    `json:"is_default"`
}

type savedData struct {
	Config aiservice.ListView `json:"config"`
}

type listData struct {
	Items []aiservice.ListView `json:"items"`
}

type deletedData struct {
	DeletedID int64 `json:"deleted_id"`
}

// Routes 返回 AI 配置相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/ai/configs/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/ai/configs/save", Handler: handleSave(config)},
		{Method: http.MethodPost, Path: "/api/ai/configs/delete", Handler: handleDelete(config)},
		{Method: http.MethodPost, Path: "/api/ai/configs/test", Handler: handleTest(config)},
	}
}

// handleList 返回不包含真实 API Key 的配置列表。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Store == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "ai_config_store_unavailable", context)
			return
		}

		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		configs, err := config.Store.ListAIConfigs(request.Context())
		if err != nil {
			writeStoreError(response, err, context)
			return
		}
		httpx.WriteOK(response, listData{Items: configsToViews(configs)}, context)
	}
}

// handleTest 使用 Rust 注入的运行期密钥测试模型连通性，不保存明文 Key。
func handleTest(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Store == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "ai_config_store_unavailable", context)
			return
		}
		tester := config.Tester
		if tester == nil {
			tester = aiservice.OpenAIConfigTester{}
		}

		var payload testRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if payload.ID == 0 || payload.ResolvedAPIKey == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40022, string(xerr.AIInvalidRequest), context)
			return
		}

		configModel, err := config.Store.GetAIConfig(request.Context(), payload.ID)
		if err != nil {
			writeStoreError(response, err, context)
			return
		}
		result, err := tester.TestAIConfig(request.Context(), modelToConfig(configModel), payload.ResolvedAPIKey)
		if err != nil {
			writeTesterError(response, err, context)
			return
		}
		httpx.WriteOK(response, result, context)
	}
}

// handleSave 保存 Rust 处理凭据后的 AI 配置元数据，拒绝明文 Key。
func handleSave(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Store == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "ai_config_store_unavailable", context)
			return
		}

		var payload saveRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		serviceConfig, err := aiservice.BuildConfigForSave(aiservice.SaveRequest{
			ID:             payload.ID,
			Name:           payload.Name,
			Provider:       payload.Provider,
			BaseURL:        payload.BaseURL,
			APIKeyRef:      payload.APIKeyRef,
			MaskedAPIKey:   payload.MaskedAPIKey,
			HasAPIKey:      payload.HasAPIKey,
			RawAPIKey:      payload.RawAPIKey,
			ModelName:      payload.ModelName,
			Temperature:    payload.Temperature,
			MaxTokens:      payload.MaxTokens,
			TimeoutSeconds: payload.TimeoutSeconds,
			StreamEnabled:  payload.StreamEnabled,
			IsDefault:      payload.IsDefault,
		})
		if err != nil {
			writeRuleError(response, err, context)
			return
		}

		modelConfig := configToModel(serviceConfig)
		if err := config.Store.SaveAIConfig(request.Context(), &modelConfig); err != nil {
			writeStoreError(response, err, context)
			return
		}
		httpx.WriteOK(response, savedData{Config: modelToConfig(modelConfig).ListView()}, context)
	}
}

// handleDelete 对 AI 配置执行软删除，真实凭据删除由 Rust 凭据服务负责。
func handleDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Store == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50307, "ai_config_store_unavailable", context)
			return
		}

		var payload deleteRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if err := config.Store.SoftDeleteAIConfig(request.Context(), payload.ID); err != nil {
			writeStoreError(response, err, context)
			return
		}
		httpx.WriteOK(response, deletedData{DeletedID: payload.ID}, context)
	}
}

// configsToViews 转换持久化模型为安全展示模型。
func configsToViews(configs []model.AIConfig) []aiservice.ListView {
	views := make([]aiservice.ListView, 0, len(configs))
	for _, config := range configs {
		views = append(views, modelToConfig(config).ListView())
	}
	return views
}

// configToModel 转换 service 配置为数据库模型。
func configToModel(config aiservice.Config) model.AIConfig {
	return model.AIConfig{
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

// modelToConfig 转换数据库模型为 service 配置。
func modelToConfig(config model.AIConfig) aiservice.Config {
	return aiservice.Config{
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

// writeRuleError 将 AI 配置规则错误映射为稳定 HTTP 错误响应。
func writeRuleError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) {
		httpx.WriteError(response, http.StatusBadRequest, 40021, string(ruleError.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusInternalServerError, 50021, "ai_config_failed", context)
}

// writeStoreError 记录数据访问错误且不泄露密钥。
func writeStoreError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	slog.Warn(
		"AI 配置数据访问失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50022, "ai_config_store_failed", context)
}

// writeTesterError 将 Provider 测试错误映射为稳定响应，避免泄露密钥和请求头。
func writeTesterError(response http.ResponseWriter, err error, context httpx.RequestContext) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) {
		slog.Warn(
			"AI 配置连通性测试失败",
			logger.FieldRequestID, context.RequestID,
			logger.FieldTraceID, context.TraceID,
			"error", logger.RedactError(err),
		)
		httpx.WriteError(response, http.StatusBadGateway, 50201, string(ruleError.Code), context)
		return
	}
	slog.Warn(
		"AI 配置连通性测试失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusBadGateway, 50201, string(xerr.AIUpstream), context)
}
