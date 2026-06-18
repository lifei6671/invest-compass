package watchlist

import (
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestCreateItemRejectsDuplicateActiveSymbol 验证同一 active symbol 只能添加一次。
func TestCreateItemRejectsDuplicateActiveSymbol(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	existing, err := CreateItem(nil, CreateRequest{ID: 1, Symbol: "cn:sh:600519"}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}

	_, err = CreateItem([]Item{existing}, CreateRequest{ID: 2, Symbol: "CN:SH:600519"}, now)

	assertWatchlistErrorCode(t, err, xerr.WatchlistDuplicateActiveSymbol)
}

// TestCreateItemAllowsReaddAfterSoftDelete 验证软删除后同一股票可以重新添加。
func TestCreateItemAllowsReaddAfterSoftDelete(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	existing, err := CreateItem(nil, CreateRequest{ID: 1, Symbol: "US:AAPL"}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	deleted := DeleteItem(existing, now.Add(time.Minute))

	recreated, err := CreateItem([]Item{deleted}, CreateRequest{ID: 2, Symbol: "us:aapl"}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("CreateItem after delete returned error: %v", err)
	}

	if recreated.ID != 2 || recreated.Symbol.String() != "US:AAPL" || !recreated.DeletedAt.IsZero() {
		t.Fatalf("unexpected recreated item: %+v", recreated)
	}
}

// TestListActiveFiltersDeletedAndSorts 验证自选列表不返回软删除数据，并按 sort_order 稳定排序。
func TestListActiveFiltersDeletedAndSorts(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	first, err := CreateItem(nil, CreateRequest{ID: 1, Symbol: "US:AAPL", SortOrder: 20}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	second, err := CreateItem([]Item{first}, CreateRequest{ID: 2, Symbol: "CN:SH:600519", SortOrder: 10}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	deleted, err := CreateItem([]Item{first, second}, CreateRequest{ID: 3, Symbol: "HK:00700", SortOrder: 1}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	deleted = DeleteItem(deleted, now.Add(time.Minute))

	listed := ListActive([]Item{first, second, deleted})

	if len(listed) != 2 {
		t.Fatalf("expected 2 active items, got %d", len(listed))
	}
	if listed[0].ID != 2 || listed[1].ID != 1 {
		t.Fatalf("unexpected list order: %+v", listed)
	}
}

// TestUpdateItemAppliesMetadata 验证备注、标签和排序更新只影响自选股元数据。
func TestUpdateItemAppliesMetadata(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	item, err := CreateItem(nil, CreateRequest{ID: 1, Symbol: "US:AAPL"}, now)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}

	updated := UpdateItem(item, UpdateRequest{
		SortOrder: 3,
		Tags:      []string{"长期观察", "科技"},
		Note:      "等待财报",
	}, now.Add(time.Minute))

	if updated.Symbol.String() != "US:AAPL" {
		t.Fatalf("UpdateItem should not change symbol: %+v", updated)
	}
	if updated.SortOrder != 3 || updated.Note != "等待财报" || len(updated.Tags) != 2 {
		t.Fatalf("unexpected updated metadata: %+v", updated)
	}
	if !updated.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("unexpected updated_at: %s", updated.UpdatedAt)
	}
}

// assertWatchlistErrorCode 校验自选股错误码稳定。
func assertWatchlistErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var watchlistError *xerr.Error
	if !errors.As(err, &watchlistError) {
		t.Fatalf("expected xerr.Error, got %T", err)
	}
	if watchlistError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, watchlistError.Code)
	}
}
