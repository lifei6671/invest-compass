package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dashboard"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/settings"
)

const tokenHeader = "X-Invest-Compass-Token"

// Config 是 Go core 本地 HTTP handler 的运行期依赖配置。
type Config struct {
	Version        string
	Token          string
	DBStatus       string
	Ready          bool
	MarketProvider market.MarketProvider
	DashboardInput dashboard.Input
	CacheUsages    []settings.CacheUsage
	CacheCleaner   CacheCleaner
	OnShutdown     func()
}

// CacheCleaner 是缓存清理 API 的单一执行边界，由实际存储层注入。
type CacheCleaner interface {
	CleanCache(ctx context.Context, targets []settings.CacheTarget) error
}

type apiResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

type healthData struct {
	Version  string `json:"version"`
	DBStatus string `json:"dbStatus"`
}

// stockSearchRequest 是股票搜索 API 的请求体。
type stockSearchRequest struct {
	Keyword string `json:"keyword"`
}

// stockSearchResult 是股票搜索 API 对前端稳定暴露的基础信息字段。
type stockSearchResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Market   string `json:"market"`
	Exchange string `json:"exchange"`
}

// cacheCleanRequest 是缓存清理 API 的请求体。
type cacheCleanRequest struct {
	Targets []settings.CacheTarget `json:"targets"`
}

type appHandler struct {
	config Config
}

// NewHandler 创建本地 HTTP handler，并把当前进程的 ready 状态和 token 边界注入进去。
func NewHandler(config Config) http.Handler {
	return recoverHTTP(appHandler{config: config})
}

// recoverHTTP 将 handler panic 转为统一错误响应，并确保异常文本先脱敏再进入日志。
func recoverHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				context := requestContextFrom(request)
				slog.Error(
					"本地 HTTP handler panic",
					logger.FieldRequestID, context.requestID,
					logger.FieldTraceID, context.traceID,
					"error", logger.RedactText(fmt.Sprint(recovered)),
				)
				writeError(response, http.StatusInternalServerError, 50000, "internal_error", context)
			}
		}()

		next.ServeHTTP(response, request)
	})
}

// ServeHTTP 统一处理本地 core API 的入口约束，确保所有请求先经过 POST 限制。
func (handler appHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	context := requestContextFrom(request)

	// 本地 core API 统一只接受 POST，避免浏览器或外部工具通过 GET 探测业务接口。
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, 40500, "method_not_allowed", context)
		return
	}

	switch request.URL.Path {
	case "/internal/health":
		handler.handleHealth(response, request, context)
	case "/internal/shutdown":
		handler.handleShutdown(response, request, context)
	case "/api/stocks/search":
		handler.handleStockSearch(response, request, context)
	case "/api/providers/status":
		handler.handleProviderStatus(response, request, context)
	case "/api/dashboard/summary":
		handler.handleDashboardSummary(response, request, context)
	case "/api/cache/stats":
		handler.handleCacheStats(response, request, context)
	case "/api/cache/clean":
		handler.handleCacheClean(response, request, context)
	default:
		writeError(response, http.StatusNotFound, 40400, "not_found", context)
	}
}

