package search

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// ActiveDocumentBatchStateKey 是菜单范围文档索引当前可查询 batch 的状态 key。
	ActiveDocumentBatchStateKey = "active_document_batch_id"

	// SearchDocTypeNews 表示资讯中心菜单范围的搜索文档。
	SearchDocTypeNews = "news"
	// SearchDocTypeReport 表示报告历史菜单范围的搜索文档。
	SearchDocTypeReport = "report"
	// SearchDocTypeWatchlistNote 表示自选备注菜单范围的搜索文档。
	SearchDocTypeWatchlistNote = "watchlist_note"
)

// DocumentIndexStore 定义文档索引器需要的最小 DAO 能力。
type DocumentIndexStore interface {
	GetSearchIndexState(ctx context.Context, key string) (string, bool, error)
	UpsertSearchDocument(ctx context.Context, document *model.SearchDocument) error
	ReplaceSearchDocumentFTS(ctx context.Context, row dao.SearchDocumentFTSRow) error
	SoftDeleteSearchDocument(ctx context.Context, batchID string, docUID string) error
}

// DocumentIndexerConfig 是菜单范围文档索引器配置。
type DocumentIndexerConfig struct {
	Store     DocumentIndexStore
	Tokenizer Tokenizer
}

// DocumentIndexer 将业务源对象转换为统一搜索文档，并写入当前 active document batch。
type DocumentIndexer struct {
	store     DocumentIndexStore
	tokenizer Tokenizer
}

// ReportSearchSource 是报告搜索索引输入，要求调用方只传入已脱敏摘要。
type ReportSearchSource struct {
	Report                 model.AnalysisReport
	SanitizedSearchSummary string
}

// NewDocumentIndexer 创建菜单范围文档索引器。
func NewDocumentIndexer(config DocumentIndexerConfig) *DocumentIndexer {
	tokenizer := config.Tokenizer
	if tokenizer == nil {
		tokenizer = SimpleTokenizer{}
	}
	return &DocumentIndexer{
		store:     config.Store,
		tokenizer: tokenizer,
	}
}

// IndexNews 将新闻资讯写入 active document batch 的 news 搜索范围。
func (indexer *DocumentIndexer) IndexNews(ctx context.Context, item model.NewsItem) error {
	sourceTime := firstNonZeroTime(item.PublishedAt, item.UpdatedAt, item.CreatedAt)
	document := model.SearchDocument{
		DocUID:     buildDocumentUID(SearchDocTypeNews, item.ID),
		DocType:    SearchDocTypeNews,
		RefTable:   "news_items",
		RefID:      strconv.FormatInt(item.ID, 10),
		Symbol:     firstDelimitedValue(item.Symbols),
		Title:      strings.TrimSpace(item.Title),
		Summary:    strings.TrimSpace(item.Summary),
		Source:     strings.TrimSpace(item.Source),
		SourceTime: sourceTime,
	}
	row := buildSearchDocumentFTSRow(indexer.tokenizer, document, []string{item.Summary}, []string{item.Tags})
	return indexer.upsert(ctx, document, row)
}

// IndexReport 将 AI 报告写入 active document batch 的 report 搜索范围。
func (indexer *DocumentIndexer) IndexReport(ctx context.Context, source ReportSearchSource) error {
	report := source.Report
	summary := strings.TrimSpace(source.SanitizedSearchSummary)
	sourceTime := firstNonZeroTime(report.UpdatedAt, report.CreatedAt)
	document := model.SearchDocument{
		DocUID:     buildDocumentUID(SearchDocTypeReport, report.ID),
		DocType:    SearchDocTypeReport,
		RefTable:   "analysis_reports",
		RefID:      strconv.FormatInt(report.ID, 10),
		Symbol:     strings.TrimSpace(report.Symbol),
		Title:      strings.TrimSpace(report.Title),
		Summary:    summary,
		Source:     strings.TrimSpace(report.ModelName),
		SourceTime: sourceTime,
	}
	row := buildSearchDocumentFTSRow(indexer.tokenizer, document, []string{summary, report.RiskSummary}, nil)
	return indexer.upsert(ctx, document, row)
}

