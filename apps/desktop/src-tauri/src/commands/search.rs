use crate::{commands::blocking::post_core_api_with_recovery, sidecar::CoreState};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, State};

const SEARCH_MAX_LIMIT: i32 = 100;
const SEARCH_MAX_SYMBOLS: usize = 20;

#[derive(Deserialize, Serialize)]
pub struct SearchDocumentsPayload {
    keyword: String,
    symbols: Vec<String>,
    limit: i32,
    offset: i32,
    sort: String,
}

#[derive(Serialize)]
struct SearchStatusRequest {}

#[derive(Deserialize, Serialize)]
pub struct SearchRebuildPayload {
    scope: String,
}

/// 搜索报告历史范围，固定转发到 Go core `/api/search/reports`。
#[tauri::command]
pub async fn search_reports(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: SearchDocumentsPayload,
) -> Result<serde_json::Value, String> {
    validate_search_payload(&payload)?;
    post_search_api(
        app_handle,
        state.inner().clone(),
        "/api/search/reports",
        payload,
    )
    .await
}

/// 搜索资讯范围，固定转发到 Go core `/api/search/news`。
#[tauri::command]
pub async fn search_news(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: SearchDocumentsPayload,
) -> Result<serde_json::Value, String> {
    validate_search_payload(&payload)?;
    post_search_api(
        app_handle,
        state.inner().clone(),
        "/api/search/news",
        payload,
    )
    .await
}

/// 搜索自选备注范围，固定转发到 Go core `/api/search/watchlist-notes`。
#[tauri::command]
pub async fn search_watchlist_notes(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: SearchDocumentsPayload,
) -> Result<serde_json::Value, String> {
    validate_search_payload(&payload)?;
    post_search_api(
        app_handle,
        state.inner().clone(),
        "/api/search/watchlist-notes",
        payload,
    )
    .await
}

/// 读取搜索索引状态，固定转发到 Go core `/api/search/status`。
#[tauri::command]
pub async fn search_status(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_search_api(
        app_handle,
        state.inner().clone(),
        "/api/search/status",
        SearchStatusRequest {},
    )
    .await
}

/// 触发搜索索引重建，固定转发到 Go core `/api/search/rebuild`。
#[tauri::command]
pub async fn search_rebuild(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: SearchRebuildPayload,
) -> Result<serde_json::Value, String> {
    validate_search_rebuild_payload(&payload)?;
    post_search_api(
        app_handle,
        state.inner().clone(),
        "/api/search/rebuild",
        payload,
    )
    .await
}

/// 搜索命令统一通过状态恢复 API 调用 Go core，处理启动瞬间旧端口失效的连接竞态。
async fn post_search_api<TRequest>(
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

/// 校验菜单范围搜索请求，避免 Rust command 转发空关键词或无界分页。
fn validate_search_payload(payload: &SearchDocumentsPayload) -> Result<(), String> {
    if payload.keyword.trim().is_empty() {
        return Err("invalid search keyword".to_string());
    }
    if payload.limit <= 0 || payload.limit > SEARCH_MAX_LIMIT {
        return Err("invalid search limit".to_string());
    }
    if payload.offset < 0 {
        return Err("invalid search offset".to_string());
    }
    validate_search_symbols(&payload.symbols)
}

/// 校验 symbol 过滤条件，避免空 symbol 或超长过滤集合进入 Go core。
fn validate_search_symbols(symbols: &[String]) -> Result<(), String> {
    if symbols.len() > SEARCH_MAX_SYMBOLS {
        return Err("invalid search symbols".to_string());
    }
    if symbols.iter().any(|symbol| symbol.trim().is_empty()) {
        return Err("invalid search symbols".to_string());
    }
    Ok(())
}

/// 校验索引重建范围，只允许技术方案列出的固定 scope。
fn validate_search_rebuild_payload(payload: &SearchRebuildPayload) -> Result<(), String> {
    match payload.scope.as_str() {
        "all" | "stock" | "reports" | "news" | "watchlist_notes" => Ok(()),
        _ => Err("invalid search rebuild scope".to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证菜单范围搜索请求在 Rust 边界拒绝空关键词和无界分页。
    fn validate_search_payload_rejects_invalid_keyword_and_pagination() {
        assert!(validate_search_payload(&SearchDocumentsPayload {
            keyword: "茅台".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
            limit: 20,
            offset: 0,
            sort: "relevance".to_string(),
        })
        .is_ok());
        assert_eq!(
            validate_search_payload(&SearchDocumentsPayload {
                keyword: " \t\n".to_string(),
                symbols: vec![],
                limit: 20,
                offset: 0,
                sort: "relevance".to_string(),
            })
            .expect_err("blank keyword should fail"),
            "invalid search keyword"
        );
        assert_eq!(
            validate_search_payload(&SearchDocumentsPayload {
                keyword: "茅台".to_string(),
                symbols: vec![],
                limit: 101,
                offset: 0,
                sort: "relevance".to_string(),
            })
            .expect_err("too large limit should fail"),
            "invalid search limit"
        );
        assert_eq!(
            validate_search_payload(&SearchDocumentsPayload {
                keyword: "茅台".to_string(),
                symbols: vec![],
                limit: 20,
                offset: -1,
                sort: "relevance".to_string(),
            })
            .expect_err("negative offset should fail"),
            "invalid search offset"
        );
    }

    #[test]
    /// 验证菜单范围搜索在 Rust 边界拒绝空 symbol 和过长 symbol 过滤集合。
    fn validate_search_payload_rejects_invalid_symbols() {
        assert_eq!(
            validate_search_payload(&SearchDocumentsPayload {
                keyword: "茅台".to_string(),
                symbols: vec![" ".to_string()],
                limit: 20,
                offset: 0,
                sort: "relevance".to_string(),
            })
            .expect_err("blank symbol should fail"),
            "invalid search symbols"
        );
        assert_eq!(
            validate_search_payload(&SearchDocumentsPayload {
                keyword: "茅台".to_string(),
                symbols: (0..21).map(|index| format!("CN:SH:{index:06}")).collect(),
                limit: 20,
                offset: 0,
                sort: "relevance".to_string(),
            })
            .expect_err("too many symbols should fail"),
            "invalid search symbols"
        );
    }

    #[test]
    /// 验证索引重建 command 只允许固定 scope，不接受任意 doc_types。
    fn validate_search_rebuild_scope_rejects_unknown_scope() {
        for scope in ["all", "stock", "reports", "news", "watchlist_notes"] {
            assert!(validate_search_rebuild_payload(&SearchRebuildPayload {
                scope: scope.to_string(),
            })
            .is_ok());
        }
        assert_eq!(
            validate_search_rebuild_payload(&SearchRebuildPayload {
                scope: "global".to_string(),
            })
            .expect_err("unknown scope should fail"),
            "invalid search rebuild scope"
        );
    }
}
