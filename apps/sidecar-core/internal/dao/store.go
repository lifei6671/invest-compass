package dao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是 Go core dao 层的统一数据库操作入口。
type Store struct {
	db *gorm.DB
}

// NewStore 使用已初始化的 GORM 连接创建 repository 入口。
func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("dao db is required")
	}
	return &Store{db: db}, nil
}

// WithTransaction 在单个数据库事务内执行一组写操作。
func (store *Store) WithTransaction(ctx context.Context, run func(*Store) error) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return run(&Store{db: tx})
	})
}

// UpsertStocks 按标准 symbol 幂等写入股票基础信息缓存。
func (store *Store) UpsertStocks(ctx context.Context, stocks []model.Stock) error {
	if len(stocks) == 0 {
		return nil
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "symbol"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"market",
				"code",
				"name",
				"pinyin",
				"exchange",
				"industry",
				"concept",
				"list_date",
				"status",
				"updated_at",
			}),
		}).
		Create(&stocks).Error
}

// SaveQuote 按 symbol 保存最新行情快照，避免 quote 缓存表无限追加同一股票记录。
func (store *Store) SaveQuote(ctx context.Context, quote *model.Quote) error {
	if quote == nil {
		return fmt.Errorf("dao quote is required")
	}

	var existing model.Quote
	err := store.db.WithContext(ctx).Where("symbol = ?", quote.Symbol).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return store.db.WithContext(ctx).Create(quote).Error
	}
	if err != nil {
		return err
	}

	quote.ID = existing.ID
	quote.CreatedAt = existing.CreatedAt
	return store.db.WithContext(ctx).Save(quote).Error
}

// LatestQuote 返回指定 symbol 在最大缓存年龄内的最新 quote。
func (store *Store) LatestQuote(ctx context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error) {
	var quote model.Quote
	query := store.db.WithContext(ctx).Where("symbol = ?", symbol)
	if maxAge > 0 {
		query = query.Where("updated_at >= ?", time.Now().UTC().Add(-maxAge))
	}
	err := query.Order("updated_at DESC").First(&quote).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Quote{}, false, nil
	}
	if err != nil {
		return model.Quote{}, false, err
	}
	return quote, true, nil
}

// SaveKlines 按 symbol、period、adjust、trade_date 幂等保存 K 线缓存。
func (store *Store) SaveKlines(ctx context.Context, klines []model.Kline) error {
	if len(klines) == 0 {
		return nil
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "symbol"},
				{Name: "period"},
				{Name: "adjust"},
				{Name: "trade_date"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"open",
				"high",
				"low",
				"close",
				"volume",
				"amount",
				"provider",
				"updated_at",
			}),
		}).
		Create(&klines).Error
}

