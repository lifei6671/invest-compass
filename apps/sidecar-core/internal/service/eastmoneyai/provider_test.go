package eastmoneyai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestProviderRecognizesEntityAndSendsSafeHeaders 验证实体识别请求会携带必要鉴权头并解析标准实体。
func TestProviderRecognizesEntityAndSendsSafeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/proxy/entity/dialogTagsV2" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("em_api_key") != "test-key" {
			t.Fatalf("missing em api key header: %q", request.Header.Get("em_api_key"))
		}
		if !strings.Contains(request.Header.Get("em_base_info"), `"productType":"mx"`) {
			t.Fatalf("missing em base info: %q", request.Header.Get("em_base_info"))
		}
		_, _ = writer.Write([]byte(`{
			"code": 0,
			"status": 0,
			"data": {
				"entityMetricList": [[{
					"classCode": "002001",
					"secuCode": "600519",
					"marketChar": "SH",
					"shortName": "贵州茅台"
				}]]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	entity, err := provider.RecognizeEntity(context.Background(), "贵州茅台")
	if err != nil {
		t.Fatalf("RecognizeEntity returned error: %v", err)
	}
	if entity.Code != "600519" || entity.Exchange != "SH" || entity.Name != "贵州茅台" || entity.EastMoneyCode != "600519.SH" {
		t.Fatalf("unexpected entity: %+v", entity)
	}
}

// TestProviderFetchesFinancialQA 验证金融问答会保留回答正文和引用元信息。
func TestProviderFetchesFinancialQA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/proxy/app-robo-advisor-api/assistant/ask" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{
			"code": 200,
			"data": {
				"displayData": "这是回答",
				"refIndexList": [
					{"refId":1,"type":"资讯","referenceType":"CITED_REFERENCE","title":"新闻标题","jumpUrl":"https://example.com/news","source":"东方财富"}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	answer, err := provider.FinancialQA(context.Background(), QARequest{Question: "茅台怎么样", DeepThink: true})
	if err != nil {
		t.Fatalf("FinancialQA returned error: %v", err)
	}
	if answer.Answer != "这是回答" || len(answer.References) != 1 {
		t.Fatalf("unexpected QA result: %+v", answer)
	}
	if answer.References[0].Title != "新闻标题" || answer.References[0].Source != "东方财富" {
		t.Fatalf("unexpected reference: %+v", answer.References[0])
	}
}

// TestArticleResultMarkdownRejectsUnsafeShareURL 验证文章分享链接只允许安全的 http/https 链接。
func TestArticleResultMarkdownRejectsUnsafeShareURL(t *testing.T) {
	result := ArticleResult{
		Title:    "报告标题",
		Content:  "报告正文",
		ShareURL: "javascript:alert(1)",
	}

	markdown := result.Markdown()
	if strings.Contains(markdown, "[查看原文]") || strings.Contains(markdown, "javascript:") {
		t.Fatalf("markdown rendered unsafe share url:\n%s", markdown)
	}
	if !strings.Contains(markdown, "报告正文") {
		t.Fatalf("markdown dropped article content:\n%s", markdown)
	}
}

// TestArticleResultMarkdownEscapesShareURLDestination 验证分享链接不会通过右括号逃逸 Markdown link 目标。
func TestArticleResultMarkdownEscapesShareURLDestination(t *testing.T) {
	result := ArticleResult{
		Title:    "报告标题",
		Content:  "报告正文",
		ShareURL: "https://example.com/report)extra",
	}

	markdown := result.Markdown()
	if !strings.Contains(markdown, `https://example.com/report\)extra`) {
		t.Fatalf("markdown did not escape link destination:\n%s", markdown)
	}
}

// TestProviderRendersFinanceDataMarkdown 验证金融数据查询结果能转换为稳定 Markdown 表格。
func TestProviderRendersFinanceDataMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/proxy/b/mcp/tool/searchData" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{
			"code": 0,
			"status": 0,
			"data": {
				"searchDataResultDTO": {
					"dataTableDTOList": [{
						"title": "主要财务指标",
						"entityName": "贵州茅台",
						"nameMap": {"eps": "每股收益", "roe": "净资产收益率"},
						"indicatorOrder": ["eps", "roe"],
						"table": {
							"headName": ["2025Q4", "2026Q1"],
							"eps": [10.5, 12.3],
							"roe": ["30%", "32%"]
						},
						"condition": "报告期最近两期"
					}]
				}
			}
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	result, err := provider.FinanceDataQuery(context.Background(), "贵州茅台 EPS")
	if err != nil {
		t.Fatalf("FinanceDataQuery returned error: %v", err)
	}
	markdown := result.Markdown()
	for _, want := range []string{"### 主要财务指标", "| 贵州茅台 | 2025Q4 | 2026Q1 |", "| 每股收益 | 10.5 | 12.3 |", "报告期最近两期"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", want, markdown)
		}
	}
}

