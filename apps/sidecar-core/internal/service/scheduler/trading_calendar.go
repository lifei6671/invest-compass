package scheduler

import (
	"fmt"
	"strings"
	"time"
)

const (
	// TradeWindowTradingTime 表示只允许在 A 股连续竞价交易时段执行。
	TradeWindowTradingTime = "trading_time"
	// TradeWindowAfterClose 表示只允许在 A 股收盘后执行。
	TradeWindowAfterClose = "after_close"
	// TradeWindowAnyTime 表示交易日内任意阶段均可执行。
	TradeWindowAnyTime = "any_time"
)

// TradingPhase 表示 A 股交易日内的阶段分类。
type TradingPhase string

const (
	// PhaseNonTradingDay 表示非交易日，首批按周末过滤。
	PhaseNonTradingDay TradingPhase = "non_trading_day"
	// PhaseBeforeOpen 表示开盘前。
	PhaseBeforeOpen TradingPhase = "before_open"
	// PhaseTradingTime 表示连续竞价交易时段。
	PhaseTradingTime TradingPhase = "trading_time"
	// PhaseLunchBreak 表示午间休市。
	PhaseLunchBreak TradingPhase = "lunch_break"
	// PhaseAfterClose 表示收盘后。
	PhaseAfterClose TradingPhase = "after_close"
)

// TradingCalendar 封装首批支持的交易日和交易时段判断。
type TradingCalendar struct {
	market   string
	location *time.Location
}

// NewTradingCalendar 创建交易日历，首批只支持 CN A 股。
func NewTradingCalendar(market string, timezone string) (TradingCalendar, error) {
	normalizedMarket := strings.ToUpper(strings.TrimSpace(market))
	if normalizedMarket == "" {
		normalizedMarket = "CN"
	}
	if normalizedMarket != "CN" {
		return TradingCalendar{}, fmt.Errorf("scheduler market %s is unsupported", normalizedMarket)
	}
	trimmedTimezone := strings.TrimSpace(timezone)
	if trimmedTimezone == "" {
		trimmedTimezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(trimmedTimezone)
	if err != nil {
		return TradingCalendar{}, fmt.Errorf("load scheduler timezone: %w", err)
	}
	return TradingCalendar{market: normalizedMarket, location: location}, nil
}

// PhaseAt 返回指定时间在当前市场中的交易阶段。
func (calendar TradingCalendar) PhaseAt(now time.Time) TradingPhase {
	localNow := now.In(calendar.location)
	weekday := localNow.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return PhaseNonTradingDay
	}
	minutes := localNow.Hour()*60 + localNow.Minute()
	switch {
	case minutes < 9*60+30:
		return PhaseBeforeOpen
	case minutes <= 11*60+30:
		return PhaseTradingTime
	case minutes < 13*60:
		return PhaseLunchBreak
	case minutes <= 15*60:
		return PhaseTradingTime
	default:
		return PhaseAfterClose
	}
}

// AllowsWindow 判断任务配置的交易窗口是否允许在指定时间执行。
func (calendar TradingCalendar) AllowsWindow(now time.Time, tradeWindow string) bool {
	phase := calendar.PhaseAt(now)
	if phase == PhaseNonTradingDay {
		return false
	}
	switch strings.TrimSpace(tradeWindow) {
	case "", TradeWindowAnyTime:
		return true
	case TradeWindowTradingTime:
		return phase == PhaseTradingTime
	case TradeWindowAfterClose:
		return phase == PhaseAfterClose
	default:
		return false
	}
}

// MissedToday 判断五段 cron 在当天启动前是否已经错过至少一个计划窗口。
func (calendar TradingCalendar) MissedToday(cronExpr string, now time.Time) (bool, time.Time, error) {
	localNow := now.In(calendar.location)
	if calendar.PhaseAt(localNow) == PhaseNonTradingDay {
		return false, time.Time{}, nil
	}
	scheduledAt, ok := latestScheduledAtTodayFromCron(cronExpr, localNow, calendar.location)
	if !ok {
		return false, time.Time{}, nil
	}
	return true, scheduledAt, nil
}
