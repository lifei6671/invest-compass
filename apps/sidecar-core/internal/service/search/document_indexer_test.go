package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestDocumentIndexerIndexesNewsIntoActiveBatch 验证资讯索引绑定 active document batch，并写入标题、摘要和标签。
func TestDocumentIndexerIndexesNewsIntoActiveBatch(t *testing.T) {
	publishedAt := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)
	store := &fakeDocumentIndexStore{state: map[string]string{"active_document_batch_id": "doc-ready-1"}}
	indexer := NewDocumentIndexer(DocumentIndexerConfig{Store: store, Tokenizer: SimpleTokenizer{}})

	err := indexer.IndexNews(context.Background(), model.NewsItem{
		ID:          17,
		Source:      "sina",
		Title:       "贵州茅台发布分红预案",
		Summary:     "白酒板块关注现金流和估值修复。",
		Symbols:     "CN:SH:600519,CN:SZ:000858",
		Tags:        "白酒,分红",
		PublishedAt: publishedAt,
	})
	if err != nil {
		t.Fatalf("index news: %v", err)
	}

	if store.document.BatchID != "doc-ready-1" || store.document.DocType != SearchDocTypeNews {
		t.Fatalf("expected active news document, got %+v", store.document)
	}
	if store.document.DocUID != "news:17" || store.document.RefTable != "news_items" || store.document.RefID != "17" {
		t.Fatalf("unexpected news identity: %+v", store.document)
	}
	if store.document.Symbol != "CN:SH:600519" || !store.document.SourceTime.Equal(publishedAt) {
		t.Fatalf("unexpected news symbol/source time: %+v", store.document)
	}
	assertContainsText(t, store.ftsRow.TitleIndex, "贵州茅台发布分红预案")
	assertContainsText(t, store.ftsRow.BodyIndex, "白酒板块关注现金流和估值修复")
	assertContainsText(t, store.ftsRow.TagIndex, "分红")
}

// TestDocumentIndexerIndexesReportWithSanitizedSummaryOnly 验证报告索引只使用脱敏摘要和风险摘要，不接触完整正文或输入快照。
func TestDocumentIndexerIndexesReportWithSanitizedSummaryOnly(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)
	store := &fakeDocumentIndexStore{state: map[string]string{"active_document_batch_id": "doc-ready-2"}}
	indexer := NewDocumentIndexer(DocumentIndexerConfig{Store: store, Tokenizer: SimpleTokenizer{}})

	err := indexer.IndexReport(context.Background(), ReportSearchSource{
		Report: model.AnalysisReport{
			ID:              23,
			Symbol:          "CN:SH:600519",
			Title:           "贵州茅台技术面分析",
			InputSnapshot:   `{"userPosition":{"cost":123.45}}`,
			ContentMarkdown: "完整正文包含 raw_api_key sk-secret 和用户仓位，不得索引",
			RiskSummary:     "估值波动和成交量回落风险",
			CreatedAt:       createdAt,
		},
		SanitizedSearchSummary: "均线结构改善，仍需观察量能确认。",
	})
	if err != nil {
		t.Fatalf("index report: %v", err)
	}

	if store.document.DocType != SearchDocTypeReport || store.document.DocUID != "report:23" {
		t.Fatalf("unexpected report document: %+v", store.document)
	}
	if store.document.Summary != "均线结构改善，仍需观察量能确认。" {
		t.Fatalf("expected sanitized summary only, got %q", store.document.Summary)
	}
	assertContainsText(t, store.ftsRow.BodyIndex, "均线结构改善")
	assertContainsText(t, store.ftsRow.BodyIndex, "估值波动")
	assertNotContainsText(t, store.ftsRow.BodyIndex, "raw_api_key")
	assertNotContainsText(t, store.ftsRow.BodyIndex, "userposition")
	assertNotContainsText(t, store.ftsRow.BodyIndex, "完整正文")
}

// TestDocumentIndexerIndexesWatchlistNote 验证自选备注索引只覆盖当前自选记录的备注、标签和 symbol。
func TestDocumentIndexerIndexesWatchlistNote(t *testing.T) {
	store := &fakeDocumentIndexStore{state: map[string]string{"active_document_batch_id": "doc-ready-3"}}
	indexer := NewDocumentIndexer(DocumentIndexerConfig{Store: store, Tokenizer: SimpleTokenizer{}})

	err := indexer.IndexWatchlistNote(context.Background(), model.Watchlist{
		ID:     5,
		Symbol: "CN:SZ:000001",
		Tags:   "银行,观察",
		Note:   "关注息差和资产质量拐点",
	})
	if err != nil {
		t.Fatalf("index watchlist note: %v", err)
	}

	if store.document.DocType != SearchDocTypeWatchlistNote || store.document.DocUID != "watchlist_note:5" {
		t.Fatalf("unexpected watchlist note document: %+v", store.document)
	}
	if store.document.Title != "CN:SZ:000001 自选备注" || store.document.Symbol != "CN:SZ:000001" {
		t.Fatalf("unexpected watchlist note metadata: %+v", store.document)
	}
	assertContainsText(t, store.ftsRow.BodyIndex, "关注息差和资产质量拐点")
	assertContainsText(t, store.ftsRow.TagIndex, "银行")
}

// TestDocumentIndexerRemoveUsesActiveBatch 验证清理文档时也绑定 active document batch 并同步调用 DAO 软删除。
func TestDocumentIndexerRemoveUsesActiveBatch(t *testing.T) {
	store := &fakeDocumentIndexStore{state: map[string]string{"active_document_batch_id": "doc-ready-4"}}
	indexer := NewDocumentIndexer(DocumentIndexerConfig{Store: store})

	if err := indexer.Remove(context.Background(), "report:23"); err != nil {
		t.Fatalf("delete document index: %v", err)
	}
	if store.deletedBatchID != "doc-ready-4" || store.deletedDocUID != "report:23" {
		t.Fatalf("expected active batch delete, got batch=%q doc=%q", store.deletedBatchID, store.deletedDocUID)
	}
}

type fakeDocumentIndexStore struct {
	state          map[string]string
	document       model.SearchDocument
	ftsRow         dao.SearchDocumentFTSRow
	deletedBatchID string
	deletedDocUID  string
}

// GetSearchIndexState 返回测试预置的 active batch 指针。
func (store *fakeDocumentIndexStore) GetSearchIndexState(_ context.Context, key string) (string, bool, error) {
	value, ok := store.state[key]
	return value, ok, nil
}

// UpsertSearchDocument 记录写入的文档元数据。
func (store *fakeDocumentIndexStore) UpsertSearchDocument(_ context.Context, document *model.SearchDocument) error {
	store.document = *document
	return nil
}

// ReplaceSearchDocumentFTS 记录写入的文档 FTS 行。
func (store *fakeDocumentIndexStore) ReplaceSearchDocumentFTS(_ context.Context, row dao.SearchDocumentFTSRow) error {
	store.ftsRow = row
	return nil
}

// SoftDeleteSearchDocument 记录软删除目标。
func (store *fakeDocumentIndexStore) SoftDeleteSearchDocument(_ context.Context, batchID string, docUID string) error {
	store.deletedBatchID = batchID
	store.deletedDocUID = docUID
	return nil
}

// assertNotContainsText 校验索引字段不包含敏感或禁止片段。
func assertNotContainsText(t *testing.T, text string, forbidden string) {
	t.Helper()
	if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
		t.Fatalf("expected %q not to contain %q", text, forbidden)
	}
}
