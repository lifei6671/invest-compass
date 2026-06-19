package marketinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	eastMoneyMutualTop10ProviderName   = "eastmoney-mutual-top10"
	eastMoneyStockScreenerProviderName = "eastmoney-stock-screener"
	defaultEastMoneyDatacenterURL      = "https://datacenter-web.eastmoney.com/web/api/data/v1/get"
	defaultEastMoneyStockScreenerURL   = "https://data.eastmoney.com/dataapi/xuangu/list"
)

// MutualTop10Provider 定义互联互通十大成交 Provider 能力。
type MutualTop10Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchMutualTop10(ctx context.Context, request MutualTop10Request) (MutualTop10Snapshot, error)
}

// MutualTop10Request 是沪深港通十大成交查询请求。
type MutualTop10Request struct {
	MutualType string
	TradeDate  string
	Page       int
	PageSize   int
}

// MutualTop10Snapshot 表示互联互通十大成交分页快照。
type MutualTop10Snapshot struct {
	MutualType string
	TradeDate  string
	Total      int
	Pages      int
	Items      []MutualTop10Item
	Provider   string
	Source     string
	FetchedAt  time.Time
}

// MutualTop10Item 表示单只沪深港通成交榜个股。
type MutualTop10Item struct {
	Symbol       string
	Code         string
	Name         string
	TradeDate    string
	MutualType   string
	Rank         int
	ClosePrice   float64
	ChangeRate   float64
	NetBuyAmount float64
	BuyAmount    float64
	SellAmount   float64
	DealAmount   float64
}

// EastMoneyMutualTop10Config 描述东财互联互通十大成交 Provider 运行配置。
type EastMoneyMutualTop10Config struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyMutualTop10Provider 使用东财数据中心抓取互联互通十大成交数据。
type EastMoneyMutualTop10Provider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewEastMoneyMutualTop10Provider 创建东财互联互通十大成交 Provider。
func NewEastMoneyMutualTop10Provider(config EastMoneyMutualTop10Config) (*EastMoneyMutualTop10Provider, error) {
	client, now, err := newEastMoneyMarketInfoClient(eastMoneyMutualTop10ProviderName, config.HTTPClient, config.Timeout, config.Now)
	if err != nil {
		return nil, err
	}
	return &EastMoneyMutualTop10Provider{
		url:    marketInfoFirstNonEmpty(config.URL, defaultEastMoneyDatacenterURL),
		client: client,
		now:    now,
	}, nil
}

// Name 返回东财互联互通十大成交 Provider 稳定名称。
func (provider *EastMoneyMutualTop10Provider) Name() string {
	return eastMoneyMutualTop10ProviderName
}

// Status 返回东财互联互通十大成交数据源状态。
func (provider *EastMoneyMutualTop10Provider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney datacenter mutual top10 endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// FetchMutualTop10 获取互联互通十大成交分页数据。
func (provider *EastMoneyMutualTop10Provider) FetchMutualTop10(ctx context.Context, request MutualTop10Request) (MutualTop10Snapshot, error) {
	mutualType := strings.TrimSpace(request.MutualType)
	if mutualType == "" {
		mutualType = "001"
	}
	if !isAllowedMutualType(mutualType) {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "request", fmt.Errorf("unsupported mutual type %q", mutualType))
	}
	tradeDate := strings.TrimSpace(request.TradeDate)
	if tradeDate == "" {
		tradeDate = provider.now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "request", fmt.Errorf("invalid trade date %q", tradeDate))
	}
	page := request.Page
	if page <= 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	filter := fmt.Sprintf("(MUTUAL_TYPE=\"%s\")(TRADE_DATE='%s')", mutualType, tradeDate)
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Query: map[string]string{
			"callback":    "data",
			"sortColumns": "RANK",
			"sortTypes":   "1",
			"pageSize":    strconv.Itoa(pageSize),
			"pageNumber":  strconv.Itoa(page),
			"reportName":  "RPT_MUTUAL_TOP10DEAL",
			"columns":     "ALL",
			"source":      "WEB",
			"client":      "WEB",
			"filter":      filter,
			"_":           strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://data.eastmoney.com/",
		},
	})
	if err != nil {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}
	response, err := decodeEastMoneyMutualTop10(result.BodyBytes)
	if err != nil {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if !response.Success {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney mutual top10 success=false"))
	}
	items := make([]MutualTop10Item, 0, len(response.Result.Data))
	for _, row := range response.Result.Data {
		symbol := strings.TrimSpace(row.SecuCode)
		code := strings.TrimSpace(row.Code)
		name := strings.TrimSpace(row.Name)
		if symbol == "" && code == "" {
			continue
		}
		items = append(items, MutualTop10Item{
			Symbol:       symbol,
			Code:         code,
			Name:         name,
			TradeDate:    dateOnly(row.TradeDate),
			MutualType:   row.MutualType,
			Rank:         row.Rank,
			ClosePrice:   row.ClosePrice,
			ChangeRate:   row.ChangeRate,
			NetBuyAmount: row.NetBuyAmount,
			BuyAmount:    row.BuyAmount,
			SellAmount:   row.SellAmount,
			DealAmount:   row.DealAmount,
		})
	}
	if response.Result.Count > 0 && len(items) == 0 {
		return MutualTop10Snapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty valid eastmoney mutual top10 rows"))
	}
	return MutualTop10Snapshot{
		MutualType: mutualType,
		TradeDate:  tradeDate,
		Total:      response.Result.Count,
		Pages:      response.Result.Pages,
		Items:      items,
		Provider:   provider.Name(),
		Source:     provider.Status(ctx).Source,
		FetchedAt:  result.FetchedAt,
	}, nil
}

