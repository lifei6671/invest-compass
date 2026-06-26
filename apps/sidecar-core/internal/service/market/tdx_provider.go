package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	gotdx "github.com/bensema/gotdx"
	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

const tdxProviderName = "tdx-market-source"

// TdxConfig 描述通达信行情客户端的运行参数。
type TdxConfig struct {
	Timeout time.Duration
}

// TdxProvider 使用通达信 MAC 行情链路提供 A 股 K 线。
type TdxProvider struct {
	timeout time.Duration
	mu      sync.Mutex
	client  *gotdx.Client
}

// NewTdxProvider 创建通达信 K 线数据源。
func NewTdxProvider(config TdxConfig) (*TdxProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultProviderTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", tdxProviderName)
	}
	return &TdxProvider{timeout: timeout}, nil
}

// Kline 调用通达信 MACSymbolBars 并转换为统一 KlineBar。
func (provider *TdxProvider) Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error) {
	if err := ctx.Err(); err != nil {
		return nil, NewProviderError(tdxProviderName, "kline_context", err)
	}
	market, code, err := tdxMarketFromSymbol(request.Symbol)
	if err != nil {
		return nil, NewProviderError(tdxProviderName, "kline_symbol", err)
	}
	klineType, err := tdxKLineTypeFromPeriod(request.Period)
	if err != nil {
		return nil, NewProviderError(tdxProviderName, "kline_period", err)
	}
	adjust, err := tdxAdjust(request.Adjust)
	if err != nil {
		return nil, NewProviderError(tdxProviderName, "kline_adjust", err)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 500
	}

	bars, err := provider.macSymbolBars(market, code, klineType, uint32(limit), adjust)
	if err != nil {
		return nil, NewProviderError(tdxProviderName, "kline", err)
	}
	if len(bars) == 0 {
		return nil, NewProviderError(tdxProviderName, "kline_empty", fmt.Errorf("empty tdx kline data"))
	}
	result := tdxBarsFromMACSymbolBars(bars, request)
	sort.SliceStable(result, func(left int, right int) bool {
		return result[left].TradeDate < result[right].TradeDate
	})
	if request.Limit > 0 && len(result) > request.Limit {
		result = result[len(result)-request.Limit:]
	}
	return result, nil
}

// macSymbolBars 执行通达信调用，失败时重建客户端重试一次。
func (provider *TdxProvider) macSymbolBars(market uint8, code string, period uint16, count uint32, adjust uint16) ([]proto.MACSymbolBar, error) {
	if err := provider.ensureMACClient(); err != nil {
		return nil, err
	}
	provider.mu.Lock()
	bars, err := provider.client.MACSymbolBars(market, code, period, 1, 0, count, adjust)
	provider.mu.Unlock()
	if err == nil {
		return bars, nil
	}
	if reconnectErr := provider.reconnectMACClient(); reconnectErr != nil {
		return nil, reconnectErr
	}
	provider.mu.Lock()
	bars, err = provider.client.MACSymbolBars(market, code, period, 1, 0, count, adjust)
	provider.mu.Unlock()
	return bars, err
}

// ensureMACClient 延迟创建通达信 MAC 客户端，避免启动时阻塞 Provider 初始化。
func (provider *TdxProvider) ensureMACClient() error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.client == nil {
		provider.client = provider.newMACClient()
	}
	return nil
}

// reconnectMACClient 关闭旧连接并重建通达信 MAC 客户端。
func (provider *TdxProvider) reconnectMACClient() error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.client != nil {
		provider.client.Disconnect()
	}
	provider.client = provider.newMACClient()
	return nil
}

// newMACClient 构造自动选择最快服务器的通达信 MAC 客户端。
func (provider *TdxProvider) newMACClient() *gotdx.Client {
	timeoutSeconds := int(provider.timeout.Seconds())
	if timeoutSeconds <= 0 {
		timeoutSeconds = int(defaultProviderTimeout.Seconds())
	}
	return gotdx.NewMAC(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(timeoutSeconds),
	)
}

// tdxMarketFromSymbol 将标准 symbol 转换为通达信 market/code。
func tdxMarketFromSymbol(symbol stock.Symbol) (uint8, string, error) {
	if symbol.Market != string(MarketCN) {
		return 0, "", fmt.Errorf("market %s is not supported by %s", symbol.Market, tdxProviderName)
	}
	if !isCNMarketQuoteSymbol(symbol) {
		return 0, "", fmt.Errorf("code %s is not supported by %s", symbol.Code, tdxProviderName)
	}
	switch symbol.Exchange {
	case "SH":
		return types.MarketSH.Uint8(), symbol.Code, nil
	case "SZ":
		return types.MarketSZ.Uint8(), symbol.Code, nil
	default:
		return 0, "", fmt.Errorf("exchange %s is not supported by %s", symbol.Exchange, tdxProviderName)
	}
}

// tdxKLineTypeFromPeriod 将内部周期映射为通达信 K 线类型。
func tdxKLineTypeFromPeriod(period Period) (uint16, error) {
	switch period {
	case Period1Minute:
		return 8, nil
	case Period5Minute:
		return 0, nil
	case Period15Minute:
		return 1, nil
	case Period30Minute:
		return 2, nil
	case Period60Minute:
		return 3, nil
	case PeriodDay:
		return 4, nil
	case PeriodWeek:
		return 5, nil
	case PeriodMonth:
		return 6, nil
	case PeriodQuarter:
		return 10, nil
	case PeriodYear:
		return 11, nil
	default:
		return 0, fmt.Errorf("unsupported period %q", period)
	}
}

// tdxAdjust 将内部复权枚举映射为通达信复权参数。
func tdxAdjust(adjust Adjust) (uint16, error) {
	switch adjust {
	case "", AdjustNone:
		return types.AdjustNone, nil
	case AdjustForward:
		return types.AdjustQFQ, nil
	case AdjustBackward:
		return types.AdjustHFQ, nil
	default:
		return 0, fmt.Errorf("unsupported adjust %q", adjust)
	}
}

// tdxBarsFromMACSymbolBars 将通达信 MAC K 线映射为统一行情模型。
func tdxBarsFromMACSymbolBars(bars []proto.MACSymbolBar, request KlineRequest) []KlineBar {
	result := make([]KlineBar, 0, len(bars))
	for _, bar := range bars {
		result = append(result, KlineBar{
			Symbol:    request.Symbol,
			Period:    request.Period,
			Adjust:    request.Adjust,
			TradeDate: tdxTradeDate(bar.DateTime),
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Vol,
			Amount:    bar.Amount,
			Provider:  tdxProviderName,
		})
	}
	return result
}

// tdxTradeDate 统一通达信日期格式，分钟 K 保留到分钟，日线及以上只保留日期。
func tdxTradeDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= len("2006-01-02") {
		return trimmed
	}
	if len(trimmed) >= len("2006-01-02 15:04:05") && trimmed[11:] == "00:00:00" {
		return trimmed[:10]
	}
	if len(trimmed) >= len("2006-01-02 15:04") {
		return trimmed[:16]
	}
	return trimmed
}
