use crate::sidecar::CoreState;
use serde::Serialize;
use tauri::State;

const NEWS_MAX_LIMIT: i32 = 100;

#[derive(Serialize)]
struct NewsListRequest {
    symbol: String,
    limit: i32,
}

#[derive(Serialize)]
struct NewsMarketRequest {
    market: String,
    limit: i32,
}

/// 获取个股新闻，固定转发到 Go core `/api/news/list`。
#[tauri::command]
pub fn news_list(
    state: State<'_, CoreState>,
    symbol: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/news/list", &NewsListRequest { symbol, limit })
        .map_err(|error| error.to_string())
}

/// 获取市场新闻，固定转发到 Go core `/api/news/market`。
#[tauri::command]
pub fn news_market(
    state: State<'_, CoreState>,
    market: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/news/market", &NewsMarketRequest { market, limit })
        .map_err(|error| error.to_string())
}

/// 校验新闻列表长度，避免 Rust command 转发无界新闻查询。
fn validate_news_limit(limit: i32) -> Result<(), String> {
    if limit <= 0 || limit > NEWS_MAX_LIMIT {
        return Err("invalid news limit".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证新闻 command 在 Rust 边界拒绝无界 limit。
    fn validate_news_limit_rejects_unbounded_values() {
        assert!(validate_news_limit(1).is_ok());
        assert!(validate_news_limit(100).is_ok());
        assert_eq!(
            validate_news_limit(0).expect_err("zero limit should fail"),
            "invalid news limit"
        );
        assert_eq!(
            validate_news_limit(101).expect_err("too large limit should fail"),
            "invalid news limit"
        );
    }
}
