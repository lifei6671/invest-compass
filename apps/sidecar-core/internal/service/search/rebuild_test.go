package search

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestSearchRebuildManagerRebuildsAllAndActivatesReadyBatches 验证全量重建成功后切换 active batch。
func TestSearchRebuildManagerRebuildsAllAndActivatesReadyBatches(t *testing.T) {
	store := newFakeSearchRebuildStore()
	store.stocks = []model.Stock{{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台"}}
	store.aliases = map[string][]model.StockAlias{"CN:SH:600519": {{Symbol: "CN:SH:600519", Alias: "茅台"}}}
	store.news = []model.NewsItem{{ID: 7, Source: "sina", Title: "白酒新闻", Summary: "茅台分红", Symbols: "CN:SH:600519", Tags: "白酒"}}
	store.reports = []model.AnalysisReport{{ID: 8, Symbol: "CN:SH:600519", Title: "茅台报告", ContentMarkdown: "均线改善", RiskSummary: "估值波动"}}
	store.watchlists = []model.Watchlist{{ID: 9, Symbol: "CN:SH:600519", Note: "关注量价", Tags: "观察"}}
	manager := NewSearchRebuildManager(SearchRebuildConfig{
		Store: store,
		Now:   fixedRebuildNow,
		NewID: func(prefix string) string { return prefix + "-new" },
	})

	result, err := manager.Rebuild(context.Background(), SearchRebuildScopeAll)
	if err != nil {
		t.Fatalf("rebuild all search indexes: %v", err)
	}

	if result.TaskID != "search-rebuild-new" || result.StockBatchID != "stock-new" || result.DocumentBatchID != "document-new" {
		t.Fatalf("unexpected rebuild result: %+v", result)
	}
	if len(store.tasks) == 0 || store.tasks[len(store.tasks)-1].Type != SearchIndexRebuildTaskType || store.tasks[len(store.tasks)-1].Status != "SUCCESS" {
		t.Fatalf("expected successful rebuild task, got %+v", store.tasks)
	}
	if len(store.stockRows) != 1 || store.stockRows[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected stock fts row, got %+v", store.stockRows)
	}
	if len(store.documents) != 3 {
		t.Fatalf("expected three search documents, got %+v", store.documents)
	}
	if store.activatedStockBatchID != "stock-new" || store.activatedDocumentBatchID != "document-new" {
		t.Fatalf("expected new active batches, got stock=%q document=%q", store.activatedStockBatchID, store.activatedDocumentBatchID)
	}
	assertBatchStatus(t, store, "stock-new", dao.SearchIndexBatchStatusReady)
	assertBatchStatus(t, store, "document-new", dao.SearchIndexBatchStatusReady)
}

// TestScopedDocumentSearchServiceRebuildUsesConfiguredTokenizer 验证 API service 触发重建时不会丢失已配置的中文分词器。
func TestScopedDocumentSearchServiceRebuildUsesConfiguredTokenizer(t *testing.T) {
	store := newFakeSearchRebuildStore()
	store.stocks = []model.Stock{{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台"}}
	tokenizer := fakeMetadataTokenizer{
		metadata: TokenizerMetadata{Name: "gse", Version: "1", DictionaryHash: "test-domain"},
		tokens:   []string{"贵州", "茅台"},
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store, Tokenizer: tokenizer})

	_, err := service.Rebuild(context.Background(), SearchRebuildRequest{Scope: SearchRebuildScopeAll})
	if err != nil {
		t.Fatalf("rebuild with configured tokenizer: %v", err)
	}

	stockBatch := store.batches[store.activatedStockBatchID]
	if stockBatch.TokenizerName != "gse" || stockBatch.TokenizerVersion != "1" || stockBatch.DictionaryHash != "test-domain" {
		t.Fatalf("expected configured tokenizer metadata, got %+v", stockBatch)
	}
	if len(store.stockRows) != 1 || !strings.Contains(store.stockRows[0].NameIndex, "贵州") {
		t.Fatalf("expected stock index to use configured tokenizer, got %+v", store.stockRows)
	}
}

// TestSearchRebuildManagerFailureKeepsOldActiveBatch 验证重建失败时标记 FAILED，且不切换 active batch。
func TestSearchRebuildManagerFailureKeepsOldActiveBatch(t *testing.T) {
	store := newFakeSearchRebuildStore()
	store.stocks = []model.Stock{{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台"}}
	store.replaceStockErr = errors.New("replace stock fts failed with api_key: sk-secret")
	manager := NewSearchRebuildManager(SearchRebuildConfig{
		Store: store,
		Now:   fixedRebuildNow,
		NewID: func(prefix string) string { return prefix + "-failed" },
	})

	_, err := manager.Rebuild(context.Background(), SearchRebuildScopeAll)
	if err == nil {
		t.Fatal("expected rebuild failure")
	}
	if store.activatedStockBatchID != "" || store.activatedDocumentBatchID != "" {
		t.Fatalf("failed rebuild must not activate new batches, got stock=%q document=%q", store.activatedStockBatchID, store.activatedDocumentBatchID)
	}
	assertBatchStatus(t, store, "stock-failed", dao.SearchIndexBatchStatusFailed)
	assertBatchStatus(t, store, "document-failed", dao.SearchIndexBatchStatusFailed)
	lastTask := store.tasks[len(store.tasks)-1]
	if lastTask.Status != "FAILED" {
		t.Fatalf("expected failed task, got %+v", lastTask)
	}
	if len(store.events) == 0 || containsReportSearchForbiddenText(store.events[len(store.events)-1].Payload) {
		t.Fatalf("expected redacted failed event, got %+v", store.events)
	}
}

type fakeMetadataTokenizer struct {
	metadata TokenizerMetadata
	tokens   []string
}

// Tokenize 返回测试预置 token，用于验证重建流程不会依赖真实分词器。
func (tokenizer fakeMetadataTokenizer) Tokenize(_ string) []string {
	return tokenizer.tokens
}

// Metadata 返回测试预置分词元数据，用于校验索引批次可追溯字段。
func (tokenizer fakeMetadataTokenizer) Metadata() TokenizerMetadata {
	return tokenizer.metadata
}

// TestSearchRebuildManagerFirstBuildWithoutOldBatch 验证首次无旧 active batch 时也能激活新索引。
func TestSearchRebuildManagerFirstBuildWithoutOldBatch(t *testing.T) {
	store := newFakeSearchRebuildStore()
	manager := NewSearchRebuildManager(SearchRebuildConfig{
		Store: store,
		Now:   fixedRebuildNow,
		NewID: func(prefix string) string { return prefix + "-first" },
	})

	result, err := manager.Rebuild(context.Background(), SearchRebuildScopeAll)
	if err != nil {
		t.Fatalf("first rebuild should activate empty index batches: %v", err)
	}
	if result.StockBatchID != "stock-first" || result.DocumentBatchID != "document-first" {
		t.Fatalf("unexpected first rebuild result: %+v", result)
	}
	if store.activatedStockBatchID != "stock-first" || store.activatedDocumentBatchID != "document-first" {
		t.Fatalf("expected first active batches, got stock=%q document=%q", store.activatedStockBatchID, store.activatedDocumentBatchID)
	}
}

// TestSearchRebuildManagerRebuildsStockScopeOnly 验证 stock scope 只重建股票 batch，并复用当前 document batch。
func TestSearchRebuildManagerRebuildsStockScopeOnly(t *testing.T) {
	store := newFakeSearchRebuildStore()
	store.state[ActiveDocumentBatchStateKey] = "doc-ready-existing"
	store.batches["doc-ready-existing"] = model.SearchIndexBatch{BatchID: "doc-ready-existing", Scope: "document", Status: dao.SearchIndexBatchStatusReady}
	store.stocks = []model.Stock{{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台"}}
	manager := NewSearchRebuildManager(SearchRebuildConfig{
		Store: store,
		Now:   fixedRebuildNow,
		NewID: func(prefix string) string { return prefix + "-stock-scope" },
	})

	result, err := manager.Rebuild(context.Background(), SearchRebuildScopeStock)
	if err != nil {
		t.Fatalf("rebuild stock scope: %v", err)
	}

	if result.StockBatchID != "stock-stock-scope" || result.DocumentBatchID != "doc-ready-existing" {
		t.Fatalf("unexpected stock scope result: %+v", result)
	}
	if len(store.stockRows) != 1 {
		t.Fatalf("expected stock rows rebuilt, got %+v", store.stockRows)
	}
	if len(store.documents) != 0 {
		t.Fatalf("stock scope must not rebuild document rows, got %+v", store.documents)
	}
	if store.activatedStockBatchID != "stock-stock-scope" || store.activatedDocumentBatchID != "doc-ready-existing" {
		t.Fatalf("expected stock-only activation, got stock=%q document=%q", store.activatedStockBatchID, store.activatedDocumentBatchID)
	}
	assertBatchStatus(t, store, "stock-stock-scope", dao.SearchIndexBatchStatusReady)
}

// TestSearchRebuildManagerRebuildsDocumentScopesSafely 验证任一文档 scope 会重建完整 document batch，避免切换后丢失其他菜单索引。
func TestSearchRebuildManagerRebuildsDocumentScopesSafely(t *testing.T) {
	cases := []struct {
		scope string
	}{
		{scope: SearchRebuildScopeReports},
		{scope: SearchRebuildScopeNews},
		{scope: SearchRebuildScopeWatchlistNotes},
	}
	for _, item := range cases {
		t.Run(item.scope, func(t *testing.T) {
			store := newFakeSearchRebuildStore()
			store.state[activeStockBatchStateKey] = "stock-ready-existing"
			store.batches["stock-ready-existing"] = model.SearchIndexBatch{BatchID: "stock-ready-existing", Scope: "stock", Status: dao.SearchIndexBatchStatusReady}
			store.news = []model.NewsItem{{ID: 7, Title: "新闻", Summary: "摘要"}}
			store.reports = []model.AnalysisReport{{ID: 8, Symbol: "CN:SH:600519", Title: "报告", ContentMarkdown: "均线改善"}}
			store.watchlists = []model.Watchlist{{ID: 9, Symbol: "CN:SH:600519", Note: "自选备注"}}
			manager := NewSearchRebuildManager(SearchRebuildConfig{
				Store: store,
				Now:   fixedRebuildNow,
				NewID: func(prefix string) string { return prefix + "-" + item.scope },
			})

			result, err := manager.Rebuild(context.Background(), item.scope)
			if err != nil {
				t.Fatalf("rebuild %s scope: %v", item.scope, err)
			}

			if result.StockBatchID != "stock-ready-existing" || result.DocumentBatchID != "document-"+item.scope {
				t.Fatalf("unexpected document scope result: %+v", result)
			}
			if len(store.stockRows) != 0 {
				t.Fatalf("document scope must not rebuild stock rows, got %+v", store.stockRows)
			}
			if len(store.documents) != 3 {
				t.Fatalf("document scope must rebuild complete document batch, got %+v", store.documents)
			}
			if store.activatedStockBatchID != "stock-ready-existing" || store.activatedDocumentBatchID != "document-"+item.scope {
				t.Fatalf("expected document-scope activation, got stock=%q document=%q", store.activatedStockBatchID, store.activatedDocumentBatchID)
			}
			assertBatchStatus(t, store, "document-"+item.scope, dao.SearchIndexBatchStatusReady)
		})
	}
}

// TestSearchStatusIncludesIndexOverview 验证设置中心状态返回索引数量、最后重建时间和词典元数据。
func TestSearchStatusIncludesIndexOverview(t *testing.T) {
	finishedAt := time.Date(2026, 6, 22, 13, 30, 0, 0, time.UTC)
	store := newFakeSearchRebuildStore()
	store.state[SearchIndexFTS5StateKey] = SearchFTS5StatusAvailable
	store.state[activeStockBatchStateKey] = "stock-ready-1"
	store.state[ActiveDocumentBatchStateKey] = "document-ready-1"
	store.overview = dao.SearchIndexOverview{
		StockCount:         12,
		ReportCount:        3,
		NewsCount:          4,
		WatchlistNoteCount: 5,
		LastRebuildAt:      &finishedAt,
		TokenizerName:      "simple",
		TokenizerVersion:   "1",
		DictionaryHash:     "builtin",
	}
	service := NewScopedDocumentSearchService(DocumentSearchConfig{Store: store})

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status should include index overview: %v", err)
	}
	if status.SearchStatus != "READY" || status.GSEStatus != "FALLBACK" {
		t.Fatalf("unexpected search status: %+v", status)
	}
	if status.StockIndexCount != 12 || status.ReportIndexCount != 3 || status.NewsIndexCount != 4 || status.WatchlistNoteCount != 5 {
		t.Fatalf("unexpected index counts: %+v", status)
	}
	if status.LastRebuildAt != "2026-06-22T13:30:00Z" || status.TokenizerName != "simple" || status.DictionaryHash != "builtin" {
		t.Fatalf("unexpected index metadata: %+v", status)
	}
}

// fixedRebuildNow 提供稳定时间，避免重建 batch 断言受当前时间影响。
func fixedRebuildNow() time.Time {
	return time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
}

type fakeSearchRebuildStore struct {
	tasks                    []model.Task
	events                   []model.TaskEvent
	batches                  map[string]model.SearchIndexBatch
	state                    map[string]string
	stocks                   []model.Stock
	aliases                  map[string][]model.StockAlias
	news                     []model.NewsItem
	reports                  []model.AnalysisReport
	watchlists               []model.Watchlist
	stockRows                []dao.StockSearchFTSRow
	documents                []model.SearchDocument
	documentRows             []dao.SearchDocumentFTSRow
	overview                 dao.SearchIndexOverview
	activatedStockBatchID    string
	activatedDocumentBatchID string
	replaceStockErr          error
}

// newFakeSearchRebuildStore 创建搜索重建测试用内存 store。
func newFakeSearchRebuildStore() *fakeSearchRebuildStore {
	return &fakeSearchRebuildStore{
		batches: make(map[string]model.SearchIndexBatch),
		state:   make(map[string]string),
		aliases: make(map[string][]model.StockAlias),
	}
}

// SaveTask 记录搜索重建任务状态。
func (store *fakeSearchRebuildStore) SaveTask(_ context.Context, task *model.Task) error {
	store.tasks = append(store.tasks, *task)
	return nil
}

// AppendTaskEvent 记录搜索重建任务事件。
func (store *fakeSearchRebuildStore) AppendTaskEvent(_ context.Context, event *model.TaskEvent) error {
	store.events = append(store.events, *event)
	return nil
}

// SaveSearchIndexBatch 记录搜索索引 batch 状态。
func (store *fakeSearchRebuildStore) SaveSearchIndexBatch(_ context.Context, batch *model.SearchIndexBatch) error {
	store.batches[batch.BatchID] = *batch
	return nil
}

// GetSearchIndexState 返回当前 active batch 指针。
func (store *fakeSearchRebuildStore) GetSearchIndexState(_ context.Context, key string) (string, bool, error) {
	value, ok := store.state[key]
	return value, ok, nil
}

// GetSearchIndexOverview 返回设置中心搜索索引概览。
func (store *fakeSearchRebuildStore) GetSearchIndexOverview(context.Context, string, string) (dao.SearchIndexOverview, error) {
	return store.overview, nil
}

// SearchDocumentsFTS 满足菜单范围搜索 store 契约，重建状态测试不会触发该路径。
func (store *fakeSearchRebuildStore) SearchDocumentsFTS(context.Context, string, string, string, int) ([]dao.SearchDocumentFTSMatch, error) {
	return nil, nil
}

// ListSearchDocumentsByUIDs 满足菜单范围搜索 store 契约，重建状态测试不会触发该路径。
func (store *fakeSearchRebuildStore) ListSearchDocumentsByUIDs(context.Context, string, []string) ([]model.SearchDocument, error) {
	return nil, nil
}

// ListAllStocksForSearch 返回全量股票索引源。
func (store *fakeSearchRebuildStore) ListAllStocksForSearch(context.Context) ([]model.Stock, error) {
	return store.stocks, nil
}

// ListStockAliasesBySymbols 返回股票别名索引源。
func (store *fakeSearchRebuildStore) ListStockAliasesBySymbols(_ context.Context, symbols []string) (map[string][]model.StockAlias, error) {
	result := make(map[string][]model.StockAlias, len(symbols))
	for _, symbol := range symbols {
		result[symbol] = store.aliases[symbol]
	}
	return result, nil
}

// ReplaceStockSearchFTS 记录股票 FTS 重建行。
func (store *fakeSearchRebuildStore) ReplaceStockSearchFTS(_ context.Context, _ string, rows []dao.StockSearchFTSRow) error {
	if store.replaceStockErr != nil {
		return store.replaceStockErr
	}
	store.stockRows = append([]dao.StockSearchFTSRow(nil), rows...)
	return nil
}

// ListMarketNews 返回资讯索引源。
func (store *fakeSearchRebuildStore) ListMarketNews(context.Context, string, int, time.Duration) ([]model.NewsItem, error) {
	return store.news, nil
}

// ListVisibleAnalysisReports 返回报告索引源。
func (store *fakeSearchRebuildStore) ListVisibleAnalysisReports(context.Context) ([]model.AnalysisReport, error) {
	return store.reports, nil
}

// ListActiveWatchlists 返回自选备注索引源。
func (store *fakeSearchRebuildStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return store.watchlists, nil
}

// UpsertSearchDocument 记录文档索引元数据。
func (store *fakeSearchRebuildStore) UpsertSearchDocument(_ context.Context, document *model.SearchDocument) error {
	store.documents = append(store.documents, *document)
	return nil
}

// ReplaceSearchDocumentFTS 记录文档 FTS 行。
func (store *fakeSearchRebuildStore) ReplaceSearchDocumentFTS(_ context.Context, row dao.SearchDocumentFTSRow) error {
	store.documentRows = append(store.documentRows, row)
	return nil
}

// ActivateSearchIndexBatches 记录最终激活的两个 batch。
func (store *fakeSearchRebuildStore) ActivateSearchIndexBatches(_ context.Context, stockBatchID string, documentBatchID string) error {
	store.activatedStockBatchID = stockBatchID
	store.activatedDocumentBatchID = documentBatchID
	return nil
}

// assertBatchStatus 验证指定 batch 已写入期望状态。
func assertBatchStatus(t *testing.T, store *fakeSearchRebuildStore, batchID string, status string) {
	t.Helper()
	batch, ok := store.batches[batchID]
	if !ok {
		t.Fatalf("expected batch %q to exist", batchID)
	}
	if batch.Status != status {
		t.Fatalf("expected batch %q status %q, got %+v", batchID, status, batch)
	}
}
