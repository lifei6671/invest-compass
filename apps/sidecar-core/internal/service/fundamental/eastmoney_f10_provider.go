package fundamental

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/crawler"
)

const (
	eastMoneyF10ProviderName = "eastmoney-f10"
	defaultEastMoneyF10URL   = "https://datacenter.eastmoney.com/securities/api/data/v1/get"
	defaultF10Timeout        = 8 * time.Second
)

// EastMoneyF10Config 描述东方财富 F10 Provider 的可替换运行参数。
type EastMoneyF10Config struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	Now        func() time.Time
}

// EastMoneyF10Provider 使用东方财富 HSF10 数据中心接口实现基本面数据查询。
type EastMoneyF10Provider struct {
	baseURL string
	client  *crawler.Client
	now     func() time.Time
}

// NewEastMoneyF10Provider 创建东方财富 F10 Provider。
func NewEastMoneyF10Provider(config EastMoneyF10Config) (*EastMoneyF10Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultF10Timeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s invalid timeout", eastMoneyF10ProviderName)
	}
	client, err := crawler.NewClient(crawler.Config{
		Name:         eastMoneyF10ProviderName,
		Timeout:      timeout,
		HTTPClient:   config.HTTPClient,
		MaxBodyBytes: 2 * 1024 * 1024,
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
	return &EastMoneyF10Provider{
		baseURL: firstNonEmpty(config.BaseURL, defaultEastMoneyF10URL),
		client:  client,
		now:     now,
	}, nil
}

// Name 返回东方财富 F10 Provider 稳定名称。
func (provider *EastMoneyF10Provider) Name() string {
	return eastMoneyF10ProviderName
}

// Status 返回东方财富 F10 数据源来源、授权边界和限频说明，不做阻塞式远程探测。
func (provider *EastMoneyF10Provider) Status(context.Context) ProviderStatus {
	return ProviderStatus{
		Name:          provider.Name(),
		Source:        "EastMoney HSF10 public datacenter endpoint",
		License:       "公开网页接口，用户需自行确认数据授权和使用限制",
		RateLimit:     "local client best effort, no burst retry",
		Available:     provider != nil,
		LastCheckedAt: provider.now().UTC(),
	}
}

// Fetch 根据报告类型获取东方财富 F10 数据，并转换为统一 Dataset。
func (provider *EastMoneyF10Provider) Fetch(ctx context.Context, request Request) (Dataset, error) {
	spec, ok := eastMoneyF10Specs[request.Kind]
	if !ok {
		return Dataset{}, NewProviderError(provider.Name(), "fetch_kind", fmt.Errorf("unsupported report kind %q", request.Kind))
	}
	secCode, err := eastMoneyF10CodeFromSymbol(request.Symbol)
	if err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "fetch_symbol", err)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = spec.PageSize
	}
	if limit <= 0 {
		limit = 10
	}

	query := spec.query(secCode, limit, provider.now())
	result, err := provider.client.Fetch(ctx, crawler.Request{
		URL:   provider.baseURL,
		Query: query,
		Headers: map[string]string{
			"Accept":          "application/json,text/plain,*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Origin":          "https://emweb.securities.eastmoney.com",
			"Referer":         "https://emweb.securities.eastmoney.com/",
		},
	})
	if err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "fetch", err)
	}

	response, err := decodeEastMoneyF10Response(result.BodyBytes)
	if err != nil {
		return Dataset{}, NewProviderError(provider.Name(), "decode", err)
	}
	if !response.Success || response.Code != 0 {
		return Dataset{}, NewProviderError(provider.Name(), "remote_code", fmt.Errorf("eastmoney f10 code %d success %v message %s", response.Code, response.Success, response.Message))
	}
	if response.Result == nil {
		return Dataset{}, NewProviderError(provider.Name(), "remote_result", fmt.Errorf("eastmoney f10 missing result"))
	}

	return Dataset{
		Title:     spec.Title,
		Kind:      request.Kind,
		Symbol:    request.Symbol,
		Columns:   spec.Columns,
		Rows:      normalizeRows(response.Result.Data),
		Provider:  provider.Name(),
		Source:    provider.Status(ctx).Source,
		FetchedAt: result.FetchedAt,
	}, nil
}

