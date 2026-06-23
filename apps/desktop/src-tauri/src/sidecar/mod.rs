use rand::{rngs::OsRng, RngCore};
use reqwest::header::{HeaderMap, HeaderValue};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use std::{
    fmt,
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, Write},
    path::{Path, PathBuf},
    process::{Child, ChildStdin, Command, ExitStatus, Stdio},
    sync::{mpsc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

const PROTOCOL_VERSION: &str = "1";
const TOKEN_BYTES: usize = 32;
const TOKEN_HEADER: &str = "X-Invest-Compass-Token";
const CORE_BINARY_ENV: &str = "INVEST_COMPASS_CORE_BIN";
const API_REQUEST_TIMEOUT: Duration = Duration::from_secs(10);

#[derive(Debug)]
pub enum SidecarError {
    Io(std::io::Error),
    Json(serde_json::Error),
    Http(reqwest::Error),
    MissingChildStdin,
    MissingChildStdout,
    ReadyTimeout,
    CoreNotRunning,
    InvalidReadyStatus(String),
    ProtocolMismatch { expected: String, actual: String },
    SseFrameHandler(String),
}

impl fmt::Display for SidecarError {
    /// 将 sidecar 错误转换为不含敏感信息的用户可读文本。
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SidecarError::Io(error) => write!(formatter, "sidecar io error: {error}"),
            SidecarError::Json(error) => write!(formatter, "sidecar json error: {error}"),
            SidecarError::Http(error) => write!(formatter, "sidecar http error: {error}"),
            SidecarError::MissingChildStdin => write!(formatter, "sidecar stdin is unavailable"),
            SidecarError::MissingChildStdout => write!(formatter, "sidecar stdout is unavailable"),
            SidecarError::ReadyTimeout => write!(formatter, "sidecar ready timeout"),
            SidecarError::CoreNotRunning => write!(formatter, "core sidecar is not running"),
            SidecarError::InvalidReadyStatus(status) => {
                write!(formatter, "invalid sidecar ready status: {status}")
            }
            SidecarError::ProtocolMismatch { expected, actual } => {
                write!(
                    formatter,
                    "sidecar protocol mismatch: expected {expected}, got {actual}"
                )
            }
            SidecarError::SseFrameHandler(error) => {
                write!(formatter, "sidecar sse frame handler error: {error}")
            }
        }
    }
}

impl std::error::Error for SidecarError {}

impl SidecarError {
    /// 判断是否为连接层错误；仅用于本地 sidecar 端口失效后的单次恢复，不覆盖业务 HTTP 状态错误。
    pub fn is_retryable_transport_error(&self) -> bool {
        matches!(self, SidecarError::Http(error) if error.status().is_none())
    }
}

impl From<std::io::Error> for SidecarError {
    /// 统一转换 IO 错误，避免调用方重复映射。
    fn from(error: std::io::Error) -> Self {
        SidecarError::Io(error)
    }
}

impl From<serde_json::Error> for SidecarError {
    /// 统一转换 JSON 错误，保证握手和 ready 解析走同一错误类型。
    fn from(error: serde_json::Error) -> Self {
        SidecarError::Json(error)
    }
}

impl From<reqwest::Error> for SidecarError {
    /// 统一转换 HTTP 错误，保证 Rust command 不暴露底层实现细节。
    fn from(error: reqwest::Error) -> Self {
        SidecarError::Http(error)
    }
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct HandshakePayload<'a> {
    token: &'a str,
    protocol_version: &'a str,
    keepalive_stdin: bool,
}

#[derive(Debug, Deserialize)]
pub struct ReadyMessage {
    pub status: String,
    pub port: u16,
    pub pid: u32,
    #[serde(rename = "protocolVersion")]
    pub protocol_version: String,
}

#[derive(Clone)]
pub struct CoreClient {
    port: u16,
    token: String,
    http_client: reqwest::blocking::Client,
    sse_client: reqwest::blocking::Client,
}

pub struct RunningCore {
    client: CoreClient,
    child: Child,
    _stdin: ChildStdin,
}

pub struct CoreState {
    running: Mutex<Option<RunningCore>>,
}

impl CoreState {
    /// 创建空的 core 状态，避免 UI 在 sidecar 未启动时拿到伪造数据。
    pub fn empty() -> Self {
        Self {
            running: Mutex::new(None),
        }
    }

    /// 安装已经启动的 sidecar 进程，作为 Rust command 的唯一数据源。
    pub fn install(&self, running: RunningCore) {
        let mut guard = self.running.lock().expect("core state poisoned");
        *guard = Some(running);
    }

    /// 获取当前 core 客户端副本，未启动时显式返回错误。
    pub fn client(&self) -> Result<CoreClient, SidecarError> {
        let mut guard = self.running.lock().expect("core state poisoned");
        if let Some(running) = guard.as_mut() {
            if running.child.try_wait()?.is_some() {
                *guard = None;
                return Err(SidecarError::CoreNotRunning);
            }
            return Ok(running.client.clone());
        }
        Err(SidecarError::CoreNotRunning)
    }

