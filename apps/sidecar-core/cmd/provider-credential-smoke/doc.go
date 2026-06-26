// Package main 提供数据源凭据真实连接 smoke 命令。
//
// 本命令只用于本地验收，复用生产 DAO、凭据 service 和动态代理 HTTP client；
// 输出必须保持脱敏，不得打印 Cookie、Authorization、Proxy-Authorization 或其他明文凭据。
package main
