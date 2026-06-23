pub mod commands;
pub mod desktop_runtime;
pub mod sidecar;

use sidecar::CoreState;
use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
/// 启动 Tauri 桌面壳，注册白名单命令并按配置启动 Go sidecar。
pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_notification::init())
        .manage(CoreState::empty())
        .setup(|app| {
            app.handle()
                .plugin(tauri_plugin_autostart::init(
                    tauri_plugin_autostart::MacosLauncher::LaunchAgent,
                    None,
                ))
                .map_err(|error| Box::new(error) as Box<dyn std::error::Error>)?;
            // sidecar 启动由 app_boot_status 在初始化页期间触发，避免 setup 阶段阻塞窗口展示。
            desktop_runtime::install_desktop_runtime(app)?;

            Ok(())
        })
        .on_window_event(desktop_runtime::handle_window_event)
        .invoke_handler(tauri::generate_handler![
            commands::core::core_start,
            commands::core::core_health,
            commands::boot::app_boot_status,
            commands::autostart::autostart_get,
            commands::autostart::autostart_set,
            commands::settings::settings_get,
            commands::settings::settings_set,
            commands::settings::workspace_get,
            commands::settings::workspace_set,
            commands::settings::workspace_open,
            commands::settings::workspace_migration_plan,
            commands::settings::workspace_migrate,
            commands::settings::cache_stats,
            commands::settings::cache_clean,
            commands::market::stock_search,
            commands::market::market_quote,
            commands::market::market_kline,
            commands::market::market_indicators,
            commands::news::news_list,
            commands::news::news_market,
            commands::notifications::notifications_clear_read,
            commands::notifications::notifications_list,
            commands::notifications::notifications_mark_all_read,
            commands::notifications::notifications_mark_read,
            commands::notifications::notifications_unread_count,
            commands::external::open_external_url,
            commands::dashboard::dashboard_summary,
            commands::data_source_credentials::data_source_credentials_clear,
            commands::data_source_credentials::data_source_credentials_list,
            commands::data_source_credentials::data_source_credentials_save,
            commands::data_source_credentials::data_source_credentials_test,
            commands::providers::providers_status,
            commands::prompt::prompt_templates_list,
            commands::prompt::prompt_templates_get,
            commands::prompt::prompt_templates_create,
            commands::prompt::prompt_templates_update,
            commands::prompt::prompt_templates_delete,
            commands::ai_config::ai_config_list,
            commands::ai_config::ai_config_save,
            commands::ai_config::ai_config_test,
            commands::ai_config::ai_config_delete,
            commands::tasks::analysis_task_create,
            commands::tasks::analysis_task_cancel,
            commands::tasks::analysis_task_subscribe,
            commands::tasks::task_list,
            commands::tasks::task_get,
            commands::tasks::task_events,
            commands::task_logs::task_logs_list,
            commands::task_logs::task_log_get,
            commands::task_logs::task_log_summary,
            commands::task_logs::task_log_diagnosis,
            commands::task_logs::task_log_context,
            commands::task_logs::task_logs_export,
            commands::reports::report_list,
            commands::reports::report_get,
            commands::reports::report_delete,
            commands::search::search_reports,
            commands::search::search_news,
            commands::search::search_watchlist_notes,
            commands::search::search_status,
            commands::search::search_rebuild,
            commands::scheduler::scheduler_job_types,
            commands::scheduler::scheduler_jobs_backfill,
            commands::scheduler::scheduler_jobs_list,
            commands::scheduler::scheduler_jobs_get,
            commands::scheduler::scheduler_jobs_run_now,
            commands::scheduler::scheduler_jobs_save,
            commands::scheduler::scheduler_jobs_set_enabled,
            commands::scheduler::scheduler_jobs_delete,
            commands::scheduler::scheduler_runs_list,
            commands::scheduler::scheduler_runs_get,
            commands::scheduler::scheduler_runs_trigger,
            commands::scheduler::scheduler_refresh_symbol,
            commands::scheduler::scheduler_status,
            commands::logs::logs_open_directory,
            commands::logs::export_logs,
            commands::update::check_update,
            commands::watchlist::watchlist_list,
            commands::watchlist::watchlist_create,
            commands::watchlist::watchlist_update,
            commands::watchlist::watchlist_delete
        ])
        .build(tauri::generate_context!())
        .expect("failed to build Invest Compass desktop shell");

    app.run(|app_handle, event| {
        if should_stop_core_for_run_event(&event) {
            app_handle.state::<CoreState>().stop();
        }
    });
}

/// 判断 Tauri 应用级事件是否进入退出路径；`run` 会直接结束进程，必须在事件回调内停止 sidecar。
fn should_stop_core_for_run_event(event: &tauri::RunEvent) -> bool {
    matches!(
        event,
        tauri::RunEvent::Exit | tauri::RunEvent::ExitRequested { .. }
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证应用事件循环退出前会触发 core 清理判断，避免依赖 Drop 这类不会稳定执行的路径。
    fn should_stop_core_for_run_event_accepts_exit() {
        assert!(should_stop_core_for_run_event(&tauri::RunEvent::Exit));
        assert!(!should_stop_core_for_run_event(&tauri::RunEvent::Ready));
    }
}
