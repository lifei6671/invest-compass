use crate::sidecar::{CoreClient, CoreState};
use serde::{Deserialize, Serialize};
#[cfg(unix)]
use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
use std::{
    fmt, fs,
    io::Write,
    path::{Path, PathBuf},
    time::Duration,
};
use tauri::State;

const CREDENTIAL_DIR_ENV: &str = "INVEST_COMPASS_CREDENTIAL_DIR";
const LOCAL_VAULT_REF_PREFIX: &str = "local-vault://ai-config/";
const LOCAL_PROXY_VAULT_REF_PREFIX: &str = "local-vault://proxy/";
const AI_CONFIG_TEST_TIMEOUT_BUFFER_SECONDS: u64 = 5;

#[derive(Serialize)]
struct AIConfigListRequest {}

#[derive(Deserialize)]
struct CoreEnvelope<T> {
    code: i32,
    message: String,
    data: T,
}

#[derive(Deserialize)]
struct AIConfigListData {
    items: Vec<AIConfigListItem>,
}

#[derive(Clone, Deserialize)]
struct AIConfigListItem {
    id: i64,
    api_key_ref: String,
    masked_api_key: String,
    has_api_key: bool,
    timeout_seconds: i32,
}

#[derive(Clone)]
struct AIConfigCredentialMetadata {
    api_key_ref: String,
    masked_api_key: String,
    has_api_key: bool,
    timeout_seconds: i32,
}

#[derive(Serialize)]
struct AIConfigDeleteRequest {
    id: i64,
}

#[derive(Serialize)]
struct AIConfigTestRequest {
    id: i64,
    resolved_api_key: String,
}

#[derive(Deserialize, Serialize)]
pub struct AIConfigDeletePayload {
    id: i64,
    api_key_ref: Option<String>,
}

#[derive(Deserialize, Serialize)]
pub struct AIConfigTestPayload {
    id: i64,
    api_key_ref: String,
}

#[derive(Deserialize, Serialize)]
pub struct AIConfigSavePayload {
    id: i64,
    name: String,
    provider: String,
    base_url: String,
    api_key_ref: String,
    masked_api_key: String,
    has_api_key: bool,
    api_key: Option<String>,
    model_name: String,
    temperature: f64,
    max_tokens: i32,
    timeout_seconds: i32,
    stream_enabled: bool,
    is_default: bool,
}

#[derive(Serialize)]
struct AIConfigForwardPayload {
    id: i64,
    name: String,
    provider: String,
    base_url: String,
    api_key_ref: String,
    masked_api_key: String,
    has_api_key: bool,
    model_name: String,
    temperature: f64,
    max_tokens: i32,
    timeout_seconds: i32,
    stream_enabled: bool,
    is_default: bool,
}

struct AIConfigForwardPlan {
    payload: AIConfigForwardPayload,
    previous_api_key_ref: Option<String>,
    new_api_key_ref: Option<String>,
}

struct AIConfigDeletePlan {
    request: AIConfigDeleteRequest,
    api_key_ref: Option<String>,
}

pub(crate) struct StoredCredential {
    pub(crate) reference: String,
    masked: String,
}

#[derive(Debug)]
pub(crate) enum CredentialError {
    Io(std::io::Error),
    InvalidReference,
    EmptySecret,
    InvalidConfigID,
    MissingCredentialMetadata,
}

impl fmt::Display for CredentialError {
    /// 将本地 vault 错误转换为不包含密钥内容的文本。
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            CredentialError::Io(error) => write!(formatter, "credential io error: {error}"),
            CredentialError::InvalidReference => write!(formatter, "invalid credential reference"),
            CredentialError::EmptySecret => write!(formatter, "credential secret is empty"),
            CredentialError::InvalidConfigID => write!(formatter, "invalid ai config id"),
            CredentialError::MissingCredentialMetadata => {
                write!(formatter, "ai config credential metadata not found")
            }
        }
    }
}

impl std::error::Error for CredentialError {}

impl From<std::io::Error> for CredentialError {
    /// 统一转换本地文件 IO 错误，避免调用方重复映射。
    fn from(error: std::io::Error) -> Self {
        CredentialError::Io(error)
    }
}

pub(crate) struct LocalCredentialVault {
    root: PathBuf,
}

/// 按本地 vault 引用解析 AI Key，仅供 Rust 内部白名单 command 构造运行期请求。
pub(crate) fn resolve_ai_key_for_internal_request(reference: &str) -> Result<String, String> {
    LocalCredentialVault::default()
        .read(reference)
        .map_err(|error| error.to_string())
}

/// 按 AI 配置 ID 从 Go core 元数据读取绑定的 vault 引用，前端传入引用只能作为一致性校验。
pub(crate) fn resolve_ai_key_ref_for_config(
    client: &CoreClient,
    config_id: i64,
    provided_reference: Option<&str>,
) -> Result<String, String> {
    let envelope: CoreEnvelope<AIConfigListData> = client
        .post_api("/api/ai/configs/list", &AIConfigListRequest {})
        .map_err(|error| error.to_string())?;
    if envelope.code != 0 {
        return Err(envelope.message);
    }
    select_bound_ai_key_ref(&envelope.data.items, config_id, provided_reference)
}

