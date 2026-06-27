use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::{Deserialize, Serialize};
use std::fs::OpenOptions;
use std::io::Write;
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
use std::path::{Path, PathBuf};
use tauri::State;

const TASK_LOGS_LIST_PATH: &str = "/api/tasks/logs/list";
const TASK_LOG_GET_PATH: &str = "/api/tasks/logs/get";
const TASK_LOG_SUMMARY_PATH: &str = "/api/tasks/logs/summary";
const TASK_LOG_DIAGNOSIS_PATH: &str = "/api/tasks/logs/diagnosis";
const TASK_LOG_CONTEXT_PATH: &str = "/api/tasks/logs/context";
const TASK_LOG_EXPORT_PATH: &str = "/api/tasks/logs/export";

#[derive(Deserialize)]
struct CoreResponse<T> {
    data: T,
}

#[derive(Serialize)]
struct TaskLogsListRequest {
    task_id: String,
    level: String,
    module: String,
    stage: String,
    keyword: String,
    only_error: bool,
    after_id: i64,
    limit: i32,
}

#[derive(Serialize)]
struct TaskLogGetRequest {
    id: i64,
}

#[derive(Serialize)]
struct TaskLogTaskIDRequest {
    task_id: String,
}

#[derive(Deserialize)]
struct TaskLogExportBundle {
    file_name: String,
    content: String,
}

#[derive(Serialize)]
pub struct TaskLogsExportResult {
    file_path: String,
    file_name: String,
}

/// 读取任务结构化日志列表，固定转发到 Go core `/api/tasks/logs/list`。
#[tauri::command]
pub async fn task_logs_list(
    state: State<'_, CoreState>,
    task_id: String,
    level: Option<String>,
    module: Option<String>,
    stage: Option<String>,
    keyword: Option<String>,
    only_error: Option<bool>,
    after_id: Option<i64>,
    limit: Option<i32>,
) -> Result<serde_json::Value, String> {
    validate_task_id(&task_id)?;
    let request = TaskLogsListRequest {
        task_id,
        level: level.unwrap_or_default(),
        module: module.unwrap_or_default(),
        stage: stage.unwrap_or_default(),
        keyword: keyword.unwrap_or_default(),
        only_error: only_error.unwrap_or(false),
        after_id: after_id.unwrap_or(0),
        limit: limit.unwrap_or(200),
    };
    if request.after_id < 0 {
        return Err("invalid task log after id".to_string());
    }
    if request.limit <= 0 || request.limit > 500 {
        return Err("invalid task log limit".to_string());
    }
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, TASK_LOGS_LIST_PATH, request).await
}

/// 读取单条任务结构化日志详情，固定转发到 Go core `/api/tasks/logs/get`。
#[tauri::command]
pub async fn task_log_get(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_task_log_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, TASK_LOG_GET_PATH, TaskLogGetRequest { id }).await
}

/// 读取日志抽屉基础摘要，固定转发到 Go core `/api/tasks/logs/summary`。
#[tauri::command]
pub async fn task_log_summary(
    state: State<'_, CoreState>,
    task_id: String,
) -> Result<serde_json::Value, String> {
    post_task_log_task_id(state, TASK_LOG_SUMMARY_PATH, task_id).await
}

/// 读取失败诊断，固定转发到 Go core `/api/tasks/logs/diagnosis`。
#[tauri::command]
pub async fn task_log_diagnosis(
    state: State<'_, CoreState>,
    task_id: String,
) -> Result<serde_json::Value, String> {
    post_task_log_task_id(state, TASK_LOG_DIAGNOSIS_PATH, task_id).await
}

/// 读取安全上下文摘要，固定转发到 Go core `/api/tasks/logs/context`。
#[tauri::command]
pub async fn task_log_context(
    state: State<'_, CoreState>,
    task_id: String,
) -> Result<serde_json::Value, String> {
    post_task_log_task_id(state, TASK_LOG_CONTEXT_PATH, task_id).await
}

