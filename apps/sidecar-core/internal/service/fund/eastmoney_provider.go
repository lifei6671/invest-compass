package fund

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
	eastMoneyProviderName       = "eastmoney-fund"
	defaultEastMoneyFundTimeout = 8 * time.Second
	defaultSearchURL            = "https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx"
	defaultBasicURLPattern      = "https://fund.eastmoney.com/%s.html"
	defaultHistoryURL           = "https://api.fund.eastmoney.com/f10/lsjz"
	defaultRankingURL           = "https://fund.eastmoney.com/data/rankhandler.aspx"
	defaultHoldingURL           = "https://fundf10.eastmoney.com/FundArchivesDatas.aspx"
)

var fundCodePattern = regexp.MustCompile(`^\d{6}$`)

// EastMoneyConfig 描述东方财富基金 Provider 的可替换运行参数。
type EastMoneyConfig struct {
	SearchURL       string
	BasicURLPattern string
	HistoryURL      string
	RankingURL      string
	HoldingURL      string
	HTTPClient      *http.Client
	Timeout         time.Duration
	Now             func() time.Time
}

// EastMoneyProvider 使用东方财富公开网页接口抓取基金数据。
type EastMoneyProvider struct {
	searchURL       string
	basicURLPattern string
	historyURL      string
	rankingURL      string
	holdingURL      string
	client          *crawler.Client
	now             func() time.Time
}

// NewEastMoneyProvider 创建东方财富基金 Provider。
func NewEastMoneyProvider(config EastMoneyConfig) (*EastMoneyProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultEastMoneyFundTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", eastMoneyProviderName)
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         eastMoneyProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 2 * 1024 * 1024,
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
		searchURL:       firstNonEmpty(config.SearchURL, defaultSearchURL),
		basicURLPattern: firstNonEmpty(config.BasicURLPattern, defaultBasicURLPattern),
		historyURL:      firstNonEmpty(config.HistoryURL, defaultHistoryURL),
		rankingURL:      firstNonEmpty(config.RankingURL, defaultRankingURL),
		holdingURL:      firstNonEmpty(config.HoldingURL, defaultHoldingURL),
		client:          client,
		now:             now,
	}, nil
}

// Name 返回东方财富基金 Provider 稳定名称。
func (provider *EastMoneyProvider) Name() string {
	return eastMoneyProviderName
}

// Status 返回基金数据源状态说明，不做阻塞式远程探测。
func (provider *EastMoneyProvider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney public fund web endpoints",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// SearchFunds 搜索基金代码和基础类型，不写入本地库。
func (provider *EastMoneyProvider) SearchFunds(ctx context.Context, request SearchRequest) ([]SearchItem, error) {
	keyword := strings.TrimSpace(request.Keyword)
	if keyword == "" {
		return []SearchItem{}, nil
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.searchURL,
		Query: map[string]string{
			"callback": "",
			"m":        "1",
			"key":      keyword,
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": "https://fund.eastmoney.com/",
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "search_fetch", err)
	}

	var response eastMoneySearchResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "search_decode", err)
	}
	items := make([]SearchItem, 0, len(response.Datas))
	for _, row := range response.Datas {
		if row.FundBaseInfo == nil {
			continue
		}
		code := strings.TrimSpace(firstNonEmpty(row.Code, row.FundBaseInfo.Code))
		name := strings.TrimSpace(firstNonEmpty(row.Name, row.FundBaseInfo.ShortName))
		if code == "" || name == "" {
			continue
		}
		items = append(items, SearchItem{
			Code: code,
			Name: name,
			Type: strings.TrimSpace(row.FundBaseInfo.Type),
		})
	}
	if request.Limit > 0 && request.Limit < len(items) {
		items = items[:request.Limit]
	}
	return items, nil
}

// FetchBasic 抓取基金基础资料页面并清洗核心字段。
func (provider *EastMoneyProvider) FetchBasic(ctx context.Context, request BasicRequest) (Basic, error) {
	code, err := normalizeFundCode(request.Code)
	if err != nil {
		return Basic{}, NewProviderError(provider.Name(), "validate", err)
	}
	result, err := provider.client.FetchHTML(ctx, crawler.Request{
		URL: fmt.Sprintf(provider.basicURLPattern, code),
		Headers: map[string]string{
			"Accept":  "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Referer": "https://fund.eastmoney.com/",
		},
	})
	if err != nil {
		return Basic{}, NewProviderError(provider.Name(), "basic_fetch", err)
	}

	basic, err := parseBasicHTML(code, result.Body)
	if err != nil {
		return Basic{}, NewProviderError(provider.Name(), "basic_decode", err)
	}
	basic.Provider = provider.Name()
	basic.Source = provider.Status(ctx).Source
	basic.FetchedAt = result.FetchedAt
	return basic, nil
}

