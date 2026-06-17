use crate::sidecar::CoreState;
use tauri::State;

/// 通过固定白名单命令获取 Go core 健康信息，前端不能传入任意 URL 或 HTTP method。
#[tauri::command]
pub fn core_health(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client.health().map_err(|error| error.to_string())
}