/// 按 AI 配置 ID 从 Go core 读取已绑定凭据元数据，保存和删除路径不得信任 renderer 传入引用。
fn resolve_ai_config_credential_metadata(
    client: &CoreClient,
    config_id: i64,
) -> Result<Option<AIConfigCredentialMetadata>, String> {
    if config_id <= 0 {
        return Ok(None);
    }
    let envelope: CoreEnvelope<AIConfigListData> = client
        .post_api("/api/ai/configs/list", &AIConfigListRequest {})
        .map_err(|error| error.to_string())?;
    if envelope.code != 0 {
        return Err(envelope.message);
    }
    select_ai_config_credential_metadata(&envelope.data.items, config_id)
}

impl LocalCredentialVault {
    /// 创建本地文件 vault，调用方负责传入用户级目录或测试临时目录。
    pub(crate) fn new(root: PathBuf) -> Self {
        Self { root }
    }

    /// 使用默认用户目录创建本地文件 vault，避免依赖平台凭据服务。
    pub(crate) fn default() -> Self {
        Self::new(default_credential_root())
    }

    /// 保存 AI Key 到本地 vault 文件，并返回可落库的引用和脱敏展示值。
    fn save_ai_key(
        &self,
        config_id: i64,
        provider: &str,
        secret: &str,
    ) -> Result<StoredCredential, CredentialError> {
        let trimmed = secret.trim();
        if trimmed.is_empty() {
            return Err(CredentialError::EmptySecret);
        }

        let provider_slug = sanitize_ref_segment(provider);
        if provider_slug.is_empty() {
            return Err(CredentialError::InvalidReference);
        }
        let slug = format!(
            "{}-{}",
            provider_slug,
            sanitize_ref_segment(&ai_key_reference_id(config_id))
        );
        ensure_vault_dir(&self.root)?;
        let reference = format!("{LOCAL_VAULT_REF_PREFIX}{slug}");
        write_secret_file(&self.path_for_reference(&reference)?, trimmed)?;

        Ok(StoredCredential {
            reference,
            masked: mask_secret(trimmed),
        })
    }

    /// 保存代理密码到本地 vault 文件，并返回只允许落库的凭据引用。
    pub(crate) fn save_proxy_password(
        &self,
        profile: &str,
        secret: &str,
    ) -> Result<StoredCredential, CredentialError> {
        let trimmed = secret.trim();
        if trimmed.is_empty() {
            return Err(CredentialError::EmptySecret);
        }

        let slug = sanitize_ref_segment(profile);
        if slug.is_empty() {
            return Err(CredentialError::InvalidReference);
        }

        ensure_vault_dir(&self.root)?;
        let reference = format!(
            "{LOCAL_PROXY_VAULT_REF_PREFIX}{}-{:016x}",
            slug,
            rand::random::<u64>()
        );
        write_secret_file(&self.path_for_reference(&reference)?, trimmed)?;

        Ok(StoredCredential {
            reference,
            masked: mask_secret(trimmed),
        })
    }

    /// 读取本地 vault 中的密钥，只供 Rust 内部注入 Go core 的运行期请求使用。
    pub(crate) fn read(&self, reference: &str) -> Result<String, CredentialError> {
        let content = fs::read_to_string(self.path_for_reference(reference)?)?;
        Ok(content)
    }

    /// 删除本地 vault 中的密钥文件；文件不存在时按已删除处理。
    pub(crate) fn delete(&self, reference: &str) -> Result<(), CredentialError> {
        match fs::remove_file(self.path_for_reference(reference)?) {
            Ok(()) => Ok(()),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(error) => Err(CredentialError::Io(error)),
        }
    }

    /// 校验引用必须是代理密码 vault 引用，避免 settings 清理路径误删 AI Key。
    pub(crate) fn validate_proxy_reference(&self, reference: &str) -> Result<(), CredentialError> {
        self.path_for_reference_with_prefix(reference, LOCAL_PROXY_VAULT_REF_PREFIX)
            .map(|_| ())
    }

    /// 根据本地 vault 引用解析文件路径，拒绝非本地 vault scheme。
    fn path_for_reference(&self, reference: &str) -> Result<PathBuf, CredentialError> {
        let suffix = reference
            .strip_prefix(LOCAL_VAULT_REF_PREFIX)
            .or_else(|| reference.strip_prefix(LOCAL_PROXY_VAULT_REF_PREFIX))
            .ok_or(CredentialError::InvalidReference)?;
        let slug = sanitize_ref_segment(suffix);
        if slug.is_empty() {
            return Err(CredentialError::InvalidReference);
        }
        Ok(self.root.join(format!("{slug}.secret")))
    }

    /// 根据指定 vault 前缀解析文件路径，供调用方收紧具体凭据类型。
    fn path_for_reference_with_prefix(
        &self,
        reference: &str,
        prefix: &str,
    ) -> Result<PathBuf, CredentialError> {
        let suffix = reference
            .strip_prefix(prefix)
            .ok_or(CredentialError::InvalidReference)?;
        let slug = sanitize_ref_segment(suffix);
        if slug.is_empty() {
            return Err(CredentialError::InvalidReference);
        }
        Ok(self.root.join(format!("{slug}.secret")))
    }
}

/// 读取 AI 配置列表，固定转发到 Go core `/api/ai/configs/list`。
#[tauri::command]
pub async fn ai_config_list(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/ai/configs/list", &AIConfigListRequest {})
        .map_err(|error| error.to_string())
}

