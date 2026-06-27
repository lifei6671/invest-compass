package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions"
	analysisaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/analysis"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	updatecheckaction "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/updatecheck"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	analysisservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/analysis"
	datasourcecredentialservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
	logexportservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/logexport"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	netproxyservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/netproxy"
	newsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	notificationservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/notification"
	promptservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/prompt"
	schedulerservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/scheduler"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/sidecar"
	stockseedservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stockseed"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
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
	lifecycleDiagnostic("handshake", "keepalive_stdin", handshake.KeepaliveStdin)

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
	if err := seedBuiltinStockBasics(dbCtx, store); err != nil {
		slog.Error("初始化内置股票基础资料失败", "error", err)
		os.Exit(1)
	}
	if err := seedBuiltinPromptTemplates(dbCtx, store); err != nil {
		slog.Error("初始化内置 Prompt 模板失败", "error", err)
		os.Exit(1)
	}
	if err := recoverRunningTasksOnStartup(dbCtx, store); err != nil {
		slog.Error("恢复 RUNNING 任务失败", "error", err)
		os.Exit(1)
	}
	taskLogService := tasklogservice.Service{Store: store}
	ndjsonWriter, err := tasklogservice.NewNDJSONWriter(tasklogservice.NDJSONWriterConfig{WorkspaceDir: *workspace})
	if err != nil {
		slog.Error("初始化本地任务日志文件写入器失败", "error", err)
		os.Exit(1)
	}
	taskLogAppender := tasklogservice.NewMultiAppender(
		taskLogService,
		tasklogservice.WriterAppender{Writer: ndjsonWriter},
	)
	taskLogWriter, err := tasklogservice.NewAsyncWriter(tasklogservice.AsyncWriterConfig{Appender: taskLogAppender})
	if err != nil {
		slog.Error("初始化任务日志异步写入器失败", "error", err)
		os.Exit(1)
	}
	defer shutdownTaskLogWriter(taskLogWriter)
	applyTaskLogRetention(dbCtx, store, ndjsonWriter)
	searchTokenizer, err := requiredSearchTokenizer()
	if err != nil {
		slog.Error("初始化 GSE 分词器失败", "error", logger.RedactError(err))
		os.Exit(1)
	}

	logSource := newRuntimeLogSource(os.Stderr, 500)
	slog.SetDefault(slog.New(logger.NewTaskLogHandler(logSource, taskLogWriter)))

	schedulerQueue, schedulerService, err := newProductionScheduler(store, *workspace, time.Now)
	if err != nil {
		slog.Error("初始化调度服务失败", "error", err)
		os.Exit(1)
	}
	schedulerCtx, schedulerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := schedulerService.Start(schedulerCtx); err != nil {
		schedulerCancel()
		slog.Error("启动调度服务失败", "error", err)
		os.Exit(1)
	}
	schedulerCancel()
	runtimeRefresher, err := newProductionRuntimeRefresher(store, *workspace, time.Now)
	if err != nil {
		slog.Error("初始化运行期数据刷新服务失败", "error", logger.RedactError(err))
		os.Exit(1)
	}
	stopRuntimeRefresh := runtimeRefresher.Start(context.Background())
	shutdownRequested := make(chan struct{}, 1)
	lifecycle := newShutdownLifecycle(func(ctx context.Context) error {
		stopRuntimeRefresh()
		return schedulerService.Shutdown(ctx)
	}, func() {
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	})
	defer lifecycle.stopScheduler()
	if handshake.KeepaliveStdin {
		watchParentStdinEOF(os.Stdin, func() {
			lifecycleDiagnostic("shutdown_requested", "source", "stdin_eof")
			lifecycle.requestShutdown()
		})
	}
	handler := actions.NewHandler(buildActionsConfig(handshake.Token, *workspace, store, schedulerQueue, schedulerService, taskLogService, taskLogWriter, logSource, searchTokenizer, func() {
		lifecycleDiagnostic("shutdown_requested", "source", "internal_api")
		lifecycle.requestShutdown()
	}))
	serveResult := serveCoreHTTP(listener, handler, shutdownRequested)
	select {
	case err := <-serveResult:
		lifecycleDiagnostic("serve_result_before_ready", "error", err)
		slog.Error("本地 HTTP server 启动失败", "error", err)
		os.Exit(1)
	default:
	}
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := waitForCoreHTTPReady(readyCtx, readyPort, handshake.Token); err != nil {
		readyCancel()
		lifecycleDiagnostic("shutdown_requested", "source", "ready_check_failed")
		lifecycle.requestShutdown()
		slog.Error("等待本地 HTTP server ready 失败", "error", err)
		os.Exit(1)
	}
	readyCancel()

	if err := json.NewEncoder(os.Stdout).Encode(sidecar.ReadyMessage{
		Status:          "ready",
		Port:            readyPort,
		PID:             os.Getpid(),
		ProtocolVersion: sidecar.ProtocolVersion,
	}); err != nil {
		lifecycleDiagnostic("shutdown_requested", "source", "ready_output_failed")
		lifecycle.requestShutdown()
		slog.Error("输出 ready JSON 失败", "error", err)
		os.Exit(1)
	}

	if err := <-serveResult; err != nil {
		lifecycleDiagnostic("serve_result", "error", err)
		slog.Error("本地 HTTP server 异常退出", "error", err)
		os.Exit(1)
	}
	lifecycleDiagnostic("serve_result", "error", "nil")
}

