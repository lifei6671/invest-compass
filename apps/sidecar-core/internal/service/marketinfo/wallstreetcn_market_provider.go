package marketinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	wallstreetcnMarketProviderName = "wallstreetcn-market"
	defaultWallstreetcnRealURL     = "https://api-ddc-wscn.awtmt.com/market/real"
	defaultWallstreetcnKlineURL    = "https://api-ddc-wscn.awtmt.com/market/kline"
	defaultWallstreetcnMarketRef   = "https://wallstreetcn.com/"
)

var wallstreetcnDefaultProdCodes = []string{"DXY.OTC", "EURUSD.OTC", "USDJPY.OTC", "XAUUSD.OTC", "USCL.OTC", "USDCNH.OTC"}
var wallstreetcnDefaultQuoteFields = []string{"prod_name", "last_px", "px_change", "px_change_rate", "price_precision", "securities_type"}
var wallstreetcnDefaultKlineFields = []string{"tick_at", "open_px", "close_px", "high_px", "low_px"}

// GlobalQuoteProvider 定义全球宏观品种报价 Provider 能力。
type GlobalQuoteProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchGlobalQuotes(ctx context.Context, request GlobalQuoteRequest) (GlobalQuoteSnapshot, error)
}

// GlobalKlineProvider 定义全球宏观品种 K 线 Provider 能力。
type GlobalKlineProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchKline(ctx context.Context, request GlobalKlineRequest) (GlobalKlineSnapshot, error)
}

// GlobalQuoteRequest 是全球宏观品种报价查询请求。
type GlobalQuoteRequest struct {
	ProdCodes []string
	Fields    []string
}

// GlobalQuoteSnapshot 表示全球宏观品种报价快照。
type GlobalQuoteSnapshot struct {
	Items     []GlobalQuote
	Provider  string
	Source    string
	FetchedAt time.Time
}

// GlobalQuote 表示单个全球宏观品种报价。
type GlobalQuote struct {
	Code           string
	Name           string
	LastPrice      float64
	ChangeAmount   float64
	ChangePercent  float64
	Precision      int
	SecuritiesType string
}

// GlobalKlineRequest 是全球宏观品种 K 线查询请求。
type GlobalKlineRequest struct {
	ProdCode      string
	PeriodSeconds int
	TickCount     int
	Fields        []string
}

// GlobalKlineSnapshot 表示全球宏观品种 K 线快照。
type GlobalKlineSnapshot struct {
	ProdCode      string
	PeriodSeconds int
	Items         []GlobalKlineItem
	Provider      string
	Source        string
	FetchedAt     time.Time
}

// GlobalKlineItem 表示一根全球宏观品种 K 线。
type GlobalKlineItem struct {
	At     time.Time
	Open   float64
	Close  float64
	High   float64
	Low    float64
	Volume float64
	Amount float64
}

