use crate::sidecar::CoreState;
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

/// 检查首版更新提示，固定转发到 Go core `/api/update/check`。
#[tauri::command]
pub fn check_update(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/update/check", &EmptyRequest {})
        .map_err(|error| error.to_string())
}
