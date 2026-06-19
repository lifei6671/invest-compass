package marketinfo

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestTencentGlobalIndexProviderFetchesSnapshot 验证腾讯全球指数接口会按区域清洗。
func TestTencentGlobalIndexProviderFetchesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ifzqgtimg/appstock/app/rank/indexRankDetail2" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{
			"data": {
				"common": [{"code":"DJI","name":"道琼斯","location":"美国","qtcode":"100.DJI","state":"close","zdf":"0.12","zxj":"39000"}],
				"asia": [{"code":"HSI","name":"恒生指数","location":"中国香港","qtcode":"100.HSI","state":"open","zdf":"-0.22","zxj":"18000"}]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestTencentGlobalIndexProvider(t, TencentGlobalIndexConfig{URL: server.URL + "/ifzqgtimg/appstock/app/rank/indexRankDetail2"})

	snapshot, err := provider.Fetch(context.Background(), GlobalIndexRequest{})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if snapshot.Provider != "tencent-global-index" || len(snapshot.Regions) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Regions["common"][0].Name != "道琼斯" || snapshot.Regions["asia"][0].StateText() != "开盘" {
		t.Fatalf("unexpected index rows: %+v", snapshot.Regions)
	}
	md := snapshot.Markdown()
	if !strings.Contains(md, "全球主要指数概览") || !strings.Contains(md, "道琼斯") {
		t.Fatalf("unexpected markdown: %s", md)
	}
}

