package watchlist

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	watchlistservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/watchlist"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	watchlistRefreshTimeout = 60 * time.Second
	watchlistTrendPeriod    = string(marketservice.PeriodMinute)
	watchlistTrendAdjust    = string(marketservice.AdjustNone)
	watchlistTrendLimit     = 242
)

var watchlistRefreshInFlight sync.Map

// Store 是 watchlist action 依赖的数据访问边界。
type Store interface {
	SaveWatchlist(ctx context.Context, item *model.Watchlist) error
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	SoftDeleteWatchlist(ctx context.Context, id int64) error
}

// MarketStore 是自选股列表读取本地行情和分时缓存的边界。
type MarketStore interface {
	LatestQuote(ctx context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error)
	SaveQuote(ctx context.Context, quote *model.Quote) error
	ListKlines(ctx context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error)
	SaveKlines(ctx context.Context, klines []model.Kline) error
}

// StockProfileStore 是自选股列表可选使用的股票资料批量读取边界。
type StockProfileStore interface {
	GetStocksBySymbols(ctx context.Context, symbols []string) (map[string]model.Stock, error)
}

// Config 是 watchlist action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	Store          Store
	MarketProvider marketservice.MarketProvider
	MarketStore    MarketStore
}

type createRequest struct {
	Symbol    string   `json:"symbol"`
	SortOrder int      `json:"sort_order"`
	Tags      []string `json:"tags"`
	Note      string   `json:"note"`
}

type updateRequest struct {
	ID        int64    `json:"id"`
	SortOrder int      `json:"sort_order"`
	Tags      []string `json:"tags"`
	Note      string   `json:"note"`
}

type deleteRequest struct {
	ID int64 `json:"id"`
}

type refreshRequest struct {
	Symbols []string `json:"symbols"`
}

type listData struct {
	Items []itemData `json:"items"`
}

type refreshData struct {
	Accepted bool `json:"accepted"`
	Total    int  `json:"total"`
}

type quoteData struct {
	Symbol         string    `json:"symbol"`
	Price          float64   `json:"price"`
	ChangeAmount   float64   `json:"change_amount"`
	ChangePercent  float64   `json:"change_percent"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	PreClose       float64   `json:"pre_close"`
	Volume         float64   `json:"volume"`
	Amount         float64   `json:"amount"`
	TurnoverRate   float64   `json:"turnover_rate"`
	PE             float64   `json:"pe"`
	PB             float64   `json:"pb"`
	TotalMarketCap float64   `json:"total_market_cap"`
	FloatMarketCap float64   `json:"float_market_cap"`
	QuoteTime      time.Time `json:"quote_time"`
	Provider       string    `json:"provider"`
}

type itemData struct {
	ID        int64      `json:"id"`
	Symbol    string     `json:"symbol"`
	SortOrder int        `json:"sort_order"`
	Tags      []string   `json:"tags"`
	Note      string     `json:"note"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	Name      string     `json:"name,omitempty"`
	Code      string     `json:"code,omitempty"`
	Market    string     `json:"market,omitempty"`
	Exchange  string     `json:"exchange,omitempty"`
	Industry  string     `json:"industry,omitempty"`
	Concepts  []string   `json:"concepts,omitempty"`
	ListDate  string     `json:"list_date,omitempty"`
	Status    string     `json:"status,omitempty"`
	FullName  string     `json:"full_name,omitempty"`
	Quote     *quoteData `json:"quote,omitempty"`
	Trend     []float64  `json:"trend_points,omitempty"`
}

// Routes 返回自选股相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/watchlist/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/create", Handler: handleCreate(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/update", Handler: handleUpdate(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/delete", Handler: handleDelete(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/refresh", Handler: handleRefresh(config)},
	}
}

// handleList 返回未软删除自选股。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		items, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		profiles, err := loadStockProfiles(request.Context(), config.Store, items)
		if err != nil {
			writeStoreError(response, context, "读取自选股股票资料失败", err)
			return
		}
		quotes, trends, err := loadMarketCaches(request.Context(), config.MarketStore, items)
		if err != nil {
			writeStoreError(response, context, "读取自选股行情缓存失败", err)
			return
		}
		httpx.WriteOK(response, listData{Items: modelItemsToData(items, profiles, quotes, trends)}, context)
	}
}