/// 导出单个任务的脱敏日志包，并写入调用方明确选择的本地目录。
#[tauri::command]
pub async fn task_logs_export(
    state: State<'_, CoreState>,
    task_id: String,
    target_dir: String,
) -> Result<TaskLogsExportResult, String> {
    validate_task_id(&task_id)?;
    let target_dir = PathBuf::from(target_dir);
    validate_task_log_export_target_dir(&target_dir)?;
    let client = state.client().map_err(|error| error.to_string())?;
    let response: CoreResponse<TaskLogExportBundle> = post_core_api(
        client,
        TASK_LOG_EXPORT_PATH,
        TaskLogTaskIDRequest { task_id },
    )
    .await?;
    let bundle = response.data;
    let file_name = bundle.file_name.clone();
    let output_path = tauri::async_runtime::spawn_blocking(move || {
        write_task_log_export_bundle(&target_dir, &bundle)
    })
    .await
    .map_err(|error| format!("task log export task failed: {error}"))??;
    Ok(TaskLogsExportResult {
        file_path: output_path.to_string_lossy().to_string(),
        file_name,
    })
}

// post_task_log_task_id 将任务 ID 类查询固定转发到指定 Go API 路径。
async fn post_task_log_task_id(
    state: State<'_, CoreState>,
    path: &'static str,
    task_id: String,
) -> Result<serde_json::Value, String> {
    validate_task_id(&task_id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, path, TaskLogTaskIDRequest { task_id }).await
}

// validate_task_id 校验任务 ID 不能为空。
fn validate_task_id(task_id: &str) -> Result<(), String> {
    if task_id.trim().is_empty() {
        return Err("invalid task id".to_string());
    }
    Ok(())
}

// validate_task_log_id 校验任务日志 ID 必须为正数。
fn validate_task_log_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("invalid task log id".to_string());
    }
    Ok(())
}

// write_task_log_export_bundle 将脱敏日志导出包写入用户选择目录。
fn write_task_log_export_bundle(
    target_dir: &Path,
    bundle: &TaskLogExportBundle,
) -> Result<PathBuf, String> {
    validate_task_log_export_target_dir(target_dir)?;
    let file_name = safe_task_log_export_file_name(&bundle.file_name)?;
    let output_path = target_dir.join(file_name);
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    options.mode(0o600);
    let mut output_file = options
        .open(&output_path)
        .map_err(|error| format!("write task log export failed: {error}"))?;
    output_file
        .write_all(bundle.content.as_bytes())
        .map_err(|error| format!("write task log export failed: {error}"))?;
    Ok(output_path)
}

// validate_task_log_export_target_dir 校验导出目标必须是已存在目录。
fn validate_task_log_export_target_dir(target_dir: &Path) -> Result<(), String> {
    if target_dir.as_os_str().is_empty() {
        return Err("task log export target is not a directory".to_string());
    }
    if !target_dir.is_dir() {
        return Err("task log export target is not a directory".to_string());
    }
    Ok(())
}

