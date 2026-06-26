package market

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	indicatorservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/indicator"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const quoteCacheTTL = 30 * time.Second

// Store 是 market action 依赖的数据访问边界。
type Store interface {
	LatestQuote(ctx context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error)
	SaveQuote(ctx context.Context, quote *model.Quote) error
	ListKlines(ctx context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error)
	SaveKlines(ctx context.Context, klines []model.Kline) error
}

// Config 是行情 action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	MarketProvider marketservice.MarketProvider
	Store          Store
}

type quoteRequest struct {
	Symbol       string `json:"symbol"`
	ForceRefresh bool   `json:"force_refresh"`
}

type klineRequest struct {
	Symbol string `json:"symbol"`
	Period string `json:"period"`
	Adjust string `json:"adjust"`
	Limit  int    `json:"limit"`
}

type indicatorsRequest struct {
	Symbol     string   `json:"symbol"`
	Period     string   `json:"period"`
	Adjust     string   `json:"adjust"`
	Limit      int      `json:"limit"`
	Indicators []string `json:"indicators"`
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

type klineData struct {
	Items []klineItem `json:"items"`
}

type indicatorsData struct {
	Symbol     string         `json:"symbol"`
	Period     string         `json:"period"`
	Adjust     string         `json:"adjust"`
	Indicators map[string]any `json:"indicators"`
}

type klineItem struct {
	Symbol    string  `json:"symbol"`
	Period    string  `json:"period"`
	Adjust    string  `json:"adjust"`
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Amount    float64 `json:"amount"`
	Provider  string  `json:"provider"`
}

// Routes 返回行情相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/market/quote", Handler: handleQuote(config)},
		{Method: http.MethodPost, Path: "/api/market/kline", Handler: handleKline(config)},
		{Method: http.MethodPost, Path: "/api/market/indicators", Handler: handleIndicators(config)},
	}
}

// handleQuote 处理行情快照请求；首页/手动刷新可通过 force_refresh 绕过短缓存读取 Provider。
func handleQuote(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		var payload quoteRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		symbol, ok := parseSymbol(response, payload.Symbol, context)
		if !ok {
			return
		}

		if config.Store != nil && !payload.ForceRefresh {
			quote, hit, err := config.Store.LatestQuote(request.Context(), symbol.String(), quoteCacheTTL)
			if err != nil {
				writeCacheError(response, context, "行情快照缓存读取失败", err)
				return
			}
			if hit {
				httpx.WriteOK(response, quoteDataFromModel(quote), context)
				return
			}
		}
		if config.MarketProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
			return
		}

		quote, err := config.MarketProvider.Quote(request.Context(), symbol)
		if err != nil {
			writeProviderError(response, context, config.MarketProvider.Name(), err)
			return
		}
		if config.Store != nil {
			modelQuote := modelQuoteFromMarket(quote)
			if err := config.Store.SaveQuote(request.Context(), &modelQuote); err != nil {
				writeCacheError(response, context, "行情快照缓存写入失败", err)
				return
			}
		}

		httpx.WriteOK(response, quoteDataFromMarket(quote), context)
	}
}

// handleKline 处理 K 线请求，缓存命中时返回本地有序数据。
func handleKline(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		var payload klineRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		symbol, ok := parseSymbol(response, payload.Symbol, context)
		if !ok {
			return
		}
		period, ok := parsePeriod(response, payload.Period, context)
		if !ok {
			return
		}
		adjust, ok := parseAdjust(response, payload.Adjust, context)
		if !ok {
			return
		}
		if payload.Limit <= 0 || payload.Limit > 500 {
			httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_limit", context)
			return
		}

		shouldCache := shouldCacheKline(period)
		if shouldCache && config.Store != nil {
			klines, err := config.Store.ListKlines(request.Context(), symbol.String(), string(period), string(adjust), payload.Limit)
			if err != nil {
				writeCacheError(response, context, "K 线缓存读取失败", err)
				return
			}
			if len(klines) >= payload.Limit {
				httpx.WriteOK(response, klineDataFromModels(klines), context)
				return
			}
		}
		if config.MarketProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
			return
		}

		bars, err := config.MarketProvider.Kline(request.Context(), marketservice.KlineRequest{
			Symbol: symbol,
			Period: period,
			Adjust: adjust,
			Limit:  payload.Limit,
		})
		if err != nil {
			writeProviderError(response, context, config.MarketProvider.Name(), err)
			return
		}
		sortKlineBars(bars)
		if shouldCache && config.Store != nil {
			if err := config.Store.SaveKlines(request.Context(), modelKlinesFromMarket(bars)); err != nil {
				writeCacheError(response, context, "K 线缓存写入失败", err)
				return
			}
		}

		httpx.WriteOK(response, klineDataFromMarket(bars), context)
	}
}

