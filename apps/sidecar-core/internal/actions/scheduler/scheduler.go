package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
)

// Store 是 scheduler action 依赖的数据访问边界。
type Store interface {
	SaveSchedulerJob(ctx context.Context, job *model.SchedulerJob) error
	ListSchedulerJobs(ctx context.Context) ([]model.SchedulerJob, error)
	GetSchedulerJob(ctx context.Context, id int64) (model.SchedulerJob, bool, error)
	SetSchedulerJobEnabled(ctx context.Context, id int64, enabled bool) error
	SoftDeleteSchedulerJob(ctx context.Context, id int64) error
	CreateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error
	GetSchedulerRunByRunKey(ctx context.Context, runKey string) (model.SchedulerRun, bool, error)
	UpdateSchedulerRun(ctx context.Context, run *model.SchedulerRun) error
	ListSchedulerRuns(ctx context.Context, jobID int64, limit int) ([]model.SchedulerRun, error)
	GetSchedulerRun(ctx context.Context, id int64) (model.SchedulerRun, bool, error)
	ListSchedulerRunsByStatuses(ctx context.Context, statuses []string) ([]model.SchedulerRun, error)
}

// Service 是 action 委托长期任务立即执行和手动补偿编排的 service 边界。
type Service interface {
	RunNow(ctx context.Context, job model.SchedulerJob, request schedulerservice.RunNowRequest) (model.SchedulerRun, error)
	Backfill(ctx context.Context, job model.SchedulerJob, request schedulerservice.BackfillRequest) ([]model.SchedulerRun, error)
	ReloadJob(ctx context.Context, job model.SchedulerJob) error
}

// Config 是调度任务管理 API 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
	Queue    schedulerservice.Queue
	Service  Service
}

// Routes 返回 scheduler action 拥有的本地 HTTP 路由。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/scheduler/job-types", Handler: jobTypes(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/list", Handler: listJobs(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/get", Handler: getJob(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/save", Handler: saveJob(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/set-enabled", Handler: setJobEnabled(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/delete", Handler: deleteJob(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/run-now", Handler: runNow(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/jobs/backfill", Handler: backfill(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/runs/list", Handler: listRuns(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/runs/get", Handler: getRun(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/runs/trigger", Handler: triggerRun(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/refresh-symbol", Handler: refreshSymbol(config)},
		{Method: http.MethodPost, Path: "/api/scheduler/status", Handler: status(config)},
	}
}

// jobTypes 返回当前 Go core 支持创建的调度任务类型元数据。
func jobTypes(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		httpx.WriteOK(response, map[string]any{"items": schedulerservice.DefaultJobRegistry().Types()}, requestContext)
	}
}

// listJobs 返回桌面管理页可展示的调度任务配置。
func listJobs(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		jobs, err := config.Store.ListSchedulerJobs(request.Context())
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_list_jobs_failed", requestContext)
			return
		}
		items := make([]jobData, 0, len(jobs))
		for _, job := range jobs {
			items = append(items, jobToData(job))
		}
		httpx.WriteOK(response, map[string]any{"items": items}, requestContext)
	}
}

// getJob 返回单个调度任务配置。
func getJob(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload idRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_job", requestContext)
			return
		}
		job, ok, err := config.Store.GetSchedulerJob(request.Context(), payload.ID)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_get_job_failed", requestContext)
			return
		}
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40404, "scheduler_job_not_found", requestContext)
			return
		}
		httpx.WriteOK(response, jobToData(job), requestContext)
	}
}

// saveJob 创建或更新调度任务配置。
func saveJob(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload saveJobRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		job := payload.toModel()
		if err := schedulerservice.DefaultJobRegistry().ValidateJob(job); err != nil {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_job", requestContext)
			return
		}
		if err := config.Store.SaveSchedulerJob(request.Context(), &job); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_save_job_failed", requestContext)
			return
		}
		if config.Service != nil {
			if err := config.Service.ReloadJob(request.Context(), job); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_reload_job_failed", requestContext)
				return
			}
		}
		httpx.WriteOK(response, jobToData(job), requestContext)
	}
}

