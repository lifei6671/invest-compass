package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestScopedDocumentSearchServiceSearchesFixedReportScope 验证报告搜索只绑定 active batch 和 report doc_type。
func TestScopedDocumentSearchServiceSearchesFixedReportScope(t *testing.T) {
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-ready-1"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-ready-1", DocUID: "report:1", DocType: SearchDocTypeReport, Symbol: "CN:SH:600519"},
		},
		documents: map[string]model.SearchDocument{
			"report:1": {BatchID: "doc-ready-1", DocUID: "report:1", DocType: SearchDocTypeReport, RefID: "1", Symbol: "CN:SH:600519", Title: "贵州茅台报告", Summary: "均线改善"},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchReports(context.Background(), DocumentSearchRequest{Keyword: `茅台 OR doc_type:news`, Limit: 10})
	if err != nil {
		t.Fatalf("search reports: %v", err)
	}
	if store.lastBatchID != "doc-ready-1" || store.lastDocType != SearchDocTypeReport {
		t.Fatalf("expected active report scope, got batch=%q docType=%q", store.lastBatchID, store.lastDocType)
	}
	if len(results) != 1 || results[0].DocUID != "report:1" || results[0].DocType != SearchDocTypeReport {
		t.Fatalf("unexpected report results: %+v", results)
	}
	if store.lastMatch == "" || store.lastMatch == `茅台 OR doc_type:news` {
		t.Fatalf("expected sanitized FTS match, got %q", store.lastMatch)
	}
}

// TestScopedDocumentSearchServiceFiltersSymbolsAndPaginates 验证菜单范围搜索在 service 层支持 symbol 过滤和分页。
func TestScopedDocumentSearchServiceFiltersSymbolsAndPaginates(t *testing.T) {
	sourceTime := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-ready-2"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-ready-2", DocUID: "news:1", DocType: SearchDocTypeNews, Symbol: "CN:SH:600519"},
			{BatchID: "doc-ready-2", DocUID: "news:2", DocType: SearchDocTypeNews, Symbol: "CN:SZ:000001"},
			{BatchID: "doc-ready-2", DocUID: "news:3", DocType: SearchDocTypeNews, Symbol: "CN:SH:600519"},
		},
		documents: map[string]model.SearchDocument{
			"news:1": {BatchID: "doc-ready-2", DocUID: "news:1", DocType: SearchDocTypeNews, RefID: "1", Symbol: "CN:SH:600519", Title: "第一条", SourceTime: sourceTime},
			"news:2": {BatchID: "doc-ready-2", DocUID: "news:2", DocType: SearchDocTypeNews, RefID: "2", Symbol: "CN:SZ:000001", Title: "第二条", SourceTime: sourceTime},
			"news:3": {BatchID: "doc-ready-2", DocUID: "news:3", DocType: SearchDocTypeNews, RefID: "3", Symbol: "CN:SH:600519", Title: "第三条", SourceTime: sourceTime},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchNews(context.Background(), DocumentSearchRequest{
		Keyword: "白酒",
		Symbols: []string{"CN:SH:600519"},
		Limit:   1,
		Offset:  1,
	})
	if err != nil {
		t.Fatalf("search news: %v", err)
	}
	if len(results) != 1 || results[0].DocUID != "news:3" {
		t.Fatalf("expected paginated symbol-filtered result, got %+v", results)
	}
}

