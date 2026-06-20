use crate::sidecar::CoreState;
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
struct ReportListRequest {}

#[derive(Serialize)]
struct ReportIDRequest {
    id: i64,
}

/// 读取报告历史列表，固定转发到 Go core `/api/reports/list`。
#[tauri::command]
pub fn report_list(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/reports/list", &ReportListRequest {})
        .map_err(|error| error.to_string())
}

/// 读取报告详情，固定转发到 Go core `/api/reports/get`。
#[tauri::command]
pub fn report_get(state: State<'_, CoreState>, id: i64) -> Result<serde_json::Value, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/reports/get", &ReportIDRequest { id })
        .map_err(|error| error.to_string())
}

/// 删除报告，固定转发到 Go core `/api/reports/delete`。
#[tauri::command]
pub fn report_delete(state: State<'_, CoreState>, id: i64) -> Result<serde_json::Value, String> {
    validate_report_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/reports/delete", &ReportIDRequest { id })
        .map_err(|error| error.to_string())
}

/// 校验报告 ID，避免 Rust command 转发无效报告请求。
fn validate_report_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("invalid report id".to_string());
    }
    Ok(())
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
}
