package market

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// TestTdxProviderLiveMinuteKline 通过环境变量开启真实通达信分钟 K 线烟测，默认跳过避免 CI 依赖外网行情。
func TestTdxProviderLiveMinuteKline(t *testing.T) {
	if os.Getenv("INVEST_COMPASS_TDX_LIVE") != "1" {
		t.Skip("set INVEST_COMPASS_TDX_LIVE=1 to run live TDX smoke test")
	}
	symbol, err := stock.ParseSymbol("600000.SH")
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	provider, err := NewTdxProvider(TdxConfig{Timeout: 8 * time.Second})
	if err != nil {
		t.Fatalf("NewTdxProvider returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	bars, err := provider.Kline(ctx, KlineRequest{
		Symbol: symbol,
		Period: Period5Minute,
		Adjust: AdjustForward,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("Kline returned error: %v", err)
	}
	if len(bars) == 0 {
		t.Fatal("expected live TDX minute kline bars")
	}
	last := bars[len(bars)-1]
	if last.Close <= 0 || last.High <= 0 || last.Low <= 0 || last.Open <= 0 {
		t.Fatalf("unexpected live TDX bar prices: %+v", last)
	}
	if last.Period != Period5Minute || last.Symbol.String() != "CN:SH:600000" {
		t.Fatalf("unexpected live TDX metadata: %+v", last)
	}
}
