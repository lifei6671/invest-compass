package search

import (
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestBuildStockSearchFTSRowIncludesCodeNameAliasAndPinyin 验证股票索引文本覆盖代码、名称、别名和拼音入口。
func TestBuildStockSearchFTSRowIncludesCodeNameAliasAndPinyin(t *testing.T) {
	row := BuildStockSearchFTSRow(
		model.Stock{
			Symbol:   "CN:SH:600519",
			Market:   "CN",
			Exchange: "SH",
			Code:     "600519",
			Name:     "贵州茅台",
			FullName: "贵州茅台酒股份有限公司",
			Industry: "白酒",
			Concept:  "消费",
		},
		[]model.StockAlias{{Alias: "茅台"}},
		nil,
		SimpleTokenizer{},
	)

	assertContainsText(t, row.Code, "600519")
	assertContainsText(t, row.CodePrefix, "sh600519")
	assertContainsText(t, row.NameIndex, "贵州茅台")
	assertContainsText(t, row.FullNameIndex, "贵州茅台酒股份有限公司")
	assertContainsText(t, row.AliasIndex, "茅台")
	assertContainsText(t, row.PinyinFull, "guizhoumaotai")
	assertContainsText(t, row.PinyinInitials, "gzmt")
	assertContainsText(t, row.IndustryIndex, "白酒")
	assertContainsText(t, row.ConceptIndex, "消费")
}

// assertContainsText 校验索引字段包含目标片段。
func assertContainsText(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected %q to contain %q", text, want)
	}
}
