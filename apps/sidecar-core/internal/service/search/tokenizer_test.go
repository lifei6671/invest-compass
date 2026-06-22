package search

import "testing"

// TestSimpleTokenizerKeepsStableUniqueTokens 验证降级分词器能稳定处理中文、英文、数字和空输入。
func TestSimpleTokenizerKeepsStableUniqueTokens(t *testing.T) {
	tokenizer := SimpleTokenizer{}

	got := tokenizer.Tokenize(" 贵州茅台 GZMT 600519 贵州茅台 ")
	want := []string{"贵州茅台", "gzmt", "600519"}
	requireTokens(t, got, want)

	empty := tokenizer.Tokenize("   ")
	if len(empty) != 0 {
		t.Fatalf("expected empty input to return no tokens, got %+v", empty)
	}
}

// TestGSETokenizerSupportsDomainWords 验证 GSE 搜索模式分词和动态投研词典能保留领域词。
func TestGSETokenizerSupportsDomainWords(t *testing.T) {
	tokenizer, err := NewGSETokenizer([]string{"贵州茅台", "光模块", "AI服务器", "CPO"})
	if err != nil {
		t.Fatalf("new gse tokenizer: %v", err)
	}

	tokens := tokenizer.Tokenize("贵州茅台 光模块 AI服务器 CPO")
	requireContainsToken(t, tokens, "贵州茅台")
	requireContainsToken(t, tokens, "光模块")
	requireContainsToken(t, tokens, "ai服务器")
	requireContainsToken(t, tokens, "cpo")
}

// requireTokens 校验 token 数组完全一致，避免分词去重顺序不稳定。
func requireTokens(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected token length: got %+v want %+v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected token at %d: got %+v want %+v", index, got, want)
		}
	}
}

// requireContainsToken 校验 token 数组包含目标词。
func requireContainsToken(t *testing.T, tokens []string, want string) {
	t.Helper()
	for _, token := range tokens {
		if token == want {
			return
		}
	}
	t.Fatalf("expected token %q in %+v", want, tokens)
}
