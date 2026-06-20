package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Config 描述调度服务启动所需依赖。
type Config struct {
	Store       Store
	Queue       Queue
	Scheduler   gocron.Scheduler
	Runners     map[string]Runner
	Now         func() time.Time
	Location    *time.Location
	WorkerCount int
}

const defaultWorkerCount = 2

// Service 负责恢复持久化任务、注册 gocron 触发器并把执行记录送入队列。
type Service struct {
	store       Store
	queue       Queue
	scheduler   gocron.Scheduler
	runners     map[string]Runner
	now         func() time.Time
	location    *time.Location
	workerCount int
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewService 创建调度服务，并为 gocron 设置全局并发上限和单任务串行执行模式。
func NewService(config Config) (*Service, error) {
	if config.Store == nil {
		return nil, fmt.Errorf("scheduler store is required")
	}
	workerCount := config.WorkerCount
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}
	queue := config.Queue
	if queue == nil {
		queue = NewExecutionQueue()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	location := config.Location
	if location == nil {
		var err error
		location, err = time.LoadLocation("Asia/Shanghai")
		if err != nil {
			return nil, fmt.Errorf("load scheduler default timezone: %w", err)
		}
	}
	cronScheduler := config.Scheduler
	if cronScheduler == nil {
		var err error
		cronScheduler, err = gocron.NewScheduler(
			gocron.WithLocation(location),
			gocron.WithLimitConcurrentJobs(uint(workerCount), gocron.LimitModeReschedule),
			gocron.WithGlobalJobOptions(gocron.WithSingletonMode(gocron.LimitModeReschedule)),
		)
		if err != nil {
			return nil, fmt.Errorf("create gocron scheduler: %w", err)
		}
	}
	return &Service{
		store:       config.Store,
		queue:       queue,
		scheduler:   cronScheduler,
		runners:     config.Runners,
		now:         now,
		location:    location,
		workerCount: workerCount,
	}, nil
}

// Restore 恢复已启用调度任务，并补偿当天启动前已错过的交易窗口。
func (service *Service) Restore(ctx context.Context) error {
	if err := service.restoreInterruptedRuns(ctx); err != nil {
		return err
	}
	jobs, err := service.store.ListEnabledSchedulerJobs(ctx)
	if err != nil {
		return fmt.Errorf("list enabled scheduler jobs: %w", err)
	}
	for _, job := range jobs {
		if err := service.scheduleJob(ctx, job); err != nil {
			return err
		}
		if err := service.enqueueMissedToday(ctx, job); err != nil {
			return err
		}
		if err := service.enqueueCatchupGaps(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

// Start 完成恢复后启动 gocron 调度器。
func (service *Service) Start(ctx context.Context) error {
	if err := service.Restore(ctx); err != nil {
		return err
	}
	service.startWorker()
	service.scheduler.Start()
	return nil
}

// Shutdown 停止 gocron 调度器并释放后台资源。
func (service *Service) Shutdown(ctx context.Context) error {
	if service.cancel != nil {
		service.cancel()
	}
	if service.done != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-service.done:
		}
	}
	return service.scheduler.ShutdownWithContext(ctx)
}

// ReloadJob 重新加载单个长期任务的 gocron entry；禁用或删除时只移除旧 entry。
func (service *Service) ReloadJob(ctx context.Context, job model.SchedulerJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	service.scheduler.RemoveByTags(schedulerJobTag(job.ID))
	if !job.Enabled {
		return nil
	}
	return service.scheduleJob(ctx, job)
}

// RunQueuedOnce 从执行队列取出一条记录并同步执行，主要供 worker 和测试复用。
func (service *Service) RunQueuedOnce(ctx context.Context) error {
	run, err := service.queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	defer service.queue.Complete(run)
	startedAt := time.Now().UTC()
	run.Status = RunStatusRunning
	run.StartedAt = &startedAt
	if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
		return fmt.Errorf("mark scheduler run running: %w", err)
	}

	runner, ok := service.runners[run.CronType]
	if !ok {
		finishedAt := time.Now().UTC()
		run.Status = RunStatusSkipped
		run.SkippedReason = "scheduler_runner_not_registered"
		run.FinishedAt = &finishedAt
		return service.store.UpdateSchedulerRun(ctx, &run)
	}
	runCtx, cancelRun := contextWithRunTimeout(ctx, run)
	defer cancelRun()
	result, err := runner.Run(runCtx, run)
	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	run.FetchedCount = result.FetchedCount
	run.WrittenCount = result.WrittenCount
	run.SkippedReason = result.SkippedReason
	if err != nil {
		run.Status = RunStatusFailed
		run.ErrorMessage = logger.RedactError(err)
		return service.store.UpdateSchedulerRun(ctx, &run)
	}
	if run.SkippedReason != "" {
		run.Status = RunStatusSkipped
		return service.store.UpdateSchedulerRun(ctx, &run)
	}
	run.Status = RunStatusSuccess
	return service.store.UpdateSchedulerRun(ctx, &run)
}

// RunNow 根据长期任务配置创建一次用户手动触发的执行记录，并复用调度执行队列。
func (service *Service) RunNow(ctx context.Context, job model.SchedulerJob, request RunNowRequest) (model.SchedulerRun, error) {
	if err := ctx.Err(); err != nil {
		return model.SchedulerRun{}, err
	}
	targetDate := strings.TrimSpace(request.TargetDate)
	if strings.TrimSpace(targetDate) == "" {
		targetDate = service.now().In(service.location).Format("2006-01-02")
	}
	requestedAt := request.RequestedAt
	if requestedAt.IsZero() {
		requestedAt = service.now()
	}
	if !request.IgnoreTradeWindow {
		allowed, err := scheduledRunAllowed(job, requestedAt)
		if err != nil {
			return model.SchedulerRun{}, err
		}
		if !allowed {
			return model.SchedulerRun{}, ErrTradeWindowClosed
		}
	}
	scopeKey := scopeKeyFromJob(job)
	run := model.SchedulerRun{
		JobID:       job.ID,
		CronType:    job.CronType,
		DataType:    DataTypeForCronType(job.CronType),
		Period:      periodFromJob(job),
		ParamsJSON:  paramsWithTimeout(job.ParamsJSON, job.TimeoutSeconds),
		RunKey:      manualRunKey(job.CronType, scopeKey, targetDate, requestedAt),
		TriggerType: TriggerUserRequest,
		Status:      RunStatusQueued,
		Priority:    priorityForTrigger(TriggerUserRequest),
		Source:      "user_request",
		TargetDate:  targetDate,
		ScopeKey:    scopeKey,
	}
	if err := service.store.CreateSchedulerRun(ctx, &run); err != nil {
		return model.SchedulerRun{}, fmt.Errorf("create scheduler run now: %w", err)
	}
	added, err := service.queue.Enqueue(ctx, run)
	if err != nil {
		return model.SchedulerRun{}, fmt.Errorf("enqueue scheduler run now: %w", err)
	}
	if !added {
		run.Status = RunStatusSkipped
		run.SkippedReason = SkippedReasonDuplicateActiveScope
		if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
			return model.SchedulerRun{}, fmt.Errorf("mark duplicate scheduler run now skipped: %w", err)
		}
	}
	return run, nil
}

