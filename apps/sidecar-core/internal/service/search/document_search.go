package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
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

// NewsSearchMetadataStore 定义资讯搜索结果回源补展示标签所需的可选 DAO 能力。
type NewsSearchMetadataStore interface {
	ListNewsItemsByIDs(ctx context.Context, ids []int64) (map[int64]model.NewsItem, error)
}

// DocumentSearchConfig 是菜单范围搜索 service 配置。
type DocumentSearchConfig struct {
	Store     DocumentSearchStore
	Tokenizer Tokenizer
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
	URL        string
	Symbol     string
	Title      string
	Summary    string
	Source     string
	SourceTime time.Time
	Score      float64
	Tags       []string
	Sentiment  string
	Highlights []string
}

// ScopedDocumentSearchService 提供 report、news、watchlist_note 三个固定范围搜索入口。
type ScopedDocumentSearchService struct {
	store     DocumentSearchStore
	tokenizer Tokenizer
}

// NewScopedDocumentSearchService 创建固定范围文档搜索 service。
func NewScopedDocumentSearchService(config DocumentSearchConfig) *ScopedDocumentSearchService {
	tokenizer := config.Tokenizer
	if tokenizer == nil {
		tokenizer = DefaultTokenizer()
	}
	return &ScopedDocumentSearchService{store: config.Store, tokenizer: tokenizer}
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
	results, err := buildDocumentSearchResults(ctx, service.store, documents, docType, request.Symbols)
	if err != nil {
		return nil, err
	}
	return paginateDocumentSearchResults(results, offset, limit), nil
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
func buildDocumentSearchResults(ctx context.Context, store DocumentSearchStore, documents []model.SearchDocument, docType string, symbols []string) ([]DocumentSearchResult, error) {
	symbolSet := normalizedSymbolSet(symbols)
	newsMetadata, err := loadNewsSearchMetadata(ctx, store, docType, documents)
	if err != nil {
		return nil, err
	}
	results := make([]DocumentSearchResult, 0, len(documents))
	for index, document := range documents {
		if document.DocType != docType {
			continue
		}
		metadata := newsMetadata[document.RefID]
		if len(symbolSet) > 0 {
			if !documentMatchesAnySymbol(document, metadata, symbolSet) {
				continue
			}
		}
		results = append(results, DocumentSearchResult{
			DocUID:     document.DocUID,
			DocType:    document.DocType,
			RefID:      document.RefID,
			URL:        metadata.URL,
			Symbol:     document.Symbol,
			Title:      document.Title,
			Summary:    document.Summary,
			Source:     document.Source,
			SourceTime: document.SourceTime,
			Score:      1 / float64(index+1),
			Tags:       metadata.Tags,
			Sentiment:  metadata.Sentiment,
			Highlights: nil,
		})
	}
	return results, nil
}

type newsSearchMetadata struct {
	Tags      []string
	Symbols   []string
	Sentiment string
	URL       string
}

// loadNewsSearchMetadata 按搜索文档 ref_id 回源新闻缓存，补齐索引元数据未保存的业务标签。
func loadNewsSearchMetadata(ctx context.Context, store DocumentSearchStore, docType string, documents []model.SearchDocument) (map[string]newsSearchMetadata, error) {
	result := make(map[string]newsSearchMetadata)
	if docType != SearchDocTypeNews {
		return result, nil
	}
	metadataStore, ok := store.(NewsSearchMetadataStore)
	if !ok {
		return result, nil
	}
	ids := make([]int64, 0, len(documents))
	refs := make(map[int64]string, len(documents))
	for _, document := range documents {
		if document.DocType != SearchDocTypeNews {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(document.RefID), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
		refs[id] = document.RefID
	}
	items, err := metadataStore.ListNewsItemsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, item := range items {
		symbols := decodeSearchList(item.Symbols)
		result[refs[id]] = newsSearchMetadata{
			Tags:      mergeSearchTags(symbols, decodeSearchList(item.Tags)),
			Symbols:   symbols,
			Sentiment: newsservice.AnalyzeSentiment(strings.TrimSpace(item.Title + "\n" + item.Summary)).Label,
			URL:       strings.TrimSpace(item.URL),
		}
	}
	return result, nil
}

// documentMatchesAnySymbol 使用完整新闻缓存 symbols 做个股过滤，避免索引单值字段漏掉多股票资讯。
func documentMatchesAnySymbol(document model.SearchDocument, metadata newsSearchMetadata, symbolSet map[string]struct{}) bool {
	for _, symbol := range metadata.Symbols {
		if _, ok := symbolSet[strings.TrimSpace(symbol)]; ok {
			return true
		}
	}
	if _, ok := symbolSet[document.Symbol]; ok {
		return true
	}
	return false
}

// decodeSearchList 读取 JSON 数组或早期逗号分隔字段，供搜索结果展示标签复用。
func decodeSearchList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.Split(raw, ",")
}

// mergeSearchTags 合并 symbol 和业务标签，保持顺序并去重。
func mergeSearchTags(groups ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
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
