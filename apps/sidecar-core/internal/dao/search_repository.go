package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// SearchIndexBatchStatusBuilding 表示索引批次仍在构建中，不能参与查询。
	SearchIndexBatchStatusBuilding = "BUILDING"
	// SearchIndexBatchStatusReady 表示索引批次已构建完成，可以被切为 active batch。
	SearchIndexBatchStatusReady = "READY"
	// SearchIndexBatchStatusFailed 表示索引批次构建失败，必须保留旧 active batch。
	SearchIndexBatchStatusFailed = "FAILED"
	// SearchIndexBatchStatusRetired 表示索引批次已被新 active batch 替代，可延迟清理。
	SearchIndexBatchStatusRetired = "RETIRED"

	// SearchIndexJobStatusPending 表示 outbox 任务等待索引 worker 处理。
	SearchIndexJobStatusPending = "PENDING"
	// SearchIndexJobStatusRunning 表示 outbox 任务正在处理，进程重启后需要恢复。
	SearchIndexJobStatusRunning = "RUNNING"
	// SearchIndexJobStatusDone 表示 outbox 任务已成功应用到 FTS 索引。
	SearchIndexJobStatusDone = "DONE"
	// SearchIndexJobStatusFailedRetryable 表示 outbox 任务失败但仍可重试。
	SearchIndexJobStatusFailedRetryable = "FAILED_RETRYABLE"
	// SearchIndexJobStatusFailedFinal 表示 outbox 任务达到重试上限或不可恢复。
	SearchIndexJobStatusFailedFinal = "FAILED_FINAL"
)

// StockSearchFTSRow 是写入股票 FTS 虚表的预分词行，内容由 service 层构造。
type StockSearchFTSRow struct {
	Symbol         string
	Market         string
	Exchange       string
	Code           string
	CodePrefix     string
	NameIndex      string
	FullNameIndex  string
	AliasIndex     string
	PinyinFull     string
	PinyinInitials string
	IndustryIndex  string
	ConceptIndex   string
}

// StockSearchFTSMatch 是股票 FTS 查询命中的最小元数据。
type StockSearchFTSMatch struct {
	BatchID  string
	Symbol   string
	Market   string
	Exchange string
	Code     string
}

// SearchDocumentFTSRow 是写入菜单范围搜索 FTS 虚表的预分词行。
type SearchDocumentFTSRow struct {
	BatchID     string
	DocUID      string
	DocType     string
	Symbol      string
	TitleIndex  string
	BodyIndex   string
	TagIndex    string
	PinyinIndex string
}

// SearchDocumentFTSMatch 是菜单范围搜索 FTS 查询命中的最小元数据。
type SearchDocumentFTSMatch struct {
	BatchID string
	DocUID  string
	DocType string
	Symbol  string
}

// SearchIndexOverview 汇总当前 active 搜索索引的可观测状态，供设置中心展示。
type SearchIndexOverview struct {
	StockCount         int64
	ReportCount        int64
	NewsCount          int64
	WatchlistNoteCount int64
	LastRebuildAt      *time.Time
	TokenizerName      string
	TokenizerVersion   string
	DictionaryHash     string
}