// ListKlines 返回指定股票、周期和复权方式下最近 limit 条 K 线，并按交易日升序排列。
func (store *Store) ListKlines(ctx context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error) {
	query := store.db.WithContext(ctx).
		Where("symbol = ? AND period = ? AND adjust = ?", symbol, period, adjust).
		Order("trade_date DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var klines []model.Kline
	if err := query.Find(&klines).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(klines, func(left int, right int) bool {
		return klines[left].TradeDate < klines[right].TradeDate
	})
	return klines, nil
}

// SaveNewsItems 按 content_hash 幂等写入新闻缓存，合并重复转载条目。
func (store *Store) SaveNewsItems(ctx context.Context, items []model.NewsItem) error {
	if len(items) == 0 {
		return nil
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "content_hash"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"source",
				"title",
				"url",
				"summary",
				"symbols",
				"tags",
				"published_at",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(&items).Error
}

// ListNewsBySymbol 返回指定 symbol 的新闻缓存，按发布时间倒序排列。
func (store *Store) ListNewsBySymbol(ctx context.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error) {
	query := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Where("(symbols = ? OR symbols LIKE ?)", symbol, "%\""+symbol+"\"%").
		Order("published_at DESC").
		Order("id DESC")
	if maxAge > 0 {
		query = query.Where("updated_at >= ?", time.Now().UTC().Add(-maxAge))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var items []model.NewsItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListMarketNews 返回市场新闻缓存，首版不单独维护市场字段，按全量新闻倒序读取。
func (store *Store) ListMarketNews(ctx context.Context, _ string, limit int, maxAge time.Duration) ([]model.NewsItem, error) {
	query := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("published_at DESC").
		Order("id DESC")
	if maxAge > 0 {
		query = query.Where("updated_at >= ?", time.Now().UTC().Add(-maxAge))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var items []model.NewsItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// SaveWatchlist 创建或更新自选股记录。
func (store *Store) SaveWatchlist(ctx context.Context, item *model.Watchlist) error {
	return store.db.WithContext(ctx).Save(item).Error
}

// ListActiveWatchlists 按 sort_order、id 返回未软删除自选股。
func (store *Store) ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error) {
	var items []model.Watchlist
	err := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("sort_order ASC").
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// SoftDeleteWatchlist 对自选股执行软删除。
func (store *Store) SoftDeleteWatchlist(ctx context.Context, id int64) error {
	return store.db.WithContext(ctx).Delete(&model.Watchlist{}, id).Error
}

// SaveAIConfig 创建或更新 AI 配置，不保存真实 API Key。
func (store *Store) SaveAIConfig(ctx context.Context, config *model.AIConfig) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if config.IsDefault {
			query := tx.Model(&model.AIConfig{}).Where("deleted_at IS NULL")
			if config.ID > 0 {
				query = query.Where("id <> ?", config.ID)
			}
			if err := query.Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(config).Error
	})
}

// ListAIConfigs 返回未软删除 AI 配置，并把默认配置排在前面。
func (store *Store) ListAIConfigs(ctx context.Context) ([]model.AIConfig, error) {
	var configs []model.AIConfig
	err := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("is_default DESC").
		Order("updated_at DESC").
		Order("id ASC").
		Find(&configs).Error
	return configs, err
}

// GetAIConfig 按 ID 返回未软删除 AI 配置，供 Rust 注入密钥后的测试和分析任务使用。
func (store *Store) GetAIConfig(ctx context.Context, id int64) (model.AIConfig, error) {
	var config model.AIConfig
	err := store.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&config).Error
	return config, err
}

// SoftDeleteAIConfig 对 AI 配置执行软删除。
func (store *Store) SoftDeleteAIConfig(ctx context.Context, id int64) error {
	return store.db.WithContext(ctx).Delete(&model.AIConfig{}, id).Error
}

// SavePromptTemplate 创建或更新 Prompt 模板。
func (store *Store) SavePromptTemplate(ctx context.Context, template *model.PromptTemplate) error {
	return store.db.WithContext(ctx).Save(template).Error
}

// ListPromptTemplates 返回未软删除 Prompt 模板。
func (store *Store) ListPromptTemplates(ctx context.Context) ([]model.PromptTemplate, error) {
	var templates []model.PromptTemplate
	err := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("updated_at DESC").
		Order("id ASC").
		Find(&templates).Error
	return templates, err
}

// GetPromptTemplate 按 ID 返回未软删除 Prompt 模板，供分析任务执行构建 Prompt。
func (store *Store) GetPromptTemplate(ctx context.Context, id int64) (model.PromptTemplate, error) {
	var template model.PromptTemplate
	err := store.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&template).Error
	return template, err
}

// SoftDeletePromptTemplate 对 Prompt 模板执行软删除。
func (store *Store) SoftDeletePromptTemplate(ctx context.Context, id int64) error {
	return store.db.WithContext(ctx).Delete(&model.PromptTemplate{}, id).Error
}

// SaveTask 创建或更新任务记录。
func (store *Store) SaveTask(ctx context.Context, task *model.Task) error {
	return store.db.WithContext(ctx).Save(task).Error
}

// SaveTaskWithEvent 在同一事务内保存任务状态和对应事件，保证 SSE 回放不会缺失状态事件。
func (store *Store) SaveTaskWithEvent(ctx context.Context, task *model.Task, event *model.TaskEvent) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStore := Store{db: tx}
		if err := txStore.SaveTask(ctx, task); err != nil {
			return err
		}
		return txStore.AppendTaskEvent(ctx, event)
	})
}

