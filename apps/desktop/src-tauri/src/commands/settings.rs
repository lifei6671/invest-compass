use crate::sidecar::CoreState;
use serde::{Deserialize, Serialize};
use std::path::Path;
use tauri::State;

use super::ai_config::LocalCredentialVault;

#[derive(Serialize)]
struct SettingsGetRequest {
    keys: Vec<String>,
}

#[derive(Deserialize)]
struct CoreEnvelope<T> {
    code: i32,
    message: String,
    data: T,
}

#[derive(Deserialize)]
struct SettingsGetData {
    items: Vec<SettingItem>,
}

#[derive(Deserialize, Serialize)]
pub struct SettingItem {
    key: String,
    value: String,
}

#[derive(Deserialize, Serialize)]
pub struct SettingsSetPayload {
    items: Vec<SettingItem>,
    proxy_password: Option<String>,
    proxy_credential_ref: String,
    clear_proxy_credential: bool,
}

#[derive(Serialize)]
struct SettingsSetForwardPayload {
    items: Vec<SettingItem>,
}

struct SettingsForwardPlan {
    payload: SettingsSetForwardPayload,
    previous_proxy_credential_ref: Option<String>,
    new_proxy_credential_ref: Option<String>,
}

#[derive(Serialize)]
struct WorkspaceGetRequest {}

#[derive(Deserialize, Serialize)]
pub struct WorkspaceSetPayload {
    path: String,
}

#[derive(Serialize)]
struct CacheStatsRequest {}

#[derive(Deserialize, Serialize)]
pub struct CacheCleanPayload {
    targets: Vec<String>,
}

/// 读取非敏感 settings，固定转发到 Go core `/api/settings/get`。
#[tauri::command]
pub fn settings_get(
    state: State<'_, CoreState>,
    keys: Vec<String>,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/settings/get", &SettingsGetRequest { keys })
        .map_err(|error| error.to_string())
}

/// 保存非敏感 settings，固定转发到 Go core `/api/settings/set`。
#[tauri::command]
pub fn settings_set(
    state: State<'_, CoreState>,
    payload: SettingsSetPayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    let vault = LocalCredentialVault::default();
    let current_proxy_credential_ref = current_proxy_credential_ref(&client)?;
    let plan =
        build_settings_forward_plan(payload, &vault, current_proxy_credential_ref.as_deref())
            .map_err(|error| error.to_string())?;
    let response: Result<serde_json::Value, _> =
        client.post_api("/api/settings/set", &plan.payload);
    match response {
        Ok(value) => {
            cleanup_settings_forward_plan(&plan, &vault).map_err(|error| error.to_string())?;
            Ok(value)
        }
        Err(error) => {
            let _ = rollback_settings_forward_plan(&plan, &vault);
            Err(error.to_string())
        }
    }
}

/// 读取工作区路径，固定转发到 Go core `/api/workspace/get`。
#[tauri::command]
pub fn workspace_get(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/workspace/get", &WorkspaceGetRequest {})
        .map_err(|error| error.to_string())
}

/// 保存用户选择的工作区路径，固定转发到 Go core `/api/workspace/set`。
#[tauri::command]
pub fn workspace_set(
    state: State<'_, CoreState>,
    payload: WorkspaceSetPayload,
) -> Result<serde_json::Value, String> {
    validate_workspace_path(&payload.path)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/workspace/set", &payload)
        .map_err(|error| error.to_string())
}

/// 读取临时缓存统计，固定转发到 Go core `/api/cache/stats`。
#[tauri::command]
pub fn cache_stats(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/cache/stats", &CacheStatsRequest {})
        .map_err(|error| error.to_string())
}

/// 清理临时缓存目标，固定转发到 Go core `/api/cache/clean`。
#[tauri::command]
pub fn cache_clean(
    state: State<'_, CoreState>,
    payload: CacheCleanPayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/cache/clean", &payload)
        .map_err(|error| error.to_string())
}

