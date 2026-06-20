use crate::sidecar::{CoreState, SidecarError};
use serde::{Deserialize, Serialize};
use tauri::{Emitter, State, Window};

use super::ai_config::{resolve_ai_key_for_internal_request, resolve_ai_key_ref_for_config};

const ANALYSIS_TASK_EVENT: &str = "analysis-task-event";
const TASK_LIST_MAX_LIMIT: i32 = 100;
/// AI 分析任务只能读取 AI 配置 vault 引用，不能接受代理或任意文件引用。
const LOCAL_AI_CONFIG_VAULT_REF_PREFIX: &str = "local-vault://ai-config/";

#[derive(Serialize)]
struct TaskListRequest {
    limit: i32,
}

#[derive(Serialize)]
struct TaskGetRequest {
    task_id: String,
}

#[derive(Serialize)]
struct TaskEventsRequest {
    task_id: String,
    after_event_id: i64,
}

#[derive(Deserialize, Serialize)]
pub struct AnalysisTaskCreatePayload {
    symbol: String,
    analysis_type: String,
    ai_config_id: i64,
    api_key_ref: String,
    prompt_template_id: i64,
    user_position: Option<UserPositionPayload>,
}

#[derive(Deserialize, Serialize)]
struct UserPositionPayload {
    cost_price: f64,
    shares: f64,
    risk_level: String,
}

#[derive(Serialize)]
struct AnalysisTaskCreateRequest {
    symbol: String,
    analysis_type: String,
    ai_config_id: i64,
    prompt_template_id: i64,
    user_position: Option<UserPositionPayload>,
    resolved_api_key: String,
}

#[derive(Serialize)]
struct AnalysisTaskCancelRequest {
    task_id: String,
}

#[derive(Debug, Serialize)]
pub struct TaskStreamEvent {
    id: i64,
    event: String,
    data: serde_json::Value,
}

#[derive(Serialize)]
pub struct AnalysisTaskSubscribeResult {
    emitted: usize,
    last_event_id: i64,
}

/// 创建分析任务，固定读取本地 vault 后转发到 Go core `/api/analysis/tasks`。
#[tauri::command]
pub fn analysis_task_create(
    state: State<'_, CoreState>,
    payload: AnalysisTaskCreatePayload,
) -> Result<serde_json::Value, String> {
    validate_analysis_task_create_payload(&payload)?;
    let client = state.client().map_err(|error| error.to_string())?;
    let api_key_ref =
        resolve_ai_key_ref_for_config(&client, payload.ai_config_id, Some(&payload.api_key_ref))?;
    let resolved_api_key = resolve_ai_key_for_internal_request(&api_key_ref)?;
    let request = build_analysis_task_create_request(payload, resolved_api_key)?;
    client
        .post_api("/api/analysis/tasks", &request)
        .map_err(|error| error.to_string())
}

