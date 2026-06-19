# 投研罗盘首版总体验收报告

> 日期：2026-06-18
> 状态：进行中，尚未达到内部测试发布标准。
> 范围：按技术方案第 19 章逐项记录当前证据、未执行项、风险和复测命令。

## 1. 结论

当前仓库已经完成 sidecar 安全启动基线、Rust 白名单 command 基线、Go core 主要 service/action/dao 基线、日志脱敏、更新链接 allowlist、Prompt 合规规则、AI Provider、任务执行、SSE 转发、股票/自选股/行情 API 基线和发布用户手册等基础工作。

首版总体验收仍不能标记通过，原因是以下关键链路尚未形成真实闭环：

- GORM CRUD dao、settings/workspace/cache API、主要 service 业务编排和对应 Rust 白名单 command 基线已有自动化验证；迁移前 SQLite 备份已有自动化基线，发布后真实升级和恢复演练尚未完成。
- 平台凭据服务强依赖已移除，首版改为 Rust 本地文件 vault；AI Key 保存、读取、删除、模型测试注入和代理密码保存/删除已有 Rust 单测，真实桌面验收仍未闭环。
- 主要业务 API 已接入 Rust 白名单 command；AI 配置连通性测试、分析任务执行、取消传播、报告保存和 SSE 转发已有 Go/Rust 自动化基线，真实 Market/News Provider、跨平台凭据验收等关键链路尚未全部闭环。
- 前端页面主链路不在本轮“除前端外”开发目标内，但完整 T44 验收必须等待前端真实页面完成。
- macOS / Windows 真实安装、启动、托盘、通知和跨平台 sidecar 随包验收尚未完成；当前已完成 Tauri `externalBin` 配置、本机 Apple Silicon sidecar 构建基线，以及三类首版 sidecar target 选择脚本。

## 2. 当前自动化证据

本报告只记录当前仓库可重复执行的自动化检查，不把未执行的桌面人工验收写成通过。

- `cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml --check`
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
- `pnpm --dir apps format:check`
- `pnpm --dir apps check`
- `pnpm --dir apps build`
- `pnpm --dir apps --filter @invest-compass/desktop exec tauri build --bundles app`
- `cd apps/sidecar-core && go test ./...`
- `git diff --check`

后续每次进入发布候选前，还需要重新执行完整验证命令：

```bash
pnpm --dir apps format:check
pnpm --dir apps check
pnpm --dir apps test
pnpm --dir apps build
pnpm --dir apps --filter @invest-compass/desktop exec tauri build --bundles app
cd apps/sidecar-core && go test ./...
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
git diff --check
```

## 3. 技术方案第 19 章逐项验收

