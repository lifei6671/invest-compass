package indicator

import (
	"errors"
	"math"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestMAAndEMA 验证均线和指数均线使用固定收盘价可重复计算。
func TestMAAndEMA(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}

	ma, err := MA(values, 3)
	if err != nil {
		t.Fatalf("MA returned error: %v", err)
	}
	assertSeries(t, ma, []float64{math.NaN(), math.NaN(), 2, 3, 4})

	ema, err := EMA(values, 3)
	if err != nil {
		t.Fatalf("EMA returned error: %v", err)
	}
	assertSeries(t, ema, []float64{1, 1.5, 2.25, 3.125, 4.0625})
}

// TestMACD 验证 MACD 返回 DIF、DEA 和柱状值三条序列。
func TestMACD(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}

	macd, err := MACD(values, 3, 5, 3)
	if err != nil {
		t.Fatalf("MACD returned error: %v", err)
	}

	assertSeries(t, macd.DIF, []float64{0, 0.1666666667, 0.3611111111, 0.5324074074, 0.6674382716})
	assertSeries(t, macd.DEA, []float64{0, 0.0833333333, 0.2222222222, 0.3773148148, 0.5223765432})
	assertSeries(t, macd.Bar, []float64{0, 0.1666666667, 0.2777777778, 0.3101851852, 0.2901234568})
}

// TestRSI 验证 RSI 对全上涨序列返回 100。
func TestRSI(t *testing.T) {
	rsi, err := RSI([]float64{1, 2, 3, 4}, 3)
	if err != nil {
		t.Fatalf("RSI returned error: %v", err)
	}

	assertSeries(t, rsi, []float64{math.NaN(), math.NaN(), math.NaN(), 100})
}

// TestKDJ 验证 KDJ 基于最高价、最低价和收盘价生成三条序列。
func TestKDJ(t *testing.T) {
	klines := []KLine{
		{High: 10, Low: 8, Close: 9},
		{High: 11, Low: 8, Close: 10},
		{High: 12, Low: 8, Close: 11},
	}

	kdj, err := KDJ(klines, 3)
	if err != nil {
		t.Fatalf("KDJ returned error: %v", err)
	}

	assertSeries(t, kdj.K, []float64{math.NaN(), math.NaN(), 58.3333333333})
	assertSeries(t, kdj.D, []float64{math.NaN(), math.NaN(), 52.7777777778})
	assertSeries(t, kdj.J, []float64{math.NaN(), math.NaN(), 69.4444444444})
}

// TestBOLL 验证布林线返回中轨、上轨和下轨。
func TestBOLL(t *testing.T) {
	boll, err := BOLL([]float64{1, 2, 3, 4, 5}, 3, 2)
	if err != nil {
		t.Fatalf("BOLL returned error: %v", err)
	}

	assertSeries(t, boll.Middle, []float64{math.NaN(), math.NaN(), 2, 3, 4})
	assertSeries(t, boll.Upper, []float64{math.NaN(), math.NaN(), 3.6329931619, 4.6329931619, 5.6329931619})
	assertSeries(t, boll.Lower, []float64{math.NaN(), math.NaN(), 0.3670068381, 1.3670068381, 2.3670068381})
}

// TestVolumeMAChangeDrawdownAndVolatility 验证成交量均线、涨跌幅、最大回撤和波动率。
func TestVolumeMAChangeDrawdownAndVolatility(t *testing.T) {
	volumeMA, err := VolumeMA([]float64{100, 200, 300}, 2)
	if err != nil {
		t.Fatalf("VolumeMA returned error: %v", err)
	}
	assertSeries(t, volumeMA, []float64{math.NaN(), 150, 250})

	change, err := ChangePercent(100, 110)
	if err != nil {
		t.Fatalf("ChangePercent returned error: %v", err)
	}
	assertClose(t, change, 10)

	drawdown, err := MaxDrawdown([]float64{100, 120, 90, 110})
	if err != nil {
		t.Fatalf("MaxDrawdown returned error: %v", err)
	}
	assertClose(t, drawdown, 25)

	volatility, err := Volatility([]float64{100, 110, 99})
	if err != nil {
		t.Fatalf("Volatility returned error: %v", err)
	}
	assertClose(t, volatility, 10)
}

// TestChipDistributionCalculatesCostProfile 验证筹码分布基于 K 线成交量生成可归一化的成本分布。
func TestChipDistributionCalculatesCostProfile(t *testing.T) {
	klines := []ChipKLine{
		{Open: 9.8, High: 10.2, Low: 9.6, Close: 10.0, Volume: 1000, Amount: 10000, TurnoverRate: 5},
		{Open: 10.1, High: 11.2, Low: 10.0, Close: 11.0, Volume: 3000, Amount: 31800, TurnoverRate: 8},
		{Open: 10.8, High: 12.0, Low: 10.5, Close: 11.5, Volume: 2000, Amount: 22600, TurnoverRate: 6},
	}

	distribution, err := ChipDistribution(klines, ChipDistributionOptions{BinCount: 12})
	if err != nil {
		t.Fatalf("ChipDistribution returned error: %v", err)
	}

	if distribution.SampleSize != len(klines) {
		t.Fatalf("expected sample size %d, got %d", len(klines), distribution.SampleSize)
	}
	if distribution.BinCount != 12 || len(distribution.Items) != 12 {
		t.Fatalf("expected 12 bins, got count=%d items=%d", distribution.BinCount, len(distribution.Items))
	}
	assertClose(t, distribution.CurrentPrice, 11.5)
	if distribution.AverageCost < distribution.MinPrice || distribution.AverageCost > distribution.MaxPrice {
		t.Fatalf("average cost %.4f out of price range %.4f-%.4f", distribution.AverageCost, distribution.MinPrice, distribution.MaxPrice)
	}
	if distribution.ProfitRatio <= 0 || distribution.ProfitRatio > 1 {
		t.Fatalf("unexpected profit ratio %.6f", distribution.ProfitRatio)
	}

	var ratioSum float64
	for _, item := range distribution.Items {
		ratioSum += item.Ratio
	}
	assertClose(t, ratioSum, 1)

	top := distribution.TopN(2)
	if len(top) != 2 {
		t.Fatalf("expected two top bins, got %d", len(top))
	}
	if top[0].Ratio < top[1].Ratio {
		t.Fatalf("top bins are not sorted by ratio desc: %#v", top)
	}
}

