package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
	logexportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
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

// TestWatchParentStdinEOFRequestsShutdown 验证 Rust 父进程断开 stdin 后 Go core 会主动进入退出流程。
func TestWatchParentStdinEOFRequestsShutdown(t *testing.T) {
	shutdownRequested := make(chan struct{}, 1)
	watchParentStdinEOF(strings.NewReader(""), func() {
		shutdownRequested <- struct{}{}
	})

	select {
	case <-shutdownRequested:
	case <-time.After(time.Second):
		t.Fatal("expected stdin EOF to request shutdown")
	}
}

// TestRuntimeLogSourceDoesNotDeadlockStandardLogBridge 验证第三方库使用标准 log 输出时不会卡死启动链路。
func TestRuntimeLogSourceDoesNotDeadlockStandardLogBridge(t *testing.T) {
	previousSlog := slog.Default()
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	previousPrefix := log.Prefix()
	t.Cleanup(func() {
		slog.SetDefault(previousSlog)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	source := newRuntimeLogSource(io.Discard, 10)
	slog.SetDefault(slog.New(logger.NewTaskLogHandler(source, nil)))

	done := make(chan struct{})
	go func() {
		log.Println("gse startup dictionary loaded")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("standard log bridge should not deadlock runtime log source")
	}

	request, err := source.ExportLogRequest(context.Background())
	if err != nil {
		t.Fatalf("export runtime log request: %v", err)
	}
	if len(request.Lines) == 0 || !strings.Contains(request.Lines[0], "gse startup dictionary loaded") {
		t.Fatalf("expected standard log output in runtime log source, got %+v", request.Lines)
	}
}

// TestServeCoreHTTPAcceptsRequestsBeforeReady 验证 ready 通知前 HTTP server 已经开始接受请求。
func TestServeCoreHTTPAcceptsRequestsBeforeReady(t *testing.T) {
	listener, err := server.Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatalf("listen local server: %v", err)
	}
	shutdown := make(chan struct{}, 1)
	serveResult := serveCoreHTTP(listener, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte("ok"))
	}), shutdown)

	response, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		shutdown <- struct{}{}
		t.Fatalf("server should accept request before ready notification: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if closeErr := response.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		shutdown <- struct{}{}
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != "ok" {
		shutdown <- struct{}{}
		t.Fatalf("unexpected response body: %q", string(body))
	}

	shutdown <- struct{}{}
	if err := <-serveResult; err != nil {
		t.Fatalf("serve should stop cleanly: %v", err)
	}
}

// TestWaitForCoreHTTPReadyRequiresAuthenticatedHealthOK 验证 ready 通知前必须完成真实本机健康预检。
func TestWaitForCoreHTTPReadyRequiresAuthenticatedHealthOK(t *testing.T) {
	listener, err := server.Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatalf("listen local server: %v", err)
	}
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse listener port: %v", err)
	}

	shutdown := make(chan struct{}, 1)
	serveResult := serveCoreHTTP(listener, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/internal/health" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get(httpx.TokenHeader) != "test-token" {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = response.Write([]byte(`{"code":0}`))
	}), shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := waitForCoreHTTPReady(ctx, port, "test-token"); err != nil {
		shutdown <- struct{}{}
		t.Fatalf("wait for ready health: %v", err)
	}

	shutdown <- struct{}{}
	if err := <-serveResult; err != nil {
		t.Fatalf("serve should stop cleanly: %v", err)
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
	searchTokenizer, err := requiredSearchTokenizer()
	if err != nil {
		t.Fatalf("required search tokenizer: %v", err)
	}
	config := buildActionsConfig("test-token", t.TempDir(), store, queue, schedulerService, taskLogService, nil, logSource, searchTokenizer, func() {})

	if config.AIConfigTester == nil {
		t.Fatal("production actions config must inject AI config tester")
	}
	if config.AIConfigStore == nil || config.AnalysisStore == nil || config.LogExportSource == nil || config.SchedulerQueue == nil || config.SchedulerService == nil {
		t.Fatalf("production actions config missed required stores: %+v", config)
	}
}

// TestBuildActionsConfigInjectsRealProviders 验证生产配置注入真实行情 Provider，并让资讯 Provider 受凭据状态约束。
func TestBuildActionsConfigInjectsRealProviders(t *testing.T) {
	store := newMainTestStore(t)
	logSource := logexportservice.NewMemorySource(nil, 10)
	queue := schedulerservice.NewExecutionQueue()
	schedulerService, err := schedulerservice.NewService(schedulerservice.Config{Store: store, Queue: queue})
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	taskLogService := tasklogservice.Service{Store: store}
	searchTokenizer, err := requiredSearchTokenizer()
	if err != nil {
		t.Fatalf("required search tokenizer: %v", err)
	}
	config := buildActionsConfig("test-token", t.TempDir(), store, queue, schedulerService, taskLogService, nil, logSource, searchTokenizer, func() {})

	if config.MarketProvider == nil || config.NewsProvider == nil {
		t.Fatalf("production config must inject explicit provider implementations: %+v", config)
	}
	marketStatus := config.MarketProvider.Status(context.Background())
	if !marketStatus.Available || marketStatus.Source == "unconfigured" {
		t.Fatalf("unexpected market provider status: %+v", marketStatus)
	}
	newsStatus, ok := config.NewsProvider.(interface {
		Status(context.Context) newsservice.ProviderStatus
	})
	if !ok {
		t.Fatal("production news provider must expose status")
	}
	newsProviderStatus := newsStatus.Status(context.Background())
	if newsProviderStatus.Available || newsProviderStatus.LastError != "data_source_credential_not_configured" {
		t.Fatalf("news provider must require credential before reporting available: %+v", newsProviderStatus)
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

	queue, schedulerService, err := newProductionScheduler(store, t.TempDir(), func() time.Time {
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