// setJobEnabled 切换调度任务启用状态。
func setJobEnabled(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload idEnabledRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_job", requestContext)
			return
		}
		if err := config.Store.SetSchedulerJobEnabled(request.Context(), payload.ID, payload.Enabled); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_set_enabled_failed", requestContext)
			return
		}
		if config.Service != nil {
			job, ok, err := config.Store.GetSchedulerJob(request.Context(), payload.ID)
			if err != nil || !ok {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_reload_job_failed", requestContext)
				return
			}
			if err := config.Service.ReloadJob(request.Context(), job); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_reload_job_failed", requestContext)
				return
			}
		}
		httpx.WriteOK(response, map[string]any{"id": payload.ID, "enabled": payload.Enabled}, requestContext)
	}
}

// deleteJob 软删除调度任务配置。
func deleteJob(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload idRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_job", requestContext)
			return
		}
		if err := config.Store.SoftDeleteSchedulerJob(request.Context(), payload.ID); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_delete_job_failed", requestContext)
			return
		}
		if config.Service != nil {
			if err := config.Service.ReloadJob(request.Context(), model.SchedulerJob{ID: payload.ID, Enabled: false}); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_reload_job_failed", requestContext)
				return
			}
		}
		httpx.WriteOK(response, map[string]any{"id": payload.ID}, requestContext)
	}
}

// runNow 立即触发一次长期 job，对外表现为 user_request run。
func runNow(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload runNowRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		job, ok := findSchedulerJob(response, request, config, requestContext, payload.ID)
		if !ok {
			return
		}
		targetDate := strings.TrimSpace(payload.TargetDate)
		if targetDate == "" {
			targetDate = time.Now().Format("2006-01-02")
		}
		requestedAt := time.Now()
		if config.Service != nil {
			run, err := config.Service.RunNow(request.Context(), job, schedulerservice.RunNowRequest{
				TargetDate:        targetDate,
				RequestedAt:       requestedAt,
				IgnoreTradeWindow: payload.IgnoreTradeWindow,
			})
			if err != nil {
				if errors.Is(err, schedulerservice.ErrTradeWindowClosed) {
					httpx.WriteError(response, http.StatusBadRequest, 40002, "scheduler_trade_window_closed", requestContext)
					return
				}
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_run_now_failed", requestContext)
				return
			}
			httpx.WriteOK(response, runToData(run), requestContext)
			return
		}
		if config.Queue == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50300, "scheduler_queue_unavailable", requestContext)
			return
		}
		run := runFromJob(job, schedulerservice.TriggerUserRequest, targetDate, "user_request", 100)
		run.RunKey = manualRunKey(job.CronType, run.ScopeKey, targetDate, requestedAt)
		if err := config.Store.CreateSchedulerRun(request.Context(), &run); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_run_now_failed", requestContext)
			return
		}
		if err := enqueueCreatedRun(request.Context(), config, &run); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_enqueue_run_failed", requestContext)
			return
		}
		httpx.WriteOK(response, runToData(run), requestContext)
	}
}

// backfill 按日期范围创建手动补偿 run，供桌面端补历史缺口。
func backfill(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload backfillRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		job, ok := findSchedulerJob(response, request, config, requestContext, payload.ID)
		if !ok {
			return
		}
		if config.Service != nil {
			runs, err := config.Service.Backfill(request.Context(), job, schedulerservice.BackfillRequest{
				DateFrom: payload.DateFrom,
				DateTo:   payload.DateTo,
				Symbols:  payload.Symbols,
			})
			if err != nil {
				if errors.Is(err, schedulerservice.ErrInvalidBackfill) {
					httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_backfill", requestContext)
					return
				}
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_backfill_failed", requestContext)
				return
			}
			items := make([]runData, 0, len(runs))
			for _, run := range runs {
				items = append(items, runToData(run))
			}
			httpx.WriteOK(response, map[string]any{"items": items}, requestContext)
			return
		}
		dates, ok := backfillDates(job, payload)
		if !ok {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_backfill", requestContext)
			return
		}
		symbols := normalizedSymbols(payload.Symbols)
		if len(symbols) > 30 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_backfill", requestContext)
			return
		}
		if config.Queue == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50300, "scheduler_queue_unavailable", requestContext)
			return
		}
		items := make([]runData, 0, len(dates))
		for _, targetDate := range dates {
			run := runFromJob(job, schedulerservice.TriggerCatchupGap, targetDate, "manual_backfill", 60)
			if len(symbols) > 0 {
				run.ScopeKey = strings.Join(symbols, ",")
			}
			run.RunKey = backfillRunKey(job.ID, run.ScopeKey, targetDate)
			existingRun, exists, err := config.Store.GetSchedulerRunByRunKey(request.Context(), run.RunKey)
			if err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_backfill_failed", requestContext)
				return
			}
			if exists {
				items = append(items, runToData(existingRun))
				continue
			}
			if err := config.Store.CreateSchedulerRun(request.Context(), &run); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_backfill_failed", requestContext)
				return
			}
			if err := enqueueCreatedRun(request.Context(), config, &run); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_enqueue_run_failed", requestContext)
				return
			}
			items = append(items, runToData(run))
		}
		httpx.WriteOK(response, map[string]any{"items": items}, requestContext)
	}
}