// IndexWatchlistNote 将自选备注写入 active document batch 的 watchlist_note 搜索范围。
func (indexer *DocumentIndexer) IndexWatchlistNote(ctx context.Context, item model.Watchlist) error {
	sourceTime := firstNonZeroTime(item.UpdatedAt, item.CreatedAt)
	symbol := strings.TrimSpace(item.Symbol)
	document := model.SearchDocument{
		DocUID:     buildDocumentUID(SearchDocTypeWatchlistNote, item.ID),
		DocType:    SearchDocTypeWatchlistNote,
		RefTable:   "watchlists",
		RefID:      strconv.FormatInt(item.ID, 10),
		Symbol:     symbol,
		Title:      symbol + " 自选备注",
		Summary:    strings.TrimSpace(item.Note),
		SourceTime: sourceTime,
	}
	row := buildSearchDocumentFTSRow(indexer.tokenizer, document, []string{item.Note}, []string{item.Tags})
	return indexer.upsert(ctx, document, row)
}

// Remove 在 active document batch 中软删除指定搜索文档，并同步清理 FTS 行。
func (indexer *DocumentIndexer) Remove(ctx context.Context, docUID string) error {
	if indexer == nil || indexer.store == nil {
		return fmt.Errorf("search document index store is required")
	}
	batchID, err := indexer.activeBatchID(ctx)
	if err != nil {
		return err
	}
	return indexer.store.SoftDeleteSearchDocument(ctx, batchID, strings.TrimSpace(docUID))
}

// upsert 将文档元数据和 FTS 行绑定到当前 active document batch 后写入 DAO。
func (indexer *DocumentIndexer) upsert(ctx context.Context, document model.SearchDocument, row dao.SearchDocumentFTSRow) error {
	if indexer == nil || indexer.store == nil {
		return fmt.Errorf("search document index store is required")
	}
	batchID, err := indexer.activeBatchID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	document.BatchID = batchID
	document.IndexedAt = now
	document.IndexVersion = 1
	row.BatchID = batchID
	if err := indexer.store.UpsertSearchDocument(ctx, &document); err != nil {
		return err
	}
	return indexer.store.ReplaceSearchDocumentFTS(ctx, row)
}

// activeBatchID 读取当前可用的文档索引 batch，避免写入游离索引。
func (indexer *DocumentIndexer) activeBatchID(ctx context.Context) (string, error) {
	batchID, ok, err := indexer.store.GetSearchIndexState(ctx, ActiveDocumentBatchStateKey)
	if err != nil {
		return "", err
	}
	batchID = strings.TrimSpace(batchID)
	if !ok || batchID == "" {
		return "", fmt.Errorf("active document search batch is not available")
	}
	return batchID, nil
}

// buildSearchDocumentFTSRow 生成菜单范围文档的预分词 FTS 行。
func buildSearchDocumentFTSRow(tokenizer Tokenizer, document model.SearchDocument, bodyParts []string, tagParts []string) dao.SearchDocumentFTSRow {
	if tokenizer == nil {
		tokenizer = SimpleTokenizer{}
	}
	title := strings.TrimSpace(document.Title)
	body := append([]string{document.Summary}, bodyParts...)
	pinyin := BuildPinyinText(title, nil)
	return dao.SearchDocumentFTSRow{
		DocUID:      document.DocUID,
		DocType:     document.DocType,
		Symbol:      document.Symbol,
		TitleIndex:  buildTextIndex(tokenizer, title),
		BodyIndex:   buildTextIndex(tokenizer, body...),
		TagIndex:    buildTextIndex(tokenizer, tagParts...),
		PinyinIndex: joinIndexText(pinyin.Full, pinyin.Initials),
	}
}

// buildDocumentUID 生成跨批次稳定的搜索文档标识。
func buildDocumentUID(docType string, id int64) string {
	return strings.TrimSpace(docType) + ":" + strconv.FormatInt(id, 10)
}

// firstDelimitedValue 从逗号、分号或空白分隔的文本中提取首个有效值。
func firstDelimitedValue(text string) string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '|' || r == '\n' || r == '\t' || r == ' '
	})
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			return value
		}
	}
	return ""
}

// firstNonZeroTime 返回第一个非零时间，全部为空时交给 DAO 兜底当前时间。
func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
