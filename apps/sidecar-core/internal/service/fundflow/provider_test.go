package fundflow

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestEastMoneyConceptProviderFetchesSnapshot 验证东财概念资金接口能转换成统一资金流快照。
func TestEastMoneyConceptProviderFetchesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/dataapi/bkzj/getbkzj" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("key") != "f62" || query.Get("code") != "m:90+t:3" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		if !strings.Contains(request.Header.Get("Referer"), "data.eastmoney.com") {
			t.Fatalf("expected eastmoney referer, got %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{
			"rc": 0,
			"data": {
				"total": 2,
				"diff": [
					{"f12":"BK0815","f13":90,"f14":"华为概念","f62":123456789.12},
					{"f12":"BK1027","f13":90,"f14":"算力概念","f62":-9876543}
				]
			}
		}`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyConceptProvider(EastMoneyConceptConfig{
		BaseURL: server.URL + "/dataapi/bkzj/getbkzj",
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewEastMoneyConceptProvider returned error: %v", err)
	}

	snapshot, err := provider.FetchSnapshot(context.Background(), Request{Scope: ScopeConcept, Limit: 10})
	if err != nil {
		t.Fatalf("FetchSnapshot returned error: %v", err)
	}

	if snapshot.Scope != ScopeConcept || snapshot.Provider != "eastmoney-concept-fundflow" {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("expected two items, got %+v", snapshot.Items)
	}
	if snapshot.Items[0].Code != "BK0815" || snapshot.Items[0].Name != "华为概念" || snapshot.Items[0].MarketID != 90 {
		t.Fatalf("unexpected first item: %+v", snapshot.Items[0])
	}
	if snapshot.Items[0].NetInflow != 123456789.12 || snapshot.Items[1].NetInflow != -9876543 {
		t.Fatalf("unexpected net inflow values: %+v", snapshot.Items)
	}

	top := snapshot.TopN(1)
	if len(top) != 1 || top[0].Code != "BK0815" {
		t.Fatalf("unexpected top item: %+v", top)
	}
	status := provider.Status(context.Background())
	if !status.Available || status.Name != "eastmoney-concept-fundflow" || status.LastCheckedAt.IsZero() {
		t.Fatalf("unexpected status: %+v", status)
	}
}

// TestEastMoneyConceptProviderRejectsRemoteError 验证远端异常码会作为 Provider 错误返回。
func TestEastMoneyConceptProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"rc": 100, "data": {"total": 0, "diff": []}}`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyConceptProvider(EastMoneyConceptConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewEastMoneyConceptProvider returned error: %v", err)
	}

	_, err = provider.FetchSnapshot(context.Background(), Request{Scope: ScopeConcept})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Provider != "eastmoney-concept-fundflow" || providerError.Operation != "remote_code" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}