// TestChipDistributionUsesLatestValidPrice 验证尾部占位坏数据不会把当前价和获利比例置零。
func TestChipDistributionUsesLatestValidPrice(t *testing.T) {
	klines := []ChipKLine{
		{Open: 9.8, High: 10.2, Low: 9.6, Close: 10.0, Volume: 1000},
		{Open: 10.1, High: 11.2, Low: 10.0, Close: 11.0, Volume: 2000},
		{Open: 0, High: 0, Low: 0, Close: 0, Volume: 0},
	}

	distribution, err := ChipDistribution(klines, ChipDistributionOptions{BinCount: 10})
	if err != nil {
		t.Fatalf("ChipDistribution returned error: %v", err)
	}

	assertClose(t, distribution.CurrentPrice, 11.0)
	if distribution.ProfitRatio <= 0 {
		t.Fatalf("expected positive profit ratio, got %.6f", distribution.ProfitRatio)
	}
}

// TestChipCostCenterUsesOHLCWhenAmountMissing 验证缺少成交额时成本中枢仍使用完整 OHLC，而不是忽略开盘价。
func TestChipCostCenterUsesOHLCWhenAmountMissing(t *testing.T) {
	kline := ChipKLine{Open: 9, High: 12, Low: 8, Close: 11, Volume: 1000}

	center := chipCostCenter(kline, 8, 12)

	assertClose(t, center, 10)
}

// TestChipDistributionRejectsInvalidInput 验证筹码分布对非法输入快速失败，不生成误导性图表数据。
func TestChipDistributionRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		code xerr.Code
	}{
		{
			name: "empty klines",
			run: func() error {
				_, err := ChipDistribution(nil, ChipDistributionOptions{})
				return err
			},
			code: xerr.IndicatorInsufficientData,
		},
		{
			name: "invalid bin count",
			run: func() error {
				_, err := ChipDistribution([]ChipKLine{{High: 10, Low: 9, Close: 9.5, Volume: 100}}, ChipDistributionOptions{BinCount: -1})
				return err
			},
			code: xerr.IndicatorInvalidInput,
		},
		{
			name: "invalid price range",
			run: func() error {
				_, err := ChipDistribution([]ChipKLine{{High: 0, Low: 0, Close: 0, Volume: 100}}, ChipDistributionOptions{})
				return err
			},
			code: xerr.IndicatorInvalidInput,
		},
		{
			name: "mixed invalid kline with volume",
			run: func() error {
				_, err := ChipDistribution([]ChipKLine{
					{High: 10, Low: 9, Close: 9.5, Volume: 100},
					{High: 0, Low: 9, Close: 9.5, Volume: 100},
				}, ChipDistributionOptions{})
				return err
			},
			code: xerr.IndicatorInvalidInput,
		},
		{
			name: "high less than low",
			run: func() error {
				_, err := ChipDistribution([]ChipKLine{{High: 8, Low: 9, Close: 8.5, Volume: 100}}, ChipDistributionOptions{})
				return err
			},
			code: xerr.IndicatorInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected error")
			}

			var indicatorError *xerr.Error
			if !errors.As(err, &indicatorError) {
				t.Fatalf("expected xerr.Error, got %T", err)
			}
			if indicatorError.Code != tt.code {
				t.Fatalf("expected error code %q, got %q", tt.code, indicatorError.Code)
			}
		})
	}
}

// TestIndicatorsReturnStableErrors 验证数据不足或参数非法时返回稳定错误码。
func TestIndicatorsReturnStableErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		code xerr.Code
	}{
		{name: "invalid period", run: func() error { _, err := MA([]float64{1}, 0); return err }, code: xerr.IndicatorInvalidPeriod},
		{name: "not enough data", run: func() error { _, err := BOLL([]float64{1}, 2, 2); return err }, code: xerr.IndicatorInsufficientData},
		{name: "invalid previous close", run: func() error { _, err := ChangePercent(0, 1); return err }, code: xerr.IndicatorInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected error")
			}

			var indicatorError *xerr.Error
			if !errors.As(err, &indicatorError) {
				t.Fatalf("expected xerr.Error, got %T", err)
			}
			if indicatorError.Code != tt.code {
				t.Fatalf("expected error code %q, got %q", tt.code, indicatorError.Code)
			}
		})
	}
}

// assertSeries 校验浮点序列，支持 NaN 作为数据不足占位。
func assertSeries(t *testing.T, got []float64, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected length %d, got %d: %v", len(want), len(got), got)
	}
	for index := range want {
		if math.IsNaN(want[index]) {
			if !math.IsNaN(got[index]) {
				t.Fatalf("index %d expected NaN, got %.10f", index, got[index])
			}
			continue
		}
		assertClose(t, got[index], want[index])
	}
}

// assertClose 校验浮点结果允许极小舍入误差。
func assertClose(t *testing.T, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0000001 {
		t.Fatalf("expected %.10f, got %.10f", want, got)
	}
}
