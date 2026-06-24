package watchlist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestHandleListIncludesStockProfileFields 验证自选股列表关联 stocks 表返回展示资料，避免前端只能用 symbol 降级展示。
func TestHandleListIncludesStockProfileFields(t *testing.T) {
	store := &fakeWatchlistStore{
		items: []model.Watchlist{{
			ID:        1,
			Symbol:    "600000.SH",
			SortOrder: 10,
			Tags:      `[\"银行\"]`,
			Note:      "低估值观察",
		}},
		stocks: map[string]model.Stock{
			"600000.SH": {
				Symbol:   "600000.SH",
				Name:     "浦发银行",
				Code:     "600000",
				Market:   "CN",
				Exchange: "SH",
				Industry: "银行",
				Concept:  `["低估值","大金融"]`,
				ListDate: "1999-11-10",
				Status:   "active",
				FullName: "上海浦东发展银行股份有限公司",
			},
		},
	}
	recorder := performWatchlistList(t, Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if len(store.profileSymbols) != 1 || store.profileSymbols[0] != "600000.SH" {
		t.Fatalf("expected profile lookup for watchlist symbol, got %#v", store.profileSymbols)
	}
	var response httpx.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	items := response.Data.(map[string]any)["items"].([]any)
	first := items[0].(map[string]any)
	if first["name"] != "浦发银行" || first["industry"] != "银行" || first["full_name"] != "上海浦东发展银行股份有限公司" {
		t.Fatalf("unexpected watchlist item profile fields: %#v", first)
	}
	concepts := first["concepts"].([]any)
	if len(concepts) != 2 || concepts[0] != "低估值" || concepts[1] != "大金融" {
		t.Fatalf("unexpected concepts: %#v", concepts)
	}
}

// performWatchlistList 使用固定 token 执行自选股列表 action。
func performWatchlistList(t *testing.T, config Config) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/watchlist/list", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	recorder := httptest.NewRecorder()
	Routes(config)[0].Handler(recorder, request)
	return recorder
}

type fakeWatchlistStore struct {
	items          []model.Watchlist
	stocks         map[string]model.Stock
	profileSymbols []string
}

// SaveWatchlist 满足 action Store 接口；列表测试不应调用写入路径。
func (store *fakeWatchlistStore) SaveWatchlist(context.Context, *model.Watchlist) error {
	return nil
}

// ListActiveWatchlists 返回测试预置的 active 自选股。
func (store *fakeWatchlistStore) ListActiveWatchlists(context.Context) ([]model.Watchlist, error) {
	return store.items, nil
}

// SoftDeleteWatchlist 满足 action Store 接口；列表测试不应调用删除路径。
func (store *fakeWatchlistStore) SoftDeleteWatchlist(context.Context, int64) error {
	return nil
}

// GetStocksBySymbols 返回按 symbol 关联的股票基础资料。
func (store *fakeWatchlistStore) GetStocksBySymbols(_ context.Context, symbols []string) (map[string]model.Stock, error) {
	store.profileSymbols = append([]string(nil), symbols...)
	return store.stocks, nil
}
