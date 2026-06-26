use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreClient, CoreState},
};
use serde::{Deserialize, Serialize};
use std::{
    fs,
    path::{Path, PathBuf},
    process::Command,
    time::{SystemTime, UNIX_EPOCH},
};
use tauri::{AppHandle, State};

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
    #[serde(default)]
    proxy_credential_ref: String,
    #[serde(default)]
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
pub struct ProxyConnectionTestPayload {
    target: String,
}

#[derive(Deserialize, Serialize)]
pub struct WorkspaceSetPayload {
    path: String,
}

#[derive(Deserialize)]
pub struct WorkspaceMigrationPlanPayload {
    target_path: String,
}

#[derive(Deserialize)]
pub struct WorkspaceMigratePayload {
    target_path: String,
    include_cache: bool,
    create_backup: bool,
}

#[derive(Serialize)]
struct WorkspaceMigrationPlanData {
    current_path: String,
    target_path: String,
    can_migrate: bool,
    reason: String,
    warnings: Vec<String>,
    source_size_bytes: u64,
    available_space_bytes: Option<u64>,
    target_exists: bool,
    target_empty: bool,
    will_create_target: bool,
}

#[derive(Serialize)]
struct WorkspaceMigrateData {
    path: String,
    migrated_files: Vec<String>,
    backup_path: Option<String>,
}

#[derive(Serialize)]
pub struct WorkspaceOpenResult {
    opened: bool,
}

#[derive(Serialize)]
struct CacheStatsRequest {}

const WORKSPACE_SQLITE_FILE: &str = "invest-compass.sqlite3";
const WORKSPACE_LOG_DIR: &str = "logs";
const WORKSPACE_CACHE_DIR: &str = "cache";
const WORKSPACE_BACKUP_DIR: &str = "backups";

#[derive(Deserialize, Serialize)]
pub struct CacheCleanPayload {
    targets: Vec<String>,
}

/// 读取非敏感 settings，固定转发到 Go core `/api/settings/get`。
#[tauri::command]
pub fn settings_get(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    keys: Vec<String>,
) -> Result<serde_json::Value, String> {
    post_settings_api(
        &app_handle,
        &state,
        "/api/settings/get",
        &SettingsGetRequest { keys },
    )
}