// FetchHistoryNetValues 抓取场外基金历史净值。
//
// 场内基金 K 线已由行情 Provider 承担，本方法不重复封装场内基金 K 线逻辑。
func (provider *EastMoneyProvider) FetchHistoryNetValues(ctx context.Context, request HistoryRequest) ([]HistoryNetValue, error) {
	code, err := normalizeFundCode(request.Code)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "validate", err)
	}
	pageIndex := request.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 30
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.historyURL,
		Query: map[string]string{
			"fundCode":  code,
			"pageIndex": strconv.Itoa(pageIndex),
			"pageSize":  strconv.Itoa(pageSize),
			"startDate": strings.TrimSpace(request.StartDate),
			"endDate":   strings.TrimSpace(request.EndDate),
			"_":         strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "application/json,text/plain,*/*",
			"Referer": fmt.Sprintf("https://fundf10.eastmoney.com/jjjz_%s.html", code),
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "history_fetch", err)
	}

	var response eastMoneyHistoryResponse
	if err := json.Unmarshal(result.BodyBytes, &response); err != nil {
		return nil, NewProviderError(provider.Name(), "history_decode", err)
	}
	values := make([]HistoryNetValue, 0, len(response.Data.List))
	for index, row := range response.Data.List {
		value, err := historyNetValueFromRow(row.Date, row.NetValue, row.AccumulatedValue, row.DailyGrowth, row.BuyStatus, row.SellStatus)
		if err != nil {
			return nil, NewProviderError(provider.Name(), "history_decode", fmt.Errorf("history row %d: %w", index, err))
		}
		values = append(values, value)
	}
	return values, nil
}

// FetchRanking 抓取基金排行伪 JS 响应并清洗为结构化结果。
func (provider *EastMoneyProvider) FetchRanking(ctx context.Context, request RankingRequest) (RankingResult, error) {
	request = normalizeRankingRequest(request)
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.rankingURL,
		Query: map[string]string{
			"op": "ph",
			"dt": request.MarketType,
			"ft": request.FundType,
			"rs": "",
			"gs": "0",
			"sc": request.SortField,
			"st": request.SortOrder,
			"sd": "",
			"ed": "",
			"pi": strconv.Itoa(request.PageIndex),
			"pn": strconv.Itoa(request.PageSize),
			"v":  strconv.FormatInt(provider.now().UnixMilli(), 10),
		},
		Headers: map[string]string{
			"Accept":  "*/*",
			"Referer": rankingReferer(request.MarketType),
		},
	})
	if err != nil {
		return RankingResult{}, NewProviderError(provider.Name(), "ranking_fetch", err)
	}

	ranking, err := parseRankingBody(result.Body, request)
	if err != nil {
		return RankingResult{}, NewProviderError(provider.Name(), "ranking_decode", err)
	}
	ranking.Provider = provider.Name()
	ranking.Source = provider.Status(ctx).Source
	ranking.FetchedAt = result.FetchedAt
	return ranking, nil
}

// FetchTopHoldings 抓取基金十大持仓，不额外请求股票行情。
func (provider *EastMoneyProvider) FetchTopHoldings(ctx context.Context, request HoldingRequest) ([]HoldingStock, error) {
	code, err := normalizeFundCode(request.Code)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "validate", err)
	}
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL: provider.holdingURL,
		Query: map[string]string{
			"type":    "jjcc",
			"code":    code,
			"topline": "10",
			"year":    "",
			"month":   "",
			"rt":      fmt.Sprintf("%.3f", float64(provider.now().UnixMilli())/1000.0),
		},
		Headers: map[string]string{
			"Accept":  "*/*",
			"Referer": fmt.Sprintf("https://fundf10.eastmoney.com/ccmx_%s.html", code),
		},
	})
	if err != nil {
		return nil, NewProviderError(provider.Name(), "holding_fetch", err)
	}
	holdings, err := parseHoldingsBody(result.Body)
	if err != nil {
		return nil, NewProviderError(provider.Name(), "holding_decode", err)
	}
	return holdings, nil
}

type eastMoneySearchResponse struct {
	Datas []struct {
		Code         string `json:"CODE"`
		Name         string `json:"NAME"`
		FundBaseInfo *struct {
			Code      string `json:"FCODE"`
			ShortName string `json:"SHORTNAME"`
			Type      string `json:"FTYPE"`
		} `json:"FundBaseInfo"`
	} `json:"Datas"`
}

