// Package fundamental 提供股票基本面和 F10 数据的 Provider 抽象。
//
// 本包只承载基本面数据的抓取、结构化和展示格式化，不负责行情、K 线、
// 新闻、AI 调用或数据库缓存。外部数据源必须通过 Provider 接口接入，避免
// 业务层直接依赖某一家公开网页接口。
package fundamental