/// 保存非敏感 settings，固定转发到 Go core `/api/settings/set`。
#[tauri::command]
pub fn settings_set(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: SettingsSetPayload,
) -> Result<serde_json::Value, String> {
    let client = settings_core_client(&app_handle, &state)?;
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

/// 执行代理连通性测试，固定转发到 Go core `/api/proxy/test`。
#[tauri::command]
pub fn proxy_connection_test(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: ProxyConnectionTestPayload,
) -> Result<serde_json::Value, String> {
    validate_proxy_test_target(&payload.target)?;
    post_settings_api(&app_handle, &state, "/api/proxy/test", &payload)
}

/// 读取工作区路径，固定转发到 Go core `/api/workspace/get`。
#[tauri::command]
pub fn workspace_get(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let mut response: serde_json::Value = post_settings_api(
        &app_handle,
        &state,
        "/api/workspace/get",
        &WorkspaceGetRequest {},
    )?;
    if workspace_response_path_is_empty(&response) {
        response["data"]["path"] = serde_json::Value::String(
            desktop_runtime::default_workspace_path(&app_handle)
                .map_err(|error| error.to_string())?
                .to_string_lossy()
                .to_string(),
        );
    }
    Ok(response)
}

/// 保存用户选择的工作区路径，固定转发到 Go core `/api/workspace/set`。
#[tauri::command]
pub fn workspace_set(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: WorkspaceSetPayload,
) -> Result<serde_json::Value, String> {
    validate_workspace_path(&payload.path)?;
    let client = settings_core_client(&app_handle, &state)?;
    client
        .post_api("/api/workspace/set", &payload)
        .map_err(|error| error.to_string())
}

/// 使用系统文件管理器打开当前工作区目录；路径只来自后端配置或平台默认目录。
#[tauri::command]
pub fn workspace_open(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<WorkspaceOpenResult, String> {
    let current_path = current_workspace_path(&app_handle, &state)?;
    ensure_workspace_directory(&current_path)?;
    open_path_in_file_manager(&current_path)?;
    Ok(WorkspaceOpenResult { opened: true })
}

/// 生成工作区迁移预检结果，不修改文件和 settings。
#[tauri::command]
pub fn workspace_migration_plan(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: WorkspaceMigrationPlanPayload,
) -> Result<serde_json::Value, String> {
    let current_path = current_workspace_path(&app_handle, &state)?;
    let plan = build_workspace_migration_plan(&current_path, Path::new(&payload.target_path))?;
    Ok(serde_json::json!({
        "code": 0,
        "message": "ok",
        "data": plan,
    }))
}

/// 执行工作区迁移；迁移前停止 Go core，成功后用目标工作区重启并写入 workspace_path。
#[tauri::command]
pub fn workspace_migrate(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: WorkspaceMigratePayload,
) -> Result<serde_json::Value, String> {
    let current_path = current_workspace_path(&app_handle, &state)?;
    let plan = build_workspace_migration_plan(&current_path, Path::new(&payload.target_path))?;
    if !plan.can_migrate {
        return Err(plan.reason);
    }

    let binary_path = crate::sidecar::runtime_core_binary_path();
    state.stop();
    let result = match migrate_workspace_files(
        &current_path,
        Path::new(&payload.target_path),
        payload.include_cache,
        payload.create_backup,
    ) {
        Ok(result) => result,
        Err(error) => {
            let _ = state.start(
                &binary_path,
                &current_path,
                std::time::Duration::from_secs(5),
            );
            return Err(error);
        }
    };
    let restarted = state
        .start(
            &binary_path,
            Path::new(&payload.target_path),
            std::time::Duration::from_secs(5),
        )
        .map_err(|error| {
            let _ = state.start(
                &binary_path,
                &current_path,
                std::time::Duration::from_secs(5),
            );
            error.to_string()
        })?;
    restarted
        .post_api::<_, serde_json::Value>(
            "/api/workspace/set",
            &WorkspaceSetPayload {
                path: payload.target_path.clone(),
            },
        )
        .map_err(|error| {
            let _ = state.start(
                &binary_path,
                &current_path,
                std::time::Duration::from_secs(5),
            );
            error.to_string()
        })?;

    Ok(serde_json::json!({
        "code": 0,
        "message": "ok",
        "data": WorkspaceMigrateData {
            path: payload.target_path,
            migrated_files: result.migrated_files,
            backup_path: result.backup_path.map(|path| path.to_string_lossy().to_string()),
        },
    }))
}

/// 读取临时缓存统计，固定转发到 Go core `/api/cache/stats`。
#[tauri::command]
pub fn cache_stats(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    post_settings_api(
        &app_handle,
        &state,
        "/api/cache/stats",
        &CacheStatsRequest {},
    )
}

/// 清理临时缓存目标，固定转发到 Go core `/api/cache/clean`。
#[tauri::command]
pub fn cache_clean(
    app_handle: AppHandle,
    state: State<'_, CoreState>,
    payload: CacheCleanPayload,
) -> Result<serde_json::Value, String> {
    let client = settings_core_client(&app_handle, &state)?;
    client
        .post_api("/api/cache/clean", &payload)
        .map_err(|error| error.to_string())
}

/// 获取 settings 相关命令使用的 core client；若子进程已退出，则按当前默认工作区恢复启动。
fn settings_core_client(app_handle: &AppHandle, state: &CoreState) -> Result<CoreClient, String> {
    match state.client() {
        Ok(client) => Ok(client),
        Err(_) => {
            let binary_path = runtime_core_binary_path();
            let workspace_path = desktop_runtime::default_workspace_path(app_handle)
                .map_err(|error| error.to_string())?;
            state
                .start(
                    &binary_path,
                    &workspace_path,
                    std::time::Duration::from_secs(5),
                )
                .map_err(|error| error.to_string())
        }
    }
}

/// 设置页读取类命令统一走可恢复调用，避免启动瞬间旧端口失效导致初始化误报。
fn post_settings_api<TRequest>(
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

/// 校验代理测试目标必须来自页面白名单，避免前端借命令访问任意 URL。
fn validate_proxy_test_target(target: &str) -> Result<(), String> {
    match target.trim() {
        "baidu" | "google" | "openai" | "deepseek" => Ok(()),
        _ => Err("invalid proxy test target".to_string()),
    }
}

/// 从 Go core 读取持久化工作区路径；没有持久化时回退到当前平台默认目录。
fn current_workspace_path(app_handle: &AppHandle, state: &CoreState) -> Result<PathBuf, String> {
    let client = settings_core_client(app_handle, state)?;
    let response: serde_json::Value = client
        .post_api("/api/workspace/get", &WorkspaceGetRequest {})
        .map_err(|error| error.to_string())?;
    let path = response
        .pointer("/data/path")
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|item| !item.is_empty())
        .map(PathBuf::from);
    match path {
        Some(path) => Ok(path),
        None => {
            desktop_runtime::default_workspace_path(app_handle).map_err(|error| error.to_string())
        }
    }
}

/// 确保当前工作区目录可被文件管理器打开；默认工作区首次使用时允许在用户目录下创建。
fn ensure_workspace_directory(path: &Path) -> Result<(), String> {
    if path.exists() {
        if path.is_dir() {
            return Ok(());
        }
        return Err("工作区路径不是目录".to_string());
    }
    fs::create_dir_all(path).map_err(|error| format!("创建工作区目录失败: {error}"))
}

/// 判断 workspace 响应是否缺少路径；缺省时由 Rust 用当前平台默认目录回填。
fn workspace_response_path_is_empty(response: &serde_json::Value) -> bool {
    response.get("code").and_then(serde_json::Value::as_i64) == Some(0)
        && response
            .pointer("/data/path")
            .and_then(serde_json::Value::as_str)
            .map(str::trim)
            .unwrap_or("")
            .is_empty()
}

/// 构造迁移预检结果，只读取文件系统状态，不修改工作区内容。
fn build_workspace_migration_plan(
    current_path: &Path,
    target_path: &Path,
) -> Result<WorkspaceMigrationPlanData, String> {
    validate_workspace_path(current_path.to_string_lossy().as_ref())?;
    validate_workspace_path(target_path.to_string_lossy().as_ref())?;
    let available_space_bytes = read_available_space_bytes(target_path)?;
    build_workspace_migration_plan_with_available_space(
        current_path,
        target_path,
        available_space_bytes,
    )
}

/// 构造工作区迁移预检结果，并用调用方传入的可用空间做容量判断。
fn build_workspace_migration_plan_with_available_space(
    current_path: &Path,
    target_path: &Path,
    available_space_bytes: Option<u64>,
) -> Result<WorkspaceMigrationPlanData, String> {
    validate_workspace_path(current_path.to_string_lossy().as_ref())?;
    validate_workspace_path(target_path.to_string_lossy().as_ref())?;
    let target_exists = target_path.exists();
    let will_create_target = !target_exists;
    let target_empty = if target_exists && target_path.is_dir() {
        target_path
            .read_dir()
            .map_err(|error| format!("read target workspace dir: {error}"))?
            .next()
            .is_none()
    } else {
        true
    };
    let source_size_bytes = workspace_selected_size(current_path, false)
        .map_err(|error| format!("calculate workspace size: {error}"))?;
    let mut plan = WorkspaceMigrationPlanData {
        current_path: current_path.to_string_lossy().to_string(),
        target_path: target_path.to_string_lossy().to_string(),
        can_migrate: true,
        reason: String::new(),
        warnings: Vec::new(),
        source_size_bytes,
        available_space_bytes,
        target_exists,
        target_empty,
        will_create_target,
    };
    if current_path == target_path {
        plan.can_migrate = false;
        plan.reason = "目标工作区与当前工作区相同".to_string();
        return Ok(plan);
    }
    if path_is_inside(target_path, current_path) {
        plan.can_migrate = false;
        plan.reason = "目标目录不能位于当前工作区内部".to_string();
        return Ok(plan);
    }
    if target_exists && !target_path.is_dir() {
        plan.can_migrate = false;
        plan.reason = "目标路径不是目录".to_string();
        return Ok(plan);
    }
    if target_exists && !target_empty {
        plan.can_migrate = false;
        plan.reason = "目标目录非空，默认不会覆盖已有工作区".to_string();
        return Ok(plan);
    }
    if let Some(available_space_bytes) = plan.available_space_bytes {
        if available_space_bytes < plan.source_size_bytes {
            plan.can_migrate = false;
            plan.reason = "目标目录可用空间不足，无法完成工作区迁移".to_string();
            return Ok(plan);
        }
    }
    validate_workspace_target_writable(target_path)
        .map_err(|error| format!("validate target workspace writable: {error}"))?;
    Ok(plan)
}

struct WorkspaceMigrationResult {
    migrated_files: Vec<String>,
    backup_path: Option<PathBuf>,
}

/// 复制工作区必要文件；调用前必须已经停止 Go core，避免复制 SQLite 写入中状态。
fn migrate_workspace_files(
    current_path: &Path,
    target_path: &Path,
    include_cache: bool,
    create_backup: bool,
) -> Result<WorkspaceMigrationResult, String> {
    fs::create_dir_all(target_path).map_err(|error| format!("create target workspace: {error}"))?;
    let backup_path = if create_backup {
        Some(create_workspace_backup(current_path, include_cache)?)
    } else {
        None
    };
    let mut migrated_files = Vec::new();
    for entry in workspace_migration_entries(include_cache) {
        let source = current_path.join(entry);
        if !source.exists() {
            continue;
        }
        let target = target_path.join(entry);
        copy_workspace_entry(&source, &target).map_err(|error| {
            format!("copy workspace entry {}: {error}", source.to_string_lossy())
        })?;
        migrated_files.push(entry.to_string());
    }
    Ok(WorkspaceMigrationResult {
        migrated_files,
        backup_path,
    })
}

/// 创建源工作区备份，便于迁移失败或人工回退。
fn create_workspace_backup(current_path: &Path, include_cache: bool) -> Result<PathBuf, String> {
    let backup_path = latest_workspace_backup_path(current_path);
    fs::create_dir_all(&backup_path)
        .map_err(|error| format!("create migration backup: {error}"))?;
    for entry in workspace_migration_entries(include_cache) {
        let source = current_path.join(entry);
        if !source.exists() {
            continue;
        }
        copy_workspace_entry(&source, &backup_path.join(entry)).map_err(|error| {
            format!(
                "backup workspace entry {}: {error}",
                source.to_string_lossy()
            )
        })?;
    }
    Ok(backup_path)
}

/// 生成本次迁移备份目录，使用秒级时间戳避免覆盖。
fn latest_workspace_backup_path(current_path: &Path) -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default();
    current_path
        .join(WORKSPACE_BACKUP_DIR)
        .join(format!("workspace-migration-{timestamp}"))
}

/// 返回首版迁移范围，默认只迁移 SQLite 及任务日志。
fn workspace_migration_entries(include_cache: bool) -> Vec<&'static str> {
    let mut entries = vec![
        WORKSPACE_SQLITE_FILE,
        "invest-compass.sqlite3-wal",
        "invest-compass.sqlite3-shm",
        WORKSPACE_LOG_DIR,
    ];
    if include_cache {
        entries.push(WORKSPACE_CACHE_DIR);
    }
    entries
}

