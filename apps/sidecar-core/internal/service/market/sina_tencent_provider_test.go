package market

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// TestSinaTencentProviderSearchParsesSinaSuggest 验证新浪 suggest 响应能转换成标准股票搜索结果。
func TestSinaTencentProviderSearchParsesSinaSuggest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/suggest" || request.URL.Query().Get("key") != "茅台" {
			t.Fatalf("unexpected suggest request: %s", request.URL.String())
		}
		_, _ = writer.Write(mustGB18030(t, `var suggestdata="贵州茅台,11,600519,sh600519,贵州茅台,guizhoumaotai;茅台转债,12,110000,sh110000,茅台转债";`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		SuggestURL: server.URL + "/suggest",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}

	results, err := provider.Search(context.Background(), "茅台")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one stock result, got %+v", results)
	}
	if results[0].Symbol.String() != "CN:SH:600519" || results[0].Name != "贵州茅台" || results[0].Exchange != "SH" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
}

// TestSinaProviderDoesNotExposeKline 验证新浪数据源只负责搜索和实时行情，不承载腾讯 K 线职责。
func TestSinaProviderDoesNotExposeKline(t *testing.T) {
	provider, err := NewSinaProvider(SinaConfig{})
	if err != nil {
		t.Fatalf("NewSinaProvider returned error: %v", err)
	}
	if _, ok := any(provider).(interface {
		Kline(context.Context, KlineRequest) ([]KlineBar, error)
	}); ok {
		t.Fatal("SinaProvider must not implement Kline")
	}
}

// TestSinaTencentProviderQuoteParsesSinaCNQuote 验证新浪 A 股行情快照能转换成标准 Quote。
func TestSinaTencentProviderQuoteParsesSinaCNQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote" || request.URL.Query().Get("list") != "sh600519" {
			t.Fatalf("unexpected quote request: %s", request.URL.String())
		}
		parts := []string{
			"贵州茅台", "1790.00", "1780.00", "1810.50", "1820.00", "1775.00", "1810.00", "1811.00",
			"123456", "223456789.00", "100", "1810.00", "200", "1809.00", "300", "1808.00",
			"400", "1807.00", "500", "1806.00", "100", "1811.00", "200", "1812.00",
			"300", "1813.00", "400", "1814.00", "500", "1815.00", "2026-06-19", "15:00:00",
		}
		_, _ = writer.Write(mustGB18030(t, `var hq_str_sh600519="`+strings.Join(parts, ",")+`";`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		QuoteURL: server.URL + "/quote",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.Symbol.String() != "CN:SH:600519" || quote.Price != 1810.50 || quote.PreClose != 1780.00 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	if quote.ChangeAmount != 30.50 || quote.ChangePercent < 1.71 || quote.ChangePercent > 1.72 {
		t.Fatalf("unexpected quote change fields: %+v", quote)
	}
}

// TestParseSinaQuoteTimeUsesChinaMarketTimezone 验证新浪行情时间按中国市场时区解析，不受本机时区影响。
func TestParseSinaQuoteTimeUsesChinaMarketTimezone(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC-test", 0)
	defer func() {
		time.Local = originalLocal
	}()

	parsed, err := parseSinaQuoteTime("2026-06-19", "15:00:00")
	if err != nil {
		t.Fatalf("parseSinaQuoteTime returned error: %v", err)
	}
	wantUTC := time.Date(2026, 6, 19, 7, 0, 0, 0, time.UTC)
	if !parsed.UTC().Equal(wantUTC) {
		t.Fatalf("expected China market instant %s, got %s", wantUTC, parsed.UTC())
	}
}

// TestSinaTencentProviderRejectsNonStockCNCode 验证真实行情 Provider 不把转债等六位非股票代码伪装成股票行情。
func TestSinaTencentProviderRejectsNonStockCNCode(t *testing.T) {
	provider, err := NewSinaTencentProvider(SinaTencentConfig{})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:110000")

	if _, err := provider.Quote(context.Background(), symbol); err == nil {
		t.Fatal("expected non-stock quote error")
	}
	if _, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1}); err == nil {
		t.Fatal("expected non-stock kline error")
	}
}

// TestTencentProviderDoesNotExposeSearchOrQuote 验证腾讯数据源只负责 K 线，不承载新浪搜索和实时行情职责。
func TestTencentProviderDoesNotExposeSearchOrQuote(t *testing.T) {
	provider, err := NewTencentProvider(TencentConfig{})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
	}
	if _, ok := any(provider).(interface {
		Search(context.Context, string) ([]StockBasic, error)
	}); ok {
		t.Fatal("TencentProvider must not implement Search")
	}
	if _, ok := any(provider).(interface {
		Quote(context.Context, stock.Symbol) (Quote, error)
	}); ok {
		t.Fatal("TencentProvider must not implement Quote")
	}
}