// handleCreate 校验 symbol 并创建自选股。
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

		existing, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		item, err := watchlistservice.CreateItem(modelItemsToService(existing), watchlistservice.CreateRequest{
			Symbol:    payload.Symbol,
			SortOrder: payload.SortOrder,
			Tags:      payload.Tags,
			Note:      payload.Note,
		}, time.Now())
		if err != nil {
			writeValidationError(response, context, err)
			return
		}

		modelItem := serviceItemToModel(item)
		if err := config.Store.SaveWatchlist(request.Context(), &modelItem); err != nil {
			writeStoreError(response, context, "保存自选股失败", err)
			return
		}
		httpx.WriteOK(response, modelItemToData(modelItem, nil, nil, nil), context)
	}
}

// handleUpdate 更新自选股标签、备注和排序。
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

		items, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		modelItem, ok := findModelItem(items, payload.ID)
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40401, "watchlist_not_found", context)
			return
		}

		serviceItem, err := modelItemToService(modelItem)
		if err != nil {
			writeValidationError(response, context, err)
			return
		}
		updated := watchlistservice.UpdateItem(serviceItem, watchlistservice.UpdateRequest{
			SortOrder: payload.SortOrder,
			Tags:      payload.Tags,
			Note:      payload.Note,
		}, time.Now())

		modelItem = serviceItemToModel(updated)
		if err := config.Store.SaveWatchlist(request.Context(), &modelItem); err != nil {
			writeStoreError(response, context, "更新自选股失败", err)
			return
		}
		httpx.WriteOK(response, modelItemToData(modelItem, nil, nil, nil), context)
	}
}

// handleDelete 对自选股执行软删除。
func handleDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload deleteRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40004, "invalid_watchlist_id", context)
			return
		}
		if err := config.Store.SoftDeleteWatchlist(request.Context(), payload.ID); err != nil {
			writeStoreError(response, context, "删除自选股失败", err)
			return
		}

		httpx.WriteOK(response, map[string]int64{"id": payload.ID}, context)
	}
}

// handleRefresh 提交自选股行情后台刷新请求，HTTP 请求只负责入队，不等待 Provider 返回。
func handleRefresh(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}
		if config.MarketProvider == nil || config.MarketStore == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_refresh_unavailable", context)
			return
		}

		var payload refreshRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		items, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		symbols := selectedRefreshSymbols(items, payload.Symbols)
		symbols, releaseRefresh := claimWatchlistRefreshSymbols(symbols)
		if len(symbols) == 0 {
			httpx.WriteOK(response, refreshData{Accepted: true, Total: 0}, context)
			return
		}

		go func() {
			defer releaseRefresh()
			refreshWatchlistMarketCache(config, symbols)
		}()
		httpx.WriteOK(response, refreshData{Accepted: true, Total: len(symbols)}, context)
	}
}

// requireStore 校验 ready/token 和 watchlist store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "watchlist_store_unavailable", context)
		return false
	}
	return true
}

