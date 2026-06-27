use crate::{
    desktop_runtime,
    sidecar::{runtime_core_binary_path, CoreClient, CoreState},
};
use serde::{Deserialize, Serialize};
use std::{
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Mutex, OnceLock,
    },
    thread,
    time::Duration,
};
use tauri::{AppHandle, Manager, State};

const BOOT_TASK_ID: &str = "app-boot-initialization";
const SEARCH_REBUILD_TIMEOUT: Duration = Duration::from_secs(120);
const BOOT_STAGE_VISIBLE_DURATION: Duration = Duration::from_millis(500);
static BOOT_SEARCH_REBUILD_IN_FLIGHT: AtomicBool = AtomicBool::new(false);
static BOOT_RUNTIME: OnceLock<Mutex<BootRuntime>> = OnceLock::new();

struct BootRuntime {
    running: bool,
    status: BootStatusData,
}

#[derive(Deserialize)]
struct CoreEnvelope<T> {
    code: i32,
    message: String,
    data: T,
}

#[derive(Serialize)]
struct EmptyRequest {}

#[derive(Serialize)]
struct SearchRebuildRequest {
    scope: String,
}

#[derive(Clone, Debug, Default, Deserialize)]
struct SearchStatusData {
    #[serde(default)]
    fts5_status: String,
    #[serde(default)]
    gse_status: String,
    #[serde(default)]
    search_status: String,
    #[serde(default)]
    tokenizer_name: String,
}

#[derive(Clone, Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CoreHealthData {
    #[serde(default)]
    db_status: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
