package search

import (
	"context"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const activeStockBatchStateKey = "active_stock_batch_id"

// StockSearchStore 是股票搜索 service 需要的 DAO 能力边界。
type StockSearchStore interface {
	ListStocksForSearch(ctx context.Context, query string, symbols []string, limit int) ([]model.Stock, error)
	ListStockAliasesBySymbols(ctx context.Context, symbols []string) (map[string][]model.StockAlias, error)
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	GetSearchIndexState(ctx context.Context, key string) (string, bool, error)
	SearchStockFTS(ctx context.Context, batchID string, match string, limit int) ([]dao.StockSearchFTSMatch, error)
	UpsertStocks(ctx context.Context, stocks []model.Stock) error
	UpsertSearchIndexJob(ctx context.Context, job model.SearchIndexJob) error
}

// StockSearchConfig 是股票搜索 service 的运行期依赖。
type StockSearchConfig struct {
	Store    StockSearchStore
	Provider market.MarketProvider
}

// StockSearchService 编排本地缓存、FTS 和 Provider fallback 的股票搜索。
type StockSearchService struct {
	store    StockSearchStore
	provider market.MarketProvider
}

// StockSearchResult 是股票搜索对 action 层暴露的稳定结果。
type StockSearchResult struct {
	Symbol   string
	Name     string
	Code     string
	Market   string
	Exchange string
}

// NewStockSearchService 创建股票搜索 service。
func NewStockSearchService(config StockSearchConfig) *StockSearchService {
	return &StockSearchService{store: config.Store, provider: config.Provider}
}

// Search 先查询本地缓存和 FTS，本地无命中时再调用 Provider fallback。
func (service *StockSearchService) Search(ctx context.Context, keyword string, limit int) ([]StockSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	query := NormalizeQueryInput(keyword)
	if query.Kind == QueryKindEmpty {
		return nil, &xerr.Error{Code: xerr.StockEmptySymbol}
	}

	if service.store != nil {
		localResults, err := service.searchLocal(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		if len(localResults) > 0 {
			return localResults, nil
		}

		ftsResults, err := service.searchFTS(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		if len(ftsResults) > 0 {
			return ftsResults, nil
		}
	}

	return service.searchProvider(ctx, query, limit)
}

// searchLocal 查询本地 stocks 强规则候选，并复用 StockRanker 排序。
func (service *StockSearchService) searchLocal(ctx context.Context, query NormalizedQuery, limit int) ([]StockSearchResult, error) {
	stocks, err := service.store.ListStocksForSearch(ctx, query.Text, nil, limit)
	if err != nil {
		return nil, err
	}
	return service.rankStocks(ctx, query, stocks, nil, limit)
}

// searchFTS 使用当前 active batch 执行 FTS 召回，未完成索引时静默降级到 Provider。
func (service *StockSearchService) searchFTS(ctx context.Context, query NormalizedQuery, limit int) ([]StockSearchResult, error) {
	batchID, ok, err := service.store.GetSearchIndexState(ctx, activeStockBatchStateKey)
	if err != nil {
		return nil, err
	}
	if !ok || batchID == "" {
		return nil, nil
	}
	match := BuildFTSMatch(query, []string{
		"code",
		"code_prefix",
		"name_index",
		"full_name_index",
		"alias_index",
		"pinyin_full",
		"pinyin_initials",
		"industry_index",
		"concept_index",
	})
	if match == "" {
		return nil, nil
	}
	matches, err := service.store.SearchStockFTS(ctx, batchID, match, limit)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	symbols := make([]string, 0, len(matches))
	ranks := make(map[string]int, len(matches))
	for index, match := range matches {
		symbols = append(symbols, match.Symbol)
		ranks[match.Symbol] = index + 1
	}
	stocks, err := service.store.ListStocksForSearch(ctx, query.Text, symbols, limit)
	if err != nil {
		return nil, err
	}
	return service.rankStocks(ctx, query, stocks, ranks, limit)
}

// rankStocks 补齐别名和自选股权重后执行稳定排序。
func (service *StockSearchService) rankStocks(ctx context.Context, query NormalizedQuery, stocks []model.Stock, ftsRanks map[string]int, limit int) ([]StockSearchResult, error) {
	if len(stocks) == 0 {
		return nil, nil
	}
	symbols := stockSymbols(stocks)
	aliases, err := service.store.ListStockAliasesBySymbols(ctx, symbols)
	if err != nil {
		return nil, err
	}
	watchlists, err := service.store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, err
	}
	watchlistSymbols := make(map[string]struct{}, len(watchlists))
	for _, item := range watchlists {
		watchlistSymbols[item.Symbol] = struct{}{}
	}

	candidates := make([]StockCandidate, 0, len(stocks))
	for index, stock := range stocks {
		rank := index + 1
		if ftsRanks != nil && ftsRanks[stock.Symbol] > 0 {
			rank = ftsRanks[stock.Symbol]
		}
		_, inWatchlist := watchlistSymbols[stock.Symbol]
		candidates = append(candidates, StockCandidate{
			Stock:       stock,
			Aliases:     aliasStrings(aliases[stock.Symbol]),
			InWatchlist: inWatchlist,
			FTSRank:     rank,
		})
	}
	ranked := RankStocks(query, candidates)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return stockSearchResultsFromRanked(ranked), nil
}

// searchProvider 调用远端 Provider，并把结果缓存到 stocks 与搜索 outbox。
func (service *StockSearchService) searchProvider(ctx context.Context, query NormalizedQuery, limit int) ([]StockSearchResult, error) {
	if service.provider == nil {
		return nil, &xerr.Error{Code: xerr.MarketProviderUnconfigured}
	}
	stocks, err := service.provider.Search(ctx, query.Raw)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(stocks) > limit {
		stocks = stocks[:limit]
	}
	modelStocks := modelStocksFromMarket(stocks)
	if service.store != nil {
		if err := service.store.UpsertStocks(ctx, modelStocks); err != nil {
			return nil, err
		}
		for _, stock := range modelStocks {
			if err := service.store.UpsertSearchIndexJob(ctx, model.SearchIndexJob{
				DocType:   "stock",
				RefID:     stock.Symbol,
				Operation: "upsert",
				Status:    dao.SearchIndexJobStatusPending,
			}); err != nil {
				return nil, err
			}
		}
	}
	return stockSearchResultsFromModels(modelStocks), nil
}

// stockSymbols 提取股票 symbol 列表。
func stockSymbols(stocks []model.Stock) []string {
	result := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		result = append(result, stock.Symbol)
	}
	return result
}

// aliasStrings 提取别名原文。
func aliasStrings(aliases []model.StockAlias) []string {
	result := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if alias.DeletedAt.Valid {
			continue
		}
		result = append(result, alias.Alias)
	}
	return result
}

