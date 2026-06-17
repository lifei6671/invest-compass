pub mod commands;
pub mod sidecar;

use std::time::Duration;

use sidecar::CoreState;
use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
/// 启动 Tauri 桌面壳，注册白名单命令并按配置启动 Go sidecar。
pub fn run() {
    tauri::Builder::default()
        .manage(CoreState::empty())
        .setup(|app| {
            let binary_path = sidecar::runtime_core_binary_path();
            if binary_path.exists() {
                let running = sidecar::start_core_sidecar(&binary_path, Duration::from_secs(5))
                    .map_err(|error| Box::new(error) as Box<dyn std::error::Error>)?;
                app.state::<CoreState>().install(running);
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            commands::core::core_start,
            commands::core::core_health
        ])
        .run(tauri::generate_context!())
        .expect("failed to run Invest Compass desktop shell");
}
