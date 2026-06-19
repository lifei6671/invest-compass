// Package news 提供个股新闻和市场新闻 HTTP action。
//
// 本包只负责请求校验、新闻 Provider 调用、缓存读写编排和统一响应转换。
// 新闻来源和授权边界由 service 层 Provider 承担，数据库细节由 dao 层注入的 Store 承担。
package news
