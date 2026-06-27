use serde::Serialize;
use std::process::Command;

#[derive(Serialize)]
pub struct OpenExternalURLResult {
    ok: bool,
}

/// 使用系统浏览器打开新闻外链，renderer 只能通过这个固定白名单 command 触发。
#[tauri::command]
pub async fn open_external_url(url: String) -> Result<OpenExternalURLResult, String> {
    let validated = validate_external_https_url(&url)?;
    tauri::async_runtime::spawn_blocking(move || open_system_browser(&validated))
        .await
        .map_err(|error| format!("open external url task failed: {error}"))??;
    Ok(OpenExternalURLResult { ok: true })
}

/// 校验外链 URL，确保桌面端只打开 HTTPS 且不包含空白/控制字符。
fn validate_external_https_url(url: &str) -> Result<String, String> {
    let trimmed = url.trim();
    if trimmed.is_empty() {
        return Err("external url must not be blank".to_string());
    }
    if trimmed.len() > 2048 {
        return Err("external url is too long".to_string());
    }
    if trimmed
        .chars()
        .any(|character| character.is_control() || character.is_whitespace())
    {
        return Err("external url must not contain whitespace or control characters".to_string());
    }
    let scheme_prefix = "https://";
    if !trimmed.to_ascii_lowercase().starts_with(scheme_prefix) {
        return Err("external url must use https scheme".to_string());
    }
    let host_and_path = &trimmed[scheme_prefix.len()..];
    if host_and_path.is_empty()
        || host_and_path.starts_with('/')
        || host_and_path.starts_with('?')
        || host_and_path.starts_with('#')
    {
        return Err("external url must include host".to_string());
    }
    Ok(trimmed.to_string())
}

/// 调用当前平台的系统浏览器入口；测试只覆盖校验逻辑，避免单测真实打开应用。
fn open_system_browser(url: &str) -> Result<(), String> {
    let status = if cfg!(target_os = "macos") {
        Command::new("open").arg(url).status()
    } else if cfg!(target_os = "windows") {
        Command::new("rundll32")
            .args(["url.dll,FileProtocolHandler", url])
            .status()
    } else {
        Command::new("xdg-open").arg(url).status()
    }
    .map_err(|error| format!("open external url failed: {error}"))?;

    if status.success() {
        Ok(())
    } else {
        Err("open external url command failed".to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    /// 验证外链 command 只接受 HTTPS URL，避免 renderer 传入不安全 scheme。
    fn validate_external_https_url_accepts_only_https() {
        assert_eq!(
            validate_external_https_url("https://example.com/news/1")
                .expect("https url should pass"),
            "https://example.com/news/1"
        );
        assert_eq!(
            validate_external_https_url("http://example.com/news/1")
                .expect_err("http url should fail"),
            "external url must use https scheme"
        );
        assert_eq!(
            validate_external_https_url("https://example.com/news 1")
                .expect_err("space url should fail"),
            "external url must not contain whitespace or control characters"
        );
        assert_eq!(
            validate_external_https_url("https://").expect_err("missing host should fail"),
            "external url must include host"
        );
        assert_eq!(
            validate_external_https_url("   ").expect_err("blank url should fail"),
            "external url must not be blank"
        );
    }
}
