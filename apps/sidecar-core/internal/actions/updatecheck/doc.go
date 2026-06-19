// Package updatecheck 提供检查更新 HTTP action。
//
// 本包只负责把本地 API 请求转换为 updatecheck service 规则调用，并通过注入的
// Fetcher 获取远程 manifest。它不负责设置页展示、真实下载、安装或自动升级。
package updatecheck
