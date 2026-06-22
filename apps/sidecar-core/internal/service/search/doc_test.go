package search

import "testing"

// TestPackageBoundaryExists 验证 search service 包具备可测试边界。
func TestPackageBoundaryExists(t *testing.T) {
	var _ Tokenizer = SimpleTokenizer{}
}