| 序号 | 验收标准 | 当前状态 | 当前证据 / 缺口 |
| --- | --- | --- | --- |
| 1 | 应用可在 Windows 和 macOS 启动。 | 部分通过 | 本机 Apple Silicon 已真实启动 `投研罗盘.app`；macOS Intel 和 Windows 真实桌面启动验收仍未完成。 |
| 2 | Go sidecar 可自动启动和退出。 | 部分通过 | 已有 Rust sidecar 启动、stdin token、携带 `protocolVersion` 的 ready JSON、protocolVersion 兼容性校验、health/shutdown 基线和单测；Tauri `externalBin` 已声明 Go sidecar 基名，`sidecar:build` 已实际生成 Apple Silicon、macOS Intel 和 Windows x64 三类 target sidecar，其中 Windows x64 为 GUI subsystem；本机 `投研罗盘.app` 已包含 `Contents/MacOS/invest-compass-core`，包内 sidecar 通过 stdin 握手、`protocolVersion=1` ready 和 shutdown 回环验证；真实启动后进程参数确认 sidecar 来自 `.app/Contents/MacOS/invest-compass-core`，应用退出后 desktop/core 均无残留进程；macOS Intel 和 Windows 安装包仍需目标平台验收。 |
| 3 | 用户可以搜索并添加自选股。 | 未通过 | Go 搜索 API、自选股 CRUD API、stocks/watchlists 持久化和 Rust `stock_search`/`watchlist_*` command 已接入；真实合规 Provider 和前端真实链路未闭环。 |
| 4 | 用户可以查看个股行情和 K 线。 | 未通过 | Go `POST /api/market/quote`、`POST /api/market/kline`、quotes/klines 持久化缓存和 Rust `market_quote`/`market_kline` command 已接入；真实合规 Provider 和个股详情页真实展示未闭环。 |
| 5 | 用户可以配置至少一个 AI 模型。 | 未通过 | AI 配置安全规则、Rust 本地 vault 写入/读取、数据库持久化、生产 `OpenAIConfigTester` 注入、`POST /api/ai/configs/test` 和 Rust `ai_config_*` command 已有基线；模型配置页面真实交互和真实外部 Provider 桌面验收未闭环。 |
| 6 | 用户可以基于股票生成 AI 分析报告。 | 部分通过 | Prompt builder、分析请求和任务规则已有基础；`POST /api/analysis/tasks`、`POST /api/tasks/cancel` 和 Rust `analysis_task_*` command 已接入；分析执行器可读取缓存行情/K 线/新闻、构建合规 Prompt、调用 OpenAI-compatible Provider 并按 `task_id` 保存报告；报告 `input_snapshot` 会保留本次一次性持仓输入但不保存运行期 Key；取消 API 已能传播到运行中 executor context 且不会把取消写成失败；真实 Market/News Provider 填充和前端触发未闭环。 |
| 7 | 分析过程支持流式输出或进度展示。 | 部分通过 | 分析执行器会持久化 `TASK_STARTED`、`TASK_CHUNK`、`TASK_SUCCESS` / `TASK_FAILED` 事件；`POST /api/tasks/events/stream` 可输出脱敏 SSE 帧，首次无新增事件时会等待新事件并在终态事件后结束；Rust `analysis_task_subscribe` 可读取该流并 emit `analysis-task-event`，Rust SSE 解析已覆盖多行 `data:` 和 CRLF 帧边界；前端展示未闭环。 |
| 8 | 报告可以保存、查看、复制。 | 部分通过 | 报告幂等、软删除规则、`POST /api/reports/*` 和 Rust `report_*` command 已闭环；报告查询默认不返回完整 `input_snapshot`，Markdown 导出默认不包含一次性持仓输入，显式导出快照能力已有 service 规则但 UI 入口仍待 T36 前端接入。 |
| 9 | 任务失败后可以看到错误原因。 | 部分通过 | 统一错误和任务事件模型已有基础，任务创建/取消和分析执行失败均会写入持久化事件并更新任务错误；历史详情页面展示未闭环。 |
| 10 | 设置中心支持工作区、代理、通知、缓存。 | 部分通过 | Go settings/workspace/cache API 已接入真实 `dao.Store`，Rust settings/workspace/cache 白名单 command 已接入，`workspace_set` 已在 Rust 边界拒绝空路径和相对路径；`settings_set` 已支持代理密码写入/删除本地 vault 并只转发 `proxy_credential_ref`，且会在读写 vault 前拒绝同一请求同时写入新代理密码和清理旧凭据；前端设置页、通知和开机自启未闭环。 |
| 11 | 应用有明确风险提示。 | 部分通过 | 技术方案、发布指南、Prompt 合规规则已包含“仅作研究辅助，不构成投资建议”；UI 展示仍需前端验收。 |
| 12 | 数据库升级不会丢失用户已有数据。 | 部分通过 | T08 已完成空库迁移、重复迁移和唯一约束自动化验证；Go sidecar 生产启动会在 `dao.Open` / `dao.Migrate` 前把已有 SQLite 主库备份到工作区 `backups/` 目录，并拒绝覆盖同名备份；T09 CRUD repository 已有自动化覆盖，发布后真实升级和恢复演练尚未完成。 |
| 13 | API Key 存入 Rust 本地文件 vault，配置查询不回显真实 Key。 | 部分通过 | Go 配置模型和 `POST /api/ai/configs/list|save|delete|test` 已拒绝真实 Key 落库和回显，并在保存前拒绝非 `local-vault://ai-config/` 的 `api_key_ref`；Rust `ai_config_save` 已将一次性 Key 写入本地 vault 并只转发引用，新建配置尚无数据库 ID 时会生成唯一 `api_key_ref`，避免多个 `id=0` 新配置覆盖同一密钥文件；更新 API Key 时会先校验旧 `api_key_ref`，再写入新密钥并删除被替换的旧引用，旧引用非法时不会写入新 secret，旧引用删除失败时会回滚本次新写入文件；本地 vault 目录和 secret 文件写入已统一收口，Unix/macOS 下目录强制收紧为 `0700`，secret 文件创建和更新后会强制收紧为 `0600`；本地 vault 会拒绝清理后为空的 AI Provider、引用和代理 profile，避免退化为弱语义文件名或落到隐式 `.secret` 文件；Go core 会拒绝非空且没有脱敏标记的 `masked_api_key`，避免明文 Key 通过展示字段旁路落库；Go settings 会拒绝未知 `_ref`、非 `local-vault://proxy/` 的代理凭据引用和未脱敏的 `masked_api_key`；`ai_config_delete` 支持按 `api_key_ref` 删除本地 vault 文件，并已在删除 vault 前拒绝非正数配置 ID，`ai_config_test` 支持读取本地 vault 后内部注入 `resolved_api_key`；代理密码 vault 已接入 settings command，并覆盖冲突写入/清理动作早失败；自动化基线通过，真实桌面验收仍待完成。 |
| 14 | 前端不能直接访问 Go sidecar，Rust command 必须白名单化。 | 部分通过 | 已有 `core_start`、`core_health`、settings/workspace/cache、`stock_search`、`watchlist_*`、market/news/dashboard/provider/prompt 模板、AI config、analysis task、task/report、update/log export 白名单 command 和安全配置测试；`stock_search` 已在 Rust 边界拒绝空 keyword，workspace/log export 已在 Rust 边界拒绝无效本地路径，market/news/task 相关 Rust command 已在转发前拒绝无界 `limit`、非法事件游标和空 `task_id`，analysis task 创建已在读取 vault 前拒绝空 `symbol`、空 `analysis_type`、非 `local-vault://ai-config/` 的 `api_key_ref` 和非法配置 ID，watchlist/prompt/report/AI config 相关 Rust command 已拒绝非正数 ID；Go core 业务 API 已统一限制 JSON 请求体大小，仅接受 JSON object，并拒绝 `null`、尾随内容和未知字段，超限返回 `41300/request_body_too_large` 且不进入业务层；前端 `coreClient` 已通过 typed invoke service 调用 `core_health`，源码扫描禁止直连 Go core；业务页面完成后仍需逐屏确认没有绕过 typed invoke service。 |
| 15 | 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。 | 未执行 | 需要前端页面完成后逐屏检查；当前不能据此宣称通过。 |
| 16 | macOS / Windows 使用 Rust 本地文件 vault，凭据保存/删除均通过验收。 | 部分通过 | Rust 本地 vault 保存/读取/删除已有单测，覆盖 AI Key、无数据库 ID 新配置唯一 `api_key_ref`、更新 key 清理旧引用、旧引用非法时不写入新密钥、空 AI Provider 和空/非法清理后的 vault 引用拒绝、Unix/macOS vault 目录 `0700` 和 secret 文件 `0600` 权限、代理密码和代理凭据冲突动作早失败；Go core 单测覆盖 AI 配置和 settings 只接受本地 vault scheme 的凭据引用，并拒绝未脱敏的 `masked_api_key`；macOS / Windows 真实桌面路径、权限和手工验收仍待完成。 |
| 17 | Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。 | 部分通过 | 技术指标 `POST /api/market/indicators` 和 Rust `market_indicators` 已闭环；Prompt 模板 `POST /api/prompt-templates/*` 和 Rust `prompt_templates_*` 已闭环，模板 ID 已在 Rust 边界拒绝非正数；分析任务创建/取消与任务历史 `POST /api/analysis/tasks`、`POST /api/tasks/*` 和 Rust `analysis_task_*` / `task_*` 已有基线，任务列表已拒绝非法 `limit` 以避免无界查询，任务详情/取消/事件订阅已在 Rust 边界拒绝空 `task_id`，任务事件回放已拒绝负数 `after_event_id`，Rust command 已在转发前做同样入参早失败；报告历史 `POST /api/reports/*` 和 Rust `report_*` 已闭环，报告 ID 已在 Rust 边界拒绝非正数；前端真实页面仍待 T36。 |
| 18 | 检查更新链接只允许 HTTPS allowlist 域名。 | 部分通过 | Go `updatecheck` 规则、`POST /api/update/check`、settings 配置来源、远程 manifest 获取边界和 Rust `check_update` 已覆盖 HTTPS 与 allowlist；远程 manifest 超过 256 KiB 会被拒绝，不接受截断内容，且 manifest 拉取不跟随 HTTP 3xx 重定向；设置页入口和真实用户配置保存未闭环。 |
| 19 | 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。 | 部分通过 | Dashboard/provider status Go API 与 Rust `dashboard_summary`、`providers_status` 已接入；`dashboard_summary` 生产路径已从真实 `dao.Store` 聚合 active 自选股、最新行情、最近报告、任务、市场新闻和 Market/News Provider 状态，未配置真实 Provider 时分别返回 `available=false` / `source=unconfigured` 状态，新闻 Provider 实现可选 `Status(ctx)` 时会返回 Provider 自身可用性而不是默认假定可用；新闻 `POST /api/news/list`、`POST /api/news/market` 和 Rust `news_list`、`news_market` 已接入，Rust command 已拒绝非法新闻 `limit`；真实 Provider 和前端页面未闭环。 |
| 20 | AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和 SQLite。 | 部分通过 | Go 侧模型和 AI 配置 API 已避免真实 Key 落库和回显，前端/shared 扫描禁止内部密钥字段；Rust `ai_config_test` 和 `analysis_task_create` 已能从本地 vault 读取 Key 并只在内部请求中注入 `resolved_api_key`，且 `analysis_task_create` 会先拒绝空基础字段、非 `local-vault://ai-config/` 的 `api_key_ref` 和非法配置 ID 再读取 vault；分析执行器只在当前请求内存中使用该 Key 调用 AI，报告输入快照可保存一次性持仓但不保存明文 Key；真实桌面链路仍待验收。 |

