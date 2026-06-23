# 投研罗盘前后端对接实施清单

> 目标：把 UI 层和 Go sidecar 后端的真实能力按阶段接通，形成可执行、可验证、可回溯的对接流程。
>
> 本文只描述前后端对接流程，不新增首版范围，不把未验收 Provider、公告研报、资金流、授权激活、云同步或自动交易能力混入 MVP。

---

## 1. 范围

### 1.1 必须交付

- 前端页面统一通过 `apps/frontend/src/services/coreClient.ts` typed invoke service 调用能力。
- Rust/Tauri 只通过白名单 command 调用固定 Go API，禁止任意 path 代理。
- Go API 只接受 POST，并继续校验 runtime token、request ID 和 trace ID。
- Dashboard、自选股、个股详情、资讯中心、模型配置、Prompt 模板、AI 分析、报告历史、任务历史和设置中心逐页接入真实 command。
- 分析任务支持创建、取消、SSE 转发、事件回放、失败原因展示和报告落地。
- 所有页面具备加载态、空态、错误态和真实不可用状态，不用 mock 数据伪装能力。
- 界面上任何功能如发现数据库、Go API、Rust command 或后端 service 实际不支持，必须先标记阻塞并找用户确认处理方案。
- 自动化测试覆盖 command 契约、页面主流程、敏感字段不泄露和首版未闭环入口扫描。
- 最终形成 RGFE1-RGFE8 的阶段验收记录。

### 1.2 明确不做

- 不让前端直接访问 `127.0.0.1:<core_port>`。
- 不新增通用 `core_request(method, path, body)`。
- 不在前端保存 API Key、runtime token、Go core port 或代理密码。
- 不把 `resolved_api_key` 暴露到前端类型、OpenAPI、SQLite、任务事件、报告快照或日志。
- 不用假行情、假新闻、假报告、假任务或假 Provider 状态填充 UI。
- 不因为真实 Provider 未配置就把页面改回 mock 数据。
- 不忽略界面和后端能力不匹配的问题，不继续 mock 接口或 mock 数据绕过缺口。
- 不修改数据库 schema、公共 API、Rust command 名称、权限或依赖，除非先完成单独确认。
- 不宣称 RG2、RG5、RG6 完整通过，直到真实 Provider、桌面能力和跨平台验收完成。

---

## 2. 状态标记

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成并通过验证
- `[!]` 阻塞或需要人工确认

---

## 3. 依赖路线

```text
FE0 文档和契约盘点
  ↓
FE1 启动链路和安全基线
  ↓
FE1.5 数据拉取和验收数据基线
  ↓
FE2 只读页面真实数据接入
  ↓
FE3 写操作和配置接入
  ↓
FE4 AI 分析长任务和 SSE 接入
  ↓
FE5 历史、回放、复制和导出闭环
  ↓
FE6 mock 清理和页面状态收口
  ↓
FE7 自动化、桌面 smoke 和验收报告
```

并行原则：

- FE0 和 FE1 是上游门禁，契约矩阵和 sidecar 启动没有确认前，不批量改页面。
- FE1.5 必须在页面接入前完成最小数据拉取基线；没有真实数据时只能验收空态、错误态或 Provider 不可用状态。
- FE2 可按 Dashboard、自选股、个股详情、资讯中心、报告历史、任务历史分支并行。
- FE3 的 AI 配置和代理设置涉及凭据边界，必须串行复核 Rust vault 链路。
- FE4 的任务创建、SSE 订阅、事件回放和报告保存必须作为一个原子链路验收。
- 同一页面的 service、UI state 和测试可以由同一代理负责，避免多个代理同时改同一文件。
- 涉及 `coreClient.ts`、Rust command 名称、Go API schema 的改动必须先合并契约，再分发页面任务。

---

## 4. Review Gate

### RGFE1 契约门禁

- 页面、typed service、Rust command、Go API、错误态和测试文件已经形成矩阵。
- 所有前端数据入口都能追溯到 `coreClient.ts`。
- 所有 Rust command 都固定 path、method、request schema 和 response schema。
- 未发现前端直接 `fetch`、`XMLHttpRequest`、`axios` 或硬编码 Go sidecar 地址。
- 契约矩阵明确哪些能力受真实 Provider、外部模型或跨平台桌面验收阻塞。
- 界面功能如果没有对应数据库、Go API、Rust command 或后端 service 支撑，必须列为阻塞项并等待用户确认处理方案。

### RGFE2 启动和安全门禁

- Tauri 启动后 Go sidecar 完成 stdin token 握手。
- `core_health`、`providers_status`、`dashboard_summary` 至少一条真实 invoke 链路可用。
- Go API 非 POST 返回 405。
- Runtime token 不出现在 argv、env、日志、前端类型或测试快照中。
- Rust command 错误返回可被前端展示为明确失败态。

### RGFE2A 数据基线门禁

- 已明确每个页面验收需要的数据来源：本地 SQLite、真实 Provider、外部 AI 模型、用户操作生成或空态。
- 已通过现有 command 拉取最小真实数据样本，或记录 Provider/数据库/API 不支持导致的阻塞项。
- 已记录用于验收的 symbol、report ID、task ID、provider 状态和失败原因。
- 不通过直接改数据库、mock 接口或 mock 数据伪造页面验收数据，除非用户单独确认。
- 对界面存在但数据库、Go API、Rust command 或后端 service 不支持的功能，已标记阻塞并等待用户确认处理方案。

### RGFE3 只读页面门禁

- Dashboard、自选股列表、个股详情、资讯中心、报告历史和任务历史读取真实 command。
- 缓存缺失或 Provider 未配置时展示空态或不可用状态，不生成假数据。
- K 线、指标和新闻由 Go core 返回，前端不自行计算业务指标。
- 报告历史默认不展示完整 `input_snapshot`。
- 任务历史能展示 FAILED 错误原因和持久化事件。

### RGFE4 写操作门禁

- 自选股新增、更新、删除和重复添加错误可用。
- Prompt 模板 CRUD 只允许首版类型和变量白名单。
- 设置保存、缓存清理、工作区保存、检查更新和日志导出走真实 command。
- 缓存清理不会删除报告和配置。
- 代理 URL 禁止携带 username/password。

### RGFE5 凭据门禁

- AI Key 明文只在 `ai_config_save` 一次性 payload 中出现。
- 保存成功后页面清空明文输入，列表只展示 `has_api_key` 和 `masked_api_key`。
- `ai_config_test` 和 `analysis_task_create` 由 Rust 从本地 vault 注入 `resolved_api_key`。
- 测试连接失败、任务失败、日志导出和页面错误态均不泄露密钥。
- 删除 AI 配置同步删除对应本地 vault 引用。

### RGFE6 分析任务门禁

- AI 分析页可以创建任务并订阅 Rust 转发事件。
- `TASK_CHUNK` 可以增量展示，`TASK_SUCCESS` 可以打开最终报告。
- 取消任务后 UI 进入 `CANCELLED`，不会追加失败态。
- 失败任务展示脱敏后的失败原因。
- SSE 断线后可通过 `task_events(task_id, after_event_id)` 补拉。

### RGFE7 mock 清理门禁

- 前端生产源码中不存在用于伪装真实能力的 `mock`、`fake`、`dummy`、`fixture`、`demo`、`sample` 业务数据。
- 删除页面 mock 文件后，页面仍能展示加载、空态、错误态和真实数据。
- 首版未闭环入口扫描通过。
- 页面文案不出现自动交易、荐股、授权激活、云同步、公告研报或资金流专用入口。

### RGFE8 总体验收门禁