// Backfill 根据长期任务配置和日期范围创建手动补偿执行记录，并复用调度执行队列。
func (service *Service) Backfill(ctx context.Context, job model.SchedulerJob, request BackfillRequest) ([]model.SchedulerRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dates, err := backfillDates(job, request)
	if err != nil {
		return nil, err
	}
	symbols := normalizedSymbols(request.Symbols)
	if len(symbols) > 30 {
		return nil, ErrInvalidBackfill
	}
	scopeKey := scopeKeyFromJob(job)
	if len(symbols) > 0 {
		scopeKey = strings.Join(symbols, ",")
	}
	runs := make([]model.SchedulerRun, 0, len(dates))
	for _, targetDate := range dates {
		runKey := backfillRunKey(job.ID, scopeKey, targetDate)
		existingRun, exists, err := service.store.GetSchedulerRunByRunKey(ctx, runKey)
		if err != nil {
			return nil, fmt.Errorf("get scheduler backfill run by run_key: %w", err)
		}
		if exists {
			runs = append(runs, existingRun)
			continue
		}
		run := model.SchedulerRun{
			JobID:       job.ID,
			CronType:    job.CronType,
			DataType:    DataTypeForCronType(job.CronType),
			Period:      periodFromJob(job),
			ParamsJSON:  paramsWithTimeout(job.ParamsJSON, job.TimeoutSeconds),
			RunKey:      runKey,
			TriggerType: TriggerCatchupGap,
			Status:      RunStatusQueued,
			Priority:    priorityForTrigger(TriggerCatchupGap),
			Source:      "manual_backfill",
			TargetDate:  targetDate,
			ScopeKey:    scopeKey,
		}
		if err := service.store.CreateSchedulerRun(ctx, &run); err != nil {
			if isDuplicateRunKeyError(err) {
				existingRun, exists, lookupErr := service.store.GetSchedulerRunByRunKey(ctx, runKey)
				if lookupErr != nil {
					return nil, fmt.Errorf("get duplicate scheduler backfill run: %w", lookupErr)
				}
				if exists {
					runs = append(runs, existingRun)
					continue
				}
			}
			return nil, fmt.Errorf("create scheduler backfill run: %w", err)
		}
		added, err := service.queue.Enqueue(ctx, run)
		if err != nil {
			return nil, fmt.Errorf("enqueue scheduler backfill run: %w", err)
		}
		if !added {
			run.Status = RunStatusSkipped
			run.SkippedReason = SkippedReasonDuplicateActiveScope
			if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
				return nil, fmt.Errorf("mark duplicate scheduler backfill skipped: %w", err)
			}
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// startWorker 按配置启动后台 worker，重复调用不会启动多组 goroutine。
func (service *Service) startWorker() {
	if service.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.cancel = cancel
	service.done = make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(service.workerCount)
	for index := 0; index < service.workerCount; index++ {
		go func() {
			defer waitGroup.Done()
			for {
				if err := service.RunQueuedOnce(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
				}
			}
		}()
	}
	go func() {
		defer close(service.done)
		waitGroup.Wait()
	}()
}

// restoreInterruptedRuns 将上次退出前未完成的执行记录落为终态；是否需要补抓由后续水位和 missed_today 重新规划。
func (service *Service) restoreInterruptedRuns(ctx context.Context) error {
	runs, err := service.store.ListSchedulerRunsByStatuses(ctx, []string{RunStatusQueued, RunStatusRunning})
	if err != nil {
		return fmt.Errorf("list interrupted scheduler runs: %w", err)
	}
	for _, run := range runs {
		finishedAt := service.now().UTC()
		run.FinishedAt = &finishedAt
		switch run.Status {
		case RunStatusQueued:
			run.Status = RunStatusCancelled
			run.SkippedReason = "sidecar restarted before queued run started"
		case RunStatusRunning:
			run.Status = RunStatusFailed
			run.ErrorMessage = "sidecar restarted before scheduler run finished"
		default:
			continue
		}
		if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
			return fmt.Errorf("finalize interrupted scheduler run: %w", err)
		}
	}
	return nil
}

// scheduleJob 将持久化任务注册到 gocron，实际触发时只创建执行记录并入队。
func (service *Service) scheduleJob(ctx context.Context, job model.SchedulerJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	jobCopy := job
	_, err := service.scheduler.NewJob(
		gocron.CronJob(job.CronExpr, false),
		gocron.NewTask(func(jobCtx context.Context) {
			_ = service.enqueueJobRun(jobCtx, jobCopy, TriggerScheduled, service.now())
		}),
		gocron.WithName(job.Name),
		gocron.WithTags(schedulerJobTag(job.ID)),
	)
	if err != nil {
		return fmt.Errorf("register scheduler job %d: %w", job.ID, err)
	}
	return nil
}

// enqueueMissedToday 在交易日窗口已经过去且应用刚启动时创建当天补偿执行记录。
func (service *Service) enqueueMissedToday(ctx context.Context, job model.SchedulerJob) error {
	now := service.now()
	ok, scheduledAt, err := service.shouldMissToday(job, now)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return service.enqueueJobRun(ctx, job, TriggerMissedToday, scheduledAt)
}

// enqueueCatchupGaps 根据抓取水位补偿今天之前的交易日缺口；今天的过期窗口由 missed_today 处理。
func (service *Service) enqueueCatchupGaps(ctx context.Context, job model.SchedulerJob) error {
	if !job.Enabled || !job.CatchupEnabled || job.CatchupMaxDays <= 0 {
		return nil
	}
	provider := providerFromJob(job)
	if provider == "" {
		return nil
	}
	calendar, err := NewTradingCalendar(job.Market, job.Timezone)
	if err != nil {
		return err
	}
	dataType := DataTypeForCronType(job.CronType)
	period := periodFromJob(job)
	scopeKeys, err := service.catchupScopeKeys(ctx, job, dataType)
	if err != nil {
		return err
	}
	for _, scopeKey := range scopeKeys {
		watermark, ok, err := service.store.GetIngestionWatermark(ctx, dataType, scopeKey, provider, period)
		if err != nil {
			return fmt.Errorf("get ingestion watermark: %w", err)
		}
		if !ok || strings.TrimSpace(watermark.LastTradeDate) == "" {
			continue
		}
		lastTradeDate, err := time.ParseInLocation("2006-01-02", watermark.LastTradeDate, calendar.location)
		if err != nil {
			return fmt.Errorf("parse ingestion watermark trade date: %w", err)
		}
		today := dateOnly(service.now().In(calendar.location), calendar.location)
		enqueued := 0
		for nextDate := lastTradeDate.AddDate(0, 0, 1); nextDate.Before(today) && enqueued < job.CatchupMaxDays; nextDate = nextDate.AddDate(0, 0, 1) {
			if calendar.PhaseAt(nextDate) == PhaseNonTradingDay {
				continue
			}
			if err := service.enqueueCatchupRun(ctx, job, scopeKey, nextDate); err != nil {
				return err
			}
			enqueued++
		}
	}
	return nil
}

// enqueueJobRun 持久化执行记录并送入执行队列，重复 run_key 按幂等跳过处理。
func (service *Service) enqueueJobRun(ctx context.Context, job model.SchedulerJob, triggerType string, triggerTime time.Time) error {
	targetDate := triggerTime.In(service.location).Format("2006-01-02")
	runKey := schedulerRunKey(job.ID, targetDate, triggerType, triggerTime.In(service.location))
	if _, ok, err := service.store.GetSchedulerRunByRunKey(ctx, runKey); err != nil {
		return fmt.Errorf("get scheduler run by run_key: %w", err)
	} else if ok {
		return nil
	}
	status := RunStatusQueued
	skippedReason := ""
	if triggerType == TriggerScheduled {
		allowed, err := scheduledRunAllowed(job, triggerTime)
		if err != nil {
			return err
		}
		if !allowed {
			status = RunStatusSkipped
			skippedReason = SkippedReasonTradeWindowClosed
		}
	}
	run := model.SchedulerRun{
		JobID:         job.ID,
		CronType:      job.CronType,
		DataType:      DataTypeForCronType(job.CronType),
		Period:        periodFromJob(job),
		ParamsJSON:    paramsWithTimeout(job.ParamsJSON, job.TimeoutSeconds),
		RunKey:        runKey,
		TriggerType:   triggerType,
		Status:        status,
		Priority:      priorityForTrigger(triggerType),
		Source:        "scheduler",
		TargetDate:    targetDate,
		ScopeKey:      scopeKeyFromJob(job),
		SkippedReason: skippedReason,
	}
	if triggerType == TriggerMissedToday {
		run.Source = "startup_restore"
	}
	if err := service.store.CreateSchedulerRun(ctx, &run); err != nil {
		if isDuplicateRunKeyError(err) {
			return nil
		}
		return fmt.Errorf("create scheduler run: %w", err)
	}
	if run.Status == RunStatusSkipped {
		return nil
	}
	added, err := service.queue.Enqueue(ctx, run)
	if err != nil {
		return fmt.Errorf("enqueue scheduler run: %w", err)
	}
	if !added {
		run.Status = RunStatusSkipped
		run.SkippedReason = SkippedReasonDuplicateActiveScope
		if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
			return fmt.Errorf("mark duplicate scheduler run skipped: %w", err)
		}
	}
	return nil
}

