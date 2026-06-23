use crate::sidecar::{CoreClient, CoreState};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    App, AppHandle, Manager, PhysicalPosition, PhysicalSize, Position, Size, WebviewWindow, Window,
    WindowEvent,
};

const MAIN_WINDOW_LABEL: &str = "main";
const TRAY_OPEN_WORKBENCH_ID: &str = "open-workbench";
const TRAY_QUIT_APP_ID: &str = "quit-app";
const CLOSE_TO_TRAY_SETTING_KEY: &str = "window.close_to_tray";
const WINDOW_X_SETTING_KEY: &str = "window.main.x";
const WINDOW_Y_SETTING_KEY: &str = "window.main.y";
const WINDOW_WIDTH_SETTING_KEY: &str = "window.main.width";
const WINDOW_HEIGHT_SETTING_KEY: &str = "window.main.height";
const MIN_RESTORED_WINDOW_WIDTH: u32 = 640;
const MIN_RESTORED_WINDOW_HEIGHT: u32 = 480;

/// 返回默认工作区路径，优先遵循用户 Documents 目录，便于用户备份和迁移本地数据。
pub fn default_workspace_path<R: tauri::Runtime, M: Manager<R>>(
    manager: &M,
) -> tauri::Result<PathBuf> {
    manager
        .path()
        .document_dir()
        .map(workspace_path_from_documents_dir)
}

/// 基于系统 Documents 目录生成应用默认工作区路径，供启动和测试复用。
fn workspace_path_from_documents_dir(documents_dir: PathBuf) -> PathBuf {
    documents_dir.join("Invest Compass")
}

#[derive(Serialize)]
struct RuntimeSettingsGetRequest {
    keys: Vec<String>,
}

#[derive(Serialize)]
struct RuntimeSettingsSetRequest {
    items: Vec<RuntimeSettingItem>,
}

#[derive(Deserialize)]
struct RuntimeEnvelope<T> {
    code: i32,
    data: T,
}

#[derive(Deserialize)]
struct RuntimeSettingsData {
    items: Vec<RuntimeSettingItem>,
}

#[derive(Clone, Deserialize, Serialize)]
struct RuntimeSettingItem {
    key: String,
    value: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct WindowState {
    x: i32,
    y: i32,
    width: u32,
    height: u32,
}

/// 安装桌面运行期能力，确保关闭隐藏前先存在可恢复主窗口的托盘入口。
pub fn install_desktop_runtime(app: &mut App) -> tauri::Result<()> {
    install_tray_menu(app)?;
    restore_main_window_state(app);
    Ok(())
}

/// 安装首版托盘菜单；菜单只承担恢复主窗口和退出应用，避免引入未闭环入口。
fn install_tray_menu(app: &mut App) -> tauri::Result<()> {
    let open_item = MenuItem::with_id(
        app,
        TRAY_OPEN_WORKBENCH_ID,
        "打开工作台",
        true,
        None::<&str>,
    )?;
    let quit_item = MenuItem::with_id(app, TRAY_QUIT_APP_ID, "退出", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&open_item, &quit_item])?;
    let mut builder = TrayIconBuilder::new()
        .tooltip("投研罗盘")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| handle_tray_menu_event(app, event.id().as_ref()))
        .on_tray_icon_event(|tray, event| {
            if tray_event_should_restore_window(&event) {
                show_main_window(tray.app_handle());
            }
        });

    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }

    builder.build(app)?;
    Ok(())
}

/// 处理主窗口关闭事件，按 settings 决定是隐藏到托盘还是直接退出。
pub fn handle_window_event(window: &Window, event: &WindowEvent) {
    if window.label() != MAIN_WINDOW_LABEL {
        return;
    }

    if let WindowEvent::CloseRequested { api, .. } = event {
        persist_main_window_state(window);
        let hide_to_tray = should_hide_main_window_on_close(window.app_handle());
        if hide_to_tray {
            api.prevent_close();
            let _ = window.hide();
        } else if should_stop_core_on_close(hide_to_tray) {
            stop_core_sidecar(window.app_handle());
        }
    }
}

