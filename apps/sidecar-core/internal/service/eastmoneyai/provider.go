package eastmoneyai

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	providerName    = "eastmoney-ai"
	defaultBaseURL  = "https://ai-saas.eastmoney.com"
	defaultTimeout  = 30 * time.Second
	defaultMaxBytes = int64(4 * 1024 * 1024)
)

const (
	endpointEntity            = "/proxy/entity/dialogTagsV2"
	endpointReportList        = "/proxy/app-robo-advisor-api/assistant/write/choice/reportList"
	endpointPerformanceReview = "/proxy/app-robo-advisor-api/assistant/write/performance/comment"
	endpointFinancialQA       = "/proxy/app-robo-advisor-api/assistant/ask"
	endpointIndustryResearch  = "/proxy/app-robo-advisor-api/assistant/write/industry/research"
	endpointTrackingReport    = "/proxy/app-robo-advisor-api/assistant/write/tracking/report"
	endpointSearchData        = "/proxy/b/mcp/tool/searchData"
	endpointSearchNews        = "/proxy/b/mcp/tool/searchNews"
	endpointComparableCompany = "/proxy/app-robo-advisor-api/assistant/comparable-company-analysis"
	endpointHotspotDiscovery  = "/proxy/app-robo-advisor-api/assistant/hotspot-discovery"
)

// Config 描述东方财富 AI Provider 的运行参数。
type Config struct {
	APIKey      string
	BaseURL     string
	ProductType string
	HTTPClient  *http.Client
	Timeout     time.Duration
	Now         func() time.Time
}

// Provider 调用东方财富 AI SaaS 工具接口并返回结构化结果。
type Provider struct {
	apiKey      string
	productType string
	client      *crawler.Client
	now         func() time.Time
}

// ProviderStatus 描述东方财富 AI 数据源来源、授权边界和可用性。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	LastCheckedAt time.Time
	LastError     string
}

// NewProvider 创建东方财富 AI Provider。
func NewProvider(config Config) (*Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", providerName)
	}
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         providerName,
		BaseURL:      baseURL,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: defaultMaxBytes,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		},
	})
	if err != nil {
		return nil, err
	}
	productType := strings.TrimSpace(config.ProductType)
	if productType == "" {
		productType = "mx"
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Provider{
		apiKey:      strings.TrimSpace(config.APIKey),
		productType: productType,
		client:      client,
		now:         now,
	}, nil
}

// Name 返回 Provider 稳定名称。
func (provider *Provider) Name() string {
	return providerName
}

// Status 返回东方财富 AI Provider 状态，不做阻塞式远程探测。
func (provider *Provider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney AI SaaS endpoints",
		License:       "需要用户自有 em_api_key，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil && provider.apiKey != "",
		LastCheckedAt: provider.now().UTC(),
	}
}

// RecognizeEntity 调用东方财富实体识别接口，返回后续报告工具需要的实体编码。
func (provider *Provider) RecognizeEntity(ctx context.Context, query string) (Entity, error) {
	raw, _, err := provider.post(ctx, endpointEntity, map[string]string{"content": strings.TrimSpace(query)})
	if err != nil {
		return Entity{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return Entity{}, err
	}
	entityRaw := firstEntityMap(raw)
	if entityRaw == nil {
		return Entity{}, NewProviderError(provider.Name(), "entity", fmt.Errorf("missing entity"))
	}
	entity, err := entityFromMap(entityRaw)
	if err != nil {
		return Entity{}, NewProviderError(provider.Name(), "entity", err)
	}
	return entity, nil
}

// FetchReportOptions 返回指定东财实体编码可用的报告期列表。
func (provider *Provider) FetchReportOptions(ctx context.Context, eastMoneyCode string) ([]ReportOption, error) {
	raw, _, err := provider.post(ctx, endpointReportList, map[string]string{"emCode": strings.TrimSpace(eastMoneyCode)})
	if err != nil {
		return nil, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return nil, err
	}
	data, _ := raw["data"].(map[string]any)
	values, _ := data["reportDateList"].([]any)
	options := make([]ReportOption, 0, len(values))
	for _, value := range values {
		switch item := value.(type) {
		case string:
			if strings.TrimSpace(item) != "" {
				options = append(options, ReportOption{ReportDate: strings.TrimSpace(item)})
			}
		case map[string]any:
			option := reportOptionFromMap(item)
			if option.ReportDate != "" {
				options = append(options, option)
			}
		}
	}
	if len(options) == 0 {
		return nil, NewProviderError(provider.Name(), "report_options", fmt.Errorf("empty report options"))
	}
	return options, nil
}

// EarningsReview 获取指定实体和报告期的业绩点评。
func (provider *Provider) EarningsReview(ctx context.Context, request EarningsReviewRequest) (ArticleResult, error) {
	payload := map[string]string{
		"query":      strings.TrimSpace(request.EastMoneyCode),
		"reportDate": strings.TrimSpace(request.ReportDate),
	}
	raw, _, err := provider.post(ctx, endpointPerformanceReview, payload)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return ArticleResult{}, err
	}
	article := articleFromRaw(raw)
	if strings.TrimSpace(article.Content) == "" {
		return ArticleResult{}, NewProviderError(provider.Name(), "article_content", fmt.Errorf("empty article content"))
	}
	return article, nil
}

