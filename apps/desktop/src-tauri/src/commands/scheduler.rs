use crate::sidecar::CoreState;
use serde::{Deserialize, Serialize};
use tauri::State;

#[derive(Serialize)]
struct EmptyRequest {}

#[derive(Deserialize, Serialize)]
pub struct SchedulerJobPayload {
    id: i64,
    name: String,
    cron_type: String,
    cron_expr: String,
    enabled: bool,
    market: String,
    timezone: String,
    trade_window: String,
    scope_json: String,
    params_json: String,
    catchup_enabled: bool,
    catchup_max_days: i32,
    timeout_seconds: i32,
}

#[derive(Serialize)]
struct SchedulerSetEnabledRequest {
    id: i64,
    enabled: bool,
}

#[derive(Serialize)]
struct SchedulerIDRequest {
    id: i64,
}

#[derive(Deserialize, Serialize)]
pub struct SchedulerRunNowPayload {
    id: i64,
    target_date: String,
    ignore_trade_window: bool,
}

#[derive(Deserialize, Serialize)]
pub struct SchedulerBackfillPayload {
    id: i64,
    #[serde(rename = "dateFrom")]
    date_from: String,
    #[serde(rename = "dateTo")]
    date_to: String,
    symbols: Vec<String>,
}

#[derive(Serialize)]
struct SchedulerRunsListRequest {
    job_id: i64,
    limit: i32,
}

#[derive(Deserialize, Serialize)]
pub struct SchedulerTriggerPayload {
    job_id: i64,
    cron_type: String,
    scope_key: String,
    target_date: String,
}

#[derive(Deserialize, Serialize)]
pub struct SchedulerRefreshSymbolPayload {
    symbol: String,
    data_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    target_date: Option<String>,
    period: Option<String>,
    adjust: Option<String>,
    limit: Option<i32>,
}

/// 读取调度任务列表，固定转发到 Go core `/api/scheduler/jobs/list`。
#[tauri::command]
pub fn scheduler_jobs_list(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/list", &EmptyRequest {})
        .map_err(|error| error.to_string())
}

/// 读取单个调度任务，固定转发到 Go core `/api/scheduler/jobs/get`。
#[tauri::command]
pub fn scheduler_jobs_get(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_scheduler_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/get", &SchedulerIDRequest { id })
        .map_err(|error| error.to_string())
}

/// 读取调度任务类型元数据，固定转发到 Go core `/api/scheduler/job-types`。
#[tauri::command]
pub fn scheduler_job_types(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/job-types", &EmptyRequest {})
        .map_err(|error| error.to_string())
}

/// 保存调度任务配置，固定转发到 Go core `/api/scheduler/jobs/save`。
#[tauri::command]
pub fn scheduler_jobs_save(
    state: State<'_, CoreState>,
    payload: SchedulerJobPayload,
) -> Result<serde_json::Value, String> {
    if payload.id > 0 {
        validate_scheduler_id(payload.id)?;
    }
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/save", &payload)
        .map_err(|error| error.to_string())
}

/// 启用或停用调度任务，固定转发到 Go core `/api/scheduler/jobs/set-enabled`。
#[tauri::command]
pub fn scheduler_jobs_set_enabled(
    state: State<'_, CoreState>,
    id: i64,
    enabled: bool,
) -> Result<serde_json::Value, String> {
    validate_scheduler_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api(
            "/api/scheduler/jobs/set-enabled",
            &SchedulerSetEnabledRequest { id, enabled },
        )
        .map_err(|error| error.to_string())
}

/// 软删除调度任务配置，固定转发到 Go core `/api/scheduler/jobs/delete`。
#[tauri::command]
pub fn scheduler_jobs_delete(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_scheduler_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/delete", &SchedulerIDRequest { id })
        .map_err(|error| error.to_string())
}

