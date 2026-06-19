package marketinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	clsMarketStatisticProviderName = "cls-market-statistic"
	defaultCLSMarketStatisticURL   = "https://x-quote.cls.cn/quote/index/home"
)

// MarketStatisticProvider 定义市场涨跌统计 Provider 能力边界。
type MarketStatisticProvider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchMarketStatistic(ctx context.Context, request MarketStatisticRequest) (MarketStatisticSnapshot, error)
}

// MarketStatisticRequest 是市场统计查询请求。
type MarketStatisticRequest struct{}

// MarketStatisticSnapshot 表示一次市场涨跌统计快照。
type MarketStatisticSnapshot struct {
	UpCount       int
	DownCount     int
	UpRatio       float64
	UpDownRatio   float64
	SentimentDesc string
	LimitUp       int
	LimitDown     int
	LimitRatio    float64
	ShUpCount     int
	ShDownCount   int
	SzUpCount     int
	SzDownCount   int
	Distribution  UpDownDistribution
	IndexQuotes   []IndexQuote
	Provider      string
	Source        string
	FetchedAt     time.Time
}

// UpDownDistribution 表示全市场涨跌分布。
type UpDownDistribution struct {
	UpNum       int
	DownNum     int
	AverageRise float64
	RiseNum     int
	FallNum     int
	Down10      int
	Down8       int
	Down6       int
	Down4       int
	Down2       int
	FlatNum     int
	Up2         int
	Up4         int
	Up6         int
	Up8         int
	Up10        int
	SuspendNum  int
	Status      bool
}

// IndexQuote 表示市场核心指数及其内部涨跌家数。
type IndexQuote struct {
	Code          string
	Name          string
	LastPrice     float64
	ChangePercent float64
	ChangeValue   float64
	UpCount       int
	DownCount     int
	FlatCount     int
}

// CLSMarketStatisticConfig 描述财联社市场统计 Provider 配置。
type CLSMarketStatisticConfig struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// CLSMarketStatisticProvider 使用财联社行情概览接口抓取市场涨跌统计。
type CLSMarketStatisticProvider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewCLSMarketStatisticProvider 创建财联社市场统计 Provider。
func NewCLSMarketStatisticProvider(config CLSMarketStatisticConfig) (*CLSMarketStatisticProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultMarketInfoTimeout
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         clsMarketStatisticProviderName,
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
	return &CLSMarketStatisticProvider{
		url:    marketInfoFirstNonEmpty(config.URL, defaultCLSMarketStatisticURL),
		client: client,
		now:    now,
	}, nil
}

// Name 返回财联社市场统计 Provider 稳定名称。
func (provider *CLSMarketStatisticProvider) Name() string {
	return clsMarketStatisticProviderName
}

// Status 返回财联社市场统计数据源状态。
func (provider *CLSMarketStatisticProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "Cailianpress x-quote market breadth endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// FetchMarketStatistic 抓取市场涨跌统计快照。
func (provider *CLSMarketStatisticProvider) FetchMarketStatistic(ctx context.Context, request MarketStatisticRequest) (MarketStatisticSnapshot, error) {
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Query: map[string]string{
			"app": "CailianpressWeb",
			"os":  "web",
			"sv":  "8.4.6",
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://www.cls.cn/",
		},
	})
	if err != nil {
		return MarketStatisticSnapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}

	var response clsMarketStatisticResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return MarketStatisticSnapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Code != 200 {
		return MarketStatisticSnapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("cls market statistic code %d: %s", response.Code, response.Message))
	}
	if len(response.Data.IndexQuote) == 0 {
		return MarketStatisticSnapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty cls market statistic data"))
	}
	snapshot := response.Data.snapshot()
	snapshot.Provider = provider.Name()
	snapshot.Source = provider.Status(ctx).Source
	snapshot.FetchedAt = result.FetchedAt
	return snapshot, nil
}

type clsMarketStatisticResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"msg"`
	Data    clsMarketStatisticData `json:"data"`
}

type clsMarketStatisticData struct {
	IndexQuote []struct {
		Code          string  `json:"secu_code"`
		Name          string  `json:"secu_name"`
		LastPrice     float64 `json:"last_px"`
		ChangePercent float64 `json:"change"`
		ChangeValue   float64 `json:"change_px"`
		UpCount       int     `json:"up_num"`
		DownCount     int     `json:"down_num"`
		FlatCount     int     `json:"flat_num"`
	} `json:"index_quote"`
	UpDownDis struct {
		UpNum       int     `json:"up_num"`
		DownNum     int     `json:"down_num"`
		AverageRise float64 `json:"average_rise"`
		RiseNum     int     `json:"rise_num"`
		FallNum     int     `json:"fall_num"`
		Down10      int     `json:"down_10"`
		Down8       int     `json:"down_8"`
		Down6       int     `json:"down_6"`
		Down4       int     `json:"down_4"`
		Down2       int     `json:"down_2"`
		FlatNum     int     `json:"flat_num"`
		Up2         int     `json:"up_2"`
		Up4         int     `json:"up_4"`
		Up6         int     `json:"up_6"`
		Up8         int     `json:"up_8"`
		Up10        int     `json:"up_10"`
		SuspendNum  int     `json:"suspend_num"`
		Status      bool    `json:"status"`
	} `json:"up_down_dis"`
}

// snapshot 将财联社响应清洗成统一市场统计快照。
func (data clsMarketStatisticData) snapshot() MarketStatisticSnapshot {
	indexQuotes := make([]IndexQuote, 0, len(data.IndexQuote))
	var shUpCount, shDownCount, szUpCount, szDownCount int
	for _, row := range data.IndexQuote {
		quote := IndexQuote{
			Code:          strings.TrimSpace(row.Code),
			Name:          strings.TrimSpace(row.Name),
			LastPrice:     row.LastPrice,
			ChangePercent: row.ChangePercent,
			ChangeValue:   row.ChangeValue,
			UpCount:       row.UpCount,
			DownCount:     row.DownCount,
			FlatCount:     row.FlatCount,
		}
		if quote.Code == "" && quote.Name == "" {
			continue
		}
		indexQuotes = append(indexQuotes, quote)
		if quote.Code == "sh000001" || quote.Name == "上证指数" {
			shUpCount = quote.UpCount
			shDownCount = quote.DownCount
		}
		if quote.Code == "sz399001" || quote.Name == "深证成指" {
			szUpCount = quote.UpCount
			szDownCount = quote.DownCount
		}
	}

	upCount := data.UpDownDis.RiseNum
	downCount := data.UpDownDis.FallNum
	limitUp := data.UpDownDis.UpNum
	limitDown := data.UpDownDis.DownNum

	upRatio := 0.0
	if total := upCount + downCount; total > 0 {
		upRatio = float64(upCount) / float64(total) * 100
	}
	upDownRatio := ratioOrNumerator(upCount, downCount)
	limitRatio := ratioOrNumerator(limitUp, limitDown)

	return MarketStatisticSnapshot{
		UpCount:       upCount,
		DownCount:     downCount,
		UpRatio:       upRatio,
		UpDownRatio:   upDownRatio,
		SentimentDesc: MarketSentimentDesc(upDownRatio),
		LimitUp:       limitUp,
		LimitDown:     limitDown,
		LimitRatio:    limitRatio,
		ShUpCount:     shUpCount,
		ShDownCount:   shDownCount,
		SzUpCount:     szUpCount,
		SzDownCount:   szDownCount,
		Distribution: UpDownDistribution{
			UpNum:       data.UpDownDis.UpNum,
			DownNum:     data.UpDownDis.DownNum,
			AverageRise: data.UpDownDis.AverageRise,
			RiseNum:     data.UpDownDis.RiseNum,
			FallNum:     data.UpDownDis.FallNum,
			Down10:      data.UpDownDis.Down10,
			Down8:       data.UpDownDis.Down8,
			Down6:       data.UpDownDis.Down6,
			Down4:       data.UpDownDis.Down4,
			Down2:       data.UpDownDis.Down2,
			FlatNum:     data.UpDownDis.FlatNum,
			Up2:         data.UpDownDis.Up2,
			Up4:         data.UpDownDis.Up4,
			Up6:         data.UpDownDis.Up6,
			Up8:         data.UpDownDis.Up8,
			Up10:        data.UpDownDis.Up10,
			SuspendNum:  data.UpDownDis.SuspendNum,
			Status:      data.UpDownDis.Status,
		},
		IndexQuotes: indexQuotes,
	}
}

// MarketSentimentDesc 根据上涨/下跌家数比输出市场情绪描述。
func MarketSentimentDesc(upDownRatio float64) string {
	switch {
	case upDownRatio >= 2:
		return "普涨(极强)"
	case upDownRatio >= 1.5:
		return "偏强"
	case upDownRatio > 1:
		return "稍强"
	case upDownRatio == 1:
		return "中性"
	case upDownRatio > 0.5:
		return "稍弱"
	case upDownRatio > 0:
		return "偏弱"
	default:
		return "普跌(冰点)"
	}
}

// ratioOrNumerator 计算分子/分母；分母为 0 时沿用原项目语义返回分子。
func ratioOrNumerator(numerator int, denominator int) float64 {
	if denominator > 0 {
		return float64(numerator) / float64(denominator)
	}
	if numerator > 0 {
		return float64(numerator)
	}
	return 0
}
