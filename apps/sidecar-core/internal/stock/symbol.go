package stock

import (
	"regexp"
	"strings"
)

// ErrorCode 是股票代码解析失败时对外稳定的错误码。
type ErrorCode string

const (
	// ErrorEmptySymbol 表示输入为空或只包含空白字符。
	ErrorEmptySymbol ErrorCode = "empty_symbol"
	// ErrorInvalidFormat 表示股票代码分段数量不符合目标市场格式。
	ErrorInvalidFormat ErrorCode = "invalid_symbol_format"
	// ErrorUnsupportedMarket 表示首版暂不支持该市场。
	ErrorUnsupportedMarket ErrorCode = "unsupported_market"
	// ErrorUnsupportedExchange 表示 A 股市场下的交易所代码不受支持。
	ErrorUnsupportedExchange ErrorCode = "unsupported_exchange"
	// ErrorInvalidCode 表示股票代码主体不符合目标市场规则。
	ErrorInvalidCode ErrorCode = "invalid_symbol_code"
)

var (
	cnCodePattern = regexp.MustCompile(`^[0-9]{6}$`)
	hkCodePattern = regexp.MustCompile(`^[0-9]{5}$`)
	usCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9.-]{0,9}$`)
)

// Symbol 是所有行情、新闻、分析任务共用的规范化股票代码。
type Symbol struct {
	Canonical string
	Market    string
	Exchange  string
	Code      string
}

// String 返回可持久化和跨 API 传递的规范化股票代码。
func (symbol Symbol) String() string {
	return symbol.Canonical
}

// SymbolError 表示股票代码解析失败，并携带稳定错误码供 API 层映射。
type SymbolError struct {
	Code  ErrorCode
	Value string
}

// Error 返回不包含敏感信息的股票代码解析错误描述。
func (err *SymbolError) Error() string {
	return string(err.Code) + ": " + err.Value
}

// ParseSymbol 解析并标准化首版支持的股票代码格式。
func ParseSymbol(raw string) (Symbol, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return Symbol{}, newSymbolError(ErrorEmptySymbol, raw)
	}

	parts := strings.Split(normalized, ":")
	switch parts[0] {
	case "CN":
		return parseCNSymbol(parts, normalized)
	case "HK":
		return parseSimpleMarketSymbol(parts, normalized, "HK", hkCodePattern)
	case "US":
		return parseSimpleMarketSymbol(parts, normalized, "US", usCodePattern)
	default:
		return Symbol{}, newSymbolError(ErrorUnsupportedMarket, normalized)
	}
}

// parseCNSymbol 解析 A 股代码，要求包含市场、交易所和六位数字代码。
func parseCNSymbol(parts []string, normalized string) (Symbol, error) {
	if len(parts) != 3 {
		return Symbol{}, newSymbolError(ErrorInvalidFormat, normalized)
	}
	if parts[1] != "SH" && parts[1] != "SZ" {
		return Symbol{}, newSymbolError(ErrorUnsupportedExchange, normalized)
	}
	if !cnCodePattern.MatchString(parts[2]) {
		return Symbol{}, newSymbolError(ErrorInvalidCode, normalized)
	}

	return Symbol{
		Canonical: strings.Join(parts, ":"),
		Market:    "CN",
		Exchange:  parts[1],
		Code:      parts[2],
	}, nil
}

// parseSimpleMarketSymbol 解析不需要交易所分段的市场代码。
func parseSimpleMarketSymbol(parts []string, normalized string, market string, codePattern *regexp.Regexp) (Symbol, error) {
	if len(parts) != 2 {
		return Symbol{}, newSymbolError(ErrorInvalidFormat, normalized)
	}
	if !codePattern.MatchString(parts[1]) {
		return Symbol{}, newSymbolError(ErrorInvalidCode, normalized)
	}

	return Symbol{
		Canonical: market + ":" + parts[1],
		Market:    market,
		Exchange:  "",
		Code:      parts[1],
	}, nil
}

// newSymbolError 创建稳定错误码错误，集中控制错误类型。
func newSymbolError(code ErrorCode, value string) error {
	return &SymbolError{Code: code, Value: value}
}