// TestTencentGlobalIndexProviderRejectsRemoteError 验证腾讯全球指数远端异常码不会被当成空快照。
func TestTencentGlobalIndexProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":500,"msg":"remote failed","data":{}}`))
	}))
	defer server.Close()

	provider := newTestTencentGlobalIndexProvider(t, TencentGlobalIndexConfig{URL: server.URL})

	_, err := provider.Fetch(context.Background(), GlobalIndexRequest{})
	if err == nil {
		t.Fatal("expected remote code error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestTencentGlobalIndexProviderRejectsEmptyData 验证远端结构漂移或空区域不会被包装成成功快照。
func TestTencentGlobalIndexProviderRejectsEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"data":{"common":[{"code":"","name":""}]}}`))
	}))
	defer server.Close()

	provider := newTestTencentGlobalIndexProvider(t, TencentGlobalIndexConfig{URL: server.URL})

	_, err := provider.Fetch(context.Background(), GlobalIndexRequest{})
	if err == nil {
		t.Fatal("expected empty data error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "empty_data" {
		t.Fatalf("expected empty_data ProviderError, got %T %[1]v", err)
	}
}

// TestGlobalIndexMarkdownEscapesCells 验证全球指数 Markdown 不会被远端竖线或换行破坏表格结构。
func TestGlobalIndexMarkdownEscapesCells(t *testing.T) {
	snapshot := GlobalIndexSnapshot{
		Regions: map[string][]GlobalIndex{
			"common": {
				{
					Code:          "DJI",
					Name:          "道|琼斯",
					Location:      "美国\n纽约",
					LastPrice:     "39000|12",
					ChangePercent: "0.12",
					State:         "open",
				},
			},
		},
	}

	markdown := snapshot.Markdown()
	for _, want := range []string{`道\|琼斯`, `美国<br>纽约`, `39000\|12`} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain escaped cell %q, got:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "美国\n纽约") {
		t.Fatalf("markdown kept raw newline inside table cell:\n%s", markdown)
	}
}

// TestCLSMarketStatisticProviderFetchesSnapshot 验证财联社市场统计会清洗涨跌分布和情绪指标。
func TestCLSMarketStatisticProviderFetchesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote/index/home" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("app") != "CailianpressWeb" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"code": 200,
			"msg": "ok",
			"data": {
				"index_quote": [
					{"secu_code":"sh000001","secu_name":"上证指数","last_px":3188.22,"change":0.66,"change_px":20.88,"up_num":921,"down_num":459,"flat_num":20},
					{"secu_code":"sz399001","secu_name":"深证成指","last_px":10500.12,"change":1.12,"change_px":116.20,"up_num":1420,"down_num":710,"flat_num":40}
				],
				"up_down_dis": {
					"up_num": 82,
					"down_num": 13,
					"average_rise": 1.18,
					"rise_num": 3200,
					"fall_num": 1500,
					"down_10": 13,
					"down_8": 21,
					"down_6": 34,
					"down_4": 89,
					"down_2": 210,
					"flat_num": 180,
					"up_2": 890,
					"up_4": 420,
					"up_6": 160,
					"up_8": 92,
					"up_10": 82,
					"suspend_num": 11,
					"status": true
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestCLSMarketStatisticProvider(t, CLSMarketStatisticConfig{URL: server.URL + "/quote/index/home"})

	snapshot, err := provider.FetchMarketStatistic(context.Background(), MarketStatisticRequest{})
	if err != nil {
		t.Fatalf("FetchMarketStatistic returned error: %v", err)
	}
	if snapshot.Provider != "cls-market-statistic" || snapshot.Source == "" {
		t.Fatalf("unexpected metadata: %+v", snapshot)
	}
	if snapshot.UpCount != 3200 || snapshot.DownCount != 1500 || snapshot.LimitUp != 82 || snapshot.LimitDown != 13 {
		t.Fatalf("unexpected counts: %+v", snapshot)
	}
	if !closeEnough(snapshot.UpRatio, 68.08510638297872) || !closeEnough(snapshot.UpDownRatio, 2.1333333333333333) || !closeEnough(snapshot.LimitRatio, 6.3076923076923075) {
		t.Fatalf("unexpected ratios: %+v", snapshot)
	}
	if snapshot.SentimentDesc != "普涨(极强)" {
		t.Fatalf("unexpected sentiment: %s", snapshot.SentimentDesc)
	}
	if snapshot.ShUpCount != 921 || snapshot.ShDownCount != 459 || snapshot.SzUpCount != 1420 || snapshot.SzDownCount != 710 {
		t.Fatalf("unexpected index breadth: %+v", snapshot)
	}
	if snapshot.Distribution.Up8 != 92 || snapshot.Distribution.Down4 != 89 || snapshot.Distribution.SuspendNum != 11 {
		t.Fatalf("unexpected distribution: %+v", snapshot.Distribution)
	}
}

// TestCLSMarketStatisticProviderRejectsRemoteError 验证财联社异常码不会被当成有效统计。
func TestCLSMarketStatisticProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code": 500, "msg": "remote failed"}`))
	}))
	defer server.Close()

	provider := newTestCLSMarketStatisticProvider(t, CLSMarketStatisticConfig{URL: server.URL})

	_, err := provider.FetchMarketStatistic(context.Background(), MarketStatisticRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestCLSMarketStatisticProviderRejectsEmptyData 验证财联社成功码下缺少核心指数时不会生成全 0 伪快照。
func TestCLSMarketStatisticProviderRejectsEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code": 200, "data": {}}`))
	}))
	defer server.Close()

	provider := newTestCLSMarketStatisticProvider(t, CLSMarketStatisticConfig{URL: server.URL})

	_, err := provider.FetchMarketStatistic(context.Background(), MarketStatisticRequest{})
	if err == nil {
		t.Fatal("expected empty market statistic error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "empty_data" {
		t.Fatalf("expected empty_data ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyMutualTop10ProviderFetchesSnapshot 验证东财互联互通十大成交数据会清洗为统一快照。
func TestEastMoneyMutualTop10ProviderFetchesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/web/api/data/v1/get" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("reportName") != "RPT_MUTUAL_TOP10DEAL" || query.Get("pageNumber") != "2" || query.Get("pageSize") != "5" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		filter, _ := url.QueryUnescape(query.Get("filter"))
		if !strings.Contains(filter, `MUTUAL_TYPE="001"`) || !strings.Contains(filter, `TRADE_DATE='2026-06-18'`) {
			t.Fatalf("unexpected filter: %s", filter)
		}
		_, _ = writer.Write([]byte(`data({
			"success": true,
			"result": {
				"pages": 1,
				"count": 1,
				"data": [
					{
						"SECURITY_CODE": "600519",
						"SECURITY_NAME_ABBR": "贵州茅台",
						"SECUCODE": "600519.SH",
						"TRADE_DATE": "2026-06-18 00:00:00",
						"MUTUAL_TYPE": "001",
						"RANK": 1,
						"CHANGE_RATE": 1.23,
						"CLOSE_PRICE": 1600.5,
						"NET_BUY_AMT": 123456789,
						"BUY_AMT": 222222222,
						"SELL_AMT": 98765433,
						"DEAL_AMT": 320987655
					}
				]
			}
		});`))
	}))
	defer server.Close()

	provider := newTestEastMoneyMutualTop10Provider(t, EastMoneyMutualTop10Config{URL: server.URL + "/web/api/data/v1/get"})

	snapshot, err := provider.FetchMutualTop10(context.Background(), MutualTop10Request{
		MutualType: "001",
		TradeDate:  "2026-06-18",
		Page:       2,
		PageSize:   5,
	})
	if err != nil {
		t.Fatalf("FetchMutualTop10 returned error: %v", err)
	}

	if snapshot.Total != 1 || snapshot.Pages != 1 || snapshot.Provider != "eastmoney-mutual-top10" {
		t.Fatalf("unexpected metadata: %+v", snapshot)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one item, got %+v", snapshot.Items)
	}
	item := snapshot.Items[0]
	if item.Symbol != "600519.SH" || item.Name != "贵州茅台" || item.Rank != 1 {
		t.Fatalf("unexpected item identity: %+v", item)
	}
	if item.NetBuyAmount != 123456789 || item.BuyAmount != 222222222 || item.SellAmount != 98765433 {
		t.Fatalf("unexpected item amounts: %+v", item)
	}
}

