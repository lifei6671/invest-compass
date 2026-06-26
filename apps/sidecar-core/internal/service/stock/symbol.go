package stock

import (
	"regexp"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
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

// ParseSymbol 解析并标准化首版支持的股票代码格式。
func ParseSymbol(raw string) (Symbol, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return Symbol{}, newSymbolError(xerr.StockEmptySymbol, raw)
	}
	if !strings.Contains(normalized, ":") && strings.Contains(normalized, ".") {
		return parseCNDotSymbol(normalized)
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
		return Symbol{}, newSymbolError(xerr.StockUnsupportedMarket, normalized)
	}
}

// parseCNDotSymbol 兼容基础股票库和前端展示使用的 600000.SH / 399001.SZ 格式。
func parseCNDotSymbol(normalized string) (Symbol, error) {
	parts := strings.Split(normalized, ".")
	if len(parts) != 2 {
		return Symbol{}, newSymbolError(xerr.StockInvalidSymbolFormat, normalized)
	}
	return parseCNSymbol([]string{"CN", parts[1], parts[0]}, normalized)
}

// parseCNSymbol 解析 A 股代码，要求包含市场、交易所和六位数字代码。
func parseCNSymbol(parts []string, normalized string) (Symbol, error) {
	if len(parts) != 3 {
		return Symbol{}, newSymbolError(xerr.StockInvalidSymbolFormat, normalized)
	}
	if parts[1] != "SH" && parts[1] != "SZ" {
		return Symbol{}, newSymbolError(xerr.StockUnsupportedExchange, normalized)
	}
	if !cnCodePattern.MatchString(parts[2]) {
		return Symbol{}, newSymbolError(xerr.StockInvalidSymbolCode, normalized)
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
		return Symbol{}, newSymbolError(xerr.StockInvalidSymbolFormat, normalized)
	}
	if !codePattern.MatchString(parts[1]) {
		return Symbol{}, newSymbolError(xerr.StockInvalidSymbolCode, normalized)
	}

	return Symbol{
		Canonical: market + ":" + parts[1],
		Market:    market,
		Exchange:  "",
		Code:      parts[1],
	}, nil
}

// newSymbolError 创建稳定错误码错误，集中控制错误类型。
func newSymbolError(code xerr.Code, value string) error {
	return &xerr.Error{Code: code, Message: value}
}
