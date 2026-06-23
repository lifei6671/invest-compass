package news

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const newsCacheMaxAge = 60 * time.Minute

// Store 是 news action 依赖的数据访问边界。
type Store interface {
	SaveNewsItems(ctx context.Context, items []model.NewsItem) error
	ListNewsBySymbol(ctx context.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
	ListMarketNews(ctx context.Context, market string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
}

// Config 是新闻 action 的运行期依赖。
type Config struct {
	Security     httpx.SecurityConfig
	NewsProvider newsservice.Provider
	Store        Store
}

type listRequest struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

type marketRequest struct {
	Market string `json:"market"`
	Limit  int    `json:"limit"`
}

type listData struct {
	Items []itemData `json:"items"`
}

type itemData struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Summary     string    `json:"summary"`
	ContentHash string    `json:"content_hash"`
	PublishedAt time.Time `json:"published_at"`
	Symbols     []string  `json:"symbols"`
	Tags        []string  `json:"tags"`
}

// Routes 返回新闻相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/news/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/news/market", Handler: handleMarket(config)},
	}
}

// handleList 处理个股新闻请求，优先读取缓存，缓存不足时调用 Provider 并落库。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		var payload listRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		symbol, ok := parseSymbol(response, payload.Symbol, context)
		if !ok || !validateLimit(response, payload.Limit, context) {
			return
		}

		if config.Store != nil {
			items, err := config.Store.ListNewsBySymbol(request.Context(), symbol.String(), payload.Limit, newsCacheMaxAge)
			if err != nil {
				writeCacheError(response, context, "个股新闻缓存读取失败", err)
				return
			}
			if len(items) >= payload.Limit {
				httpx.WriteOK(response, listData{Items: itemDataFromModels(items)}, context)
				return
			}
		}
		if config.NewsProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "news_provider_unavailable", context)
			return
		}

		items, err := config.NewsProvider.List(request.Context(), newsservice.ListRequest{Symbol: symbol, Limit: payload.Limit})
		if err != nil {
			writeProviderError(response, context, config.NewsProvider.Name(), "list", err)
			return
		}
		normalized, err := normalizeItems(items)
		if err != nil {
			writeProviderError(response, context, config.NewsProvider.Name(), "normalize_list", err)
			return
		}
		if config.Store != nil {
			if err := config.Store.SaveNewsItems(request.Context(), modelNewsItemsFromService(normalized, symbol.Market)); err != nil {
				writeCacheError(response, context, "个股新闻缓存写入失败", err)
				return
			}
		}

		httpx.WriteOK(response, listData{Items: itemDataFromService(normalized, payload.Limit)}, context)
	}
}

// handleMarket 处理市场新闻请求，缓存不足时调用 Provider 并落库。
func handleMarket(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		var payload marketRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		market := strings.ToUpper(strings.TrimSpace(payload.Market))
		if market == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_market", context)
			return
		}
		if !validateLimit(response, payload.Limit, context) {
			return
		}

		if config.Store != nil {
			items, err := config.Store.ListMarketNews(request.Context(), market, payload.Limit, newsCacheMaxAge)
			if err != nil {
				writeCacheError(response, context, "市场新闻缓存读取失败", err)
				return
			}
			if len(items) >= payload.Limit {
				httpx.WriteOK(response, listData{Items: itemDataFromModels(items)}, context)
				return
			}
		}
		if config.NewsProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "news_provider_unavailable", context)
			return
		}

		items, err := config.NewsProvider.Market(request.Context(), newsservice.MarketRequest{Market: market, Limit: payload.Limit})
		if err != nil {
			writeProviderError(response, context, config.NewsProvider.Name(), "market", err)
			return
		}
		normalized, err := normalizeItems(items)
		if err != nil {
			writeProviderError(response, context, config.NewsProvider.Name(), "normalize_market", err)
			return
		}
		if config.Store != nil {
			if err := config.Store.SaveNewsItems(request.Context(), modelNewsItemsFromService(normalized, market)); err != nil {
				writeCacheError(response, context, "市场新闻缓存写入失败", err)
				return
			}
		}

		httpx.WriteOK(response, listData{Items: itemDataFromService(normalized, payload.Limit)}, context)
	}
}

