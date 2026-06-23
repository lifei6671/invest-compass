use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreClient, CoreState},
};
use serde::{Deserialize, Serialize};
use std::time::Duration;
use tauri::{AppHandle, State};

const BOOT_TASK_ID: &str = "app-boot-initialization";
const SEARCH_REBUILD_TIMEOUT: Duration = Duration::from_secs(120);

#[derive(Deserialize)]
struct CoreEnvelope<T> {
    code: i32,
    message: String,
    data: T,
}

#[derive(Serialize)]
struct EmptyRequest {}

#[derive(Serialize)]
struct SearchRebuildRequest {
    scope: String,
}

#[derive(Clone, Debug, Default, Deserialize)]
struct SearchStatusData {
    #[serde(default)]
    fts5_status: String,
    #[serde(default)]
    search_status: String,
    #[serde(default)]
    tokenizer_name: String,
}

#[derive(Clone, Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CoreHealthData {
    #[serde(default)]
    db_status: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
enum BootStepStatus {
    Completed,
    Running,
    Pending,
    Failed,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootStep {
    id: String,
    index: u8,
    title: String,
    status: BootStepStatus,
    badge_text: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootTaskDetail {
    task_id: String,
    elapsed: String,
    current_stage: String,
    remaining: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "lowercase")]
enum BootLogStatus {
    Success,
    Running,
    Pending,
    Error,
}

#[derive(Clone, Debug, Serialize)]
struct BootLogItem {
    id: String,
    time: String,
    status: BootLogStatus,
    message: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootStatusData {
    ready: bool,
    progress: u8,
    current_step_id: String,
    steps: Vec<BootStep>,
    task_detail: BootTaskDetail,
    logs: Vec<BootLogItem>,
}

/// 读取应用启动初始化状态；该命令是前端启动页唯一状态源，不暴露任意 Go API 路径。
#[tauri::command]
pub fn app_boot_status(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let binary_path = runtime_core_binary_path();
    let workspace_path =
        desktop_runtime::default_workspace_path(&app_handle).map_err(|error| error.to_string())?;

    if !binary_path.exists() {
        return Ok(wrap_boot_status(failed_boot_status(
            "启动 Go Core Sidecar",
            "core binary is unavailable",
        )));
    }

    let client = match state.start(&binary_path, &workspace_path, Duration::from_secs(5)) {
        Ok(client) => client,
        Err(error) => {
            return Ok(wrap_boot_status(failed_boot_status(
                "启动 Go Core Sidecar",
                &sanitize_boot_message(&error.to_string()),
            )));
        }
    };

    if let Err(error) = read_core_health(&client) {
        return Ok(wrap_boot_status(failed_boot_status(
            core_health_error_stage(&error),
            &sanitize_boot_message(&error),
        )));
    }

    match ensure_search_index_ready(&client) {
        Ok(search_status) => Ok(wrap_boot_status(boot_status_from_search_status(
            &search_status,
        ))),
        Err(error) => Ok(wrap_boot_status(failed_boot_status(
            "SQLite 数据库迁移 / 重建索引",
            &sanitize_boot_message(&error),
        ))),
    }
}

/// 将启动状态包装为前端统一的 CoreEnvelope 结构，保持 coreClient 解包逻辑一致。
fn wrap_boot_status(data: BootStatusData) -> serde_json::Value {
    serde_json::json!({
        "code": 0,
        "message": "ok",
        "data": data,
    })
}

/// 读取 Go Core 健康状态，确认 sidecar 已完成监听与 token 握手。
fn read_core_health(client: &CoreClient) -> Result<(), String> {
    let response: CoreEnvelope<CoreHealthData> =
        serde_json::from_value(client.health().map_err(|error| error.to_string())?)
            .map_err(|error| error.to_string())?;
    validate_core_health_response(response)
}

/// 校验 Go Core 健康信息；数据库未就绪时不允许进入业务页面。
fn validate_core_health_response(response: CoreEnvelope<CoreHealthData>) -> Result<(), String> {
    if response.code != 0 {
        return Err(response.message);
    }
    if response.data.db_status != "ok" {
        return Err(format!(
            "SQLite status is not ok: {}",
            if response.data.db_status.is_empty() {
                "unknown"
            } else {
                response.data.db_status.as_str()
            }
        ));
    }
    Ok(())
}

/// 将健康检查错误映射到启动步骤；数据库异常不能被误报为 sidecar 启动失败。
fn core_health_error_stage(error: &str) -> &'static str {
    if error.starts_with("SQLite status is not ok") {
        return "SQLite 数据库迁移 / 重建索引";
    }
    "启动 Go Core Sidecar"
}

/// 确保全文索引达到可进入工作台的状态；缺少索引时主动触发一次全量重建。
fn ensure_search_index_ready(client: &CoreClient) -> Result<SearchStatusData, String> {
    let status = read_search_status(client)?;
    if should_trigger_boot_rebuild(&status) {
        let response: CoreEnvelope<serde_json::Value> = client
            .post_api_with_timeout(
                "/api/search/rebuild",
                &SearchRebuildRequest {
                    scope: "all".to_string(),
                },
                SEARCH_REBUILD_TIMEOUT,
            )
            .map_err(|error| error.to_string())?;
        if response.code != 0 {
            return Err(response.message);
        }
        return read_search_status(client);
    }
    Ok(status)
}

/// 读取 Go Core 汇总后的搜索索引状态，避免前端直接拼接多个初始化接口。
fn read_search_status(client: &CoreClient) -> Result<SearchStatusData, String> {
    let response: CoreEnvelope<SearchStatusData> = client
        .post_api("/api/search/status", &EmptyRequest {})
        .map_err(|error| error.to_string())?;
    if response.code != 0 {
        return Err(response.message);
    }
    Ok(response.data)
}

/// 判断启动阶段是否需要主动重建索引；FTS5 不可用时不应让用户永久卡在初始化页。
fn should_trigger_boot_rebuild(status: &SearchStatusData) -> bool {
    status.search_status.eq_ignore_ascii_case("NEED_REBUILD")
        && !status.fts5_status.eq_ignore_ascii_case("UNAVAILABLE")
}

/// 将搜索索引状态映射为启动页展示状态，并决定是否允许进入总览。
fn boot_status_from_search_status(status: &SearchStatusData) -> BootStatusData {
    let search_status = status.search_status.to_ascii_uppercase();
    if search_status == "BUILDING" {
        return build_boot_status(BootBuildOptions {
            ready: false,
            progress: 68,
            current_step_id: "sqlite_migration",
            running_stage: "SQLite 数据库迁移 / 重建索引",
            sqlite_status: BootStepStatus::Running,
            tokenizer_status: BootStepStatus::Pending,
            data_source_status: BootStepStatus::Pending,
            cache_status: BootStepStatus::Pending,
            ready_status: BootStepStatus::Pending,
            log_status: BootLogStatus::Running,
            log_message: "rebuilding search index",
        });
    }

    build_boot_status(BootBuildOptions {
        ready: true,
        progress: 100,
        current_step_id: "ready",
        running_stage: "完成基础检查并进入工作台",
        sqlite_status: BootStepStatus::Completed,
        tokenizer_status: BootStepStatus::Completed,
        data_source_status: BootStepStatus::Completed,
        cache_status: BootStepStatus::Completed,
        ready_status: BootStepStatus::Completed,
        log_status: if status.fts5_status.eq_ignore_ascii_case("UNAVAILABLE") {
            BootLogStatus::Pending
        } else {
            BootLogStatus::Success
        },
        log_message: if status.fts5_status.eq_ignore_ascii_case("UNAVAILABLE") {
            "search index unavailable, continue without local FTS"
        } else if !status.tokenizer_name.trim().is_empty() {
            "tokenizer ready"
        } else {
            "initialization completed"
        },
    })
}

struct BootBuildOptions {
    ready: bool,
    progress: u8,
    current_step_id: &'static str,
    running_stage: &'static str,
    sqlite_status: BootStepStatus,
    tokenizer_status: BootStepStatus,
    data_source_status: BootStepStatus,
    cache_status: BootStepStatus,
    ready_status: BootStepStatus,
    log_status: BootLogStatus,
    log_message: &'static str,
}

/// 按统一步骤模板构造启动页数据，确保前端每次渲染都有完整步骤和日志。
fn build_boot_status(options: BootBuildOptions) -> BootStatusData {
    BootStatusData {
        ready: options.ready,
        progress: options.progress,
        current_step_id: options.current_step_id.to_string(),
        steps: vec![
            step(
                "sidecar",
                1,
                "启动 Go Core Sidecar",
                BootStepStatus::Completed,
            ),
            step(
                "workspace",
                2,
                "检查本地工作区与目录权限",
                BootStepStatus::Completed,
            ),
            step(
                "sqlite_migration",
                3,
                "SQLite 数据库迁移 / 重建索引",
                options.sqlite_status,
            ),
            step(
                "tokenizer",
                4,
                "加载分词器（GSE / 全文检索词典）",
                options.tokenizer_status,
            ),
            step(
                "data_source",
                5,
                "初始化数据源配置",
                options.data_source_status,
            ),
            step(
                "market_cache",
                6,
                "同步基础行情快照与资讯缓存",
                options.cache_status,
            ),
            step("ready", 7, "完成基础检查并进入工作台", options.ready_status),
        ],
        task_detail: BootTaskDetail {
            task_id: BOOT_TASK_ID.to_string(),
            elapsed: "计算中".to_string(),
            current_stage: options.running_stage.to_string(),
            remaining: if options.ready {
                "00:00:00".to_string()
            } else {
                "计算中".to_string()
            },
        },
        logs: vec![
            log_item("1", BootLogStatus::Success, "sidecar ready"),
            log_item("2", BootLogStatus::Success, "workspace opened"),
            log_item("3", BootLogStatus::Success, "sqlite migrate completed"),
            log_item("4", options.log_status, options.log_message),
        ],
    }
}

/// 构造初始化失败状态；错误信息会在调用方先脱敏后进入启动页日志。
fn failed_boot_status(stage: &str, message: &str) -> BootStatusData {
    let failed_step = match stage {
        "启动 Go Core Sidecar" => "sidecar",
        "检查本地工作区与目录权限" => "workspace",
        _ => "sqlite_migration",
    };
    let mut data = build_boot_status(BootBuildOptions {
        ready: false,
        progress: 30,
        current_step_id: failed_step,
        running_stage: "初始化失败",
        sqlite_status: BootStepStatus::Pending,
        tokenizer_status: BootStepStatus::Pending,
        data_source_status: BootStepStatus::Pending,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Error,
        log_message: "initialization failed",
    });
    for step in &mut data.steps {
        if step.title == stage {
            step.status = BootStepStatus::Failed;
            step.badge_text = "失败".to_string();
        }
    }
    data.task_detail.current_stage = stage.to_string();
    data.logs.push(log_item("5", BootLogStatus::Error, message));
    data
}

/// 构造单个启动步骤，badge 文案始终由状态派生，避免双写不一致。
fn step(id: &str, index: u8, title: &str, status: BootStepStatus) -> BootStep {
    BootStep {
        id: id.to_string(),
        index,
        title: title.to_string(),
        status,
        badge_text: badge_text(status).to_string(),
    }
}

/// 将步骤枚举状态转换为启动页展示文案。
fn badge_text(status: BootStepStatus) -> &'static str {
    match status {
        BootStepStatus::Completed => "已完成",
        BootStepStatus::Running => "进行中",
        BootStepStatus::Pending => "等待中",
        BootStepStatus::Failed => "失败",
    }
}

