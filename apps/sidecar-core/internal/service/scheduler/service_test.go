package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// TestServiceRestoreEnqueuesMissedTodayWhenAppStartsAfterOpen 验证交易日开盘后启动会补偿当天已过期窗口。
func TestServiceRestoreEnqueuesMissedTodayWhenAppStartsAfterOpen(t *testing.T) {
	location := mustShanghaiLocation(t)
	store := newMemoryStore([]model.SchedulerJob{openQuoteJob()})
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	runs := store.createdRuns
	if len(runs) != 1 {
		t.Fatalf("expected one missed_today run, got %+v", runs)
	}
	if runs[0].TriggerType != TriggerMissedToday || runs[0].TargetDate != "2026-06-19" || runs[0].Status != RunStatusQueued {
		t.Fatalf("unexpected missed_today run: %+v", runs[0])
	}
	if runs[0].RunKey != "scheduler_job:42:2026-06-19:missed_today" {
		t.Fatalf("unexpected missed_today run key: %s", runs[0].RunKey)
	}
	queued := queue.Snapshot()
	if len(queued) != 1 || queued[0].RunKey != runs[0].RunKey {
		t.Fatalf("expected missed_today run to enter execution queue, got %+v", queued)
	}
}

// TestServiceRestoreSkipsMissedTodayBeforeWindow 验证开盘窗口前启动不会提前生成补偿任务。
func TestServiceRestoreSkipsMissedTodayBeforeWindow(t *testing.T) {
	location := mustShanghaiLocation(t)
	store := newMemoryStore([]model.SchedulerJob{openQuoteJob()})
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 9, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	if len(store.createdRuns) != 0 {
		t.Fatalf("expected no missed_today run before open window, got %+v", store.createdRuns)
	}
	if len(queue.Snapshot()) != 0 {
		t.Fatalf("expected empty execution queue before open window, got %+v", queue.Snapshot())
	}
}

// TestServiceRestoreSkipsMissedTodayWhenScheduledTimeOutsideTradeWindow 验证启动补偿只补偿原计划触发时满足任务窗口的执行。
func TestServiceRestoreSkipsMissedTodayWhenScheduledTimeOutsideTradeWindow(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.CronExpr = "30 14 * * 1-5"
	job.TradeWindow = TradeWindowAfterClose
	store := newMemoryStore([]model.SchedulerJob{job})
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 15, 30, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	if len(store.createdRuns) != 0 {
		t.Fatalf("expected no missed_today run when scheduled time was outside trade window, got %+v", store.createdRuns)
	}
	if len(queue.Snapshot()) != 0 {
		t.Fatalf("expected empty execution queue when scheduled time was outside trade window, got %+v", queue.Snapshot())
	}
}

// TestServiceRestoreEnqueuesMissedTodayForIntervalCron 验证开盘后才启动时会补偿 interval cron 最近一次已错过窗口。
func TestServiceRestoreEnqueuesMissedTodayForIntervalCron(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.CronExpr = "*/5 9-11,13-14 * * 1-5"
	store := newMemoryStore([]model.SchedulerJob{job})
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	runs := store.createdRuns
	if len(runs) != 1 {
		t.Fatalf("expected one interval cron missed_today run, got %+v", runs)
	}
	if runs[0].TriggerType != TriggerMissedToday || runs[0].TargetDate != "2026-06-19" {
		t.Fatalf("unexpected interval missed_today run: %+v", runs[0])
	}
}

// TestServiceScheduledRunKeyIncludesCronSlot 验证同一天多次 scheduled 触发不会被日期级 run_key 吞掉。
func TestServiceScheduledRunKeyIncludesCronSlot(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.CronExpr = "0 */2 * * 1-5"
	job.TradeWindow = TradeWindowAnyTime
	store := newMemoryStore([]model.SchedulerJob{job})
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 8, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.enqueueJobRun(context.Background(), job, TriggerScheduled, time.Date(2026, 6, 19, 10, 0, 0, 0, location)); err != nil {
		t.Fatalf("enqueue first scheduled run: %v", err)
	}
	if err := service.enqueueJobRun(context.Background(), job, TriggerScheduled, time.Date(2026, 6, 19, 12, 0, 0, 0, location)); err != nil {
		t.Fatalf("enqueue second scheduled run: %v", err)
	}

	if len(store.createdRuns) != 2 {
		t.Fatalf("expected two same-day scheduled runs, got %+v", store.createdRuns)
	}
	if store.createdRuns[0].RunKey == store.createdRuns[1].RunKey {
		t.Fatalf("expected distinct scheduled run keys, got %q", store.createdRuns[0].RunKey)
	}
}

