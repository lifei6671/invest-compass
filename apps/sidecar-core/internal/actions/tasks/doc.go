// Package tasks 提供任务历史与事件回放本地 HTTP action。
//
// 本包只负责查询任务状态和回放已持久化事件，不执行 AI 分析任务，也不直接注册 Gin 路由。
package tasks