// listRuns 返回调度执行记录，供桌面管理页展示历史和队列状态。
func listRuns(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload listRunsRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		runs, err := config.Store.ListSchedulerRuns(request.Context(), payload.JobID, payload.Limit)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_list_runs_failed", requestContext)
			return
		}
		items := make([]runData, 0, len(runs))
		for _, run := range runs {
			items = append(items, runToData(run))
		}
		httpx.WriteOK(response, map[string]any{"items": items}, requestContext)
	}
}

// getRun 返回单条调度执行记录。
func getRun(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload idRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_run", requestContext)
			return
		}
		run, ok, err := config.Store.GetSchedulerRun(request.Context(), payload.ID)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_get_run_failed", requestContext)
			return
		}
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40404, "scheduler_run_not_found", requestContext)
			return
		}
		httpx.WriteOK(response, runToData(run), requestContext)
	}
}

// status 返回调度系统的管理页摘要状态。
func status(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		jobs, err := config.Store.ListSchedulerJobs(request.Context())
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_status_failed", requestContext)
			return
		}
		runs, err := config.Store.ListSchedulerRunsByStatuses(request.Context(), []string{
			schedulerservice.RunStatusQueued,
			schedulerservice.RunStatusRunning,
			schedulerservice.RunStatusFailed,
		})
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_status_failed", requestContext)
			return
		}
		summary := statusData{JobsTotal: len(jobs)}
		for _, job := range jobs {
			if job.Enabled {
				summary.JobsEnabled++
			}
		}
		for _, run := range runs {
			switch run.Status {
			case schedulerservice.RunStatusQueued:
				summary.QueuedRuns++
			case schedulerservice.RunStatusRunning:
				summary.RunningRuns++
			case schedulerservice.RunStatusFailed:
				summary.FailedRuns++
			}
		}
		httpx.WriteOK(response, summary, requestContext)
	}
}

// triggerRun 处理用户手动触发的数据抓取请求，并借用调度执行队列异步执行。
func triggerRun(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload triggerRunRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		cronType := strings.TrimSpace(payload.CronType)
		scopeKey := strings.TrimSpace(payload.ScopeKey)
		if cronType == "" || scopeKey == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_run", requestContext)
			return
		}
		if !schedulerservice.DefaultJobRegistry().IsSupportedCronType(cronType) {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "unsupported_scheduler_cron_type", requestContext)
			return
		}
		targetDate := strings.TrimSpace(payload.TargetDate)
		if targetDate == "" {
			targetDate = time.Now().Format("2006-01-02")
		}
		run := model.SchedulerRun{
			JobID:       payload.JobID,
			CronType:    cronType,
			DataType:    schedulerservice.DataTypeForCronType(cronType),
			RunKey:      manualRunKey(cronType, scopeKey, targetDate, time.Now()),
			TriggerType: schedulerservice.TriggerUserRequest,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    100,
			Source:      "user_request",
			TargetDate:  targetDate,
			ScopeKey:    scopeKey,
		}
		if config.Queue == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50300, "scheduler_queue_unavailable", requestContext)
			return
		}
		if err := config.Store.CreateSchedulerRun(request.Context(), &run); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_trigger_run_failed", requestContext)
			return
		}
		if err := enqueueCreatedRun(request.Context(), config, &run); err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_enqueue_run_failed", requestContext)
			return
		}
		httpx.WriteOK(response, runToData(run), requestContext)
	}
}