// handleIndicators 基于 Go core 的 K 线缓存或 Provider 结果计算技术指标，前端只展示结果。
func handleIndicators(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}

		var payload indicatorsRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		symbol, ok := parseSymbol(response, payload.Symbol, context)
		if !ok {
			return
		}
		period, ok := parsePeriod(response, payload.Period, context)
		if !ok {
			return
		}
		adjust, ok := parseAdjust(response, payload.Adjust, context)
		if !ok {
			return
		}
		if payload.Limit <= 0 || payload.Limit > 500 {
			httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_limit", context)
			return
		}
		if len(payload.Indicators) == 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40006, "invalid_indicator", context)
			return
		}

		klines, ok := loadKlines(response, request, config, context, symbol, period, adjust, payload.Limit)
		if !ok {
			return
		}
		calculated, err := calculateIndicators(klines, payload.Indicators)
		if err != nil {
			var invalidIndicator *invalidIndicatorError
			if errors.As(err, &invalidIndicator) {
				httpx.WriteError(response, http.StatusBadRequest, 40006, "invalid_indicator", context)
				return
			}
			httpx.WriteError(response, http.StatusBadRequest, 40007, "indicator_calculation_failed", context)
			return
		}

		httpx.WriteOK(response, indicatorsData{
			Symbol:     symbol.String(),
			Period:     string(period),
			Adjust:     string(adjust),
			Indicators: calculated,
		}, context)
	}
}

// loadKlines 优先读取本地 K 线缓存，缓存不足时调用 Provider 并写回缓存。
func loadKlines(
	response http.ResponseWriter,
	request *http.Request,
	config Config,
	context httpx.RequestContext,
	symbol stockservice.Symbol,
	period marketservice.Period,
	adjust marketservice.Adjust,
	limit int,
) ([]model.Kline, bool) {
	shouldCache := shouldCacheKline(period)
	if shouldCache && config.Store != nil {
		klines, err := config.Store.ListKlines(request.Context(), symbol.String(), string(period), string(adjust), limit)
		if err != nil {
			writeCacheError(response, context, "K 线缓存读取失败", err)
			return nil, false
		}
		if len(klines) >= limit {
			return klines, true
		}
	}
	if config.MarketProvider == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
		return nil, false
	}

	bars, err := config.MarketProvider.Kline(request.Context(), marketservice.KlineRequest{
		Symbol: symbol,
		Period: period,
		Adjust: adjust,
		Limit:  limit,
	})
	if err != nil {
		writeProviderError(response, context, config.MarketProvider.Name(), err)
		return nil, false
	}
	sortKlineBars(bars)
	klines := modelKlinesFromMarket(bars)
	if shouldCache && config.Store != nil {
		if err := config.Store.SaveKlines(request.Context(), klines); err != nil {
			writeCacheError(response, context, "K 线缓存写入失败", err)
			return nil, false
		}
	}
	return klines, true
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

// parsePeriod 校验首版 K 线周期白名单。
func parsePeriod(response http.ResponseWriter, raw string, context httpx.RequestContext) (marketservice.Period, bool) {
	switch marketservice.Period(strings.TrimSpace(raw)) {
	case marketservice.PeriodMinute:
		return marketservice.PeriodMinute, true
	case marketservice.Period1Minute:
		return marketservice.Period1Minute, true
	case marketservice.Period5Minute:
		return marketservice.Period5Minute, true
	case marketservice.Period15Minute:
		return marketservice.Period15Minute, true
	case marketservice.Period30Minute:
		return marketservice.Period30Minute, true
	case marketservice.Period60Minute:
		return marketservice.Period60Minute, true
	case marketservice.PeriodDay:
		return marketservice.PeriodDay, true
	case marketservice.PeriodWeek:
		return marketservice.PeriodWeek, true
	case marketservice.PeriodMonth:
		return marketservice.PeriodMonth, true
	case marketservice.PeriodQuarter:
		return marketservice.PeriodQuarter, true
	case marketservice.PeriodYear:
		return marketservice.PeriodYear, true
	default:
		httpx.WriteError(response, http.StatusBadRequest, 40003, "invalid_period", context)
		return "", false
	}
}

