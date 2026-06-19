// Package market 提供行情 quote 和 K 线 HTTP action。
//
// 本包只负责请求校验、Provider 调用、缓存读写编排和统一响应转换。
// 路由注册由 actions 根包完成，数据库细节由 dao 层注入的 Store 承担。
package market
