package watchlist

import (
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/stock"
)

// ErrorCode 是自选股模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidSymbol 表示自选股股票代码无法解析。
	ErrorInvalidSymbol ErrorCode = "invalid_watchlist_symbol"
	// ErrorDuplicateActiveSymbol 表示同一 active symbol 已存在。
	ErrorDuplicateActiveSymbol ErrorCode = "duplicate_active_symbol"
)

// Error 表示自选股规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把用户输入写入错误文本。
func (err *Error) Error() string {
	return string(err.Code)
}

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
		return Item{}, &Error{Code: ErrorInvalidSymbol}
	}
	for _, item := range existing {
		if item.DeletedAt.IsZero() && item.Symbol.String() == symbol.String() {
			return Item{}, &Error{Code: ErrorDuplicateActiveSymbol}
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

// ListActive 返回未软删除自选股，并按 sort_order、id 稳定排序。
func ListActive(items []Item) []Item {
	listed := make([]Item, 0, len(items))
	for _, item := range items {
		if item.DeletedAt.IsZero() {
			listed = append(listed, item)
		}
	}
	sort.SliceStable(listed, func(left int, right int) bool {
		if listed[left].SortOrder == listed[right].SortOrder {
			return listed[left].ID < listed[right].ID
		}
		return listed[left].SortOrder < listed[right].SortOrder
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
