package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
	logexportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
)

// TestValidateListenHostOnlyAllowsLoopback 验证 sidecar 只能监听本机回环地址。
func TestValidateListenHostOnlyAllowsLoopback(t *testing.T) {
	listener, err := server.Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatalf("expected 127.0.0.1 to be allowed, got %v", err)
	}
	listener.Close()
	if listener, err := server.Listen("0.0.0.0", "0"); err == nil {
		listener.Close()
		t.Fatal("expected 0.0.0.0 to be rejected")
	}
}

// TestDatabasePathForWorkspaceRequiresAbsolutePath 验证 sidecar 只能用明确的绝对工作区路径初始化本地库。
func TestDatabasePathForWorkspaceRequiresAbsolutePath(t *testing.T) {
	if _, err := databasePathForWorkspace(""); err == nil {
		t.Fatal("expected empty workspace to be rejected")
	}
	if _, err := databasePathForWorkspace("relative/workspace"); err == nil {
		t.Fatal("expected relative workspace to be rejected")
	}

	path, err := databasePathForWorkspace("/Users/demo/InvestCompass")
	if err != nil {
		t.Fatalf("expected absolute workspace to be accepted, got %v", err)
	}
	if want := filepath.Join("/Users/demo/InvestCompass", "invest-compass.sqlite3"); path != want {
		t.Fatalf("unexpected db path: got %q want %q", path, want)
	}
}

// TestPrepareDatabaseFileBacksUpExistingDatabase 验证 sidecar 启动会在迁移前备份已有用户库。
func TestPrepareDatabaseFileBacksUpExistingDatabase(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "invest-compass.sqlite3")
	if err := os.WriteFile(dbPath, []byte("existing-user-data"), 0o600); err != nil {
		t.Fatalf("write sqlite file: %v", err)
	}

	if err := prepareDatabaseFile(ctx, dbPath, time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC)); err != nil {
		t.Fatalf("prepare database file: %v", err)
	}

	backupPath := filepath.Join(tempDir, "backups", "invest-compass-20260618T080910000000000Z.sqlite3.bak")
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read migration backup: %v", err)
	}
	if string(content) != "existing-user-data" {
		t.Fatalf("unexpected migration backup content: %q", string(content))
	}
}

