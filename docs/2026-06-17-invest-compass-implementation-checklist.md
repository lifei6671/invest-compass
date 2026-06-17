# 投研罗盘 Invest Compass 首版实施任务清单

> 来源：`docs/2026-06-17-invest-compass-technical-solution.md`
>
> 目标：把技术方案拆解为可执行、可推进、可验证的任务清单。本文只描述首版 MVP，后续版本能力不得提前混入首版主链路。

---

## 1. 首版范围

### 1.1 必须交付

- Tauri v2 桌面壳、React 前端、Go sidecar core 可以端到端启动。
- Rust 只暴露白名单 command，不提供任意路径代理。
- Go sidecar 只监听 `127.0.0.1`，runtime token 通过 stdin 握手传递。
- SQLite 使用 `sqlc + database/sql`，所有业务表通过 migration 管理。
- 股票搜索、自选股、行情、K 线、技术指标、新闻资讯形成基础数据闭环。
- OpenAI-compatible AI Provider、模型配置、Prompt 模板、模型测试可用。
- API Key 和代理密码保存到系统凭据管理器，SQLite 只保存引用标识。
- 个股 AI 分析任务支持进度/流式事件、取消、失败原因、报告保存。
- 报告历史、任务历史、事件回放和 RUNNING 任务恢复规则可用。
- 设置中心覆盖工作区、代理、通知、缓存、开机自启、检查更新入口。
- macOS 和 Windows 均完成基础启动、凭据、sidecar、打包验证。

### 1.2 明确不做

- 不做券商账户、实盘交易、自动下单、收益承诺。
- 不做云端同步、多人协作、移动端。
- 不做策略观察独立页面。
- 不做公告、研报、资金流专用数据源。
- 不做持仓信息落库。
- 不做授权激活闭环。
- 不做真实自动更新下载和安装。

---

## 2. 依赖路线

```text
P0 规划与仓库基线
  ↓
P1 项目骨架与 sidecar 安全启动
  ↓
P2 数据库、配置、日志与通用 API 基线
  ↓
P3 股票、行情、K线、指标、新闻基础能力
  ↓
P4 AI 配置、凭据、Provider、Prompt 模板
  ↓
P5 分析任务、SSE 转发、报告和任务历史
  ↓
P6 前端页面闭环与设置中心
  ↓
P7 跨平台桌面能力、打包、发布验收
```

并行原则：

- 同一阶段内，Go core、Rust command、前端页面可以在接口契约稳定后并行。
- 数据库 migration、API schema、Rust command 名称属于上游契约，必须先落定。
- API Key、SSE、sidecar token、日志脱敏属于安全关键路径，不能用临时弱实现绕过。
- 每个任务完成后只勾选本任务，不批量勾选后续依赖任务。

---

## 3. 状态标记

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成并通过验证
- `[!]` 阻塞或需要人工确认

---

## 4. P0：规划与仓库基线

### T00 固化实施清单和开发规则

- 状态：`[x]`
- 依赖：无
- 交付物：
  - 本文档作为首版实施 checklist。
  - README 说明项目定位、MVP 范围和本地开发入口。
  - 项目级 agent 规则，至少覆盖验证命令、文档同步、密钥边界。
- 执行动作：
  - 对齐技术方案第 1、16、19、20 章。
  - 明确首版不可出现的页面和入口。
  - 确认 docs-only 修改只需 `git diff --check`。
- 验证：
  - `git diff --check`
  - README 不承诺未实现能力。
  - checklist 中每个任务都有依赖、交付物、验证和退出条件。
- 退出条件：
  - 后续开发可以按本文任务号推进和勾选。

### T01 初始化 monorepo 工程骨架

- 状态：`[x]`
- 依赖：T00
- 交付物：
  - `apps/package.json`
  - `apps/pnpm-workspace.yaml`
  - `apps/desktop/`
  - `apps/frontend/`
  - `apps/sidecar-core/`
  - `apps/packages/shared/`
  - `scripts/`
- 执行动作：
  - 建立 pnpm workspace。
  - 建立 Go module。
  - 建立 Tauri v2 桌面应用目录。
  - 建立 shared API schema 目录。
- 验证：
  - `cd apps && pnpm install`
  - `go test ./...`
  - `cargo check` 或项目定义的 Tauri Rust 检查命令。
- 退出条件：
  - 三端项目均可独立执行最小构建/检查命令。

### T02 建立基础 CI 验证

- 状态：`[x]`
- 依赖：T01
- 交付物：
  - GitHub Actions workflow。
  - 前端类型检查、Go 测试、Rust 检查、格式化检查。
- 执行动作：
  - 按 workspace 定义 CI job。
  - 设置每类测试的超时。
  - 不加入发布签名和真实打包密钥。
- 验证：
  - 本地可执行与 CI 等价的检查命令。
  - CI 不需要真实 API Key。
- 退出条件：
  - PR 或 push 可以自动暴露构建、测试、格式问题。

---

## 5. P1：项目骨架与 sidecar 安全启动

### T03 Go core 最小 HTTP server

- 状态：`[x]`
- 依赖：T01
- 交付物：
  - `apps/sidecar-core/cmd/invest-compass-core/main.go`
  - `apps/sidecar-core/internal/server`
  - `/internal/health`
- 执行动作：
  - 只监听 `127.0.0.1`。
  - 所有 API 只接受 POST。
  - 返回统一响应结构和 `requestId` / `traceId`。
- 验证：
  - 非 POST 请求返回 405。
  - 未握手前业务路由不可用。
  - health 在 token 有效后返回版本和 DB 状态。
- 退出条件：
  - Go core 可以本地启动，并具备最小健康检查。

### T04 sidecar stdin token 握手

- 状态：`[x]`
- 依赖：T03
- 交付物：
  - Go stdin 握手逻辑。
  - 5 秒超时退出。
  - ready JSON 输出。
- 执行动作：
  - 从 stdin 读取单行 JSON。
  - 校验 `token` 和 `protocolVersion`。
  - token 安装前不注册业务路由。