## 4. macOS 验收记录

尚未完成完整真实 macOS 验收；Apple Silicon `.app` 启动、包内 sidecar 路径和退出清理已有本机证据。

已完成的本机证据：

- `tauri build --bundles app` 已生成 `投研罗盘.app`。
- `投研罗盘.app/Contents/MacOS/` 同时包含 `invest-compass-desktop` 和 `invest-compass-core`。
- 包内 `invest-compass-core` 已通过 stdin 握手、`protocolVersion=1` ready JSON 和 `/internal/shutdown` 关闭验证。
- 真实启动 `投研罗盘.app` 后，`ps` 进程参数确认 Go core 路径为 `.app/Contents/MacOS/invest-compass-core`。
- 通过 macOS 应用退出路径关闭后，`pgrep -af "invest-compass-(desktop|core)"` 无残留进程。

待验收项：

- Intel 启动和退出。
- 本地 vault 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- 脱敏日志导出。

## 5. Windows 验收记录

尚未完成真实 Windows 验收。

待验收项：

- Windows x64 启动和退出。
- Go sidecar 随应用启动、退出后无残留进程。
- 本地 vault 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- NSIS / MSI 安装包包含正确 sidecar 文件名。
- 脱敏日志导出。

## 6. 已知风险和遗留项

- GORM schema/migration、迁移前 SQLite 备份、关键 CRUD repository、settings/workspace 真实存储和 cache cleaner 基线已落地，现有业务模块已迁入 `internal/service/<module>`，HTTP 路由和 handler 已拆入 `internal/actions`，`internal/server` 已收缩为监听和启动层，公共日志能力已迁入 `pkg/logger`，通用错误码和错误类型已迁入 `pkg/xerr`；真实 Market/News Provider 未确认前，行情、新闻和分析上下文仍不能证明完整真实数据闭环。
- 日志导出已具备 Go core 内存 slog 日志源、二次脱敏包、Rust `export_logs(target_dir)` 写入目标目录、目标目录早校验、路径穿越文件名拦截、同名文件不覆盖保护和 Unix/macOS `0600` 私有文件权限；真实目录选择入口和跨平台手工验收仍待完成。
- 本地 vault 安全性低于平台凭据服务；首版只能证明前端、Go core、SQLite、日志和报告不直接保存真实 Key，且 Unix/macOS vault 目录权限已收紧为 `0700`、secret 文件权限已收紧为 `0600`，但仍不能抵御当前用户账号下的本机文件读取风险。
- 当前非 UI 自动化基线的停止线已经明确：继续推进 T13/T18 需要确认真实 Market/News Provider 数据源和授权；继续推进 T39 需要确认 Tauri 托盘、通知、开机自启相关 feature/plugin 与 capability；T31-T38、T40/T41 的用户触发和 T44 的未闭环入口检查必须等待真实页面完成后验收。
- 真实行情/新闻 Provider 未确认数据源授权前，不能接入或宣称可用。
- 前端基础外壳和 `core_health` 启动检查已接入；具体业务页面未完成前，仍不能验证“界面无 mock 数据”和“首版不出现未闭环入口”。
- 关闭到托盘的 settings 读取、防误隐藏决策和主窗口位置/尺寸恢复已接入 Rust 运行期；真正托盘菜单、通知和开机自启仍需确认 Tauri feature/plugin 与 capability 配置后实现。
- Tauri `externalBin`、三类首版 target sidecar 实际构建基线、Windows GUI subsystem sidecar 构建参数和 Apple Silicon `.app` 包内 sidecar 启动/关闭验证已落地；macOS Intel 和 Windows 安装包随包启动仍未验收，暂不能证明所有平台安装包都不依赖运行时下载 sidecar。

## 7. 复测入口

T44 重新验收时，按以下顺序执行：

1. 先通过 T13/T18/T21/T26/T27/T39-T42 的任务级剩余验收。
2. 执行完整自动化验证命令。
3. 在 macOS 和 Windows 上分别安装并启动真实桌面应用。
4. 逐项完成第 3 节 20 条验收标准。
5. 更新本报告的“当前状态”和平台验收记录。
6. 确认无未闭环入口、无 mock 数据、无明文 secret 后，才能把 T44 标记为 `[x]`。