/// 保存 AI 配置元数据，固定转发到 Go core `/api/ai/configs/save`。
#[tauri::command]
pub async fn ai_config_save(
    state: State<'_, CoreState>,
    payload: AIConfigSavePayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    let vault = LocalCredentialVault::default();
    let credential_metadata = resolve_ai_config_credential_metadata(&client, payload.id)?;
    let plan = build_forward_payload(payload, &vault, credential_metadata.as_ref())
        .map_err(|error| error.to_string())?;
    let response: Result<serde_json::Value, _> =
        client.post_api("/api/ai/configs/save", &plan.payload);
    match response {
        Ok(value) => {
            cleanup_forward_payload(&plan, &vault).map_err(|error| error.to_string())?;
            Ok(value)
        }
        Err(error) => {
            let _ = rollback_forward_payload(&plan, &vault);
            Err(error.to_string())
        }
    }
}

/// 测试 AI 配置连通性，固定读取本地 vault 后转发到 Go core `/api/ai/configs/test`。
#[tauri::command]
pub async fn ai_config_test(
    state: State<'_, CoreState>,
    payload: AIConfigTestPayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    let vault = LocalCredentialVault::default();
    let credential_metadata = resolve_ai_config_credential_metadata(&client, payload.id)?;
    let metadata = credential_metadata.ok_or_else(|| "ai config not found".to_string())?;
    let api_key_ref = select_bound_ai_key_ref_from_metadata(&metadata, Some(&payload.api_key_ref))?;
    let request = build_test_request(
        AIConfigTestPayload {
            id: payload.id,
            api_key_ref,
        },
        &vault,
    )
    .map_err(|error| error.to_string())?;
    client
        .post_api_with_timeout(
            "/api/ai/configs/test",
            &request,
            ai_config_test_request_timeout(metadata.timeout_seconds),
        )
        .map_err(|error| error.to_string())
}

/// 删除 AI 配置元数据和可选本地 vault 密钥，固定转发到 Go core `/api/ai/configs/delete`。
#[tauri::command]
pub async fn ai_config_delete(
    state: State<'_, CoreState>,
    payload: AIConfigDeletePayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    let vault = LocalCredentialVault::default();
    validate_ai_config_id(payload.id).map_err(|error| error.to_string())?;
    let credential_metadata = resolve_ai_config_credential_metadata(&client, payload.id)?;
    let plan = build_delete_request(payload.id, credential_metadata.as_ref(), &vault)
        .map_err(|error| error.to_string())?;
    let response: serde_json::Value = client
        .post_api("/api/ai/configs/delete", &plan.request)
        .map_err(|error| error.to_string())?;
    cleanup_delete_request(&plan, &vault).map_err(|error| error.to_string())?;
    Ok(response)
}

/// 构造模型连通性测试请求，真实密钥只在 Rust 内部读取后注入给 Go core。
fn build_test_request(
    payload: AIConfigTestPayload,
    vault: &LocalCredentialVault,
) -> Result<AIConfigTestRequest, CredentialError> {
    validate_ai_config_id(payload.id)?;
    let resolved_api_key = vault.read(&payload.api_key_ref)?;
    if resolved_api_key.trim().is_empty() {
        return Err(CredentialError::EmptySecret);
    }
    Ok(AIConfigTestRequest {
        id: payload.id,
        resolved_api_key,
    })
}

/// 构造删除请求并先校验配置 ID，避免非法 ID 误删本地 vault 文件。
fn build_delete_request(
    config_id: i64,
    credential_metadata: Option<&AIConfigCredentialMetadata>,
    vault: &LocalCredentialVault,
) -> Result<AIConfigDeletePlan, CredentialError> {
    validate_ai_config_id(config_id)?;
    let api_key_ref = credential_metadata
        .and_then(|metadata| {
            if metadata.has_api_key {
                Some(metadata.api_key_ref.trim())
            } else {
                None
            }
        })
        .filter(|reference| !reference.is_empty())
        .map(|reference| {
            vault.path_for_reference(reference)?;
            Ok::<String, CredentialError>(reference.to_string())
        })
        .transpose()?;
    Ok(AIConfigDeletePlan {
        request: AIConfigDeleteRequest { id: config_id },
        api_key_ref,
    })
}

/// 构造转发给 Go core 的安全 payload，明文 API Key 只写本地 vault，不进入 JSON。
fn build_forward_payload(
    payload: AIConfigSavePayload,
    vault: &LocalCredentialVault,
    credential_metadata: Option<&AIConfigCredentialMetadata>,
) -> Result<AIConfigForwardPlan, CredentialError> {
    let mut api_key_ref = String::new();
    let mut masked_api_key = String::new();
    let mut has_api_key = false;
    let mut replaced_api_key_ref = None;
    let mut new_api_key_ref = None;

    if let Some(secret) = payload
        .api_key
        .as_deref()
        .map(str::trim)
        .filter(|item| !item.is_empty())
    {
        let previous_reference = bound_ai_key_ref(payload.id, credential_metadata, vault)?;
        if !previous_reference.trim().is_empty() {
            vault.path_for_reference(&previous_reference)?;
        }

        let stored = vault.save_ai_key(payload.id, &payload.provider, secret)?;
        let stored_reference = stored.reference.clone();
        api_key_ref = stored.reference;
        masked_api_key = stored.masked;
        has_api_key = true;
        new_api_key_ref = Some(stored_reference);
        if !previous_reference.trim().is_empty() && previous_reference != api_key_ref {
            replaced_api_key_ref = Some(previous_reference);
        } else {
            replaced_api_key_ref = None;
        }
    } else if payload.id > 0 {
        let metadata = credential_metadata.ok_or(CredentialError::MissingCredentialMetadata)?;
        let bound_reference = bound_ai_key_ref(payload.id, credential_metadata, vault)?;
        api_key_ref = bound_reference;
        masked_api_key = if metadata.has_api_key {
            metadata.masked_api_key.clone()
        } else {
            String::new()
        };
        has_api_key = metadata.has_api_key;
    }

    Ok(AIConfigForwardPlan {
        payload: AIConfigForwardPayload {
            id: payload.id,
            name: payload.name,
            provider: payload.provider,
            base_url: payload.base_url,
            api_key_ref,
            masked_api_key,
            has_api_key,
            model_name: payload.model_name,
            temperature: payload.temperature,
            max_tokens: payload.max_tokens,
            timeout_seconds: payload.timeout_seconds,
            stream_enabled: payload.stream_enabled,
            is_default: payload.is_default,
        },
        previous_api_key_ref: replaced_api_key_ref,
        new_api_key_ref,
    })
}