- 验证：
  - 无 stdin 输入时 5 秒内退出。
  - 非法 JSON 时退出且不打印 token。
  - 合法握手后输出 `{"status":"ready","port":...}`。
- 退出条件：
  - runtime token 不通过 argv、env、日志、配置、数据库传递。

### T05 Tauri 启动和管理 sidecar

- 状态：`[x]`
- 依赖：T04
- 交付物：
  - Tauri sidecar 管理模块。
  - 随机端口和内存 token。
  - 启动、健康检查、退出清理。
- 执行动作：
  - Tauri 启动时生成 token。
  - 通过 stdin 写入握手 JSON。
  - 读取 ready JSON 后保存 port/token 到 Rust 内存。
  - 应用退出时调用 `/internal/shutdown`，失败时 kill sidecar。
- 验证：
  - 前端启动后可触发 health。
  - 退出应用后 sidecar 进程不存在。
  - 进程列表、日志、配置文件中没有 token。
- 退出条件：
  - Tauri + Go sidecar 生命周期可控。
- 当前进展：
  - 已实现 Rust sidecar token、stdin 握手、ready JSON 解析、health/shutdown client 和 `core_health` 白名单 command。
  - 已补 Go `/internal/shutdown`，合法 token 才能触发关闭回调。
  - 已补 `pnpm sidecar:build`，默认产物为 `apps/desktop/src-tauri/binaries/invest-compass-core`。
  - 已验证项目内 sidecar 二进制 stdin 握手、health、非 POST 拒绝和 shutdown 后进程退出。

### T06 Rust command 白名单代理基线

- 状态：`[x]`
- 依赖：T05
- 交付物：
  - `core_start`
  - `core_health`
  - 白名单 command 到 Go API 映射层。
- 执行动作：
  - 禁止 `core_request(method, path, body)`。
  - 每个 command 固定 Go API、HTTP 方法、请求 schema、响应字段。
  - Rust 向 Go 请求统一注入 token、request_id、trace_id。
- 验证：
  - 未注册 command 无法调用。
  - `/internal/*` 只能由 Rust 生命周期代码调用。
  - 任意 path 代理不存在。
- 退出条件：
  - 前端只能通过明确 command 访问 Go core。
- 当前进展：
  - 已注册 `core_start` 和 `core_health`，没有实现任意 path / method 代理。
  - Rust 内部请求统一注入 runtime token、`X-Request-Id`、`X-Trace-Id`。
  - Go core 二进制路径由 Rust 侧单一函数解析，环境变量仅作为本地覆盖入口。

### T07 Tauri capabilities 和 CSP 基线

- 状态：`[x]`
- 依赖：T06
- 交付物：
  - `apps/desktop/src-tauri/capabilities/`
  - 最小 command 权限。
  - CSP 配置。
- 执行动作：
  - 默认只给主窗口必要 core/window/event 权限。
  - 文件、通知、shell、updater 等插件按功能单独授权。
  - 外部链接只允许 HTTPS 并通过系统浏览器打开。
- 验证：
  - 无权限窗口不能调用敏感 command。
  - 远程 URL 不具备本地 command 权限。
  - Markdown 或外链不突破 CSP。
- 退出条件：
  - 桌面壳最小权限边界清晰。
- 当前进展：
  - 主窗口显式声明 `label: "main"`，并在 `tauri.conf.json` 中只启用 `main` capability。
  - 默认 capability 只包含 `core:default`、`core:window:default`、`core:event:default`。
  - 未使用的文件、通知、shell、updater 等插件权限不进入默认 capability。
  - CSP 保持 `default-src 'self'`，并通过自动化测试防止任意远程脚本和 `unsafe-eval`。
  - 已新增 `@invest-compass/desktop` 配置测试，覆盖 capability 显式绑定、最小插件权限和 CSP 脚本边界。

---

## 6. P2：数据库、配置、日志与通用 API 基线

### T08 SQLite migration 基线

- 状态：`[ ]`
- 依赖：T03
- 交付物：
  - `apps/sidecar-core/migrations/`
  - `apps/sidecar-core/internal/storage`
  - 初始 schema migration。
- 执行动作：
  - 建立 `stocks`、`watchlists`、`quotes`、`klines`、`news_items`。
  - 建立 `ai_configs`、`prompt_templates`、`analysis_reports`。
  - 建立 `tasks`、`task_events`、`settings`。
  - 所有表包含 `created_at`、`updated_at`。
- 验证：
  - migration 可在空库执行成功。
  - 重复执行不会破坏已有库。
  - 唯一约束和软删除索引符合技术方案。
- 退出条件：
  - 本地数据库结构可支撑首版闭环。

### T09 sqlc 查询和 storage 事务层

- 状态：`[ ]`
- 依赖：T08
- 交付物：
  - sqlc 配置。
  - 关键 CRUD query。
  - 事务封装。
- 执行动作：
  - 为自选股、AI 配置、Prompt、任务、事件、报告生成 query。
  - 重要写操作使用事务。
  - 不写手拼 SQL 到业务 handler。
- 验证：
  - `sqlc generate`
  - Go 单测覆盖 CRUD、唯一约束、软删除。
- 退出条件：
  - 数据访问层类型安全且可测试。

### T10 统一日志、错误和脱敏

- 状态：`[x]`
- 依赖：T03
- 交付物：
  - `apps/sidecar-core/internal/logger`
  - 统一错误响应。
  - secret redaction。
- 执行动作：
  - slog 字段统一使用 `request_id`、`trace_id`、`task_id`、`provider`、`symbol`。
  - redaction 覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户持仓输入。
  - panic recovery 返回统一错误结构。
- 验证：
  - 单测输入包含密钥时日志和错误都脱敏。
  - Provider 错误不包含完整请求头。
