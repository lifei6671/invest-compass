# 投研罗盘首版总体验收报告

> 日期：2026-06-18
> 状态：进行中，尚未达到内部测试发布标准。
> 范围：按技术方案第 19 章逐项记录当前证据、未执行项、风险和复测命令。

## 1. 结论

当前仓库已经完成 sidecar 安全启动基线、Rust 白名单 command 基线、Go core 主要 service/action/dao 基线、日志脱敏、更新链接 allowlist、Prompt 合规规则、AI Provider、任务执行、SSE 转发、股票/自选股/行情 API 基线和发布用户手册等基础工作。

首版总体验收仍不能标记通过，原因是以下关键链路尚未形成真实闭环：

- GORM CRUD dao、settings/workspace/cache API、主要 service 业务编排和对应 Rust 白名单 command 基线已有自动化验证；迁移前 SQLite 备份和备份恢复后重新迁移读取用户 settings 数据已有自动化基线，发布后真实升级演练尚未完成。
- 平台凭据服务强依赖已移除，首版改为 Rust 本地文件 vault；AI Key 保存、读取、删除、模型测试注入和代理密码保存/删除已有 Rust 单测，真实桌面验收仍未闭环。
- 主要业务 API 已接入 Rust 白名单 command；AI 配置连通性测试、分析任务执行、取消传播、报告保存和 SSE 转发已有 Go/Rust 自动化基线，真实 Market/News Provider、跨平台凭据验收等关键链路尚未全部闭环。
- 数据刷新调度专题已落地 Go `gocron/v2` 调度服务、SQLite job/run/watermark、启动补偿、手动补偿、单股刷新复用队列、Rust 白名单 command 和 `/scheduler` 管理入口；股票详情页已接入行情/K 线/指标/新闻主链路，新闻外链已通过 Rust 白名单 command 打开系统浏览器，真实 Provider 授权和跨平台桌面人工验收仍未闭环。
- 前端页面主链路已接入真实 command；Dashboard、自选股、个股详情、资讯中心、任务调度、AI 设置、AI 分析、报告历史、任务历史和设置页面已有自动化基线，但完整 T44 验收必须等待真实 Provider 和跨平台桌面验收完成。
- macOS / Windows 真实安装、启动、托盘、通知和跨平台 sidecar 随包验收尚未完成；当前已完成 Tauri `externalBin` 配置、本机 Apple Silicon sidecar 构建基线，以及三类首版 sidecar target 构建和产物校验脚本。

## 2. 当前自动化证据

本报告只记录当前仓库可重复执行的自动化检查，不把未执行的桌面人工验收写成通过。

- `cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml --check`
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
- `pnpm --dir apps format:check`
- `pnpm --dir apps check`
- `pnpm --dir apps build`
- `pnpm --dir apps sqlite:upgrade-rehearsal`
- `pnpm --dir apps sidecar:check-targets`
- `pnpm --dir apps --filter @invest-compass/desktop exec tauri build --bundles app`
- `cd apps/sidecar-core && go test ./...`
- `git diff --check`

后续每次进入发布候选前，还需要重新执行完整验证命令：

```bash
pnpm --dir apps release:check:local
```

展开后等价于本机 Apple Silicon 发布候选自动化链路：

```bash
pnpm --dir apps acceptance:check
pnpm --dir apps sqlite:upgrade-rehearsal
pnpm --dir apps sidecar:check-targets
pnpm --dir apps build
pnpm --dir apps --filter @invest-compass/desktop exec tauri build --bundles app
pnpm --dir apps package:verify -- --platform=darwin --target=aarch64-apple-darwin "desktop/src-tauri/target/release/bundle/macos/投研罗盘.app"
pnpm --dir apps sidecar:smoke
git diff --check
```

