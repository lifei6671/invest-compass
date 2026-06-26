package watchlist

import (
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Item 是自选股业务模型，删除时通过 DeletedAt 软删除。
type Item struct {
	ID        int64
	Symbol    stock.Symbol
	SortOrder int
	Tags      []string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

// CreateRequest 是创建自选股所需的输入。
type CreateRequest struct {
	ID        int64
	Symbol    string
	SortOrder int
	Tags      []string
	Note      string
}

// UpdateRequest 是更新自选股元数据所需的输入。
type UpdateRequest struct {
	SortOrder int
	Tags      []string
	Note      string
}

// CreateItem 校验并创建自选股，保证 active symbol 唯一。
func CreateItem(existing []Item, request CreateRequest, now time.Time) (Item, error) {
	symbol, err := stock.ParseSymbol(request.Symbol)
	if err != nil {
		return Item{}, &xerr.Error{Code: xerr.WatchlistInvalidSymbol}
	}
	for _, item := range existing {
		if item.DeletedAt.IsZero() && item.Symbol.String() == symbol.String() {
			return Item{}, &xerr.Error{Code: xerr.WatchlistDuplicateActiveSymbol}
		}
	}

	return Item{
		ID:        request.ID,
		Symbol:    symbol,
		SortOrder: request.SortOrder,
		Tags:      copyTags(request.Tags),
		Note:      request.Note,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateItem 更新自选股标签、备注和排序，不允许修改 symbol。
func UpdateItem(item Item, request UpdateRequest, now time.Time) Item {
	item.SortOrder = request.SortOrder
	item.Tags = copyTags(request.Tags)
	item.Note = request.Note
	item.UpdatedAt = now
	return item
}

// DeleteItem 软删除自选股，让同一 symbol 后续可以重新添加。
func DeleteItem(item Item, now time.Time) Item {
	item.DeletedAt = now
	item.UpdatedAt = now
	return item
}

// ListActive 返回未软删除自选股，并按添加时间倒序稳定排序。
func ListActive(items []Item) []Item {
	listed := make([]Item, 0, len(items))
	for _, item := range items {
		if item.DeletedAt.IsZero() {
			listed = append(listed, item)
		}
	}
	sort.SliceStable(listed, func(left int, right int) bool {
		if listed[left].CreatedAt.Equal(listed[right].CreatedAt) {
			return listed[left].ID > listed[right].ID
		}
		return listed[left].CreatedAt.After(listed[right].CreatedAt)
	})
	return listed
}

// copyTags 复制标签切片，避免调用方后续修改影响自选股模型。
func copyTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return append([]string(nil), tags...)
}
