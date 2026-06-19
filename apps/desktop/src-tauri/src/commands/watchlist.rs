use crate::sidecar::CoreState;
use serde::{Deserialize, Serialize};
use tauri::State;

#[derive(Serialize)]
struct WatchlistListRequest {}

#[derive(Deserialize, Serialize)]
pub struct WatchlistCreatePayload {
    symbol: String,
    sort_order: i32,
    tags: Vec<String>,
    note: String,
}

#[derive(Deserialize, Serialize)]
pub struct WatchlistUpdatePayload {
    id: i64,
    sort_order: i32,
    tags: Vec<String>,
    note: String,
}

#[derive(Serialize)]
struct WatchlistDeleteRequest {
    id: i64,
}

/// 读取自选股列表，固定转发到 Go core `/api/watchlist/list`。
#[tauri::command]
pub async fn watchlist_list(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/watchlist/list", &WatchlistListRequest {})
        .map_err(|error| error.to_string())
}

/// 创建自选股，固定转发到 Go core `/api/watchlist/create`。
#[tauri::command]
pub async fn watchlist_create(
    state: State<'_, CoreState>,
    payload: WatchlistCreatePayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/watchlist/create", &payload)
        .map_err(|error| error.to_string())
}

/// 更新自选股元数据，固定转发到 Go core `/api/watchlist/update`。
#[tauri::command]
pub async fn watchlist_update(
    state: State<'_, CoreState>,
    payload: WatchlistUpdatePayload,
) -> Result<serde_json::Value, String> {
    validate_watchlist_id(payload.id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/watchlist/update", &payload)
        .map_err(|error| error.to_string())
}

/// 删除自选股，固定转发到 Go core `/api/watchlist/delete`。
#[tauri::command]
pub async fn watchlist_delete(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_watchlist_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/watchlist/delete", &WatchlistDeleteRequest { id })
        .map_err(|error| error.to_string())
}

/// 校验自选股 ID，避免 Rust command 转发无效更新或删除请求。
fn validate_watchlist_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("invalid watchlist id".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证自选股更新和删除 command 在 Rust 边界拒绝非正数 ID。
    fn validate_watchlist_id_rejects_non_positive_values() {
        assert!(validate_watchlist_id(1).is_ok());
        assert_eq!(
            validate_watchlist_id(0).expect_err("zero watchlist id should fail"),
            "invalid watchlist id"
        );
        assert_eq!(
            validate_watchlist_id(-1).expect_err("negative watchlist id should fail"),
            "invalid watchlist id"
        );
    }
}
