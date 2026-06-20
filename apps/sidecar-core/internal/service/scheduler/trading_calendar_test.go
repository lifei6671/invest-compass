package scheduler

import (
	"testing"
	"time"
)

// TestTradingCalendarClassifiesCNTradingWindows 验证 A 股交易日内的交易时段、午休和收盘后分类。
func TestTradingCalendarClassifiesCNTradingWindows(t *testing.T) {
	calendar, err := NewTradingCalendar("CN", "Asia/Shanghai")
	if err != nil {
		t.Fatalf("new CN trading calendar: %v", err)
	}
	location := mustShanghaiLocation(t)

	cases := []struct {
		name string
		now  time.Time
		want TradingPhase
	}{
		{name: "before open", now: time.Date(2026, 6, 19, 9, 0, 0, 0, location), want: PhaseBeforeOpen},
		{name: "morning trading", now: time.Date(2026, 6, 19, 10, 0, 0, 0, location), want: PhaseTradingTime},
		{name: "lunch break", now: time.Date(2026, 6, 19, 12, 0, 0, 0, location), want: PhaseLunchBreak},
		{name: "afternoon trading", now: time.Date(2026, 6, 19, 14, 30, 0, 0, location), want: PhaseTradingTime},
		{name: "after close", now: time.Date(2026, 6, 19, 15, 30, 0, 0, location), want: PhaseAfterClose},
		{name: "weekend", now: time.Date(2026, 6, 20, 10, 0, 0, 0, location), want: PhaseNonTradingDay},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := calendar.PhaseAt(item.now); got != item.want {
				t.Fatalf("unexpected phase: got %s want %s", got, item.want)
			}
		})
	}
}

// TestTradingCalendarAllowsJobWindow 验证不同任务窗口只在对应交易阶段执行。
func TestTradingCalendarAllowsJobWindow(t *testing.T) {
	calendar, err := NewTradingCalendar("CN", "Asia/Shanghai")
	if err != nil {
		t.Fatalf("new CN trading calendar: %v", err)
	}
	location := mustShanghaiLocation(t)

	if !calendar.AllowsWindow(time.Date(2026, 6, 19, 10, 0, 0, 0, location), TradeWindowTradingTime) {
		t.Fatal("trading_time window should allow morning trading time")
	}
	if calendar.AllowsWindow(time.Date(2026, 6, 19, 12, 0, 0, 0, location), TradeWindowTradingTime) {
		t.Fatal("trading_time window should reject lunch break")
	}
	if !calendar.AllowsWindow(time.Date(2026, 6, 19, 15, 30, 0, 0, location), TradeWindowAfterClose) {
		t.Fatal("after_close window should allow after close")
	}
	if calendar.AllowsWindow(time.Date(2026, 6, 20, 15, 30, 0, 0, location), TradeWindowAnyTime) {
		t.Fatal("any_time window should still reject non-trading days")
	}
}

// TestTradingCalendarDetectsMissedTodayOpenJob 验证 09:30 任务在 10:00 启动时会被识别为当天错过窗口。
func TestTradingCalendarDetectsMissedTodayOpenJob(t *testing.T) {
	calendar, err := NewTradingCalendar("CN", "Asia/Shanghai")
	if err != nil {
		t.Fatalf("new CN trading calendar: %v", err)
	}
	location := mustShanghaiLocation(t)

	missed, scheduledAt, err := calendar.MissedToday("30 9 * * 1-5", time.Date(2026, 6, 19, 10, 0, 0, 0, location))
	if err != nil {
		t.Fatalf("detect missed today: %v", err)
	}
	if !missed || scheduledAt.Format("15:04") != "09:30" {
		t.Fatalf("expected 09:30 to be missed by 10:00, missed=%v scheduledAt=%s", missed, scheduledAt)
	}

	missed, _, err = calendar.MissedToday("30 9 * * 1-5", time.Date(2026, 6, 19, 9, 0, 0, 0, location))
	if err != nil {
		t.Fatalf("detect missed today before open: %v", err)
	}
	if missed {
		t.Fatal("09:30 task must not be missed before 09:30")
	}

	missed, _, err = calendar.MissedToday("30 9 * * 1-5", time.Date(2026, 6, 20, 10, 0, 0, 0, location))
	if err != nil {
		t.Fatalf("detect missed today weekend: %v", err)
	}
	if missed {
		t.Fatal("weekend must not generate missed_today")
	}
}

// TestTradingCalendarRejectsUnsupportedMarket 验证首批只开放 CN A 股调度。
func TestTradingCalendarRejectsUnsupportedMarket(t *testing.T) {
	if _, err := NewTradingCalendar("HK", "Asia/Shanghai"); err == nil {
		t.Fatal("expected HK scheduler calendar to be unsupported")
	}
}