// WallstreetcnMarketConfig 描述华尔街见闻市场 Provider 运行配置。
type WallstreetcnMarketConfig struct {
	RealURL    string
	KlineURL   string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// WallstreetcnMarketProvider 抓取华尔街见闻全球报价和宏观品种 K 线。
type WallstreetcnMarketProvider struct {
	realURL  string
	klineURL string
	client   *crawler.Client
	now      func() time.Time
}

// NewWallstreetcnMarketProvider 创建华尔街见闻市场 Provider。
func NewWallstreetcnMarketProvider(config WallstreetcnMarketConfig) (*WallstreetcnMarketProvider, error) {
	client, now, err := newWallstreetcnMarketClient(config)
	if err != nil {
		return nil, err
	}
	return &WallstreetcnMarketProvider{
		realURL:  marketInfoFirstNonEmpty(config.RealURL, defaultWallstreetcnRealURL),
		klineURL: marketInfoFirstNonEmpty(config.KlineURL, defaultWallstreetcnKlineURL),
		client:   client,
		now:      now,
	}, nil
}

// Name 返回华尔街见闻市场 Provider 稳定名称。
func (provider *WallstreetcnMarketProvider) Name() string {
	return wallstreetcnMarketProviderName
}

// Status 返回华尔街见闻市场数据源状态。
func (provider *WallstreetcnMarketProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Wallstreetcn global market endpoints",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// FetchGlobalQuotes 抓取华尔街见闻全球宏观品种实时行情。
func (provider *WallstreetcnMarketProvider) FetchGlobalQuotes(ctx context.Context, request GlobalQuoteRequest) (GlobalQuoteSnapshot, error) {
	prodCodes := cleanStringList(request.ProdCodes)
	if len(prodCodes) == 0 {
		prodCodes = append([]string(nil), wallstreetcnDefaultProdCodes...)
	}
	fields := cleanStringList(request.Fields)
	if len(fields) == 0 {
		fields = append([]string(nil), wallstreetcnDefaultQuoteFields...)
	}

	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.realURL,
		Query: map[string]string{
			"fields":    strings.Join(fields, ","),
			"prod_code": strings.Join(prodCodes, ","),
		},
		Headers: wallstreetcnMarketHeaders(),
	})
	if err != nil {
		return GlobalQuoteSnapshot{}, NewProviderError(provider.Name(), "quote_fetch", err)
	}
	response, err := decodeWallstreetcnMarketResponse[wallstreetcnRealResponse](result.BodyBytes)
	if err != nil {
		return GlobalQuoteSnapshot{}, NewProviderError(provider.Name(), "quote_decode", err)
	}
	if response.Code != 20000 {
		return GlobalQuoteSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("wallstreetcn real code %d: %s", response.Code, response.Message))
	}

	items, err := wallstreetcnQuotesFromSnapshot(response.Data.Fields, response.Data.Snapshot, prodCodes)
	if err != nil {
		return GlobalQuoteSnapshot{}, NewProviderError(provider.Name(), "quote_parse", err)
	}
	return GlobalQuoteSnapshot{
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

// FetchKline 抓取华尔街见闻全球宏观品种 K 线。
func (provider *WallstreetcnMarketProvider) FetchKline(ctx context.Context, request GlobalKlineRequest) (GlobalKlineSnapshot, error) {
	prodCode := strings.TrimSpace(request.ProdCode)
	if prodCode == "" {
		prodCode = "XAUUSD.OTC"
	}
	periodSeconds := request.PeriodSeconds
	if periodSeconds <= 0 {
		periodSeconds = 300
	}
	tickCount := request.TickCount
	if tickCount <= 0 {
		tickCount = 256
	}
	if tickCount > 512 {
		tickCount = 512
	}
	fields := cleanStringList(request.Fields)
	if len(fields) == 0 {
		fields = append([]string(nil), wallstreetcnDefaultKlineFields...)
	}

	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.klineURL,
		Query: map[string]string{
			"fields":      strings.Join(fields, ","),
			"period_type": strconv.Itoa(periodSeconds),
			"prod_code":   prodCode,
			"tick_count":  strconv.Itoa(tickCount),
		},
		Headers: wallstreetcnMarketHeaders(),
	})
	if err != nil {
		return GlobalKlineSnapshot{}, NewProviderError(provider.Name(), "kline_fetch", err)
	}
	response, err := decodeWallstreetcnMarketResponse[wallstreetcnKlineResponse](result.BodyBytes)
	if err != nil {
		return GlobalKlineSnapshot{}, NewProviderError(provider.Name(), "kline_decode", err)
	}
	if response.Code != 20000 {
		return GlobalKlineSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("wallstreetcn kline code %d: %s", response.Code, response.Message))
	}

	items, err := wallstreetcnKlineItems(response.Data.Fields, response.Data.Candle, prodCode)
	if err != nil {
		return GlobalKlineSnapshot{}, NewProviderError(provider.Name(), "kline_parse", err)
	}
	return GlobalKlineSnapshot{
		ProdCode:      prodCode,
		PeriodSeconds: periodSeconds,
		Items:         items,
		Provider:      provider.Name(),
		Source:        provider.Status(ctx).Source,
		FetchedAt:     result.FetchedAt,
	}, nil
}

type wallstreetcnRealResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Fields   []string         `json:"fields"`
		Snapshot map[string][]any `json:"snapshot"`
	} `json:"data"`
}

type wallstreetcnKlineResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Fields []string                      `json:"fields"`
		Candle map[string]wallstreetcnCandle `json:"candle"`
	} `json:"data"`
}

type wallstreetcnCandle struct {
	Lines [][]any `json:"lines"`
}

// newWallstreetcnMarketClient 创建华尔街见闻市场抓取 Client。
func newWallstreetcnMarketClient(config WallstreetcnMarketConfig) (*crawler.Client, func() time.Time, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultMarketInfoTimeout
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         wallstreetcnMarketProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 1024 * 1024,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
	if err != nil {
		return nil, nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return client, now, nil
}

// decodeWallstreetcnMarketResponse 使用 json.Number 解码，避免精度在 any 字段中被提前固定。
func decodeWallstreetcnMarketResponse[T any](payload []byte) (T, error) {
	var response T
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	err := decoder.Decode(&response)
	return response, err
}

// wallstreetcnQuotesFromSnapshot 将华尔街见闻 snapshot 清洗成报价列表。
func wallstreetcnQuotesFromSnapshot(fields []string, snapshot map[string][]any, prodCodes []string) ([]GlobalQuote, error) {
	fieldIndex := fieldIndexMap(fields, wallstreetcnDefaultQuoteFields)
	items := make([]GlobalQuote, 0, len(snapshot))
	for _, code := range prodCodes {
		values, ok := snapshot[code]
		if !ok {
			continue
		}
		item, err := wallstreetcnQuoteFromValues(code, values, fieldIndex)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if len(items) > 0 {
		return items, nil
	}
	for code, values := range snapshot {
		item, err := wallstreetcnQuoteFromValues(code, values, fieldIndex)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].Code < items[right].Code
	})
	return items, nil
}

// wallstreetcnQuoteFromValues 按字段表解析一条全球报价，远端坏数值必须失败。
func wallstreetcnQuoteFromValues(code string, values []any, fieldIndex map[string]int) (GlobalQuote, error) {
	name := stringAt(values, fieldIndex, "prod_name")
	if name == "" {
		name = code
	}
	lastPrice, err := floatAt(values, fieldIndex, "last_px")
	if err != nil {
		return GlobalQuote{}, fmt.Errorf("%s last_px: %w", code, err)
	}
	changeAmount, err := floatAt(values, fieldIndex, "px_change")
	if err != nil {
		return GlobalQuote{}, fmt.Errorf("%s px_change: %w", code, err)
	}
	changePercent, err := floatAt(values, fieldIndex, "px_change_rate")
	if err != nil {
		return GlobalQuote{}, fmt.Errorf("%s px_change_rate: %w", code, err)
	}
	precision, err := intAt(values, fieldIndex, "price_precision")
	if err != nil {
		return GlobalQuote{}, fmt.Errorf("%s price_precision: %w", code, err)
	}
	return GlobalQuote{
		Code:           code,
		Name:           name,
		LastPrice:      lastPrice,
		ChangeAmount:   changeAmount,
		ChangePercent:  changePercent,
		Precision:      precision,
		SecuritiesType: stringAt(values, fieldIndex, "securities_type"),
	}, nil
}

// wallstreetcnKlineItems 将 K 线二维数组按 fields 映射为结构化数据。
func wallstreetcnKlineItems(fields []string, candles map[string]wallstreetcnCandle, prodCode string) ([]GlobalKlineItem, error) {
	fieldIndex := fieldIndexMap(fields, wallstreetcnDefaultKlineFields)
	candle, ok := candles[prodCode]
	if !ok {
		for _, value := range candles {
			candle = value
			break
		}
	}
	items := make([]GlobalKlineItem, 0, len(candle.Lines))
	for _, line := range candle.Lines {
		item, ok, err := wallstreetcnKlineItemFromLine(line, fieldIndex)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty wallstreetcn kline data")
	}
	return items, nil
}

