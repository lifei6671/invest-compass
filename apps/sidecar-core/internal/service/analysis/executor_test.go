package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
)

// TestExecutorCompletesTaskAndSavesReport 验证分析执行器使用真实缓存上下文调用 AI 并保存报告。
func TestExecutorCompletesTaskAndSavesReport(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{
		ID:             7,
		Provider:       aiservice.ProviderOpenAICompatible,
		BaseURL:        "https://api.example.test/v1",
		ModelName:      "gpt-analysis",
		Temperature:    0.2,
		MaxTokens:      2048,
		TimeoutSeconds: 30,
	}
	store.template = model.PromptTemplate{
		ID:      9,
		Name:    "综合分析",
		Type:    "stock_full",
		Content: "请基于 {{ quote }}、{{ kline_summary }}、{{ indicators }}、{{ news }} 输出 {{ analysis_language }} 报告。",
	}
	store.quote = model.Quote{
		Symbol:         "US:AAPL",
		Price:          210.5,
		ChangePercent:  1.25,
		Volume:         1200000,
		Amount:         252600000,
		TurnoverRate:   3.98,
		PE:             28.6,
		PB:             8.2,
		TotalMarketCap: 123456789000,
		FloatMarketCap: 98765432100,
		QuoteTime:      time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		Provider:       "test-provider",
		UpdatedAt:      time.Now().UTC(),
	}
	store.klines = makeAnalysisIndicatorKlines(defaultKlineLimit)
	store.news = []model.NewsItem{
		{Title: "新品发布", Summary: "公司发布新产品", PublishedAt: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)},
	}
	chat := &recordingChatClient{response: aiservice.ChatResponse{Content: "报告正文\n风险提示：市场波动。"}}
	taskLogWriter := &recordingTaskLogWriter{}
	taskNotifier := &recordingTaskNotifier{}
	executor := Executor{
		Store: store,
		NewChatClient: func(config aiservice.Config, resolvedAPIKey string) ChatClient {
			if resolvedAPIKey != "sk-runtime-secret" {
				t.Fatalf("unexpected resolved key: %q", resolvedAPIKey)
			}
			if config.ModelName != "gpt-analysis" {
				t.Fatalf("unexpected AI config: %+v", config)
			}
			return chat
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
		},
		TaskLogWriter: taskLogWriter,
		TaskNotifier:  taskNotifier,
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
		UserPosition:     &UserPosition{CostPrice: 123.45, Shares: 10, RiskLevel: "medium"},
	})

	if err := executor.Execute(context.Background(), "task-1", validated, "sk-runtime-secret"); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(chat.requests) != 1 {
		t.Fatalf("expected one AI request, got %d", len(chat.requests))
	}
	joinedMessages := chat.requests[0].Messages[0].Content + "\n" + chat.requests[0].Messages[1].Content
	if !strings.Contains(joinedMessages, "不构成投资建议") ||
		!strings.Contains(joinedMessages, "210.50") ||
		!strings.Contains(joinedMessages, "turnover_rate=3.98%") ||
		!strings.Contains(joinedMessages, "total_market_cap=123456789000") ||
		!strings.Contains(joinedMessages, "float_market_cap=98765432100") ||
		!strings.Contains(joinedMessages, "MA ma5=") ||
		!strings.Contains(joinedMessages, "MACD dif=") ||
		!strings.Contains(joinedMessages, "RSI rsi6=") ||
		!strings.Contains(joinedMessages, "KDJ k=") ||
		!strings.Contains(joinedMessages, "BOLL middle=") ||
		!strings.Contains(joinedMessages, "新品发布") {
		t.Fatalf("AI request missing analysis context: %#v", chat.requests[0].Messages)
	}
	if strings.Contains(joinedMessages, "用户一次性持仓输入") || strings.Contains(joinedMessages, "123.45") {
		t.Fatalf("AI request must omit user position when template does not use user_position: %#v", chat.requests[0].Messages)
	}

	taskModel := store.tasks["task-1"]
	if taskModel.Status != string(task.StatusSuccess) || taskModel.Progress != 100 || taskModel.FinishedAt.IsZero() {
		t.Fatalf("expected success task, got %+v", taskModel)
	}
	if len(store.events) < 3 ||
		store.events[0].EventType != string(task.EventStarted) ||
		store.events[len(store.events)-1].EventType != string(task.EventSuccess) {
		t.Fatalf("unexpected task events: %+v", store.events)
	}
	assertProgressEvents(t, store.events, []expectedProgressEvent{
		{stage: "quote_fetch", progress: 20, message: "行情快照已读取"},
		{stage: "kline_fetch", progress: 35, message: "K 线数据已准备"},
		{stage: "calc_macd", progress: 50, message: "技术指标已计算"},
		{stage: "prompt_build", progress: 65, message: "Prompt 已构建"},
		{stage: "stream_start", progress: 80, message: "AI 模型已响应"},
		{stage: "stream_chunk", progress: 90, message: "AI 输出已生成"},
	})
	if store.report.TaskID != "task-1" ||
		store.report.ContentMarkdown != "报告正文\n风险提示：市场波动。" ||
		!strings.Contains(store.report.InputSnapshot, "123.45") ||
		!strings.Contains(store.report.InputSnapshot, "medium") ||
		!strings.Contains(store.report.InputSnapshot, "System:") ||
		!strings.Contains(store.report.InputSnapshot, "请基于") ||
		!strings.Contains(store.report.InputSnapshot, `"model":"gpt-analysis"`) ||
		!strings.Contains(store.report.InputSnapshot, `"temperature":0.2`) ||
		!strings.Contains(store.report.InputSnapshot, `"max_tokens":2048`) ||
		!strings.Contains(store.report.InputSnapshot, `"prompt_template":"综合分析"`) ||
		strings.Contains(store.report.InputSnapshot, "sk-runtime-secret") {
		t.Fatalf("unexpected saved report: %+v", store.report)
	}
	assertTaskLogStages(t, taskLogWriter.entries, []string{
		"quote_fetch",
		"kline_fetch",
		"calc_macd",
		"prompt_build",
		"stream_start",
		"stream_chunk",
	})
	if len(taskNotifier.tasks) != 1 ||
		taskNotifier.tasks[0].ID != "task-1" ||
		taskNotifier.tasks[0].Status != string(task.StatusSuccess) {
		t.Fatalf("expected success task notification, got %+v", taskNotifier.tasks)
	}
}

