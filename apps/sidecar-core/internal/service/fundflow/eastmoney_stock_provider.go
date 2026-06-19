package fundflow

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
	eastMoneyStockFundFlowProviderName  = "eastmoney-stock-fundflow"
	defaultEastMoneyStockFlowRankingURL = "https://push2.eastmoney.com/api/qt/clist/get"
	defaultEastMoneyStockFlowHistoryURL = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"
)

// StockFundFlowProvider 定义个股资金流 Provider 的抓取能力。
type StockFundFlowProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchRanking(ctx context.Context, request StockFundFlowRankingRequest) (StockFundFlowRankingSnapshot, error)
	FetchHistory(ctx context.Context, request StockFundFlowHistoryRequest) (StockFundFlowHistory, error)
}

// StockFundFlowRankingRequest 是个股资金流榜单请求。
type StockFundFlowRankingRequest struct {
	Page     int
	PageSize int
}

// StockFundFlowHistoryRequest 是单只股票历史资金流请求。
type StockFundFlowHistoryRequest struct {
	Symbol string
	Limit  int
}

// StockFundFlowRankingSnapshot 表示个股资金流榜单快照。
type StockFundFlowRankingSnapshot struct {
	Total     int
	Items     []StockFundFlowItem
	Provider  string
	Source    string
	FetchedAt time.Time
}

// StockFundFlowItem 表示个股资金流榜单中的一行。
type StockFundFlowItem struct {
	Symbol                   string
	Code                     string
	Name                     string
	MarketID                 int
	Price                    float64
	ChangePercent            float64
	MainNetInflow            float64
	MainNetInflowRatio       float64
	SuperLargeNetInflow      float64
	SuperLargeNetInflowRatio float64
	LargeNetInflow           float64
	LargeNetInflowRatio      float64
	MediumNetInflow          float64
	MediumNetInflowRatio     float64
	SmallNetInflow           float64
	SmallNetInflowRatio      float64
	Industry                 string
}

// StockFundFlowHistory 表示单只股票历史资金流序列。
type StockFundFlowHistory struct {
	Symbol    string
	Items     []StockFundFlowHistoryItem
	Provider  string
	Source    string
	FetchedAt time.Time
}

// StockFundFlowHistoryItem 表示单日个股资金流数据。
type StockFundFlowHistoryItem struct {
	Date                     string
	MainNetInflow            float64
	SmallNetInflow           float64
	MediumNetInflow          float64
	LargeNetInflow           float64
	SuperLargeNetInflow      float64
	MainNetInflowRatio       float64
	SmallNetInflowRatio      float64
	MediumNetInflowRatio     float64
	LargeNetInflowRatio      float64
	SuperLargeNetInflowRatio float64
	Close                    float64
	ChangePercent            float64
}

// EastMoneyStockFundFlowConfig 描述东财个股资金流 Provider 运行配置。
type EastMoneyStockFundFlowConfig struct {
	RankingURL string
	HistoryURL string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyStockFundFlowProvider 使用东方财富公开接口抓取个股资金流榜单和历史资金流。
type EastMoneyStockFundFlowProvider struct {
	rankingURL string
	historyURL string
	client     *crawler.Client
	now        func() time.Time
}

// NewEastMoneyStockFundFlowProvider 创建东财个股资金流 Provider。
func NewEastMoneyStockFundFlowProvider(config EastMoneyStockFundFlowConfig) (*EastMoneyStockFundFlowProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultEastMoneyFlowTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", eastMoneyStockFundFlowProviderName)
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         eastMoneyStockFundFlowProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 2 * 1024 * 1024,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &EastMoneyStockFundFlowProvider{
		rankingURL: firstNonEmpty(config.RankingURL, defaultEastMoneyStockFlowRankingURL),
		historyURL: firstNonEmpty(config.HistoryURL, defaultEastMoneyStockFlowHistoryURL),
		client:     client,
		now:        now,
	}, nil
}

// Name 返回东财个股资金流 Provider 稳定名称。
func (provider *EastMoneyStockFundFlowProvider) Name() string {
	return eastMoneyStockFundFlowProviderName
}

// Status 返回东财个股资金流来源、授权边界和限频说明。
func (provider *EastMoneyStockFundFlowProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney push2 public stock fund flow endpoints",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
		SupportedScopes: []Scope{
			ScopeStock,
		},
	}
}

