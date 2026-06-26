package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	promptservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/prompt"
	taskservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	tasklogservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/tasklog"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	defaultKlinePeriod = "day"
	defaultKlineAdjust = "none"
	defaultKlineLimit  = 60
	defaultNewsLimit   = 8
)

// ExecutionStore 是分析执行器需要的数据读取和写入边界。
type ExecutionStore interface {
	GetAIConfig(ctx context.Context, id int64) (model.AIConfig, error)
	GetPromptTemplate(ctx context.Context, id int64) (model.PromptTemplate, error)
	LatestQuote(ctx context.Context, symbol string, maxAge time.Duration) (model.Quote, bool, error)
	ListKlines(ctx context.Context, symbol string, period string, adjust string, limit int) ([]model.Kline, error)
	ListNewsBySymbol(ctx context.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
	SaveTaskWithEvent(ctx context.Context, task *model.Task, event *model.TaskEvent) error
	SaveAnalysisReportWithTaskCompletion(ctx context.Context, report *model.AnalysisReport, chunkEvent *model.TaskEvent, task *model.Task, successEvent *model.TaskEvent) error
}

// ChatClient 是分析执行器依赖的 AI 对话边界，便于单测替换外部 Provider。
type ChatClient interface {
	Chat(ctx context.Context, request aiservice.ChatRequest) (aiservice.ChatResponse, error)
}

// ChatClientFactory 根据 AI 配置和运行期密钥创建 AI 客户端。
type ChatClientFactory func(config aiservice.Config, resolvedAPIKey string) ChatClient

// TaskNotifier 是分析任务终态通知边界，执行器只负责触发，不直接依赖通知持久化细节。
type TaskNotifier interface {
	NotifyTaskTerminal(ctx context.Context, task model.Task) (bool, error)
}

// Executor 负责执行单个分析任务，不处理 HTTP 路由和桌面 command。
type Executor struct {
	Store         ExecutionStore
	NewChatClient ChatClientFactory
	HTTPClient    *http.Client
	Now           func() time.Time
	TaskLogWriter tasklogservice.StageWriter
	TaskNotifier  TaskNotifier
}

// Execute 拉取已缓存上下文、调用 AI、保存报告并推进任务事件。
func (executor Executor) Execute(ctx context.Context, taskID string, request ValidatedCreateRequest, resolvedAPIKey string) error {
	if strings.TrimSpace(resolvedAPIKey) == "" {
		return &xerr.Error{Code: xerr.AIInvalidRequest}
	}
	if executor.Store == nil {
		return fmt.Errorf("analysis execution store is required")
	}

	now := executor.now()
	runningTask := taskservice.Task{
		ID:        taskID,
		Type:      taskservice.TypeAnalysis,
		Status:    taskservice.StatusRunning,
		Title:     request.Symbol.String() + " " + string(request.AnalysisType),
		Progress:  5,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := executor.saveTaskAndEvent(ctx, runningTask, taskservice.Event{
		TaskID:    taskID,
		Type:      taskservice.EventStarted,
		Payload:   `{"progress":5}`,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return err
	}

	report, content, err := executor.executeBody(ctx, taskID, request, resolvedAPIKey)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		_ = executor.markFailed(ctx, runningTask, err)
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	chunkTime := executor.now()
	chunkEvent := model.TaskEvent{
		TaskID:    taskID,
		EventType: string(taskservice.EventChunk),
		Payload:   taskservice.SanitizeEventPayload(mustJSON(map[string]any{"content": content})),
		CreatedAt: chunkTime,
		UpdatedAt: chunkTime,
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	finishedAt := executor.now()
	successTask := runningTask
	successTask.Status = taskservice.StatusSuccess
	successTask.Progress = 100
	successTask.FinishedAt = finishedAt
	successTask.UpdatedAt = finishedAt
	successTaskModel := taskToModel(successTask)
	successEvent := taskEventToModel(taskservice.Event{
		TaskID:    taskID,
		Type:      taskservice.EventSuccess,
		Payload:   `{"progress":100}`,
		CreatedAt: finishedAt,
		UpdatedAt: finishedAt,
	})
	if err := executor.Store.SaveAnalysisReportWithTaskCompletion(ctx, &report, &chunkEvent, &successTaskModel, &successEvent); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		_ = executor.markFailed(ctx, runningTask, err)
		return err
	}
	executor.notifyTaskTerminal(ctx, successTaskModel)
	return nil
}

// executeBody 执行分析主体，返回待保存报告和 AI 正文。
func (executor Executor) executeBody(ctx context.Context, taskID string, request ValidatedCreateRequest, resolvedAPIKey string) (model.AnalysisReport, string, error) {
	aiConfigModel, err := executor.Store.GetAIConfig(ctx, request.AIConfigID)
	if err != nil {
		return model.AnalysisReport{}, "", err
	}
	promptTemplateModel, err := executor.Store.GetPromptTemplate(ctx, request.PromptTemplateID)
	if err != nil {
		return model.AnalysisReport{}, "", err
	}
	promptTemplate, err := promptTemplateFromModel(promptTemplateModel)
	if err != nil {
		return model.AnalysisReport{}, "", err
	}

	aiConfig := aiConfigFromModel(aiConfigModel)
	stageMeta := executor.stageMeta(taskID, request, aiConfig)

	var quote model.Quote
	var ok bool
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StageQuoteFetch, func(ctx context.Context) error {
		var err error
		quote, ok, err = executor.Store.LatestQuote(ctx, request.Symbol.String(), 0)
		if err != nil {
			return err
		}
		if !ok {
			return &xerr.Error{Code: xerr.PromptMissingData, Message: "missing quote"}
		}
		return nil
	}); err != nil {
		return model.AnalysisReport{}, "", err
	}

	var klines []model.Kline
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StageKlineFetch, func(ctx context.Context) error {
		var err error
		klines, err = executor.Store.ListKlines(ctx, request.Symbol.String(), defaultKlinePeriod, defaultKlineAdjust, defaultKlineLimit)
		if err != nil {
			return err
		}
		if len(klines) == 0 {
			return &xerr.Error{Code: xerr.PromptMissingData, Message: "missing klines"}
		}
		return nil
	}); err != nil {
		return model.AnalysisReport{}, "", err
	}
	newsItems, err := executor.Store.ListNewsBySymbol(ctx, request.Symbol.String(), defaultNewsLimit, 0)
	if err != nil {
		return model.AnalysisReport{}, "", err
	}
	var indicators string
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StageCalcMACD, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		indicators = indicatorSummary(klines)
		return nil
	}); err != nil {
		return model.AnalysisReport{}, "", err
	}

	promptInput := promptservice.BuildInput{
		StockName:        request.Symbol.String(),
		StockCode:        request.Symbol.String(),
		Market:           request.Symbol.Market,
		Quote:            quoteSummary(quote),
		KlineSummary:     klineSummary(klines),
		Indicators:       indicators,
		News:             newsSummary(newsItems),
		AnalysisLanguage: "简体中文",
		UserQuestion:     "请生成符合投研罗盘首版合规要求的个股分析报告。",
		UserPosition:     userPositionSummary(request.UserPosition),
	}
	var builtPrompt promptservice.BuiltPrompt
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StagePromptBuild, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		builtPrompt, err = buildPromptForAnalysis(request.AnalysisType, promptTemplate, promptInput)
		return err
	}); err != nil {
		return model.AnalysisReport{}, "", err
	}

	client := executor.chatClient(aiConfig, resolvedAPIKey)
	var chatResponse aiservice.ChatResponse
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StageStreamStart, func(ctx context.Context) error {
		var err error
		chatResponse, err = client.Chat(ctx, aiservice.ChatRequest{
			Model:       aiConfig.ModelName,
			Temperature: aiConfig.Temperature,
			MaxTokens:   aiConfig.MaxTokens,
			Messages: []aiservice.Message{
				{Role: aiservice.RoleSystem, Content: builtPrompt.System},
				{Role: aiservice.RoleUser, Content: builtPrompt.Context + "\n\n" + builtPrompt.User},
			},
		})
		return err
	}); err != nil {
		_ = executor.writeFailedStreamStage(ctx, stageMeta, err)
		return model.AnalysisReport{}, "", err
	}

	content := strings.TrimSpace(chatResponse.Content)
	if err := executor.runStage(ctx, stageMeta, tasklogservice.StageStreamChunk, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return model.AnalysisReport{}, "", err
	}
	now := executor.now()
	return model.AnalysisReport{
		TaskID:           taskID,
		Symbol:           request.Symbol.String(),
		Title:            request.Symbol.String() + " " + string(request.AnalysisType) + " AI 分析报告",
		AnalysisType:     string(request.AnalysisType),
		ModelName:        aiConfig.ModelName,
		PromptTemplateID: request.PromptTemplateID,
		InputSnapshot:    request.InputSnapshotForReport(),
		ContentMarkdown:  content,
		RiskSummary:      riskSummary(content),
		CreatedAt:        now,
		UpdatedAt:        now,
	}, content, nil
}

