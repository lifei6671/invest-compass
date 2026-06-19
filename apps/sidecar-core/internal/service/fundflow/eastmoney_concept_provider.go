package fundflow

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
	eastMoneyConceptProviderName = "eastmoney-concept-fundflow"
	defaultEastMoneyConceptURL   = "https://data.eastmoney.com/dataapi/bkzj/getbkzj"
	defaultEastMoneyFlowTimeout  = 8 * time.Second
)

// EastMoneyConceptConfig 描述东方财富概念资金流 Provider 的可替换运行参数。
type EastMoneyConceptConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyConceptProvider 使用东方财富板块资金接口获取概念板块主力净流入快照。
type EastMoneyConceptProvider struct {
	baseURL string
	client  *crawler.Client
	now     func() time.Time
}

// NewEastMoneyConceptProvider 创建东方财富概念资金流 Provider。
func NewEastMoneyConceptProvider(config EastMoneyConceptConfig) (*EastMoneyConceptProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultEastMoneyFlowTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", eastMoneyConceptProviderName)
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         eastMoneyConceptProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 1024 * 1024,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		},
	})
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &EastMoneyConceptProvider{
		baseURL: firstNonEmpty(config.BaseURL, defaultEastMoneyConceptURL),
		client:  client,
		now:     now,
	}, nil
}

// Name 返回东方财富概念资金流 Provider 稳定名称。
func (provider *EastMoneyConceptProvider) Name() string {
	return eastMoneyConceptProviderName
}

// Status 返回东方财富概念资金流来源、授权边界和限频说明，不做阻塞式远程探测。
func (provider *EastMoneyConceptProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney bkzj public concept fund flow endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
		SupportedScopes: []Scope{
			ScopeConcept,
		},
	}
}

// FetchSnapshot 获取概念板块资金流快照。
//
// 当前实现只支持 ScopeConcept。行业板块资金流应由独立 Provider 或组合 Provider 承载，
// 避免 UI 把概念和行业资金流混为同一数据集。
func (provider *EastMoneyConceptProvider) FetchSnapshot(ctx context.Context, request Request) (Snapshot, error) {
	scope := request.Scope
	if scope == "" {
		scope = ScopeConcept
	}
	if scope != ScopeConcept {
		return Snapshot{}, NewProviderError(provider.Name(), "scope", fmt.Errorf("unsupported fund flow scope %q", request.Scope))
	}

	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.baseURL,
		Query: map[string]string{
			"key":  "f62",
			"code": "m:90+t:3",
		},
		Headers: map[string]string{
			"Accept":          "application/json,text/plain,*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Referer":         "https://data.eastmoney.com/",
		},
	})
	if err != nil {
		return Snapshot{}, NewProviderError(provider.Name(), "fetch", err)
	}

	response, err := decodeEastMoneyFundFlowResponse(result.BodyBytes)
	if err != nil {
		return Snapshot{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.RC != 0 {
		return Snapshot{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney fundflow rc %d", response.RC))
	}
	if len(response.Data.Diff) == 0 {
		return Snapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney concept fundflow diff"))
	}
	items := make([]Item, 0, len(response.Data.Diff))
	for _, row := range response.Data.Diff {
		code := strings.TrimSpace(row.Code)
		name := strings.TrimSpace(row.Name)
		if code == "" || name == "" {
			continue
		}
		items = append(items, Item{
			Code:      code,
			Name:      name,
			MarketID:  row.MarketID,
			NetInflow: row.NetInflow,
		})
	}
	if len(items) == 0 {
		return Snapshot{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty valid eastmoney concept fundflow items"))
	}
	limit := request.Limit
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return Snapshot{
		Scope:     ScopeConcept,
		Total:     response.Data.Total,
		Items:     items,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

type eastMoneyFundFlowResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			Code      string  `json:"f12"`
			MarketID  int     `json:"f13"`
			Name      string  `json:"f14"`
			NetInflow float64 `json:"f62"`
		} `json:"diff"`
	} `json:"data"`
}

// decodeEastMoneyFundFlowResponse 解析东财资金流 JSON 响应。
func decodeEastMoneyFundFlowResponse(payload []byte) (eastMoneyFundFlowResponse, error) {
	var response eastMoneyFundFlowResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return eastMoneyFundFlowResponse{}, err
	}
	return response, nil
}

// firstNonEmpty 返回第一个非空字符串，用于合并默认配置。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