/// 从 settings 恢复主窗口位置和尺寸；读取失败时保留 Tauri 默认窗口状态。
fn restore_main_window_state(app: &App) {
    let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) else {
        return;
    };
    let state = app.state::<CoreState>();
    let Ok(client) = state.client() else {
        return;
    };
    let Some(window_state) = window_state_from_settings(&client) else {
        return;
    };
    apply_window_state(&window, window_state);
}

/// 保存主窗口位置和尺寸到 Go core settings，供下次启动恢复。
fn persist_main_window_state(window: &Window) {
    let Ok(position) = window.outer_position() else {
        return;
    };
    let Ok(size) = window.outer_size() else {
        return;
    };
    let state = window.app_handle().state::<CoreState>();
    let Ok(client) = state.client() else {
        return;
    };
    let window_state = WindowState {
        x: position.x,
        y: position.y,
        width: size.width,
        height: size.height,
    };
    let _ = save_window_state(&client, window_state);
}

/// 应用已校验的窗口状态；平台拒绝恢复时保留默认窗口，避免启动失败。
fn apply_window_state(window: &WebviewWindow, state: WindowState) {
    let _ = window.set_size(Size::Physical(PhysicalSize::new(state.width, state.height)));
    let _ = window.set_position(Position::Physical(PhysicalPosition::new(state.x, state.y)));
}

/// 处理托盘菜单动作；退出路径交给 Tauri run event 统一停止 sidecar。
fn handle_tray_menu_event(app: &AppHandle, item_id: &str) {
    match item_id {
        TRAY_OPEN_WORKBENCH_ID => show_main_window(app),
        TRAY_QUIT_APP_ID => app.exit(0),
        _ => {}
    }
}

/// 判断托盘点击是否应恢复主窗口；只处理左键释放，避免按下和右键菜单重复触发。
fn tray_event_should_restore_window(event: &TrayIconEvent) -> bool {
    matches!(
        event,
        TrayIconEvent::Click {
            button: MouseButton::Left,
            button_state: MouseButtonState::Up,
            ..
        }
    )
}

/// 显示并聚焦主窗口，用于托盘点击和“打开工作台”菜单项。
fn show_main_window(app: &AppHandle) {
    let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) else {
        return;
    };
    let _ = window.unminimize();
    let _ = window.show();
    let _ = window.set_focus();
}

/// 从 Go core settings 读取主窗口状态。
fn window_state_from_settings(client: &CoreClient) -> Option<WindowState> {
    let request = RuntimeSettingsGetRequest {
        keys: window_state_settings_keys(),
    };
    let Ok(envelope) =
        client.post_api::<_, RuntimeEnvelope<RuntimeSettingsData>>("/api/settings/get", &request)
    else {
        return None;
    };
    if envelope.code != 0 {
        return None;
    }
    parse_window_state_from_settings_items(&envelope.data.items)
}

/// 把主窗口状态写入 settings，所有 key 固定在 Rust 运行期这一处。
fn save_window_state(client: &CoreClient, state: WindowState) -> Result<serde_json::Value, String> {
    let request = RuntimeSettingsSetRequest {
        items: window_state_to_settings_items(state),
    };
    client
        .post_api("/api/settings/set", &request)
        .map_err(|error| error.to_string())
}

/// 返回窗口状态恢复需要读取的固定 settings key。
fn window_state_settings_keys() -> Vec<String> {
    [
        WINDOW_X_SETTING_KEY,
        WINDOW_Y_SETTING_KEY,
        WINDOW_WIDTH_SETTING_KEY,
        WINDOW_HEIGHT_SETTING_KEY,
    ]
    .into_iter()
    .map(str::to_string)
    .collect()
}

/// 按 settings 响应解析主窗口状态，缺字段或尺寸异常时不恢复。
fn parse_window_state_from_settings_items(items: &[RuntimeSettingItem]) -> Option<WindowState> {
    let state = WindowState {
        x: setting_i32(items, WINDOW_X_SETTING_KEY)?,
        y: setting_i32(items, WINDOW_Y_SETTING_KEY)?,
        width: setting_u32(items, WINDOW_WIDTH_SETTING_KEY)?,
        height: setting_u32(items, WINDOW_HEIGHT_SETTING_KEY)?,
    };
    if state.width < MIN_RESTORED_WINDOW_WIDTH || state.height < MIN_RESTORED_WINDOW_HEIGHT {
        return None;
    }
    Some(state)
}

