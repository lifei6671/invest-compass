package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	runtimeQuoteIntervalKey            = "data_source.quote_refresh_interval"
	runtimeBasicQuoteIntervalKey       = "quote.refresh_interval"
	runtimeNewsIntervalKey             = "data_source.news_sync_interval"
	defaultRuntimeQuoteInterval        = 60 * time.Second
	defaultRuntimeNewsInterval         = 30 * time.Minute
	runtimeManualSettingPollInterval   = 30 * time.Second
	runtimeRefreshSymbolTimeout        = 12 * time.Second
	runtimeRefreshNewsTimeout          = 15 * time.Second
	runtimeMinuteKlineLimit            = 242
	runtimeNewsRefreshLimit            = 30
	runtimeIndexRefreshScope           = "CN:SH:000001,CN:SZ:399001,CN:SZ:399006,CN:SH:000300"
	runtimeKlineRefreshParamsJSON      = `{"period":"minute","adjust":"none","limit":242}`
	runtimeMarketNewsRefreshParamsJSON = `{"limit":30}`
)

// RuntimeRefreshStore 定义运行期刷新需要的最小数据库能力。
type RuntimeRefreshStore interface {
	QuoteRefreshStore
	KlineRefreshStore
	NewsRefreshStore
	GetSettings(ctx context.Context, keys []string) ([]model.Setting, error)
}

// RuntimeRefresher 按设置中的刷新频率在后台更新行情、分时和新闻缓存。
type RuntimeRefresher struct {
	store          RuntimeRefreshStore
	marketProvider marketservice.MarketProvider
	newsProvider   newsservice.Provider
	now            func() time.Time
	calendar       TradingCalendar
	mu             sync.Mutex
	quoteRunning   bool
	newsRunning    bool
}

// NewRuntimeRefresher 创建进程级刷新服务，前端页面只负责读取缓存，不承担真实抓取职责。
func NewRuntimeRefresher(store RuntimeRefreshStore, marketProvider marketservice.MarketProvider, newsProvider newsservice.Provider, now func() time.Time) (*RuntimeRefresher, error) {
	if store == nil {
		return nil, fmt.Errorf("runtime refresh store is required")
	}
	calendar, err := NewTradingCalendar("CN", "Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &RuntimeRefresher{
		store:          store,
		marketProvider: marketProvider,
		newsProvider:   newsProvider,
		now:            now,
		calendar:       calendar,
	}, nil
}

// Start 启动两个后台循环：行情按交易时段执行，新闻不受交易时段限制。
func (refresher *RuntimeRefresher) Start(parent context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(parent)
	go refresher.quoteLoop(ctx)
	go refresher.newsLoop(ctx)
	return cancel
}

func (refresher *RuntimeRefresher) quoteLoop(ctx context.Context) {
	startup := true
	for {
		interval, enabled := refresher.quoteInterval(ctx)
		if !enabled {
			if !waitRuntimeInterval(ctx, runtimeManualSettingPollInterval) {
				return
			}
			continue
		}
		if startup {
			refresher.runQuoteCycle(ctx, true)
			startup = false
			continue
		}
		if !waitRuntimeInterval(ctx, interval) {
			return
		}
		refresher.runQuoteCycle(ctx, false)
	}
}

func (refresher *RuntimeRefresher) newsLoop(ctx context.Context) {
	for {
		interval, enabled := refresher.newsInterval(ctx)
		if !enabled {
			if !waitRuntimeInterval(ctx, runtimeManualSettingPollInterval) {
				return
			}
			continue
		}
		if !waitRuntimeInterval(ctx, interval) {
			return
		}
		refresher.runNewsCycle(ctx)
	}
}