    /// 按固定二进制路径启动 sidecar，已启动时直接复用当前客户端。
    pub fn start(
        &self,
        binary_path: &Path,
        workspace_path: &Path,
        ready_timeout: Duration,
    ) -> Result<CoreClient, SidecarError> {
        let mut guard = self.running.lock().expect("core state poisoned");
        if let Some(running) = guard.as_mut() {
            if let Some(status) = running.child.try_wait()? {
                append_sidecar_runtime_log(
                    workspace_path,
                    &format!(
                        "core_process_exited_before_start_reuse port={} status={}",
                        running.client.port(),
                        describe_exit_status(status)
                    ),
                );
            } else {
                return Ok(running.client.clone());
            }
            *guard = None;
        }

        let running = start_core_sidecar(binary_path, workspace_path, ready_timeout)?;
        let client = running.client.clone();
        *guard = Some(running);
        Ok(client)
    }

    /// 当调用方持有的 client 端口已经失效时，只在状态仍指向同一端口时重启；若其他命令已恢复出新端口，则直接复用新 client。
    pub fn recover_after_failed_client(
        &self,
        failed_client: &CoreClient,
        binary_path: &Path,
        workspace_path: &Path,
        ready_timeout: Duration,
    ) -> Result<CoreClient, SidecarError> {
        let mut guard = self.running.lock().expect("core state poisoned");
        if let Some(running) = guard.as_mut() {
            if running.client.port != failed_client.port {
                if let Some(status) = running.child.try_wait()? {
                    append_sidecar_runtime_log(
                        workspace_path,
                        &format!(
                            "core_process_exited_during_recover old_failed_port={} active_port={} status={}",
                            failed_client.port(),
                            running.client.port(),
                            describe_exit_status(status)
                        ),
                    );
                } else {
                    return Ok(running.client.clone());
                }
                *guard = None;
            } else {
                if let Some(status) = running.child.try_wait()? {
                    append_sidecar_runtime_log(
                        workspace_path,
                        &format!(
                            "core_process_exited_during_recover failed_port={} status={}",
                            failed_client.port(),
                            describe_exit_status(status)
                        ),
                    );
                } else {
                    return Ok(running.client.clone());
                }
                *guard = None;
            }
        }

        let running = start_core_sidecar(binary_path, workspace_path, ready_timeout)?;
        let client = running.client.clone();
        *guard = Some(running);
        Ok(client)
    }

    /// 执行普通 POST API；若本地端口在取 client 后失效，只恢复一次并重试当前读取请求。
    pub fn post_api_with_recovery<TRequest, TResponse>(
        &self,
        binary_path: &Path,
        workspace_path: &Path,
        ready_timeout: Duration,
        path: &str,
        payload: &TRequest,
    ) -> Result<TResponse, SidecarError>
    where
        TRequest: Serialize,
        TResponse: DeserializeOwned,
    {
        let client = match self.client() {
            Ok(client) => client,
            Err(_) => self.start(binary_path, workspace_path, ready_timeout)?,
        };
        match client.post_api(path, payload) {
            Ok(value) => Ok(value),
            Err(error) if error.is_retryable_transport_error() => {
                append_sidecar_runtime_log(
                    workspace_path,
                    &format!(
                        "api_transport_error path={path} failed_port={} error={error}",
                        client.port()
                    ),
                );
                let recovered = self.recover_after_failed_client(
                    &client,
                    binary_path,
                    workspace_path,
                    ready_timeout,
                )?;
                append_sidecar_runtime_log(
                    workspace_path,
                    &format!(
                        "api_transport_recovered path={path} failed_port={} active_port={}",
                        client.port(),
                        recovered.port()
                    ),
                );
                let result = recovered.post_api(path, payload);
                if let Err(error) = &result {
                    append_sidecar_runtime_log(
                        workspace_path,
                        &format!(
                            "api_transport_retry_failed path={path} active_port={} error={error}",
                            recovered.port()
                        ),
                    );
                }
                result
            }
            Err(error) => {
                append_sidecar_runtime_log(
                    workspace_path,
                    &format!(
                        "api_request_failed path={path} port={} retryable=false error={error}",
                        client.port()
                    ),
                );
                Err(error)
            }
        }
    }

    /// 停止当前 sidecar，优先走内部 shutdown，再兜底 kill 本地子进程。
    pub fn stop(&self) {
        let mut guard = self.running.lock().expect("core state poisoned");
        if let Some(mut running) = guard.take() {
            let _ = running.client.shutdown();
            let _ = running.child.kill();
            let _ = running.child.wait();
        }
    }
}

impl CoreClient {
    /// 创建 core HTTP 客户端，端口和 token 由 sidecar ready/handshake 单一来源注入。
    pub fn new(port: u16, token: String) -> Self {
        Self {
            port,
            token,
            http_client: build_api_http_client(),
            sse_client: build_sse_http_client(),
        }
    }

    /// 构造内部 API URL，固定只能访问本机回环地址。
    pub fn internal_url(&self, path: &str) -> String {
        format!("http://127.0.0.1:{}{}", self.port, path)
    }

    /// 返回 runtime token，仅供测试和内部请求组装使用，禁止写入日志。
    pub fn token(&self) -> &str {
        &self.token
    }

    /// 返回当前 client 绑定的本机端口，只用于状态恢复判断，禁止作为前端可配置项。
    pub fn port(&self) -> u16 {
        self.port
    }

    /// 调用 Go core 健康检查接口，所有请求都带固定 token header。
    pub fn health(&self) -> Result<serde_json::Value, SidecarError> {
        self.post_internal("/internal/health")
    }

    /// 请求 Go core 优雅关闭，失败时由进程管理层继续执行 kill 兜底。
    pub fn shutdown(&self) -> Result<serde_json::Value, SidecarError> {
        self.post_internal("/internal/shutdown")
    }

