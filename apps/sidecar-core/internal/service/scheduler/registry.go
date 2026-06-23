package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// CronTypeStockProfileRefresh 表示股票基础资料主动刷新任务。
	CronTypeStockProfileRefresh = "stock_profile_refresh"
	// CronTypeCNAShareKlineRefresh 表示 A 股 K 线刷新任务。
	CronTypeCNAShareKlineRefresh = "cn_a_share_kline_refresh"
	// CronTypeMarketNewsRefresh 表示市场新闻刷新任务。
	CronTypeMarketNewsRefresh = "market_news_refresh"
	// CronTypeSymbolNewsRefresh 表示个股新闻刷新任务。
	CronTypeSymbolNewsRefresh = "symbol_news_refresh"
	// CronTypeManualSymbolRefresh 表示用户手动单股刷新任务，不创建长期 job。
	CronTypeManualSymbolRefresh = "manual_symbol_refresh"
)

// JobTypeMetadata 描述一个可由桌面管理的调度任务类型。
type JobTypeMetadata struct {
	CronType        string `json:"cron_type"`
	Label           string `json:"label"`
	DefaultCronExpr string `json:"default_cron_expr"`
	DefaultWindow   string `json:"default_window"`
	Market          string `json:"market"`
	Enabled         bool   `json:"enabled"`
}

// JobRegistry 保存允许创建的 scheduler cron_type 和默认配置。
type JobRegistry struct {
	types map[string]JobTypeMetadata
}

// DefaultJobRegistry 返回首批支持或规划中的调度任务类型。
func DefaultJobRegistry() JobRegistry {
	types := []JobTypeMetadata{
		{CronType: CronTypeStockProfileRefresh, Label: "股票基础资料刷新", DefaultCronExpr: "0 8 * * 1-5", DefaultWindow: TradeWindowAnyTime, Market: "CN", Enabled: true},
		{CronType: CronTypeCNAShareQuoteRefresh, Label: "A 股行情刷新", DefaultCronExpr: "30 9 * * 1-5", DefaultWindow: TradeWindowTradingTime, Market: "CN", Enabled: true},
		{CronType: CronTypeCNAShareKlineRefresh, Label: "A 股 K 线刷新", DefaultCronExpr: "30 15 * * 1-5", DefaultWindow: TradeWindowAfterClose, Market: "CN", Enabled: true},
		{CronType: CronTypeMarketNewsRefresh, Label: "市场新闻刷新", DefaultCronExpr: "0 */2 * * 1-5", DefaultWindow: TradeWindowAnyTime, Market: "CN", Enabled: true},
		{CronType: CronTypeSymbolNewsRefresh, Label: "个股新闻刷新", DefaultCronExpr: "15 */2 * * 1-5", DefaultWindow: TradeWindowAnyTime, Market: "CN", Enabled: true},
	}
	registry := JobRegistry{types: make(map[string]JobTypeMetadata, len(types))}
	for _, item := range types {
		registry.types[item.CronType] = item
	}
	return registry
}

// Types 返回按稳定顺序排列的任务类型元数据。
func (registry JobRegistry) Types() []JobTypeMetadata {
	orderedCronTypes := []string{
		CronTypeStockProfileRefresh,
		CronTypeCNAShareQuoteRefresh,
		CronTypeCNAShareKlineRefresh,
		CronTypeMarketNewsRefresh,
		CronTypeSymbolNewsRefresh,
	}
	items := make([]JobTypeMetadata, 0, len(registry.types))
	for _, cronType := range orderedCronTypes {
		if item, ok := registry.types[cronType]; ok {
			items = append(items, item)
		}
	}
	return items
}

// IsSupportedCronType 判断 cron_type 是否属于当前允许创建和触发的调度任务类型。
func (registry JobRegistry) IsSupportedCronType(cronType string) bool {
	_, ok := registry.types[strings.TrimSpace(cronType)]
	return ok
}

// ValidateJob 在调度任务写入或注册前校验 cron_type、cron、市场、窗口和 JSON 参数。
func (registry JobRegistry) ValidateJob(job model.SchedulerJob) error {
	if !registry.IsSupportedCronType(job.CronType) {
		return fmt.Errorf("scheduler cron_type is unsupported")
	}
	if strings.TrimSpace(job.Name) == "" {
		return fmt.Errorf("scheduler job name is required")
	}
	if err := validateCronExpr(job.CronExpr); err != nil {
		return err
	}
	if _, err := NewTradingCalendar(job.Market, job.Timezone); err != nil {
		return err
	}
	if !validTradeWindow(job.TradeWindow) {
		return fmt.Errorf("scheduler trade_window is unsupported")
	}
	if err := validateJSONObject(job.ScopeJSON, true); err != nil {
		return fmt.Errorf("scheduler scope_json is invalid: %w", err)
	}
	if err := validateJSONObject(job.ParamsJSON, false); err != nil {
		return fmt.Errorf("scheduler params_json is invalid: %w", err)
	}
	if job.CatchupMaxDays < 0 {
		return fmt.Errorf("scheduler catchup_max_days is invalid")
	}
	if job.TimeoutSeconds < 0 {
		return fmt.Errorf("scheduler timeout_seconds is invalid")
	}
	return nil
}

// validateCronExpr 使用 gocron 创建一次未启动任务，确保 cron 表达式符合当前调度库语义。
func validateCronExpr(cronExpr string) error {
	if strings.TrimSpace(cronExpr) == "" {
		return fmt.Errorf("scheduler cron_expr is required")
	}
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return fmt.Errorf("create cron validator: %w", err)
	}
	defer scheduler.ShutdownWithContext(context.Background())
	if _, err := scheduler.NewJob(gocron.CronJob(cronExpr, false), gocron.NewTask(func() {})); err != nil {
		return fmt.Errorf("scheduler cron_expr is invalid: %w", err)
	}
	return nil
}

// validTradeWindow 判断任务窗口是否属于首批支持集合。
func validTradeWindow(tradeWindow string) bool {
	switch strings.TrimSpace(tradeWindow) {
	case "", TradeWindowTradingTime, TradeWindowAfterClose, TradeWindowAnyTime:
		return true
	default:
		return false
	}
}

// validateJSONObject 校验 JSON 字符串必须是对象，允许空 params 但 scope 必须显式给出对象。
func validateJSONObject(raw string, required bool) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if required {
			return fmt.Errorf("json object is required")
		}
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return err
	}
	return nil
}
