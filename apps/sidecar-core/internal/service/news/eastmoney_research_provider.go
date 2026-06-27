package news

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	eastMoneyResearchProviderName   = "eastmoney-research"
	defaultEastMoneyStockReportURL  = "https://reportapi.eastmoney.com/report/list2"
	defaultEastMoneyIndustryURL     = "https://reportapi.eastmoney.com/report/list"
	defaultEastMoneyAnnouncementURL = "https://np-anotice-stock.eastmoney.com/api/security/ann"
	defaultEastMoneyResearchDays    = 365
)

var eastMoneyLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// EastMoneyResearchConfig 描述东方财富研报、行业研究和公司公告 Provider 的运行配置。
type EastMoneyResearchConfig struct {
	StockReportURL    string
	IndustryReportURL string
	AnnouncementURL   string
	HTTPClient        *http.Client
	Timeout           time.Duration
}

// EastMoneyResearchProvider 抓取东方财富研报、行业研究和公告并清洗为统一资讯模型。
type EastMoneyResearchProvider struct {
	stockReportURL    string
	industryReportURL string
	announcementURL   string
	client            *crawler.Client
}

// NewEastMoneyResearchProvider 创建东方财富研究资讯 Provider。
func NewEastMoneyResearchProvider(config EastMoneyResearchConfig) (*EastMoneyResearchProvider, error) {
	client, err := newNewsCrawler(eastMoneyResearchProviderName, config.Timeout, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &EastMoneyResearchProvider{
		stockReportURL:    newsFirstNonEmpty(config.StockReportURL, defaultEastMoneyStockReportURL),
		industryReportURL: newsFirstNonEmpty(config.IndustryReportURL, defaultEastMoneyIndustryURL),
		announcementURL:   newsFirstNonEmpty(config.AnnouncementURL, defaultEastMoneyAnnouncementURL),
		client:            client,
	}, nil
}

// Name 返回东方财富研究资讯 Provider 稳定名称。
func (provider *EastMoneyResearchProvider) Name() string {
	return eastMoneyResearchProviderName
}

// Status 返回东方财富研报与公告源状态说明。
func (provider *EastMoneyResearchProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    "东方财富研报、行业研究与公司公告接口",
		Available: provider != nil,
	}
}

// List 返回指定 A 股股票的个股研报和公司公告。
func (provider *EastMoneyResearchProvider) List(ctx context.Context, request ListRequest) ([]Item, error) {
	if request.Symbol.Market != "CN" {
		return []Item{}, nil
	}
	items, err := provider.collectResearch(ctx, request.Symbol.Code, false, request.Limit)
	if err != nil {
		return nil, err
	}
	return sortedLimitedItems(items, request.Limit)
}

// Market 返回东方财富市场级研究线索，包含最新个股研报、行业研究和公司公告。
func (provider *EastMoneyResearchProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	if request.Market != "" && strings.ToUpper(strings.TrimSpace(request.Market)) != "CN" {
		return []Item{}, nil
	}
	items, err := provider.collectResearch(ctx, "", true, request.Limit)
	if err != nil {
		return nil, err
	}
	return sortedLimitedItems(items, request.Limit)
}

func (provider *EastMoneyResearchProvider) collectResearch(ctx context.Context, stockCode string, includeIndustry bool, limit int) ([]Item, error) {
	type result struct {
		items []Item
		err   error
	}
	fetchers := []func() ([]Item, error){
		func() ([]Item, error) { return provider.fetchStockReports(ctx, stockCode, limit) },
		func() ([]Item, error) { return provider.fetchAnnouncements(ctx, stockCode, limit) },
	}
	if includeIndustry {
		fetchers = append(fetchers, func() ([]Item, error) { return provider.fetchIndustryReports(ctx, limit) })
	}

	resultCh := make(chan result, len(fetchers))
	var wg sync.WaitGroup
	for _, fetch := range fetchers {
		wg.Add(1)
		go func(fetch func() ([]Item, error)) {
			defer wg.Done()
			items, err := fetch()
			resultCh <- result{items: items, err: err}
		}(fetch)
	}
	wg.Wait()
	close(resultCh)

	var items []Item
	var firstErr error
	for result := range resultCh {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		items = append(items, result.items...)
	}
	if len(items) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return items, nil
}