// TestExecutorDefaultChatClientUsesInjectedHTTPClient 验证分析任务默认 AI client 复用运行时代理 HTTP client。
func TestExecutorDefaultChatClientUsesInjectedHTTPClient(t *testing.T) {
	var called bool
	executor := Executor{
		HTTPClient: &http.Client{Transport: analysisRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			called = true
			if request.URL.Host != "ai.example.test" {
				t.Fatalf("unexpected host: %s", request.URL.Host)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","choices":[{"message":{"content":"报告正文"}}]}`)),
			}, nil
		})},
	}
	client := executor.chatClient(aiservice.Config{
		BaseURL:        "https://ai.example.test",
		ModelName:      "gpt-analysis",
		TimeoutSeconds: 1,
	}, "sk-runtime-secret")

	response, err := client.Chat(context.Background(), aiservice.ChatRequest{
		Model:    "gpt-analysis",
		Messages: []aiservice.Message{{Role: aiservice.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if !called || response.Content != "报告正文" {
		t.Fatalf("expected injected HTTP client to be used, called=%v response=%+v", called, response)
	}
}

// TestDailyKlinesKeepsDailyBarsSeparateFromSummary 验证每日 K 线明细独立填入 Prompt，不混在 K 线摘要里。
func TestDailyKlinesKeepsDailyBarsSeparateFromSummary(t *testing.T) {
	klines := []model.Kline{
		{TradeDate: "2025-06-27", Open: 10.1, High: 10.8, Low: 9.9, Close: 10.5, Volume: 1000, Amount: 10500},
		{TradeDate: "2026-06-26", Open: 12.1, High: 12.8, Low: 11.9, Close: 12.5, Volume: 1200, Amount: 15000},
	}
	summary := klineSummary(klines)
	daily := dailyKlines(klines)

	if strings.Contains(summary, "date=2025-06-27") || strings.Contains(summary, "daily_kline") {
		t.Fatalf("kline summary must not contain daily bars: %s", summary)
	}
	for _, expected := range []string{
		"date=2025-06-27 open=10.10 high=10.80 low=9.90 close=10.50 volume=1000 amount=10500",
		"date=2026-06-26 open=12.10 high=12.80 low=11.90 close=12.50 volume=1200 amount=15000",
	} {
		if !strings.Contains(daily, expected) {
			t.Fatalf("daily klines missing %q: %s", expected, daily)
		}
	}
}

// TestDefaultKlineLimitUsesRecentSixtyDailyBars 验证分析任务默认只拉取最近 60 日 K 数据。
func TestDefaultKlineLimitUsesRecentSixtyDailyBars(t *testing.T) {
	if defaultKlineLimit != 60 {
		t.Fatalf("defaultKlineLimit must be 60 daily bars, got %d", defaultKlineLimit)
	}
}

// TestIndicatorSummaryIncludesCommonTechnicalIndicators 验证个股综合分析会把常用技术指标填入 AI 输入。
func TestIndicatorSummaryIncludesCommonTechnicalIndicators(t *testing.T) {
	summary := indicatorSummary(makeAnalysisIndicatorKlines(defaultKlineLimit))

	for _, expected := range []string{
		"MA ma5=",
		"ma10=",
		"ma20=",
		"ma60=",
		"EMA ema12=",
		"ema26=",
		"MACD dif=",
		"dea=",
		"bar=",
		"RSI rsi6=",
		"rsi12=",
		"rsi24=",
		"KDJ k=",
		"d=",
		"j=",
		"BOLL middle=",
		"upper=",
		"lower=",
		"volatility=",
		"max_drawdown=",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("indicator summary missing %q: %s", expected, summary)
		}
	}
}

// TestExecutorFetchesMissingKlinesFromMarketProvider 验证分析任务不会要求前端先打开 K 线页预热缓存。
func TestExecutorFetchesMissingKlinesFromMarketProvider(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{
		ID:      9,
		Name:    "综合分析",
		Type:    "stock_full",
		Content: "请基于 {{ quote }}、{{ kline_summary }}、{{ indicators }} 输出报告。",
	}
	store.quote = model.Quote{
		Symbol:    "CN:SH:600522",
		Price:     12.3,
		QuoteTime: time.Date(2026, 6, 26, 15, 0, 0, 0, time.UTC),
		Provider:  "cache-provider",
		UpdatedAt: time.Now().UTC(),
	}
	provider := &recordingMarketProvider{
		klines: []marketservice.KlineBar{
			{
				Symbol:    mustParseStockSymbol(t, "CN:SH:600522"),
				Period:    marketservice.PeriodDay,
				Adjust:    marketservice.AdjustNone,
				TradeDate: "2026-06-25",
				Open:      11.8,
				High:      12.5,
				Low:       11.6,
				Close:     12.3,
				Volume:    1000,
				Amount:    12300,
				Provider:  "provider-kline",
			},
		},
	}
	executor := Executor{
		Store:          store,
		MarketProvider: provider,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			return &recordingChatClient{response: aiservice.ChatResponse{Content: "报告正文"}}
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 26, 15, 30, 0, 0, time.UTC)
		},
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "CN:SH:600522",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
	})

	if err := executor.Execute(context.Background(), "task-fetch-klines", validated, "sk-runtime-secret"); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if provider.klineCalls != 1 {
		t.Fatalf("expected one provider kline call, got %d", provider.klineCalls)
	}
	if len(store.klines) != 1 || store.klines[0].Symbol != "CN:SH:600522" || store.klines[0].Provider != "provider-kline" {
		t.Fatalf("expected provider klines cached, got %+v", store.klines)
	}
	if store.tasks["task-fetch-klines"].Status != string(task.StatusSuccess) {
		t.Fatalf("expected success task, got %+v", store.tasks["task-fetch-klines"])
	}
}

