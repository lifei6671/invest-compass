use crate::sidecar::CoreState;
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

/// 获取数据源状态，固定转发到 Go core `/api/providers/status`。
#[tauri::command]
pub fn providers_status(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/providers/status", &EmptyRequest {})
        .map_err(|error| error.to_string())
}
