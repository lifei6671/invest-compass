use crate::{commands::blocking::post_core_api, desktop_runtime, sidecar::CoreState};
use serde::{Deserialize, Serialize};
use std::fs::OpenOptions;
use std::io::Write;
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
use std::path::{Path, PathBuf};
use std::process::Command;
use tauri::{AppHandle, State};

#[derive(Serialize)]
struct EmptyRequest {}

#[derive(Deserialize)]
struct CoreResponse<T> {
    data: T,
}

#[derive(Deserialize)]
struct LogExportBundle {
    file_name: String,
    content: String,
}

#[derive(Serialize)]
pub struct ExportLogsResult {
    file_path: String,
    file_name: String,
}

#[derive(Serialize)]
pub struct LogsOpenDirectoryResult {
    opened: bool,
}

/// 打开默认工作区下的运行日志目录；初始化失败时不能依赖 Go core 读取工作区设置。
#[tauri::command]
pub async fn logs_open_directory(app_handle: AppHandle) -> Result<LogsOpenDirectoryResult, String> {
    let logs_dir = default_logs_directory(&app_handle)?;
    tauri::async_runtime::spawn_blocking(move || {
        std::fs::create_dir_all(&logs_dir).map_err(|error| format!("创建日志目录失败: {error}"))?;
        open_path_in_file_manager(&logs_dir)
    })
    .await
    .map_err(|error| format!("open logs directory task failed: {error}"))??;
    Ok(LogsOpenDirectoryResult { opened: true })
}

/// 生成已脱敏日志导出包，并写入调用方明确指定的本地目录。
#[tauri::command]
pub async fn export_logs(
    state: State<'_, CoreState>,
    target_dir: String,
) -> Result<ExportLogsResult, String> {
    let target_dir = PathBuf::from(target_dir);
    validate_log_export_target_dir(&target_dir)?;
    let client = state.client().map_err(|error| error.to_string())?;
    let response: CoreResponse<LogExportBundle> =
        post_core_api(client, "/api/logs/export", EmptyRequest {}).await?;
    let bundle = response.data;
    let file_name = bundle.file_name.clone();
    let output_path =
        tauri::async_runtime::spawn_blocking(move || write_log_export_bundle(&target_dir, &bundle))
            .await
            .map_err(|error| format!("log export task failed: {error}"))??;
    Ok(ExportLogsResult {
        file_path: output_path.to_string_lossy().to_string(),
        file_name,
    })
}

/// 将脱敏日志包写入用户授权目录，拒绝由 Go core 返回的路径穿越文件名。
fn write_log_export_bundle(target_dir: &Path, bundle: &LogExportBundle) -> Result<PathBuf, String> {
    validate_log_export_target_dir(target_dir)?;
    let file_name = safe_log_export_file_name(&bundle.file_name)?;
    let output_path = target_dir.join(file_name);
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    options.mode(0o600);
    let mut output_file = options
        .open(&output_path)
        .map_err(|error| format!("write log export failed: {error}"))?;
    output_file
        .write_all(bundle.content.as_bytes())
        .map_err(|error| format!("write log export failed: {error}"))?;
    Ok(output_path)
}

/// 计算默认运行日志目录；只基于平台推荐工作区目录派生，不接受 renderer 输入路径。
fn default_logs_directory(app_handle: &AppHandle) -> Result<PathBuf, String> {
    desktop_runtime::default_workspace_path(app_handle)
        .map(|path| path.join("logs"))
        .map_err(|error| error.to_string())
}

/// 使用平台文件管理器打开固定日志目录。
fn open_path_in_file_manager(path: &Path) -> Result<(), String> {
    let status = if cfg!(target_os = "macos") {
        Command::new("open").arg(path).status()
    } else if cfg!(target_os = "windows") {
        Command::new("explorer").arg(path).status()
    } else {
        Command::new("xdg-open").arg(path).status()
    }
    .map_err(|error| format!("open logs directory failed: {error}"))?;
    if status.success() {
        Ok(())
    } else {
        Err("open logs directory command failed".to_string())
    }
}