// StockScreenerProvider 定义东财条件选股 Provider 能力。
type StockScreenerProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchStocks(ctx context.Context, request StockScreenerRequest) (StockScreenerSnapshot, error)
}

// ScreenerCondition 描述一个东财条件选股筛选条件。
type ScreenerCondition struct {
	Field    string
	Operator string
	Value    string
}

// StockScreenerRequest 是条件选股分页查询请求。
type StockScreenerRequest struct {
	Page       int
	PageSize   int
	Keyword    string
	Conditions []ScreenerCondition
}

// StockScreenerSnapshot 表示条件选股分页快照。
type StockScreenerSnapshot struct {
	Total     int
	Pages     int
	Items     []StockScreenerItem
	Provider  string
	Source    string
	FetchedAt time.Time
}

// StockScreenerItem 表示东财条件选股返回的一只股票。
type StockScreenerItem struct {
	Symbol       string
	Code         string
	Name         string
	Price        float64
	ChangeRate   float64
	VolumeRatio  float64
	HighPrice    float64
	LowPrice     float64
	PreClose     float64
	Volume       float64
	Amount       float64
	TurnoverRate float64
	Market       string
	Concept      string
	Industry     string
}

// EastMoneyStockScreenerConfig 描述东财条件选股 Provider 运行配置。
type EastMoneyStockScreenerConfig struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyStockScreenerProvider 使用东财条件选股接口抓取股票池快照。
type EastMoneyStockScreenerProvider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewEastMoneyStockScreenerProvider 创建东财条件选股 Provider。
func NewEastMoneyStockScreenerProvider(config EastMoneyStockScreenerConfig) (*EastMoneyStockScreenerProvider, error) {
	client, now, err := newEastMoneyMarketInfoClient(eastMoneyStockScreenerProviderName, config.HTTPClient, config.Timeout, config.Now)
	if err != nil {
		return nil, err
	}
	return &EastMoneyStockScreenerProvider{
		url:    marketInfoFirstNonEmpty(config.URL, defaultEastMoneyStockScreenerURL),
		client: client,
		now:    now,
	}, nil
}

// Name 返回东财条件选股 Provider 稳定名称。
func (provider *EastMoneyStockScreenerProvider) Name() string {
	return eastMoneyStockScreenerProviderName
}