// ListTasksByStatus 按状态查询任务，供 RUNNING 恢复和任务历史筛选使用。
func (store *Store) ListTasksByStatus(ctx context.Context, status string) ([]model.Task, error) {
	var tasks []model.Task
	err := store.db.WithContext(ctx).
		Where("status = ?", status).
		Order("updated_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// ListTasks 按更新时间倒序返回任务历史，limit 小于等于 0 时不限制数量。
func (store *Store) ListTasks(ctx context.Context, limit int) ([]model.Task, error) {
	query := store.db.WithContext(ctx).
		Order("updated_at DESC").
		Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var tasks []model.Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTask 按 task_id 读取任务详情，缺失时返回 ok=false。
func (store *Store) GetTask(ctx context.Context, taskID string) (model.Task, bool, error) {
	var task model.Task
	err := store.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Task{}, false, nil
	}
	if err != nil {
		return model.Task{}, false, err
	}
	return task, true, nil
}

// AppendTaskEvent 写入任务事件，支撑 SSE 断线恢复和任务历史详情。
func (store *Store) AppendTaskEvent(ctx context.Context, event *model.TaskEvent) error {
	if event != nil {
		event.Payload = logger.RedactText(event.Payload)
	}
	return store.db.WithContext(ctx).Create(event).Error
}

// ListTaskEventsAfter 返回指定任务在某事件 ID 之后的事件。
func (store *Store) ListTaskEventsAfter(ctx context.Context, taskID string, afterID int64) ([]model.TaskEvent, error) {
	var events []model.TaskEvent
	err := store.db.WithContext(ctx).
		Where("task_id = ? AND id > ?", taskID, afterID).
		Order("id ASC").
		Find(&events).Error
	return events, err
}

// SaveAnalysisReportByTaskID 按 task_id 幂等保存分析报告。
func (store *Store) SaveAnalysisReportByTaskID(ctx context.Context, report *model.AnalysisReport) error {
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"symbol",
				"title",
				"analysis_type",
				"model_name",
				"prompt_template_id",
				"input_snapshot",
				"content_markdown",
				"risk_summary",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(report).Error
}

// SaveAnalysisReportWithTaskCompletion 在同一事务内保存报告、输出事件和任务成功终态。
func (store *Store) SaveAnalysisReportWithTaskCompletion(ctx context.Context, report *model.AnalysisReport, chunkEvent *model.TaskEvent, task *model.Task, successEvent *model.TaskEvent) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStore := Store{db: tx}
		if err := txStore.SaveAnalysisReportByTaskID(ctx, report); err != nil {
			return err
		}
		if err := txStore.AppendTaskEvent(ctx, chunkEvent); err != nil {
			return err
		}
		if err := txStore.SaveTask(ctx, task); err != nil {
			return err
		}
		return txStore.AppendTaskEvent(ctx, successEvent)
	})
}

// ListVisibleAnalysisReports 返回未软删除报告，并按更新时间倒序排序。
func (store *Store) ListVisibleAnalysisReports(ctx context.Context) ([]model.AnalysisReport, error) {
	var reports []model.AnalysisReport
	err := store.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("updated_at DESC").
		Order("id ASC").
		Find(&reports).Error
	return reports, err
}

// GetAnalysisReportByTaskID 按 task_id 读取未软删除报告，供任务日志上下文摘要使用。
func (store *Store) GetAnalysisReportByTaskID(ctx context.Context, taskID string) (model.AnalysisReport, bool, error) {
	var report model.AnalysisReport
	err := store.db.WithContext(ctx).
		Where("task_id = ? AND deleted_at IS NULL", taskID).
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AnalysisReport{}, false, nil
	}
	if err != nil {
		return model.AnalysisReport{}, false, err
	}
	return report, true, nil
}

// SoftDeleteAnalysisReport 对分析报告执行软删除。
func (store *Store) SoftDeleteAnalysisReport(ctx context.Context, id int64) error {
	return store.db.WithContext(ctx).Delete(&model.AnalysisReport{}, id).Error
}

// UpsertSetting 写入或更新非敏感设置项。
func (store *Store) UpsertSetting(ctx context.Context, setting model.Setting) error {
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&setting).Error
}

// GetSettings 按 key 批量读取设置项，空 key 列表直接返回空结果。
func (store *Store) GetSettings(ctx context.Context, keys []string) ([]model.Setting, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	var settings []model.Setting
	err := store.db.WithContext(ctx).
		Where("key IN ?", keys).
		Order("key ASC").
		Find(&settings).Error
	return settings, err
}

// SaveSchedulerJob 创建或更新桌面可管理的调度任务配置。
func (store *Store) SaveSchedulerJob(ctx context.Context, job *model.SchedulerJob) error {
	if job == nil {
		return fmt.Errorf("dao scheduler job is required")
	}
	return store.db.WithContext(ctx).Save(job).Error
}

// ListSchedulerJobs 返回未软删除的调度任务，供桌面管理页展示。
func (store *Store) ListSchedulerJobs(ctx context.Context) ([]model.SchedulerJob, error) {
	var jobs []model.SchedulerJob
	err := store.db.WithContext(ctx).
		Order("updated_at DESC").
		Order("id ASC").
		Find(&jobs).Error
	return jobs, err
}