// TestExecutorMarksTaskFailedWhenRequiredDataMissing 验证缺少真实行情上下文时任务失败而不是伪造数据。
func TestExecutorMarksTaskFailedWhenRequiredDataMissing(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{ID: 9, Name: "综合分析", Type: "stock_full", Content: "请分析 {{ quote }}"}
	taskNotifier := &recordingTaskNotifier{}
	executor := Executor{
		Store: store,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			t.Fatal("missing quote must not call AI")
			return nil
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
		},
		TaskNotifier: taskNotifier,
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
	})

	if err := executor.Execute(context.Background(), "task-2", validated, "sk-runtime-secret"); err == nil {
		t.Fatal("expected missing data error")
	}

	taskModel := store.tasks["task-2"]
	if taskModel.Status != string(task.StatusFailed) || taskModel.ErrorMessage == "" || taskModel.FinishedAt.IsZero() {
		t.Fatalf("expected failed task, got %+v", taskModel)
	}
	lastEvent := store.events[len(store.events)-1]
	if lastEvent.EventType != string(task.EventFailed) || strings.Contains(lastEvent.Payload, "sk-runtime-secret") {
		t.Fatalf("unexpected failed event: %+v", lastEvent)
	}
	if store.report.TaskID != "" {
		t.Fatalf("failed task must not save report: %+v", store.report)
	}
	if len(taskNotifier.tasks) != 1 ||
		taskNotifier.tasks[0].ID != "task-2" ||
		taskNotifier.tasks[0].Status != string(task.StatusFailed) {
		t.Fatalf("expected failed task notification, got %+v", taskNotifier.tasks)
	}
}

