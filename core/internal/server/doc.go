// Package server 提供 Go core 的本地 HTTP server 基线能力。
//
// 该包负责统一响应结构、请求追踪 ID、POST-only 限制、token 校验和健康检查。
// 业务路由应在后续任务按模块接入，不应在这里直接实现行情、AI 或数据库逻辑。
package server
