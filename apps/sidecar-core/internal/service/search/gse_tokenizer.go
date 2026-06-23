package search

import (
	"fmt"

	"github.com/go-ego/gse"
)

// GSETokenizer 使用 GSE 搜索引擎模式进行中文分词。
type GSETokenizer struct {
	segmenter gse.Segmenter
}

// NewGSETokenizer 创建 GSE 分词器，并把投研领域词加入动态词典。
func NewGSETokenizer(domainWords []string) (*GSETokenizer, error) {
	segmenter, err := gse.New("zh_s")
	if err != nil {
		return nil, fmt.Errorf("init gse tokenizer: %w", err)
	}
	tokenizer := &GSETokenizer{segmenter: segmenter}
	if err := tokenizer.AddWords(domainWords); err != nil {
		return nil, err
	}
	return tokenizer, nil
}

// AddWords 将股票名称、别名、行业概念等领域词加入 GSE 词典。
func (tokenizer *GSETokenizer) AddWords(words []string) error {
	for _, word := range words {
		token := normalizeToken(word)
		if token == "" {
			continue
		}
		if err := tokenizer.segmenter.AddTokenForce(token, 100000, "n"); err != nil {
			return fmt.Errorf("add gse token %q: %w", token, err)
		}
	}
	return nil
}

// Tokenize 使用 GSE CutSearch 生成搜索模式 token，并保持稳定去重。
func (tokenizer *GSETokenizer) Tokenize(text string) []string {
	if tokenizer == nil {
		return nil
	}
	return uniqueTokens(tokenizer.segmenter.CutSearch(normalizeFullWidth(text), true))
}

// Metadata 返回 GSE 分词器的稳定元数据。
func (tokenizer *GSETokenizer) Metadata() TokenizerMetadata {
	return TokenizerMetadata{Name: "gse", Version: "1", DictionaryHash: "zh_s+builtin-domain"}
}
