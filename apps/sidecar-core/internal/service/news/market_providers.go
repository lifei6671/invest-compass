package news

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	cailianpressProviderName = "cailianpress-news"
	sinaLiveProviderName     = "sina-live-news"
	defaultNewsTimeout       = 8 * time.Second
	defaultCLSTelegraphURL   = "https://www.cls.cn/api/cache"
	defaultSinaLiveFeedURL   = "https://zhibo.sina.com.cn/api/zhibo/feed"
)

var sinaLiveLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// CailianpressConfig 描述财联社快讯 Provider 的运行配置。
type CailianpressConfig struct {
	TelegraphURL       string
	HTTPClient         *http.Client
	Timeout            time.Duration
	CredentialResolver CookieCredentialResolver
}

// CailianpressProvider 抓取财联社电报快讯并清洗成统一新闻模型。
type CailianpressProvider struct {
	telegraphURL       string
	client             *crawler.Client
	credentialResolver CookieCredentialResolver
}

// CookieCredentialResolver 为需要 Cookie 的新闻 Provider 提供运行时凭据。
type CookieCredentialResolver interface {
	ResolveCookie(ctx context.Context, providerID string) (string, error)
}

// NewCailianpressProvider 创建财联社快讯 Provider。
func NewCailianpressProvider(config CailianpressConfig) (*CailianpressProvider, error) {
	client, err := newNewsCrawler(cailianpressProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &CailianpressProvider{
		telegraphURL:       newsFirstNonEmpty(config.TelegraphURL, defaultCLSTelegraphURL),
		client:             client,
		credentialResolver: config.CredentialResolver,
	}, nil
}

// Name 返回财联社快讯 Provider 稳定名称。
func (provider *CailianpressProvider) Name() string {
	return cailianpressProviderName
}

// Status 返回财联社快讯数据源状态说明。
func (provider *CailianpressProvider) Status(ctx context.Context) ProviderStatus {
	if provider == nil {
		return ProviderStatus{Name: cailianpressProviderName, Source: "Cailianpress web telegraph endpoint", Available: false}
	}
	if provider.credentialResolver == nil {
		return ProviderStatus{
			Name:      provider.Name(),
			Source:    "Cailianpress web telegraph endpoint",
			Available: true,
		}
	}
	if _, err := provider.credentialResolver.ResolveCookie(ctx, "cls"); err != nil {
		return ProviderStatus{
			Name:      provider.Name(),
			Source:    "Cailianpress web telegraph endpoint",
			Available: false,
			LastError: redactProviderCredentialError(err),
		}
	}
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    "Cailianpress web telegraph endpoint",
		Available: provider != nil,
	}
}

// List 当前不提供个股新闻能力，显式失败避免调用方把不支持误判为空结果。
func (provider *CailianpressProvider) List(context.Context, ListRequest) ([]Item, error) {
	return nil, unsupportedStockNewsError(provider.Name())
}

// Market 抓取财联社市场快讯。
func (provider *CailianpressProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	cookie, err := provider.resolveCookie(ctx)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "credential_resolve", err)
	}
	headers := map[string]string{
		"Accept":  "application/json,text/plain,*/*",
		"Referer": "https://www.cls.cn/",
	}
	if cookie != "" {
		headers["Cookie"] = cookie
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.telegraphURL,
		Query: map[string]string{
			"app":  "CailianpressWeb",
			"name": "telegraph",
			"os":   "web",
			"sv":   "8.7.9",
		},
		Headers: headers,
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "market_fetch", err)
	}

	var response clsTelegraphResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "market_decode", err)
	}
	if response.Errno != 0 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("cls errno %d", response.Errno))
	}
	items := make([]Item, 0, len(response.Data.RollData))
	for _, row := range response.Data.RollData {
		title := strings.TrimSpace(row.Title)
		summary := strings.TrimSpace(row.Content)
		if title == "" {
			title = summary
		}
		if title == "" {
			continue
		}
		newsURL := strings.TrimSpace(row.ShareURL)
		if newsURL == "" && row.ID != 0 {
			newsURL = fmt.Sprintf("https://www.cls.cn/telegraph/%d", row.ID)
		}
		publishedAt := result.FetchedAt
		if row.CTime > 0 {
			publishedAt = time.Unix(row.CTime, 0).UTC()
		}
		items = append(items, Item{
			Source:      "财联社电报",
			Title:       title,
			URL:         newsURL,
			Summary:     summary,
			PublishedAt: publishedAt,
			Tags:        row.SubjectNames(),
		})
	}
	return normalizeLimit(items, request.Limit)
}

// resolveCookie 读取财联社运行时 Cookie，未配置 resolver 时兼容公开接口请求。
func (provider *CailianpressProvider) resolveCookie(ctx context.Context) (string, error) {
	if provider.credentialResolver == nil {
		return "", nil
	}
	return provider.credentialResolver.ResolveCookie(ctx, "cls")
}

// redactProviderCredentialError 返回可展示的脱敏凭据错误，不泄露真实 Cookie。
func redactProviderCredentialError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err.Error()
	}
	return "data_source_credential_not_configured"
}