- 退出条件：
  - 日志、错误、导出前都具备统一脱敏入口。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/logger`，提供稳定日志字段常量和统一脱敏入口。
  - 脱敏覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户一次性持仓输入。
  - HTTP handler 已接入 panic recovery，panic 会返回统一错误 envelope，并在写入日志前脱敏。
  - 单测覆盖字段契约、错误脱敏、密钥脱敏、nil error 和 panic recovery。

### T11 settings / workspace / cache API 基线

- 状态：`[ ]`
- 依赖：T08、T10
- 交付物：
  - `/api/settings/get`
  - `/api/settings/set`
  - `/api/workspace/get`
  - `/api/workspace/set`
  - `/api/cache/stats`
  - `/api/cache/clean`
- 执行动作：
  - settings 只保存非敏感配置或凭据引用。
  - 工作区路径通过系统目录或用户选择得到。
  - 缓存清理不得删除报告和配置。
- 验证：
  - 设置保存和读取一致。
  - 缓存清理只影响临时缓存。
  - 非法路径返回明确错误。
- 退出条件：
  - 设置中心可依赖真实 API，不需要假状态。

---

## 7. P3：股票、行情、K线、指标、新闻基础能力

### T12 股票代码模型和标准化

- 状态：`[~]`
- 依赖：T09
- 交付物：
  - `apps/sidecar-core/internal/stock`
  - Symbol parser / validator。
- 执行动作：
  - 支持 `CN:SH:600519`、`CN:SZ:300750`、`HK:00700`、`US:AAPL` 等格式。
  - 错误响应使用稳定错误码。
- 验证：
  - table-driven tests 覆盖合法、非法、边界 symbol。
- 退出条件：
  - 股票代码成为所有后续 API 的统一输入类型。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/stock` 纯模型包，完成 `CN:SH:600519`、`CN:SZ:300750`、`HK:00700`、`US:AAPL` 解析和大小写标准化。
  - 已定义稳定错误码：空输入、格式错误、不支持市场、不支持交易所、代码非法。
  - table-driven tests 已覆盖合法、非法和边界输入。
  - 受 T09 依赖约束，后续 API/storage 接入完成后再标记为 `[x]`。

### T13 首版合规 Market Provider

- 状态：`[~]`
- 依赖：T12
- 交付物：
  - `MarketProvider` 接口。
  - 至少一个合规可用数据 Provider。
  - Provider 状态描述。
- 执行动作：
  - 明确数据来源、授权边界和频率限制。
  - 实现 search、quote、kline。
  - Provider 错误要可观测但不泄露敏感请求。
- 验证：
  - Provider mock 单测。
  - 真实 Provider 在本地开发环境可查询样例股票。
- 退出条件：
  - 首版不依赖未授权或不稳定的隐式抓取路径。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/market` 契约包，定义 `MarketProvider`、Provider 状态、股票基础信息、行情快照和 K 线模型。
  - Provider 状态模型强制携带来源、授权边界、频率限制和支持市场描述。
  - Provider 错误会保留 provider/operation 可观测上下文，并复用统一脱敏入口避免泄露授权头和 API Key。
  - 单测已覆盖接口契约、标准 symbol 使用、合规状态描述和错误脱敏。
  - 真实合规数据 Provider 仍需确认数据源授权和访问限制后接入，完成后再标记为 `[x]`。

### T14 股票搜索和基础信息 API

- 状态：`[ ]`
- 依赖：T12、T13
- 交付物：
  - `/api/stocks/search`
  - `stock_search(keyword)` Rust command。
- 执行动作：
  - keyword 做输入校验。
  - 搜索结果统一返回 symbol、name、code、market、exchange。
  - 写入或更新 `stocks` 缓存。
- 验证：
  - 空 keyword 返回 400。
  - 有效 keyword 返回标准 symbol。
  - Rust command 到 Go API 闭环可用。
- 退出条件：
  - 前端可以基于真实 command 搜索股票。

### T15 自选股 CRUD

- 状态：`[ ]`
- 依赖：T14
- 交付物：
  - `/api/watchlist/list`
  - `/api/watchlist/create`
  - `/api/watchlist/update`
  - `/api/watchlist/delete`
  - 对应 Rust command。
- 执行动作：
  - 支持标签、备注、排序。
  - 删除使用软删除。
  - active symbol 唯一约束生效。
- 验证：
  - 重复添加同一 active symbol 返回明确错误。
  - 删除后可重新添加。
  - 列表不返回 deleted 数据。
- 退出条件：
  - 自选股页面可完整使用真实数据。

### T16 行情、K线和缓存

- 状态：`[ ]`
- 依赖：T13、T15
- 交付物：
  - `/api/market/quote`
  - `/api/market/kline`
  - `market_quote`
  - `market_kline`
  - quotes / klines 缓存写入。
- 执行动作：
  - K 线支持 period、adjust。
  - 行情短缓存 10-60 秒。
  - K 线按交易日缓存。
- 验证：
  - quote 返回价格、涨跌幅、时间、provider。
  - kline 返回有序 K 线数组。
  - 缓存命中不重复请求 Provider。
- 退出条件：
  - 个股详情页可展示真实行情和 K 线。

### T17 技术指标计算

- 状态：`[~]`
- 依赖：T16
- 交付物：
  - `apps/sidecar-core/internal/indicator`
  - `/api/market/indicators`
  - `market_indicators`
- 执行动作：
  - 实现 MA、EMA、MACD、RSI、KDJ、BOLL。
  - 实现成交量均线、涨跌幅、区间最大回撤、区间波动率。
  - 指标由 Go core 计算，前端只展示。
- 验证：
  - 每个指标都有固定输入输出单测。
  - 数据不足时返回明确错误或空结果语义。
- 退出条件：
  - 前端无需自行计算业务指标。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/indicator` 纯计算模块。
  - 已实现 MA、EMA、MACD、RSI、KDJ、BOLL、成交量均线、涨跌幅、区间最大回撤、区间波动率。
  - 固定输入输出单测覆盖全部指标，并覆盖数据不足、非法周期、非法输入的稳定错误码。
  - 受 T16 依赖约束，`/api/market/indicators` 和 `market_indicators` command 接入完成后再标记为 `[x]`。

### T18 新闻资讯基础能力

- 状态：`[~]`
- 依赖：T12、T10
- 交付物：
  - `apps/sidecar-core/internal/news`
  - `/api/news/list`
  - `/api/news/market`
  - 对应 Rust command。