type builtinPromptStoreAdapter struct {
	store *dao.Store
}

// GetPromptTemplateByKey 按稳定 key 读取当前库中的模板，供内置模板 seed 判断是否需要更新。
func (adapter builtinPromptStoreAdapter) GetPromptTemplateByKey(ctx context.Context, key string) (promptservice.Template, bool, error) {
	return adapter.store.GetPromptTemplateByKey(ctx, key)
}

// SavePromptTemplate 只服务内置模板 seed，将 service 模型转换为 DAO 写入。
func (adapter builtinPromptStoreAdapter) SavePromptTemplate(ctx context.Context, template promptservice.Template) error {
	return adapter.store.SaveBuiltinPromptTemplate(ctx, template)
}

// seedBuiltinPromptTemplates 将打包模板写入 SQLite；内置模板只按 key/checksum 由应用升级维护。
func seedBuiltinPromptTemplates(ctx context.Context, store *dao.Store) error {
	templates, err := promptservice.LoadPackagedBuiltinPromptTemplates()
	if err != nil {
		return err
	}
	return promptservice.SeedBuiltinPromptTemplates(ctx, builtinPromptStoreAdapter{store: store}, templates)
}

// seedBuiltinStockBasics 将打包的 A 股基础资料写入 SQLite，保证首启后自选和搜索有本地股票池。
func seedBuiltinStockBasics(ctx context.Context, store *dao.Store) error {
	result, err := stockseedservice.SeedPackagedStocks(ctx, store)
	if err != nil {
		return err
	}
	slog.Info(
		"初始化内置股票基础资料完成",
		"total_rows", result.TotalRows,
		"seeded_rows", result.SeededRows,
		"skipped_unsupported_exchange", result.SkippedUnsupportedExchange,
		"skipped_invalid_rows", result.SkippedInvalidRows,
	)
	return nil
}

// serveCoreHTTP 在后台启动本地 HTTP server；调用方必须在 ready 前启动它，避免 Rust 收到 ready 后立刻请求时连接被拒绝。
func serveCoreHTTP(listener net.Listener, handler http.Handler, shutdown <-chan struct{}) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(listener, handler, shutdown)
	}()
	return result
}