/// 计算迁移范围内文件体积，用于预检展示。
fn workspace_selected_size(path: &Path, include_cache: bool) -> std::io::Result<u64> {
    let mut size = 0;
    for entry in workspace_migration_entries(include_cache) {
        size += path_size(&path.join(entry))?;
    }
    Ok(size)
}

/// 递归统计路径大小，缺失的可选迁移目录按 0 字节处理。
fn path_size(path: &Path) -> std::io::Result<u64> {
    if !path.exists() {
        return Ok(0);
    }
    let metadata = path.metadata()?;
    if metadata.is_file() {
        return Ok(metadata.len());
    }
    let mut size = 0;
    for entry in path.read_dir()? {
        size += path_size(&entry?.path())?;
    }
    Ok(size)
}

/// 复制单个工作区条目，目录会递归复制，文件会先确保父目录存在。
fn copy_workspace_entry(source: &Path, target: &Path) -> std::io::Result<()> {
    let metadata = source.metadata()?;
    if metadata.is_file() {
        if let Some(parent) = target.parent() {
            fs::create_dir_all(parent)?;
        }
        fs::copy(source, target)?;
        return Ok(());
    }
    fs::create_dir_all(target)?;
    for entry in source.read_dir()? {
        let entry = entry?;
        copy_workspace_entry(&entry.path(), &target.join(entry.file_name()))?;
    }
    Ok(())
}