// TestEastMoneyProviderDoesNotExposeSearchOrQuote 验证东财数据源只负责 K 线，不承载搜索和实时行情职责。
func TestEastMoneyProviderDoesNotExposeSearchOrQuote(t *testing.T) {
	provider, err := NewEastMoneyProvider(EastMoneyConfig{})
	if err != nil {
		t.Fatalf("NewEastMoneyProvider returned error: %v", err)
	}
	if _, ok := any(provider).(interface {
		Search(context.Context, string) ([]StockBasic, error)
	}); ok {
		t.Fatal("EastMoneyProvider must not implement Search")
	}
	if _, ok := any(provider).(interface {
		Quote(context.Context, stock.Symbol) (Quote, error)
	}); ok {
		t.Fatal("EastMoneyProvider must not implement Quote")
	}
}

// TestSinaTencentProviderKlineMapsNoAdjustForTencent 验证腾讯普通 K 线请求使用空复权参数，而不是内部 none 枚举值。
func TestSinaTencentProviderKlineMapsNoAdjustForTencent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/kline" || request.URL.Query().Get("param") != "sh600519,day,,,2," {
			t.Fatalf("unexpected kline request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"sh600519":{"day":[["2026-06-18","1790.00","1810.50","1820.00","1775.00","123456"]]}}}`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		KlineURL: server.URL + "/kline",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{
		Symbol: symbol,
		Period: PeriodDay,
		Adjust: AdjustNone,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 1 || bars[0].Adjust != AdjustNone || bars[0].TradeDate != "2026-06-18" {
		t.Fatalf("unexpected bars: %+v", bars)
	}
}

// TestSinaTencentProviderKlineParsesTencentResponse 验证腾讯 K 线结构化响应能转换成标准 KlineBar。
func TestSinaTencentProviderKlineParsesTencentResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/kline" || request.URL.Query().Get("param") != "sh600519,day,,,2,qfq" {
			t.Fatalf("unexpected kline request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"sh600519":{"qfqday":[["2026-06-18","1790.00","1810.50","1820.00","1775.00","123456"],["2026-06-19","1810.00","1822.00","1830.00","1800.00","223456"]]}}}`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		KlineURL: server.URL + "/kline",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{
		Symbol: symbol,
		Period: PeriodDay,
		Adjust: AdjustForward,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 2 || bars[0].TradeDate != "2026-06-18" || bars[1].Close != 1822.00 {
		t.Fatalf("unexpected bars: %+v", bars)
	}
	if bars[0].Symbol.String() != "CN:SH:600519" || bars[0].Period != PeriodDay || bars[0].Adjust != AdjustForward {
		t.Fatalf("unexpected bar metadata: %+v", bars[0])
	}
}

// TestEastMoneyProviderKlineParsesPush2HisResponse 验证东财 push2his 响应能转换成标准 KlineBar。
func TestEastMoneyProviderKlineParsesPush2HisResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/qt/stock/kline/get" {
			t.Fatalf("unexpected eastmoney path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("secid") != "1.600519" || query.Get("klt") != "101" || query.Get("fqt") != "1" || query.Get("lmt") != "2" {
			t.Fatalf("unexpected eastmoney query: %s", request.URL.RawQuery)
		}
		if !strings.Contains(request.Header.Get("Referer"), "quote.eastmoney.com") {
			t.Fatalf("expected eastmoney referer, got %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{"rc":0,"data":{"klines":["2026-06-18,1790.00,1810.50,1820.00,1775.00,123456,223456789.00","2026-06-19,1810.00,1822.00,1830.00,1800.00,223456,323456789.00"]}}`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyProvider(EastMoneyConfig{
		KlineURL: server.URL + "/api/qt/stock/kline/get",
	})
	if err != nil {
		t.Fatalf("NewEastMoneyProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{
		Symbol: symbol,
		Period: PeriodDay,
		Adjust: AdjustForward,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 2 || bars[0].TradeDate != "2026-06-18" || bars[1].Close != 1822.00 {
		t.Fatalf("unexpected eastmoney bars: %+v", bars)
	}
	if bars[0].Symbol.String() != "CN:SH:600519" || bars[0].Amount != 223456789.00 || bars[0].Provider != "eastmoney-market-source" {
		t.Fatalf("unexpected eastmoney bar metadata: %+v", bars[0])
	}
}

// TestCompositeMarketProviderCombinesSinaAndTencent 验证组合 Provider 对外提供完整 MarketProvider 契约。
func TestCompositeMarketProviderCombinesSinaAndTencent(t *testing.T) {
	sina := &recordingSinaSource{
		searchResult: []StockBasic{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Name: "贵州茅台", Code: "600519", Market: "CN", Exchange: "SH"}},
		quoteResult:  Quote{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Price: 1810.50},
	}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	provider := NewCompositeMarketProvider("composite-test", sina, tencent, func() time.Time {
		return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	})

	var _ MarketProvider = provider
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")
	if _, err := provider.Search(context.Background(), "茅台"); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if _, err := provider.Quote(context.Background(), symbol); err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if _, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1}); err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !sina.searchCalled || !sina.quoteCalled || !tencent.klineCalled {
		t.Fatalf("expected composite provider to delegate calls, sina=%+v tencent=%+v", sina, tencent)
	}
	status := provider.Status(context.Background())
	if status.Name != "composite-test" || !status.Available || !status.Supports(MarketCN) {
		t.Fatalf("unexpected composite status: %+v", status)
	}
}