    /// 按固定 path 调用 Go core POST API，供 Rust 白名单 command 复用。
    pub fn post_api<TRequest, TResponse>(
        &self,
        path: &str,
        payload: &TRequest,
    ) -> Result<TResponse, SidecarError>
    where
        TRequest: Serialize,
        TResponse: DeserializeOwned,
    {
        self.post_api_with_optional_timeout(path, payload, None)
    }

    /// 按固定 path 调用 Go core POST API，并为慢外部 Provider 场景指定请求超时。
    pub fn post_api_with_timeout<TRequest, TResponse>(
        &self,
        path: &str,
        payload: &TRequest,
        timeout: Duration,
    ) -> Result<TResponse, SidecarError>
    where
        TRequest: Serialize,
        TResponse: DeserializeOwned,
    {
        self.post_api_with_optional_timeout(path, payload, Some(timeout))
    }

    /// 执行普通 POST API 请求，timeout 仅允许白名单 command 显式传入。
    fn post_api_with_optional_timeout<TRequest, TResponse>(
        &self,
        path: &str,
        payload: &TRequest,
        timeout: Option<Duration>,
    ) -> Result<TResponse, SidecarError>
    where
        TRequest: Serialize,
        TResponse: DeserializeOwned,
    {
        let mut request = self
            .http_client
            .post(self.internal_url(path))
            .headers(build_internal_headers(&self.token))
            .json(payload);
        if let Some(timeout) = timeout {
            request = request.timeout(timeout);
        }
        let response = request.send()?.error_for_status()?.json()?;

        Ok(response)
    }

    /// 按固定 path 调用 Go core SSE POST API，并逐帧回调给 Rust command 转发。
    pub fn post_sse<TRequest, F>(
        &self,
        path: &str,
        payload: &TRequest,
        on_frame: F,
    ) -> Result<usize, SidecarError>
    where
        TRequest: Serialize,
        F: FnMut(&str) -> Result<(), SidecarError>,
    {
        let response = self
            .sse_client
            .post(self.internal_url(path))
            .headers(build_internal_headers(&self.token))
            .json(payload)
            .send()?
            .error_for_status()?;

        stream_sse_frames(BufReader::new(response), on_frame)
    }

    /// 执行内部 POST 请求，确保前端无法传入任意 path 或 method。
    fn post_internal(&self, path: &str) -> Result<serde_json::Value, SidecarError> {
        self.post_api(path, &serde_json::json!({}))
    }
}

impl Drop for CoreState {
    /// 应用状态释放时停止 sidecar，避免桌面进程退出后残留本地服务。
    fn drop(&mut self) {
        self.stop();
    }
}

/// 生成 256-bit runtime token，并用十六进制承载，避免出现在 argv/env/config 中。
pub fn generate_runtime_token() -> String {
    let mut bytes = [0_u8; TOKEN_BYTES];
    OsRng.fill_bytes(&mut bytes);

    bytes
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>()
}

/// 返回当前平台的 Go core 二进制文件名，Windows 使用 `.exe` 后缀。
pub fn core_binary_file_name() -> &'static str {
    if cfg!(all(target_os = "macos", target_arch = "aarch64")) {
        core_binary_file_name_for_target("aarch64-apple-darwin")
    } else if cfg!(all(target_os = "macos", target_arch = "x86_64")) {
        core_binary_file_name_for_target("x86_64-apple-darwin")
    } else if cfg!(all(target_os = "windows", target_arch = "x86_64")) {
        core_binary_file_name_for_target("x86_64-pc-windows-msvc")
    } else if cfg!(windows) {
        "invest-compass-core.exe"
    } else {
        "invest-compass-core"
    }
}

/// 按 target triple 返回 Tauri sidecar 二进制文件名。
pub fn core_binary_file_name_for_target(target: &str) -> &'static str {
    match target {
        "aarch64-apple-darwin" => "invest-compass-core-aarch64-apple-darwin",
        "x86_64-apple-darwin" => "invest-compass-core-x86_64-apple-darwin",
        "x86_64-pc-windows-msvc" => "invest-compass-core-x86_64-pc-windows-msvc.exe",
        _ => "invest-compass-core",
    }
}

/// 返回发布包内 Tauri externalBin 的运行期文件名。
pub fn bundled_core_binary_file_name() -> &'static str {
    if cfg!(windows) {
        "invest-compass-core.exe"
    } else {
        "invest-compass-core"
    }
}

/// 解析 Go core 二进制路径，发布包优先使用可执行文件同目录 sidecar，本地开发回到 `src-tauri/binaries`。
pub fn resolve_core_binary_path(
    env_override: Option<String>,
    manifest_dir: &Path,
    executable_dir: Option<&Path>,
) -> PathBuf {
    if let Some(path) = env_override {
        return PathBuf::from(path);
    }

    if let Some(dir) = executable_dir {
        let bundled_path = dir.join(bundled_core_binary_file_name());
        if bundled_path.exists() {
            return bundled_path;
        }
    }

    manifest_dir.join("binaries").join(core_binary_file_name())
}

/// 获取运行期 Go core 二进制路径，作为 setup 和 command 的唯一来源。
pub fn runtime_core_binary_path() -> PathBuf {
    let executable_dir = std::env::current_exe()
        .ok()
        .and_then(|path| path.parent().map(Path::to_path_buf));

    resolve_core_binary_path(
        std::env::var(CORE_BINARY_ENV).ok(),
        Path::new(env!("CARGO_MANIFEST_DIR")),
        executable_dir.as_deref(),
    )
}

