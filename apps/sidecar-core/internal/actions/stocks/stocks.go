package stocks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Store 是股票 action 依赖的数据访问边界。
type Store interface {
	UpsertStocks(ctx context.Context, stocks []model.Stock) error
}

// Config 是股票 action 的运行期依赖。
type Config struct {
	Security       httpx.SecurityConfig
	MarketProvider market.MarketProvider
	Store          Store
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
		if config.MarketProvider == nil {
			httpx.WriteError(response, http.StatusServiceUnavailable, 50301, "market_provider_unavailable", context)
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

		stocks, err := config.MarketProvider.Search(request.Context(), keyword)
		if err != nil {
			var ruleError *xerr.Error
			if errors.As(err, &ruleError) && ruleError.Code == xerr.MarketProviderUnconfigured {
				httpx.WriteError(response, http.StatusServiceUnavailable, 50301, string(ruleError.Code), context)
				return
			}
			slog.Warn(
				"股票搜索 Provider 调用失败",
				logger.FieldRequestID, context.RequestID,
				logger.FieldTraceID, context.TraceID,
				logger.FieldProvider, config.MarketProvider.Name(),
				"error", logger.RedactError(err),
			)
			httpx.WriteError(response, http.StatusBadGateway, 50200, "market_provider_error", context)
			return
		}
		if config.Store != nil {
			if err := config.Store.UpsertStocks(request.Context(), modelStocksFromMarket(stocks)); err != nil {
				slog.Warn(
					"股票基础信息缓存写入失败",
					logger.FieldRequestID, context.RequestID,
					logger.FieldTraceID, context.TraceID,
					"error", logger.RedactError(err),
				)
				httpx.WriteError(response, http.StatusInternalServerError, 50004, "stock_cache_error", context)
				return
			}
		}

		httpx.WriteOK(response, buildSearchResults(stocks), context)
	}
}

// buildSearchResults 转换 Provider 模型为 API 稳定响应字段。
func buildSearchResults(stocks []market.StockBasic) []searchResult {
	results := make([]searchResult, 0, len(stocks))
	for _, item := range stocks {
		results = append(results, searchResult{
			Symbol:   item.Symbol.String(),
			Name:     item.Name,
			Code:     item.Code,
			Market:   item.Market,
			Exchange: item.Exchange,
		})
	}
	return results
}

// modelStocksFromMarket 转换 Provider 股票基础信息为可持久化缓存模型。
func modelStocksFromMarket(stocks []market.StockBasic) []model.Stock {
	result := make([]model.Stock, 0, len(stocks))
	for _, item := range stocks {
		result = append(result, model.Stock{
			Symbol:   item.Symbol.String(),
			Market:   item.Market,
			Code:     item.Code,
			Name:     item.Name,
			Exchange: item.Exchange,
			Industry: item.Industry,
			Concept:  item.Concept,
		})
	}
	return result
}