/// 读取 Go core 已绑定的 AI Key 引用；已有配置必须先查到元数据，不能从 renderer payload 继承。
fn bound_ai_key_ref(
    config_id: i64,
    credential_metadata: Option<&AIConfigCredentialMetadata>,
    vault: &LocalCredentialVault,
) -> Result<String, CredentialError> {
    if config_id <= 0 {
        return Ok(String::new());
    }
    let metadata = credential_metadata.ok_or(CredentialError::MissingCredentialMetadata)?;
    if !metadata.has_api_key {
        return Ok(String::new());
    }
    let reference = metadata.api_key_ref.trim();
    if reference.is_empty() {
        return Err(CredentialError::InvalidReference);
    }
    vault.path_for_reference(reference)?;
    Ok(reference.to_string())
}

/// Go core 保存失败时回滚本次新写入的 secret，不触碰仍被 SQLite 引用的旧 secret。
fn rollback_forward_payload(
    plan: &AIConfigForwardPlan,
    vault: &LocalCredentialVault,
) -> Result<(), CredentialError> {
    if let Some(reference) = &plan.new_api_key_ref {
        vault.delete(reference)?;
    }
    Ok(())
}

/// Go core 保存成功后再清理被替换的旧 secret，避免保存失败造成元数据悬空。
fn cleanup_forward_payload(
    plan: &AIConfigForwardPlan,
    vault: &LocalCredentialVault,
) -> Result<(), CredentialError> {
    if let Some(reference) = &plan.previous_api_key_ref {
        vault.delete(reference)?;
    }
    Ok(())
}

/// Go core 软删除配置成功后再删除本地 secret，避免失败时配置引用失效。
fn cleanup_delete_request(
    plan: &AIConfigDeletePlan,
    vault: &LocalCredentialVault,
) -> Result<(), CredentialError> {
    if let Some(reference) = &plan.api_key_ref {
        vault.delete(reference)?;
    }
    Ok(())
}

/// 校验 AI 配置 ID，删除和测试必须引用已存在配置。
fn validate_ai_config_id(id: i64) -> Result<(), CredentialError> {
    if id <= 0 {
        return Err(CredentialError::InvalidConfigID);
    }
    Ok(())
}

/// 从 AI 配置列表中选择与配置 ID 绑定的 vault 引用，拒绝前端传入的错配引用。
fn select_bound_ai_key_ref(
    configs: &[AIConfigListItem],
    config_id: i64,
    provided_reference: Option<&str>,
) -> Result<String, String> {
    if config_id <= 0 {
        return Err(CredentialError::InvalidConfigID.to_string());
    }
    let item = configs
        .iter()
        .find(|item| item.id == config_id)
        .ok_or_else(|| "ai config not found".to_string())?;
    let stored_reference = item.api_key_ref.trim();
    if !item.has_api_key || stored_reference.is_empty() {
        return Err(CredentialError::InvalidReference.to_string());
    }
    if let Some(provided) = provided_reference
        .map(str::trim)
        .filter(|item| !item.is_empty())
    {
        if provided != stored_reference {
            return Err(CredentialError::InvalidReference.to_string());
        }
    }
    Ok(stored_reference.to_string())
}

/// 从配置列表中选择指定配置的凭据元数据，供保存和删除路径绑定本地 vault 操作。
fn select_ai_config_credential_metadata(
    configs: &[AIConfigListItem],
    config_id: i64,
) -> Result<Option<AIConfigCredentialMetadata>, String> {
    if config_id <= 0 {
        return Ok(None);
    }
    let item = configs
        .iter()
        .find(|item| item.id == config_id)
        .ok_or_else(|| "ai config not found".to_string())?;
    Ok(Some(AIConfigCredentialMetadata {
        api_key_ref: item.api_key_ref.clone(),
        masked_api_key: item.masked_api_key.clone(),
        has_api_key: item.has_api_key,
        timeout_seconds: item.timeout_seconds,
    }))
}

/// 从配置元数据中选择与配置 ID 绑定的 vault 引用，拒绝前端传入的错配引用。
fn select_bound_ai_key_ref_from_metadata(
    metadata: &AIConfigCredentialMetadata,
    provided_reference: Option<&str>,
) -> Result<String, String> {
    let stored_reference = metadata.api_key_ref.trim();
    if !metadata.has_api_key || stored_reference.is_empty() {
        return Err(CredentialError::InvalidReference.to_string());
    }
    if let Some(provided) = provided_reference
        .map(str::trim)
        .filter(|item| !item.is_empty())
    {
        if provided != stored_reference {
            return Err(CredentialError::InvalidReference.to_string());
        }
    }
    Ok(stored_reference.to_string())
}

