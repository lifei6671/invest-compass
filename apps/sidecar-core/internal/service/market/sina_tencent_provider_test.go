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

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	settingsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
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
		if request.URL.Query().Get("rn") != "" {
			t.Fatalf("sina quote request must not append rn because Sina treats the full list value as symbol: %s", request.URL.String())
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

	provider, err := NewSinaProvider(SinaConfig{
		QuoteURL: server.URL + "/quote",
	})
	if err != nil {
		t.Fatalf("NewSinaProvider returned error: %v", err)
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

// TestSinaTencentProviderQuoteNormalizesPreOpenZeroPrice 验证未开盘时新浪返回 0 现价不会被计算成 -100%。
func TestSinaTencentProviderQuoteNormalizesPreOpenZeroPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote" || request.URL.Query().Get("list") != "sh603026" {
			t.Fatalf("unexpected quote request: %s", request.URL.String())
		}
		parts := []string{
			"石大胜华", "0.00", "93.78", "0.00", "0.00", "0.00", "0.00", "0.00",
			"0", "0.00", "0", "0.00", "0", "0.00", "0", "0.00",
			"0", "0.00", "0", "0.00", "0", "0.00", "0", "0.00",
			"0", "0.00", "0", "0.00", "0", "0.00", "2026-06-26", "09:14:33",
		}
		_, _ = writer.Write(mustGB18030(t, `var hq_str_sh603026="`+strings.Join(parts, ",")+`";`))
	}))
	defer server.Close()

	provider, err := NewSinaProvider(SinaConfig{
		QuoteURL: server.URL + "/quote",
	})
	if err != nil {
		t.Fatalf("NewSinaProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:603026")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.Price != 93.78 || quote.Open != 93.78 || quote.High != 93.78 || quote.Low != 93.78 {
		t.Fatalf("unexpected normalized pre-open price fields: %+v", quote)
	}
	if quote.ChangeAmount != 0 || quote.ChangePercent != 0 {
		t.Fatalf("pre-open zero price must not become negative change: %+v", quote)
	}
}

// TestSinaTencentProviderQuoteEnrichesValuationFields 验证 A 股实时行情会补充换手率、市盈率和市净率字段。
func TestSinaTencentProviderQuoteEnrichesValuationFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/quote":
			if request.URL.Query().Get("list") != "sz000001" {
				t.Fatalf("unexpected quote request: %s", request.URL.String())
			}
			parts := []string{
				"平安银行", "11.00", "10.50", "11.20", "11.30", "10.90", "11.20", "11.21",
				"123456", "223456789.00", "100", "11.20", "200", "11.19", "300", "11.18",
				"400", "11.17", "500", "11.16", "100", "11.21", "200", "11.22",
				"300", "11.23", "400", "11.24", "500", "11.25", "2026-06-25", "09:56:49",
			}
			_, _ = writer.Write(mustGB18030(t, `var hq_str_sz000001="`+strings.Join(parts, ",")+`";`))
		case "/valuation":
			if request.URL.Query().Get("secid") != "0.000001" {
				t.Fatalf("unexpected valuation request: %s", request.URL.String())
			}
			if request.URL.Query().Get("ut") == "" {
				t.Fatalf("valuation request must include eastmoney ut token: %s", request.URL.String())
			}
			if request.URL.Query().Get("fields") != "f168,f162,f167,f116,f117" {
				t.Fatalf("unexpected valuation fields: %s", request.URL.String())
			}
			_, _ = writer.Write([]byte(`{"rc":0,"data":{"f168":398,"f162":35210,"f167":239072,"f116":123456789000,"f117":98765432100}}`))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.String())
		}
	}))
	defer server.Close()

	provider, err := NewSinaProvider(SinaConfig{
		QuoteURL:     server.URL + "/quote",
		ValuationURL: server.URL + "/valuation",
	})
	if err != nil {
		t.Fatalf("NewSinaProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SZ:000001")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.TurnoverRate != 3.98 || quote.PE != 352.10 || quote.PB != 2390.72 {
		t.Fatalf("unexpected valuation fields: %+v", quote)
	}
	if quote.TotalMarketCap != 123456789000 || quote.FloatMarketCap != 98765432100 {
		t.Fatalf("unexpected market cap fields: %+v", quote)
	}
}