// modelStocksFromMarket 转换 Provider 股票基础信息为本地缓存模型。
func modelStocksFromMarket(stocks []market.StockBasic) []model.Stock {
	result := make([]model.Stock, 0, len(stocks))
	for _, item := range stocks {
		result = append(result, model.Stock{
			Symbol:   item.Symbol.String(),
			Market:   item.Market,
			Code:     item.Code,
			Name:     item.Name,
			Exchange: item.Exchange,
			Industry: item.Industry,
			Concept:  item.Concept,
		})
	}
	return result
}

// stockSearchResultsFromRanked 转换排序结果为 API 友好的搜索结果。
func stockSearchResultsFromRanked(ranked []RankedStock) []StockSearchResult {
	result := make([]StockSearchResult, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, StockSearchResult{
			Symbol:   item.Stock.Symbol,
			Name:     item.Stock.Name,
			Code:     item.Stock.Code,
			Market:   item.Stock.Market,
			Exchange: item.Stock.Exchange,
		})
	}
	return result
}

// stockSearchResultsFromModels 转换缓存模型为 API 友好的搜索结果。
func stockSearchResultsFromModels(stocks []model.Stock) []StockSearchResult {
	result := make([]StockSearchResult, 0, len(stocks))
	for _, stock := range stocks {
		result = append(result, StockSearchResult{
			Symbol:   stock.Symbol,
			Name:     stock.Name,
			Code:     stock.Code,
			Market:   stock.Market,
			Exchange: stock.Exchange,
		})
	}
	return result
}