type eastMoneyF10Response struct {
	Version string              `json:"version"`
	Result  *eastMoneyF10Result `json:"result"`
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Code    int                 `json:"code"`
}

type eastMoneyF10Result struct {
	Count int              `json:"count"`
	Data  []map[string]any `json:"data"`
}

type eastMoneyF10Spec struct {
	Kind        ReportKind
	Title       string
	ReportName  string
	Columns     []Column
	PageSize    int
	SortTypes   string
	SortColumns string
	FilterExtra string
}

var eastMoneyF10Specs = map[ReportKind]eastMoneyF10Spec{
	ReportLatestFinance: {
		Kind:        ReportLatestFinance,
		Title:       "最新财务主要数据",
		ReportName:  "RPT_PCF10_FINANCEMAINFINADATA",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "REPORT_DATE", "REPORT_TYPE", "EPSJB", "EPSKCJB", "EPSXS", "BPS", "MGZBGJ", "MGWFPLR", "MGJYXJJE", "TOTAL_OPERATEINCOME", "PARENT_NETPROFIT", "KCFJCXSYJLR", "ROEJQ", "XSMLL", "ZCFZL", "TOTALOPERATEREVETZ", "PARENTNETPROFITTZ", "KCFJCXSYJLRTZ", "TOTAL_SHARE", "FREE_SHARE"),
		PageSize:    1,
		SortTypes:   "-1",
		SortColumns: "REPORT_DATE",
	},
	ReportQuarterFinance: {
		Kind:        ReportQuarterFinance,
		Title:       "季度主要财务指标",
		ReportName:  "RPT_F10_QTR_MAINFINADATA",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "REPORT_DATE", "EPSJB", "BPS", "PER_CAPITAL_RESERVE", "PER_UNASSIGN_PROFIT", "PER_NETCASH", "TOTALOPERATEREVE", "GROSS_PROFIT", "PARENTNETPROFIT", "DEDU_PARENT_PROFIT", "TOTALOPERATEREVETZ", "PARENTNETPROFITTZ", "DPNP_YOY_RATIO", "ROE_DILUTED", "JROA", "NET_PROFIT_RATIO", "GROSS_PROFIT_RATIO"),
		PageSize:    9,
		SortTypes:   "-1",
		SortColumns: "REPORT_DATE",
	},
	ReportOrgForecast: {
		Kind:       ReportOrgForecast,
		Title:      "机构预测明细",
		ReportName: "RPT_HSF10_RES_ORGPREDICT",
		Columns:    columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "PUBLISH_DATE", "ORG_NAME_ABBR", "YEAR1", "YEAR_MARK1", "EPS1", "PE1", "YEAR2", "YEAR_MARK2", "EPS2", "PE2", "YEAR3", "YEAR_MARK3", "EPS3", "PE3", "YEAR4", "YEAR_MARK4", "EPS4", "PE4"),
		PageSize:   200,
	},
	ReportForecastSummary: {
		Kind:        ReportForecastSummary,
		Title:       "机构预测汇总",
		ReportName:  "RPT_HSF10_RESPREDICT_STATISTICS",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "YEAR", "YEAR_MARK", "EPS", "EPS_RATIO", "PE", "RANK"),
		PageSize:    200,
		SortTypes:   "1",
		SortColumns: "RANK",
	},
	ReportValuationPercentile: {
		Kind:        ReportValuationPercentile,
		Title:       "估值百分位",
		ReportName:  "RPT_STOCKVALUATIONTANTILE",
		Columns:     columns("SECUCODE", "STATISTICS_CYCLE", "INDEX_TYPE", "PERCENTILE_THIRTY", "PERCENTILE_FIFTY", "PERCENTILE_SEVENTY"),
		PageSize:    10,
		FilterExtra: `(INDEX_TYPE="1")(STATISTICS_CYCLE="3")`,
	},
	ReportMarginTrading: {
		Kind:        ReportMarginTrading,
		Title:       "融资融券数据",
		ReportName:  "RPT_MARGIN_STATISTICS_STOCKS",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE", "FIN_BUY_AMT", "FIN_REPAY_AMT", "FIN_BALANCE", "LOAN_SELL_VOL", "LOAN_REPAY_VOL", "LOAN_BALANCE"),
		PageSize:    10,
		SortTypes:   "-1",
		SortColumns: "TRADE_DATE",
	},
	ReportBlockTrade: {
		Kind:        ReportBlockTrade,
		Title:       "大宗交易数据",
		ReportName:  "RPT_DATA_BLOCKTRADE",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE", "DEAL_PRICE", "PREMIUM_RATIO", "DEAL_VOLUME", "DEAL_AMT", "BUYER_NAME", "SELLER_NAME", "DAILY_RANK", "CLOSE_PRICE", "TURNOVER_RATE", "CHANGE_RATE"),
		PageSize:    10,
		SortTypes:   "-1",
		SortColumns: "TRADE_DATE",
	},
	ReportHolderTrend: {
		Kind:        ReportHolderTrend,
		Title:       "户均持股趋势",
		ReportName:  "RPT_CUSTOM_DMSK_TREND",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE", "INDICATOR_VALUE"),
		PageSize:    20,
		SortTypes:   "1",
		SortColumns: "TRADE_DATE",
		FilterExtra: `(INDICATORTYPE=1)(DATETYPE=3)`,
	},
	ReportBillboard: {
		Kind:        ReportBillboard,
		Title:       "龙虎榜数据",
		ReportName:  "RPT_BILLBOARD_DAILYDETAILS",
		Columns:     columns("SECUCODE", "SECURITY_CODE", "TRADE_DATE", "EXPLANATION", "TOTAL_BUY", "TOTAL_SELL", "TOTAL_BUYRIOTOP", "TOTAL_SELLRIOTOP"),
		PageSize:    5,
		SortTypes:   "-1,-1",
		SortColumns: "TRADE_DATE,EXPLANATION",
	},
	ReportOperatingDepartment: {
		Kind:        ReportOperatingDepartment,
		Title:       "营业部买卖明细",
		ReportName:  "RPT_OPERATEDEPT_TRADE",
		Columns:     columns("SECUCODE", "TRADE_DATE", "EXPLANATION", "OPERATEDEPT_NAME", "BUY_AMT_REAL", "BUY_RATIO", "SELL_AMT_REAL", "SELL_RATIO"),
		PageSize:    15,
		SortTypes:   "-1,-1,1",
		SortColumns: "TRADE_DATE,EXPLANATION,RANK",
		FilterExtra: `(TRADE_DIRECTION="0")`,
	},
}

