package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	sinaTencentProviderName = "sina-tencent-market"
	defaultSuggestURL       = "https://suggest3.sinajs.cn/suggest/"
	defaultQuoteURL         = "https://hq.sinajs.cn/"
	defaultKlineURL         = "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get"
	defaultProviderTimeout  = 8 * time.Second
)

var (
	// sinaQuoteLocation 固定按中国市场本地时间解释新浪行情时间，避免受运行机器时区影响。
	sinaQuoteLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// SinaConfig 描述新浪搜索和实时行情接口的可替换运行参数。
type SinaConfig struct {
	SuggestURL string
	QuoteURL   string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// TencentConfig 描述腾讯 K 线接口的可替换运行参数。
type TencentConfig struct {
	KlineURL   string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// SinaTencentConfig 描述组合 Provider 的可替换运行参数。
type SinaTencentConfig struct {
	Sina       SinaConfig
	Tencent    TencentConfig
	EastMoney  EastMoneyConfig
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time

	// SuggestURL、QuoteURL 和 KlineURL 保留为便捷配置入口，避免测试和调用方为默认组合构造重复拆配置。
	SuggestURL        string
	QuoteURL          string
	KlineURL          string
	EastMoneyKlineURL string
}

// SinaSource 是组合 Provider 依赖的新浪侧能力边界，只包含搜索和实时行情。
type SinaSource interface {
	Search(ctx context.Context, keyword string) ([]StockBasic, error)
	Quote(ctx context.Context, symbol stock.Symbol) (Quote, error)
}

// TencentSource 是组合 Provider 依赖的腾讯侧能力边界，只包含 K 线。
type TencentSource interface {
	Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error)
}

// EastMoneySource 是组合 Provider 依赖的东财侧能力边界，只包含 K 线兜底。
type EastMoneySource interface {
	Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error)
}

// SinaProvider 使用新浪结构化接口实现股票搜索和 CN A 股实时行情。
type SinaProvider struct {
	suggestURL string
	quoteURL   string
	client     *crawler.Client
	now        func() time.Time
}

// TencentProvider 使用腾讯结构化接口实现 CN A 股 K 线。
type TencentProvider struct {
	klineURL string
	client   *crawler.Client
}

// CompositeMarketProvider 组合新浪搜索/实时行情、腾讯 K 线和东财 K 线兜底，对外提供完整 MarketProvider。
type CompositeMarketProvider struct {
	name      string
	sina      SinaSource
	tencent   TencentSource
	eastMoney EastMoneySource
	now       func() time.Time
}

// NewSinaProvider 创建新浪搜索和实时行情数据源。
func NewSinaProvider(config SinaConfig) (*SinaProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultProviderTimeout
	}

	client, err := newMarketCrawler("sina-market-source", timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}

	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &SinaProvider{
		suggestURL: firstNonEmpty(config.SuggestURL, defaultSuggestURL),
		quoteURL:   firstNonEmpty(config.QuoteURL, defaultQuoteURL),
		client:     client,
		now:        now,
	}, nil
}

// NewTencentProvider 创建腾讯 K 线数据源。
func NewTencentProvider(config TencentConfig) (*TencentProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultProviderTimeout
	}

	client, err := newMarketCrawler("tencent-market-source", timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &TencentProvider{
		klineURL: firstNonEmpty(config.KlineURL, defaultKlineURL),
		client:   client,
	}, nil
}

// NewCompositeMarketProvider 组合拆分后的新浪和腾讯数据源，形成完整行情 Provider。
func NewCompositeMarketProvider(name string, sina SinaSource, tencent TencentSource, now func() time.Time) *CompositeMarketProvider {
	return NewCompositeMarketProviderWithKlineFallback(name, sina, tencent, nil, now)
}

// NewCompositeMarketProviderWithKlineFallback 组合新浪、腾讯和东财数据源，东财只作为 K 线兜底源。
func NewCompositeMarketProviderWithKlineFallback(name string, sina SinaSource, tencent TencentSource, eastMoney EastMoneySource, now func() time.Time) *CompositeMarketProvider {
	if now == nil {
		now = time.Now
	}
	return &CompositeMarketProvider{
		name:      firstNonEmpty(name, sinaTencentProviderName),
		sina:      sina,
		tencent:   tencent,
		eastMoney: eastMoney,
		now:       now,
	}
}

