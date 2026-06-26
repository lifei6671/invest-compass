package dao

import (
	"context"
	"errors"
	"path/filepath"
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

// TestWatchlistRepositoryPersistsAndSoftDeletes 验证自选股 repository 按添加时间倒序、软删除和重新添加。
func TestWatchlistRepositoryPersistsAndSoftDeletes(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	first := model.Watchlist{Symbol: "US:AAPL", SortOrder: 2, Tags: "tech", Note: "first", CreatedAt: base, UpdatedAt: base}
	second := model.Watchlist{Symbol: "CN:SH:600519", SortOrder: 1, Tags: "white_wine", CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour)}
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
	if err := store.SaveWatchlist(ctx, &model.Watchlist{Symbol: "CN:SH:600519", SortOrder: 3, CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base.Add(2 * time.Hour)}); err != nil {
		t.Fatalf("re-add soft deleted watchlist symbol: %v", err)
	}
	items, err = store.ListActiveWatchlists(ctx)
	if err != nil {
		t.Fatalf("list active watchlists after re-add: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two active watchlists after re-add, got %+v", items)
	}
	if items[0].Symbol != "CN:SH:600519" || items[1].Symbol != "US:AAPL" {
		t.Fatalf("expected re-added watchlist to sort first by created_at desc, got %+v", items)
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

// TestBuiltinStockRepositoryKeepsPackagedProfileFields 验证内置基础股票池可更新详情字段，且不会依赖远端简版 upsert。
func TestBuiltinStockRepositoryKeepsPackagedProfileFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpsertBuiltinStocks(ctx, []model.Stock{{
		Symbol:         "CN:SH:600519",
		Market:         "CN",
		Code:           "600519",
		Name:           "贵州茅台",
		Exchange:       "SH",
		Industry:       "白酒",
		FullName:       "贵州茅台酒股份有限公司",
		PinyinInitials: "gzmt",
		Status:         "LISTED",
	}})
	if err != nil {
		t.Fatalf("upsert builtin stock: %v", err)
	}
	err = store.UpsertStocks(ctx, []model.Stock{{
		Symbol:   "CN:SH:600519",
		Market:   "CN",
		Code:     "600519",
		Name:     "贵州茅台",
		Exchange: "SH",
		Industry: "消费",
	}})
	if err != nil {
		t.Fatalf("upsert remote stock: %v", err)
	}

	stock, ok, err := store.GetStockBySymbol(ctx, "CN:SH:600519")
	if err != nil {
		t.Fatalf("get stock by symbol: %v", err)
	}
	if !ok {
		t.Fatalf("expected stock exists")
	}
	if stock.Industry != "消费" || stock.FullName != "贵州茅台酒股份有限公司" || stock.PinyinInitials != "gzmt" {
		t.Fatalf("unexpected stock after mixed upserts: %+v", stock)
	}
}

// TestStockSearchRepositoryListsLocalCandidates 验证股票搜索 DAO 支持本地强规则候选和 symbol 回表。
func TestStockSearchRepositoryListsLocalCandidates(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertStocks(ctx, []model.Stock{
		{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台", FullName: "贵州茅台酒股份有限公司", PinyinFull: "guizhoumaotai", PinyinInitials: "gzmt", Status: "LISTED"},
		{Symbol: "CN:SH:601398", Market: "CN", Exchange: "SH", Code: "601398", Name: "工商银行", PinyinFull: "gongshangyinhang", PinyinInitials: "gsyh", Status: "LISTED"},
	}); err != nil {
		t.Fatalf("seed stocks: %v", err)
	}

	candidates, err := store.ListStocksForSearch(ctx, "gzmt", nil, 10)
	if err != nil {
		t.Fatalf("list stock search candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Symbol != "CN:SH:600519" {
		t.Fatalf("unexpected local candidates: %+v", candidates)
	}

	bySymbols, err := store.ListStocksForSearch(ctx, "", []string{"CN:SH:601398", "CN:SH:600519"}, 10)
	if err != nil {
		t.Fatalf("list stock candidates by symbols: %v", err)
	}
	if len(bySymbols) != 2 || bySymbols[0].Symbol != "CN:SH:601398" || bySymbols[1].Symbol != "CN:SH:600519" {
		t.Fatalf("expected symbol order preserved, got %+v", bySymbols)
	}
}

// TestStockSearchRepositoryListsAllStocksForRebuild 验证全量重建能读取所有股票基础信息。
func TestStockSearchRepositoryListsAllStocksForRebuild(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertStocks(ctx, []model.Stock{
		{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", Name: "贵州茅台"},
		{Symbol: "CN:SH:601398", Market: "CN", Exchange: "SH", Code: "601398", Name: "工商银行"},
	}); err != nil {
		t.Fatalf("seed stocks: %v", err)
	}

	stocks, err := store.ListAllStocksForSearch(ctx)
	if err != nil {
		t.Fatalf("list all stocks for search: %v", err)
	}
	if len(stocks) != 2 || stocks[0].Symbol != "CN:SH:600519" || stocks[1].Symbol != "CN:SH:601398" {
		t.Fatalf("unexpected rebuild stocks: %+v", stocks)
	}
}

// TestStockAliasRepositoryListsBySymbols 验证股票别名能按 symbol 批量读取。
func TestStockAliasRepositoryListsBySymbols(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.db.WithContext(ctx).Create(&model.StockAlias{Symbol: "CN:SH:600519", Alias: "茅台", AliasType: "manual"}).Error; err != nil {
		t.Fatalf("seed stock alias: %v", err)
	}

	aliases, err := store.ListStockAliasesBySymbols(ctx, []string{"CN:SH:600519", "CN:SH:601398"})
	if err != nil {
		t.Fatalf("list stock aliases by symbols: %v", err)
	}
	if len(aliases["CN:SH:600519"]) != 1 || aliases["CN:SH:600519"][0].Alias != "茅台" {
		t.Fatalf("unexpected aliases: %+v", aliases)
	}
	if len(aliases["CN:SH:601398"]) != 0 {
		t.Fatalf("expected empty alias slice for missing symbol, got %+v", aliases)
	}
}

// TestSearchDocumentRepositoryListsByUIDsInMatchOrder 验证菜单搜索回表保留 FTS 命中顺序并排除软删除文档。
func TestSearchDocumentRepositoryListsByUIDsInMatchOrder(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	first := model.SearchDocument{BatchID: "doc-ready-1", DocUID: "report:1", DocType: "report", RefTable: "analysis_reports", RefID: "1", Title: "第一篇", SourceTime: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)}
	second := model.SearchDocument{BatchID: "doc-ready-1", DocUID: "report:2", DocType: "report", RefTable: "analysis_reports", RefID: "2", Title: "第二篇", SourceTime: time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)}
	deleted := model.SearchDocument{BatchID: "doc-ready-1", DocUID: "report:3", DocType: "report", RefTable: "analysis_reports", RefID: "3", Title: "已删除"}

	for _, document := range []*model.SearchDocument{&first, &second, &deleted} {
		if err := store.UpsertSearchDocument(ctx, document); err != nil {
			t.Fatalf("seed search document: %v", err)
		}
	}
	if err := store.SoftDeleteSearchDocument(ctx, "doc-ready-1", "report:3"); err != nil {
		t.Fatalf("soft delete search document: %v", err)
	}

	documents, err := store.ListSearchDocumentsByUIDs(ctx, "doc-ready-1", []string{"report:2", "report:3", "report:1"})
	if err != nil {
		t.Fatalf("list search documents by uids: %v", err)
	}
	if len(documents) != 2 || documents[0].DocUID != "report:2" || documents[1].DocUID != "report:1" {
		t.Fatalf("expected matched order without deleted document, got %+v", documents)
	}
}

// TestMarketQuoteRepositoryUpsertsBySymbol 验证行情快照缓存按 symbol 幂等更新并支持短缓存读取。
func TestMarketQuoteRepositoryUpsertsBySymbol(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	quoteTime := time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC)

	if err := store.SaveQuote(ctx, &model.Quote{
		Symbol:         "CN:SH:600519",
		Price:          1688.5,
		ChangePercent:  0.73,
		TurnoverRate:   3.98,
		PE:             352.10,
		PB:             2390.72,
		TotalMarketCap: 123456789000,
		FloatMarketCap: 98765432100,
		QuoteTime:      quoteTime,
		Provider:       "provider-a",
	}); err != nil {
		t.Fatalf("save quote: %v", err)
	}
	if err := store.SaveQuote(ctx, &model.Quote{
		Symbol:         "CN:SH:600519",
		Price:          1700,
		ChangePercent:  1.2,
		TurnoverRate:   0,
		PE:             0,
		PB:             0,
		TotalMarketCap: 0,
		FloatMarketCap: 0,
		QuoteTime:      quoteTime.Add(time.Minute),
		Provider:       "provider-b",
	}); err != nil {
		t.Fatalf("update quote: %v", err)
	}

	quote, ok, err := store.LatestQuote(ctx, "CN:SH:600519", time.Minute)
	if err != nil {
		t.Fatalf("latest quote: %v", err)
	}
	if !ok ||
		quote.Price != 1700 ||
		quote.ChangePercent != 1.2 ||
		quote.TurnoverRate != 0 ||
		quote.PE != 0 ||
		quote.PB != 0 ||
		quote.TotalMarketCap != 0 ||
		quote.FloatMarketCap != 0 ||
		quote.Provider != "provider-b" {
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

// TestNewsRepositoryFiltersMarketNewsByMarket 验证市场新闻缓存会保存 market 字段并支持按市场过滤。
func TestNewsRepositoryFiltersMarketNewsByMarket(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)

	if err := store.SaveNewsItems(ctx, []model.NewsItem{
		{Title: "A股新闻", Market: "CN", URL: "https://example.com/cn", ContentHash: "hash-cn", PublishedAt: now, Source: "provider-a"},
		{Title: "美股新闻", Market: "US", URL: "https://example.com/us", ContentHash: "hash-us", PublishedAt: now.Add(-time.Hour), Source: "provider-a"},
	}); err != nil {
		t.Fatalf("save market news items: %v", err)
	}
	if err := store.SaveNewsItems(ctx, []model.NewsItem{
		{Title: "A股新闻更新", Market: "HK", URL: "https://example.com/hk", ContentHash: "hash-cn", PublishedAt: now, Source: "provider-b"},
	}); err != nil {
		t.Fatalf("update market news item: %v", err)
	}

	cnItems, err := store.ListMarketNews(ctx, "CN", 10, time.Hour)
	if err != nil {
		t.Fatalf("list CN market news: %v", err)
	}
	if len(cnItems) != 0 {
		t.Fatalf("expected updated item to leave CN market, got %+v", cnItems)
	}
	hkItems, err := store.ListMarketNews(ctx, "HK", 10, time.Hour)
	if err != nil {
		t.Fatalf("list HK market news: %v", err)
	}
	if len(hkItems) != 1 || hkItems[0].ContentHash != "hash-cn" || hkItems[0].Market != "HK" {
		t.Fatalf("unexpected HK market news: %+v", hkItems)
	}
	allItems, err := store.ListMarketNews(ctx, "", 10, time.Hour)
	if err != nil {
		t.Fatalf("list all market news: %v", err)
	}
	if len(allItems) != 2 {
		t.Fatalf("expected unscoped market news to include all markets, got %+v", allItems)
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

// TestAIConfigRepositoryKeepsSingleDefault 验证默认模型是 ai_configs 的唯一状态。
func TestAIConfigRepositoryKeepsSingleDefault(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first := model.AIConfig{Name: "First", Provider: "openai-compatible", ModelName: "gpt-4o", IsDefault: true}
	if err := store.SaveAIConfig(ctx, &first); err != nil {
		t.Fatalf("save first ai config: %v", err)
	}
	second := model.AIConfig{Name: "Second", Provider: "openai-compatible", ModelName: "qwen-max", IsDefault: true}
	if err := store.SaveAIConfig(ctx, &second); err != nil {
		t.Fatalf("save second ai config: %v", err)
	}

	configs, err := store.ListAIConfigs(ctx)
	if err != nil {
		t.Fatalf("list ai configs: %v", err)
	}
	defaultIDs := make([]int64, 0)
	for _, config := range configs {
		if config.IsDefault {
			defaultIDs = append(defaultIDs, config.ID)
		}
	}
	if len(defaultIDs) != 1 || defaultIDs[0] != second.ID {
		t.Fatalf("expected only second config as default, got default ids %+v from %+v", defaultIDs, configs)
	}
}

// TestAIConfigRepositoryPersistsAfterDatabaseReopen 验证新增模型配置写入文件型 SQLite 后可跨进程重启读取。
func TestAIConfigRepositoryPersistsAfterDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "invest-compass.sqlite3")

	db, err := Open(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate sqlite database: %v", err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	config := model.AIConfig{
		Name:           "DeepSeek",
		Provider:       "deepseek",
		APIKeyRef:      "local-vault://ai-config/deepseek-test",
		MaskedAPIKey:   "sk-3****88f4",
		HasAPIKey:      true,
		ModelName:      "deepseek-v4-flash",
		Temperature:    0.7,
		MaxTokens:      4096,
		TimeoutSeconds: 60,
	}
	if err := store.SaveAIConfig(ctx, &config); err != nil {
		t.Fatalf("save ai config: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("read sqlite handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite database: %v", err)
	}

	reopenedDB, err := Open(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("reopen sqlite database: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := reopenedDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	if err := Migrate(ctx, reopenedDB); err != nil {
		t.Fatalf("migrate reopened sqlite database: %v", err)
	}
	reopenedStore, err := NewStore(reopenedDB)
	if err != nil {
		t.Fatalf("new reopened store: %v", err)
	}
	configs, err := reopenedStore.ListAIConfigs(ctx)
	if err != nil {
		t.Fatalf("list ai configs after reopen: %v", err)
	}
	if len(configs) != 1 ||
		configs[0].Name != "DeepSeek" ||
		configs[0].Provider != "deepseek" ||
		configs[0].ModelName != "deepseek-v4-flash" ||
		!configs[0].HasAPIKey {
		t.Fatalf("unexpected ai config after reopen: %+v", configs)
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
	reports, err := store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		t.Fatalf("list visible reports before favorite: %v", err)
	}
	if err := store.UpdateAnalysisReportFavorite(ctx, reports[0].ID, true); err != nil {
		t.Fatalf("favorite report: %v", err)
	}
	report.Title = "新报告"
	report.ContentMarkdown = "new"
	if err := store.SaveAnalysisReportByTaskID(ctx, &report); err != nil {
		t.Fatalf("upsert report: %v", err)
	}

	reports, err = store.ListVisibleAnalysisReports(ctx)
	if err != nil {
		t.Fatalf("list visible reports: %v", err)
	}
	if len(reports) != 1 || reports[0].Title != "新报告" || reports[0].ContentMarkdown != "new" || !reports[0].Favorite {
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

// TestSearchIndexRepositoryStoresActiveBatches 验证搜索索引状态表能稳定保存当前可查询 batch。
func TestSearchIndexRepositoryStoresActiveBatches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SetSearchIndexState(ctx, "active_stock_batch_id", "stock-batch-1"); err != nil {
		t.Fatalf("set active stock batch: %v", err)
	}
	if err := store.SetSearchIndexState(ctx, "active_document_batch_id", "doc-batch-1"); err != nil {
		t.Fatalf("set active document batch: %v", err)
	}
	if err := store.SetSearchIndexState(ctx, "active_stock_batch_id", "stock-batch-2"); err != nil {
		t.Fatalf("update active stock batch: %v", err)
	}

	stockBatch, ok, err := store.GetSearchIndexState(ctx, "active_stock_batch_id")
	if err != nil {
		t.Fatalf("get active stock batch: %v", err)
	}
	if !ok || stockBatch != "stock-batch-2" {
		t.Fatalf("unexpected active stock batch: ok=%v value=%q", ok, stockBatch)
	}
	documentBatch, ok, err := store.GetSearchIndexState(ctx, "active_document_batch_id")
	if err != nil {
		t.Fatalf("get active document batch: %v", err)
	}
	if !ok || documentBatch != "doc-batch-1" {
		t.Fatalf("unexpected active document batch: ok=%v value=%q", ok, documentBatch)
	}
}

// TestSearchIndexBatchRepositoryKeepsReadyBatchActive 验证失败重建不会覆盖旧的 READY batch。
func TestSearchIndexBatchRepositoryKeepsReadyBatchActive(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	readyBatch := model.SearchIndexBatch{
		BatchID:             "stock-ready-1",
		Scope:               "stock",
		Status:              SearchIndexBatchStatusReady,
		SourceSchemaVersion: 1,
	}
	if err := store.SaveSearchIndexBatch(ctx, &readyBatch); err != nil {
		t.Fatalf("save ready batch: %v", err)
	}
	if err := store.SetSearchIndexState(ctx, "active_stock_batch_id", readyBatch.BatchID); err != nil {
		t.Fatalf("activate ready batch: %v", err)
	}

	failedBatch := model.SearchIndexBatch{
		BatchID:             "stock-building-2",
		Scope:               "stock",
		Status:              SearchIndexBatchStatusBuilding,
		SourceSchemaVersion: 1,
	}
	if err := store.SaveSearchIndexBatch(ctx, &failedBatch); err != nil {
		t.Fatalf("save building batch: %v", err)
	}
	failedBatch.Status = SearchIndexBatchStatusFailed
	failedBatch.ErrorMessage = "fts rebuild failed"
	if err := store.SaveSearchIndexBatch(ctx, &failedBatch); err != nil {
		t.Fatalf("mark failed batch: %v", err)
	}

	activeBatch, ok, err := store.GetSearchIndexState(ctx, "active_stock_batch_id")
	if err != nil {
		t.Fatalf("get active batch after failed rebuild: %v", err)
	}
	if !ok || activeBatch != readyBatch.BatchID {
		t.Fatalf("failed rebuild must keep old active batch, got ok=%v value=%q", ok, activeBatch)
	}
}

// TestSearchIndexBatchRepositoryActivatesReadyBatchesAtomically 验证 READY batch 切换会更新 active 指针并退休旧 batch。
func TestSearchIndexBatchRepositoryActivatesReadyBatchesAtomically(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	oldStock := model.SearchIndexBatch{BatchID: "stock-old", Scope: "stock", Status: SearchIndexBatchStatusReady, SourceSchemaVersion: 1}
	oldDocument := model.SearchIndexBatch{BatchID: "doc-old", Scope: "document", Status: SearchIndexBatchStatusReady, SourceSchemaVersion: 1}
	newStock := model.SearchIndexBatch{BatchID: "stock-new", Scope: "stock", Status: SearchIndexBatchStatusReady, SourceSchemaVersion: 1}
	newDocument := model.SearchIndexBatch{BatchID: "doc-new", Scope: "document", Status: SearchIndexBatchStatusReady, SourceSchemaVersion: 1}
	for _, batch := range []*model.SearchIndexBatch{&oldStock, &oldDocument, &newStock, &newDocument} {
		if err := store.SaveSearchIndexBatch(ctx, batch); err != nil {
			t.Fatalf("save batch: %v", err)
		}
	}
	if err := store.SetSearchIndexState(ctx, "active_stock_batch_id", oldStock.BatchID); err != nil {
		t.Fatalf("set old stock active: %v", err)
	}
	if err := store.SetSearchIndexState(ctx, "active_document_batch_id", oldDocument.BatchID); err != nil {
		t.Fatalf("set old document active: %v", err)
	}

	if err := store.ActivateSearchIndexBatches(ctx, newStock.BatchID, newDocument.BatchID); err != nil {
		t.Fatalf("activate search index batches: %v", err)
	}

	activeStock, ok, err := store.GetSearchIndexState(ctx, "active_stock_batch_id")
	if err != nil || !ok || activeStock != newStock.BatchID {
		t.Fatalf("unexpected active stock batch: value=%q ok=%v err=%v", activeStock, ok, err)
	}
	activeDocument, ok, err := store.GetSearchIndexState(ctx, "active_document_batch_id")
	if err != nil || !ok || activeDocument != newDocument.BatchID {
		t.Fatalf("unexpected active document batch: value=%q ok=%v err=%v", activeDocument, ok, err)
	}

	var retired []model.SearchIndexBatch
	if err := store.db.WithContext(ctx).Where("status = ?", SearchIndexBatchStatusRetired).Find(&retired).Error; err != nil {
		t.Fatalf("list retired batches: %v", err)
	}
	if len(retired) != 2 {
		t.Fatalf("expected old batches retired, got %+v", retired)
	}
}

// TestSearchIndexOverviewCountsActiveBatches 验证设置中心索引概览只统计 active batch，并返回词典元数据。
func TestSearchIndexOverviewCountsActiveBatches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	stockFinishedAt := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	documentFinishedAt := time.Date(2026, 6, 22, 13, 30, 0, 0, time.UTC)
	stockBatch := model.SearchIndexBatch{
		BatchID:             "stock-ready-1",
		Scope:               "stock",
		Status:              SearchIndexBatchStatusReady,
		SourceSchemaVersion: 1,
		TokenizerName:       "simple",
		TokenizerVersion:    "1",
		DictionaryHash:      "builtin",
		FinishedAt:          &stockFinishedAt,
	}
	documentBatch := model.SearchIndexBatch{
		BatchID:             "document-ready-1",
		Scope:               "document",
		Status:              SearchIndexBatchStatusReady,
		SourceSchemaVersion: 1,
		TokenizerName:       "simple",
		TokenizerVersion:    "1",
		DictionaryHash:      "builtin",
		FinishedAt:          &documentFinishedAt,
	}
	for _, batch := range []*model.SearchIndexBatch{&stockBatch, &documentBatch} {
		if err := store.SaveSearchIndexBatch(ctx, batch); err != nil {
			t.Fatalf("save search index batch: %v", err)
		}
	}
	if err := store.ReplaceStockSearchFTS(ctx, stockBatch.BatchID, []StockSearchFTSRow{
		{Symbol: "CN:SH:600519", Market: "CN", Exchange: "SH", Code: "600519", NameIndex: "贵州茅台"},
		{Symbol: "CN:SZ:000001", Market: "CN", Exchange: "SZ", Code: "000001", NameIndex: "平安银行"},
	}); err != nil {
		t.Fatalf("replace stock fts: %v", err)
	}
	for _, document := range []model.SearchDocument{
		{BatchID: documentBatch.BatchID, DocUID: "report:1", DocType: "report", RefTable: "analysis_reports", RefID: "1", Title: "报告"},
		{BatchID: documentBatch.BatchID, DocUID: "news:2", DocType: "news", RefTable: "news_items", RefID: "2", Title: "新闻"},
		{BatchID: documentBatch.BatchID, DocUID: "news:3", DocType: "news", RefTable: "news_items", RefID: "3", Title: "新闻二"},
		{BatchID: documentBatch.BatchID, DocUID: "watchlist_note:4", DocType: "watchlist_note", RefTable: "watchlists", RefID: "4", Title: "备注"},
		{BatchID: "document-old", DocUID: "report:old", DocType: "report", RefTable: "analysis_reports", RefID: "99", Title: "旧报告"},
	} {
		doc := document
		if err := store.UpsertSearchDocument(ctx, &doc); err != nil {
			t.Fatalf("upsert search document: %v", err)
		}
	}

	overview, err := store.GetSearchIndexOverview(ctx, stockBatch.BatchID, documentBatch.BatchID)
	if err != nil {
		t.Fatalf("get search index overview: %v", err)
	}
	if overview.StockCount != 2 || overview.ReportCount != 1 || overview.NewsCount != 2 || overview.WatchlistNoteCount != 1 {
		t.Fatalf("unexpected overview counts: %+v", overview)
	}
	if overview.LastRebuildAt == nil || !overview.LastRebuildAt.Equal(documentFinishedAt) {
		t.Fatalf("unexpected last rebuild time: %+v", overview.LastRebuildAt)
	}
	if overview.TokenizerName != "simple" || overview.TokenizerVersion != "1" || overview.DictionaryHash != "builtin" {
		t.Fatalf("unexpected tokenizer metadata: %+v", overview)
	}
}

// TestSearchIndexBatchRepositoryRejectsNonReadyActivation 验证非 READY batch 不能切换 active 指针。
func TestSearchIndexBatchRepositoryRejectsNonReadyActivation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	oldStock := model.SearchIndexBatch{BatchID: "stock-old", Scope: "stock", Status: SearchIndexBatchStatusReady, SourceSchemaVersion: 1}
	buildingDocument := model.SearchIndexBatch{BatchID: "doc-building", Scope: "document", Status: SearchIndexBatchStatusBuilding, SourceSchemaVersion: 1}
	if err := store.SaveSearchIndexBatch(ctx, &oldStock); err != nil {
		t.Fatalf("save old stock batch: %v", err)
	}
	if err := store.SaveSearchIndexBatch(ctx, &buildingDocument); err != nil {
		t.Fatalf("save building document batch: %v", err)
	}
	if err := store.SetSearchIndexState(ctx, "active_stock_batch_id", oldStock.BatchID); err != nil {
		t.Fatalf("set old stock active: %v", err)
	}

	if err := store.ActivateSearchIndexBatches(ctx, oldStock.BatchID, buildingDocument.BatchID); err == nil {
		t.Fatal("expected non-ready document batch activation to fail")
	}
	activeStock, ok, err := store.GetSearchIndexState(ctx, "active_stock_batch_id")
	if err != nil || !ok || activeStock != oldStock.BatchID {
		t.Fatalf("failed activation must preserve old stock active batch, value=%q ok=%v err=%v", activeStock, ok, err)
	}
}

// TestStockSearchFTSRepositoryFiltersByBatch 验证股票 FTS 查询只读取指定 batch，避免 BUILDING batch 泄露。
func TestStockSearchFTSRepositoryFiltersByBatch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.ReplaceStockSearchFTS(ctx, "stock-ready-1", []StockSearchFTSRow{{
		Symbol:         "CN:SH:600519",
		Market:         "CN",
		Exchange:       "SH",
		Code:           "600519",
		CodePrefix:     "600",
		NameIndex:      "贵州茅台 maotai",
		FullNameIndex:  "贵州茅台酒股份有限公司",
		AliasIndex:     "茅台",
		PinyinFull:     "guizhoumaotai",
		PinyinInitials: "gzmt",
		IndustryIndex:  "白酒",
		ConceptIndex:   "消费",
	}}); err != nil {
		t.Fatalf("replace ready stock fts: %v", err)
	}
	if err := store.ReplaceStockSearchFTS(ctx, "stock-building-2", []StockSearchFTSRow{{
		Symbol:         "CN:SH:601398",
		Market:         "CN",
		Exchange:       "SH",
		Code:           "601398",
		CodePrefix:     "601",
		NameIndex:      "工商银行 maotai",
		PinyinFull:     "gongshangyinhang",
		PinyinInitials: "gsyh",
	}}); err != nil {
		t.Fatalf("replace building stock fts: %v", err)
	}

	matches, err := store.SearchStockFTS(ctx, "stock-ready-1", "maotai", 10)
	if err != nil {
		t.Fatalf("search ready stock fts: %v", err)
	}
	if len(matches) != 1 || matches[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected only ready batch stock, got %+v", matches)
	}
}

// TestSearchDocumentRepositorySoftDeleteRemovesFTS 验证菜单文档软删除后不会继续被 FTS 命中。
func TestSearchDocumentRepositorySoftDeleteRemovesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	sourceTime := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)

	document := model.SearchDocument{
		BatchID:    "doc-ready-1",
		DocUID:     "report:task-1",
		DocType:    "report",
		RefTable:   "analysis_reports",
		RefID:      "task-1",
		Symbol:     "CN:SH:600519",
		Title:      "贵州茅台分析报告",
		Summary:    "sanitized momentum summary",
		Source:     "analysis_report",
		SourceTime: sourceTime,
		IndexedAt:  sourceTime,
	}
	if err := store.UpsertSearchDocument(ctx, &document); err != nil {
		t.Fatalf("upsert search document: %v", err)
	}
	if err := store.ReplaceSearchDocumentFTS(ctx, SearchDocumentFTSRow{
		BatchID:     document.BatchID,
		DocUID:      document.DocUID,
		DocType:     document.DocType,
		Symbol:      document.Symbol,
		TitleIndex:  document.Title,
		BodyIndex:   document.Summary,
		TagIndex:    "report stock_full",
		PinyinIndex: "guizhoumaotai",
	}); err != nil {
		t.Fatalf("replace search document fts: %v", err)
	}

	matches, err := store.SearchDocumentsFTS(ctx, "doc-ready-1", "report", "guizhoumaotai", 10)
	if err != nil {
		t.Fatalf("search documents fts: %v", err)
	}
	if len(matches) != 1 || matches[0].DocUID != document.DocUID {
		t.Fatalf("expected document match, got %+v", matches)
	}

	if err := store.SoftDeleteSearchDocument(ctx, document.BatchID, document.DocUID); err != nil {
		t.Fatalf("soft delete search document: %v", err)
	}
	matches, err = store.SearchDocumentsFTS(ctx, "doc-ready-1", "report", "guizhoumaotai", 10)
	if err != nil {
		t.Fatalf("search documents fts after delete: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected soft deleted document removed from fts, got %+v", matches)
	}
}