// FinancialQA 调用金融问答接口，返回回答正文和引用。
func (provider *Provider) FinancialQA(ctx context.Context, request QARequest) (QAResult, error) {
	payload := map[string]any{"question": strings.TrimSpace(request.Question)}
	if request.DeepThink {
		payload["deepThink"] = true
	}
	raw, _, err := provider.post(ctx, endpointFinancialQA, payload)
	if err != nil {
		return QAResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return QAResult{}, err
	}
	data, _ := raw["data"].(map[string]any)
	answer := strings.TrimSpace(stringField(data, "displayData", "answer", "content"))
	if answer == "" {
		return QAResult{}, NewProviderError(provider.Name(), "qa_answer", fmt.Errorf("empty answer"))
	}
	return QAResult{
		Answer:     answer,
		References: referencesFromAny(data["refIndexList"]),
	}, nil
}

// IndustryResearch 生成行业研究报告。
func (provider *Provider) IndustryResearch(ctx context.Context, query string) (ArticleResult, error) {
	return provider.article(ctx, endpointIndustryResearch, map[string]string{"query": strings.TrimSpace(query)})
}

// TrackingReport 生成跟踪报告。
func (provider *Provider) TrackingReport(ctx context.Context, query string) (TrackingReportResult, error) {
	raw, _, err := provider.post(ctx, endpointTrackingReport, map[string]string{"query": strings.TrimSpace(query)})
	if err != nil {
		return TrackingReportResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return TrackingReportResult{}, err
	}
	data, _ := raw["data"].(map[string]any)
	if data == nil {
		return TrackingReportResult{}, NewProviderError(provider.Name(), "tracking_data", fmt.Errorf("missing tracking data"))
	}
	result := TrackingReportResult{
		Title:      stringField(data, "title"),
		Content:    stringField(data, "content"),
		EntityType: stringField(data, "entityType", "entity_type"),
		ShareURL:   stringField(data, "shareUrl"),
	}
	if strings.TrimSpace(result.Content) == "" {
		return TrackingReportResult{}, NewProviderError(provider.Name(), "tracking_content", fmt.Errorf("empty tracking content"))
	}
	return result, nil
}

// FinanceDataQuery 调用金融数据查询工具，返回可渲染表格。
func (provider *Provider) FinanceDataQuery(ctx context.Context, query string) (SearchDataResult, error) {
	payload := map[string]any{
		"query": strings.TrimSpace(query),
		"toolContext": map[string]any{
			"callId":   generateCallID(),
			"userInfo": map[string]string{"userId": generateUserID()},
		},
	}
	raw, _, err := provider.post(ctx, endpointSearchData, payload)
	if err != nil {
		return SearchDataResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return SearchDataResult{}, err
	}
	result := searchDataResultFromRaw(raw)
	if !result.hasRenderableTable() {
		return SearchDataResult{}, NewProviderError(provider.Name(), "search_data_empty", fmt.Errorf("empty finance data table"))
	}
	return result, nil
}

// FinanceSearch 调用金融资讯搜索工具，返回搜索摘要内容。
func (provider *Provider) FinanceSearch(ctx context.Context, query string) (SearchNewsResult, error) {
	payload := map[string]any{
		"query": strings.TrimSpace(query),
		"toolContext": map[string]any{
			"callId": generateCallID(),
		},
	}
	raw, _, err := provider.post(ctx, endpointSearchNews, payload)
	if err != nil {
		return SearchNewsResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return SearchNewsResult{}, err
	}
	content := extractSearchContent(raw)
	if content == "" {
		return SearchNewsResult{}, NewProviderError(provider.Name(), "search_content", fmt.Errorf("empty search content"))
	}
	return SearchNewsResult{Content: content}, nil
}

