package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions"
	analysisaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/analysis"
	updatecheckaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/updatecheck"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	analysisservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/analysis"
	logexportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/sidecar"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
)

const version = "0.1.0"

// main 启动 Go sidecar，完成本地监听、stdin token 握手和 ready JSON 输出。
func main() {
	host := flag.String("host", "127.0.0.1", "local listen host")
	port := flag.String("port", "0", "local listen port")
	workspace := flag.String("workspace", "", "absolute app data workspace path")
	flag.Parse()

	listener, err := server.Listen(*host, *port)
	if err != nil {
		slog.Error("启动本地 HTTP server 失败", "error", err)
		os.Exit(1)
	}
	defer listener.Close()

	_, listenPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		slog.Error("解析本地监听端口失败", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// runtime token 只能通过 stdin 握手进入 Go core，不能写入 argv、env、日志或配置。
	handshake, err := sidecar.ReadHandshake(ctx, os.Stdin)
	if err != nil {
		slog.Error("sidecar stdin 握手失败", "error", "invalid_or_timeout")
		os.Exit(1)
	}

	var readyPort int
	if _, err := fmt.Sscanf(listenPort, "%d", &readyPort); err != nil {
		slog.Error("解析本地监听端口失败", "error", err)
		os.Exit(1)
	}

	dbPath, err := databasePathForWorkspace(*workspace)
	if err != nil {
		slog.Error("解析工作区数据库路径失败", "error", err)
		os.Exit(1)
	}
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()
	if err := prepareDatabaseFile(dbCtx, dbPath, time.Now()); err != nil {
		slog.Error("准备本地数据库文件失败", "error", err)
		os.Exit(1)
	}
	db, err := dao.Open(dbCtx, dao.Config{Path: dbPath})
	if err != nil {
		slog.Error("打开本地数据库失败", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("获取本地数据库句柄失败", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()
	if err := dao.Migrate(dbCtx, db); err != nil {
		slog.Error("初始化本地数据库 schema 失败", "error", err)
		os.Exit(1)
	}
	store, err := dao.NewStore(db)
	if err != nil {
		slog.Error("初始化本地数据仓库失败", "error", err)
		os.Exit(1)
	}
	if err := recoverRunningTasksOnStartup(dbCtx, store); err != nil {
		slog.Error("恢复 RUNNING 任务失败", "error", err)
		os.Exit(1)
	}
	logSource := logexportservice.NewMemorySource(slog.Default().Handler(), 500)
	slog.SetDefault(slog.New(logSource))

	if err := json.NewEncoder(os.Stdout).Encode(sidecar.ReadyMessage{
		Status:          "ready",
		Port:            readyPort,
		PID:             os.Getpid(),
		ProtocolVersion: sidecar.ProtocolVersion,
	}); err != nil {
		slog.Error("输出 ready JSON 失败", "error", err)
		os.Exit(1)
	}

	shutdownRequested := make(chan struct{}, 1)
	handler := actions.NewHandler(buildActionsConfig(handshake.Token, store, logSource, func() {
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	}))

	if err := server.Serve(listener, handler, shutdownRequested); err != nil {
		slog.Error("本地 HTTP server 异常退出", "error", err)
		os.Exit(1)
	}
}

// buildActionsConfig 组装生产 actions 依赖，避免 main 漏注入后端能力。
func buildActionsConfig(token string, store *dao.Store, logSource *logexportservice.MemorySource, onShutdown func()) actions.Config {
	return actions.Config{
		Version:             version,
		Token:               token,
		DBStatus:            "ok",
		Ready:               true,
		MarketProvider:      marketservice.UnconfiguredProvider{},
		NewsProvider:        newsservice.UnconfiguredProvider{},
		CacheStatsProvider:  store,
		CacheCleaner:        store,
		StockStore:          store,
		MarketStore:         store,
		NewsStore:           store,
		WatchlistStore:      store,
		PromptTemplateStore: store,
		AIConfigStore:       store,
		AIConfigTester:      aiservice.OpenAIConfigTester{},
		AnalysisStore:       store,
		AnalysisExecutor:    analysisservice.Executor{Store: store},
		AnalysisTransact: func(ctx context.Context, run func(analysisaction.Store) error) error {
			return store.WithTransaction(ctx, func(tx *dao.Store) error {
				return run(tx)
			})
		},
		TaskStore:             store,
		ReportStore:           store,
		DashboardStore:        store,
		SettingsStore:         store,
		LogExportSource:       logSource,
		UpdateManifestFetcher: updatecheckaction.HTTPFetcher{},
		OnShutdown: func() {
			if onShutdown == nil {
				return
			}
			onShutdown()
		},
	}
}

// databasePathForWorkspace 从 Rust 传入的 app data 目录派生 SQLite 文件路径。
func databasePathForWorkspace(workspace string) (string, error) {
	trimmed := strings.TrimSpace(workspace)
	if trimmed == "" {
		return "", fmt.Errorf("workspace path is required")
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("workspace path must be absolute")
	}
	return filepath.Join(trimmed, "invest-compass.sqlite3"), nil
}

// prepareDatabaseFile 在打开数据库前创建目录并备份已有库，避免迁移破坏用户数据后无恢复点。
func prepareDatabaseFile(ctx context.Context, dbPath string, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return fmt.Errorf("create workspace database dir: %w", err)
	}
	_, err := dao.BackupBeforeMigration(ctx, dao.BackupConfig{
		Path:      dbPath,
		BackupDir: filepath.Join(filepath.Dir(dbPath), "backups"),
		Now:       now,
	})
	if err != nil {
		return fmt.Errorf("backup sqlite before migration: %w", err)
	}
	return nil
}

// recoverRunningTasksOnStartup 将上次异常退出遗留的 RUNNING 任务恢复为终态。
func recoverRunningTasksOnStartup(ctx context.Context, store *dao.Store) error {
	runningTasks, err := store.ListTasksByStatus(ctx, string(taskservice.StatusRunning))
	if err != nil {
		return err
	}
	if len(runningTasks) == 0 {
		return nil
	}

	return store.WithTransaction(ctx, func(tx *dao.Store) error {
		for _, runningTask := range runningTasks {
			recoveredTask, recoveryEvent := taskservice.RecoverRunningTask(taskFromModel(runningTask))
			modelTask := taskToModel(recoveredTask)
			if err := tx.SaveTask(ctx, &modelTask); err != nil {
				return err
			}
			if recoveryEvent.Type != "" {
				modelEvent := taskEventToModel(recoveryEvent)
				if err := tx.AppendTaskEvent(ctx, &modelEvent); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// taskFromModel 转换数据库任务模型为 service 状态机模型。
func taskFromModel(item model.Task) taskservice.Task {
	return taskservice.Task{
		ID:         item.ID,
		Type:       taskservice.Type(item.Type),
		Status:     taskservice.Status(item.Status),
		Title:      item.Title,
		Progress:   item.Progress,
		Error:      item.ErrorMessage,
		StartedAt:  item.StartedAt,
		FinishedAt: item.FinishedAt,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

// taskToModel 转换 service 任务模型为数据库模型。
func taskToModel(item taskservice.Task) model.Task {
	return model.Task{
		ID:           item.ID,
		Type:         string(item.Type),
		Status:       string(item.Status),
		Title:        item.Title,
		Progress:     item.Progress,
		ErrorMessage: item.Error,
		StartedAt:    item.StartedAt,
		FinishedAt:   item.FinishedAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

// taskEventToModel 转换 service 任务事件为数据库模型。
func taskEventToModel(event taskservice.Event) model.TaskEvent {
	return model.TaskEvent{
		ID:        event.ID,
		TaskID:    event.TaskID,
		EventType: string(event.Type),
		Payload:   event.Payload,
		CreatedAt: event.CreatedAt,
		UpdatedAt: event.UpdatedAt,
	}
}