// ListStocksForSearch 返回股票搜索候选；symbols 非空时按传入顺序回表。
func (store *Store) ListStocksForSearch(ctx context.Context, query string, symbols []string, limit int) ([]model.Stock, error) {
	if len(symbols) > 0 {
		var stocks []model.Stock
		if err := store.db.WithContext(ctx).Where("symbol IN ?", symbols).Find(&stocks).Error; err != nil {
			return nil, err
		}
		bySymbol := make(map[string]model.Stock, len(stocks))
		for _, stock := range stocks {
			bySymbol[stock.Symbol] = stock
		}
		result := make([]model.Stock, 0, len(symbols))
		for _, symbol := range symbols {
			if stock, ok := bySymbol[symbol]; ok {
				result = append(result, stock)
			}
		}
		return result, nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, nil
	}
	like := "%" + query + "%"
	db := store.db.WithContext(ctx).
		Where(
			`LOWER(symbol) LIKE ?
			 OR LOWER(code) LIKE ?
			 OR LOWER(exchange || code) LIKE ?
			 OR LOWER(name) LIKE ?
			 OR LOWER(full_name) LIKE ?
			 OR LOWER(search_name) LIKE ?
			 OR LOWER(pinyin_full) LIKE ?
			 OR LOWER(pinyin_initials) LIKE ?
			 OR LOWER(industry) LIKE ?
			 OR LOWER(concept) LIKE ?
			 OR symbol IN (
				SELECT symbol FROM stock_aliases
				WHERE deleted_at IS NULL AND LOWER(alias) LIKE ?
			 )`,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
		).
		Order("updated_at DESC").
		Order("code ASC")
	if limit > 0 {
		db = db.Limit(limit)
	}
	var stocks []model.Stock
	if err := db.Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

// ListAllStocksForSearch 返回全量股票基础信息，供搜索索引重建使用。
func (store *Store) ListAllStocksForSearch(ctx context.Context) ([]model.Stock, error) {
	var stocks []model.Stock
	if err := store.db.WithContext(ctx).
		Order("code ASC").
		Order("symbol ASC").
		Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

// ListStockAliasesBySymbols 按 symbol 批量读取未删除别名，并为缺失 symbol 返回空切片。
func (store *Store) ListStockAliasesBySymbols(ctx context.Context, symbols []string) (map[string][]model.StockAlias, error) {
	result := make(map[string][]model.StockAlias, len(symbols))
	if len(symbols) == 0 {
		return result, nil
	}
	for _, symbol := range symbols {
		result[symbol] = nil
	}
	var aliases []model.StockAlias
	if err := store.db.WithContext(ctx).
		Where("symbol IN ? AND deleted_at IS NULL", symbols).
		Order("symbol ASC").
		Order("id ASC").
		Find(&aliases).Error; err != nil {
		return nil, err
	}
	for _, alias := range aliases {
		result[alias.Symbol] = append(result[alias.Symbol], alias)
	}
	return result, nil
}

// SetSearchIndexState 幂等写入搜索索引状态，主要用于保存 active batch 指针和健康状态。
func (store *Store) SetSearchIndexState(ctx context.Context, key string, value string) error {
	if key == "" {
		return fmt.Errorf("dao search index state key is required")
	}
	now := time.Now().UTC()
	state := model.SearchIndexState{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"value":      value,
				"updated_at": now,
			}),
		}).
		Create(&state).Error
}

// GetSearchIndexState 读取搜索索引状态，缺失时返回 ok=false。
func (store *Store) GetSearchIndexState(ctx context.Context, key string) (string, bool, error) {
	var state model.SearchIndexState
	err := store.db.WithContext(ctx).Where("key = ?", key).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return state.Value, true, nil
}

// GetSearchIndexOverview 基于 active batch 读取索引数量、最近重建时间和词典元数据。
func (store *Store) GetSearchIndexOverview(ctx context.Context, stockBatchID string, documentBatchID string) (SearchIndexOverview, error) {
	overview := SearchIndexOverview{}
	if stockBatchID != "" {
		if err := store.db.WithContext(ctx).
			Table("stock_search_fts").
			Where("batch_id = ?", stockBatchID).
			Count(&overview.StockCount).Error; err != nil {
			return SearchIndexOverview{}, err
		}
	}
	if documentBatchID != "" {
		counts, err := store.countSearchDocumentsByType(ctx, documentBatchID)
		if err != nil {
			return SearchIndexOverview{}, err
		}
		overview.ReportCount = counts["report"]
		overview.NewsCount = counts["news"]
		overview.WatchlistNoteCount = counts["watchlist_note"]
	}
	if err := store.fillSearchIndexBatchOverview(ctx, &overview, stockBatchID, documentBatchID); err != nil {
		return SearchIndexOverview{}, err
	}
	return overview, nil
}

// countSearchDocumentsByType 按 doc_type 汇总 active 文档索引数量，只统计未软删除元数据。
func (store *Store) countSearchDocumentsByType(ctx context.Context, documentBatchID string) (map[string]int64, error) {
	type row struct {
		DocType string
		Count   int64
	}
	var rows []row
	if err := store.db.WithContext(ctx).
		Model(&model.SearchDocument{}).
		Select("doc_type, COUNT(*) AS count").
		Where("batch_id = ?", documentBatchID).
		Group("doc_type").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, item := range rows {
		counts[item.DocType] = item.Count
	}
	return counts, nil
}

