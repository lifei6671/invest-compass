package watchlist

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	stockservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	watchlistservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/watchlist"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是 watchlist action 依赖的数据访问边界。
type Store interface {
	SaveWatchlist(ctx context.Context, item *model.Watchlist) error
	ListActiveWatchlists(ctx context.Context) ([]model.Watchlist, error)
	SoftDeleteWatchlist(ctx context.Context, id int64) error
}

// StockProfileStore 是自选股列表可选使用的股票资料批量读取边界。
type StockProfileStore interface {
	GetStocksBySymbols(ctx context.Context, symbols []string) (map[string]model.Stock, error)
}

// Config 是 watchlist action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Store    Store
}

type createRequest struct {
	Symbol    string   `json:"symbol"`
	SortOrder int      `json:"sort_order"`
	Tags      []string `json:"tags"`
	Note      string   `json:"note"`
}

type updateRequest struct {
	ID        int64    `json:"id"`
	SortOrder int      `json:"sort_order"`
	Tags      []string `json:"tags"`
	Note      string   `json:"note"`
}

type deleteRequest struct {
	ID int64 `json:"id"`
}

type listData struct {
	Items []itemData `json:"items"`
}

type itemData struct {
	ID        int64    `json:"id"`
	Symbol    string   `json:"symbol"`
	SortOrder int      `json:"sort_order"`
	Tags      []string `json:"tags"`
	Note      string   `json:"note"`
	Name      string   `json:"name,omitempty"`
	Code      string   `json:"code,omitempty"`
	Market    string   `json:"market,omitempty"`
	Exchange  string   `json:"exchange,omitempty"`
	Industry  string   `json:"industry,omitempty"`
	Concepts  []string `json:"concepts,omitempty"`
	ListDate  string   `json:"list_date,omitempty"`
	Status    string   `json:"status,omitempty"`
	FullName  string   `json:"full_name,omitempty"`
}

// Routes 返回自选股相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/watchlist/list", Handler: handleList(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/create", Handler: handleCreate(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/update", Handler: handleUpdate(config)},
		{Method: http.MethodPost, Path: "/api/watchlist/delete", Handler: handleDelete(config)},
	}
}

// handleList 返回未软删除自选股。
func handleList(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		items, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		profiles, err := loadStockProfiles(request.Context(), config.Store, items)
		if err != nil {
			writeStoreError(response, context, "读取自选股股票资料失败", err)
			return
		}
		httpx.WriteOK(response, listData{Items: modelItemsToData(items, profiles)}, context)
	}
}

// handleCreate 校验 symbol 并创建自选股。
func handleCreate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload createRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		existing, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		item, err := watchlistservice.CreateItem(modelItemsToService(existing), watchlistservice.CreateRequest{
			Symbol:    payload.Symbol,
			SortOrder: payload.SortOrder,
			Tags:      payload.Tags,
			Note:      payload.Note,
		}, time.Now())
		if err != nil {
			writeValidationError(response, context, err)
			return
		}

		modelItem := serviceItemToModel(item)
		if err := config.Store.SaveWatchlist(request.Context(), &modelItem); err != nil {
			writeStoreError(response, context, "保存自选股失败", err)
			return
		}
		httpx.WriteOK(response, modelItemToData(modelItem, nil), context)
	}
}

// handleUpdate 更新自选股标签、备注和排序。
func handleUpdate(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload updateRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		items, err := config.Store.ListActiveWatchlists(request.Context())
		if err != nil {
			writeStoreError(response, context, "读取自选股失败", err)
			return
		}
		modelItem, ok := findModelItem(items, payload.ID)
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40401, "watchlist_not_found", context)
			return
		}

		serviceItem, err := modelItemToService(modelItem)
		if err != nil {
			writeValidationError(response, context, err)
			return
		}
		updated := watchlistservice.UpdateItem(serviceItem, watchlistservice.UpdateRequest{
			SortOrder: payload.SortOrder,
			Tags:      payload.Tags,
			Note:      payload.Note,
		}, time.Now())

		modelItem = serviceItemToModel(updated)
		if err := config.Store.SaveWatchlist(request.Context(), &modelItem); err != nil {
			writeStoreError(response, context, "更新自选股失败", err)
			return
		}
		httpx.WriteOK(response, modelItemToData(modelItem, nil), context)
	}
}

// handleDelete 对自选股执行软删除。
func handleDelete(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !requireStore(response, request, config, context) {
			return
		}

		var payload deleteRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		if payload.ID <= 0 {
			httpx.WriteError(response, http.StatusBadRequest, 40004, "invalid_watchlist_id", context)
			return
		}
		if err := config.Store.SoftDeleteWatchlist(request.Context(), payload.ID); err != nil {
			writeStoreError(response, context, "删除自选股失败", err)
			return
		}

		httpx.WriteOK(response, map[string]int64{"id": payload.ID}, context)
	}
}