// SinaLiveConfig 描述新浪财经直播 Provider 的运行配置。
type SinaLiveConfig struct {
	FeedURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// SinaLiveProvider 抓取新浪财经直播快讯并清洗成统一新闻模型。
type SinaLiveProvider struct {
	feedURL string
	client  *crawler.Client
}

// NewSinaLiveProvider 创建新浪财经直播 Provider。
func NewSinaLiveProvider(config SinaLiveConfig) (*SinaLiveProvider, error) {
	client, err := newNewsCrawler(sinaLiveProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &SinaLiveProvider{
		feedURL: newsFirstNonEmpty(config.FeedURL, defaultSinaLiveFeedURL),
		client:  client,
	}, nil
}

// Name 返回新浪财经直播 Provider 稳定名称。
func (provider *SinaLiveProvider) Name() string {
	return sinaLiveProviderName
}

// Status 返回新浪财经直播数据源状态说明。
func (provider *SinaLiveProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    "Sina finance live feed endpoint",
		Available: provider != nil,
	}
}

// List 当前不提供个股新闻能力，显式失败避免调用方把不支持误判为空结果。
func (provider *SinaLiveProvider) List(context.Context, ListRequest) ([]Item, error) {
	return nil, unsupportedStockNewsError(provider.Name())
}

// Market 抓取新浪财经直播快讯。
func (provider *SinaLiveProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.feedURL,
		Query: map[string]string{
			"callback":  "callback",
			"page":      "1",
			"page_size": "20",
			"zhibo_id":  "152",
			"tag_id":    "0",
			"dire":      "f",
			"dpc":       "1",
			"pagesize":  "20",
			"type":      "0",
			"_":         strconv.FormatInt(time.Now().Unix(), 10),
		},
		Headers: map[string]string{
			"Accept":  "*/*",
			"Referer": "https://finance.sina.com.cn",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "market_fetch", err)
	}

	payload, err := extractJSONP(result.Body)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "jsonp_decode", err)
	}
	var response sinaLiveResponse
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		return nil, NewProviderError(provider.Name(), "market_decode", err)
	}
	rows := response.Result.Data.Feed.List
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		content := strings.TrimSpace(row.RichText)
		if content == "" {
			continue
		}
		publishedAt := result.FetchedAt
		if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", row.CreateTime, sinaLiveLocation); err == nil {
			publishedAt = parsedAt.UTC()
		}
		items = append(items, Item{
			Source:      "新浪财经",
			Title:       sinaTitle(content),
			Summary:     content,
			PublishedAt: publishedAt,
			Tags:        row.TagNames(),
		})
	}
	return normalizeLimit(items, request.Limit)
}

type clsTelegraphResponse struct {
	Errno int `json:"errno"`
	Data  struct {
		RollData []clsTelegraphRow `json:"roll_data"`
	} `json:"data"`
}

type clsTelegraphRow struct {
	ID       int64  `json:"id"`
	CTime    int64  `json:"ctime"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	ShareURL string `json:"shareurl"`
	Subjects []struct {
		Name string `json:"subject_name"`
	} `json:"subjects"`
}

// SubjectNames 返回财联社主题标签名称。
func (row clsTelegraphRow) SubjectNames() []string {
	tags := make([]string, 0, len(row.Subjects))
	for _, subject := range row.Subjects {
		name := strings.TrimSpace(subject.Name)
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

type sinaLiveResponse struct {
	Result struct {
		Data struct {
			Feed struct {
				List []sinaLiveFeedRow `json:"list"`
			} `json:"feed"`
		} `json:"data"`
	} `json:"result"`
}

type sinaLiveFeedRow struct {
	RichText   string `json:"rich_text"`
	CreateTime string `json:"create_time"`
	Tags       []struct {
		Name string `json:"name"`
	} `json:"tag"`
}

// TagNames 返回新浪直播标签名称。
func (row sinaLiveFeedRow) TagNames() []string {
	tags := make([]string, 0, len(row.Tags))
	for _, tag := range row.Tags {
		name := strings.TrimSpace(tag.Name)
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

// newNewsCrawler 创建新闻抓取 Client。
func newNewsCrawler(name string, timeout time.Duration, httpClient *http.Client) (*crawler.Client, error) {
	if timeout == 0 {
		timeout = defaultNewsTimeout
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

// normalizeLimit 标准化、去重并限制新闻数量。
func normalizeLimit(items []Item, limit int) ([]Item, error) {
	normalized, err := NormalizeItems(items)
	if err != nil {
		return nil, err
	}
	deduped := Deduplicate(normalized)
	if limit > 0 && limit < len(deduped) {
		return deduped[:limit], nil
	}
	return deduped, nil
}

// unsupportedStockNewsError 返回市场快讯 Provider 不支持个股新闻查询的明确错误。
func unsupportedStockNewsError(providerName string) error {
	return NewProviderError(providerName, "list_unsupported", fmt.Errorf("stock news list is unsupported"))
}

// extractJSONP 从 JSONP 包裹体中提取 JSON。
func extractJSONP(body string) (string, error) {
	if marker := "callback("; strings.Contains(body, marker) {
		start := strings.Index(body, marker) + len(marker)
		tail := body[start:]
		end := strings.Index(tail, ");")
		if end > 0 {
			return tail[:end], nil
		}
	}
	start := strings.Index(body, "(")
	end := strings.LastIndex(body, ")")
	if start < 0 || end <= start {
		return "", fmt.Errorf("invalid jsonp response")
	}
	return body[start+1 : end], nil
}

// sinaTitle 从新浪直播正文中提取标题。
func sinaTitle(content string) string {
	matches := regexp.MustCompile(`^【([^】]+)】`).FindStringSubmatch(content)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return content
}

// newsFirstNonEmpty 返回第一个非空字符串。
func newsFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