// handleHealth 返回最小健康信息，只在 sidecar 握手完成且 token 正确时可用。
func (handler appHandler) handleHealth(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.config.Ready || handler.config.Token == "" {
		writeError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return
	}
	if !tokenMatches(request.Header.Get(tokenHeader), handler.config.Token) {
		writeError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:    0,
		Message: "ok",
		Data: healthData{
			Version:  handler.config.Version,
			DBStatus: handler.config.DBStatus,
		},
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleShutdown 处理 Rust 生命周期管理层发起的关闭请求，并复用 runtime token 安全边界。
func (handler appHandler) handleShutdown(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.config.Ready || handler.config.Token == "" {
		writeError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return
	}
	if !tokenMatches(request.Header.Get(tokenHeader), handler.config.Token) {
		writeError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      map[string]string{"status": "shutting_down"},
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})

	if handler.config.OnShutdown != nil {
		handler.config.OnShutdown()
	}
}

// handleStockSearch 处理股票搜索请求，返回标准化股票基础信息。
func (handler appHandler) handleStockSearch(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.requireReadyToken(response, request, context) {
		return
	}
	if handler.config.MarketProvider == nil {
		writeError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
		return
	}

	var payload stockSearchRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return
	}

	keyword := strings.TrimSpace(payload.Keyword)
	if keyword == "" {
		writeError(response, http.StatusBadRequest, 40002, "invalid_keyword", context)
		return
	}

	stocks, err := handler.config.MarketProvider.Search(request.Context(), keyword)
	if err != nil {
		slog.Warn(
			"股票搜索 Provider 调用失败",
			logger.FieldRequestID, context.requestID,
			logger.FieldTraceID, context.traceID,
			logger.FieldProvider, handler.config.MarketProvider.Name(),
			"error", logger.RedactError(err),
		)
		writeError(response, http.StatusBadGateway, 50200, "market_provider_error", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      buildStockSearchResults(stocks),
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleProviderStatus 返回数据源状态的安全展示数据。
func (handler appHandler) handleProviderStatus(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.requireReadyToken(response, request, context) {
		return
	}
	if handler.config.MarketProvider == nil {
		writeError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
		return
	}

	status := handler.config.MarketProvider.Status(request.Context())
	summary := dashboard.BuildSummary(dashboard.Input{
		ProviderStatuses: []market.ProviderStatus{status},
	})

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      summary.ProviderStatuses,
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleDashboardSummary 返回 Dashboard 首版聚合结果，数据只来自 Config.DashboardInput。
func (handler appHandler) handleDashboardSummary(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.requireReadyToken(response, request, context) {
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      dashboard.BuildSummary(handler.config.DashboardInput),
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleCacheStats 返回可清理临时缓存统计，不包含报告和配置。
func (handler appHandler) handleCacheStats(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.requireReadyToken(response, request, context) {
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      settings.BuildCacheStats(handler.config.CacheUsages),
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleCacheClean 只清理 settings 允许的临时缓存目标。
func (handler appHandler) handleCacheClean(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.requireReadyToken(response, request, context) {
		return
	}
	if handler.config.CacheCleaner == nil {
		writeError(response, http.StatusServiceUnavailable, 50302, "cache_cleaner_unavailable", context)
		return
	}

	var payload cacheCleanRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return
	}

	targets := settings.FilterCacheCleanupTargets(payload.Targets)
	if err := handler.config.CacheCleaner.CleanCache(request.Context(), targets); err != nil {
		slog.Warn(
			"缓存清理失败",
			logger.FieldRequestID, context.requestID,
			logger.FieldTraceID, context.traceID,
			"error", logger.RedactError(err),
		)
		writeError(response, http.StatusInternalServerError, 50001, "cache_clean_failed", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      map[string][]settings.CacheTarget{"cleaned_targets": targets},
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// requireReadyToken 校验业务 API 必须在 sidecar ready 后携带正确 runtime token。
func (handler appHandler) requireReadyToken(response http.ResponseWriter, request *http.Request, context requestContext) bool {
	if !handler.config.Ready || handler.config.Token == "" {
		writeError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return false
	}
	if !tokenMatches(request.Header.Get(tokenHeader), handler.config.Token) {
		writeError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return false
	}
	return true
}

// buildStockSearchResults 转换 Provider 模型为 API 稳定响应字段。
func buildStockSearchResults(stocks []market.StockBasic) []stockSearchResult {
	results := make([]stockSearchResult, 0, len(stocks))
	for _, item := range stocks {
		results = append(results, stockSearchResult{
			Symbol:   item.Symbol.String(),
			Name:     item.Name,
			Code:     item.Code,
			Market:   item.Market,
			Exchange: item.Exchange,
		})
	}
	return results
}

type requestContext struct {
	requestID string
	traceID   string
}

// requestContextFrom 从请求头提取追踪 ID，缺失时生成本地 ID 方便排障关联。
func requestContextFrom(request *http.Request) requestContext {
	requestID := request.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = newID()
	}

	traceID := request.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = newID()
	}

	return requestContext{
		requestID: requestID,
		traceID:   traceID,
	}
}

// writeError 用统一错误结构返回失败原因，避免各 handler 自行拼响应。
func writeError(response http.ResponseWriter, status int, code int, message string, context requestContext) {
	writeJSON(response, status, apiResponse{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// writeJSON 写入统一 JSON 响应，保证 Content-Type 和 envelope 结构一致。
func writeJSON(response http.ResponseWriter, status int, payload apiResponse) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		panic(err)
	}
}

// tokenMatches 使用常量时间比较校验 runtime token，避免在安全边界上使用普通字符串比较。
func tokenMatches(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

// newID 生成 requestId/traceId，在随机源异常时退化为时间戳编码以保留可观测性。
func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}

	// 系统随机源异常时仍返回可追踪 ID，避免错误响应缺少 requestId/traceId。
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}