// TestExecutorKeepsSuccessWhenNotificationFails 验证通知写入失败不会反向破坏已完成的分析任务。
func TestExecutorKeepsSuccessWhenNotificationFails(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{
		ID:      9,
		Name:    "综合分析",
		Type:    "stock_full",
		Content: "请基于 {{ quote }}、{{ kline_summary }}、{{ indicators }} 输出报告。",
	}
	store.quote = model.Quote{
		Symbol:    "US:AAPL",
		Price:     210.5,
		QuoteTime: time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		Provider:  "test-provider",
		UpdatedAt: time.Now().UTC(),
	}
	store.klines = []model.Kline{
		{Symbol: "US:AAPL", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 210, High: 212, Low: 199, Volume: 1200},
	}
	taskNotifier := &recordingTaskNotifier{err: errors.New("notification unavailable")}
	executor := Executor{
		Store: store,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			return &recordingChatClient{response: aiservice.ChatResponse{Content: "报告正文"}}
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
		},
		TaskNotifier: taskNotifier,
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
	})

	if err := executor.Execute(context.Background(), "task-notify-failure", validated, "sk-runtime-secret"); err != nil {
		t.Fatalf("notification failure must not fail analysis execution: %v", err)
	}
	taskModel := store.tasks["task-notify-failure"]
	if taskModel.Status != string(task.StatusSuccess) || store.report.TaskID != "task-notify-failure" {
		t.Fatalf("expected completed task and report, got task=%+v report=%+v", taskModel, store.report)
	}
}

