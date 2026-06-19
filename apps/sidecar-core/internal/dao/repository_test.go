package dao

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"gorm.io/gorm"
)

// TestStoreTransactionRollsBackWrites 验证事务封装会在业务错误时回滚写入。
func TestStoreTransactionRollsBackWrites(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	errRollback := errors.New("rollback")
	err := store.WithTransaction(ctx, func(tx *Store) error {
		if err := tx.SaveWatchlist(ctx, &model.Watchlist{Symbol: "US:AAPL"}); err != nil {
			t.Fatalf("save watchlist in transaction: %v", err)
		}
		return errRollback
	})

	if !errors.Is(err, errRollback) {
		t.Fatalf("expected rollback error, got %v", err)
	}
	items, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		t.Fatalf("list watchlists after rollback: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected rollback to remove watchlist, got %+v", items)
	}
}

// TestWatchlistRepositoryPersistsAndSoftDeletes 验证自选股 repository 支持排序、软删除和重新添加。
func TestWatchlistRepositoryPersistsAndSoftDeletes(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first := model.Watchlist{Symbol: "US:AAPL", SortOrder: 2, Tags: "tech", Note: "first"}
	second := model.Watchlist{Symbol: "CN:SH:600519", SortOrder: 1, Tags: "white_wine"}
	if err := store.SaveWatchlist(ctx, &first); err != nil {
		t.Fatalf("save first watchlist: %v", err)
	}
	if err := store.SaveWatchlist(ctx, &second); err != nil {
		t.Fatalf("save second watchlist: %v", err)
	}

	items, err := store.ListActiveWatchlists(ctx)
	if err != nil {
		t.Fatalf("list active watchlists: %v", err)
	}
	if len(items) != 2 || items[0].Symbol != "CN:SH:600519" || items[1].Symbol != "US:AAPL" {
		t.Fatalf("unexpected watchlist order: %+v", items)
	}

	if err := store.SoftDeleteWatchlist(ctx, second.ID); err != nil {
		t.Fatalf("soft delete watchlist: %v", err)
	}
	if err := store.SaveWatchlist(ctx, &model.Watchlist{Symbol: "CN:SH:600519", SortOrder: 3}); err != nil {
		t.Fatalf("re-add soft deleted watchlist symbol: %v", err)
	}
	items, err = store.ListActiveWatchlists(ctx)
	if err != nil {
		t.Fatalf("list active watchlists after re-add: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two active watchlists after re-add, got %+v", items)
	}
}

// TestStockRepositoryUpsertsBySymbol 验证股票基础信息缓存按 symbol 幂等更新。
func TestStockRepositoryUpsertsBySymbol(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpsertStocks(ctx, []model.Stock{{
		Symbol:   "CN:SH:600519",
		Market:   "CN",
		Code:     "600519",
		Name:     "贵州茅台",
		Exchange: "SH",
		Industry: "白酒",
	}})
	if err != nil {
		t.Fatalf("upsert stock: %v", err)
	}
	err = store.UpsertStocks(ctx, []model.Stock{{
		Symbol:   "CN:SH:600519",
		Market:   "CN",
		Code:     "600519",
		Name:     "贵州茅台股份",
		Exchange: "SH",
		Industry: "消费",
		Concept:  "蓝筹",
	}})
	if err != nil {
		t.Fatalf("update stock: %v", err)
	}

	var stocks []model.Stock
	if err := store.db.WithContext(ctx).Find(&stocks).Error; err != nil {
		t.Fatalf("list stocks: %v", err)
	}
	if len(stocks) != 1 ||
		stocks[0].Name != "贵州茅台股份" ||
		stocks[0].Industry != "消费" ||
		stocks[0].Concept != "蓝筹" {
		t.Fatalf("unexpected stocks after upsert: %+v", stocks)
	}
}