// TestSearchIndexJobRepositoryMergesDuplicatePendingJobs 验证同一对象的重复索引任务会被 outbox 合并。
func TestSearchIndexJobRepositoryMergesDuplicatePendingJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	job := model.SearchIndexJob{
		DocType:   "report",
		RefID:     "report-1",
		Operation: "upsert",
		Status:    SearchIndexJobStatusPending,
	}
	if err := store.UpsertSearchIndexJob(ctx, job); err != nil {
		t.Fatalf("upsert first search index job: %v", err)
	}
	job.Attempts = 1
	job.LastError = "previous retry failed"
	if err := store.UpsertSearchIndexJob(ctx, job); err != nil {
		t.Fatalf("upsert duplicate search index job: %v", err)
	}

	jobs, err := store.ListRunnableSearchIndexJobs(ctx, 10)
	if err != nil {
		t.Fatalf("list runnable search index jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected duplicate job to merge, got %+v", jobs)
	}
	if jobs[0].DocType != "report" || jobs[0].RefID != "report-1" || jobs[0].Attempts != 1 {
		t.Fatalf("unexpected merged job: %+v", jobs[0])
	}
	requireCount(t, store, &model.SearchIndexJob{}, 1)
}

// TestSearchIndexJobRepositoryRecoversRunningJobs 验证进程重启后 RUNNING 任务会回到可重试队列。
func TestSearchIndexJobRepositoryRecoversRunningJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSearchIndexJob(ctx, model.SearchIndexJob{
		DocType:   "news",
		RefID:     "news-1",
		Operation: "delete",
		Status:    SearchIndexJobStatusRunning,
		Attempts:  2,
	}); err != nil {
		t.Fatalf("insert running search index job: %v", err)
	}
	if err := store.RecoverRunningSearchIndexJobs(ctx); err != nil {
		t.Fatalf("recover running search index jobs: %v", err)
	}

	jobs, err := store.ListRunnableSearchIndexJobs(ctx, 10)
	if err != nil {
		t.Fatalf("list recovered search index jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != SearchIndexJobStatusFailedRetryable || jobs[0].Attempts != 2 {
		t.Fatalf("unexpected recovered job: %+v", jobs)
	}
}

