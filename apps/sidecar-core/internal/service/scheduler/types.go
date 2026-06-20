package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	// CronTypeCNAShareQuoteRefresh 表示 A 股行情刷新任务。
	CronTypeCNAShareQuoteRefresh = "cn_a_share_quote_refresh"

	// TriggerScheduled 表示由 gocron 按 cron 表达式触发。
	TriggerScheduled = "scheduled"
	// TriggerMissedToday 表示应用启动时补偿当天已经错过的交易窗口。
	TriggerMissedToday = "missed_today"
	// TriggerCatchupGap 表示根据水位补偿历史缺口。
	TriggerCatchupGap = "catchup_gap"
	// TriggerUserRequest 表示用户手动触发的数据抓取。
	TriggerUserRequest = "user_request"

	// RunStatusQueued 表示执行记录已进入本地队列。
	RunStatusQueued = "queued"
	// RunStatusRunning 表示执行记录正在被 worker 处理。
	RunStatusRunning = "running"
	// RunStatusSuccess 表示执行记录已经成功完成。
	RunStatusSuccess = "success"
	// RunStatusFailed 表示执行记录执行失败。
	RunStatusFailed = "failed"
	// RunStatusSkipped 表示执行记录因去重或窗口不满足被跳过。
	RunStatusSkipped = "skipped"
	// RunStatusCancelled 表示排队中的执行记录因 sidecar 重启或用户取消而终止。
	RunStatusCancelled = "cancelled"

	// SkippedReasonDuplicateActiveScope 表示同一数据范围已有执行记录在队列中等待或运行。
	SkippedReasonDuplicateActiveScope = "scheduler_duplicate_active_scope"
	// SkippedReasonTradeWindowClosed 表示计划任务触发时不满足配置的交易窗口。
	SkippedReasonTradeWindowClosed = "scheduler_trade_window_closed"
)

// ErrDuplicateRunKey 表示 scheduler_runs.run_key 已存在，调用方应把它视为幂等跳过。
var ErrDuplicateRunKey = errors.New("scheduler duplicate run key")

// ErrInvalidBackfill 表示手动补偿请求的日期范围或 symbol 范围不合法。
var ErrInvalidBackfill = errors.New("scheduler invalid backfill request")

// ErrTradeWindowClosed 表示当前交易阶段不允许执行该调度任务。
var ErrTradeWindowClosed = errors.New("scheduler trade window closed")

// RunNowRequest 描述桌面端立即执行长期任务的请求。
type RunNowRequest struct {
	TargetDate        string
	RequestedAt       time.Time
	IgnoreTradeWindow bool
}

// BackfillRequest 描述桌面端发起的手动补偿范围。
type BackfillRequest struct {
	DateFrom string
	DateTo   string
	Symbols  []string
}

// Store 定义调度服务依赖的数据访问边界。
type Store interface {
	ListEnabledSchedulerJobs(ctx context.Context) ([]model.SchedulerJob, error)
	CreateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error
	GetSchedulerRunByRunKey(ctx context.Context, runKey string) (model.SchedulerRun, bool, error)
	ListSchedulerRunsByStatuses(ctx context.Context, statuses []string) ([]model.SchedulerRun, error)
	UpdateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	UpsertIngestionWatermark(ctx context.Context, watermark *model.IngestionWatermark) error
	GetIngestionWatermark(ctx context.Context, dataType string, scopeKey string, provider string, period string) (model.IngestionWatermark, bool, error)
}

// Queue 定义调度任务进入业务执行层前的本地队列边界。
type Queue interface {
	Enqueue(ctx context.Context, run model.SchedulerRun) (bool, error)
	Dequeue(ctx context.Context) (model.SchedulerRun, error)
	Complete(run model.SchedulerRun)
}

// Runner 定义 cron_type 对应的业务执行器。
type Runner interface {
	Run(ctx context.Context, run model.SchedulerRun) (RunResult, error)
}

// RunResult 描述一次业务抓取执行后的统计结果。
type RunResult struct {
	FetchedCount  int
	WrittenCount  int
	SkippedReason string
}
