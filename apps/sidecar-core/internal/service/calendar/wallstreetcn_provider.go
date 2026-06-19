package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	wallstreetcnProviderName       = "wallstreetcn-calendar"
	defaultWallstreetcnCalendarURL = "https://api-one-wscn.awtmt.com/apiv1/finance/indicator/search"
	wallstreetcnReferer            = "https://wallstreetcn.com/"
)

// WallstreetcnConfig 描述华尔街见闻财经日历 Provider 配置。
type WallstreetcnConfig struct {
	CalendarURL string
	HTTPClient  *http.Client
	Timeout     time.Duration
	Now         func() time.Time
}

// WallstreetcnProvider 使用华尔街见闻公开接口抓取财经日历。
type WallstreetcnProvider struct {
	calendarURL string
	client      *crawler.Client
	now         func() time.Time
}

// NewWallstreetcnProvider 创建华尔街见闻财经日历 Provider。
func NewWallstreetcnProvider(config WallstreetcnConfig) (*WallstreetcnProvider, error) {
	client, err := newCalendarCrawler(wallstreetcnProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &WallstreetcnProvider{
		calendarURL: calendarFirstNonEmpty(config.CalendarURL, defaultWallstreetcnCalendarURL),
		client:      client,
		now:         now,
	}, nil
}

// Name 返回华尔街见闻财经日历 Provider 稳定名称。
func (provider *WallstreetcnProvider) Name() string {
	return wallstreetcnProviderName
}

// Status 返回华尔街见闻财经日历状态。
func (provider *WallstreetcnProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Wallstreetcn finance calendar endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 抓取华尔街见闻财经日历。
func (provider *WallstreetcnProvider) Fetch(ctx context.Context, request Request) ([]Event, error) {
	start, end, err := provider.wallstreetcnRange(request)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "request", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.calendarURL,
		Query: map[string]string{
			"start_time": strconv.FormatInt(start.Unix(), 10),
			"end_time":   strconv.FormatInt(end.Unix(), 10),
			"limit":      "50",
		},
		Headers: map[string]string{
			"Accept":        "application/json,text/plain,*/*",
			"Referer":       wallstreetcnReferer,
			"x-client-type": "pc",
			"x-ivanka-app":  "wscn|web|0.40.40|0.0|0",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "fetch", err)
	}

	var response wallstreetcnCalendarResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Code != 20000 {
		return nil, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("wallstreetcn calendar code %d: %s", response.Code, response.Message))
	}
	return wallstreetcnCalendarRowsToEvents(response.Data.Items), nil
}

// wallstreetcnRange 计算财经日历查询时间范围。
func (provider *WallstreetcnProvider) wallstreetcnRange(request Request) (time.Time, time.Time, error) {
	yearMonth := strings.TrimSpace(request.YearMonth)
	if yearMonth == "" {
		start := provider.now().UTC()
		return start, start.Add(7 * 24 * time.Hour), nil
	}
	start, err := time.ParseInLocation("2006-01", yearMonth, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid year_month %q", yearMonth)
	}
	return start, start.AddDate(0, 1, 0), nil
}

type wallstreetcnCalendarResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []wallstreetcnCalendarItem `json:"items"`
	} `json:"data"`
}

type wallstreetcnCalendarItem struct {
	PublicDate int64  `json:"public_date"`
	Country    string `json:"country"`
	Title      string `json:"title"`
	Event      string `json:"event"`
	Importance int    `json:"importance"`
	Actual     string `json:"actual"`
	Forecast   string `json:"forecast"`
	Previous   string `json:"previous"`
	Revised    string `json:"revised"`
	Period     string `json:"period"`
	Assets     string `json:"assets"`
}

// wallstreetcnCalendarRowsToEvents 将华尔街见闻日历响应清洗为统一事件。
func wallstreetcnCalendarRowsToEvents(rows []wallstreetcnCalendarItem) []Event {
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		title := calendarFirstNonEmpty(row.Event, row.Title)
		if title == "" {
			continue
		}
		if row.PublicDate <= 0 {
			continue
		}
		publishedAt := time.Unix(row.PublicDate, 0).UTC()
		events = append(events, Event{
			Title:      title,
			Date:       publishedAt.Format("2006-01-02"),
			Time:       publishedAt.Format("15:04"),
			Country:    strings.TrimSpace(row.Country),
			Importance: strconv.Itoa(row.Importance),
			Source:     "华尔街见闻日历",
			Raw: map[string]any{
				"actual":     strings.TrimSpace(row.Actual),
				"forecast":   strings.TrimSpace(row.Forecast),
				"previous":   strings.TrimSpace(row.Previous),
				"revised":    strings.TrimSpace(row.Revised),
				"period":     strings.TrimSpace(row.Period),
				"assets":     strings.TrimSpace(row.Assets),
				"publicDate": row.PublicDate,
			},
		})
	}
	return events
}