/// 通过临时文件验证目标目录或其父目录可写。
fn validate_workspace_target_writable(target_path: &Path) -> std::io::Result<()> {
    let probe_dir = if target_path.exists() {
        target_path
    } else {
        target_path.parent().unwrap_or_else(|| Path::new("/"))
    };
    let probe = probe_dir.join(format!(".invest-compass-write-test-{}", std::process::id()));
    fs::OpenOptions::new()
        .create_new(true)
        .write(true)
        .open(&probe)?;
    fs::remove_file(probe)?;
    Ok(())
}

/// 判断候选路径是否位于基准目录内部，用于阻止嵌套迁移目标。
fn path_is_inside(candidate: &Path, base: &Path) -> bool {
    candidate.starts_with(base) && candidate != base
}

/// 使用平台文件管理器打开目录；不接受 renderer 传入路径，避免任意打开文件。
fn open_path_in_file_manager(path: &Path) -> Result<(), String> {
    let status = if cfg!(target_os = "macos") {
        Command::new("open").arg(path).status()
    } else if cfg!(target_os = "windows") {
        Command::new("explorer").arg(path).status()
    } else {
        Command::new("xdg-open").arg(path).status()
    }
    .map_err(|error| format!("open workspace directory failed: {error}"))?;
    if status.success() {
        Ok(())
    } else {
        Err("open workspace directory command failed".to_string())
    }
}