`acceptance:check` 聚合本地测试、类型/编译检查、Rust 桌面单元测试和格式检查，不执行联网 Provider smoke、真实打包包结构复核或跨平台桌面人工验收。`sqlite:upgrade-rehearsal` 会创建用户 SQLite、迁移、备份、恢复到新库、再次迁移并验证 settings 数据保留，用于发布前重复演练数据库升级恢复链路；它仍不替代真实安装包升级已有用户 profile 的手工验收。`sidecar:check-targets` 会重复构建并校验 Apple Silicon、macOS Intel 和 Windows x64 三类首版 sidecar target 产物，覆盖文件存在性、非 symlink、非空、macOS 执行位、Mach-O / PE 架构和 Windows GUI subsystem，但不证明对应安装包可在目标平台启动。`release:check:local` 只聚合本机 Apple Silicon 自动化链路，不替代 macOS Intel、Windows 或桌面能力人工验收。`package:verify` 只校验生成包里的主程序、Go sidecar、macOS `.app` 包名与 Tauri `productName` 一致、macOS `Info.plist` 可启动元数据、`CFBundlePackageType=APPL`、`CFBundleDisplayName` / `CFBundleName` 与 Tauri `productName` 一致、`CFBundleIdentifier` / `CFBundleShortVersionString` / `CFBundleVersion` 与 Tauri 配置一致、`CFBundleIconFile` 对应 `Contents/Resources` 直接子文件、执行位、包根是目录、包根、macOS `Contents` / `Contents/MacOS` 关键目录、声明图标时的 `Contents/Resources` 目录、`Info.plist` 和包内二进制不是 symlink、目标架构和 Windows GUI subsystem。`sidecar:smoke` 会启动包内 Go core，完成 stdin token 握手、`/internal/health` 和 `/internal/shutdown`；这些脚本仍不替代真实桌面启动验收。

## 3. 技术方案第 19 章逐项验收