/// 立即执行一次调度任务，固定转发到 Go core `/api/scheduler/jobs/run-now`。
#[tauri::command]
pub fn scheduler_jobs_run_now(
    state: State<'_, CoreState>,
    payload: SchedulerRunNowPayload,
) -> Result<serde_json::Value, String> {
    validate_scheduler_id(payload.id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/run-now", &payload)
        .map_err(|error| error.to_string())
}

/// 手动补偿调度任务，固定转发到 Go core `/api/scheduler/jobs/backfill`。
#[tauri::command]
pub fn scheduler_jobs_backfill(
    state: State<'_, CoreState>,
    payload: SchedulerBackfillPayload,
) -> Result<serde_json::Value, String> {
    validate_backfill_payload(&payload)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/jobs/backfill", &payload)
        .map_err(|error| error.to_string())
}

/// 读取调度执行记录，固定转发到 Go core `/api/scheduler/runs/list`。
#[tauri::command]
pub fn scheduler_runs_list(
    state: State<'_, CoreState>,
    job_id: i64,
    limit: i32,
) -> Result<serde_json::Value, String> {
    validate_scheduler_run_list_params(job_id, limit)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api(
            "/api/scheduler/runs/list",
            &SchedulerRunsListRequest { job_id, limit },
        )
        .map_err(|error| error.to_string())
}

/// 读取单条调度执行记录，固定转发到 Go core `/api/scheduler/runs/get`。
#[tauri::command]
pub fn scheduler_runs_get(
    state: State<'_, CoreState>,
    id: i64,
) -> Result<serde_json::Value, String> {
    validate_scheduler_id(id)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/runs/get", &SchedulerIDRequest { id })
        .map_err(|error| error.to_string())
}

/// 手动触发一次数据抓取，固定转发到 Go core `/api/scheduler/runs/trigger` 并复用调度队列。
#[tauri::command]
pub fn scheduler_runs_trigger(
    state: State<'_, CoreState>,
    payload: SchedulerTriggerPayload,
) -> Result<serde_json::Value, String> {
    validate_trigger_payload(&payload)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/runs/trigger", &payload)
        .map_err(|error| error.to_string())
}

/// 手动刷新某只股票的数据，固定转发到 Go core `/api/scheduler/refresh-symbol` 并复用调度队列。
#[tauri::command]
pub fn scheduler_refresh_symbol(
    state: State<'_, CoreState>,
    payload: SchedulerRefreshSymbolPayload,
) -> Result<serde_json::Value, String> {
    validate_refresh_symbol_payload(&payload)?;
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/refresh-symbol", &payload)
        .map_err(|error| error.to_string())
}

/// 读取调度系统摘要状态，固定转发到 Go core `/api/scheduler/status`。
#[tauri::command]
pub fn scheduler_status(state: State<'_, CoreState>) -> Result<serde_json::Value, String> {
    let client = state.client().map_err(|error| error.to_string())?;
    client
        .post_api("/api/scheduler/status", &EmptyRequest {})
        .map_err(|error| error.to_string())
}

/// validate_scheduler_id 在 Rust command 边界拒绝非法调度 ID，避免无效请求进入 Go core。
fn validate_scheduler_id(id: i64) -> Result<(), String> {
    if id <= 0 {
        return Err("scheduler id must be positive".to_string());
    }
    Ok(())
}

/// validate_refresh_symbol_payload 校验单股强制刷新请求的必要字段。
fn validate_refresh_symbol_payload(payload: &SchedulerRefreshSymbolPayload) -> Result<(), String> {
    if payload.symbol.trim().is_empty() {
        return Err("scheduler refresh symbol must not be blank".to_string());
    }
    if payload.data_type.trim().is_empty() {
        return Err("scheduler refresh data_type must not be blank".to_string());
    }
    if payload
        .target_date
        .as_deref()
        .is_some_and(|target_date| target_date.trim().is_empty())
    {
        return Err("scheduler refresh target_date must not be blank".to_string());
    }
    Ok(())
}

/// validate_scheduler_run_list_params 校验执行记录列表查询范围，避免无界查询和非法任务 ID。
fn validate_scheduler_run_list_params(job_id: i64, limit: i32) -> Result<(), String> {
    if job_id < 0 {
        return Err("scheduler job id must be non-negative".to_string());
    }
    if !(1..=100).contains(&limit) {
        return Err("scheduler run list limit must be between 1 and 100".to_string());
    }
    Ok(())
}

/// validate_backfill_payload 校验手动补偿请求边界，避免非法范围或过大请求进入 Go core。
fn validate_backfill_payload(payload: &SchedulerBackfillPayload) -> Result<(), String> {
    validate_scheduler_id(payload.id)?;
    if payload.date_from.trim().is_empty() {
        return Err("scheduler backfill dateFrom must not be blank".to_string());
    }
    if payload.date_to.trim().is_empty() {
        return Err("scheduler backfill dateTo must not be blank".to_string());
    }
    let date_from = parse_backfill_date(&payload.date_from, "dateFrom")?;
    let date_to = parse_backfill_date(&payload.date_to, "dateTo")?;
    if date_from > date_to {
        return Err("scheduler backfill dateFrom must not be after dateTo".to_string());
    }
    if date_to - date_from + 1 > 30 {
        return Err("scheduler backfill date range must not exceed 30 days".to_string());
    }
    if payload.symbols.len() > 30 {
        return Err("scheduler backfill symbols must not exceed 30 items".to_string());
    }
    if payload
        .symbols
        .iter()
        .any(|symbol| symbol.trim().is_empty())
    {
        return Err("scheduler backfill symbols must not contain blank item".to_string());
    }
    Ok(())
}

/// parse_backfill_date 解析 YYYY-MM-DD 并返回可比较的自然日序号。
fn parse_backfill_date(value: &str, field: &str) -> Result<i32, String> {
    let trimmed = value.trim();
    let invalid = || format!("scheduler backfill {field} must use YYYY-MM-DD");
    if trimmed.len() != 10 {
        return Err(invalid());
    }
    let bytes = trimmed.as_bytes();
    if bytes[4] != b'-' || bytes[7] != b'-' {
        return Err(invalid());
    }
    if !bytes[0..4].iter().all(u8::is_ascii_digit)
        || !bytes[5..7].iter().all(u8::is_ascii_digit)
        || !bytes[8..10].iter().all(u8::is_ascii_digit)
    {
        return Err(invalid());
    }
    let year = trimmed[0..4].parse::<i32>().map_err(|_| invalid())?;
    let month = trimmed[5..7].parse::<u32>().map_err(|_| invalid())?;
    let day = trimmed[8..10].parse::<u32>().map_err(|_| invalid())?;
    if year <= 0 || !(1..=12).contains(&month) {
        return Err(invalid());
    }
    let month_days = days_in_month(year, month);
    if day == 0 || day > month_days {
        return Err(invalid());
    }
    Ok(days_before_year(year) + days_before_month(year, month) + day as i32)
}

/// days_before_year 返回公历中当前年份开始前的累计天数。
fn days_before_year(year: i32) -> i32 {
    let previous_year = year - 1;
    previous_year * 365 - previous_year / 100 + previous_year / 4 + previous_year / 400
}

/// days_before_month 返回当前年份内指定月份开始前的累计天数。
fn days_before_month(year: i32, month: u32) -> i32 {
    let mut days = 0;
    for current_month in 1..month {
        days += days_in_month(year, current_month) as i32;
    }
    days
}

/// days_in_month 返回指定月份的天数，并处理闰年二月。
fn days_in_month(year: i32, month: u32) -> u32 {
    match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        2 => 28,
        _ => 0,
    }
}