- 执行动作：
  - 支持个股新闻和市场新闻。
  - 使用 content_hash 去重。
  - 新闻缓存 30-120 分钟。
  - 不接公告、研报、资金流专用数据源。
- 验证：
  - 重复新闻不会重复入库。
  - 列表按发布时间排序。
  - 新闻 URL 输出前经过 scheme 校验。
- 退出条件：
  - Dashboard、资讯中心、分析上下文可使用新闻数据。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/news` 纯模块，定义新闻条目、个股新闻请求、市场新闻请求和 Provider 契约。
  - 已实现 HTTP(S) URL scheme 白名单校验，禁止 `javascript:`、`file:` 等危险链接进入输出。
  - 已实现 `content_hash` 去重，忽略 URL 和来源以合并多来源转载，并保留首次出现条目。
  - Provider 错误复用统一脱敏入口，避免授权头和 API Key 泄露。
  - 单测覆盖 URL 校验、标准 symbol 绑定、去重、个股/市场新闻契约和错误脱敏。
  - 受 T09/T14 后续数据与 API 链路约束，缓存、入库、排序 API 和 Rust command 接入完成后再标记为 `[x]`。

### T19 Dashboard summary 和 provider status

- 状态：`[ ]`
- 依赖：T15、T16、T18
- 交付物：
  - `/api/dashboard/summary`
  - `/api/providers/status`
  - 对应 Rust command。
- 执行动作：
  - Dashboard 只聚合自选股、最近报告、最近任务、市场新闻、风险提示。
  - Provider status 展示可用性、最近错误、数据来源说明。
- 验证：
  - 不返回策略、公告、研报、资金流字段。
  - Provider 异常时 Dashboard 可显示可恢复错误。
- 退出条件：
  - 总览页和数据源状态页没有假数据入口。

---

## 8. P4：AI 配置、凭据、Provider、Prompt 模板

### T20 系统凭据管理适配

- 状态：`[ ]`
- 依赖：T07
- 交付物：
  - macOS Keychain 适配。
  - Windows Credential Manager 适配。
  - Rust 凭据服务接口。
- 执行动作：
  - 保存、读取、删除 AI Key。
  - 保存、读取、删除代理密码。
  - SQLite 只保存 `api_key_ref`、`proxy_credential_ref`。
- 验证：
  - macOS 保存/读取/删除验收通过。
  - Windows 保存/读取/删除验收通过。
  - 前端和 Go core 不持久化真实 Key。
- 退出条件：
  - 敏感凭据只存在系统凭据管理器和运行期内存。

### T21 AI 配置 API 和 Rust 内部密钥注入

- 状态：`[ ]`
- 依赖：T09、T20
- 交付物：
  - `/api/ai/configs/list`
  - `/api/ai/configs/save`
  - `/api/ai/configs/delete`
  - `/api/ai/configs/test`
  - `ai_config_*` Rust command。
- 执行动作：
  - 前端保存时只把明文 Key 交给 Rust command。
  - Rust 写入系统凭据管理器。
  - Go 只保存 `api_key_ref` 和脱敏状态。
  - test 和 analysis 复用 `resolved_api_key` 内部注入协议。
- 验证：
  - list 不返回真实 API Key。
  - delete 同步删除系统凭据项。
  - test 失败错误不包含 Key、请求头、代理认证。
- 退出条件：
  - 模型配置页可真实保存和测试模型。

### T22 OpenAI-compatible Provider

- 状态：`[~]`
- 依赖：T21
- 交付物：
  - `apps/sidecar-core/internal/ai`
  - OpenAI-compatible chat / stream chat。
- 执行动作：
  - 支持 base URL、model、temperature、max_tokens、timeout。
  - 支持普通响应和流式响应。
  - 所有调用支持 context cancellation。
- 验证：
  - mock server 覆盖成功、401、429、5xx、超时、流式 chunk。
  - 请求日志不包含 Authorization。
- 退出条件：
  - AI Provider 可被模型测试和分析任务复用。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/ai`，实现 OpenAI-compatible `/v1/chat/completions` 标准库 HTTP 客户端。
  - 已支持 base URL、model、temperature、max_tokens、timeout、普通 chat 和 stream chat。
  - 已支持 context cancellation，并将 401、429、5xx、取消/超时映射为稳定错误码。
  - 流式响应支持 `data: ...` chunk 和 `[DONE]` 结束语义。
  - Provider 错误会统一脱敏，错误文本不泄露 Authorization 或 API Key。
  - mock server 单测覆盖成功、401、429、5xx、context cancellation 和流式 chunk。
  - 受 T21 依赖约束，模型配置读取、系统凭据注入、真实连通性测试和分析任务复用接入后再标记为 `[x]`。

### T23 Prompt 模板 CRUD 和变量白名单

- 状态：`[~]`
- 依赖：T09
- 交付物：
  - `/api/prompt-templates/list`
  - `/api/prompt-templates/get`
  - `/api/prompt-templates/create`
  - `/api/prompt-templates/update`
  - `/api/prompt-templates/delete`
  - 对应 Rust command。
- 执行动作：
  - 首版只允许 `system`、`stock_full`、`technical`、`custom`。
  - 变量只允许 `stock_name`、`stock_code`、`market`、`quote`、`kline_summary`、`indicators`、`news`、`analysis_language`。
  - 内置模板只读。
- 验证：
  - 未支持变量保存失败。
  - 删除只对 custom 或非内置模板生效。
  - CRUD 不返回已软删除模板。
