// Package sidecar 处理 Rust/Tauri 与 Go core 之间的 sidecar 启动握手协议。
//
// 该包只负责 stdin token 握手和 ready 消息结构，不负责进程管理；
// sidecar 生命周期由 Rust 层负责，Go core 只接收一次性启动上下文。
package sidecar
