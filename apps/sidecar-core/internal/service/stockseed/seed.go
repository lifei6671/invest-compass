package stockseed

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/stockdata"
)

const defaultBatchSize = 500

//go:embed stock_basic.json
var packagedStockBasicJSON []byte

// Store 定义内置股票基础资料 seed 需要的最小 DAO 写入能力。
type Store interface {
	UpsertBuiltinStocks(ctx context.Context, stocks []model.Stock) error
}

// Result 汇总本次基础股票池解析和写入结果，便于启动日志排障。
type Result struct {
	TotalRows                  int
	SeededRows                 int
	SkippedUnsupportedExchange int
	SkippedInvalidRows         int
}

type stockBasicPayload struct {
	Data stockBasicData `json:"data"`
}

type stockBasicData struct {
	Fields []string `json:"fields"`
	Items  [][]any  `json:"items"`
}

var selectedFields = []string{
	"ts_code",
	"symbol",
	"name",
	"industry",
	"cnspell",
	"list_date",
	"fullname",
	"exchange",
	"list_status",
}

// LoadPackagedStocks 读取随 sidecar 打包的 stock_basic.json，并转换为本地 Stock 模型。
func LoadPackagedStocks() ([]model.Stock, Result, error) {
	return DecodeStocks(packagedStockBasicJSON)
}

// SeedPackagedStocks 将随应用打包的基础股票池幂等写入本地 SQLite。
func SeedPackagedStocks(ctx context.Context, store Store) (Result, error) {
	stocks, result, err := LoadPackagedStocks()
	if err != nil {
		return Result{}, err
	}
	if _, err := SeedStocks(ctx, store, stocks); err != nil {
		return Result{}, err
	}
	return result, nil
}

// DecodeStocks 将 Tushare 风格 fields/items JSON 转换为标准化股票基础资料。
func DecodeStocks(payload []byte) ([]model.Stock, Result, error) {
	var decoded stockBasicPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, Result{}, fmt.Errorf("decode stock basic payload: %w", err)
	}
	rows, err := stockdata.MapFieldRows(decoded.Data.Fields, decoded.Data.Items, selectedFields)
	if err != nil {
		return nil, Result{}, err
	}

	result := Result{TotalRows: len(rows)}
	stocks := make([]model.Stock, 0, len(rows))
	for _, row := range rows {
		stock, skipReason := stockFromRecord(row)
		switch skipReason {
		case "":
			stocks = append(stocks, stock)
		case "unsupported_exchange":
			result.SkippedUnsupportedExchange++
		default:
			result.SkippedInvalidRows++
		}
	}
	result.SeededRows = len(stocks)
	return stocks, result, nil
}

// SeedStocks 将已解析的股票基础资料分批写入数据库，避免 SQLite 单次参数过多。
func SeedStocks(ctx context.Context, store Store, stocks []model.Stock) (Result, error) {
	if store == nil {
		return Result{}, fmt.Errorf("stock seed store is required")
	}
	result := Result{TotalRows: len(stocks), SeededRows: len(stocks)}
	for start := 0; start < len(stocks); start += defaultBatchSize {
		end := start + defaultBatchSize
		if end > len(stocks) {
			end = len(stocks)
		}
		if err := store.UpsertBuiltinStocks(ctx, stocks[start:end]); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

// stockFromRecord 将单条 fields/items 记录映射为本地股票模型，并返回跳过原因。
func stockFromRecord(record stockdata.Record) (model.Stock, string) {
	code := stringValue(record, "symbol")
	name := stringValue(record, "name")
	if code == "" || name == "" {
		return model.Stock{}, "invalid"
	}
	exchange, ok := normalizeExchange(stringValue(record, "exchange"), stringValue(record, "ts_code"))
	if !ok {
		return model.Stock{}, "unsupported_exchange"
	}

	pinyin := stringValue(record, "cnspell")
	return model.Stock{
		Symbol:         "CN:" + exchange + ":" + code,
		Market:         "CN",
		Code:           code,
		Name:           name,
		Pinyin:         pinyin,
		Exchange:       exchange,
		Industry:       stringValue(record, "industry"),
		ListDate:       stringValue(record, "list_date"),
		Status:         normalizeStatus(stringValue(record, "list_status")),
		FullName:       stringValue(record, "fullname"),
		PinyinInitials: pinyin,
	}, ""
}

// normalizeExchange 将外部交易所代码收敛为项目内部 SH/SZ 枚举，北交所暂不导入。
func normalizeExchange(exchange string, tsCode string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(exchange)) {
	case "SSE", "SH":
		return "SH", true
	case "SZSE", "SZ":
		return "SZ", true
	case "BSE", "BJ":
		return "", false
	}
	if strings.HasSuffix(strings.ToUpper(tsCode), ".SH") {
		return "SH", true
	}
	if strings.HasSuffix(strings.ToUpper(tsCode), ".SZ") {
		return "SZ", true
	}
	if strings.HasSuffix(strings.ToUpper(tsCode), ".BJ") {
		return "", false
	}
	return "", false
}

// normalizeStatus 将 Tushare 上市状态转换为项目内更易读的英文状态。
func normalizeStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "L":
		return "LISTED"
	case "D":
		return "DELISTED"
	case "P":
		return "PENDING"
	default:
		return strings.TrimSpace(status)
	}
}

// stringValue 从松散字段记录中读取字符串，nil 字段统一当作空值。
func stringValue(record stockdata.Record, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