// refreshSymbol 处理单股手动刷新入口，按 data_type 拆分为调度执行记录并复用统一队列。
func refreshSymbol(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestContext := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, requestContext) {
			return
		}
		var payload refreshSymbolRequest
		if !httpx.DecodeJSON(response, request, requestContext, &payload) {
			return
		}
		runs, ok := buildRefreshSymbolRuns(payload, time.Now())
		if !ok {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_refresh_symbol", requestContext)
			return
		}
		if config.Queue == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50300, "scheduler_queue_unavailable", requestContext)
			return
		}
		items := make([]runData, 0, len(runs))
		for index := range runs {
			run := runs[index]
			existingRun, exists, err := findActiveRunForScope(request.Context(), config, run)
			if err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_refresh_symbol_failed", requestContext)
				return
			}
			if exists {
				items = append(items, runToData(existingRun))
				continue
			}
			if err := config.Store.CreateSchedulerRun(request.Context(), &run); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_refresh_symbol_failed", requestContext)
				return
			}
			if err := enqueueCreatedRun(request.Context(), config, &run); err != nil {
				httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_enqueue_run_failed", requestContext)
				return
			}
			items = append(items, runToData(run))
		}
		httpx.WriteOK(response, map[string]any{"items": items}, requestContext)
	}
}

// findActiveRunForScope 查找同一数据范围内仍在排队或运行的 run，用于手动刷新幂等返回。
func findActiveRunForScope(ctx context.Context, config Config, candidate model.SchedulerRun) (model.SchedulerRun, bool, error) {
	runs, err := config.Store.ListSchedulerRunsByStatuses(ctx, []string{schedulerservice.RunStatusQueued, schedulerservice.RunStatusRunning})
	if err != nil {
		return model.SchedulerRun{}, false, err
	}
	for _, run := range runs {
		if run.CronType == candidate.CronType &&
			run.DataType == candidate.DataType &&
			run.ScopeKey == candidate.ScopeKey &&
			run.Period == candidate.Period &&
			run.TargetDate == candidate.TargetDate {
			return run, true, nil
		}
	}
	return model.SchedulerRun{}, false, nil
}

// enqueueCreatedRun 把已持久化的执行记录送入队列；若被范围去重拦下，则立即标记为 skipped。
func enqueueCreatedRun(ctx context.Context, config Config, run *model.SchedulerRun) error {
	added, err := config.Queue.Enqueue(ctx, *run)
	if err != nil {
		return err
	}
	if added {
		return nil
	}
	run.Status = schedulerservice.RunStatusSkipped
	run.SkippedReason = schedulerservice.SkippedReasonDuplicateActiveScope
	return config.Store.UpdateSchedulerRun(ctx, run)
}

