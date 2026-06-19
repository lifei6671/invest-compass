// Package aiconfig 提供 AI 配置元数据 HTTP action。
//
// 本包只处理可持久化的配置元数据。真实 API Key 必须由 Rust 写入本地 vault，
// Go core 只接收 api_key_ref、masked_api_key 和 has_api_key，不保存明文密钥。
package aiconfig