// TestServiceRestoreEnqueuesCatchupGapFromWatermark 验证启动时会根据水位补偿今天之前的交易日缺口。
func TestServiceRestoreEnqueuesCatchupGapFromWatermark(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ParamsJSON = `{"provider":"sina_tencent"}`
	store := newMemoryStore([]model.SchedulerJob{job})
	store.watermarks = map[string]model.IngestionWatermark{
		"quote|CN:SH:600519|sina_tencent|": {
			DataType:      "quote",
			ScopeKey:      "CN:SH:600519",
			Provider:      "sina_tencent",
			LastTradeDate: "2026-06-17",
		},
	}
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	var catchupRun model.SchedulerRun
	for _, run := range store.createdRuns {
		if run.TriggerType == TriggerCatchupGap {
			catchupRun = run
		}
	}
	if catchupRun.RunKey == "" {
		t.Fatalf("expected catchup_gap run, got %+v", store.createdRuns)
	}
	if catchupRun.TargetDate != "2026-06-18" || catchupRun.ScopeKey != "CN:SH:600519" || catchupRun.RunKey != "scheduler_job:42:CN:SH:600519:2026-06-18:catchup_gap" {
		t.Fatalf("unexpected catchup run: %+v", catchupRun)
	}
}

// TestServiceRestoreCatchupUsesActiveSymbolWatermarksForMarketScope 验证市场范围任务按 active symbol 水位补历史缺口。
func TestServiceRestoreCatchupUsesActiveSymbolWatermarksForMarketScope(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ScopeJSON = `{"market":"CN"}`
	job.ParamsJSON = `{"provider":"sina_tencent"}`
	store := newMemoryStore([]model.SchedulerJob{job})
	store.activeWatchlists = []model.Watchlist{{Symbol: "CN:SH:600519"}}
	store.watermarks = map[string]model.IngestionWatermark{
		"quote|CN:SH:600519|sina_tencent|": {
			DataType:      "quote",
			ScopeKey:      "CN:SH:600519",
			Provider:      "sina_tencent",
			LastTradeDate: "2026-06-17",
		},
	}
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	var catchupRun model.SchedulerRun
	for _, run := range store.createdRuns {
		if run.TriggerType == TriggerCatchupGap {
			catchupRun = run
		}
	}
	if catchupRun.RunKey == "" {
		t.Fatalf("expected market-scope catchup run from symbol watermark, got %+v", store.createdRuns)
	}
	if catchupRun.ScopeKey != "CN:SH:600519" || catchupRun.TargetDate != "2026-06-18" {
		t.Fatalf("unexpected market-scope catchup run: %+v", catchupRun)
	}
}

// TestServiceRestoreCatchupUsesSymbolWatermarksForExplicitMultiSymbolScope 验证显式多股票任务按逐 symbol 水位补历史缺口。
func TestServiceRestoreCatchupUsesSymbolWatermarksForExplicitMultiSymbolScope(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ScopeJSON = `{"symbols":["CN:SH:600519","CN:SZ:000001"]}`
	job.ParamsJSON = `{"provider":"sina_tencent"}`
	store := newMemoryStore([]model.SchedulerJob{job})
	store.watermarks = map[string]model.IngestionWatermark{
		"quote|CN:SH:600519|sina_tencent|": {
			DataType:      "quote",
			ScopeKey:      "CN:SH:600519",
			Provider:      "sina_tencent",
			LastTradeDate: "2026-06-17",
		},
		"quote|CN:SZ:000001|sina_tencent|": {
			DataType:      "quote",
			ScopeKey:      "CN:SZ:000001",
			Provider:      "sina_tencent",
			LastTradeDate: "2026-06-17",
		},
	}
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	catchupScopes := map[string]string{}
	for _, run := range store.createdRuns {
		if run.TriggerType == TriggerCatchupGap {
			catchupScopes[run.ScopeKey] = run.TargetDate
		}
	}
	if len(catchupScopes) != 2 || catchupScopes["CN:SH:600519"] != "2026-06-18" || catchupScopes["CN:SZ:000001"] != "2026-06-18" {
		t.Fatalf("expected catchup runs from per-symbol watermarks, got %+v", store.createdRuns)
	}
}

// TestServiceRestoreSkipsNonTradingDaysWhenPlanningCatchupGaps 验证历史补偿会跳过周末等非交易日。
func TestServiceRestoreSkipsNonTradingDaysWhenPlanningCatchupGaps(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ParamsJSON = `{"provider":"sina_tencent"}`
	store := newMemoryStore([]model.SchedulerJob{job})
	store.watermarks = map[string]model.IngestionWatermark{
		"quote|CN:SH:600519|sina_tencent|": {
			DataType:      "quote",
			ScopeKey:      "CN:SH:600519",
			Provider:      "sina_tencent",
			LastTradeDate: "2026-06-19",
		},
	}
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 8, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	var catchupDates []string
	for _, run := range store.createdRuns {
		if run.TriggerType == TriggerCatchupGap {
			catchupDates = append(catchupDates, run.TargetDate)
		}
	}
	if len(catchupDates) != 1 || catchupDates[0] != "2026-06-22" {
		t.Fatalf("expected only Monday catchup after weekend, got %+v", catchupDates)
	}
}