// TestEastMoneyMutualTop10ProviderRejectsUnsafeFilterInput 验证外部参数不能改写东财 filter 表达式。
func TestEastMoneyMutualTop10ProviderRejectsUnsafeFilterInput(t *testing.T) {
	provider := newTestEastMoneyMutualTop10Provider(t, EastMoneyMutualTop10Config{})

	tests := []MutualTop10Request{
		{MutualType: `001")(TRADE_DATE='2026-06-18'`, TradeDate: "2026-06-18"},
		{MutualType: "001", TradeDate: `2026-06-18')(MUTUAL_TYPE="003"`},
	}
	for _, request := range tests {
		_, err := provider.FetchMutualTop10(context.Background(), request)
		if err == nil {
			t.Fatalf("expected unsafe filter input error for %+v", request)
		}
		var providerError *ProviderError
		if !errors.As(err, &providerError) || providerError.Operation != "request" {
			t.Fatalf("expected request ProviderError, got %T %[1]v", err)
		}
	}
}

// TestEastMoneyMutualTop10ProviderRejectsEmptyValidRows 验证字段漂移时不把空有效行当作成功。
func TestEastMoneyMutualTop10ProviderRejectsEmptyValidRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`data({
			"success": true,
			"result": {
				"pages": 1,
				"count": 1,
				"data": [{"SECURITY_NAME_ABBR": "贵州茅台"}]
			}
		});`))
	}))
	defer server.Close()

	provider := newTestEastMoneyMutualTop10Provider(t, EastMoneyMutualTop10Config{URL: server.URL})

	_, err := provider.FetchMutualTop10(context.Background(), MutualTop10Request{MutualType: "001", TradeDate: "2026-06-18"})
	if err == nil {
		t.Fatal("expected empty valid rows error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "empty_data" {
		t.Fatalf("expected empty_data ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyStockScreenerProviderFetchesStocks 验证东财条件选股列表会编码筛选条件并清洗行情字段。
func TestEastMoneyStockScreenerProviderFetchesStocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/dataapi/xuangu/list" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("p") != "3" || query.Get("ps") != "20" || query.Get("source") != "SELECT_SECURITIES" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		filter, _ := url.QueryUnescape(query.Get("filter"))
		if !strings.Contains(filter, `SECURITY_NAME_ABBR in ("贵州茅台")`) || !strings.Contains(filter, `(UPP_DAYS>=3)`) {
			t.Fatalf("unexpected filter: %s", filter)
		}
		_, _ = writer.Write([]byte(`{
			"success": true,
			"result": {
				"count": 1,
				"pages": 1,
				"data": [
					{
						"SECUCODE": "600519.SH",
						"SECURITY_CODE": "600519",
						"SECURITY_NAME_ABBR": "贵州茅台",
						"NEW_PRICE": 1600.5,
						"CHANGE_RATE": 1.23,
						"VOLUME_RATIO": 1.8,
						"HIGH_PRICE": 1620,
						"LOW_PRICE": 1580,
						"PRE_CLOSE_PRICE": 1581.05,
						"VOLUME": 123456,
						"DEAL_AMOUNT": 987654321,
						"TURNOVERRATE": 0.56,
						"MARKET": "上交所主板",
						"CONCEPT": "白酒概念",
						"INDUSTRY": "酿酒行业"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestEastMoneyStockScreenerProvider(t, EastMoneyStockScreenerConfig{URL: server.URL + "/dataapi/xuangu/list"})

	snapshot, err := provider.FetchStocks(context.Background(), StockScreenerRequest{
		Page:     3,
		PageSize: 20,
		Keyword:  "贵州茅台",
		Conditions: []ScreenerCondition{
			{Field: "UPP_DAYS", Operator: ">=", Value: "3"},
		},
	})
	if err != nil {
		t.Fatalf("FetchStocks returned error: %v", err)
	}

	if snapshot.Total != 1 || snapshot.Provider != "eastmoney-stock-screener" {
		t.Fatalf("unexpected metadata: %+v", snapshot)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one item, got %+v", snapshot.Items)
	}
	item := snapshot.Items[0]
	if item.Symbol != "600519.SH" || item.Name != "贵州茅台" || item.Industry != "酿酒行业" || item.Concept != "白酒概念" {
		t.Fatalf("unexpected item identity: %+v", item)
	}
	if item.Price != 1600.5 || item.ChangeRate != 1.23 || item.TurnoverRate != 0.56 {
		t.Fatalf("unexpected quote values: %+v", item)
	}
}

// TestEastMoneyStockScreenerProviderAllowsEmptyResult 验证合法空结果不会被当成抓取失败。
func TestEastMoneyStockScreenerProviderAllowsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"success": true,
			"result": {
				"count": 0,
				"pages": 0,
				"data": []
			}
		}`))
	}))
	defer server.Close()

	provider := newTestEastMoneyStockScreenerProvider(t, EastMoneyStockScreenerConfig{URL: server.URL})

	snapshot, err := provider.FetchStocks(context.Background(), StockScreenerRequest{Keyword: "不存在的股票"})
	if err != nil {
		t.Fatalf("FetchStocks returned error: %v", err)
	}
	if snapshot.Total != 0 || snapshot.Pages != 0 || len(snapshot.Items) != 0 {
		t.Fatalf("unexpected empty snapshot: %+v", snapshot)
	}
}