- 退出条件：
  - Prompt 模板页没有不可执行模板类型。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/prompt` 纯规则模块，定义首版模板类型和变量白名单。
  - 已限制模板类型只允许 `system`、`stock_full`、`technical`、`custom`。
  - 已限制变量只允许 `stock_name`、`stock_code`、`market`、`quote`、`kline_summary`、`indicators`、`news`、`analysis_language`。
  - 已实现内置模板删除规则、软删除过滤和变量提取去重。
  - 单测覆盖未支持模板类型、未支持变量、删除规则、软删除过滤和变量提取顺序。
  - 受 T09 依赖约束，CRUD API、Rust command 和数据库持久化接入完成后再标记为 `[x]`。

### T24 Prompt 构建和合规输出约束

- 状态：`[~]`
- 依赖：T17、T18、T23
- 交付物：
  - `apps/sidecar-core/internal/prompt`
  - 个股综合分析 Prompt builder。
  - 技术面分析 Prompt builder。
- 执行动作：
  - System、Context、User 三层 Prompt 分离。
  - 用户一次性持仓输入只进入本次任务上下文。
  - 输出约束必须包含非投资建议、风险、数据时效、事实/推断区分。
- 验证：
  - 单测检查缺失数据、含 userPosition、无 userPosition。
  - 生成 Prompt 不包含未支持变量。
- 退出条件：
  - 分析任务可以稳定构建首版支持的 Prompt。
- 当前进展：
  - 已在 `apps/sidecar-core/internal/prompt` 中新增个股综合分析和技术面分析 Prompt builder。
  - 已实现 System、Context、User 三层 Prompt 分离。
  - System Prompt 固定包含“不构成投资建议”、风险、数据时效、事实/推断/观点区分、观察指标和用户自行决策提示。
  - 用户一次性持仓输入只进入 User Prompt，不进入 System 或 Context。
  - 单测覆盖缺失数据、含 userPosition、无 userPosition、合规文案和未替换变量残留。
  - 受 T26 分析任务链路约束，任务创建流程接入后再标记为 `[x]`。

---

## 9. P5：分析任务、SSE 转发、报告和任务历史

### T25 任务状态机和事件持久化

- 状态：`[~]`
- 依赖：T09、T10
- 交付物：
  - `apps/sidecar-core/internal/task`
  - tasks / task_events 写入逻辑。
- 执行动作：
  - 支持 `PENDING`、`RUNNING`、`SUCCESS`、`FAILED`、`CANCELLED`。
  - 支持 `TASK_CREATED`、`TASK_STARTED`、`TASK_PROGRESS`、`TASK_LOG`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`、`TASK_CANCELLED`。
  - 事件 payload 进入数据库前脱敏。
- 验证：
  - 非法状态迁移失败。
  - task_events 按 id 递增可回放。
  - 敏感字段不会进入事件 payload。
- 退出条件：
  - 长任务有可恢复、可查询的状态和事件基础。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/task` 纯状态机和事件模型。
  - 已实现 `PENDING`、`RUNNING`、`SUCCESS`、`FAILED`、`CANCELLED` 状态定义和合法流转校验。
  - 已实现 `TASK_CREATED`、`TASK_STARTED`、`TASK_PROGRESS`、`TASK_LOG`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`、`TASK_CANCELLED` 事件类型。
  - 已实现事件 payload 入库前统一脱敏、按 id 递增回放、RUNNING 任务恢复为终态的基础决策。
  - 单测覆盖非法状态迁移、事件状态映射、事件回放排序、RUNNING 恢复和敏感 payload 脱敏。
  - 受 T09 依赖约束，tasks / task_events 数据库写入逻辑完成后再标记为 `[x]`。

### T26 分析任务创建和取消

- 状态：`[~]`
- 依赖：T16、T17、T18、T21、T24、T25
- 交付物：
  - `/api/analysis/tasks`
  - `/api/tasks/cancel`
  - `analysis_task_create`
  - `analysis_task_cancel`
- 执行动作：
  - 校验 symbol、analysisType、aiConfigId、promptTemplateId。
  - Rust 注入 `resolved_api_key`。
  - Go 按流程拉行情、K 线、指标、新闻、构建 Prompt、调用 AI。
  - 支持取消并传播 context cancellation。
- 验证：
  - 缺少模型配置返回明确错误。
  - 取消任务后状态为 CANCELLED。
  - 用户持仓输入不进入普通日志。
- 退出条件：
  - AI 分析主链路可从前端请求启动并取消。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/analysis` 纯规则模块。
  - 已实现 `symbol`、`analysisType`、`aiConfigId`、`promptTemplateId` 创建请求校验和股票代码标准化。
  - 已限制首版分析类型为 `stock_full`、`technical`。
  - 已实现普通日志输入快照，默认只记录 `has_user_position`，不记录一次性持仓明细。
  - 已实现取消规则：非终态任务可切换为 `CANCELLED` 并生成 `TASK_CANCELLED` 事件，终态任务不可重复取消。
  - 单测覆盖缺少模型配置、缺少模板配置、非法分析类型、持仓输入脱敏快照和取消状态流转。
  - 受 T16/T21/T25 依赖约束，真实任务创建、数据拉取、AI 调用、context cancellation 和 API/Rust command 接入完成后再标记为 `[x]`。

### T27 SSE 事件和 Rust 转发

- 状态：`[~]`
- 依赖：T25、T26
- 交付物：
  - `/api/tasks/events/stream`
  - `/api/tasks/events`
  - `analysis_task_subscribe`
  - Tauri event 或 channel 转发。
- 执行动作：
  - SSE 只允许 Rust 订阅 Go core。
  - 前端不直接使用 EventSource 连接 Go core。
  - `afterEventId` 支持断线补拉。
- 验证：
  - Rust 可接收 Go SSE 并转发给前端。
  - 断线后先补拉历史事件再恢复订阅。
  - 前端拿不到 Go core port/token。
- 退出条件：
  - 分析页可展示真实进度和流式输出。
- 当前进展：
  - 已在 `apps/sidecar-core/internal/task` 新增任务事件 SSE 帧编码基础能力。
  - 已复用 `ReplayEvents` 支持 afterEventID 之后的事件补拉和按事件 ID 递增排序。
  - SSE 帧包含 `id`、`event`、`data`，转发前统一复用事件 payload 脱敏。
  - 单测覆盖 SSE 帧结构、payload 脱敏和 afterEventID 补拉编码顺序。
  - 受 T25/T26 数据库持久化和真实任务链路约束，`/api/tasks/events/stream`、`/api/tasks/events`、Rust 订阅转发和前端真实进度展示接入后再标记为 `[x]`。

### T28 报告保存、查询、删除

- 状态：`[~]`
- 依赖：T26、T27
- 交付物：
  - `/api/reports/list`
  - `/api/reports/get`
  - `/api/reports/delete`
  - 对应 Rust command。
- 执行动作：
  - 报告按 `task_id` 幂等保存。
  - 删除使用软删除。
  - 默认复制/导出 Markdown 不包含完整 `input_snapshot`。
- 验证：
  - 同一 task_id 不重复生成报告。
  - 删除后列表不可见，详情返回 404。
  - 导出默认不包含 userPosition。
- 退出条件：
  - 报告历史页和分析完成页可以使用真实报告数据。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/report` 报告域规则模块。
  - 已实现同一 `task_id` 报告去重规则，保留 `updated_at` 最新报告，支撑后续按 `task_id` 幂等保存。
  - 已实现软删除过滤规则，列表和详情可复用同一可见报告口径。
  - 已实现 Markdown 导出规则，默认只导出报告元信息、AI 正文和风险摘要，不包含完整 `input_snapshot`。
  - 已支持显式选择时导出 `input_snapshot`，对应用户提示和 UI 入口待前端阶段接入。
  - 单测覆盖 `task_id` 去重、软删除过滤、默认导出不包含 userPosition 和显式导出输入快照。
  - 受 T09/T26/T27 依赖约束，数据库持久化、`/api/reports/*`、Rust command 和真实报告历史页接入后再标记为 `[x]`。

