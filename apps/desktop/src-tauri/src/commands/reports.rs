use crate::{commands::blocking::post_core_api, sidecar::CoreState};
use serde::{Deserialize, Serialize};
use std::fs::OpenOptions;
use std::io::Write;
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
use std::path::{Path, PathBuf};
use tauri::{AppHandle, State};
use tauri_plugin_dialog::DialogExt;

#[derive(Serialize)]
struct ReportListRequest {}

#[derive(Serialize)]
struct ReportIDRequest {
    id: i64,
}

#[derive(Serialize)]
struct ReportIDsRequest {
    ids: Vec<i64>,
}

#[derive(Serialize)]
struct ReportUpdateRequest {
    id: i64,
    favorite: bool,
}

#[derive(Deserialize)]
struct CoreResponse<T> {
    data: T,
}

#[derive(Deserialize)]
struct ReportExportBundle {
    file_name: String,
    content: String,
}

#[derive(Serialize)]
pub struct ReportExportResult {
    saved: bool,
    file_path: String,
    file_name: String,
}

/// 读取报告历史列表，固定转发到 Go core `/api/reports/list`。
#[tauri::command]
pub async fn report_list(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/reports/list", ReportListRequest {}).await
}

/// 读取报告详情，固定转发到 Go core `/api/reports/get`。
#[tauri::command]
pub async fn report_get(state: State<'_, CoreState>, id: i64) -> Result<serde_json::Value, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/reports/get", ReportIDRequest { id }).await
}

/// 读取报告聚合统计，固定转发到 Go core `/api/reports/stats`。
#[tauri::command]
pub async fn report_stats(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/reports/stats", ReportListRequest {}).await
}

/// 删除报告，固定转发到 Go core `/api/reports/delete`。
#[tauri::command]
pub async fn report_delete(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(client, "/api/reports/delete", ReportIDRequest { id }).await
}

/// 批量删除报告，固定转发到 Go core `/api/reports/batch-delete`。
#[tauri::command]
pub async fn report_batch_delete(
    state: State<'_, CoreState>,
    ids: Vec<i64>,
) -> Result<serde_json::Value, String> {
    validate_report_ids(&ids)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/reports/batch-delete",
        ReportIDsRequest { ids },
    )
    .await
}

/// 更新报告收藏状态，固定转发到 Go core `/api/reports/update`。
#[tauri::command]
pub async fn report_update(
    state: State<'_, CoreState>,
    id: i64,
    favorite: bool,
) -> Result<serde_json::Value, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    post_core_api(
        client,
        "/api/reports/update",
        ReportUpdateRequest { id, favorite },
    )
    .await
}

/// 导出报告 Markdown。保存路径必须由系统保存对话框返回，前端不能传入任意写入路径。
#[tauri::command]
pub async fn report_export(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    id: i64,
) -> Result<ReportExportResult, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    let response: CoreResponse<ReportExportBundle> =
        post_core_api(client, "/api/reports/export", ReportIDRequest { id }).await?;
    let default_file_name = safe_report_export_file_name(&response.data.file_name)?.to_string();
    let Some(file_path) = app_handle
        .dialog()
        .file()
        .set_title("导出报告")
        .set_file_name(&default_file_name)
        .add_filter("Markdown", &["md"])
        .blocking_save_file()
    else {
        return Ok(ReportExportResult {
            saved: false,
            file_path: String::new(),
            file_name: default_file_name,
        });
    };
    let output_path = file_path
        .into_path()
        .map_err(|error| format!("解析导出路径失败: {error}"))?;
    let bundle = response.data;
    tauri::async_runtime::spawn_blocking({
        let output_path = output_path.clone();
        move || write_report_export_bundle(&output_path, &bundle)
    })
    .await
    .map_err(|error| format!("report export task failed: {error}"))??;
    Ok(ReportExportResult {
        saved: true,
        file_path: output_path.to_string_lossy().to_string(),
        file_name: default_file_name,
    })
}

