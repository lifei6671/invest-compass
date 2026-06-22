package search

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestStockSearchServiceReturnsLocalRankedResults 验证本地缓存命中时不调用远端 Provider。
func TestStockSearchServiceReturnsLocalRankedResults(t *testing.T) {
	store := &fakeStockSearchStore{
		localStocks: []model.Stock{
			{Symbol: "CN:SH:601001", Code: "601001", Name: "煤炭股", Concept: "茅台概念", Status: "LISTED"},
			{Symbol: "CN:SH:600519", Code: "600519", Name: "贵州茅台", Status: "LISTED"},
		},
		aliases: map[string][]model.StockAlias{
			"CN:SH:600519": {{Symbol: "CN:SH:600519", Alias: "茅台"}},
		},
	}
	provider := &fakeStockSearchProvider{err: errors.New("provider must not be called")}
	service := NewStockSearchService(StockSearchConfig{Store: store, Provider: provider})

	results, err := service.Search(context.Background(), "茅台", 10)
	if err != nil {
		t.Fatalf("search local stocks: %v", err)
	}
	if len(results) != 2 || results[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected local ranked result first, got %+v", results)
	}
	if provider.searchCalls != 0 {
		t.Fatalf("expected provider not called on local hit, got %d calls", provider.searchCalls)
	}
}

// TestStockSearchServiceUsesFTSActiveBatch 验证本地强规则无命中时会使用当前 active FTS batch。
func TestStockSearchServiceUsesFTSActiveBatch(t *testing.T) {
	store := &fakeStockSearchStore{
		state: map[string]string{"active_stock_batch_id": "stock-ready-1"},
		ftsMatches: []dao.StockSearchFTSMatch{
			{BatchID: "stock-ready-1", Symbol: "CN:SH:600519", Code: "600519"},
		},
		stocksBySymbol: map[string]model.Stock{
			"CN:SH:600519": {Symbol: "CN:SH:600519", Code: "600519", Name: "贵州茅台", Status: "LISTED"},
		},
	}
	service := NewStockSearchService(StockSearchConfig{Store: store, Provider: market.UnconfiguredProvider{}})

	results, err := service.Search(context.Background(), "gzmt", 10)
	if err != nil {
		t.Fatalf("search fts stocks: %v", err)
	}
	if len(results) != 1 || results[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected fts result, got %+v", results)
	}
	if store.lastFTSBatch != "stock-ready-1" {
		t.Fatalf("expected fts search against active batch, got %q", store.lastFTSBatch)
	}
}

// TestStockSearchServiceFallsBackToProviderAndEnqueuesIndexJobs 验证本地无命中时写入 Provider 结果和增量索引任务。
func TestStockSearchServiceFallsBackToProviderAndEnqueuesIndexJobs(t *testing.T) {
	symbol, err := stockservice.ParseSymbol("CN:SH:600519")
	if err != nil {
		t.Fatalf("parse symbol: %v", err)
	}
	store := &fakeStockSearchStore{}
	provider := &fakeStockSearchProvider{
		results: []market.StockBasic{{
			Symbol:   symbol,
			Name:     "贵州茅台",
			Code:     "600519",
			Market:   "CN",
			Exchange: "SH",
			Industry: "白酒",
		}},
	}
	service := NewStockSearchService(StockSearchConfig{Store: store, Provider: provider})

	results, err := service.Search(context.Background(), "600519", 10)
	if err != nil {
		t.Fatalf("provider fallback: %v", err)
	}
	if len(results) != 1 || results[0].Symbol != "CN:SH:600519" {
		t.Fatalf("unexpected provider fallback results: %+v", results)
	}
	if len(store.upsertedStocks) != 1 || store.upsertedStocks[0].Symbol != "CN:SH:600519" {
		t.Fatalf("expected provider stock cached, got %+v", store.upsertedStocks)
	}
	if len(store.indexJobs) != 1 || store.indexJobs[0].DocType != "stock" || store.indexJobs[0].RefID != "CN:SH:600519" {
		t.Fatalf("expected stock index job, got %+v", store.indexJobs)
	}
}