/// 构造 settings 安全转发计划，代理密码只进本地 vault，清理旧引用必须等 Go core 保存成功。
fn build_settings_forward_plan(
    payload: SettingsSetPayload,
    vault: &LocalCredentialVault,
    current_proxy_credential_ref: Option<&str>,
) -> Result<SettingsForwardPlan, String> {
    let mut items = payload.items;
    reject_credential_metadata_items(&items)?;
    let current_ref = current_proxy_credential_ref
        .map(str::trim)
        .filter(|item| !item.is_empty())
        .unwrap_or("");
    let provided_ref = payload.proxy_credential_ref.trim();
    if !provided_ref.is_empty() {
        vault
            .validate_proxy_reference(provided_ref)
            .map_err(|_| "invalid proxy credential reference".to_string())?;
    }
    if !provided_ref.is_empty() && provided_ref != current_ref {
        return Err("stale proxy credential reference".to_string());
    }
    if !current_ref.is_empty() {
        vault
            .validate_proxy_reference(current_ref)
            .map_err(|_| "invalid proxy credential reference".to_string())?;
    }

    let mut credential_ref = current_ref.to_string();
    let mut previous_proxy_credential_ref = None;
    let mut new_proxy_credential_ref = None;
    let has_proxy_password = payload
        .proxy_password
        .as_deref()
        .map(str::trim)
        .is_some_and(|item| !item.is_empty());

    if payload.clear_proxy_credential && has_proxy_password {
        return Err("conflicting proxy credential action".to_string());
    }

    if payload.clear_proxy_credential {
        if !current_ref.is_empty() {
            previous_proxy_credential_ref = Some(credential_ref.clone());
        }
        upsert_setting(&mut items, "proxy_credential_ref", "");
        return Ok(SettingsForwardPlan {
            payload: SettingsSetForwardPayload { items },
            previous_proxy_credential_ref,
            new_proxy_credential_ref,
        });
    }

    if let Some(secret) = payload
        .proxy_password
        .as_deref()
        .map(str::trim)
        .filter(|_| has_proxy_password)
    {
        let previous_reference = credential_ref.clone();
        let stored = vault
            .save_proxy_password("default", secret)
            .map_err(|error| error.to_string())?;
        let stored_reference = stored.reference.clone();
        credential_ref = stored.reference;
        new_proxy_credential_ref = Some(stored_reference);
        if !previous_reference.trim().is_empty() && previous_reference != credential_ref {
            previous_proxy_credential_ref = Some(previous_reference);
        }
    }

    if !credential_ref.trim().is_empty() {
        upsert_setting(&mut items, "proxy_credential_ref", &credential_ref);
    }
    Ok(SettingsForwardPlan {
        payload: SettingsSetForwardPayload { items },
        previous_proxy_credential_ref,
        new_proxy_credential_ref,
    })
}

/// 从 Go core 当前 settings 读取已绑定的代理凭据引用，避免信任 renderer 传入旧引用。
fn current_proxy_credential_ref(
    client: &crate::sidecar::CoreClient,
) -> Result<Option<String>, String> {
    let envelope: CoreEnvelope<SettingsGetData> = client
        .post_api(
            "/api/settings/get",
            &SettingsGetRequest {
                keys: vec!["proxy_credential_ref".to_string()],
            },
        )
        .map_err(|error| error.to_string())?;
    if envelope.code != 0 {
        return Err(envelope.message);
    }
    Ok(envelope
        .data
        .items
        .into_iter()
        .find(|item| item.key == "proxy_credential_ref")
        .map(|item| item.value)
        .filter(|value| !value.trim().is_empty()))
}

/// 拒绝普通 settings items 携带凭据元数据，真实凭据引用只能由 Rust 专用字段生成。
fn reject_credential_metadata_items(items: &[SettingItem]) -> Result<(), String> {
    for item in items {
        match item.key.trim().to_ascii_lowercase().as_str() {
            "api_key_ref" | "proxy_credential_ref" | "masked_api_key" | "has_api_key" => {
                return Err("credential metadata must use dedicated fields".to_string());
            }
            _ => {}
        }
    }
    Ok(())
}

