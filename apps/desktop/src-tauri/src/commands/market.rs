use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::Serialize;
use tauri::State;

const MARKET_MAX_LIMIT: i32 = 500;

#[derive(Serialize)]
struct StockSearchRequest {
    keyword: String,
}

#[derive(Serialize)]
struct StockProfileRequest {
    symbol: String,
}

#[derive(Serialize)]
struct MarketQuoteRequest {
    symbol: String,
    force_refresh: bool,
}

#[derive(Serialize)]
struct MarketKlineRequest {
    symbol: String,
    period: String,
    adjust: String,
    limit: i32,
}

#[derive(Serialize)]
struct MarketIndicatorsRequest {
    symbol: String,
    period: String,
    adjust: String,
    limit: i32,
    indicators: Vec<String>,
}

/// 搜索股票基础信息，固定转发到 Go core `/api/stocks/search`。
#[tauri::command]
pub async fn stock_search(
    state: State<'_, CoreState>,
    keyword: String,
) -> Result<serde_json::Value, String> {
    validate_stock_search_keyword(&keyword)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/stocks/search", StockSearchRequest { keyword }).await
}

/// 获取股票基础资料，固定转发到 Go core `/api/stocks/profile`。
#[tauri::command]
pub async fn stock_profile(
    state: State<'_, CoreState>,
    symbol: String,
) -> Result<serde_json::Value, String> {
    validate_stock_symbol(&symbol)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/stocks/profile",
        StockProfileRequest { symbol },
    )
    .await
}

/// 获取股票行情快照，固定转发到 Go core `/api/market/quote`。
#[tauri::command]
pub async fn market_quote(
    state: State<'_, CoreState>,
    symbol: String,
    force_refresh: Option<bool>,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/market/quote",
        MarketQuoteRequest {
            symbol,
            force_refresh: force_refresh.unwrap_or(false),
        },
    )
    .await
}

/// 获取股票 K 线，固定转发到 Go core `/api/market/kline`。
#[tauri::command]
pub async fn market_kline(
    state: State<'_, CoreState>,
    symbol: String,
    period: String,
    adjust: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_market_limit(limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/market/kline",
        MarketKlineRequest {
            symbol,
            period,
            adjust,
            limit,
        },
    )
    .await
}

/// 获取股票技术指标，固定转发到 Go core `/api/market/indicators`。
#[tauri::command]
pub async fn market_indicators(
    state: State<'_, CoreState>,
    symbol: String,
    period: String,
    adjust: String,
    limit: i32,
    indicators: Vec<String>,
) -> Result<serde_json::Value, String> {
    validate_market_limit(limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/market/indicators",
        MarketIndicatorsRequest {
            symbol,
            period,
            adjust,
            limit,
            indicators,
        },
    )
    .await
}

/// 校验股票搜索关键词，避免 Rust command 转发空查询到 Go core。
fn validate_stock_search_keyword(keyword: &str) -> Result<(), String> {
    if keyword.trim().is_empty() {
        return Err("invalid stock search keyword".to_string());
    }
    Ok(())
}

/// 校验股票 symbol，避免 Rust command 转发空股票资料请求。
fn validate_stock_symbol(symbol: &str) -> Result<(), String> {
    if symbol.trim().is_empty() {
        return Err("invalid stock symbol".to_string());
    }
    Ok(())
}

/// 校验行情类列表长度，避免 Rust command 转发无界 K 线或指标请求。
fn validate_market_limit(limit: i32) -> Result<(), String> {
    if limit <= 0 || limit > MARKET_MAX_LIMIT {
        return Err("invalid market limit".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证股票搜索 command 在 Rust 边界拒绝空关键词。
    fn validate_stock_search_keyword_rejects_blank_text() {
        assert!(validate_stock_search_keyword("茅台").is_ok());
        assert_eq!(
            validate_stock_search_keyword(" \t\n").expect_err("blank keyword should fail"),
            "invalid stock search keyword"
        );
    }

    #[test]
    /// 验证股票资料 command 在 Rust 边界拒绝空 symbol。
    fn validate_stock_symbol_rejects_blank_text() {
        assert!(validate_stock_symbol("CN:SH:600000").is_ok());
        assert_eq!(
            validate_stock_symbol(" \t\n").expect_err("blank symbol should fail"),
            "invalid stock symbol"
        );
    }

    #[test]
    /// 验证 K 线和技术指标 command 在 Rust 边界拒绝无界 limit。
    fn validate_market_limit_rejects_unbounded_values() {
        assert!(validate_market_limit(1).is_ok());
        assert!(validate_market_limit(500).is_ok());
        assert_eq!(
            validate_market_limit(0).expect_err("zero limit should fail"),
            "invalid market limit"
        );
        assert_eq!(
            validate_market_limit(501).expect_err("too large limit should fail"),
            "invalid market limit"
        );
    }
}
