package search

import (
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestRankStocksPrefersExactNameOverConcept 验证名称精确匹配优先于行业概念弱匹配。
func TestRankStocksPrefersExactNameOverConcept(t *testing.T) {
	query := NormalizeQueryInput("茅台")
	ranked := RankStocks(query, []StockCandidate{
		{Stock: model.Stock{Symbol: "CN:SH:601001", Code: "601001", Name: "煤炭股", Concept: "茅台概念", Status: "LISTED"}, FTSRank: 1},
		{Stock: model.Stock{Symbol: "CN:SH:600519", Code: "600519", Name: "贵州茅台", Status: "LISTED"}, Aliases: []string{"茅台"}, FTSRank: 5},
	})

	if len(ranked) != 2 || ranked[0].Stock.Symbol != "CN:SH:600519" {
		t.Fatalf("expected exact name or alias match first, got %+v", ranked)
	}
}

// TestRankStocksBoostsWatchlistAndPenalizesDelisted 验证自选股加权且退市股票降权。
func TestRankStocksBoostsWatchlistAndPenalizesDelisted(t *testing.T) {
	query := NormalizeQueryInput("600519")
	ranked := RankStocks(query, []StockCandidate{
		{Stock: model.Stock{Symbol: "CN:SH:600519", Code: "600519", Name: "贵州茅台", Status: "DELISTED"}, FTSRank: 1},
		{Stock: model.Stock{Symbol: "CN:SH:600519B", Code: "600519", Name: "贵州茅台B", Status: "LISTED"}, InWatchlist: true, FTSRank: 2},
	})

	if len(ranked) != 2 || ranked[0].Stock.Symbol != "CN:SH:600519B" {
		t.Fatalf("expected listed watchlist candidate first, got %+v", ranked)
	}
}

// TestRankStocksUsesStableTieBreakers 验证同分时按 FTS rank 和 code 稳定排序。
func TestRankStocksUsesStableTieBreakers(t *testing.T) {
	query := NormalizeQueryInput("银行")
	ranked := RankStocks(query, []StockCandidate{
		{Stock: model.Stock{Symbol: "CN:SH:601398", Code: "601398", Name: "工商银行", Status: "LISTED"}, FTSRank: 2},
		{Stock: model.Stock{Symbol: "CN:SH:600000", Code: "600000", Name: "浦发银行", Status: "LISTED"}, FTSRank: 1},
	})

	if len(ranked) != 2 || ranked[0].Stock.Code != "600000" {
		t.Fatalf("expected lower fts rank first on tie, got %+v", ranked)
	}
}