// waitForCoreHTTPReady 在输出 ready JSON 前做一次本机认证健康检查，确保 Rust 收到 ready 后立即调用业务 API 不会撞上监听竞态。
func waitForCoreHTTPReady(ctx context.Context, port int, token string) error {
	client := http.Client{Timeout: 200 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/internal/health", port)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(httpx.TokenHeader, token)

		response, err := client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			if response.StatusCode == http.StatusOK && closeErr == nil {
				return nil
			}
			if closeErr != nil {
				lastErr = closeErr
			} else {
				lastErr = fmt.Errorf("health status %d", response.StatusCode)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("wait core http ready: %w", lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// watchParentStdinEOF 监听 Rust 父进程持有的 stdin 管道；父进程退出或断开时主动关闭 Go core。
func watchParentStdinEOF(reader io.Reader, requestShutdown func()) {
	go func() {
		lifecycleDiagnostic("stdin_watch_start")
		_, err := io.Copy(io.Discard, reader)
		if err != nil {
			lifecycleDiagnostic("stdin_watch_end", "error", err)
		} else {
			lifecycleDiagnostic("stdin_watch_end", "error", "eof")
		}
		requestShutdown()
	}()
}

// lifecycleDiagnostic 输出不含 token 和凭据的 sidecar 生命周期诊断，专门用于定位父子进程边界问题。
func lifecycleDiagnostic(message string, attrs ...any) {
	fields := []string{fmt.Sprintf("go_lifecycle %s", message)}
	for index := 0; index+1 < len(attrs); index += 2 {
		fields = append(fields, fmt.Sprintf("%v=%v", attrs[index], attrs[index+1]))
	}
	_, _ = fmt.Fprintln(os.Stderr, strings.Join(fields, " "))
}

// newRuntimeLogSource 构造生产运行期日志采集器。
// 下游直接写 stderr，避免第三方库使用标准 log 时再次回到 log.Logger 导致重入死锁。
func newRuntimeLogSource(output io.Writer, capacity int) *logexportservice.MemorySource {
	return logexportservice.NewMemorySource(slog.NewTextHandler(output, nil), capacity)
}

// requiredSearchTokenizer 初始化生产必需的 GSE 分词器；失败时启动应阻断，不能降级为 simple。
func requiredSearchTokenizer() (searchservice.Tokenizer, error) {
	tokenizer, err := searchservice.NewGSETokenizer(searchservice.DefaultDomainWords())
	if err != nil {
		return nil, fmt.Errorf("required gse tokenizer unavailable: %w", err)
	}
	return tokenizer, nil
}

// newProductionScheduler 组装生产调度队列和 SchedulerService，确保启动恢复、手动触发和桌面管理使用同一执行队列。
func newProductionScheduler(store *dao.Store, workspace string, now func() time.Time) (*schedulerservice.ExecutionQueue, *schedulerservice.Service, error) {
	schedulerQueue := schedulerservice.NewExecutionQueue()
	dataSourceCredentials := datasourcecredentialservice.NewService(store, datasourcecredentialservice.FileKeyProvider{Path: filepath.Join(workspace, "credentials", "data-source.key")})
	dataSourceCredentials.HTTPClient = externalDataHTTPClient(store, 15*time.Second)
	marketProvider := buildMarketProvider(store)
	newsProvider := buildNewsProvider(dataSourceCredentialCookieResolver{Service: dataSourceCredentials}, store)
	schedulerService, err := schedulerservice.NewService(schedulerservice.Config{
		Store: store,
		Queue: schedulerQueue,
		Now:   now,
		Runners: map[string]schedulerservice.Runner{
			schedulerservice.CronTypeStockProfileRefresh: schedulerservice.StockProfileRefreshRunner{
				Provider: marketProvider,
				Store:    store,
			},
			schedulerservice.CronTypeCNAShareQuoteRefresh: schedulerservice.QuoteRefreshRunner{
				Provider: marketProvider,
				Store:    store,
			},
			schedulerservice.CronTypeCNAShareKlineRefresh: schedulerservice.KlineRefreshRunner{
				Provider: marketProvider,
				Store:    store,
			},
			schedulerservice.CronTypeMarketNewsRefresh: schedulerservice.NewsRefreshRunner{
				Provider: newsProvider,
				Store:    store,
			},
			schedulerservice.CronTypeSymbolNewsRefresh: schedulerservice.NewsRefreshRunner{
				Provider: newsProvider,
				Store:    store,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return schedulerQueue, schedulerService, nil
}

// newProductionRuntimeRefresher 组装进程级实时刷新器，启动后异步刷新缓存，前端页面只读取本地库。
func newProductionRuntimeRefresher(store *dao.Store, workspace string, now func() time.Time) (*schedulerservice.RuntimeRefresher, error) {
	dataSourceCredentials := datasourcecredentialservice.NewService(store, datasourcecredentialservice.FileKeyProvider{Path: filepath.Join(workspace, "credentials", "data-source.key")})
	dataSourceCredentials.HTTPClient = externalDataHTTPClient(store, 15*time.Second)
	marketProvider := buildMarketProvider(store)
	newsProvider := buildNewsProvider(dataSourceCredentialCookieResolver{Service: dataSourceCredentials}, store)
	return schedulerservice.NewRuntimeRefresher(store, marketProvider, newsProvider, now)
}

type shutdownLifecycle struct {
	once                  sync.Once
	shutdownScheduler     func(context.Context) error
	requestServerShutdown func()
}

// newShutdownLifecycle 创建 sidecar 关闭协调器，保证 scheduler 在 HTTP server 退出前先停止。
func newShutdownLifecycle(shutdownScheduler func(context.Context) error, requestServerShutdown func()) *shutdownLifecycle {
	return &shutdownLifecycle{
		shutdownScheduler:     shutdownScheduler,
		requestServerShutdown: requestServerShutdown,
	}
}

// requestShutdown 处理 Rust 发起的关闭请求：先停后台调度，再让 HTTP server 退出。
func (lifecycle *shutdownLifecycle) requestShutdown() {
	lifecycle.stopScheduler()
	if lifecycle.requestServerShutdown != nil {
		lifecycle.requestServerShutdown()
	}
}

// stopScheduler 停止调度器并等待 worker 收尾；多次调用只执行一次。
func (lifecycle *shutdownLifecycle) stopScheduler() {
	lifecycle.once.Do(func() {
		if lifecycle.shutdownScheduler == nil {
			return
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := lifecycle.shutdownScheduler(shutdownCtx); err != nil {
			slog.Error("关闭调度服务失败", "error", err)
		}
	})
}

// shutdownTaskLogWriter 在 sidecar 退出前 flush 任务日志队列，避免运行中日志丢失。
func shutdownTaskLogWriter(writer *tasklogservice.AsyncWriter) {
	if writer == nil {
		return
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := writer.Shutdown(shutdownCtx); err != nil {
		slog.Warn("关闭任务日志写入器失败", "error", logger.RedactError(err))
	}
}

// applyTaskLogRetention 启动时执行数据库和 NDJSON 文件保留策略，防止日志无限增长。
func applyTaskLogRetention(ctx context.Context, store *dao.Store, ndjsonWriter *tasklogservice.NDJSONWriter) {
	retention, err := tasklogservice.NewRetentionManager(tasklogservice.RetentionConfig{Store: store})
	if err != nil {
		slog.Warn("初始化任务日志保留策略失败", "error", logger.RedactError(err))
		return
	}
	if _, err := retention.Apply(ctx); err != nil {
		slog.Warn("执行任务日志数据库保留策略失败", "error", logger.RedactError(err))
	}
	if ndjsonWriter == nil {
		return
	}
	if err := ndjsonWriter.ApplyRetention(ctx); err != nil {
		slog.Warn("执行任务日志文件保留策略失败", "error", logger.RedactError(err))
	}
}

// buildActionsConfig 组装生产 actions 依赖，避免 main 漏注入后端能力。
func buildActionsConfig(token string, workspace string, store *dao.Store, schedulerQueue *schedulerservice.ExecutionQueue, schedulerService *schedulerservice.Service, taskLogService tasklogservice.Service, taskLogWriter tasklogservice.StageWriter, logSource *logexportservice.MemorySource, searchTokenizer searchservice.Tokenizer, onShutdown func()) actions.Config {
	dataSourceCredentialKeyPath := filepath.Join(workspace, "credentials", "data-source.key")
	notificationService := notificationservice.NewService(store)
	aiHTTPClient := externalDataHTTPClient(store, 120*time.Second)
	dataSourceCredentials := datasourcecredentialservice.NewService(store, datasourcecredentialservice.FileKeyProvider{Path: dataSourceCredentialKeyPath})
	dataSourceCredentials.HTTPClient = externalDataHTTPClient(store, 15*time.Second)
	marketProvider := buildMarketProvider(store)
	newsProvider := buildNewsProvider(dataSourceCredentialCookieResolver{Service: dataSourceCredentials}, store)
	documentSearchService := searchservice.NewScopedDocumentSearchService(searchservice.DocumentSearchConfig{Store: store, Tokenizer: searchTokenizer})
	return actions.Config{
		Version:               version,
		Token:                 token,
		DBStatus:              "ok",
		Ready:                 true,
		MarketProvider:        marketProvider,
		NewsProvider:          newsProvider,
		CacheStatsProvider:    store,
		CacheCleaner:          store,
		StockStore:            store,
		DocumentSearchStore:   store,
		DocumentSearchService: documentSearchService,
		MarketStore:           store,
		NewsStore:             store,
		WatchlistStore:        store,
		PromptTemplateStore:   store,
		AIConfigStore:         store,
		AIConfigTester:        aiservice.OpenAIConfigTester{HTTPClient: aiHTTPClient},
		ProviderNotifier:      notificationService,
		AnalysisStore:         store,
		AnalysisExecutor:      analysisservice.Executor{Store: store, MarketProvider: marketProvider, HTTPClient: aiHTTPClient, TaskLogWriter: taskLogWriter, TaskNotifier: notificationService},
		AnalysisTransact: func(ctx context.Context, run func(analysisaction.Store) error) error {
			return store.WithTransaction(ctx, func(tx *dao.Store) error {
				return run(tx)
			})
		},
		TaskStore:             store,
		TaskLogService:        taskLogService,
		ReportStore:           store,
		DashboardStore:        store,
		SettingsStore:         store,
		NotificationStore:     store,
		DataSourceCredentials: dataSourceCredentials,
		SchedulerStore:        store,
		SchedulerQueue:        schedulerQueue,
		SchedulerService:      schedulerService,
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

// buildMarketProvider 创建生产行情 Provider，初始化失败时明确回退为未配置状态。
func buildMarketProvider(store *dao.Store) marketservice.MarketProvider {
	provider, err := marketservice.NewSinaTencentProvider(marketservice.SinaTencentConfig{HTTPClient: externalDataHTTPClient(store, 15*time.Second)})
	if err != nil {
		slog.Warn("初始化行情 Provider 失败", "error", logger.RedactError(err))
		return marketservice.UnconfiguredProvider{}
	}
	return marketservice.NewSettingsBackedMarketProvider(store, provider)
}

// buildNewsProvider 创建生产资讯 Provider，财联社请求会按需读取本地加密 Cookie。
func buildNewsProvider(resolver newsservice.CookieCredentialResolver, store *dao.Store) newsservice.Provider {
	httpClient := externalDataHTTPClient(store, 15*time.Second)
	cailianpress, err := newsservice.NewCailianpressProvider(newsservice.CailianpressConfig{
		CredentialResolver: resolver,
		HTTPClient:         httpClient,
	})
	if err != nil {
		slog.Warn("初始化财联社资讯 Provider 失败", "error", logger.RedactError(err))
		return newsservice.UnconfiguredProvider{}
	}
	sinaLive, err := newsservice.NewSinaLiveProvider(newsservice.SinaLiveConfig{HTTPClient: httpClient})
	if err != nil {
		slog.Warn("初始化新浪资讯 Provider 失败", "error", logger.RedactError(err))
		return newsservice.UnconfiguredProvider{}
	}
	wallstreetcnLive, err := newsservice.NewWallstreetcnLiveProvider(newsservice.WallstreetcnLiveConfig{HTTPClient: httpClient})
	if err != nil {
		slog.Warn("初始化华尔街见闻资讯 Provider 失败", "error", logger.RedactError(err))
		return newsservice.UnconfiguredProvider{}
	}
	tradingView, err := newsservice.NewTradingViewProvider(newsservice.TradingViewConfig{HTTPClient: httpClient})
	if err != nil {
		slog.Warn("初始化 TradingView 资讯 Provider 失败", "error", logger.RedactError(err))
		return newsservice.UnconfiguredProvider{}
	}
	eastMoneyResearch, err := newsservice.NewEastMoneyResearchProvider(newsservice.EastMoneyResearchConfig{HTTPClient: httpClient})
	if err != nil {
		slog.Warn("初始化东方财富研报公告 Provider 失败", "error", logger.RedactError(err))
		return newsservice.UnconfiguredProvider{}
	}
	return newsservice.NewCompositeProvider("multi-market-news", []newsservice.Provider{
		cailianpress,
		sinaLive,
		wallstreetcnLive,
		tradingView,
		eastMoneyResearch,
	})
}

// externalDataHTTPClient 创建外部数据请求 HTTP client，每次请求按最新 settings 解析代理模式。
func externalDataHTTPClient(store *dao.Store, timeout time.Duration) *http.Client {
	return netproxyservice.DynamicClientForSettings(store, timeout)
}

// dataSourceCredentialCookieResolver 适配新闻 Provider 所需的 Cookie 凭据读取接口。
type dataSourceCredentialCookieResolver struct {
	Service datasourcecredentialservice.Service
}

// ResolveCookie 从数据源凭据 service 解密指定 Provider 的 Cookie，调用方不得记录返回值。
func (resolver dataSourceCredentialCookieResolver) ResolveCookie(ctx context.Context, providerID string) (string, error) {
	credential, err := resolver.Service.Resolve(ctx, providerID)
	if err != nil {
		return "", err
	}
	return credential.Cookie, nil
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
