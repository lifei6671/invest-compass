// Package watchlist 提供自选股增删改查 action。
//
// 本包只声明 HTTP handler 和固定路由，不直接依赖 Gin 注册 API；业务规则复用
// service/watchlist，数据库访问通过 Store 接口注入。
package watchlist
