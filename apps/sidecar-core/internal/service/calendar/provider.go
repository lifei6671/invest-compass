package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	clsProviderName        = "cls-calendar"
	jiuyangProviderName    = "jiuyang-calendar"
	defaultCalendarTimeout = 8 * time.Second
	defaultCLSURL          = "https://www.cls.cn/api/calendar/web/list"
	defaultJiuyangURL      = "https://app.jiuyangongshe.com/jystock-app/api/v1/timeline/list"
)

// ChannelCredential 表示需要用户配置的渠道凭据。
type ChannelCredential struct {
	Cookie string
	Token  string
	APIKey string
}

// Provider 定义财经日历 Provider 能力边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Fetch(ctx context.Context, request Request) ([]Event, error)
}

// ProviderStatus 描述财经日历数据源状态。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	NeedsCookie   bool
	LastCheckedAt time.Time
	LastError     string
}

// Request 是财经日历查询请求。
type Request struct {
	YearMonth string
}

// Event 表示财经日历事件。
type Event struct {
	Title      string
	Date       string
	Time       string
	Country    string
	Importance string
	Source     string
	Raw        map[string]any
}

// ProviderError 是财经日历 Provider 错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏错误文本。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("calendar provider %s %s failed: %s", err.Provider, err.Operation, logger.RedactError(err.Cause))
}

// Unwrap 返回底层错误。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建财经日历 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{Provider: provider, Operation: operation, Cause: cause}
}

// CLSConfig 描述财联社日历 Provider 配置。
type CLSConfig struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// CLSProvider 使用财联社日历公开接口。
type CLSProvider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewCLSProvider 创建财联社日历 Provider。
func NewCLSProvider(config CLSConfig) (*CLSProvider, error) {
	client, err := newCalendarCrawler(clsProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &CLSProvider{url: calendarFirstNonEmpty(config.URL, defaultCLSURL), client: client, now: now}, nil
}

// Name 返回财联社日历 Provider 稳定名称。
func (provider *CLSProvider) Name() string {
	return clsProviderName
}

// Status 返回财联社日历状态。
func (provider *CLSProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Cailianpress calendar endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 抓取财联社财经日历。
func (provider *CLSProvider) Fetch(ctx context.Context, request Request) ([]Event, error) {
	if strings.TrimSpace(request.YearMonth) != "" {
		return nil, NewProviderError(provider.Name(), "request", fmt.Errorf("cls calendar year month filter is unsupported"))
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Query: map[string]string{
			"app":  "CailianpressWeb",
			"flag": "0",
			"os":   "web",
			"sv":   "8.4.6",
			"type": "0",
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://www.cls.cn/",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "fetch", err)
	}
	var response struct {
		Errno int              `json:"errno"`
		Code  int              `json:"code"`
		Msg   string           `json:"msg"`
		Error string           `json:"error"`
		Data  []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Errno != 0 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("cls calendar errno %d: %s", response.Errno, response.Msg))
	}
	if response.Code != 0 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("cls calendar code %d: %s", response.Code, calendarFirstNonEmpty(response.Error, response.Msg)))
	}
	return calendarRowsToEvents(response.Data, "财联社日历"), nil
}

// JiuyangConfig 描述九阳公社日历 Provider 配置。
type JiuyangConfig struct {
	URL        string
	Credential ChannelCredential
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// JiuyangProvider 使用用户配置的 Cookie/token 抓取九阳公社日历。
type JiuyangProvider struct {
	url        string
	credential ChannelCredential
	client     *crawler.Client
	now        func() time.Time
}

// NewJiuyangProvider 创建九阳公社日历 Provider。
func NewJiuyangProvider(config JiuyangConfig) (*JiuyangProvider, error) {
	client, err := newCalendarCrawler(jiuyangProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &JiuyangProvider{
		url:        calendarFirstNonEmpty(config.URL, defaultJiuyangURL),
		credential: config.Credential,
		client:     client,
		now:        now,
	}, nil
}

// Name 返回九阳公社日历 Provider 稳定名称。
func (provider *JiuyangProvider) Name() string {
	return jiuyangProviderName
}

// Status 返回九阳公社日历状态。
func (provider *JiuyangProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Jiuyangongshe timeline endpoint",
		License:       "需要用户自行配置渠道 Cookie/token 并确认授权边界",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil && provider.credential.Cookie != "" && provider.credential.Token != "",
		NeedsCookie:   true,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 使用用户配置凭据抓取九阳公社日历。
func (provider *JiuyangProvider) Fetch(ctx context.Context, request Request) ([]Event, error) {
	if strings.TrimSpace(provider.credential.Cookie) == "" || strings.TrimSpace(provider.credential.Token) == "" {
		return nil, NewProviderError(provider.Name(), "credential", fmt.Errorf("missing jiuyang cookie or token"))
	}
	yearMonth := strings.TrimSpace(request.YearMonth)
	if yearMonth == "" {
		yearMonth = provider.now().Format("2006-01")
	}
	body, err := json.Marshal(map[string]string{
		"date":  yearMonth,
		"grade": "0",
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "encode", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL:    provider.url,
		Method: http.MethodPost,
		Body:   body,
		Headers: map[string]string{
			"Accept":       "application/json,text/plain,*/*",
			"Content-Type": "application/json",
			"Cookie":       provider.credential.Cookie,
			"Origin":       "https://www.jiuyangongshe.com",
			"Referer":      "https://www.jiuyangongshe.com/",
			"platform":     "3",
			"timestamp":    fmt.Sprintf("%d", provider.now().UnixMilli()),
			"token":        provider.credential.Token,
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "fetch", err)
	}
	var response struct {
		Code    int              `json:"code"`
		Status  int              `json:"status"`
		Msg     string           `json:"msg"`
		Message string           `json:"message"`
		Error   string           `json:"error"`
		Data    []map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(result.BodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return nil, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Code != 0 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("jiuyang calendar code %d: %s", response.Code, calendarFirstNonEmpty(response.Msg, response.Message, response.Error)))
	}
	if response.Status != 0 && response.Status != 200 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("jiuyang calendar status %d: %s", response.Status, calendarFirstNonEmpty(response.Msg, response.Message, response.Error)))
	}
	return calendarRowsToEvents(response.Data, "九阳公社日历"), nil
}

// newCalendarCrawler 创建财经日历抓取 Client。
func newCalendarCrawler(name string, timeout time.Duration, httpClient *http.Client) (*crawler.Client, error) {
	if timeout == 0 {
		timeout = defaultCalendarTimeout
	}
	return crawler.NewClient(crawler.Config{
		Name:         name,
		Timeout:      timeout,
		HTTPClient:   httpClient,
		MaxBodyBytes: 1024 * 1024,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
}

// calendarRowsToEvents 将松散 JSON 行清洗成日历事件。
func calendarRowsToEvents(rows []map[string]any, source string) []Event {
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		title := calendarValue(row, "title", "event", "name")
		if title == "" {
			continue
		}
		events = append(events, Event{
			Title:      title,
			Date:       calendarValue(row, "date", "day"),
			Time:       calendarValue(row, "time"),
			Country:    calendarValue(row, "country", "region"),
			Importance: calendarValue(row, "importance", "grade"),
			Source:     source,
			Raw:        row,
		})
	}
	return events
}

// calendarValue 从多个候选字段中读取字符串。
func calendarValue(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

// calendarFirstNonEmpty 返回第一个非空字符串。
func calendarFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
