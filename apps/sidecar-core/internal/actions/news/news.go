package news

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
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
const newsSearchIndexTimeout = 30 * time.Second

// Store 是 news action 依赖的数据访问边界。
type Store interface {
	SaveNewsItems(ctx stdcontext.Context, items []model.NewsItem) error
	ListNewsBySymbol(ctx stdcontext.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
	ListMarketNews(ctx stdcontext.Context, market string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
}

// SearchIndexer 是新闻入库后写入 active 文档搜索索引的最小依赖边界。
type SearchIndexer interface {
	IndexNews(ctx stdcontext.Context, item model.NewsItem) error
}

// Config 是新闻 action 的运行期依赖。
type Config struct {
	Security      httpx.SecurityConfig
	NewsProvider  newsservice.Provider
	Store         Store
	SearchIndexer SearchIndexer
}

type listRequest struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

type marketRequest struct {
	Market       string `json:"market"`
	Limit        int    `json:"limit"`
	ForceRefresh bool   `json:"force_refresh"`
}

type statsRequest struct {
	Market string `json:"market"`
	Limit  int    `json:"limit"`
}

type listData struct {
	Items []itemData `json:"items"`
}

type statsData struct {
	TotalCount             int    `json:"total_count"`
	SourceCount            int    `json:"source_count"`
	LatestPublished        string `json:"latest_published_at"`
	SentimentPositiveCount int    `json:"sentiment_positive_count"`
	SentimentNeutralCount  int    `json:"sentiment_neutral_count"`
	SentimentNegativeCount int    `json:"sentiment_negative_count"`
	SentimentSummary       string `json:"sentiment_summary"`
}

type hotTopicsData struct {
	Industries      []hotIndustryData    `json:"industries"`
	MentionedStocks []mentionedStockData `json:"mentioned_stocks"`
	UpdatedAt       string               `json:"updated_at"`
}

type hotIndustryData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mentionedStockData struct {
	Symbol string `json:"symbol"`
	Count  int    `json:"count"`
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
	Sentiment   string    `json:"sentiment"`
}

// Routes 返回新闻相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/news/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/news/market", Handler: handleMarket(config)},
		{Method: http.MethodPost, Path: "/api/news/stats", Handler: handleStats(config)},
		{Method: http.MethodPost, Path: "/api/news/hot-topics", Handler: handleHotTopics(config)},
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

		var cachedItems []model.NewsItem
		if config.Store != nil {
			items, err := config.Store.ListNewsBySymbol(request.Context(), symbol.String(), payload.Limit, newsCacheMaxAge)
			if err != nil {
				writeCacheError(response, context, "个股新闻缓存读取失败", err)
				return
			}
			cachedItems = items
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
			if isUnsupportedStockNewsList(err) {
				httpx.WriteOK(response, listData{Items: itemDataFromModels(cachedItems)}, context)
				return
			}
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
			triggerNewsSearchIndex(config, request.Context(), context, func(ctx stdcontext.Context) ([]model.NewsItem, error) {
				return config.Store.ListNewsBySymbol(ctx, symbol.String(), payload.Limit, 0)
			})
		}

		httpx.WriteOK(response, listData{Items: itemDataFromService(normalized, payload.Limit)}, context)
	}
}

// isUnsupportedStockNewsList 判断 Provider 是否仅不支持个股新闻；该场景应回退到缓存而不是让详情页整体失败。
func isUnsupportedStockNewsList(err error) bool {
	var providerError *newsservice.ProviderError
	return errors.As(err, &providerError) && providerError.Operation == "list_unsupported"
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

		var cachedItems []model.NewsItem
		if config.Store != nil {
			items, err := config.Store.ListMarketNews(request.Context(), market, payload.Limit, newsCacheMaxAge)
			if err != nil {
				writeCacheError(response, context, "市场新闻缓存读取失败", err)
				return
			}
			cachedItems = items
			if !payload.ForceRefresh && len(items) >= payload.Limit {
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
			if len(cachedItems) > 0 {
				slog.Warn(
					"市场新闻 Provider 调用失败，返回已有缓存",
					logger.FieldRequestID, context.RequestID,
					logger.FieldTraceID, context.TraceID,
					logger.FieldProvider, config.NewsProvider.Name(),
					"error", logger.RedactError(err),
				)
				httpx.WriteOK(response, listData{Items: itemDataFromModels(cachedItems)}, context)
				return
			}
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
				slog.Warn(
					"市场新闻缓存写入失败，返回本次 Provider 结果",
					logger.FieldRequestID, context.RequestID,
					logger.FieldTraceID, context.TraceID,
					"error", logger.RedactError(err),
				)
			} else {
				triggerNewsSearchIndex(config, request.Context(), context, func(ctx stdcontext.Context) ([]model.NewsItem, error) {
					return config.Store.ListMarketNews(ctx, market, payload.Limit, 0)
				})
			}
		}

		httpx.WriteOK(response, listData{Items: itemDataFromService(normalized, payload.Limit)}, context)
	}
}

