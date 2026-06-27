use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

/// 获取数据源状态，固定转发到 Go core `/api/providers/status`。
#[tauri::command]
pub async fn providers_status(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/providers/status", EmptyRequest {}).await
}