enum BootStepStatus {
    Completed,
    Running,
    Pending,
    Failed,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootStep {
    id: String,
    index: u8,
    title: String,
    status: BootStepStatus,
    badge_text: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootTaskDetail {
    task_id: String,
    elapsed: String,
    current_stage: String,
    remaining: String,
}

#[derive(Clone, Copy, Debug, Serialize)]
#[serde(rename_all = "lowercase")]
enum BootLogStatus {
    Success,
    Running,
    Error,
}

#[derive(Clone, Debug, Serialize)]
struct BootLogItem {
    id: String,
    time: String,
    status: BootLogStatus,
    message: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct BootStatusData {
    ready: bool,
    progress: u8,
    current_step_id: String,
    steps: Vec<BootStep>,
    task_detail: BootTaskDetail,
    logs: Vec<BootLogItem>,
}

/// 读取应用启动初始化状态；该命令是前端启动页唯一状态源，不暴露任意 Go API 路径。
#[tauri::command]
pub async fn app_boot_status(
    app_handle: AppHandle,
    _state: State<'_, CoreState>,
) -> Result<serde_json::Value, String> {
    let binary_path = runtime_core_binary_path();
    let workspace_path = match desktop_runtime::default_workspace_path(&app_handle) {
        Ok(path) => path,
        Err(error) => {
            let failed = failed_boot_status(
                "检查本地工作区与目录权限",
                &sanitize_boot_message(&error.to_string()),
            );
            update_boot_status(failed.clone());
            return Ok(wrap_boot_status(failed));
        }
    };

    if !binary_path.exists() {
        let failed = failed_boot_status("启动 Go Core Sidecar", "core binary is unavailable");
        update_boot_status(failed.clone());
        return Ok(wrap_boot_status(failed));
    }

    let status = ensure_boot_worker_started(app_handle, binary_path, workspace_path);
    Ok(wrap_boot_status(status))
}

/// 获取启动运行态容器；状态保存在 Tauri 进程内，供前端轮询读取。
fn boot_runtime() -> &'static Mutex<BootRuntime> {
    BOOT_RUNTIME.get_or_init(|| {
        Mutex::new(BootRuntime {
            running: false,
            status: frontend_loading_boot_status(),
        })
    })
}

/// 写入最新启动状态；后台线程只通过这个入口更新前端可见进度。
fn update_boot_status(status: BootStatusData) {
    if let Ok(mut runtime) = boot_runtime().lock() {
        runtime.status = status;
    }
}

/// 发布启动阶段并保留一个短暂可见窗口，避免真实接口过快导致前端轮询跳过中间步骤。
fn publish_boot_status(status: BootStatusData) {
    update_boot_status(status);
    thread::sleep(BOOT_STAGE_VISIBLE_DURATION);
}

/// 启动后台初始化任务；命令本身立即返回当前状态，避免前端被长请求阻塞。
fn ensure_boot_worker_started(
    app_handle: AppHandle,
    binary_path: PathBuf,
    workspace_path: PathBuf,
) -> BootStatusData {
    let runtime = boot_runtime();
    let mut guard = runtime.lock().expect("boot runtime mutex poisoned");
    if guard.running || guard.status.ready || boot_status_has_failed_step(&guard.status) {
        return guard.status.clone();
    }

    guard.running = true;
    guard.status = starting_sidecar_boot_status();
    let status = guard.status.clone();
    drop(guard);

    thread::spawn(move || {
        run_boot_sequence(app_handle, binary_path, workspace_path);
    });

    status
}

/// 检查启动状态是否已经进入失败态；失败后停留在初始化页等待用户处理。
fn boot_status_has_failed_step(status: &BootStatusData) -> bool {
    status
        .steps
        .iter()
        .any(|step| step.status == BootStepStatus::Failed)
}

/// 串行执行真实启动门禁，并在每个阶段完成后刷新启动页状态。
fn run_boot_sequence(app_handle: AppHandle, binary_path: PathBuf, workspace_path: PathBuf) {
    let result = (|| -> Result<(), (&'static str, String)> {
        publish_boot_status(starting_sidecar_boot_status());
        let state = app_handle.state::<CoreState>();
        let client = state
            .start(&binary_path, &workspace_path, Duration::from_secs(5))
            .map_err(|error| ("启动 Go Core Sidecar", error.to_string()))?;

        publish_boot_status(sidecar_ready_workspace_running_boot_status());
        read_core_health(&client).map_err(|error| (core_health_error_stage(&error), error))?;

        publish_boot_status(workspace_ready_sqlite_running_boot_status());
        wait_until_search_ready(&client)?;

        publish_boot_status(tokenizer_ready_data_source_running_boot_status());
        read_core_post_ok(&client, "/api/providers/status")
            .map_err(|error| ("初始化数据源配置", error))?;

        publish_boot_status(data_source_ready_cache_running_boot_status());
        read_core_post_ok(&client, "/api/dashboard/summary")
            .map_err(|error| ("同步基础行情快照与资讯缓存", error))?;

        publish_boot_status(cache_ready_scheduler_running_boot_status());
        read_core_post_ok(&client, "/api/scheduler/status")
            .map_err(|error| ("完成基础检查并进入工作台", error))?;

        publish_boot_status(ready_boot_status());
        Ok(())
    })();

    if let Err((stage, error)) = result {
        update_boot_status(failed_boot_status(stage, &sanitize_boot_message(&error)));
    }

    if let Ok(mut runtime) = boot_runtime().lock() {
        runtime.running = false;
    }
}

/// 等待搜索索引和 GSE 分词器真实就绪；必要时只触发一次启动期索引重建。
fn wait_until_search_ready(client: &CoreClient) -> Result<(), (&'static str, String)> {
    let mut rebuild_requested = false;
    loop {
        let status = ensure_search_index_ready_for_boot(client, &mut rebuild_requested)
            .map_err(|error| (search_error_stage(&error), error))?;
        if !status.search_status.eq_ignore_ascii_case("BUILDING") && is_gse_active_batch(&status) {
            return Ok(());
        }

        publish_boot_status(boot_status_from_search_status(&status));
        thread::sleep(Duration::from_millis(300));
    }
}

/// 将启动状态包装为前端统一的 CoreEnvelope 结构，保持 coreClient 解包逻辑一致。
fn wrap_boot_status(data: BootStatusData) -> serde_json::Value {
    serde_json::json!({
        "code": 0,
        "message": "ok",
        "data": data,
    })
}

/// 读取 Go Core 健康状态，确认 sidecar 已完成监听与 token 握手。
fn read_core_health(client: &CoreClient) -> Result<(), String> {
    let response: CoreEnvelope<CoreHealthData> =
        serde_json::from_value(client.health().map_err(|error| error.to_string())?)
            .map_err(|error| error.to_string())?;
    validate_core_health_response(response)
}

/// 校验 Go Core 健康信息；数据库未就绪时不允许进入业务页面。
fn validate_core_health_response(response: CoreEnvelope<CoreHealthData>) -> Result<(), String> {
    if response.code != 0 {
        return Err(response.message);
    }
    if response.data.db_status != "ok" {
        return Err(format!(
            "SQLite status is not ok: {}",
            if response.data.db_status.is_empty() {
                "unknown"
            } else {
                response.data.db_status.as_str()
            }
        ));
    }
    Ok(())
}

/// 将健康检查错误映射到启动步骤；数据库异常不能被误报为 sidecar 启动失败。
fn core_health_error_stage(error: &str) -> &'static str {
    if error.starts_with("SQLite status is not ok") {
        return "SQLite 数据库迁移 / 重建索引";
    }
    "启动 Go Core Sidecar"
}

/// 将搜索初始化错误映射到具体启动步骤，避免 GSE 分词器失败被误报为 SQLite 失败。
fn search_error_stage(error: &str) -> &'static str {
    if error.to_ascii_lowercase().contains("gse tokenizer") {
        return "加载分词器（GSE / 全文检索词典）";
    }
    "SQLite 数据库迁移 / 重建索引"
}