// triggerNewsSearchIndex 在新闻入库成功后后台增量写入 active news FTS 索引，不创建完整重建任务。
func triggerNewsSearchIndex(config Config, parent stdcontext.Context, requestContext httpx.RequestContext, load func(stdcontext.Context) ([]model.NewsItem, error)) {
	if config.SearchIndexer == nil || load == nil {
		return
	}
	go func() {
		indexContext, cancel := stdcontext.WithTimeout(stdcontext.WithoutCancel(parent), newsSearchIndexTimeout)
		defer cancel()
		items, err := load(indexContext)
		if err != nil {
			slog.Warn(
				"资讯入库后搜索索引回源失败",
				logger.FieldRequestID, requestContext.RequestID,
				logger.FieldTraceID, requestContext.TraceID,
				"error", logger.RedactError(err),
			)
			return
		}
		for _, item := range items {
			if item.ID == 0 {
				continue
			}
			if err := config.SearchIndexer.IndexNews(indexContext, item); err != nil {
				slog.Warn(
					"资讯入库后搜索索引增量写入失败",
					logger.FieldRequestID, requestContext.RequestID,
					logger.FieldTraceID, requestContext.TraceID,
					"error", logger.RedactError(err),
				)
				return
			}
		}
	}()
}

// handleStats 返回资讯中心侧栏统计，统计口径仅来自本地新闻缓存。
func handleStats(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		payload, ok := decodeStatsPayload(response, request, context)
		if !ok {
			return
		}
		items, ok := loadCachedMarketNews(response, request, context, config, payload.Market, payload.Limit)
		if !ok {
			return
		}
		httpx.WriteOK(response, buildStatsData(items), context)
	}
}

// handleHotTopics 返回资讯热点标签和高频关联股票，避免前端硬编码热点榜。
func handleHotTopics(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		payload, ok := decodeStatsPayload(response, request, context)
		if !ok {
			return
		}
		items, ok := loadCachedMarketNews(response, request, context, config, payload.Market, payload.Limit)
		if !ok {
			return
		}
		httpx.WriteOK(response, buildHotTopicsData(items), context)
	}
}

// decodeStatsPayload 解析资讯统计类请求，共用 market 和 limit 边界校验。
func decodeStatsPayload(response http.ResponseWriter, request *http.Request, context httpx.RequestContext) (statsRequest, bool) {
	var payload statsRequest
	if !httpx.DecodeJSON(response, request, context, &payload) {
		return statsRequest{}, false
	}
	payload.Market = strings.ToUpper(strings.TrimSpace(payload.Market))
	if payload.Market == "" {
		httpx.WriteError(response, http.StatusBadRequest, 40008, "invalid_market", context)
		return statsRequest{}, false
	}
	if !validateLimit(response, payload.Limit, context) {
		return statsRequest{}, false
	}
	return payload, true
}

// loadCachedMarketNews 只读取本地缓存，不触发外部 Provider，保证侧栏统计不会伪造或隐式联网。
func loadCachedMarketNews(response http.ResponseWriter, request *http.Request, context httpx.RequestContext, config Config, market string, limit int) ([]model.NewsItem, bool) {
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "news_store_unavailable", context)
		return nil, false
	}
	items, err := config.Store.ListMarketNews(request.Context(), market, limit, 0)
	if err != nil {
		writeCacheError(response, context, "市场新闻缓存读取失败", err)
		return nil, false
	}
	return items, true
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
			Sentiment:   newsSentimentLabel(item.Sentiment, item.Title, item.Summary),
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
			Sentiment:   newsservice.AnalyzeSentiment(newsSentimentText(item.Title, item.Summary)).Label,
		})
	}
	return result
}

