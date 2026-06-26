package stockseed

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestDecodeStocksMapsMainlandShareBasics 验证内置股票基础 JSON 能转换为标准 symbol 和本地 Stock 字段。
func TestDecodeStocksMapsMainlandShareBasics(t *testing.T) {
	payload := mustStockBasicPayload(t, []string{
		"ts_code",
		"symbol",
		"name",
		"area",
		"industry",
		"cnspell",
		"market",
		"list_date",
		"act_name",
		"act_ent_type",
		"fullname",
		"exchange",
		"list_status",
	}, [][]any{
		{"000001.SZ", "000001", "平安银行", "深圳", "银行", "payh", "主板", "19910403", "无", "无", "平安银行股份有限公司", "SZSE", "L"},
		{"600519.SH", "600519", "贵州茅台", "贵州", "白酒", "gzmt", "主板", "20010827", "无", "无", "贵州茅台酒股份有限公司", "SSE", "L"},
		{"920019.BJ", "920019", "铜冠矿建", "安徽", "建筑", "tgkj", "北交所", "20250522", "无", "无", "铜陵有色金属集团铜冠矿山建设股份有限公司", "BSE", "L"},
		{"", "", "缺代码", "北京", "测试", "qdm", "主板", "20250101", "无", "无", "缺代码股份有限公司", "SSE", "L"},
	})

	stocks, result, err := DecodeStocks(payload)
	if err != nil {
		t.Fatalf("decode stocks: %v", err)
	}

	if result.TotalRows != 4 || result.SeededRows != 2 || result.SkippedUnsupportedExchange != 1 || result.SkippedInvalidRows != 1 {
		t.Fatalf("unexpected decode result: %+v", result)
	}
	if len(stocks) != 2 {
		t.Fatalf("expected 2 supported stocks, got %+v", stocks)
	}
	assertStock(t, stocks[0], model.Stock{
		Symbol:         "CN:SZ:000001",
		Market:         "CN",
		Code:           "000001",
		Name:           "平安银行",
		Pinyin:         "payh",
		Exchange:       "SZ",
		Industry:       "银行",
		ListDate:       "19910403",
		Status:         "LISTED",
		FullName:       "平安银行股份有限公司",
		PinyinInitials: "payh",
	})
	assertStock(t, stocks[1], model.Stock{
		Symbol:         "CN:SH:600519",
		Market:         "CN",
		Code:           "600519",
		Name:           "贵州茅台",
		Pinyin:         "gzmt",
		Exchange:       "SH",
		Industry:       "白酒",
		ListDate:       "20010827",
		Status:         "LISTED",
		FullName:       "贵州茅台酒股份有限公司",
		PinyinInitials: "gzmt",
	})
}

// TestSeedStocksWritesInBatches 验证 seed 写库会分批执行，避免 SQLite 单次参数过多。
func TestSeedStocksWritesInBatches(t *testing.T) {
	store := &fakeSeedStore{}
	stocks := make([]model.Stock, 0, defaultBatchSize+1)
	for index := 0; index < defaultBatchSize+1; index++ {
		code := fmt.Sprintf("%06d", index)
		stocks = append(stocks, model.Stock{Symbol: "CN:SH:" + code, Market: "CN", Exchange: "SH", Code: code, Name: "测试"})
	}

	result, err := SeedStocks(context.Background(), store, stocks)
	if err != nil {
		t.Fatalf("seed stocks: %v", err)
	}

	if result.SeededRows != defaultBatchSize+1 {
		t.Fatalf("unexpected seed result: %+v", result)
	}
	if len(store.batches) != 2 || len(store.batches[0]) != defaultBatchSize || len(store.batches[1]) != 1 {
		t.Fatalf("unexpected seed batches: %+v", store.batches)
	}
}

// TestLoadPackagedStocksUsesEmbeddedJSON 验证打包进 sidecar 的基础股票池可被启动 seed 直接加载。
func TestLoadPackagedStocksUsesEmbeddedJSON(t *testing.T) {
	stocks, result, err := LoadPackagedStocks()
	if err != nil {
		t.Fatalf("load packaged stocks: %v", err)
	}

	if len(stocks) == 0 || result.SeededRows == 0 {
		t.Fatalf("expected packaged stocks, got result=%+v len=%d", result, len(stocks))
	}
	if result.SkippedUnsupportedExchange == 0 {
		t.Fatalf("expected BSE rows to be skipped until BJ symbol support is implemented: %+v", result)
	}
}

type fakeSeedStore struct {
	batches [][]model.Stock
}

// UpsertBuiltinStocks 记录 seed 批次，模拟 DAO 的内置基础资料写入能力。
func (store *fakeSeedStore) UpsertBuiltinStocks(_ context.Context, stocks []model.Stock) error {
	copied := append([]model.Stock(nil), stocks...)
	store.batches = append(store.batches, copied)
	return nil
}

// mustStockBasicPayload 构造测试用 Tushare 风格基础股票响应。
func mustStockBasicPayload(t *testing.T, fields []string, items [][]any) []byte {
	t.Helper()
	payload := map[string]any{
		"code":       0,
		"request_id": "test",
		"msg":        "",
		"data": map[string]any{
			"fields": fields,
			"items":  items,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return encoded
}

// assertStock 对比本次 seed 关心的基础字段，避免测试受数据库自动字段影响。
func assertStock(t *testing.T, got model.Stock, want model.Stock) {
	t.Helper()
	if got.Symbol != want.Symbol ||
		got.Market != want.Market ||
		got.Code != want.Code ||
		got.Name != want.Name ||
		got.Pinyin != want.Pinyin ||
		got.Exchange != want.Exchange ||
		got.Industry != want.Industry ||
		got.ListDate != want.ListDate ||
		got.Status != want.Status ||
		got.FullName != want.FullName ||
		got.PinyinInitials != want.PinyinInitials {
		t.Fatalf("unexpected stock:\n got=%+v\nwant=%+v", got, want)
	}
}