// safe_task_log_export_file_name 校验导出文件名不能包含路径穿越片段。
fn safe_task_log_export_file_name(file_name: &str) -> Result<&str, String> {
    let trimmed = file_name.trim();
    if trimmed.is_empty() || trimmed.contains('/') || trimmed.contains('\\') {
        return Err("invalid task log export file name".to_string());
    }
    if Path::new(trimmed)
        .file_name()
        .and_then(|name| name.to_str())
        != Some(trimmed)
    {
        return Err("invalid task log export file name".to_string());
    }
    Ok(trimmed)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    // unique_test_dir 创建当前测试独占的临时目录路径。
    fn unique_test_dir(label: &str) -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system clock should be after unix epoch")
            .as_nanos();
        std::env::temp_dir().join(format!(
            "invest-compass-task-log-{label}-{}-{nanos}",
            std::process::id()
        ))
    }

    #[test]
    /// 验证任务日志列表 command 固定转发到白名单路径，不暴露任意路径代理。
    fn task_logs_list_fixed_path() {
        assert_eq!(TASK_LOGS_LIST_PATH, "/api/tasks/logs/list");
    }

    #[test]
    /// 验证任务日志 command 在 Rust 边界拒绝空任务 ID。
    fn validate_task_id_rejects_blank_values() {
        assert!(validate_task_id("task_1").is_ok());
        assert_eq!(
            validate_task_id(" \t\n").expect_err("blank task id should fail"),
            "invalid task id"
        );
    }

    #[test]
    /// 验证上下文摘要 command 复用任务 ID 校验，空 task_id 不会进入 Go core。
    fn task_log_context_rejects_empty_task_id() {
        assert_eq!(
            validate_task_id("").expect_err("empty task id should fail"),
            "invalid task id"
        );
    }

    #[test]
    /// 验证单条日志详情 command 在 Rust 边界拒绝非正数日志 ID。
    fn task_log_get_rejects_non_positive_id() {
        assert!(validate_task_log_id(1).is_ok());
        assert_eq!(
            validate_task_log_id(0).expect_err("zero log id should fail"),
            "invalid task log id"
        );
        assert_eq!(
            validate_task_log_id(-1).expect_err("negative log id should fail"),
            "invalid task log id"
        );
    }

    #[test]
    /// 验证任务日志导出只允许写入调用方授权目录，且拒绝路径穿越文件名。
    fn task_logs_export_rejects_path_traversal_filename() {
        let target_dir = unique_test_dir("export");
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let bundle = TaskLogExportBundle {
            file_name: "invest-compass-task-log-task-1-20260618-010203.txt".to_string(),
            content: "request_id=req trace_id=trace task_id=task_1".to_string(),
        };

        let output_path =
            write_task_log_export_bundle(&target_dir, &bundle).expect("bundle should write");

        assert!(output_path.starts_with(&target_dir));
        assert_eq!(
            std::fs::read_to_string(&output_path).expect("exported log should be readable"),
            bundle.content
        );

        let escaped = TaskLogExportBundle {
            file_name: "../escape.txt".to_string(),
            content: "request_id=req trace_id=trace task_id=task_1".to_string(),
        };
        assert!(write_task_log_export_bundle(&target_dir, &escaped).is_err());

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }

    #[test]
    /// 验证任务日志导出不会覆盖用户目录里已有的同名文件。
    fn task_logs_export_does_not_overwrite_existing_file() {
        let target_dir = unique_test_dir("existing");
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let output_path = target_dir.join("invest-compass-task-log-existing.txt");
        std::fs::write(&output_path, "existing content").expect("existing file should be created");
        let bundle = TaskLogExportBundle {
            file_name: "invest-compass-task-log-existing.txt".to_string(),
            content: "new content".to_string(),
        };

        let error = write_task_log_export_bundle(&target_dir, &bundle)
            .expect_err("existing file should fail");

        assert!(error.contains("write task log export failed"));
        assert_eq!(
            std::fs::read_to_string(&output_path).expect("existing file should still be readable"),
            "existing content"
        );

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }

    #[test]
    /// 验证任务日志导出拒绝空路径。
    fn task_logs_export_rejects_empty_target_dir() {
        assert_eq!(
            validate_task_log_export_target_dir(Path::new(""))
                .expect_err("empty export target should fail"),
            "task log export target is not a directory"
        );
    }

    #[test]
    /// 验证任务日志导出拒绝不存在目录。
    fn task_logs_export_rejects_missing_target_dir() {
        let missing_dir = unique_test_dir("missing");
        assert_eq!(
            validate_task_log_export_target_dir(&missing_dir)
                .expect_err("missing export target should fail"),
            "task log export target is not a directory"
        );
    }

    #[cfg(unix)]
    #[test]
    /// 验证 Unix 下脱敏日志导出文件使用私有权限，避免其他本机用户读取。
    fn task_logs_export_creates_private_file_on_unix() {
        use std::os::unix::fs::PermissionsExt;

        let target_dir = unique_test_dir("private-mode");
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let bundle = TaskLogExportBundle {
            file_name: "invest-compass-task-log-private.txt".to_string(),
            content: "redacted task log".to_string(),
        };

        let output_path =
            write_task_log_export_bundle(&target_dir, &bundle).expect("bundle should write");

        let mode = std::fs::metadata(&output_path)
            .expect("exported log metadata should be readable")
            .permissions()
            .mode()
            & 0o777;
        assert_eq!(mode, 0o600);

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }
}
