package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	aiservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/ai"
	indicatorservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/indicator"
	marketservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
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
	SaveKlines(ctx context.Context, klines []model.Kline) error
	ListNewsBySymbol(ctx context.Context, symbol string, limit int, maxAge time.Duration) ([]model.NewsItem, error)
	AppendTaskEvent(ctx context.Context, event *model.TaskEvent) error
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
	Store          ExecutionStore
	NewChatClient  ChatClientFactory
	MarketProvider marketservice.MarketProvider
	HTTPClient     *http.Client
	Now            func() time.Time
	TaskLogWriter  tasklogservice.StageWriter
	TaskNotifier   TaskNotifier
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
			klines, err = executor.fetchAndCacheKlines(ctx, request)
			if err != nil {
				return err
			}
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
		DailyKlines:      dailyKlines(klines),
		Indicators:       indicators,
		News:             newsSummary(newsItems),
		DataAsof:         dataAsof(quote, klines, executor.now()),
		ContextQuality:   contextQuality(klines, indicators),
		PromptKey:        promptKey(promptTemplate),
		PromptVersion:    promptTemplate.Version,
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
		InputSnapshot: request.InputSnapshotForReportWithContext(rawPromptForSnapshot(builtPrompt), ReportSnapshotMeta{
			PromptTemplate: promptTemplate.Name,
			Model:          aiConfig.ModelName,
			Temperature:    aiConfig.Temperature,
			MaxTokens:      aiConfig.MaxTokens,
		}),
		ContentMarkdown: content,
		RiskSummary:     riskSummary(content),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, content, nil
}