// ComparableCompanyAnalysis 调用可比公司分析工具，返回原始分段表格。
func (provider *Provider) ComparableCompanyAnalysis(ctx context.Context, query string) (ComparableCompanyResult, error) {
	raw, _, err := provider.post(ctx, endpointComparableCompany, map[string]string{"question": strings.TrimSpace(query)})
	if err != nil {
		return ComparableCompanyResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return ComparableCompanyResult{}, err
	}
	values, _ := raw["data"].([]any)
	result := ComparableCompanyResult{Sections: make([]DataTable, 0, len(values))}
	for _, value := range values {
		if sectionRaw, ok := value.(map[string]any); ok {
			result.Sections = append(result.Sections, dataTableFromMap(sectionRaw))
		}
	}
	if len(result.Sections) == 0 {
		return ComparableCompanyResult{}, NewProviderError(provider.Name(), "comparable_data", fmt.Errorf("empty comparable data"))
	}
	return result, nil
}

// HotspotDiscovery 调用热点发现工具，返回模型生成内容。
func (provider *Provider) HotspotDiscovery(ctx context.Context, question string) (string, error) {
	raw, _, err := provider.post(ctx, endpointHotspotDiscovery, map[string]string{"question": strings.TrimSpace(question)})
	if err != nil {
		return "", err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return "", err
	}
	data, _ := raw["data"].(map[string]any)
	content := strings.TrimSpace(stringField(data, "displayData", "content", "answer"))
	if content == "" {
		return "", NewProviderError(provider.Name(), "hotspot_content", fmt.Errorf("empty hotspot content"))
	}
	return content, nil
}

// article 调用返回 title/content/shareUrl 结构的文章类接口。
func (provider *Provider) article(ctx context.Context, endpoint string, payload any) (ArticleResult, error) {
	raw, _, err := provider.post(ctx, endpoint, payload)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := ensureRemoteOK(provider.Name(), raw); err != nil {
		return ArticleResult{}, err
	}
	article := articleFromRaw(raw)
	if strings.TrimSpace(article.Content) == "" {
		return ArticleResult{}, NewProviderError(provider.Name(), "article_content", fmt.Errorf("empty article content"))
	}
	return article, nil
}

// post 执行东方财富 AI POST 请求，并统一解码为 map。
func (provider *Provider) post(ctx context.Context, endpoint string, payload any) (map[string]any, crawler.Result, error) {
	if strings.TrimSpace(provider.apiKey) == "" {
		return nil, crawler.Result{}, NewProviderError(provider.Name(), "auth", fmt.Errorf("eastmoney api key is required"))
	}
	var raw map[string]any
	result, err := provider.client.PostJSON(ctx, crawler.Request{
		URL:     endpoint,
		Headers: provider.headers(),
	}, payload, &raw)
	if err != nil {
		return nil, crawler.Result{}, NewProviderError(provider.Name(), "fetch", err)
	}
	return raw, result, nil
}

// headers 生成单次请求所需 Header，密钥只停留在内存和请求头中。
func (provider *Provider) headers() map[string]string {
	baseInfo, _ := json.Marshal(map[string]string{"productType": provider.productType})
	return map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"em_base_info": string(baseInfo),
		"em_api_key":   provider.apiKey,
	}
}

// ensureRemoteOK 校验远端统一 code/status 字段。
func ensureRemoteOK(provider string, raw map[string]any) error {
	code, hasCode := numericField(raw, "code")
	status, hasStatus := numericField(raw, "status")
	codeOK := !hasCode || code == 0 || code == 200
	statusOK := !hasStatus || status == 0 || status == 200
	if codeOK && statusOK {
		return nil
	}
	message := stringField(raw, "message", "msg")
	if message == "" {
		if data, ok := raw["data"].(map[string]any); ok {
			message = stringField(data, "message", "msg")
		}
	}
	return NewProviderError(provider, "remote_code", fmt.Errorf("code=%.0f status=%.0f message=%s", code, status, message))
}

// generateCallID 生成工具调用 ID，用于东财搜索类接口的上下文。
func generateCallID() string {
	return "call_" + randomDigits(8)
}

// generateUserID 生成匿名用户 ID，用于东财搜索类接口的上下文。
func generateUserID() string {
	return "user_" + randomDigits(8)
}

// randomDigits 返回固定长度数字串，熵不足时回退时间戳片段。
func randomDigits(length int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		text := fmt.Sprintf("%d", time.Now().UnixNano())
		if len(text) > length {
			return text[len(text)-length:]
		}
		return text
	}
	return fmt.Sprintf("%0*d", length, value)
}

// ProviderError 是东方财富 AI Provider 调用失败时的可观测错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏后的错误，避免 API Key 进入日志或响应。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf(
		"eastmoney ai provider %s %s failed: %s",
		err.Provider,
		err.Operation,
		logger.RedactError(err.Cause),
	)
}

// Unwrap 返回原始错误，供调用方分类。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建会自动脱敏输出的 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}
