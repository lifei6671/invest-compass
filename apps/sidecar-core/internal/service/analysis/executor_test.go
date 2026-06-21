package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
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
		Symbol:        "US:AAPL",
		Price:         210.5,
		ChangePercent: 1.25,
		QuoteTime:     time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		Provider:      "test-provider",
		UpdatedAt:     time.Now().UTC(),
	}
	store.klines = []model.Kline{
		{Symbol: "US:AAPL", Period: "day", Adjust: "none", TradeDate: "2026-06-17", Close: 200, High: 205, Low: 198, Volume: 1000},
		{Symbol: "US:AAPL", Period: "day", Adjust: "none", TradeDate: "2026-06-18", Close: 210, High: 212, Low: 199, Volume: 1200},
	}
	store.news = []model.NewsItem{
		{Title: "新品发布", Summary: "公司发布新产品", PublishedAt: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)},
	}
	chat := &recordingChatClient{response: aiservice.ChatResponse{Content: "报告正文\n风险提示：市场波动。"}}
	taskLogWriter := &recordingTaskLogWriter{}
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
		!strings.Contains(joinedMessages, "新品发布") ||
		!strings.Contains(joinedMessages, "用户一次性持仓输入") {
		t.Fatalf("AI request missing analysis context: %#v", chat.requests[0].Messages)
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
	if store.report.TaskID != "task-1" ||
		store.report.ContentMarkdown != "报告正文\n风险提示：市场波动。" ||
		!strings.Contains(store.report.InputSnapshot, "123.45") ||
		!strings.Contains(store.report.InputSnapshot, "medium") ||
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
}

// TestExecutorMarksTaskFailedWhenRequiredDataMissing 验证缺少真实行情上下文时任务失败而不是伪造数据。
func TestExecutorMarksTaskFailedWhenRequiredDataMissing(t *testing.T) {
	store := newExecutionStore()
	store.aiConfig = model.AIConfig{ID: 7, Provider: aiservice.ProviderOpenAICompatible, ModelName: "gpt-analysis"}
	store.template = model.PromptTemplate{ID: 9, Name: "综合分析", Type: "stock_full", Content: "请分析 {{ quote }}"}
	executor := Executor{
		Store: store,
		NewChatClient: func(aiservice.Config, string) ChatClient {
			t.Fatal("missing quote must not call AI")
			return nil
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
	if len(store.events) != 1 || store.events[0].EventType != string(task.EventStarted) {
		t.Fatalf("cancelled execution must only keep started event, got %+v", store.events)
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

// mustValidateCreateRequest 校验测试请求并在失败时终止测试。
func mustValidateCreateRequest(t *testing.T, request CreateRequest) ValidatedCreateRequest {
	t.Helper()
	validated, err := ValidateCreateRequest(request)
	if err != nil {
		t.Fatalf("ValidateCreateRequest returned error: %v", err)
	}
	return validated
}
