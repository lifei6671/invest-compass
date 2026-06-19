package marketinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	eastMoneySmartScreenerProviderName = "eastmoney-smart-screener"
	defaultEastMoneySmartStockURL      = "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
	defaultEastMoneySmartBoardURL      = "https://np-tjxg-b.eastmoney.com/api/smart-tag/bkc/v3/pw/search-code"
	defaultEastMoneySmartETFURL        = "https://np-tjxg-b.eastmoney.com/api/smart-tag/etf/v3/pw/search-code"
	defaultEastMoneyHotStrategyURL     = "https://np-ipick.eastmoney.com/recommend/stock/heat/ranking"
)

// SmartScreenerKind 表示东财 smart-tag 自然语言筛选类型。
type SmartScreenerKind string

const (
	SmartScreenerKindStock SmartScreenerKind = "stock"
	SmartScreenerKindBoard SmartScreenerKind = "board"
	SmartScreenerKindETF   SmartScreenerKind = "etf"
)

// SmartScreenerProvider 定义东财自然语言筛选 Provider 能力。
type SmartScreenerProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Fetch(ctx context.Context, request SmartScreenerRequest) (SmartScreenerSnapshot, error)
	FetchHotStrategies(ctx context.Context, request HotStrategyRequest) (HotStrategySnapshot, error)
}

// SmartScreenerRequest 是东财自然语言筛选请求。
type SmartScreenerRequest struct {
	Kind     SmartScreenerKind
	Query    string
	Page     int
	PageSize int
}

// SmartScreenerSnapshot 表示东财自然语言筛选结果。
type SmartScreenerSnapshot struct {
	Kind      SmartScreenerKind
	Query     string
	Total     int
	Columns   []SmartScreenerColumn
	Rows      []map[string]string
	Provider  string
	Source    string
	FetchedAt time.Time
}

// SmartScreenerColumn 表示东财返回列和本地展示标题的映射。
type SmartScreenerColumn struct {
	Key   string
	Title string
}

// HotStrategyRequest 是东财热门策略请求。
type HotStrategyRequest struct {
	Count int
}

// HotStrategySnapshot 表示东财热门策略快照。
type HotStrategySnapshot struct {
	Items     []HotStrategyItem
	Provider  string
	Source    string
	FetchedAt time.Time
}

// HotStrategyItem 表示一条热门选股策略。
type HotStrategyItem struct {
	Rank          int
	Code          string
	Market        string
	Question      string
	HeatValue     int
	ChangePercent float64
}

// EastMoneySmartScreenerConfig 描述东财自然语言筛选 Provider 配置。
type EastMoneySmartScreenerConfig struct {
	StockURL       string
	BoardURL       string
	ETFURL         string
	HotStrategyURL string
	Fingerprint    string
	HTTPClient     *http.Client
	Timeout        time.Duration
	Now            func() time.Time
}

// EastMoneySmartScreenerProvider 抓取东财 smart-tag 自然语言筛选和热门策略数据。
type EastMoneySmartScreenerProvider struct {
	stockURL       string
	boardURL       string
	etfURL         string
	hotStrategyURL string
	fingerprint    string
	client         *crawler.Client
	now            func() time.Time
}

// NewEastMoneySmartScreenerProvider 创建东财自然语言筛选 Provider。
func NewEastMoneySmartScreenerProvider(config EastMoneySmartScreenerConfig) (*EastMoneySmartScreenerProvider, error) {
	client, now, err := newEastMoneyMarketInfoClient(eastMoneySmartScreenerProviderName, config.HTTPClient, config.Timeout, config.Now)
	if err != nil {
		return nil, err
	}
	return &EastMoneySmartScreenerProvider{
		stockURL:       marketInfoFirstNonEmpty(config.StockURL, defaultEastMoneySmartStockURL),
		boardURL:       marketInfoFirstNonEmpty(config.BoardURL, defaultEastMoneySmartBoardURL),
		etfURL:         marketInfoFirstNonEmpty(config.ETFURL, defaultEastMoneySmartETFURL),
		hotStrategyURL: marketInfoFirstNonEmpty(config.HotStrategyURL, defaultEastMoneyHotStrategyURL),
		fingerprint:    strings.TrimSpace(config.Fingerprint),
		client:         client,
		now:            now,
	}, nil
}