// Status 返回东财条件选股数据源状态。
func (provider *EastMoneyStockScreenerProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney xuangu public stock screener endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// FetchStocks 获取东财条件选股分页快照。
func (provider *EastMoneyStockScreenerProvider) FetchStocks(ctx context.Context, request StockScreenerRequest) (StockScreenerSnapshot, error) {
	page := request.Page
	if page <= 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	filter, err := buildStockScreenerFilter(request)
	if err != nil {
		return StockScreenerSnapshot{}, NewProviderError(provider.Name(), "filter", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Query: map[string]string{
			"st":        "CHANGE_RATE",
			"sr":        "-1",
			"ps":        strconv.Itoa(pageSize),
			"p":         strconv.Itoa(page),
			"sty":       "SECUCODE,SECURITY_CODE,SECURITY_NAME_ABBR,NEW_PRICE,CHANGE_RATE,VOLUME_RATIO,HIGH_PRICE,LOW_PRICE,PRE_CLOSE_PRICE,VOLUME,DEAL_AMOUNT,TURNOVERRATE,MARKET,CONCEPT,INDUSTRY",
			"filter":    filter,
			"source":    "SELECT_SECURITIES",
			"client":    "WEB",
			"hyversion": "v2",
			"_":         strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://data.eastmoney.com/",
		},
	})
	if err != nil {
		return StockScreenerSnapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}
	response, err := decodeEastMoneyStockScreener(result.BodyBytes)
	if err != nil {
		return StockScreenerSnapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if !response.Success {
		return StockScreenerSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney stock screener success=false"))
	}
	items := make([]StockScreenerItem, 0, len(response.Result.Data))
	for _, row := range response.Result.Data {
		symbol := strings.TrimSpace(row.SecuCode)
		name := strings.TrimSpace(row.Name)
		if symbol == "" || name == "" {
			continue
		}
		items = append(items, StockScreenerItem{
			Symbol:       symbol,
			Code:         row.Code,
			Name:         name,
			Price:        row.Price,
			ChangeRate:   row.ChangeRate,
			VolumeRatio:  row.VolumeRatio,
			HighPrice:    row.HighPrice,
			LowPrice:     row.LowPrice,
			PreClose:     row.PreClose,
			Volume:       row.Volume,
			Amount:       row.Amount,
			TurnoverRate: row.TurnoverRate,
			Market:       row.Market,
			Concept:      row.Concept,
			Industry:     row.Industry,
		})
	}
	if len(items) == 0 && (response.Result.Count > 0 || len(response.Result.Data) > 0) {
		return StockScreenerSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney stock screener data"))
	}
	return StockScreenerSnapshot{
		Total:     response.Result.Count,
		Pages:     response.Result.Pages,
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

type eastMoneyMutualTop10Response struct {
	Success bool `json:"success"`
	Result  struct {
		Pages int `json:"pages"`
		Count int `json:"count"`
		Data  []struct {
			Code         string  `json:"SECURITY_CODE"`
			Name         string  `json:"SECURITY_NAME_ABBR"`
			SecuCode     string  `json:"SECUCODE"`
			TradeDate    string  `json:"TRADE_DATE"`
			MutualType   string  `json:"MUTUAL_TYPE"`
			Rank         int     `json:"RANK"`
			ChangeRate   float64 `json:"CHANGE_RATE"`
			ClosePrice   float64 `json:"CLOSE_PRICE"`
			NetBuyAmount float64 `json:"NET_BUY_AMT"`
			BuyAmount    float64 `json:"BUY_AMT"`
			SellAmount   float64 `json:"SELL_AMT"`
			DealAmount   float64 `json:"DEAL_AMT"`
		} `json:"data"`
	} `json:"result"`
}

type eastMoneyStockScreenerResponse struct {
	Success bool `json:"success"`
	Result  struct {
		Pages int `json:"pages"`
		Count int `json:"count"`
		Data  []struct {
			SecuCode     string  `json:"SECUCODE"`
			Code         string  `json:"SECURITY_CODE"`
			Name         string  `json:"SECURITY_NAME_ABBR"`
			Price        float64 `json:"NEW_PRICE"`
			ChangeRate   float64 `json:"CHANGE_RATE"`
			VolumeRatio  float64 `json:"VOLUME_RATIO"`
			HighPrice    float64 `json:"HIGH_PRICE"`
			LowPrice     float64 `json:"LOW_PRICE"`
			PreClose     float64 `json:"PRE_CLOSE_PRICE"`
			Volume       float64 `json:"VOLUME"`
			Amount       float64 `json:"DEAL_AMOUNT"`
			TurnoverRate float64 `json:"TURNOVERRATE"`
			Market       string  `json:"MARKET"`
			Concept      string  `json:"CONCEPT"`
			Industry     string  `json:"INDUSTRY"`
		} `json:"data"`
	} `json:"result"`
}

// newEastMoneyMarketInfoClient 创建东财市场信息类 Provider 共享 HTTP Client。
func newEastMoneyMarketInfoClient(name string, httpClient *http.Client, timeout time.Duration, now func() time.Time) (*crawler.Client, func() time.Time, error) {
	if timeout == 0 {
		timeout = defaultMarketInfoTimeout
	}
	if timeout < 0 {
		return nil, nil, fmt.Errorf("%s invalid timeout", name)
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         name,
		Timeout:      timeout,
		HTTPClient:   httpClient,
		MaxBodyBytes: 2 * 1024 * 1024,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
	if err != nil {
		return nil, nil, err
	}
	if now == nil {
		now = time.Now
	}
	return client, now, nil
}

// decodeEastMoneyMutualTop10 解析互联互通十大成交 JSON 或 JSONP。
func decodeEastMoneyMutualTop10(payload []byte) (eastMoneyMutualTop10Response, error) {
	var response eastMoneyMutualTop10Response
	if err := json.Unmarshal(stripMarketInfoJSONP(payload), &response); err != nil {
		return eastMoneyMutualTop10Response{}, err
	}
	return response, nil
}

// decodeEastMoneyStockScreener 解析东财条件选股 JSON。
func decodeEastMoneyStockScreener(payload []byte) (eastMoneyStockScreenerResponse, error) {
	var response eastMoneyStockScreenerResponse
	if err := json.Unmarshal(stripMarketInfoJSONP(payload), &response); err != nil {
		return eastMoneyStockScreenerResponse{}, err
	}
	return response, nil
}

// buildStockScreenerFilter 构造东财条件选股 filter 参数。
func buildStockScreenerFilter(request StockScreenerRequest) (string, error) {
	parts := []string{`(MARKET in ("上交所主板","深交所主板","深交所创业板","上交所科创板","上交所风险警示板","深交所风险警示板","北京证券交易所"))`}
	if keyword := strings.TrimSpace(request.Keyword); keyword != "" {
		if !isAllowedScreenerKeyword(keyword) {
			return "", fmt.Errorf("unsupported screener keyword %q", keyword)
		}
		parts = append(parts, fmt.Sprintf(`(SECURITY_NAME_ABBR in ("%s"))`, strings.ReplaceAll(keyword, `"`, `\"`)))
	}
	for _, condition := range request.Conditions {
		field := strings.TrimSpace(condition.Field)
		operator := strings.TrimSpace(condition.Operator)
		value := strings.TrimSpace(condition.Value)
		if field == "" || operator == "" || value == "" {
			return "", fmt.Errorf("invalid screener condition %+v", condition)
		}
		if !isAllowedScreenerField(field) {
			return "", fmt.Errorf("unsupported screener field %q", field)
		}
		if !isAllowedScreenerOperator(operator) {
			return "", fmt.Errorf("unsupported screener operator %q", operator)
		}
		if !isAllowedScreenerValue(value) {
			return "", fmt.Errorf("unsupported screener value %q", value)
		}
		parts = append(parts, fmt.Sprintf("(%s%s%s)", field, operator, value))
	}
	return strings.Join(parts, ""), nil
}

// isAllowedMutualType 限定互联互通类型枚举，避免调用方改写东财 filter DSL。
func isAllowedMutualType(mutualType string) bool {
	switch mutualType {
	case "001", "003", "005", "006":
		return true
	default:
		return false
	}
}

// isAllowedScreenerField 判断条件字段是否为东财字段名，避免拼出非预期 filter 表达式。
func isAllowedScreenerField(field string) bool {
	if field == "" {
		return false
	}
	for _, char := range field {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

// isAllowedScreenerValue 限制条件值字符集，避免拼出额外括号表达式。
func isAllowedScreenerValue(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		switch char {
		case '_', '.', '-', '%':
			continue
		default:
			return false
		}
	}
	return true
}

// isAllowedScreenerKeyword 限制关键词不能闭合东财 filter 字符串或拼接额外表达式。
func isAllowedScreenerKeyword(keyword string) bool {
	if keyword == "" {
		return false
	}
	for _, char := range keyword {
		switch char {
		case '(', ')', '"', '\'', '\\', ';', '\r', '\n':
			return false
		}
	}
	return true
}

// isAllowedScreenerOperator 判断条件选股操作符是否在允许范围内。
func isAllowedScreenerOperator(operator string) bool {
	switch operator {
	case "=", "!=", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}

// stripMarketInfoJSONP 去掉 data(...) 一类 JSONP 包装。
func stripMarketInfoJSONP(payload []byte) []byte {
	text := strings.TrimSpace(string(payload))
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return []byte(text)
	}
	if start := strings.Index(text, "("); start >= 0 {
		if end := strings.LastIndex(text, ")"); end > start {
			text = text[start+1 : end]
		}
	}
	return []byte(strings.TrimSpace(text))
}

// dateOnly 将东财日期时间字符串压缩为日期。
func dateOnly(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}