/// 把主窗口状态转换为 settings 写入项。
fn window_state_to_settings_items(state: WindowState) -> Vec<RuntimeSettingItem> {
    vec![
        RuntimeSettingItem {
            key: WINDOW_X_SETTING_KEY.to_string(),
            value: state.x.to_string(),
        },
        RuntimeSettingItem {
            key: WINDOW_Y_SETTING_KEY.to_string(),
            value: state.y.to_string(),
        },
        RuntimeSettingItem {
            key: WINDOW_WIDTH_SETTING_KEY.to_string(),
            value: state.width.to_string(),
        },
        RuntimeSettingItem {
            key: WINDOW_HEIGHT_SETTING_KEY.to_string(),
            value: state.height.to_string(),
        },
    ]
}

/// 读取 i32 settings 值，窗口坐标允许为负以支持副屏布局。
fn setting_i32(items: &[RuntimeSettingItem], key: &str) -> Option<i32> {
    items
        .iter()
        .find(|item| item.key == key)?
        .value
        .trim()
        .parse()
        .ok()
}

/// 读取 u32 settings 值，用于窗口尺寸恢复。
fn setting_u32(items: &[RuntimeSettingItem], key: &str) -> Option<u32> {
    items
        .iter()
        .find(|item| item.key == key)?
        .value
        .trim()
        .parse()
        .ok()
}

/// 从 Go core settings 读取关闭到托盘开关；读取失败时快速失败为直接关闭。
fn should_hide_main_window_on_close(app: &AppHandle) -> bool {
    if !desktop_tray_available() {
        return false;
    }
    let state = app.state::<CoreState>();
    let Ok(client) = state.client() else {
        return false;
    };
    should_hide_window(true, close_to_tray_enabled(&client))
}

/// 从 Go core settings 读取 window.close_to_tray 开关。
fn close_to_tray_enabled(client: &CoreClient) -> bool {
    let request = RuntimeSettingsGetRequest {
        keys: vec![CLOSE_TO_TRAY_SETTING_KEY.to_string()],
    };
    let Ok(envelope) =
        client.post_api::<_, RuntimeEnvelope<RuntimeSettingsData>>("/api/settings/get", &request)
    else {
        return false;
    };
    if envelope.code != 0 {
        return false;
    }
    close_to_tray_enabled_from_settings_items(&envelope.data.items)
}

/// 按 settings 响应项解析关闭到托盘开关。
fn close_to_tray_enabled_from_settings_items(items: &[RuntimeSettingItem]) -> bool {
    items
        .iter()
        .find(|item| item.key == CLOSE_TO_TRAY_SETTING_KEY)
        .map(|item| parse_close_to_tray_value(&item.value))
        .unwrap_or(false)
}

/// 解析关闭到托盘开关，只接受明确的 true，避免误把其他值当成启用。
fn parse_close_to_tray_value(value: &str) -> bool {
    value.trim().eq_ignore_ascii_case("true")
}

/// 判断关闭窗口时是否应隐藏到托盘，必须同时满足托盘可恢复和用户开启设置。
fn should_hide_window(tray_available: bool, setting_enabled: bool) -> bool {
    tray_available && setting_enabled
}

/// 判断关闭主窗口是否会真正退出应用；真正退出前必须同步停止 Go core。
fn should_stop_core_on_close(hide_to_tray: bool) -> bool {
    !hide_to_tray
}

/// 停止 Go core sidecar，避免桌面进程退出后留下孤儿进程。
fn stop_core_sidecar(app: &AppHandle) {
    app.state::<CoreState>().stop();
}

