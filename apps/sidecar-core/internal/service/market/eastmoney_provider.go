package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	eastMoneyProviderName = "eastmoney-market-source"
	defaultEastMoneyURL   = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
)

// EastMoneyConfig 描述东方财富 K 线接口的可替换运行参数。
type EastMoneyConfig struct {
	KlineURL   string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// EastMoneyProvider 使用东方财富 push2his 接口实现 CN A 股 K 线。
type EastMoneyProvider struct {
	klineURL string
	client   *crawler.Client
}

// NewEastMoneyProvider 创建东方财富 K 线数据源。
func NewEastMoneyProvider(config EastMoneyConfig) (*EastMoneyProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultProviderTimeout
	}

	client, err := newMarketCrawler(eastMoneyProviderName, timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &EastMoneyProvider{
		klineURL: firstNonEmpty(config.KlineURL, defaultEastMoneyURL),
		client:   client,
	}, nil
}

// Kline 调用东方财富 push2his K 线接口并转换为统一 K 线数组。
func (provider *EastMoneyProvider) Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error) {
	secID, err := eastMoneySecIDFromSymbol(request.Symbol)
	if err != nil {
		return nil, NewProviderError(eastMoneyProviderName, "kline_symbol", err)
	}
	period, err := eastMoneyPeriod(request.Period)
	if err != nil {
		return nil, NewProviderError(eastMoneyProviderName, "kline_period", err)
	}
	adjust, err := eastMoneyAdjust(request.Adjust)
	if err != nil {
		return nil, NewProviderError(eastMoneyProviderName, "kline_adjust", err)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 200
	}

	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.klineURL,
		Query: map[string]string{
			"secid":   secID,
			"klt":     period,
			"fqt":     adjust,
			"end":     "20500101",
			"lmt":     strconv.Itoa(limit),
			"fields1": "f1,f2,f3,f4,f5,f6",
			"fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61",
			"_":       strconv.FormatInt(time.Now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":          "application/json,text/plain,*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Referer":         "https://quote.eastmoney.com/",
		},
	})
	if err != nil {
		return nil, NewProviderError(eastMoneyProviderName, "kline", err)
	}
	bars, err := parseEastMoneyKline(result.BodyBytes, request, eastMoneyProviderName)
	if err != nil {
		return nil, NewProviderError(eastMoneyProviderName, "kline_parse", err)
	}
	sort.SliceStable(bars, func(left int, right int) bool {
		return bars[left].TradeDate < bars[right].TradeDate
	})
	return bars, nil
}

type eastMoneyKlineResponse struct {
	RC      int    `json:"rc"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Klines []string `json:"klines"`
	} `json:"data"`
}

// parseEastMoneyKline 解析东方财富逗号分隔 K 线字段，并拒绝异常响应码和坏行。
func parseEastMoneyKline(payload []byte, request KlineRequest, providerName string) ([]KlineBar, error) {
	var response eastMoneyKlineResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.RC != 0 {
		return nil, fmt.Errorf("eastmoney kline rc %d: %s", response.RC, response.Message)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("eastmoney kline code %d: %s", response.Code, response.Message)
	}
	if len(response.Data.Klines) == 0 {
		return nil, fmt.Errorf("eastmoney kline empty data")
	}

	bars := make([]KlineBar, 0, len(response.Data.Klines))
	for index, row := range response.Data.Klines {
		fields := strings.Split(row, ",")
		if len(fields) < 6 {
			return nil, fmt.Errorf("eastmoney kline row %d has %d fields", index, len(fields))
		}
		bar, err := parseEastMoneyKlineRow(fields, request, providerName)
		if err != nil {
			return nil, fmt.Errorf("eastmoney kline row %d: %w", index, err)
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

// parseEastMoneyKlineRow 将单行 push2his 字符串字段映射为统一 KlineBar。
func parseEastMoneyKlineRow(fields []string, request KlineRequest, providerName string) (KlineBar, error) {
	open, err := parseFloatField(fields[1], "open")
	if err != nil {
		return KlineBar{}, err
	}
	closeValue, err := parseFloatField(fields[2], "close")
	if err != nil {
		return KlineBar{}, err
	}
	high, err := parseFloatField(fields[3], "high")
	if err != nil {
		return KlineBar{}, err
	}
	low, err := parseFloatField(fields[4], "low")
	if err != nil {
		return KlineBar{}, err
	}
	volume, err := parseFloatField(fields[5], "volume")
	if err != nil {
		return KlineBar{}, err
	}
	amount := 0.0
	if len(fields) > 6 && strings.TrimSpace(fields[6]) != "" {
		amount, err = parseFloatField(fields[6], "amount")
		if err != nil {
			return KlineBar{}, err
		}
	}
	return KlineBar{
		Symbol:    request.Symbol,
		Period:    request.Period,
		Adjust:    request.Adjust,
		TradeDate: strings.TrimSpace(fields[0]),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closeValue,
		Volume:    volume,
		Amount:    amount,
		Provider:  providerName,
	}, nil
}

// eastMoneySecIDFromSymbol 将标准 symbol 转换为东方财富 secid，当前只支持沪深 A 股。
func eastMoneySecIDFromSymbol(symbol stock.Symbol) (string, error) {
	if symbol.Market != string(MarketCN) {
		return "", fmt.Errorf("market %s is not supported by %s", symbol.Market, eastMoneyProviderName)
	}
	switch symbol.Exchange {
	case "SH":
		if !isCNStockSymbol(symbol) {
			return "", fmt.Errorf("code %s is not supported by %s", symbol.Code, eastMoneyProviderName)
		}
		return "1." + symbol.Code, nil
	case "SZ":
		if !isCNStockSymbol(symbol) {
			return "", fmt.Errorf("code %s is not supported by %s", symbol.Code, eastMoneyProviderName)
		}
		return "0." + symbol.Code, nil
	default:
		return "", fmt.Errorf("exchange %s is not supported by %s", symbol.Exchange, eastMoneyProviderName)
	}
}

// eastMoneyPeriod 将内部 K 线周期映射为东方财富 klt 参数。
func eastMoneyPeriod(period Period) (string, error) {
	switch period {
	case Period1Minute:
		return "1", nil
	case Period5Minute:
		return "5", nil
	case Period15Minute:
		return "15", nil
	case Period30Minute:
		return "30", nil
	case Period60Minute:
		return "60", nil
	case PeriodDay:
		return "101", nil
	case PeriodWeek:
		return "102", nil
	case PeriodMonth:
		return "103", nil
	case PeriodQuarter:
		return "104", nil
	case PeriodYear:
		return "106", nil
	default:
		return "", fmt.Errorf("unsupported period %q", period)
	}
}

// eastMoneyAdjust 将内部复权方式映射为东方财富 fqt 参数。
func eastMoneyAdjust(adjust Adjust) (string, error) {
	switch adjust {
	case "", AdjustNone:
		return "0", nil
	case AdjustForward:
		return "1", nil
	case AdjustBackward:
		return "2", nil
	default:
		return "", fmt.Errorf("unsupported adjust %q", adjust)
	}
}
