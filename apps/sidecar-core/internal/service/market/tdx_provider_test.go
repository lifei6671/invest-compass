package market

import "testing"

// TestTdxKLineTypeFromPeriodSupportsChartPeriods 验证内部 K 线周期能稳定映射到通达信 K 线类型。
func TestTdxKLineTypeFromPeriodSupportsChartPeriods(t *testing.T) {
	tests := []struct {
		period   Period
		expected uint16
	}{
		{period: Period1Minute, expected: 8},
		{period: Period5Minute, expected: 0},
		{period: Period15Minute, expected: 1},
		{period: Period30Minute, expected: 2},
		{period: Period60Minute, expected: 3},
		{period: PeriodDay, expected: 4},
		{period: PeriodWeek, expected: 5},
		{period: PeriodMonth, expected: 6},
		{period: PeriodQuarter, expected: 10},
		{period: PeriodYear, expected: 11},
	}

	for _, test := range tests {
		actual, err := tdxKLineTypeFromPeriod(test.period)
		if err != nil {
			t.Fatalf("tdxKLineTypeFromPeriod(%q) returned error: %v", test.period, err)
		}
		if actual != test.expected {
			t.Fatalf("tdxKLineTypeFromPeriod(%q)=%d, expected %d", test.period, actual, test.expected)
		}
	}
}