// buildStatsData 从缓存新闻构造来源、时间和情绪统计摘要。
func buildStatsData(items []model.NewsItem) statsData {
	sources := make(map[string]struct{})
	var latest time.Time
	var positiveCount int
	var neutralCount int
	var negativeCount int
	for _, item := range items {
		if source := strings.TrimSpace(item.Source); source != "" {
			sources[source] = struct{}{}
		}
		if item.PublishedAt.After(latest) {
			latest = item.PublishedAt
		}
		switch newsservice.AnalyzeSentiment(newsSentimentText(item.Title, item.Summary)).Label {
		case "positive":
			positiveCount++
		case "negative":
			negativeCount++
		default:
			neutralCount++
		}
	}
	latestText := ""
	if !latest.IsZero() {
		latestText = latest.Format(time.RFC3339Nano)
	}
	return statsData{
		TotalCount:             len(items),
		SourceCount:            len(sources),
		LatestPublished:        latestText,
		SentimentPositiveCount: positiveCount,
		SentimentNeutralCount:  neutralCount,
		SentimentNegativeCount: negativeCount,
		SentimentSummary:       formatSentimentSummary(positiveCount, neutralCount, negativeCount),
	}
}

// newsSentimentText 合并标题和摘要，作为本地规则情绪分析输入。
func newsSentimentText(title string, summary string) string {
	return strings.TrimSpace(strings.Join([]string{title, summary}, "\n"))
}

// newsSentimentLabel 返回 Provider 显式情绪标签，缺失时按标题和摘要本地计算。
func newsSentimentLabel(label string, title string, summary string) string {
	label = strings.TrimSpace(label)
	switch label {
	case "positive", "neutral", "negative":
		return label
	default:
		return newsservice.AnalyzeSentiment(newsSentimentText(title, summary)).Label
	}
}

// formatSentimentSummary 生成资讯中心侧栏情绪统计文案。
func formatSentimentSummary(positiveCount int, neutralCount int, negativeCount int) string {
	return "利好 " + strconv.Itoa(positiveCount) + " 条，中性 " + strconv.Itoa(neutralCount) + " 条，利空 " + strconv.Itoa(negativeCount) + " 条。"
}

// buildHotTopicsData 从新闻 tags 和 symbols 统计热点，排序稳定且不依赖前端硬编码。
func buildHotTopicsData(items []model.NewsItem) hotTopicsData {
	tagCounts := make(map[string]int)
	symbolCounts := make(map[string]int)
	var latest time.Time
	for _, item := range items {
		if item.PublishedAt.After(latest) {
			latest = item.PublishedAt
		}
		for _, tag := range decodeStringList(item.Tags) {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagCounts[tag]++
			}
		}
		for _, symbol := range decodeStringList(item.Symbols) {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				symbolCounts[symbol]++
			}
		}
	}
	updatedAt := ""
	if !latest.IsZero() {
		updatedAt = latest.Format(time.RFC3339Nano)
	}
	return hotTopicsData{
		Industries:      topHotIndustries(tagCounts, 5),
		MentionedStocks: topMentionedStocks(symbolCounts, 10),
		UpdatedAt:       updatedAt,
	}
}

// topHotIndustries 按出现次数和名称稳定排序，返回侧栏可展示的热点标签。
func topHotIndustries(counts map[string]int, limit int) []hotIndustryData {
	items := make([]hotIndustryData, 0, len(counts))
	for name, count := range counts {
		items = append(items, hotIndustryData{Name: name, Count: count})
	}
	sort.SliceStable(items, func(left int, right int) bool {
		if items[left].Count == items[right].Count {
			return items[left].Name < items[right].Name
		}
		return items[left].Count > items[right].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

// topMentionedStocks 按出现次数和 symbol 稳定排序，返回缓存新闻中的高频提及股票。
func topMentionedStocks(counts map[string]int, limit int) []mentionedStockData {
	items := make([]mentionedStockData, 0, len(counts))
	for symbol, count := range counts {
		items = append(items, mentionedStockData{Symbol: symbol, Count: count})
	}
	sort.SliceStable(items, func(left int, right int) bool {
		if items[left].Count == items[right].Count {
			return items[left].Symbol < items[right].Symbol
		}
		return items[left].Count > items[right].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
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