func (provider *EastMoneyResearchProvider) fetchStockReports(ctx context.Context, stockCode string, limit int) ([]Item, error) {
	var response eastMoneyReportResponse
	_, err := provider.client.PostJSON(ctx, crawler.Request{
		URL:     provider.stockReportURL,
		Headers: eastMoneyReportHeaders(),
	}, eastMoneyStockReportRequest{
		Code:         stockCode,
		IndustryCode: "*",
		BeginTime:    eastMoneyBeginDate(),
		EndTime:      eastMoneyEndDate(),
		PageNo:       1,
		PageSize:     eastMoneyPageSize(limit),
		P:            1,
		PageNum:      1,
		PageNumber:   1,
	}, &response)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "stock_report_fetch", err)
	}
	items := make([]Item, 0, len(response.Data))
	for _, row := range response.Data {
		item, ok := row.StockReportItem()
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func (provider *EastMoneyResearchProvider) fetchIndustryReports(ctx context.Context, limit int) ([]Item, error) {
	var response eastMoneyReportResponse
	_, err := provider.client.FetchJSON(ctx, crawler.Request{
		URL: provider.industryReportURL,
		Query: map[string]string{
			"industry":     "*",
			"industryCode": "",
			"beginTime":    eastMoneyBeginDate(),
			"endTime":      eastMoneyEndDate(),
			"pageNo":       "1",
			"pageSize":     strconv.Itoa(eastMoneyPageSize(limit)),
			"p":            "1",
			"pageNum":      "1",
			"pageNumber":   "1",
			"qType":        "1",
		},
		Headers: eastMoneyReportHeaders(),
	}, &response)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "industry_report_fetch", err)
	}
	items := make([]Item, 0, len(response.Data))
	for _, row := range response.Data {
		item, ok := row.IndustryReportItem()
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func (provider *EastMoneyResearchProvider) fetchAnnouncements(ctx context.Context, stockCode string, limit int) ([]Item, error) {
	var response eastMoneyAnnouncementResponse
	_, err := provider.client.FetchJSON(ctx, crawler.Request{
		URL: provider.announcementURL,
		Query: map[string]string{
			"page_size":     strconv.Itoa(eastMoneyPageSize(limit)),
			"page_index":    "1",
			"ann_type":      "SHA,CYB,SZA,BJA,INV",
			"client_source": "web",
			"f_node":        "0",
			"stock_list":    stockCode,
		},
		Headers: map[string]string{
			"Host":    "np-anotice-stock.eastmoney.com",
			"Referer": "https://data.eastmoney.com/notices/hsa/5.html",
			"Accept":  "application/json",
		},
	}, &response)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "announcement_fetch", err)
	}
	items := make([]Item, 0, len(response.Data.List))
	for _, row := range response.Data.List {
		item, ok := row.Item()
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

type eastMoneyStockReportRequest struct {
	BeginTime    string `json:"beginTime"`
	EndTime      string `json:"endTime"`
	IndustryCode string `json:"industryCode"`
	Code         string `json:"code"`
	PageSize     int    `json:"pageSize"`
	PageNo       int    `json:"pageNo"`
	P            int    `json:"p"`
	PageNum      int    `json:"pageNum"`
	PageNumber   int    `json:"pageNumber"`
}

type eastMoneyReportResponse struct {
	Data []eastMoneyReportRow `json:"data"`
}

type eastMoneyReportRow struct {
	InfoCode     string `json:"infoCode"`
	Title        string `json:"title"`
	StockCode    string `json:"stockCode"`
	StockName    string `json:"stockName"`
	IndustryName string `json:"industryName"`
	IndvInduName string `json:"indvInduName"`
	EMRatingName string `json:"emRatingName"`
	SRatingName  string `json:"sRatingName"`
	Researcher   string `json:"researcher"`
	OrgSName     string `json:"orgSName"`
	PublishDate  string `json:"publishDate"`
}

// StockReportItem 将东方财富个股研报行转换为统一资讯模型。
func (row eastMoneyReportRow) StockReportItem() (Item, bool) {
	title := strings.TrimSpace(row.Title)
	if title == "" {
		return Item{}, false
	}
	symbols := symbolsFromStockCode(row.StockCode)
	tags := uniqueNonEmpty("个股研报", row.StockName, row.IndvInduName, row.EMRatingName, row.SRatingName, row.OrgSName)
	return Item{
		Source:      "东方财富研报",
		Title:       title,
		URL:         eastMoneyPDFURL(row.InfoCode),
		Summary:     joinSummary("机构", row.OrgSName, "分析师", row.Researcher, "评级", firstNonEmpty(row.EMRatingName, row.SRatingName), "行业", row.IndvInduName),
		PublishedAt: parseEastMoneyTime(row.PublishDate),
		Symbols:     symbols,
		Tags:        tags,
	}, true
}

// IndustryReportItem 将东方财富行业研究行转换为统一资讯模型。
func (row eastMoneyReportRow) IndustryReportItem() (Item, bool) {
	title := strings.TrimSpace(row.Title)
	if title == "" {
		return Item{}, false
	}
	industry := firstNonEmpty(row.IndustryName, row.IndvInduName)
	tags := uniqueNonEmpty("行业研究", industry, row.OrgSName)
	return Item{
		Source:      "东方财富行业研究",
		Title:       title,
		URL:         eastMoneyPDFURL(row.InfoCode),
		Summary:     joinSummary("机构", row.OrgSName, "行业", industry),
		PublishedAt: parseEastMoneyTime(row.PublishDate),
		Tags:        tags,
	}, true
}

type eastMoneyAnnouncementResponse struct {
	Data struct {
		List []eastMoneyAnnouncementRow `json:"list"`
	} `json:"data"`
}

type eastMoneyAnnouncementRow struct {
	ArtCode    string                        `json:"art_code"`
	ArtCodeAlt string                        `json:"artCode"`
	Title      string                        `json:"title"`
	NoticeDate string                        `json:"notice_date"`
	AttachURL  string                        `json:"attach_url"`
	URL        string                        `json:"url"`
	Columns    []eastMoneyAnnouncementColumn `json:"columns"`
	Codes      []eastMoneyAnnouncementCode   `json:"codes"`
}

type eastMoneyAnnouncementColumn struct {
	ColumnName string `json:"column_name"`
}

type eastMoneyAnnouncementCode struct {
	StockCode string `json:"stock_code"`
	ShortName string `json:"short_name"`
}

// Item 将东方财富公司公告行转换为统一资讯模型。
func (row eastMoneyAnnouncementRow) Item() (Item, bool) {
	title := strings.TrimSpace(row.Title)
	if title == "" {
		return Item{}, false
	}
	symbols := make([]stock.Symbol, 0, len(row.Codes))
	tags := []string{"公司公告"}
	for _, column := range row.Columns {
		tags = append(tags, column.ColumnName)
	}
	for _, code := range row.Codes {
		if symbol, ok := symbolFromCNStockCode(code.StockCode); ok {
			symbols = append(symbols, symbol)
		}
		tags = append(tags, code.ShortName)
	}
	return Item{
		Source:      "东方财富公告",
		Title:       title,
		URL:         row.NewsURL(),
		Summary:     joinSummary("公告类型", row.firstColumnName(), "关联股票", row.stockNames()),
		PublishedAt: parseEastMoneyTime(row.NoticeDate),
		Symbols:     symbols,
		Tags:        uniqueNonEmpty(tags...),
	}, true
}

// NewsURL 返回公告原文链接，优先使用远端显式链接，否则按东方财富详情页规则生成。
func (row eastMoneyAnnouncementRow) NewsURL() string {
	if url := strings.TrimSpace(firstNonEmpty(row.URL, row.AttachURL)); url != "" {
		return url
	}
	artCode := firstNonEmpty(row.ArtCode, row.ArtCodeAlt)
	if artCode == "" || len(row.Codes) == 0 {
		return ""
	}
	stockCode := strings.TrimSpace(row.Codes[0].StockCode)
	if stockCode == "" {
		return ""
	}
	return fmt.Sprintf("https://data.eastmoney.com/notices/detail/%s/%s.html", stockCode, artCode)
}

func (row eastMoneyAnnouncementRow) firstColumnName() string {
	if len(row.Columns) == 0 {
		return ""
	}
	return row.Columns[0].ColumnName
}

func (row eastMoneyAnnouncementRow) stockNames() string {
	names := make([]string, 0, len(row.Codes))
	for _, code := range row.Codes {
		if strings.TrimSpace(code.ShortName) != "" {
			names = append(names, strings.TrimSpace(code.ShortName))
		}
	}
	return strings.Join(names, "、")
}

func eastMoneyReportHeaders() map[string]string {
	return map[string]string{
		"Host":         "reportapi.eastmoney.com",
		"Origin":       "https://data.eastmoney.com",
		"Referer":      "https://data.eastmoney.com/report/stock.jshtml",
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
}

func eastMoneyBeginDate() string {
	return time.Now().Add(-defaultEastMoneyResearchDays * 24 * time.Hour).Format("2006-01-02")
}

func eastMoneyEndDate() string {
	return time.Now().Format("2006-01-02")
}

func eastMoneyPageSize(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func eastMoneyPDFURL(infoCode string) string {
	infoCode = strings.TrimSpace(infoCode)
	if infoCode == "" {
		return ""
	}
	return fmt.Sprintf("https://pdf.dfcfw.com/pdf/H3_%s_1.pdf", infoCode)
}

func parseEastMoneyTime(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, trimmed, eastMoneyLocation); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func symbolsFromStockCode(code string) []stock.Symbol {
	if symbol, ok := symbolFromCNStockCode(code); ok {
		return []stock.Symbol{symbol}
	}
	return nil
}

func symbolFromCNStockCode(code string) (stock.Symbol, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return stock.Symbol{}, false
	}
	exchange := "SZ"
	if strings.HasPrefix(code, "6") {
		exchange = "SH"
	}
	symbol, err := stock.ParseSymbol("CN:" + exchange + ":" + code)
	if err != nil {
		return stock.Symbol{}, false
	}
	return symbol, true
}

func joinSummary(parts ...string) string {
	if len(parts)%2 != 0 {
		return ""
	}
	values := make([]string, 0, len(parts)/2)
	for index := 0; index < len(parts); index += 2 {
		label := strings.TrimSpace(parts[index])
		value := strings.TrimSpace(parts[index+1])
		if label != "" && value != "" {
			values = append(values, label+"："+value)
		}
	}
	return strings.Join(values, "；")
}

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) > 1 {
		sort.Strings(result[1:])
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