// FetchRanking 获取东方财富个股资金流榜单快照。
func (provider *EastMoneyStockFundFlowProvider) FetchRanking(ctx context.Context, request StockFundFlowRankingRequest) (StockFundFlowRankingSnapshot, error) {
	page := request.Page
	if page <= 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.rankingURL,
		Query: map[string]string{
			"cb":     "data",
			"fid":    "f62",
			"po":     "1",
			"pz":     strconv.Itoa(pageSize),
			"pn":     strconv.Itoa(page),
			"np":     "1",
			"fltt":   "2",
			"invt":   "2",
			"ut":     "8dec03ba335b81bf4ebdf7b29ec27d15",
			"fs":     "m:0+t:6+f:!2,m:0+t:13+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2,m:0+t:7+f:!2,m:1+t:3+f:!2",
			"fields": "f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f204,f205,f124,f1,f13,f100,f265",
			"_":      strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://quote.eastmoney.com/",
		},
	})
	if err != nil {
		return StockFundFlowRankingSnapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}
	response, err := decodeEastMoneyStockFundFlowRanking(result.BodyBytes)
	if err != nil {
		return StockFundFlowRankingSnapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.RC != 0 {
		return StockFundFlowRankingSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney stock fundflow rc %d", response.RC))
	}
	if len(response.Data.Diff) == 0 {
		return StockFundFlowRankingSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney stock fundflow diff"))
	}
	items := make([]StockFundFlowItem, 0, len(response.Data.Diff))
	for _, row := range response.Data.Diff {
		code := strings.TrimSpace(row.Code)
		name := strings.TrimSpace(row.Name)
		if code == "" || name == "" {
			continue
		}
		items = append(items, StockFundFlowItem{
			Symbol:                   eastMoneySymbolFromCode(code, row.MarketID),
			Code:                     code,
			Name:                     name,
			MarketID:                 row.MarketID,
			Price:                    row.Price,
			ChangePercent:            row.ChangePercent,
			MainNetInflow:            row.MainNetInflow,
			MainNetInflowRatio:       row.MainNetInflowRatio,
			SuperLargeNetInflow:      row.SuperLargeNetInflow,
			SuperLargeNetInflowRatio: row.SuperLargeNetInflowRatio,
			LargeNetInflow:           row.LargeNetInflow,
			LargeNetInflowRatio:      row.LargeNetInflowRatio,
			MediumNetInflow:          row.MediumNetInflow,
			MediumNetInflowRatio:     row.MediumNetInflowRatio,
			SmallNetInflow:           row.SmallNetInflow,
			SmallNetInflowRatio:      row.SmallNetInflowRatio,
			Industry:                 strings.TrimSpace(row.Industry),
		})
	}
	if len(items) == 0 {
		return StockFundFlowRankingSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty valid eastmoney stock fundflow items"))
	}
	return StockFundFlowRankingSnapshot{
		Total:     response.Data.Total,
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

// FetchHistory 获取单只股票的历史资金流序列。
func (provider *EastMoneyStockFundFlowProvider) FetchHistory(ctx context.Context, request StockFundFlowHistoryRequest) (StockFundFlowHistory, error) {
	symbol := strings.TrimSpace(request.Symbol)
	secID := eastMoneySecID(symbol)
	if secID == "" {
		return StockFundFlowHistory{}, NewProviderError(provider.Name(), "symbol", fmt.Errorf("invalid stock symbol %q", request.Symbol))
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 120
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.historyURL,
		Query: map[string]string{
			"cb":      "data",
			"lmt":     strconv.Itoa(limit),
			"klt":     "101",
			"fields1": "f1,f2,f3,f7",
			"fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65",
			"ut":      "b2884a393a59ad64002292a3e90d46a5",
			"secid":   secID,
			"_":       strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://quote.eastmoney.com/",
		},
	})
	if err != nil {
		return StockFundFlowHistory{}, NewProviderError(provider.Name(), "fetch", err)
	}
	response, err := decodeEastMoneyStockFundFlowHistory(result.BodyBytes)
	if err != nil {
		return StockFundFlowHistory{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.RC != 0 {
		return StockFundFlowHistory{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney stock fundflow history rc %d", response.RC))
	}
	items := make([]StockFundFlowHistoryItem, 0, len(response.Data.Klines))
	for index, line := range response.Data.Klines {
		item, err := parseStockFundFlowHistoryLine(line)
		if err != nil {
			return StockFundFlowHistory{}, NewProviderError(provider.Name(), "decode", fmt.Errorf("history line %d: %w", index, err))
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return StockFundFlowHistory{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney stock fundflow history"))
	}
	return StockFundFlowHistory{
		Symbol:    normalizeStockSymbol(symbol),
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

type eastMoneyStockFundFlowRankingResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			Code                     string  `json:"f12"`
			MarketID                 int     `json:"f13"`
			Name                     string  `json:"f14"`
			Price                    float64 `json:"f2"`
			ChangePercent            float64 `json:"f3"`
			MainNetInflow            float64 `json:"f62"`
			MainNetInflowRatio       float64 `json:"f184"`
			SuperLargeNetInflow      float64 `json:"f66"`
			SuperLargeNetInflowRatio float64 `json:"f69"`
			LargeNetInflow           float64 `json:"f72"`
			LargeNetInflowRatio      float64 `json:"f75"`
			MediumNetInflow          float64 `json:"f78"`
			MediumNetInflowRatio     float64 `json:"f81"`
			SmallNetInflow           float64 `json:"f84"`
			SmallNetInflowRatio      float64 `json:"f87"`
			Industry                 string  `json:"f100"`
		} `json:"diff"`
	} `json:"data"`
}

type eastMoneyStockFundFlowHistoryResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Klines []string `json:"klines"`
	} `json:"data"`
}

// decodeEastMoneyStockFundFlowRanking 解析东财个股资金流榜单 JSON 或 JSONP。
func decodeEastMoneyStockFundFlowRanking(payload []byte) (eastMoneyStockFundFlowRankingResponse, error) {
	var response eastMoneyStockFundFlowRankingResponse
	if err := json.Unmarshal(stripJSONPCallback(payload), &response); err != nil {
		return eastMoneyStockFundFlowRankingResponse{}, err
	}
	return response, nil
}

// decodeEastMoneyStockFundFlowHistory 解析东财个股历史资金流 JSON 或 JSONP。
func decodeEastMoneyStockFundFlowHistory(payload []byte) (eastMoneyStockFundFlowHistoryResponse, error) {
	var response eastMoneyStockFundFlowHistoryResponse
	if err := json.Unmarshal(stripJSONPCallback(payload), &response); err != nil {
		return eastMoneyStockFundFlowHistoryResponse{}, err
	}
	return response, nil
}

// parseStockFundFlowHistoryLine 按东财 fields2 顺序解析单行历史资金流。
func parseStockFundFlowHistoryLine(line string) (StockFundFlowHistoryItem, error) {
	parts := strings.Split(line, ",")
	if len(parts) < 13 {
		return StockFundFlowHistoryItem{}, fmt.Errorf("expected 13 fields, got %d", len(parts))
	}
	values := make([]float64, 12)
	fieldNames := []string{
		"main net inflow",
		"small net inflow",
		"medium net inflow",
		"large net inflow",
		"super large net inflow",
		"main net inflow ratio",
		"small net inflow ratio",
		"medium net inflow ratio",
		"large net inflow ratio",
		"super large net inflow ratio",
		"close",
		"change percent",
	}
	for index, name := range fieldNames {
		value, err := parseRequiredFloat(parts[index+1], name)
		if err != nil {
			return StockFundFlowHistoryItem{}, err
		}
		values[index] = value
	}
	return StockFundFlowHistoryItem{
		Date:                     strings.TrimSpace(parts[0]),
		MainNetInflow:            values[0],
		SmallNetInflow:           values[1],
		MediumNetInflow:          values[2],
		LargeNetInflow:           values[3],
		SuperLargeNetInflow:      values[4],
		MainNetInflowRatio:       values[5],
		SmallNetInflowRatio:      values[6],
		MediumNetInflowRatio:     values[7],
		LargeNetInflowRatio:      values[8],
		SuperLargeNetInflowRatio: values[9],
		Close:                    values[10],
		ChangePercent:            values[11],
	}, nil
}

// eastMoneySecID 将常见股票代码转换为东财 secid。
func eastMoneySecID(symbol string) string {
	normalized := normalizeStockSymbol(symbol)
	if normalized == "" || !strings.Contains(normalized, ".") {
		return ""
	}
	parts := strings.Split(normalized, ".")
	if len(parts) != 2 {
		return ""
	}
	switch parts[1] {
	case "SH":
		return "1." + parts[0]
	case "SZ", "BJ":
		return "0." + parts[0]
	default:
		return ""
	}
}

// eastMoneySymbolFromCode 将东财市场 ID 和证券代码转换为标准股票代码。
func eastMoneySymbolFromCode(code string, marketID int) string {
	switch marketID {
	case 1:
		return code + ".SH"
	case 0:
		return code + ".SZ"
	default:
		return normalizeStockSymbol(code)
	}
}

// normalizeStockSymbol 将常见输入规范化为 000001.SZ / 600000.SH / 430000.BJ。
func normalizeStockSymbol(symbol string) string {
	value := strings.ToUpper(strings.TrimSpace(symbol))
	value = strings.TrimPrefix(value, "SH")
	value = strings.TrimPrefix(value, "SZ")
	value = strings.TrimPrefix(value, "BJ")
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0] + "." + parts[1]
		}
		return ""
	}
	if len(value) != 6 {
		return ""
	}
	switch value[0] {
	case '6':
		return value + ".SH"
	case '0', '3':
		return value + ".SZ"
	case '4', '8', '9':
		return value + ".BJ"
	default:
		return ""
	}
}

// stripJSONPCallback 去掉 data(...) 一类 JSONP 包装。
func stripJSONPCallback(payload []byte) []byte {
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

// parseFloat 将东财可能返回的空值、横线或数字字符串转成 float64。
func parseFloat(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return 0
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return number
}

// parseRequiredFloat 解析历史资金流核心字段，远端占位符或坏字段必须失败。
func parseRequiredFloat(value string, fieldName string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" || value == "--" {
		return 0, fmt.Errorf("missing %s", fieldName)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return number, nil
}