| 序号 | 验收标准 | 当前状态 | 当前证据 / 缺口 |
| --- | --- | --- | --- |
| 1 | 应用可在 Windows 和 macOS 启动。 | 部分通过 | 本机 Apple Silicon 已真实启动 `投研罗盘.app`；macOS Intel 和 Windows 真实桌面启动验收仍未完成。 |
| 2 | Go sidecar 可自动启动和退出。 | 部分通过 | 已有 Rust sidecar 启动、stdin token、携带 `protocolVersion` 的 ready JSON、protocolVersion 兼容性校验、health/shutdown 基线和单测；Tauri `externalBin` 已声明 Go sidecar 基名，`sidecar:check-targets` 可重复构建并校验 Apple Silicon、macOS Intel 和 Windows x64 三类 target sidecar，其中 Windows x64 为 GUI subsystem；脚本测试已覆盖三类 target sidecar 文件名必须匹配 Tauri `externalBin` 基名，避免构建脚本和打包配置漂移；`verify-desktop-package.mjs` 可在生成桌面包后检查 macOS `.app` 或 Windows 解包目录中主程序和 Go sidecar 是否同时存在、非空，并可通过 `--target` 复核包内二进制架构；本机 `投研罗盘.app` 已包含 `Contents/MacOS/invest-compass-core`，包内 sidecar 通过 stdin 握手、`protocolVersion=1` ready 和 shutdown 回环验证；真实启动后进程参数确认 sidecar 来自 `.app/Contents/MacOS/invest-compass-core`，应用退出后 desktop/core 均无残留进程；macOS Intel 和 Windows 安装包仍需目标平台验收。 |
| 3 | 用户可以搜索并添加自选股。 | 部分通过 | Go 搜索 API、自选股 CRUD API、stocks/watchlists 持久化和 Rust `stock_search`/`watchlist_*` command 已接入；前端 `/watchlist` 已通过 typed invoke service 接入搜索、添加、更新、删除、刷新、行内单股 quote 刷新和真实 `market_quote` 行情读取，并覆盖重复添加错误展示；真实合规 Provider 授权和外网样例仍待验收。 |
| 4 | 用户可以查看个股行情和 K 线。 | 部分通过 | Go `POST /api/market/quote`、`POST /api/market/kline`、`POST /api/market/indicators`、quotes/klines 持久化缓存和 Rust `market_quote`/`market_kline`/`market_indicators` command 已接入；前端 `/stocks/:symbol` 已通过 typed invoke service 展示行情摘要、Lightweight Charts K 线、K 线表格摘要和技术指标，period/adjust 切换和数据不足空态已有测试覆盖；新闻外链已通过 Rust `open_external_url` 白名单 command 打开系统浏览器；真实合规 Provider 授权和跨平台桌面验收仍未闭环。 |
| 5 | 用户可以配置至少一个 AI 模型。 | 部分通过 | AI 配置安全规则、Rust 本地 vault 写入/读取、数据库持久化、生产 `OpenAIConfigTester` 注入、`POST /api/ai/configs/test` 和 Rust `ai_config_*` command 已有基线；前端 `/ai-settings` 已通过 typed invoke service 接入列表、保存、删除和连通性测试，保存后只展示脱敏字段，测试失败会二次脱敏；真实外部 Provider 和跨平台桌面凭据验收仍未闭环。 |
| 6 | 用户可以基于股票生成 AI 分析报告。 | 部分通过 | Prompt builder、分析请求和任务规则已有基础；`POST /api/analysis/tasks`、`POST /api/tasks/cancel` 和 Rust `analysis_task_*` command 已接入；分析执行器可读取缓存行情/K 线/新闻、构建合规 Prompt、调用 OpenAI-compatible Provider 并按 `task_id` 保存报告；报告 `input_snapshot` 会保留本次一次性持仓输入但不保存运行期 Key；取消 API 已能传播到运行中 executor context 且不会把取消写成失败；前端 `/analysis` 已通过 typed invoke service 创建分析任务、读取任务事件、展示成功报告，并支持复制/导出默认不含 `input_snapshot` 的 Markdown；真实 Market/News Provider 填充、外部 Provider 真实连通和跨平台桌面剪贴板/下载验收仍未闭环。 |
| 7 | 分析过程支持流式输出或进度展示。 | 部分通过 | 分析执行器会持久化 `TASK_STARTED`、`TASK_CHUNK`、`TASK_SUCCESS` / `TASK_FAILED` 事件；`POST /api/tasks/events/stream` 可输出脱敏 SSE 帧，首次无新增事件时会等待新事件并在终态事件后结束；Rust `analysis_task_subscribe` 可读取该流并 emit `analysis-task-event`，Rust SSE 解析已覆盖多行 `data:` 和 CRLF 帧边界；前端 `/analysis` 会调用订阅 command 并从 `task_events` 回放持久化事件展示生成片段、最终状态、取消状态和 `TASK_FAILED` 失败原因；真实桌面 SSE 长连接体验仍待 T44 验收。 |
| 8 | 报告可以保存、查看、复制。 | 部分通过 | 报告幂等、软删除规则、`POST /api/reports/*` 和 Rust `report_*` command 已闭环；报告查询默认不返回完整 `input_snapshot`，Markdown 导出默认不包含一次性持仓输入，显式导出快照能力已有 service 规则；前端 `/reports` 已通过 typed invoke service 接入报告列表、详情查看、复制 Markdown、导出 Markdown 和删除，且前端默认复制/导出内容不包含 `input_snapshot`；真实桌面剪贴板/下载行为仍待 T44 跨平台验收。 |
| 9 | 任务失败后可以看到错误原因。 | 部分通过 | 统一错误和任务事件模型已有基础，任务创建/取消和分析执行失败均会写入持久化事件并更新任务错误；前端 `/tasks` 已通过 typed invoke service 接入任务列表、详情、事件回放和详情打开后的新增事件订阅，FAILED 任务可展示 `error_message`；真实桌面长连接体验仍待 T44 验收。 |
| 10 | 设置中心支持工作区、代理、缓存、数据源状态、检查更新和日志导出。 | 部分通过 | Go settings/workspace/cache API 已接入真实 `dao.Store`，Rust settings/workspace/cache 白名单 command 已接入，`workspace_set` 已在 Rust 边界拒绝空路径和相对路径；`settings_set` 已支持代理密码写入/删除本地 vault 并只转发 `proxy_credential_ref`，且会在读写 vault 前拒绝同一请求同时写入新代理密码和清理旧凭据；前端 `/settings` 已通过 typed invoke service 接入 settings、workspace、cache、provider status、update、autostart 和 log export，并在提交前拒绝带 username/password 的代理 URL；关闭到托盘开关已重新接入并保存 `window.close_to_tray`；AI 分析任务成功/失败通知已有 Tauri notification 插件和最小 capability 自动化基线，并受 `notifications.task_terminal` 设置控制；开机自启已有官方 Tauri autostart 插件和 Rust 白名单 command 基线；通知、托盘和开机自启仍需 macOS / Windows 真实桌面验收。 |
| 11 | 应用有明确风险提示。 | 部分通过 | 技术方案、发布指南、Prompt 合规规则已包含“仅作研究辅助，不构成投资建议”；Dashboard、AI 分析页和设置关于区已展示风险提示；仍需完整 UI 逐屏验收。 |
| 12 | 数据库升级不会丢失用户已有数据。 | 部分通过 | T08 已完成空库迁移、重复迁移和唯一约束自动化验证；Go sidecar 生产启动会在 `dao.Open` / `dao.Migrate` 前把已有 SQLite 主库备份到工作区 `backups/` 目录，并拒绝覆盖同名备份；DAO 自动化已覆盖备份文件恢复为新 SQLite 后重新迁移并读取用户 settings 数据；T09 CRUD repository 已有自动化覆盖，发布后真实升级演练尚未完成。 |
| 13 | API Key 存入 Rust 本地文件 vault，配置查询不回显真实 Key。 | 部分通过 | Go 配置模型和 `POST /api/ai/configs/list|save|delete|test` 已拒绝真实 Key 落库和回显，并在保存前拒绝非 `local-vault://ai-config/` 的 `api_key_ref`；Rust `ai_config_save` 已将一次性 Key 写入本地 vault 并只转发引用，新建配置尚无数据库 ID 时会生成唯一 `api_key_ref`，避免多个 `id=0` 新配置覆盖同一密钥文件；更新 API Key 时会先校验旧 `api_key_ref`，再写入新密钥并删除被替换的旧引用，旧引用非法时不会写入新 secret，旧引用删除失败时会回滚本次新写入文件；本地 vault 目录和 secret 文件写入已统一收口，Unix/macOS 下目录强制收紧为 `0700`，secret 文件创建和更新后会强制收紧为 `0600`；本地 vault 会拒绝清理后为空的 AI Provider、引用和代理 profile，避免退化为弱语义文件名或落到隐式 `.secret` 文件；Go core 会拒绝非空且没有脱敏标记的 `masked_api_key`，避免明文 Key 通过展示字段旁路落库；Go settings 会拒绝未知 `_ref`、非 `local-vault://proxy/` 的代理凭据引用和未脱敏的 `masked_api_key`；`ai_config_delete` 支持按 `api_key_ref` 删除本地 vault 文件，并已在删除 vault 前拒绝非正数配置 ID，`ai_config_test` 支持读取本地 vault 后内部注入 `resolved_api_key`；代理密码 vault 已接入 settings command，并覆盖冲突写入/清理动作早失败；自动化基线通过，真实桌面验收仍待完成。 |
| 14 | 前端不能直接访问 Go sidecar，Rust command 必须白名单化。 | 部分通过 | 已有 `core_start`、`core_health`、settings/workspace/cache、`stock_search`、`watchlist_*`、market/news/dashboard/provider/prompt 模板、AI config、analysis task、task/report、update/log export 白名单 command 和安全配置测试；`stock_search` 已在 Rust 边界拒绝空 keyword，workspace/log export 已在 Rust 边界拒绝无效本地路径，market/news/task 相关 Rust command 已在转发前拒绝无界 `limit`、非法事件游标和空 `task_id`，analysis task 创建已在读取 vault 前拒绝空 `symbol`、空 `analysis_type`、非 `local-vault://ai-config/` 的 `api_key_ref` 和非法配置 ID，watchlist/prompt/report/AI config 相关 Rust command 已拒绝非正数 ID；Go core 业务 API 已统一限制 JSON 请求体大小，仅接受 JSON object，并拒绝 `null`、尾随内容和未知字段，超限返回 `41300/request_body_too_large` 且不进入业务层；前端 `coreClient` 已通过 typed invoke service 调用 `core_health`，源码扫描禁止直连 Go core；业务页面完成后仍需逐屏确认没有绕过 typed invoke service。 |
| 15 | 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。 | 部分通过 | 已新增前端生产源码自动化扫描，禁止策略观察、授权激活、公告、研报、资金流、券商账户、自动下单、云同步和移动端等首版未闭环入口文案进入 renderer 源码；前端 `APP_NAV_ITEMS` / `APP_ROUTE_PATHS` 已由测试锁定首版主导航和路由集合，避免非 MVP 页面静默进入；真实桌面应用仍需逐屏确认没有隐藏入口或运行期配置入口。 |
| 16 | macOS / Windows 使用 Rust 本地文件 vault，凭据保存/删除均通过验收。 | 部分通过 | Rust 本地 vault 保存/读取/删除已有单测，覆盖 AI Key、无数据库 ID 新配置唯一 `api_key_ref`、更新 key 清理旧引用、旧引用非法时不写入新密钥、空 AI Provider 和空/非法清理后的 vault 引用拒绝、Unix/macOS vault 目录 `0700` 和 secret 文件 `0600` 权限、代理密码和代理凭据冲突动作早失败；Go core 单测覆盖 AI 配置和 settings 只接受本地 vault scheme 的凭据引用，并拒绝未脱敏的 `masked_api_key`；macOS / Windows 真实桌面路径、权限和手工验收仍待完成。 |
| 17 | Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。 | 部分通过 | 技术指标 `POST /api/market/indicators` 和 Rust `market_indicators` 已闭环；Prompt 模板 `POST /api/prompt-templates/*` 和 Rust `prompt_templates_*` 已闭环，模板 ID 已在 Rust 边界拒绝非正数；前端 `/ai-settings` 已展示首版模板类型和变量白名单，并在保存前拒绝未支持变量；分析任务创建/取消与任务历史 `POST /api/analysis/tasks`、`POST /api/tasks/*` 和 Rust `analysis_task_*` / `task_*` 已有基线，任务列表已拒绝非法 `limit` 以避免无界查询，任务详情/取消/事件订阅已在 Rust 边界拒绝空 `task_id`，任务事件回放已拒绝负数 `after_event_id`，Rust command 已在转发前做同样入参早失败；报告历史 `POST /api/reports/*` 和 Rust `report_*` 已闭环，报告 ID 已在 Rust 边界拒绝非正数；前端 `/reports` 和 `/tasks` 已接入真实 command，复制/导出和任务详情订阅均已有自动化基线，真实桌面剪贴板、下载和长连接体验仍待 T44 验收。 |
| 18 | 检查更新链接只允许 HTTPS allowlist 域名。 | 部分通过 | Go `updatecheck` 规则、`POST /api/update/check`、settings 配置来源、远程 manifest 获取边界和 Rust `check_update` 已覆盖 HTTPS 与 allowlist；远程 manifest 超过 256 KiB 会被拒绝，不接受截断内容，且 manifest 拉取不跟随 HTTP 3xx 重定向；前端 `/settings` 已接入检查更新入口，并可读取、编辑和保存 `update.manifest_url` / `update.allowed_hosts`；跨平台桌面验收仍未闭环。 |
| 19 | 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。 | 部分通过 | Dashboard/provider status Go API 与 Rust `dashboard_summary`、`providers_status` 已接入；`dashboard_summary` 生产路径已从真实 `dao.Store` 聚合 active 自选股、最新行情、最近报告、任务、市场新闻和 Market/News Provider 状态，未配置真实 Provider 时分别返回 `available=false` / `source=unconfigured` 状态，新闻 Provider 实现可选 `Status(ctx)` 时会返回 Provider 自身可用性而不是默认假定可用；首页 Dashboard 已通过 typed invoke service 接入 `dashboard_summary`，覆盖正常、无数据和 Provider 异常状态；设置页已展示 Provider 状态；新闻 `POST /api/news/list`、`POST /api/news/market` 和 Rust `news_list`、`news_market` 已接入，Rust command 已拒绝非法新闻 `limit`；前端 `/news` 已通过 typed invoke service 接入市场新闻和个股新闻，支持标签筛选、空状态、错误状态和非 HTTPS 新闻过滤，并通过 `open_external_url` 白名单 command 打开 HTTPS 新闻外链；真实 Provider 授权和跨平台桌面验收仍未闭环。 |
| 20 | AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和 SQLite。 | 部分通过 | Go 侧模型和 AI 配置 API 已避免真实 Key 落库和回显，前端/shared 扫描禁止内部密钥字段；Rust `ai_config_test` 和 `analysis_task_create` 已能从本地 vault 读取 Key 并只在内部请求中注入 `resolved_api_key`，且 `analysis_task_create` 会先拒绝空基础字段、非 `local-vault://ai-config/` 的 `api_key_ref` 和非法配置 ID 再读取 vault；分析执行器只在当前请求内存中使用该 Key 调用 AI，报告输入快照可保存一次性持仓但不保存明文 Key；真实桌面链路仍待验收。 |

