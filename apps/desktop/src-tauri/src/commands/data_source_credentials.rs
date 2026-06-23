use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreState},
};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, State};

#[derive(Serialize)]
struct DataSourceCredentialListRequest {}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DataSourceCredentialConfigPayload {
    provider_id: String,
    provider_name: String,
    capability: String,
    auth_type: String,
    base_url: String,
    credential_status: String,
    expires_at: String,
    timeout_seconds: i32,
    rate_limit_per_minute: i32,
    masked_credential: String,
    note: String,
}

#[derive(Deserialize, Serialize)]
pub struct DataSourceCredentialSavePayload {
    config: DataSourceCredentialConfigPayload,
    credential: String,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DataSourceCredentialProviderPayload {
    provider_id: String,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DataSourceCredentialTestPayload {
    provider_id: String,
    target: String,
}

/// 读取数据源凭据页数据，固定转发到 Go core `/api/data-source/credentials/list`。
#[tauri::command]
pub fn data_source_credentials_list(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_data_source_credential_api(
        &app_handle,
        &state,
        "/api/data-source/credentials/list",
        &DataSourceCredentialListRequest {},
    )
}

/// 保存数据源凭据配置，固定转发到 Go core `/api/data-source/credentials/save`。
#[tauri::command]
pub fn data_source_credentials_save(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: DataSourceCredentialSavePayload,
) -> Result<serde_json::Value, String> {
    validate_save_payload(&payload)?;
    post_data_source_credential_api(
        &app_handle,
        &state,
        "/api/data-source/credentials/save",
        &payload,
    )
}

/// 清除数据源凭据，固定转发到 Go core `/api/data-source/credentials/clear`。
#[tauri::command]
pub fn data_source_credentials_clear(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: DataSourceCredentialProviderPayload,
) -> Result<serde_json::Value, String> {
    validate_provider_id(&payload.provider_id)?;
    post_data_source_credential_api(
        &app_handle,
        &state,
        "/api/data-source/credentials/clear",
        &payload,
    )
}

/// 执行数据源凭据本地预检，固定转发到 Go core `/api/data-source/credentials/test`。
#[tauri::command]
pub fn data_source_credentials_test(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: DataSourceCredentialTestPayload,
) -> Result<serde_json::Value, String> {
    validate_provider_id(&payload.provider_id)?;
    if payload.target.trim().is_empty() {
        return Err("invalid test target".to_string());
    }
    post_data_source_credential_api(
        &app_handle,
        &state,
        "/api/data-source/credentials/test",
        &payload,
    )
}

/// 数据源凭据命令统一通过恢复式调用 Go core，避免启动瞬间旧端口失效。
fn post_data_source_credential_api<TRequest>(
    app_handle: &AppHandle,
    state: &CoreState,
    path: &str,
    payload: &TRequest,
) -> Result<serde_json::Value, String>
where
    TRequest: Serialize,
{
    let binary_path = runtime_core_binary_path();
    let workspace_path =
        desktop_runtime::default_workspace_path(app_handle).map_err(|error| error.to_string())?;
    state
        .post_api_with_recovery(
            &binary_path,
            &workspace_path,
            std::time::Duration::from_secs(5),
            path,
            payload,
        )
        .map_err(|error| error.to_string())
}

/// 校验保存 payload，明文凭据可以为空但基础配置不能缺失。
fn validate_save_payload(payload: &DataSourceCredentialSavePayload) -> Result<(), String> {
    validate_provider_id(&payload.config.provider_id)?;
    if payload.config.provider_name.trim().is_empty()
        || payload.config.auth_type.trim().is_empty()
        || payload.config.base_url.trim().is_empty()
    {
        return Err("invalid data source credential config".to_string());
    }
    if payload.config.timeout_seconds <= 0 || payload.config.rate_limit_per_minute <= 0 {
        return Err("invalid data source credential limits".to_string());
    }
    Ok(())
}

/// 校验 Provider ID，避免空字符串进入 Go core。
fn validate_provider_id(provider_id: &str) -> Result<(), String> {
    if provider_id.trim().is_empty() {
        return Err("invalid data source provider".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证数据源凭据保存命令拒绝缺失关键字段。
    fn validate_save_payload_rejects_missing_required_fields() {
        let payload = DataSourceCredentialSavePayload {
            config: DataSourceCredentialConfigPayload {
                provider_id: "cls".to_string(),
                provider_name: "".to_string(),
                capability: "快讯 / 日历".to_string(),
                auth_type: "cookie".to_string(),
                base_url: "https://www.cls.cn".to_string(),
                credential_status: "normal".to_string(),
                expires_at: "".to_string(),
                timeout_seconds: 15,
                rate_limit_per_minute: 30,
                masked_credential: "".to_string(),
                note: "".to_string(),
            },
            credential: "uid=secret".to_string(),
        };
        assert_eq!(
            validate_save_payload(&payload).expect_err("missing provider name should fail"),
            "invalid data source credential config"
        );
    }

    #[test]
    /// 验证数据源凭据保存命令允许空明文凭据，用于只保存非敏感配置。
    fn validate_save_payload_allows_empty_secret() {
        let payload = DataSourceCredentialSavePayload {
            config: DataSourceCredentialConfigPayload {
                provider_id: "eastmoney".to_string(),
                provider_name: "EastMoney".to_string(),
                capability: "行情 / K线".to_string(),
                auth_type: "none".to_string(),
                base_url: "https://quote.eastmoney.com".to_string(),
                credential_status: "normal".to_string(),
                expires_at: "".to_string(),
                timeout_seconds: 15,
                rate_limit_per_minute: 60,
                masked_credential: "无需凭据".to_string(),
                note: "".to_string(),
            },
            credential: "".to_string(),
        };
        assert!(validate_save_payload(&payload).is_ok());
    }
}