// shouldCacheKline 判断 K 线周期是否可持久化复用；盘中数据必须实时读取，避免隔日复用旧曲线。
func shouldCacheKline(period marketservice.Period) bool {
	return !period.IsIntraday()
}

// parseAdjust 校验首版 K 线复权方式白名单。
func parseAdjust(response http.ResponseWriter, raw string, context httpx.RequestContext) (marketservice.Adjust, bool) {
	switch marketservice.Adjust(strings.TrimSpace(raw)) {
	case marketservice.AdjustNone:
		return marketservice.AdjustNone, true
	case marketservice.AdjustForward:
		return marketservice.AdjustForward, true
	case marketservice.AdjustBackward:
		return marketservice.AdjustBackward, true
	default:
		httpx.WriteError(response, http.StatusBadRequest, 40004, "invalid_adjust", context)
		return "", false
	}
}

// writeProviderError 统一处理行情 Provider 调用失败，日志脱敏后只向前端返回稳定错误码。
func writeProviderError(response http.ResponseWriter, context httpx.RequestContext, provider string, err error) {
	var ruleError *xerr.Error
	if errors.As(err, &ruleError) && ruleError.Code == xerr.MarketProviderUnconfigured {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50301, string(ruleError.Code), context)
		return
	}
	slog.Warn(
		"行情 Provider 调用失败",
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		logger.FieldProvider, provider,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusBadGateway, 50200, "market_provider_error", context)
}

// writeCacheError 统一处理行情缓存读写失败，避免 handler 中散落数据库错误处理。
func writeCacheError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50005, "market_cache_error", context)
}

