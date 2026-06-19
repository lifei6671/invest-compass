package macro

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEastMoneyProviderFetchesGDP 验证东财宏观 JSONP 会被清洗成统一指标数据。
func TestEastMoneyProviderFetchesGDP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/data/v1/get" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("reportName") != "RPT_ECONOMY_GDP" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`data({
			"result": {
				"data": [
					{"REPORT_DATE":"2026-03-31 00:00:00","TIME":"2026Q1","DOMESTICL_PRODUCT_BASE":"318758","SUM_SAME":"5.3"}
				]
			}
		})`))
	}))
	defer server.Close()

	provider := newTestEastMoneyMacroProvider(t, EastMoneyConfig{URL: server.URL + "/api/data/v1/get"})

	dataset, err := provider.Fetch(context.Background(), Request{Indicator: IndicatorGDP, Limit: 20})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if dataset.Indicator != IndicatorGDP || len(dataset.Rows) != 1 {
		t.Fatalf("unexpected dataset: %+v", dataset)
	}
	if dataset.Rows[0].Period != "2026Q1" || dataset.Rows[0].Values["DOMESTICL_PRODUCT_BASE"] != "318758" {
		t.Fatalf("unexpected row: %+v", dataset.Rows[0])
	}
}

// TestEastMoneyProviderRejectsRemoteError 验证东财宏观远端错误不会被当成空数据。
func TestEastMoneyProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`data({"success":false,"code":500,"message":"remote failed"});`))
	}))
	defer server.Close()

	provider := newTestEastMoneyMacroProvider(t, EastMoneyConfig{URL: server.URL})

	_, err := provider.Fetch(context.Background(), Request{Indicator: IndicatorGDP, Limit: 20})
	if err == nil {
		t.Fatal("expected remote error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "remote_code" {
		t.Fatalf("expected remote_code ProviderError, got %T %[1]v", err)
	}
}

// TestEastMoneyProviderRejectsMissingMacroRows 验证缺失宏观结果集时显式失败。
func TestEastMoneyProviderRejectsMissingMacroRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`data({"success":true,"code":0,"result":{"data":[]}});`))
	}))
	defer server.Close()

	provider := newTestEastMoneyMacroProvider(t, EastMoneyConfig{URL: server.URL})

	_, err := provider.Fetch(context.Background(), Request{Indicator: IndicatorGDP, Limit: 20})
	if err == nil {
		t.Fatal("expected empty data error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Operation != "empty_data" {
		t.Fatalf("expected empty_data ProviderError, got %T %[1]v", err)
	}
}

// newTestEastMoneyMacroProvider 创建测试用东财宏观 Provider。
func newTestEastMoneyMacroProvider(t *testing.T, config EastMoneyConfig) *EastMoneyProvider {
	t.Helper()
	provider, err := NewEastMoneyProvider(config)
	if err != nil {
		t.Fatalf("NewEastMoneyProvider returned error: %v", err)
	}
	return provider
}