- Go、Rust、前端自动化验证通过，或在验收记录中写明未执行原因和剩余风险。
- 本地 Tauri app 能完成核心主流程 smoke。
- 验收报告列出通过项、阻塞项、真实 Provider 待验收项和跨平台桌面待验收项。
- checklist 只标记已经实现且验证通过的任务。

---

## 5. FE0：文档和契约盘点

### FE00 固化前后端对接专题清单

- 状态：`[x]`
- 依赖：无
- 交付物：
  - `docs/2026-06-22-invest-compass-frontend-backend-integration-checklist.md`
- 执行动作：
  - 将对接流程拆成阶段、任务、验收标准和 Review Gate。
  - 明确首版范围、停止线、安全边界和并行原则。
  - 保持任务 ID 可引用、可勾选、可复盘。
- 验收标准：
  - 文档覆盖契约盘点、读链路、写链路、AI 长任务、mock 清理和总体验收。
  - 文档没有把未执行对接写成已完成。
  - `git diff --check` 通过或说明失败原因。
- 退出条件：
  - 后续对接工作可以按 FE 任务 ID 推进。

### FE01 同步主 checklist 专题入口

- 状态：`[x]`
- 依赖：FE00
- 交付物：
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
- 执行动作：
  - 在“专题实施清单”章节补充本文入口。
  - 标明本文只负责对接流程，不改变 T00-T44 的任务状态。
- 验收标准：
  - 主 checklist 能导航到前后端对接专题清单。
  - 没有批量展开 FE00-FE28 到主 checklist。
- 退出条件：
  - 主实施清单和专题文档之间可互相追溯。

### FE02 生成页面契约矩阵

- 状态：`[x]`
- 依赖：FE00
- 交付物：
  - 对接记录，可放入本文附录或单独验收记录。
- 执行动作：
  - 按页面列出使用的 typed service。
  - 为每个 service 标注 Rust command、Go API path、请求字段、响应字段和错误态。
  - 标注该页面依赖真实 Provider、外部模型、本地 vault 或跨平台桌面能力的部分。
  - 标注界面功能是否存在对应数据库、Go API、Rust command 和后端 service 支撑。
  - 标注现有测试文件和缺口。
- 验收标准：
  - Dashboard、自选股、个股详情、资讯中心、模型配置、Prompt 模板、AI 分析、报告历史、任务历史和设置中心都已覆盖。
  - 每个页面至少有一个可执行的自动化或手工验收入口。
  - 未闭环能力被标记为阻塞或停止线，而不是写成已完成。
  - 对后端或数据库不支持的界面功能，已记录为待确认项，不允许继续接 mock 接口或 mock 数据。
- 退出条件：
  - 后续页面接入不需要靠猜测查接口。

### FE03 检查安全扫描和架构护栏

- 状态：`[x]`
- 依赖：FE02
- 交付物：
  - `apps/desktop/test/security-config.test.mjs`
  - 相关前端测试或验收记录
- 执行动作：
  - 确认前端生产源码扫描覆盖 mock 数据、直连 Go、runtime token、敏感存储和未闭环入口。
  - 确认 Rust command 注册和固定 path 扫描覆盖新增 command。
  - 确认 Go 生产源码没有绕过 dao/model/service 的散落读写。
- 验收标准：
  - `pnpm --dir apps test` 中安全护栏测试通过。
  - 未发现 `resolved_api_key`、runtime token header 或 Go sidecar 地址出现在 renderer 源码。
  - 如有安全扫描失败，必须先修复或降级为阻塞任务。
- 退出条件：
  - 对接期间不会因页面改造绕过既有安全边界。

阻塞记录：

- 2026-06-23 执行 `pnpm --dir apps --filter @invest-compass/desktop test`，46 项中 45 项通过。
- 唯一失败项为“前端生产源码禁止 mock 数据伪装真实能力”，命中 `apps/frontend/src/pages/settings/data-source/credentials/DataSourceCredentialPage.tsx` 引用 `credentials/mock.ts`。
- 该页面涉及 Provider token / Cookie 凭据保存、脱敏回显和连接测试，当前缺少真实 schema、Rust command 和 Go API，已映射到 S3-02、S3-03、S3-04、S3-05。
- 按本清单规则，不能通过改名、继续本地 mock 或假测试绕过，需要先确认数据源凭据真实方案或页面降级方案。

解决记录：

- 2026-06-23 已新增数据源凭据加密 SQLite schema、Go service/action、Rust 白名单 command 和前端 typed service。
- `DataSourceCredentialPage` 已移除 `credentials/mock.ts`，改为调用 `dataSourceCredentialsList/Save/Clear/Test`。
- 验证通过：`pnpm --dir apps --filter @invest-compass/desktop test`、`pnpm --dir apps check`、`git diff --check`。

---

## 6. FE1：启动链路和安全基线

### FE04 验证 sidecar 启动和 health 链路

- 状态：`[~]`
- 依赖：FE03
- 交付物：
  - 本地启动记录或自动化 smoke 输出。
- 执行动作：
  - 启动 Tauri app。
  - 确认 Go sidecar 完成 stdin token 握手。
  - 从前端调用 `core_health()`。
  - 记录失败时的前端错误态、Rust 错误和 Go 日志摘要。
- 验收标准：
  - `core_health` 返回版本和数据库状态。
  - sidecar 未启动时 UI 显示明确错误，不显示假正常状态。
  - token 不出现在日志、前端状态或错误消息中。
- 退出条件：
  - 真实 invoke 主链路可作为后续页面接入基础。
- 实施记录：
  - 前端 `CoreHealth` 已接收 Go core 返回的 `dbStatus`，设置页左下角 SQLite 状态不再只按 Go Core 连接状态推断。
  - 已通过 `core_health` 前端单测和 Rust sidecar 串行测试验证 token 不进入前端入参，sidecar 启动参数不包含 token。
  - 待补真实 Tauri app 启动 smoke 记录；Go actions/main 相关测试当前受本机 SQLite FTS5 不可用阻塞。
  - `make dev` 是长驻 Tauri/Vite GUI 开发命令，本清单推进时不在自动化步骤中直接挂起执行；真实桌面 smoke 需要用户本地启动后补记录。

### FE05 验证统一响应和错误解包

- 状态：`[x]`
- 依赖：FE04
- 交付物：
  - `apps/frontend/src/services/coreClient.test.ts`
- 执行动作：
  - 覆盖成功响应解包。
  - 覆盖后端业务错误、Rust command 错误和 sidecar 未启动错误。
  - 确认页面只展示脱敏后的 `message`、`requestId`、`traceId`。
- 验收标准：
  - 错误态不丢失 request ID / trace ID。
  - 错误态不泄露 API Key、代理密码或用户持仓输入。
  - 测试覆盖核心错误分支。
- 退出条件：
  - 页面可以统一处理真实 command 的成功和失败。
- 实施记录：
  - `coreClient` 已统一包装 Tauri command 错误为 `CoreClientError`。
  - 后端业务错误会保留 `code`、`requestId`、`traceId`，展示消息会脱敏 `Authorization`、`Proxy-Authorization`、`api_key`、`token`、`cookie`、`password`、`secret` 和 `sk-*` 形态密钥。
  - `apps/frontend/src/services/coreClient.test.ts` 已覆盖成功响应、业务错误、Rust command/sidecar transport 错误。

### FE05A 接入全局初始化等待页

- 状态：`[~]`
- 依赖：FE04、FE05
- 交付物：
  - `apps/frontend/src/pages/initialization/`
  - `apps/frontend/src/app/App.tsx`
  - `apps/desktop/src-tauri/src/commands/boot.rs`
