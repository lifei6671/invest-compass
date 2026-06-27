use crate::{commands::blocking::post_core_api_with_recovery, sidecar::CoreState};
use serde::Serialize;
use tauri::{AppHandle, State};

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
    #[serde(skip_serializing_if = "is_false")]
    force_refresh: bool,
}

#[derive(Serialize)]
struct NewsStatsRequest {
    market: String,
    limit: i32,
}

/// 获取个股新闻，固定转发到 Go core `/api/news/list`。
#[tauri::command]
pub async fn news_list(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    symbol: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    run_news_request(
        app_handle,
        state.inner().clone(),
        "/api/news/list",
        NewsListRequest { symbol, limit },
    )
    .await
}

/// 获取市场新闻，固定转发到 Go core `/api/news/market`。
#[tauri::command]
pub async fn news_market(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    market: String,
    limit: i32,
    force_refresh: Option<bool>,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    let force_refresh = force_refresh.unwrap_or(false);
    let result = run_news_request(
        app_handle.clone(),
        state.inner().clone(),
        "/api/news/market",
        news_market_request(market.clone(), limit, force_refresh),
    )
    .await;
    if let Err(error) = &result {
        if should_retry_legacy_news_market_request(error, force_refresh) {
            return run_news_request(
                app_handle,
                state.inner().clone(),
                "/api/news/market",
                news_market_request(market, limit, false),
            )
            .await;
        }
    }
    result
}

/// 判断是否为旧版 Go core 不识别 force_refresh 造成的 400，用于开发期热更新兼容回退。
fn should_retry_legacy_news_market_request(error: &str, force_refresh: bool) -> bool {
    force_refresh && error.contains("400 Bad Request") && error.contains("/api/news/market")
}

/// 序列化市场新闻请求时跳过 false，避免旧版 Go core 因未知字段拒绝普通加载请求。
fn is_false(value: &bool) -> bool {
    !*value
}

/// 构造市场新闻请求 payload，集中保证默认请求不携带新增兼容字段。
fn news_market_request(market: String, limit: i32, force_refresh: bool) -> NewsMarketRequest {
    NewsMarketRequest {
        market,
        limit,
        force_refresh,
    }
}

/// 获取资讯缓存统计，固定转发到 Go core `/api/news/stats`。
#[tauri::command]
pub async fn news_stats(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    market: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    run_news_request(
        app_handle,
        state.inner().clone(),
        "/api/news/stats",
        NewsStatsRequest { market, limit },
    )
    .await
}

/// 获取资讯缓存热点统计，固定转发到 Go core `/api/news/hot-topics`。
#[tauri::command]
pub async fn news_hot_topics(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    market: String,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_news_limit(limit)?;
    run_news_request(
        app_handle,
        state.inner().clone(),
        "/api/news/hot-topics",
        NewsStatsRequest { market, limit },
    )
    .await
}

/// 新闻命令统一通过状态恢复 API 调用 Go core，避免启动瞬间旧端口失效。
async fn run_news_request<TRequest>(
    app_handle: AppHandle,
    state: CoreState,
    path: &'static str,
    payload: TRequest,
) -> Result<serde_json::Value, String>
where
    TRequest: Serialize + Send + 'static,
{
    post_core_api_with_recovery(state, app_handle, path, payload).await
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
    use std::fs;

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

    #[test]
    /// 验证市场新闻默认请求不携带 force_refresh，避免旧版 Go core 因未知字段返回 400。
    fn news_market_request_omits_false_force_refresh() {
        let payload = serde_json::to_value(news_market_request("CN".to_string(), 80, false))
            .expect("news market request should serialize");

        assert_eq!(payload["market"], "CN");
        assert_eq!(payload["limit"], 80);
        assert!(
            payload.get("force_refresh").is_none(),
            "default market news request should not send force_refresh=false"
        );
    }

    #[test]
    /// 验证市场新闻手动刷新仍携带 force_refresh，以便新版 Go core 绕过缓存。
    fn news_market_request_includes_true_force_refresh() {
        let payload = serde_json::to_value(news_market_request("CN".to_string(), 80, true))
            .expect("news market request should serialize");

        assert_eq!(payload["force_refresh"], true);
    }

    #[test]
    /// 验证只有强制刷新遇到旧版 market 接口 400 时才触发兼容回退。
    fn legacy_news_market_retry_only_handles_force_refresh_bad_request() {
        let bad_request = "sidecar http error: HTTP status client error (400 Bad Request) for url (http://127.0.0.1:56880/api/news/market)";

        assert!(should_retry_legacy_news_market_request(bad_request, true));
        assert!(!should_retry_legacy_news_market_request(bad_request, false));
        assert!(!should_retry_legacy_news_market_request(
            "sidecar http error: HTTP status client error (500 Internal Server Error) for url (http://127.0.0.1:56880/api/news/market)",
            true,
        ));
        assert!(!should_retry_legacy_news_market_request(
            "sidecar http error: HTTP status client error (400 Bad Request) for url (http://127.0.0.1:56880/api/news/list)",
            true,
        ));
    }

    #[test]
    /// 验证新闻 command 通过可恢复请求路径访问 Go core，避免旧端口失效导致资讯页空白。
    fn news_commands_use_recoverable_core_requests() {
        let source = fs::read_to_string(file!()).expect("news command source should be readable");

        for command in ["news_list", "news_market", "news_stats", "news_hot_topics"] {
            assert!(
                source.contains(&format!("pub async fn {command}(")),
                "{command} must be async to avoid blocking the WebKit main thread"
            );
        }
        assert!(
            source.contains("post_core_api_with_recovery"),
            "news commands must recover stale sidecar ports before surfacing transport errors"
        );
        assert!(
            source.contains("async fn run_news_request"),
            "news commands should share the recoverable request boundary"
        );
        let fn_start = source
            .find("pub async fn news_market(")
            .expect("news_market command should exist");
        let fn_end = source[fn_start..]
            .find("/// 获取资讯缓存统计")
            .map(|offset| fn_start + offset)
            .expect("news_market command should be followed by news_stats");
        let market_command_source = &source[fn_start..fn_end];
        let expected_market_signature = [
            "pub async fn news_market(",
            "app_handle: AppHandle,",
            "state: State<'_, CoreState>,",
            "market: String,",
            "limit: i32,",
            "force_refresh: Option<bool>,",
        ];
        for snippet in expected_market_signature {
            assert!(
                market_command_source.contains(snippet),
                "news_market must accept force_refresh so manual refresh can bypass cached market news"
            );
        }
        assert!(
            market_command_source.contains("let force_refresh = force_refresh.unwrap_or(false);"),
            "news_market must forward force_refresh to Go core as snake_case"
        );
        assert!(
            market_command_source.contains("should_retry_legacy_news_market_request"),
            "news_market should retry without force_refresh when an old sidecar rejects the new field"
        );
        let stale_client_binding = ["let client = ", "state.client"].concat();
        assert!(
            !source.contains(&stale_client_binding),
            "news commands must not capture a stale CoreClient directly"
        );
    }
}