// requireStore 校验 ready/token 和 watchlist store 注入。
func requireStore(response http.ResponseWriter, request *http.Request, config Config, context httpx.RequestContext) bool {
	if !httpx.RequireReadyToken(response, request, config.Security, context) {
		return false
	}
	if config.Store == nil {
		httpx.WriteError(response, http.StatusServiceUnavailable, 50305, "watchlist_store_unavailable", context)
		return false
	}
	return true
}

// writeValidationError 把 service 层稳定错误码映射为 HTTP 错误消息。
func writeValidationError(response http.ResponseWriter, context httpx.RequestContext, err error) {
	if xerrValue, ok := err.(*xerr.Error); ok {
		httpx.WriteError(response, http.StatusBadRequest, 40005, string(xerrValue.Code), context)
		return
	}
	httpx.WriteError(response, http.StatusBadRequest, 40005, "invalid_watchlist", context)
}

// writeStoreError 记录脱敏后的数据库错误，并返回统一错误 envelope。
func writeStoreError(response http.ResponseWriter, context httpx.RequestContext, message string, err error) {
	slog.Warn(
		message,
		logger.FieldRequestID, context.RequestID,
		logger.FieldTraceID, context.TraceID,
		"error", logger.RedactError(err),
	)
	httpx.WriteError(response, http.StatusInternalServerError, 50005, "watchlist_store_error", context)
}

// modelItemsToData 转换数据库模型为 API 响应模型，并合并同 symbol 股票资料。
func modelItemsToData(items []model.Watchlist, profiles map[string]model.Stock) []itemData {
	result := make([]itemData, 0, len(items))
	for _, item := range items {
		result = append(result, modelItemToData(item, profiles))
	}
	return result
}

// modelItemToData 转换单个数据库模型为 API 响应模型。
func modelItemToData(item model.Watchlist, profiles map[string]model.Stock) itemData {
	data := itemData{
		ID:        item.ID,
		Symbol:    item.Symbol,
		SortOrder: item.SortOrder,
		Tags:      decodeTags(item.Tags),
		Note:      item.Note,
	}
	if stock, ok := profiles[item.Symbol]; ok {
		data.Name = stock.Name
		data.Code = stock.Code
		data.Market = stock.Market
		data.Exchange = stock.Exchange
		data.Industry = stock.Industry
		data.Concepts = decodeConcepts(stock.Concept)
		data.ListDate = stock.ListDate
		data.Status = stock.Status
		data.FullName = stock.FullName
	}
	return data
}

// modelItemsToService 转换数据库模型为 service 模型，用于复用业务规则。
func modelItemsToService(items []model.Watchlist) []watchlistservice.Item {
	result := make([]watchlistservice.Item, 0, len(items))
	for _, item := range items {
		serviceItem, err := modelItemToService(item)
		if err == nil {
			result = append(result, serviceItem)
		}
	}
	return result
}

// modelItemToService 转换单个数据库模型为 service 模型。
func modelItemToService(item model.Watchlist) (watchlistservice.Item, error) {
	symbol, err := stockFromModel(item.Symbol)
	if err != nil {
		return watchlistservice.Item{}, err
	}
	return watchlistservice.Item{
		ID:        item.ID,
		Symbol:    symbol,
		SortOrder: item.SortOrder,
		Tags:      decodeTags(item.Tags),
		Note:      item.Note,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

// serviceItemToModel 转换 service 模型为数据库模型。
func serviceItemToModel(item watchlistservice.Item) model.Watchlist {
	return model.Watchlist{
		ID:        item.ID,
		Symbol:    item.Symbol.String(),
		SortOrder: item.SortOrder,
		Tags:      encodeTags(item.Tags),
		Note:      item.Note,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

// findModelItem 按 ID 查找当前 active 自选股。
func findModelItem(items []model.Watchlist, id int64) (model.Watchlist, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Watchlist{}, false
}

// encodeTags 使用 JSON 数组保存标签，避免分隔符和用户输入冲突。
func encodeTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(payload)
}

// decodeTags 解析 JSON 标签数组，历史异常值按空标签处理。
func decodeTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

// loadStockProfiles 使用同一个 Store 的可选能力批量读取股票资料，未实现时保持列表基础字段可用。
func loadStockProfiles(ctx context.Context, store Store, items []model.Watchlist) (map[string]model.Stock, error) {
	profileStore, ok := store.(StockProfileStore)
	if !ok {
		return nil, nil
	}
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		symbols = append(symbols, item.Symbol)
	}
	return profileStore.GetStocksBySymbols(ctx, symbols)
}

// decodeConcepts 兼容 JSON 数组和历史分隔符文本，统一向前端暴露数组。
func decodeConcepts(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var concepts []string
	if err := json.Unmarshal([]byte(value), &concepts); err == nil {
		return concepts
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	result := make([]string, 0, len(fields))
	for _, item := range fields {
		if concept := strings.TrimSpace(item); concept != "" {
			result = append(result, concept)
		}
	}
	return result
}

// stockFromModel 解析数据库中的标准 symbol。
func stockFromModel(symbol string) (stockservice.Symbol, error) {
	parsed, err := stockservice.ParseSymbol(symbol)
	if err != nil {
		return stockservice.Symbol{}, err
	}
	return parsed, nil
}