// NewSinaTencentProvider 创建默认完整行情 Provider，由新浪、腾讯和东财数据源组合而成。
func NewSinaTencentProvider(config SinaTencentConfig) (MarketProvider, error) {
	sinaConfig := config.Sina
	sinaConfig.SuggestURL = firstNonEmpty(sinaConfig.SuggestURL, config.SuggestURL)
	sinaConfig.QuoteURL = firstNonEmpty(sinaConfig.QuoteURL, config.QuoteURL)
	sinaConfig.HTTPClient = firstHTTPClient(sinaConfig.HTTPClient, config.HTTPClient)
	sinaConfig.Timeout = firstDuration(sinaConfig.Timeout, config.Timeout)
	sinaConfig.Now = firstNow(sinaConfig.Now, config.Now)

	tencentConfig := config.Tencent
	tencentConfig.KlineURL = firstNonEmpty(tencentConfig.KlineURL, config.KlineURL)
	tencentConfig.HTTPClient = firstHTTPClient(tencentConfig.HTTPClient, config.HTTPClient)
	tencentConfig.Timeout = firstDuration(tencentConfig.Timeout, config.Timeout)

	eastMoneyConfig := config.EastMoney
	eastMoneyConfig.KlineURL = firstNonEmpty(eastMoneyConfig.KlineURL, config.EastMoneyKlineURL)
	eastMoneyConfig.HTTPClient = firstHTTPClient(eastMoneyConfig.HTTPClient, config.HTTPClient)
	eastMoneyConfig.Timeout = firstDuration(eastMoneyConfig.Timeout, config.Timeout)

	sina, err := NewSinaProvider(sinaConfig)
	if err != nil {
		return nil, err
	}
	tencent, err := NewTencentProvider(tencentConfig)
	if err != nil {
		return nil, err
	}
	eastMoney, err := NewEastMoneyProvider(eastMoneyConfig)
	if err != nil {
		return nil, err
	}
	return NewCompositeMarketProviderWithKlineFallback(sinaTencentProviderName, sina, tencent, eastMoney, firstNow(config.Now, sinaConfig.Now)), nil
}

// newMarketCrawler 创建拆分数据源共享的受控 HTTP client。
func newMarketCrawler(name string, timeout time.Duration, httpClient *http.Client) (*crawler.Client, error) {
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", name)
	}
	return crawler.NewClient(crawler.Config{
		Name:         name,
		Timeout:      timeout,
		HTTPClient:   httpClient,
		MaxBodyBytes: 2 * 1024 * 1024,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		},
	})
}

// Search 调用新浪 suggest 接口并转换为统一股票基础信息。
func (provider *SinaProvider) Search(ctx context.Context, keyword string) ([]StockBasic, error) {
	trimmed := strings.TrimSpace(keyword)
	if trimmed == "" {
		return nil, fmt.Errorf("sina provider empty search keyword")
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.suggestURL,
		Query: map[string]string{
			"key":  trimmed,
			"name": "suggestdata",
			"type": "11,12,13",
		},
		Decoder: gb18030ToUTF8,
		Headers: map[string]string{
			"Referer": "https://finance.sina.com.cn/",
		},
	})
	if err != nil {
		return nil, NewProviderError("sina-market-source", "search", err)
	}
	stocks, err := parseSinaSuggest(result.Body)
	if err != nil {
		return nil, NewProviderError("sina-market-source", "search_parse", err)
	}
	return stocks, nil
}

// Quote 调用新浪 A 股实时行情接口并转换为统一行情快照。
func (provider *SinaProvider) Quote(ctx context.Context, symbol stock.Symbol) (Quote, error) {
	sinaCode, err := sinaCodeFromSymbol(symbol)
	if err != nil {
		return Quote{}, NewProviderError("sina-market-source", "quote_symbol", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.quoteURL,
		Query: map[string]string{
			"list": sinaCode,
			"rn":   strconv.FormatInt(provider.now().Unix(), 10),
		},
		Decoder: gb18030ToUTF8,
		Headers: map[string]string{
			"Referer": "https://finance.sina.com.cn/",
		},
	})
	if err != nil {
		return Quote{}, NewProviderError("sina-market-source", "quote", err)
	}
	quote, err := parseSinaCNQuote(result.Body, symbol)
	if err != nil {
		return Quote{}, NewProviderError("sina-market-source", "quote_parse", err)
	}
	quote.Provider = sinaTencentProviderName
	return quote, nil
}