// TestMarketQuoteRepositoryUpsertsBySymbol 验证行情快照缓存按 symbol 幂等更新并支持短缓存读取。
func TestMarketQuoteRepositoryUpsertsBySymbol(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	quoteTime := time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC)

	if err := store.SaveQuote(ctx, &model.Quote{
		Symbol:        "CN:SH:600519",
		Price:         1688.5,
		ChangePercent: 0.73,
		QuoteTime:     quoteTime,
		Provider:      "provider-a",
	}); err != nil {
		t.Fatalf("save quote: %v", err)
	}
	if err := store.SaveQuote(ctx, &model.Quote{
		Symbol:        "CN:SH:600519",
		Price:         1700,
		ChangePercent: 1.2,
		QuoteTime:     quoteTime.Add(time.Minute),
		Provider:      "provider-b",
	}); err != nil {
		t.Fatalf("update quote: %v", err)
	}

	quote, ok, err := store.LatestQuote(ctx, "CN:SH:600519", time.Minute)
	if err != nil {
		t.Fatalf("latest quote: %v", err)
	}
	if !ok || quote.Price != 1700 || quote.ChangePercent != 1.2 || quote.Provider != "provider-b" {
		t.Fatalf("unexpected latest quote: ok=%v quote=%+v", ok, quote)
	}
	requireCount(t, store, &model.Quote{}, 1)
}

// TestMarketKlineRepositoryUpsertsAndOrdersBars 验证 K 线缓存按交易日幂等更新并稳定升序返回。
func TestMarketKlineRepositoryUpsertsAndOrdersBars(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.SaveKlines(ctx, []model.Kline{
		{Symbol: "CN:SH:600519", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 101, Provider: "provider-a"},
		{Symbol: "CN:SH:600519", Period: "day", Adjust: "none", TradeDate: "2026-06-17", Close: 100, Provider: "provider-a"},
	})
	if err != nil {
		t.Fatalf("save klines: %v", err)
	}
	err = store.SaveKlines(ctx, []model.Kline{
		{Symbol: "CN:SH:600519", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 102, Provider: "provider-b"},
	})
	if err != nil {
		t.Fatalf("update kline: %v", err)
	}

	klines, err := store.ListKlines(ctx, "CN:SH:600519", "day", "none", 10)
	if err != nil {
		t.Fatalf("list klines: %v", err)
	}
	if len(klines) != 2 ||
		klines[0].TradeDate != "2026-06-17" ||
		klines[1].TradeDate != "2026-06-18" ||
		klines[1].Close != 102 ||
		klines[1].Provider != "provider-b" {
		t.Fatalf("unexpected klines: %+v", klines)
	}
	requireCount(t, store, &model.Kline{}, 2)
}

// TestNewsRepositoryUpsertsByContentHashAndSorts 验证新闻缓存按 content_hash 去重并按发布时间倒序返回。
func TestNewsRepositoryUpsertsByContentHashAndSorts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	newer := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	older := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)

	err := store.SaveNewsItems(ctx, []model.NewsItem{
		{Title: "旧新闻", URL: "https://example.com/old", ContentHash: "hash-old", Symbols: "CN:SH:600519", PublishedAt: older, Source: "provider-a"},
		{Title: "新新闻", URL: "https://example.com/new", ContentHash: "hash-new", Symbols: "CN:SH:600519", PublishedAt: newer, Source: "provider-a"},
	})
	if err != nil {
		t.Fatalf("save news items: %v", err)
	}
	err = store.SaveNewsItems(ctx, []model.NewsItem{
		{Title: "新新闻更新", URL: "https://example.com/newer", ContentHash: "hash-new", Symbols: "CN:SH:600519", PublishedAt: newer, Source: "provider-b"},
	})
	if err != nil {
		t.Fatalf("update news item: %v", err)
	}

	items, err := store.ListNewsBySymbol(ctx, "CN:SH:600519", 10, time.Hour)
	if err != nil {
		t.Fatalf("list news by symbol: %v", err)
	}
	if len(items) != 2 ||
		items[0].ContentHash != "hash-new" ||
		items[0].Title != "新新闻更新" ||
		items[0].Source != "provider-b" ||
		items[1].ContentHash != "hash-old" {
		t.Fatalf("unexpected news items: %+v", items)
	}
	requireCount(t, store, &model.NewsItem{}, 2)
}

// TestNewsRepositoryMatchesSymbolExactly 验证新闻缓存按完整 symbol 匹配，避免 US:A 命中 US:AAPL。
func TestNewsRepositoryMatchesSymbolExactly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)

	if err := store.SaveNewsItems(ctx, []model.NewsItem{
		{Title: "Apple news", URL: "https://example.com/aapl", ContentHash: "hash-aapl", Symbols: `["US:AAPL"]`, PublishedAt: now, Source: "provider-a"},
	}); err != nil {
		t.Fatalf("save news items: %v", err)
	}

	items, err := store.ListNewsBySymbol(ctx, "US:A", 10, time.Hour)
	if err != nil {
		t.Fatalf("list news by symbol: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no substring symbol matches, got %+v", items)
	}
}