/// 启动期检查搜索索引；缺索引时只触发一次重建，避免失败后无限重建导致启动页卡住。
fn ensure_search_index_ready_for_boot(
    client: &CoreClient,
    rebuild_requested: &mut bool,
) -> Result<SearchStatusData, String> {
    let status = read_search_status(client)?;
    validate_required_storage_capabilities(&status)?;
    if should_trigger_boot_rebuild(&status) {
        if *rebuild_requested && !BOOT_SEARCH_REBUILD_IN_FLIGHT.load(Ordering::SeqCst) {
            if status.search_status.eq_ignore_ascii_case("NEED_REBUILD") {
                return Err("search index rebuild did not complete".to_string());
            }
            validate_required_search_capabilities(&status)?;
            return Ok(status);
        }
        trigger_boot_search_rebuild(client.clone());
        *rebuild_requested = true;
        let mut building_status = status;
        building_status.search_status = "BUILDING".to_string();
        return Ok(building_status);
    }
    validate_required_search_capabilities(&status)?;
    Ok(status)
}

/// 启动阶段发现索引缺失时后台触发一次重建，避免状态读取 command 长时间阻塞启动页轮询。
fn trigger_boot_search_rebuild(client: CoreClient) {
    if BOOT_SEARCH_REBUILD_IN_FLIGHT
        .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
        .is_err()
    {
        return;
    }
    thread::spawn(move || {
        let _: Result<CoreEnvelope<serde_json::Value>, _> = client.post_api_with_timeout(
            "/api/search/rebuild",
            &SearchRebuildRequest {
                scope: "all".to_string(),
            },
            SEARCH_REBUILD_TIMEOUT,
        );
        BOOT_SEARCH_REBUILD_IN_FLIGHT.store(false, Ordering::SeqCst);
    });
}

/// 读取 Go Core 汇总后的搜索索引状态，避免前端直接拼接多个初始化接口。
fn read_search_status(client: &CoreClient) -> Result<SearchStatusData, String> {
    let response: CoreEnvelope<SearchStatusData> = client
        .post_api("/api/search/status", &EmptyRequest {})
        .map_err(|error| error.to_string())?;
    if response.code != 0 {
        return Err(response.message);
    }
    Ok(response.data)
}

/// 校验 SQLite FTS5 能力；FTS5 是启动阻断项，不能降级后进入工作台。
fn validate_required_storage_capabilities(status: &SearchStatusData) -> Result<(), String> {
    if !status.fts5_status.eq_ignore_ascii_case("AVAILABLE") {
        return Err(format!(
            "SQLite FTS5 is required but current status is {}",
            if status.fts5_status.trim().is_empty() {
                "UNKNOWN"
            } else {
                status.fts5_status.trim()
            }
        ));
    }
    Ok(())
}

/// 校验搜索基础能力；重建完成后的 active batch 必须由 GSE 分词器生成。
fn validate_required_search_capabilities(status: &SearchStatusData) -> Result<(), String> {
    validate_required_storage_capabilities(status)?;
    if status.search_status.eq_ignore_ascii_case("BUILDING")
        || status.search_status.eq_ignore_ascii_case("NEED_REBUILD")
    {
        return Ok(());
    }
    if !status.gse_status.eq_ignore_ascii_case("AVAILABLE")
        || !status.tokenizer_name.eq_ignore_ascii_case("gse")
    {
        return Err(format!(
            "GSE tokenizer is required but current status is {}",
            if status.gse_status.trim().is_empty() {
                "UNKNOWN"
            } else {
                status.gse_status.trim()
            }
        ));
    }
    Ok(())
}

