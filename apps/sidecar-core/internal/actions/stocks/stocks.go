package stocks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Service 是股票 action 依赖的业务搜索边界。
type Service interface {
	Search(ctx context.Context, keyword string, limit int) ([]searchservice.StockSearchResult, error)
}

// ProfileStore 是股票资料查询依赖的数据库读取边界。
type ProfileStore interface {
	GetStockBySymbol(ctx context.Context, symbol string) (model.Stock, bool, error)
}

// Config 是股票 action 的运行期依赖。
type Config struct {
	Security     httpx.SecurityConfig
	Service      Service
	ProfileStore ProfileStore
}

type searchRequest struct {
	Keyword string `json:"keyword"`
}

type searchResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Market   string `json:"market"`
	Exchange string `json:"exchange"`
}

type profileRequest struct {
	Symbol string `json:"symbol"`
}

type profileData struct {
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Code     string   `json:"code"`
	Market   string   `json:"market"`
	Exchange string   `json:"exchange"`
	Industry string   `json:"industry"`
	Concepts []string `json:"concepts"`
	ListDate string   `json:"list_date"`
	Status   string   `json:"status"`
	FullName string   `json:"full_name"`
}

// Routes 返回股票相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/stocks/search", Handler: handleSearch(config)},
		{Method: http.MethodPost, Path: "/api/stocks/profile", Handler: handleProfile(config)},
	}
}

// handleSearch 处理股票搜索请求，返回标准化股票基础信息。
func handleSearch(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.Service == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "stock_search_service_unavailable", context)
			return
		}

		var payload searchRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}

		keyword := strings.TrimSpace(payload.Keyword)
		if keyword == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_keyword", context)
			return
		}

		results, err := config.Service.Search(request.Context(), keyword, 20)
		if err != nil {
			var ruleError *xerr.Error
			if errors.As(err, &ruleError) && ruleError.Code == xerr.MarketProviderUnconfigured {
				httpx.WriteError(response, http.StatusServiceUnavailable, 50301, string(ruleError.Code), context)
				return
			}
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "stock_search_error", context)
			return
		}
		httpx.WriteOK(response, buildSearchResults(results), context)
	}
}

// handleProfile 返回 SQLite 中的股票基础资料，避免详情页和自选股各自拼装资料。
func handleProfile(config Config) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		context := httpx.ContextFrom(request)
		if !httpx.RequireReadyToken(response, request, config.Security, context) {
			return
		}
		if config.ProfileStore == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "stock_profile_store_unavailable", context)
			return
		}

		var payload profileRequest
		if !httpx.DecodeJSON(response, request, context, &payload) {
			return
		}
		symbol := strings.TrimSpace(payload.Symbol)
		if symbol == "" {
			httpx.WriteError(response, http.StatusBadRequest, 40002, "invalid_stock_symbol", context)
			return
		}

		stock, ok, err := config.ProfileStore.GetStockBySymbol(request.Context(), symbol)
		if err != nil {
			httpx.WriteError(response, http.StatusInternalServerError, 50000, "stock_profile_store_error", context)
			return
		}
		if !ok {
			httpx.WriteError(response, http.StatusNotFound, 40401, "stock_profile_not_found", context)
			return
		}
		httpx.WriteOK(response, stockToProfileData(stock), context)
	}
}

// buildSearchResults 转换 service 模型为 API 稳定响应字段。
func buildSearchResults(stocks []searchservice.StockSearchResult) []searchResult {
	results := make([]searchResult, 0, len(stocks))
	for _, item := range stocks {
		results = append(results, searchResult{
			Symbol:   item.Symbol,
			Name:     item.Name,
			Code:     item.Code,
			Market:   item.Market,
			Exchange: item.Exchange,
		})
	}
	return results
}

// stockToProfileData 把持久化股票资料转换为前端稳定使用的资料字段。
func stockToProfileData(stock model.Stock) profileData {
	return profileData{
		Symbol:   stock.Symbol,
		Name:     stock.Name,
		Code:     stock.Code,
		Market:   stock.Market,
		Exchange: stock.Exchange,
		Industry: stock.Industry,
		Concepts: decodeConcepts(stock.Concept),
		ListDate: stock.ListDate,
		Status:   stock.Status,
		FullName: stock.FullName,
	}
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
