package search

import (
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// BuildStockSearchFTSRow 从股票基础信息和别名构造可写入 FTS 的索引文本。
func BuildStockSearchFTSRow(stock model.Stock, aliases []model.StockAlias, overrides map[string]PinyinOverride, tokenizer Tokenizer) dao.StockSearchFTSRow {
	if tokenizer == nil {
		tokenizer = SimpleTokenizer{}
	}
	namePinyin := BuildPinyinText(stock.Name, overrides)
	fullNamePinyin := BuildPinyinText(stock.FullName, overrides)
	aliasTexts := aliasTextList(aliases)
	aliasPinyin := buildJoinedPinyin(aliasTexts, overrides)

	pinyinFull := joinIndexText(stock.PinyinFull, namePinyin.Full, fullNamePinyin.Full, aliasPinyin.Full)
	pinyinInitials := joinIndexText(stock.PinyinInitials, namePinyin.Initials, fullNamePinyin.Initials, aliasPinyin.Initials)

	return dao.StockSearchFTSRow{
		Symbol:         stock.Symbol,
		Market:         stock.Market,
		Exchange:       stock.Exchange,
		Code:           stock.Code,
		CodePrefix:     buildCodeIndex(stock.Exchange, stock.Code),
		NameIndex:      buildTextIndex(tokenizer, stock.Name, stock.SearchName),
		FullNameIndex:  buildTextIndex(tokenizer, stock.FullName),
		AliasIndex:     buildTextIndex(tokenizer, aliasTexts...),
		PinyinFull:     pinyinFull,
		PinyinInitials: pinyinInitials,
		IndustryIndex:  buildTextIndex(tokenizer, stock.Industry),
		ConceptIndex:   buildTextIndex(tokenizer, stock.Concept),
	}
}

// aliasTextList 提取未软删除别名的原词。
func aliasTextList(aliases []model.StockAlias) []string {
	result := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if alias.DeletedAt.Valid {
			continue
		}
		if strings.TrimSpace(alias.Alias) != "" {
			result = append(result, alias.Alias)
		}
	}
	return result
}

// buildJoinedPinyin 为多个文本合并全拼和首字母。
func buildJoinedPinyin(texts []string, overrides map[string]PinyinOverride) PinyinText {
	var full []string
	var initials []string
	for _, text := range texts {
		pinyin := BuildPinyinText(text, overrides)
		full = append(full, pinyin.Full)
		initials = append(initials, pinyin.Initials)
	}
	return PinyinText{
		Full:     joinIndexText(full...),
		Initials: joinIndexText(initials...),
	}
}

// buildCodeIndex 生成代码、代码前缀和交易所代码形式，支持 sh600519 这类搜索。
func buildCodeIndex(exchange string, code string) string {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	code = strings.TrimSpace(code)
	var parts []string
	if code != "" {
		parts = append(parts, code)
		for index := 1; index <= len(code) && index <= 6; index++ {
			parts = append(parts, code[:index])
		}
	}
	if exchange != "" && code != "" {
		parts = append(parts, exchange+code)
	}
	return joinIndexText(parts...)
}

// buildTextIndex 保留原词，同时追加 tokenizer 生成的 token。
func buildTextIndex(tokenizer Tokenizer, texts ...string) string {
	var parts []string
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
		parts = append(parts, tokenizer.Tokenize(text)...)
	}
	return joinIndexText(parts...)
}

// joinIndexText 合并索引片段并稳定去重。
func joinIndexText(parts ...string) string {
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		for _, token := range strings.Fields(part) {
			if normalized := normalizeToken(token); normalized != "" {
				tokens = append(tokens, normalized)
			}
		}
	}
	return strings.Join(uniqueTokens(tokens), " ")
}
