package stocks

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	searchservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/search"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Service 是股票 action 依赖的业务搜索边界。
type Service interface {
	Search(ctx context.Context, keyword string, limit int) ([]searchservice.StockSearchResult, error)
}

// Config 是股票 action 的运行期依赖。
type Config struct {
	Security httpx.SecurityConfig
	Service  Service
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

// Routes 返回股票相关路由定义，不直接注册到 Gin。
func Routes(config Config) []httpx.Route {
	return []httpx.Route{
		{Method: http.MethodPost, Path: "/api/stocks/search", Handler: handleSearch(config)},
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