- 执行动作：
  - 应用启动默认进入初始化等待页，业务 Sidebar 和 TopBar 处于锁定状态。
  - React bundle 挂载前由 `index.html` 展示轻量启动占位，避免桌面窗口先出现白屏。
  - 前端通过固定 Rust command `app_boot_status` 轮询启动状态，不使用前端定时器假完成。
  - 后端首次即 ready 时，前端仍保留一段最短初始化页展示时间，避免从白屏直接跳到总览。
  - Rust command 在初始化页期间启动 Go Core Sidecar，确认 `core_health`，读取 `/api/search/status`。
  - 当搜索状态为 `NEED_REBUILD` 且 FTS5 可用时，启动期主动调用 `/api/search/rebuild` 的 `all` 范围并等待状态刷新。
  - 当搜索状态为 `BUILDING` 时继续停留初始化页；当后端返回可进入状态时切换到总览页面。
- 验收标准：
  - 初始化期间不进入 Dashboard、自选股、AI 分析等业务页面。
  - Sidecar、SQLite migration、搜索索引状态由后端真实状态驱动。
  - 启动错误进入初始化页日志和步骤失败态，错误文案不泄露 token、cookie、API Key、Authorization 或真实敏感路径。
  - FTS5 环境不可用时不永久阻塞启动页，但左侧状态和设置页搜索索引仍展示真实不可用状态。
  - 需要端到端验证项：`make dev` 启动后确认初始化页出现、索引重建时停留、完成后进入总览。
- 实施记录：
  - 已新增 `app_boot_status` Rust command，并注册到 Tauri invoke handler。
  - 已将 sidecar 同步启动从 Tauri setup 移到初始化页状态 command，避免窗口展示前阻塞。
  - 已新增前端 `appBootStatus()` typed service 和启动页轮询逻辑。
  - 已新增 HTML 级启动占位和最短初始化页展示窗口，处理 `make dev` 下 WebView 先白屏、后端快速 ready 直接进入总览的问题。
  - 已验证：`pnpm --dir apps --filter @invest-compass/frontend test -- src/services/coreClient.test.ts src/app/App.test.tsx -t "appBootStatus|初始化"`。
  - 已验证：`pnpm --dir apps --filter @invest-compass/frontend check`、`pnpm --dir apps --filter @invest-compass/frontend build`、`cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml boot::tests -- --nocapture`、`cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml`、`cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml -- --check`、`git diff --check`。

### FE06 验证 Provider 状态基线

- 状态：`[x]`
- 依赖：FE04
- 交付物：
  - Dashboard 或设置中心数据源状态验收记录。
- 执行动作：
  - 调用 `providers_status()`。
  - 验证 Market/News Provider 未配置时展示 `available=false` 和真实 source。
  - 验证页面不把 Provider 不可用显示成正常行情能力。
- 验收标准：
  - Provider 不可用时不影响 app 启动。
  - 页面展示明确不可用状态和脱敏错误。
  - Dashboard summary 不生成假行情、假新闻或假报告。
- 退出条件：
  - 真实 Provider 阻塞项和 UI 对接项已拆开。
- 实施记录：
  - `providers_status()` 已通过固定 Rust command 读取 Go core `/api/providers/status`，后端统一返回脱敏后的 Provider summary。
  - Dashboard provider summary 与 Provider 状态接口共用 `dashboard.ProviderStatusSummary`，不可用 Provider 不会被聚合为正常状态。
  - 前端左下角和 Dashboard 状态展示已覆盖 Provider 不可用状态；不可用时显示“不可用”并保留脱敏错误信息。
  - 已验证：`go test ./internal/actions -run 'TestProviderStatusReturnsSafeStatus|TestProviderStatusUsesNewsProviderStatus'`、`go test ./internal/service/dashboard -run 'TestProviderStatus|TestBuildSummary'`、`pnpm --dir apps --filter @invest-compass/frontend test -- App.test.tsx DashboardOverview.test.tsx`。
  - 混合运行包含 Dashboard store 迁移的 Go 测试仍受本机 SQLite FTS5 不可用阻塞，不影响 Provider 状态基线结论。

### FE06A 建立数据拉取和验收数据基线

- 状态：`[~]`
- 依赖：FE04-FE06
- 交付物：
  - 数据准备记录，可放入本文附录或阶段验收记录。
- 执行动作：
  - 按页面列出验收所需数据：自选股、行情、K 线、指标、新闻、AI 配置、Prompt 模板、任务、报告、设置和日志。
  - 标注每类数据来源：SQLite 已有数据、真实 Market/News Provider、外部 AI Provider、用户操作生成或仅能验收空态。
  - 通过现有 typed service / Rust command 拉取最小真实数据样本，不直接绕过前后端链路写数据库。
  - 记录用于验收的 symbol、report ID、task ID、provider 状态、错误码、request ID 和 trace ID。
  - 如果某个界面功能缺少数据库、Go API、Rust command 或后端 service 支撑，标记为 `[!]` 并找用户确认处理方案。
- 验收标准：
  - FE07-FE21 每个页面或流程至少有一个明确的数据验收场景。
  - Provider 未配置或数据库为空时，已明确只能验收空态、错误态或不可用状态。
  - 不存在为通过 UI 验收而新增的 mock 接口、mock 数据或假成功状态。
  - 数据准备记录能解释哪些验证依赖真实外部环境，哪些可以本地自动化复测。
- 退出条件：
  - 页面接入任务可以基于明确数据样本或明确阻塞项推进。
- 初版数据基线：

| 后续任务 | 页面 / 流程 | 主要 typed service | 数据来源 | 最小验收场景 | 阻塞 / 待确认 |
| --- | --- | --- | --- | --- | --- |
| FE07 | Dashboard 总览 | `coreHealth`、`dashboardSummary`、`providersStatus` | `watchlists`、`quotes`、`analysis_reports`、`tasks`、`news_items`、Provider 状态 | 空库展示空态；Provider 不可用展示不可用；有报告/任务/新闻时读取后端摘要 | 无新增阻塞；真实外部 Provider 样本待桌面 smoke |
| FE08 / FE13 | 自选股列表和写操作 | `watchlistList`、`watchlistCreate`、`watchlistUpdate`、`watchlistDelete`、`stockSearch`、`marketQuote` | `watchlists`、`stocks`、`quotes`、Market Provider | 空自选展示空态；搜索后新增；删除后消失；行情失败不填假价格 | `[!]` S5-04：行业、趋势、星标等扩展字段口径待确认 |
| FE09 | 个股详情 | `marketQuote`、`marketKline`、`marketIndicators`、`newsList`、`openExternalURL` | `quotes`、`klines`、`news_items`、Market/News Provider | 用自选股或搜索结果 symbol 验证 quote、K 线、指标、新闻加载/空态/错误态；非 HTTPS 外链不打开 | `[!]` S5-06：公司资料和标签接口待确认 |
| FE10 | 资讯中心 | `newsMarket`、`newsList`、`openExternalURL` | `news_items`、News Provider | 市场新闻空态；个股新闻按 symbol 查询；Provider 未配置显示不可用；HTTPS 原文打开 | `[!]` S5-13：资讯侧栏统计、热点、加入上下文待确认 |
| FE11 / FE20 | 报告历史、报告详情、复制和导出 | `reportList`、`reportGet`、`reportDelete`、`searchReports` | `analysis_reports`、`search_documents`、FTS | 空报告列表；报告详情失败态；有报告时列表和详情字段一致；复制/导出不含完整 `input_snapshot` | `[!]` S5-10：收藏、批量、统计接口待确认 |
| FE12 / FE21 | 任务历史、事件回放、日志 | `taskList`、`taskGet`、`taskEvents`、`taskLogsList`、`taskLogGet`、`taskLogSummary`、`taskLogDiagnosis`、`taskLogContext`、`taskLogsExport` | `tasks`、`task_events`、`task_log_entries` | 空任务列表；FAILED 展示脱敏失败原因；事件按 ID 递增；日志空态和错误态 | `[!]` S5-15：任务重试和报告跳转规则待确认 |
| FE14 | Prompt 配置 | `promptTemplatesList`、`promptTemplatesGet`、`promptTemplatesCreate`、`promptTemplatesUpdate`、`promptTemplatesDelete` | `prompt_templates` | 列表/详情/创建/更新/删除；非法变量拒绝；内置模板删除展示后端错误 | 无新增阻塞 |
| FE15 / FE17-FE19 | 模型配置和 AI 分析任务 | `aiConfigList`、`aiConfigSave`、`aiConfigDelete`、`aiConfigTest`、`analysisTaskCreate`、`analysisTaskCancel`、`analysisTaskSubscribe`、`taskEvents`、`reportGet` | `ai_configs`、Rust vault、外部模型 Provider、`tasks`、`task_events`、`analysis_reports` | 未配置模型不能创建任务；配置模型后测试连接；任务创建后订阅事件；成功读取报告，失败展示脱敏原因 | 外部模型网络和用户密钥依赖真实环境 |
| FE16 | 设置中心基础能力 | `settingsGet`、`settingsSet`、`workspaceGet`、`workspaceSet`、`workspaceOpen`、`workspaceMigrationPlan`、`workspaceMigrate`、`cacheStats`、`cacheClean`、`providersStatus`、`searchStatus`、`searchRebuild`、`checkUpdate`、`exportLogs` | `settings`、工作区文件系统、缓存目录、Provider 状态、搜索索引、日志目录 | 设置读取/保存后刷新回显；缓存清理确认；搜索索引刷新/重建；日志导出脱敏 | `[!]` S2-07：代理连接测试；`[!]` S6-03：LICENSE/手册/发布说明打开方式 |