// TestServiceRestoreIgnoresDuplicateMissedTodayRunKey 验证重复启动时相同 run_key 不会让恢复流程失败。
func TestServiceRestoreIgnoresDuplicateMissedTodayRunKey(t *testing.T) {
	location := mustShanghaiLocation(t)
	store := newMemoryStore([]model.SchedulerJob{openQuoteJob()})
	store.duplicateRunKeys = map[string]struct{}{
		"scheduler_job:42:2026-06-19:missed_today": {},
	}
	service, err := NewService(Config{
		Store: store,
		Queue: NewExecutionQueue(),
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler should ignore duplicate missed_today run: %v", err)
	}
}

// TestServiceRestoreSkipsExistingSuccessfulMissedTodayRun 验证已成功的 missed_today run 不会在启动恢复时重复创建。
func TestServiceRestoreSkipsExistingSuccessfulMissedTodayRun(t *testing.T) {
	location := mustShanghaiLocation(t)
	store := newMemoryStore([]model.SchedulerJob{openQuoteJob()})
	store.runsByKey = map[string]model.SchedulerRun{
		"scheduler_job:42:2026-06-19:missed_today": {
			RunKey:      "scheduler_job:42:2026-06-19:missed_today",
			TriggerType: TriggerMissedToday,
			Status:      RunStatusSuccess,
			TargetDate:  "2026-06-19",
		},
	}
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	if store.getRunByKeyCalls != 1 {
		t.Fatalf("expected explicit run_key lookup before creating missed_today run, got %d", store.getRunByKeyCalls)
	}
	if len(store.createdRuns) != 0 {
		t.Fatalf("existing successful missed_today run must not be recreated, got %+v", store.createdRuns)
	}
	if len(queue.Snapshot()) != 0 {
		t.Fatalf("existing successful missed_today run must not be enqueued, got %+v", queue.Snapshot())
	}
}

// TestServiceRestoreMarksDuplicateActiveScopeSkipped 验证启动补偿被队列范围去重时不会残留 queued 记录。
func TestServiceRestoreMarksDuplicateActiveScopeSkipped(t *testing.T) {
	location := mustShanghaiLocation(t)
	store := newMemoryStore([]model.SchedulerJob{openQuoteJob()})
	queue := NewExecutionQueue()
	if _, err := queue.Enqueue(context.Background(), model.SchedulerRun{
		RunKey:     "existing-user-request",
		DataType:   "quote",
		ScopeKey:   "CN:SH:600519",
		Status:     RunStatusQueued,
		TargetDate: "2026-06-19",
	}); err != nil {
		t.Fatalf("enqueue existing scheduler run: %v", err)
	}
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	if len(store.updatedRuns) != 1 {
		t.Fatalf("expected duplicate active scope to be marked skipped, got %+v", store.updatedRuns)
	}
	if store.updatedRuns[0].Status != RunStatusSkipped || store.updatedRuns[0].SkippedReason != "scheduler_duplicate_active_scope" {
		t.Fatalf("unexpected duplicate active scope state: %+v", store.updatedRuns[0])
	}
}

// TestServiceRestoreFinalizesInterruptedRuns 验证 sidecar 重启时不会重放旧 run，而是把遗留记录落为终态。
func TestServiceRestoreFinalizesInterruptedRuns(t *testing.T) {
	queued := model.SchedulerRun{ID: 11, RunKey: "queued-before-restart", Status: RunStatusQueued}
	running := model.SchedulerRun{ID: 12, RunKey: "running-before-restart", Status: RunStatusRunning}
	store := newMemoryStore(nil)
	store.interruptedRuns = []model.SchedulerRun{queued, running}
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.Restore(context.Background()); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	if len(queue.Snapshot()) != 0 {
		t.Fatalf("expected interrupted runs not to be replayed, got queue %+v", queue.Snapshot())
	}
	if len(store.updatedRuns) != 2 {
		t.Fatalf("expected two interrupted runs to be finalized, got %+v", store.updatedRuns)
	}
	if store.updatedRuns[0].Status != RunStatusCancelled ||
		store.updatedRuns[0].SkippedReason != "sidecar restarted before queued run started" {
		t.Fatalf("unexpected queued recovery result: %+v", store.updatedRuns[0])
	}
	if store.updatedRuns[1].Status != RunStatusFailed ||
		store.updatedRuns[1].ErrorMessage != "sidecar restarted before scheduler run finished" {
		t.Fatalf("unexpected running recovery result: %+v", store.updatedRuns[1])
	}
}

// TestServiceRunNowCreatesUserRequestRun 验证 service 层可为长期 job 创建立即执行记录并复用执行队列。
func TestServiceRunNowCreatesUserRequestRun(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ID = 7
	job.ParamsJSON = `{"provider":"sina_tencent"}`
	job.TimeoutSeconds = 45
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	run, err := service.RunNow(context.Background(), job, RunNowRequest{
		TargetDate:        "2026-06-19",
		RequestedAt:       time.Date(2026, 6, 19, 10, 5, 0, 0, location),
		IgnoreTradeWindow: true,
	})
	if err != nil {
		t.Fatalf("run scheduler job now: %v", err)
	}

	if len(store.createdRuns) != 1 {
		t.Fatalf("expected one service run-now record, got %+v", store.createdRuns)
	}
	created := store.createdRuns[0]
	if created.JobID != 7 || created.TriggerType != TriggerUserRequest || created.Source != "user_request" {
		t.Fatalf("unexpected service run-now source: %+v", created)
	}
	if created.Status != RunStatusQueued || created.Priority != 100 || created.TargetDate != "2026-06-19" || created.ScopeKey != "CN:SH:600519" {
		t.Fatalf("unexpected service run-now queue fields: %+v", created)
	}
	if !strings.HasPrefix(created.RunKey, "user_request:cn_a_share_quote_refresh:CN:SH:600519:2026-06-19:") {
		t.Fatalf("unexpected service run-now key: %s", created.RunKey)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(created.ParamsJSON), &params); err != nil {
		t.Fatalf("run-now params_json should stay valid JSON, got %q: %v", created.ParamsJSON, err)
	}
	if params["provider"] != "sina_tencent" || params["timeout_seconds"] != float64(45) {
		t.Fatalf("expected run-now params to preserve provider and timeout, got %+v", params)
	}
	if run.RunKey != created.RunKey {
		t.Fatalf("returned run should match created run, got returned=%+v created=%+v", run, created)
	}
	if len(queue.Snapshot()) != 1 || queue.Snapshot()[0].RunKey != created.RunKey {
		t.Fatalf("expected service run-now to enter queue, got %+v", queue.Snapshot())
	}
}

// TestServiceRunNowRejectsClosedTradeWindowByDefault 验证立即执行默认遵守交易窗口，避免手动操作绕过运行边界。
func TestServiceRunNowRejectsClosedTradeWindowByDefault(t *testing.T) {
	location := mustShanghaiLocation(t)
	job := openQuoteJob()
	job.ID = 7
	job.TradeWindow = TradeWindowTradingTime
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	_, err = service.RunNow(context.Background(), job, RunNowRequest{
		TargetDate:  "2026-06-19",
		RequestedAt: time.Date(2026, 6, 19, 8, 0, 0, 0, location),
	})

	if !errors.Is(err, ErrTradeWindowClosed) {
		t.Fatalf("expected closed trade window error, got %v", err)
	}
	if len(store.createdRuns) != 0 || len(queue.Snapshot()) != 0 {
		t.Fatalf("closed trade window must not create or enqueue run, created=%+v queue=%+v", store.createdRuns, queue.Snapshot())
	}
}

// TestServiceBackfillCreatesCatchupRuns 验证 service 层可按交易日范围创建手动补偿 run 并复用执行队列。
func TestServiceBackfillCreatesCatchupRuns(t *testing.T) {
	job := openQuoteJob()
	job.ID = 7
	job.CronType = CronTypeCNAShareKlineRefresh
	job.ParamsJSON = `{"period":"day","adjust":"none","limit":120}`
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	runs, err := service.Backfill(context.Background(), job, BackfillRequest{
		DateFrom: "2026-06-19",
		DateTo:   "2026-06-22",
		Symbols:  []string{" CN:SH:600519 "},
	})
	if err != nil {
		t.Fatalf("backfill scheduler job: %v", err)
	}

	if len(runs) != 2 || len(store.createdRuns) != 2 {
		t.Fatalf("expected Friday and Monday backfill runs, got returned=%+v created=%+v", runs, store.createdRuns)
	}
	if len(queue.Snapshot()) != 2 {
		t.Fatalf("expected backfill runs to enter queue, got %+v", queue.Snapshot())
	}
	for _, run := range store.createdRuns {
		if run.TriggerType != TriggerCatchupGap || run.Source != "manual_backfill" || run.Priority != 60 {
			t.Fatalf("unexpected backfill run source fields: %+v", run)
		}
		if run.ScopeKey != "CN:SH:600519" || run.Period != "day" || run.DataType != "kline" {
			t.Fatalf("unexpected backfill run scope fields: %+v", run)
		}
		if run.TargetDate != "2026-06-19" && run.TargetDate != "2026-06-22" {
			t.Fatalf("unexpected backfill target date: %+v", run)
		}
	}
}

// TestServiceBackfillKeepsDifferentSymbolScopes 验证同一任务同一交易日的不同 symbol scope 不会被 run_key 误判为重复。
func TestServiceBackfillKeepsDifferentSymbolScopes(t *testing.T) {
	job := openQuoteJob()
	job.ID = 7
	job.CronType = CronTypeCNAShareKlineRefresh
	job.ParamsJSON = `{"period":"day","adjust":"none","limit":120}`
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	firstRuns, err := service.Backfill(context.Background(), job, BackfillRequest{
		DateFrom: "2026-06-19",
		DateTo:   "2026-06-19",
		Symbols:  []string{"CN:SH:600519"},
	})
	if err != nil {
		t.Fatalf("backfill first symbol scope: %v", err)
	}
	if len(firstRuns) != 1 || len(store.createdRuns) != 1 {
		t.Fatalf("expected first symbol scope to create one run, returned=%+v created=%+v", firstRuns, store.createdRuns)
	}
	store.runsByKey = map[string]model.SchedulerRun{store.createdRuns[0].RunKey: store.createdRuns[0]}

	secondRuns, err := service.Backfill(context.Background(), job, BackfillRequest{
		DateFrom: "2026-06-19",
		DateTo:   "2026-06-19",
		Symbols:  []string{"CN:SH:000001"},
	})
	if err != nil {
		t.Fatalf("backfill second symbol scope: %v", err)
	}

	if len(secondRuns) != 1 || len(store.createdRuns) != 2 {
		t.Fatalf("expected second symbol scope to create its own run, returned=%+v created=%+v", secondRuns, store.createdRuns)
	}
	if store.createdRuns[0].RunKey == store.createdRuns[1].RunKey {
		t.Fatalf("expected different symbol scopes to use distinct run keys, got %q", store.createdRuns[0].RunKey)
	}
	if store.createdRuns[1].ScopeKey != "CN:SH:000001" {
		t.Fatalf("unexpected second run scope key: %+v", store.createdRuns[1])
	}
}

// TestServiceReloadJobReplacesExistingSchedule 验证重载同一长期任务时会先移除旧 cron entry，避免重复触发。
func TestServiceReloadJobReplacesExistingSchedule(t *testing.T) {
	job := openQuoteJob()
	job.ID = 7
	job.Name = "旧任务名称"
	store := newMemoryStore(nil)
	service, err := NewService(Config{Store: store, Queue: NewExecutionQueue()})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}

	if err := service.ReloadJob(context.Background(), job); err != nil {
		t.Fatalf("reload initial scheduler job: %v", err)
	}
	job.Name = "新任务名称"
	job.CronExpr = "35 9 * * 1-5"
	if err := service.ReloadJob(context.Background(), job); err != nil {
		t.Fatalf("reload updated scheduler job: %v", err)
	}

	jobs := service.scheduler.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected one scheduler entry after reload, got %d", len(jobs))
	}
	if jobs[0].Name() != "新任务名称" {
		t.Fatalf("expected reloaded scheduler entry name, got %s", jobs[0].Name())
	}
}