// writeValidationError 把 service 层稳定错误码映射为 HTTP 错误消息。
func writeValidationError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	if xerrValue, ok := err.(*xerr.Error); ok {
		httpx.WriteError(response, http.StatusBadRequest, 40005, string(xerrValue.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_watchlist", context)
}

// writeStoreError 记录脱敏后的数据库错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50005, "watchlist_store_error", context)
}

// modelItemsToData 转换数据库模型为 API 响应模型，并合并同 symbol 股票资料与本地行情缓存。
func modelItemsToData(items []model.Watchlist, profiles map[string]model.Stock, quotes map[string]model.Quote, trends map[string][]float64) []itemData {
	result := make([]itemData, 0, len(items))
	for _, item := range items {
		result = append(result, modelItemToData(item, profiles, quotes, trends))
	}
	return result
}

// modelItemToData 转换单个数据库模型为 API 响应模型。
func modelItemToData(item model.Watchlist, profiles map[string]model.Stock, quotes map[string]model.Quote, trends map[string][]float64) itemData {
	data := itemData{
		ID:        item.ID,
		Symbol:    item.Symbol,
		SortOrder: item.SortOrder,
		Tags:      decodeTags(item.Tags),
		Note:      item.Note,
		CreatedAt: formatAPITime(item.CreatedAt),
		UpdatedAt: formatAPITime(item.UpdatedAt),
	}
	if stock, ok := profiles[item.Symbol]; ok {
		data.Name = stock.Name
		data.Code = stock.Code
		data.Market = stock.Market
		data.Exchange = stock.Exchange
		data.Industry = stock.Industry
		data.Concepts = decodeConcepts(stock.Concept)
		data.ListDate = stock.ListDate
		data.Status = stock.Status
		data.FullName = stock.FullName
	}
	if quote, ok := quoteForSymbol(quotes, item.Symbol); ok {
		data.Quote = quoteDataFromModel(quote)
	}
	if points := trends[item.Symbol]; len(points) > 0 {
		data.Trend = points
	}
	return data
}

// loadMarketCaches 只读取本地缓存，不触发远端 Provider，保证进入自选股页面不阻塞 UI。
func loadMarketCaches(ctx context.Context, store MarketStore, items []model.Watchlist) (map[string]model.Quote, map[string][]float64, error) {
	if store == nil {
		return nil, nil, nil
	}
	quotes := make(map[string]model.Quote, len(items))
	trends := make(map[string][]float64, len(items))
	for _, item := range items {
		symbols := cacheLookupSymbols(item.Symbol)
		for _, symbol := range symbols {
			quote, hit, err := store.LatestQuote(ctx, symbol, 0)
			if err != nil {
				return nil, nil, err
			}
			if hit {
				quotes[item.Symbol] = quote
				break
			}
		}
		for _, symbol := range symbols {
			klines, err := store.ListKlines(ctx, symbol, watchlistTrendPeriod, watchlistTrendAdjust, watchlistTrendLimit)
			if err != nil {
				return nil, nil, err
			}
			if len(klines) > 0 {
				trends[item.Symbol] = trendPointsFromKlines(klines)
				break
			}
		}
	}
	return quotes, trends, nil
}

// cacheLookupSymbols 同时兼容历史 600000.SH 和规范 CN:SH:600000 缓存键。
func cacheLookupSymbols(raw string) []string {
	if symbol, err := stockFromModel(raw); err == nil && symbol.String() != raw {
		return []string{symbol.String(), raw}
	}
	return []string{raw}
}

// quoteForSymbol 从已装载缓存中读取当前自选项对应的 quote。
func quoteForSymbol(quotes map[string]model.Quote, symbol string) (model.Quote, bool) {
	if quotes == nil {
		return model.Quote{}, false
	}
	quote, ok := quotes[symbol]
	return quote, ok
}

// quoteDataFromModel 转换缓存 quote 为自选股列表可展示字段。
func quoteDataFromModel(quote model.Quote) *quoteData {
	normalized := marketservice.NormalizeQuote(marketservice.Quote{
		Price:          quote.Price,
		ChangeAmount:   quote.ChangeAmount,
		ChangePercent:  quote.ChangePercent,
		Open:           quote.Open,
		High:           quote.High,
		Low:            quote.Low,
		PreClose:       quote.PreClose,
		Volume:         quote.Volume,
		Amount:         quote.Amount,
		TurnoverRate:   quote.TurnoverRate,
		PE:             quote.PE,
		PB:             quote.PB,
		TotalMarketCap: quote.TotalMarketCap,
		FloatMarketCap: quote.FloatMarketCap,
		QuoteTime:      quote.QuoteTime,
		Provider:       quote.Provider,
	})
	return &quoteData{
		Symbol:         quote.Symbol,
		Price:          normalized.Price,
		ChangeAmount:   normalized.ChangeAmount,
		ChangePercent:  normalized.ChangePercent,
		Open:           normalized.Open,
		High:           normalized.High,
		Low:            normalized.Low,
		PreClose:       normalized.PreClose,
		Volume:         normalized.Volume,
		Amount:         normalized.Amount,
		TurnoverRate:   normalized.TurnoverRate,
		PE:             normalized.PE,
		PB:             normalized.PB,
		TotalMarketCap: normalized.TotalMarketCap,
		FloatMarketCap: normalized.FloatMarketCap,
		QuoteTime:      normalized.QuoteTime,
		Provider:       normalized.Provider,
	}
}

// trendPointsFromKlines 只暴露最新交易日 close 序列，前端迷你走势图不需要完整 K 线字段。
func trendPointsFromKlines(klines []model.Kline) []float64 {
	tradeDate := latestKlineDate(klines)
	points := make([]float64, 0, len(klines))
	for _, item := range klines {
		if tradeDate != "" && klineDate(item.TradeDate) != tradeDate {
			continue
		}
		points = append(points, item.Close)
	}
	return points
}

// latestKlineDate 提取 K 线交易日期前缀，支持 "YYYY-MM-DD HH:mm" 和 RFC3339 文本。
func latestKlineDate(klines []model.Kline) string {
	for index := len(klines) - 1; index >= 0; index -= 1 {
		value := strings.TrimSpace(klines[index].TradeDate)
		if date := klineDate(value); date != "" {
			return date
		}
	}
	return ""
}

// klineDate 返回 K 线交易时间中的日期部分。
func klineDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < len("2006-01-02") {
		return ""
	}
	return value[:len("2006-01-02")]
}

