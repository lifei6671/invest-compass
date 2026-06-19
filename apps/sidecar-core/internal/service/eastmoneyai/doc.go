// Package eastmoneyai 提供东方财富 AI SaaS 工具接口的可选 Provider。
//
// 本包只实现后端可复用的调用和解析能力，不读取全局配置、不保存 API Key、不注册首版
// HTTP API、Rust command 或 UI 入口。后续若要接入产品链路，应由 Rust 本地 vault 解析
// 临时密钥后注入本 Provider，并重新确认东方财富接口授权、限频和报告合规展示边界。
package eastmoneyai
