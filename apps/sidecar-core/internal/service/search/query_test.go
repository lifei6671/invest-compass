package search

import (
	"strings"
	"testing"
)

// TestNormalizeQueryInputClassifiesKeyword 验证查询归一化会处理大小写、全角字符和常见股票格式。
func TestNormalizeQueryInputClassifiesKeyword(t *testing.T) {
	cases := []struct {
		input string
		want  NormalizedQuery
	}{
		{input: "  ＳＨ600519  ", want: NormalizedQuery{Raw: "ＳＨ600519", Text: "sh600519", Kind: QueryKindExchangeCode}},
		{input: "CN:SH:600519", want: NormalizedQuery{Raw: "CN:SH:600519", Text: "cn:sh:600519", Kind: QueryKindSymbol}},
		{input: " GZMT ", want: NormalizedQuery{Raw: "GZMT", Text: "gzmt", Kind: QueryKindPinyinInitials}},
		{input: "贵州  茅台", want: NormalizedQuery{Raw: "贵州  茅台", Text: "贵州 茅台", Kind: QueryKindChinese}},
	}

	for _, item := range cases {
		got := NormalizeQueryInput(item.input)
		if got != item.want {
			t.Fatalf("normalize %q: got %+v want %+v", item.input, got, item.want)
		}
	}
}

// TestBuildFTSMatchEscapesUserSyntax 验证用户输入不能直接变成 FTS5 MATCH 语法。
func TestBuildFTSMatchEscapesUserSyntax(t *testing.T) {
	inputs := []string{
		"茅台 OR 1=1",
		"NEAR(茅台,10)",
		`symbol:CN:SH:600519`,
		`"贵州茅台"`,
	}

	for _, input := range inputs {
		match := BuildFTSMatch(NormalizeQueryInput(input), nil)
		for _, unsafe := range []string{" OR ", "NEAR", "=", "\"", "symbol:"} {
			if strings.Contains(match, unsafe) {
				t.Fatalf("match %q still contains unsafe syntax %q for input %q", match, unsafe, input)
			}
		}
		if strings.TrimSpace(match) == "" {
			t.Fatalf("expected non-empty safe match for %q", input)
		}
	}
}

// TestBuildFTSMatchCanScopeAllowedColumns 验证内部调用可以显式生成允许列的安全 MATCH。
func TestBuildFTSMatchCanScopeAllowedColumns(t *testing.T) {
	match := BuildFTSMatch(NormalizeQueryInput("茅台"), []string{"name_index", "alias_index"})
	if !strings.Contains(match, "name_index:") || !strings.Contains(match, "alias_index:") {
		t.Fatalf("expected allowed column scoped match, got %q", match)
	}
	if strings.Contains(match, "symbol:") {
		t.Fatalf("unexpected user controlled column in match: %q", match)
	}
}