// TestStockSearchServiceReturnsUnconfiguredErrorWithoutFakeData 验证 Provider 未配置时不会伪造股票。
func TestStockSearchServiceReturnsUnconfiguredErrorWithoutFakeData(t *testing.T) {
	service := NewStockSearchService(StockSearchConfig{Store: &fakeStockSearchStore{}, Provider: market.UnconfiguredProvider{}})

	results, err := service.Search(context.Background(), "不存在", 10)
	if len(results) != 0 {
		t.Fatalf("expected no fake data, got %+v", results)
	}
	var ruleError *xerr.Error
	if !errors.As(err, &ruleError) || ruleError.Code != xerr.MarketProviderUnconfigured {
		t.Fatalf("expected market provider unconfigured error, got %v", err)
	}
}

type fakeStockSearchStore struct {
	localStocks    []model.Stock
	stocksBySymbol map[string]model.Stock
	aliases        map[string][]model.StockAlias
	watchlists     []model.Watchlist
	state          map[string]string
	ftsMatches     []dao.StockSearchFTSMatch
	lastFTSBatch   string
	upsertedStocks []model.Stock
	indexJobs      []model.SearchIndexJob
}

// ListStocksForSearch 返回预置本地候选，symbols 非空时按 symbol 回表。
func (store *fakeStockSearchStore) ListStocksForSearch(_ context.Context, query string, symbols []string, limit int) ([]model.Stock, error) {
	if len(symbols) > 0 {
		result := make([]model.Stock, 0, len(symbols))
		for _, symbol := range symbols {
			if stock, ok := store.stocksBySymbol[symbol]; ok {
				result = append(result, stock)
			}
		}
		return result, nil
	}
	if limit > 0 && len(store.localStocks) > limit {
		return store.localStocks[:limit], nil
	}
	_ = query
	return store.localStocks, nil
}

// ListStockAliasesBySymbols 返回预置股票别名集合。
func (store *fakeStockSearchStore) ListStockAliasesBySymbols(_ context.Context, symbols []string) (map[string][]model.StockAlias, error) {
	result := make(map[string][]model.StockAlias, len(symbols))
	for _, symbol := range symbols {
		result[symbol] = store.aliases[symbol]
	}
	return result, nil
}

// ListActiveWatchlists 返回预置自选股集合。
func (store *fakeStockSearchStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return store.watchlists, nil
}

// GetSearchIndexState 返回预置搜索索引状态。
func (store *fakeStockSearchStore) GetSearchIndexState(_ context.Context, key string) (string, bool, error) {
	if store.state == nil {
		return "", false, nil
	}
	value, ok := store.state[key]
	return value, ok, nil
}

// SearchStockFTS 记录 active batch 并返回预置 FTS 命中。
func (store *fakeStockSearchStore) SearchStockFTS(_ context.Context, batchID string, _ string, _ int) ([]dao.StockSearchFTSMatch, error) {
	store.lastFTSBatch = batchID
	return store.ftsMatches, nil
}

// UpsertStocks 记录 Provider fallback 后写入的股票缓存。
func (store *fakeStockSearchStore) UpsertStocks(_ context.Context, stocks []model.Stock) error {
	store.upsertedStocks = append(store.upsertedStocks, stocks...)
	return nil
}

// UpsertSearchIndexJob 记录 Provider fallback 后追加的搜索 outbox。
func (store *fakeStockSearchStore) UpsertSearchIndexJob(_ context.Context, job model.SearchIndexJob) error {
	store.indexJobs = append(store.indexJobs, job)
	return nil
}

type fakeStockSearchProvider struct {
	results     []market.StockBasic
	err         error
	searchCalls int
}

// Name 返回测试 Provider 名称。
func (provider *fakeStockSearchProvider) Name() string {
	return "fake-market"
}

// Status 返回测试 Provider 状态。
func (provider *fakeStockSearchProvider) Status(context.Context) market.ProviderStatus {
	return market.ProviderStatus{Name: provider.Name(), Available: provider.err == nil}
}

// Search 返回预置远端搜索结果并记录调用次数。
func (provider *fakeStockSearchProvider) Search(context.Context, string) ([]market.StockBasic, error) {
	provider.searchCalls++
	if provider.err != nil {
		return nil, provider.err
	}
	return provider.results, nil
}

// Quote 满足 MarketProvider 接口，股票搜索测试不使用该路径。
func (provider *fakeStockSearchProvider) Quote(context.Context, stockservice.Symbol) (market.Quote, error) {
	return market.Quote{}, nil
}

// Kline 满足 MarketProvider 接口，股票搜索测试不使用该路径。
func (provider *fakeStockSearchProvider) Kline(context.Context, market.KlineRequest) ([]market.KlineBar, error) {
	return nil, nil
}