// enqueueCatchupRun 按具体数据范围创建历史补偿 run，避免市场级任务和 symbol 水位错位。
func (service *Service) enqueueCatchupRun(ctx context.Context, job model.SchedulerJob, scopeKey string, triggerTime time.Time) error {
	targetDate := triggerTime.In(service.location).Format("2006-01-02")
	runKey := backfillRunKey(job.ID, scopeKey, targetDate)
	if _, ok, err := service.store.GetSchedulerRunByRunKey(ctx, runKey); err != nil {
		return fmt.Errorf("get scheduler catchup run by run_key: %w", err)
	} else if ok {
		return nil
	}
	run := model.SchedulerRun{
		JobID:       job.ID,
		CronType:    job.CronType,
		DataType:    DataTypeForCronType(job.CronType),
		Period:      periodFromJob(job),
		ParamsJSON:  paramsWithTimeout(job.ParamsJSON, job.TimeoutSeconds),
		RunKey:      runKey,
		TriggerType: TriggerCatchupGap,
		Status:      RunStatusQueued,
		Priority:    priorityForTrigger(TriggerCatchupGap),
		Source:      "scheduler",
		TargetDate:  targetDate,
		ScopeKey:    scopeKey,
	}
	if err := service.store.CreateSchedulerRun(ctx, &run); err != nil {
		if isDuplicateRunKeyError(err) {
			return nil
		}
		return fmt.Errorf("create scheduler catchup run: %w", err)
	}
	added, err := service.queue.Enqueue(ctx, run)
	if err != nil {
		return fmt.Errorf("enqueue scheduler catchup run: %w", err)
	}
	if !added {
		run.Status = RunStatusSkipped
		run.SkippedReason = SkippedReasonDuplicateActiveScope
		if err := service.store.UpdateSchedulerRun(ctx, &run); err != nil {
			return fmt.Errorf("mark duplicate scheduler catchup skipped: %w", err)
		}
	}
	return nil
}

