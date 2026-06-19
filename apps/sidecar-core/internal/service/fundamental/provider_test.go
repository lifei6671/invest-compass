package fundamental

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestProviderStatusDescribesComplianceBoundary 验证基本面 Provider 状态必须说明来源、授权边界和频率限制。
func TestProviderStatusDescribesComplianceBoundary(t *testing.T) {
	status := ProviderStatus{
		Name:          "example-fundamental",
		Source:        "用户配置的授权基本面数据源",
		License:       "由用户自行确认数据授权",
		RateLimit:     "60 requests/minute",
		Available:     true,
		LastCheckedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
	}

	if status.Name == "" || status.Source == "" || status.License == "" || status.RateLimit == "" {
		t.Fatalf("provider status must include compliance metadata: %+v", status)
	}
}

// TestEastMoneyF10ProviderFetchesLatestFinance 验证东财 F10 Provider 通过规格化参数获取最新财务数据。
func TestEastMoneyF10ProviderFetchesLatestFinance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if request.URL.Path != "/securities/api/data/v1/get" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if query.Get("reportName") != "RPT_PCF10_FINANCEMAINFINADATA" {
			t.Fatalf("unexpected reportName: %s", query.Get("reportName"))
		}
		if !strings.Contains(query.Get("filter"), `SECUCODE="600519.SH"`) {
			t.Fatalf("unexpected filter: %s", query.Get("filter"))
		}
		if request.Header.Get("Referer") != "https://emweb.securities.eastmoney.com/" {
			t.Fatalf("unexpected referer: %s", request.Header.Get("Referer"))
		}
		_, _ = writer.Write([]byte(`{
			"version":"1.0",
			"success":true,
			"code":0,
			"result":{
				"count":1,
				"data":[{
					"SECUCODE":"600519.SH",
					"SECURITY_CODE":"600519",
					"SECURITY_NAME_ABBR":"贵州茅台",
					"REPORT_DATE":"2026-03-31 00:00:00",
					"EPSJB":12.34,
					"TOTAL_OPERATEINCOME":12345678900,
					"ROEJQ":21.3456
				}]
			}
		}`))
	}))
	defer server.Close()

	provider, err := NewEastMoneyF10Provider(EastMoneyF10Config{
		BaseURL: server.URL + "/securities/api/data/v1/get",
	})
	if err != nil {
		t.Fatalf("NewEastMoneyF10Provider returned error: %v", err)
	}

	dataset, err := provider.Fetch(context.Background(), Request{
		Symbol: mustParseSymbol(t, "CN:SH:600519"),
		Kind:   ReportLatestFinance,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if dataset.Kind != ReportLatestFinance || dataset.Symbol.String() != "CN:SH:600519" || len(dataset.Rows) != 1 {
		t.Fatalf("unexpected dataset metadata: %+v", dataset)
	}
	if dataset.Rows[0]["SECURITY_NAME_ABBR"] != "贵州茅台" || dataset.Rows[0]["EPSJB"] != float64(12.34) {
		t.Fatalf("unexpected dataset row: %+v", dataset.Rows[0])
	}
}

// TestMarkdownRendererFormatsAndHidesFields 验证 Markdown 输出使用中文字段名、隐藏技术字段并格式化金额和百分比。
func TestMarkdownRendererFormatsAndHidesFields(t *testing.T) {
	dataset := Dataset{
		Title:  "最新财务主要数据",
		Kind:   ReportLatestFinance,
		Symbol: mustParseSymbol(t, "CN:SH:600519"),
		Columns: []Column{
			{Key: "SECUCODE", Label: "证券代码", Hidden: true},
			{Key: "SECURITY_NAME_ABBR", Label: "股票简称"},
			{Key: "TOTAL_OPERATEINCOME", Label: "营业总收入", Format: FormatMoney},
			{Key: "ROEJQ", Label: "ROE(加权)", Format: FormatPercent},
			{Key: "REPORT_DATE", Label: "报告日期", Format: FormatDate},
		},
		Rows: []Row{{
			"SECUCODE":            "600519.SH",
			"SECURITY_NAME_ABBR":  "贵州茅台",
			"TOTAL_OPERATEINCOME": float64(12345678900),
			"ROEJQ":               float64(21.3456),
			"REPORT_DATE":         "2026-03-31 00:00:00",
		}},
	}

	markdown := RenderMarkdown(dataset)
	if strings.Contains(markdown, "600519.SH") || strings.Contains(markdown, "SECUCODE") {
		t.Fatalf("markdown leaked hidden field: %s", markdown)
	}
	for _, expected := range []string{"## 最新财务主要数据", "| 股票简称 | 贵州茅台 |", "| 营业总收入 | 123.46亿 |", "| ROE(加权) | 21.35% |", "| 报告日期 | 2026-03-31 |"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("markdown missing %q:\n%s", expected, markdown)
		}
	}
}

