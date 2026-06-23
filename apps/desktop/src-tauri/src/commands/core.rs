use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreState},
};
use std::time::Duration;
use tauri::{AppHandle, State};

/// 启动 Go core sidecar，只能使用 Rust 侧解析出的固定二进制路径。
#[tauri::command]
pub fn core_start(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let binary_path = runtime_core_binary_path();
    let workspace_path =
        desktop_runtime::default_workspace_path(&app_handle).map_err(|error| error.to_string())?;
    let client = state
        .start(&binary_path, &workspace_path, Duration::from_secs(5))
        .map_err(|error| error.to_string())?;

    client.health().map_err(|error| error.to_string())
}

/// 通过固定白名单命令获取 Go core 健康信息，前端不能传入任意 URL 或 HTTP method。
#[tauri::command]
pub fn core_health(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client.health().map_err(|error| error.to_string())
}
