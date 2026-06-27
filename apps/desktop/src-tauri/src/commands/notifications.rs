use crate::{commands::blocking::post_core_api_with_recovery, sidecar::CoreState};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, State};

const NOTIFICATION_MAX_LIMIT: i32 = 50;

#[derive(Deserialize, Serialize)]
pub struct NotificationsListPayload {
    unread_only: bool,
    limit: i32,
    offset: i32,
}

#[derive(Deserialize, Serialize)]
pub struct NotificationsMarkReadPayload {
    ids: Vec<i64>,
}

#[derive(Serialize)]
struct EmptyRequest {}

/// 读取应用内通知列表，固定转发到 Go core `/api/notifications/list`。
#[tauri::command]
pub async fn notifications_list(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: NotificationsListPayload,
) -> Result<serde_json::Value, String> {
    validate_list_payload(&payload)?;
    post_notifications_api(
        app_handle,
        state.inner().clone(),
        "/api/notifications/list",
        payload,
    )
    .await
}

/// 读取应用内未读通知数量，固定转发到 Go core `/api/notifications/unread-count`。
#[tauri::command]
pub async fn notifications_unread_count(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_notifications_api(
        app_handle,
        state.inner().clone(),
        "/api/notifications/unread-count",
        EmptyRequest {},
    )
    .await
}

/// 标记指定应用内通知为已读，固定转发到 Go core `/api/notifications/mark-read`。
#[tauri::command]
pub async fn notifications_mark_read(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: NotificationsMarkReadPayload,
) -> Result<serde_json::Value, String> {
    validate_mark_read_payload(&payload)?;
    post_notifications_api(
        app_handle,
        state.inner().clone(),
        "/api/notifications/mark-read",
        payload,
    )
    .await
}

/// 标记全部应用内通知为已读，固定转发到 Go core `/api/notifications/mark-all-read`。
#[tauri::command]
pub async fn notifications_mark_all_read(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_notifications_api(
        app_handle,
        state.inner().clone(),
        "/api/notifications/mark-all-read",
        EmptyRequest {},
    )
    .await
}

/// 清理已读应用内通知，固定转发到 Go core `/api/notifications/clear-read`。
#[tauri::command]
pub async fn notifications_clear_read(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_notifications_api(
        app_handle,
        state.inner().clone(),
        "/api/notifications/clear-read",
        EmptyRequest {},
    )
    .await
}

/// 通知命令统一通过状态恢复 API 调用 Go core，处理启动瞬间旧端口失效的连接竞态。
async fn post_notifications_api<TRequest>(
    app_handle: AppHandle,
    state: CoreState,
    path: &'static str,
    payload: TRequest,
) -> Result<serde_json::Value, String>
where
    TRequest: Serialize + Send + 'static,
{
    post_core_api_with_recovery(state, app_handle, path, payload).await
}

/// 校验通知分页请求，避免无界列表进入 Go core。
fn validate_list_payload(payload: &NotificationsListPayload) -> Result<(), String> {
    if payload.limit <= 0 || payload.limit > NOTIFICATION_MAX_LIMIT {
        return Err("invalid notification limit".to_string());
    }
    if payload.offset < 0 {
        return Err("invalid notification offset".to_string());
    }
    Ok(())
}

/// 校验标记已读请求，避免空 ID 被误解为全量操作。
fn validate_mark_read_payload(payload: &NotificationsMarkReadPayload) -> Result<(), String> {
    if payload.ids.is_empty() || payload.ids.iter().any(|id| *id <= 0) {
        return Err("invalid notification ids".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证通知列表 Rust 边界拒绝无界分页。
    fn validate_list_payload_rejects_unbounded_pagination() {
        assert!(validate_list_payload(&NotificationsListPayload {
            unread_only: false,
            limit: 20,
            offset: 0,
        })
        .is_ok());
        assert_eq!(
            validate_list_payload(&NotificationsListPayload {
                unread_only: false,
                limit: 0,
                offset: 0,
            })
            .expect_err("zero limit should fail"),
            "invalid notification limit"
        );
        assert_eq!(
            validate_list_payload(&NotificationsListPayload {
                unread_only: false,
                limit: 20,
                offset: -1,
            })
            .expect_err("negative offset should fail"),
            "invalid notification offset"
        );
    }

    #[test]
    /// 验证标记已读 Rust 边界拒绝空 ID 和非法 ID。
    fn validate_mark_read_payload_rejects_empty_or_invalid_ids() {
        assert!(
            validate_mark_read_payload(&NotificationsMarkReadPayload { ids: vec![1, 2] }).is_ok()
        );
        assert_eq!(
            validate_mark_read_payload(&NotificationsMarkReadPayload { ids: vec![] })
                .expect_err("empty ids should fail"),
            "invalid notification ids"
        );
        assert_eq!(
            validate_mark_read_payload(&NotificationsMarkReadPayload { ids: vec![1, 0] })
                .expect_err("zero id should fail"),
            "invalid notification ids"
        );
    }
}
