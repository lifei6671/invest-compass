// Package main 是 invest-compas-core sidecar 的进程入口。
//
// 该包只负责解析启动参数、完成 stdin 握手、启动本地 HTTP server，
// 不承载行情、AI、数据库等业务逻辑，避免入口层变成业务聚合点。
package main
