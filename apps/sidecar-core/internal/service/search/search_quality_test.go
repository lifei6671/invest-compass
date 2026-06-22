package search

import (
	"context"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestSearchQualityCoversStockQueryForms 验证股票搜索关键输入形态都有稳定归一化保护。
func TestSearchQualityCoversStockQueryForms(t *testing.T) {
	cases := []struct {
		input string
		kind  QueryKind
		text  string
	}{
		{input: "600519", kind: QueryKindCode, text: "600519"},
		{input: "sh600519", kind: QueryKindExchangeCode, text: "sh600519"},
		{input: "CN:SH:600519", kind: QueryKindSymbol, text: "cn:sh:600519"},
		{input: "贵州茅台", kind: QueryKindChinese, text: "贵州茅台"},
		{input: "茅台", kind: QueryKindChinese, text: "茅台"},
		{input: "gzmt", kind: QueryKindPinyinInitials, text: "gzmt"},
		{input: "宁王", kind: QueryKindChinese, text: "宁王"},
		{input: "ningdeshidai", kind: QueryKindPinyin, text: "ningdeshidai"},
		{input: "ndsd", kind: QueryKindPinyinInitials, text: "ndsd"},
		{input: "重庆啤酒", kind: QueryKindChinese, text: "重庆啤酒"},
		{input: "chongqingpijiu", kind: QueryKindPinyin, text: "chongqingpijiu"},
		{input: "cqpj", kind: QueryKindPinyinInitials, text: "cqpj"},
	}

	for _, item := range cases {
		got := NormalizeQueryInput(item.input)
		if got.Kind != item.kind || got.Text != item.text {
			t.Fatalf("normalize %q got kind=%q text=%q want kind=%q text=%q", item.input, got.Kind, got.Text, item.kind, item.text)
		}
	}
}

// TestSearchQualitySanitizesFTSOperatorsAndColumns 验证用户输入中的 FTS 操作符和列语法不会原样进入 MATCH。
func TestSearchQualitySanitizesFTSOperatorsAndColumns(t *testing.T) {
	match := BuildFTSMatch(NormalizeQueryInput(`茅台 OR doc_type:news NEAR secret`), []string{"title_index", "bad-column", "body_index"})

	if match == "" {
		t.Fatal("expected non-empty sanitized FTS match")
	}
	for _, unsafe := range []string{"doc_type:news", "NEAR", "near", "bad-column"} {
		if strings.Contains(match, unsafe) {
			t.Fatalf("expected sanitized match without %q, got %q", unsafe, match)
		}
	}
	if !strings.Contains(match, "title_index") || !strings.Contains(match, "body_index") {
		t.Fatalf("expected allowed columns in match, got %q", match)
	}
}

// TestSearchQualityKeepsThemeKeywordsScoped 验证同一主题词在不同菜单入口下只返回对应范围。
func TestSearchQualityKeepsThemeKeywordsScoped(t *testing.T) {
	cases := []struct {
		name     string
		keyword  string
		search   func(*ScopedDocumentSearchService) ([]DocumentSearchResult, error)
		wantUID  string
		wantType string
	}{
		{
			name:    "reports",
			keyword: "光模块",
			search: func(service *ScopedDocumentSearchService) ([]DocumentSearchResult, error) {
				return service.SearchReports(context.Background(), DocumentSearchRequest{Keyword: "光模块", Limit: 10})
			},
			wantUID:  "report:theme",
			wantType: SearchDocTypeReport,
		},
		{
			name:    "news",
			keyword: "CPO",
			search: func(service *ScopedDocumentSearchService) ([]DocumentSearchResult, error) {
				return service.SearchNews(context.Background(), DocumentSearchRequest{Keyword: "CPO", Limit: 10})
			},
			wantUID:  "news:theme",
			wantType: SearchDocTypeNews,
		},
		{
			name:    "watchlist",
			keyword: "AI服务器",
			search: func(service *ScopedDocumentSearchService) ([]DocumentSearchResult, error) {
				return service.SearchWatchlistNotes(context.Background(), DocumentSearchRequest{Keyword: "AI服务器", Limit: 10})
			},
			wantUID:  "watchlist_note:theme",
			wantType: SearchDocTypeWatchlistNote,
		},
	}

	for _, item := range cases {
		store := searchQualityThemeStore()
		service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})
		results, err := item.search(service)
		if err != nil {
			t.Fatalf("%s search %q: %v", item.name, item.keyword, err)
		}
		if len(results) != 1 || results[0].DocUID != item.wantUID || results[0].DocType != item.wantType {
			t.Fatalf("%s expected %s/%s, got %+v", item.name, item.wantUID, item.wantType, results)
		}
	}
}