var f10ColumnMeta = map[string]Column{
	"SECUCODE":            {Key: "SECUCODE", Label: "证券代码", Hidden: true},
	"SECURITY_CODE":       {Key: "SECURITY_CODE", Label: "股票代码"},
	"SECURITY_NAME_ABBR":  {Key: "SECURITY_NAME_ABBR", Label: "股票简称"},
	"REPORT_DATE":         {Key: "REPORT_DATE", Label: "报告日期", Format: FormatDate},
	"REPORT_TYPE":         {Key: "REPORT_TYPE", Label: "报告类型"},
	"PUBLISH_DATE":        {Key: "PUBLISH_DATE", Label: "发布日期", Format: FormatDate},
	"TRADE_DATE":          {Key: "TRADE_DATE", Label: "交易日期", Format: FormatDate},
	"EPSJB":               {Key: "EPSJB", Label: "基本每股收益", Format: FormatPrice},
	"EPSKCJB":             {Key: "EPSKCJB", Label: "扣非每股收益", Format: FormatPrice},
	"EPSXS":               {Key: "EPSXS", Label: "稀释每股收益", Format: FormatPrice},
	"BPS":                 {Key: "BPS", Label: "每股净资产", Format: FormatPrice},
	"MGZBGJ":              {Key: "MGZBGJ", Label: "每股资本公积", Format: FormatPrice},
	"MGWFPLR":             {Key: "MGWFPLR", Label: "每股未分配利润", Format: FormatPrice},
	"MGJYXJJE":            {Key: "MGJYXJJE", Label: "每股经营现金流", Format: FormatPrice},
	"TOTAL_OPERATEINCOME": {Key: "TOTAL_OPERATEINCOME", Label: "营业总收入", Format: FormatMoney},
	"TOTALOPERATEREVE":    {Key: "TOTALOPERATEREVE", Label: "营业总收入", Format: FormatMoney},
	"PARENT_NETPROFIT":    {Key: "PARENT_NETPROFIT", Label: "归属净利润", Format: FormatMoney},
	"PARENTNETPROFIT":     {Key: "PARENTNETPROFIT", Label: "归属净利润", Format: FormatMoney},
	"KCFJCXSYJLR":         {Key: "KCFJCXSYJLR", Label: "扣非净利润", Format: FormatMoney},
	"DEDU_PARENT_PROFIT":  {Key: "DEDU_PARENT_PROFIT", Label: "扣非净利润", Format: FormatMoney},
	"GROSS_PROFIT":        {Key: "GROSS_PROFIT", Label: "毛利润", Format: FormatMoney},
	"ROEJQ":               {Key: "ROEJQ", Label: "ROE(加权)", Format: FormatPercent},
	"ROE_DILUTED":         {Key: "ROE_DILUTED", Label: "ROE(摊薄)", Format: FormatPercent},
	"JROA":                {Key: "JROA", Label: "总资产净利率", Format: FormatPercent},
	"XSMLL":               {Key: "XSMLL", Label: "销售毛利率", Format: FormatPercent},
	"ZCFZL":               {Key: "ZCFZL", Label: "资产负债率", Format: FormatPercent},
	"NET_PROFIT_RATIO":    {Key: "NET_PROFIT_RATIO", Label: "净利率", Format: FormatPercent},
	"GROSS_PROFIT_RATIO":  {Key: "GROSS_PROFIT_RATIO", Label: "毛利率", Format: FormatPercent},
	"TOTALOPERATEREVETZ":  {Key: "TOTALOPERATEREVETZ", Label: "营收同比增长", Format: FormatPercent},
	"PARENTNETPROFITTZ":   {Key: "PARENTNETPROFITTZ", Label: "净利同比增长", Format: FormatPercent},
	"KCFJCXSYJLRTZ":       {Key: "KCFJCXSYJLRTZ", Label: "扣非净利同比增长", Format: FormatPercent},
	"DPNP_YOY_RATIO":      {Key: "DPNP_YOY_RATIO", Label: "扣非净利同比增长", Format: FormatPercent},
	"TOTAL_SHARE":         {Key: "TOTAL_SHARE", Label: "总股本", Format: FormatVolume},
	"FREE_SHARE":          {Key: "FREE_SHARE", Label: "流通股", Format: FormatVolume},
	"PER_CAPITAL_RESERVE": {Key: "PER_CAPITAL_RESERVE", Label: "每股资本公积", Format: FormatPrice},
	"PER_UNASSIGN_PROFIT": {Key: "PER_UNASSIGN_PROFIT", Label: "每股未分配利润", Format: FormatPrice},
	"PER_NETCASH":         {Key: "PER_NETCASH", Label: "每股经营现金流", Format: FormatPrice},
	"ORG_NAME_ABBR":       {Key: "ORG_NAME_ABBR", Label: "机构简称"},
	"YEAR1":               {Key: "YEAR1", Label: "预测年份1", Format: FormatInteger},
	"YEAR2":               {Key: "YEAR2", Label: "预测年份2", Format: FormatInteger},
	"YEAR3":               {Key: "YEAR3", Label: "预测年份3", Format: FormatInteger},
	"YEAR4":               {Key: "YEAR4", Label: "预测年份4", Format: FormatInteger},
	"YEAR":                {Key: "YEAR", Label: "年份", Format: FormatInteger},
	"YEAR_MARK1":          {Key: "YEAR_MARK1", Label: "标识1"},
	"YEAR_MARK2":          {Key: "YEAR_MARK2", Label: "标识2"},
	"YEAR_MARK3":          {Key: "YEAR_MARK3", Label: "标识3"},
	"YEAR_MARK4":          {Key: "YEAR_MARK4", Label: "标识4"},
	"YEAR_MARK":           {Key: "YEAR_MARK", Label: "标识"},
	"EPS1":                {Key: "EPS1", Label: "每股收益预测1", Format: FormatPrice},
	"EPS2":                {Key: "EPS2", Label: "每股收益预测2", Format: FormatPrice},
	"EPS3":                {Key: "EPS3", Label: "每股收益预测3", Format: FormatPrice},
	"EPS4":                {Key: "EPS4", Label: "每股收益预测4", Format: FormatPrice},
	"EPS":                 {Key: "EPS", Label: "每股收益", Format: FormatPrice},
	"EPS_RATIO":           {Key: "EPS_RATIO", Label: "EPS增长率", Format: FormatPercent},
	"PE1":                 {Key: "PE1", Label: "预测市盈率1", Format: FormatPrice},
	"PE2":                 {Key: "PE2", Label: "预测市盈率2", Format: FormatPrice},
	"PE3":                 {Key: "PE3", Label: "预测市盈率3", Format: FormatPrice},
	"PE4":                 {Key: "PE4", Label: "预测市盈率4", Format: FormatPrice},
	"PE":                  {Key: "PE", Label: "市盈率", Format: FormatPrice},
	"RANK":                {Key: "RANK", Label: "排名", Format: FormatInteger},
	"STATISTICS_CYCLE":    {Key: "STATISTICS_CYCLE", Label: "统计周期", Hidden: true},
	"INDEX_TYPE":          {Key: "INDEX_TYPE", Label: "指标类型", Hidden: true},
	"PERCENTILE_THIRTY":   {Key: "PERCENTILE_THIRTY", Label: "30%分位", Format: FormatPrice},
	"PERCENTILE_FIFTY":    {Key: "PERCENTILE_FIFTY", Label: "50%分位", Format: FormatPrice},
	"PERCENTILE_SEVENTY":  {Key: "PERCENTILE_SEVENTY", Label: "70%分位", Format: FormatPrice},
	"FIN_BUY_AMT":         {Key: "FIN_BUY_AMT", Label: "融资买入额", Format: FormatMoney},
	"FIN_REPAY_AMT":       {Key: "FIN_REPAY_AMT", Label: "融资偿还额", Format: FormatMoney},
	"FIN_BALANCE":         {Key: "FIN_BALANCE", Label: "融资余额", Format: FormatMoney},
	"LOAN_SELL_VOL":       {Key: "LOAN_SELL_VOL", Label: "融券卖出量", Format: FormatVolume},
	"LOAN_REPAY_VOL":      {Key: "LOAN_REPAY_VOL", Label: "融券偿还量", Format: FormatVolume},
	"LOAN_BALANCE":        {Key: "LOAN_BALANCE", Label: "融券余额", Format: FormatMoney},
	"DEAL_PRICE":          {Key: "DEAL_PRICE", Label: "成交价", Format: FormatPrice},
	"PREMIUM_RATIO":       {Key: "PREMIUM_RATIO", Label: "溢价率", Format: FormatPercent},
	"DEAL_VOLUME":         {Key: "DEAL_VOLUME", Label: "成交量", Format: FormatVolume},
	"DEAL_AMT":            {Key: "DEAL_AMT", Label: "成交额", Format: FormatMoney},
	"BUYER_NAME":          {Key: "BUYER_NAME", Label: "买方营业部"},
	"SELLER_NAME":         {Key: "SELLER_NAME", Label: "卖方营业部"},
	"DAILY_RANK":          {Key: "DAILY_RANK", Label: "当日排名", Format: FormatInteger},
	"CLOSE_PRICE":         {Key: "CLOSE_PRICE", Label: "收盘价", Format: FormatPrice},
	"TURNOVER_RATE":       {Key: "TURNOVER_RATE", Label: "换手率", Format: FormatPercent},
	"CHANGE_RATE":         {Key: "CHANGE_RATE", Label: "涨跌幅", Format: FormatPercent},
	"INDICATOR_VALUE":     {Key: "INDICATOR_VALUE", Label: "户均持股", Format: FormatVolume},
	"EXPLANATION":         {Key: "EXPLANATION", Label: "上榜原因"},
	"TOTAL_BUY":           {Key: "TOTAL_BUY", Label: "买入额", Format: FormatMoney},
	"TOTAL_SELL":          {Key: "TOTAL_SELL", Label: "卖出额", Format: FormatMoney},
	"TOTAL_BUYRIOTOP":     {Key: "TOTAL_BUYRIOTOP", Label: "买入占比", Format: FormatPercent},
	"TOTAL_SELLRIOTOP":    {Key: "TOTAL_SELLRIOTOP", Label: "卖出占比", Format: FormatPercent},
	"OPERATEDEPT_NAME":    {Key: "OPERATEDEPT_NAME", Label: "营业部名称"},
	"BUY_AMT_REAL":        {Key: "BUY_AMT_REAL", Label: "买入额", Format: FormatMoney},
	"BUY_RATIO":           {Key: "BUY_RATIO", Label: "买入占比", Format: FormatPercent},
	"SELL_AMT_REAL":       {Key: "SELL_AMT_REAL", Label: "卖出额", Format: FormatMoney},
	"SELL_RATIO":          {Key: "SELL_RATIO", Label: "卖出占比", Format: FormatPercent},
	"TRADE_DIRECTION":     {Key: "TRADE_DIRECTION", Label: "交易方向", Hidden: true},
}