// selectedRefreshSymbols 将前端请求收口到当前 active 自选股，避免刷新任意 symbol。
func selectedRefreshSymbols(items []model.Watchlist, requested []string) []string {
	active := make(map[string]string, len(items)*2)
	for _, item := range items {
		active[item.Symbol] = item.Symbol
		if parsed, err := stockFromModel(item.Symbol); err == nil {
			active[parsed.String()] = item.Symbol
		}
	}
	if len(requested) == 0 {
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.Symbol)
		}
		return result
	}
	seen := make(map[string]struct{}, len(requested))
	result := make([]string, 0, len(requested))
	for _, raw := range requested {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if parsed, err := stockFromModel(key); err == nil {
			key = parsed.String()
		}
		activeSymbol, ok := active[key]
		if !ok {
			continue
		}
		if _, duplicated := seen[activeSymbol]; duplicated {
			continue
		}
		seen[activeSymbol] = struct{}{}
		result = append(result, activeSymbol)
	}
	return result
}

// claimWatchlistRefreshSymbols 标记正在刷新的 symbol，避免重复点击并发打同一 Provider。
func claimWatchlistRefreshSymbols(symbols []string) ([]string, func()) {
	claimed := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if _, loaded := watchlistRefreshInFlight.LoadOrStore(symbol, struct{}{}); loaded {
			continue
		}
		claimed = append(claimed, symbol)
	}
	return claimed, func() {
		for _, symbol := range claimed {
			watchlistRefreshInFlight.Delete(symbol)
		}
	}
}

// refreshWatchlistMarketCache 后台刷新 quote 和分时走势，失败按 symbol 记录日志，不阻塞 UI。
func refreshWatchlistMarketCache(config Config, symbols []string) {
	ctx, cancel := context.WithTimeout(context.Background(), watchlistRefreshTimeout)
	defer cancel()

	for _, rawSymbol := range symbols {
		symbol, err := stockFromModel(rawSymbol)
		if err != nil {
			logWatchlistRefreshWarn("自选股刷新跳过非法股票代码", rawSymbol, err)
			continue
		}
		quote, err := config.MarketProvider.Quote(ctx, symbol)
		if err != nil {
			logWatchlistRefreshWarn("自选股行情刷新失败", rawSymbol, err)
		} else {
			modelQuote := modelQuoteFromMarket(quote)
			if err := config.MarketStore.SaveQuote(ctx, &modelQuote); err != nil {
				logWatchlistRefreshWarn("自选股行情缓存写入失败", rawSymbol, err)
			}
		}

		bars, err := config.MarketProvider.Kline(ctx, marketservice.KlineRequest{
			Symbol: symbol,
			Period: marketservice.PeriodMinute,
			Adjust: marketservice.AdjustNone,
			Limit:  watchlistTrendLimit,
		})
		if err != nil {
			logWatchlistRefreshWarn("自选股分时刷新失败", rawSymbol, err)
			continue
		}
		if err := config.MarketStore.SaveKlines(ctx, modelKlinesFromMarket(bars)); err != nil {
			logWatchlistRefreshWarn("自选股分时缓存写入失败", rawSymbol, err)
		}
	}
}

