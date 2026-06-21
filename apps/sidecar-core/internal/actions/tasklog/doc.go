// Package tasklog exposes fixed local HTTP actions for task structured logs.
//
// 本包只负责把 tasklog service 通过固定 POST 路由暴露给 Rust 白名单命令。
// 它不承载普通应用日志平台能力，也不提供任意路径代理。
package tasklog
