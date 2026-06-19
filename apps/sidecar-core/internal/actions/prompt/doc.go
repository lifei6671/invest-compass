// Package prompt 提供 Prompt 模板本地 HTTP action。
//
// 本包只负责请求解析、统一 envelope 输出和调用 service/dao 边界，不直接注册 Gin 路由，
// 也不承载 Prompt 构建、AI 调用或前端展示逻辑。
package prompt