// TestSearchIndexJobRepositoryMarksTerminalStatus 验证 outbox 状态更新会保存终态并脱敏错误摘要。
func TestSearchIndexJobRepositoryMarksTerminalStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSearchIndexJob(ctx, model.SearchIndexJob{
		DocType:   "watchlist_note",
		RefID:     "watchlist-1",
		Operation: "upsert",
		Status:    SearchIndexJobStatusPending,
	}); err != nil {
		t.Fatalf("insert terminal status job: %v", err)
	}
	jobs, err := store.ListRunnableSearchIndexJobs(ctx, 10)
	if err != nil {
		t.Fatalf("list terminal status job: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one terminal status job, got %+v", jobs)
	}

	if err := store.MarkSearchIndexJobStatus(ctx, jobs[0].ID, SearchIndexJobStatusFailedFinal, "raw_api_key=sk-secret"); err != nil {
		t.Fatalf("mark failed final job: %v", err)
	}
	var stored model.SearchIndexJob
	if err := store.db.WithContext(ctx).First(&stored, jobs[0].ID).Error; err != nil {
		t.Fatalf("get terminal status job: %v", err)
	}
	if stored.Status != SearchIndexJobStatusFailedFinal {
		t.Fatalf("unexpected terminal status: %+v", stored)
	}
	if strings.Contains(stored.LastError, "sk-secret") {
		t.Fatalf("expected terminal error redacted, got %q", stored.LastError)
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
	if err := store.AppendTaskLogs(ctx, []model.TaskLogEntry{{
		TaskID: "task-1", Level: "ERROR", Module: "ai", Stage: "stream_failed", Message: "timeout",
	}}); err != nil {
		t.Fatalf("seed task log: %v", err)
	}
	if err := store.UpsertTaskErrorDiagnosis(ctx, model.TaskErrorDiagnosis{
		TaskID:          "task-1",
		ErrorCode:       "provider_timeout",
		ErrorStage:      "stream_failed",
		Summary:         "模型服务响应超时",
		CausesJSON:      `["timeout"]`,
		SuggestionsJSON: `["retry"]`,
	}); err != nil {
		t.Fatalf("seed task diagnosis: %v", err)
	}
	if err := store.SaveAnalysisReportByTaskID(ctx, &model.AnalysisReport{TaskID: "task-1", Symbol: "US:AAPL", Title: "report", AnalysisType: "stock_full"}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.Task{ID: "task-1", Type: "AI 分析", Status: "FAILED"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := store.db.WithContext(ctx).Create(&model.TaskEvent{TaskID: "task-1", EventType: "TASK_FAILED"}).Error; err != nil {
		t.Fatalf("seed task event: %v", err)
	}
	if err := store.UpsertSetting(ctx, model.Setting{Key: "theme", Value: "dark"}); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	err := store.CleanCache(ctx, []settings.CacheTarget{
		settings.CacheTargetQuote,
		settings.CacheTargetKline,
		settings.CacheTargetNews,
		settings.CacheTargetTaskLogs,
		settings.CacheTargetReport,
		settings.CacheTargetConfig,
	})
	if err != nil {
		t.Fatalf("clean cache: %v", err)
	}

	requireCount(t, store, &model.Quote{}, 0)
	requireCount(t, store, &model.Kline{}, 0)
	requireCount(t, store, &model.NewsItem{}, 0)
	requireCount(t, store, &model.TaskLogEntry{}, 0)
	requireCount(t, store, &model.TaskErrorDiagnosis{}, 0)
	requireCount(t, store, &model.Task{}, 1)
	requireCount(t, store, &model.TaskEvent{}, 1)
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
	if err := store.AppendTaskLogs(ctx, []model.TaskLogEntry{{
		TaskID: "task-1", Level: "INFO", Module: "ai", Stage: "prompt_build", Message: "Prompt 构建完成",
	}}); err != nil {
		t.Fatalf("seed task log: %v", err)
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
	requirePositiveCacheUsage(t, usages, settings.CacheTargetTaskLogs)
	requireMissingCacheUsage(t, usages, settings.CacheTargetReport)
	requireMissingCacheUsage(t, usages, settings.CacheTargetConfig)
}

// TestSchedulerRepositoryPersistsJobsRunsAndWatermarks 验证调度任务、执行记录和抓取水位的最小持久化契约。
func TestSchedulerRepositoryPersistsJobsRunsAndWatermarks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	nextRunAt := time.Date(2026, 6, 19, 9, 30, 0, 0, time.Local)

	job := model.SchedulerJob{
		Name:           "A 股开盘行情刷新",
		CronType:       "cn_a_share_quote_refresh",
		CronExpr:       "30 9 * * 1-5",
		Enabled:        true,
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TradeWindow:    "open",
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON:     `{"period":"1m"}`,
		CatchupEnabled: true,
		CatchupMaxDays: 5,
		TimeoutSeconds: 120,
		NextRunAt:      &nextRunAt,
	}
	if err := store.SaveSchedulerJob(ctx, &job); err != nil {
		t.Fatalf("save scheduler job: %v", err)
	}

	jobs, err := store.ListSchedulerJobs(ctx)
	if err != nil {
		t.Fatalf("list scheduler jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].CronType != "cn_a_share_quote_refresh" || !jobs[0].Enabled {
		t.Fatalf("unexpected scheduler jobs: %+v", jobs)
	}
	gotJob, ok, err := store.GetSchedulerJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get scheduler job: %v", err)
	}
	if !ok || gotJob.ID != job.ID || gotJob.CronType != job.CronType {
		t.Fatalf("unexpected scheduler job detail: ok=%v job=%+v", ok, gotJob)
	}
	enabledJobs, err := store.ListEnabledSchedulerJobs(ctx)
	if err != nil {
		t.Fatalf("list enabled scheduler jobs: %v", err)
	}
	if len(enabledJobs) != 1 {
		t.Fatalf("expected one enabled scheduler job, got %+v", enabledJobs)
	}
	if err := store.SetSchedulerJobEnabled(ctx, job.ID, false); err != nil {
		t.Fatalf("disable scheduler job: %v", err)
	}
	enabledJobs, err = store.ListEnabledSchedulerJobs(ctx)
	if err != nil {
		t.Fatalf("list enabled scheduler jobs after disable: %v", err)
	}
	if len(enabledJobs) != 0 {
		t.Fatalf("expected disabled scheduler job to be excluded, got %+v", enabledJobs)
	}

	run := model.SchedulerRun{
		JobID:       job.ID,
		CronType:    "cn_a_share_quote_refresh",
		DataType:    "quote",
		Period:      "",
		RunKey:      "job-1:2026-06-19:missed_today",
		TriggerType: "missed_today",
		Status:      "queued",
		Priority:    50,
		Source:      "startup_restore",
		TargetDate:  "2026-06-19",
		ScopeKey:    "CN:SH:600519",
	}
	if err := store.CreateSchedulerRun(ctx, &run); err != nil {
		t.Fatalf("create scheduler run: %v", err)
	}
	if err := store.CreateSchedulerRun(ctx, &model.SchedulerRun{JobID: job.ID, CronType: "cn_a_share_quote_refresh", DataType: "quote", RunKey: run.RunKey, TriggerType: "missed_today", Status: "queued"}); err == nil {
		t.Fatal("expected duplicate scheduler run_key to fail")
	}
	startedAt := time.Date(2026, 6, 19, 10, 0, 0, 0, time.Local)
	run.Status = "running"
	run.StartedAt = &startedAt
	if err := store.UpdateSchedulerRun(ctx, &run); err != nil {
		t.Fatalf("update scheduler run: %v", err)
	}
	gotJob, ok, err = store.GetSchedulerJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get scheduler job after run update: %v", err)
	}
	if !ok || gotJob.LastStatus != "running" || gotJob.LastError != "" || gotJob.LastRunAt == nil {
		t.Fatalf("expected scheduler job recent state to follow run update, ok=%v job=%+v", ok, gotJob)
	}
	runningRuns, err := store.ListSchedulerRunsByStatuses(ctx, []string{"running"})
	if err != nil {
		t.Fatalf("list running scheduler runs: %v", err)
	}
	if len(runningRuns) != 1 || runningRuns[0].RunKey != run.RunKey || runningRuns[0].TriggerType != "missed_today" {
		t.Fatalf("unexpected running scheduler runs: %+v", runningRuns)
	}
	allRuns, err := store.ListSchedulerRuns(ctx, job.ID, 10)
	if err != nil {
		t.Fatalf("list scheduler runs: %v", err)
	}
	if len(allRuns) != 1 || allRuns[0].RunKey != run.RunKey {
		t.Fatalf("unexpected scheduler runs: %+v", allRuns)
	}
	gotRun, ok, err := store.GetSchedulerRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("get scheduler run: %v", err)
	}
	if !ok || gotRun.ID != run.ID || gotRun.RunKey != run.RunKey {
		t.Fatalf("unexpected scheduler run detail: ok=%v run=%+v", ok, gotRun)
	}

	watermark := model.IngestionWatermark{
		DataType:      "quote",
		ScopeKey:      "CN:SH:600519",
		Provider:      "sina_tencent",
		Period:        "",
		LastSuccessAt: &startedAt,
		LastTradeDate: "2026-06-19",
		CursorJSON:    `{"last_quote_time":"2026-06-19T10:00:00+08:00"}`,
	}
	if err := store.UpsertIngestionWatermark(ctx, &watermark); err != nil {
		t.Fatalf("upsert ingestion watermark: %v", err)
	}
	watermark.LastTradeDate = "2026-06-20"
	if err := store.UpsertIngestionWatermark(ctx, &watermark); err != nil {
		t.Fatalf("update ingestion watermark: %v", err)
	}
	gotWatermark, ok, err := store.GetIngestionWatermark(ctx, "quote", "CN:SH:600519", "sina_tencent", "")
	if err != nil {
		t.Fatalf("get ingestion watermark: %v", err)
	}
	if !ok || gotWatermark.LastTradeDate != "2026-06-20" {
		t.Fatalf("unexpected ingestion watermark: ok=%v watermark=%+v", ok, gotWatermark)
	}

	if err := store.SoftDeleteSchedulerJob(ctx, job.ID); err != nil {
		t.Fatalf("soft delete scheduler job: %v", err)
	}
	jobs, err = store.ListSchedulerJobs(ctx)
	if err != nil {
		t.Fatalf("list scheduler jobs after delete: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected soft deleted scheduler job to be hidden, got %+v", jobs)
	}
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