// quoteDataFromMarket 转换 Provider quote 为 API 响应字段。
func quoteDataFromMarket(quote marketservice.Quote) quoteData {
	quote = marketservice.NormalizeQuote(quote)
	return quoteData{
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

// quoteDataFromModel 转换缓存 quote 为 API 响应字段。
func quoteDataFromModel(quote model.Quote) quoteData {
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
	return quoteData{
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

// modelQuoteFromMarket 转换 Provider quote 为可持久化缓存模型。
func modelQuoteFromMarket(quote marketservice.Quote) model.Quote {
	quote = marketservice.NormalizeQuote(quote)
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

// klineDataFromMarket 转换 Provider K 线为 API 响应字段。
func klineDataFromMarket(bars []marketservice.KlineBar) klineData {
	items := make([]klineItem, 0, len(bars))
	for _, bar := range bars {
		items = append(items, klineItem{
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
	return klineData{Items: items}
}

// klineDataFromModels 转换缓存 K 线为 API 响应字段。
func klineDataFromModels(klines []model.Kline) klineData {
	items := make([]klineItem, 0, len(klines))
	for _, item := range klines {
		items = append(items, klineItem{
			Symbol:    item.Symbol,
			Period:    item.Period,
			Adjust:    item.Adjust,
			TradeDate: item.TradeDate,
			Open:      item.Open,
			High:      item.High,
			Low:       item.Low,
			Close:     item.Close,
			Volume:    item.Volume,
			Amount:    item.Amount,
			Provider:  item.Provider,
		})
	}
	return klineData{Items: items}
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

// calculateIndicators 根据请求白名单计算指标，不把指标结果单独落库。
func calculateIndicators(klines []model.Kline, names []string) (map[string]any, error) {
	closes := closeValues(klines)
	volumes := volumeValues(klines)
	result := make(map[string]any, len(names))

	for _, rawName := range names {
		name := strings.TrimSpace(strings.ToLower(rawName))
		switch name {
		case "ma":
			values, err := indicatorservice.MA(closes, 5)
			if err != nil {
				return nil, err
			}
			result["ma"] = map[string]any{"ma5": jsonSeries(values)}
		case "ema":
			values, err := indicatorservice.EMA(closes, 12)
			if err != nil {
				return nil, err
			}
			result["ema"] = map[string]any{"ema12": jsonSeries(values)}
		case "macd":
			values, err := indicatorservice.MACD(closes, 12, 26, 9)
			if err != nil {
				return nil, err
			}
			result["macd"] = map[string]any{
				"dif": jsonSeries(values.DIF),
				"dea": jsonSeries(values.DEA),
				"bar": jsonSeries(values.Bar),
			}
		case "rsi":
			values, err := indicatorservice.RSI(closes, 6)
			if err != nil {
				return nil, err
			}
			result["rsi"] = map[string]any{"rsi6": jsonSeries(values)}
		case "kdj":
			values, err := indicatorservice.KDJ(indicatorKLines(klines), 9)
			if err != nil {
				return nil, err
			}
			result["kdj"] = map[string]any{
				"k": jsonSeries(values.K),
				"d": jsonSeries(values.D),
				"j": jsonSeries(values.J),
			}
		case "boll":
			values, err := indicatorservice.BOLL(closes, 20, 2)
			if err != nil {
				return nil, err
			}
			result["boll"] = map[string]any{
				"middle": jsonSeries(values.Middle),
				"upper":  jsonSeries(values.Upper),
				"lower":  jsonSeries(values.Lower),
			}
		case "volume_ma":
			values, err := indicatorservice.VolumeMA(volumes, 5)
			if err != nil {
				return nil, err
			}
			result["volume_ma"] = map[string]any{"ma5": jsonSeries(values)}
		case "change_percent":
			values, err := changePercentSeries(closes)
			if err != nil {
				return nil, err
			}
			result["change_percent"] = jsonSeries(values)
		case "max_drawdown":
			value, err := indicatorservice.MaxDrawdown(closes)
			if err != nil {
				return nil, err
			}
			result["max_drawdown"] = value
		case "volatility":
			value, err := indicatorservice.Volatility(closes)
			if err != nil {
				return nil, err
			}
			result["volatility"] = value
		default:
			return nil, &invalidIndicatorError{}
		}
	}
	return result, nil
}

type invalidIndicatorError struct{}

// Error 返回稳定错误文本，具体 HTTP message 由 handler 统一控制。
func (*invalidIndicatorError) Error() string {
	return "invalid indicator"
}

// closeValues 提取收盘价序列，作为大部分技术指标的统一输入。
func closeValues(klines []model.Kline) []float64 {
	values := make([]float64, 0, len(klines))
	for _, item := range klines {
		values = append(values, item.Close)
	}
	return values
}

// volumeValues 提取成交量序列，用于成交量均线计算。
func volumeValues(klines []model.Kline) []float64 {
	values := make([]float64, 0, len(klines))
	for _, item := range klines {
		values = append(values, item.Volume)
	}
	return values
}

// indicatorKLines 转换持久化 K 线为指标计算所需最小模型。
func indicatorKLines(klines []model.Kline) []indicatorservice.KLine {
	values := make([]indicatorservice.KLine, 0, len(klines))
	for _, item := range klines {
		values = append(values, indicatorservice.KLine{
			High:  item.High,
			Low:   item.Low,
			Close: item.Close,
		})
	}
	return values
}

// changePercentSeries 生成逐日涨跌幅序列，首个点没有前收盘价，用 null 表示。
func changePercentSeries(closes []float64) ([]float64, error) {
	if len(closes) < 2 {
		return nil, &xerr.Error{Code: xerr.IndicatorInsufficientData}
	}
	values := make([]float64, len(closes))
	values[0] = math.NaN()
	for index := 1; index < len(closes); index++ {
		value, err := indicatorservice.ChangePercent(closes[index-1], closes[index])
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

// jsonSeries 将指标中的 NaN/Inf 转为 JSON null，避免 json encoder 因非法浮点数失败。
func jsonSeries(values []float64) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			result = append(result, nil)
			continue
		}
		result = append(result, value)
	}
	return result
}

// sortKlineBars 保证无论 Provider 原始顺序如何，API 都按交易日升序返回。
func sortKlineBars(bars []marketservice.KlineBar) {
	sort.SliceStable(bars, func(left int, right int) bool {
		return bars[left].TradeDate < bars[right].TradeDate
	})
}
