package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// SearchIndexRebuildTaskType 是搜索索引全量重建任务类型。
	SearchIndexRebuildTaskType = "SEARCH_INDEX_REBUILD"

	// SearchRebuildScopeAll 表示重建首批所有搜索范围。
	SearchRebuildScopeAll = "all"
	// SearchRebuildScopeStock 表示只重建股票搜索索引。
	SearchRebuildScopeStock = "stock"
	// SearchRebuildScopeReports 表示重建报告历史所在的完整文档索引 batch。
	SearchRebuildScopeReports = "reports"
	// SearchRebuildScopeNews 表示重建资讯中心所在的完整文档索引 batch。
	SearchRebuildScopeNews = "news"
	// SearchRebuildScopeWatchlistNotes 表示重建自选备注所在的完整文档索引 batch。
	SearchRebuildScopeWatchlistNotes = "watchlist_notes"

	// SearchIndexFTS5StateKey 保存当前 SQLite FTS5 能力状态。
	SearchIndexFTS5StateKey = "fts5_status"
	// SearchFTS5StatusAvailable 表示 FTS5 可用。
	SearchFTS5StatusAvailable = "AVAILABLE"
	// SearchFTS5StatusUnavailable 表示 FTS5 不可用。
	SearchFTS5StatusUnavailable = "UNAVAILABLE"
)

var (
	// ErrSearchRebuildAlreadyRunning 表示同一时间已有搜索索引重建任务在运行。
	ErrSearchRebuildAlreadyRunning = errors.New("search rebuild already running")
	// ErrSearchFTS5Unavailable 表示当前 SQLite 不支持 FTS5，不能触发菜单范围搜索重建。
	ErrSearchFTS5Unavailable = errors.New("search fts5 unavailable")
)

