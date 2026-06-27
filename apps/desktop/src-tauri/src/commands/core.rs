use crate::{
    commands::blocking::{post_core_api, start_core},
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreState},
};
use std::time::Duration;
use tauri::{AppHandle, State};

/// 启动 Go core sidecar，只能使用 Rust 侧解析出的固定二进制路径。
#[tauri::command]
pub async fn core_start(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let binary_path = runtime_core_binary_path();
    let workspace_path =
        desktop_runtime::default_workspace_path(&app_handle).map_err(|error| error.to_string())?;
    let client = start_core(
        state.inner().clone(),
        binary_path,
        workspace_path,
        Duration::from_secs(5),
    )
    .await?;

    post_core_api(client, "/internal/health", serde_json::json!({})).await
}

/// 通过固定白名单命令获取 Go core 健康信息，前端不能传入任意 URL 或 HTTP method。
#[tauri::command]
pub async fn core_health(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/internal/health", serde_json::json!({})).await
}