func waitRuntimeInterval(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// runQuoteCycle 刷新四大指数与全部自选股的 quote 和当日分时缓存。
func (refresher *RuntimeRefresher) runQuoteCycle(ctx context.Context, startup bool) {
	if refresher.marketProvider == nil {
		return
	}
	if !startup && refresher.calendar.PhaseAt(refresher.now()) != PhaseTradingTime {
		return
	}
	if !refresher.beginQuoteCycle() {
		return
	}
	defer refresher.endQuoteCycle()

	symbols, err := refresher.runtimeQuoteSymbols(ctx)
	if err != nil {
		slog.Warn("读取运行期行情刷新范围失败", "error", logger.RedactError(err))
		return
	}
	refresher.refreshQuotes(ctx, symbols)
	refresher.refreshMinuteKlines(ctx, symbols)
}

func (refresher *RuntimeRefresher) beginQuoteCycle() bool {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	if refresher.quoteRunning {
		return false
	}
	refresher.quoteRunning = true
	return true
}

func (refresher *RuntimeRefresher) endQuoteCycle() {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	refresher.quoteRunning = false
}

func (refresher *RuntimeRefresher) runtimeQuoteSymbols(ctx context.Context) ([]string, error) {
	indexSymbols := symbolsFromScopeKey(runtimeIndexRefreshScope)
	watchlistSymbols, err := quoteSymbols(ctx, refresher.store, "CN")
	if err != nil {
		return nil, err
	}
	return dedupeRuntimeSymbols(append(indexSymbols, watchlistSymbols...)), nil
}

func (refresher *RuntimeRefresher) refreshQuotes(ctx context.Context, symbols []string) {
	run := model.SchedulerRun{
		CronType:   CronTypeCNAShareQuoteRefresh,
		ScopeKey:   strings.Join(symbols, ","),
		TargetDate: refresher.now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02"),
	}
	runner := QuoteRefreshRunner{Provider: refresher.marketProvider, Store: refresher.store}
	if _, err := runner.Run(ctx, run); err != nil {
		slog.Warn("运行期行情刷新失败", "error", logger.RedactError(err))
	}
}

func (refresher *RuntimeRefresher) refreshMinuteKlines(ctx context.Context, symbols []string) {
	runner := KlineRefreshRunner{Provider: refresher.marketProvider, Store: refresher.store}
	for _, rawSymbol := range symbols {
		runCtx, cancel := context.WithTimeout(ctx, runtimeRefreshSymbolTimeout)
		_, err := runner.Run(runCtx, model.SchedulerRun{
			CronType:   CronTypeCNAShareKlineRefresh,
			ScopeKey:   rawSymbol,
			ParamsJSON: runtimeKlineRefreshParamsJSON,
			TargetDate: refresher.now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02"),
		})
		cancel()
		if err != nil {
			slog.Warn("运行期分时刷新失败", "symbol", rawSymbol, "error", logger.RedactError(err))
		}
	}
}

// runNewsCycle 刷新市场新闻缓存；新闻不依赖交易时段。
func (refresher *RuntimeRefresher) runNewsCycle(ctx context.Context) {
	if refresher.newsProvider == nil {
		return
	}
	if !refresher.beginNewsCycle() {
		return
	}
	defer refresher.endNewsCycle()

	runCtx, cancel := context.WithTimeout(ctx, runtimeRefreshNewsTimeout)
	defer cancel()
	runner := NewsRefreshRunner{Provider: refresher.newsProvider, Store: refresher.store}
	_, err := runner.Run(runCtx, model.SchedulerRun{
		CronType:   CronTypeMarketNewsRefresh,
		ScopeKey:   "CN",
		ParamsJSON: runtimeMarketNewsRefreshParamsJSON,
		TargetDate: refresher.now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02"),
	})
	if err != nil {
		slog.Warn("运行期新闻刷新失败", "error", logger.RedactError(err))
	}
}

func (refresher *RuntimeRefresher) beginNewsCycle() bool {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	if refresher.newsRunning {
		return false
	}
	refresher.newsRunning = true
	return true
}

func (refresher *RuntimeRefresher) endNewsCycle() {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	refresher.newsRunning = false
}

func (refresher *RuntimeRefresher) quoteInterval(ctx context.Context) (time.Duration, bool) {
	items, err := refresher.store.GetSettings(ctx, []string{runtimeQuoteIntervalKey, runtimeBasicQuoteIntervalKey})
	if err != nil {
		slog.Warn("读取行情刷新频率失败", "error", logger.RedactError(err))
		return defaultRuntimeQuoteInterval, true
	}
	return parseRuntimeInterval(settingValue(items, runtimeQuoteIntervalKey, runtimeBasicQuoteIntervalKey), defaultRuntimeQuoteInterval)
}

func (refresher *RuntimeRefresher) newsInterval(ctx context.Context) (time.Duration, bool) {
	items, err := refresher.store.GetSettings(ctx, []string{runtimeNewsIntervalKey})
	if err != nil {
		slog.Warn("读取新闻刷新频率失败", "error", logger.RedactError(err))
		return defaultRuntimeNewsInterval, true
	}
	return parseRuntimeInterval(settingValue(items, runtimeNewsIntervalKey), defaultRuntimeNewsInterval)
}

func settingValue(items []model.Setting, keys ...string) string {
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.Key] = item.Value
	}
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func parseRuntimeInterval(raw string, fallback time.Duration) (time.Duration, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return fallback, true
	}
	if trimmed == "manual" {
		return 0, false
	}
	interval, err := time.ParseDuration(trimmed)
	if err != nil || interval <= 0 {
		return fallback, true
	}
	return interval, true
}

func dedupeRuntimeSymbols(symbols []string) []string {
	result := make([]string, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, rawSymbol := range symbols {
		symbol, err := stockservice.ParseSymbol(rawSymbol)
		if err != nil || symbol.Market != "CN" {
			continue
		}
		key := symbol.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
