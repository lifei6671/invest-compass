package fund

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestEastMoneyProviderSearchFunds 验证基金搜索接口会清洗东财 suggest 响应。
func TestEastMoneyProviderSearchFunds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/FundSearch/api/FundSearchAPI.ashx" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("key") != "沪深300" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"Datas": [
				{"CODE":"000300","NAME":"沪深300ETF联接A","FundBaseInfo":{"FCODE":"000300","SHORTNAME":"沪深300ETF联接A","FTYPE":"指数型"}},
				{"CODE":"broken","NAME":"无基础信息"}
			]
		}`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		SearchURL: server.URL + "/FundSearch/api/FundSearchAPI.ashx",
	})

	items, err := provider.SearchFunds(context.Background(), SearchRequest{Keyword: " 沪深300 ", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFunds returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}
	if items[0].Code != "000300" || items[0].Name != "沪深300ETF联接A" || items[0].Type != "指数型" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

// TestEastMoneyProviderFetchBasic 验证基金基础资料页面会被清洗成结构化字段。
func TestEastMoneyProviderFetchBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/110022.html" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`
			<html>
				<body>
					<div class="fundDetail-tit">易方达消费行业股票 查看相关ETF&gt;</div>
					<div class="infoOfFund">
						<table>
							<tr><td>基金类型：股票型</td><td>成立日期：2010-08-20</td><td>基金规模：320.12亿元</td></tr>
							<tr><td>管理人：易方达基金</td><td>基金经理：张坤</td><td>基金评级：五星</td></tr>
						</table>
					</div>
					<div class="dataOfFund">
						<dl>
							<dd>近1月：1.20%</dd>
							<dd>近3月：3.40%</dd>
							<dd>今年来：6.70%</dd>
						</dl>
					</div>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		BasicURLPattern: server.URL + "/%s.html",
	})

	basic, err := provider.FetchBasic(context.Background(), BasicRequest{Code: "110022"})
	if err != nil {
		t.Fatalf("FetchBasic returned error: %v", err)
	}
	if basic.Code != "110022" || basic.Name != "易方达消费行业股票" || basic.Type != "股票型" {
		t.Fatalf("unexpected basic fields: %+v", basic)
	}
	if basic.Company != "易方达基金" || basic.Manager != "张坤" || basic.Rating != "五星" {
		t.Fatalf("unexpected manager fields: %+v", basic)
	}
	if basic.Month1Growth == nil || *basic.Month1Growth != 1.2 || basic.YTDGrowth == nil || *basic.YTDGrowth != 6.7 {
		t.Fatalf("unexpected growth fields: %+v", basic)
	}
}

// TestEastMoneyProviderFetchHistoryNetValues 验证历史净值接口会转换数值字段并保留申赎状态。
func TestEastMoneyProviderFetchHistoryNetValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/f10/lsjz" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("fundCode") != "110022" || request.URL.Query().Get("pageSize") != "2" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{
			"Data": {
				"TotalCount": 2,
				"LSJZList": [
					{"FSRQ":"2026-06-18","DWJZ":"1.2345","LJJZ":"2.3456","JZZZL":"0.68","SGZT":"开放申购","SHZT":"开放赎回"},
					{"FSRQ":"2026-06-17","DWJZ":"1.2262","LJJZ":"2.3373","JZZZL":"-0.11","SGZT":"暂停申购","SHZT":"开放赎回"}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		HistoryURL: server.URL + "/f10/lsjz",
	})

	values, err := provider.FetchHistoryNetValues(context.Background(), HistoryRequest{Code: "110022", PageIndex: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("FetchHistoryNetValues returned error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected two values, got %+v", values)
	}
	if values[0].Date != "2026-06-18" || values[0].NetValue != 1.2345 || values[0].AccumulatedValue != 2.3456 || values[0].DailyGrowth != 0.68 {
		t.Fatalf("unexpected first value: %+v", values[0])
	}
	if values[1].BuyStatus != "暂停申购" || values[1].SellStatus != "开放赎回" {
		t.Fatalf("unexpected status fields: %+v", values[1])
	}
}