// stageMeta 生成分析任务阶段日志的公共上下文，避免真实密钥和完整 Prompt 进入日志。
func (executor Executor) stageMeta(taskID string, request ValidatedCreateRequest, aiConfig aiservice.Config) tasklogservice.StageMeta {
	return tasklogservice.StageMeta{
		TaskID:    taskID,
		Module:    "analysis",
		Provider:  aiConfig.Provider,
		Model:     aiConfig.ModelName,
		Symbol:    request.Symbol.String(),
		Retryable: true,
		Now:       executor.now,
	}
}

// runStage 在生产环境写入阶段日志；未注入 writer 的单测路径保持原始执行语义。
func (executor Executor) runStage(ctx context.Context, meta tasklogservice.StageMeta, stage string, fn func(context.Context) error) error {
	if executor.TaskLogWriter == nil {
		return fn(ctx)
	}
	return tasklogservice.RunStage(ctx, executor.TaskLogWriter, meta, stage, fn)
}

// writeFailedStreamStage 将 AI 调用失败进一步落到超时或失败阶段，方便日志抽屉按阶段排障。
func (executor Executor) writeFailedStreamStage(ctx context.Context, meta tasklogservice.StageMeta, cause error) error {
	if executor.TaskLogWriter == nil || cause == nil {
		return nil
	}
	stage := tasklogservice.StageStreamFailed
	if isTimeoutError(cause) {
		stage = tasklogservice.StageStreamTimeout
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return tasklogservice.RunStage(writeCtx, executor.TaskLogWriter, meta, stage, func(context.Context) error {
		return cause
	})
}

// isTimeoutError 识别 Provider 或上下文返回的超时类错误。
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") || strings.Contains(lower, "超时")
}