// TestExecutorDoesNotOverwriteCancelledTaskAsFailed 验证取消错误不会追加 FAILED 事件覆盖取消状态。
func TestExecutorDoesNotOverwriteCancelledTaskAsFailed(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{
		ID:      9,
		Name:    "综合分析",
		Type:    "stock_full",
		Content: "请基于 {{ quote }}、{{ kline_summary }}、{{ indicators }} 输出报告。",
	}
	store.quote = model.Quote{
		Symbol:    "US:AAPL",
		Price:     210.5,
		QuoteTime: time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		Provider:  "test-provider",
		UpdatedAt: time.Now().UTC(),
	}
	store.klines = []model.Kline{
		{Symbol: "US:AAPL", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 210, High: 212, Low: 199, Volume: 1200},
	}
	executor := Executor{
		Store: store,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			return &recordingChatClient{err: context.Canceled}
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
		},
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
	})

	err := executor.Execute(context.Background(), "task-cancelled", validated, "sk-runtime-secret")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	for _, event := range store.events {
		if event.EventType == string(task.EventFailed) {
			t.Fatalf("cancelled execution must not write failed event, got %+v", store.events)
		}
	}
	if store.tasks["task-cancelled"].Status != string(task.StatusRunning) {
		t.Fatalf("cancelled execution must not overwrite task status, got %+v", store.tasks["task-cancelled"])
	}
}

// TestExecutorSaveTaskAndEventRollsBackTaskWhenEventFails 验证任务状态和事件必须原子写入，避免 SSE 回放缺失终态事件。
func TestExecutorSaveTaskAndEventRollsBackTaskWhenEventFails(t *testing.T) {
	store := newExecutionStore()
	store.appendTaskEventErr = errors.New("event write failed")
	executor := Executor{Store: store}
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

	err := executor.saveTaskAndEvent(context.Background(), task.Task{
		ID:        "task-atomic",
		Type:      task.TypeAnalysis,
		Status:    task.StatusSuccess,
		Progress:  100,
		CreatedAt: now,
		UpdatedAt: now,
	}, task.Event{
		TaskID:    "task-atomic",
		Type:      task.EventSuccess,
		Payload:   `{"progress":100}`,
		CreatedAt: now,
		UpdatedAt: now,
	})

	if !errors.Is(err, store.appendTaskEventErr) {
		t.Fatalf("expected append event error, got %v", err)
	}
	if _, ok := store.tasks["task-atomic"]; ok {
		t.Fatalf("task state must roll back when event write fails: %+v", store.tasks["task-atomic"])
	}
}

// TestExecutorRollsBackReportWhenCompletionEventFails 验证报告、输出事件和成功终态必须原子写入，避免失败任务残留可见报告。
func TestExecutorRollsBackReportWhenCompletionEventFails(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{
		ID:      9,
		Name:    "综合分析",
		Type:    "stock_full",
		Content: "请基于 {{ quote }}、{{ kline_summary }}、{{ indicators }} 输出报告。",
	}
	store.quote = model.Quote{
		Symbol:    "US:AAPL",
		Price:     210.5,
		QuoteTime: time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		Provider:  "test-provider",
		UpdatedAt: time.Now().UTC(),
	}
	store.klines = []model.Kline{
		{Symbol: "US:AAPL", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 210, High: 212, Low: 199, Volume: 1200},
	}
	store.appendTaskEventErr = errors.New("chunk event write failed")
	store.appendTaskEventErrAtCall = 2
	executor := Executor{
		Store: store,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			return &recordingChatClient{response: aiservice.ChatResponse{Content: "报告正文"}}
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
		},
	}
	validated := mustValidateCreateRequest(t, CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
	})

	err := executor.Execute(context.Background(), "task-report-atomic", validated, "sk-runtime-secret")

	if !errors.Is(err, store.appendTaskEventErr) {
		t.Fatalf("expected chunk event error, got %v", err)
	}
	if store.report.TaskID != "" {
		t.Fatalf("report must roll back when completion event write fails: %+v", store.report)
	}
	taskModel := store.tasks["task-report-atomic"]
	if taskModel.Status != string(task.StatusFailed) {
		t.Fatalf("expected failed task after completion write failure, got %+v", taskModel)
	}
	if taskModel.Progress == 100 {
		t.Fatalf("failed task must not keep success progress: %+v", taskModel)
	}
	for _, event := range store.events {
		if event.EventType == string(task.EventChunk) || event.EventType == string(task.EventSuccess) {
			t.Fatalf("completion transaction must not leave chunk or success event: %+v", store.events)
		}
	}
}