// GetSchedulerJob 按 ID 返回未软删除的调度任务配置，缺失时返回 ok=false。
func (store *Store) GetSchedulerJob(ctx context.Context, id int64) (model.SchedulerJob, bool, error) {
	var job model.SchedulerJob
	err := store.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SchedulerJob{}, false, nil
	}
	if err != nil {
		return model.SchedulerJob{}, false, err
	}
	return job, true, nil
}

// ListEnabledSchedulerJobs 返回启动时需要恢复到调度器中的启用任务。
func (store *Store) ListEnabledSchedulerJobs(ctx context.Context) ([]model.SchedulerJob, error) {
	var jobs []model.SchedulerJob
	err := store.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("id ASC").
		Find(&jobs).Error
	return jobs, err
}

// SetSchedulerJobEnabled 切换调度任务启停状态，保留任务配置和历史执行记录。
func (store *Store) SetSchedulerJobEnabled(ctx context.Context, id int64, enabled bool) error {
	return store.db.WithContext(ctx).
		Model(&model.SchedulerJob{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"enabled": enabled}).Error
}

// SoftDeleteSchedulerJob 软删除调度任务配置，历史执行记录仍可用于审计。
func (store *Store) SoftDeleteSchedulerJob(ctx context.Context, id int64) error {
	return store.db.WithContext(ctx).Delete(&model.SchedulerJob{}, id).Error
}

// CreateSchedulerRun 创建一次调度执行记录，run_key 唯一约束用于防止重复入队。
func (store *Store) CreateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error {
	if run == nil {
		return fmt.Errorf("dao scheduler run is required")
	}
	return store.db.WithContext(ctx).Create(run).Error
}

// GetSchedulerRunByRunKey 按 run_key 返回调度执行记录，用于启动补偿前做显式幂等判断。
func (store *Store) GetSchedulerRunByRunKey(ctx context.Context, runKey string) (model.SchedulerRun, bool, error) {
	var run model.SchedulerRun
	err := store.db.WithContext(ctx).Where("run_key = ?", runKey).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SchedulerRun{}, false, nil
	}
	if err != nil {
		return model.SchedulerRun{}, false, err
	}
	return run, true, nil
}

// UpdateSchedulerRun 更新调度执行状态、统计和错误信息。
func (store *Store) UpdateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error {
	if run == nil {
		return fmt.Errorf("dao scheduler run is required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(run).Error; err != nil {
			return err
		}
		if run.JobID <= 0 {
			return nil
		}
		lastRunAt := run.FinishedAt
		if lastRunAt == nil {
			lastRunAt = run.StartedAt
		}
		if lastRunAt == nil {
			now := time.Now().UTC()
			lastRunAt = &now
		}
		lastError := run.ErrorMessage
		if lastError == "" {
			lastError = run.SkippedReason
		}
		return tx.Model(&model.SchedulerJob{}).
			Where("id = ?", run.JobID).
			Updates(map[string]any{
				"last_run_at": lastRunAt,
				"last_status": run.Status,
				"last_error":  lastError,
			}).Error
	})
}

// ListSchedulerRunsByStatuses 返回指定状态集合的执行记录，供队列恢复和运行中任务扫描使用。
func (store *Store) ListSchedulerRunsByStatuses(ctx context.Context, statuses []string) ([]model.SchedulerRun, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	var runs []model.SchedulerRun
	err := store.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("created_at ASC").
		Order("id ASC").
		Find(&runs).Error
	return runs, err
}

// ListSchedulerRuns 返回调度执行记录，jobID 为 0 时返回全部任务记录。
func (store *Store) ListSchedulerRuns(ctx context.Context, jobID int64, limit int) ([]model.SchedulerRun, error) {
	query := store.db.WithContext(ctx).
		Order("created_at DESC").
		Order("id DESC")
	if jobID > 0 {
		query = query.Where("job_id = ?", jobID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var runs []model.SchedulerRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

// GetSchedulerRun 按 ID 返回单条调度执行记录，缺失时返回 ok=false。
func (store *Store) GetSchedulerRun(ctx context.Context, id int64) (model.SchedulerRun, bool, error) {
	var run model.SchedulerRun
	err := store.db.WithContext(ctx).Where("id = ?", id).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SchedulerRun{}, false, nil
	}
	if err != nil {
		return model.SchedulerRun{}, false, err
	}
	return run, true, nil
}

// UpsertIngestionWatermark 按数据类型、范围、Provider 和周期幂等更新抓取水位。
func (store *Store) UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error {
	if watermark == nil {
		return fmt.Errorf("dao ingestion watermark is required")
	}
	return store.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "data_type"},
				{Name: "scope_key"},
				{Name: "provider"},
				{Name: "period"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"last_success_at",
				"last_trade_date",
				"cursor_json",
				"updated_at",
			}),
		}).
		Create(watermark).Error
}