// fillSearchIndexBatchOverview 读取 active batch 元数据，并以最近完成时间作为最后重建时间。
func (store *Store) fillSearchIndexBatchOverview(ctx context.Context, overview *SearchIndexOverview, batchIDs ...string) error {
	ids := make([]string, 0, len(batchIDs))
	for _, batchID := range batchIDs {
		if batchID != "" {
			ids = append(ids, batchID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var batches []model.SearchIndexBatch
	if err := store.db.WithContext(ctx).
		Where("batch_id IN ?", ids).
		Find(&batches).Error; err != nil {
		return err
	}
	for _, batch := range batches {
		if batch.FinishedAt != nil && (overview.LastRebuildAt == nil || batch.FinishedAt.After(*overview.LastRebuildAt)) {
			finishedAt := *batch.FinishedAt
			overview.LastRebuildAt = &finishedAt
		}
		if overview.TokenizerName == "" && batch.TokenizerName != "" {
			overview.TokenizerName = batch.TokenizerName
		}
		if overview.TokenizerVersion == "" && batch.TokenizerVersion != "" {
			overview.TokenizerVersion = batch.TokenizerVersion
		}
		if overview.DictionaryHash == "" && batch.DictionaryHash != "" {
			overview.DictionaryHash = batch.DictionaryHash
		}
	}
	return nil
}

// ReplaceStockSearchFTS 用一批预分词股票行替换指定 batch 的股票 FTS 内容。
func (store *Store) ReplaceStockSearchFTS(ctx context.Context, batchID string, rows []StockSearchFTSRow) error {
	if batchID == "" {
		return fmt.Errorf("dao stock search batch_id is required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM stock_search_fts WHERE batch_id = ?`, batchID).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.Exec(
				`INSERT INTO stock_search_fts (
					batch_id, symbol, market, exchange, code, code_prefix, name_index,
					full_name_index, alias_index, pinyin_full, pinyin_initials,
					industry_index, concept_index
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				batchID,
				row.Symbol,
				row.Market,
				row.Exchange,
				row.Code,
				row.CodePrefix,
				row.NameIndex,
				row.FullNameIndex,
				row.AliasIndex,
				row.PinyinFull,
				row.PinyinInitials,
				row.IndustryIndex,
				row.ConceptIndex,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SearchStockFTS 在指定 batch 内执行股票 FTS 查询，MATCH 字符串由上层 QueryBuilder 生成。
func (store *Store) SearchStockFTS(ctx context.Context, batchID string, match string, limit int) ([]StockSearchFTSMatch, error) {
	if batchID == "" {
		return nil, fmt.Errorf("dao stock search batch_id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	var matches []StockSearchFTSMatch
	err := store.db.WithContext(ctx).
		Raw(
			`SELECT batch_id, symbol, market, exchange, code
			 FROM stock_search_fts
			 WHERE stock_search_fts MATCH ? AND batch_id = ?
			 LIMIT ?`,
			match,
			batchID,
			limit,
		).
		Scan(&matches).Error
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// UpsertSearchDocument 幂等保存菜单范围搜索文档元数据，真实展示仍回源业务表。
func (store *Store) UpsertSearchDocument(ctx context.Context, document *model.SearchDocument) error {
	if document == nil {
		return fmt.Errorf("dao search document is required")
	}
	if document.BatchID == "" || document.DocUID == "" {
		return fmt.Errorf("dao search document batch_id and doc_uid are required")
	}
	now := time.Now().UTC()
	if document.IndexedAt.IsZero() {
		document.IndexedAt = now
	}
	if document.SourceTime.IsZero() {
		document.SourceTime = now
	}
	if document.IndexVersion == 0 {
		document.IndexVersion = 1
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "batch_id"},
				{Name: "doc_uid"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"doc_type",
				"ref_table",
				"ref_id",
				"symbol",
				"title",
				"summary",
				"source",
				"source_time",
				"indexed_at",
				"index_version",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(document).Error
}

// ReplaceSearchDocumentFTS 替换单个菜单文档在 FTS 虚表中的索引内容。
func (store *Store) ReplaceSearchDocumentFTS(ctx context.Context, row SearchDocumentFTSRow) error {
	if row.BatchID == "" || row.DocUID == "" {
		return fmt.Errorf("dao search document fts batch_id and doc_uid are required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM search_documents_fts WHERE batch_id = ? AND doc_uid = ?`, row.BatchID, row.DocUID).Error; err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO search_documents_fts (
				batch_id, doc_uid, doc_type, symbol, title_index, body_index, tag_index, pinyin_index
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			row.BatchID,
			row.DocUID,
			row.DocType,
			row.Symbol,
			row.TitleIndex,
			row.BodyIndex,
			row.TagIndex,
			row.PinyinIndex,
		).Error
	})
}

// SearchDocumentsFTS 在指定 batch 和菜单范围内执行文档 FTS 查询。
func (store *Store) SearchDocumentsFTS(ctx context.Context, batchID string, docType string, match string, limit int) ([]SearchDocumentFTSMatch, error) {
	if batchID == "" || docType == "" {
		return nil, fmt.Errorf("dao search document batch_id and doc_type are required")
	}
	if limit <= 0 {
		limit = 20
	}
	var matches []SearchDocumentFTSMatch
	err := store.db.WithContext(ctx).
		Raw(
			`SELECT batch_id, doc_uid, doc_type, symbol
			 FROM search_documents_fts
			 WHERE search_documents_fts MATCH ? AND batch_id = ? AND doc_type = ?
			 LIMIT ?`,
			match,
			batchID,
			docType,
			limit,
		).
		Scan(&matches).Error
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// ListSearchDocumentsByUIDs 按 FTS 命中的 doc_uid 顺序回表搜索文档元数据。
func (store *Store) ListSearchDocumentsByUIDs(ctx context.Context, batchID string, docUIDs []string) ([]model.SearchDocument, error) {
	if batchID == "" {
		return nil, fmt.Errorf("dao search document batch_id is required")
	}
	if len(docUIDs) == 0 {
		return nil, nil
	}
	var documents []model.SearchDocument
	if err := store.db.WithContext(ctx).
		Where("batch_id = ? AND doc_uid IN ?", batchID, docUIDs).
		Find(&documents).Error; err != nil {
		return nil, err
	}
	byUID := make(map[string]model.SearchDocument, len(documents))
	for _, document := range documents {
		byUID[document.DocUID] = document
	}
	result := make([]model.SearchDocument, 0, len(docUIDs))
	for _, docUID := range docUIDs {
		if document, ok := byUID[docUID]; ok {
			result = append(result, document)
		}
	}
	return result, nil
}

// SoftDeleteSearchDocument 软删除文档元数据，并同步移除对应 FTS 行。
func (store *Store) SoftDeleteSearchDocument(ctx context.Context, batchID string, docUID string) error {
	if batchID == "" || docUID == "" {
		return fmt.Errorf("dao search document batch_id and doc_uid are required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("batch_id = ? AND doc_uid = ?", batchID, docUID).
			Delete(&model.SearchDocument{}).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM search_documents_fts WHERE batch_id = ? AND doc_uid = ?`, batchID, docUID).Error
	})
}

// SaveSearchIndexBatch 幂等保存搜索索引批次元数据，不负责切换 active batch。
func (store *Store) SaveSearchIndexBatch(ctx context.Context, batch *model.SearchIndexBatch) error {
	if batch == nil {
		return fmt.Errorf("dao search index batch is required")
	}
	if batch.BatchID == "" {
		return fmt.Errorf("dao search index batch_id is required")
	}
	if batch.SourceSchemaVersion == 0 {
		batch.SourceSchemaVersion = 1
	}
	if batch.StartedAt.IsZero() {
		batch.StartedAt = time.Now().UTC()
	}
	batch.ErrorMessage = logger.RedactText(batch.ErrorMessage)

	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "batch_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"scope",
				"status",
				"source_schema_version",
				"tokenizer_name",
				"tokenizer_version",
				"dictionary_hash",
				"started_at",
				"finished_at",
				"error_message",
				"updated_at",
			}),
		}).
		Create(batch).Error
}

// ActivateSearchIndexBatches 在短事务中切换 active batch，并把旧 batch 标记为 RETIRED。
func (store *Store) ActivateSearchIndexBatches(ctx context.Context, stockBatchID string, documentBatchID string) error {
	if stockBatchID == "" || documentBatchID == "" {
		return fmt.Errorf("dao active stock and document batch are required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStore := Store{db: tx}
		if err := requireReadySearchIndexBatch(ctx, tx, stockBatchID, "stock"); err != nil {
			return err
		}
		if err := requireReadySearchIndexBatch(ctx, tx, documentBatchID, "document"); err != nil {
			return err
		}

		oldStockBatchID, _, err := txStore.GetSearchIndexState(ctx, "active_stock_batch_id")
		if err != nil {
			return err
		}
		oldDocumentBatchID, _, err := txStore.GetSearchIndexState(ctx, "active_document_batch_id")
		if err != nil {
			return err
		}
		if err := txStore.SetSearchIndexState(ctx, "active_stock_batch_id", stockBatchID); err != nil {
			return err
		}
		if err := txStore.SetSearchIndexState(ctx, "active_document_batch_id", documentBatchID); err != nil {
			return err
		}
		return retireSearchIndexBatches(ctx, tx, []string{oldStockBatchID, oldDocumentBatchID}, []string{stockBatchID, documentBatchID})
	})
}

// requireReadySearchIndexBatch 确认待激活 batch 已 READY 且属于目标 scope。
func requireReadySearchIndexBatch(ctx context.Context, tx *gorm.DB, batchID string, scope string) error {
	var batch model.SearchIndexBatch
	err := tx.WithContext(ctx).Where("batch_id = ? AND scope = ?", batchID, scope).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("dao search index batch %s not found", batchID)
	}
	if err != nil {
		return err
	}
	if batch.Status != SearchIndexBatchStatusReady {
		return fmt.Errorf("dao search index batch %s is not ready", batchID)
	}
	return nil
}

// retireSearchIndexBatches 将已被替换的旧 batch 标记为 RETIRED。
func retireSearchIndexBatches(ctx context.Context, tx *gorm.DB, oldBatchIDs []string, newBatchIDs []string) error {
	newSet := make(map[string]struct{}, len(newBatchIDs))
	for _, batchID := range newBatchIDs {
		if batchID != "" {
			newSet[batchID] = struct{}{}
		}
	}
	retireIDs := make([]string, 0, len(oldBatchIDs))
	seen := make(map[string]struct{}, len(oldBatchIDs))
	for _, batchID := range oldBatchIDs {
		if batchID == "" {
			continue
		}
		if _, ok := newSet[batchID]; ok {
			continue
		}
		if _, ok := seen[batchID]; ok {
			continue
		}
		seen[batchID] = struct{}{}
		retireIDs = append(retireIDs, batchID)
	}
	if len(retireIDs) == 0 {
		return nil
	}
	return tx.WithContext(ctx).
		Model(&model.SearchIndexBatch{}).
		Where("batch_id IN ?", retireIDs).
		Updates(map[string]any{
			"status":     SearchIndexBatchStatusRetired,
			"updated_at": time.Now().UTC(),
		}).Error
}

// UpsertSearchIndexJob 将业务变更写入搜索索引 outbox，同一对象同一操作会合并为一条待处理任务。
func (store *Store) UpsertSearchIndexJob(ctx context.Context, job model.SearchIndexJob) error {
	if job.DocType == "" || job.RefID == "" || job.Operation == "" {
		return fmt.Errorf("dao search index job doc_type, ref_id and operation are required")
	}
	if job.Status == "" {
		job.Status = SearchIndexJobStatusPending
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.LastError = logger.RedactText(job.LastError)

	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "doc_type"},
				{Name: "ref_id"},
				{Name: "operation"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"status":     job.Status,
				"attempts":   job.Attempts,
				"last_error": job.LastError,
				"updated_at": now,
			}),
		}).
		Create(&job).Error
}

// ListRunnableSearchIndexJobs 返回等待处理或可重试的搜索索引 outbox 任务。
func (store *Store) ListRunnableSearchIndexJobs(ctx context.Context, limit int) ([]model.SearchIndexJob, error) {
	query := store.db.WithContext(ctx).
		Where("status IN ?", []string{SearchIndexJobStatusPending, SearchIndexJobStatusFailedRetryable}).
		Order("updated_at ASC").
		Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var jobs []model.SearchIndexJob
	if err := query.Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// MarkSearchIndexJobStatus 更新 outbox 任务状态，并对错误信息做统一脱敏。
func (store *Store) MarkSearchIndexJobStatus(ctx context.Context, id int64, status string, lastError string) error {
	if id == 0 {
		return fmt.Errorf("dao search index job id is required")
	}
	return store.db.WithContext(ctx).
		Model(&model.SearchIndexJob{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     status,
			"last_error": logger.RedactText(lastError),
			"updated_at": time.Now().UTC(),
		}).Error
}

// RecoverRunningSearchIndexJobs 将崩溃前未完成的 RUNNING 任务恢复为可重试状态。
func (store *Store) RecoverRunningSearchIndexJobs(ctx context.Context) error {
	return store.db.WithContext(ctx).
		Model(&model.SearchIndexJob{}).
		Where("status = ?", SearchIndexJobStatusRunning).
		Updates(map[string]any{
			"status":     SearchIndexJobStatusFailedRetryable,
			"last_error": "search index job recovered after restart",
			"updated_at": time.Now().UTC(),
		}).Error
}
