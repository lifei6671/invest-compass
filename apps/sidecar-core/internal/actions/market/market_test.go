package market

import (
	"net/http/httptest"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
)

// TestParsePeriodAcceptsKlinePeriods 验证行情 API 明确接收全屏 K 线使用的所有周期。
func TestParsePeriodAcceptsKlinePeriods(t *testing.T) {
	tests := []struct {
		raw      string
		expected marketservice.Period
	}{
		{raw: "minute", expected: marketservice.PeriodMinute},
		{raw: "1m", expected: marketservice.Period1Minute},
		{raw: "5m", expected: marketservice.Period5Minute},
		{raw: "15m", expected: marketservice.Period15Minute},
		{raw: "30m", expected: marketservice.Period30Minute},
		{raw: "60m", expected: marketservice.Period60Minute},
		{raw: "day", expected: marketservice.PeriodDay},
		{raw: "week", expected: marketservice.PeriodWeek},
		{raw: "month", expected: marketservice.PeriodMonth},
		{raw: "quarter", expected: marketservice.PeriodQuarter},
		{raw: "year", expected: marketservice.PeriodYear},
	}

	for _, test := range tests {
		recorder := httptest.NewRecorder()
		period, ok := parsePeriod(recorder, test.raw, httpx.RequestContext{})
		if !ok {
			t.Fatalf("parsePeriod(%q) rejected minute kline period", test.raw)
		}
		if period != test.expected {
			t.Fatalf("parsePeriod(%q)=%q, expected %q", test.raw, period, test.expected)
		}
	}
}

// TestShouldNotCacheIntradayKlines 验证分钟级 K 线不写入本地持久缓存，避免盘中数据被隔日复用。
func TestShouldNotCacheIntradayKlines(t *testing.T) {
	intradayPeriods := []marketservice.Period{
		marketservice.PeriodMinute,
		marketservice.Period1Minute,
		marketservice.Period5Minute,
		marketservice.Period15Minute,
		marketservice.Period30Minute,
		marketservice.Period60Minute,
	}

	for _, period := range intradayPeriods {
		if shouldCacheKline(period) {
			t.Fatalf("expected intraday period %q to bypass persistent cache", period)
		}
	}
}