### T29 任务查询、事件回放和 RUNNING 恢复

- 状态：`[~]`
- 依赖：T25、T28
- 交付物：
  - `/api/tasks/list`
  - `/api/tasks/get`
  - `/api/tasks/events`
  - Go core 启动恢复逻辑。
- 执行动作：
  - 任务历史页优先读取任务详情和事件回放。
  - Go core 重启时扫描 RUNNING 任务。
  - 按任务类型标记 FAILED 或 CANCELLED，避免永久悬挂。
- 验证：
  - 人为制造 RUNNING 任务后重启 core，会被恢复为终态。
  - 事件回放按 afterEventId 返回增量。
- 退出条件：
  - sidecar 崩溃或重启不会留下永久 RUNNING 任务。
- 当前进展：
  - 已在 `apps/sidecar-core/internal/task` 补充任务历史列表排序规则，按 `updated_at` 倒序返回。
  - 已实现批量 `RUNNING` 任务恢复规则，复用单任务恢复逻辑，将悬挂任务恢复为终态并生成恢复事件。
  - 已复用现有 `ReplayEvents` 支持 `afterEventId` 增量事件回放。
  - 单测覆盖任务列表排序、批量 RUNNING 恢复和事件回放增量顺序。
  - 受 T09/T25/T28 依赖约束，数据库扫描、`/api/tasks/*`、Rust command 和真实任务历史页接入后再标记为 `[x]`。

---

## 10. P6：前端页面闭环与设置中心

### T30 前端应用框架和 invoke 服务层

- 状态：`[ ]`
- 依赖：T06、T07
- 交付物：
  - React + TypeScript + Vite 应用。
  - Ant Design + Tailwind CSS 基线。
  - Tauri invoke service。
- 执行动作：
  - 建立路由、布局、错误边界、加载态。
  - 所有 API 调用通过 typed invoke service。
  - 不在前端保存 API Key、token、Go core port。
- 验证：
  - 前端 health 页面或启动检查可调用 `core_health`。
  - TypeScript 类型检查通过。
- 退出条件：
  - 前端具备真实 command 调用基础。

### T31 Dashboard 页面

- 状态：`[ ]`
- 依赖：T19、T30
- 交付物：
  - 总览页。
- 执行动作：
  - 展示市场摘要、自选股涨跌分布、今日热点、最近报告、最近任务、风险提示。
  - 只使用首版 `dashboard_summary`。
- 验证：
  - 无数据、Provider 异常、正常数据三种状态可用。
  - 页面不出现策略、公告、研报、资金流入口。
- 退出条件：
  - 首页可以反映真实首版数据状态。

### T32 自选股页面

- 状态：`[ ]`
- 依赖：T14、T15、T16、T30
- 交付物：
  - 自选股列表、搜索、添加、删除、标签、备注、刷新、排序。
- 执行动作：
  - 搜索结果使用 `stock_search`。
  - 添加/删除使用 watchlist command。
  - 表格行情使用真实 quote 数据。
- 验证：
  - 添加后立即出现在列表。
  - 删除后列表消失且可重新添加。
  - 重复添加显示明确错误。
- 退出条件：
  - 用户可以完成自选股基础管理。

### T33 个股详情和 K线图页面

- 状态：`[ ]`
- 依赖：T16、T17、T18、T30
- 交付物：
  - 个股详情页。
  - K 线图和指标展示。
  - 新闻列表。
- 执行动作：
  - Lightweight Charts 展示 K 线。
  - ECharts 或表格展示指标摘要。
  - 新闻外链通过系统浏览器打开。
- 验证：
  - 不同 period/adjust 切换不破坏图表。
  - 数据不足时显示明确空状态。
  - 外链只允许 HTTPS。
- 退出条件：
  - 用户可查看股票行情、K 线、技术指标和相关新闻。

### T34 模型配置和 Prompt 模板页面

- 状态：`[ ]`
- 依赖：T21、T22、T23、T30
- 交付物：
  - 模型配置页。
  - Prompt 模板页。
- 执行动作：
  - API Key 输入只在保存时传给 Rust command。
  - 列表只展示 has_api_key、masked_api_key。
  - Prompt 模板只展示首版支持类型和变量。
- 验证：
  - 保存后刷新页面不出现真实 Key。
  - 测试连接失败不暴露密钥。
  - 未支持变量无法保存。
- 退出条件：
  - 用户可以配置模型和模板，并完成连通性测试。

### T35 AI 分析页面

- 状态：`[ ]`
- 依赖：T26、T27、T30、T34
- 交付物：
  - AI 分析页。
  - 流式输出展示。
  - 停止生成、保存报告、复制 Markdown、导出 Markdown。