type executionStore struct {
	aiConfig                 model.AIConfig
	template                 model.PromptTemplate
	quote                    model.Quote
	klines                   []model.Kline
	news                     []model.NewsItem
	tasks                    map[string]model.Task
	events                   []model.TaskEvent
	report                   model.AnalysisReport
	appendTaskEventErr       error
	appendTaskEventErrAtCall int
	appendTaskEventCalls     int
}

// newExecutionStore 创建分析执行器测试用内存 store。
func newExecutionStore() *executionStore {
	return &executionStore{tasks: make(map[string]model.Task)}
}

// GetAIConfig 返回测试用 AI 配置。
func (store *executionStore) GetAIConfig(context.Context, int64) (model.AIConfig, error) {
	return store.aiConfig, nil
}

// GetPromptTemplate 返回测试用 Prompt 模板。
func (store *executionStore) GetPromptTemplate(context.Context, int64) (model.PromptTemplate, error) {
	return store.template, nil
}

// LatestQuote 返回测试用最新行情，空 symbol 表示缓存缺失。
func (store *executionStore) LatestQuote(context.Context, string, time.Duration) (model.Quote, bool, error) {
	if store.quote.Symbol == "" {
		return model.Quote{}, false, nil
	}
	return store.quote, true, nil
}

// ListKlines 返回测试用 K 线序列。
func (store *executionStore) ListKlines(context.Context, string, string, string, int) ([]model.Kline, error) {
	return store.klines, nil
}

// SaveKlines 保存测试 K 线缓存。
func (store *executionStore) SaveKlines(_ context.Context, klines []model.Kline) error {
	store.klines = append([]model.Kline(nil), klines...)
	return nil
}

// ListNewsBySymbol 返回测试用新闻序列。
func (store *executionStore) ListNewsBySymbol(context.Context, string, int, time.Duration) ([]model.NewsItem, error) {
	return store.news, nil
}

// SaveTask 保存测试任务状态。
func (store *executionStore) SaveTask(_ context.Context, item *model.Task) error {
	store.tasks[item.ID] = *item
	return nil
}

// SaveTaskWithEvent 模拟生产事务语义，事件写入失败时回滚任务状态。
func (store *executionStore) SaveTaskWithEvent(ctx context.Context, task *model.Task, event *model.TaskEvent) error {
	previousTasks := make(map[string]model.Task, len(store.tasks))
	for id, item := range store.tasks {
		previousTasks[id] = item
	}
	previousEvents := append([]model.TaskEvent(nil), store.events...)

	if err := store.SaveTask(ctx, task); err != nil {
		return err
	}
	if err := store.AppendTaskEvent(ctx, event); err != nil {
		store.tasks = previousTasks
		store.events = previousEvents
		return err
	}
	return nil
}

// AppendTaskEvent 追加测试任务事件并模拟自增 ID。
func (store *executionStore) AppendTaskEvent(_ context.Context, item *model.TaskEvent) error {
	store.appendTaskEventCalls++
	if store.appendTaskEventErr != nil &&
		(store.appendTaskEventErrAtCall == 0 || store.appendTaskEventCalls == store.appendTaskEventErrAtCall) {
		return store.appendTaskEventErr
	}
	if item.ID == 0 {
		item.ID = int64(len(store.events) + 1)
	}
	store.events = append(store.events, *item)
	return nil
}

