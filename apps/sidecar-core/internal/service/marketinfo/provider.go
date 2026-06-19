package marketinfo

import (
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
	tencentGlobalIndexProviderName = "tencent-global-index"
	defaultTencentGlobalIndexURL   = "https://proxy.finance.qq.com/ifzqgtimg/appstock/app/rank/indexRankDetail2"
	defaultMarketInfoTimeout       = 8 * time.Second
)

// Provider 定义市场概览 Provider 能力边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Fetch(ctx context.Context, request GlobalIndexRequest) (GlobalIndexSnapshot, error)
}

// ProviderStatus 描述市场概览数据源状态。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	LastCheckedAt time.Time
	LastError     string
}

// GlobalIndexRequest 是全球指数查询请求。
type GlobalIndexRequest struct{}

// GlobalIndexSnapshot 表示全球指数快照。
type GlobalIndexSnapshot struct {
	Regions   map[string][]GlobalIndex
	Provider  string
	Source    string
	FetchedAt time.Time
}

// Markdown 将全球指数快照渲染为 AI 上下文可读文本。
func (snapshot GlobalIndexSnapshot) Markdown() string {
	if len(snapshot.Regions) == 0 {
		return "暂无全球指数数据。"
	}
	regionNames := []struct {
		Key   string
		Title string
	}{
		{Key: "common", Title: "重点关注"},
		{Key: "asia", Title: "亚洲市场"},
		{Key: "america", Title: "美洲市场"},
		{Key: "europe", Title: "欧洲市场"},
		{Key: "other", Title: "其他市场"},
	}
	var builder strings.Builder
	builder.WriteString("# 全球主要指数概览\n")
	builder.WriteString("> 数据来源：腾讯财经，已按区域整理。\n\n")
	written := 0
	for _, region := range regionNames {
		rows := snapshot.Regions[region.Key]
		if len(rows) == 0 {
			continue
		}
		written++
		builder.WriteString("## ")
		builder.WriteString(region.Title)
		builder.WriteString("\n")
		builder.WriteString("| 指数 | 地区 | 最新点位 | 涨跌幅(%) | 状态 |\n")
		builder.WriteString("| --- | --- | ---: | ---: | --- |\n")
		for _, row := range rows {
			name := row.Name
			if name == "" {
				name = row.Code
			}
			builder.WriteString(fmt.Sprintf(
				"| %s | %s | %s | %s | %s |\n",
				marketInfoMarkdownTableCell(name),
				marketInfoMarkdownTableCell(row.Location),
				marketInfoMarkdownTableCell(row.LastPrice),
				marketInfoMarkdownTableCell(row.ChangePercent),
				marketInfoMarkdownTableCell(row.StateText()),
			))
		}
		builder.WriteString("\n")
	}
	if written == 0 {
		return "暂无可解析的全球指数数据。"
	}
	return builder.String()
}

// GlobalIndex 表示单个全球指数。
type GlobalIndex struct {
	Code          string
	Name          string
	Location      string
	QuoteCode     string
	State         string
	ChangePercent string
	LastPrice     string
	ImageURL      string
	Region        string
}

// StateText 返回本地化交易状态。
func (index GlobalIndex) StateText() string {
	switch strings.ToLower(strings.TrimSpace(index.State)) {
	case "open":
		return "开盘"
	case "close":
		return "收盘"
	default:
		if index.State == "" {
			return "-"
		}
		return index.State
	}
}

// marketInfoMarkdownTableCell 转义 Markdown 表格单元，避免远端文本改变列结构。
func marketInfoMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "\r", "<br>")
	return value
}

// ProviderError 是市场概览 Provider 错误。
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
	return fmt.Sprintf("marketinfo provider %s %s failed: %s", err.Provider, err.Operation, logger.RedactError(err.Cause))
}

// Unwrap 返回底层错误。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建市场概览 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{Provider: provider, Operation: operation, Cause: cause}
}

// TencentGlobalIndexConfig 描述腾讯全球指数 Provider 运行配置。
type TencentGlobalIndexConfig struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// TencentGlobalIndexProvider 使用腾讯财经接口抓取全球指数。
type TencentGlobalIndexProvider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewTencentGlobalIndexProvider 创建腾讯全球指数 Provider。
func NewTencentGlobalIndexProvider(config TencentGlobalIndexConfig) (*TencentGlobalIndexProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultMarketInfoTimeout
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         tencentGlobalIndexProviderName,
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
	return &TencentGlobalIndexProvider{
		url:    marketInfoFirstNonEmpty(config.URL, defaultTencentGlobalIndexURL),
		client: client,
		now:    now,
	}, nil
}

// Name 返回腾讯全球指数 Provider 稳定名称。
func (provider *TencentGlobalIndexProvider) Name() string {
	return tencentGlobalIndexProviderName
}

// Status 返回腾讯全球指数数据源状态。
func (provider *TencentGlobalIndexProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Tencent finance global index endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 抓取全球指数快照。
func (provider *TencentGlobalIndexProvider) Fetch(ctx context.Context, request GlobalIndexRequest) (GlobalIndexSnapshot, error) {
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://stockapp.finance.qq.com/mstats",
		},
	})
	if err != nil {
		return GlobalIndexSnapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}
	var response struct {
		Code    *int   `json:"code"`
		Status  *int   `json:"status"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    map[string][]struct {
			Code          string `json:"code"`
			Name          string `json:"name"`
			Location      string `json:"location"`
			QuoteCode     string `json:"qtcode"`
			State         string `json:"state"`
			ChangePercent string `json:"zdf"`
			LastPrice     string `json:"zxj"`
			ImageURL      string `json:"img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return GlobalIndexSnapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Code != nil && *response.Code != 0 && *response.Code != http.StatusOK {
		return GlobalIndexSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("tencent global index code %d: %s", *response.Code, marketInfoFirstNonEmpty(response.Msg, response.Message)))
	}
	if response.Status != nil && *response.Status != 0 && *response.Status != http.StatusOK {
		return GlobalIndexSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("tencent global index status %d: %s", *response.Status, marketInfoFirstNonEmpty(response.Msg, response.Message)))
	}
	regions := make(map[string][]GlobalIndex, len(response.Data))
	validRows := 0
	for region, rows := range response.Data {
		for _, row := range rows {
			if strings.TrimSpace(row.Code) == "" && strings.TrimSpace(row.Name) == "" {
				continue
			}
			validRows++
			regions[region] = append(regions[region], GlobalIndex{
				Code:          row.Code,
				Name:          row.Name,
				Location:      row.Location,
				QuoteCode:     row.QuoteCode,
				State:         row.State,
				ChangePercent: row.ChangePercent,
				LastPrice:     row.LastPrice,
				ImageURL:      row.ImageURL,
				Region:        region,
			})
		}
	}
	if validRows == 0 {
		return GlobalIndexSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty tencent global index data"))
	}
	return GlobalIndexSnapshot{
		Regions:   regions,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

// marketInfoFirstNonEmpty 返回第一个非空字符串。
func marketInfoFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
