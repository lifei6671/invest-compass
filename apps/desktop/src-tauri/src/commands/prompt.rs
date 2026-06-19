use crate::sidecar::CoreState;
use serde::{Deserialize, Serialize};
use tauri::State;

#[derive(Serialize)]
struct PromptTemplateListRequest {}

#[derive(Serialize)]
struct PromptTemplateGetRequest {
    id: i64,
}

#[derive(Deserialize, Serialize)]
pub struct PromptTemplateCreatePayload {
    name: String,
    #[serde(rename = "type")]
    template_type: String,
    description: String,
    content: String,
}

#[derive(Deserialize, Serialize)]
pub struct PromptTemplateUpdatePayload {
    id: i64,
    name: String,
    #[serde(rename = "type")]
    template_type: String,
    description: String,
    content: String,
}

#[derive(Serialize)]
struct PromptTemplateDeleteRequest {
    id: i64,
}

/// 读取 Prompt 模板列表，固定转发到 Go core `/api/prompt-templates/list`。
#[tauri::command]
pub async fn prompt_templates_list(
    state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/prompt-templates/list", &PromptTemplateListRequest {})
        .map_err(|error| error.to_string())
}

/// 读取单个 Prompt 模板，固定转发到 Go core `/api/prompt-templates/get`。
#[tauri::command]
pub async fn prompt_templates_get(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_prompt_template_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api(
            "/api/prompt-templates/get",
            &PromptTemplateGetRequest { id },
        )
        .map_err(|error| error.to_string())
}

/// 创建 Prompt 模板，固定转发到 Go core `/api/prompt-templates/create`。
#[tauri::command]
pub async fn prompt_templates_create(
    state: State<'_, CoreState>,
    payload: PromptTemplateCreatePayload,
) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/prompt-templates/create", &payload)
        .map_err(|error| error.to_string())
}

/// 更新 Prompt 模板，固定转发到 Go core `/api/prompt-templates/update`。
#[tauri::command]
pub async fn prompt_templates_update(
    state: State<'_, CoreState>,
    payload: PromptTemplateUpdatePayload,
) -> Result<serde_json::Value, String> {
    validate_prompt_template_id(payload.id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/prompt-templates/update", &payload)
        .map_err(|error| error.to_string())
}

/// 删除 Prompt 模板，固定转发到 Go core `/api/prompt-templates/delete`。
#[tauri::command]
pub async fn prompt_templates_delete(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_prompt_template_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api(
            "/api/prompt-templates/delete",
            &PromptTemplateDeleteRequest { id },
        )
        .map_err(|error| error.to_string())
}

/// 校验 Prompt 模板 ID，避免 Rust command 转发无效模板请求。
fn validate_prompt_template_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("invalid prompt template id".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证 Prompt 模板 command 在 Rust 边界拒绝非正数 ID。
    fn validate_prompt_template_id_rejects_non_positive_values() {
        assert!(validate_prompt_template_id(1).is_ok());
        assert_eq!(
            validate_prompt_template_id(0).expect_err("zero template id should fail"),
            "invalid prompt template id"
        );
        assert_eq!(
            validate_prompt_template_id(-1).expect_err("negative template id should fail"),
            "invalid prompt template id"
        );
    }
}
