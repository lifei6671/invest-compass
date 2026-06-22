package search

import (
	"regexp"
	"strings"
	"unicode"
)

// QueryKind 表示归一化查询词的主要形态，供股票强规则排序使用。
type QueryKind string

const (
	// QueryKindEmpty 表示空查询。
	QueryKindEmpty QueryKind = "empty"
	// QueryKindCode 表示 6 位股票代码。
	QueryKindCode QueryKind = "code"
	// QueryKindExchangeCode 表示 sh/sz/bj + 6 位代码。
	QueryKindExchangeCode QueryKind = "exchange_code"
	// QueryKindSymbol 表示内部标准 symbol，例如 cn:sh:600519。
	QueryKindSymbol QueryKind = "symbol"
	// QueryKindPinyin 表示较长拼音全拼。
	QueryKindPinyin QueryKind = "pinyin"
	// QueryKindPinyinInitials 表示拼音首字母。
	QueryKindPinyinInitials QueryKind = "pinyin_initials"
	// QueryKindChinese 表示中文查询。
	QueryKindChinese QueryKind = "chinese"
	// QueryKindMixed 表示混合查询。
	QueryKindMixed QueryKind = "mixed"
)

var (
	codePattern         = regexp.MustCompile(`^\d{6}$`)
	exchangeCodePattern = regexp.MustCompile(`^(sh|sz|bj)\d{6}$`)
	symbolPattern       = regexp.MustCompile(`^[a-z]{2}:[a-z]{2}:\d{6}$`)
	alphaPattern        = regexp.MustCompile(`^[a-z]+$`)
	columnPattern       = regexp.MustCompile(`^[a-z_]+$`)
)

// NormalizedQuery 是用户搜索输入经过安全归一化后的结果。
type NormalizedQuery struct {
	Raw  string
	Text string
	Kind QueryKind
}

// NormalizeQueryInput 归一化用户输入，并识别常见股票搜索形态。
func NormalizeQueryInput(input string) NormalizedQuery {
	raw := strings.TrimSpace(input)
	text := strings.ToLower(collapseSpaces(normalizeFullWidth(raw)))
	query := NormalizedQuery{Raw: raw, Text: text, Kind: classifyQuery(text)}
	return query
}

// BuildFTSMatch 从归一化查询构造受控 MATCH 表达式，不透传用户原始 FTS 语法。
func BuildFTSMatch(query NormalizedQuery, allowedColumns []string) string {
	tokens := safeMatchTokens(query.Text)
	if len(tokens) == 0 {
		return ""
	}

	columns := safeColumns(allowedColumns)
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(columns) == 0 {
			parts = append(parts, token+"*")
			continue
		}
		columnParts := make([]string, 0, len(columns))
		for _, column := range columns {
			columnParts = append(columnParts, column+":"+token+"*")
		}
		parts = append(parts, "("+strings.Join(columnParts, " OR ")+")")
	}
	return strings.Join(parts, " AND ")
}

// classifyQuery 判断归一化查询词的主要形态。
func classifyQuery(text string) QueryKind {
	switch {
	case text == "":
		return QueryKindEmpty
	case symbolPattern.MatchString(text):
		return QueryKindSymbol
	case exchangeCodePattern.MatchString(text):
		return QueryKindExchangeCode
	case codePattern.MatchString(text):
		return QueryKindCode
	case isChineseOnly(text):
		return QueryKindChinese
	case alphaPattern.MatchString(text) && len(text) <= 6:
		return QueryKindPinyinInitials
	case alphaPattern.MatchString(text):
		return QueryKindPinyin
	default:
		return QueryKindMixed
	}
}

// collapseSpaces 合并连续空白。
func collapseSpaces(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// isChineseOnly 判断文本是否只包含中文和空格。
func isChineseOnly(text string) bool {
	hasHan := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if !unicode.Is(unicode.Han, r) {
			return false
		}
		hasHan = true
	}
	return hasHan
}

// safeMatchTokens 把用户输入降级为普通 token，剔除 FTS 操作符和列语法。
func safeMatchTokens(text string) []string {
	rawTokens := splitSimpleTokens(text, false)
	result := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		switch token {
		case "and", "or", "not", "near":
			continue
		}
		result = append(result, token)
	}
	return uniqueTokens(result)
}

// safeColumns 只接受内部白名单列名格式，用户输入不能参与列名生成。
func safeColumns(columns []string) []string {
	result := make([]string, 0, len(columns))
	seen := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		column = strings.ToLower(strings.TrimSpace(column))
		if !columnPattern.MatchString(column) {
			continue
		}
		if _, ok := seen[column]; ok {
			continue
		}
		seen[column] = struct{}{}
		result = append(result, column)
	}
	return result
}
