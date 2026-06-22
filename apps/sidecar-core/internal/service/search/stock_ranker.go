package search

import (
	"sort"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// StockCandidate 是股票搜索排序的候选项。
type StockCandidate struct {
	Stock       model.Stock
	Aliases     []string
	InWatchlist bool
	FTSRank     int
}

// RankedStock 是带排序分数的股票搜索结果。
type RankedStock struct {
	Stock model.Stock
	Score int
	Rank  int
}

// RankStocks 按业务强规则对股票候选项排序。
func RankStocks(query NormalizedQuery, candidates []StockCandidate) []RankedStock {
	ranked := make([]RankedStock, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, RankedStock{
			Stock: candidate.Stock,
			Score: scoreStock(query, candidate),
			Rank:  candidate.FTSRank,
		})
	}
	sort.SliceStable(ranked, func(left int, right int) bool {
		if ranked[left].Score != ranked[right].Score {
			return ranked[left].Score > ranked[right].Score
		}
		leftRank := ranked[left].Rank
		rightRank := ranked[right].Rank
		if leftRank == 0 {
			leftRank = 1 << 30
		}
		if rightRank == 0 {
			rightRank = 1 << 30
		}
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return ranked[left].Stock.Code < ranked[right].Stock.Code
	})
	return ranked
}

// scoreStock 计算单个候选项的业务分数。
func scoreStock(query NormalizedQuery, candidate StockCandidate) int {
	text := query.Text
	stock := candidate.Stock
	score := 0

	if strings.EqualFold(stock.Symbol, text) {
		score += 1000
	}
	if stock.Code == text || strings.EqualFold(stock.Exchange+stock.Code, text) {
		score += 900
	}
	if normalizeCompareText(stock.Name) == text {
		score += 850
	} else if strings.Contains(normalizeCompareText(stock.Name), text) {
		score += 650
	}
	if normalizeCompareText(stock.FullName) == text {
		score += 760
	} else if strings.Contains(normalizeCompareText(stock.FullName), text) {
		score += 520
	}
	for _, alias := range candidate.Aliases {
		if normalizeCompareText(alias) == text {
			score += 800
		} else if strings.Contains(normalizeCompareText(alias), text) {
			score += 600
		}
	}
	if stock.PinyinFull != "" && strings.Contains(strings.ToLower(stock.PinyinFull), text) {
		score += 420
	}
	if stock.PinyinInitials != "" && strings.Contains(strings.ToLower(stock.PinyinInitials), text) {
		score += 430
	}
	if strings.Contains(normalizeCompareText(stock.Industry), text) {
		score += 180
	}
	if strings.Contains(normalizeCompareText(stock.Concept), text) {
		score += 120
	}
	if candidate.InWatchlist {
		score += 80
	}
	if isListedStatus(stock.Status) {
		score += 40
	} else {
		score -= 200
	}
	return score
}

// normalizeCompareText 统一排序比较用文本。
func normalizeCompareText(text string) string {
	return strings.ToLower(collapseSpaces(normalizeFullWidth(strings.TrimSpace(text))))
}

// isListedStatus 判断股票状态是否应视为正常上市。
func isListedStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "", "LISTED", "ACTIVE", "NORMAL", "正常", "上市":
		return true
	default:
		return false
	}
}
