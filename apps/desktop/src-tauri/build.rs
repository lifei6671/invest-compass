/// 执行 Tauri 官方构建脚本，保持桌面端资源和权限清单由 Tauri 统一生成。
fn main() {
    tauri_build::build();
}
