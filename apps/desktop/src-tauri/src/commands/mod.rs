pub mod ai_config;
pub mod autostart;
pub mod blocking;
pub mod boot;
pub mod core;
pub mod dashboard;
pub mod data_source_credentials;
pub mod external;
pub mod logs;
pub mod market;
pub mod news;
pub mod notifications;
pub mod prompt;
pub mod providers;
pub mod reports;
pub mod scheduler;
pub mod search;
pub mod settings;
pub mod task_logs;
pub mod tasks;
pub mod update;
pub mod watchlist;

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::Path;

    #[test]
    /// 验证所有 Tauri command 都使用 async 边界，避免新增同步阻塞 IPC 调用。
    fn tauri_commands_must_be_async() {
        let commands_dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("src/commands");
        for entry in fs::read_dir(&commands_dir).expect("commands dir should be readable") {
            let path = entry.expect("command file should be readable").path();
            if path.extension().and_then(|item| item.to_str()) != Some("rs") {
                continue;
            }
            if matches!(
                path.file_name().and_then(|item| item.to_str()),
                Some("mod.rs" | "blocking.rs")
            ) {
                continue;
            }
            let source = fs::read_to_string(&path).expect("command source should be readable");
            let lines: Vec<&str> = source.lines().collect();
            for (index, line) in lines.iter().enumerate() {
                if !line.contains("#[tauri::command]") {
                    continue;
                }
                let signature = lines
                    .iter()
                    .skip(index + 1)
                    .find(|candidate| {
                        let trimmed = candidate.trim();
                        !trimmed.is_empty() && !trimmed.starts_with("///")
                    })
                    .copied()
                    .unwrap_or("");
                assert!(
                    signature.trim_start().starts_with("pub async fn "),
                    "{}:{} command must be declared pub async fn, got {}",
                    path.display(),
                    index + 1,
                    signature.trim()
                );
            }
        }
    }
}