/// 取消分析任务，固定转发到 Go core `/api/tasks/cancel`。
#[tauri::command]
pub fn analysis_task_cancel(
    state: State<'_, CoreState>,
    task_id: String,
) -> Result<serde_json::Value, String> {
    validate_task_id(&task_id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/tasks/cancel", &AnalysisTaskCancelRequest { task_id })
        .map_err(|error| error.to_string())
}

/// 订阅分析任务事件，固定读取 Go core SSE 后逐帧通过 Tauri event 分发。
#[tauri::command]
pub fn analysis_task_subscribe(
    window: Window,
    state: State<'_, CoreState>,
    task_id: String,
    after_event_id: i64,
) -> Result<AnalysisTaskSubscribeResult, String> {
    validate_task_id(&task_id)?;
    validate_after_event_id(after_event_id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    let mut last_event_id = after_event_id;
    let emitted = client
        .post_sse(
            "/api/tasks/events/stream",
            &TaskEventsRequest {
                task_id,
                after_event_id,
            },
            |frame| {
                let event =
                    parse_task_stream_event(frame).map_err(SidecarError::SseFrameHandler)?;
                last_event_id = event.id;
                window
                    .emit(ANALYSIS_TASK_EVENT, &event)
                    .map_err(|error| SidecarError::SseFrameHandler(error.to_string()))?;
                Ok(())
            },
        )
        .map_err(|error| error.to_string())?;
    Ok(AnalysisTaskSubscribeResult {
        emitted,
        last_event_id,
    })
}

/// 读取任务历史列表，固定转发到 Go core `/api/tasks/list`。
#[tauri::command]
pub fn task_list(state: State<'_, CoreState>, limit: i32) -> Result<serde_json::Value, String> {
    validate_task_list_limit(limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/tasks/list", &TaskListRequest { limit })
        .map_err(|error| error.to_string())
}

/// 读取任务详情，固定转发到 Go core `/api/tasks/get`。
#[tauri::command]
pub fn task_get(state: State<'_, CoreState>, task_id: String) -> Result<serde_json::Value, String> {
    validate_task_id(&task_id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/tasks/get", &TaskGetRequest { task_id })
        .map_err(|error| error.to_string())
}

/// 增量读取任务事件，固定转发到 Go core `/api/tasks/events`。
#[tauri::command]
pub fn task_events(
    state: State<'_, CoreState>,
    task_id: String,
    after_event_id: i64,
) -> Result<serde_json::Value, String> {
    validate_task_id(&task_id)?;
    validate_after_event_id(after_event_id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api(
            "/api/tasks/events",
            &TaskEventsRequest {
                task_id,
                after_event_id,
            },
        )
        .map_err(|error| error.to_string())
}

/// 构造分析任务创建内部请求，安全凭据引用不进入 Go core 请求体。
fn build_analysis_task_create_request(
    payload: AnalysisTaskCreatePayload,
    resolved_api_key: String,
) -> Result<AnalysisTaskCreateRequest, String> {
    validate_analysis_task_create_payload(&payload)?;
    if resolved_api_key.trim().is_empty() {
        return Err("credential secret is empty".to_string());
    }
    Ok(AnalysisTaskCreateRequest {
        symbol: payload.symbol,
        analysis_type: payload.analysis_type,
        ai_config_id: payload.ai_config_id,
        prompt_template_id: payload.prompt_template_id,
        user_position: payload.user_position,
        resolved_api_key,
    })
}

/// 校验分析任务创建的本地边界，避免非法配置 ID 触发 vault 读取或转发到 Go core。
fn validate_analysis_task_create_payload(
    payload: &AnalysisTaskCreatePayload,
) -> Result<(), String> {
    if payload.symbol.trim().is_empty() {
        return Err("invalid symbol".to_string());
    }
    if payload.analysis_type.trim().is_empty() {
        return Err("invalid analysis_type".to_string());
    }
    if payload.ai_config_id <= 0 {
        return Err("invalid ai_config_id".to_string());
    }
    if payload.prompt_template_id <= 0 {
        return Err("invalid prompt_template_id".to_string());
    }
    let api_key_ref = payload.api_key_ref.trim();
    if api_key_ref.is_empty() || !api_key_ref.starts_with(LOCAL_AI_CONFIG_VAULT_REF_PREFIX) {
        return Err("invalid api_key_ref".to_string());
    }
    Ok(())
}

/// 校验任务列表分页边界，避免 Rust command 转发无界查询到 Go core。
fn validate_task_list_limit(limit: i32) -> Result<(), String> {
    if limit <= 0 || limit > TASK_LIST_MAX_LIMIT {
        return Err("invalid task list limit".to_string());
    }
    Ok(())
}

/// 校验任务 ID，空 ID 没有合法查询、取消或事件订阅语义。
fn validate_task_id(task_id: &str) -> Result<(), String> {
    if task_id.trim().is_empty() {
        return Err("invalid task_id".to_string());
    }
    Ok(())
}

/// 校验任务事件回放游标，0 表示从头补拉，负数直接在 Rust 边界失败。
fn validate_after_event_id(after_event_id: i64) -> Result<(), String> {
    if after_event_id < 0 {
        return Err("invalid after_event_id".to_string());
    }
    Ok(())
}

/// 解析 Go core 返回的 SSE 帧文本，转为 Tauri event payload。
#[cfg(test)]
fn parse_task_stream_events(text: &str) -> Result<Vec<TaskStreamEvent>, String> {
    // SSE 协议允许不同 HTTP 栈使用 CRLF；Rust 转发层先统一行结束符，再按空行分帧。
    let normalized_text = text.replace("\r\n", "\n").replace('\r', "\n");
    let mut events = Vec::new();
    for frame in normalized_text.split("\n\n") {
        if frame.trim().is_empty() {
            continue;
        }
        events.push(parse_task_stream_event(frame)?);
    }
    Ok(events)
}

/// 解析单个 SSE 事件帧，严格要求 id、event 和 JSON data。
fn parse_task_stream_event(frame: &str) -> Result<TaskStreamEvent, String> {
    let mut id = None;
    let mut event = None;
    let mut data_lines = Vec::new();

    for line in frame.lines() {
        if line.starts_with(':') {
            continue;
        }
        if let Some(value) = line.strip_prefix("id:") {
            id = Some(
                value
                    .trim()
                    .parse::<i64>()
                    .map_err(|error| format!("invalid sse id: {error}"))?,
            );
            continue;
        }
        if let Some(value) = line.strip_prefix("event:") {
            event = Some(value.trim().to_string());
            continue;
        }
        if let Some(value) = line.strip_prefix("data:") {
            data_lines.push(value.trim_start());
        }
    }

    let raw_data = data_lines.join("\n");
    let data = serde_json::from_str::<serde_json::Value>(&raw_data)
        .map_err(|error| format!("invalid sse data json: {error}"))?;
    Ok(TaskStreamEvent {
        id: id.ok_or_else(|| "missing sse id".to_string())?,
        event: event.ok_or_else(|| "missing sse event".to_string())?,
        data,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证分析任务内部请求只包含运行期密钥，不把 vault 引用转发给 Go core。
    fn build_analysis_task_create_request_strips_api_key_ref() {
        let request = build_analysis_task_create_request(
            AnalysisTaskCreatePayload {
                symbol: "US:AAPL".to_string(),
                analysis_type: "stock_full".to_string(),
                ai_config_id: 7,
                api_key_ref: "local-vault://ai-config/openai-compatible-7".to_string(),
                prompt_template_id: 9,
                user_position: Some(UserPositionPayload {
                    cost_price: 123.45,
                    shares: 10.0,
                    risk_level: "medium".to_string(),
                }),
            },
            "sk-runtime-secret".to_string(),
        )
        .expect("request should build");
        let encoded = serde_json::to_string(&request).expect("request should serialize");

        assert!(encoded.contains("\"resolved_api_key\":\"sk-runtime-secret\""));
        assert!(!encoded.contains("api_key_ref"));
        assert!(!encoded.contains("local-vault://"));
    }

    #[test]
    /// 验证任务列表 command 在 Rust 边界拒绝无界 limit。
    fn validate_task_list_limit_rejects_unbounded_values() {
        assert!(validate_task_list_limit(1).is_ok());
        assert!(validate_task_list_limit(100).is_ok());
        assert_eq!(
            validate_task_list_limit(0).expect_err("zero limit should fail"),
            "invalid task list limit"
        );
        assert_eq!(
            validate_task_list_limit(101).expect_err("too large limit should fail"),
            "invalid task list limit"
        );
    }

    #[test]
    /// 验证事件回放 command 保留 0 游标并拒绝负数游标。
    fn validate_after_event_id_rejects_negative_cursor() {
        assert!(validate_after_event_id(0).is_ok());
        assert!(validate_after_event_id(1).is_ok());
        assert_eq!(
            validate_after_event_id(-1).expect_err("negative cursor should fail"),
            "invalid after_event_id"
        );
    }

    #[test]
    /// 验证任务详情、取消和事件订阅在 Rust 边界拒绝空任务 ID。
    fn validate_task_id_rejects_blank_text() {
        assert!(validate_task_id("analysis-request-1").is_ok());
        assert_eq!(
            validate_task_id(" \t\n").expect_err("blank task id should fail"),
            "invalid task_id"
        );
    }

    #[test]
    /// 验证分析任务创建在读取 vault 前先拒绝非法配置 ID。
    fn validate_analysis_task_create_payload_rejects_non_positive_ids() {
        let mut payload = AnalysisTaskCreatePayload {
            symbol: "US:AAPL".to_string(),
            analysis_type: "stock_full".to_string(),
            ai_config_id: 7,
            api_key_ref: "local-vault://ai-config/openai-compatible-7".to_string(),
            prompt_template_id: 9,
            user_position: None,
        };

        payload.ai_config_id = 0;
        assert_eq!(
            validate_analysis_task_create_payload(&payload)
                .expect_err("missing ai config id should fail"),
            "invalid ai_config_id"
        );

        payload.ai_config_id = 7;
        payload.prompt_template_id = 0;
        assert_eq!(
            validate_analysis_task_create_payload(&payload)
                .expect_err("missing prompt template id should fail"),
            "invalid prompt_template_id"
        );
    }

    #[test]
    /// 验证分析任务创建在 Rust 边界拒绝空基础字段，避免无意义请求读取 vault。
    fn validate_analysis_task_create_payload_rejects_blank_core_fields() {
        let mut payload = AnalysisTaskCreatePayload {
            symbol: "US:AAPL".to_string(),
            analysis_type: "stock_full".to_string(),
            ai_config_id: 7,
            api_key_ref: "local-vault://ai-config/openai-compatible-7".to_string(),
            prompt_template_id: 9,
            user_position: None,
        };

        payload.symbol = " ".to_string();
        assert_eq!(
            validate_analysis_task_create_payload(&payload).expect_err("blank symbol should fail"),
            "invalid symbol"
        );

        payload.symbol = "US:AAPL".to_string();
        payload.analysis_type = " ".to_string();
        assert_eq!(
            validate_analysis_task_create_payload(&payload)
                .expect_err("blank analysis type should fail"),
            "invalid analysis_type"
        );

        payload.analysis_type = "stock_full".to_string();
        payload.api_key_ref = " ".to_string();
        assert_eq!(
            validate_analysis_task_create_payload(&payload)
                .expect_err("blank api key ref should fail"),
            "invalid api_key_ref"
        );
    }

    #[test]
    /// 验证分析任务创建只接受 AI 配置本地 vault 引用，避免任意引用触发凭据读取。
    fn validate_analysis_task_create_payload_rejects_invalid_api_key_ref_scheme() {
        let mut payload = AnalysisTaskCreatePayload {
            symbol: "US:AAPL".to_string(),
            analysis_type: "stock_full".to_string(),
            ai_config_id: 7,
            api_key_ref: "local-vault://ai-config/openai-compatible-7".to_string(),
            prompt_template_id: 9,
            user_position: None,
        };

        for api_key_ref in [
            "local-vault://proxy/default",
            "file:///tmp/plain-secret",
            "plain-reference",
        ] {
            payload.api_key_ref = api_key_ref.to_string();
            assert_eq!(
                validate_analysis_task_create_payload(&payload)
                    .expect_err("invalid api key ref scheme should fail"),
                "invalid api_key_ref"
            );
        }
    }

    #[test]
    /// 验证 Go core SSE 帧会被解析为可通过 Tauri event 转发的结构化事件。
    fn parse_task_stream_events_decodes_sse_frames() {
        let events = parse_task_stream_events(
            "id: 2\nevent: TASK_CHUNK\ndata: {\"content\":\"第一行\\n第二行\"}\n\nid: 3\nevent: TASK_SUCCESS\ndata: {\ndata: \"progress\":100\ndata: }\n\n",
        )
        .expect("sse frames should parse");

        assert_eq!(events.len(), 2);
        assert_eq!(events[0].id, 2);
        assert_eq!(events[0].event, "TASK_CHUNK");
        assert_eq!(events[0].data["content"], "第一行\n第二行");
        assert_eq!(events[1].id, 3);
        assert_eq!(events[1].data["progress"], 100);
    }

    #[test]
    /// 验证 Rust SSE 转发兼容 CRLF 帧边界，避免不同 HTTP 栈换行风格导致任务事件丢失。
    fn parse_task_stream_events_accepts_crlf_frame_boundaries() {
        let events = parse_task_stream_events(
            "id: 8\r\nevent: TASK_LOG\r\ndata: {\"message\":\"ready\"}\r\n\r\nid: 9\r\nevent: TASK_SUCCESS\r\ndata: {\"progress\":100}\r\n\r\n",
        )
        .expect("crlf sse frames should parse");

        assert_eq!(events.len(), 2);
        assert_eq!(events[0].id, 8);
        assert_eq!(events[0].event, "TASK_LOG");
        assert_eq!(events[0].data["message"], "ready");
        assert_eq!(events[1].id, 9);
        assert_eq!(events[1].event, "TASK_SUCCESS");
        assert_eq!(events[1].data["progress"], 100);
    }
}
