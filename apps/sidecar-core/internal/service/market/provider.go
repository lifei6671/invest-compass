package market

import (
	"context"
	"fmt"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// MarketArea 表示行情 Provider 支持的市场区域。
type MarketArea string

const (
	// MarketCN 表示中国 A 股市场。
	MarketCN MarketArea = "CN"
	// MarketHK 表示香港股票市场。
	MarketHK MarketArea = "HK"
	// MarketUS 表示美国股票市场。
	MarketUS MarketArea = "US"
)

// Period 表示 K 线周期。
type Period string

const (
	// PeriodMinute 表示当日分时走势，主要用于首页和自选股迷你走势。
	PeriodMinute Period = "minute"
	// Period1Minute 表示 1 分钟 K 线。
	Period1Minute Period = "1m"
	// Period5Minute 表示 5 分钟 K 线。
	Period5Minute Period = "5m"
	// Period15Minute 表示 15 分钟 K 线。
	Period15Minute Period = "15m"
	// Period30Minute 表示 30 分钟 K 线。
	Period30Minute Period = "30m"
	// Period60Minute 表示 60 分钟 K 线。
	Period60Minute Period = "60m"
	// PeriodDay 表示日 K。
	PeriodDay Period = "day"
	// PeriodWeek 表示周 K。
	PeriodWeek Period = "week"
	// PeriodMonth 表示月 K。
	PeriodMonth Period = "month"
	// PeriodQuarter 表示季 K。
	PeriodQuarter Period = "quarter"
	// PeriodYear 表示年 K。
	PeriodYear Period = "year"
)

// IsIntraday 判断周期是否属于盘中数据，盘中数据不能被持久缓存跨日复用。
func (period Period) IsIntraday() bool {
	return period == PeriodMinute || period.IsMinuteKline()
}

// IsMinuteKline 判断周期是否属于真实分钟级 K 线。
func (period Period) IsMinuteKline() bool {
	switch period {
	case Period1Minute, Period5Minute, Period15Minute, Period30Minute, Period60Minute:
		return true
	default:
		return false
	}
}

// Adjust 表示 K 线复权方式。
type Adjust string

const (
	// AdjustNone 表示不复权。
	AdjustNone Adjust = "none"
	// AdjustForward 表示前复权。
	AdjustForward Adjust = "qfq"
	// AdjustBackward 表示后复权。
	AdjustBackward Adjust = "hfq"
)

// MarketProvider 定义首版行情数据源必须实现的能力边界。
type MarketProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Search(ctx context.Context, keyword string) ([]StockBasic, error)
	Quote(ctx context.Context, symbol stock.Symbol) (Quote, error)
	Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error)
}

// ProviderStatus 描述数据源来源、授权边界、频率限制和可用性。
type ProviderStatus struct {
	Name           string
	Source         string
	License        string
	RateLimit      string
	Available      bool
	LastCheckedAt  time.Time
	LastError      string
	SupportedAreas []MarketArea
}

// UnconfiguredProviderStatus 返回未配置真实行情 Provider 时的安全展示状态。
func UnconfiguredProviderStatus() ProviderStatus {
	return ProviderStatus{
		Name:      "market-provider",
		Source:    "unconfigured",
		Available: false,
		LastError: "market_provider_unconfigured",
	}
}

// UnconfiguredProvider 是生产默认行情 Provider，明确表示首版尚未配置真实数据源。
type UnconfiguredProvider struct{}

// Name 返回未配置行情 Provider 的稳定名称。
func (UnconfiguredProvider) Name() string {
	return "market-provider"
}

// Status 返回不可用状态，避免 UI 或日志把未配置误判成真实数据源。
func (UnconfiguredProvider) Status(context.Context) ProviderStatus {
	return UnconfiguredProviderStatus()
}

// Search 在未配置真实数据源时快速失败，不返回假股票数据。
func (UnconfiguredProvider) Search(context.Context, string) ([]StockBasic, error) {
	return nil, &xerr.Error{Code: xerr.MarketProviderUnconfigured}
}

// Quote 在未配置真实数据源时快速失败，不返回假行情。
func (UnconfiguredProvider) Quote(context.Context, stock.Symbol) (Quote, error) {
	return Quote{}, &xerr.Error{Code: xerr.MarketProviderUnconfigured}
}

// Kline 在未配置真实数据源时快速失败，不返回假 K 线。
func (UnconfiguredProvider) Kline(context.Context, KlineRequest) ([]KlineBar, error) {
	return nil, &xerr.Error{Code: xerr.MarketProviderUnconfigured}
}

// Supports 判断 Provider 是否声明支持目标市场区域。
func (status ProviderStatus) Supports(area MarketArea) bool {
	for _, supported := range status.SupportedAreas {
		if supported == area {
			return true
		}
	}
	return false
}

// StockBasic 是股票搜索和基础信息查询的统一返回模型。
type StockBasic struct {
	Symbol   stock.Symbol
	Name     string
	Code     string
	Market   string
	Exchange string
	Industry string
	Concept  string
}

// Quote 是个股行情快照模型。
type Quote struct {
	Symbol         stock.Symbol
	Price          float64
	ChangeAmount   float64
	ChangePercent  float64
	Open           float64
	High           float64
	Low            float64
	PreClose       float64
	Volume         float64
	Amount         float64
	TurnoverRate   float64
	PE             float64
	PB             float64
	TotalMarketCap float64
	FloatMarketCap float64
	QuoteTime      time.Time
	Provider       string
}

// NormalizeQuote 统一修正行情快照中的无效 0 价，避免未开盘时把 0 当作真实现价计算成 -100%。
func NormalizeQuote(quote Quote) Quote {
	if quote.Price <= 0 && quote.PreClose > 0 {
		quote.Price = quote.PreClose
	}
	if quote.Price > 0 {
		if quote.Open <= 0 {
			quote.Open = quote.Price
		}
		if quote.High <= 0 {
			quote.High = quote.Price
		}
		if quote.Low <= 0 {
			quote.Low = quote.Price
		}
	}
	if quote.Price > 0 && quote.PreClose > 0 {
		quote.ChangeAmount = quote.Price - quote.PreClose
		quote.ChangePercent = quote.ChangeAmount / quote.PreClose * 100
	}
	return quote
}

// KlineRequest 是 K 线查询请求。
type KlineRequest struct {
	Symbol stock.Symbol
	Period Period
	Adjust Adjust
	Limit  int
}

// KlineBar 是标准 K 线数据点。
type KlineBar struct {
	Symbol    stock.Symbol
	Period    Period
	Adjust    Adjust
	TradeDate string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Amount    float64
	Provider  string
}

// ProviderError 是 Provider 调用失败时的可观测错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏后的 Provider 错误，避免请求头或密钥泄露进日志和响应。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf(
		"market provider %s %s failed: %s",
		err.Provider,
		err.Operation,
		logger.RedactError(err.Cause),
	)
}

// Unwrap 返回原始错误，供内部错误分类使用。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建会自动脱敏输出的 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}