// TestSinaTencentProviderQuoteDoesNotOverwriteMissingValuationFields 验证东财部分增强字段缺失时不把已有行情字段覆盖成 0。
func TestSinaTencentProviderQuoteDoesNotOverwriteMissingValuationFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/quote":
			parts := []string{
				"铜冠铜箔", "175.00", "183.27", "170.30", "176.00", "160.96", "170.30", "170.31",
				"57412732", "9679600141.69", "27637", "170.30", "4900", "170.29", "39300", "170.28",
				"1400", "170.27", "200", "170.26", "58600", "170.31", "200", "170.34",
				"1100", "170.35", "200", "170.38", "100", "170.44", "2026-06-26", "15:35:00",
			}
			_, _ = writer.Write(mustGB18030(t, `var hq_str_sz301217="`+strings.Join(parts, ",")+`";`))
		case "/valuation":
			_, _ = writer.Write([]byte(`{"rc":0,"data":{"f168":"-","f162":35210,"f167":239072,"f116":352100000000,"f117":180000000000}}`))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.String())
		}
	}))
	defer server.Close()

	provider, err := NewSinaProvider(SinaConfig{
		QuoteURL:     server.URL + "/quote",
		ValuationURL: server.URL + "/valuation",
	})
	if err != nil {
		t.Fatalf("NewSinaProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SZ:301217")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.TurnoverRate < 5.43 || quote.TurnoverRate > 5.44 {
		t.Fatalf("expected turnover rate to be derived from volume and float market cap, got %+v", quote)
	}
	if quote.PE != 352.10 || quote.PB != 2390.72 {
		t.Fatalf("unexpected valuation fields: %+v", quote)
	}
}

// TestParseEastMoneyQuoteValuationKeepsPartialFields 验证东财估值部分缺失时仍保留可用字段。
func TestParseEastMoneyQuoteValuationKeepsPartialFields(t *testing.T) {
	valuation, err := parseEastMoneyQuoteValuation(eastMoneyQuoteValuationResponse{
		RC:   0,
		Code: 0,
		Data: eastMoneyQuoteValuationData{
			TurnoverRate:   []byte(`398`),
			PE:             []byte(`null`),
			PB:             []byte(`239072`),
			TotalMarketCap: []byte(`123456789000`),
			FloatMarketCap: []byte(`98765432100`),
		},
	})
	if err != nil {
		t.Fatalf("parseEastMoneyQuoteValuation returned error: %v", err)
	}
	if valuation.TurnoverRate != 3.98 || valuation.PE != 0 || valuation.PB != 2390.72 {
		t.Fatalf("unexpected partial valuation fields: %+v", valuation)
	}
	if valuation.TotalMarketCap != 123456789000 || valuation.FloatMarketCap != 98765432100 {
		t.Fatalf("unexpected partial market cap fields: %+v", valuation)
	}

	_, err = parseEastMoneyQuoteValuation(eastMoneyQuoteValuationResponse{
		RC:   0,
		Code: 0,
		Data: eastMoneyQuoteValuationData{
			TurnoverRate:   []byte(`"-"`),
			PE:             []byte(`null`),
			PB:             []byte(`"-"`),
			TotalMarketCap: []byte(`null`),
			FloatMarketCap: []byte(`"-"`),
		},
	})
	if err == nil {
		t.Fatal("expected fully missing valuation to return error")
	}
}

// TestSinaTencentProviderQuoteParsesSinaCNIndexQuote 验证首页指数代码能通过新浪实时行情接口读取。
func TestSinaTencentProviderQuoteParsesSinaCNIndexQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote" || request.URL.Query().Get("list") != "sh000001" {
			t.Fatalf("unexpected index quote request: %s", request.URL.String())
		}
		if request.URL.Query().Get("rn") != "" {
			t.Fatalf("sina index quote request must not append rn because Sina treats the full list value as symbol: %s", request.URL.String())
		}
		parts := []string{
			"上证指数", "4090.1001", "4106.2517", "4110.8134", "4117.2848", "4075.4921", "0", "0",
			"644527518", "1514193285000", "0", "0", "0", "0", "0", "0",
			"0", "0", "0", "0", "0", "0", "0", "0",
			"0", "0", "0", "0", "0", "0", "2026-06-24", "15:30:39",
		}
		_, _ = writer.Write(mustGB18030(t, `var hq_str_sh000001="`+strings.Join(parts, ",")+`";`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		QuoteURL: server.URL + "/quote",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:000001")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.Symbol.String() != "CN:SH:000001" || quote.Price != 4110.8134 || quote.PreClose != 4106.2517 {
		t.Fatalf("unexpected index quote: %+v", quote)
	}
	if quote.ChangeAmount < 4.56 || quote.ChangeAmount > 4.57 || quote.ChangePercent < 0.11 || quote.ChangePercent > 0.12 {
		t.Fatalf("unexpected index change fields: %+v", quote)
	}
}

