package news

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	tradingViewProviderName   = "tradingview-news"
	defaultTradingViewNewsURL = "https://news-mediator.tradingview.com/news-flow/v2/news"
	defaultTradingViewReferer = "https://cn.tradingview.com/"
)

// TradingViewConfig 描述 TradingView 中文新闻 Provider 的运行配置。
type TradingViewConfig struct {
	NewsURL    string
	DetailURL  string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// TradingViewProvider 抓取 TradingView 中文市场新闻，并按 go-stock 的来源语义清洗成资讯模型。
type TradingViewProvider struct {
	newsURL   string
	detailURL string
	client    *crawler.Client
}

// NewTradingViewProvider 创建 TradingView 中文新闻 Provider。
func NewTradingViewProvider(config TradingViewConfig) (*TradingViewProvider, error) {
	client, err := newNewsCrawler(tradingViewProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &TradingViewProvider{
		newsURL:   newsFirstNonEmpty(config.NewsURL, defaultTradingViewNewsURL),
		detailURL: strings.TrimSpace(config.DetailURL),
		client:    client,
	}, nil
}

// Name 返回 TradingView 新闻 Provider 稳定名称。
func (provider *TradingViewProvider) Name() string {
	return tradingViewProviderName
}

// Status 返回 TradingView 新闻源状态说明。
func (provider *TradingViewProvider) Status(context.Context) ProviderStatus {
	if provider == nil {
		return UnconfiguredProviderStatus()
	}
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    "TradingView Chinese news endpoint",
		Available: provider != nil,
	}
}

// List 当前不提供个股新闻能力，显式失败避免调用方把不支持误判为空结果。
func (provider *TradingViewProvider) List(context.Context, ListRequest) ([]Item, error) {
	return nil, unsupportedStockNewsError(provider.Name())
}

// Market 抓取 TradingView 中文市场新闻。
func (provider *TradingViewProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.newsURL,
		Query: map[string]string{
			"filter":    "lang:zh-Hans",
			"client":    "screener",
			"streaming": "false",
		},
		Headers: tradingViewHeaders("news-mediator.tradingview.com"),
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "market_fetch", err)
	}
	var response tradingViewNewsResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "market_decode", err)
	}
	items := make([]Item, 0, len(response.Items))
	for index, row := range response.Items {
		if index >= limit {
			break
		}
		title := strings.TrimSpace(row.Title)
		if title == "" {
			continue
		}
		detail := tradingViewDetailResponse{}
		if provider.detailURL != "" {
			detail = provider.fetchDetail(ctx, row.ID)
		}
		summary := strings.TrimSpace(detail.ShortDescription)
		publishedAt := result.FetchedAt
		if row.Published > 0 {
			publishedAt = time.Unix(int64(row.Published), 0).UTC()
		}
		source := "TradingView"
		if providerName := strings.TrimSpace(row.Provider.Name); providerName != "" {
			source = source + "-" + providerName
		}
		items = append(items, Item{
			Source:      source,
			Title:       title,
			URL:         tradingViewNewsURL(row.ID),
			Summary:     newsFirstNonEmpty(summary, title),
			PublishedAt: publishedAt,
			Tags:        detail.TagTitles(),
		})
	}
	return normalizeLimit(items, request.Limit)
}

func (provider *TradingViewProvider) fetchDetail(ctx context.Context, id string) tradingViewDetailResponse {
	id = strings.TrimSpace(id)
	if id == "" {
		return tradingViewDetailResponse{}
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.detailURL,
		Query: map[string]string{
			"id":   id,
			"lang": "zh-Hans",
		},
		Headers: tradingViewHeaders("news-headlines.tradingview.com"),
	})
	if err != nil {
		return tradingViewDetailResponse{}
	}
	var response tradingViewDetailResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return tradingViewDetailResponse{}
	}
	return response
}

type tradingViewNewsResponse struct {
	Items []tradingViewNewsItem `json:"items"`
}

type tradingViewNewsItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Published int    `json:"published"`
	Provider  struct {
		Name string `json:"name"`
	} `json:"provider"`
}

type tradingViewDetailResponse struct {
	ShortDescription string `json:"shortDescription"`
	Tags             []struct {
		Title string `json:"title"`
	} `json:"tags"`
}

// TagTitles 返回 TradingView 详情中的中文标签。
func (response tradingViewDetailResponse) TagTitles() []string {
	tags := make([]string, 0, len(response.Tags))
	for _, tag := range response.Tags {
		title := strings.TrimSpace(tag.Title)
		if title != "" {
			tags = append(tags, title)
		}
	}
	return tags
}

func tradingViewNewsURL(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	return "https://cn.tradingview.com/news/" + url.PathEscape(trimmed)
}

func tradingViewHeaders(host string) map[string]string {
	return map[string]string{
		"Accept":     "application/json,text/plain,*/*",
		"Host":       host,
		"Origin":     defaultTradingViewReferer,
		"Referer":    defaultTradingViewReferer,
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:146.0) Gecko/20100101 Firefox/146.0",
	}
}