// scheduledRunAllowed 判断计划触发是否满足任务配置的交易窗口。
func scheduledRunAllowed(job model.SchedulerJob, triggerTime time.Time) (bool, error) {
	calendar, err := NewTradingCalendar(job.Market, job.Timezone)
	if err != nil {
		return false, err
	}
	return calendar.AllowsWindow(triggerTime, job.TradeWindow), nil
}

// DataTypeForCronType 将任务类型映射成队列和水位使用的数据类型。
func DataTypeForCronType(cronType string) string {
	switch cronType {
	case CronTypeCNAShareQuoteRefresh:
		return "quote"
	case CronTypeCNAShareKlineRefresh:
		return "kline"
	case CronTypeMarketNewsRefresh, CronTypeSymbolNewsRefresh:
		return "news"
	default:
		return cronType
	}
}

// periodFromJob 从 params_json 中提取周期字段，只有 K 线类任务需要。
func periodFromJob(job model.SchedulerJob) string {
	var payload struct {
		Period string `json:"period"`
	}
	if err := json.Unmarshal([]byte(job.ParamsJSON), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Period)
}

// shouldMissToday 判断当前启动时间是否已经错过当天固定交易窗口。
func (service *Service) shouldMissToday(job model.SchedulerJob, now time.Time) (bool, time.Time, error) {
	if !job.Enabled || !job.CatchupEnabled {
		return false, time.Time{}, nil
	}
	calendar, err := NewTradingCalendar(job.Market, job.Timezone)
	if err != nil {
		return false, time.Time{}, err
	}
	missed, scheduledAt, err := calendar.MissedToday(job.CronExpr, now)
	if err != nil || !missed {
		return missed, scheduledAt, err
	}
	if !calendar.AllowsWindow(scheduledAt, job.TradeWindow) {
		return false, time.Time{}, nil
	}
	return true, scheduledAt, nil
}