/// 构造 Rust 到 Go core 的内部请求头，统一注入 token 和追踪 ID。
fn build_internal_headers(token: &str) -> HeaderMap {
    let mut headers = HeaderMap::new();
    headers.insert(
        TOKEN_HEADER,
        HeaderValue::from_str(token).expect("runtime token must be valid header text"),
    );
    headers.insert(
        "X-Request-Id",
        HeaderValue::from_str(&new_trace_id()).expect("request id must be valid header text"),
    );
    headers.insert(
        "X-Trace-Id",
        HeaderValue::from_str(&new_trace_id()).expect("trace id must be valid header text"),
    );
    headers
}

/// 生成本地请求追踪 ID，用于 Rust 与 Go core 的内部调用关联。
fn new_trace_id() -> String {
    let mut bytes = [0_u8; 16];
    OsRng.fill_bytes(&mut bytes);

    bytes
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>()
}

/// 构造写入 sidecar stdin 的单行握手 JSON。
pub fn build_handshake_line(token: &str) -> Result<String, SidecarError> {
    let payload = HandshakePayload {
        token,
        protocol_version: PROTOCOL_VERSION,
        keepalive_stdin: true,
    };
    let mut line = serde_json::to_string(&payload)?;
    line.push('\n');
    Ok(line)
}

/// 解析 Go core 输出的 ready JSON，并拒绝非 ready 状态。
pub fn parse_ready_message(line: &str) -> Result<ReadyMessage, SidecarError> {
    let ready: ReadyMessage = serde_json::from_str(line)?;
    if ready.status != "ready" {
        return Err(SidecarError::InvalidReadyStatus(ready.status));
    }
    if ready.protocol_version != PROTOCOL_VERSION {
        return Err(SidecarError::ProtocolMismatch {
            expected: PROTOCOL_VERSION.to_string(),
            actual: ready.protocol_version,
        });
    }
    Ok(ready)
}

/// 启动 Go sidecar，完成 stdin token 握手，并从 stdout 读取 ready JSON。
pub fn start_core_sidecar(
    binary_path: &Path,
    workspace_path: &Path,
    ready_timeout: Duration,
) -> Result<RunningCore, SidecarError> {
    let token = generate_runtime_token();
    append_sidecar_runtime_log(
        workspace_path,
        &format!("core_start binary={}", binary_path.display()),
    );
    let mut child = Command::new(binary_path)
        .args(build_core_start_args(workspace_path))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(core_stderr(workspace_path)?)
        .spawn()?;

    let (ready, stdin) = match complete_core_start(&mut child, &token, ready_timeout) {
        Ok(result) => result,
        Err(error) => {
            append_sidecar_runtime_log(
                workspace_path,
                &format!("core_start_failed pid={} error={error}", child.id()),
            );
            terminate_child(&mut child);
            return Err(error);
        }
    };
    append_sidecar_runtime_log(
        workspace_path,
        &format!("core_ready pid={} port={}", ready.pid, ready.port),
    );

    Ok(RunningCore {
        client: CoreClient::new(ready.port, token),
        child,
        _stdin: stdin,
    })
}

/// 打开 Go core stderr 日志文件；该文件只记录 core 自身错误输出，不写入 runtime token。
fn core_stderr(workspace_path: &Path) -> Result<Stdio, SidecarError> {
    let log_dir = workspace_path.join("logs");
    fs::create_dir_all(&log_dir)?;
    let file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_dir.join("go-core-stderr.log"))?;
    Ok(Stdio::from(file))
}

/// 追加 Rust sidecar 运行期诊断日志；只记录端口、路径和错误类型，不记录 token 或凭据。
fn append_sidecar_runtime_log(workspace_path: &Path, message: &str) {
    let log_dir = workspace_path.join("logs");
    if fs::create_dir_all(&log_dir).is_err() {
        return;
    }
    let Ok(mut file) = OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_dir.join("sidecar-runtime.log"))
    else {
        return;
    };
    let _ = writeln!(file, "{} {}", runtime_log_timestamp(), message);
}

/// 生成简单稳定的毫秒时间戳，避免为诊断日志引入额外依赖。
fn runtime_log_timestamp() -> String {
    let duration = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    format!("{}.{}", duration.as_secs(), duration.subsec_millis())
}

/// 将子进程退出状态转为安全诊断文本，不包含启动参数、token 或用户路径。
fn describe_exit_status(status: ExitStatus) -> String {
    if let Some(code) = status.code() {
        return format!("code:{code}");
    }
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;

        if let Some(signal) = status.signal() {
            return format!("signal:{signal}");
        }
    }
    "signal".to_string()
}

/// 完成已启动 sidecar 的 stdin 握手和 ready 读取；调用方负责错误路径清理进程。
fn complete_core_start(
    child: &mut Child,
    token: &str,
    ready_timeout: Duration,
) -> Result<(ReadyMessage, ChildStdin), SidecarError> {
    let mut stdin = child.stdin.take().ok_or(SidecarError::MissingChildStdin)?;
    stdin.write_all(build_handshake_line(token)?.as_bytes())?;

    let stdout = child
        .stdout
        .take()
        .ok_or(SidecarError::MissingChildStdout)?;
    let ready_line = read_ready_line(stdout, ready_timeout)?;
    Ok((parse_ready_message(&ready_line)?, stdin))
}