- 初版执行记录：
  - 已基于 FE02 契约矩阵和 S0/S5 清单完成页面级数据来源梳理。
  - 未通过真实 Tauri GUI 拉取最小样本；真实样本记录需在 FE04 桌面 smoke 后补充。
  - 当前可先推进 FE07-FE12 的空态、错误态和 Provider 不可用态对接；涉及 `[!]` 的扩展能力不得继续 mock，需要先确认方案。

---

## 7. FE2：只读页面真实数据接入

### FE07 接入 Dashboard 只读数据

- 状态：`[~]`
- 依赖：FE06A
- 交付物：
  - `apps/frontend/src/pages/dashboard/DashboardPage.tsx`
  - Dashboard 相关组件和测试
- 执行动作：
  - 使用 `core_health()` 和 `dashboardSummary()` 读取真实数据。
  - 展示自选股摘要、最近报告、最近任务、市场新闻和 Provider 状态。
  - 移除页面内硬编码业务统计。
- 验收标准：
  - 正常、空数据、Provider 异常三种状态可用。
  - 不出现策略、公告、研报、资金流等首版未闭环入口。
  - `App.test.tsx` 或页面测试覆盖主要状态。
- 退出条件：
  - 首页能反映真实首版数据状态。
- 实施记录：
  - Dashboard store 已通过 `coreHealth()`、`dashboardSummary()`、`watchlistList()`、`marketQuote()`、`marketKline()` 编排首页真实数据。
  - 总览页已展示后端返回的自选股摘要、最近报告、最近任务、市场新闻和 Provider 状态。
  - 已移除首页热点区未闭环的行业热点、概念热点、重点观察入口；涨跌分布卡不再展示硬编码“暂未接入”统计占位。
  - 自动化覆盖：`apps/frontend/src/stores/dashboardStore.test.ts`、`apps/frontend/src/components/dashboard/DashboardOverview.test.tsx`。
  - 端到端待测：真实桌面环境中 Dashboard 正常态、空态、Provider 异常态和 K 线迷你走势由用户验收。

### FE08 接入自选股只读和详情入口

- 状态：`[~]`
- 依赖：FE06A
- 交付物：
  - `apps/frontend/src/components/watchlist/*`
- 执行动作：
  - 使用 `watchlistList()` 获取自选股。
  - 对每个 symbol 调用 `marketQuote()` 展示行情。
  - 行情失败时展示错误，不填假价格。
  - 点击 symbol 进入 `/stocks/:symbol`。
  - 使用已加载的真实列表字段做市场和标签本地筛选，不新增后端接口。
- 验收标准：
  - 空自选展示空态。
  - 行情失败不影响列表整体展示。
  - 详情入口携带标准 symbol。
- 退出条件：
  - 自选股列表不再依赖本地 mock。
- 执行记录：
  - 2026-06-23 已确认自选股页面通过 `watchlistList()` 拉取列表，通过 `marketQuote()` 补行情；行情失败降级为空值，不填假价格。
  - 2026-06-23 已确认新增、编辑、删除分别调用 `watchlistCreate()`、`watchlistUpdate()`、`watchlistDelete()`；新增前通过 `stockSearch()` 选取真实股票。
  - 2026-06-23 已将市场和标签筛选改为基于已加载列表的本地过滤，移除“筛选功能待接入”假交互。
  - 自动化覆盖：`apps/frontend/src/components/watchlist/WatchlistPage.test.tsx` 已覆盖空态、搜索新增、编辑、删除、备注范围搜索、市场/标签本地筛选。
  - 待用户端到端验收：桌面环境连接真实 Go Core 后，空列表、新增、编辑、删除、刷新、详情跳转、行情 Provider 异常手工复核。
  - 仍阻塞：行业、趋势、星标等扩展字段见 S5-04，未补假数据。

### FE09 接入个股详情、K 线、指标和新闻

- 状态：`[ ]`
- 依赖：FE08
- 交付物：
  - `apps/frontend/src/components/stock-detail/*`
- 执行动作：
  - 调用 `marketQuote()`、`marketKline()`、`marketIndicators()` 和 `newsList()`。
  - K 线按后端返回数据展示，不在前端补计算业务指标。
  - period/adjust 切换重新请求并保留合理加载态。
  - 新闻外链只允许 HTTPS，并通过 `open_external_url` 打开。
- 验收标准：
  - quote、K 线、指标、新闻各自有加载、空态和错误态。
  - period/adjust 切换不破坏图表。
  - 非 HTTPS 新闻链接不会被打开。
- 退出条件：
  - 用户可以查看真实股票行情、K 线、技术指标和相关新闻状态。

### FE10 接入资讯中心

- 状态：`[ ]`
- 依赖：FE06A
- 交付物：
  - `apps/frontend/src/pages/news/*`
- 执行动作：
  - 使用 `newsMarket()` 读取市场新闻。
  - 输入股票代码后使用 `newsList()` 读取个股新闻。
  - 支持后端返回 tags 的本地筛选。
  - 移除公告、研报、资金流入口。
- 验收标准：
  - 市场新闻、个股新闻、空态、错误态可用。
  - 新闻链接过滤 HTTPS。
  - Provider 未配置时显示不可用而不是假新闻。
- 退出条件：
  - 资讯中心只展示首版真实可用数据。

### FE11 接入报告历史只读详情

- 状态：`[ ]`
- 依赖：FE05、FE06A
- 交付物：
  - `apps/frontend/src/pages/reports/*`
- 执行动作：
  - 使用 `reportList()` 和 `reportGet()`。
  - 报告列表支持筛选、分页和空态。
  - 报告详情默认只展示公开字段。
  - 不展示完整 `input_snapshot`。