// TestEastMoneyStockScreenerProviderRejectsRemoteError 验证条件选股远端失败不会被当成空数据。
func TestEastMoneyStockScreenerProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"success": false, "message": "failed", "result": {"count": 0, "data": []}}`))
	}))
	defer server.Close()

	provider := newTestEastMoneyStockScreenerProvider(t, EastMoneyStockScreenerConfig{URL: server.URL})

	_, err := provider.FetchStocks(context.Background(), StockScreenerRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyStockScreenerProviderRejectsUnsafeConditionField 验证条件字段不能直接拼接成非预期 filter。
func TestEastMoneyStockScreenerProviderRejectsUnsafeConditionField(t *testing.T) {
	_, err := buildStockScreenerFilter(StockScreenerRequest{
		Conditions: []ScreenerCondition{
			{Field: `UPP_DAYS)(SECURITY_NAME_ABBR`, Operator: "=", Value: `"贵州茅台"`},
		},
	})
	if err == nil {
		t.Fatal("expected unsafe condition field error")
	}
}

// TestEastMoneyStockScreenerProviderRejectsUnsafeConditionValue 验证条件值不能直接拼接成非预期 filter。
func TestEastMoneyStockScreenerProviderRejectsUnsafeConditionValue(t *testing.T) {
	_, err := buildStockScreenerFilter(StockScreenerRequest{
		Conditions: []ScreenerCondition{
			{Field: "UPP_DAYS", Operator: ">=", Value: `3)(SECURITY_NAME_ABBR in ("贵州茅台")`},
		},
	})
	if err == nil {
		t.Fatal("expected unsafe condition value error")
	}
}

// TestEastMoneyStockScreenerProviderRejectsUnsafeKeyword 验证关键词不能闭合字符串后拼接额外筛选表达式。
func TestEastMoneyStockScreenerProviderRejectsUnsafeKeyword(t *testing.T) {
	_, err := buildStockScreenerFilter(StockScreenerRequest{
		Keyword: `贵州茅台")(UPP_DAYS>=3)`,
	})
	if err == nil {
		t.Fatal("expected unsafe keyword error")
	}
}