// GetIngestionWatermark 读取指定抓取范围的水位，缺失时返回 ok=false。
func (store *Store) GetIngestionWatermark(ctx context.Context, dataType string, scopeKey string, provider string, period string) (model.IngestionWatermark, bool, error) {
	var watermark model.IngestionWatermark
	err := store.db.WithContext(ctx).
		Where("data_type = ? AND scope_key = ? AND provider = ? AND period = ?", dataType, scopeKey, provider, period).
		First(&watermark).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.IngestionWatermark{}, false, nil
	}
	if err != nil {
		return model.IngestionWatermark{}, false, err
	}
	return watermark, true, nil
}

// CleanCache 只清理临时缓存表，忽略报告和配置等受保护目标。
func (store *Store) CleanCache(ctx context.Context, targets []settings.CacheTarget) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, target := range settings.FilterCacheCleanupTargets(targets) {
			if err := cleanCacheTarget(tx, target); err != nil {
				return err
			}
		}
		return nil
	})
}

// CacheUsages 从真实缓存表读取可清理缓存体积，不统计报告和配置。
func (store *Store) CacheUsages(ctx context.Context) ([]settings.CacheUsage, error) {
	result := make([]settings.CacheUsage, 0, 4)

	var quotes []model.Quote
	if err := store.db.WithContext(ctx).Find(&quotes).Error; err != nil {
		return nil, err
	}
	appendCacheUsage(&result, settings.CacheTargetQuote, quotes)

	var klines []model.Kline
	if err := store.db.WithContext(ctx).Find(&klines).Error; err != nil {
		return nil, err
	}
	appendCacheUsage(&result, settings.CacheTargetKline, klines)

	var newsItems []model.NewsItem
	if err := store.db.WithContext(ctx).Where("deleted_at IS NULL").Find(&newsItems).Error; err != nil {
		return nil, err
	}
	appendCacheUsage(&result, settings.CacheTargetNews, newsItems)

	var taskLogs []model.TaskLogEntry
	if err := store.db.WithContext(ctx).Find(&taskLogs).Error; err != nil {
		return nil, err
	}
	var taskDiagnoses []model.TaskErrorDiagnosis
	if err := store.db.WithContext(ctx).Find(&taskDiagnoses).Error; err != nil {
		return nil, err
	}
	appendCacheUsage(&result, settings.CacheTargetTaskLogs, map[string]any{
		"logs":      taskLogs,
		"diagnoses": taskDiagnoses,
	})

	return result, nil
}

// cleanCacheTarget 将单个允许清理的缓存目标映射到对应 GORM 模型。
func cleanCacheTarget(tx *gorm.DB, target settings.CacheTarget) error {
	switch target {
	case settings.CacheTargetQuote:
		return tx.Where("1 = 1").Delete(&model.Quote{}).Error
	case settings.CacheTargetKline:
		return tx.Where("1 = 1").Delete(&model.Kline{}).Error
	case settings.CacheTargetNews:
		return tx.Unscoped().Where("1 = 1").Delete(&model.NewsItem{}).Error
	case settings.CacheTargetChartImage:
		return nil
	case settings.CacheTargetTaskLogs:
		if err := tx.Where("1 = 1").Delete(&model.TaskLogEntry{}).Error; err != nil {
			return err
		}
		return tx.Where("1 = 1").Delete(&model.TaskErrorDiagnosis{}).Error
	case settings.CacheTargetAppLogs:
		// NDJSON 文件日志依赖工作区路径，由 tasklog 文件 writer 的保留策略治理。
		return nil
	default:
		return nil
	}
}

// appendCacheUsage 将缓存记录序列化为稳定字节数，用于设置页展示近似体积。
func appendCacheUsage(result *[]settings.CacheUsage, target settings.CacheTarget, records any) {
	payload, err := json.Marshal(records)
	if err != nil || len(payload) <= 2 {
		return
	}
	*result = append(*result, settings.CacheUsage{Target: target, Bytes: int64(len(payload))})
}