// TestNewMarketCrawlerUsesDesktopUserAgent 验证行情 Provider 请求使用桌面 UA，避免外部数据源拒绝默认 Go UA。
func TestNewMarketCrawlerUsesDesktopUserAgent(t *testing.T) {
	original := randomDesktopUserAgent
	randomDesktopUserAgent = func() string {
		return "Mozilla/5.0 test desktop chrome"
	}
	defer func() {
		randomDesktopUserAgent = original
	}()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "Mozilla/5.0 test desktop chrome" {
			t.Fatalf("unexpected user agent: %s", request.Header.Get("User-Agent"))
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := newMarketCrawler("ua-test-source", time.Second, nil)
	if err != nil {
		t.Fatalf("newMarketCrawler returned error: %v", err)
	}
	if _, err := client.Fetch(context.Background(), crawler.Request{URL: server.URL}); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
}

// TestNewMarketCrawlerFallsBackDesktopUserAgent 验证随机 UA 生成失败时回退到固定桌面 Chrome UA。
func TestNewMarketCrawlerFallsBackDesktopUserAgent(t *testing.T) {
	original := randomDesktopUserAgent
	randomDesktopUserAgent = func() string {
		return ""
	}
	defer func() {
		randomDesktopUserAgent = original
	}()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != fallbackDesktopUserAgent {
			t.Fatalf("unexpected fallback user agent: %s", request.Header.Get("User-Agent"))
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := newMarketCrawler("ua-fallback-source", time.Second, nil)
	if err != nil {
		t.Fatalf("newMarketCrawler returned error: %v", err)
	}
	if _, err := client.Fetch(context.Background(), crawler.Request{URL: server.URL}); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
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

// TestTencentProviderDoesNotExposeSearch 验证腾讯数据源不承载新浪搜索职责。
func TestTencentProviderDoesNotExposeSearch(t *testing.T) {
	provider, err := NewTencentProvider(TencentConfig{})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
	}
	if _, ok := any(provider).(interface {
		Search(context.Context, string) ([]StockBasic, error)
	}); ok {
		t.Fatal("TencentProvider must not implement Search")
	}
}

// TestTencentProviderQuoteParsesTencentStockQuote 验证腾讯实时行情字段按 go-stock 口径归一化为 Quote。
func TestTencentProviderQuoteParsesTencentStockQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote" || request.URL.Query().Get("q") != "sz301217" {
			t.Fatalf("unexpected tencent quote request: %s", request.URL.String())
		}
		payload := `v_sz301217="51~铜冠铜箔~301217~170.30~183.27~175.00~574127~260165~313962~170.30~276~170.29~49~170.28~393~170.27~14~170.26~2~170.31~586~170.34~2~170.35~11~170.38~2~170.44~1~~20260626161451~-12.97~-7.08~176.00~160.96~170.30/574127/9679600142~574127~967960~6.93~859.58~~176.00~160.96~8.21~1411.81~1411.81~25.66~219.92~146.62~1.15~132~168.60~331.89~2253.50~~~3.12~967960.0142~161.7850~95~ AR~GP-A-CYB~396.79~-14.85~0.01~2.99~2.10~202.15~12.01~33.07~43.01~349.46~829015544~829015544~9.88~423.68~829015544~~~1267.87~-0.70~~CNY~0~~170.20~159~";`
		_, _ = writer.Write(mustGB18030(t, payload))
	}))
	defer server.Close()

	provider, err := NewTencentProvider(TencentConfig{
		QuoteURL: server.URL + "/quote",
	})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SZ:301217")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if quote.Symbol.String() != "CN:SZ:301217" || quote.Price != 170.30 || quote.PreClose != 183.27 {
		t.Fatalf("unexpected core quote fields: %+v", quote)
	}
	if quote.Volume != 57412700 || quote.Amount != 9679600142 {
		t.Fatalf("unexpected normalized turnover base fields: %+v", quote)
	}
	if quote.TurnoverRate != 6.93 || quote.TotalMarketCap != 141181000000 || quote.FloatMarketCap != 141181000000 {
		t.Fatalf("unexpected valuation fields: %+v", quote)
	}
	if quote.PE != 859.58 || quote.PB != 25.66 {
		t.Fatalf("unexpected PE/PB fields: %+v", quote)
	}
	if quote.QuoteTime.UTC() != time.Date(2026, 6, 26, 8, 14, 51, 0, time.UTC) {
		t.Fatalf("unexpected quote time: %s", quote.QuoteTime)
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

	provider, err := NewTencentProvider(TencentConfig{
		KlineURL: server.URL + "/kline",
	})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
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

	provider, err := NewTencentProvider(TencentConfig{
		KlineURL: server.URL + "/kline",
	})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
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

// TestSinaTencentProviderKlineParsesTencentIndexResponse 验证腾讯指数 K 线响应即使包含 qt 等非 K 线对象也能读取 day 数组。
func TestSinaTencentProviderKlineParsesTencentIndexResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/kline" || request.URL.Query().Get("param") != "sh000001,day,,,2,qfq" {
			t.Fatalf("unexpected index kline request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"sh000001":{"day":[["2026-06-23","4153.590","4106.250","4175.350","4085.590","709003913.000"],["2026-06-24","4090.100","4110.810","4117.280","4075.490","644527518.000"]],"qt":{"sh000001":["1","上证指数"]},"market":["SH_close_已收盘"],"version":"16"}}}`))
	}))
	defer server.Close()

	provider, err := NewTencentProvider(TencentConfig{
		KlineURL: server.URL + "/kline",
	})
	if err != nil {
		t.Fatalf("NewTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:000001")

	bars, err := provider.Kline(context.Background(), KlineRequest{
		Symbol: symbol,
		Period: PeriodDay,
		Adjust: AdjustForward,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 2 || bars[1].Close != 4110.810 || bars[1].Symbol.String() != "CN:SH:000001" {
		t.Fatalf("unexpected index bars: %+v", bars)
	}
}

// TestSinaTencentProviderMinuteKlineParsesTencentMinuteResponse 验证腾讯分时接口转换为当日 minute KlineBar。
func TestSinaTencentProviderMinuteKlineParsesTencentMinuteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/minute" || request.URL.Query().Get("code") != "sh600519" {
			t.Fatalf("unexpected minute request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"sh600519":{"data":{"date":"20260624","data":["0959 1810.50 123456 223456789","1000 1811.00 223456 323456789"]}}}}`))
	}))
	defer server.Close()

	provider, err := NewSinaTencentProvider(SinaTencentConfig{
		MinuteURL: server.URL + "/minute",
	})
	if err != nil {
		t.Fatalf("NewSinaTencentProvider returned error: %v", err)
	}
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{
		Symbol: symbol,
		Period: PeriodMinute,
		Adjust: AdjustNone,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) != 2 || bars[0].TradeDate != "2026-06-24 09:59" || bars[1].TradeDate != "2026-06-24 10:00" || bars[1].Close != 1811.00 {
		t.Fatalf("unexpected minute bars: %+v", bars)
	}
	if bars[0].Symbol.String() != "CN:SH:600519" || bars[0].Period != PeriodMinute || bars[0].Adjust != AdjustNone || bars[0].Amount != 223456789 {
		t.Fatalf("unexpected minute bar metadata: %+v", bars[0])
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

// TestEastMoneyPeriodSupportsGoStockAlignedPeriods 验证东财 klt 映射覆盖 go-stock 已使用的主要 K 线周期。
func TestEastMoneyPeriodSupportsGoStockAlignedPeriods(t *testing.T) {
	tests := []struct {
		period   Period
		expected string
	}{
		{period: Period1Minute, expected: "1"},
		{period: Period5Minute, expected: "5"},
		{period: Period15Minute, expected: "15"},
		{period: Period30Minute, expected: "30"},
		{period: Period60Minute, expected: "60"},
		{period: PeriodDay, expected: "101"},
		{period: PeriodWeek, expected: "102"},
		{period: PeriodMonth, expected: "103"},
		{period: PeriodQuarter, expected: "104"},
		{period: PeriodYear, expected: "106"},
	}

	for _, test := range tests {
		actual, err := eastMoneyPeriod(test.period)
		if err != nil {
			t.Fatalf("eastMoneyPeriod(%q) returned error: %v", test.period, err)
		}
		if actual != test.expected {
			t.Fatalf("eastMoneyPeriod(%q)=%q, expected %q", test.period, actual, test.expected)
		}
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

// TestCompositeMarketProviderPrefersTencentQuoteForCNStock 验证沪深个股实时行情优先走腾讯字段完整链路。
func TestCompositeMarketProviderPrefersTencentQuoteForCNStock(t *testing.T) {
	sina := &recordingSinaSource{
		quoteResult: Quote{Symbol: mustParseMarketSymbol(t, "CN:SZ:301217"), Price: 170.30},
	}
	tencent := &recordingTencentQuoteSource{
		quoteResult: Quote{
			Symbol:       mustParseMarketSymbol(t, "CN:SZ:301217"),
			Price:        170.30,
			TurnoverRate: 6.93,
		},
	}
	provider := NewCompositeMarketProvider("composite-test", sina, tencent, nil)
	symbol := mustParseMarketSymbol(t, "CN:SZ:301217")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if !tencent.quoteCalled || sina.quoteCalled {
		t.Fatalf("expected tencent quote only, sina=%+v tencent=%+v", sina, tencent)
	}
	if quote.Provider != "composite-test" || quote.TurnoverRate != 6.93 {
		t.Fatalf("unexpected composite quote: %+v", quote)
	}
}

// TestCompositeMarketProviderUsesSinaQuoteForCNIndex 验证指数行情继续使用新浪，避免腾讯个股字段解析污染指数。
func TestCompositeMarketProviderUsesSinaQuoteForCNIndex(t *testing.T) {
	sina := &recordingSinaSource{
		quoteResult: Quote{Symbol: mustParseMarketSymbol(t, "CN:SH:000001"), Price: 4042.86},
	}
	tencent := &recordingTencentQuoteSource{
		quoteResult: Quote{Symbol: mustParseMarketSymbol(t, "CN:SH:000001"), Price: 1},
	}
	provider := NewCompositeMarketProvider("composite-test", sina, tencent, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:000001")

	quote, err := provider.Quote(context.Background(), symbol)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if !sina.quoteCalled || tencent.quoteCalled {
		t.Fatalf("expected sina index quote only, sina=%+v tencent=%+v", sina, tencent)
	}
	if quote.Provider != "composite-test" || quote.Price != 4042.86 {
		t.Fatalf("unexpected index quote: %+v", quote)
	}
}

// TestCompositeMarketProviderFallsBackToEastMoneyKline 验证 TDX 不可用时组合 Provider 使用东财 K 线兜底。
func TestCompositeMarketProviderFallsBackToEastMoneyKline(t *testing.T) {
	sina := &recordingSinaSource{}
	tdx := &recordingTdxSource{klineErr: errors.New("tdx unavailable")}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	provider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tdx.klineCalled || !eastMoney.klineCalled || tencent.klineCalled {
		t.Fatalf("expected tdx then eastmoney calls, tdx=%+v eastMoney=%+v tencent=%+v", tdx, eastMoney, tencent)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1822 {
		t.Fatalf("unexpected fallback bars: %+v", bars)
	}
}

// TestCompositeMarketProviderDoesNotFallbackMinuteKline 验证分时走势不能用东财日 K 兜底，避免首页显示错误周期曲线。
func TestCompositeMarketProviderDoesNotFallbackMinuteKline(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{klineErr: errors.New("tencent minute unavailable")}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	provider := NewCompositeMarketProviderWithKlineFallback("composite-test", sina, tencent, eastMoney, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	_, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodMinute, Adjust: AdjustNone, Limit: 1})
	if err == nil {
		t.Fatal("expected minute kline error instead of daily fallback")
	}
	if !tencent.klineCalled || eastMoney.klineCalled {
		t.Fatalf("expected only tencent minute call, tencent=%+v eastMoney=%+v", tencent, eastMoney)
	}
}

// TestCompositeMarketProviderUsesTdxForMinuteKlinePeriods 验证分钟级 K 线优先使用通达信链路。
func TestCompositeMarketProviderUsesTdxForMinuteKlinePeriods(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: Period5Minute, Adjust: AdjustForward, TradeDate: "2026-06-19 10:30", Close: 1800}},
	}
	tdx := &recordingTdxSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: Period5Minute, Adjust: AdjustForward, TradeDate: "2026-06-19 10:30", Close: 1822}},
	}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: Period5Minute, Adjust: AdjustForward, TradeDate: "2026-06-19 10:30", Close: 1810}},
	}
	provider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: Period5Minute, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tdx.klineCalled || tencent.klineCalled || eastMoney.klineCalled {
		t.Fatalf("expected only tdx minute kline call, tdx=%+v tencent=%+v eastMoney=%+v", tdx, tencent, eastMoney)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1822 {
		t.Fatalf("unexpected tdx bars: %+v", bars)
	}
}