// query 根据报告规格构造东方财富 F10 查询参数。
func (spec eastMoneyF10Spec) query(secCode string, limit int, now time.Time) map[string]string {
	filter := fmt.Sprintf(`(SECUCODE="%s")`, secCode) + spec.FilterExtra
	return map[string]string{
		"reportName":   spec.ReportName,
		"columns":      spec.columnKeys(),
		"quoteColumns": "",
		"filter":       filter,
		"pageNumber":   "1",
		"pageSize":     strconv.Itoa(limit),
		"sortTypes":    spec.SortTypes,
		"sortColumns":  spec.SortColumns,
		"source":       "HSF10",
		"client":       "PC",
		"v":            strconv.FormatInt(now.Unix(), 10),
	}
}

// columnKeys 返回远端接口需要的字段列表。
func (spec eastMoneyF10Spec) columnKeys() string {
	keys := make([]string, 0, len(spec.Columns))
	for _, column := range spec.Columns {
		keys = append(keys, column.Key)
	}
	return strings.Join(keys, ",")
}

// columns 根据字段 key 生成带显示元信息的列定义。
func columns(keys ...string) []Column {
	result := make([]Column, 0, len(keys))
	for _, key := range keys {
		if column, ok := f10ColumnMeta[key]; ok {
			result = append(result, column)
			continue
		}
		result = append(result, Column{Key: key, Label: key})
	}
	return result
}