// Name 返回东财自然语言筛选 Provider 稳定名称。
func (provider *EastMoneySmartScreenerProvider) Name() string {
	return eastMoneySmartScreenerProviderName
}

// Status 返回东财自然语言筛选数据源状态。
func (provider *EastMoneySmartScreenerProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney smart-tag natural language screener endpoint",
		License:       "需要用户自行配置东财 qgqp_b_id 并确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil && provider.fingerprint != "",
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 根据自然语言筛选股票、板块或 ETF。
func (provider *EastMoneySmartScreenerProvider) Fetch(ctx context.Context, request SmartScreenerRequest) (SmartScreenerSnapshot, error) {
	if provider.fingerprint == "" {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "credential", fmt.Errorf("missing eastmoney qgqp_b_id fingerprint"))
	}
	url, kind, err := provider.smartScreenerURL(request.Kind)
	if err != nil {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "request", err)
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "request", fmt.Errorf("empty smart screener query"))
	}
	page := request.Page
	if page <= 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 120 {
		pageSize = 120
	}

	body, err := json.Marshal(provider.smartScreenerPayload(query, page, pageSize))
	if err != nil {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "encode", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL:    url,
		Method: http.MethodPost,
		Body:   body,
		Headers: map[string]string{
			"Accept":       "application/json,text/plain,*/*",
			"Content-Type": "application/json",
			"Origin":       "https://xuangu.eastmoney.com",
			"Referer":      "https://xuangu.eastmoney.com/",
		},
	})
	if err != nil {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}
	response, err := decodeEastMoneySmartScreener(result.BodyBytes)
	if err != nil {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Code != 100 {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney smart screener code %d: %s", response.Code, response.Message))
	}
	columns := normalizeSmartScreenerColumns(response.Data.Result.Columns)
	rows := normalizeSmartScreenerRows(response.Data.Result.DataList, columns)
	if len(rows) == 0 {
		return SmartScreenerSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney smart screener data"))
	}
	return SmartScreenerSnapshot{
		Kind:      kind,
		Query:     query,
		Total:     response.Data.Result.Count,
		Columns:   columns,
		Rows:      rows,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

// FetchHotStrategies 抓取东财热门选股策略。
func (provider *EastMoneySmartScreenerProvider) FetchHotStrategies(ctx context.Context, request HotStrategyRequest) (HotStrategySnapshot, error) {
	count := request.Count
	if count <= 0 {
		count = 20
	}
	if count > 50 {
		count = 50
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.hotStrategyURL,
		Query: map[string]string{
			"biz":    "web_smart_tag",
			"client": "web",
			"count":  strconv.Itoa(count),
			"trace":  strconv.FormatInt(provider.now().Unix(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Origin":  "https://xuangu.eastmoney.com",
			"Referer": "https://xuangu.eastmoney.com/",
		},
	})
	if err != nil {
		return HotStrategySnapshot{}, NewProviderError(provider.Name(), "hot_strategy_fetch", err)
	}
	var response eastMoneyHotStrategyResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return HotStrategySnapshot{}, NewProviderError(provider.Name(), "hot_strategy_decode", err)
	}
	if response.Code != 0 && response.Code != 100 {
		return HotStrategySnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney hot strategy code %d: %s", response.Code, response.Message))
	}
	items := make([]HotStrategyItem, 0, len(response.Data))
	for _, row := range response.Data {
		if strings.TrimSpace(row.Question) == "" {
			continue
		}
		items = append(items, HotStrategyItem{
			Rank:          row.Rank,
			Code:          strings.TrimSpace(row.Code),
			Market:        strings.TrimSpace(row.Market),
			Question:      strings.TrimSpace(row.Question),
			HeatValue:     row.HeatValue,
			ChangePercent: roundPercent(row.ChangeRate * 100),
		})
	}
	if len(items) == 0 {
		return HotStrategySnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney hot strategy data"))
	}
	return HotStrategySnapshot{
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

// smartScreenerURL 根据筛选类型返回 endpoint。
func (provider *EastMoneySmartScreenerProvider) smartScreenerURL(kind SmartScreenerKind) (string, SmartScreenerKind, error) {
	switch kind {
	case "", SmartScreenerKindStock:
		return provider.stockURL, SmartScreenerKindStock, nil
	case SmartScreenerKindBoard:
		return provider.boardURL, SmartScreenerKindBoard, nil
	case SmartScreenerKindETF:
		return provider.etfURL, SmartScreenerKindETF, nil
	default:
		return "", "", fmt.Errorf("unsupported smart screener kind %q", kind)
	}
}

// smartScreenerPayload 构造东财 smart-tag 请求体。
func (provider *EastMoneySmartScreenerProvider) smartScreenerPayload(query string, page int, pageSize int) map[string]any {
	return map[string]any{
		"keyWord":                query,
		"pageSize":               pageSize,
		"pageNo":                 page,
		"fingerprint":            provider.fingerprint,
		"gids":                   []string{},
		"matchWord":              "",
		"timestamp":              strconv.FormatInt(provider.now().Unix(), 10),
		"shareToGuba":            false,
		"requestId":              "",
		"needCorrect":            true,
		"removedConditionIdList": []string{},
		"xcId":                   "",
		"ownSelectAll":           false,
		"dxInfo":                 []string{},
		"extraCondition":         "",
	}
}

type eastMoneySmartScreenerResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result struct {
			Count    int                         `json:"count"`
			DataList []map[string]any            `json:"dataList"`
			Columns  []eastMoneySmartScreenerCol `json:"columns"`
		} `json:"result"`
	} `json:"data"`
}

type eastMoneySmartScreenerCol struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	DateMsg string `json:"dateMsg"`
	Unit    string `json:"unit"`
}

type eastMoneyHotStrategyResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		Rank       int     `json:"rank"`
		Code       string  `json:"code"`
		Market     string  `json:"market"`
		Question   string  `json:"question"`
		HeatValue  int     `json:"heatValue"`
		ChangeRate float64 `json:"chg"`
	} `json:"data"`
}

