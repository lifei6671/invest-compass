use rand::{rngs::OsRng, RngCore};
use reqwest::header::{HeaderMap, HeaderValue};
use serde::{Deserialize, Serialize};
use std::{
    fmt,
    io::{BufRead, BufReader, Write},
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    sync::{mpsc, Mutex},
    time::Duration,
};

const PROTOCOL_VERSION: &str = "1";
const TOKEN_BYTES: usize = 32;
const TOKEN_HEADER: &str = "X-Invest-Compass-Token";
const CORE_BINARY_ENV: &str = "INVEST_COMPASS_CORE_BIN";

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
        }
    }
}

impl std::error::Error for SidecarError {}

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
}

#[derive(Debug, Deserialize)]
pub struct ReadyMessage {
    pub status: String,
    pub port: u16,
    pub pid: u32,
}

#[derive(Clone)]
pub struct CoreClient {
    port: u16,
    token: String,
    http_client: reqwest::blocking::Client,
}

pub struct RunningCore {
    client: CoreClient,
    child: Child,
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
        let guard = self.running.lock().expect("core state poisoned");
        guard
            .as_ref()
            .map(|running| running.client.clone())
            .ok_or(SidecarError::CoreNotRunning)
    }

    /// 按固定二进制路径启动 sidecar，已启动时直接复用当前客户端。
    pub fn start(
        &self,
        binary_path: &Path,
        ready_timeout: Duration,
    ) -> Result<CoreClient, SidecarError> {
        if let Ok(client) = self.client() {
            return Ok(client);
        }

        let running = start_core_sidecar(binary_path, ready_timeout)?;
        let client = running.client.clone();
        self.install(running);
        Ok(client)
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
            http_client: reqwest::blocking::Client::new(),
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

    /// 调用 Go core 健康检查接口，所有请求都带固定 token header。
    pub fn health(&self) -> Result<serde_json::Value, SidecarError> {
        self.post_internal("/internal/health")
    }

    /// 请求 Go core 优雅关闭，失败时由进程管理层继续执行 kill 兜底。
    pub fn shutdown(&self) -> Result<serde_json::Value, SidecarError> {
        self.post_internal("/internal/shutdown")
    }

    /// 执行内部 POST 请求，确保前端无法传入任意 path 或 method。
    fn post_internal(&self, path: &str) -> Result<serde_json::Value, SidecarError> {
        let response = self
            .http_client
            .post(self.internal_url(path))
            .headers(build_internal_headers(&self.token))
            .json(&serde_json::json!({}))
            .send()?
            .error_for_status()?
            .json()?;

        Ok(response)
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

/// 解析 Go core 二进制路径，环境变量优先，默认回到项目内 `src-tauri/binaries`。
pub fn resolve_core_binary_path(env_override: Option<String>, manifest_dir: &Path) -> PathBuf {
    if let Some(path) = env_override {
        return PathBuf::from(path);
    }

    manifest_dir.join("binaries").join(core_binary_file_name())
}

/// 获取运行期 Go core 二进制路径，作为 setup 和 command 的唯一来源。
pub fn runtime_core_binary_path() -> PathBuf {
    resolve_core_binary_path(
        std::env::var(CORE_BINARY_ENV).ok(),
        Path::new(env!("CARGO_MANIFEST_DIR")),
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
    Ok(ready)
}

/// 启动 Go sidecar，完成 stdin token 握手，并从 stdout 读取 ready JSON。
pub fn start_core_sidecar(
    binary_path: &Path,
    ready_timeout: Duration,
) -> Result<RunningCore, SidecarError> {
    let token = generate_runtime_token();
    let mut child = Command::new(binary_path)
        .arg("--host")
        .arg("127.0.0.1")
        .arg("--port")
        .arg("0")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()?;

    let mut stdin = child.stdin.take().ok_or(SidecarError::MissingChildStdin)?;
    stdin.write_all(build_handshake_line(&token)?.as_bytes())?;
    drop(stdin);

    let stdout = child
        .stdout
        .take()
        .ok_or(SidecarError::MissingChildStdout)?;
    let ready_line = read_ready_line(stdout, ready_timeout)?;
    let ready = parse_ready_message(&ready_line)?;

    Ok(RunningCore {
        client: CoreClient::new(ready.port, token),
        child,
    })
}

/// 在限定时间内读取 ready 行，避免 Tauri 启动阶段无限等待 sidecar。
fn read_ready_line(
    stdout: std::process::ChildStdout,
    ready_timeout: Duration,
) -> Result<String, SidecarError> {
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        let mut line = String::new();
        let result = BufReader::new(stdout).read_line(&mut line).map(|_| line);
        let _ = sender.send(result);
    });

    match receiver.recv_timeout(ready_timeout) {
        Ok(result) => result.map_err(SidecarError::Io),
        Err(mpsc::RecvTimeoutError::Timeout) => Err(SidecarError::ReadyTimeout),
        Err(mpsc::RecvTimeoutError::Disconnected) => Err(SidecarError::ReadyTimeout),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;

    #[test]
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
    fn build_handshake_line_only_contains_token_and_protocol_version() {
        let line = build_handshake_line("runtime-token").expect("handshake should serialize");
        let payload: Value =
            serde_json::from_str(line.trim_end()).expect("handshake should be json");

        assert!(line.ends_with('\n'));
        assert_eq!(payload["token"], "runtime-token");
        assert_eq!(payload["protocolVersion"], "1");
        assert_eq!(payload.as_object().expect("object").len(), 2);
    }

    #[test]
    fn parse_ready_message_accepts_ready_payload() {
        let ready = parse_ready_message(r#"{"status":"ready","port":58123,"pid":1201}"#)
            .expect("ready payload should parse");

        assert_eq!(ready.status, "ready");
        assert_eq!(ready.port, 58123);
        assert_eq!(ready.pid, 1201);
    }

    #[test]
    fn parse_ready_message_rejects_non_ready_status() {
        let err = parse_ready_message(r#"{"status":"starting","port":58123,"pid":1201}"#)
            .expect_err("non-ready status must fail");

        assert!(matches!(err, SidecarError::InvalidReadyStatus(status) if status == "starting"));
    }

    #[test]
    fn core_client_builds_internal_urls_from_single_source() {
        let client = CoreClient::new(58123, "runtime-token".to_string());

        assert_eq!(
            client.internal_url("/internal/health"),
            "http://127.0.0.1:58123/internal/health"
        );
        assert_eq!(client.token(), "runtime-token");
    }

    #[test]
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
    fn resolve_core_binary_path_uses_env_override_first() {
        let path = resolve_core_binary_path(
            Some("/tmp/custom-core".to_string()),
            Path::new("/workspace/apps/desktop/src-tauri"),
        );

        assert_eq!(path, Path::new("/tmp/custom-core"));
    }

    #[test]
    fn resolve_core_binary_path_falls_back_to_project_binary_dir() {
        let path = resolve_core_binary_path(None, Path::new("/workspace/apps/desktop/src-tauri"));

        assert_eq!(
            path,
            Path::new("/workspace/apps/desktop/src-tauri")
                .join("binaries")
                .join(core_binary_file_name())
        );
    }

    #[test]
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
}
