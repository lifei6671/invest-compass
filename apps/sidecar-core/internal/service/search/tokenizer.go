package search

import (
	"strings"
	"unicode"
)

// Tokenizer 定义搜索索引和查询构造共享的分词边界。
type Tokenizer interface {
	Tokenize(text string) []string
}

// SimpleTokenizer 是 GSE 不可用时的可预测降级分词器。
type SimpleTokenizer struct{}

// Tokenize 按中文连续片段、英文数字片段切词，并保持去重后的稳定顺序。
func (SimpleTokenizer) Tokenize(text string) []string {
	return uniqueTokens(splitSimpleTokens(text, true))
}

// splitSimpleTokens 执行轻量切词，keepColon 控制是否保留 symbol 里的冒号。
func splitSimpleTokens(text string, keepColon bool) []string {
	var tokens []string
	var current strings.Builder
	var currentKind rune

	flush := func() {
		if current.Len() == 0 {
			return
		}
		token := normalizeToken(current.String())
		if token != "" {
			tokens = append(tokens, token)
		}
		current.Reset()
		currentKind = 0
	}

	for _, r := range normalizeFullWidth(strings.TrimSpace(text)) {
		kind := simpleRuneKind(r, keepColon)
		if kind == 0 {
			flush()
			continue
		}
		if currentKind != 0 && currentKind != kind {
			flush()
		}
		currentKind = kind
		current.WriteRune(r)
	}
	flush()
	return tokens
}

// simpleRuneKind 返回降级分词使用的字符类别。
func simpleRuneKind(r rune, keepColon bool) rune {
	switch {
	case unicode.Is(unicode.Han, r):
		return 'h'
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 'a'
	case r == '_' || r == '.' || r == '-' || (keepColon && r == ':'):
		return 'a'
	default:
		return 0
	}
}

// normalizeToken 统一 token 形态，过滤纯符号片段。
func normalizeToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(normalizeFullWidth(token)))
	if token == "" {
		return ""
	}
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			return token
		}
	}
	return ""
}

// uniqueTokens 对 token 去重并保留首次出现顺序。
func uniqueTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = normalizeToken(token)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

// normalizeFullWidth 将常见全角 ASCII 转为半角，便于代码和拼音查询归一。
func normalizeFullWidth(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch {
		case r == 0x3000:
			builder.WriteRune(' ')
		case r >= 0xFF01 && r <= 0xFF5E:
			builder.WriteRune(r - 0xFEE0)
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