// TestCompositeMarketProviderFallsBackToEastMoneyForMinuteKline 验证通达信分钟 K 失败时使用东财分钟 K 兜底。
func TestCompositeMarketProviderFallsBackToEastMoneyForMinuteKline(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{}
	tdx := &recordingTdxSource{klineErr: errors.New("tdx unavailable")}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: Period15Minute, Adjust: AdjustForward, TradeDate: "2026-06-19 10:30", Close: 1810}},
	}
	provider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: Period15Minute, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tdx.klineCalled || tencent.klineCalled || !eastMoney.klineCalled {
		t.Fatalf("expected tdx then eastmoney minute kline calls, tdx=%+v tencent=%+v eastMoney=%+v", tdx, tencent, eastMoney)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1810 {
		t.Fatalf("unexpected fallback bars: %+v", bars)
	}
}

// TestSettingsBackedMarketProviderReadsDefaultSource 验证行情运行时会读取数据源默认行情源配置，并沿用自动降级链路。
func TestSettingsBackedMarketProviderReadsDefaultSource(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	tdx := &recordingTdxSource{klineErr: errors.New("tdx unavailable")}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	baseProvider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	store := &recordingMarketSettingsStore{
		settings: []model.Setting{{Key: settingsservice.SettingKeyDataSourceDefaultMarketSource, Value: settingsservice.DataSourceMarketSourceAutoFallback}},
	}
	provider := NewSettingsBackedMarketProvider(store, baseProvider)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(store.requestedKeys) != 1 || store.requestedKeys[0] != settingsservice.SettingKeyDataSourceDefaultMarketSource {
		t.Fatalf("expected provider to read default source setting, got %+v", store.requestedKeys)
	}
	if !tdx.klineCalled || !eastMoney.klineCalled || tencent.klineCalled {
		t.Fatalf("expected auto fallback kline calls, tdx=%+v eastMoney=%+v tencent=%+v", tdx, eastMoney, tencent)
	}
	if len(bars) != 1 || bars[0].Close != 1822 {
		t.Fatalf("unexpected bars: %+v", bars)
	}
}