type eastMoneyHistoryResponse struct {
	Data struct {
		List []struct {
			Date             string `json:"FSRQ"`
			NetValue         string `json:"DWJZ"`
			AccumulatedValue string `json:"LJJZ"`
			DailyGrowth      string `json:"JZZZL"`
			BuyStatus        string `json:"SGZT"`
			SellStatus       string `json:"SHZT"`
		} `json:"LSJZList"`
	} `json:"Data"`
}

// parseBasicHTML 清洗东财基金基础资料 HTML。
func parseBasicHTML(code string, body string) (Basic, error) {
	text := cleanHTMLText(body)
	basic := Basic{
		Code:          code,
		Name:          cleanFundName(findClassText(body, "fundDetail-tit")),
		Type:          labelValue(text, "基金类型", "类型"),
		EstablishedAt: labelValue(text, "成立日期", "成立日"),
		Scale:         labelValue(text, "基金规模", "规模"),
		Company:       labelValue(text, "管理人", "基金公司"),
		Manager:       labelValue(text, "基金经理", "经理人"),
		Rating:        labelValue(text, "基金评级", "评级"),
		Month1Growth:  percentLabelValue(text, "近1月"),
		Month3Growth:  percentLabelValue(text, "近3月"),
		Month6Growth:  percentLabelValue(text, "近6月"),
		Year1Growth:   percentLabelValue(text, "近1年"),
		Year3Growth:   percentLabelValue(text, "近3年"),
		Year5Growth:   percentLabelValue(text, "近5年"),
		YTDGrowth:     percentLabelValue(text, "今年来"),
		AllGrowth:     percentLabelValue(text, "成立来"),
	}
	if basic.Name == "" {
		return Basic{}, fmt.Errorf("missing fund name")
	}
	return basic, nil
}

// parseRankingBody 解析东财 rankhandler 伪 JS 响应。
func parseRankingBody(body string, request RankingRequest) (RankingResult, error) {
	datasContent, err := extractBracketSection(body, "datas:[")
	if err != nil {
		return RankingResult{}, err
	}
	totalCount := extractIntValue(body, `allRecords:(\d+)`)
	totalPages := extractIntValue(body, `allPages:(\d+)`)
	recordPattern := regexp.MustCompile(`"([^"]*)"`)
	records := recordPattern.FindAllStringSubmatch(datasContent, -1)
	items := make([]RankingItem, 0, len(records))
	for _, record := range records {
		if len(record) < 2 {
			return RankingResult{}, fmt.Errorf("malformed ranking record")
		}
		fields := strings.Split(record[1], ",")
		if len(fields) < 17 {
			return RankingResult{}, fmt.Errorf("ranking record has %d fields", len(fields))
		}
		item := RankingItem{
			Code:             fields[0],
			Name:             fields[1],
			Pinyin:           fields[2],
			NetValueDate:     fields[3],
			NetUnitValue:     parseFloatPtr(fields[4]),
			NetAccumulated:   parseFloatPtr(fields[5]),
			DailyGrowth:      parseFloatPtr(fields[6]),
			WeekGrowth:       parseFloatPtr(fields[7]),
			MonthGrowth:      parseFloatPtr(fields[8]),
			ThreeMonthGrowth: parseFloatPtr(fields[9]),
			SixMonthGrowth:   parseFloatPtr(fields[10]),
			YearGrowth:       parseFloatPtr(fields[11]),
			TwoYearGrowth:    parseFloatPtr(fields[12]),
			ThreeYearGrowth:  parseFloatPtr(fields[13]),
			YTDGrowth:        parseFloatPtr(fields[14]),
			SinceInception:   parseFloatPtr(fields[15]),
			EstablishDate:    fields[16],
		}
		if request.MarketType == "kf" && len(fields) >= 21 {
			item.Purchasable = fields[17] == "1"
			item.Scale = parseFloatPtr(fields[18])
			item.PurchaseRate = parseFloatPtr(fields[19])
			item.DiscountRate = parseFloatPtr(fields[20])
		}
		if request.MarketType == "fb" && len(fields) >= 23 {
			item.FundTypeDetail = fields[21]
			item.Scale = parseFloatPtr(fields[22])
		}
		items = append(items, item)
	}
	if totalCount > 0 && len(items) == 0 {
		return RankingResult{}, fmt.Errorf("empty valid ranking rows")
	}
	return RankingResult{
		Items:      items,
		TotalCount: totalCount,
		PageIndex:  request.PageIndex,
		PageSize:   request.PageSize,
		TotalPages: totalPages,
	}, nil
}