/// 读取目标目录所在磁盘可用空间；失败时返回错误，避免空间状态不明时继续迁移。
fn read_available_space_bytes(path: &Path) -> Result<Option<u64>, String> {
    let probe_path = nearest_existing_path(path);
    if cfg!(target_os = "windows") {
        read_windows_available_space_bytes(&probe_path).map(Some)
    } else {
        read_unix_available_space_bytes(&probe_path).map(Some)
    }
}

/// 从目标路径向上查找最近存在的目录，便于读取所在卷的可用空间。
fn nearest_existing_path(path: &Path) -> PathBuf {
    let mut current = path;
    loop {
        if current.exists() {
            return current.to_path_buf();
        }
        match current.parent() {
            Some(parent) => current = parent,
            None => return PathBuf::from("."),
        }
    }
}

/// 通过 POSIX `df -Pk` 读取 Unix/macOS 目标路径所在卷的可用空间。
fn read_unix_available_space_bytes(path: &Path) -> Result<u64, String> {
    let output = Command::new("df")
        .args(["-Pk"])
        .arg(path)
        .output()
        .map_err(|error| format!("read available workspace space failed: {error}"))?;
    if !output.status.success() {
        return Err("read available workspace space command failed".to_string());
    }
    let text = String::from_utf8(output.stdout)
        .map_err(|error| format!("parse available workspace space output: {error}"))?;
    parse_df_available_space_bytes(&text)
}