// SaveAnalysisReportByTaskID 保存测试分析报告。
func (store *executionStore) SaveAnalysisReportByTaskID(_ context.Context, item *model.AnalysisReport) error {
	store.report = *item
	return nil
}

// SaveAnalysisReportWithTaskCompletion 模拟生产事务语义，完成事件失败时回滚报告和终态。
func (store *executionStore) SaveAnalysisReportWithTaskCompletion(ctx context.Context, report *model.AnalysisReport, chunkEvent *model.TaskEvent, task *model.Task, successEvent *model.TaskEvent) error {
	previousTasks := make(map[string]model.Task, len(store.tasks))
	for id, item := range store.tasks {
		previousTasks[id] = item
	}
	previousEvents := append([]model.TaskEvent(nil), store.events...)
	previousReport := store.report

	if err := store.SaveAnalysisReportByTaskID(ctx, report); err != nil {
		return err
	}
	if err := store.AppendTaskEvent(ctx, chunkEvent); err != nil {
		store.tasks = previousTasks
		store.events = previousEvents
		store.report = previousReport
		return err
	}
	if err := store.SaveTask(ctx, task); err != nil {
		store.tasks = previousTasks
		store.events = previousEvents
		store.report = previousReport
		return err
	}
	if err := store.AppendTaskEvent(ctx, successEvent); err != nil {
		store.tasks = previousTasks
		store.events = previousEvents
		store.report = previousReport
		return err
	}
	return nil
}

type recordingChatClient struct {
	requests []aiservice.ChatRequest
	response aiservice.ChatResponse
	err      error
}

type recordingMarketProvider struct {
	klines     []marketservice.KlineBar
	klineCalls int
}

// Name 返回测试行情 Provider 名称。
func (provider *recordingMarketProvider) Name() string {
	return "recording-market"
}

// Status 返回测试行情 Provider 状态。
func (provider *recordingMarketProvider) Status(context.Context) marketservice.ProviderStatus {
	return marketservice.ProviderStatus{Name: provider.Name(), Available: true}
}

// Search 在分析执行器测试中不使用。
func (provider *recordingMarketProvider) Search(context.Context, string) ([]marketservice.StockBasic, error) {
	return nil, errors.New("unexpected search")
}

// Quote 在当前测试中不使用。
func (provider *recordingMarketProvider) Quote(context.Context, stockservice.Symbol) (marketservice.Quote, error) {
	return marketservice.Quote{}, errors.New("unexpected quote")
}

// Kline 记录测试 K 线请求。
func (provider *recordingMarketProvider) Kline(_ context.Context, request marketservice.KlineRequest) ([]marketservice.KlineBar, error) {
	provider.klineCalls++
	if request.Symbol.String() != "CN:SH:600522" || request.Period != marketservice.PeriodDay || request.Adjust != marketservice.AdjustNone || request.Limit != defaultKlineLimit {
		return nil, errors.New("unexpected kline request")
	}
	return provider.klines, nil
}

type analysisRoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip 让分析执行器测试用函数捕获外部 HTTP 请求。
func (fn analysisRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

// Chat 记录测试 AI 请求并返回预设响应。
func (client *recordingChatClient) Chat(_ context.Context, request aiservice.ChatRequest) (aiservice.ChatResponse, error) {
	client.requests = append(client.requests, request)
	return client.response, client.err
}

type recordingTaskLogWriter struct {
	entries []model.TaskLogEntry
}

// WriteTaskLog 记录分析执行器产生的阶段日志，验证任务日志链路不会丢阶段上下文。
func (writer *recordingTaskLogWriter) WriteTaskLog(_ context.Context, entry model.TaskLogEntry) error {
	writer.entries = append(writer.entries, entry)
	return nil
}

type recordingTaskNotifier struct {
	tasks []model.Task
	err   error
}

// NotifyTaskTerminal 记录分析任务终态通知请求，验证执行器只在任务持久化后触发通知。
func (notifier *recordingTaskNotifier) NotifyTaskTerminal(_ context.Context, task model.Task) (bool, error) {
	notifier.tasks = append(notifier.tasks, task)
	return notifier.err == nil, notifier.err
}

// assertTaskLogStages 验证分析执行器为关键阶段写入可检索、可关联的结构化日志。
func assertTaskLogStages(t *testing.T, entries []model.TaskLogEntry, expectedStages []string) {
	t.Helper()
	if len(entries) < len(expectedStages) {
		t.Fatalf("expected at least %d task log entries, got %+v", len(expectedStages), entries)
	}
	seen := make(map[string]model.TaskLogEntry, len(entries))
	for _, entry := range entries {
		seen[entry.Stage] = entry
	}
	for _, stage := range expectedStages {
		entry, ok := seen[stage]
		if !ok {
			t.Fatalf("missing task log stage %q in %+v", stage, entries)
		}
		if entry.TaskID != "task-1" ||
			entry.Module != "analysis" ||
			entry.Provider != aiservice.ProviderOpenAICompatible ||
			entry.Model != "gpt-analysis" ||
			entry.Symbol != "US:AAPL" {
			t.Fatalf("unexpected task log entry for stage %q: %+v", stage, entry)
		}
		if entry.Level != "INFO" {
			t.Fatalf("expected INFO level for stage %q, got %+v", stage, entry)
		}
	}
}

type expectedProgressEvent struct {
	stage    string
	progress int
	message  string
}

// assertProgressEvents 验证分析阶段会写入可被 SSE 实时推送的任务进度事件。
func assertProgressEvents(t *testing.T, events []model.TaskEvent, expected []expectedProgressEvent) {
	t.Helper()
	seen := make(map[string]map[string]any, len(events))
	for _, event := range events {
		if event.EventType != string(task.EventProgress) {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatalf("progress event payload must be valid json: %+v", event)
		}
		stage, _ := payload["stage"].(string)
		seen[stage] = payload
	}
	for _, item := range expected {
		payload, ok := seen[item.stage]
		if !ok {
			t.Fatalf("missing progress event for stage %q in %+v", item.stage, events)
		}
		if payload["message"] != item.message || int(payload["progress"].(float64)) != item.progress {
			t.Fatalf("unexpected progress payload for stage %q: %+v", item.stage, payload)
		}
	}
}

// mustValidateCreateRequest 校验测试请求并在失败时终止测试。
func mustValidateCreateRequest(t *testing.T, request CreateRequest) ValidatedCreateRequest {
	t.Helper()
	validated, err := ValidateCreateRequest(request)
	if err != nil {
		t.Fatalf("ValidateCreateRequest returned error: %v", err)
	}
	return validated
}

// mustParseStockSymbol 解析测试用标准股票代码。
func mustParseStockSymbol(t *testing.T, raw string) stockservice.Symbol {
	t.Helper()
	symbol, err := stockservice.ParseSymbol(raw)
	if err != nil {
		t.Fatalf("ParseSymbol returned error: %v", err)
	}
	return symbol
}

// makeAnalysisIndicatorKlines 构造足够覆盖 60 日均线和常用指标的日 K 序列。
func makeAnalysisIndicatorKlines(count int) []model.Kline {
	klines := make([]model.Kline, 0, count)
	for index := 0; index < count; index++ {
		closePrice := 10 + float64(index)*0.18 + float64(index%5)*0.03
		klines = append(klines, model.Kline{
			Symbol:    "CN:SH:600522",
			Period:    "day",
			Adjust:    "none",
			TradeDate: time.Date(2026, 4, 1+index, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			Open:      closePrice - 0.08,
			High:      closePrice + 0.32,
			Low:       closePrice - 0.28,
			Close:     closePrice,
			Volume:    1000000 + float64(index)*12000,
			Amount:    closePrice * (1000000 + float64(index)*12000),
		})
	}
	return klines
}