/// 标记当前二进制是否已经安装真实托盘恢复入口。
fn desktop_tray_available() -> bool {
    true
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证默认工作区遵循用户 Documents 目录，避免业务数据默认落到应用安装或缓存目录。
    fn workspace_path_from_documents_dir_uses_invest_compass_folder() {
        let path = workspace_path_from_documents_dir(PathBuf::from("/Users/demo/Documents"));
        assert_eq!(
            path,
            PathBuf::from("/Users/demo/Documents").join("Invest Compass")
        );
    }

    #[test]
    /// 验证关闭到托盘只接受明确开启值，缺省或其他值都保持直接退出。
    fn parse_close_to_tray_value_only_accepts_true() {
        assert!(parse_close_to_tray_value("true"));
        assert!(parse_close_to_tray_value(" TRUE "));
        assert!(!parse_close_to_tray_value(""));
        assert!(!parse_close_to_tray_value("false"));
        assert!(!parse_close_to_tray_value("1"));
    }

    #[test]
    /// 验证 settings 响应只通过 window.close_to_tray 决定关闭行为。
    fn close_to_tray_enabled_from_settings_items_reads_dedicated_key() {
        let data = RuntimeSettingsData {
            items: vec![
                RuntimeSettingItem {
                    key: "theme".to_string(),
                    value: "dark".to_string(),
                },
                RuntimeSettingItem {
                    key: "window.close_to_tray".to_string(),
                    value: "true".to_string(),
                },
            ],
        };

        assert!(close_to_tray_enabled_from_settings_items(&data.items));
    }

    #[test]
    /// 验证没有真实托盘恢复入口时，即使 setting 开启也不能隐藏主窗口。
    fn should_hide_window_requires_tray_availability_and_setting() {
        assert!(should_hide_window(true, true));
        assert!(!should_hide_window(false, true));
        assert!(!should_hide_window(true, false));
    }

    #[test]
    /// 验证主窗口真正关闭时必须停止 Go core，只有隐藏到托盘时才保留 sidecar。
    fn should_stop_core_on_close_when_window_will_exit() {
        assert!(!should_stop_core_on_close(true));
        assert!(should_stop_core_on_close(false));
    }

    #[test]
    /// 验证真实托盘启用后，关闭到托盘设置具备可恢复入口。
    fn desktop_tray_available_after_tray_installation_is_enabled() {
        assert!(desktop_tray_available());
    }

    #[test]
    /// 验证托盘菜单只暴露首版已闭环动作，避免出现未实现页面入口。
    fn tray_menu_ids_use_first_version_actions() {
        assert_eq!(TRAY_OPEN_WORKBENCH_ID, "open-workbench");
        assert_eq!(TRAY_QUIT_APP_ID, "quit-app");
    }

    #[test]
    /// 验证窗口状态恢复只接受完整且尺寸合理的 settings 数据。
    fn parse_window_state_from_settings_items_requires_complete_sane_values() {
        let items = vec![
            RuntimeSettingItem {
                key: "window.main.x".to_string(),
                value: "-120".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.y".to_string(),
                value: "80".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.width".to_string(),
                value: "1280".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.height".to_string(),
                value: "800".to_string(),
            },
        ];

        let state = parse_window_state_from_settings_items(&items)
            .expect("complete state should be restored");
        assert_eq!(state.x, -120);
        assert_eq!(state.y, 80);
        assert_eq!(state.width, 1280);
        assert_eq!(state.height, 800);

        let missing_height = &items[..3];
        assert!(parse_window_state_from_settings_items(missing_height).is_none());

        let tiny_window = vec![
            RuntimeSettingItem {
                key: "window.main.x".to_string(),
                value: "0".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.y".to_string(),
                value: "0".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.width".to_string(),
                value: "320".to_string(),
            },
            RuntimeSettingItem {
                key: "window.main.height".to_string(),
                value: "200".to_string(),
            },
        ];
        assert!(parse_window_state_from_settings_items(&tiny_window).is_none());
    }

    #[test]
    /// 验证窗口状态保存只写入固定 settings key，避免散落多套窗口状态来源。
    fn window_state_to_settings_items_uses_fixed_keys() {
        let items = window_state_to_settings_items(WindowState {
            x: 10,
            y: 20,
            width: 1440,
            height: 900,
        });

        let pairs: Vec<(&str, &str)> = items
            .iter()
            .map(|item| (item.key.as_str(), item.value.as_str()))
            .collect();
        assert_eq!(
            pairs,
            vec![
                ("window.main.x", "10"),
                ("window.main.y", "20"),
                ("window.main.width", "1440"),
                ("window.main.height", "900"),
            ]
        );
    }
}