// TestScopedDocumentSearchServiceFiltersNewsByAllCachedSymbols 验证个股资讯搜索不会漏掉多股票新闻里的非首个 symbol。
func TestScopedDocumentSearchServiceFiltersNewsByAllCachedSymbols(t *testing.T) {
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-ready-multi-symbol"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-ready-multi-symbol", DocUID: "news:8", DocType: SearchDocTypeNews, Symbol: "CN:SH:600519"},
		},
		documents: map[string]model.SearchDocument{
			"news:8": {BatchID: "doc-ready-multi-symbol", DocUID: "news:8", DocType: SearchDocTypeNews, RefID: "8", Symbol: "CN:SH:600519", Title: "CPO 产业链扩产"},
		},
		newsByID: map[int64]model.NewsItem{
			8: {ID: 8, Symbols: `["CN:SH:600519","CN:SZ:300308"]`, Tags: `["光模块"]`, Title: "CPO 产业链扩产"},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchNews(context.Background(), DocumentSearchRequest{
		Keyword: "CPO",
		Symbols: []string{"CN:SZ:300308"},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search news: %v", err)
	}
	if len(results) != 1 || results[0].DocUID != "news:8" {
		t.Fatalf("expected second cached symbol to match, got %+v", results)
	}
}

// TestScopedDocumentSearchServiceEnrichesNewsTagsAndSentiment 验证资讯搜索结果回源补齐业务标签和情绪标签。
func TestScopedDocumentSearchServiceEnrichesNewsTagsAndSentiment(t *testing.T) {
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-ready-tags"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-ready-tags", DocUID: "news:42", DocType: SearchDocTypeNews},
		},
		documents: map[string]model.SearchDocument{
			"news:42": {BatchID: "doc-ready-tags", DocUID: "news:42", DocType: SearchDocTypeNews, RefID: "42", Title: "订单超预期", Summary: "行业景气度回升"},
		},
		newsByID: map[int64]model.NewsItem{
			42: {ID: 42, URL: "https://example.com/news/42", Tags: `["光模块","CPO"]`, Symbols: `["CN:SZ:300308"]`, Title: "订单超预期", Summary: "行业景气度回升"},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchNews(context.Background(), DocumentSearchRequest{Keyword: "CPO", Limit: 10})
	if err != nil {
		t.Fatalf("search news: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one news result, got %+v", results)
	}
	if got := strings.Join(results[0].Tags, ","); got != "CN:SZ:300308,光模块,CPO" {
		t.Fatalf("expected source tags to be preserved, got %q", got)
	}
	if results[0].Sentiment != "positive" {
		t.Fatalf("expected positive sentiment, got %+v", results[0])
	}
	if results[0].URL != "https://example.com/news/42" {
		t.Fatalf("expected source url to be preserved, got %+v", results[0])
	}
}

// TestScopedDocumentSearchServiceSearchesWatchlistNotes 验证自选备注搜索固定使用 watchlist_note 范围。
func TestScopedDocumentSearchServiceSearchesWatchlistNotes(t *testing.T) {
	store := &fakeDocumentSearchStore{
		state: map[string]string{"active_document_batch_id": "doc-ready-3"},
		matches: []dao.SearchDocumentFTSMatch{
			{BatchID: "doc-ready-3", DocUID: "watchlist_note:5", DocType: SearchDocTypeWatchlistNote, Symbol: "CN:SZ:000001"},
		},
		documents: map[string]model.SearchDocument{
			"watchlist_note:5": {BatchID: "doc-ready-3", DocUID: "watchlist_note:5", DocType: SearchDocTypeWatchlistNote, RefID: "5", Symbol: "CN:SZ:000001", Title: "CN:SZ:000001 自选备注"},
		},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	results, err := service.SearchWatchlistNotes(context.Background(), DocumentSearchRequest{Keyword: "息差", Limit: 10})
	if err != nil {
		t.Fatalf("search watchlist notes: %v", err)
	}
	if store.lastDocType != SearchDocTypeWatchlistNote || len(results) != 1 {
		t.Fatalf("expected watchlist note scope result, docType=%q results=%+v", store.lastDocType, results)
	}
}

type fakeDocumentSearchStore struct {
	state       map[string]string
	matches     []dao.SearchDocumentFTSMatch
	documents   map[string]model.SearchDocument
	newsByID    map[int64]model.NewsItem
	lastBatchID string
	lastDocType string
	lastMatch   string
}

// GetSearchIndexState 返回测试预置的 active document batch。
func (store *fakeDocumentSearchStore) GetSearchIndexState(_ context.Context, key string) (string, bool, error) {
	value, ok := store.state[key]
	return value, ok, nil
}

// SearchDocumentsFTS 记录查询范围并返回预置 FTS 命中。
func (store *fakeDocumentSearchStore) SearchDocumentsFTS(_ context.Context, batchID string, docType string, match string, _ int) ([]dao.SearchDocumentFTSMatch, error) {
	store.lastBatchID = batchID
	store.lastDocType = docType
	store.lastMatch = match
	return store.matches, nil
}

// ListSearchDocumentsByUIDs 按传入顺序回表预置文档。
func (store *fakeDocumentSearchStore) ListSearchDocumentsByUIDs(_ context.Context, _ string, docUIDs []string) ([]model.SearchDocument, error) {
	documents := make([]model.SearchDocument, 0, len(docUIDs))
	for _, docUID := range docUIDs {
		if document, ok := store.documents[docUID]; ok {
			documents = append(documents, document)
		}
	}
	return documents, nil
}

// ListNewsItemsByIDs 按搜索文档 ref_id 回源新闻缓存，补齐搜索元数据未保存的业务展示字段。
func (store *fakeDocumentSearchStore) ListNewsItemsByIDs(_ context.Context, ids []int64) (map[int64]model.NewsItem, error) {
	result := make(map[int64]model.NewsItem, len(ids))
	for _, id := range ids {
		if item, ok := store.newsByID[id]; ok {
			result[id] = item
		}
	}
	return result, nil
}