/// 生成模型连通性测试专用超时，避免普通本地 API 短超时截断真实 Provider 测试。
fn ai_config_test_request_timeout(timeout_seconds: i32) -> Duration {
    let provider_timeout = if timeout_seconds > 0 {
        timeout_seconds as u64
    } else {
        120
    };
    Duration::from_secs(provider_timeout + AI_CONFIG_TEST_TIMEOUT_BUFFER_SECONDS)
}

/// 生成 AI Key vault 引用 ID；每次写入都使用新后缀，避免保存失败时覆盖仍被 SQLite 引用的旧密钥。
fn ai_key_reference_id(config_id: i64) -> String {
    if config_id > 0 {
        return format!("{}-{:016x}", config_id, rand::random::<u64>());
    }
    format!("new-{:016x}", rand::random::<u64>())
}

/// 确保本地 vault 目录存在；Unix/macOS 上只允许当前用户进入和枚举。
fn ensure_vault_dir(path: &Path) -> Result<(), std::io::Error> {
    fs::create_dir_all(path)?;
    #[cfg(unix)]
    fs::set_permissions(path, fs::Permissions::from_mode(0o700))?;
    Ok(())
}

/// 写入本地 vault secret 文件；Unix/macOS 上强制收紧为当前用户读写。
fn write_secret_file(path: &Path, secret: &str) -> Result<(), std::io::Error> {
    let mut options = fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    options.mode(0o600);

    let mut file = options.open(path)?;
    file.write_all(secret.as_bytes())?;
    #[cfg(unix)]
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    Ok(())
}