## 4. 调度专题验收记录

当前调度专题的自动化验收结论为部分通过，不能作为完整发布验收通过。

已通过项：

- `scheduler_jobs` 使用 `cron_type` 字段，未使用保留语义的 `type` 字段。
- `scheduler_runs` 和 `ingestion_watermarks` 已进入 SQLite 迁移和 repository 基线。
- Go scheduler service 使用 `github.com/go-co-op/gocron/v2` 注册 enabled jobs，并通过执行队列统一承载 `scheduled`、`missed_today`、`catchup_gap` 和 `user_request`。
- 启动恢复会处理遗留 queued/running run，并支持 A 股 09:30 任务在用户 10:00 启动软件时生成当天 `missed_today` 补偿 run。
- 手动 backfill 支持 30 个 symbol 和 30 个自然日范围限制，只为交易日生成补偿 run。
- Rust scheduler command 均固定映射到 Go API path，没有新增任意 method/path/body 代理。
- 前端 `/scheduler` 入口可读取真实 scheduler status、job list、run list 和 job types，并支持创建/编辑、启停、删除、立即执行、手动补偿和单股刷新。
- 单股刷新已接入 `/scheduler`、`/stocks/:symbol` 股票详情页和自选股行内操作；详情页提交 `data_type=all`，自选股行提交 `data_type=quote`，均调用 `scheduler_refresh_symbol` 并复用统一执行队列。
- Provider 不可用提示、run 详情查看、状态/触发类型过滤和主要前端校验矩阵已有自动化基线；真实桌面端视觉和交互验收仍归 T44。
- 安全扫描已限制 `gocron` 只能出现在 `internal/service/scheduler`，并禁止非 scheduler service 直接引用调度表名和调度模型。

