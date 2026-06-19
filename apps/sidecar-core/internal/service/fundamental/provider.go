package fundamental

import (
	"context"
	"fmt"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// ReportKind 表示基本面/F10 报告类型。
type ReportKind string

const (
	// ReportLatestFinance 表示最新一期财务主要数据。
	ReportLatestFinance ReportKind = "latest_finance"
	// ReportQuarterFinance 表示季度主要财务指标。
	ReportQuarterFinance ReportKind = "quarter_finance"
	// ReportOrgForecast 表示机构预测明细。
	ReportOrgForecast ReportKind = "org_forecast"
	// ReportForecastSummary 表示机构预测汇总。
	ReportForecastSummary ReportKind = "forecast_summary"
	// ReportValuationPercentile 表示估值百分位。
	ReportValuationPercentile ReportKind = "valuation_percentile"
	// ReportMarginTrading 表示融资融券数据。
	ReportMarginTrading ReportKind = "margin_trading"
	// ReportBlockTrade 表示大宗交易数据。
	ReportBlockTrade ReportKind = "block_trade"
	// ReportHolderTrend 表示户均持股趋势。
	ReportHolderTrend ReportKind = "holder_trend"
	// ReportBillboard 表示龙虎榜数据。
	ReportBillboard ReportKind = "billboard"
	// ReportOperatingDepartment 表示营业部买卖明细。
	ReportOperatingDepartment ReportKind = "operating_department"
)

// ValueFormat 表示单元格展示格式。
type ValueFormat string

const (
	// FormatText 表示普通文本。
	FormatText ValueFormat = "text"
	// FormatMoney 表示金额，按万/亿压缩展示。
	FormatMoney ValueFormat = "money"
	// FormatVolume 表示股数或数量。
	FormatVolume ValueFormat = "volume"
	// FormatPercent 表示百分比数值。
	FormatPercent ValueFormat = "percent"
	// FormatPrice 表示价格或每股指标。
	FormatPrice ValueFormat = "price"
	// FormatDate 表示日期时间，仅展示日期部分。
	FormatDate ValueFormat = "date"
	// FormatInteger 表示整数。
	FormatInteger ValueFormat = "integer"
)

// Provider 定义基本面/F10 数据源必须实现的能力边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Fetch(ctx context.Context, request Request) (Dataset, error)
}

// ProviderStatus 描述基本面数据源来源、授权边界、频率限制和可用性。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	LastCheckedAt time.Time
	LastError     string
}

// Request 是基本面/F10 数据查询请求。
type Request struct {
	Symbol stock.Symbol
	Kind   ReportKind
	Limit  int
}

// Dataset 是基本面 Provider 返回的统一结构化数据。
type Dataset struct {
	Title     string
	Kind      ReportKind
	Symbol    stock.Symbol
	Columns   []Column
	Rows      []Row
	Provider  string
	Source    string
	FetchedAt time.Time
}

// Column 描述 Dataset 中一个字段的显示元信息。
type Column struct {
	Key    string
	Label  string
	Format ValueFormat
	Hidden bool
}

// Row 是一条基本面数据记录，key 必须对应 Column.Key 或远端原始字段。
type Row map[string]any

// ProviderError 是基本面 Provider 调用失败时的可观测错误。
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
		"fundamental provider %s %s failed: %s",
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

// NewProviderError 创建会自动脱敏输出的基本面 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}
