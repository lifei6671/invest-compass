use serde::Serialize;
use tauri::AppHandle;
use tauri_plugin_autostart::ManagerExt;

#[derive(Serialize)]
pub struct AutostartState {
    enabled: bool,
}

/// 读取系统开机自启动状态，只暴露布尔结果给前端设置页。
#[tauri::command]
pub async fn autostart_get(app: AppHandle) -> Result<AutostartState, String> {
    let enabled = tauri::async_runtime::spawn_blocking(move || {
        app.autolaunch()
            .is_enabled()
            .map_err(|error| error.to_string())
    })
    .await
    .map_err(|error| format!("autostart get task failed: {error}"))??;
    Ok(AutostartState { enabled })
}

/// 设置系统开机自启动状态，前端不能直接调用任意插件 API。
#[tauri::command]
pub async fn autostart_set(app: AppHandle, enabled: bool) -> Result<AutostartState, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let manager = app.autolaunch();
        if enabled {
            manager.enable().map_err(|error| error.to_string())?;
        } else {
            manager.disable().map_err(|error| error.to_string())?;
        }
        Ok::<(), String>(())
    })
    .await
    .map_err(|error| format!("autostart set task failed: {error}"))??;
    Ok(AutostartState { enabled })
}