// TestServiceReloadJobRemovesDisabledSchedule 验证禁用任务重载时只移除 cron entry，不再注册未来触发器。
func TestServiceReloadJobRemovesDisabledSchedule(t *testing.T) {
	job := openQuoteJob()
	job.ID = 7
	store := newMemoryStore(nil)
	service, err := NewService(Config{Store: store, Queue: NewExecutionQueue()})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	if err := service.ReloadJob(context.Background(), job); err != nil {
		t.Fatalf("reload enabled scheduler job: %v", err)
	}

	job.Enabled = false
	if err := service.ReloadJob(context.Background(), job); err != nil {
		t.Fatalf("reload disabled scheduler job: %v", err)
	}

	if jobs := service.scheduler.Jobs(); len(jobs) != 0 {
		t.Fatalf("expected disabled scheduler job to remove entry, got %d", len(jobs))
	}
}

// TestServiceEnqueueScheduledRunSkipsClosedTradeWindows 验证 scheduled run 不满足交易窗口时会落库为 skipped 且不入队。
func TestServiceEnqueueScheduledRunSkipsClosedTradeWindows(t *testing.T) {
	location := mustShanghaiLocation(t)
	cases := []struct {
		name        string
		tradeWindow string
		triggerTime time.Time
	}{
		{
			name:        "trading time before open",
			tradeWindow: TradeWindowTradingTime,
			triggerTime: time.Date(2026, 6, 19, 8, 0, 0, 0, location),
		},
		{
			name:        "trading time lunch break",
			tradeWindow: TradeWindowTradingTime,
			triggerTime: time.Date(2026, 6, 19, 12, 0, 0, 0, location),
		},
		{
			name:        "after close before close",
			tradeWindow: TradeWindowAfterClose,
			triggerTime: time.Date(2026, 6, 19, 14, 30, 0, 0, location),
		},
		{
			name:        "any time on weekend",
			tradeWindow: TradeWindowAnyTime,
			triggerTime: time.Date(2026, 6, 20, 10, 0, 0, 0, location),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			job := openQuoteJob()
			job.TradeWindow = tt.tradeWindow
			store := newMemoryStore(nil)
			queue := NewExecutionQueue()
			service, err := NewService(Config{
				Store: store,
				Queue: queue,
			})
			if err != nil {
				t.Fatalf("new scheduler service: %v", err)
			}

			err = service.enqueueJobRun(
				context.Background(),
				job,
				TriggerScheduled,
				tt.triggerTime,
			)
			if err != nil {
				t.Fatalf("enqueue scheduled run: %v", err)
			}

			if len(store.createdRuns) != 1 {
				t.Fatalf("expected one skipped run to be created, got %+v", store.createdRuns)
			}
			run := store.createdRuns[0]
			if run.Status != RunStatusSkipped || run.SkippedReason != SkippedReasonTradeWindowClosed {
				t.Fatalf("unexpected closed window run state: %+v", run)
			}
			if len(queue.Snapshot()) != 0 {
				t.Fatalf("closed trade window run must not enter queue, got %+v", queue.Snapshot())
			}
		})
	}
}