// TestAIConfigRepositoryHidesSoftDeletedRows 验证 AI 配置 repository 只返回未删除配置。
func TestAIConfigRepositoryHidesSoftDeletedRows(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	config := model.AIConfig{
		Name:         "OpenAI",
		Provider:     "openai-compatible",
		APIKeyRef:    "local-vault://ai-config/openai-compatible",
		MaskedAPIKey: "sk-p...7890",
		HasAPIKey:    true,
		ModelName:    "gpt-4.1-mini",
		IsDefault:    true,
	}
	if err := store.SaveAIConfig(ctx, &config); err != nil {
		t.Fatalf("save ai config: %v", err)
	}
	if err := store.SoftDeleteAIConfig(ctx, config.ID); err != nil {
		t.Fatalf("soft delete ai config: %v", err)
	}

	configs, err := store.ListAIConfigs(ctx)
	if err != nil {
		t.Fatalf("list ai configs: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected soft deleted ai config to be hidden, got %+v", configs)
	}
}

// TestPromptTemplateRepositoryPersistsVariables 验证 Prompt 模板 repository 保留变量序列化结果并隐藏软删除模板。
func TestPromptTemplateRepositoryPersistsVariables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	template := model.PromptTemplate{
		Name:      "个股分析",
		Type:      "stock_full",
		Content:   "分析 {{stock_name}}",
		Variables: `["stock_name"]`,
		IsBuiltin: true,
	}
	if err := store.SavePromptTemplate(ctx, &template); err != nil {
		t.Fatalf("save prompt template: %v", err)
	}

	templates, err := store.ListPromptTemplates(ctx)
	if err != nil {
		t.Fatalf("list prompt templates: %v", err)
	}
	if len(templates) != 1 || templates[0].Variables != `["stock_name"]` {
		t.Fatalf("unexpected prompt templates: %+v", templates)
	}
	gotTemplate, err := store.GetPromptTemplate(ctx, template.ID)
	if err != nil {
		t.Fatalf("get prompt template: %v", err)
	}
	if gotTemplate.ID != template.ID || gotTemplate.Variables != `["stock_name"]` {
		t.Fatalf("unexpected prompt template detail: %+v", gotTemplate)
	}

	if err := store.SoftDeletePromptTemplate(ctx, template.ID); err != nil {
		t.Fatalf("soft delete prompt template: %v", err)
	}
	templates, err = store.ListPromptTemplates(ctx)
	if err != nil {
		t.Fatalf("list prompt templates after delete: %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("expected deleted prompt template hidden, got %+v", templates)
	}
	if _, err := store.GetPromptTemplate(ctx, template.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected deleted prompt template to be hidden from detail, got %v", err)
	}
}