// TestMarkdownRendererKeepsMissingNumericValuesAsEmpty 验证远端占位符不会被误渲染成真实 0。
func TestMarkdownRendererKeepsMissingNumericValuesAsEmpty(t *testing.T) {
	dataset := Dataset{
		Title:  "最新财务主要数据",
		Kind:   ReportLatestFinance,
		Symbol: mustParseSymbol(t, "CN:SH:600519"),
		Columns: []Column{
			{Key: "TOTAL_OPERATEINCOME", Label: "营业总收入", Format: FormatMoney},
			{Key: "ROEJQ", Label: "ROE(加权)", Format: FormatPercent},
			{Key: "EPSJB", Label: "基本每股收益", Format: FormatPrice},
		},
		Rows: []Row{{
			"TOTAL_OPERATEINCOME": "--",
			"ROEJQ":               "-",
			"EPSJB":               "",
		}},
	}

	markdown := RenderMarkdown(dataset)
	for _, forbidden := range []string{"0.00", "0.00%"} {
		if strings.Contains(markdown, forbidden) {
			t.Fatalf("markdown rendered missing numeric value as zero:\n%s", markdown)
		}
	}
	for _, expected := range []string{"| 营业总收入 | - |", "| ROE(加权) | - |", "| 基本每股收益 | - |"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("markdown missing %q:\n%s", expected, markdown)
		}
	}
}

// TestMarkdownRendererEscapesTableCells 验证远端文本不会通过竖线或换行破坏 Markdown 表格结构。
func TestMarkdownRendererEscapesTableCells(t *testing.T) {
	dataset := Dataset{
		Title:  "最新财务主要数据",
		Kind:   ReportLatestFinance,
		Symbol: mustParseSymbol(t, "CN:SH:600519"),
		Columns: []Column{
			{Key: "SECURITY_NAME_ABBR", Label: "股票|简称"},
			{Key: "MAIN_BUSINESS", Label: "主营业务"},
		},
		Rows: []Row{{
			"SECURITY_NAME_ABBR": "贵州|茅台",
			"MAIN_BUSINESS":      "白酒\n销售",
		}},
	}

	markdown := RenderMarkdown(dataset)
	for _, expected := range []string{`股票\|简称`, `贵州\|茅台`, `白酒<br>销售`} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("markdown missing escaped cell %q:\n%s", expected, markdown)
		}
	}
	if strings.Contains(markdown, "白酒\n销售") {
		t.Fatalf("markdown kept raw newline inside table cell:\n%s", markdown)
	}
}

// TestEastMoneyF10ProviderRejectsUnsupportedSymbol 验证基本面 Provider 不把未支持市场伪装成空数据。
func TestEastMoneyF10ProviderRejectsUnsupportedSymbol(t *testing.T) {
	provider, err := NewEastMoneyF10Provider(EastMoneyF10Config{})
	if err != nil {
		t.Fatalf("NewEastMoneyF10Provider returned error: %v", err)
	}
	_, err = provider.Fetch(context.Background(), Request{
		Symbol: mustParseSymbol(t, "US:AAPL"),
		Kind:   ReportLatestFinance,
	})
	if err == nil {
		t.Fatal("expected unsupported symbol error")
	}
}

// mustParseSymbol 解析测试用标准股票代码。
func mustParseSymbol(t *testing.T, value string) stock.Symbol {
	t.Helper()
	symbol, err := stock.ParseSymbol(value)
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	return symbol
}