// SearchRebuildStore 定义全量搜索索引重建需要的 DAO 能力。
type SearchRebuildStore interface {
	SaveTask(ctx context.Context, task *model.Task) error
	AppendTaskEvent(ctx context.Context, event *model.TaskEvent) error
	SaveSearchIndexBatch(ctx context.Context, batch *model.SearchIndexBatch) error
	GetSearchIndexState(ctx context.Context, key string) (string, bool, error)
	ListAllStocksForSearch(ctx context.Context) ([]model.Stock, error)
	ListStockAliasesBySymbols(ctx context.Context, symbols []string) (map[string][]model.StockAlias, error)
	ReplaceStockSearchFTS(ctx context.Context, batchID string, rows []dao.StockSearchFTSRow) error
	ListMarketNews(ctx context.Context, market string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
	ListVisibleAnalysisReports(ctx context.Context) ([]model.AnalysisReport, error)
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	UpsertSearchDocument(ctx context.Context, document *model.SearchDocument) error
	ReplaceSearchDocumentFTS(ctx context.Context, row dao.SearchDocumentFTSRow) error
	ActivateSearchIndexBatches(ctx context.Context, stockBatchID string, documentBatchID string) error
}

// SearchTaskStatusStore 定义搜索重建状态检查需要的任务查询能力。
type SearchTaskStatusStore interface {
	ListTasksByStatus(ctx context.Context, status string) ([]model.Task, error)
}

// SearchIndexOverviewStore 定义设置中心索引状态概览需要的只读 DAO 能力。
type SearchIndexOverviewStore interface {
	GetSearchIndexOverview(ctx context.Context, stockBatchID string, documentBatchID string) (dao.SearchIndexOverview, error)
}

// SearchRebuildConfig 是搜索索引重建管理器配置。
type SearchRebuildConfig struct {
	Store     SearchRebuildStore
	Tokenizer Tokenizer
	Now       func() time.Time
	NewID     func(prefix string) string
}

// SearchRebuildResult 是一次重建完成后的关键指针。
type SearchRebuildResult struct {
	TaskID          string
	StockBatchID    string
	DocumentBatchID string
}

// SearchStatus 是设置中心索引管理入口展示的搜索索引状态。
type SearchStatus struct {
	FTS5Status            string `json:"fts5_status"`
	GSEStatus             string `json:"gse_status"`
	SearchStatus          string `json:"search_status"`
	ActiveStockBatchID    string `json:"active_stock_batch_id"`
	ActiveDocumentBatchID string `json:"active_document_batch_id"`
	RunningRebuildTaskID  string `json:"running_rebuild_task_id"`
	StockIndexCount       int64  `json:"stock_index_count"`
	ReportIndexCount      int64  `json:"report_index_count"`
	NewsIndexCount        int64  `json:"news_index_count"`
	WatchlistNoteCount    int64  `json:"watchlist_note_index_count"`
	LastRebuildAt         string `json:"last_rebuild_at"`
	TokenizerName         string `json:"tokenizer_name"`
	TokenizerVersion      string `json:"tokenizer_version"`
	DictionaryHash        string `json:"dictionary_hash"`
}

// SearchRebuildRequest 是触发搜索索引重建的 service 请求。
type SearchRebuildRequest struct {
	Scope string
}

// SearchRebuildAccepted 是重建触发后的 API 可返回摘要。
type SearchRebuildAccepted struct {
	TaskID          string `json:"task_id"`
	Scope           string `json:"scope"`
	StockBatchID    string `json:"stock_batch_id"`
	DocumentBatchID string `json:"document_batch_id"`
}

// SearchRebuildManager 编排搜索索引全量重建和 active batch 切换。
type SearchRebuildManager struct {
	store     SearchRebuildStore
	tokenizer Tokenizer
	metadata  TokenizerMetadata
	now       func() time.Time
	newID     func(prefix string) string
}

type searchRebuildPlan struct {
	Scope                 string
	StockBatchID          string
	DocumentBatchID       string
	RebuildStockBatch     bool
	RebuildDocumentBatch  bool
	RebuildDocumentReason string
}

// NewSearchRebuildManager 创建搜索索引重建管理器。
func NewSearchRebuildManager(config SearchRebuildConfig) *SearchRebuildManager {
	tokenizer := config.Tokenizer
	if tokenizer == nil {
		tokenizer = SimpleTokenizer{}
	}
	metadata := tokenizerMetadata(tokenizer)
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := config.NewID
	if newID == nil {
		newID = defaultSearchRebuildID
	}
	return &SearchRebuildManager{
		store:     config.Store,
		tokenizer: tokenizer,
		metadata:  metadata,
		now:       now,
		newID:     newID,
	}
}

// Status 返回当前搜索索引状态；缺失 active batch 时提示需要重建。
func (service *ScopedDocumentSearchService) Status(ctx context.Context) (SearchStatus, error) {
	if service == nil || service.store == nil {
		return SearchStatus{}, fmt.Errorf("search document store is required")
	}
	status := SearchStatus{
		FTS5Status:   SearchFTS5StatusAvailable,
		GSEStatus:    "UNKNOWN",
		SearchStatus: "READY",
	}
	if value, ok, err := service.store.GetSearchIndexState(ctx, SearchIndexFTS5StateKey); err != nil {
		return SearchStatus{}, err
	} else if ok && value != "" {
		status.FTS5Status = value
	}
	if value, ok, err := service.store.GetSearchIndexState(ctx, activeStockBatchStateKey); err != nil {
		return SearchStatus{}, err
	} else if ok {
		status.ActiveStockBatchID = value
	}
	if value, ok, err := service.store.GetSearchIndexState(ctx, ActiveDocumentBatchStateKey); err != nil {
		return SearchStatus{}, err
	} else if ok {
		status.ActiveDocumentBatchID = value
	}
	if status.FTS5Status == SearchFTS5StatusUnavailable {
		status.SearchStatus = "UNAVAILABLE"
		return status, nil
	}
	if overviewStore, ok := service.store.(SearchIndexOverviewStore); ok {
		overview, err := overviewStore.GetSearchIndexOverview(ctx, status.ActiveStockBatchID, status.ActiveDocumentBatchID)
		if err != nil {
			return SearchStatus{}, err
		}
		status.applyOverview(overview)
	}
	if taskStatusStore, ok := service.store.(SearchTaskStatusStore); ok {
		tasks, err := taskStatusStore.ListTasksByStatus(ctx, "RUNNING")
		if err != nil {
			return SearchStatus{}, err
		}
		for _, task := range tasks {
			if task.Type == SearchIndexRebuildTaskType {
				status.SearchStatus = "BUILDING"
				status.RunningRebuildTaskID = task.ID
				return status, nil
			}
		}
	}
	if status.ActiveStockBatchID == "" || status.ActiveDocumentBatchID == "" {
		status.SearchStatus = "NEED_REBUILD"
	}
	return status, nil
}

// applyOverview 合并 active 索引的统计和词典元数据，状态字段仍由健康检查主流程决定。
func (status *SearchStatus) applyOverview(overview dao.SearchIndexOverview) {
	status.StockIndexCount = overview.StockCount
	status.ReportIndexCount = overview.ReportCount
	status.NewsIndexCount = overview.NewsCount
	status.WatchlistNoteCount = overview.WatchlistNoteCount
	status.TokenizerName = overview.TokenizerName
	status.TokenizerVersion = overview.TokenizerVersion
	status.DictionaryHash = overview.DictionaryHash
	status.GSEStatus = inferGSEStatus(overview.TokenizerName)
	if overview.LastRebuildAt != nil {
		status.LastRebuildAt = overview.LastRebuildAt.UTC().Format(time.RFC3339)
	}
}

// inferGSEStatus 根据 active batch 记录的 tokenizer 名称推导 GSE 是否真正参与索引。
func inferGSEStatus(tokenizerName string) string {
	switch strings.ToLower(strings.TrimSpace(tokenizerName)) {
	case "gse":
		return "AVAILABLE"
	case "simple":
		return "FALLBACK"
	case "":
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

// Rebuild 触发搜索索引重建；当前 service 阶段只实际执行 all 范围。
func (service *ScopedDocumentSearchService) Rebuild(ctx context.Context, request SearchRebuildRequest) (SearchRebuildAccepted, error) {
	status, err := service.Status(ctx)
	if err != nil {
		return SearchRebuildAccepted{}, err
	}
	if status.FTS5Status == SearchFTS5StatusUnavailable {
		return SearchRebuildAccepted{}, ErrSearchFTS5Unavailable
	}
	if status.RunningRebuildTaskID != "" {
		return SearchRebuildAccepted{}, ErrSearchRebuildAlreadyRunning
	}
	rebuildStore, ok := service.store.(SearchRebuildStore)
	if !ok {
		return SearchRebuildAccepted{}, fmt.Errorf("search rebuild store is required")
	}
	manager := NewSearchRebuildManager(SearchRebuildConfig{Store: rebuildStore, Tokenizer: service.tokenizer})
	result, err := manager.Rebuild(ctx, request.Scope)
	if err != nil {
		return SearchRebuildAccepted{}, err
	}
	return SearchRebuildAccepted{
		TaskID:          result.TaskID,
		Scope:           request.Scope,
		StockBatchID:    result.StockBatchID,
		DocumentBatchID: result.DocumentBatchID,
	}, nil
}

// Rebuild 同步执行指定范围的索引重建，文档类 scope 会重建完整 document batch。
func (manager *SearchRebuildManager) Rebuild(ctx context.Context, scope string) (SearchRebuildResult, error) {
	if manager == nil || manager.store == nil {
		return SearchRebuildResult{}, fmt.Errorf("search rebuild store is required")
	}
	plan, err := manager.plan(ctx, scope)
	if err != nil {
		return SearchRebuildResult{}, err
	}

	taskID := manager.newID("search-rebuild")
	result := SearchRebuildResult{TaskID: taskID, StockBatchID: plan.StockBatchID, DocumentBatchID: plan.DocumentBatchID}
	task := manager.newTask(taskID, "RUNNING")
	if err := manager.store.SaveTask(ctx, &task); err != nil {
		return result, err
	}
	if err := manager.appendTaskEvent(ctx, taskID, "TASK_STARTED", "search index rebuild started"); err != nil {
		return result, err
	}

	if err := manager.saveBuildingBatches(ctx, plan); err != nil {
		return result, manager.fail(ctx, task, plan, err)
	}
	if plan.RebuildStockBatch {
		if err := manager.rebuildStockIndex(ctx, plan.StockBatchID); err != nil {
			return result, manager.fail(ctx, task, plan, err)
		}
	}
	if plan.RebuildDocumentBatch {
		if err := manager.rebuildDocumentIndex(ctx, plan.DocumentBatchID); err != nil {
			return result, manager.fail(ctx, task, plan, err)
		}
	}
	if err := manager.markBatchesReady(ctx, plan); err != nil {
		return result, manager.fail(ctx, task, plan, err)
	}
	if err := manager.store.ActivateSearchIndexBatches(ctx, plan.StockBatchID, plan.DocumentBatchID); err != nil {
		return result, manager.fail(ctx, task, plan, err)
	}

	finished := manager.now()
	task.Status = "SUCCESS"
	task.Progress = 100
	task.FinishedAt = finished
	task.UpdatedAt = finished
	if err := manager.store.SaveTask(ctx, &task); err != nil {
		return result, err
	}
	if err := manager.appendTaskEvent(ctx, taskID, "TASK_SUCCESS", "search index rebuild completed"); err != nil {
		return result, err
	}
	return result, nil
}

// plan 生成重建计划；文档类范围必须重建完整 document batch，避免其他菜单索引被清空。
func (manager *SearchRebuildManager) plan(ctx context.Context, scope string) (searchRebuildPlan, error) {
	plan := searchRebuildPlan{Scope: scope}
	switch scope {
	case SearchRebuildScopeAll:
		plan.StockBatchID = manager.newID("stock")
		plan.DocumentBatchID = manager.newID("document")
		plan.RebuildStockBatch = true
		plan.RebuildDocumentBatch = true
	case SearchRebuildScopeStock:
		documentBatchID, err := manager.currentBatchID(ctx, ActiveDocumentBatchStateKey)
		if err != nil {
			return plan, err
		}
		plan.StockBatchID = manager.newID("stock")
		plan.DocumentBatchID = documentBatchID
		plan.RebuildStockBatch = true
	case SearchRebuildScopeReports, SearchRebuildScopeNews, SearchRebuildScopeWatchlistNotes:
		stockBatchID, err := manager.currentBatchID(ctx, activeStockBatchStateKey)
		if err != nil {
			return plan, err
		}
		plan.StockBatchID = stockBatchID
		plan.DocumentBatchID = manager.newID("document")
		plan.RebuildDocumentBatch = true
		plan.RebuildDocumentReason = scope
	default:
		return plan, fmt.Errorf("unsupported search rebuild scope %q", scope)
	}
	return plan, nil
}

// currentBatchID 读取未被本次重建替换的一侧 active batch。
func (manager *SearchRebuildManager) currentBatchID(ctx context.Context, key string) (string, error) {
	batchID, ok, err := manager.store.GetSearchIndexState(ctx, key)
	if err != nil {
		return "", err
	}
	if !ok || batchID == "" {
		return "", fmt.Errorf("search active batch %s is required", key)
	}
	return batchID, nil
}

// newTask 创建搜索重建任务记录。
func (manager *SearchRebuildManager) newTask(taskID string, status string) model.Task {
	now := manager.now()
	return model.Task{
		ID:        taskID,
		Type:      SearchIndexRebuildTaskType,
		Status:    status,
		Title:     "搜索索引重建",
		Progress:  0,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// saveBuildingBatches 创建本次重建的 BUILDING batch。
func (manager *SearchRebuildManager) saveBuildingBatches(ctx context.Context, plan searchRebuildPlan) error {
	for _, batch := range manager.plannedBatches(plan, dao.SearchIndexBatchStatusBuilding) {
		if err := manager.store.SaveSearchIndexBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// markBatchesReady 标记两个 batch 均已构建完成，可进入短事务切换。
func (manager *SearchRebuildManager) markBatchesReady(ctx context.Context, plan searchRebuildPlan) error {
	finished := manager.now()
	for _, batch := range manager.plannedBatches(plan, dao.SearchIndexBatchStatusReady) {
		batch.FinishedAt = &finished
		if err := manager.store.SaveSearchIndexBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// markBatchesFailed 标记重建失败，查询仍使用旧 active batch。
func (manager *SearchRebuildManager) markBatchesFailed(ctx context.Context, plan searchRebuildPlan) error {
	finished := manager.now()
	for _, batch := range manager.plannedBatches(plan, dao.SearchIndexBatchStatusFailed) {
		batch.FinishedAt = &finished
		batch.ErrorMessage = "search index rebuild failed"
		if err := manager.store.SaveSearchIndexBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// plannedBatches 返回本次实际重建的新 batch，不包含复用的当前 active batch。
func (manager *SearchRebuildManager) plannedBatches(plan searchRebuildPlan, status string) []*model.SearchIndexBatch {
	var batches []*model.SearchIndexBatch
	if plan.RebuildStockBatch {
		batches = append(batches, manager.newBatch(plan.StockBatchID, "stock", status))
	}
	if plan.RebuildDocumentBatch {
		batches = append(batches, manager.newBatch(plan.DocumentBatchID, "document", status))
	}
	return batches
}

// newBatch 创建搜索索引 batch 元数据。
func (manager *SearchRebuildManager) newBatch(batchID string, scope string, status string) *model.SearchIndexBatch {
	now := manager.now()
	return &model.SearchIndexBatch{
		BatchID:             batchID,
		Scope:               scope,
		Status:              status,
		SourceSchemaVersion: 1,
		TokenizerName:       manager.metadata.Name,
		TokenizerVersion:    manager.metadata.Version,
		DictionaryHash:      manager.metadata.DictionaryHash,
		StartedAt:           now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// tokenizerMetadata 返回索引 batch 可追溯的分词器元数据。
func tokenizerMetadata(tokenizer Tokenizer) TokenizerMetadata {
	if tokenizerWithMetadata, ok := tokenizer.(MetadataTokenizer); ok {
		metadata := tokenizerWithMetadata.Metadata()
		if metadata.Name != "" && metadata.Version != "" && metadata.DictionaryHash != "" {
			return metadata
		}
	}
	return SimpleTokenizer{}.Metadata()
}

// rebuildStockIndex 重建股票 FTS batch。
func (manager *SearchRebuildManager) rebuildStockIndex(ctx context.Context, batchID string) error {
	stocks, err := manager.store.ListAllStocksForSearch(ctx)
	if err != nil {
		return err
	}
	symbols := stockSymbols(stocks)
	aliases, err := manager.store.ListStockAliasesBySymbols(ctx, symbols)
	if err != nil {
		return err
	}
	rows := make([]dao.StockSearchFTSRow, 0, len(stocks))
	for _, stock := range stocks {
		rows = append(rows, BuildStockSearchFTSRow(stock, aliases[stock.Symbol], nil, manager.tokenizer))
	}
	return manager.store.ReplaceStockSearchFTS(ctx, batchID, rows)
}

// rebuildDocumentIndex 重建菜单范围文档 batch。
func (manager *SearchRebuildManager) rebuildDocumentIndex(ctx context.Context, batchID string) error {
	newsItems, err := manager.store.ListMarketNews(ctx, "", 0, 0)
	if err != nil {
		return err
	}
	for _, item := range newsItems {
		document, row := manager.newsDocument(batchID, item)
		if err := manager.writeDocument(ctx, document, row); err != nil {
			return err
		}
	}

	reports, err := manager.store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		return err
	}
	for _, report := range reports {
		document, row := manager.reportDocument(batchID, report)
		if err := manager.writeDocument(ctx, document, row); err != nil {
			return err
		}
	}

	watchlists, err := manager.store.ListActiveWatchlists(ctx)
	if err != nil {
		return err
	}
	for _, item := range watchlists {
		document, row := manager.watchlistDocument(batchID, item)
		if err := manager.writeDocument(ctx, document, row); err != nil {
			return err
		}
	}
	return nil
}

// writeDocument 写入文档元数据和对应 FTS 行。
func (manager *SearchRebuildManager) writeDocument(ctx context.Context, document model.SearchDocument, row dao.SearchDocumentFTSRow) error {
	if err := manager.store.UpsertSearchDocument(ctx, &document); err != nil {
		return err
	}
	return manager.store.ReplaceSearchDocumentFTS(ctx, row)
}

// newsDocument 构造资讯中心搜索文档。
func (manager *SearchRebuildManager) newsDocument(batchID string, item model.NewsItem) (model.SearchDocument, dao.SearchDocumentFTSRow) {
	document := model.SearchDocument{
		BatchID:      batchID,
		DocUID:       buildDocumentUID(SearchDocTypeNews, item.ID),
		DocType:      SearchDocTypeNews,
		RefTable:     "news_items",
		RefID:        fmt.Sprintf("%d", item.ID),
		Symbol:       firstDelimitedValue(item.Symbols),
		Title:        item.Title,
		Summary:      item.Summary,
		Source:       item.Source,
		SourceTime:   firstNonZeroTime(item.PublishedAt, item.UpdatedAt, item.CreatedAt),
		IndexedAt:    manager.now(),
		IndexVersion: 1,
	}
	row := buildSearchDocumentFTSRow(manager.tokenizer, document, []string{item.Summary}, []string{item.Tags})
	row.BatchID = batchID
	return document, row
}

// reportDocument 构造报告历史搜索文档，只使用脱敏摘要和风险摘要。
func (manager *SearchRebuildManager) reportDocument(batchID string, report model.AnalysisReport) (model.SearchDocument, dao.SearchDocumentFTSRow) {
	summary := BuildReportSearchSummary(report)
	document := model.SearchDocument{
		BatchID:      batchID,
		DocUID:       buildDocumentUID(SearchDocTypeReport, report.ID),
		DocType:      SearchDocTypeReport,
		RefTable:     "analysis_reports",
		RefID:        fmt.Sprintf("%d", report.ID),
		Symbol:       report.Symbol,
		Title:        report.Title,
		Summary:      summary,
		Source:       report.ModelName,
		SourceTime:   firstNonZeroTime(report.UpdatedAt, report.CreatedAt),
		IndexedAt:    manager.now(),
		IndexVersion: 1,
	}
	row := buildSearchDocumentFTSRow(manager.tokenizer, document, []string{summary, report.RiskSummary}, nil)
	row.BatchID = batchID
	return document, row
}

// watchlistDocument 构造自选备注搜索文档。
func (manager *SearchRebuildManager) watchlistDocument(batchID string, item model.Watchlist) (model.SearchDocument, dao.SearchDocumentFTSRow) {
	document := model.SearchDocument{
		BatchID:      batchID,
		DocUID:       buildDocumentUID(SearchDocTypeWatchlistNote, item.ID),
		DocType:      SearchDocTypeWatchlistNote,
		RefTable:     "watchlists",
		RefID:        fmt.Sprintf("%d", item.ID),
		Symbol:       item.Symbol,
		Title:        item.Symbol + " 自选备注",
		Summary:      item.Note,
		SourceTime:   firstNonZeroTime(item.UpdatedAt, item.CreatedAt),
		IndexedAt:    manager.now(),
		IndexVersion: 1,
	}
	row := buildSearchDocumentFTSRow(manager.tokenizer, document, []string{item.Note}, []string{item.Tags})
	row.BatchID = batchID
	return document, row
}

// fail 统一落失败任务和 FAILED batch，但不切换 active batch。
func (manager *SearchRebuildManager) fail(ctx context.Context, task model.Task, plan searchRebuildPlan, cause error) error {
	_ = manager.markBatchesFailed(ctx, plan)
	finished := manager.now()
	task.Status = "FAILED"
	task.ErrorMessage = "search index rebuild failed"
	task.FinishedAt = finished
	task.UpdatedAt = finished
	_ = manager.store.SaveTask(ctx, &task)
	_ = manager.appendTaskEvent(ctx, task.ID, "TASK_FAILED", "search index rebuild failed")
	return cause
}

// appendTaskEvent 追加搜索重建任务事件，payload 不写入原始错误文本。
func (manager *SearchRebuildManager) appendTaskEvent(ctx context.Context, taskID string, eventType string, payload string) error {
	return manager.store.AppendTaskEvent(ctx, &model.TaskEvent{
		TaskID:    taskID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: manager.now(),
		UpdatedAt: manager.now(),
	})
}

// defaultSearchRebuildID 生成本地重建 ID。
func defaultSearchRebuildID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}
