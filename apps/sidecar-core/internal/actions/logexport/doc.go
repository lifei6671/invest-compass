// Package logexport 提供日志导出 HTTP action。
//
// 本包只负责从注入的数据源读取日志行，生成已二次脱敏的导出包。它不负责把文件
// 写到用户目录，文件系统写入必须留给 Rust 白名单命令和用户授权路径处理。
package logexport
