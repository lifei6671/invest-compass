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
	// PeriodDay 表示日 K。
	PeriodDay Period = "day"
	// PeriodWeek 表示周 K。
	PeriodWeek Period = "week"
	// PeriodMonth 表示月 K。
	PeriodMonth Period = "month"
)

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
	Symbol        stock.Symbol
	Price         float64
	ChangeAmount  float64
	ChangePercent float64
	Open          float64
	High          float64
	Low           float64
	PreClose      float64
	Volume        float64
	Amount        float64
	TurnoverRate  float64
	PE            float64
	PB            float64
	QuoteTime     time.Time
	Provider      string
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
