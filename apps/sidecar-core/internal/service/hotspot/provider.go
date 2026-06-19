package hotspot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	xueqiuProviderName    = "xueqiu-hotspot"
	defaultHotspotTimeout = 8 * time.Second
	defaultXueqiuHotURL   = "https://stock.xueqiu.com/v5/stock/hot_stock/list.json"
)

// ChannelCredential 表示需要用户配置的热点渠道凭据。
type ChannelCredential struct {
	Cookie string
	APIKey string
	Token  string
}

// ProviderStatus 描述热点数据源状态。
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

// HotStockRequest 是热股榜请求。
type HotStockRequest struct {
	Size       int
	MarketType string
}

// HotStock 表示单只热股。
type HotStock struct {
	Code  string
	Name  string
	Value float64
	Raw   map[string]any
}

// ProviderError 是热点 Provider 错误。
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
	return fmt.Sprintf("hotspot provider %s %s failed: %s", err.Provider, err.Operation, logger.RedactError(err.Cause))
}

// Unwrap 返回底层错误。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建热点 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{Provider: provider, Operation: operation, Cause: cause}
}

// XueqiuConfig 描述雪球热点 Provider 配置。
type XueqiuConfig struct {
	HotStockURL string
	Credential  ChannelCredential
	HTTPClient  *http.Client
	Timeout     time.Duration
	Now         func() time.Time
}

// XueqiuProvider 使用用户配置 Cookie 抓取雪球热点榜。
type XueqiuProvider struct {
	hotStockURL string
	credential  ChannelCredential
	client      *crawler.Client
	now         func() time.Time
}

// NewXueqiuProvider 创建雪球热点 Provider。
func NewXueqiuProvider(config XueqiuConfig) (*XueqiuProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultHotspotTimeout
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         xueqiuProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 1024 * 1024,
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &XueqiuProvider{
		hotStockURL: hotspotFirstNonEmpty(config.HotStockURL, defaultXueqiuHotURL),
		credential:  config.Credential,
		client:      client,
		now:         now,
	}, nil
}

// Name 返回雪球热点 Provider 稳定名称。
func (provider *XueqiuProvider) Name() string {
	return xueqiuProviderName
}

// Status 返回雪球热点状态。
func (provider *XueqiuProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Xueqiu hot stock endpoint",
		License:       "需要用户自行配置渠道 Cookie 并确认授权边界",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil && provider.credential.Cookie != "",
		NeedsCookie:   true,
		LastCheckedAt: provider.now().UTC(),
	}
}

// FetchHotStocks 使用用户 Cookie 抓取雪球热股。
func (provider *XueqiuProvider) FetchHotStocks(ctx context.Context, request HotStockRequest) ([]HotStock, error) {
	if strings.TrimSpace(provider.credential.Cookie) == "" {
		return nil, NewProviderError(provider.Name(), "credential", fmt.Errorf("missing xueqiu cookie"))
	}
	size := request.Size
	if size <= 0 {
		size = 20
	}
	marketType := strings.TrimSpace(request.MarketType)
	if marketType == "" {
		marketType = "10"
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.hotStockURL,
		Query: map[string]string{
			"page":  "1",
			"size":  strconv.Itoa(size),
			"_type": marketType,
			"type":  marketType,
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Cookie":  provider.credential.Cookie,
			"Origin":  "https://xueqiu.com",
			"Referer": "https://xueqiu.com/",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "fetch_hot_stocks", err)
	}
	var response struct {
		ErrorCode        int    `json:"error_code"`
		ErrorDescription string `json:"error_description"`
		Data             struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "decode_hot_stocks", err)
	}
	if response.ErrorCode != 0 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("xueqiu error %d: %s", response.ErrorCode, response.ErrorDescription))
	}
	items := make([]HotStock, 0, len(response.Data.Items))
	for _, row := range response.Data.Items {
		code := hotspotValue(row, "code", "symbol")
		name := hotspotValue(row, "name")
		if code == "" && name == "" {
			continue
		}
		hotValue, err := hotspotFloat(row["value"])
		if err != nil {
			return nil, NewProviderError(provider.Name(), "parse_hot_stocks", fmt.Errorf("invalid hot stock value for %s %s: %w", code, name, err))
		}
		items = append(items, HotStock{
			Code:  code,
			Name:  name,
			Value: hotValue,
			Raw:   row,
		})
	}
	return items, nil
}

// hotspotValue 从多个候选字段中读取字符串。
func hotspotValue(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

// hotspotFloat 将任意 JSON 数值转换为 float64，坏字段必须显式失败。
func hotspotFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty value")
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported value type %T", value)
	}
}

// hotspotFirstNonEmpty 返回第一个非空字符串。
func hotspotFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