// TestDataTableMarkdownEscapesCells 验证远端表头、指标名和单元格不会破坏 Markdown 表格结构。
func TestDataTableMarkdownEscapesCells(t *testing.T) {
	table := DataTable{
		EntityName:     "贵州|茅台",
		NameMap:        map[string]any{"eps": "每股\n收益"},
		IndicatorOrder: []any{"eps"},
		Table: map[string]any{
			"headName": []any{"2026|Q1"},
			"eps":      []any{"12|3\n元"},
		},
	}

	markdown := table.Markdown()
	for _, want := range []string{`贵州\|茅台`, `2026\|Q1`, `每股<br>收益`, `12\|3<br>元`} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain escaped cell %q, got:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "12|3\n元") {
		t.Fatalf("markdown kept raw table-breaking cell:\n%s", markdown)
	}
}

// TestGenericTableMarkdownEscapesCells 验证泛型表格同样会转义远端字段。
func TestGenericTableMarkdownEscapesCells(t *testing.T) {
	markdown := genericTableToMarkdown(map[string]any{
		"name|col": []any{"贵州|茅台"},
		"desc":     []any{"白酒\n销售"},
	}, map[string]any{"name|col": "名称|列"})

	for _, want := range []string{`名称\|列`, `贵州\|茅台`, `白酒<br>销售`} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain escaped cell %q, got:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "白酒\n销售") {
		t.Fatalf("markdown kept raw newline inside table cell:\n%s", markdown)
	}
}

// TestProviderRejectsEmptyFinanceDataTables 验证金融数据接口缺少表格时显式失败，避免把解析漂移渲染成无数据。
func TestProviderRejectsEmptyFinanceDataTables(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty table list",
			body: `{"code":0,"status":0,"data":{"searchDataResultDTO":{"dataTableDTOList":[]}}}`,
		},
		{
			name: "empty renderable table",
			body: `{"code":0,"status":0,"data":{"searchDataResultDTO":{"dataTableDTOList":[{"title":"空表","entityName":"贵州茅台","table":{}}]}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/proxy/b/mcp/tool/searchData" {
					t.Fatalf("unexpected path: %s", request.URL.Path)
				}
				_, _ = writer.Write([]byte(tt.body))
			}))
			defer server.Close()

			provider := newTestProvider(t, server.URL)
			_, err := provider.FinanceDataQuery(context.Background(), "贵州茅台 EPS")
			if err == nil {
				t.Fatal("expected empty finance data error")
			}
			var providerError *ProviderError
			if !errors.As(err, &providerError) {
				t.Fatalf("expected ProviderError, got %T", err)
			}
			if providerError.Operation != "search_data_empty" {
				t.Fatalf("unexpected provider error: %+v", providerError)
			}
		})
	}
}

// TestProviderRejectsRemoteError 验证远端异常响应会返回脱敏的 ProviderError。
func TestProviderRejectsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code": 500, "status": 500, "message": "boom"}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	_, err := provider.FinancialQA(context.Background(), QARequest{Question: "测试"})
	if err == nil {
		t.Fatal("expected error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Provider != "eastmoney-ai" || providerError.Operation != "remote_code" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
	if strings.Contains(providerError.Error(), "test-key") {
		t.Fatalf("provider error leaked api key: %s", providerError.Error())
	}
}

// TestProviderRejectsEmptyArticleContent 验证文章类接口不会把空正文当作真实 AI 报告成功返回。
func TestProviderRejectsEmptyArticleContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":0,"status":0,"data":{"title":"空报告","content":""}}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	_, err := provider.EarningsReview(context.Background(), EarningsReviewRequest{EastMoneyCode: "600519.SH", ReportDate: "2026-03-31"})
	if err == nil {
		t.Fatal("expected empty article content error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Operation != "article_content" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}

// TestProviderRejectsEmptyTrackingReportContent 验证跟踪报告缺少正文时显式失败。
func TestProviderRejectsEmptyTrackingReportContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":0,"status":0,"data":{"title":"跟踪报告"}}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	_, err := provider.TrackingReport(context.Background(), "贵州茅台")
	if err == nil {
		t.Fatal("expected empty tracking content error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Operation != "tracking_content" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}

// TestProviderRequiresExplicitAPIKey 验证 Provider 不从全局配置猜测密钥，调用方必须显式注入。
func TestProviderRequiresExplicitAPIKey(t *testing.T) {
	provider, err := NewProvider(Config{})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}
	_, err = provider.FinancialQA(context.Background(), QARequest{Question: "测试"})
	if err == nil {
		t.Fatal("expected missing api key error")
	}
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerError.Operation != "auth" {
		t.Fatalf("unexpected provider error: %+v", providerError)
	}
}

// newTestProvider 构造带测试密钥和本地服务地址的东方财富 AI Provider。
func newTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	provider, err := NewProvider(Config{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}
	return provider
}