未通过或未闭环项：

- 未完成真实 Market/News Provider 授权和频率限制验收。
- macOS Intel 和 Windows 上的真实调度启动、休眠恢复、退出恢复和补偿行为尚未手工验收。

本轮已执行并通过的调度相关验证：

```bash
cd apps/sidecar-core && go test ./...
pnpm --dir apps test
pnpm --dir apps check
cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml
cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml --check
git diff --check
```

## 5. macOS 验收记录

尚未完成完整真实 macOS 验收；Apple Silicon `.app` 启动、包内 sidecar 路径和退出清理已有本机证据。

已完成的本机证据：

- `tauri build --bundles app` 已生成 `投研罗盘.app`。
- `投研罗盘.app/Contents/MacOS/` 同时包含 `invest-compass-desktop` 和 `invest-compass-core`。
- `scripts/verify-desktop-package.mjs --platform=darwin --target=aarch64-apple-darwin <app>` 可复核 `.app` 包名、`CFBundleDisplayName`、`CFBundleName` 与 Tauri `productName` 一致、包结构中主程序和 sidecar 是否同时存在、非空、具备执行位、`Info.plist` 指向主程序、bundle 类型为 `APPL`、bundle identifier 和版本号与 Tauri 配置一致、声明的图标资源作为 `Contents/Resources` 直接子文件随包存在且架构匹配 Apple Silicon。
- `pnpm --dir apps sidecar:smoke` 已验证 `.app` 包内 `invest-compass-core` 可通过 stdin token 握手输出 `protocolVersion=1` ready JSON，并可响应 `/internal/health` 和 `/internal/shutdown`。
- 包内 `invest-compass-core` 已通过 stdin 握手、`protocolVersion=1` ready JSON 和 `/internal/shutdown` 关闭验证。
- 真实启动 `投研罗盘.app` 后，`ps` 进程参数确认 Go core 路径为 `.app/Contents/MacOS/invest-compass-core`。
- 通过 macOS 应用退出路径关闭后，`pgrep -af "(invest-compass-desktop|invest-compass-core)"` 无残留进程。

