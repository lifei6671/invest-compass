package macro

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
	eastMoneyProviderName = "eastmoney-macro"
	defaultEastMoneyURL   = "https://datacenter-web.eastmoney.com/api/data/v1/get"
	defaultMacroTimeout   = 8 * time.Second
)

// Indicator 表示宏观指标类型。
type Indicator string

const (
	// IndicatorGDP 表示 GDP。
	IndicatorGDP Indicator = "gdp"
	// IndicatorCPI 表示 CPI。
	IndicatorCPI Indicator = "cpi"
	// IndicatorPPI 表示 PPI。
	IndicatorPPI Indicator = "ppi"
	// IndicatorPMI 表示 PMI。
	IndicatorPMI Indicator = "pmi"
)

// Provider 定义宏观指标 Provider 能力边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Fetch(ctx context.Context, request Request) (Dataset, error)
}

// ProviderStatus 描述宏观数据源状态。
type ProviderStatus struct {
	Name          string
	Source        string
	License       string
	RateLimit     string
	Available     bool
	LastCheckedAt time.Time
	LastError     string
}

// Request 是宏观指标查询请求。
type Request struct {
	Indicator Indicator
	Limit     int
}

// Dataset 表示宏观指标数据集。
type Dataset struct {
	Indicator Indicator
	Rows      []Row
	Provider  string
	Source    string
	FetchedAt time.Time
}

// Row 表示单期宏观指标。
type Row struct {
	ReportDate string
	Period     string
	Values     map[string]string
}

// ProviderError 是宏观 Provider 错误。
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
	return fmt.Sprintf("macro provider %s %s failed: %s", err.Provider, err.Operation, logger.RedactError(err.Cause))
}

// Unwrap 返回底层错误。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建宏观 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{Provider: provider, Operation: operation, Cause: cause}
}

// EastMoneyConfig 描述东财宏观 Provider 运行配置。
type EastMoneyConfig struct {
	URL        string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyProvider 使用东方财富宏观数据接口抓取 GDP/CPI/PPI/PMI。
type EastMoneyProvider struct {
	url    string
	client *crawler.Client
	now    func() time.Time
}

// NewEastMoneyProvider 创建东财宏观 Provider。
func NewEastMoneyProvider(config EastMoneyConfig) (*EastMoneyProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultMacroTimeout
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         eastMoneyProviderName,
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
	return &EastMoneyProvider{
		url:    macroFirstNonEmpty(config.URL, defaultEastMoneyURL),
		client: client,
		now:    now,
	}, nil
}

// Name 返回东财宏观 Provider 稳定名称。
func (provider *EastMoneyProvider) Name() string {
	return eastMoneyProviderName
}

// Status 返回东财宏观数据源状态。
func (provider *EastMoneyProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney datacenter macro endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 抓取宏观指标数据集。
func (provider *EastMoneyProvider) Fetch(ctx context.Context, request Request) (Dataset, error) {
	spec, ok := macroSpecs[request.Indicator]
	if !ok {
		return Dataset{}, NewProviderError(provider.Name(), "indicator", fmt.Errorf("unsupported macro indicator %q", request.Indicator))
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.url,
		Query: map[string]string{
			"callback":    "data",
			"columns":     spec.Columns,
			"pageNumber":  "1",
			"pageSize":    strconv.Itoa(limit),
			"sortColumns": "REPORT_DATE",
			"sortTypes":   "-1",
			"source":      "WEB",
			"client":      "WEB",
			"reportName":  spec.ReportName,
			"p":           "1",
			"pageNo":      "1",
			"pageNum":     "1",
			"_":           strconv.FormatInt(provider.now().Unix(), 10),
		},
		Headers: map[string]string{
			"Accept":  "*/*",
			"Referer": "https://data.eastmoney.com/cjsj/gdp.html",
		},
	})
	if err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "fetch", err)
	}
	payload, err := macroExtractJSONP(result.Body)
	if err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "jsonp_decode", err)
	}
	var response struct {
		Success *bool  `json:"success"`
		Code    *int   `json:"code"`
		Message string `json:"message"`
		Result  *struct {
			Data []map[string]any `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "decode", err)
	}
	if response.Success != nil && !*response.Success {
		return Dataset{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney macro success=false: %s", response.Message))
	}
	if response.Code != nil && *response.Code != 0 && *response.Code != 200 {
		return Dataset{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney macro code %d: %s", *response.Code, response.Message))
	}
	if response.Result == nil || len(response.Result.Data) == 0 {
		return Dataset{}, NewProviderError(provider.Name(), "empty_data", fmt.Errorf("empty eastmoney macro data"))
	}
	rows := make([]Row, 0, len(response.Result.Data))
	for _, item := range response.Result.Data {
		values := make(map[string]string, len(item))
		for key, value := range item {
			values[key] = fmt.Sprint(value)
		}
		rows = append(rows, Row{
			ReportDate: values["REPORT_DATE"],
			Period:     values["TIME"],
			Values:     values,
		})
	}
	return Dataset{
		Indicator: request.Indicator,
		Rows:      rows,
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

type macroSpec struct {
	ReportName string
	Columns    string
}

var macroSpecs = map[Indicator]macroSpec{
	IndicatorGDP: {ReportName: "RPT_ECONOMY_GDP", Columns: "REPORT_DATE,TIME,DOMESTICL_PRODUCT_BASE,FIRST_PRODUCT_BASE,SECOND_PRODUCT_BASE,THIRD_PRODUCT_BASE,SUM_SAME,FIRST_SAME,SECOND_SAME,THIRD_SAME"},
	IndicatorCPI: {ReportName: "RPT_ECONOMY_CPI", Columns: "REPORT_DATE,TIME,NATIONAL_SAME,NATIONAL_BASE,NATIONAL_SEQUENTIAL,NATIONAL_ACCUMULATE,CITY_SAME,CITY_BASE,CITY_SEQUENTIAL,CITY_ACCUMULATE,RURAL_SAME,RURAL_BASE,RURAL_SEQUENTIAL,RURAL_ACCUMULATE"},
	IndicatorPPI: {ReportName: "RPT_ECONOMY_PPI", Columns: "REPORT_DATE,TIME,BASE,BASE_SAME,BASE_ACCUMULATE"},
	IndicatorPMI: {ReportName: "RPT_ECONOMY_PMI", Columns: "REPORT_DATE,TIME,MAKE_INDEX,MAKE_SAME,NMAKE_INDEX,NMAKE_SAME"},
}

// macroExtractJSONP 从 JSONP 响应中提取 JSON。
func macroExtractJSONP(body string) (string, error) {
	start := strings.Index(body, "(")
	end := strings.LastIndex(body, ")")
	if start < 0 || end <= start {
		return "", fmt.Errorf("invalid jsonp response")
	}
	return body[start+1 : end], nil
}

// macroFirstNonEmpty 返回第一个非空字符串。
func macroFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
