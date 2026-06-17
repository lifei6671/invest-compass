package stock

import (
	"errors"
	"testing"
)

// TestParseSymbolAcceptsSupportedMarkets 验证首版支持的 A 股、港股和美股代码格式。
func TestParseSymbolAcceptsSupportedMarkets(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		canonical string
		market    string
		exchange  string
		code      string
	}{
		{name: "shanghai a share", raw: "CN:SH:600519", canonical: "CN:SH:600519", market: "CN", exchange: "SH", code: "600519"},
		{name: "shenzhen a share normalizes case", raw: "cn:sz:300750", canonical: "CN:SZ:300750", market: "CN", exchange: "SZ", code: "300750"},
		{name: "hong kong stock", raw: "HK:00700", canonical: "HK:00700", market: "HK", exchange: "", code: "00700"},
		{name: "us stock normalizes case", raw: "us:aapl", canonical: "US:AAPL", market: "US", exchange: "", code: "AAPL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbol, err := ParseSymbol(tt.raw)
			if err != nil {
				t.Fatalf("ParseSymbol returned error: %v", err)
			}
			if symbol.Canonical != tt.canonical || symbol.Market != tt.market || symbol.Exchange != tt.exchange || symbol.Code != tt.code {
				t.Fatalf("unexpected symbol: %+v", symbol)
			}
			if symbol.String() != tt.canonical {
				t.Fatalf("expected String() %q, got %q", tt.canonical, symbol.String())
			}
		})
	}
}

// TestParseSymbolRejectsInvalidInputs 验证非法股票代码返回稳定错误码。
func TestParseSymbolRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		code ErrorCode
	}{
		{name: "empty", raw: "", code: ErrorEmptySymbol},
		{name: "unknown market", raw: "EU:SAP", code: ErrorUnsupportedMarket},
		{name: "cn missing exchange", raw: "CN:600519", code: ErrorInvalidFormat},
		{name: "cn unknown exchange", raw: "CN:BJ:430047", code: ErrorUnsupportedExchange},
		{name: "cn code must be six digits", raw: "CN:SH:60051A", code: ErrorInvalidCode},
		{name: "hk code must be five digits", raw: "HK:700", code: ErrorInvalidCode},
		{name: "us code must be simple ticker", raw: "US:AA PL", code: ErrorInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSymbol(tt.raw)
			if err == nil {
				t.Fatal("expected error")
			}

			var symbolError *SymbolError
			if !errors.As(err, &symbolError) {
				t.Fatalf("expected SymbolError, got %T", err)
			}
			if symbolError.Code != tt.code {
				t.Fatalf("expected error code %q, got %q", tt.code, symbolError.Code)
			}
		})
	}
}