- 验收标准：
  - 空报告列表展示空态。
  - 报告详情加载失败展示错误。
  - 默认复制和导出不包含完整输入快照。
- 退出条件：
  - 报告历史不再依赖静态 mock 报告。

### FE12 接入任务历史和事件回放

- 状态：`[ ]`
- 依赖：FE05、FE06A
- 交付物：
  - `apps/frontend/src/pages/tasks/*`
- 执行动作：
  - 使用 `taskList()`、`taskGet()` 和 `taskEvents()`。
  - 任务详情先补拉持久化事件。
  - FAILED 任务展示 `error_message` 或脱敏失败原因。
  - RUNNING 任务以 Go core 恢复结果为准。
- 验收标准：
  - 空任务列表展示空态。
  - FAILED 任务可看到失败原因。
  - 事件按 ID 递增展示。
  - 重启 core 后不会展示永久 RUNNING 假状态。
- 退出条件：
  - 任务历史可追溯、可复查。

---

## 8. FE3：写操作和配置接入

### FE13 接入自选股写操作

- 状态：`[ ]`
- 依赖：FE08
- 交付物：
  - 自选股页面、添加弹窗和测试
- 执行动作：
  - 使用 `stockSearch()` 搜索股票。
  - 使用 `watchlistCreate()` 添加自选。
  - 使用 `watchlistUpdate()` 更新排序、标签和备注。
  - 使用 `watchlistDelete()` 删除自选。
  - 重复添加展示后端业务错误。
- 验收标准：
  - 添加后列表可见。
  - 删除后列表消失且可重新添加。
  - 重复添加不显示成功态。
  - 非法 id 不进入 Go core。
- 退出条件：
  - 用户可完成自选股基础管理。

### FE14 接入 Prompt 模板 CRUD

- 状态：`[ ]`
- 依赖：FE05
- 交付物：
  - `apps/frontend/src/pages/prompt-template/*`
- 执行动作：
  - 使用 `promptTemplatesList()`、`promptTemplatesGet()`、`promptTemplatesCreate()`、`promptTemplatesUpdate()`、`promptTemplatesDelete()`。
  - 前端只展示首版模板类型。
  - 保存前拒绝未支持变量。
  - 删除内置模板时展示后端错误。
- 验收标准：
  - 列表、详情、创建、更新、删除可用。
  - `announcements`、`reports`、`portfolio` 等未支持变量无法保存。
  - 非正数模板 ID 早失败。
- 退出条件：
  - Prompt 模板页面走真实 CRUD。

### FE15 接入 AI 配置和模型测试

- 状态：`[ ]`
- 依赖：FE05
- 交付物：
  - `apps/frontend/src/pages/settings/model-config/*`
- 执行动作：
  - 使用 `aiConfigList()`、`aiConfigSave()`、`aiConfigDelete()` 和 `aiConfigTest()`。
  - API Key 明文只在保存时传给 Rust command。
  - 保存成功后清空明文输入。
  - 列表只展示 `has_api_key` 和 `masked_api_key`。
  - 失败消息在页面层二次脱敏。
- 验收标准：
  - 保存后刷新页面不出现真实 Key。
  - 测试连接失败不暴露密钥、请求头或代理认证。
  - 删除配置同步删除 vault 引用。
  - 前端源码不出现 `resolved_api_key`。
- 退出条件：
  - 用户可以配置模型并执行安全的连通性测试。

### FE16 接入设置中心基础能力

- 状态：`[ ]`
- 依赖：FE06A
- 交付物：
  - `apps/frontend/src/pages/settings/*`
- 执行动作：
  - 使用 `settingsGet()`、`settingsSet()`、`workspaceGet()`、`workspaceSet()`。
  - 使用 `cacheStats()`、`cacheClean()`、`providersStatus()`。
  - 使用 `checkUpdate()` 和 `exportLogs()`。
  - 使用 `autostartGet()` 和 `autostartSet()` 展示开机自启状态。
- 验收标准：
  - 设置页读取真实 settings/cache/provider API。
  - 代理 URL 带 username/password 时前端早失败。
  - 缓存清理不会包含报告和配置。
  - 空日志导出目录早失败，不触发 Rust command。
  - 关于页只展示 FREE 占位，不提供授权激活入口。
- 退出条件：
  - 设置页没有假按钮、假状态或半成品入口。

---

## 9. FE4：AI 分析长任务和 SSE 接入

### FE17 接入分析任务创建

- 状态：`[ ]`
- 依赖：FE15
- 交付物：
  - `apps/frontend/src/pages/analysis/*`
- 执行动作：
  - 选择股票、分析类型、AI 模型和 Prompt 模板。
  - 使用 `analysisTaskCreate()` 创建任务。
  - 请求只携带 `api_key_ref`，不携带真实 Key。
  - `user_position` 只作为本次任务上下文。
- 验收标准：
  - 参数缺失时前端阻止提交。
  - 创建成功返回 task ID。
  - 创建失败展示脱敏错误。
  - 前端类型不包含 `resolved_api_key`。
- 退出条件：
  - AI 分析页能发起真实任务。

### FE18 接入 SSE 订阅和事件合并

- 状态：`[ ]`
- 依赖：FE17
- 交付物：
  - 分析页运行态和任务事件处理逻辑
- 执行动作：
  - 创建任务后调用 `analysisTaskSubscribe()`。
  - 监听 Rust 转发的 `analysis-task-event`。
  - 通过 `taskEvents(task_id, after_event_id)` 补拉持久化事件。
  - 合并事件时按 ID 去重并递增展示。