/// 校验报告 ID，避免 Rust command 转发无效报告请求。
fn validate_report_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("invalid report id".to_string());
    }
    Ok(())
}

/// 校验批量报告 ID，避免 Rust command 转发空列表或非法 ID。
fn validate_report_ids(ids: &[i64]) -> Result<(), String> {
    if ids.is_empty() || ids.iter().any(|id| *id <= 0) {
        return Err("invalid report ids".to_string());
    }
    Ok(())
}

/// 将报告导出包写入用户通过保存对话框授权的文件路径。
fn write_report_export_bundle(
    output_path: &Path,
    bundle: &ReportExportBundle,
) -> Result<PathBuf, String> {
    let file_name = safe_report_export_file_name(&bundle.file_name)?;
    if file_name.is_empty() {
        return Err("invalid report export file name".to_string());
    }
    let Some(parent) = output_path.parent() else {
        return Err("invalid report export path".to_string());
    };
    if !parent.is_dir() {
        return Err("report export target directory does not exist".to_string());
    }
    let mut options = OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    options.mode(0o600);
    let mut output_file = options
        .open(output_path)
        .map_err(|error| format!("write report export failed: {error}"))?;
    output_file
        .write_all(bundle.content.as_bytes())
        .map_err(|error| format!("write report export failed: {error}"))?;
    Ok(output_path.to_path_buf())
}

/// 校验 Go core 返回的建议文件名只能是普通文件名，不能包含路径分隔符。
fn safe_report_export_file_name(file_name: &str) -> Result<&str, String> {
    let trimmed = file_name.trim();
    if trimmed.is_empty() || trimmed.contains('/') || trimmed.contains('\\') {
        return Err("invalid report export file name".to_string());
    }
    if Path::new(trimmed)
        .file_name()
        .and_then(|name| name.to_str())
        != Some(trimmed)
    {
        return Err("invalid report export file name".to_string());
    }
    Ok(trimmed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证报告详情和删除 command 在 Rust 边界拒绝非正数 ID。
    fn validate_report_id_rejects_non_positive_values() {
        assert!(validate_report_id(1).is_ok());
        assert_eq!(
            validate_report_id(0).expect_err("zero report id should fail"),
            "invalid report id"
        );
        assert_eq!(
            validate_report_id(-1).expect_err("negative report id should fail"),
            "invalid report id"
        );
    }

    #[test]
    /// 验证批量删除报告 command 在 Rust 边界拒绝空列表和非正数 ID。
    fn validate_report_ids_rejects_empty_or_non_positive_values() {
        assert!(validate_report_ids(&[1, 2]).is_ok());
        assert_eq!(
            validate_report_ids(&[]).expect_err("empty report ids should fail"),
            "invalid report ids"
        );
        assert_eq!(
            validate_report_ids(&[1, 0]).expect_err("zero report id should fail"),
            "invalid report ids"
        );
    }

    #[test]
    /// 验证报告导出写入用户授权路径，并拒绝 Go core 返回的路径穿越文件名。
    fn write_report_export_bundle_writes_selected_file_only() {
        let target_dir = std::env::temp_dir().join(format!(
            "invest-compass-report-export-test-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&target_dir).expect("target dir should be created");
        let output_path = target_dir.join("report.md");
        let bundle = ReportExportBundle {
            file_name: "report.md".to_string(),
            content: "# 报告\n\n仅供研究参考。".to_string(),
        };

        let written_path =
            write_report_export_bundle(&output_path, &bundle).expect("bundle should write");

        assert_eq!(written_path, output_path);
        assert_eq!(
            std::fs::read_to_string(&written_path).expect("exported report should be readable"),
            bundle.content
        );

        let escaped = ReportExportBundle {
            file_name: "../escape.md".to_string(),
            content: "# 报告".to_string(),
        };
        assert!(write_report_export_bundle(&target_dir.join("escape.md"), &escaped).is_err());

        std::fs::remove_dir_all(&target_dir).expect("target dir should be removed");
    }
}
