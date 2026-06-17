package market

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/stock"
)

// TestProviderStatusDescribesComplianceBoundary 验证 Provider 状态必须说明来源、授权边界和频率限制。
func TestProviderStatusDescribesComplianceBoundary(t *testing.T) {
	status := ProviderStatus{
		Name:           "example-provider",
		Source:         "用户配置的授权数据源",
		License:        "由用户自行确认数据授权",
		RateLimit:      "60 requests/minute",
		Available:      true,
		LastCheckedAt:  time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		LastError:      "",
		SupportedAreas: []MarketArea{MarketCN, MarketHK, MarketUS},
	}

	if status.Name == "" || status.Source == "" || status.License == "" || status.RateLimit == "" {
		t.Fatalf("provider status must include compliance metadata: %+v", status)
	}
	if !status.Supports(MarketCN) || !status.Supports(MarketHK) || !status.Supports(MarketUS) {
		t.Fatalf("expected provider status to include supported markets: %+v", status.SupportedAreas)
	}
}

// TestMarketProviderContractUsesStandardSymbols 验证 Provider 接口以统一 Symbol 模型作为输入输出边界。
func TestMarketProviderContractUsesStandardSymbols(t *testing.T) {
	provider := fakeProvider{}
	ctx := context.Background()

	results, err := provider.Search(ctx, "茅台")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Symbol.String() != "CN:SH:600519" {
		t.Fatalf("unexpected search results: %+v", results)
	}

	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	quote, err := provider.Quote(ctx, symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.Symbol.String() != "CN:SH:600519" || quote.Provider != provider.Name() || quote.Price == 0 {
		t.Fatalf("unexpected quote: %+v", quote)
	}

	bars, err := provider.Kline(ctx, KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 1 || bars[0].Symbol.String() != "CN:SH:600519" || bars[0].Close == 0 {
		t.Fatalf("unexpected kline bars: %+v", bars)
	}
}

// TestProviderErrorRedactsSensitiveRequest 验证 Provider 错误可观测但不会泄露请求头或密钥。
func TestProviderErrorRedactsSensitiveRequest(t *testing.T) {
	err := NewProviderError(
		"example-provider",
		"quote",
		errors.New("remote failed with Authorization: Bearer sk-market-secret api_key=raw-secret"),
	)

	message := err.Error()
	if strings.Contains(message, "sk-market-secret") || strings.Contains(message, "raw-secret") {
		t.Fatalf("provider error leaked secret: %s", message)
	}
	if !strings.Contains(message, "example-provider") || !strings.Contains(message, "quote") {
		t.Fatalf("provider error should keep observable context: %s", message)
	}
}

type fakeProvider struct{}

// Name 返回测试 Provider 名称。
func (fakeProvider) Name() string {
	return "fake-provider"
}

// Status 返回测试 Provider 的合规元信息。
func (fakeProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:      "fake-provider",
		Source:    "单元测试内存数据",
		License:   "仅用于单元测试",
		RateLimit: "unlimited in test",
		Available: true,
	}
}

// Search 返回固定的标准化股票结果。
func (fakeProvider) Search(context.Context, string) ([]StockBasic, error) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		return nil, err
	}
	return []StockBasic{{
		Symbol:   symbol,
		Name:     "贵州茅台",
		Code:     "600519",
		Market:   "CN",
		Exchange: "SH",
	}}, nil
}

// Quote 返回固定行情结果，用于验证接口契约。
func (fakeProvider) Quote(context.Context, stock.Symbol) (Quote, error) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		return Quote{}, err
	}
	return Quote{
		Symbol:        symbol,
		Price:         100,
		ChangePercent: 1.23,
		QuoteTime:     time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		Provider:      "fake-provider",
	}, nil
}

// Kline 返回固定 K 线结果，用于验证接口契约。
func (fakeProvider) Kline(context.Context, KlineRequest) ([]KlineBar, error) {
	symbol, err := stock.ParseSymbol("CN:SH:600519")
	if err != nil {
		return nil, err
	}
	return []KlineBar{{
		Symbol:    symbol,
		Period:    PeriodDay,
		Adjust:    AdjustForward,
		TradeDate: "2026-06-17",
		Open:      99,
		High:      101,
		Low:       98,
		Close:     100,
		Volume:    1000,
		Provider:  "fake-provider",
	}}, nil
}
