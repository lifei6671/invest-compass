package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// defaultDocumentSearchLimit 是菜单范围搜索默认返回数量。
	defaultDocumentSearchLimit = 20
	// maxDocumentSearchLimit 限制单次菜单范围搜索返回数量，避免 UI 误触发大结果集。
	maxDocumentSearchLimit = 100
)

// DocumentSearchStore 定义菜单范围搜索 service 需要的最小 DAO 能力。
type DocumentSearchStore interface {
	GetSearchIndexState(ctx context.Context, key string) (string, bool, error)
	SearchDocumentsFTS(ctx context.Context, batchID string, docType string, match string, limit int) ([]dao.SearchDocumentFTSMatch, error)
	ListSearchDocumentsByUIDs(ctx context.Context, batchID string, docUIDs []string) ([]model.SearchDocument, error)
}

// DocumentSearchConfig 是菜单范围搜索 service 配置。
type DocumentSearchConfig struct {
	Store DocumentSearchStore
}

// DocumentSearchRequest 是固定菜单范围搜索请求，不包含可由前端任意传入的 doc_type。
type DocumentSearchRequest struct {
	Keyword string
	Symbols []string
	Limit   int
	Offset  int
	Sort    string
}

// DocumentSearchResult 是菜单范围搜索结果，展示字段来自 search_documents 元数据。
type DocumentSearchResult struct {
	DocUID     string
	DocType    string
	RefID      string
	Symbol     string
	Title      string
	Summary    string
	Source     string
	SourceTime time.Time
	Score      float64
	Highlights []string
}

// ScopedDocumentSearchService 提供 report、news、watchlist_note 三个固定范围搜索入口。
type ScopedDocumentSearchService struct {
	store DocumentSearchStore
}

// NewScopedDocumentSearchService 创建固定范围文档搜索 service。
func NewScopedDocumentSearchService(config DocumentSearchConfig) *ScopedDocumentSearchService {
	return &ScopedDocumentSearchService{store: config.Store}
}

// SearchReports 搜索报告历史范围，只返回 report 文档。
func (service *ScopedDocumentSearchService) SearchReports(ctx context.Context, request DocumentSearchRequest) ([]DocumentSearchResult, error) {
	return service.searchByDocType(ctx, SearchDocTypeReport, request)
}

// SearchNews 搜索资讯中心范围，只返回 news 文档。
func (service *ScopedDocumentSearchService) SearchNews(ctx context.Context, request DocumentSearchRequest) ([]DocumentSearchResult, error) {
	return service.searchByDocType(ctx, SearchDocTypeNews, request)
}

// SearchWatchlistNotes 搜索自选备注范围，只返回 watchlist_note 文档。
func (service *ScopedDocumentSearchService) SearchWatchlistNotes(ctx context.Context, request DocumentSearchRequest) ([]DocumentSearchResult, error) {
	return service.searchByDocType(ctx, SearchDocTypeWatchlistNote, request)
}

// searchByDocType 绑定内部固定 doc_type，前端不能传入任意类型组合。
func (service *ScopedDocumentSearchService) searchByDocType(ctx context.Context, docType string, request DocumentSearchRequest) ([]DocumentSearchResult, error) {
	if service == nil || service.store == nil {
		return nil, fmt.Errorf("search document store is required")
	}
	query := NormalizeQueryInput(request.Keyword)
	match := BuildFTSMatch(query, []string{"title_index", "body_index", "tag_index", "pinyin_index", "symbol"})
	if match == "" {
		return nil, nil
	}
	batchID, err := service.activeBatchID(ctx)
	if err != nil {
		return nil, err
	}
	limit := normalizeDocumentSearchLimit(request.Limit)
	offset := normalizeDocumentSearchOffset(request.Offset)
	matches, err := service.store.SearchDocumentsFTS(ctx, batchID, docType, match, documentSearchFetchLimit(limit, offset))
	if err != nil {
		return nil, err
	}
	docUIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.DocType == docType {
			docUIDs = append(docUIDs, match.DocUID)
		}
	}
	documents, err := service.store.ListSearchDocumentsByUIDs(ctx, batchID, docUIDs)
	if err != nil {
		return nil, err
	}
	return paginateDocumentSearchResults(buildDocumentSearchResults(documents, docType, request.Symbols), offset, limit), nil
}

// activeBatchID 读取当前可查询的菜单范围文档索引 batch。
func (service *ScopedDocumentSearchService) activeBatchID(ctx context.Context) (string, error) {
	batchID, ok, err := service.store.GetSearchIndexState(ctx, ActiveDocumentBatchStateKey)
	if err != nil {
		return "", err
	}
	batchID = strings.TrimSpace(batchID)
	if !ok || batchID == "" {
		return "", fmt.Errorf("active document search batch is not available")
	}
	return batchID, nil
}

// buildDocumentSearchResults 将搜索文档元数据转换为前端可展示结果。
func buildDocumentSearchResults(documents []model.SearchDocument, docType string, symbols []string) []DocumentSearchResult {
	symbolSet := normalizedSymbolSet(symbols)
	results := make([]DocumentSearchResult, 0, len(documents))
	for index, document := range documents {
		if document.DocType != docType {
			continue
		}
		if len(symbolSet) > 0 {
			if _, ok := symbolSet[document.Symbol]; !ok {
				continue
			}
		}
		results = append(results, DocumentSearchResult{
			DocUID:     document.DocUID,
			DocType:    document.DocType,
			RefID:      document.RefID,
			Symbol:     document.Symbol,
			Title:      document.Title,
			Summary:    document.Summary,
			Source:     document.Source,
			SourceTime: document.SourceTime,
			Score:      1 / float64(index+1),
			Highlights: nil,
		})
	}
	return results
}

// paginateDocumentSearchResults 在过滤后执行 offset/limit，保证 symbol 过滤后的分页稳定。
func paginateDocumentSearchResults(results []DocumentSearchResult, offset int, limit int) []DocumentSearchResult {
	if offset >= len(results) {
		return nil
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}
	return results[offset:end]
}

// normalizedSymbolSet 归一化 symbol 过滤集合。
func normalizedSymbolSet(symbols []string) map[string]struct{} {
	result := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol != "" {
			result[symbol] = struct{}{}
		}
	}
	return result
}

// normalizeDocumentSearchLimit 归一化菜单范围搜索 limit。
func normalizeDocumentSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultDocumentSearchLimit
	}
	if limit > maxDocumentSearchLimit {
		return maxDocumentSearchLimit
	}
	return limit
}

// normalizeDocumentSearchOffset 归一化菜单范围搜索 offset。
func normalizeDocumentSearchOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// documentSearchFetchLimit 为过滤后分页预留足够 FTS 命中，避免小 offset 直接截断。
func documentSearchFetchLimit(limit int, offset int) int {
	fetchLimit := limit + offset + 50
	if fetchLimit < defaultDocumentSearchLimit {
		return defaultDocumentSearchLimit
	}
	return fetchLimit
}