// TestRecoverRunningTasksOnStartupPersistsTerminalState 验证 core 启动恢复会清理悬挂 RUNNING 任务。
func TestRecoverRunningTasksOnStartupPersistsTerminalState(t *testing.T) {
	ctx := context.Background()
	db, err := dao.Open(ctx, dao.Config{Path: filepath.Join(t.TempDir(), "test.sqlite3")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dao.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	store, err := dao.NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	now := time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
	if err := store.SaveTask(ctx, &model.Task{
		ID:        "task-running",
		Type:      "ANALYSIS",
		Status:    "RUNNING",
		Title:     "恢复测试",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save running task: %v", err)
	}

	if err := recoverRunningTasksOnStartup(ctx, store); err != nil {
		t.Fatalf("recover running tasks: %v", err)
	}

	running, err := store.ListTasksByStatus(ctx, "RUNNING")
	if err != nil {
		t.Fatalf("list running tasks: %v", err)
	}
	if len(running) != 0 {
		t.Fatalf("expected no running tasks after recovery, got %+v", running)
	}

	failed, err := store.ListTasksByStatus(ctx, "FAILED")
	if err != nil {
		t.Fatalf("list failed tasks: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != "task-running" || failed[0].ErrorMessage == "" {
		t.Fatalf("expected recovered failed task, got %+v", failed)
	}

	events, err := store.ListTaskEventsAfter(ctx, "task-running", 0)
	if err != nil {
		t.Fatalf("list recovery events: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "TASK_FAILED" {
		t.Fatalf("expected recovery TASK_FAILED event, got %+v", events)
	}
}

// TestBuildActionsConfigInjectsProductionAIConfigTester 验证生产 handler 配置不会遗漏模型连通性测试能力。
func TestBuildActionsConfigInjectsProductionAIConfigTester(t *testing.T) {
	store := newMainTestStore(t)
	logSource := logexportservice.NewMemorySource(nil, 10)
	queue := schedulerservice.NewExecutionQueue()
	schedulerService, err := schedulerservice.NewService(schedulerservice.Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	taskLogService := tasklogservice.Service{Store: store}
	config := buildActionsConfig("test-token", store, queue, schedulerService, taskLogService, nil, logSource, func() {})

	if config.AIConfigTester == nil {
		t.Fatal("production actions config must inject AI config tester")
	}
	if config.AIConfigStore == nil || config.AnalysisStore == nil || config.LogExportSource == nil || config.SchedulerQueue == nil || config.SchedulerService == nil {
		t.Fatalf("production actions config missed required stores: %+v", config)
	}
}

// TestBuildActionsConfigKeepsMarketProviderUnconfiguredUntilComplianceReady 验证生产配置在授权验收前保持行情 Provider 安全未配置状态。
func TestBuildActionsConfigKeepsMarketProviderUnconfiguredUntilComplianceReady(t *testing.T) {
	store := newMainTestStore(t)
	logSource := logexportservice.NewMemorySource(nil, 10)
	queue := schedulerservice.NewExecutionQueue()
	schedulerService, err := schedulerservice.NewService(schedulerservice.Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	taskLogService := tasklogservice.Service{Store: store}
	config := buildActionsConfig("test-token", store, queue, schedulerService, taskLogService, nil, logSource, func() {})

	if config.MarketProvider == nil || config.NewsProvider == nil {
		t.Fatalf("production config must inject explicit provider implementations: %+v", config)
	}
	marketStatus := config.MarketProvider.Status(context.Background())
	if marketStatus.Available || marketStatus.Source != "unconfigured" {
		t.Fatalf("unexpected market provider status: %+v", marketStatus)
	}
	newsStatus, ok := config.NewsProvider.(interface {
		Status(context.Context) newsservice.ProviderStatus
	})
	if !ok {
		t.Fatal("production news provider must expose status")
	}
	if newsStatus.Status(context.Background()).Available {
		t.Fatal("unconfigured news provider must not report available")
	}
}

// TestNewProductionSchedulerRestoreCreatesMissedTodayRun 验证生产 scheduler 组装会在启动恢复阶段补偿当天错过的窗口。
func TestNewProductionSchedulerRestoreCreatesMissedTodayRun(t *testing.T) {
	ctx := context.Background()
	store := newMainTestStore(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	job := model.SchedulerJob{
		Name:           "A 股开盘行情刷新",
		CronType:       schedulerservice.CronTypeCNAShareQuoteRefresh,
		CronExpr:       "30 9 * * 1-5",
		Enabled:        true,
		Market:         "CN",
		Timezone:       "Asia/Shanghai",
		TradeWindow:    "trading_time",
		ScopeJSON:      `{"symbols":["CN:SH:600519"]}`,
		ParamsJSON:     `{}`,
		CatchupEnabled: true,
		CatchupMaxDays: 5,
		TimeoutSeconds: 120,
	}
	if err := store.SaveSchedulerJob(ctx, &job); err != nil {
		t.Fatalf("save scheduler job: %v", err)
	}

	queue, schedulerService, err := newProductionScheduler(store, func() time.Time {
		return time.Date(2026, 6, 19, 10, 0, 0, 0, location)
	})
	if err != nil {
		t.Fatalf("new production scheduler: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = schedulerService.Shutdown(shutdownCtx)
	})

	if err := schedulerService.Restore(ctx); err != nil {
		t.Fatalf("restore scheduler: %v", err)
	}

	runs, err := store.ListSchedulerRuns(ctx, job.ID, 10)
	if err != nil {
		t.Fatalf("list scheduler runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one missed_today run, got %+v", runs)
	}
	run := runs[0]
	if run.TriggerType != schedulerservice.TriggerMissedToday || run.Status != schedulerservice.RunStatusQueued || run.Source != "startup_restore" {
		t.Fatalf("unexpected startup compensation run: %+v", run)
	}
	if run.TargetDate != "2026-06-19" || run.ScopeKey != "CN:SH:600519" || run.CronType != schedulerservice.CronTypeCNAShareQuoteRefresh {
		t.Fatalf("unexpected missed_today scope/date/type: %+v", run)
	}
	snapshot := queue.Snapshot()
	if len(snapshot) != 1 || snapshot[0].RunKey != run.RunKey {
		t.Fatalf("expected missed_today run to enter production queue, got %+v", snapshot)
	}
}

// TestShutdownLifecycleStopsSchedulerBeforeServer 验证关闭请求会先停调度器，再通知 HTTP server 退出。
func TestShutdownLifecycleStopsSchedulerBeforeServer(t *testing.T) {
	var calls []string
	lifecycle := newShutdownLifecycle(
		func(context.Context) error {
			calls = append(calls, "scheduler")
			return nil
		},
		func() {
			calls = append(calls, "server")
		},
	)

	lifecycle.requestShutdown()
	lifecycle.stopScheduler()

	if got := strings.Join(calls, ","); got != "scheduler,server" {
		t.Fatalf("expected scheduler to stop before server shutdown and only once, got %s", got)
	}
}

// newMainTestStore 创建 main 包生产配置测试使用的 SQLite store。
func newMainTestStore(t *testing.T) *dao.Store {
	t.Helper()

	ctx := context.Background()
	db, err := dao.Open(ctx, dao.Config{Path: filepath.Join(t.TempDir(), "main-test.sqlite3")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dao.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	store, err := dao.NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	return store
}