type saveJobRequest struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CronType       string `json:"cron_type"`
	CronExpr       string `json:"cron_expr"`
	Enabled        bool   `json:"enabled"`
	Market         string `json:"market"`
	Timezone       string `json:"timezone"`
	TradeWindow    string `json:"trade_window"`
	ScopeJSON      string `json:"scope_json"`
	ParamsJSON     string `json:"params_json"`
	CatchupEnabled bool   `json:"catchup_enabled"`
	CatchupMaxDays int    `json:"catchup_max_days"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// toModel 将 HTTP 请求转换为数据库调度任务模型。
func (request saveJobRequest) toModel() model.SchedulerJob {
	return model.SchedulerJob{
		ID:             request.ID,
		Name:           strings.TrimSpace(request.Name),
		CronType:       strings.TrimSpace(request.CronType),
		CronExpr:       strings.TrimSpace(request.CronExpr),
		Enabled:        request.Enabled,
		Market:         defaultString(request.Market, "CN"),
		Timezone:       defaultString(request.Timezone, "Asia/Shanghai"),
		TradeWindow:    strings.TrimSpace(request.TradeWindow),
		ScopeJSON:      request.ScopeJSON,
		ParamsJSON:     request.ParamsJSON,
		CatchupEnabled: request.CatchupEnabled,
		CatchupMaxDays: defaultInt(request.CatchupMaxDays, 5),
		TimeoutSeconds: defaultInt(request.TimeoutSeconds, 120),
	}
}

type idEnabledRequest struct {
	ID      int64 `json:"id"`
	Enabled bool  `json:"enabled"`
}

type idRequest struct {
	ID int64 `json:"id"`
}

type listRunsRequest struct {
	JobID int64 `json:"job_id"`
	Limit int   `json:"limit"`
}

type runNowRequest struct {
	ID                int64  `json:"id"`
	TargetDate        string `json:"target_date"`
	IgnoreTradeWindow bool   `json:"ignore_trade_window"`
}

type backfillRequest struct {
	ID       int64    `json:"id"`
	DateFrom string   `json:"dateFrom"`
	DateTo   string   `json:"dateTo"`
	Symbols  []string `json:"symbols"`
}

type triggerRunRequest struct {
	JobID      int64  `json:"job_id"`
	CronType   string `json:"cron_type"`
	ScopeKey   string `json:"scope_key"`
	TargetDate string `json:"target_date"`
}

type refreshSymbolRequest struct {
	Symbol     string `json:"symbol"`
	DataType   string `json:"data_type"`
	TargetDate string `json:"target_date"`
	Period     string `json:"period"`
	Adjust     string `json:"adjust"`
	Limit      int    `json:"limit"`
}

type jobData struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CronType       string `json:"cron_type"`
	CronExpr       string `json:"cron_expr"`
	Enabled        bool   `json:"enabled"`
	Market         string `json:"market"`
	Timezone       string `json:"timezone"`
	TradeWindow    string `json:"trade_window"`
	ScopeJSON      string `json:"scope_json"`
	ParamsJSON     string `json:"params_json"`
	CatchupEnabled bool   `json:"catchup_enabled"`
	CatchupMaxDays int    `json:"catchup_max_days"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	LastStatus     string `json:"last_status"`
	LastError      string `json:"last_error"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type runData struct {
	ID            int64  `json:"id"`
	JobID         int64  `json:"job_id"`
	CronType      string `json:"cron_type"`
	DataType      string `json:"data_type"`
	Period        string `json:"period"`
	RunKey        string `json:"run_key"`
	TriggerType   string `json:"trigger_type"`
	Status        string `json:"status"`
	Priority      int    `json:"priority"`
	Source        string `json:"source"`
	TargetDate    string `json:"target_date"`
	ScopeKey      string `json:"scope_key"`
	FetchedCount  int    `json:"fetched_count"`
	WrittenCount  int    `json:"written_count"`
	SkippedReason string `json:"skipped_reason"`
	ErrorMessage  string `json:"error_message"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type statusData struct {
	JobsTotal   int `json:"jobs_total"`
	JobsEnabled int `json:"jobs_enabled"`
	QueuedRuns  int `json:"queued_runs"`
	RunningRuns int `json:"running_runs"`
	FailedRuns  int `json:"failed_runs"`
}

// findSchedulerJob 读取并校验调度任务 ID。
func findSchedulerJob(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext, id int64) (model.SchedulerJob, bool) {
	if id <= 0 {
		httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_scheduler_job", context)
		return model.SchedulerJob{}, false
	}
	job, ok, err := config.Store.GetSchedulerJob(request.Context(), id)
	if err != nil {
		httpx.WriteError(response, http.StatusInternalServerError, 50000, "scheduler_get_job_failed", context)
		return model.SchedulerJob{}, false
	}
	if !ok {
		httpx.WriteError(response, http.StatusNotFound, 40404, "scheduler_job_not_found", context)
		return model.SchedulerJob{}, false
	}
	return job, true
}