- 执行动作：
  - 选择股票、分析类型、AI 模型、Prompt 模板。
  - 可选 userPosition 只作为本次上下文。
  - 订阅 Tauri 转发事件。
- 验证：
  - 成功任务可看到流式输出和最终报告。
  - 取消任务后 UI 进入取消状态。
  - 失败任务显示明确原因。
  - 导出默认不包含完整输入快照。
- 退出条件：
  - 个股 AI 分析闭环可真实使用。

### T36 报告历史和任务历史页面

- 状态：`[ ]`
- 依赖：T28、T29、T30
- 交付物：
  - 报告历史页。
  - 任务历史页。
- 执行动作：
  - 报告支持列表、详情、复制、删除。
  - 任务支持列表、详情、事件回放、失败原因查看。
  - 任务详情先补拉事件再订阅。
- 验证：
  - 删除报告后列表消失。
  - FAILED 任务展示 error_message。
  - 重启 core 后历史页不会显示永久 RUNNING。
- 退出条件：
  - 历史数据可追溯、可复查、可清理。

### T37 设置中心

- 状态：`[ ]`
- 依赖：T11、T20、T30
- 交付物：
  - 基础设置、数据源设置、代理设置、通知设置、工作区设置、缓存管理、开机自启、检查更新、关于应用。
- 执行动作：
  - 设置页读取真实 settings/cache/provider API。
  - 代理密码写入系统凭据管理器。
  - 授权信息只展示 FREE 状态占位，不提供激活入口。
- 验证：
  - 代理 URL 禁止包含 username/password。
  - 缓存清理不会删除报告和配置。
  - 关于页不展示未实现授权能力。
- 退出条件：
  - 设置页没有假按钮、假状态或半成品入口。

### T38 资讯中心页面

- 状态：`[ ]`
- 依赖：T18、T30
- 交付物：
  - 资讯中心。
- 执行动作：
  - 使用 `news_list` 和 `news_market`。
  - 支持市场新闻、个股新闻、行业标签筛选。
  - 不展示公告、研报、资金流专用入口。
- 验证：
  - 新闻列表、空状态、错误状态可用。
  - 外链打开前做 HTTPS scheme 校验。
- 退出条件：
  - 资讯中心只展示首版真实可用数据。

---

## 11. P7：跨平台桌面能力、打包、发布验收

### T39 托盘、窗口状态、通知、开机自启

- 状态：`[ ]`
- 依赖：T07、T37
- 交付物：
  - 托盘菜单。
  - 窗口状态恢复。
  - 系统通知。
  - 开机自启。
- 执行动作：
  - 关闭到托盘行为与设置项一致。
  - 任务成功/失败触发通知。
  - 开机自启按平台能力实现。
- 验证：
  - macOS 验收托盘、通知、开机自启。
  - Windows 验收托盘、通知、开机自启。
- 退出条件：
  - 首版桌面体验可用于内部测试。

### T40 日志导出和二次脱敏

- 状态：`[ ]`
- 依赖：T10、T37
- 交付物：
  - `/api/logs/export`
  - `export_logs`
  - 导出目录选择。
- 执行动作：
  - 导出前再次 redaction。
  - 只允许用户选择的导出目标。
  - 日志包含 request_id、trace_id、task_id，方便排障。
- 验证：
  - 构造含密钥日志后导出仍为脱敏值。
  - 未授权路径无法写入。
- 退出条件：
  - 用户可以导出可排障且不泄露敏感信息的日志。

### T41 检查更新入口

- 状态：`[ ]`
- 依赖：T37
- 交付物：
  - `check_update`
  - 更新配置 allowlist。
- 执行动作：
  - 只请求 HTTPS 更新 JSON。
  - 域名、下载链接、发布说明链接必须命中 allowlist。
  - 首版只提示版本，不下载、不安装、不静默升级。
- 验证：
  - HTTP URL 被拒绝。
  - 非 allowlist 域名被拒绝。
  - 有新版本时只展示外链或提示。
- 退出条件：
  - 检查更新符合首版信任边界。

### T42 sidecar 二进制命名和打包

- 状态：`[ ]`
- 依赖：T05、T39
- 交付物：
  - macOS Apple Silicon sidecar。
  - macOS Intel sidecar。
  - Windows x64 sidecar。
  - Tauri externalBin 配置。
- 执行动作：
  - sidecar 文件名符合 target triple。
  - 安装包不从运行时下载 sidecar。
  - desktop/core protocolVersion 启动时校验兼容性。
- 验证：
  - macOS aarch64 包可启动 core。
  - macOS x86_64 包可启动 core。
  - Windows x64 包可启动 core。
- 退出条件：
  - 打包产物包含正确平台 sidecar 并可启动。

### T43 发布文档、风险声明和用户手册

- 状态：`[ ]`
- 依赖：T31-T42
- 交付物：
  - README 开发和使用说明。
  - 用户手册。
  - 风险声明。
  - 发布说明。
- 执行动作：
  - 明确“仅为研究辅助，不构成投资建议”。
  - 说明数据来源、时效和限制。
  - 说明 API Key 和代理密码存储方式。
  - 不写未实现的授权、自动更新、交易能力。
- 验证：
  - 文档和 UI 能力一致。
  - 无真实密钥、Token、密码示例。
- 退出条件：
  - 内测用户可以理解安装、配置、使用和风险边界。

### T44 首版总体验收

- 状态：`[ ]`
- 依赖：T00-T43
- 交付物：
  - 首版验收报告。
  - macOS / Windows 验收记录。
  - 已知风险和遗留项。
- 执行动作：
  - 按技术方案第 19 章逐项验收。
  - 用真实桌面应用完成完整主链路。
  - 验证首版不出现非目标入口。
