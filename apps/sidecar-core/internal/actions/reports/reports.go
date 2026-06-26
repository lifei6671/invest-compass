package reports

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"
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
	BatchSoftDeleteAnalysisReports(ctx context.Context, ids []int64) error
	UpdateAnalysisReportFavorite(ctx context.Context, id int64, favorite bool) error
}

// Config 是报告历史 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type idRequest struct {
	ID int64 `json:"id"`
}

type batchDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

type updateRequest struct {
	ID       int64 `json:"id"`
	Favorite bool  `json:"favorite"`
}

type listData struct {
	Items []reportData `json:"items"`
}

type batchDeleteData struct {
	IDs []int64 `json:"ids"`
}

type exportData struct {
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}

type statsData struct {
	Total           int              `json:"total"`
	UniqueSymbols   int              `json:"unique_symbols"`
	LatestCreatedAt string           `json:"latest_created_at"`
	AnalysisTypes   []statsCountData `json:"analysis_types"`
	TopModels       []statsCountData `json:"top_models"`
}

type statsCountData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
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
	Favorite         bool   `json:"favorite"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// Routes 返回报告历史相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/reports/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/reports/get", Handler: handleGet(config)},
		{Method: http.MethodPost, Path: "/api/reports/delete", Handler: handleDelete(config)},
		{Method: http.MethodPost, Path: "/api/reports/batch-delete", Handler: handleBatchDelete(config)},
		{Method: http.MethodPost, Path: "/api/reports/update", Handler: handleUpdate(config)},
		{Method: http.MethodPost, Path: "/api/reports/export", Handler: handleExport(config)},
		{Method: http.MethodPost, Path: "/api/reports/stats", Handler: handleStats(config)},
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

// handleBatchDelete 对一组报告执行软删除，任一 ID 不可见时整体失败，避免部分删除造成误解。
func handleBatchDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		payload, ok := decodeBatchDeleteRequest(response, request, context)
		if !ok {
			return
		}
		modelReports, err := config.Store.ListVisibleAnalysisReports(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取报告详情失败", err)
			return
		}
		reports := modelReportsToService(modelReports)
		for _, id := range payload.IDs {
			if _, err := reportservice.FindVisibleReport(reports, id); err != nil {
				writeReportError(response, context, err)
				return
			}
		}
		if err := config.Store.BatchSoftDeleteAnalysisReports(request.Context(), payload.IDs); err != nil {
			writeStoreError(response, context, "批量删除报告失败", err)
			return
		}
		httpx.WriteOK(response, batchDeleteData{IDs: payload.IDs}, context)
	}
}

// handleUpdate 更新报告元数据。当前只允许修改收藏状态，正文和输入快照不可通过该接口变更。
func handleUpdate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		payload, ok := decodeUpdateRequest(response, request, context)
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
		if err := config.Store.UpdateAnalysisReportFavorite(request.Context(), payload.ID, payload.Favorite); err != nil {
			writeStoreError(response, context, "更新报告收藏状态失败", err)
			return
		}
		report.Favorite = payload.Favorite
		httpx.WriteOK(response, reportToData(report, false), context)
	}
}

// handleExport 生成报告导出内容。默认不包含 input_snapshot，避免一次性持仓等敏感上下文被写入文件。
func handleExport(config Config) http.HandlerFunc {
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
			writeStoreError(response, context, "读取报告导出内容失败", err)
			return
		}
		report, err := reportservice.FindVisibleReport(modelReportsToService(modelReports), payload.ID)
		if err != nil {
			writeReportError(response, context, err)
			return
		}
		content := reportservice.ExportMarkdown(report, reportservice.ExportOptions{})
		httpx.WriteOK(response, exportData{
			FileName: safeExportFileName(report),
			Content:  logger.RedactText(content),
		}, context)
	}
}

// handleStats 返回报告聚合统计，只使用报告元数据，不包含正文或输入快照。
func handleStats(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		modelReports, err := config.Store.ListVisibleAnalysisReports(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取报告统计失败", err)
			return
		}
		reports := reportservice.UniqueByTaskID(reportservice.VisibleReports(modelReportsToService(modelReports)))
		httpx.WriteOK(response, buildStatsData(reports), context)
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

// decodeBatchDeleteRequest 解析批量删除 ID，拒绝空列表和非正数 ID。
func decodeBatchDeleteRequest(response http.ResponseWriter, request *http.Request, context httpx.RequestContext) (batchDeleteRequest, bool) {
	var payload batchDeleteRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return batchDeleteRequest{}, false
	}
	if len(payload.IDs) == 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40009, "invalid_report_ids", context)
		return batchDeleteRequest{}, false
	}
	seen := make(map[int64]struct{}, len(payload.IDs))
	ids := make([]int64, 0, len(payload.IDs))
	for _, id := range payload.IDs {
		if id <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40009, "invalid_report_ids", context)
			return batchDeleteRequest{}, false
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return batchDeleteRequest{IDs: ids}, true
}

// decodeUpdateRequest 解析报告更新请求，并限制只能操作有效报告 ID。
func decodeUpdateRequest(response http.ResponseWriter, request *http.Request, context httpx.RequestContext) (updateRequest, bool) {
	var payload updateRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return updateRequest{}, false
	}
	if payload.ID <= 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40009, "invalid_report_id", context)
		return updateRequest{}, false
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

// buildStatsData 从可见报告元数据聚合统计，避免把正文、风险摘要或输入快照放入统计响应。
func buildStatsData(reports []reportservice.Report) statsData {
	symbols := make(map[string]struct{}, len(reports))
	analysisTypeCounts := make(map[string]int)
	modelCounts := make(map[string]int)
	var latestCreatedAt time.Time
	for _, report := range reports {
		if report.Symbol != "" {
			symbols[report.Symbol] = struct{}{}
		}
		if report.AnalysisType != "" {
			analysisTypeCounts[report.AnalysisType]++
		}
		if report.ModelName != "" {
			modelCounts[report.ModelName]++
		}
		if report.CreatedAt.After(latestCreatedAt) {
			latestCreatedAt = report.CreatedAt
		}
	}
	return statsData{
		Total:           len(reports),
		UniqueSymbols:   len(symbols),
		LatestCreatedAt: formatTime(latestCreatedAt),
		AnalysisTypes:   countsToData(analysisTypeCounts),
		TopModels:       countsToData(modelCounts),
	}
}

// safeExportFileName 生成本地保存对话框使用的建议文件名，避免路径分隔符进入 Rust 文件写入边界。
func safeExportFileName(report reportservice.Report) string {
	title := strings.TrimSpace(report.Title)
	if title == "" {
		title = "report"
	}
	var builder strings.Builder
	for _, char := range title {
		switch char {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			builder.WriteRune('-')
		default:
			if char < 32 {
				builder.WriteRune('-')
				continue
			}
			builder.WriteRune(char)
		}
		if builder.Len() >= 80 {
			break
		}
	}
	name := strings.Trim(builder.String(), " .-")
	if name == "" {
		name = "report"
	}
	return name + ".md"
}

// countsToData 将计数 map 转成稳定排序的响应数组，便于前端和测试复现。
func countsToData(counts map[string]int) []statsCountData {
	items := make([]statsCountData, 0, len(counts))
	for name, count := range counts {
		items = append(items, statsCountData{Name: name, Count: count})
	}
	sort.SliceStable(items, func(left int, right int) bool {
		if items[left].Count == items[right].Count {
			return items[left].Name < items[right].Name
		}
		return items[left].Count > items[right].Count
	})
	return items
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
		Favorite:         modelReport.Favorite,
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
		Favorite:         report.Favorite,
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