// TestEastMoneyConceptProviderRejectsEmptyDiff 验证东财资金流缺少有效榜单时显式失败。
func TestEastMoneyConceptProviderRejectsEmptyDiff(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty diff", body: `{"rc":0,"data":{"total":0,"diff":[]}}`},
		{name: "invalid rows", body: `{"rc":0,"data":{"total":1,"diff":[{"f12":"","f13":90,"f14":"","f62":1}]}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(tt.body))
			}))
			defer server.Close()

			provider, err := NewEastMoneyConceptProvider(EastMoneyConceptConfig{BaseURL: server.URL})
			if err != nil {
				t.Fatalf("NewEastMoneyConceptProvider returned error: %v", err)
			}

			_, err = provider.FetchSnapshot(context.Background(), Request{Scope: ScopeConcept})
			if err == nil {
				t.Fatal("expected empty data error")
			}
			var providerError *ProviderError
			if !errors.As(err, &providerError) {
				t.Fatalf("expected ProviderError, got %T", err)
			}
			if providerError.Operation != "empty_data" {
				t.Fatalf("unexpected provider error: %+v", providerError)
			}
		})
	}
}

// TestEastMoneyConceptProviderRejectsUnsupportedScope 验证概念 Provider 不伪装支持行业板块资金流。
func TestEastMoneyConceptProviderRejectsUnsupportedScope(t *testing.T) {
	provider, err := NewEastMoneyConceptProvider(EastMoneyConceptConfig{})
	if err != nil {
		t.Fatalf("NewEastMoneyConceptProvider returned error: %v", err)
	}

	_, err = provider.FetchSnapshot(context.Background(), Request{Scope: ScopeIndustry})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Operation != "scope" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}

// TestEastMoneyStockFundFlowProviderFetchesRanking 验证东财个股资金流榜单能清洗为统一快照。
func TestEastMoneyStockFundFlowProviderFetchesRanking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/qt/clist/get" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("fid") != "f62" || query.Get("pn") != "2" || query.Get("pz") != "3" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		if query.Get("fs") == "" || !strings.Contains(query.Get("fields"), "f62") {
			t.Fatalf("missing eastmoney fundflow query: %s", request.URL.RawQuery)
		}
		if !strings.Contains(request.Header.Get("Referer"), "quote.eastmoney.com") {
			t.Fatalf("expected eastmoney referer, got %q", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`data({
			"rc": 0,
			"data": {
				"total": 1,
				"diff": [
					{
						"f12": "600519",
						"f13": 1,
						"f14": "贵州茅台",
						"f2": 1600.5,
						"f3": 1.23,
						"f62": 123456789,
						"f184": 6.78,
						"f66": 111,
						"f69": 1.1,
						"f72": 222,
						"f75": 2.2,
						"f78": -333,
						"f81": -3.3,
						"f84": -444,
						"f87": -4.4,
						"f100": "白酒"
					}
				]
			}
		});`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyStockFundFlowProvider(EastMoneyStockFundFlowConfig{
		RankingURL: server.URL + "/api/qt/clist/get",
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewEastMoneyStockFundFlowProvider returned error: %v", err)
	}

	snapshot, err := provider.FetchRanking(context.Background(), StockFundFlowRankingRequest{Page: 2, PageSize: 3})
	if err != nil {
		t.Fatalf("FetchRanking returned error: %v", err)
	}

	if snapshot.Total != 1 || snapshot.Provider != "eastmoney-stock-fundflow" {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one item, got %+v", snapshot.Items)
	}
	item := snapshot.Items[0]
	if item.Symbol != "600519.SH" || item.Name != "贵州茅台" || item.Industry != "白酒" {
		t.Fatalf("unexpected item identity: %+v", item)
	}
	if item.MainNetInflow != 123456789 || item.MainNetInflowRatio != 6.78 || item.SmallNetInflow != -444 {
		t.Fatalf("unexpected item fundflow values: %+v", item)
	}
}

// TestEastMoneyStockFundFlowProviderFetchesHistory 验证东财个股历史资金流 K 线能按字段顺序清洗。
func TestEastMoneyStockFundFlowProviderFetchesHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/qt/stock/fflow/daykline/get" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query, _ := url.ParseQuery(request.URL.RawQuery)
		if query.Get("secid") != "1.600519" || query.Get("lmt") != "5" || query.Get("klt") != "101" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`data({
			"rc": 0,
			"data": {
				"klines": [
					"2026-06-18,100,10,20,30,40,1,0.1,0.2,0.3,0.4,1600,1.5",
					"2026-06-19,-50,-5,-10,-15,-20,-0.5,-0.1,-0.2,-0.3,-0.4,1590,-0.6"
				]
			}
		});`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyStockFundFlowProvider(EastMoneyStockFundFlowConfig{
		HistoryURL: server.URL + "/api/qt/stock/fflow/daykline/get",
	})
	if err != nil {
		t.Fatalf("NewEastMoneyStockFundFlowProvider returned error: %v", err)
	}

	history, err := provider.FetchHistory(context.Background(), StockFundFlowHistoryRequest{Symbol: "600519.SH", Limit: 5})
	if err != nil {
		t.Fatalf("FetchHistory returned error: %v", err)
	}

	if history.Symbol != "600519.SH" || len(history.Items) != 2 {
		t.Fatalf("unexpected history: %+v", history)
	}
	first := history.Items[0]
	if first.Date != "2026-06-18" || first.MainNetInflow != 100 || first.Close != 1600 || first.ChangePercent != 1.5 {
		t.Fatalf("unexpected first history item: %+v", first)
	}
	if first.SuperLargeNetInflow != 40 || first.MainNetInflowRatio != 1 {
		t.Fatalf("unexpected detailed history item: %+v", first)
	}
}

// TestEastMoneyStockFundFlowProviderRejectsInvalidHistoryValue 验证历史资金流核心数值异常时不会静默置零。
func TestEastMoneyStockFundFlowProviderRejectsInvalidHistoryValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`data({
			"rc": 0,
			"data": {
				"klines": [
					"2026-06-18,not-a-number,10,20,30,40,1,0.1,0.2,0.3,0.4,1600,1.5"
				]
			}
		});`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyStockFundFlowProvider(EastMoneyStockFundFlowConfig{
		HistoryURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewEastMoneyStockFundFlowProvider returned error: %v", err)
	}

	_, err = provider.FetchHistory(context.Background(), StockFundFlowHistoryRequest{Symbol: "600519.SH", Limit: 5})
	if err == nil {
		t.Fatal("expected invalid history value error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "decode" {
		t.Fatalf("expected decode ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyStockFundFlowProviderRejectsRemoteError 验证东财个股资金流远端异常码会显式失败。
func TestEastMoneyStockFundFlowProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`data({"rc": 100, "data": {"total": 0, "diff": []}});`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyStockFundFlowProvider(EastMoneyStockFundFlowConfig{RankingURL: server.URL})
	if err != nil {
		t.Fatalf("NewEastMoneyStockFundFlowProvider returned error: %v", err)
	}

	_, err = provider.FetchRanking(context.Background(), StockFundFlowRankingRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Provider != "eastmoney-stock-fundflow" || providerError.Operation != "remote_code" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}