// TestSettingsBackedMarketProviderPrefersTdxWhenConfigured 验证设置为通达信时，K 线优先走 TDX 源。
func TestSettingsBackedMarketProviderPrefersTdxWhenConfigured(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	tdx := &recordingTdxSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	eastMoney := &recordingEastMoneySource{}
	baseProvider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	store := &recordingMarketSettingsStore{
		settings: []model.Setting{{Key: settingsservice.SettingKeyDataSourceDefaultMarketSource, Value: settingsservice.DataSourceMarketSourceTdx}},
	}
	provider := NewSettingsBackedMarketProvider(store, baseProvider)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tdx.klineCalled || tencent.klineCalled || eastMoney.klineCalled {
		t.Fatalf("expected tdx preferred kline call, tdx=%+v tencent=%+v eastMoney=%+v", tdx, tencent, eastMoney)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1822 {
		t.Fatalf("unexpected tdx bars: %+v", bars)
	}
}

// TestSettingsBackedMarketProviderFallsBackWhenPreferredTdxFails 验证 TDX 不可用时不会让 K 线直接空白。
func TestSettingsBackedMarketProviderFallsBackWhenPreferredTdxFails(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	tdx := &recordingTdxSource{klineErr: errors.New("tdx unavailable")}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1818}},
	}
	baseProvider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	store := &recordingMarketSettingsStore{
		settings: []model.Setting{{Key: settingsservice.SettingKeyDataSourceDefaultMarketSource, Value: settingsservice.DataSourceMarketSourceTdx}},
	}
	provider := NewSettingsBackedMarketProvider(store, baseProvider)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if !tdx.klineCalled || !eastMoney.klineCalled || tencent.klineCalled {
		t.Fatalf("expected tdx failure to fall back to eastmoney before tencent, tdx=%+v eastMoney=%+v tencent=%+v", tdx, eastMoney, tencent)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1818 {
		t.Fatalf("unexpected fallback bars: %+v", bars)
	}
}