// Kline 调用腾讯复权 K 线接口并转换为统一 K 线数组。
func (provider *TencentProvider) Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error) {
	sinaCode, err := sinaCodeFromSymbol(request.Symbol)
	if err != nil {
		return nil, NewProviderError("tencent-market-source", "kline_symbol", err)
	}
	adjust, err := tencentAdjustParam(request.Adjust)
	if err != nil {
		return nil, NewProviderError("tencent-market-source", "kline_adjust", err)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 200
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.klineURL,
		Query: map[string]string{
			"param": fmt.Sprintf("%s,%s,,,%d,%s", sinaCode, request.Period, limit, adjust),
		},
		Headers: map[string]string{
			"Referer": "https://gu.qq.com/",
		},
	})
	if err != nil {
		return nil, NewProviderError("tencent-market-source", "kline", err)
	}
	bars, err := parseTencentKline(result.BodyBytes, sinaCode, request, sinaTencentProviderName)
	if err != nil {
		return nil, NewProviderError("tencent-market-source", "kline_parse", err)
	}
	sort.SliceStable(bars, func(left int, right int) bool {
		return bars[left].TradeDate < bars[right].TradeDate
	})
	return bars, nil
}

// Name 返回组合 Provider 稳定名称，用于日志、缓存和状态展示。
func (provider *CompositeMarketProvider) Name() string {
	return provider.name
}

// Status 返回组合数据源来源、授权边界、限频和当前支持市场，不做阻塞式远程探测。
func (provider *CompositeMarketProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:           provider.Name(),
		Source:         "Sina public quote endpoint, Tencent public kline endpoint, and EastMoney public kline fallback endpoint",
		License:        "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:      "local client best effort, no burst retry",
		Available:      provider.sina != nil && provider.tencent != nil,
		LastCheckedAt:  provider.now().UTC(),
		SupportedAreas: []MarketArea{MarketCN},
	}
}

// Search 将搜索请求委派给新浪数据源。
func (provider *CompositeMarketProvider) Search(ctx context.Context, keyword string) ([]StockBasic, error) {
	if provider.sina == nil {
		return nil, NewProviderError(provider.Name(), "search", fmt.Errorf("sina source is required"))
	}
	return provider.sina.Search(ctx, keyword)
}

// Quote 将实时行情请求委派给新浪数据源。
func (provider *CompositeMarketProvider) Quote(ctx context.Context, symbol stock.Symbol) (Quote, error) {
	if provider.sina == nil {
		return Quote{}, NewProviderError(provider.Name(), "quote", fmt.Errorf("sina source is required"))
	}
	quote, err := provider.sina.Quote(ctx, symbol)
	if err != nil {
		return Quote{}, err
	}
	quote.Provider = provider.Name()
	return quote, nil
}

// Kline 优先使用腾讯 K 线；腾讯失败或无数据时使用东财 K 线兜底。
func (provider *CompositeMarketProvider) Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error) {
	if provider.tencent == nil && provider.eastMoney == nil {
		return nil, NewProviderError(provider.Name(), "kline", fmt.Errorf("tencent source is required"))
	}
	var primaryErr error
	var bars []KlineBar
	if provider.tencent != nil {
		var err error
		bars, err = provider.tencent.Kline(ctx, request)
		if err == nil && len(bars) > 0 {
			markKlineProvider(bars, provider.Name())
			return bars, nil
		}
		primaryErr = err
		if provider.eastMoney == nil {
			if err != nil {
				return nil, err
			}
			return nil, NewProviderError(provider.Name(), "kline_empty", fmt.Errorf("empty primary kline data"))
		}
	}

	fallbackBars, fallbackErr := provider.eastMoney.Kline(ctx, request)
	if fallbackErr != nil {
		if primaryErr != nil {
			return nil, fmt.Errorf("primary kline failed: %v; fallback kline failed: %w", primaryErr, fallbackErr)
		}
		return nil, fallbackErr
	}
	if len(fallbackBars) == 0 {
		return nil, NewProviderError(provider.Name(), "kline_empty", fmt.Errorf("empty fallback kline data"))
	}
	markKlineProvider(fallbackBars, provider.Name())
	return fallbackBars, nil
}

// markKlineProvider 将内部数据源名称收口为组合 Provider 名称，避免上层依赖兜底细节。
func markKlineProvider(bars []KlineBar, providerName string) {
	for index := range bars {
		bars[index].Provider = providerName
	}
}