// schedulerRunKey 生成调度执行去重键；scheduled 使用触发时间槽，启动补偿继续按天幂等。
func schedulerRunKey(jobID int64, targetDate string, triggerType string, triggerTime time.Time) string {
	if triggerType == TriggerScheduled {
		return fmt.Sprintf("scheduler_job:%d:%s:%s:%s", jobID, targetDate, triggerTime.Format("150405"), triggerType)
	}
	return fmt.Sprintf("scheduler_job:%d:%s:%s", jobID, targetDate, triggerType)
}

// backfillRunKey 生成手动补偿执行去重键，同一任务同一天允许按不同 scope 分别补偿。
func backfillRunKey(jobID int64, scopeKey string, targetDate string) string {
	return fmt.Sprintf("scheduler_job:%d:%s:%s:%s", jobID, scopeKey, targetDate, TriggerCatchupGap)
}

// schedulerJobTag 返回 gocron entry 使用的任务级 tag，供重载时精确移除旧 entry。
func schedulerJobTag(jobID int64) string {
	return fmt.Sprintf("scheduler-job-%d", jobID)
}

// manualRunKey 为用户手动触发生成唯一执行键，避免和定时补偿键冲突。
func manualRunKey(cronType string, scopeKey string, targetDate string, now time.Time) string {
	return fmt.Sprintf("user_request:%s:%s:%s:%d", cronType, scopeKey, targetDate, now.UTC().UnixNano())
}

