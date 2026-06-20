package scheduler

import (
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestDefaultJobRegistryValidatesQuoteRefreshJob 验证默认注册表接受首批 A 股 quote 刷新任务。
func TestDefaultJobRegistryValidatesQuoteRefreshJob(t *testing.T) {
	registry := DefaultJobRegistry()
	job := model.SchedulerJob{
		Name:           "A 股开盘行情刷新",
		CronType:       CronTypeCNAShareQuoteRefresh,
		CronExpr:       "30 9 * * 1-5",
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TradeWindow:    TradeWindowTradingTime,
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON:     `{}`,
		CatchupEnabled: true,
		CatchupMaxDays: 5,
		TimeoutSeconds: 120,
	}

	if err := registry.ValidateJob(job); err != nil {
		t.Fatalf("validate quote refresh job: %v", err)
	}
}

// TestDefaultJobRegistryRejectsInvalidJobs 验证注册表在任务进入调度器前快速拒绝非法配置。
func TestDefaultJobRegistryRejectsInvalidJobs(t *testing.T) {
	registry := DefaultJobRegistry()
	validJob := model.SchedulerJob{
		Name:           "A 股开盘行情刷新",
		CronType:       CronTypeCNAShareQuoteRefresh,
		CronExpr:       "30 9 * * 1-5",
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TradeWindow:    TradeWindowTradingTime,
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON:     `{}`,
		CatchupMaxDays: 5,
		TimeoutSeconds: 120,
	}

	cases := []struct {
		name   string
		mutate func(*model.SchedulerJob)
	}{
		{name: "unknown cron type", mutate: func(job *model.SchedulerJob) { job.CronType = "unknown" }},
		{name: "invalid cron expression", mutate: func(job *model.SchedulerJob) { job.CronExpr = "bad cron" }},
		{name: "unsupported market", mutate: func(job *model.SchedulerJob) { job.Market = "HK" }},
		{name: "invalid trade window", mutate: func(job *model.SchedulerJob) { job.TradeWindow = "pre_market" }},
		{name: "invalid scope json", mutate: func(job *model.SchedulerJob) { job.ScopeJSON = `{"symbols":` }},
		{name: "invalid params json", mutate: func(job *model.SchedulerJob) { job.ParamsJSON = `{"period":` }},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			job := validJob
			item.mutate(&job)
			if err := registry.ValidateJob(job); err == nil {
				t.Fatalf("expected invalid job %s to fail", item.name)
			}
		})
	}
}

// TestDefaultJobRegistryListsCronTypeMetadata 验证 UI 可读取稳定的 cron_type 元数据。
func TestDefaultJobRegistryListsCronTypeMetadata(t *testing.T) {
	types := DefaultJobRegistry().Types()
	if len(types) == 0 {
		t.Fatal("expected scheduler job type metadata")
	}
	foundQuote := false
	for _, item := range types {
		if item.CronType == CronTypeCNAShareQuoteRefresh {
			foundQuote = item.Label != "" && item.DefaultCronExpr != ""
		}
	}
	if !foundQuote {
		t.Fatalf("missing quote refresh metadata: %+v", types)
	}
}