// runFromJob 按 job 配置生成一次执行记录，调用方负责设置 run_key 和入队。
func runFromJob(job model.SchedulerJob, triggerType string, targetDate string, source string, priority int) model.SchedulerRun {
	return model.SchedulerRun{
		JobID:       job.ID,
		CronType:    job.CronType,
		DataType:    schedulerservice.DataTypeForCronType(job.CronType),
		Period:      periodFromParams(job.ParamsJSON),
		ParamsJSON:  paramsWithTimeout(job.ParamsJSON, job.TimeoutSeconds),
		TriggerType: triggerType,
		Status:      schedulerservice.RunStatusQueued,
		Priority:    priority,
		Source:      source,
		TargetDate:  targetDate,
		ScopeKey:    scopeKeyFromJob(job),
	}
}

// paramsWithTimeout 将长期 job 的 timeout 配置写入一次性 run 参数，供 worker 执行时限制时长。
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

// backfillDates 校验并展开手动补偿日期范围，只返回交易日。
func backfillDates(job model.SchedulerJob, request backfillRequest) ([]string, bool) {
	startDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(request.DateFrom), time.Local)
	if err != nil {
		return nil, false
	}
	endDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(request.DateTo), time.Local)
	if err != nil || endDate.Before(startDate) {
		return nil, false
	}
	if int(endDate.Sub(startDate).Hours()/24)+1 > 30 {
		return nil, false
	}
	calendar, err := schedulerservice.NewTradingCalendar(job.Market, job.Timezone)
	if err != nil {
		return nil, false
	}
	dates := make([]string, 0, 30)
	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		if calendar.PhaseAt(current) == schedulerservice.PhaseNonTradingDay {
			continue
		}
		dates = append(dates, current.Format("2006-01-02"))
	}
	return dates, len(dates) > 0
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