/// 判断启动阶段是否需要主动重建索引；基础能力已在调用前校验。
fn should_trigger_boot_rebuild(status: &SearchStatusData) -> bool {
    if status.search_status.eq_ignore_ascii_case("BUILDING") {
        return false;
    }
    status.search_status.eq_ignore_ascii_case("NEED_REBUILD") || !is_gse_active_batch(status)
}

/// 判断当前 active 搜索索引是否已由 GSE 分词器构建。
fn is_gse_active_batch(status: &SearchStatusData) -> bool {
    status.gse_status.eq_ignore_ascii_case("AVAILABLE")
        && status.tokenizer_name.eq_ignore_ascii_case("gse")
}

/// 调用固定 Go Core POST 接口并只校验统一响应结构，避免启动页使用假状态。
fn read_core_post_ok(client: &CoreClient, path: &str) -> Result<(), String> {
    let response: CoreEnvelope<serde_json::Value> = client
        .post_api(path, &EmptyRequest {})
        .map_err(|error| error.to_string())?;
    if response.code != 0 {
        return Err(response.message);
    }
    Ok(())
}

/// 将搜索索引状态映射为启动页展示状态，并决定是否允许进入总览。
fn boot_status_from_search_status(status: &SearchStatusData) -> BootStatusData {
    let search_status = status.search_status.to_ascii_uppercase();
    if !status.fts5_status.eq_ignore_ascii_case("AVAILABLE") {
        return failed_boot_status(
            "SQLite 数据库迁移 / 重建索引",
            "SQLite FTS5 is required but unavailable",
        );
    }
    if search_status == "BUILDING" {
        let rebuilding_gse_index = !is_gse_active_batch(status);
        return build_boot_status(BootBuildOptions {
            ready: false,
            progress: if rebuilding_gse_index { 58 } else { 68 },
            current_step_id: if rebuilding_gse_index {
                "tokenizer"
            } else {
                "sqlite_migration"
            },
            running_stage: if rebuilding_gse_index {
                "加载分词器（GSE / 全文检索词典）"
            } else {
                "SQLite 数据库迁移 / 重建索引"
            },
            sidecar_status: BootStepStatus::Completed,
            workspace_status: BootStepStatus::Completed,
            sqlite_status: if rebuilding_gse_index {
                BootStepStatus::Completed
            } else {
                BootStepStatus::Running
            },
            tokenizer_status: if rebuilding_gse_index {
                BootStepStatus::Running
            } else {
                BootStepStatus::Pending
            },
            data_source_status: BootStepStatus::Pending,
            cache_status: BootStepStatus::Pending,
            ready_status: BootStepStatus::Pending,
            log_status: BootLogStatus::Running,
            log_message: if rebuilding_gse_index {
                "rebuilding search index with GSE tokenizer"
            } else {
                "rebuilding search index"
            },
        });
    }
    if !is_gse_active_batch(status) {
        return failed_boot_status(
            "加载分词器（GSE / 全文检索词典）",
            "GSE tokenizer is required but unavailable",
        );
    }

    ready_boot_status()
}

/// 构造前端工作台刚加载、尚未连上 Rust 启动状态命令时的初始状态。
fn frontend_loading_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 10,
        current_step_id: "sidecar",
        running_stage: "前端启动",
        sidecar_status: BootStepStatus::Pending,
        workspace_status: BootStepStatus::Pending,
        sqlite_status: BootStepStatus::Pending,
        tokenizer_status: BootStepStatus::Pending,
        data_source_status: BootStepStatus::Pending,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "waiting app boot status",
    })
}

/// 构造 Go Core Sidecar 启动中的状态。
fn starting_sidecar_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 18,
        current_step_id: "sidecar",
        running_stage: "启动 Go Core Sidecar",
        sidecar_status: BootStepStatus::Running,
        workspace_status: BootStepStatus::Pending,
        sqlite_status: BootStepStatus::Pending,
        tokenizer_status: BootStepStatus::Pending,
        data_source_status: BootStepStatus::Pending,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "starting sidecar",
    })
}

/// 构造 sidecar 已就绪并开始检查本地工作区的状态。
fn sidecar_ready_workspace_running_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 30,
        current_step_id: "workspace",
        running_stage: "检查本地工作区与目录权限",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Running,
        sqlite_status: BootStepStatus::Pending,
        tokenizer_status: BootStepStatus::Pending,
        data_source_status: BootStepStatus::Pending,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "checking workspace",
    })
}