/// 尽力终止启动失败的 sidecar 子进程，避免留下孤儿本地服务。
fn terminate_child(child: &mut Child) {
    let _ = child.kill();
    let _ = child.wait();
}

/// 创建普通 API HTTP client，所有短请求必须有总超时。
fn build_api_http_client() -> reqwest::blocking::Client {
    reqwest::blocking::Client::builder()
        .timeout(API_REQUEST_TIMEOUT)
        .build()
        .expect("api http client config must be valid")
}

/// 创建 SSE HTTP client，长任务流不设置总请求超时，由订阅生命周期负责关闭。
fn build_sse_http_client() -> reqwest::blocking::Client {
    let mut builder = reqwest::blocking::Client::builder();
    if let Some(timeout) = sse_total_timeout() {
        builder = builder.timeout(timeout);
    }
    builder
        .build()
        .expect("sse http client config must be valid")
}

/// 返回 SSE 总请求超时策略；长连接必须为 None，避免总超时打断仍在运行的任务。
fn sse_total_timeout() -> Option<Duration> {
    None
}

/// 构造 Go core 启动参数，只传递非敏感运行上下文，runtime token 只能走 stdin。
fn build_core_start_args(workspace_path: &Path) -> Vec<String> {
    vec![
        "--host".to_string(),
        "127.0.0.1".to_string(),
        "--port".to_string(),
        "0".to_string(),
        "--workspace".to_string(),
        workspace_path.to_string_lossy().to_string(),
    ]
}

/// 逐行读取 SSE 响应并在完整帧到达时立即回调，避免等待长连接结束后才通知前端。
fn stream_sse_frames<R, F>(reader: R, mut on_frame: F) -> Result<usize, SidecarError>
where
    R: BufRead,
    F: FnMut(&str) -> Result<(), SidecarError>,
{
    let mut frame = String::new();
    let mut emitted = 0;

    for line in reader.lines() {
        let line = line?;
        if line.trim().is_empty() {
            if !frame.trim().is_empty() {
                on_frame(frame.trim_end())?;
                emitted += 1;
                frame.clear();
            }
            continue;
        }
        frame.push_str(&line);
        frame.push('\n');
    }

    if !frame.trim().is_empty() {
        on_frame(frame.trim_end())?;
        emitted += 1;
    }

    Ok(emitted)
}

/// 在限定时间内读取 ready 行，随后持续 drain stdout，避免 Go core 后续 stdout 写入触发 SIGPIPE。
fn read_ready_line(
    stdout: std::process::ChildStdout,
    ready_timeout: Duration,
) -> Result<String, SidecarError> {
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        let mut reader = BufReader::new(stdout);
        let mut line = String::new();
        let result = reader.read_line(&mut line).map(|_| line);
        let _ = sender.send(result);
        drain_core_stdout(reader);
    });

    match receiver.recv_timeout(ready_timeout) {
        Ok(result) => result.map_err(SidecarError::Io),
        Err(mpsc::RecvTimeoutError::Timeout) => Err(SidecarError::ReadyTimeout),
        Err(mpsc::RecvTimeoutError::Disconnected) => Err(SidecarError::ReadyTimeout),
    }
}