- 验收标准：
  - `TASK_CREATED`、`TASK_STARTED`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`、`TASK_CANCELLED` 状态展示正确。
  - SSE 断线后可补拉。
  - 多行 `data:` 事件经 Rust 解析后前端仍可展示。
- 退出条件：
  - 分析任务运行态可追踪。

### FE19 接入取消、失败和终态报告

- 状态：`[ ]`
- 依赖：FE18
- 交付物：
  - 分析页操作区、报告展示区和测试
- 执行动作：
  - 使用 `analysisTaskCancel()` 取消任务。
  - `TASK_SUCCESS` 携带 `report_id` 时调用 `reportGet()`。
  - `TASK_FAILED` 展示脱敏失败原因。
  - `TASK_CANCELLED` 展示取消态，不追加失败态。
- 验收标准：
  - 取消后 UI 为 `CANCELLED`。
  - 成功后展示最终报告正文和风险摘要。
  - 失败后展示可理解的错误原因。
  - 复制和导出默认不包含完整 `input_snapshot`。
- 退出条件：
  - 分析任务从创建到终态报告形成闭环。

---

## 10. FE5：历史、回放、复制和导出闭环

### FE20 收口报告复制和 Markdown 导出

- 状态：`[ ]`
- 依赖：FE11、FE19
- 交付物：
  - 报告历史页、报告详情页和分析完成页
- 执行动作：
  - 统一报告 Markdown 生成字段。
  - 默认只包含标题、股票代码、分析类型、生成时间、正文和风险摘要。
  - 不包含完整 `input_snapshot`。
  - 复制和导出失败时展示明确错误。
- 验收标准：
  - 报告详情和分析完成页输出一致。
  - 导出文件不含 API Key、代理密码或完整用户持仓输入。
  - 复制失败不影响报告状态。
- 退出条件：
  - 报告可安全复用和留存。

### FE21 收口任务历史详情和日志入口

- 状态：`[ ]`
- 依赖：FE12
- 交付物：
  - `TaskHistoryPage`
  - `TaskLogDrawer`
- 执行动作：
  - 任务详情展示基础信息、事件时间线和失败原因。
  - 结构化日志入口读取真实 `task_logs_*` command。
  - Raw JSON 只展示脱敏 payload。
- 验收标准：
  - 没有任务日志时显示空态。
  - 日志查询失败不影响任务详情基础信息。
  - Raw JSON 不包含密钥、完整请求头、代理密码或用户隐私明细。
- 退出条件：
  - 用户可以从历史页复查任务执行过程。

---

## 11. FE6：mock 清理和页面状态收口

### FE22 删除页面 mock 数据入口

- 状态：`[ ]`
- 依赖：FE07-FE21
- 交付物：
  - 前端页面和组件源码
- 执行动作：
  - 搜索并删除生产页面中的 mock 数据文件和静态业务数组。
  - 将必要的测试 fixture 移入测试文件或测试目录。
  - 确认默认页面数据全部来自 typed service 或明确空态。
- 验收标准：
  - mock 数据扫描通过。
  - 页面仍能渲染加载、空态、错误态和真实数据。
  - 测试 fixture 不进入生产 bundle。
- 退出条件：
  - UI 不再以 mock 数据伪装真实能力。

### FE23 收口页面状态和风险提示

- 状态：`[ ]`
- 依赖：FE22
- 交付物：
  - 各页面状态展示和风险提示组件
- 执行动作：
  - 为每个页面确认加载、空态、错误态、不可用状态。
  - 保留“仅作研究辅助，不构成投资建议”边界。
  - 错误态展示 request ID / trace ID。
  - 避免页面因空数据出现布局塌陷。
- 验收标准：
  - 每个主页面至少覆盖 4 类状态之一的自动化测试或手工验收记录。
  - 投研合规文案没有被删除。
  - 错误态不泄露敏感字段。
- 退出条件：
  - 真实后端状态不会破坏 UI 基础体验。

### FE24 扫描首版未闭环入口

- 状态：`[ ]`
- 依赖：FE22
- 交付物：
  - 首版入口扫描测试或验收记录
- 执行动作：
  - 扫描策略观察、授权激活、公告、研报、资金流、券商账户、自动下单、云同步、移动端等文案和入口。
  - 保留必要的不可用说明，但不能有可点击未闭环操作。
- 验收标准：
  - 首版导航和路由白名单测试通过。
  - 未闭环能力不会作为可用入口出现在主界面。
- 退出条件：
  - 页面范围与 MVP 一致。

---

## 12. FE7：自动化、桌面 smoke 和验收报告

### FE25 执行 Go 验证

- 状态：`[ ]`
- 依赖：FE03-FE24、FE06A
- 交付物：
  - Go 测试输出
- 执行动作：
  - 在 `apps/sidecar-core` 执行 Go 测试。
  - 重点关注 actions、service、dao、task、report、settings、market、news、ai config 和 tasklog。
- 验收标准：
  - `go test ./...` 通过。
  - 如因网络、真实 Provider 或沙箱失败，记录失败命令、原因、影响范围和剩余风险。
- 退出条件：
  - Go API 和 service 基线未被前端对接破坏。

### FE26 执行前端验证

- 状态：`[ ]`
- 依赖：FE03-FE24、FE06A
- 交付物：
  - 前端测试、类型检查和构建输出
- 执行动作：
  - 执行 frontend 单测。
  - 执行类型检查。
  - 执行生产构建。
- 验收标准：
  - `pnpm --dir apps --filter @invest-compass/frontend test` 通过。
  - `pnpm --dir apps --filter @invest-compass/frontend check` 通过。
  - `pnpm --dir apps --filter @invest-compass/frontend build` 通过。
  - mock 数据、直连 Go、敏感字段和未闭环入口扫描通过。
- 退出条件：
  - 前端对接结果可自动化回归。

### FE27 执行 Rust 和桌面验证

- 状态：`[ ]`
- 依赖：FE04-FE24、FE06A
- 交付物：
  - Rust 测试输出和桌面 smoke 记录
- 执行动作：
  - 执行 Tauri Rust 测试。
  - 本地启动桌面应用，完成 health、provider status、settings、报告历史、任务历史和 AI 分析 smoke。
  - 如环境允许，执行 sidecar smoke。
- 验收标准：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml` 通过。
  - 本地 app 页面不会直接访问 Go sidecar。
  - 本地 vault、SSE 转发、日志导出路径等桌面能力表现符合预期。
  - 沙箱限制导致本地端口绑定失败时，记录为环境限制而不是默认代码通过。
- 退出条件：
  - Rust command 和桌面桥接满足对接要求。

### FE28 更新验收报告和剩余风险

- 状态：`[ ]`
- 依赖：FE25-FE27
- 交付物：
  - `docs/2026-06-18-invest-compass-acceptance-report.md`
  - 必要时同步用户手册或技术方案
- 执行动作：
  - 记录 FE0-FE7 每个阶段通过项、失败项和未执行项。
  - 明确真实 Market/News Provider、外部模型连通、跨平台桌面能力的剩余风险。
  - 不把 `[~]` 任务批量改成 `[x]`。
- 验收标准：
  - 验收报告能解释哪些对接已经真实可用，哪些仍待真实环境验证。
  - 文档与代码行为一致。
  - `git diff --check` 通过。
- 退出条件：
  - 前后端对接进入可验收、可复测状态。

---

## 13. 推荐验证命令

按最小必要到完整验收执行：

```bash
git diff --check
```

```bash
cd apps/sidecar-core
go test ./...
```

```bash
pnpm --dir apps --filter @invest-compass/frontend test
pnpm --dir apps --filter @invest-compass/frontend check
pnpm --dir apps --filter @invest-compass/frontend build
```

```bash
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
```

完整发布候选再执行：

```bash
pnpm --dir apps test
pnpm --dir apps check
pnpm --dir apps build
pnpm --dir apps release:check:local
```

如果本地沙箱禁止监听 `127.0.0.1`，`sidecar:smoke` 或 `release:check:local` 可能因端口绑定失败。此时必须在验收记录中写明环境限制，并在允许本地 bind 的环境复跑。

---

## 14. 子任务代理契约模板

下发并行任务时使用以下格式：

```text
代理名称：<职责>_<类型>

任务定义：
- 目标：完成 FE<编号> 的明确交付物。
- 输入：本文、技术方案第 8 章、主 checklist 对应 T 任务、目标文件路径。

执行动作：
- 只修改列出的文件范围。
- 不修改公共 API、数据库 schema、Rust command 名称、权限和依赖。
- 不引入 mock 数据伪装真实能力。
- 不降低 sidecar token、Rust 白名单、SSE、vault、日志脱敏边界。

预期结果：
- 列出修改文件。
- 列出验证命令和结果。
- 列出未完成项、阻塞项和剩余风险。
```

---

## 15. 对接契约矩阵模板

执行 FE02 时按这个格式补充：

| 页面 | typed service | Rust command | Go API | 主要状态 | 验收入口 |
| --- | --- | --- | --- | --- | --- |
| Dashboard | `dashboardSummary` | `dashboard_summary` | `POST /api/dashboard/summary` | 加载、空态、Provider 不可用、成功 | `App.test.tsx` |
| 自选股 | `watchlistList` / `marketQuote` | `watchlist_list` / `market_quote` | `POST /api/watchlist/list` / `POST /api/market/quote` | 空态、行情失败、成功 | `WatchlistPage.test.tsx` |
| 个股详情 | `marketKline` / `marketIndicators` / `newsList` | `market_kline` / `market_indicators` / `news_list` | `POST /api/market/kline` / `POST /api/market/indicators` / `POST /api/news/list` | K 线空态、指标空态、新闻错误 | `App.test.tsx` |
| AI 分析 | `analysisTaskCreate` / `analysisTaskSubscribe` / `taskEvents` | `analysis_task_create` / `analysis_task_subscribe` / `task_events` | `POST /api/analysis/tasks` / `POST /api/tasks/events/stream` / `POST /api/tasks/events` | 运行中、成功、失败、取消 | `App.test.tsx` |

