package news

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	wallstreetcnLiveProviderName = "wallstreetcn-live-news"
	defaultWallstreetcnLivesURL  = "https://api-one-wscn.awtmt.com/apiv1/content/lives"
	defaultWallstreetcnReferer   = "https://wallstreetcn.com/"
)

var wallstreetcnHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

var wallstreetcnChannels = map[string]string{
	"global-channel":    "全球7x24",
	"a-stock-channel":   "A股",
	"us-stock-channel":  "美股",
	"hk-stock-channel":  "港股",
	"forex-channel":     "外汇",
	"commodity-channel": "商品",
	"goldc-channel":     "黄金",
	"oil-channel":       "原油",
	"bond-channel":      "债券",
	"crypto-channel":    "加密货币",
}

// WallstreetcnLiveConfig 描述华尔街见闻快讯 Provider 的运行配置。
type WallstreetcnLiveConfig struct {
	LivesURL   string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// WallstreetcnLiveProvider 抓取华尔街见闻 7x24 快讯并清洗成统一新闻模型。
type WallstreetcnLiveProvider struct {
	livesURL string
	client   *crawler.Client
}

// NewWallstreetcnLiveProvider 创建华尔街见闻快讯 Provider。
func NewWallstreetcnLiveProvider(config WallstreetcnLiveConfig) (*WallstreetcnLiveProvider, error) {
	client, err := newNewsCrawler(wallstreetcnLiveProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &WallstreetcnLiveProvider{
		livesURL: newsFirstNonEmpty(config.LivesURL, defaultWallstreetcnLivesURL),
		client:   client,
	}, nil
}

// Name 返回华尔街见闻快讯 Provider 稳定名称。
func (provider *WallstreetcnLiveProvider) Name() string {
	return wallstreetcnLiveProviderName
}

// Status 返回华尔街见闻快讯数据源状态说明。
func (provider *WallstreetcnLiveProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    "Wallstreetcn live news endpoint",
		Available: provider != nil,
	}
}

// List 当前不提供个股新闻能力，显式失败避免调用方把不支持误判为空结果。
func (provider *WallstreetcnLiveProvider) List(context.Context, ListRequest) ([]Item, error) {
	return nil, unsupportedStockNewsError(provider.Name())
}

// Market 抓取华尔街见闻 7x24 市场快讯。
func (provider *WallstreetcnLiveProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	channel := wallstreetcnNormalizeChannel(request.Market)
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.livesURL,
		Query: map[string]string{
			"accept":     "live,vip-live",
			"channel":    channel,
			"client":     "pc",
			"first_page": "true",
			"limit":      strconv.Itoa(limit),
		},
		Headers: map[string]string{
			"Accept":        "application/json,text/plain,*/*",
			"Referer":       defaultWallstreetcnReferer,
			"x-client-type": "pc",
			"x-ivanka-app":  "wscn|web|0.40.40|0.0|0",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "market_fetch", err)
	}

	var response wallstreetcnLivesResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "market_decode", err)
	}
	if response.Code != 20000 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("wallstreetcn lives code %d: %s", response.Code, response.Message))
	}

	source := "华尔街见闻-" + wallstreetcnChannelName(channel)
	items := make([]Item, 0, len(response.Data.Items))
	for _, row := range response.Data.Items {
		summary := wallstreetcnCleanHTML(newsFirstNonEmpty(row.ContentText, row.Content))
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = summary
		}
		if title == "" {
			continue
		}
		publishedAt := result.FetchedAt
		if row.DisplayTime > 0 {
			publishedAt = time.Unix(row.DisplayTime, 0).UTC()
		}
		items = append(items, Item{
			Source:      source,
			Title:       title,
			URL:         wallstreetcnNewsURL(row.URI, row.ID),
			Summary:     summary,
			PublishedAt: publishedAt,
			Tags:        wallstreetcnChannelTags(row.Channels),
		})
	}
	return normalizeLimit(items, limit)
}

type wallstreetcnLivesResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []wallstreetcnLiveItem `json:"items"`
	} `json:"data"`
}

type wallstreetcnLiveItem struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	ContentText string   `json:"content_text"`
	DisplayTime int64    `json:"display_time"`
	URI         string   `json:"uri"`
	Channels    []string `json:"channels"`
}

// wallstreetcnNormalizeChannel 返回华尔街见闻支持的频道代码。
func wallstreetcnNormalizeChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	if _, ok := wallstreetcnChannels[channel]; ok {
		return channel
	}
	return "global-channel"
}

// wallstreetcnChannelName 返回频道中文名称。
func wallstreetcnChannelName(channel string) string {
	if name, ok := wallstreetcnChannels[channel]; ok {
		return name
	}
	return wallstreetcnChannels["global-channel"]
}

// wallstreetcnChannelTags 将频道代码转换为中文标签。
func wallstreetcnChannelTags(channels []string) []string {
	tags := make([]string, 0, len(channels))
	for _, channel := range channels {
		if name, ok := wallstreetcnChannels[strings.TrimSpace(channel)]; ok && name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

// wallstreetcnCleanHTML 将快讯正文中的 HTML 标签清理成纯文本。
func wallstreetcnCleanHTML(raw string) string {
	text := strings.ReplaceAll(raw, "</p>", "\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = wallstreetcnHTMLTagPattern.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	lines := strings.Fields(text)
	return strings.TrimSpace(strings.Join(lines, " "))
}

// wallstreetcnNewsURL 生成安全的华尔街见闻快讯链接。
func wallstreetcnNewsURL(uri string, id int64) string {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri
	}
	if strings.HasPrefix(uri, "//") {
		return "https:" + uri
	}
	if strings.HasPrefix(uri, "/") {
		return "https://wallstreetcn.com" + uri
	}
	if id > 0 {
		return fmt.Sprintf("https://wallstreetcn.com/livenews/%d", id)
	}
	return ""
}
