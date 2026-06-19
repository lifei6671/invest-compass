// Package fundflow 提供资金流数据源的后端抽象和解析实现。
//
// 当前包只承载后续版本可复用的 Provider 能力，不注册首版 HTTP API、Rust command、
// SQLite 表或前端入口。资金流属于后续市场复盘和个股详情增强能力，进入用户可见链路前
// 需要重新确认数据授权、访问频率、缓存策略和产品范围。
package fundflow
