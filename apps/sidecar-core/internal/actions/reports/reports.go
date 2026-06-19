package reports

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	reportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/report"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是 reports action 依赖的数据访问边界。
type Store interface {
	ListVisibleAnalysisReports(ctx context.Context) ([]model.AnalysisReport, error)
	SoftDeleteAnalysisReport(ctx context.Context, id int64) error
}

// Config 是报告历史 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type idRequest struct {
	ID int64 `json:"id"`
}

type listData struct {
	Items []reportData `json:"items"`
}

type reportData struct {
	ID               int64  `json:"id"`
	TaskID           string `json:"task_id"`
	Symbol           string `json:"symbol"`
	Title            string `json:"title"`
	AnalysisType     string `json:"analysis_type"`
	ModelName        string `json:"model_name"`
	PromptTemplateID int64  `json:"prompt_template_id"`
	ContentMarkdown  string `json:"content_markdown,omitempty"`
	RiskSummary      string `json:"risk_summary"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// Routes 返回报告历史相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/reports/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/reports/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/reports/delete", Handler: handleDelete(config)},
	}
}

// handleList 返回可见报告列表，默认不包含正文和完整输入快照。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		modelReports, err := config.Store.ListVisibleAnalysisReports(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取报告列表失败", err)
			return
		}
		reports := reportservice.UniqueByTaskID(reportservice.VisibleReports(modelReportsToService(modelReports)))
		httpx.WriteOK(response, listData{Items: reportsToData(reports, false)}, context)
	}
}

// handleGet 返回单个报告详情，仍然不回显完整 input_snapshot。
func handleGet(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		payload, ok := decodeIDRequest(response, request, context)
		if !ok {
			return
		}
		modelReports, err := config.Store.ListVisibleAnalysisReports(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取报告详情失败", err)
			return
		}
		report, err := reportservice.FindVisibleReport(modelReportsToService(modelReports), payload.ID)
		if err != nil {
			writeReportError(response, context, err)
			return
		}
		httpx.WriteOK(response, reportToData(report, true), context)
	}
}

// handleDelete 对报告执行软删除，删除后列表和详情不可见。
func handleDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		payload, ok := decodeIDRequest(response, request, context)
		if !ok {
			return
		}
		modelReports, err := config.Store.ListVisibleAnalysisReports(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取报告详情失败", err)
			return
		}
		if _, err := reportservice.FindVisibleReport(modelReportsToService(modelReports), payload.ID); err != nil {
			writeReportError(response, context, err)
			return
		}
		if err := config.Store.SoftDeleteAnalysisReport(request.Context(), payload.ID); err != nil {
			writeStoreError(response, context, "删除报告失败", err)
			return
		}
		httpx.WriteOK(response, map[string]int64{"id": payload.ID}, context)
	}
}

// requireStore 校验 ready/token 和报告 store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50308, "report_store_unavailable", context)
		return false
	}
	return true
}

// decodeIDRequest 解析报告 ID 请求并校验 ID 必须为正数。
func decodeIDRequest(response http.ResponseWriter, request *http.Request, context httpx.RequestContext) (idRequest, bool) {
	var payload idRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return idRequest{}, false
	}
	if payload.ID <= 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40009, "invalid_report_id", context)
		return idRequest{}, false
	}
	return payload, true
}

// writeReportError 把报告 service 稳定错误码映射为 HTTP 错误响应。
func writeReportError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	if xerrValue, ok := err.(*xerr.Error); ok && xerrValue.Code == xerr.ReportNotFound {
		httpx.WriteError(response, http.StatusNotFound, 40404, string(xerrValue.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusBadRequest, 40010, "invalid_report", context)
}

// writeStoreError 记录脱敏后的报告存储错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50008, "report_store_error", context)
}

// modelReportsToService 转换数据库报告列表为 service 模型列表。
func modelReportsToService(modelReports []model.AnalysisReport) []reportservice.Report {
	reports := make([]reportservice.Report, 0, len(modelReports))
	for _, modelReport := range modelReports {
		reports = append(reports, modelReportToService(modelReport))
	}
	return reports
}

// modelReportToService 转换数据库报告模型为 service 模型。
func modelReportToService(modelReport model.AnalysisReport) reportservice.Report {
	var deletedAt *time.Time
	if modelReport.DeletedAt.Valid {
		value := modelReport.DeletedAt.Time
		deletedAt = &value
	}
	return reportservice.Report{
		ID:               modelReport.ID,
		TaskID:           modelReport.TaskID,
		Symbol:           modelReport.Symbol,
		Title:            modelReport.Title,
		AnalysisType:     modelReport.AnalysisType,
		ModelName:        modelReport.ModelName,
		PromptTemplateID: modelReport.PromptTemplateID,
		InputSnapshot:    modelReport.InputSnapshot,
		ContentMarkdown:  modelReport.ContentMarkdown,
		RiskSummary:      modelReport.RiskSummary,
		CreatedAt:        modelReport.CreatedAt,
		UpdatedAt:        modelReport.UpdatedAt,
		DeletedAt:        deletedAt,
	}
}

// reportsToData 转换报告列表为 API 响应模型。
func reportsToData(reports []reportservice.Report, includeContent bool) []reportData {
	items := make([]reportData, 0, len(reports))
	for _, report := range reports {
		items = append(items, reportToData(report, includeContent))
	}
	return items
}

// reportToData 转换报告为 API 响应模型，明确不包含 input_snapshot。
func reportToData(report reportservice.Report, includeContent bool) reportData {
	data := reportData{
		ID:               report.ID,
		TaskID:           report.TaskID,
		Symbol:           report.Symbol,
		Title:            report.Title,
		AnalysisType:     report.AnalysisType,
		ModelName:        report.ModelName,
		PromptTemplateID: report.PromptTemplateID,
		RiskSummary:      logger.RedactText(report.RiskSummary),
		CreatedAt:        formatTime(report.CreatedAt),
		UpdatedAt:        formatTime(report.UpdatedAt),
	}
	if includeContent {
		data.ContentMarkdown = logger.RedactText(report.ContentMarkdown)
	}
	return data
}

// formatTime 统一输出 RFC3339 时间；零值保留为空字符串。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