#[cfg(target_os = "windows")]
/// 通过 PowerShell 读取 Windows 目标盘符的可用空间。
fn read_windows_available_space_bytes(path: &Path) -> Result<u64, String> {
    let drive = path
        .components()
        .next()
        .map(|component| component.as_os_str().to_string_lossy().to_string())
        .ok_or_else(|| "workspace target drive is invalid".to_string())?;
    let drive_name = drive.trim_end_matches('\\').trim_end_matches(':');
    let script = format!(
        "(Get-PSDrive -Name '{}').Free",
        drive_name.replace('\'', "''")
    );
    let output = Command::new("powershell")
        .args(["-NoProfile", "-Command", &script])
        .output()
        .map_err(|error| format!("read available workspace space failed: {error}"))?;
    if !output.status.success() {
        return Err("read available workspace space command failed".to_string());
    }
    let text = String::from_utf8(output.stdout)
        .map_err(|error| format!("parse available workspace space output: {error}"))?;
    text.trim()
        .parse::<u64>()
        .map_err(|error| format!("parse available workspace space bytes: {error}"))
}

#[cfg(not(target_os = "windows"))]
/// 非 Windows 平台不提供 Windows 可用空间读取实现。
fn read_windows_available_space_bytes(_path: &Path) -> Result<u64, String> {
    Err("windows available space reader is unavailable on this platform".to_string())
}