// priorityForTrigger 返回不同触发来源的队列优先级。
func priorityForTrigger(triggerType string) int {
	switch triggerType {
	case TriggerUserRequest:
		return 100
	case TriggerMissedToday:
		return 80
	case TriggerCatchupGap:
		return 60
	default:
		return 50
	}
}

// contextWithRunTimeout 根据执行记录中的 timeout_seconds 创建 runner 子 context。
func contextWithRunTimeout(ctx context.Context, run model.SchedulerRun) (context.Context, context.CancelFunc) {
	timeoutSeconds := timeoutSecondsFromRun(run)
	if timeoutSeconds <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
}

// timeoutSecondsFromRun 从 params_json 读取执行超时；缺失或非法时表示不额外限制。
func timeoutSecondsFromRun(run model.SchedulerRun) int {
	var payload struct {
		TimeoutSeconds int `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(run.ParamsJSON), &payload); err != nil {
		return 0
	}
	if payload.TimeoutSeconds <= 0 {
		return 0
	}
	return payload.TimeoutSeconds
}

// paramsWithTimeout 在不新增 scheduler_runs 字段的前提下，把 job 超时配置随 run 参数持久化。
func paramsWithTimeout(paramsJSON string, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		return paramsJSON
	}
	payload := map[string]any{}
	if strings.TrimSpace(paramsJSON) != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &payload); err != nil {
			return paramsJSON
		}
	}
	payload["timeout_seconds"] = timeoutSeconds
	encoded, err := json.Marshal(payload)
	if err != nil {
		return paramsJSON
	}
	return string(encoded)
}

// scopeKeyFromJob 从任务范围配置中提取执行记录的可读范围标识。
func scopeKeyFromJob(job model.SchedulerJob) string {
	var payload struct {
		Symbols []string `json:"symbols"`
		Market  string   `json:"market"`
	}
	if err := json.Unmarshal([]byte(job.ScopeJSON), &payload); err == nil {
		if len(payload.Symbols) > 0 {
			return strings.Join(payload.Symbols, ",")
		}
		if strings.TrimSpace(payload.Market) != "" {
			return payload.Market
		}
	}
	if strings.TrimSpace(job.Market) != "" {
		return job.Market
	}
	return job.CronType
}

// catchupScopeKeys 返回历史补偿要查询的实际数据范围；quote/kline 任务按逐 symbol 水位展开。
func (service *Service) catchupScopeKeys(ctx context.Context, job model.SchedulerJob, dataType string) ([]string, error) {
	scopeKey := scopeKeyFromJob(job)
	if dataType != "quote" && dataType != "kline" {
		return []string{scopeKey}, nil
	}
	var payload struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(job.ScopeJSON), &payload); err == nil && len(payload.Symbols) > 0 {
		scopeKeys := make([]string, 0, len(payload.Symbols))
		seen := map[string]struct{}{}
		for _, rawSymbol := range payload.Symbols {
			symbol, err := stockservice.ParseSymbol(rawSymbol)
			if err != nil || symbol.Market != "CN" {
				continue
			}
			symbolKey := symbol.String()
			if _, ok := seen[symbolKey]; ok {
				continue
			}
			seen[symbolKey] = struct{}{}
			scopeKeys = append(scopeKeys, symbolKey)
		}
		if len(scopeKeys) > 0 {
			return scopeKeys, nil
		}
	}
	if !strings.EqualFold(scopeKey, "CN") {
		return []string{scopeKey}, nil
	}
	watchlists, err := service.store.ListActiveWatchlists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active watchlists for scheduler catchup: %w", err)
	}
	scopeKeys := make([]string, 0, len(watchlists))
	seen := map[string]struct{}{}
	for _, item := range watchlists {
		symbol, err := stockservice.ParseSymbol(item.Symbol)
		if err != nil || symbol.Market != "CN" {
			continue
		}
		symbolKey := symbol.String()
		if _, ok := seen[symbolKey]; ok {
			continue
		}
		seen[symbolKey] = struct{}{}
		scopeKeys = append(scopeKeys, symbolKey)
	}
	if len(scopeKeys) == 0 {
		return []string{scopeKey}, nil
	}
	return scopeKeys, nil
}

// normalizedSymbols 返回去空白后的 symbol 列表。
func normalizedSymbols(rawSymbols []string) []string {
	symbols := make([]string, 0, len(rawSymbols))
	for _, rawSymbol := range rawSymbols {
		trimmed := strings.TrimSpace(rawSymbol)
		if trimmed != "" {
			symbols = append(symbols, trimmed)
		}
	}
	return symbols
}

// providerFromJob 从 params_json 读取调度水位使用的 Provider 标识；缺失时不做历史水位补偿。
func providerFromJob(job model.SchedulerJob) string {
	var payload struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(job.ParamsJSON), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Provider)
}

// dateOnly 返回指定时区里的日期零点。
func dateOnly(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

// backfillDates 校验并展开手动补偿日期范围，只返回交易日。
func backfillDates(job model.SchedulerJob, request BackfillRequest) ([]string, error) {
	calendar, err := NewTradingCalendar(job.Market, job.Timezone)
	if err != nil {
		return nil, err
	}
	startDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(request.DateFrom), calendar.location)
	if err != nil {
		return nil, ErrInvalidBackfill
	}
	endDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(request.DateTo), calendar.location)
	if err != nil || endDate.Before(startDate) {
		return nil, ErrInvalidBackfill
	}
	if int(endDate.Sub(startDate).Hours()/24)+1 > 30 {
		return nil, ErrInvalidBackfill
	}
	dates := make([]string, 0, 30)
	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		if calendar.PhaseAt(current) == PhaseNonTradingDay {
			continue
		}
		dates = append(dates, current.Format("2006-01-02"))
	}
	if len(dates) == 0 {
		return nil, ErrInvalidBackfill
	}
	return dates, nil
}

// latestScheduledAtTodayFromCron 返回指定时间当天最近一次已经到达的五段 cron 计划时间。
func latestScheduledAtTodayFromCron(cronExpr string, localNow time.Time, location *time.Location) (time.Time, bool) {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return time.Time{}, false
	}
	if !cronAllowsWeekday(cronExpr, localNow.Weekday()) {
		return time.Time{}, false
	}
	minutes, ok := cronFieldEntries(fields[0], 0, 59)
	if !ok {
		return time.Time{}, false
	}
	hours, ok := cronFieldEntries(fields[1], 0, 23)
	if !ok {
		return time.Time{}, false
	}
	for hourIndex := len(hours) - 1; hourIndex >= 0; hourIndex-- {
		hour := hours[hourIndex]
		if hour > localNow.Hour() {
			continue
		}
		for minuteIndex := len(minutes) - 1; minuteIndex >= 0; minuteIndex-- {
			minute := minutes[minuteIndex]
			if hour == localNow.Hour() && minute > localNow.Minute() {
				continue
			}
			scheduledAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, location)
			if !scheduledAt.After(localNow) {
				return scheduledAt, true
			}
		}
	}
	return time.Time{}, false
}

// cronAllowsWeekday 判断五段 cron 表达式的星期字段是否包含指定星期。
func cronAllowsWeekday(cronExpr string, weekday time.Weekday) bool {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return false
	}
	return cronWeekdayFieldAllows(fields[4], int(weekday))
}

// cronWeekdayFieldAllows 判断简单 cron 星期字段是否允许指定星期，支持星号、逗号、单值、范围和步长。
func cronWeekdayFieldAllows(field string, weekday int) bool {
	entries, ok := cronFieldEntries(field, 0, 7)
	if !ok {
		return false
	}
	for _, value := range entries {
		if value == weekday || value == 7 && weekday == 0 {
			return true
		}
	}
	return false
}

// cronFieldEntries 展开简单 cron 字段，返回升序去重后的可执行值。
func cronFieldEntries(field string, min int, max int) ([]int, bool) {
	entries := make(map[int]struct{})
	for _, rawPart := range strings.Split(field, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return nil, false
		}
		step := 1
		if strings.Contains(part, "/") {
			pieces := strings.SplitN(part, "/", 2)
			part = strings.TrimSpace(pieces[0])
			parsedStep, err := strconv.Atoi(strings.TrimSpace(pieces[1]))
			if err != nil || parsedStep <= 0 {
				return nil, false
			}
			step = parsedStep
		}
		start, end, ok := cronFieldRange(part, min, max, step > 1)
		if !ok || start > end {
			return nil, false
		}
		for value := start; value <= end; value += step {
			entries[value] = struct{}{}
		}
	}
	expanded := make([]int, 0, len(entries))
	for value := min; value <= max; value++ {
		if _, ok := entries[value]; ok {
			expanded = append(expanded, value)
		}
	}
	return expanded, len(expanded) > 0
}

// cronFieldRange 解析单个 cron 字段片段的数值范围。
func cronFieldRange(part string, min int, max int, stepped bool) (int, int, bool) {
	switch {
	case part == "*" || part == "?":
		return min, max, true
	case strings.Contains(part, "-"):
		bounds := strings.SplitN(part, "-", 2)
		start, startErr := strconv.Atoi(strings.TrimSpace(bounds[0]))
		end, endErr := strconv.Atoi(strings.TrimSpace(bounds[1]))
		if startErr != nil || endErr != nil || start < min || end > max {
			return 0, 0, false
		}
		return start, end, true
	default:
		value, err := strconv.Atoi(part)
		if err != nil || value < min || value > max {
			return 0, 0, false
		}
		if stepped {
			return value, max, true
		}
		return value, value, true
	}
}

// isDuplicateRunKeyError 判断底层数据库或测试 store 是否返回 run_key 唯一冲突。
func isDuplicateRunKeyError(err error) bool {
	if err == nil {
		return false
	}
	if err == ErrDuplicateRunKey {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicated key") ||
		strings.Contains(message, "duplicate")
}