// TestServiceRunQueuedOnceExecutesRegisteredRunner 验证队列 worker 会按 cron_type 调用注册 runner 并更新执行结果。
func TestServiceRunQueuedOnceExecutesRegisteredRunner(t *testing.T) {
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Runners: map[string]Runner{
			"test_runner": runnerFunc(func(context.Context, model.SchedulerRun) (RunResult, error) {
				return RunResult{FetchedCount: 2, WrittenCount: 2}, nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	if _, err := queue.Enqueue(context.Background(), model.SchedulerRun{
		ID:       9,
		RunKey:   "run-9",
		CronType: "test_runner",
		Status:   RunStatusQueued,
	}); err != nil {
		t.Fatalf("enqueue scheduler run: %v", err)
	}

	if err := service.RunQueuedOnce(context.Background()); err != nil {
		t.Fatalf("run queued once: %v", err)
	}

	if len(store.updatedRuns) != 2 {
		t.Fatalf("expected running and success updates, got %+v", store.updatedRuns)
	}
	if store.updatedRuns[0].Status != RunStatusRunning {
		t.Fatalf("expected first update to mark running, got %+v", store.updatedRuns[0])
	}
	finalRun := store.updatedRuns[1]
	if finalRun.Status != RunStatusSuccess || finalRun.FetchedCount != 2 || finalRun.WrittenCount != 2 {
		t.Fatalf("unexpected final scheduler run update: %+v", finalRun)
	}
}

// TestServiceRunQueuedOncePersistsSkippedRunnerResult 验证 runner 主动跳过时会落库为 skipped，而不是误记为成功。
func TestServiceRunQueuedOncePersistsSkippedRunnerResult(t *testing.T) {
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Runners: map[string]Runner{
			"test_runner": runnerFunc(func(context.Context, model.SchedulerRun) (RunResult, error) {
				return RunResult{SkippedReason: "empty_scope"}, nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	if _, err := queue.Enqueue(context.Background(), model.SchedulerRun{
		ID:       10,
		RunKey:   "run-10",
		CronType: "test_runner",
		Status:   RunStatusQueued,
	}); err != nil {
		t.Fatalf("enqueue scheduler run: %v", err)
	}

	if err := service.RunQueuedOnce(context.Background()); err != nil {
		t.Fatalf("run queued once: %v", err)
	}

	if len(store.updatedRuns) != 2 {
		t.Fatalf("expected running and skipped updates, got %+v", store.updatedRuns)
	}
	finalRun := store.updatedRuns[1]
	if finalRun.Status != RunStatusSkipped || finalRun.SkippedReason != "empty_scope" {
		t.Fatalf("expected skipped scheduler run, got %+v", finalRun)
	}
}

// TestServiceRunQueuedOnceRedactsRunnerError 验证 runner 失败写入执行记录前会脱敏敏感字段。
func TestServiceRunQueuedOnceRedactsRunnerError(t *testing.T) {
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Runners: map[string]Runner{
			"test_runner": runnerFunc(func(context.Context, model.SchedulerRun) (RunResult, error) {
				return RunResult{}, errors.New("provider failed Authorization: Bearer scheduler-secret-token api_key=scheduler-secret-key")
			}),
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	if _, err := queue.Enqueue(context.Background(), model.SchedulerRun{
		ID:       10,
		RunKey:   "run-10",
		CronType: "test_runner",
		Status:   RunStatusQueued,
	}); err != nil {
		t.Fatalf("enqueue scheduler run: %v", err)
	}

	if err := service.RunQueuedOnce(context.Background()); err != nil {
		t.Fatalf("run queued once: %v", err)
	}

	if len(store.updatedRuns) != 2 {
		t.Fatalf("expected running and failed updates, got %+v", store.updatedRuns)
	}
	finalRun := store.updatedRuns[1]
	if finalRun.Status != RunStatusFailed {
		t.Fatalf("expected failed scheduler run, got %+v", finalRun)
	}
	for _, secret := range []string{"scheduler-secret-token", "scheduler-secret-key"} {
		if strings.Contains(finalRun.ErrorMessage, secret) {
			t.Fatalf("scheduler run error leaked secret %q: %s", secret, finalRun.ErrorMessage)
		}
	}
	if !strings.Contains(finalRun.ErrorMessage, logger.RedactedValue) {
		t.Fatalf("scheduler run error should contain redaction marker, got %q", finalRun.ErrorMessage)
	}
}

// TestServiceRunQueuedOnceTimesOutRunner 验证执行记录中的 timeout_seconds 会限制 runner 执行时长。
func TestServiceRunQueuedOnceTimesOutRunner(t *testing.T) {
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Runners: map[string]Runner{
			"test_runner": runnerFunc(func(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
				<-ctx.Done()
				return RunResult{}, ctx.Err()
			}),
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	if _, err := queue.Enqueue(context.Background(), model.SchedulerRun{
		ID:         11,
		RunKey:     "run-11",
		CronType:   "test_runner",
		Status:     RunStatusQueued,
		ParamsJSON: `{"timeout_seconds":1}`,
	}); err != nil {
		t.Fatalf("enqueue scheduler run: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		result <- service.RunQueuedOnce(context.Background())
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("run queued once should persist timeout failure instead of returning it: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("expected runner to be cancelled by timeout_seconds")
	}
	if len(store.updatedRuns) != 2 {
		t.Fatalf("expected running and failed updates, got %+v", store.updatedRuns)
	}
	finalRun := store.updatedRuns[1]
	if finalRun.Status != RunStatusFailed || !strings.Contains(finalRun.ErrorMessage, context.DeadlineExceeded.Error()) {
		t.Fatalf("expected timeout failure to be persisted, got %+v", finalRun)
	}
}

// TestServiceStartUsesDefaultTwoWorkers 验证默认调度 worker 数为 2，避免手动刷新和补偿任务长期串行阻塞。
func TestServiceStartUsesDefaultTwoWorkers(t *testing.T) {
	store := newMemoryStore(nil)
	queue := NewExecutionQueue()
	startedRuns := make(chan string, 2)
	releaseRunner := make(chan struct{})
	service, err := NewService(Config{
		Store: store,
		Queue: queue,
		Runners: map[string]Runner{
			"test_runner": runnerFunc(func(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
				startedRuns <- run.RunKey
				select {
				case <-ctx.Done():
					return RunResult{}, ctx.Err()
				case <-releaseRunner:
					return RunResult{FetchedCount: 1, WrittenCount: 1}, nil
				}
			}),
		},
	})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	t.Cleanup(func() {
		close(releaseRunner)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = service.Shutdown(shutdownCtx)
	})

	for _, run := range []model.SchedulerRun{
		{ID: 1, RunKey: "run-1", CronType: "test_runner", DataType: "quote", ScopeKey: "CN:SH:600519", TargetDate: "2026-06-19", Status: RunStatusQueued},
		{ID: 2, RunKey: "run-2", CronType: "test_runner", DataType: "quote", ScopeKey: "CN:SH:000001", TargetDate: "2026-06-19", Status: RunStatusQueued},
	} {
		if _, err := queue.Enqueue(context.Background(), run); err != nil {
			t.Fatalf("enqueue scheduler run: %v", err)
		}
	}

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler service: %v", err)
	}

	seen := map[string]struct{}{}
	deadline := time.After(200 * time.Millisecond)
	for len(seen) < 2 {
		select {
		case runKey := <-startedRuns:
			seen[runKey] = struct{}{}
		case <-deadline:
			t.Fatalf("expected default two workers to start two runs concurrently, got %+v", seen)
		}
	}
}

// openQuoteJob 返回交易日 09:30 执行的 A 股行情刷新任务。
func openQuoteJob() model.SchedulerJob {
	return model.SchedulerJob{
		ID:             42,
		Name:           "A 股开盘行情刷新",
		CronType:       CronTypeCNAShareQuoteRefresh,
		CronExpr:       "30 9 * * 1-5",
		Enabled:        true,
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TradeWindow:    TradeWindowTradingTime,
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		CatchupEnabled: true,
		CatchupMaxDays: 5,
		TimeoutSeconds: 120,
	}
}

// mustShanghaiLocation 加载测试使用的 Asia/Shanghai 时区。
func mustShanghaiLocation(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai location: %v", err)
	}
	return location
}

type memoryStore struct {
	mutex            sync.Mutex
	jobs             []model.SchedulerJob
	interruptedRuns  []model.SchedulerRun
	createdRuns      []model.SchedulerRun
	updatedRuns      []model.SchedulerRun
	activeWatchlists []model.Watchlist
	duplicateRunKeys map[string]struct{}
	watermarks       map[string]model.IngestionWatermark
	runsByKey        map[string]model.SchedulerRun
	getRunByKeyCalls int
}

// newMemoryStore 创建调度服务测试使用的内存 store。
func newMemoryStore(jobs []model.SchedulerJob) *memoryStore {
	return &memoryStore{jobs: jobs}
}

// ListEnabledSchedulerJobs 返回测试预置的启用任务。
func (store *memoryStore) ListEnabledSchedulerJobs(context.Context) ([]model.SchedulerJob, error) {
	return store.jobs, nil
}

// CreateSchedulerRun 记录服务创建的执行记录，并模拟 run_key 唯一冲突。
func (store *memoryStore) CreateSchedulerRun(_ context.Context, run *model.SchedulerRun) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, ok := store.duplicateRunKeys[run.RunKey]; ok {
		return ErrDuplicateRunKey
	}
	store.createdRuns = append(store.createdRuns, *run)
	return nil
}

// GetSchedulerRunByRunKey 按 run_key 返回测试预置执行记录。
func (store *memoryStore) GetSchedulerRunByRunKey(_ context.Context, runKey string) (model.SchedulerRun, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.getRunByKeyCalls++
	run, ok := store.runsByKey[runKey]
	if !ok {
		return model.SchedulerRun{}, false, nil
	}
	return run, true, nil
}

// ListSchedulerRunsByStatuses 返回测试预置的异常退出遗留记录。
func (store *memoryStore) ListSchedulerRunsByStatuses(context.Context, []string) ([]model.SchedulerRun, error) {
	return store.interruptedRuns, nil
}

// UpdateSchedulerRun 在当前测试中不需要更新执行记录。
func (store *memoryStore) UpdateSchedulerRun(_ context.Context, run *model.SchedulerRun) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.updatedRuns = append(store.updatedRuns, *run)
	return nil
}

// UpsertIngestionWatermark 在当前测试中不需要更新抓取水位。
func (store *memoryStore) UpsertIngestionWatermark(context.Context, *model.IngestionWatermark) error {
	return nil
}

// GetIngestionWatermark 在当前测试中模拟没有历史水位。
func (store *memoryStore) GetIngestionWatermark(_ context.Context, dataType string, scopeKey string, provider string, period string) (model.IngestionWatermark, bool, error) {
	watermark, ok := store.watermarks[dataType+"|"+scopeKey+"|"+provider+"|"+period]
	if ok {
		return watermark, true, nil
	}
	return model.IngestionWatermark{}, false, nil
}

// ListActiveWatchlists 返回测试预置的 active watchlist，用于市场范围 catchup 展开。
func (store *memoryStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return store.activeWatchlists, nil
}

var _ Store = (*memoryStore)(nil)

type runnerFunc func(context.Context, model.SchedulerRun) (RunResult, error)

// Run 执行测试 runner 函数。
func (runner runnerFunc) Run(ctx context.Context, run model.SchedulerRun) (RunResult, error) {
	return runner(ctx, run)
}

// TestErrDuplicateRunKeyIsComparable 验证重复 run_key 错误可被 errors.Is 识别。
func TestErrDuplicateRunKeyIsComparable(t *testing.T) {
	if !errors.Is(ErrDuplicateRunKey, ErrDuplicateRunKey) {
		t.Fatal("expected ErrDuplicateRunKey to be comparable")
	}
}