// TestEastMoneyProviderRejectsInvalidHistoryNetValues 验证历史净值核心字段缺失时不会静默置零。
func TestEastMoneyProviderRejectsInvalidHistoryNetValues(t *testing.T) {
	tests := []struct {
		name string
		row  string
	}{
		{
			name: "missing net value",
			row:  `{"FSRQ":"2026-06-18","DWJZ":"--","LJJZ":"2.3456","JZZZL":"0.68","SGZT":"开放申购","SHZT":"开放赎回"}`,
		},
		{
			name: "missing daily growth",
			row:  `{"FSRQ":"2026-06-18","DWJZ":"1.2345","LJJZ":"2.3456","JZZZL":"--","SGZT":"开放申购","SHZT":"开放赎回"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(`{"Data":{"LSJZList":[` + tt.row + `]}}`))
			}))
			defer server.Close()

			provider := newTestEastMoneyProvider(t, EastMoneyConfig{
				HistoryURL: server.URL,
			})

			_, err := provider.FetchHistoryNetValues(context.Background(), HistoryRequest{Code: "110022", PageIndex: 1, PageSize: 1})
			if err == nil {
				t.Fatal("expected invalid history net value error")
			}
			var providerError *ProviderError
			if !errors.As(err, &providerError) {
				t.Fatalf("expected ProviderError, got %T", err)
			}
			if providerError.Operation != "history_decode" {
				t.Fatalf("unexpected provider error: %+v", providerError)
			}
		})
	}
}

// TestEastMoneyProviderFetchRanking 验证基金排行伪 JS 响应会被清洗成结构化排行。
func TestEastMoneyProviderFetchRanking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/data/rankhandler.aspx" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("dt") != "kf" || request.URL.Query().Get("sc") != "jnzf" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`var rankData={datas:["000001,华夏成长,huaxia,2026-06-18,1.1000,3.2000,0.10,0.20,1.20,3.40,5.60,8.90,12.30,18.40,6.70,120.50,2001-12-18,1,123.45,0.15,0.10"],allRecords:1,allPages:1};`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		RankingURL: server.URL + "/data/rankhandler.aspx",
	})

	result, err := provider.FetchRanking(context.Background(), RankingRequest{})
	if err != nil {
		t.Fatalf("FetchRanking returned error: %v", err)
	}
	if result.TotalCount != 1 || result.TotalPages != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	item := result.Items[0]
	if item.Code != "000001" || item.Name != "华夏成长" || item.NetUnitValue == nil || *item.NetUnitValue != 1.1 {
		t.Fatalf("unexpected ranking item: %+v", item)
	}
	if !item.Purchasable || item.Scale == nil || *item.Scale != 123.45 {
		t.Fatalf("unexpected extended fields: %+v", item)
	}
}

// TestEastMoneyProviderRejectsMalformedRankingRows 验证排行 schema 异常时不会静默返回空排行。
func TestEastMoneyProviderRejectsMalformedRankingRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`var rankData={datas:["000001,华夏成长,too-short"],allRecords:1,allPages:1};`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		RankingURL: server.URL,
	})

	_, err := provider.FetchRanking(context.Background(), RankingRequest{})
	if err == nil {
		t.Fatal("expected malformed ranking error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "ranking_decode" {
		t.Fatalf("expected ranking_decode ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyProviderFetchTopHoldings 验证基金十大持仓接口只清洗持仓本身，不重复抓股票行情。
func TestEastMoneyProviderFetchTopHoldings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/FundArchivesDatas.aspx" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`var apidata={content:"<div>2026年03月31日</div><table class='tzpgtab'><tbody><tr><td>1</td><td><a href='//quote.eastmoney.com/sh600519.html'>600519</a></td><td>贵州茅台</td><td>1500.00</td><td>1.23%</td><td></td><td>9.88%</td><td>100.00万股</td><td>15.00亿元</td></tr></tbody></table>",arryear:[]};`))
	}))
	defer server.Close()

	provider := newTestEastMoneyProvider(t, EastMoneyConfig{
		HoldingURL: server.URL + "/FundArchivesDatas.aspx",
	})

	holdings, err := provider.FetchTopHoldings(context.Background(), HoldingRequest{Code: "110022"})
	if err != nil {
		t.Fatalf("FetchTopHoldings returned error: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("expected one holding, got %+v", holdings)
	}
	holding := holdings[0]
	if holding.Rank != 1 || holding.StockCode != "600519" || holding.StockName != "贵州茅台" || holding.Ratio != 9.88 {
		t.Fatalf("unexpected holding: %+v", holding)
	}
	if holding.Price == nil || *holding.Price != 1500 || holding.ChangeRate == nil || *holding.ChangeRate != 1.23 {
		t.Fatalf("unexpected quote snapshot fields: %+v", holding)
	}
	if holding.Quarter != "2026年03月31日" || holding.Market != "SH" {
		t.Fatalf("unexpected metadata: %+v", holding)
	}
}

// TestEastMoneyProviderRejectsInvalidFundCode 验证 Provider 在外部边界拒绝非法基金代码。
func TestEastMoneyProviderRejectsInvalidFundCode(t *testing.T) {
	provider := newTestEastMoneyProvider(t, EastMoneyConfig{})

	_, err := provider.FetchHistoryNetValues(context.Background(), HistoryRequest{Code: "abc"})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "validate" {
		t.Fatalf("expected validate ProviderError, got %T %[1]v", err)
	}
}

// newTestEastMoneyProvider 创建测试用东财基金 Provider。
func newTestEastMoneyProvider(t *testing.T, config EastMoneyConfig) *EastMoneyProvider {
	t.Helper()
	config.Now = func() time.Time {
		return time.Date(2026, 6, 19, 10, 30, 0, 0, time.UTC)
	}
	provider, err := NewEastMoneyProvider(config)
	if err != nil {
		t.Fatalf("NewEastMoneyProvider returned error: %v", err)
	}
	return provider
}