// parseSinaSuggest 解析新浪 suggest 脚本响应，只保留可标准化为 CN symbol 的条目。
func parseSinaSuggest(payload string) ([]StockBasic, error) {
	content, err := quotedScriptPayload(payload)
	if err != nil {
		return nil, err
	}
	items := strings.Split(content, ";")
	stocks := make([]StockBasic, 0, len(items))
	for _, item := range items {
		fields := strings.Split(strings.TrimSpace(item), ",")
		if len(fields) < 4 {
			continue
		}
		marketCode := ""
		for _, field := range fields {
			candidate := strings.ToLower(strings.TrimSpace(field))
			if len(candidate) == 8 && (strings.HasPrefix(candidate, "sh") || strings.HasPrefix(candidate, "sz")) {
				marketCode = candidate
				break
			}
		}
		if marketCode == "" {
			continue
		}
		symbol, err := symbolFromSinaCode(marketCode)
		if err != nil {
			continue
		}
		if !isCNStockSymbol(symbol) {
			continue
		}
		stocks = append(stocks, StockBasic{
			Symbol:   symbol,
			Name:     strings.TrimSpace(fields[0]),
			Code:     symbol.Code,
			Market:   symbol.Market,
			Exchange: symbol.Exchange,
		})
	}
	return stocks, nil
}

// parseSinaCNQuote 解析新浪 A 股逗号分隔行情响应。
func parseSinaCNQuote(payload string, symbol stock.Symbol) (Quote, error) {
	content, err := quotedScriptPayload(payload)
	if err != nil {
		return Quote{}, err
	}
	fields := strings.Split(content, ",")
	if len(fields) < 32 {
		return Quote{}, fmt.Errorf("sina quote field count %d is less than 32", len(fields))
	}
	open, err := parseFloatField(fields[1], "open")
	if err != nil {
		return Quote{}, err
	}
	preClose, err := parseFloatField(fields[2], "pre_close")
	if err != nil {
		return Quote{}, err
	}
	price, err := parseFloatField(fields[3], "price")
	if err != nil {
		return Quote{}, err
	}
	high, err := parseFloatField(fields[4], "high")
	if err != nil {
		return Quote{}, err
	}
	low, err := parseFloatField(fields[5], "low")
	if err != nil {
		return Quote{}, err
	}
	volume, err := parseFloatField(fields[8], "volume")
	if err != nil {
		return Quote{}, err
	}
	amount, err := parseFloatField(fields[9], "amount")
	if err != nil {
		return Quote{}, err
	}
	quoteTime, err := parseSinaQuoteTime(fields[30], fields[31])
	if err != nil {
		return Quote{}, err
	}
	changeAmount := price - preClose
	changePercent := 0.0
	if preClose != 0 {
		changePercent = changeAmount / preClose * 100
	}
	return Quote{
		Symbol:        symbol,
		Price:         price,
		ChangeAmount:  changeAmount,
		ChangePercent: changePercent,
		Open:          open,
		High:          high,
		Low:           low,
		PreClose:      preClose,
		Volume:        volume,
		Amount:        amount,
		QuoteTime:     quoteTime,
	}, nil
}

type tencentKlineResponse struct {
	Code int                              `json:"code"`
	Msg  string                           `json:"msg"`
	Data map[string]map[string][][]string `json:"data"`
}

// parseTencentKline 解析腾讯 K 线 JSON 响应，并拒绝缺字段或异常 code。
func parseTencentKline(payload []byte, providerCode string, request KlineRequest, providerName string) ([]KlineBar, error) {
	var response tencentKlineResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("tencent kline code %d: %s", response.Code, response.Msg)
	}
	stockData, ok := response.Data[providerCode]
	if !ok {
		return nil, fmt.Errorf("tencent kline missing stock %s", providerCode)
	}
	rows := stockData[tencentKlineKey(request.Period, request.Adjust)]
	if len(rows) == 0 && request.Adjust != AdjustNone {
		rows = stockData[string(request.Period)]
	}
	bars := make([]KlineBar, 0, len(rows))
	for index, row := range rows {
		if len(row) < 6 {
			return nil, fmt.Errorf("tencent kline row %d has %d fields", index, len(row))
		}
		open, err := parseFloatField(row[1], "open")
		if err != nil {
			return nil, err
		}
		closeValue, err := parseFloatField(row[2], "close")
		if err != nil {
			return nil, err
		}
		high, err := parseFloatField(row[3], "high")
		if err != nil {
			return nil, err
		}
		low, err := parseFloatField(row[4], "low")
		if err != nil {
			return nil, err
		}
		volume, err := parseFloatField(row[5], "volume")
		if err != nil {
			return nil, err
		}
		bars = append(bars, KlineBar{
			Symbol:    request.Symbol,
			Period:    request.Period,
			Adjust:    request.Adjust,
			TradeDate: strings.TrimSpace(row[0]),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeValue,
			Volume:    volume,
			Provider:  providerName,
		})
	}
	return bars, nil
}