// decodeEastMoneySmartScreener 解析东财 smart-tag JSON。
func decodeEastMoneySmartScreener(payload []byte) (eastMoneySmartScreenerResponse, error) {
	var response eastMoneySmartScreenerResponse
	if err := json.Unmarshal(stripMarketInfoJSONP(payload), &response); err != nil {
		return eastMoneySmartScreenerResponse{}, err
	}
	return response, nil
}

// normalizeSmartScreenerColumns 清洗东财返回列标题。
func normalizeSmartScreenerColumns(columns []eastMoneySmartScreenerCol) []SmartScreenerColumn {
	normalized := make([]SmartScreenerColumn, 0, len(columns))
	for _, column := range columns {
		key := strings.TrimSpace(column.Key)
		title := strings.TrimSpace(column.Title)
		if key == "" || title == "" {
			continue
		}
		if dateMsg := strings.TrimSpace(column.DateMsg); dateMsg != "" {
			title += "[" + dateMsg + "]"
		}
		if unit := strings.TrimSpace(column.Unit); unit != "" {
			title += "(" + unit + ")"
		}
		normalized = append(normalized, SmartScreenerColumn{Key: key, Title: title})
	}
	return normalized
}

// normalizeSmartScreenerRows 按列定义把原始行转换为稳定展示字段。
func normalizeSmartScreenerRows(rows []map[string]any, columns []SmartScreenerColumn) []map[string]string {
	normalized := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]string, len(columns))
		for _, column := range columns {
			value, ok := row[column.Key]
			if !ok || value == nil {
				continue
			}
			item[column.Title] = strings.TrimSpace(fmt.Sprint(value))
		}
		if len(item) > 0 {
			normalized = append(normalized, item)
		}
	}
	return normalized
}

// roundPercent 把百分比保留两位小数。
func roundPercent(value float64) float64 {
	return math.Round(value*100) / 100
}