/// 构造工作区检查完成并开始检查 SQLite 与索引的状态。
fn workspace_ready_sqlite_running_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 42,
        current_step_id: "sqlite_migration",
        running_stage: "SQLite 数据库迁移 / 重建索引",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Completed,
        sqlite_status: BootStepStatus::Running,
        tokenizer_status: BootStepStatus::Pending,
        data_source_status: BootStepStatus::Pending,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "checking sqlite and search index",
    })
}

/// 构造搜索索引与 GSE 就绪后，进入数据源检查的状态。
fn tokenizer_ready_data_source_running_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 74,
        current_step_id: "data_source",
        running_stage: "初始化数据源配置",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Completed,
        sqlite_status: BootStepStatus::Completed,
        tokenizer_status: BootStepStatus::Completed,
        data_source_status: BootStepStatus::Running,
        cache_status: BootStepStatus::Pending,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "checking provider status",
    })
}

/// 构造数据源检查完成后，进入行情快照与资讯缓存检查的状态。
fn data_source_ready_cache_running_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 86,
        current_step_id: "market_cache",
        running_stage: "同步基础行情快照与资讯缓存",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Completed,
        sqlite_status: BootStepStatus::Completed,
        tokenizer_status: BootStepStatus::Completed,
        data_source_status: BootStepStatus::Completed,
        cache_status: BootStepStatus::Running,
        ready_status: BootStepStatus::Pending,
        log_status: BootLogStatus::Running,
        log_message: "checking market cache",
    })
}

/// 构造基础缓存检查完成后，进入定时任务状态检查的状态。
fn cache_ready_scheduler_running_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: false,
        progress: 94,
        current_step_id: "ready",
        running_stage: "完成基础检查并进入工作台",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Completed,
        sqlite_status: BootStepStatus::Completed,
        tokenizer_status: BootStepStatus::Completed,
        data_source_status: BootStepStatus::Completed,
        cache_status: BootStepStatus::Completed,
        ready_status: BootStepStatus::Running,
        log_status: BootLogStatus::Running,
        log_message: "checking scheduler status",
    })
}

/// 构造所有真实启动门禁通过后的完成状态。
fn ready_boot_status() -> BootStatusData {
    build_boot_status(BootBuildOptions {
        ready: true,
        progress: 100,
        current_step_id: "ready",
        running_stage: "完成基础检查并进入工作台",
        sidecar_status: BootStepStatus::Completed,
        workspace_status: BootStepStatus::Completed,
        sqlite_status: BootStepStatus::Completed,
        tokenizer_status: BootStepStatus::Completed,
        data_source_status: BootStepStatus::Completed,
        cache_status: BootStepStatus::Completed,
        ready_status: BootStepStatus::Completed,
        log_status: BootLogStatus::Success,
        log_message: "initialization completed",
    })
}

struct BootBuildOptions {
    ready: bool,
    progress: u8,
    current_step_id: &'static str,
    running_stage: &'static str,
    sidecar_status: BootStepStatus,
    workspace_status: BootStepStatus,
    sqlite_status: BootStepStatus,
    tokenizer_status: BootStepStatus,
    data_source_status: BootStepStatus,
    cache_status: BootStepStatus,
    ready_status: BootStepStatus,
    log_status: BootLogStatus,
    log_message: &'static str,
}

/// 按统一步骤模板构造启动页数据，确保前端每次渲染都有完整步骤和日志。
fn build_boot_status(options: BootBuildOptions) -> BootStatusData {
    BootStatusData {
        ready: options.ready,
        progress: options.progress,
        current_step_id: options.current_step_id.to_string(),
        steps: vec![
            step("sidecar", 1, "启动 Go Core Sidecar", options.sidecar_status),
            step(
                "workspace",
                2,
                "检查本地工作区与目录权限",
                options.workspace_status,
            ),
            step(
                "sqlite_migration",
                3,
                "SQLite 数据库迁移 / 重建索引",
                options.sqlite_status,
            ),
            step(
                "tokenizer",
                4,
                "加载分词器（GSE / 全文检索词典）",
                options.tokenizer_status,
            ),
            step(
                "data_source",
                5,
                "初始化数据源配置",
                options.data_source_status,
            ),
            step(
                "market_cache",
                6,
                "同步基础行情快照与资讯缓存",
                options.cache_status,
            ),
            step("ready", 7, "完成基础检查并进入工作台", options.ready_status),
        ],
        task_detail: BootTaskDetail {
            task_id: BOOT_TASK_ID.to_string(),
            elapsed: "计算中".to_string(),
            current_stage: options.running_stage.to_string(),
            remaining: if options.ready {
                "00:00:00".to_string()
            } else {
                "计算中".to_string()
            },
        },
        logs: build_boot_logs(&options),
    }
}

