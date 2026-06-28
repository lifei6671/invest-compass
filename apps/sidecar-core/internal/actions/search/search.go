package search

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
)

// Service 是菜单范围搜索 action 依赖的业务边界，不暴露任意 doc_type 搜索入口。
type Service interface {
	SearchReports(ctx context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error)
	SearchNews(ctx context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error)
	SearchWatchlistNotes(ctx context.Context, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error)
	Status(ctx context.Context) (searchservice.SearchStatus, error)
	Rebuild(ctx context.Context, request searchservice.SearchRebuildRequest) (searchservice.SearchRebuildAccepted, error)
}

// Config 是菜单范围搜索 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Service  Service
}

type documentSearchRequest struct {
	Keyword string   `json:"keyword"`
	Symbols []string `json:"symbols"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Sort    string   `json:"sort"`
}

type documentSearchResult struct {
	DocUID     string   `json:"doc_uid"`
	DocType    string   `json:"doc_type"`
	RefID      string   `json:"ref_id"`
	URL        string   `json:"url"`
	Symbol     string   `json:"symbol"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Source     string   `json:"source"`
	SourceTime string   `json:"source_time"`
	Score      float64  `json:"score"`
	Tags       []string `json:"tags"`
	Sentiment  string   `json:"sentiment"`
	Highlights []string `json:"highlights"`
}

type rebuildRequest struct {
	Scope string `json:"scope"`
}

// Routes 返回菜单范围搜索路由定义，不提供全局搜索路径。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/search/reports", Handler: handleSearchReports(config)},
		{Method: http.MethodPost, Path: "/api/search/news", Handler: handleSearchNews(config)},
		{Method: http.MethodPost, Path: "/api/search/watchlist-notes", Handler: handleSearchWatchlistNotes(config)},
		{Method: http.MethodPost, Path: "/api/search/status", Handler: handleStatus(config)},
		{Method: http.MethodPost, Path: "/api/search/rebuild", Handler: handleRebuild(config)},
	}
}

// handleSearchReports 处理报告历史菜单范围搜索。
func handleSearchReports(config Config) http.HandlerFunc {
	return handleDocumentSearch(config, func(ctx context.Context, service Service, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
		return service.SearchReports(ctx, request)
	})
}

// handleSearchNews 处理资讯中心菜单范围搜索。
func handleSearchNews(config Config) http.HandlerFunc {
	return handleDocumentSearch(config, func(ctx context.Context, service Service, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
		return service.SearchNews(ctx, request)
	})
}

// handleSearchWatchlistNotes 处理自选备注菜单范围搜索。
func handleSearchWatchlistNotes(config Config) http.HandlerFunc {
	return handleDocumentSearch(config, func(ctx context.Context, service Service, request searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error) {
		return service.SearchWatchlistNotes(ctx, request)
	})
}

// handleStatus 返回搜索索引运行状态。
func handleStatus(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "document_search_service_unavailable", context)
			return
		}
		var payload struct{}
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		status, err := config.Service.Status(request.Context())
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "search_status_error", context)
			return
		}
		httpx.WriteOK(response, status, context)
	}
}

// handleRebuild 触发搜索索引重建，scope 必须来自固定白名单。
func handleRebuild(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "document_search_service_unavailable", context)
			return
		}
		var payload rebuildRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		scope := strings.TrimSpace(payload.Scope)
		if !isAllowedRebuildScope(scope) {
			httpx.WriteError(response, http.StatusBadRequest, 40004, "invalid_scope", context)
			return
		}
		result, err := config.Service.Rebuild(request.Context(), searchservice.SearchRebuildRequest{Scope: scope})
		if err != nil {
			switch {
			case errors.Is(err, searchservice.ErrSearchRebuildAlreadyRunning):
				httpx.WriteError(response, http.StatusConflict, 40900, "search_rebuild_already_running", context)
			case errors.Is(err, searchservice.ErrSearchFTS5Unavailable):
				httpx.WriteError(response, http.StatusServiceUnavailable, 50302, "search_fts5_unavailable", context)
			default:
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "search_rebuild_error", context)
			}
			return
		}
		httpx.WriteOK(response, result, context)
	}
}

// handleDocumentSearch 复用 token、ready 和 JSON 解码边界，scope 只能由内部调用方固定选择。
func handleDocumentSearch(config Config, call func(context.Context, Service, searchservice.DocumentSearchRequest) ([]searchservice.DocumentSearchResult, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "document_search_service_unavailable", context)
			return
		}

		var payload documentSearchRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if strings.TrimSpace(payload.Keyword) == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_keyword", context)
			return
		}
		if payload.Limit < 0 || payload.Limit > 100 || payload.Offset < 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40003, "invalid_request", context)
			return
		}

		results, err := call(request.Context(), config.Service, searchservice.DocumentSearchRequest{
			Keyword: strings.TrimSpace(payload.Keyword),
			Symbols: payload.Symbols,
			Limit:   payload.Limit,
			Offset:  payload.Offset,
			Sort:    strings.TrimSpace(payload.Sort),
		})
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "document_search_error", context)
			return
		}
		httpx.WriteOK(response, buildDocumentSearchResponse(results), context)
	}
}

// isAllowedRebuildScope 限制重建范围，避免出现全局搜索或任意业务对象组合。
func isAllowedRebuildScope(scope string) bool {
	switch scope {
	case searchservice.SearchRebuildScopeAll, "stock", "reports", "news", "watchlist_notes":
		return true
	default:
		return false
	}
}

// buildDocumentSearchResponse 转换 service 结果为 API 稳定 JSON 字段。
func buildDocumentSearchResponse(results []searchservice.DocumentSearchResult) []documentSearchResult {
	items := make([]documentSearchResult, 0, len(results))
	for _, result := range results {
		highlights := result.Highlights
		if highlights == nil {
			highlights = []string{}
		}
		tags := result.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, documentSearchResult{
			DocUID:     result.DocUID,
			DocType:    result.DocType,
			RefID:      result.RefID,
			URL:        result.URL,
			Symbol:     result.Symbol,
			Title:      result.Title,
			Summary:    result.Summary,
			Source:     result.Source,
			SourceTime: formatSearchSourceTime(result.SourceTime),
			Score:      result.Score,
			Tags:       tags,
			Sentiment:  result.Sentiment,
			Highlights: highlights,
		})
	}
	return items
}

// formatSearchSourceTime 统一搜索结果时间序列化，空时间返回空字符串。
func formatSearchSourceTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
