// Package stockdata 提供股票数据源适配时可复用的轻量解析工具。
//
// 本包不定义 Provider 契约、不发起 HTTP 请求、不依赖具体数据源，也不持有数据库逻辑。
// 真实行情能力统一收口在 internal/service/market.MarketProvider。
package stockdata