// rawPromptForSnapshot 合并实际发送给模型的 System/User Prompt，供报告详情审计回放。
func rawPromptForSnapshot(prompt promptservice.BuiltPrompt) string {
	parts := []string{
		"System:\n" + strings.TrimSpace(prompt.System),
		"User:\n" + strings.TrimSpace(prompt.Context+"\n\n"+prompt.User),
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
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
	var err error
	if executor.TaskLogWriter == nil {
		err = fn(ctx)
	} else {
		err = tasklogservice.RunStage(ctx, executor.TaskLogWriter, meta, stage, fn)
	}
	if err != nil {
		return err
	}
	return executor.appendProgressEvent(ctx, meta.TaskID, stage)
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

// appendProgressEvent 将分析阶段写入任务事件表，供 SSE 流实时推送到运行页。
func (executor Executor) appendProgressEvent(ctx context.Context, taskID string, stage string) error {
	progress, message, ok := analysisStageProgress(stage)
	if !ok {
		return nil
	}
	now := executor.now()
	event := taskEventToModel(taskservice.Event{
		TaskID: strings.TrimSpace(taskID),
		Type:   taskservice.EventProgress,
		Payload: taskservice.SanitizeEventPayload(mustJSON(map[string]any{
			"stage":    stage,
			"progress": progress,
			"message":  message,
		})),
		CreatedAt: now,
		UpdatedAt: now,
	})
	return executor.Store.AppendTaskEvent(ctx, &event)
}

// analysisStageProgress 定义分析任务阶段到前端步骤显示的稳定映射。
func analysisStageProgress(stage string) (int, string, bool) {
	switch stage {
	case tasklogservice.StageQuoteFetch:
		return 20, "行情快照已读取", true
	case tasklogservice.StageKlineFetch:
		return 35, "K 线数据已准备", true
	case tasklogservice.StageCalcMACD:
		return 50, "技术指标已计算", true
	case tasklogservice.StagePromptBuild:
		return 65, "Prompt 已构建", true
	case tasklogservice.StageStreamStart:
		return 80, "AI 模型已响应", true
	case tasklogservice.StageStreamChunk:
		return 90, "AI 输出已生成", true
	default:
		return 0, "", false
	}
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

// fetchAndCacheKlines 在分析任务缺少本地 K 线缓存时补拉真实行情源，避免前端承担隐藏的预热职责。
func (executor Executor) fetchAndCacheKlines(ctx context.Context, request ValidatedCreateRequest) ([]model.Kline, error) {
	if executor.MarketProvider == nil {
		return nil, &xerr.Error{Code: xerr.PromptMissingData, Message: "missing klines"}
	}
	bars, err := executor.MarketProvider.Kline(ctx, marketservice.KlineRequest{
		Symbol: request.Symbol,
		Period: marketservice.Period(defaultKlinePeriod),
		Adjust: marketservice.Adjust(defaultKlineAdjust),
		Limit:  defaultKlineLimit,
	})
	if err != nil {
		return nil, err
	}
	klines := modelKlinesFromMarket(bars)
	if len(klines) == 0 {
		return nil, nil
	}
	if err := executor.Store.SaveKlines(ctx, klines); err != nil {
		return nil, err
	}
	return klines, nil
}

// modelKlinesFromMarket 将行情 Provider 的 K 线转换为分析任务复用的本地缓存模型。
func modelKlinesFromMarket(bars []marketservice.KlineBar) []model.Kline {
	sort.SliceStable(bars, func(left int, right int) bool {
		return bars[left].TradeDate < bars[right].TradeDate
	})
	klines := make([]model.Kline, 0, len(bars))
	for _, bar := range bars {
		klines = append(klines, model.Kline{
			Symbol:    bar.Symbol.String(),
			Period:    string(bar.Period),
			Adjust:    string(bar.Adjust),
			TradeDate: bar.TradeDate,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Amount:    bar.Amount,
			Provider:  bar.Provider,
		})
	}
	return klines
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
		ID:            item.ID,
		Key:           item.Key,
		Name:          item.Name,
		Type:          promptservice.TemplateType(item.Type),
		Description:   item.Description,
		Content:       item.Content,
		Variables:     variables,
		IsBuiltin:     item.IsBuiltin,
		BuiltinLocked: item.BuiltinLocked,
		Version:       item.Version,
		Checksum:      item.Checksum,
		Source:        item.Source,
		Deleted:       item.DeletedAt.Valid,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		DeletedAt:     item.DeletedAt.Time,
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
		"price=%.2f change_percent=%.2f%% volume=%.0f amount=%.0f turnover_rate=%.2f%% pe=%.2f pb=%.2f total_market_cap=%.0f float_market_cap=%.0f quote_time=%s provider=%s",
		quote.Price,
		quote.ChangePercent,
		quote.Volume,
		quote.Amount,
		quote.TurnoverRate,
		quote.PE,
		quote.PB,
		quote.TotalMarketCap,
		quote.FloatMarketCap,
		formatTime(quote.QuoteTime),
		quote.Provider,
	)
}

// klineSummary 将 K 线序列压缩为简介，逐日明细由 dailyKlines 单独提供。
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

// dailyKlines 输出最近日 K 的逐日 OHLCV 明细，供 AI 直接基于真实每日结构分析。
func dailyKlines(klines []model.Kline) string {
	lines := make([]string, 0, len(klines))
	for _, item := range klines {
		lines = append(lines, fmt.Sprintf(
			"date=%s open=%.2f high=%.2f low=%.2f close=%.2f volume=%.0f amount=%.0f",
			item.TradeDate,
			item.Open,
			item.High,
			item.Low,
			item.Close,
			item.Volume,
			item.Amount,
		))
	}
	return strings.Join(lines, "\n")
}

// dataAsof 选择输入数据截止时间，优先使用行情时间，其次使用最后一根 K 线日期。
func dataAsof(quote model.Quote, klines []model.Kline, fallback time.Time) string {
	if !quote.QuoteTime.IsZero() {
		return formatTime(quote.QuoteTime)
	}
	if len(klines) > 0 && strings.TrimSpace(klines[len(klines)-1].TradeDate) != "" {
		return klines[len(klines)-1].TradeDate
	}
	return formatTime(fallback)
}

// contextQuality 给 Prompt 提供简短质量说明，避免模型把缺失数据当成已知事实。
func contextQuality(klines []model.Kline, indicators string) string {
	indicatorStatus := "技术指标已生成"
	if strings.TrimSpace(indicators) == "" {
		indicatorStatus = "技术指标缺失"
	}
	if len(klines) >= defaultKlineLimit {
		return fmt.Sprintf("日 K 线 %d 条，覆盖最近 60 日样本；%s。", len(klines), indicatorStatus)
	}
	return fmt.Sprintf("日 K 线 %d 条，少于最近 60 日样本；%s，技术分析可靠性下降。", len(klines), indicatorStatus)
}

// promptKey 返回模板稳定 key；历史自定义模板缺 key 时使用 ID 标识，避免 Prompt 出现空字段。
func promptKey(template promptservice.Template) string {
	if strings.TrimSpace(template.Key) != "" {
		return template.Key
	}
	if template.ID > 0 {
		return fmt.Sprintf("template_%d", template.ID)
	}
	return "custom_template"
}

// indicatorSummary 输出个股分析所需的常用技术指标快照，供 AI 基于结构化数值分析。
func indicatorSummary(klines []model.Kline) string {
	first := klines[0]
	last := klines[len(klines)-1]
	change := 0.0
	if first.Close != 0 {
		change = (last.Close - first.Close) / first.Close * 100
	}
	closes := analysisCloseValues(klines)
	volumes := analysisVolumeValues(klines)
	indicatorKlines := analysisIndicatorKLines(klines)

	lines := []string{
		fmt.Sprintf("close_change_percent=%.2f%% latest_volume=%.0f", change, last.Volume),
		fmt.Sprintf(
			"MA ma5=%s ma10=%s ma20=%s ma60=%s",
			latestIndicatorValue(indicatorservice.MA(closes, 5)),
			latestIndicatorValue(indicatorservice.MA(closes, 10)),
			latestIndicatorValue(indicatorservice.MA(closes, 20)),
			latestIndicatorValue(indicatorservice.MA(closes, 60)),
		),
		fmt.Sprintf(
			"EMA ema12=%s ema26=%s",
			latestIndicatorValue(indicatorservice.EMA(closes, 12)),
			latestIndicatorValue(indicatorservice.EMA(closes, 26)),
		),
	}
	if macd, err := indicatorservice.MACD(closes, 12, 26, 9); err == nil {
		lines = append(lines, fmt.Sprintf(
			"MACD dif=%s dea=%s bar=%s",
			latestIndicatorValue(macd.DIF, nil),
			latestIndicatorValue(macd.DEA, nil),
			latestIndicatorValue(macd.Bar, nil),
		))
	} else {
		lines = append(lines, "MACD 数据不足")
	}
	lines = append(lines, fmt.Sprintf(
		"RSI rsi6=%s rsi12=%s rsi24=%s",
		latestIndicatorValue(indicatorservice.RSI(closes, 6)),
		latestIndicatorValue(indicatorservice.RSI(closes, 12)),
		latestIndicatorValue(indicatorservice.RSI(closes, 24)),
	))
	if kdj, err := indicatorservice.KDJ(indicatorKlines, 9); err == nil {
		lines = append(lines, fmt.Sprintf(
			"KDJ k=%s d=%s j=%s",
			latestIndicatorValue(kdj.K, nil),
			latestIndicatorValue(kdj.D, nil),
			latestIndicatorValue(kdj.J, nil),
		))
	} else {
		lines = append(lines, "KDJ 数据不足")
	}
	if boll, err := indicatorservice.BOLL(closes, 20, 2); err == nil {
		lines = append(lines, fmt.Sprintf(
			"BOLL middle=%s upper=%s lower=%s",
			latestIndicatorValue(boll.Middle, nil),
			latestIndicatorValue(boll.Upper, nil),
			latestIndicatorValue(boll.Lower, nil),
		))
	} else {
		lines = append(lines, "BOLL 数据不足")
	}
	lines = append(lines, fmt.Sprintf(
		"VOLUME_MA ma5=%s ma10=%s",
		latestIndicatorValue(indicatorservice.VolumeMA(volumes, 5)),
		latestIndicatorValue(indicatorservice.VolumeMA(volumes, 10)),
	))
	lines = append(lines, fmt.Sprintf(
		"RISK_METRICS volatility=%s max_drawdown=%s",
		singleIndicatorValue(indicatorservice.Volatility(closes)),
		singleIndicatorValue(indicatorservice.MaxDrawdown(closes)),
	))
	return strings.Join(lines, "\n")
}

// latestIndicatorValue 提取指标序列最后一个有效值；数据不足时明确标注，避免 AI 误读空值。
func latestIndicatorValue(values []float64, err error) string {
	if err != nil {
		return "数据不足"
	}
	for index := len(values) - 1; index >= 0; index-- {
		value := values[index]
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			return fmt.Sprintf("%.2f", value)
		}
	}
	return "数据不足"
}

// singleIndicatorValue 格式化单值指标；错误不吞掉，用“数据不足”暴露给 Prompt。
func singleIndicatorValue(value float64, err error) string {
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "数据不足"
	}
	return fmt.Sprintf("%.2f%%", value)
}

// analysisCloseValues 提取分析指标计算用收盘价序列。
func analysisCloseValues(klines []model.Kline) []float64 {
	values := make([]float64, 0, len(klines))
	for _, item := range klines {
		values = append(values, item.Close)
	}
	return values
}

// analysisVolumeValues 提取分析指标计算用成交量序列。
func analysisVolumeValues(klines []model.Kline) []float64 {
	values := make([]float64, 0, len(klines))
	for _, item := range klines {
		values = append(values, item.Volume)
	}
	return values
}

// analysisIndicatorKLines 转换 KDJ 所需的高低收盘价结构。
func analysisIndicatorKLines(klines []model.Kline) []indicatorservice.KLine {
	values := make([]indicatorservice.KLine, 0, len(klines))
	for _, item := range klines {
		values = append(values, indicatorservice.KLine{
			High:  item.High,
			Low:   item.Low,
			Close: item.Close,
		})
	}
	return values
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