- 验收清单：
  - 应用可在 Windows 和 macOS 启动。
  - Go sidecar 可自动启动和退出。
  - 用户可以搜索并添加自选股。
  - 用户可以查看个股行情和 K 线。
  - 用户可以配置至少一个 AI 模型。
  - 用户可以基于股票生成 AI 分析报告。
  - 分析过程支持流式输出或进度展示。
  - 报告可以保存、查看、复制。
  - 任务失败后可以看到错误原因。
  - 设置中心支持工作区、代理、通知、缓存。
  - 应用有明确风险提示。
  - 数据库升级不会丢失用户已有数据。
  - API Key 存入系统凭据管理器，配置查询不回显真实 Key。
  - 前端不能直接访问 Go sidecar，Rust command 必须白名单化。
  - 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。
  - macOS 使用 Keychain，Windows 使用 Credential Manager，凭据保存/删除均通过验收。
  - Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。
  - 检查更新链接只允许 HTTPS allowlist 域名。
  - 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。
  - AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和持久化存储。
- 退出条件：
  - 验收报告写明通过项、未执行项、风险和复测命令。

---

## 12. 推荐并行批次

### Batch A：骨架和安全主链

- T01 初始化 monorepo 工程骨架
- T03 Go core 最小 HTTP server
- T04 sidecar stdin token 握手
- T05 Tauri 启动和管理 sidecar
- T06 Rust command 白名单代理基线

说明：T03/T04/T05/T06 是首版最高优先级主链，任何后续页面都不能绕过这条链。

### Batch B：数据和基础能力

- T08 SQLite migration 基线
- T09 sqlc 查询和 storage 事务层
- T12 股票代码模型和标准化
- T13 首版合规 Market Provider
- T10 统一日志、错误和脱敏

说明：先固定 DB 和错误/日志，再推进股票、行情、自选股。

### Batch C：行情闭环

- T14 股票搜索和基础信息 API
- T15 自选股 CRUD
- T16 行情、K线和缓存
- T17 技术指标计算
- T18 新闻资讯基础能力

说明：完成后前端可以开始 Dashboard、自选股、个股详情并行。

### Batch D：AI 和任务闭环

- T20 系统凭据管理适配
- T21 AI 配置 API 和 Rust 内部密钥注入
- T22 OpenAI-compatible Provider
- T23 Prompt 模板 CRUD 和变量白名单
- T24 Prompt 构建和合规输出约束

说明：T20/T21 是安全关键路径，禁止用前端或 SQLite 明文 Key 临时替代。

### Batch E：分析、报告、历史

- T25 任务状态机和事件持久化
- T26 分析任务创建和取消
- T27 SSE 事件和 Rust 转发
- T28 报告保存、查询、删除
- T29 任务查询、事件回放和 RUNNING 恢复

说明：任务、事件、报告必须一起验收，否则容易出现“能生成但不可追溯”的半成品。

### Batch F：前端页面

- T30 前端应用框架和 invoke 服务层
- T31 Dashboard 页面
- T32 自选股页面
- T33 个股详情和 K线图页面
- T34 模型配置和 Prompt 模板页面
- T35 AI 分析页面
- T36 报告历史和任务历史页面
- T37 设置中心
- T38 资讯中心页面

说明：页面必须绑定真实 command；接口未完成时允许做空状态，不允许做假数据假按钮。

### Batch G：发布和验收

- T39 托盘、窗口状态、通知、开机自启
- T40 日志导出和二次脱敏
- T41 检查更新入口
- T42 sidecar 二进制命名和打包
- T43 发布文档、风险声明和用户手册
- T44 首版总体验收

说明：这一批必须在真实 macOS 和 Windows 桌面应用里验收。

---

## 13. 子任务代理契约模板

下发并行子任务时，必须使用以下结构，避免职责重叠和写冲突：

```text
代理名称：<职责>_<类型>

任务定义：
- 背景：基于 Invest Compass 首版技术方案和实施 checklist。
- 目标：完成 <任务号> 的明确交付物。
- 输入：相关技术方案章节、上游任务产物、目标文件路径。

执行动作：
- 只修改列出的文件或模块。
- 不改动其他代理负责的文件。
- 不引入未确认依赖。
- 不降低 sidecar、Keychain、SSE、日志脱敏等安全边界。

预期结果：
- 列出修改文件。
- 列出已完成 checklist 项。
- 列出执行的验证命令和结果。
- 列出未完成项、风险和后续依赖。
```

---

## 14. 验证命令选择规则

项目骨架落地前：

```bash
git diff --check
```

项目骨架落地后，优先使用仓库实际定义的命令。初始建议如下，落地后以 `package.json`、`Makefile`、`Cargo.toml`、`go.mod` 为准：

```bash
cd apps && pnpm install
cd apps && pnpm build
cd apps && pnpm test
cd apps/sidecar-core && go test ./...
cargo check
cd apps/desktop && pnpm tauri build
```

跨平台和真实桌面能力必须补充手工验收：

```text
macOS：启动、Keychain、通知、托盘、sidecar、打包、签名/公证方案
Windows：启动、Credential Manager、通知、托盘、sidecar、NSIS/MSI、签名方案
```

---

## 15. Review Gate

### RG1：sidecar 安全启动门禁

- T03-T07 全部完成。
- token 不出现在 argv/env/日志/配置/数据库。
- 前端不能直接访问 Go core。
- Rust 不存在任意路径代理。
- 当前状态：已通过本地自动化验证，后续跨平台打包验收仍归入 RG6。

### RG2：基础数据闭环门禁

- T08-T19 全部完成。
- 股票搜索、自选、行情、K 线、指标、新闻都通过 Rust command 到 Go API。
- Dashboard 和 Provider status 不依赖假数据。

### RG3：AI 和凭据门禁

- T20-T24 全部完成。
- API Key 和代理密码只在系统凭据管理器和运行期内存中出现。
- 配置查询、日志、错误、任务事件、报告快照不泄露真实 Key。

### RG4：分析任务门禁

- T25-T29 全部完成。
- 任务事件可回放，SSE 只由 Rust 转发。
- sidecar 重启后 RUNNING 任务不会永久悬挂。
- 报告按 task_id 幂等保存。

### RG5：首版页面门禁

- T30-T38 全部完成。
- 页面只展示首版真实可执行功能。
- 不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。

### RG6：发布验收门禁

- T39-T44 全部完成。
- macOS 和 Windows 均完成真实安装/启动/主链路验收。
- README、用户手册、风险声明与实际能力一致。