// decodeEastMoneyF10Response 解码远端响应，并要求 JSON 对象格式正确。
func decodeEastMoneyF10Response(payload []byte) (eastMoneyF10Response, error) {
	var response eastMoneyF10Response
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return eastMoneyF10Response{}, err
	}
	return response, nil
}

// normalizeRows 复制远端行数据，避免调用方修改 Provider 内部切片。
func normalizeRows(data []map[string]any) []Row {
	rows := make([]Row, 0, len(data))
	for _, item := range data {
		row := make(Row, len(item))
		for key, value := range item {
			row[key] = normalizeJSONValue(value)
		}
		rows = append(rows, row)
	}
	return rows
}

// normalizeJSONValue 将 json.Number 转成 float64，保持 renderer 和调用方更易使用。
func normalizeJSONValue(value any) any {
	if number, ok := value.(json.Number); ok {
		parsed, err := number.Float64()
		if err == nil {
			return parsed
		}
	}
	return value
}

// eastMoneyF10CodeFromSymbol 将标准 symbol 转换为东财 F10 使用的 SECUCODE。
func eastMoneyF10CodeFromSymbol(symbol stock.Symbol) (string, error) {
	if symbol.Market != string(market.MarketCN) {
		return "", fmt.Errorf("market %s is not supported by %s", symbol.Market, eastMoneyF10ProviderName)
	}
	switch symbol.Exchange {
	case "SH", "SZ", "BJ":
		return symbol.Code + "." + symbol.Exchange, nil
	default:
		return "", fmt.Errorf("exchange %s is not supported by %s", symbol.Exchange, eastMoneyF10ProviderName)
	}
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