// TestTaskRepositoryPersistsTasksAndEvents 验证任务和事件 repository 支持状态查询与事件回放。
func TestTaskRepositoryPersistsTasksAndEvents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	task := model.Task{
		ID:        "task-1",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		Title:     "贵州茅台分析",
		Progress:  10,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveTask(ctx, &task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := store.AppendTaskEvent(ctx, &model.TaskEvent{TaskID: task.ID, EventType: "TASK_STARTED", Payload: `{"progress":10}`}); err != nil {
		t.Fatalf("append start event: %v", err)
	}
	if err := store.AppendTaskEvent(ctx, &model.TaskEvent{TaskID: task.ID, EventType: "TASK_PROGRESS", Payload: `{"progress":20}`}); err != nil {
		t.Fatalf("append progress event: %v", err)
	}

	running, err := store.ListTasksByStatus(ctx, "RUNNING")
	if err != nil {
		t.Fatalf("list running tasks: %v", err)
	}
	if len(running) != 1 || running[0].ID != task.ID {
		t.Fatalf("unexpected running tasks: %+v", running)
	}

	events, err := store.ListTaskEventsAfter(ctx, task.ID, 1)
	if err != nil {
		t.Fatalf("list task events after id: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "TASK_PROGRESS" {
		t.Fatalf("unexpected replay events: %+v", events)
	}
}

// TestTaskEventRepositoryRedactsSensitivePayload 验证任务事件入库前统一脱敏敏感字段。
func TestTaskEventRepositoryRedactsSensitivePayload(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	event := model.TaskEvent{
		TaskID:    "task-secret",
		EventType: "TASK_LOG",
		Payload:   `{"Authorization":"Bearer sk-secret","userPosition":"满仓","message":"ok"}`,
	}
	if err := store.AppendTaskEvent(ctx, &event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	events, err := store.ListTaskEventsAfter(ctx, "task-secret", 0)
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one task event, got %+v", events)
	}
	if strings.Contains(events[0].Payload, "sk-secret") || strings.Contains(events[0].Payload, "满仓") {
		t.Fatalf("expected event payload redacted before persistence, got %s", events[0].Payload)
	}
}

// TestTaskRepositoryListsAndGetsTasks 验证任务 repository 支持历史列表和详情读取。
func TestTaskRepositoryListsAndGetsTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	for _, item := range []model.Task{
		{ID: "task-old", Type: "ANALYSIS", Status: "SUCCESS", UpdatedAt: base, CreatedAt: base},
		{ID: "task-new", Type: "ANALYSIS", Status: "FAILED", UpdatedAt: base.Add(time.Hour), CreatedAt: base},
	} {
		task := item
		if err := store.SaveTask(ctx, &task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	tasks, err := store.ListTasks(ctx, 10)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 2 || tasks[0].ID != "task-new" || tasks[1].ID != "task-old" {
		t.Fatalf("expected tasks sorted by updated_at desc, got %+v", tasks)
	}

	got, ok, err := store.GetTask(ctx, "task-old")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !ok || got.ID != "task-old" || got.Status != "SUCCESS" {
		t.Fatalf("unexpected task detail: ok=%v task=%+v", ok, got)
	}
}

// TestReportRepositoryUpsertsByTaskID 验证报告按 task_id 幂等保存并支持软删除可见性。
func TestReportRepositoryUpsertsByTaskID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	report := model.AnalysisReport{
		TaskID:          "task-1",
		Symbol:          "CN:SH:600519",
		Title:           "旧报告",
		AnalysisType:    "stock_full",
		ContentMarkdown: "old",
	}
	if err := store.SaveAnalysisReportByTaskID(ctx, &report); err != nil {
		t.Fatalf("save report: %v", err)
	}
	report.Title = "新报告"
	report.ContentMarkdown = "new"
	if err := store.SaveAnalysisReportByTaskID(ctx, &report); err != nil {
		t.Fatalf("upsert report: %v", err)
	}

	reports, err := store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		t.Fatalf("list visible reports: %v", err)
	}
	if len(reports) != 1 || reports[0].Title != "新报告" || reports[0].ContentMarkdown != "new" {
		t.Fatalf("unexpected reports after upsert: %+v", reports)
	}

	if err := store.SoftDeleteAnalysisReport(ctx, reports[0].ID); err != nil {
		t.Fatalf("soft delete report: %v", err)
	}
	reports, err = store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		t.Fatalf("list visible reports after delete: %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected deleted report hidden, got %+v", reports)
	}
}