// TestSettingsBackedMarketProviderPrefersEastMoneyWhenConfigured 验证显式选择东财时不再先走自动 TDX 链路。
func TestSettingsBackedMarketProviderPrefersEastMoneyWhenConfigured(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	tdx := &recordingTdxSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-17", Close: 1790}},
	}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	baseProvider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	store := &recordingMarketSettingsStore{
		settings: []model.Setting{{Key: settingsservice.SettingKeyDataSourceDefaultMarketSource, Value: settingsservice.DataSourceMarketSourceEastMoney}},
	}
	provider := NewSettingsBackedMarketProvider(store, baseProvider)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if tdx.klineCalled || !eastMoney.klineCalled || tencent.klineCalled {
		t.Fatalf("expected eastmoney-only kline call, tdx=%+v eastMoney=%+v tencent=%+v", tdx, eastMoney, tencent)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1822 {
		t.Fatalf("unexpected eastmoney bars: %+v", bars)
	}
}

// TestSettingsBackedMarketProviderPrefersTencentWhenConfigured 验证显式选择腾讯时不再被自动 TDX/东财链路覆盖。
func TestSettingsBackedMarketProviderPrefersTencentWhenConfigured(t *testing.T) {
	sina := &recordingSinaSource{}
	tencent := &recordingTencentSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-18", Close: 1800}},
	}
	tdx := &recordingTdxSource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-17", Close: 1790}},
	}
	eastMoney := &recordingEastMoneySource{
		klineResult: []KlineBar{{Symbol: mustParseMarketSymbol(t, "CN:SH:600519"), Period: PeriodDay, Adjust: AdjustForward, TradeDate: "2026-06-19", Close: 1822}},
	}
	baseProvider := NewCompositeMarketProviderWithKlineFallbacks("composite-test", sina, tencent, eastMoney, tdx, nil)
	store := &recordingMarketSettingsStore{
		settings: []model.Setting{{Key: settingsservice.SettingKeyDataSourceDefaultMarketSource, Value: settingsservice.DataSourceMarketSourceTencent}},
	}
	provider := NewSettingsBackedMarketProvider(store, baseProvider)
	symbol := mustParseMarketSymbol(t, "CN:SH:600519")

	bars, err := provider.Kline(context.Background(), KlineRequest{Symbol: symbol, Period: PeriodDay, Adjust: AdjustForward, Limit: 1})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if tdx.klineCalled || eastMoney.klineCalled || !tencent.klineCalled {
		t.Fatalf("expected tencent-only kline call, tdx=%+v eastMoney=%+v tencent=%+v", tdx, eastMoney, tencent)
	}
	if len(bars) != 1 || bars[0].Provider != "composite-test" || bars[0].Close != 1800 {
		t.Fatalf("unexpected tencent bars: %+v", bars)
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

type recordingTencentQuoteSource struct {
	recordingTencentSource
	quoteCalled bool
	quoteResult Quote
	quoteErr    error
}

// Quote 记录腾讯实时行情调用并返回固定 quote。
func (source *recordingTencentQuoteSource) Quote(context.Context, stock.Symbol) (Quote, error) {
	source.quoteCalled = true
	if source.quoteErr != nil {
		return Quote{}, source.quoteErr
	}
	return source.quoteResult, nil
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

type recordingTdxSource struct {
	klineCalled bool
	klineResult []KlineBar
	klineErr    error
}

// Kline 记录通达信 K 线调用并返回固定 K 线。
func (source *recordingTdxSource) Kline(context.Context, KlineRequest) ([]KlineBar, error) {
	source.klineCalled = true
	if source.klineErr != nil {
		return nil, source.klineErr
	}
	return source.klineResult, nil
}

type recordingMarketSettingsStore struct {
	requestedKeys []string
	settings      []model.Setting
	err           error
}

// GetSettings 记录行情 Provider 读取的配置 key，并返回测试预置配置。
func (store *recordingMarketSettingsStore) GetSettings(_ context.Context, keys []string) ([]model.Setting, error) {
	store.requestedKeys = append(store.requestedKeys, keys...)
	if store.err != nil {
		return nil, store.err
	}
	return store.settings, nil
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