/// 根据步骤真实状态生成初始化日志，避免未完成阶段被误显示为成功。
fn build_boot_logs(options: &BootBuildOptions) -> Vec<BootLogItem> {
    let mut logs = Vec::new();
    push_step_log(
        &mut logs,
        options.sidecar_status,
        "sidecar ready",
        "starting sidecar",
        "sidecar failed",
    );
    push_step_log(
        &mut logs,
        options.workspace_status,
        "workspace opened",
        "checking workspace",
        "workspace failed",
    );
    push_step_log(
        &mut logs,
        options.sqlite_status,
        "sqlite migrate completed",
        "checking sqlite and search index",
        "sqlite initialization failed",
    );
    push_step_log(
        &mut logs,
        options.tokenizer_status,
        "tokenizer ready",
        "loading tokenizer and search index",
        "tokenizer initialization failed",
    );
    push_step_log(
        &mut logs,
        options.data_source_status,
        "provider status checked",
        "checking provider status",
        "provider initialization failed",
    );
    push_step_log(
        &mut logs,
        options.cache_status,
        "market cache checked",
        "checking market cache",
        "market cache initialization failed",
    );
    push_step_log(
        &mut logs,
        options.ready_status,
        "initialization completed",
        "checking scheduler status",
        "final initialization check failed",
    );

    if !options.log_message.is_empty()
        && !logs.iter().any(|item| item.message == options.log_message)
    {
        logs.push(log_item(
            &(logs.len() + 1).to_string(),
            options.log_status,
            options.log_message,
        ));
    }
    logs
}

/// 将单个步骤状态追加为日志项；等待中的步骤不生成日志，降低启动页噪声。
fn push_step_log(
    logs: &mut Vec<BootLogItem>,
    status: BootStepStatus,
    success_message: &str,
    running_message: &str,
    failed_message: &str,
) {
    let next_id = (logs.len() + 1).to_string();
    match status {
        BootStepStatus::Completed => {
            logs.push(log_item(&next_id, BootLogStatus::Success, success_message));
        }
        BootStepStatus::Running => {
            logs.push(log_item(&next_id, BootLogStatus::Running, running_message));
        }
        BootStepStatus::Failed => {
            logs.push(log_item(&next_id, BootLogStatus::Error, failed_message));
        }
        BootStepStatus::Pending => {}
    }
}

/// 构造初始化失败状态；错误信息会在调用方先脱敏后进入启动页日志。
fn failed_boot_status(stage: &str, message: &str) -> BootStatusData {
    let (failed_step, failed_index) = match stage {
        "启动 Go Core Sidecar" => ("sidecar", 1),
        "检查本地工作区与目录权限" => ("workspace", 2),
        "SQLite 数据库迁移 / 重建索引" => ("sqlite_migration", 3),
        "加载分词器（GSE / 全文检索词典）" => ("tokenizer", 4),
        "初始化数据源配置" => ("data_source", 5),
        "同步基础行情快照与资讯缓存" => ("market_cache", 6),
        "完成基础检查并进入工作台" => ("ready", 7),
        _ => ("sqlite_migration", 3),
    };
    let status_for = |step_index: u8| {
        if step_index < failed_index {
            BootStepStatus::Completed
        } else if step_index == failed_index {
            BootStepStatus::Failed
        } else {
            BootStepStatus::Pending
        }
    };
    let progress = if failed_index <= 2 {
        15
    } else if failed_index == 3 {
        30
    } else if failed_index == 4 {
        58
    } else if failed_index == 5 {
        74
    } else if failed_index == 6 {
        86
    } else {
        95
    };
    let mut data = build_boot_status(BootBuildOptions {
        ready: false,
        progress,
        current_step_id: failed_step,
        running_stage: "初始化失败",
        sidecar_status: status_for(1),
        workspace_status: status_for(2),
        sqlite_status: status_for(3),
        tokenizer_status: status_for(4),
        data_source_status: status_for(5),
        cache_status: status_for(6),
        ready_status: status_for(7),
        log_status: BootLogStatus::Error,
        log_message: "initialization failed",
    });
    for step in &mut data.steps {
        if step.title == stage {
            step.status = BootStepStatus::Failed;
            step.badge_text = "失败".to_string();
        }
    }
    data.task_detail.current_stage = stage.to_string();
    data.logs.push(log_item("5", BootLogStatus::Error, message));
    data
}

