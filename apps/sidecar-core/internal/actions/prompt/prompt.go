package prompt

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	promptservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/prompt"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是 Prompt 模板 action 依赖的数据访问边界。
type Store interface {
	SavePromptTemplate(ctx context.Context, template *model.PromptTemplate) error
	ListPromptTemplates(ctx context.Context) ([]model.PromptTemplate, error)
	SoftDeletePromptTemplate(ctx context.Context, id int64) error
}

// Config 是 Prompt 模板 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type listData struct {
	Items []templateData `json:"items"`
}

type idRequest struct {
	ID int64 `json:"id"`
}

type createRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type updateRequest struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type templateData struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Variables   []string `json:"variables"`
	IsBuiltin   bool     `json:"is_builtin"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// Routes 返回 Prompt 模板相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/prompt-templates/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/prompt-templates/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/prompt-templates/create", Handler: handleCreate(config)},
		{Method: http.MethodPost, Path: "/api/prompt-templates/update", Handler: handleUpdate(config)},
		{Method: http.MethodPost, Path: "/api/prompt-templates/delete", Handler: handleDelete(config)},
	}
}

// handleList 返回未软删除 Prompt 模板列表。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		models, err := config.Store.ListPromptTemplates(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取 Prompt 模板失败", err)
			return
		}
		templates, err := modelTemplatesToService(models)
		if err != nil {
			writeStoreError(response, context, "解析 Prompt 模板失败", err)
			return
		}
		httpx.WriteOK(response, listData{Items: serviceTemplatesToData(promptservice.ListTemplates(templates))}, context)
	}
}

// handleGet 按 ID 返回单个未软删除 Prompt 模板。
func handleGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload idRequest
		if !decodeIDRequest(response, request, context, &payload) {
			return
		}
		modelTemplate, ok := findActiveModelTemplate(response, request, config, context, payload.ID)
		if !ok {
			return
		}
		template, err := modelTemplateToService(modelTemplate)
		if err != nil {
			writeStoreError(response, context, "解析 Prompt 模板失败", err)
			return
		}
		httpx.WriteOK(response, serviceTemplateToData(template), context)
	}
}

// handleCreate 校验模板类型和变量白名单后创建 Prompt 模板。
func handleCreate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload createRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		template, err := promptservice.CreateTemplate(promptservice.CreateRequest{
			Name:        payload.Name,
			Type:        promptservice.TemplateType(payload.Type),
			Description: payload.Description,
			Content:     payload.Content,
			IsBuiltin:   false,
		}, time.Now().UTC())
		if err != nil {
			writeValidationError(response, context, err)
			return
		}

		modelTemplate, err := serviceTemplateToModel(template)
		if err != nil {
			writeStoreError(response, context, "序列化 Prompt 模板失败", err)
			return
		}
		if err := config.Store.SavePromptTemplate(request.Context(), &modelTemplate); err != nil {
			writeStoreError(response, context, "保存 Prompt 模板失败", err)
			return
		}
		httpx.WriteOK(response, modelTemplateToData(modelTemplate), context)
	}
}

// handleUpdate 更新非内置 Prompt 模板，并重新固化变量列表。
func handleUpdate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload updateRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40006, "invalid_prompt_template_id", context)
			return
		}

		modelTemplate, ok := findActiveModelTemplate(response, request, config, context, payload.ID)
		if !ok {
			return
		}
		existing, err := modelTemplateToService(modelTemplate)
		if err != nil {
			writeStoreError(response, context, "解析 Prompt 模板失败", err)
			return
		}
		updated, err := promptservice.UpdateTemplate(existing, promptservice.UpdateRequest{
			Name:        payload.Name,
			Type:        promptservice.TemplateType(payload.Type),
			Description: payload.Description,
			Content:     payload.Content,
		}, time.Now().UTC())
		if err != nil {
			writeValidationError(response, context, err)
			return
		}

		modelTemplate, err = serviceTemplateToModel(updated)
		if err != nil {
			writeStoreError(response, context, "序列化 Prompt 模板失败", err)
			return
		}
		if err := config.Store.SavePromptTemplate(request.Context(), &modelTemplate); err != nil {
			writeStoreError(response, context, "更新 Prompt 模板失败", err)
			return
		}
		httpx.WriteOK(response, modelTemplateToData(modelTemplate), context)
	}
}

// handleDelete 对允许删除的 Prompt 模板执行软删除。
func handleDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload idRequest
		if !decodeIDRequest(response, request, context, &payload) {
			return
		}
		modelTemplate, ok := findActiveModelTemplate(response, request, config, context, payload.ID)
		if !ok {
			return
		}
		template, err := modelTemplateToService(modelTemplate)
		if err != nil {
			writeStoreError(response, context, "解析 Prompt 模板失败", err)
			return
		}
		if _, err := promptservice.DeleteTemplate(template, time.Now().UTC()); err != nil {
			writeValidationError(response, context, err)
			return
		}
		if err := config.Store.SoftDeletePromptTemplate(request.Context(), payload.ID); err != nil {
			writeStoreError(response, context, "删除 Prompt 模板失败", err)
			return
		}

		httpx.WriteOK(response, map[string]int64{"id": payload.ID}, context)
	}
}

// requireStore 校验 ready/token 和 Prompt 模板 store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50306, "prompt_template_store_unavailable", context)
		return false
	}
	return true
}

// decodeIDRequest 解析 ID 请求并统一校验 ID 必须为正数。
func decodeIDRequest(response http.ResponseWriter, request *http.Request, context httpx.RequestContext, payload *idRequest) bool {
	if !httpx.DecodeJSON(response, request, context, payload) {
		return false
	}
	if payload.ID <= 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40006, "invalid_prompt_template_id", context)
		return false
	}
	return true
}

// findActiveModelTemplate 从可见模板中查找目标 ID，避免误操作已软删除模板。
func findActiveModelTemplate(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext, id int64) (model.PromptTemplate, bool) {
	models, err := config.Store.ListPromptTemplates(request.Context())
	if err != nil {
		writeStoreError(response, context, "读取 Prompt 模板失败", err)
		return model.PromptTemplate{}, false
	}
	for _, template := range models {
		if template.ID == id {
			return template, true
		}
	}
	httpx.WriteError(response, http.StatusNotFound, 40402, "prompt_template_not_found", context)
	return model.PromptTemplate{}, false
}

// writeValidationError 把 service 层稳定错误码映射为 HTTP 错误消息。
func writeValidationError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	if xerrValue, ok := err.(*xerr.Error); ok {
		httpx.WriteError(response, http.StatusBadRequest, 40007, string(xerrValue.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusBadRequest, 40007, "invalid_prompt_template", context)
}

// writeStoreError 记录脱敏后的数据库错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50006, "prompt_template_store_error", context)
}

// modelTemplatesToService 转换数据库模型列表为 service 模型列表。
func modelTemplatesToService(models []model.PromptTemplate) ([]promptservice.Template, error) {
	templates := make([]promptservice.Template, 0, len(models))
	for _, modelTemplate := range models {
		template, err := modelTemplateToService(modelTemplate)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

// modelTemplateToService 转换数据库模型为 Prompt service 模型。
func modelTemplateToService(modelTemplate model.PromptTemplate) (promptservice.Template, error) {
	var rawVariables []string
	if modelTemplate.Variables != "" {
		if err := json.Unmarshal([]byte(modelTemplate.Variables), &rawVariables); err != nil {
			return promptservice.Template{}, err
		}
	}
	variables := make([]promptservice.Variable, 0, len(rawVariables))
	for _, variable := range rawVariables {
		variables = append(variables, promptservice.Variable(variable))
	}
	return promptservice.Template{
		ID:          modelTemplate.ID,
		Name:        modelTemplate.Name,
		Type:        promptservice.TemplateType(modelTemplate.Type),
		Description: modelTemplate.Description,
		Content:     modelTemplate.Content,
		Variables:   variables,
		IsBuiltin:   modelTemplate.IsBuiltin,
		Deleted:     modelTemplate.DeletedAt.Valid,
		CreatedAt:   modelTemplate.CreatedAt,
		UpdatedAt:   modelTemplate.UpdatedAt,
		DeletedAt:   modelTemplate.DeletedAt.Time,
	}, nil
}

// serviceTemplateToModel 转换 Prompt service 模型为数据库模型。
func serviceTemplateToModel(template promptservice.Template) (model.PromptTemplate, error) {
	variables := variablesToStrings(template.Variables)
	payload, err := json.Marshal(variables)
	if err != nil {
		return model.PromptTemplate{}, err
	}
	return model.PromptTemplate{
		ID:          template.ID,
		Name:        template.Name,
		Type:        string(template.Type),
		Description: template.Description,
		Content:     template.Content,
		Variables:   string(payload),
		IsBuiltin:   template.IsBuiltin,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
	}, nil
}

// serviceTemplatesToData 转换 service 模型列表为 API 响应模型。
func serviceTemplatesToData(templates []promptservice.Template) []templateData {
	items := make([]templateData, 0, len(templates))
	for _, template := range templates {
		items = append(items, serviceTemplateToData(template))
	}
	return items
}

// serviceTemplateToData 转换单个 service 模型为 API 响应模型。
func serviceTemplateToData(template promptservice.Template) templateData {
	return templateData{
		ID:          template.ID,
		Name:        template.Name,
		Type:        string(template.Type),
		Description: template.Description,
		Content:     template.Content,
		Variables:   variablesToStrings(template.Variables),
		IsBuiltin:   template.IsBuiltin,
		CreatedAt:   formatTime(template.CreatedAt),
		UpdatedAt:   formatTime(template.UpdatedAt),
	}
}

// modelTemplateToData 转换数据库模型为 API 响应模型。
func modelTemplateToData(modelTemplate model.PromptTemplate) templateData {
	template, err := modelTemplateToService(modelTemplate)
	if err != nil {
		return templateData{}
	}
	return serviceTemplateToData(template)
}

// variablesToStrings 输出稳定字符串变量列表，避免前端接触内部类型。
func variablesToStrings(variables []promptservice.Variable) []string {
	result := make([]string, 0, len(variables))
	for _, variable := range variables {
		result = append(result, string(variable))
	}
	return result
}

// formatTime 统一输出 RFC3339 时间；零值保留为空字符串，便于前端区分未持久化状态。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
