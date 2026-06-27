use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreClient, CoreState},
};
use serde::{de::DeserializeOwned, Serialize};
use std::{path::PathBuf, time::Duration};
use tauri::AppHandle;

/// 在线程池执行普通 Core API 请求，避免 blocking HTTP 占住 Tauri IPC 线程。
pub async fn post_core_api<TRequest, TResponse>(
    client: CoreClient,
    path: &'static str,
    payload: TRequest,
) -> Result<TResponse, String>
where
    TRequest: Serialize + Send + 'static,
    TResponse: DeserializeOwned + Send + 'static,
{
    tauri::async_runtime::spawn_blocking(move || {
        client
            .post_api(path, &payload)
            .map_err(|error| error.to_string())
    })
    .await
    .map_err(|error| format!("core request task failed: {error}"))?
}

/// 在线程池执行带恢复逻辑的 Core API 请求，慢启动或旧端口失效都不阻塞前端。
pub async fn post_core_api_with_recovery<TRequest, TResponse>(
    state: CoreState,
    app_handle: AppHandle,
    path: &'static str,
    payload: TRequest,
) -> Result<TResponse, String>
where
    TRequest: Serialize + Send + 'static,
    TResponse: DeserializeOwned + Send + 'static,
{
    let binary_path = runtime_core_binary_path();
    let workspace_path =
        desktop_runtime::default_workspace_path(&app_handle).map_err(|error| error.to_string())?;
    post_core_api_with_recovery_paths(
        state,
        binary_path,
        workspace_path,
        Duration::from_secs(5),
        path,
        payload,
    )
    .await
}

/// 在线程池执行带显式路径的 Core API 恢复请求，供工作区切换等流程复用。
pub async fn post_core_api_with_recovery_paths<TRequest, TResponse>(
    state: CoreState,
    binary_path: PathBuf,
    workspace_path: PathBuf,
    ready_timeout: Duration,
    path: &'static str,
    payload: TRequest,
) -> Result<TResponse, String>
where
    TRequest: Serialize + Send + 'static,
    TResponse: DeserializeOwned + Send + 'static,
{
    tauri::async_runtime::spawn_blocking(move || {
        state
            .post_api_with_recovery(&binary_path, &workspace_path, ready_timeout, path, &payload)
            .map_err(|error| error.to_string())
    })
    .await
    .map_err(|error| format!("core recovery request task failed: {error}"))?
}

/// 在线程池启动或恢复 Core，避免首个 command 阻塞 Tauri 主事件循环。
pub async fn start_core(
    state: CoreState,
    binary_path: PathBuf,
    workspace_path: PathBuf,
    ready_timeout: Duration,
) -> Result<CoreClient, String> {
    tauri::async_runtime::spawn_blocking(move || {
        state
            .start(&binary_path, &workspace_path, ready_timeout)
            .map_err(|error| error.to_string())
    })
    .await
    .map_err(|error| format!("core start task failed: {error}"))?
}