// TestSettingsRepositoryUpsertsAndGetsKeys 验证 settings repository 支持非敏感配置的 upsert 和批量读取。
func TestSettingsRepositoryUpsertsAndGetsKeys(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSetting(ctx, model.Setting{Key: "workspace_path", Value: "/tmp/invest-compass"}); err != nil {
		t.Fatalf("upsert setting: %v", err)
	}
	if err := store.UpsertSetting(ctx, model.Setting{Key: "workspace_path", Value: "/tmp/updated"}); err != nil {
		t.Fatalf("update setting: %v", err)
	}

	settings, err := store.GetSettings(ctx, []string{"workspace_path", "missing"})
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if len(settings) != 1 || settings[0].Value != "/tmp/updated" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

// TestStoreCleanCacheDeletesTemporaryTablesOnly 验证缓存清理不会误删报告和配置。
func TestStoreCleanCacheDeletesTemporaryTablesOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.db.WithContext(ctx).Create(&model.Quote{Symbol: "US:AAPL"}).Error; err != nil {
		t.Fatalf("seed quote: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.Kline{Symbol: "US:AAPL", Period: "1d", Adjust: "none", TradeDate: "2026-06-18"}).Error; err != nil {
		t.Fatalf("seed kline: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.NewsItem{Title: "news", ContentHash: "hash-1"}).Error; err != nil {
		t.Fatalf("seed news: %v", err)
	}
	if err := store.SaveAnalysisReportByTaskID(ctx, &model.AnalysisReport{TaskID: "task-1", Symbol: "US:AAPL", Title: "report", AnalysisType: "stock_full"}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if err := store.UpsertSetting(ctx, model.Setting{Key: "theme", Value: "dark"}); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	err := store.CleanCache(ctx, []settings.CacheTarget{
		settings.CacheTargetQuote,
		settings.CacheTargetKline,
		settings.CacheTargetNews,
		settings.CacheTargetReport,
		settings.CacheTargetConfig,
	})
	if err != nil {
		t.Fatalf("clean cache: %v", err)
	}

	requireCount(t, store, &model.Quote{}, 0)
	requireCount(t, store, &model.Kline{}, 0)
	requireCount(t, store, &model.NewsItem{}, 0)
	requireCount(t, store, &model.AnalysisReport{}, 1)
	requireCount(t, store, &model.Setting{}, 1)
}

// TestStoreCacheUsagesReadsTemporaryCacheData 验证缓存统计来自真实缓存表且不包含报告和配置。
func TestStoreCacheUsagesReadsTemporaryCacheData(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.db.WithContext(ctx).Create(&model.Quote{Symbol: "US:AAPL", Price: 100}).Error; err != nil {
		t.Fatalf("seed quote: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.Kline{Symbol: "US:AAPL", Period: "1d", Adjust: "none", TradeDate: "2026-06-18", Close: 100}).Error; err != nil {
		t.Fatalf("seed kline: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.NewsItem{Title: "news", Summary: "summary", ContentHash: "hash-1"}).Error; err != nil {
		t.Fatalf("seed news: %v", err)
	}
	if err := store.SaveAnalysisReportByTaskID(ctx, &model.AnalysisReport{TaskID: "task-1", Symbol: "US:AAPL", Title: "report", AnalysisType: "stock_full"}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if err := store.UpsertSetting(ctx, model.Setting{Key: "theme", Value: "dark"}); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	usages, err := store.CacheUsages(ctx)
	if err != nil {
		t.Fatalf("cache usages: %v", err)
	}
	requirePositiveCacheUsage(t, usages, settings.CacheTargetQuote)
	requirePositiveCacheUsage(t, usages, settings.CacheTargetKline)
	requirePositiveCacheUsage(t, usages, settings.CacheTargetNews)
	requireMissingCacheUsage(t, usages, settings.CacheTargetReport)
	requireMissingCacheUsage(t, usages, settings.CacheTargetConfig)
}

// newTestStore 创建已迁移的内存数据库，供 DAO repository 测试使用。
func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	return store
}

// requireCount 校验指定模型在测试数据库中的记录数量。
func requireCount(t *testing.T, store *Store, modelValue any, want int64) {
	t.Helper()

	var got int64
	if err := store.db.Model(modelValue).Count(&got).Error; err != nil {
		t.Fatalf("count model %T: %v", modelValue, err)
	}
	if got != want {
		t.Fatalf("unexpected count for %T: got %d want %d", modelValue, got, want)
	}
}

// requirePositiveCacheUsage 验证指定缓存目标有真实统计值。
func requirePositiveCacheUsage(t *testing.T, usages []settings.CacheUsage, target settings.CacheTarget) {
	t.Helper()
	for _, usage := range usages {
		if usage.Target == target {
			if usage.Bytes <= 0 {
				t.Fatalf("expected positive cache usage for %s, got %d", target, usage.Bytes)
			}
			return
		}
	}
	t.Fatalf("missing cache usage for %s in %+v", target, usages)
}

// requireMissingCacheUsage 验证受保护目标不会出现在缓存统计里。
func requireMissingCacheUsage(t *testing.T, usages []settings.CacheUsage, target settings.CacheTarget) {
	t.Helper()
	for _, usage := range usages {
		if usage.Target == target {
			t.Fatalf("protected cache target %s must not be reported: %+v", target, usages)
		}
	}
}

// requireRecordNotFound 验证 repository 按 GORM 标准返回记录不存在错误。
func requireRecordNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}
