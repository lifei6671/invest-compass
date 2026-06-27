use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

/// 检查首版更新提示，固定转发到 Go core `/api/update/check`。
#[tauri::command]
pub async fn check_update(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/update/check", EmptyRequest {}).await
}