/// 构造单个启动步骤，badge 文案始终由状态派生，避免双写不一致。
fn step(id: &str, index: u8, title: &str, status: BootStepStatus) -> BootStep {
    BootStep {
        id: id.to_string(),
        index,
        title: title.to_string(),
        status,
        badge_text: badge_text(status).to_string(),
    }
}

/// 将步骤枚举状态转换为启动页展示文案。
fn badge_text(status: BootStepStatus) -> &'static str {
    match status {
        BootStepStatus::Completed => "已完成",
        BootStepStatus::Running => "进行中",
        BootStepStatus::Pending => "等待中",
        BootStepStatus::Failed => "失败",
    }
}

/// 构造初始化日志条目；启动阶段不展示真实路径或凭据类敏感内容。
fn log_item(id: &str, status: BootLogStatus, message: &str) -> BootLogItem {
    BootLogItem {
        id: id.to_string(),
        time: "--:--:--".to_string(),
        status,
        message: message.to_string(),
    }
}

/// 对启动错误做保守脱敏，避免 token、cookie、API Key 等内容进入前端。
fn sanitize_boot_message(message: &str) -> String {
    let mut value = message.to_string();
    let lower = value.to_ascii_lowercase();
    for marker in [
        "Authorization",
        "Proxy-Authorization",
        "token=",
        "token:",
        " token ",
        "cookie",
        "api_key",
        "api-key",
        "password",
        "secret",
    ] {
        if lower.contains(&marker.to_ascii_lowercase()) {
            value = "initialization error redacted".to_string();
            break;
        }
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 验证搜索索引处于 READY 时启动页进入 ready，不继续阻塞总览。
    #[test]
    fn boot_status_from_ready_search_marks_all_steps_completed() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            gse_status: "AVAILABLE".to_string(),
            search_status: "READY".to_string(),
            tokenizer_name: "gse".to_string(),
        });

        assert!(data.ready);
        assert_eq!(data.progress, 100);
        assert!(data
            .steps
            .iter()
            .all(|step| step.status == BootStepStatus::Completed));
    }

    /// 验证前端初始等待态不伪装任何后端初始化步骤已完成。
    #[test]
    fn frontend_loading_status_keeps_backend_steps_pending() {
        let data = frontend_loading_boot_status();

        assert!(!data.ready);
        assert_eq!(data.progress, 10);
        assert!(data
            .steps
            .iter()
            .all(|step| step.status == BootStepStatus::Pending));
        assert!(data
            .logs
            .iter()
            .any(|item| item.message == "waiting app boot status"));
    }

    /// 验证失败态会被启动轮询识别为终态，避免后台初始化任务反复重启。
    #[test]
    fn boot_status_has_failed_step_detects_terminal_failure() {
        let failed = failed_boot_status(
            "加载分词器（GSE / 全文检索词典）",
            "GSE tokenizer is required but unavailable",
        );
        let running = starting_sidecar_boot_status();

        assert!(boot_status_has_failed_step(&failed));
        assert!(!boot_status_has_failed_step(&running));
    }

    /// 验证搜索索引正在重建时启动页停留在初始化页，避免提前进入业务页面。
    #[test]
    fn boot_status_from_building_search_keeps_initializing() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "BUILDING".to_string(),
            ..SearchStatusData::default()
        });

        assert!(!data.ready);
        assert_eq!(data.current_step_id, "tokenizer");
        assert_eq!(data.steps[2].status, BootStepStatus::Completed);
        assert_eq!(data.steps[3].status, BootStepStatus::Running);
    }

    /// 验证启动期在缺索引或 active batch 仍是 simple 时触发自动重建；基础能力校验由调用方先执行。
    #[test]
    fn should_trigger_boot_rebuild_for_missing_or_simple_index() {
        assert!(should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "NEED_REBUILD".to_string(),
            ..SearchStatusData::default()
        }));
        assert!(should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "READY".to_string(),
            gse_status: "FALLBACK".to_string(),
            tokenizer_name: "simple".to_string(),
        }));
        assert!(!should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            gse_status: "AVAILABLE".to_string(),
            search_status: "READY".to_string(),
            tokenizer_name: "gse".to_string(),
        }));
        assert!(!should_trigger_boot_rebuild(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "BUILDING".to_string(),
            ..SearchStatusData::default()
        }));
    }

    /// 验证 FTS5 是启动阻断项，不再允许降级进入总览。
    #[test]
    fn boot_status_blocks_when_fts5_is_unavailable() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "UNAVAILABLE".to_string(),
            search_status: "READY".to_string(),
            ..SearchStatusData::default()
        });

        assert!(!data.ready);
        assert_eq!(data.current_step_id, "sqlite_migration");
        assert_eq!(data.steps[2].status, BootStepStatus::Failed);
    }

    /// 验证 GSE 是最终启动阻断项，状态映射层不会让 simple active batch 进入工作台。
    #[test]
    fn boot_status_blocks_when_gse_falls_back_to_simple() {
        let data = boot_status_from_search_status(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            gse_status: "FALLBACK".to_string(),
            search_status: "READY".to_string(),
            tokenizer_name: "simple".to_string(),
        });

        assert!(!data.ready);
        assert_eq!(data.current_step_id, "tokenizer");
        assert_eq!(data.steps[2].status, BootStepStatus::Completed);
        assert_eq!(data.steps[3].status, BootStepStatus::Failed);
    }

    /// 验证启动校验允许重建中状态，但拒绝重建后仍未使用 GSE 的状态。
    #[test]
    fn validate_required_search_capabilities_requires_gse_after_rebuild() {
        assert!(validate_required_search_capabilities(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            search_status: "BUILDING".to_string(),
            ..SearchStatusData::default()
        })
        .is_ok());

        let err = validate_required_search_capabilities(&SearchStatusData {
            fts5_status: "AVAILABLE".to_string(),
            gse_status: "FALLBACK".to_string(),
            search_status: "READY".to_string(),
            tokenizer_name: "simple".to_string(),
        })
        .expect_err("fallback tokenizer should block boot");
        assert!(err.contains("GSE tokenizer is required"));
    }

    /// 验证数据库未就绪时启动门禁失败，不能提前进入总览页面。
    #[test]
    fn validate_core_health_response_rejects_non_ok_database_status() {
        let result = validate_core_health_response(CoreEnvelope {
            code: 0,
            message: "ok".to_string(),
            data: CoreHealthData {
                db_status: "not_configured".to_string(),
            },
        });

        assert_eq!(
            result.expect_err("db not ready should block boot"),
            "SQLite status is not ok: not_configured"
        );
    }

    /// 验证数据库健康异常会落到 SQLite 初始化步骤，而不是误导为 sidecar 连接失败。
    #[test]
    fn core_health_error_stage_maps_database_error_to_sqlite_step() {
        assert_eq!(
            core_health_error_stage("SQLite status is not ok: not_configured"),
            "SQLite 数据库迁移 / 重建索引"
        );
        assert_eq!(
            core_health_error_stage("sidecar http error"),
            "启动 Go Core Sidecar"
        );
    }

    /// 验证搜索初始化错误会落到准确步骤，避免 GSE 失败误报为 SQLite 失败。
    #[test]
    fn search_error_stage_maps_tokenizer_error_to_gse_step() {
        assert_eq!(
            search_error_stage("GSE tokenizer is required but current status is FALLBACK"),
            "加载分词器（GSE / 全文检索词典）"
        );
        assert_eq!(
            search_error_stage("SQLite FTS5 is required but current status is UNAVAILABLE"),
            "SQLite 数据库迁移 / 重建索引"
        );
    }

    /// 验证启动错误脱敏不会把 tokenizer 误判为敏感 token，但仍会清理真实 token 字段。
    #[test]
    fn sanitize_boot_message_keeps_tokenizer_word_and_redacts_token_value() {
        assert_eq!(
            sanitize_boot_message("GSE tokenizer is required but current status is FALLBACK"),
            "GSE tokenizer is required but current status is FALLBACK"
        );
        assert_eq!(
            sanitize_boot_message("provider token=secret-value is invalid"),
            "initialization error redacted"
        );
    }
}