/// Go core 保存 settings 失败时删除本次新写入的代理 secret，避免孤儿凭据。
fn rollback_settings_forward_plan(
    plan: &SettingsForwardPlan,
    vault: &LocalCredentialVault,
) -> Result<(), String> {
    if let Some(reference) = &plan.new_proxy_credential_ref {
        vault.delete(reference).map_err(|error| error.to_string())?;
    }
    Ok(())
}

/// Go core 保存 settings 成功后再清理被替换或清空的旧代理 secret。
fn cleanup_settings_forward_plan(
    plan: &SettingsForwardPlan,
    vault: &LocalCredentialVault,
) -> Result<(), String> {
    if let Some(reference) = &plan.previous_proxy_credential_ref {
        vault.delete(reference).map_err(|error| error.to_string())?;
    }
    Ok(())
}

/// 更新或追加 settings 项，避免重复 key 导致 Go core 保存顺序依赖调用方。
fn upsert_setting(items: &mut Vec<SettingItem>, key: &str, value: &str) {
    if let Some(item) = items.iter_mut().find(|item| item.key == key) {
        item.value = value.to_string();
        return;
    }
    items.push(SettingItem {
        key: key.to_string(),
        value: value.to_string(),
    });
}

/// 校验工作区路径必须是用户选择后的绝对路径。
fn validate_workspace_path(path: &str) -> Result<(), String> {
    let trimmed = path.trim();
    if trimmed.is_empty() || !Path::new(trimmed).is_absolute() {
        return Err("invalid workspace path".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::ai_config::LocalCredentialVault;

    #[test]
    /// 验证代理密码只写入本地 vault，转发给 Go core 的 settings 只包含凭据引用。
    fn build_settings_forward_plan_stores_proxy_password_in_vault() {
        let root =
            std::env::temp_dir().join(format!("invest-compass-proxy-vault-{}", std::process::id()));
        let vault = LocalCredentialVault::new(root.clone());
        let payload = SettingsSetPayload {
            items: vec![SettingItem {
                key: "proxy.http_url".to_string(),
                value: "http://127.0.0.1:7890".to_string(),
            }],
            proxy_password: Some("proxy-secret-123456".to_string()),
            proxy_credential_ref: String::new(),
            clear_proxy_credential: false,
        };

        let plan =
            build_settings_forward_plan(payload, &vault, None).expect("payload should build");
        let encoded = serde_json::to_string(&plan.payload).expect("payload should serialize");

        assert!(encoded.contains("proxy_credential_ref"));
        assert!(encoded.contains("local-vault://proxy/"));
        assert!(!encoded.contains("proxy_password"));
        assert!(!encoded.contains("proxy-secret-123456"));

        let reference = plan
            .payload
            .items
            .iter()
            .find(|item| item.key == "proxy_credential_ref")
            .expect("proxy credential ref should exist")
            .value
            .clone();
        assert_eq!(
            vault.read(&reference).expect("proxy password should read"),
            "proxy-secret-123456"
        );

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证清理代理凭据时只生成清空引用 payload，等待 Go core 保存成功后才删除本地 vault。
    fn build_settings_forward_plan_defers_proxy_credential_cleanup() {
        let root =
            std::env::temp_dir().join(format!("invest-compass-proxy-clear-{}", std::process::id()));
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_proxy_password("default", "proxy-secret-654321")
            .expect("proxy password should save");
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: None,
            proxy_credential_ref: stored.reference.clone(),
            clear_proxy_credential: true,
        };

        let plan = build_settings_forward_plan(payload, &vault, Some(&stored.reference))
            .expect("payload should build");

        let credential_ref = plan
            .payload
            .items
            .iter()
            .find(|item| item.key == "proxy_credential_ref")
            .expect("proxy credential ref should exist");
        assert_eq!(credential_ref.value, "");
        assert_eq!(
            plan.previous_proxy_credential_ref,
            Some(stored.reference.clone())
        );
        assert_eq!(
            vault
                .read(&stored.reference)
                .expect("old proxy password should remain before cleanup"),
            "proxy-secret-654321"
        );
        cleanup_settings_forward_plan(&plan, &vault).expect("cleanup should remove old secret");
        assert!(vault.read(&stored.reference).is_err());

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证清理代理凭据时即使 renderer 传空旧引用，也会使用 Go core 当前绑定引用。
    fn build_settings_forward_plan_uses_current_proxy_ref_when_payload_ref_is_empty() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-current-ref-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_proxy_password("default", "proxy-secret-current")
            .expect("proxy password should save");
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: None,
            proxy_credential_ref: String::new(),
            clear_proxy_credential: true,
        };

        let plan = build_settings_forward_plan(payload, &vault, Some(&stored.reference))
            .expect("payload should build");

        assert_eq!(
            plan.previous_proxy_credential_ref,
            Some(stored.reference.clone())
        );

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 renderer 传入过期代理凭据引用时会早失败，避免删除错误 vault 文件。
    fn build_settings_forward_plan_rejects_stale_proxy_ref() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-stale-ref-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let current = vault
            .save_proxy_password("default", "proxy-secret-current")
            .expect("current proxy password should save");
        let stale = vault
            .save_proxy_password("default", "proxy-secret-stale")
            .expect("stale proxy password should save");
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: None,
            proxy_credential_ref: stale.reference.clone(),
            clear_proxy_credential: true,
        };

        match build_settings_forward_plan(payload, &vault, Some(&current.reference)) {
            Ok(_) => panic!("stale proxy credential ref should fail"),
            Err(error) => assert_eq!(error, "stale proxy credential reference"),
        }
        assert_eq!(
            vault
                .read(&current.reference)
                .expect("current secret should remain"),
            "proxy-secret-current"
        );
        assert_eq!(
            vault
                .read(&stale.reference)
                .expect("stale secret should remain"),
            "proxy-secret-stale"
        );

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证代理凭据请求不能同时写入新密码和清理旧凭据，避免本地 vault 副作用语义冲突。
    fn build_settings_forward_payload_rejects_conflicting_proxy_credential_actions() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-conflict-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: Some("proxy-secret-123456".to_string()),
            proxy_credential_ref: "local-vault://proxy/default".to_string(),
            clear_proxy_credential: true,
        };

        match build_settings_forward_plan(payload, &vault, Some("local-vault://proxy/default")) {
            Ok(_) => panic!("conflicting proxy credential action should fail"),
            Err(error) => assert_eq!(error, "conflicting proxy credential action"),
        }

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证前端不能通过普通 items 直接写入代理凭据引用，必须走 Rust vault 分支生成。
    fn build_settings_forward_plan_rejects_direct_proxy_credential_ref_item() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-direct-ref-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let payload = SettingsSetPayload {
            items: vec![SettingItem {
                key: "proxy_credential_ref".to_string(),
                value: "local-vault://proxy/default".to_string(),
            }],
            proxy_password: None,
            proxy_credential_ref: String::new(),
            clear_proxy_credential: false,
        };

        match build_settings_forward_plan(payload, &vault, None) {
            Ok(_) => panic!("direct proxy credential ref item should fail"),
            Err(error) => assert_eq!(error, "credential metadata must use dedicated fields"),
        }

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证凭据元数据保留 key 使用大小写变体时也会被 Rust 边界拒绝。
    fn build_settings_forward_plan_rejects_case_variant_credential_metadata_items() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-case-ref-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let payload = SettingsSetPayload {
            items: vec![SettingItem {
                key: "Proxy_Credential_Ref".to_string(),
                value: "local-vault://proxy/default".to_string(),
            }],
            proxy_password: None,
            proxy_credential_ref: String::new(),
            clear_proxy_credential: false,
        };

        match build_settings_forward_plan(payload, &vault, None) {
            Ok(_) => panic!("case variant credential metadata item should fail"),
            Err(error) => assert_eq!(error, "credential metadata must use dedicated fields"),
        }

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证清理代理凭据只允许代理 vault 引用，不能误删 AI Key vault 文件。
    fn build_settings_forward_plan_rejects_ai_key_ref_for_proxy_cleanup() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-ai-ref-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        std::fs::create_dir_all(&root).expect("create vault dir");
        let ai_key_ref = "local-vault://ai-config/openai-compatible-7";
        std::fs::write(
            root.join("openai-compatible-7.secret"),
            "sk-ai-secret-123456",
        )
        .expect("write ai key fixture");
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: None,
            proxy_credential_ref: ai_key_ref.to_string(),
            clear_proxy_credential: true,
        };

        match build_settings_forward_plan(payload, &vault, None) {
            Ok(_) => panic!("ai key ref must not be accepted as proxy credential"),
            Err(error) => assert_eq!(error, "invalid proxy credential reference"),
        }
        assert_eq!(
            vault
                .read(ai_key_ref)
                .expect("ai key should remain readable"),
            "sk-ai-secret-123456"
        );

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 Go core 保存代理设置失败时会回滚本次新写入的 proxy secret。
    fn rollback_settings_forward_plan_removes_new_proxy_secret() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-rollback-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: Some("proxy-secret-new".to_string()),
            proxy_credential_ref: String::new(),
            clear_proxy_credential: false,
        };

        let plan =
            build_settings_forward_plan(payload, &vault, None).expect("payload should build");
        let new_reference = plan
            .new_proxy_credential_ref
            .clone()
            .expect("new proxy credential ref should be tracked");
        assert_eq!(
            vault
                .read(&new_reference)
                .expect("new proxy password should exist"),
            "proxy-secret-new"
        );

        rollback_settings_forward_plan(&plan, &vault).expect("rollback should delete new secret");
        assert!(vault.read(&new_reference).is_err());

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证替换代理密码时使用新 vault 引用，Go core 保存成功前旧 secret 仍保持可用。
    fn build_settings_forward_plan_replaces_proxy_secret_without_overwriting_old_ref() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-proxy-replace-{}",
            std::process::id()
        ));
        let vault = LocalCredentialVault::new(root.clone());
        let previous = vault
            .save_proxy_password("default", "proxy-secret-old")
            .expect("old proxy password should save");
        let payload = SettingsSetPayload {
            items: Vec::new(),
            proxy_password: Some("proxy-secret-new".to_string()),
            proxy_credential_ref: previous.reference.clone(),
            clear_proxy_credential: false,
        };

        let plan = build_settings_forward_plan(payload, &vault, Some(&previous.reference))
            .expect("payload should build");
        let new_reference = plan
            .new_proxy_credential_ref
            .clone()
            .expect("new proxy credential ref should be tracked");

        assert_ne!(new_reference, previous.reference);
        assert_eq!(
            plan.previous_proxy_credential_ref,
            Some(previous.reference.clone())
        );
        assert_eq!(
            vault
                .read(&previous.reference)
                .expect("old proxy password should remain before cleanup"),
            "proxy-secret-old"
        );
        assert_eq!(
            vault
                .read(&new_reference)
                .expect("new proxy password should exist"),
            "proxy-secret-new"
        );

        rollback_settings_forward_plan(&plan, &vault).expect("rollback should remove new secret");
        assert_eq!(
            vault
                .read(&previous.reference)
                .expect("old proxy password should survive rollback"),
            "proxy-secret-old"
        );
        assert!(vault.read(&new_reference).is_err());

        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证工作区设置 command 在 Rust 边界拒绝空路径和相对路径。
    fn validate_workspace_path_rejects_blank_and_relative_path() {
        let absolute = std::env::temp_dir();

        assert!(validate_workspace_path(absolute.to_string_lossy().as_ref()).is_ok());
        assert_eq!(
            validate_workspace_path(" \t\n").expect_err("blank workspace path should fail"),
            "invalid workspace path"
        );
        assert_eq!(
            validate_workspace_path("relative/workspace")
                .expect_err("relative workspace path should fail"),
            "invalid workspace path"
        );
    }
}