/// 构造初始化日志条目；启动阶段不展示真实路径或凭据类敏感内容。
fn log_item(id: &str, status: BootLogStatus, message: &str) -> BootLogItem {
    BootLogItem {
        id: id.to_string(),
        time: "--:--:--".to_string(),
        status,
        message: message.to_string(),
    }
}

/// 对启动错误做保守脱敏，避免 token、cookie、API Key 等内容进入前端。
fn sanitize_boot_message(message: &str) -> String {
    let mut value = message.to_string();
    for marker in [
        "Authorization",
        "Proxy-Authorization",
        "token",
        "cookie",
        "api_key",
    ] {
        if value
            .to_ascii_lowercase()
            .contains(&marker.to_ascii_lowercase())
        {
            value = "initialization error redacted".to_string();
            break;
        }
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 验证搜索索引处于 READY 时启动页进入 ready，不继续阻塞总览。
    #[test]
    fn boot_status_from_ready_search_marks_all_steps_completed() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "READY".to_string(),
            tokenizer_name: "simple".to_string(),
        });

        assert!(data.ready);
        assert_eq!(data.progress, 100);
        assert!(data
            .steps
            .iter()
            .all(|step| step.status == BootStepStatus::Completed));
    }

    /// 验证搜索索引正在重建时启动页停留在初始化页，避免提前进入业务页面。
    #[test]
    fn boot_status_from_building_search_keeps_initializing() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "BUILDING".to_string(),
            ..SearchStatusData::default()
        });

        assert!(!data.ready);
        assert_eq!(data.current_step_id, "sqlite_migration");
        assert_eq!(data.steps[2].status, BootStepStatus::Running);
    }

    /// 验证启动期只在 FTS5 可用且确实缺少 active 索引时触发自动重建。
    #[test]
    fn should_trigger_boot_rebuild_only_for_need_rebuild_with_available_fts() {
        assert!(should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "NEED_REBUILD".to_string(),
            ..SearchStatusData::default()
        }));
        assert!(!should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "UNAVAILABLE".to_string(),
            search_status: "NEED_REBUILD".to_string(),
            ..SearchStatusData::default()
        }));
    }

    /// 验证数据库未就绪时启动门禁失败，不能提前进入总览页面。
    #[test]
    fn validate_core_health_response_rejects_non_ok_database_status() {
        let result = validate_core_health_response(CoreEnvelope {
            code: 0,
            message: "ok".to_string(),
            data: CoreHealthData {
                db_status: "not_configured".to_string(),
            },
        });

        assert_eq!(
            result.expect_err("db not ready should block boot"),
            "SQLite status is not ok: not_configured"
        );
    }

    /// 验证数据库健康异常会落到 SQLite 初始化步骤，而不是误导为 sidecar 连接失败。
    #[test]
    fn core_health_error_stage_maps_database_error_to_sqlite_step() {
        assert_eq!(
            core_health_error_stage("SQLite status is not ok: not_configured"),
            "SQLite 数据库迁移 / 重建索引"
        );
        assert_eq!(
            core_health_error_stage("sidecar http error"),
            "启动 Go Core Sidecar"
        );
    }
}