// periodFromParams 从 params_json 读取 K 线周期，非 K 线任务返回空字符串。
func periodFromParams(raw string) string {
	var payload struct {
		Period string `json:"period"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Period)
}

// scopeKeyFromJob 从任务 scope_json 生成执行范围标识。
func scopeKeyFromJob(job model.SchedulerJob) string {
	var payload struct {
		Symbols []string `json:"symbols"`
		Market  string   `json:"market"`
	}
	if err := json.Unmarshal([]byte(job.ScopeJSON), &payload); err == nil {
		symbols := normalizedSymbols(payload.Symbols)
		if len(symbols) > 0 {
			return strings.Join(symbols, ",")
		}
		if strings.TrimSpace(payload.Market) != "" {
			return strings.TrimSpace(payload.Market)
		}
	}
	if strings.TrimSpace(job.Market) != "" {
		return strings.TrimSpace(job.Market)
	}
	return job.CronType
}

// jobToData 将数据库任务模型转换为 HTTP 响应结构。
func jobToData(job model.SchedulerJob) jobData {
	return jobData{
		ID:             job.ID,
		Name:           job.Name,
		CronType:       job.CronType,
		CronExpr:       job.CronExpr,
		Enabled:        job.Enabled,
		Market:         job.Market,
		Timezone:       job.Timezone,
		TradeWindow:    job.TradeWindow,
		ScopeJSON:      job.ScopeJSON,
		ParamsJSON:     job.ParamsJSON,
		CatchupEnabled: job.CatchupEnabled,
		CatchupMaxDays: job.CatchupMaxDays,
		TimeoutSeconds: job.TimeoutSeconds,
		LastStatus:     job.LastStatus,
		LastError:      job.LastError,
		CreatedAt:      formatTime(job.CreatedAt),
		UpdatedAt:      formatTime(job.UpdatedAt),
	}
}

// runToData 将数据库执行模型转换为 HTTP 响应结构。
func runToData(run model.SchedulerRun) runData {
	return runData{
		ID:            run.ID,
		JobID:         run.JobID,
		CronType:      run.CronType,
		DataType:      run.DataType,
		Period:        run.Period,
		RunKey:        run.RunKey,
		TriggerType:   run.TriggerType,
		Status:        run.Status,
		Priority:      run.Priority,
		Source:        run.Source,
		TargetDate:    run.TargetDate,
		ScopeKey:      run.ScopeKey,
		FetchedCount:  run.FetchedCount,
		WrittenCount:  run.WrittenCount,
		SkippedReason: run.SkippedReason,
		ErrorMessage:  run.ErrorMessage,
		StartedAt:     formatOptionalTime(run.StartedAt),
		FinishedAt:    formatOptionalTime(run.FinishedAt),
		CreatedAt:     formatTime(run.CreatedAt),
		UpdatedAt:     formatTime(run.UpdatedAt),
	}
}

// manualRunKey 为用户手动触发生成唯一执行键，避免和定时补偿键冲突。
func manualRunKey(cronType string, scopeKey string, targetDate string, now time.Time) string {
	return fmt.Sprintf("user_request:%s:%s:%s:%d", cronType, scopeKey, targetDate, now.UTC().UnixNano())
}

// backfillRunKey 生成手动补偿执行去重键，同一任务同一天允许按不同 scope 分别补偿。
func backfillRunKey(jobID int64, scopeKey string, targetDate string) string {
	return fmt.Sprintf("scheduler_job:%d:%s:%s:%s", jobID, scopeKey, targetDate, schedulerservice.TriggerCatchupGap)
}

// buildRefreshSymbolRuns 把桌面单股刷新请求转换成一个或多个异步执行记录。
func buildRefreshSymbolRuns(request refreshSymbolRequest, now time.Time) ([]model.SchedulerRun, bool) {
	symbol := strings.TrimSpace(request.Symbol)
	if symbol == "" {
		return nil, false
	}
	targetDate := strings.TrimSpace(request.TargetDate)
	if targetDate == "" {
		targetDate = now.Format("2006-01-02")
	}
	dataType := strings.TrimSpace(request.DataType)
	if dataType == "" {
		dataType = "all"
	}
	parts, ok := refreshCronParts(dataType)
	if !ok {
		return nil, false
	}
	runs := make([]model.SchedulerRun, 0, len(parts))
	for _, part := range parts {
		period := ""
		params := map[string]any{}
		if part.dataType == "kline" {
			period = defaultString(request.Period, "day")
			params["period"] = period
			params["adjust"] = defaultString(request.Adjust, "none")
			params["limit"] = defaultInt(request.Limit, 120)
		}
		if part.dataType == "news" {
			params["limit"] = defaultInt(request.Limit, 20)
		}
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return nil, false
		}
		runs = append(runs, model.SchedulerRun{
			CronType:    part.cronType,
			DataType:    part.dataType,
			Period:      period,
			ParamsJSON:  string(paramsJSON),
			RunKey:      manualRunKey(part.cronType, symbol, targetDate, now),
			TriggerType: schedulerservice.TriggerUserRequest,
			Status:      schedulerservice.RunStatusQueued,
			Priority:    100,
			Source:      "user_request",
			TargetDate:  targetDate,
			ScopeKey:    symbol,
		})
	}
	return runs, true
}

type refreshCronPart struct {
	cronType string
	dataType string
}

// refreshCronParts 返回单股手动刷新需要生成的 cron_type 列表。
func refreshCronParts(dataType string) ([]refreshCronPart, bool) {
	switch dataType {
	case "quote":
		return []refreshCronPart{{cronType: schedulerservice.CronTypeCNAShareQuoteRefresh, dataType: "quote"}}, true
	case "kline":
		return []refreshCronPart{{cronType: schedulerservice.CronTypeCNAShareKlineRefresh, dataType: "kline"}}, true
	case "news":
		return []refreshCronPart{{cronType: schedulerservice.CronTypeSymbolNewsRefresh, dataType: "news"}}, true
	case "all":
		return []refreshCronPart{
			{cronType: schedulerservice.CronTypeCNAShareQuoteRefresh, dataType: "quote"},
			{cronType: schedulerservice.CronTypeCNAShareKlineRefresh, dataType: "kline"},
			{cronType: schedulerservice.CronTypeSymbolNewsRefresh, dataType: "news"},
		}, true
	default:
		return nil, false
	}
}

// defaultString 返回去空白后的输入值，空值时返回默认值。
func defaultString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

// defaultInt 返回正整数输入值，非正数时返回默认值。
func defaultInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// formatTime 使用 RFC3339Nano 输出非零时间，零值保持为空字符串。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// formatOptionalTime 输出可空时间字段，避免 HTTP 响应暴露 Go 指针语义。
func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}