// parseSymbol 将用户输入规范化为 service 层标准 symbol。
func parseSymbol(response http.ResponseWriter, raw string, context httpx.RequestContext) (stockservice.Symbol, bool) {
	symbol, err := stockservice.ParseSymbol(strings.TrimSpace(raw))
	if err != nil {
		httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_symbol", context)
		return stockservice.Symbol{}, false
	}
	return symbol, true
}

// validateLimit 校验新闻列表 limit，避免无界查询和无界 Provider 调用。
func validateLimit(response http.ResponseWriter, limit int, context httpx.RequestContext) bool {
	if limit <= 0 || limit > 100 {
		httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_limit", context)
		return false
	}
	return true
}

// normalizeItems 标准化、去重并按发布时间倒序整理新闻条目。
func normalizeItems(items []newsservice.Item) ([]newsservice.Item, error) {
	normalized, err := newsservice.NormalizeItems(items)
	if err != nil {
		return nil, err
	}
	deduped := newsservice.Deduplicate(normalized)
	sort.SliceStable(deduped, func(left int, right int) bool {
		return deduped[left].PublishedAt.After(deduped[right].PublishedAt)
	})
	return deduped, nil
}

// writeProviderError 统一处理新闻 Provider 或输出标准化失败，响应不暴露原始错误细节。
func writeProviderError(response http.ResponseWriter, context httpx.RequestContext, provider string, operation string, err error) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) && ruleError.Code == xerr.NewsProviderUnconfigured {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50302, string(ruleError.Code), context)
		return
	}
	slog.Warn(
		"新闻 Provider 调用失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		logger.FieldProvider, provider,
		"operation", operation,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusBadGateway, 50201, "news_provider_error", context)
}

// writeCacheError 统一处理新闻缓存读写失败，避免 handler 中散落数据库错误处理。
func writeCacheError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50006, "news_cache_error", context)
}

// itemDataFromService 转换 Provider 新闻为 API 响应字段。
func itemDataFromService(items []newsservice.Item, limit int) []itemData {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	result := make([]itemData, 0, len(items))
	for _, item := range items {
		result = append(result, itemData{
			ID:          item.ID,
			Source:      item.Source,
			Title:       item.Title,
			URL:         item.URL,
			Summary:     item.Summary,
			ContentHash: item.ContentHash,
			PublishedAt: item.PublishedAt,
			Symbols:     symbolStrings(item.Symbols),
			Tags:        append([]string(nil), item.Tags...),
		})
	}
	return result
}

// itemDataFromModels 转换缓存新闻为 API 响应字段。
func itemDataFromModels(items []model.NewsItem) []itemData {
	result := make([]itemData, 0, len(items))
	for _, item := range items {
		result = append(result, itemData{
			ID:          item.ContentHash,
			Source:      item.Source,
			Title:       item.Title,
			URL:         item.URL,
			Summary:     item.Summary,
			ContentHash: item.ContentHash,
			PublishedAt: item.PublishedAt,
			Symbols:     decodeStringList(item.Symbols),
			Tags:        decodeStringList(item.Tags),
		})
	}
	return result
}

// modelNewsItemsFromService 转换 Provider 新闻为可持久化缓存模型。
func modelNewsItemsFromService(items []newsservice.Item, market string) []model.NewsItem {
	normalizedMarket := strings.ToUpper(strings.TrimSpace(market))
	result := make([]model.NewsItem, 0, len(items))
	for _, item := range items {
		result = append(result, model.NewsItem{
			Source:      item.Source,
			Market:      normalizedMarket,
			Title:       item.Title,
			URL:         item.URL,
			Summary:     item.Summary,
			ContentHash: item.ContentHash,
			Symbols:     encodeStringList(symbolStrings(item.Symbols)),
			Tags:        encodeStringList(item.Tags),
			PublishedAt: item.PublishedAt,
		})
	}
	return result
}

// symbolStrings 将标准 symbol 转为稳定字符串列表。
func symbolStrings(symbols []stockservice.Symbol) []string {
	values := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		values = append(values, symbol.String())
	}
	return values
}

// encodeStringList 将字符串列表序列化为 JSON，避免分隔符歧义。
func encodeStringList(values []string) string {
	payload, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(payload)
}

// decodeStringList 从缓存字段读取字符串列表，兼容空值和早期逗号分隔测试数据。
func decodeStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.Split(raw, ",")
}