// TestCompositeMarketProviderFallsBackToEastMoneyKline 验证腾讯 K 线不可用时组合 Provider 使用东财 K 线兜底。
func TestCompositeMarketProviderFallsBackToEastMoneyKline(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{klineErr: errors.New("tencent unavailable")}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	provider := NewCompositeMarketProviderWithKlineFallback("composite-test", sina, tencent, eastMoney, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tencent.klineCalled || !eastMoney.klineCalled {
		t.Fatalf("expected tencent then eastmoney calls, tencent=%+v eastMoney=%+v", tencent, eastMoney)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1822 {
		t.Fatalf("unexpected fallback bars: %+v", bars)
	}
}

// TestCompositeMarketProviderRejectsEmptyFallbackKline 验证兜底源返回空 K 线时不会被当作成功。
func TestCompositeMarketProviderRejectsEmptyFallbackKline(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{klineErr: errors.New("tencent unavailable")}
	eastMoney := &recordingEastMoneySource{klineResult: []KlineBar{}}
	provider := NewCompositeMarketProviderWithKlineFallback("composite-test", sina, tencent, eastMoney, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	_, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err == nil {
		t.Fatal("expected empty fallback kline error")
	}
}

// TestEastMoneyProviderRejectsEmptyKlineResponse 验证东财 K 线成功码下缺少 K 线时显式失败。
func TestEastMoneyProviderRejectsEmptyKlineResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"rc":0,"code":0,"data":{"klines":[]}}`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyProvider(EastMoneyConfig{
		KlineURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewEastMoneyProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	_, err = provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err == nil {
		t.Fatal("expected empty eastmoney kline error")
	}
}

// TestSinaTencentProviderRejectsMalformedQuote 验证行情字段不足时显式失败而不是 panic。
func TestSinaTencentProviderRejectsMalformedQuote(t *testing.T) {
	_, err := parseSinaCNQuote(`var hq_str_sh600519="贵州茅台,1790.00";`, mustParseMarketSymbol(t, "CN:SH:600519"))
	if err == nil {
		t.Fatal("expected malformed quote error")
	}
}

type recordingSinaSource struct {
	searchCalled bool
	quoteCalled  bool
	searchResult []StockBasic
	quoteResult  Quote
}

// Search 记录搜索调用并返回固定股票结果。
func (source *recordingSinaSource) Search(context.Context, string) ([]StockBasic, error) {
	source.searchCalled = true
	return source.searchResult, nil
}

// Quote 记录行情调用并返回固定 quote。
func (source *recordingSinaSource) Quote(context.Context, stock.Symbol) (Quote, error) {
	source.quoteCalled = true
	return source.quoteResult, nil
}

type recordingTencentSource struct {
	klineCalled bool
	klineResult []KlineBar
	klineErr    error
}

// Kline 记录 K 线调用并返回固定 K 线。
func (source *recordingTencentSource) Kline(context.Context, KlineRequest) ([]KlineBar, error) {
	source.klineCalled = true
	if source.klineErr != nil {
		return nil, source.klineErr
	}
	return source.klineResult, nil
}

type recordingEastMoneySource struct {
	klineCalled bool
	klineResult []KlineBar
	klineErr    error
}

// Kline 记录东财 K 线调用并返回固定 K 线。
func (source *recordingEastMoneySource) Kline(context.Context, KlineRequest) ([]KlineBar, error) {
	source.klineCalled = true
	if source.klineErr != nil {
		return nil, source.klineErr
	}
	return source.klineResult, nil
}

// TestSinaTencentProviderStatusDescribesBoundary 验证真实 Provider 状态说明来源、授权和限频边界。
func TestSinaTencentProviderStatusDescribesBoundary(t *testing.T) {
	provider, err := NewSinaTencentProvider(SinaTencentConfig{})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}

	status := provider.Status(context.Background())
	if status.Name != provider.Name() || status.Source == "" || status.License == "" || status.RateLimit == "" {
		t.Fatalf("provider status missing compliance boundary: %+v", status)
	}
	if !status.Supports(MarketCN) || status.Supports(MarketHK) || status.Supports(MarketUS) {
		t.Fatalf("unexpected supported markets: %+v", status.SupportedAreas)
	}
}

// TestSinaTencentProviderRejectsUnsupportedSymbol 验证当前真实 Provider 不把未支持市场伪装为可用数据。
func TestSinaTencentProviderRejectsUnsupportedSymbol(t *testing.T) {
	provider, err := NewSinaTencentProvider(SinaTencentConfig{})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol, err := stock.ParseSymbol("US:AAPL")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}

	_, err = provider.Quote(context.Background(), symbol)
	if err == nil {
		t.Fatal("expected unsupported symbol error")
	}
}

// mustGB18030 将测试文本编码为新浪接口常见的 GB18030 字节。
func mustGB18030(t *testing.T, value string) []byte {
	t.Helper()
	reader := transform.NewReader(strings.NewReader(value), simplifiedchinese.GB18030.NewEncoder())
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, reader); err != nil {
		t.Fatalf("encode GB18030: %v", err)
	}
	return buffer.Bytes()
}