// TestEastMoneySmartScreenerProviderRequiresFingerprint 验证东财自然语言筛选缺少用户标识时快速失败。
func TestEastMoneySmartScreenerProviderRequiresFingerprint(t *testing.T) {
	provider := newTestEastMoneySmartScreenerProvider(t, EastMoneySmartScreenerConfig{})

	_, err := provider.Fetch(context.Background(), SmartScreenerRequest{Kind: SmartScreenerKindStock, Query: "涨幅前十"})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "credential" {
		t.Fatalf("expected credential ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneySmartScreenerProviderFetchesStocks 验证自然语言选股请求体使用注入凭据并清洗列标题。
func TestEastMoneySmartScreenerProviderFetchesStocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/smart-tag/stock/v3/pw/search-code" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["keyWord"] != `量比大于2，"非ST"` || body["fingerprint"] != "user-qgqp" || body["pageSize"].(float64) != 20 {
			t.Fatalf("unexpected body: %+v", body)
		}
		if !strings.Contains(request.Header.Get("Referer"), "xuangu.eastmoney.com") {
			t.Fatalf("unexpected referer: %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{
			"code": 100,
			"message": "ok",
			"data": {
				"result": {
					"dataList": [
						{"SECURITY_CODE":"600519","SECURITY_NAME_ABBR":"贵州茅台","CHANGE_RATE":"1.23","NEW_PRICE":1600.5}
					],
					"columns": [
						{"key":"SECURITY_CODE","title":"代码"},
						{"key":"SECURITY_NAME_ABBR","title":"名称"},
						{"key":"CHANGE_RATE","title":"涨跌幅","unit":"%"},
						{"key":"NEW_PRICE","title":"最新价","dateMsg":"2026-06-18"}
					],
					"count": 1
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestEastMoneySmartScreenerProvider(t, EastMoneySmartScreenerConfig{
		StockURL:    server.URL + "/api/smart-tag/stock/v3/pw/search-code",
		Fingerprint: "user-qgqp",
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
		},
	})

	snapshot, err := provider.Fetch(context.Background(), SmartScreenerRequest{
		Kind:     SmartScreenerKindStock,
		Query:    `量比大于2，"非ST"`,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if snapshot.Provider != "eastmoney-smart-screener" || snapshot.Kind != SmartScreenerKindStock || snapshot.Total != 1 {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if len(snapshot.Columns) != 4 || snapshot.Columns[2].Title != "涨跌幅(%)" || snapshot.Columns[3].Title != "最新价[2026-06-18]" {
		t.Fatalf("unexpected columns: %+v", snapshot.Columns)
	}
	if len(snapshot.Rows) != 1 || snapshot.Rows[0]["名称"] != "贵州茅台" || snapshot.Rows[0]["最新价[2026-06-18]"] != "1600.5" {
		t.Fatalf("unexpected rows: %+v", snapshot.Rows)
	}
}

// TestEastMoneySmartScreenerProviderRoutesKinds 验证股票、板块和 ETF 使用不同 smart-tag endpoint。
func TestEastMoneySmartScreenerProviderRoutesKinds(t *testing.T) {
	seenPaths := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seenPaths[request.URL.Path] = true
		_, _ = writer.Write([]byte(`{"code":100,"data":{"result":{"dataList":[{"name":"x"}],"columns":[{"key":"name","title":"名称"}],"count":1}}}`))
	}))
	defer server.Close()
	provider := newTestEastMoneySmartScreenerProvider(t, EastMoneySmartScreenerConfig{
		StockURL:    server.URL + "/stock",
		BoardURL:    server.URL + "/board",
		ETFURL:      server.URL + "/etf",
		Fingerprint: "user-qgqp",
	})

	for _, kind := range []SmartScreenerKind{SmartScreenerKindStock, SmartScreenerKindBoard, SmartScreenerKindETF} {
		if _, err := provider.Fetch(context.Background(), SmartScreenerRequest{Kind: kind, Query: "今日涨幅前5"}); err != nil {
			t.Fatalf("Fetch(%s) returned error: %v", kind, err)
		}
	}
	if !seenPaths["/stock"] || !seenPaths["/board"] || !seenPaths["/etf"] {
		t.Fatalf("unexpected paths: %+v", seenPaths)
	}
}

// TestEastMoneySmartScreenerProviderFetchesHotStrategies 验证热门策略会清洗涨幅百分比和排序字段。
func TestEastMoneySmartScreenerProviderFetchesHotStrategies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/recommend/stock/heat/ranking" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("count") != "2" || request.URL.Query().Get("client") != "web" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"code": 0,
			"data": [
				{"rank":1,"question":"近期放量上涨","code":"A","market":"沪深A股","heatValue":980,"chg":0.1234},
				{"rank":2,"question":"低估值高股息","code":"B","market":"沪深A股","heatValue":870,"chg":-0.015}
			]
		}`))
	}))
	defer server.Close()
	provider := newTestEastMoneySmartScreenerProvider(t, EastMoneySmartScreenerConfig{
		HotStrategyURL: server.URL + "/recommend/stock/heat/ranking",
		Fingerprint:    "user-qgqp",
	})

	snapshot, err := provider.FetchHotStrategies(context.Background(), HotStrategyRequest{Count: 2})
	if err != nil {
		t.Fatalf("FetchHotStrategies returned error: %v", err)
	}
	if snapshot.Provider != "eastmoney-smart-screener" || len(snapshot.Items) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Items[0].Question != "近期放量上涨" || snapshot.Items[0].ChangePercent != 12.34 || snapshot.Items[1].ChangePercent != -1.5 {
		t.Fatalf("unexpected items: %+v", snapshot.Items)
	}
}

// TestWallstreetcnMarketProviderFetchesGlobalQuotes 验证华尔街见闻全球行情会清洗成稳定报价模型。
func TestWallstreetcnMarketProviderFetchesGlobalQuotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/market/real" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("prod_code") != "XAUUSD.OTC,DXY.OTC" {
			t.Fatalf("unexpected prod_code: %s", request.URL.RawQuery)
		}
		if !strings.Contains(query.Get("fields"), "prod_name,last_px,px_change,px_change_rate,price_precision,securities_type") {
			t.Fatalf("unexpected fields: %s", query.Get("fields"))
		}
		if !strings.Contains(request.Header.Get("Referer"), "wallstreetcn.com") {
			t.Fatalf("expected wallstreetcn referer, got %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["prod_name", "last_px", "px_change", "px_change_rate", "price_precision", "securities_type"],
				"snapshot": {
					"XAUUSD.OTC": ["现货黄金", 2330.12, 10.5, 0.45, 2, "commodity"],
					"DXY.OTC": ["美元指数", "105.55", "-0.12", "-0.11", 3, "index"]
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{RealURL: server.URL + "/market/real"})

	snapshot, err := provider.FetchGlobalQuotes(context.Background(), GlobalQuoteRequest{ProdCodes: []string{"XAUUSD.OTC", "DXY.OTC"}})
	if err != nil {
		t.Fatalf("FetchGlobalQuotes returned error: %v", err)
	}
	if snapshot.Provider != "wallstreetcn-market" || len(snapshot.Items) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Items[0].Code != "XAUUSD.OTC" || snapshot.Items[0].Name != "现货黄金" || snapshot.Items[0].LastPrice != 2330.12 {
		t.Fatalf("unexpected first quote: %+v", snapshot.Items[0])
	}
	if snapshot.Items[1].Code != "DXY.OTC" || snapshot.Items[1].Precision != 3 || snapshot.Items[1].ChangePercent != -0.11 {
		t.Fatalf("unexpected second quote: %+v", snapshot.Items[1])
	}
}

// TestWallstreetcnMarketProviderRejectsInvalidQuoteNumber 验证远端坏报价字段不会被静默清洗成 0。
func TestWallstreetcnMarketProviderRejectsInvalidQuoteNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["prod_name", "last_px", "px_change", "px_change_rate", "price_precision", "securities_type"],
				"snapshot": {
					"XAUUSD.OTC": ["现货黄金", "bad-price", 10.5, 0.45, 2, "commodity"]
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{RealURL: server.URL})

	_, err := provider.FetchGlobalQuotes(context.Background(), GlobalQuoteRequest{ProdCodes: []string{"XAUUSD.OTC"}})
	if err == nil {
		t.Fatal("expected invalid quote number error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "quote_parse" {
		t.Fatalf("expected quote_parse ProviderError, got %T %[1]v", err)
	}
}

// TestWallstreetcnMarketProviderFetchesKline 验证华尔街见闻 K 线字段按响应 fields 映射，避免依赖固定下标。
func TestWallstreetcnMarketProviderFetchesKline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/market/kline" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("prod_code") != "XAUUSD.OTC" || query.Get("period_type") != "300" || query.Get("tick_count") != "2" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["tick_at", "open_px", "close_px", "high_px", "low_px"],
				"candle": {
					"XAUUSD.OTC": {
						"lines": [
							[1781805600, 2320.1, 2325.2, 2330.3, 2318.4],
							[1781805900, 2325.2, 2326.5, 2331.0, 2320.0]
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{KlineURL: server.URL + "/market/kline"})

	snapshot, err := provider.FetchKline(context.Background(), GlobalKlineRequest{
		ProdCode:      "XAUUSD.OTC",
		PeriodSeconds: 300,
		TickCount:     2,
	})
	if err != nil {
		t.Fatalf("FetchKline returned error: %v", err)
	}
	if snapshot.Provider != "wallstreetcn-market" || len(snapshot.Items) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	first := snapshot.Items[0]
	if first.At.Format(time.RFC3339) != "2026-06-18T18:00:00Z" || first.Open != 2320.1 || first.Close != 2325.2 || first.High != 2330.3 || first.Low != 2318.4 {
		t.Fatalf("unexpected first kline: %+v", first)
	}
}

// TestWallstreetcnMarketProviderRejectsInvalidKlineNumber 验证远端坏 K 线字段不会被静默清洗成 0。
func TestWallstreetcnMarketProviderRejectsInvalidKlineNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["tick_at", "open_px", "close_px", "high_px", "low_px"],
				"candle": {
					"XAUUSD.OTC": {
						"lines": [
							[1781805600, 2320.1, "bad-close", 2330.3, 2318.4]
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{KlineURL: server.URL})

	_, err := provider.FetchKline(context.Background(), GlobalKlineRequest{ProdCode: "XAUUSD.OTC"})
	if err == nil {
		t.Fatal("expected invalid kline number error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "kline_parse" {
		t.Fatalf("expected kline_parse ProviderError, got %T %[1]v", err)
	}
}

// TestWallstreetcnMarketProviderRejectsMissingRequiredKlineField 验证缺少必要 OHLC 字段时不会静默补 0。
func TestWallstreetcnMarketProviderRejectsMissingRequiredKlineField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["tick_at", "open_px", "high_px", "low_px"],
				"candle": {
					"XAUUSD.OTC": {
						"lines": [
							[1781805600, 2320.1, 2330.3, 2318.4]
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{KlineURL: server.URL})

	_, err := provider.FetchKline(context.Background(), GlobalKlineRequest{ProdCode: "XAUUSD.OTC"})
	if err == nil {
		t.Fatal("expected missing kline field error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "kline_parse" {
		t.Fatalf("expected kline_parse ProviderError, got %T %[1]v", err)
	}
}

// TestWallstreetcnMarketProviderRejectsEmptyKline 验证远端成功码下缺少有效 K 线时显式失败。
func TestWallstreetcnMarketProviderRejectsEmptyKline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{
			"code": 20000,
			"data": {
				"fields": ["tick_at", "open_px", "close_px", "high_px", "low_px"],
				"candle": {
					"XAUUSD.OTC": {
						"lines": [[0, 2320.1, 2325.2, 2330.3, 2318.4]]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestWallstreetcnMarketProvider(t, WallstreetcnMarketConfig{KlineURL: server.URL})

	_, err := provider.FetchKline(context.Background(), GlobalKlineRequest{ProdCode: "XAUUSD.OTC"})
	if err == nil {
		t.Fatal("expected empty kline error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "kline_parse" {
		t.Fatalf("expected kline_parse ProviderError, got %T %[1]v", err)
	}
}

// newTestTencentGlobalIndexProvider 创建测试用全球指数 Provider。
func newTestTencentGlobalIndexProvider(t *testing.T, config TencentGlobalIndexConfig) *TencentGlobalIndexProvider {
	t.Helper()
	provider, err := NewTencentGlobalIndexProvider(config)
	if err != nil {
		t.Fatalf("NewTencentGlobalIndexProvider returned error: %v", err)
	}
	return provider
}

// newTestCLSMarketStatisticProvider 创建测试用财联社市场统计 Provider。
func newTestCLSMarketStatisticProvider(t *testing.T, config CLSMarketStatisticConfig) *CLSMarketStatisticProvider {
	t.Helper()
	provider, err := NewCLSMarketStatisticProvider(config)
	if err != nil {
		t.Fatalf("NewCLSMarketStatisticProvider returned error: %v", err)
	}
	return provider
}

// newTestEastMoneyMutualTop10Provider 创建测试用东财互联互通十大成交 Provider。
func newTestEastMoneyMutualTop10Provider(t *testing.T, config EastMoneyMutualTop10Config) *EastMoneyMutualTop10Provider {
	t.Helper()
	provider, err := NewEastMoneyMutualTop10Provider(config)
	if err != nil {
		t.Fatalf("NewEastMoneyMutualTop10Provider returned error: %v", err)
	}
	return provider
}

// newTestEastMoneyStockScreenerProvider 创建测试用东财条件选股 Provider。
func newTestEastMoneyStockScreenerProvider(t *testing.T, config EastMoneyStockScreenerConfig) *EastMoneyStockScreenerProvider {
	t.Helper()
	provider, err := NewEastMoneyStockScreenerProvider(config)
	if err != nil {
		t.Fatalf("NewEastMoneyStockScreenerProvider returned error: %v", err)
	}
	return provider
}

// newTestEastMoneySmartScreenerProvider 创建测试用东财自然语言筛选 Provider。
func newTestEastMoneySmartScreenerProvider(t *testing.T, config EastMoneySmartScreenerConfig) *EastMoneySmartScreenerProvider {
	t.Helper()
	provider, err := NewEastMoneySmartScreenerProvider(config)
	if err != nil {
		t.Fatalf("NewEastMoneySmartScreenerProvider returned error: %v", err)
	}
	return provider
}

// newTestWallstreetcnMarketProvider 创建测试用华尔街见闻市场 Provider。
func newTestWallstreetcnMarketProvider(t *testing.T, config WallstreetcnMarketConfig) *WallstreetcnMarketProvider {
	t.Helper()
	provider, err := NewWallstreetcnMarketProvider(config)
	if err != nil {
		t.Fatalf("NewWallstreetcnMarketProvider returned error: %v", err)
	}
	return provider
}

// closeEnough 判断浮点计算结果是否在可接受误差内。
func closeEnough(left float64, right float64) bool {
	return math.Abs(left-right) < 0.000001
}