/// 返回本地 vault 默认目录，支持测试和开发通过环境变量覆盖。
fn default_credential_root() -> PathBuf {
    if let Ok(path) = std::env::var(CREDENTIAL_DIR_ENV) {
        return PathBuf::from(path);
    }
    if cfg!(windows) {
        return std::env::var("APPDATA")
            .map(PathBuf::from)
            .unwrap_or_else(|_| std::env::temp_dir())
            .join("Invest Compass")
            .join("credentials");
    }
    std::env::var("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| std::env::temp_dir())
        .join("Library")
        .join("Application Support")
        .join("Invest Compass")
        .join("credentials")
}

/// 将密钥脱敏为首尾少量字符，避免页面和日志出现完整密钥。
fn mask_secret(secret: &str) -> String {
    let chars = secret.chars().collect::<Vec<_>>();
    if chars.len() <= 8 {
        return "****".to_string();
    }
    let prefix = chars.iter().take(4).collect::<String>();
    let suffix = chars
        .iter()
        .skip(chars.len().saturating_sub(4))
        .collect::<String>();
    format!("{prefix}****{suffix}")
}

/// 清理本地 vault 引用片段，只保留可安全作为文件名的字符。
fn sanitize_ref_segment(value: &str) -> String {
    value
        .chars()
        .map(|item| {
            if item.is_ascii_alphanumeric() || item == '-' || item == '_' {
                item
            } else {
                '-'
            }
        })
        .collect::<String>()
        .trim_matches('-')
        .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    /// 验证本地 vault 保存、读取、删除 AI Key，引用中不包含明文密钥。
    fn local_credential_vault_persists_and_deletes_api_key() {
        let root = unique_test_dir("vault-roundtrip");
        let vault = LocalCredentialVault::new(root.clone());

        let stored = vault
            .save_ai_key(7, "openai-compatible", "sk-test-secret-123456")
            .expect("api key should save");

        assert!(stored.reference.starts_with("local-vault://ai-config/"));
        assert!(!stored.reference.contains("sk-test-secret"));
        assert_eq!(stored.masked, "sk-t****3456");
        assert_eq!(
            vault
                .read(&stored.reference)
                .expect("api key should read back"),
            "sk-test-secret-123456"
        );

        vault
            .delete(&stored.reference)
            .expect("api key should delete");
        assert!(vault.read(&stored.reference).is_err());

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 Rust 转发 AI 配置时剥离明文 Key，只把本地 vault 引用交给 Go core。
    fn build_forward_payload_strips_plain_api_key() {
        let root = unique_test_dir("forward-payload");
        let vault = LocalCredentialVault::new(root.clone());
        let payload = AIConfigSavePayload {
            id: 0,
            name: "默认模型".to_string(),
            provider: "openai-compatible".to_string(),
            base_url: "https://api.example.test".to_string(),
            api_key_ref: String::new(),
            masked_api_key: String::new(),
            has_api_key: false,
            api_key: Some("sk-test-secret-123456".to_string()),
            model_name: "gpt-4.1-mini".to_string(),
            temperature: 0.2,
            max_tokens: 4096,
            timeout_seconds: 30,
            stream_enabled: true,
            is_default: true,
        };

        let plan = build_forward_payload(payload, &vault, None).expect("payload should build");
        let encoded = serde_json::to_string(&plan.payload).expect("payload should serialize");

        assert!(encoded.contains("local-vault://ai-config/"));
        assert!(encoded.contains("sk-t****3456"));
        assert!(!encoded.contains("\"api_key\""));
        assert!(!encoded.contains("sk-test-secret"));

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证新建 AI 配置尚无数据库 ID 时，本地 vault 引用仍然互不覆盖。
    fn build_forward_payload_uses_unique_vault_refs_for_new_configs() {
        let root = unique_test_dir("new-config-refs");
        let vault = LocalCredentialVault::new(root.clone());

        let first = build_forward_payload(
            AIConfigSavePayload {
                id: 0,
                name: "模型一".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: String::new(),
                masked_api_key: String::new(),
                has_api_key: false,
                api_key: Some("sk-first-secret-123456".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: false,
            },
            &vault,
            None,
        )
        .expect("first payload should build");
        let second = build_forward_payload(
            AIConfigSavePayload {
                id: 0,
                name: "模型二".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: String::new(),
                masked_api_key: String::new(),
                has_api_key: false,
                api_key: Some("sk-second-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: false,
            },
            &vault,
            None,
        )
        .expect("second payload should build");

        assert_ne!(first.payload.api_key_ref, second.payload.api_key_ref);
        assert_eq!(
            vault
                .read(&first.payload.api_key_ref)
                .expect("first key should remain readable"),
            "sk-first-secret-123456"
        );
        assert_eq!(
            vault
                .read(&second.payload.api_key_ref)
                .expect("second key should remain readable"),
            "sk-second-secret-654321"
        );

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证构造保存 payload 时不会提前删除旧 vault 引用，避免 Go core 保存失败后元数据失效。
    fn build_forward_payload_keeps_previous_key_until_core_save_succeeds() {
        let root = unique_test_dir("replace-api-key-ref");
        let vault = LocalCredentialVault::new(root.clone());
        let old = vault
            .save_ai_key(0, "openai-compatible", "sk-old-secret-123456")
            .expect("old key should save");

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: old.reference.clone(),
                masked_api_key: old.masked.clone(),
                has_api_key: true,
                api_key: Some("sk-new-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&metadata_from_stored(&old)),
        )
        .expect("payload should build");

        assert_ne!(plan.payload.api_key_ref, old.reference);
        assert_eq!(
            vault
                .read(&old.reference)
                .expect("old key should remain readable before core save"),
            "sk-old-secret-123456"
        );
        assert_eq!(
            vault
                .read(&plan.payload.api_key_ref)
                .expect("new key should be readable"),
            "sk-new-secret-654321"
        );

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 Go core 保存失败时会删除本次新写入的 secret，旧引用保持可用。
    fn rollback_forward_payload_removes_new_key_and_keeps_previous_key() {
        let root = unique_test_dir("rollback-api-key-ref");
        let vault = LocalCredentialVault::new(root.clone());
        let old = vault
            .save_ai_key(0, "openai-compatible", "sk-old-secret-123456")
            .expect("old key should save");

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: old.reference.clone(),
                masked_api_key: old.masked.clone(),
                has_api_key: true,
                api_key: Some("sk-new-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&metadata_from_stored(&old)),
        )
        .expect("payload should build");
        let new_ref = plan.payload.api_key_ref.clone();

        rollback_forward_payload(&plan, &vault).expect("rollback should delete new key");

        assert_eq!(
            vault
                .read(&old.reference)
                .expect("old key should remain readable after rollback"),
            "sk-old-secret-123456"
        );
        assert!(vault.read(&new_ref).is_err());

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证同一配置更新同 provider 时也使用新引用，避免失败回滚误删仍被 SQLite 引用的旧 secret。
    fn build_forward_payload_uses_distinct_ref_when_replacing_same_config_key() {
        let root = unique_test_dir("same-config-replace");
        let vault = LocalCredentialVault::new(root.clone());
        let old = vault
            .save_ai_key(7, "openai-compatible", "sk-old-secret-123456")
            .expect("old key should save");

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: old.reference.clone(),
                masked_api_key: old.masked.clone(),
                has_api_key: true,
                api_key: Some("sk-new-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&metadata_from_stored(&old)),
        )
        .expect("payload should build");

        assert_ne!(plan.payload.api_key_ref, old.reference);
        rollback_forward_payload(&plan, &vault).expect("rollback should delete only new key");
        assert_eq!(
            vault
                .read(&old.reference)
                .expect("old key should remain readable after rollback"),
            "sk-old-secret-123456"
        );

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 Go core 保存成功后才清理旧 secret，避免历史密钥长期残留。
    fn cleanup_forward_payload_removes_previous_key_after_core_save_succeeds() {
        let root = unique_test_dir("cleanup-api-key-ref");
        let vault = LocalCredentialVault::new(root.clone());
        let old = vault
            .save_ai_key(0, "openai-compatible", "sk-old-secret-123456")
            .expect("old key should save");

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: old.reference.clone(),
                masked_api_key: old.masked.clone(),
                has_api_key: true,
                api_key: Some("sk-new-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&metadata_from_stored(&old)),
        )
        .expect("payload should build");

        cleanup_forward_payload(&plan, &vault).expect("cleanup should delete old key");

        assert!(vault.read(&old.reference).is_err());
        assert_eq!(
            vault
                .read(&plan.payload.api_key_ref)
                .expect("new key should remain readable"),
            "sk-new-secret-654321"
        );

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证未提交新 API Key 时，保存 payload 只能沿用 Go core 已绑定的凭据元数据。
    fn build_forward_payload_uses_bound_metadata_when_api_key_is_unchanged() {
        let root = unique_test_dir("bound-metadata-save");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(7, "openai-compatible", "sk-bound-secret-123456")
            .expect("bound key should save");
        let bound_metadata = metadata_from_stored(&stored);

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: "local-vault://ai-config/openai-compatible-other".to_string(),
                masked_api_key: "fake****fake".to_string(),
                has_api_key: false,
                api_key: None,
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&bound_metadata),
        )
        .expect("payload should preserve bound metadata");

        assert_eq!(plan.payload.api_key_ref, stored.reference);
        assert_eq!(plan.payload.masked_api_key, stored.masked);
        assert!(plan.payload.has_api_key);

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证已有配置未提交新 API Key 时，缺少 Go core 绑定元数据会快速失败。
    fn build_forward_payload_rejects_existing_config_without_bound_metadata() {
        let root = unique_test_dir("missing-bound-metadata-save");
        let vault = LocalCredentialVault::new(root.clone());

        let error = match build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: "local-vault://ai-config/openai-compatible-other".to_string(),
                masked_api_key: "fake****fake".to_string(),
                has_api_key: true,
                api_key: None,
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            None,
        ) {
            Ok(_) => panic!("existing config without bound metadata should fail"),
            Err(error) => error,
        };

        assert_eq!(error.to_string(), "ai config credential metadata not found");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证替换 API Key 时忽略 renderer 提交的旧引用，只按 Go core 已绑定引用清理。
    fn build_forward_payload_ignores_payload_ref_when_replacing_api_key() {
        let root = unique_test_dir("ignore-payload-ref");
        let vault = LocalCredentialVault::new(root.clone());
        let old = vault
            .save_ai_key(7, "openai-compatible", "sk-old-secret-123456")
            .expect("old key should save");

        let plan = build_forward_payload(
            AIConfigSavePayload {
                id: 7,
                name: "默认模型".to_string(),
                provider: "openai-compatible".to_string(),
                base_url: "https://api.example.test".to_string(),
                api_key_ref: "invalid-vault://ai-config/openai-compatible-7".to_string(),
                masked_api_key: "sk-o****3456".to_string(),
                has_api_key: true,
                api_key: Some("sk-new-secret-654321".to_string()),
                model_name: "gpt-4.1-mini".to_string(),
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 30,
                stream_enabled: true,
                is_default: true,
            },
            &vault,
            Some(&metadata_from_stored(&old)),
        )
        .expect("payload should build from bound metadata");

        assert_ne!(plan.payload.api_key_ref, old.reference);
        assert_eq!(plan.previous_api_key_ref, Some(old.reference.clone()));
        assert_eq!(
            vault
                .read(&old.reference)
                .expect("old key should remain before cleanup"),
            "sk-old-secret-123456"
        );

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证模型测试请求从本地 vault 读取密钥，只在 Rust 内部构造运行期注入 payload。
    fn build_test_request_reads_local_vault_secret() {
        let root = unique_test_dir("test-request");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let request = build_test_request(
            AIConfigTestPayload {
                id: 9,
                api_key_ref: stored.reference.clone(),
            },
            &vault,
        )
        .expect("test request should build");
        let encoded = serde_json::to_string(&request).expect("request should serialize");

        assert_eq!(request.id, 9);
        assert!(encoded.contains("\"resolved_api_key\":\"sk-runtime-secret-654321\""));
        assert!(!encoded.contains("api_key_ref"));
        assert!(!encoded.contains(&stored.reference));

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证运行期密钥引用必须来自 Go core 中与配置 ID 绑定的元数据。
    fn select_bound_ai_key_ref_rejects_mismatched_provided_ref() {
        let configs = vec![AIConfigListItem {
            id: 9,
            api_key_ref: "local-vault://ai-config/openai-compatible-new-abc".to_string(),
            masked_api_key: "sk-r****abcd".to_string(),
            has_api_key: true,
            timeout_seconds: 30,
        }];

        assert_eq!(
            select_bound_ai_key_ref(
                &configs,
                9,
                Some("local-vault://ai-config/openai-compatible-other")
            )
            .expect_err("mismatched api key ref should fail"),
            "invalid credential reference"
        );
        assert_eq!(
            select_bound_ai_key_ref(
                &configs,
                9,
                Some("local-vault://ai-config/openai-compatible-new-abc")
            )
            .expect("matching ref should pass"),
            "local-vault://ai-config/openai-compatible-new-abc"
        );
    }

    #[test]
    /// 验证模型连通性测试 HTTP 超时覆盖 Provider 配置超时和缓冲时间。
    fn ai_config_test_request_timeout_adds_provider_buffer() {
        assert_eq!(ai_config_test_request_timeout(30), Duration::from_secs(35));
        assert_eq!(ai_config_test_request_timeout(0), Duration::from_secs(125));
    }

    #[cfg(unix)]
    #[test]
    /// 验证本地 vault 写入的凭据文件只允许当前用户读写，降低本机其他用户读取风险。
    fn local_vault_secret_file_uses_private_unix_permissions() {
        use std::os::unix::fs::PermissionsExt;

        let root = unique_test_dir("secret-file-permissions");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let metadata = fs::metadata(
            vault
                .path_for_reference(&stored.reference)
                .expect("secret path should resolve"),
        )
        .expect("secret file should exist");

        assert_eq!(metadata.permissions().mode() & 0o777, 0o600);
        let _ = fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证本地 vault 目录只允许当前用户访问，避免其他本机用户枚举凭据文件名。
    fn local_vault_directory_uses_private_unix_permissions() {
        use std::os::unix::fs::PermissionsExt;

        let root = unique_test_dir("directory-permissions");
        let vault = LocalCredentialVault::new(root.clone());
        vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let metadata = fs::metadata(&root).expect("vault directory should exist");
        assert_eq!(metadata.permissions().mode() & 0o777, 0o700);

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证本地 vault 引用的有效文件名片段不能为空，避免空引用落到 `.secret` 文件。
    fn local_vault_rejects_empty_or_unsafely_sanitized_reference_suffix() {
        let root = unique_test_dir("empty-ref-suffix");
        let vault = LocalCredentialVault::new(root.clone());

        for reference in [
            "local-vault://ai-config/",
            "local-vault://proxy/",
            "local-vault://ai-config/../../",
            "local-vault://proxy/   ",
        ] {
            let error = vault
                .path_for_reference(reference)
                .expect_err("empty sanitized reference should fail");
            assert_eq!(error.to_string(), "invalid credential reference");
        }

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证代理密码 profile 清理后不能为空，避免多个非法 profile 覆盖同一个 `.secret` 文件。
    fn local_vault_rejects_empty_proxy_profile() {
        let root = unique_test_dir("empty-proxy-profile");
        let vault = LocalCredentialVault::new(root.clone());

        let error = match vault.save_proxy_password(" /// ", "proxy-secret-123456") {
            Ok(_) => panic!("empty proxy profile should fail"),
            Err(error) => error,
        };

        assert_eq!(error.to_string(), "invalid credential reference");
        let no_secret_files = !root.exists()
            || fs::read_dir(&root)
                .expect("test dir should be readable")
                .next()
                .is_none();
        assert!(no_secret_files);

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 AI Key 的 provider 清理后不能为空，避免凭据引用退化为只按配置 ID 命名。
    fn local_vault_rejects_empty_ai_provider_segment() {
        let root = unique_test_dir("empty-ai-provider");
        let vault = LocalCredentialVault::new(root.clone());

        let error = match vault.save_ai_key(9, " /// ", "sk-runtime-secret-654321") {
            Ok(_) => panic!("empty provider segment should fail"),
            Err(error) => error,
        };

        assert_eq!(error.to_string(), "invalid credential reference");
        let no_secret_files = !root.exists()
            || fs::read_dir(&root)
                .expect("test dir should be readable")
                .next()
                .is_none();
        assert!(no_secret_files);

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证模型测试请求在读取本地 vault 前拒绝非正数配置 ID。
    fn build_test_request_rejects_non_positive_config_id() {
        let root = unique_test_dir("test-request-invalid-id");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let error = match build_test_request(
            AIConfigTestPayload {
                id: 0,
                api_key_ref: stored.reference,
            },
            &vault,
        ) {
            Ok(_) => panic!("non-positive config id should fail"),
            Err(error) => error,
        };

        assert_eq!(error.to_string(), "invalid ai config id");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证删除请求先校验配置 ID，非法 ID 不会误删本地 vault 凭据。
    fn build_delete_request_rejects_invalid_id_before_deleting_vault_secret() {
        let root = unique_test_dir("delete-invalid-id");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let metadata = metadata_from_stored(&stored);
        let error = match build_delete_request(0, Some(&metadata), &vault) {
            Ok(_) => panic!("non-positive config id should fail"),
            Err(error) => error,
        };

        assert_eq!(error.to_string(), "invalid ai config id");
        assert_eq!(
            vault
                .read(&stored.reference)
                .expect("credential should not be deleted"),
            "sk-runtime-secret-654321"
        );
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证删除配置请求不会在 Go core 软删除元数据前提前删除本地 vault secret。
    fn build_delete_request_keeps_secret_until_core_delete_succeeds() {
        let root = unique_test_dir("delete-after-core");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");

        let metadata = metadata_from_stored(&stored);
        let plan =
            build_delete_request(9, Some(&metadata), &vault).expect("delete request should build");

        assert_eq!(plan.request.id, 9);
        assert_eq!(
            vault
                .read(&stored.reference)
                .expect("secret should remain before core delete succeeds"),
            "sk-runtime-secret-654321"
        );

        cleanup_delete_request(&plan, &vault).expect("cleanup should delete secret");
        assert!(vault.read(&stored.reference).is_err());

        let _ = fs::remove_dir_all(root);
    }

    #[test]
    /// 验证删除计划只使用 Go core 已绑定元数据，不接受 renderer 指定其他 vault 引用。
    fn build_delete_request_uses_bound_metadata_only() {
        let root = unique_test_dir("delete-bound-metadata");
        let vault = LocalCredentialVault::new(root.clone());
        let stored = vault
            .save_ai_key(9, "openai-compatible", "sk-runtime-secret-654321")
            .expect("api key should save");
        let other = vault
            .save_ai_key(10, "openai-compatible", "sk-other-secret-654321")
            .expect("other api key should save");
        let metadata = metadata_from_stored(&stored);

        let plan = build_delete_request(9, Some(&metadata), &vault)
            .expect("delete request should build from bound metadata");

        cleanup_delete_request(&plan, &vault).expect("cleanup should delete bound secret");
        assert!(vault.read(&stored.reference).is_err());
        assert_eq!(
            vault
                .read(&other.reference)
                .expect("unrelated secret should remain"),
            "sk-other-secret-654321"
        );

        let _ = fs::remove_dir_all(root);
    }

    /// 为本地 vault 测试生成唯一目录，避免不同测试之间互相污染。
    fn unique_test_dir(name: &str) -> std::path::PathBuf {
        std::env::temp_dir().join(format!("invest-compass-{name}-{}", std::process::id()))
    }

    /// 将测试中真实写入的 vault 凭据转换为 Go core 已绑定的安全元数据。
    fn metadata_from_stored(stored: &StoredCredential) -> AIConfigCredentialMetadata {
        AIConfigCredentialMetadata {
            api_key_ref: stored.reference.clone(),
            masked_api_key: stored.masked.clone(),
            has_api_key: true,
            timeout_seconds: 30,
        }
    }
}