待验收项：

- Intel 启动和退出。
- 本地 vault 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- 脱敏日志导出。

## 6. Windows 验收记录

尚未完成真实 Windows 验收。

待验收项：

- Windows x64 启动和退出。
- Go sidecar 随应用启动、退出后无残留进程。
- `scripts/verify-desktop-package.mjs --platform=win32 --target=x86_64-pc-windows-msvc <unpacked-dir>` 可先复核解包目录中 `invest-compass-desktop.exe` 和 `invest-compass-core.exe` 是否同时存在、非空、架构匹配 Windows x64 且为 GUI subsystem。
- 本地 vault 保存、读取、删除 API Key 和代理密码。
- 通知、托盘、开机自启。
- NSIS / MSI 安装包包含正确 sidecar 文件名。
- 脱敏日志导出。

## 7. 已知风险和遗留项

- GORM schema/migration、迁移前 SQLite 备份、关键 CRUD repository、settings/workspace 真实存储和 cache cleaner 基线已落地，现有业务模块已迁入 `internal/service/<module>`，HTTP 路由和 handler 已拆入 `internal/actions`，`internal/server` 已收缩为监听和启动层，公共日志能力已迁入 `pkg/logger`，通用错误码和错误类型已迁入 `pkg/xerr`；真实 Market/News Provider 未确认前，行情、新闻和分析上下文仍不能证明完整真实数据闭环。
- 日志导出已具备 Go core 内存 slog 日志源、二次脱敏包、quoted JSON 敏感字段值含逗号时的完整脱敏回归覆盖、Rust `export_logs(target_dir)` 写入目标目录、目标目录早校验、路径穿越文件名拦截、同名文件不覆盖保护、Unix/macOS `0600` 私有文件权限，以及设置中心手动目录输入、Tauri dialog 原生目录选择、前端空值校验和导出触发；跨平台手工验收仍待完成。
- 本地 vault 安全性低于平台凭据服务；首版只能证明前端、Go core、SQLite、日志和报告不直接保存真实 Key，且 Unix/macOS vault 目录权限已收紧为 `0700`、secret 文件权限已收紧为 `0600`，但仍不能抵御当前用户账号下的本机文件读取风险。
- 当前非 UI 自动化基线的停止线已经明确：继续推进 T13/T18 需要确认真实 Market/News Provider 数据源和授权；`scripts/provider-smoke.mjs` 已提供默认不联网的 smoke 计划和显式确认后的 live 样例检查入口，且 live smoke 会校验端点特征响应体，避免把错误页或空壳 200 当成样例通过，但联网 smoke 仍不能替代授权、可分发边界或生产可用性结论；T39 的托盘、通知和开机自启需要真实 macOS / Windows 桌面验收；T44 的未闭环入口检查已有生产源码扫描基线，但仍需真实桌面应用逐屏验收。
- 真实行情/新闻 Provider 未确认数据源授权前，不能接入或宣称可用。
- 前端基础外壳、`core_health` 启动检查、首页 Dashboard、自选股、个股详情/K 线、资讯中心、任务调度、AI 配置/Prompt 模板、AI 分析、报告历史、任务历史和设置页面已接入；仍需用真实桌面应用逐屏复核“界面无 mock 数据”和“首版不出现未闭环入口”。
- 关闭到托盘的 settings 读取、真实托盘菜单、主窗口恢复入口、主窗口位置/尺寸恢复、前端设置开关、任务成功/失败通知、通知设置开关和开机自启设置开关已接入；托盘/通知/开机自启仍需 macOS / Windows 真实桌面验收。
- Tauri `externalBin`、三类首版 target sidecar 实际构建基线、Windows GUI subsystem sidecar 构建参数和 Apple Silicon `.app` 包内 sidecar 启动/关闭验证已落地；macOS Intel 和 Windows 安装包随包启动仍未验收，暂不能证明所有平台安装包都不依赖运行时下载 sidecar。

## 8. 复测入口

T44 重新验收时，按以下顺序执行：

1. 先通过 T13/T18/T21/T26/T27/T39-T42 的任务级剩余验收。
2. 执行 `pnpm --dir apps acceptance:check` 和发布候选打包验证命令。
3. 在 macOS 和 Windows 上分别安装并启动真实桌面应用。
4. 逐项完成第 3 节 20 条验收标准。
5. 更新本报告的“当前状态”和平台验收记录。
6. 确认无未闭环入口、无 mock 数据、无明文 secret 后，才能把 T44 标记为 `[x]`。