实际执行时必须补全所有首版页面，并把缺失测试或阻塞项写入验收记录。

---

## 16. FE02 页面契约矩阵执行结果

> 生成时间：2026-06-23
>
> 代码基线：
>
> - typed service：`apps/frontend/src/services/coreClient.ts`
> - Rust command：`apps/desktop/src-tauri/src/commands/`
> - Go API：`apps/sidecar-core/internal/actions/`
> - SQLite 模型：`apps/sidecar-core/internal/model/schema.go`
>
> 说明：
>
> - 本节是后续 FE03-FE28 的对接索引，不表示下列页面都已通过真实数据验收。
> - 标记为 `[!]` 的能力不得继续 mock；需要先确认接口、数据库或页面处理方案。

### 16.1 全局壳层与只读页面

| 页面 / 区域 | 展示数据与点击动作 | typed service | Rust command | Go API / 桌面能力 | SQLite / 数据来源 | 状态与后续任务 |
| --- | --- | --- | --- | --- | --- | --- |
| AppShell / Sidebar / TopBar | Go Core、SQLite、数据源状态；刷新；股票搜索；应用内通知未读角标与列表浮层 | `coreHealth`、`providersStatus`、`stockSearch`、`notificationsUnreadCount`、`notificationsList`、`notificationsMarkRead`、`notificationsMarkAllRead`、`notificationsClearRead` | `core_health`、`providers_status`、`stock_search`、`notifications_*` | `POST /internal/health`、`POST /api/providers/status`、`POST /api/stocks/search`、`POST /api/notifications/*` | `stocks`、Provider 状态、sidecar runtime、`notifications` | `[~]` FE04、FE06、S4-03、S5-01；系统级通知触发仍见 S4-04/S4-05 |
| Dashboard 总览 | 概览指标、最近报告、自选摘要、Provider 状态 | `dashboardSummary`、`providersStatus` | `dashboard_summary`、`providers_status` | `POST /api/dashboard/summary`、`POST /api/providers/status` | `watchlists`、`analysis_reports`、`tasks`、Provider 状态 | `[ ]` FE07、S5-02；真实 Provider 未配置时只能展示不可用或空态 |
| 自选股 | 列表、行情补全、新增、编辑、删除、搜索股票 | `watchlistList`、`watchlistCreate`、`watchlistUpdate`、`watchlistDelete`、`marketQuote`、`stockSearch` | `watchlist_*`、`market_quote`、`stock_search` | `POST /api/watchlist/*`、`POST /api/market/quote`、`POST /api/stocks/search` | `watchlists`、`stocks`、`quotes`、Provider | `[ ]` FE08、FE13、S5-03；扩展字段需确认，见 S5-04 |
| 个股详情 | 行情、K 线、指标、新闻、外链打开 | `marketQuote`、`marketKline`、`marketIndicators`、`newsList`、`openExternalURL` | `market_quote`、`market_kline`、`market_indicators`、`news_list`、`open_external_url` | `POST /api/market/quote`、`POST /api/market/kline`、`POST /api/market/indicators`、`POST /api/news/list`、系统浏览器 | `quotes`、`klines`、`news_items`、Provider | `[ ]` FE09、S5-05；公司资料和标签接口缺口见 S5-06 |
| 资讯中心 | 市场新闻、个股新闻、筛选、打开原文 | `newsMarket`、`newsList`、`openExternalURL` | `news_market`、`news_list`、`open_external_url` | `POST /api/news/market`、`POST /api/news/list`、系统浏览器 | `news_items`、新闻 Provider | `[ ]` FE10、S5-12；侧栏统计、热点、加入上下文见 S5-13 |
| 报告历史 | 报告列表、详情、删除、全文搜索 | `reportList`、`reportGet`、`reportDelete`、`searchReports` | `report_list`、`report_get`、`report_delete`、`search_reports` | `POST /api/reports/*`、`POST /api/search/reports` | `analysis_reports`、`search_documents`、FTS | `[ ]` FE11、FE20、S5-09；收藏、批量、导出、统计见 S5-10 |
| 任务历史 | 任务列表、详情、事件、日志、诊断、上下文、日志导出 | `taskList`、`taskGet`、`taskEvents`、`taskLogsList`、`taskLogGet`、`taskLogSummary`、`taskLogDiagnosis`、`taskLogContext`、`taskLogsExport` | `task_*`、`task_log_*`、`task_logs_export` | `POST /api/tasks/*`、`POST /api/tasks/logs/*` | `tasks`、`task_events`、`task_log_entries` | `[ ]` FE12、FE21、S5-14；重试和报告跳转规则见 S5-15 |

### 16.2 AI、模型、Prompt 与长任务

| 页面 / 区域 | 展示数据与点击动作 | typed service | Rust command | Go API / 桌面能力 | SQLite / 数据来源 | 状态与后续任务 |
| --- | --- | --- | --- | --- | --- | --- |
| 模型配置 | 模型列表、新增、编辑、删除、默认模型、连通性测试 | `aiConfigList`、`aiConfigSave`、`aiConfigDelete`、`aiConfigTest` | `ai_config_list`、`ai_config_save`、`ai_config_delete`、`ai_config_test` | `POST /api/ai/configs/*`；Rust vault 注入 API Key | `ai_configs` + 本地 vault 引用 | `[~]` FE15、S2-01/S2-02/S2-03；测试依赖外部模型网络和用户密钥 |
| 基础设置默认 AI 模型 | 读取同一份默认模型；修改后立即保存 | `settingsGet`、`settingsSet`、`aiConfigList`、`aiConfigSave` | `settings_get`、`settings_set`、`ai_config_list`、`ai_config_save` | `POST /api/settings/*`、`POST /api/ai/configs/*` | `settings`、`ai_configs` | `[~]` S1-01、S2-02；与模型配置页共享同一配置源 |
| Prompt 配置 | 模板列表、详情、新增、编辑、删除 | `promptTemplatesList`、`promptTemplatesGet`、`promptTemplatesCreate`、`promptTemplatesUpdate`、`promptTemplatesDelete` | `prompt_templates_*` | `POST /api/prompt-templates/*` | `prompt_templates` | `[ ]` FE14、S2-04/S2-05；必须校验首版类型和变量白名单 |
| AI 分析 | 创建分析任务、取消、SSE 事件、任务完成后报告落地 | `analysisTaskCreate`、`analysisTaskCancel`、`analysisTaskSubscribe`、`taskEvents`、`reportGet` | `analysis_task_create`、`analysis_task_cancel`、`analysis_task_subscribe`、`task_events`、`report_get` | `POST /api/analysis/tasks`、`POST /api/tasks/cancel`、`POST /api/tasks/events/stream`、`POST /api/tasks/events` | `tasks`、`task_events`、`analysis_reports`、模型 Provider | `[ ]` FE17、FE18、FE19、S5-07/S5-08；依赖可用默认模型和真实行情/新闻上下文 |

### 16.3 设置中心