/// ready 之后 Go core 不应继续向 stdout 输出业务内容；这里仍持续读取并丢弃，保证管道不因 Rust 端关闭而杀死子进程。
fn drain_core_stdout<R: BufRead>(mut reader: R) {
    let mut buffer = Vec::with_capacity(1024);
    while reader.read_until(b'\n', &mut buffer).is_ok() {
        if buffer.is_empty() {
            break;
        }
        buffer.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;
    use std::{
        cell::Cell,
        io::{self, Read},
        rc::Rc,
    };

    struct ChunkedSseRead {
        chunks: Vec<&'static [u8]>,
        next_chunk: usize,
        first_frame_seen: Rc<Cell<bool>>,
    }

    impl Read for ChunkedSseRead {
        /// 模拟网络分块：读取第二块前必须已经触发第一帧回调，证明实现不是先读完整响应。
        fn read(&mut self, buffer: &mut [u8]) -> io::Result<usize> {
            if self.next_chunk >= self.chunks.len() {
                return Ok(0);
            }
            if self.next_chunk == 1 && !self.first_frame_seen.get() {
                return Err(io::Error::new(
                    io::ErrorKind::Other,
                    "first frame was not handled before reading next chunk",
                ));
            }

            let chunk = self.chunks[self.next_chunk];
            let length = chunk.len().min(buffer.len());
            buffer[..length].copy_from_slice(&chunk[..length]);
            if length != chunk.len() {
                return Err(io::Error::new(
                    io::ErrorKind::Other,
                    "test buffer is smaller than chunk",
                ));
            }
            self.next_chunk += 1;
            Ok(length)
        }
    }

    #[test]
    /// 验证 runtime token 使用随机十六进制密钥，避免固定值进入握手链路。
    fn generate_runtime_token_returns_random_hex_secret() {
        let first = generate_runtime_token();
        let second = generate_runtime_token();

        assert_eq!(first.len(), 64);
        assert_eq!(second.len(), 64);
        assert_ne!(first, second);
        assert!(first.chars().all(|item| item.is_ascii_hexdigit()));
        assert!(second.chars().all(|item| item.is_ascii_hexdigit()));
    }

    #[test]
    /// 验证 stdin 握手只携带 token、协议版本和 keepalive 标记，不把端口或路径等运行细节暴露给 Go core。
    fn build_handshake_line_only_contains_token_and_protocol_version() {
        let line = build_handshake_line("runtime-token").expect("handshake should serialize");
        let payload: Value =
            serde_json::from_str(line.trim_end()).expect("handshake should be json");

        assert!(line.ends_with('\n'));
        assert_eq!(payload["token"], "runtime-token");
        assert_eq!(payload["protocolVersion"], "1");
        assert_eq!(payload["keepaliveStdin"], true);
        assert_eq!(payload.as_object().expect("object").len(), 3);
    }

    #[test]
    /// 验证 ready JSON 的成功状态可以被 Rust 解析成固定结构。
    fn parse_ready_message_accepts_ready_payload() {
        let ready = parse_ready_message(
            r#"{"status":"ready","port":58123,"pid":1201,"protocolVersion":"1"}"#,
        )
        .expect("ready payload should parse");

        assert_eq!(ready.status, "ready");
        assert_eq!(ready.port, 58123);
        assert_eq!(ready.pid, 1201);
        assert_eq!(ready.protocol_version, "1");
    }

    #[test]
    /// 验证 ready JSON 必须带兼容协议版本，避免 desktop 与 core 二进制版本不匹配时继续启动。
    fn parse_ready_message_rejects_protocol_mismatch() {
        let err = parse_ready_message(
            r#"{"status":"ready","port":58123,"pid":1201,"protocolVersion":"2"}"#,
        )
        .expect_err("protocol mismatch must fail");

        assert!(matches!(
            err,
            SidecarError::ProtocolMismatch { expected, actual }
                if expected == "1" && actual == "2"
        ));
    }

    #[test]
    /// 验证非 ready 状态会快速失败，避免 Rust 在未就绪 sidecar 上继续发请求。
    fn parse_ready_message_rejects_non_ready_status() {
        let err = parse_ready_message(
            r#"{"status":"starting","port":58123,"pid":1201,"protocolVersion":"1"}"#,
        )
        .expect_err("non-ready status must fail");

        assert!(matches!(err, SidecarError::InvalidReadyStatus(status) if status == "starting"));
    }

    #[test]
    /// 验证内部 URL 只能从 CoreClient 的本机端口和固定路径生成。
    fn core_client_builds_internal_urls_from_single_source() {
        let client = CoreClient::new(58123, "runtime-token".to_string());

        assert_eq!(
            client.internal_url("/internal/health"),
            "http://127.0.0.1:58123/internal/health"
        );
        assert_eq!(client.token(), "runtime-token");
    }

    #[test]
    /// 验证 Rust 到 Go core 的内部请求始终带 token、request id 和 trace id。
    fn build_internal_headers_includes_token_request_id_and_trace_id() {
        let headers = build_internal_headers("runtime-token");

        assert_eq!(
            headers
                .get(TOKEN_HEADER)
                .expect("token header")
                .to_str()
                .expect("token header text"),
            "runtime-token"
        );
        assert!(headers.get("X-Request-Id").is_some());
        assert!(headers.get("X-Trace-Id").is_some());
    }

    #[test]
    /// 验证 SSE 读取在每个完整帧到达时立即回调，而不是等响应 EOF 后一次性处理。
    fn stream_sse_frames_handles_each_frame_before_reading_next_chunk() {
        let first_frame_seen = Rc::new(Cell::new(false));
        let reader = ChunkedSseRead {
            chunks: vec![
                b"id: 1\nevent: TASK_PROGRESS\ndata: {\"progress\":10}\n\n",
                b"id: 2\nevent: TASK_SUCCESS\ndata: {\"progress\":100}\n\n",
            ],
            next_chunk: 0,
            first_frame_seen: first_frame_seen.clone(),
        };
        let mut frames = Vec::new();

        let emitted = stream_sse_frames(BufReader::new(reader), |frame| {
            if frame.contains("id: 1") {
                first_frame_seen.set(true);
            }
            frames.push(frame.to_string());
            Ok(())
        })
        .expect("sse frames should stream");

        assert_eq!(emitted, 2);
        assert_eq!(frames.len(), 2);
        assert!(frames[0].contains("TASK_PROGRESS"));
        assert!(frames[1].contains("TASK_SUCCESS"));
    }

    #[test]
    /// 验证 SSE 长连接不配置总请求超时，避免长任务超过 60 秒时被 Rust 主动断开。
    fn sse_total_timeout_is_not_configured() {
        assert_eq!(sse_total_timeout(), None);
    }

    #[test]
    /// 验证 sidecar 启动参数必须携带 app data 目录作为 workspace，token 仍不进入 argv。
    fn build_core_start_args_includes_workspace_without_token() {
        let args = build_core_start_args(Path::new(
            "/Users/demo/Library/Application Support/InvestCompass",
        ));

        assert_eq!(
            args,
            vec![
                "--host".to_string(),
                "127.0.0.1".to_string(),
                "--port".to_string(),
                "0".to_string(),
                "--workspace".to_string(),
                "/Users/demo/Library/Application Support/InvestCompass".to_string(),
            ]
        );
        assert!(!args.iter().any(|item| item.contains("runtime-token")));
    }

    #[test]
    /// 验证本地开发覆盖路径优先，便于在不改配置文件的情况下调试 sidecar。
    fn resolve_core_binary_path_uses_env_override_first() {
        let path = resolve_core_binary_path(
            Some("/tmp/custom-core".to_string()),
            Path::new("/workspace/apps/desktop/src-tauri"),
            Some(Path::new("/Applications/InvestCompass.app/Contents/MacOS")),
        );

        assert_eq!(path, Path::new("/tmp/custom-core"));
    }

    #[test]
    /// 验证发布包运行时优先使用 `.app` / `.exe` 同目录的 sidecar，避免安装包依赖源码目录。
    fn resolve_core_binary_path_prefers_bundled_sibling_binary() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-bundled-core-{}",
            std::process::id()
        ));
        let executable_dir = root
            .join("InvestCompass.app")
            .join("Contents")
            .join("MacOS");
        std::fs::create_dir_all(&executable_dir).expect("create executable dir");
        let bundled_core = executable_dir.join(bundled_core_binary_file_name());
        std::fs::write(&bundled_core, "").expect("create bundled sidecar");

        let path = resolve_core_binary_path(
            None,
            Path::new("/workspace/apps/desktop/src-tauri"),
            Some(&executable_dir),
        );

        assert_eq!(path, bundled_core);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证默认 sidecar 路径收敛在 src-tauri/binaries 目录。
    fn resolve_core_binary_path_falls_back_to_project_binary_dir() {
        let path = resolve_core_binary_path(
            None,
            Path::new("/workspace/apps/desktop/src-tauri"),
            Some(Path::new("/Applications/InvestCompass.app/Contents/MacOS")),
        );

        assert_eq!(
            path,
            Path::new("/workspace/apps/desktop/src-tauri")
                .join("binaries")
                .join(core_binary_file_name())
        );
    }

    #[test]
    /// 验证 Tauri sidecar 文件名按目标平台 triple 命名，支撑后续打包配置。
    fn core_binary_file_name_for_target_uses_tauri_sidecar_triples() {
        assert_eq!(
            core_binary_file_name_for_target("aarch64-apple-darwin"),
            "invest-compass-core-aarch64-apple-darwin"
        );
        assert_eq!(
            core_binary_file_name_for_target("x86_64-apple-darwin"),
            "invest-compass-core-x86_64-apple-darwin"
        );
        assert_eq!(
            core_binary_file_name_for_target("x86_64-pc-windows-msvc"),
            "invest-compass-core-x86_64-pc-windows-msvc.exe"
        );
    }

    #[test]
    /// 验证 Go core stderr 会落到工作区 logs 目录，便于排查 sidecar 启动和异常退出。
    fn core_stderr_creates_workspace_log_file() {
        let root =
            std::env::temp_dir().join(format!("invest-compass-core-stderr-{}", std::process::id()));
        let stdio = core_stderr(&root).expect("core stderr should open");
        drop(stdio);

        assert!(root.join("logs").join("go-core-stderr.log").exists());
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    /// 验证 Rust sidecar 运行期诊断日志能写入工作区，便于排查旧端口和恢复流程。
    fn append_sidecar_runtime_log_writes_workspace_log() {
        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-runtime-log-{}",
            std::process::id()
        ));

        append_sidecar_runtime_log(
            &root,
            "api_transport_error path=/api/search/status failed_port=58123",
        );

        let content = std::fs::read_to_string(root.join("logs").join("sidecar-runtime.log"))
            .expect("runtime log should exist");
        assert!(content.contains("api_transport_error"));
        assert!(content.contains("/api/search/status"));
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证握手完成后 Rust 仍持有 stdin 并持续 drain stdout，避免 Go core 误判退出或被 SIGPIPE 杀掉。
    fn start_core_sidecar_keeps_stdio_alive_after_ready() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-stdio-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&root).expect("create stdin test dir");
        let eof_file = root.join("stdin-eof");
        let stdout_survived_file = root.join("stdout-survived");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nread handshake\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58123,\"pid\":1201,\"protocolVersion\":\"1\"}}'\nsleep 0.1\nprintf '%s\\n' 'after-ready-stdout'\nprintf survived > '{}'\nif ! read extra; then printf eof > '{}'; fi\n",
                stdout_survived_file.display(),
                eof_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let running =
            start_core_sidecar(&script, &root, Duration::from_secs(2)).expect("start fake core");
        std::thread::sleep(Duration::from_millis(300));

        assert!(
            !eof_file.exists(),
            "sidecar stdin must stay open while RunningCore is alive"
        );
        assert!(
            stdout_survived_file.exists(),
            "sidecar stdout must stay drained after ready to avoid SIGPIPE"
        );

        drop(running);
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证并发恢复启动只创建一个 sidecar，避免设置页多个初始化 command 竞争启动。
    fn core_state_start_is_serialized() {
        use std::os::unix::fs::PermissionsExt;
        use std::sync::Arc;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-serialized-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&root);
        std::fs::create_dir_all(&root).expect("create serialized test dir");
        let pid_file = root.join("pid-list");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nread handshake\nsleep 0.2\nprintf '%s\\n' $$ >> '{}'\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58123,\"pid\":1201,\"protocolVersion\":\"1\"}}'\nsleep 30\n",
                pid_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let state = Arc::new(CoreState::empty());
        let mut handles = Vec::new();
        for _ in 0..4 {
            let state = Arc::clone(&state);
            let script = script.clone();
            let root = root.clone();
            handles.push(std::thread::spawn(move || {
                state
                    .start(&script, &root, Duration::from_secs(5))
                    .expect("start fake core")
                    .port
            }));
        }

        let ports: Vec<u16> = handles
            .into_iter()
            .map(|handle| handle.join().expect("join start thread"))
            .collect();
        assert_eq!(ports, vec![58123, 58123, 58123, 58123]);

        let pid_list = std::fs::read_to_string(&pid_file).expect("pid list should exist");
        assert_eq!(
            pid_list.lines().count(),
            1,
            "concurrent start should spawn exactly one sidecar"
        );

        state.stop();
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证旧端口连接失败但子进程仍存活时不会重启，避免并发初始化互相杀 core。
    fn recover_after_failed_client_reuses_matching_running_port() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-recover-matching-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&root).expect("create recover test dir");
        let pid_file = root.join("pid-list");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nread handshake\nprintf '%s\\n' $$ >> '{}'\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58123,\"pid\":1201,\"protocolVersion\":\"1\"}}'\nsleep 30\n",
                pid_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let state = CoreState::empty();
        let failed_client = state
            .start(&script, &root, Duration::from_secs(2))
            .expect("start first fake core");
        let recovered = state
            .recover_after_failed_client(&failed_client, &script, &root, Duration::from_secs(2))
            .expect("recover fake core");

        assert_eq!(recovered.port(), 58123);
        let pid_list = std::fs::read_to_string(&pid_file).expect("pid list should exist");
        assert_eq!(
            pid_list.lines().count(),
            1,
            "matching running port should be reused without restarting sidecar"
        );

        state.stop();
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证旧端口连接失败且子进程已经退出时才会重启 sidecar。
    fn recover_after_failed_client_restarts_exited_matching_port() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-recover-exited-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&root);
        std::fs::create_dir_all(&root).expect("create exited recover test dir");
        let pid_file = root.join("pid-list");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nread handshake\nprintf '%s\\n' $$ >> '{}'\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58123,\"pid\":1201,\"protocolVersion\":\"1\"}}'\nif [ \"$(wc -l < '{}')\" -le 1 ]; then exit 0; fi\nsleep 30\n",
                pid_file.display(),
                pid_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let state = CoreState::empty();
        let failed_client = state
            .start(&script, &root, Duration::from_secs(5))
            .expect("start exiting fake core");
        std::thread::sleep(Duration::from_millis(100));
        let recovered = state
            .recover_after_failed_client(&failed_client, &script, &root, Duration::from_secs(5))
            .expect("recover exited fake core");

        assert_eq!(recovered.port(), 58123);
        let pid_list = std::fs::read_to_string(&pid_file).expect("pid list should exist");
        assert_eq!(
            pid_list.lines().count(),
            2,
            "exited matching port should restart sidecar once"
        );

        state.stop();
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证其他命令已经恢复出新端口时，旧 client 的失败不会误杀当前健康 sidecar。
    fn recover_after_failed_client_reuses_newer_running_port() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-recover-newer-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&root).expect("create newer recover test dir");
        let pid_file = root.join("pid-list");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nread handshake\nprintf '%s\\n' $$ >> '{}'\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58124,\"pid\":1201,\"protocolVersion\":\"1\"}}'\nsleep 30\n",
                pid_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let state = CoreState::empty();
        let current = state
            .start(&script, &root, Duration::from_secs(2))
            .expect("start current fake core");
        let stale_client = CoreClient::new(58123, "stale-token".to_string());
        let recovered = state
            .recover_after_failed_client(&stale_client, &script, &root, Duration::from_secs(2))
            .expect("reuse current fake core");

        assert_eq!(current.port(), 58124);
        assert_eq!(recovered.port(), 58124);
        let pid_list = std::fs::read_to_string(&pid_file).expect("pid list should exist");
        assert_eq!(
            pid_list.lines().count(),
            1,
            "different current port should be reused without spawning another sidecar"
        );

        state.stop();
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    /// 验证 sidecar ready 解析失败时会清理已启动子进程，避免留下孤儿本地服务。
    fn start_core_sidecar_cleans_child_when_ready_protocol_mismatches() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "invest-compass-sidecar-cleanup-{}",
            std::process::id()
        ));
        std::fs::create_dir_all(&root).expect("create cleanup test dir");
        let pid_file = root.join("pid");
        let script = root.join("fake-core.sh");
        std::fs::write(
            &script,
            format!(
                "#!/bin/sh\nprintf '%s\\n' $$ > '{}'\nprintf '%s\\n' '{{\"status\":\"ready\",\"port\":58123,\"pid\":1201,\"protocolVersion\":\"2\"}}'\nsleep 30\n",
                pid_file.display()
            ),
        )
        .expect("write fake core");
        let mut permissions = std::fs::metadata(&script).expect("metadata").permissions();
        permissions.set_mode(0o700);
        std::fs::set_permissions(&script, permissions).expect("chmod fake core");

        let error = match start_core_sidecar(&script, &root, Duration::from_secs(2)) {
            Ok(_) => panic!("protocol mismatch should fail"),
            Err(error) => error,
        };
        assert!(matches!(error, SidecarError::ProtocolMismatch { .. }));

        let pid = std::fs::read_to_string(&pid_file)
            .expect("pid should be written")
            .trim()
            .to_string();
        let still_running = std::process::Command::new("kill")
            .args(["-0", &pid])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .expect("kill -0 should run")
            .success();
        assert!(!still_running, "failed sidecar child should be cleaned up");

        let _ = std::fs::remove_dir_all(root);
    }
}