// parseHoldingsBody 解析东财基金持仓 JS 包裹体。
func parseHoldingsBody(body string) ([]HoldingStock, error) {
	htmlContent := extractJSStringProperty(body, "content")
	if htmlContent == "" {
		return nil, fmt.Errorf("missing holding content")
	}
	quarter := ""
	if matches := regexp.MustCompile(`(\d{4})[年-](\d{1,2})[月-](\d{1,2})日?`).FindStringSubmatch(htmlContent); len(matches) > 0 {
		quarter = matches[0]
	}
	rows := regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`).FindAllStringSubmatch(htmlContent, -1)
	holdings := make([]HoldingStock, 0, 10)
	for index, row := range rows {
		cells := regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`).FindAllStringSubmatch(row[1], -1)
		if len(cells) < 7 || len(holdings) >= 10 {
			continue
		}
		cellTexts := make([]string, len(cells))
		for i, cell := range cells {
			cellTexts[i] = cleanHTMLText(cell[1])
		}
		rank := int(parseFloat(cellTexts[0]))
		if rank <= 0 {
			rank = index + 1
		}
		price := parseFloatPtr(cellTexts[3])
		changeRate := parseFloatPtr(strings.TrimSuffix(cellTexts[4], "%"))
		stockCode := cellTexts[1]
		stockName := cellTexts[2]
		if stockCode == "" && stockName == "" {
			continue
		}
		holdings = append(holdings, HoldingStock{
			Rank:       rank,
			StockCode:  stockCode,
			StockName:  stockName,
			Ratio:      parseFloat(strings.TrimSuffix(cellTexts[6], "%")),
			Shares:     optionalCell(cellTexts, 7),
			MarketCap:  optionalCell(cellTexts, 8),
			Quarter:    quarter,
			Price:      price,
			ChangeRate: changeRate,
			Market:     detectStockMarket(cells[1][1], stockCode),
		})
	}
	if len(holdings) == 0 {
		return nil, fmt.Errorf("empty holding rows")
	}
	return holdings, nil
}

// normalizeRankingRequest 合并基金排行默认参数。
func normalizeRankingRequest(request RankingRequest) RankingRequest {
	if request.MarketType == "" {
		request.MarketType = "kf"
	}
	if request.FundType == "" {
		request.FundType = "all"
	}
	if request.MarketType == "fb" {
		switch request.FundType {
		case "all", "gp", "hh", "zq", "zs", "qdii", "fof":
			request.FundType = "ct"
		}
	}
	if request.SortField == "" {
		request.SortField = "jnzf"
	}
	if request.SortOrder == "" {
		request.SortOrder = "desc"
	}
	if request.PageIndex <= 0 {
		request.PageIndex = 1
	}
	if request.PageSize <= 0 {
		request.PageSize = 50
	}
	return request
}

// rankingReferer 返回排行接口对应的 Referer。
func rankingReferer(marketType string) string {
	if marketType == "fb" {
		return "https://fund.eastmoney.com/data/fbsfundranking.html"
	}
	return "https://fund.eastmoney.com/data/fundranking.html"
}

// normalizeFundCode 校验外部输入的基金代码。
func normalizeFundCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if !fundCodePattern.MatchString(code) {
		return "", fmt.Errorf("invalid fund code %q", code)
	}
	return code, nil
}