/// 校验日志导出目标必须是调用方明确授权的已存在目录。
fn validate_log_export_target_dir(target_dir: &Path) -> Result<(), String> {
    if !target_dir.is_dir() {
        return Err("log export target is not a directory".to_string());
    }
    Ok(())
}

/// 校验日志导出文件名只能是单个普通文件名，不能包含目录分隔符。
fn safe_log_export_file_name(file_name: &str) -> Result<&str, String> {
    let trimmed = file_name.trim();
    if trimmed.is_empty() || trimmed.contains('/') || trimmed.contains('\\') {
        return Err("invalid log export file name".to_string());
    }
    if Path::new(trimmed)
        .file_name()
        .and_then(|name| name.to_str())
        != Some(trimmed)
    {
        return Err("invalid log export file name".to_string());
    }
    Ok(trimmed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证日志导出只会写入调用方指定目录，并拒绝路径穿越文件名。
    fn write_log_export_bundle_writes_only_inside_target_dir() {
        let target_dir = std::env::temp_dir().join(format!(
            "invest-compass-log-export-test-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let bundle = LogExportBundle {
            file_name: "invest-compass-logs-20260618-010203.txt".to_string(),
            content: "request_id=req trace_id=trace task_id=task".to_string(),
        };

        let output_path =
            write_log_export_bundle(&target_dir, &bundle).expect("bundle should write");

        assert!(output_path.starts_with(&target_dir));
        assert_eq!(
            std::fs::read_to_string(&output_path).expect("exported log should be readable"),
            bundle.content
        );

        let escaped = LogExportBundle {
            file_name: "../escape.txt".to_string(),
            content: "request_id=req trace_id=trace task_id=task".to_string(),
        };
        assert!(write_log_export_bundle(&target_dir, &escaped).is_err());

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }

    #[test]
    /// 验证日志导出 command 在请求 Go core 前拒绝无效目标目录。
    fn validate_log_export_target_dir_rejects_invalid_directory() {
        let target_dir = std::env::temp_dir().join(format!(
            "invest-compass-log-export-target-test-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");

        assert!(validate_log_export_target_dir(&target_dir).is_ok());
        assert_eq!(
            validate_log_export_target_dir(Path::new(" \t\n"))
                .expect_err("blank target should fail"),
            "log export target is not a directory"
        );
        assert_eq!(
            validate_log_export_target_dir(&target_dir.join("missing"))
                .expect_err("missing target should fail"),
            "log export target is not a directory"
        );

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }

    #[test]
    /// 验证日志导出不会覆盖用户目录中已有的同名文件。
    fn write_log_export_bundle_rejects_existing_file() {
        let target_dir = std::env::temp_dir().join(format!(
            "invest-compass-log-export-existing-test-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let output_path = target_dir.join("invest-compass-logs-20260618-010203.txt");
        std::fs::write(&output_path, "existing content").expect("existing file should be created");
        let bundle = LogExportBundle {
            file_name: "invest-compass-logs-20260618-010203.txt".to_string(),
            content: "new content".to_string(),
        };

        let error =
            write_log_export_bundle(&target_dir, &bundle).expect_err("existing file should fail");

        assert!(error.contains("write log export failed"));
        assert_eq!(
            std::fs::read_to_string(&output_path).expect("existing file should still be readable"),
            "existing content"
        );

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }

    #[cfg(unix)]
    #[test]
    /// 验证 Unix/macOS 日志导出文件只允许当前用户读写。
    fn write_log_export_bundle_creates_private_unix_file() {
        use std::os::unix::fs::PermissionsExt;

        let target_dir = std::env::temp_dir().join(format!(
            "invest-compass-log-export-mode-test-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let bundle = LogExportBundle {
            file_name: "invest-compass-logs-20260618-010203.txt".to_string(),
            content: "request_id=req trace_id=trace task_id=task".to_string(),
        };

        let output_path =
            write_log_export_bundle(&target_dir, &bundle).expect("bundle should write");
        let mode = std::fs::metadata(&output_path)
            .expect("exported log should have metadata")
            .permissions()
            .mode()
            & 0o777;

        assert_eq!(mode, 0o600);

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }
}