// tencentKlineKey 返回腾讯响应中 period 和 adjust 对应的数据字段。
func tencentKlineKey(period Period, adjust Adjust) string {
	if adjust == AdjustForward {
		return "qfq" + string(period)
	}
	if adjust == AdjustBackward {
		return "hfq" + string(period)
	}
	return string(period)
}

// tencentAdjustParam 将内部复权枚举映射为腾讯接口参数，不复权使用空字符串。
func tencentAdjustParam(adjust Adjust) (string, error) {
	switch adjust {
	case "", AdjustNone:
		return "", nil
	case AdjustForward:
		return "qfq", nil
	case AdjustBackward:
		return "hfq", nil
	default:
		return "", fmt.Errorf("unsupported adjust %q", adjust)
	}
}

// sinaCodeFromSymbol 将标准 symbol 转换为新浪/腾讯接口代码，当前真实 Provider 只支持 CN A 股。
func sinaCodeFromSymbol(symbol stock.Symbol) (string, error) {
	if symbol.Market != string(MarketCN) {
		return "", fmt.Errorf("market %s is not supported by %s", symbol.Market, sinaTencentProviderName)
	}
	if symbol.Exchange != "SH" && symbol.Exchange != "SZ" {
		return "", fmt.Errorf("exchange %s is not supported by %s", symbol.Exchange, sinaTencentProviderName)
	}
	if !isCNStockSymbol(symbol) {
		return "", fmt.Errorf("code %s is not supported by %s", symbol.Code, sinaTencentProviderName)
	}
	return strings.ToLower(symbol.Exchange) + symbol.Code, nil
}

// isCNStockSymbol 判断标准 symbol 是否属于当前 Provider 明确支持的沪深 A 股代码段。
func isCNStockSymbol(symbol stock.Symbol) bool {
	if symbol.Market != string(MarketCN) {
		return false
	}
	return isCNAStockCode(symbol.Exchange, symbol.Code)
}

// isCNAStockCode 只放行首版沪深 A 股常见代码段，避免转债、基金、B 股混入股票模型。
func isCNAStockCode(exchange string, code string) bool {
	if len(code) != 6 {
		return false
	}
	switch exchange {
	case "SH":
		return strings.HasPrefix(code, "600") ||
			strings.HasPrefix(code, "601") ||
			strings.HasPrefix(code, "603") ||
			strings.HasPrefix(code, "605") ||
			strings.HasPrefix(code, "688")
	case "SZ":
		return strings.HasPrefix(code, "000") ||
			strings.HasPrefix(code, "001") ||
			strings.HasPrefix(code, "002") ||
			strings.HasPrefix(code, "003") ||
			strings.HasPrefix(code, "300") ||
			strings.HasPrefix(code, "301")
	default:
		return false
	}
}

// symbolFromSinaCode 将 sh/sz 行情代码转换为标准 symbol。
func symbolFromSinaCode(code string) (stock.Symbol, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if len(normalized) != 8 {
		return stock.Symbol{}, fmt.Errorf("invalid sina code %q", code)
	}
	return stock.ParseSymbol("CN:" + normalized[:2] + ":" + normalized[2:])
}

// quotedScriptPayload 提取 var name=\"...\" 这类脚本响应中的字符串内容。
func quotedScriptPayload(payload string) (string, error) {
	trimmed := strings.TrimSpace(payload)
	start := strings.Index(trimmed, "\"")
	end := strings.LastIndex(trimmed, "\"")
	if start < 0 || end <= start {
		return "", fmt.Errorf("script payload missing quoted content")
	}
	return trimmed[start+1 : end], nil
}

// parseSinaQuoteTime 解析新浪行情日期和时间字段。
func parseSinaQuoteTime(dateValue string, timeValue string) (time.Time, error) {
	parsed, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		strings.TrimSpace(dateValue)+" "+strings.TrimSpace(timeValue),
		sinaQuoteLocation,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse quote time: %w", err)
	}
	return parsed, nil
}

// parseFloatField 解析行情数值字段，并在错误里保留字段名。
func parseFloatField(value string, fieldName string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return parsed, nil
}

// gb18030ToUTF8 将新浪 GB18030 响应体转换为 UTF-8 文本。
func gb18030ToUTF8(body []byte) (string, error) {
	reader := transform.NewReader(bytes.NewReader(body), simplifiedchinese.GB18030.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// firstNonEmpty 返回第一个非空字符串，用于合并默认配置。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// firstHTTPClient 返回第一个非空 HTTP client。
func firstHTTPClient(values ...*http.Client) *http.Client {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

// firstDuration 返回第一个非零 duration。
func firstDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

// firstNow 返回第一个非空时钟函数。
func firstNow(values ...func() time.Time) func() time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