// modelQuoteFromMarket 转换 Provider quote 为可持久化缓存模型。
func modelQuoteFromMarket(quote marketservice.Quote) model.Quote {
	return model.Quote{
		Symbol:         quote.Symbol.String(),
		Price:          quote.Price,
		ChangeAmount:   quote.ChangeAmount,
		ChangePercent:  quote.ChangePercent,
		Open:           quote.Open,
		High:           quote.High,
		Low:            quote.Low,
		PreClose:       quote.PreClose,
		Volume:         quote.Volume,
		Amount:         quote.Amount,
		TurnoverRate:   quote.TurnoverRate,
		PE:             quote.PE,
		PB:             quote.PB,
		TotalMarketCap: quote.TotalMarketCap,
		FloatMarketCap: quote.FloatMarketCap,
		QuoteTime:      quote.QuoteTime,
		Provider:       quote.Provider,
	}
}

// modelKlinesFromMarket 转换 Provider K 线为可持久化缓存模型。
func modelKlinesFromMarket(bars []marketservice.KlineBar) []model.Kline {
	result := make([]model.Kline, 0, len(bars))
	for _, bar := range bars {
		result = append(result, model.Kline{
			Symbol:    bar.Symbol.String(),
			Period:    string(bar.Period),
			Adjust:    string(bar.Adjust),
			TradeDate: bar.TradeDate,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Amount:    bar.Amount,
			Provider:  bar.Provider,
		})
	}
	return result
}

// logWatchlistRefreshWarn 记录后台刷新异常，避免错误吞掉后难以定位。
func logWatchlistRefreshWarn(message string, symbol string, err error) {
	slog.Warn(
		message,
		"symbol", symbol,
		"error", logger.RedactError(err),
	)
}

// formatAPITime 使用 RFC3339 输出前端可稳定排序和展示的时间字段。
func formatAPITime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// modelItemsToService 转换数据库模型为 service 模型，用于复用业务规则。
func modelItemsToService(items []model.Watchlist) []watchlistservice.Item {
	result := make([]watchlistservice.Item, 0, len(items))
	for _, item := range items {
		serviceItem, err := modelItemToService(item)
		if err == nil {
			result = append(result, serviceItem)
		}
	}
	return result
}

// modelItemToService 转换单个数据库模型为 service 模型。
func modelItemToService(item model.Watchlist) (watchlistservice.Item, error) {
	symbol, err := stockFromModel(item.Symbol)
	if err != nil {
		return watchlistservice.Item{}, err
	}
	return watchlistservice.Item{
		ID:        item.ID,
		Symbol:    symbol,
		SortOrder: item.SortOrder,
		Tags:      decodeTags(item.Tags),
		Note:      item.Note,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

// serviceItemToModel 转换 service 模型为数据库模型。
func serviceItemToModel(item watchlistservice.Item) model.Watchlist {
	return model.Watchlist{
		ID:        item.ID,
		Symbol:    item.Symbol.String(),
		SortOrder: item.SortOrder,
		Tags:      encodeTags(item.Tags),
		Note:      item.Note,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

// findModelItem 按 ID 查找当前 active 自选股。
func findModelItem(items []model.Watchlist, id int64) (model.Watchlist, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Watchlist{}, false
}

// encodeTags 使用 JSON 数组保存标签，避免分隔符和用户输入冲突。
func encodeTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(payload)
}

// decodeTags 解析 JSON 标签数组，历史异常值按空标签处理。
func decodeTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

// loadStockProfiles 使用同一个 Store 的可选能力批量读取股票资料，未实现时保持列表基础字段可用。
func loadStockProfiles(ctx context.Context, store Store, items []model.Watchlist) (map[string]model.Stock, error) {
	profileStore, ok := store.(StockProfileStore)
	if !ok {
		return nil, nil
	}
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		symbols = append(symbols, item.Symbol)
	}
	return profileStore.GetStocksBySymbols(ctx, symbols)
}

// decodeConcepts 兼容 JSON 数组和历史分隔符文本，统一向前端暴露数组。
func decodeConcepts(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var concepts []string
	if err := json.Unmarshal([]byte(value), &concepts); err == nil {
		return concepts
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	result := make([]string, 0, len(fields))
	for _, item := range fields {
		if concept := strings.TrimSpace(item); concept != "" {
			result = append(result, concept)
		}
	}
	return result
}

// stockFromModel 解析数据库中的标准 symbol。
func stockFromModel(symbol string) (stockservice.Symbol, error) {
	parsed, err := stockservice.ParseSymbol(symbol)
	if err != nil {
		return stockservice.Symbol{}, err
	}
	return parsed, nil
}