| 页面 / 区域 | 展示数据与点击动作 | typed service | Rust command | Go API / 桌面能力 | SQLite / 数据来源 | 状态与后续任务 |
| --- | --- | --- | --- | --- | --- | --- |
| 基础设置 | 主题、语言、市场、刷新频率、K 线周期、复权类型、默认 AI 模型 | `settingsGet`、`settingsSet`、`aiConfigList` | `settings_get`、`settings_set`、`ai_config_list` | `POST /api/settings/get`、`POST /api/settings/set`、`POST /api/ai/configs/list` | `settings`、`ai_configs` | `[~]` S1-01；本期已隐藏“其他设置” |
| 工作区设置 | 当前目录、选择目录、打开目录、迁移预检、迁移执行 | `workspaceGet`、`workspaceSet`、`workspaceOpen`、`workspaceMigrationPlan`、`workspaceMigrate`、`selectDirectory` | `workspace_get`、`workspace_set`、`workspace_open`、`workspace_migration_plan`、`workspace_migrate`、`select_directory` | `POST /api/workspace/get`、`POST /api/workspace/set`、系统文件选择/文件管理器 | `settings` + 用户文件系统 | `[~]` S1-02/S1-03；跨平台人工验收仍待补齐 |
| 缓存管理 | 缓存统计、清理缓存 | `cacheStats`、`cacheClean` | `cache_stats`、`cache_clean` | `POST /api/cache/stats`、`POST /api/cache/clean` | 工作区缓存目录、任务日志 | `[~]` S1-05；清理不得删除报告和配置 |
| 搜索索引 | FTS/GSE 状态、索引数量、重建全部/分类索引 | `searchStatus`、`searchRebuild` | `search_status`、`search_rebuild` | `POST /api/search/status`、`POST /api/search/rebuild` | `search_index_*`、`stocks`、`analysis_reports`、`news_items`、`watchlists` | `[~]` S1-06；GSE 当前可显示 fallback，不作为阻塞 |
| 桌面能力 | 开机自启、关闭后托盘 | `autostartGet`、`autostartSet`、`settingsGet`、`settingsSet` | `autostart_get`、`autostart_set`、`settings_get`、`settings_set` | Tauri autostart plugin、`POST /api/settings/*` | 系统登录项、`settings` | `[~]` S1-04；Windows/macOS 差异需手工验收 |
| 代理设置 | 代理模式、代理地址、脱敏回显、编辑入口 | `settingsGet`、`settingsSet` | `settings_get`、`settings_set` | `POST /api/settings/*` | `settings`；代理密码应走 vault | `[!]` S2-06/S2-07；代理密码安全存储和连接测试接口需确认 |
| 数据源概览 | Provider 真实状态、Provider 不可用空态；默认进入数据源设置时定位到概览 | `providersStatus` | `providers_status` | `POST /api/providers/status` | Provider runtime 状态 | `[ ]` S3-01；不能用假 Provider 状态；2026-06-23 已修正默认子 Tab 为数据源概览 |
| 凭据管理 | Provider 凭据脱敏状态、保存、清除、测试连接；股票行情源配置 | `dataSourceCredentialsList`、`dataSourceCredentialsSave`、`dataSourceCredentialsClear`、`dataSourceCredentialsTest` | `data_source_credentials_list`、`data_source_credentials_save`、`data_source_credentials_clear`、`data_source_credentials_test` | `POST /api/data-source/credentials/list`、`POST /api/data-source/credentials/save`、`POST /api/data-source/credentials/clear`、`POST /api/data-source/credentials/test` | `data_source_credentials`；AES-GCM 密文 + 工作区密钥文件；`sina`、`tencent` 为无需凭据股票数据 Provider；`settings.data_source.default_market_source` | `[~]` S3-02/S3-03 已完成；S3-04 待页面手工验收；S3-05 已补 resolver；2026-06-23 已拆分“新浪财经 / 腾讯财经”配置和对应真实预检目标；“自动降级”已接入 Go runtime 设置读取，当前沿用新浪搜索/实时行情、腾讯 K 线、东财 K 线兜底链路；显式单一源路由策略待确认 |
| 数据说明 | 数据范围、来源、时效、AI 上下文边界、FAQ | 静态说明 + 子 Tab 切换 + `message.info` | 无 | 无 | 文档型静态页面 | `[~]` S3-06；数据源概览、凭据管理跳转已接入，页面手工验收待补 |
| 通知设置 / TopBar 通知 | 系统通知开关、应用内未读角标、通知列表浮层 | `settingsGet/settingsSet`、`notificationsList`、`notificationsUnreadCount`、`notificationsMarkRead`、`notificationsMarkAllRead`、`notificationsClearRead`、`sendConfiguredDesktopNotification` | `settings_get/settings_set`、`notifications_*`、Tauri notification plugin | `POST /api/settings/*`、`POST /api/notifications/*`；系统通知走 Rust/Tauri | `settings`、`notifications` | `[~]` S4-01/S4-02/S4-03 已接 settings、通知表/API、TopBar Badge/Popover；S4-04 系统通知服务和权限降级已接，真实事件触发和 macOS 手工验收待 S4-05 |
| 关于应用 | 检查更新、日志导出 | `checkUpdate`、`exportLogs` | `check_update`、`export_logs` | `POST /api/update/check`、`POST /api/logs/export` | manifest 源、日志目录 | `[ ]` S6-01/S6-02；LICENSE/用户手册/发布说明打开方式见 S6-03 |

### 16.4 隐藏或冻结入口

| 入口 | 现有能力 | 风险 | 后续任务 |
| --- | --- | --- | --- |
| Scheduler / 调度任务 | `scheduler*` typed service、Rust command、Go API 已存在 | 页面入口是否进入首版仍需产品确认；不要主动暴露复杂调度后台 | S0-02 |
| Provider 配置、同步策略子 Tab | 当前 UI 已按要求从数据源子标签移除 | 不应在未确认前重新加回入口 | S0-02、S3-01 |
| 数据源凭据真实保存 | 已新增加密 SQLite 保存、脱敏回显、本地预检 API 和运行时 resolver | 生产请求尚未确认切换到哪些真实 Provider，不能宣称页面 Provider 已可用凭据访问 | S3-05 |

### 16.5 FE02 缺口索引

- `[!]` S2-07：代理连接测试接口未确认，不能实现假测试成功。
- `[~]` S3-05：Provider 运行时 resolver 已完成；配置好的 Cookie/token 要用于哪些生产 Provider 请求仍需确认。
- `[~]` S4-02/S4-03/S4-04/S4-05：应用内通知表/API、Rust command、TopBar 未读角标和通知列表浮层已实现；分析任务终态和 Provider 真实异常已生成应用内通知；TopBar 已消费运行期新增通知并按设置触发系统通知；macOS 系统通知手工验收和权限拒绝 UI 提示仍需推进。
- `[!]` S5-04：自选股扩展字段与展示口径未确认。
- `[!]` S5-06：个股详情公司资料和标签接口未确认。
- `[!]` S5-10：报告收藏、批量、导出、统计接口未确认。
- `[!]` S5-13：资讯侧栏统计、热点和加入上下文接口未确认。
- `[!]` S5-15：任务重试和报告跳转规则未确认。
- `[!]` S6-03：LICENSE、用户手册、发布说明打开方式未确认。

### 16.6 FE02 验收记录

- 已覆盖页面：Dashboard、自选股、个股详情、资讯中心、模型配置、Prompt 配置、AI 分析、报告历史、任务历史、设置中心、AppShell/TopBar。
- 已覆盖链路：typed service、Rust command、Go API、SQLite / Provider / vault / 文件系统数据来源。
- 已标记停止线：所有缺后端、缺数据库或缺页面决策的能力均映射到 `[!]` 任务编号。
- 本次验证：docs-only 修改执行 `git diff --check`。
