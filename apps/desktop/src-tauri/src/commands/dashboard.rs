use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

/// 获取 Dashboard 汇总，固定转发到 Go core `/api/dashboard/summary`。
#[tauri::command]
pub async fn dashboard_summary(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/dashboard/summary", EmptyRequest {}).await
}