/// 解析 `df -Pk` 输出中的 Available 列，并转换为字节数。
fn parse_df_available_space_bytes(output: &str) -> Result<u64, String> {
    let line = output
        .lines()
        .skip(1)
        .find(|line| !line.trim().is_empty())
        .ok_or_else(|| "available workspace space output is empty".to_string())?;
    let available_kib = line
        .split_whitespace()
        .nth(3)
        .ok_or_else(|| "available workspace space output missing available column".to_string())?
        .parse::<u64>()
        .map_err(|error| format!("parse available workspace space blocks: {error}"))?;
    available_kib
        .checked_mul(1024)
        .ok_or_else(|| "available workspace space value overflow".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::ai_config::LocalCredentialVault;

    #[test]
    /// 验证迁移预检允许空目标目录，并计算 SQLite 与日志体积。
    fn workspace_migration_plan_allows_empty_target_dir() {
        let root = unique_test_dir("workspace-plan-empty");
        let source = root.join("source");
        let target = root.join("target");
        fs::create_dir_all(source.join("logs")).expect("create source logs");
        fs::create_dir_all(&target).expect("create target");
        fs::write(source.join(WORKSPACE_SQLITE_FILE), b"sqlite").expect("write sqlite");
        fs::write(source.join("logs").join("task.ndjson"), b"log").expect("write log");

        let plan = build_workspace_migration_plan(&source, &target).expect("build plan");

        assert!(
            plan.can_migrate,
            "plan should allow empty target: {}",
            plan.reason
        );
        assert!(plan.target_exists);
        assert!(plan.target_empty);
        assert_eq!(plan.source_size_bytes, 9);
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证迁移预检拒绝非空目标目录，避免覆盖已有工作区。
    fn workspace_migration_plan_rejects_non_empty_target_dir() {
        let root = unique_test_dir("workspace-plan-non-empty");
        let source = root.join("source");
        let target = root.join("target");
        fs::create_dir_all(&source).expect("create source");
        fs::create_dir_all(&target).expect("create target");
        fs::write(target.join(WORKSPACE_SQLITE_FILE), b"existing").expect("write existing");

        let plan = build_workspace_migration_plan(&source, &target).expect("build plan");

        assert!(!plan.can_migrate);
        assert_eq!(plan.reason, "目标目录非空，默认不会覆盖已有工作区");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证迁移预检拒绝把目标目录放到当前工作区内部，避免复制时递归污染源目录。
    fn workspace_migration_plan_rejects_target_inside_current_workspace() {
        let root = unique_test_dir("workspace-plan-inside");
        let source = root.join("source");
        let target = source.join("nested-target");
        fs::create_dir_all(&source).expect("create source");

        let plan = build_workspace_migration_plan(&source, &target).expect("build plan");

        assert!(!plan.can_migrate);
        assert_eq!(plan.reason, "目标目录不能位于当前工作区内部");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证迁移预检会展示目标目录可用空间，并在空间不足时拒绝迁移。
    fn workspace_migration_plan_rejects_when_available_space_is_not_enough() {
        let root = unique_test_dir("workspace-plan-space");
        let source = root.join("source");
        let target = root.join("target");
        fs::create_dir_all(&source).expect("create source");
        fs::create_dir_all(&target).expect("create target");
        fs::write(source.join(WORKSPACE_SQLITE_FILE), b"sqlite").expect("write sqlite");

        let plan = build_workspace_migration_plan_with_available_space(&source, &target, Some(1))
            .expect("build plan");

        assert!(!plan.can_migrate);
        assert_eq!(plan.available_space_bytes, Some(1));
        assert_eq!(plan.reason, "目标目录可用空间不足，无法完成工作区迁移");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证迁移执行只复制必要工作区数据，默认不复制 backups。
    fn migrate_workspace_files_copies_required_entries_without_backups() {
        let root = unique_test_dir("workspace-migrate-copy");
        let source = root.join("source");
        let target = root.join("target");
        fs::create_dir_all(source.join("logs")).expect("create logs");
        fs::create_dir_all(source.join(WORKSPACE_BACKUP_DIR)).expect("create backups");
        fs::write(source.join(WORKSPACE_SQLITE_FILE), b"sqlite").expect("write sqlite");
        fs::write(source.join("logs").join("task.ndjson"), b"log").expect("write log");
        fs::write(source.join(WORKSPACE_BACKUP_DIR).join("old.bak"), b"backup")
            .expect("write backup");

        let result = migrate_workspace_files(&source, &target, false, false).expect("migrate");

        assert!(target.join(WORKSPACE_SQLITE_FILE).exists());
        assert!(target.join("logs").join("task.ndjson").exists());
        assert!(!target.join(WORKSPACE_BACKUP_DIR).exists());
        assert_eq!(result.backup_path, None);
        assert!(result
            .migrated_files
            .contains(&WORKSPACE_SQLITE_FILE.to_string()));
        assert!(result
            .migrated_files
            .contains(&WORKSPACE_LOG_DIR.to_string()));
        let _ = fs::remove_dir_all(root);
    }

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

    #[test]
    /// 验证代理测试命令只接受固定目标标识，不能被用作任意 URL 访问代理。
    fn validate_proxy_test_target_rejects_arbitrary_url() {
        assert!(validate_proxy_test_target("baidu").is_ok());
        assert_eq!(
            validate_proxy_test_target("https://example.com")
                .expect_err("arbitrary URL should fail"),
            "invalid proxy test target"
        );
    }

    #[test]
    /// 验证打开工作区前会创建缺失目录，避免默认用户目录首次使用时按钮报错。
    fn ensure_workspace_directory_creates_missing_dir() {
        let root = unique_test_dir("workspace-open-create");
        let workspace = root.join("Invest Compass");

        ensure_workspace_directory(&workspace).expect("workspace dir should be created");

        assert!(workspace.is_dir());
        let _ = fs::remove_dir_all(root);
    }

    /// 为工作区命令测试生成唯一临时目录，避免并发测试互相污染。
    fn unique_test_dir(name: &str) -> PathBuf {
        std::env::temp_dir().join(format!(
            "invest-compass-{name}-{}-{}",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("system clock before unix epoch")
                .as_nanos()
        ))
    }
}