/// is_leap_year 判断公历闰年，供本地日期边界校验使用。
fn is_leap_year(year: i32) -> bool {
    year % 4 == 0 && (year % 100 != 0 || year % 400 == 0)
}

/// validate_trigger_payload 校验手动触发请求的业务标识字段。
fn validate_trigger_payload(payload: &SchedulerTriggerPayload) -> Result<(), String> {
    if payload.job_id < 0 {
        return Err("scheduler job id must be non-negative".to_string());
    }
    if payload.cron_type.trim().is_empty() {
        return Err("scheduler trigger cron_type must not be blank".to_string());
    }
    if payload.scope_key.trim().is_empty() {
        return Err("scheduler trigger scope_key must not be blank".to_string());
    }
    if payload.target_date.trim().is_empty() {
        return Err("scheduler trigger target_date must not be blank".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证调度 ID 校验会拒绝 0 和负数，避免无效 ID 进入 Go core。
    fn validate_scheduler_id_rejects_non_positive_values() {
        assert!(validate_scheduler_id(1).is_ok());
        assert_eq!(
            validate_scheduler_id(0).expect_err("zero scheduler id should fail"),
            "scheduler id must be positive"
        );
        assert_eq!(
            validate_scheduler_id(-1).expect_err("negative scheduler id should fail"),
            "scheduler id must be positive"
        );
    }

    #[test]
    /// 验证单股刷新校验会拒绝空白必要字段。
    fn validate_refresh_symbol_payload_rejects_blank_required_fields() {
        let payload = SchedulerRefreshSymbolPayload {
            symbol: "CN:SH:600519".to_string(),
            data_type: "quote".to_string(),
            target_date: Some("2026-06-19".to_string()),
            period: None,
            adjust: None,
            limit: None,
        };
        assert!(validate_refresh_symbol_payload(&payload).is_ok());

        let blank_symbol = SchedulerRefreshSymbolPayload {
            symbol: " \t\n".to_string(),
            data_type: payload.data_type.clone(),
            target_date: payload.target_date.clone(),
            period: None,
            adjust: None,
            limit: None,
        };
        assert_eq!(
            validate_refresh_symbol_payload(&blank_symbol).expect_err("blank symbol should fail"),
            "scheduler refresh symbol must not be blank"
        );

        let blank_data_type = SchedulerRefreshSymbolPayload {
            symbol: payload.symbol.clone(),
            data_type: " ".to_string(),
            target_date: payload.target_date.clone(),
            period: None,
            adjust: None,
            limit: None,
        };
        assert_eq!(
            validate_refresh_symbol_payload(&blank_data_type)
                .expect_err("blank data type should fail"),
            "scheduler refresh data_type must not be blank"
        );
    }

    #[test]
    /// 验证单股刷新允许省略 target_date，由 Go core 按当前交易日兜底。
    fn validate_refresh_symbol_payload_allows_omitted_target_date() {
        let payload: SchedulerRefreshSymbolPayload = serde_json::from_value(serde_json::json!({
            "symbol": "CN:SH:600519",
            "data_type": "quote"
        }))
        .expect("refresh payload without target_date should deserialize");

        assert!(validate_refresh_symbol_payload(&payload).is_ok());
    }

    #[test]
    /// 验证执行记录列表 limit 必须在本地边界内，避免无界查询进入 Go core。
    fn validate_scheduler_run_list_limit_rejects_unbounded_values() {
        assert!(validate_scheduler_run_list_params(0, 1).is_ok());
        assert!(validate_scheduler_run_list_params(7, 100).is_ok());
        assert_eq!(
            validate_scheduler_run_list_params(0, 0).expect_err("zero limit should fail"),
            "scheduler run list limit must be between 1 and 100"
        );
        assert_eq!(
            validate_scheduler_run_list_params(0, 101).expect_err("too large limit should fail"),
            "scheduler run list limit must be between 1 and 100"
        );
        assert_eq!(
            validate_scheduler_run_list_params(-1, 20).expect_err("negative job id should fail"),
            "scheduler job id must be non-negative"
        );
    }

    #[test]
    /// 验证手动补偿 payload 会拒绝空日期范围和空白 symbol。
    fn validate_backfill_payload_rejects_blank_required_fields() {
        let payload = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-19".to_string(),
            date_to: "2026-06-19".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
        };
        assert!(validate_backfill_payload(&payload).is_ok());

        let blank_date = SchedulerBackfillPayload {
            id: payload.id,
            date_from: " ".to_string(),
            date_to: payload.date_to.clone(),
            symbols: payload.symbols.clone(),
        };
        assert_eq!(
            validate_backfill_payload(&blank_date).expect_err("blank date_from should fail"),
            "scheduler backfill dateFrom must not be blank"
        );

        let blank_symbol = SchedulerBackfillPayload {
            id: payload.id,
            date_from: payload.date_from,
            date_to: payload.date_to,
            symbols: vec!["CN:SH:600519".to_string(), " \t".to_string()],
        };
        assert_eq!(
            validate_backfill_payload(&blank_symbol).expect_err("blank symbol should fail"),
            "scheduler backfill symbols must not contain blank item"
        );
    }

    #[test]
    /// 验证手动补偿日期必须使用合法 YYYY-MM-DD 格式，避免非法日期进入 Go core。
    fn validate_backfill_payload_rejects_invalid_date_format() {
        let payload = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-19".to_string(),
            date_to: "2026-06-19".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
        };
        assert!(validate_backfill_payload(&payload).is_ok());

        let slash_date = SchedulerBackfillPayload {
            id: payload.id,
            date_from: "2026/06/19".to_string(),
            date_to: payload.date_to.clone(),
            symbols: payload.symbols.clone(),
        };
        assert_eq!(
            validate_backfill_payload(&slash_date).expect_err("slash date should fail"),
            "scheduler backfill dateFrom must use YYYY-MM-DD"
        );

        let invalid_calendar_date = SchedulerBackfillPayload {
            id: payload.id,
            date_from: payload.date_from,
            date_to: "2026-02-30".to_string(),
            symbols: payload.symbols,
        };
        assert_eq!(
            validate_backfill_payload(&invalid_calendar_date)
                .expect_err("invalid calendar date should fail"),
            "scheduler backfill dateTo must use YYYY-MM-DD"
        );
    }

    #[test]
    /// 验证手动补偿会拒绝倒置日期和超过 30 个自然日的请求。
    fn validate_backfill_payload_rejects_invalid_date_range() {
        let reversed = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-20".to_string(),
            date_to: "2026-06-19".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
        };
        assert_eq!(
            validate_backfill_payload(&reversed).expect_err("reversed date range should fail"),
            "scheduler backfill dateFrom must not be after dateTo"
        );

        let thirty_days = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-01".to_string(),
            date_to: "2026-06-30".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
        };
        assert!(validate_backfill_payload(&thirty_days).is_ok());

        let thirty_one_days = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-01".to_string(),
            date_to: "2026-07-01".to_string(),
            symbols: vec!["CN:SH:600519".to_string()],
        };
        assert_eq!(
            validate_backfill_payload(&thirty_one_days).expect_err("31-day date range should fail"),
            "scheduler backfill date range must not exceed 30 days"
        );
    }

    #[test]
    /// 验证手动补偿允许空 symbol 复用任务 scope，同时仍限制显式 symbol 数量上限。
    fn validate_backfill_payload_allows_empty_symbols_and_rejects_too_many_symbols() {
        let no_symbol = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-19".to_string(),
            date_to: "2026-06-19".to_string(),
            symbols: Vec::new(),
        };
        assert!(validate_backfill_payload(&no_symbol).is_ok());

        let too_many_symbols = SchedulerBackfillPayload {
            id: 7,
            date_from: "2026-06-19".to_string(),
            date_to: "2026-06-19".to_string(),
            symbols: (0..31).map(|index| format!("CN:SH:{index:06}")).collect(),
        };
        assert_eq!(
            validate_backfill_payload(&too_many_symbols).expect_err("too many symbols should fail"),
            "scheduler backfill symbols must not exceed 30 items"
        );
    }

    #[test]
    /// 验证手动触发 payload 会拒绝空白业务标识。
    fn validate_trigger_payload_rejects_blank_required_fields() {
        let payload = SchedulerTriggerPayload {
            job_id: 0,
            cron_type: "cn_a_share_quote_refresh".to_string(),
            scope_key: "CN:SH:600519".to_string(),
            target_date: "2026-06-19".to_string(),
        };
        assert!(validate_trigger_payload(&payload).is_ok());

        let blank_cron_type = SchedulerTriggerPayload {
            job_id: 0,
            cron_type: " ".to_string(),
            scope_key: payload.scope_key.clone(),
            target_date: payload.target_date.clone(),
        };
        assert_eq!(
            validate_trigger_payload(&blank_cron_type).expect_err("blank cron_type should fail"),
            "scheduler trigger cron_type must not be blank"
        );

        let blank_target_date = SchedulerTriggerPayload {
            job_id: 0,
            cron_type: payload.cron_type,
            scope_key: payload.scope_key,
            target_date: "\n".to_string(),
        };
        assert_eq!(
            validate_trigger_payload(&blank_target_date)
                .expect_err("blank target_date should fail"),
            "scheduler trigger target_date must not be blank"
        );
    }
}