// historyNetValueFromRow 校验历史净值核心字段，避免远端占位符被误转成 0。
func historyNetValueFromRow(date string, netValue string, accumulatedValue string, dailyGrowth string, buyStatus string, sellStatus string) (HistoryNetValue, error) {
	trimmedDate := strings.TrimSpace(date)
	if trimmedDate == "" {
		return HistoryNetValue{}, fmt.Errorf("missing date")
	}
	parsedNetValue, err := parseRequiredFloat(netValue, "net value")
	if err != nil {
		return HistoryNetValue{}, err
	}
	parsedAccumulatedValue, err := parseRequiredFloat(accumulatedValue, "accumulated value")
	if err != nil {
		return HistoryNetValue{}, err
	}
	parsedDailyGrowth, err := parseRequiredFloat(dailyGrowth, "daily growth")
	if err != nil {
		return HistoryNetValue{}, err
	}
	return HistoryNetValue{
		Date:             trimmedDate,
		NetValue:         parsedNetValue,
		AccumulatedValue: parsedAccumulatedValue,
		DailyGrowth:      parsedDailyGrowth,
		BuyStatus:        strings.TrimSpace(buyStatus),
		SellStatus:       strings.TrimSpace(sellStatus),
	}, nil
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// parseFloatPtr 将字符串清洗为浮点指针，空值或非法值返回 nil。
func parseFloatPtr(value string) *float64 {
	trimmed := normalizeNumber(value)
	if trimmed == "" || trimmed == "--" {
		return nil
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

// parseFloat 将字符串清洗为浮点数，非法值返回 0。
func parseFloat(value string) float64 {
	parsed := parseFloatPtr(value)
	if parsed == nil {
		return 0
	}
	return *parsed
}

// normalizeNumber 移除百分号、逗号和空白，保留数值本身。
func normalizeNumber(value string) string {
	replacer := strings.NewReplacer("%", "", ",", "", "％", "", "\u00a0", "", " ", "")
	return strings.TrimSpace(replacer.Replace(value))
}

// parseRequiredFloat 解析历史净值核心数值，空值或占位符必须失败。
func parseRequiredFloat(value string, fieldName string) (float64, error) {
	parsed := parseFloatPtr(value)
	if parsed == nil {
		return 0, fmt.Errorf("missing %s", fieldName)
	}
	return *parsed, nil
}

// cleanFundName 清理东财页面标题中的跳转提示。
func cleanFundName(value string) string {
	value = strings.ReplaceAll(value, "查看相关ETF联接>", "")
	value = strings.ReplaceAll(value, "查看相关ETF>", "")
	value = strings.ReplaceAll(value, "查看相关ETF联接", "")
	value = strings.ReplaceAll(value, "查看相关ETF", "")
	return strings.TrimSpace(value)
}

// cleanHTMLText 移除 HTML 标签并合并空白字符。
func cleanHTMLText(value string) string {
	value = html.UnescapeString(value)
	value = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(value, " ")
	value = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(value, " ")
	value = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}

// findClassText 提取指定 class 节点的文本。
func findClassText(body string, className string) string {
	pattern := regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*` + regexp.QuoteMeta(className) + `[^"']*["'][^>]*>(.*?)</[^>]+>`)
	matches := pattern.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	return cleanHTMLText(matches[1])
}

// labelValue 从已清洗文本中提取 label 后面的值。
func labelValue(text string, labels ...string) string {
	for _, label := range labels {
		pattern := regexp.MustCompile(regexp.QuoteMeta(label) + `[:：]\s*([^:：]+?)(?:\s+[^\s:：]{2,12}[:：]|$)`)
		matches := pattern.FindStringSubmatch(text)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

// percentLabelValue 从文本中提取百分比指标。
func percentLabelValue(text string, label string) *float64 {
	pattern := regexp.MustCompile(regexp.QuoteMeta(label) + `[:：]\s*([-+]?\d+(?:\.\d+)?)\s*%?`)
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil
	}
	return parseFloatPtr(matches[1])
}

// extractBracketSection 提取形如 datas:[...] 的数组内容。
func extractBracketSection(body string, marker string) (string, error) {
	start := strings.Index(body, marker)
	if start < 0 {
		return "", fmt.Errorf("missing section %s", marker)
	}
	start += len(marker)
	end := strings.Index(body[start:], "]")
	if end < 0 {
		return "", fmt.Errorf("unterminated section %s", marker)
	}
	return body[start : start+end], nil
}

// extractIntValue 使用正则提取整数。
func extractIntValue(body string, pattern string) int {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if len(matches) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(matches[1])
	return value
}

// extractJSStringProperty 提取 JS 对象中的字符串属性内容。
func extractJSStringProperty(body string, property string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?is)` + regexp.QuoteMeta(property) + `\s*:\s*"(.*?)"`),
		regexp.MustCompile(`(?is)` + regexp.QuoteMeta(property) + `\s*:\s*'(.*?)'`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(body)
		if len(matches) < 2 {
			continue
		}
		value := matches[1]
		replacer := strings.NewReplacer(`\"`, `"`, `\'`, `'`, `\/`, `/`, `\n`, "", `\r`, "", `\t`, " ")
		return html.UnescapeString(replacer.Replace(value))
	}
	return ""
}

// optionalCell 安全读取表格列。
func optionalCell(cells []string, index int) string {
	if index < 0 || index >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[index])
}

// detectStockMarket 根据链接和代码推断持仓股票市场。
func detectStockMarket(href string, code string) string {
	normalized := strings.ToLower(href)
	switch {
	case strings.Contains(normalized, "sh") || strings.HasPrefix(code, "6") || strings.HasPrefix(code, "5"):
		return "SH"
	case strings.Contains(normalized, "sz") || strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") || strings.HasPrefix(code, "1"):
		return "SZ"
	case strings.Contains(normalized, "hk"):
		return "HK"
	case strings.Contains(normalized, "us"):
		return "US"
	default:
		return ""
	}
}
