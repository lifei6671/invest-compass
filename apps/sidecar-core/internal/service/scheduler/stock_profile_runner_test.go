package scheduler

import (
	"context"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestStockProfileRefreshRunnerFetchesWatchlistStocks 验证主动刷新会从 active watchlist 派生股票基础资料并写入 stocks 与搜索索引 outbox。
func TestStockProfileRefreshRunnerFetchesWatchlistStocks(t *testing.T) {
	provider := &stockProfileProvider{
		results: map[string][]marketservice.StockBasic{
			"600519": {
				{
					Symbol:   mustParseSymbol(t, "CN:SH:600519"),
					Code:     "600519",
					Name:     "贵州茅台",
					Market:   "CN",
					Exchange: "SH",
					Industry: "白酒",
					Concept:  "消费",
				},
			},
		},
	}
	store := &stockProfileStore{
		watchlists: []model.Watchlist{
			{Symbol: "US:AAPL"},
			{Symbol: "CN:SH:600519"},
		},
	}
	runner := StockProfileRefreshRunner{Provider: provider, Store: store}

	result, err := runner.Run(context.Background(), model.SchedulerRun{
		CronType:   CronTypeStockProfileRefresh,
		ScopeKey:   "CN",
		TargetDate: "2026-06-22",
	})
	if err != nil {
		t.Fatalf("run stock profile refresh: %v", err)
	}

	if result.FetchedCount != 1 || result.WrittenCount != 1 {
		t.Fatalf("unexpected stock profile refresh result: %+v", result)
	}
	if len(store.stocks) != 1 || store.stocks[0].Symbol != "CN:SH:600519" || store.stocks[0].Industry != "白酒" {
		t.Fatalf("unexpected upserted stocks: %+v", store.stocks)
	}
	if len(store.indexJobs) != 1 || store.indexJobs[0].DocType != "stock" || store.indexJobs[0].RefID != "CN:SH:600519" {
		t.Fatalf("unexpected search index jobs: %+v", store.indexJobs)
	}
	if len(store.watermarks) != 1 || store.watermarks[0].DataType != "stock_profile" || store.watermarks[0].ScopeKey != "CN:SH:600519" {
		t.Fatalf("unexpected stock profile watermarks: %+v", store.watermarks)
	}
	if len(provider.keywords) != 1 || provider.keywords[0] != "600519" {
		t.Fatalf("expected provider search by code, got %+v", provider.keywords)
	}
}

// TestDefaultJobRegistryIncludesStockProfileRefresh 验证股票基础资料主动刷新进入可配置调度任务列表。
func TestDefaultJobRegistryIncludesStockProfileRefresh(t *testing.T) {
	types := DefaultJobRegistry().Types()
	for _, item := range types {
		if item.CronType == CronTypeStockProfileRefresh && item.Label == "股票基础资料刷新" && item.Market == "CN" {
			return
		}
	}
	t.Fatalf("stock profile refresh type is missing from default registry: %+v", types)
}

type stockProfileProvider struct {
	results  map[string][]marketservice.StockBasic
	keywords []string
}

// Name 返回测试 Provider 名称。
func (provider *stockProfileProvider) Name() string {
	return "test-market"
}

// Status 返回测试 Provider 可用状态。
func (provider *stockProfileProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Available: true}
}

// Search 返回测试预置的股票基础资料。
func (provider *stockProfileProvider) Search(_ context.Context, keyword string) ([]marketservice.StockBasic, error) {
	provider.keywords = append(provider.keywords, keyword)
	return append([]marketservice.StockBasic(nil), provider.results[keyword]...), nil
}

// Quote 在股票基础资料刷新测试中不使用。
func (provider *stockProfileProvider) Quote(context.Context, stockservice.Symbol) (marketservice.Quote, error) {
	return marketservice.Quote{}, nil
}

// Kline 在股票基础资料刷新测试中不使用。
func (provider *stockProfileProvider) Kline(context.Context, marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	return nil, nil
}

type stockProfileStore struct {
	watchlists []model.Watchlist
	stocks     []model.Stock
	indexJobs  []model.SearchIndexJob
	watermarks []model.IngestionWatermark
}

// ListActiveWatchlists 返回测试预置 active watchlist。
func (store *stockProfileStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return append([]model.Watchlist(nil), store.watchlists...), nil
}

// UpsertStocks 记录测试写入的股票基础资料。
func (store *stockProfileStore) UpsertStocks(_ context.Context, stocks []model.Stock) error {
	store.stocks = append(store.stocks, stocks...)
	return nil
}

// UpsertSearchIndexJob 记录测试写入的搜索索引 outbox。
func (store *stockProfileStore) UpsertSearchIndexJob(_ context.Context, job model.SearchIndexJob) error {
	store.indexJobs = append(store.indexJobs, job)
	return nil
}

// UpsertIngestionWatermark 记录测试写入的抓取水位。
func (store *stockProfileStore) UpsertIngestionWatermark(_ context.Context, watermark *model.IngestionWatermark) error {
	store.watermarks = append(store.watermarks, *watermark)
	return nil
}