// wallstreetcnKlineItemFromLine 解析单根 K 线，缺失时间跳过，坏数值直接失败。
func wallstreetcnKlineItemFromLine(line []any, fieldIndex map[string]int) (GlobalKlineItem, bool, error) {
	tickAtValue, err := requiredFloatAt(line, fieldIndex, "tick_at")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("tick_at: %w", err)
	}
	tickAt := int64(tickAtValue)
	if tickAt <= 0 {
		return GlobalKlineItem{}, false, nil
	}
	openPrice, err := requiredFloatAt(line, fieldIndex, "open_px")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("open_px: %w", err)
	}
	closePrice, err := requiredFloatAt(line, fieldIndex, "close_px")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("close_px: %w", err)
	}
	highPrice, err := requiredFloatAt(line, fieldIndex, "high_px")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("high_px: %w", err)
	}
	lowPrice, err := requiredFloatAt(line, fieldIndex, "low_px")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("low_px: %w", err)
	}
	volume, err := floatAt(line, fieldIndex, "business_amount")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("business_amount: %w", err)
	}
	amount, err := floatAt(line, fieldIndex, "business_balance")
	if err != nil {
		return GlobalKlineItem{}, false, fmt.Errorf("business_balance: %w", err)
	}
	return GlobalKlineItem{
		At:     time.Unix(tickAt, 0).UTC(),
		Open:   openPrice,
		Close:  closePrice,
		High:   highPrice,
		Low:    lowPrice,
		Volume: volume,
		Amount: amount,
	}, true, nil
}

// wallstreetcnMarketHeaders 返回华尔街见闻市场接口所需请求头。
func wallstreetcnMarketHeaders() map[string]string {
	return map[string]string{
		"Accept":        "application/json,text/plain,*/*",
		"Referer":       defaultWallstreetcnMarketRef,
		"x-client-type": "pc",
		"x-ivanka-app":  "wscn|web|0.40.40|0.0|0",
	}
}

// fieldIndexMap 构建响应字段到下标的映射，fields 缺失时使用默认字段顺序。
func fieldIndexMap(fields []string, defaults []string) map[string]int {
	if len(fields) == 0 {
		fields = defaults
	}
	index := make(map[string]int, len(fields))
	for i, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			index[trimmed] = i
		}
	}
	return index
}

// stringAt 按字段名读取字符串。
func stringAt(values []any, fieldIndex map[string]int, field string) string {
	index, ok := fieldIndex[field]
	if !ok || index < 0 || index >= len(values) {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(values[index]))
}

// floatAt 按字段名读取浮点数；缺失字段视为 0，存在但坏值必须失败。
func floatAt(values []any, fieldIndex map[string]int, field string) (float64, error) {
	index, ok := fieldIndex[field]
	if !ok || index < 0 || index >= len(values) {
		return 0, nil
	}
	return numberToFloat(values[index])
}

// requiredFloatAt 按字段名读取必要浮点数；字段缺失必须失败，避免远端结构变化被清洗成 0。
func requiredFloatAt(values []any, fieldIndex map[string]int, field string) (float64, error) {
	index, ok := fieldIndex[field]
	if !ok || index < 0 || index >= len(values) {
		return 0, fmt.Errorf("missing field")
	}
	return numberToFloat(values[index])
}

// intAt 按字段名读取整数；缺失字段视为 0，存在但坏值必须失败。
func intAt(values []any, fieldIndex map[string]int, field string) (int, error) {
	index, ok := fieldIndex[field]
	if !ok || index < 0 || index >= len(values) {
		return 0, nil
	}
	number, err := numberToFloat(values[index])
	if err != nil {
		return 0, err
	}
	return int(number), nil
}

// numberToFloat 将 JSON number/string/float 类型统一转换为 float64，坏字段必须显式失败。
func numberToFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return 0, err
		}
		return number, nil
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty number")
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, err
		}
		return number, nil
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return 0, fmt.Errorf("empty number")
		}
		number, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, err
		}
		return number, nil
	}
}

// cleanStringList 去除空字符串并保留调用方指定顺序。
func cleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}