// markFailed 将任务切换为 FAILED，并写入脱敏失败事件。
func (executor Executor) markFailed(ctx context.Context, runningTask taskservice.Task, cause error) error {
	now := executor.now()
	runningTask.Status = taskservice.StatusFailed
	runningTask.Error = logger.RedactText(cause.Error())
	runningTask.FinishedAt = now
	runningTask.UpdatedAt = now
	if err := executor.saveTaskAndEvent(ctx, runningTask, taskservice.Event{
		TaskID:    runningTask.ID,
		Type:      taskservice.EventFailed,
		Payload:   taskservice.SanitizeEventPayload(mustJSON(map[string]any{"error": cause.Error()})),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return err
	}
	executor.notifyTaskTerminal(ctx, taskToModel(runningTask))
	return nil
}

// notifyTaskTerminal 写入应用内通知；通知失败只影响提醒，不反向改变分析任务终态。
func (executor Executor) notifyTaskTerminal(ctx context.Context, task model.Task) {
	if executor.TaskNotifier == nil {
		return
	}
	if _, err := executor.TaskNotifier.NotifyTaskTerminal(ctx, task); err != nil {
		slog.Warn(
			"分析任务通知写入失败",
			"task_id", task.ID,
			"status", task.Status,
			"error", logger.RedactText(err.Error()),
		)
	}
}

// saveTaskAndEvent 按任务状态和事件成对持久化。
func (executor Executor) saveTaskAndEvent(ctx context.Context, item taskservice.Task, event taskservice.Event) error {
	taskModel := taskToModel(item)
	eventModel := taskEventToModel(event)
	return executor.Store.SaveTaskWithEvent(ctx, &taskModel, &eventModel)
}

// chatClient 创建 AI 客户端，默认使用 OpenAI-compatible Provider。
func (executor Executor) chatClient(config aiservice.Config, resolvedAPIKey string) ChatClient {
	if executor.NewChatClient != nil {
		return executor.NewChatClient(config, resolvedAPIKey)
	}
	return aiservice.NewOpenAICompatibleClient(aiservice.ClientConfig{
		BaseURL: config.BaseURL,
		APIKey:  resolvedAPIKey,
		Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		Client:  executor.HTTPClient,
	})
}

// now 返回当前时间，测试可以注入稳定时间。
func (executor Executor) now() time.Time {
	if executor.Now != nil {
		return executor.Now()
	}
	return time.Now().UTC()
}

// promptTemplateFromModel 转换持久化 Prompt 模板为 service 模型。
func promptTemplateFromModel(item model.PromptTemplate) (promptservice.Template, error) {
	var rawVariables []string
	if item.Variables != "" {
		if err := json.Unmarshal([]byte(item.Variables), &rawVariables); err != nil {
			return promptservice.Template{}, err
		}
	}
	variables := make([]promptservice.Variable, 0, len(rawVariables))
	for _, variable := range rawVariables {
		variables = append(variables, promptservice.Variable(variable))
	}
	return promptservice.Template{
		ID:          item.ID,
		Name:        item.Name,
		Type:        promptservice.TemplateType(item.Type),
		Description: item.Description,
		Content:     item.Content,
		Variables:   variables,
		IsBuiltin:   item.IsBuiltin,
		Deleted:     item.DeletedAt.Valid,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		DeletedAt:   item.DeletedAt.Time,
	}, nil
}

// aiConfigFromModel 转换持久化 AI 配置为 service Provider 配置。
func aiConfigFromModel(item model.AIConfig) aiservice.Config {
	return aiservice.Config{
		ID:             item.ID,
		Name:           item.Name,
		Provider:       item.Provider,
		BaseURL:        item.BaseURL,
		APIKeyRef:      item.APIKeyRef,
		MaskedAPIKey:   item.MaskedAPIKey,
		HasAPIKey:      item.HasAPIKey,
		ModelName:      item.ModelName,
		Temperature:    item.Temperature,
		MaxTokens:      item.MaxTokens,
		TimeoutSeconds: item.TimeoutSeconds,
		StreamEnabled:  item.StreamEnabled,
		IsDefault:      item.IsDefault,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

// buildPromptForAnalysis 按分析类型选择对应 Prompt builder。
func buildPromptForAnalysis(analysisType AnalysisType, template promptservice.Template, input promptservice.BuildInput) (promptservice.BuiltPrompt, error) {
	switch analysisType {
	case AnalysisStockFull:
		return promptservice.BuildStockFullPrompt(template, input)
	case AnalysisTechnical:
		return promptservice.BuildTechnicalPrompt(template, input)
	default:
		return promptservice.BuiltPrompt{}, &xerr.Error{Code: xerr.AnalysisUnsupportedType}
	}
}

// quoteSummary 将行情快照压缩为 Prompt 上下文。
func quoteSummary(quote model.Quote) string {
	return fmt.Sprintf(
		"price=%.2f change_percent=%.2f%% volume=%.0f amount=%.0f quote_time=%s provider=%s",
		quote.Price,
		quote.ChangePercent,
		quote.Volume,
		quote.Amount,
		formatTime(quote.QuoteTime),
		quote.Provider,
	)
}

// klineSummary 将 K 线序列压缩为可审计摘要。
func klineSummary(klines []model.Kline) string {
	first := klines[0]
	last := klines[len(klines)-1]
	return fmt.Sprintf(
		"count=%d range=%s..%s first_close=%.2f last_close=%.2f high=%.2f low=%.2f",
		len(klines),
		first.TradeDate,
		last.TradeDate,
		first.Close,
		last.Close,
		maxHigh(klines),
		minLow(klines),
	)
}

// indicatorSummary 输出首版分析任务内置的轻量技术指标摘要。
func indicatorSummary(klines []model.Kline) string {
	first := klines[0]
	last := klines[len(klines)-1]
	change := 0.0
	if first.Close != 0 {
		change = (last.Close - first.Close) / first.Close * 100
	}
	return fmt.Sprintf("close_change_percent=%.2f%% latest_volume=%.0f", change, last.Volume)
}

// newsSummary 将新闻条目压缩为 Prompt 上下文；没有新闻时明确说明缺失，不伪造新闻。
func newsSummary(items []model.NewsItem) string {
	if len(items) == 0 {
		return "暂无相关新闻缓存"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		summary := strings.TrimSpace(item.Summary)
		if summary == "" {
			summary = "无摘要"
		}
		parts = append(parts, item.Title+"："+summary)
	}
	return strings.Join(parts, "\n")
}

// userPositionSummary 将一次性持仓输入放入 Prompt，不进入日志快照。
func userPositionSummary(position *UserPosition) string {
	if position == nil {
		return ""
	}
	return fmt.Sprintf("cost_price=%.2f shares=%.2f risk_level=%s", position.CostPrice, position.Shares, position.RiskLevel)
}

// riskSummary 提取报告风险摘要，缺失时保留合规兜底提示。
func riskSummary(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "风险") {
			return strings.TrimSpace(line)
		}
	}
	return "AI 输出需结合数据时效和市场风险审慎解读，不构成投资建议。"
}

// maxHigh 返回 K 线最高价。
func maxHigh(klines []model.Kline) float64 {
	result := math.Inf(-1)
	for _, item := range klines {
		if item.High > result {
			result = item.High
		}
	}
	return result
}

// minLow 返回 K 线最低价。
func minLow(klines []model.Kline) float64 {
	result := math.Inf(1)
	for _, item := range klines {
		if item.Low < result {
			result = item.Low
		}
	}
	return result
}

// formatTime 输出 UTC 时间；空时间明确标记为 unknown。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format(time.RFC3339)
}

// mustJSON 编码小型事件 payload，失败时返回空对象保持事件结构有效。
func mustJSON(payload map[string]any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
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
