# 投研罗盘首版总体验收报告

> 日期：2026-06-18
> 状态：进行中，尚未达到内部测试发布标准。
> 范围：按技术方案第 19 章逐项记录当前证据、未执行项、风险和复测命令。

## 1. 结论

当前仓库已经完成 sidecar 安全启动基线、Rust 白名单 command 基线、Go core 多个纯规则模块、日志脱敏、更新链接 allowlist、Prompt 合规规则和发布用户手册等基础工作。

首版总体验收仍不能标记通过，原因是以下关键链路尚未形成真实闭环：

- SQLite migration、`sqlc + database/sql` storage 和数据库升级验证尚未完成。
- 系统凭据管理器适配尚未完成，macOS Keychain 和 Windows Credential Manager 保存/读取/删除未完成真实验收。
- 多数业务 API 仍停留在规则模块或部分 Go API，尚未全部接入 Rust 白名单 command。
- 前端页面主链路不在本轮“除前端外”开发目标内，但完整 T44 验收必须等待前端真实页面完成。
- macOS / Windows 真实安装、启动、打包、托盘、通知和 sidecar 随包验收尚未完成。

## 2. 当前自动化证据

本报告只记录当前仓库可重复执行的自动化检查，不把未执行的桌面人工验收写成通过。

- `cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml --check`
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
- `git diff --check`

后续每次进入发布候选前，还需要重新执行完整验证命令：

```bash
pnpm --dir apps format:check
pnpm --dir apps check
pnpm --dir apps test
pnpm --dir apps build
cd apps/sidecar-core && go test ./...
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
git diff --check
```

## 3. 技术方案第 19 章逐项验收

| 序号 | 验收标准 | 当前状态 | 当前证据 / 缺口 |
| --- | --- | --- | --- |
| 1 | 应用可在 Windows 和 macOS 启动。 | 未通过 | 尚未完成 macOS / Windows 真实桌面启动验收。 |
| 2 | Go sidecar 可自动启动和退出。 | 部分通过 | 已有 Rust sidecar 启动、stdin token、ready JSON、health/shutdown 基线和单测；随包启动、跨平台退出仍需验收。 |
| 3 | 用户可以搜索并添加自选股。 | 未通过 | Go 搜索 API 和自选股规则已有基础；数据库、Rust command、前端真实链路未闭环。 |
| 4 | 用户可以查看个股行情和 K 线。 | 未通过 | 行情 Provider 契约、缓存规则和指标计算已有基础；真实 Provider、持久化、Rust command、页面未闭环。 |
| 5 | 用户可以配置至少一个 AI 模型。 | 未通过 | AI 配置安全规则已有基础；系统凭据写入、数据库持久化、Rust command、页面未闭环。 |
| 6 | 用户可以基于股票生成 AI 分析报告。 | 未通过 | Prompt builder、分析请求和任务规则已有基础；真实数据拉取、AI 调用、任务执行 API、报告落库未闭环。 |
| 7 | 分析过程支持流式输出或进度展示。 | 未通过 | SSE 帧编码已有基础；Go SSE API、Rust 订阅转发、前端展示未闭环。 |
| 8 | 报告可以保存、查看、复制。 | 未通过 | 报告幂等和软删除规则已有基础；数据库持久化、Rust command、真实页面未闭环。 |
| 9 | 任务失败后可以看到错误原因。 | 未通过 | 统一错误和任务事件模型已有基础；任务执行链路和历史详情未闭环。 |
| 10 | 设置中心支持工作区、代理、通知、缓存。 | 未通过 | settings/cache 规则和部分 Go cache API 已有基础；真实 settings/workspace 存储、通知、代理密码凭据链路未闭环。 |
| 11 | 应用有明确风险提示。 | 部分通过 | 技术方案、发布指南、Prompt 合规规则已包含“仅作研究辅助，不构成投资建议”；UI 展示仍需前端验收。 |
| 12 | 数据库升级不会丢失用户已有数据。 | 未通过 | T08/T09 尚未完成，migration 可重复验证和升级备份策略未验收。 |
| 13 | API Key 存入系统凭据管理器，配置查询不回显真实 Key。 | 未通过 | Go 配置模型已拒绝真实 Key 落库和回显；系统凭据写入/删除未完成。 |
| 14 | 前端不能直接访问 Go sidecar，Rust command 必须白名单化。 | 部分通过 | 已有 `core_start`、`core_health` 白名单基线和安全配置测试；全部业务 command 尚未补齐。 |
| 15 | 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。 | 未执行 | 需要前端页面完成后逐屏检查；当前不能据此宣称通过。 |
| 16 | macOS 使用 Keychain，Windows 使用 Credential Manager，凭据保存/删除均通过验收。 | 未通过 | T20 未完成。验收时只能确认条目存在和删除成功，禁止打印明文 secret。 |
| 17 | Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。 | 未通过 | 相关 Go 规则已有基础；数据库、API、Rust command 和前端真实链路未全部闭环。 |
| 18 | 检查更新链接只允许 HTTPS allowlist 域名。 | 部分通过 | Go `updatecheck` 规则和单测已覆盖 HTTPS 与 allowlist；Rust command、远程 JSON 获取和设置页入口未闭环。 |
| 19 | 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。 | 未通过 | Dashboard/provider status Go API 有基础；资讯 API、Rust command 和真实页面未闭环。 |
| 20 | AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和持久化存储。 | 未通过 | Go 侧模型已避免真实 Key 落库和回显；Rust 凭据读取与内部注入链路未完成。 |

## 4. macOS 验收记录

尚未完成真实 macOS 验收。

待验收项：

- Apple Silicon 启动和退出。
- Intel 启动和退出。
- Go sidecar 随应用启动、退出后无残留进程。
- Keychain 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- 打包产物包含正确 sidecar 文件名。
- 脱敏日志导出。

## 5. Windows 验收记录

尚未完成真实 Windows 验收。

待验收项：

- Windows x64 启动和退出。
- Go sidecar 随应用启动、退出后无残留进程。
- Credential Manager 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- NSIS / MSI 安装包包含正确 sidecar 文件名。
- 脱敏日志导出。

## 6. 已知风险和遗留项

- 数据库层未落地前，业务模块只能验证规则，不能证明真实数据闭环。
- 凭据层未落地前，不能证明 API Key 和代理密码只存在系统凭据管理器和运行期内存。
- 真实行情/新闻 Provider 未确认数据源授权前，不能接入或宣称可用。
- 前端页面未完成前，不能验证“界面无 mock 数据”和“首版不出现未闭环入口”。
- 打包配置未完成前，不能证明安装包不依赖运行时下载 sidecar。

## 7. 复测入口

T44 重新验收时，按以下顺序执行：

1. 先通过 T08/T09/T20/T21/T25-T29/T39-T42 的任务级验收。
2. 执行完整自动化验证命令。
3. 在 macOS 和 Windows 上分别安装并启动真实桌面应用。
4. 逐项完成第 3 节 20 条验收标准。
5. 更新本报告的“当前状态”和平台验收记录。
6. 确认无未闭环入口、无 mock 数据、无明文 secret 后，才能把 T44 标记为 `[x]`。