// TestSearchQualityKeepsDocumentScopesIsolated 验证即使 DAO 返回混合命中，service 也只回传当前菜单范围文档。
func TestSearchQualityKeepsDocumentScopesIsolated(t *testing.T) {
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-quality-1"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-quality-1", DocUID: "news:1", DocType: SearchDocTypeNews, Symbol: "CN:SH:600519"},
			{BatchID: "doc-quality-1", DocUID: "report:1", DocType: SearchDocTypeReport, Symbol: "CN:SH:600519"},
			{BatchID: "doc-quality-1", DocUID: "watchlist_note:1", DocType: SearchDocTypeWatchlistNote, Symbol: "CN:SH:600519"},
		},
		documents: map[string]model.SearchDocument{
			"news:1":           {BatchID: "doc-quality-1", DocUID: "news:1", DocType: SearchDocTypeNews, RefID: "1", Symbol: "CN:SH:600519", Title: "新闻"},
			"report:1":         {BatchID: "doc-quality-1", DocUID: "report:1", DocType: SearchDocTypeReport, RefID: "1", Symbol: "CN:SH:600519", Title: "报告"},
			"watchlist_note:1": {BatchID: "doc-quality-1", DocUID: "watchlist_note:1", DocType: SearchDocTypeWatchlistNote, RefID: "1", Symbol: "CN:SH:600519", Title: "自选备注"},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchReports(context.Background(), DocumentSearchRequest{Keyword: "茅台", Limit: 10})
	if err != nil {
		t.Fatalf("search reports: %v", err)
	}
	if len(results) != 1 || results[0].DocUID != "report:1" || results[0].DocType != SearchDocTypeReport {
		t.Fatalf("expected only report result, got %+v", results)
	}
}

// searchQualityThemeStore 构造混合主题文档 fixture，用于验证不同菜单范围不会互相串结果。
func searchQualityThemeStore() *fakeDocumentSearchStore {
	return &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-quality-theme"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-quality-theme", DocUID: "report:theme", DocType: SearchDocTypeReport, Symbol: "CN:SZ:300308"},
			{BatchID: "doc-quality-theme", DocUID: "news:theme", DocType: SearchDocTypeNews, Symbol: "CN:SZ:300308"},
			{BatchID: "doc-quality-theme", DocUID: "watchlist_note:theme", DocType: SearchDocTypeWatchlistNote, Symbol: "CN:SZ:300308"},
		},
		documents: map[string]model.SearchDocument{
			"report:theme":         {BatchID: "doc-quality-theme", DocUID: "report:theme", DocType: SearchDocTypeReport, RefID: "1", Symbol: "CN:SZ:300308", Title: "光模块报告"},
			"news:theme":           {BatchID: "doc-quality-theme", DocUID: "news:theme", DocType: SearchDocTypeNews, RefID: "2", Symbol: "CN:SZ:300308", Title: "CPO 新闻"},
			"watchlist_note:theme": {BatchID: "doc-quality-theme", DocUID: "watchlist_note:theme", DocType: SearchDocTypeWatchlistNote, RefID: "3", Symbol: "CN:SZ:300308", Title: "AI服务器自选备注"},
		},
	}
}
