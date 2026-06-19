package fund

import (
	"context"
	"fmt"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Provider 定义基金数据源的抓取和清洗边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	SearchFunds(ctx context.Context, request SearchRequest) ([]SearchItem, error)
	FetchBasic(ctx context.Context, request BasicRequest) (Basic, error)
	FetchHistoryNetValues(ctx context.Context, request HistoryRequest) ([]HistoryNetValue, error)
	FetchRanking(ctx context.Context, request RankingRequest) (RankingResult, error)
	FetchTopHoldings(ctx context.Context, request HoldingRequest) ([]HoldingStock, error)
}

// ProviderStatus 描述基金 Provider 的来源、授权边界和运行状态。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	LastCheckedAt time.Time
	LastError     string
}

// SearchRequest 是基金搜索请求。
type SearchRequest struct {
	Keyword string
	Limit   int
}

// BasicRequest 是基金基础资料请求。
type BasicRequest struct {
	Code string
}

// HistoryRequest 是基金历史净值请求。
type HistoryRequest struct {
	Code      string
	PageIndex int
	PageSize  int
	StartDate string
	EndDate   string
}

// RankingRequest 是基金排行请求。
type RankingRequest struct {
	MarketType string
	FundType   string
	SortField  string
	SortOrder  string
	PageIndex  int
	PageSize   int
}

// HoldingRequest 是基金十大持仓请求。
type HoldingRequest struct {
	Code string
}

// SearchItem 表示基金搜索结果。
type SearchItem struct {
	Code string
	Name string
	Type string
}

// Basic 表示基金基础资料快照。
type Basic struct {
	Code          string
	Name          string
	Type          string
	EstablishedAt string
	Scale         string
	Company       string
	Manager       string
	Rating        string

	Month1Growth *float64
	Month3Growth *float64
	Month6Growth *float64
	Year1Growth  *float64
	Year3Growth  *float64
	Year5Growth  *float64
	YTDGrowth    *float64
	AllGrowth    *float64

	Provider  string
	Source    string
	FetchedAt time.Time
}

// HistoryNetValue 表示基金历史净值条目。
type HistoryNetValue struct {
	Date             string
	NetValue         float64
	AccumulatedValue float64
	DailyGrowth      float64
	BuyStatus        string
	SellStatus       string
}

// RankingItem 表示基金排行条目。
type RankingItem struct {
	Code             string
	Name             string
	Pinyin           string
	NetValueDate     string
	NetUnitValue     *float64
	NetAccumulated   *float64
	DailyGrowth      *float64
	WeekGrowth       *float64
	MonthGrowth      *float64
	ThreeMonthGrowth *float64
	SixMonthGrowth   *float64
	YearGrowth       *float64
	TwoYearGrowth    *float64
	ThreeYearGrowth  *float64
	YTDGrowth        *float64
	SinceInception   *float64
	EstablishDate    string
	Purchasable      bool
	Scale            *float64
	PurchaseRate     *float64
	DiscountRate     *float64
	FundTypeDetail   string
}

// RankingResult 表示一次基金排行响应。
type RankingResult struct {
	Items      []RankingItem
	TotalCount int
	PageIndex  int
	PageSize   int
	TotalPages int
	Provider   string
	Source     string
	FetchedAt  time.Time
}

// HoldingStock 表示基金持仓股票条目。
type HoldingStock struct {
	Rank       int
	StockCode  string
	StockName  string
	Ratio      float64
	Shares     string
	MarketCap  string
	Quarter    string
	Price      *float64
	ChangeRate *float64
	Market     string
}

// ProviderError 是基金 Provider 调用失败时的脱敏错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏后的 Provider 错误。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("fund provider %s %s failed: %s", err.Provider, err.Operation, logger.RedactError(err.Cause))
}

// Unwrap 返回底层错误，便于调用方分类处理。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建基金 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}
