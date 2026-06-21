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
- SQLite 使用 `GORM`，所有业务表通过 migration 管理。
- 股票搜索、自选股、行情、K 线、技术指标、新闻资讯形成基础数据闭环。
- OpenAI-compatible AI Provider、模型配置、Prompt 模板、模型测试可用。
- API Key 和代理密码保存到 Rust 管理的本地文件 vault，SQLite 只保存引用标识和脱敏状态。
- 个股 AI 分析任务支持进度/流式事件、取消、失败原因、报告保存。
- 报告历史、任务历史、事件回放和 RUNNING 任务恢复规则可用。
- 设置中心覆盖工作区、代理、缓存、数据源状态、任务通知、开机自启、检查更新和日志导出入口；托盘、通知、开机自启和跨平台桌面行为仍归 T39/T44 验收。
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

## 3.1 专题实施清单

当前总 checklist 只承载首版主链路和 Review Gate。专题能力使用独立清单推进，避免把大量子任务平铺到主文档导致状态不可维护。

- 数据刷新调度专题：`docs/2026-06-19-invest-compass-scheduler-implementation-checklist.md`
  - 范围：`github.com/go-co-op/gocron/v2` 调度服务、`scheduler_jobs` / `scheduler_runs` / `ingestion_watermarks`、启动补偿、手动补偿、单股刷新和桌面管理接口。
  - 停止线：`analysis_report_schedule`、资金流刷新、独立市场快照专题和跨平台人工验收不随调度 MVP 自动完成。

- 任务结构化日志专题：`docs/2026-06-21-invest-compass-task-structured-logs-implementation-checklist.md`
  - 范围：`task_log_entries`、任务日志写入链路、`TaskLogDrawer` 真实数据接入、错误诊断、上下文摘要、脱敏日志导出和后续容量治理。
  - 停止线：远程日志平台、日志上传、公网指标服务、复杂统计图表、完整 AI 输出重复入库和用户隐私输入入库不随任务日志 MVP 自动完成。

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
- 当前进展：
  - 已新增桌面端 Node 测试护栏，自动扫描 Go 包 `doc.go`、Go 函数中文注释和 Rust 函数中文注释。
  - 该护栏包含缺少中文注释的样例断言，后续新增 Go/Rust 函数时如果未补中文注释会在 `pnpm --dir apps test` 中失败。

---

## 5. P1：项目骨架与 sidecar 安全启动

### T03 Go core 最小 HTTP server

- 状态：`[x]`
- 依赖：T01
- 交付物：
  - `apps/sidecar-core/cmd/invest-compass-core/main.go`
  - `apps/sidecar-core/internal/server`
  - `apps/sidecar-core/internal/actions`
  - `/internal/health`
- 执行动作：
  - `internal/server` 只负责本地 HTTP server 监听、初始化、启动和优雅关闭。
  - `internal/actions/router.go` 使用 Gin 集中注册本地 HTTP API。
  - `internal/actions/<module>` 负责各自业务 handler，并对外提供路由定义。
  - 只监听 `127.0.0.1`。
  - 所有 API 只接受 POST。
  - 返回统一响应结构和 `requestId` / `traceId`。
- 验证：
  - 非 POST 请求返回 405。
  - 未握手前业务路由不可用。
  - health 在 token 有效后返回版本和 DB 状态。
- 退出条件：
  - Go core 可以本地启动，并具备最小健康检查。
- 当前进展：
  - `internal/server` 已收缩为本地监听、HTTP server 创建、启动和优雅关闭，不承载业务路由。
  - `server.Listen` 拒绝非 `127.0.0.1` 监听地址，避免 sidecar 暴露到局域网或公网。
  - `server.NewHTTPServer` 已配置读 header、读请求、写响应和 idle 超时，避免异常连接长期占用本地 sidecar。
  - `server.Serve` 会把优雅关闭超时等 `Shutdown` 错误返回给调用方，避免生命周期层误判 core 已干净退出。
  - 单测覆盖非本机监听拒绝、handler 注入边界、HTTP server 超时配置和关闭超时错误传播。

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
  - 合法握手后输出 `{"status":"ready","port":...,"protocolVersion":"1"}`。
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
  - 已实现 Rust sidecar token、stdin 握手、ready JSON 解析、protocolVersion 兼容性校验、health/shutdown client 和 `core_health` 白名单 command。
  - 已补 Go `/internal/shutdown`，合法 token 才能触发关闭回调。
  - 已补 `pnpm sidecar:build`，默认产物为 `apps/desktop/src-tauri/binaries/invest-compas-core`。
  - 已验证项目内 sidecar 二进制 stdin 握手、health、非 POST 拒绝和 shutdown 后进程退出。
  - Go server 优雅关闭失败会返回错误，Rust 侧仍保留 `/internal/shutdown` 失败后 kill 子进程的兜底清理。

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
  - 已新增桌面端安全测试，自动校验所有 `#[tauri::command]` 都显式注册到 `invoke_handler`，并禁止 `core_request(method, path, body)` 这类任意代理。

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

- 状态：`[x]`
- 依赖：T03
- 交付物：
  - `apps/sidecar-core/migrations/`
  - `apps/sidecar-core/internal/dao`
  - `apps/sidecar-core/internal/model`
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
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/dao.Migrate`，通过 GORM `AutoMigrate` 创建首版 SQLite schema。
  - 已把首版 GORM schema 模型放入 `apps/sidecar-core/internal/model`，供 service 和 dao 共享。
  - 已建立 `stocks`、`watchlists`、`quotes`、`klines`、`news_items`、`ai_configs`、`prompt_templates`、`analysis_reports`、`tasks`、`task_events`、`settings`。
  - 所有业务表已包含 `created_at`、`updated_at`。
  - `watchlists`、`news_items`、`ai_configs`、`prompt_templates`、`analysis_reports` 已具备软删除列。
  - 已验证空库迁移成功、重复迁移幂等、active watchlist symbol 唯一约束和 K 线复合唯一约束。
  - 已新增 `dao.BackupBeforeMigration`，sidecar 生产启动会在 `dao.Open` 和 `dao.Migrate` 前备份已有 SQLite 文件到工作区 `backups/` 目录。
  - 迁移前备份会跳过首次启动缺失库和内存库，复制已有主库文件并拒绝覆盖同名备份，避免破坏用户可恢复点。
  - 单测覆盖已有库备份、缺失库/内存库跳过、同名备份拒绝覆盖、备份文件恢复后重新打开并迁移仍保留用户 settings 数据，以及生产启动路径在迁移前触发备份。
  - 已保留 `apps/sidecar-core/migrations/` 目录，用于后续发布后版本化迁移扩展。

### T09 GORM dao 和事务层

- 状态：`[x]`
- 依赖：T08
- 交付物：
  - `internal/dao` GORM 入口。
  - `internal/model` 共享模型。
  - `internal/service` 业务编排入口。
  - `pkg/constant` 跨包非错误类常量。
  - `pkg/xerr` 跨包错误码和通用错误类型。
  - 关键 CRUD repository。
  - 事务封装。
- 执行动作：
  - 为自选股、AI 配置、Prompt、任务、事件、报告实现 GORM 数据访问。
  - 重要写操作使用事务。
  - 不写手拼 SQL 到业务 handler。
- 验证：
  - `go test ./...`
  - Go 单测覆盖 CRUD、唯一约束、软删除。
- 退出条件：
  - 数据访问层类型安全且可测试。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/dao.Store` 作为统一 GORM repository 入口。
  - 已实现 `WithTransaction` 事务封装，业务错误会回滚事务内写入。
  - 已实现自选股、AI 配置、Prompt 模板、任务、任务事件、分析报告和 settings 的基础 GORM 数据访问。
  - 已实现 quote/kline 行情缓存 repository，quote 按 symbol 保存最新快照，K 线按 `symbol + period + adjust + trade_date` 幂等写入。
  - 自选股、AI 配置、Prompt 模板和分析报告支持软删除可见性过滤；报告按 `task_id` 幂等保存。
  - 任务事件支持按 `task_id + after_id` 回放，RUNNING 任务恢复可基于状态查询接入。
  - DAO 单测覆盖事务回滚、CRUD、软删除、唯一约束依赖、事件回放、报告 upsert 和 settings upsert/get。
  - 已新增 Go core 生产源码扫描，禁止非 `internal/dao` 层直接使用 `database/sql` 或手写 SQL 语句，防止业务 handler 绕过 GORM dao 数据访问边界。
  - 已新增 Gin/GORM 组件护栏，后续 Go HTTP 路由必须在 `internal/actions/router.go` 走 Gin，数据库访问入口必须走 `internal/dao` 的 GORM。
  - 已新增 server/actions 架构护栏，`internal/server` 不得注册业务路由或承载 handler，action 子包不得直接依赖 Gin 路由注册 API。
  - 已新增 `internal/service`、`internal/model`、`internal/dao`、`pkg/constant`、`pkg/xerr` 分层目录护栏。
  - 已将现有业务模块迁移为 `internal/service/<module>` 子包，例如 `stock` 已迁入 `internal/service/stock`。
  - 已将公共日志和脱敏辅助能力迁入 `apps/sidecar-core/pkg/logger`。
  - 已将 service 层通用错误码和通用错误结构迁入 `apps/sidecar-core/pkg/xerr`，并新增护栏禁止 service 重新定义。

### T10 统一日志、错误和脱敏

- 状态：`[x]`
- 依赖：T03
- 交付物：
  - `apps/sidecar-core/pkg/logger`
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
  - 已新增 `apps/sidecar-core/pkg/logger`，提供稳定日志字段常量和统一脱敏入口。
  - 脱敏覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户一次性持仓输入。
  - HTTP handler 已接入 panic recovery，panic 会返回统一错误 envelope，并在写入日志前脱敏。
  - HTTP 响应层会先完成 JSON 编码再写状态码；遇到不可编码响应时返回 `50000/internal_error`，保留 `requestId` / `traceId`，避免单个 handler 响应异常触发 panic。
  - Go core 会规范化请求头中的 `X-Request-Id` / `X-Trace-Id`，只接受 1-128 位 ASCII 字母、数字、`-`、`_`、`.`，非法或超长值会替换为本地生成 ID，避免异常追踪头进入响应和日志。
  - Go core 业务 API 已统一通过 `actions/httpx.DecodeJSON` 解码 JSON 请求体，单个请求体上限为 1 MiB，且只允许包含一个 JSON object 文档；超限返回 `41300/request_body_too_large`，`null`、尾随内容或未知字段返回 `40001/invalid_json`，避免异常本地请求占用过多内存、拼写错误、零值请求或未知字段被业务层忽略。
  - 单测覆盖字段契约、错误脱敏、密钥脱敏、nil error、panic recovery、响应编码失败降级、非法/超长追踪头规范化、JSON 请求体大小限制、尾随内容拒绝、未知字段拒绝，以及 `null` 顶层请求拒绝。

### T11 settings / workspace / cache API 基线

- 状态：`[x]`
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
- 当前进展：
  - 已扩展 `apps/sidecar-core/internal/service/settings` 设置规则模块。
  - 已实现 settings 明文敏感配置拦截，禁止 `api_key`、密码、token、secret、Authorization 等敏感值作为普通配置保存。
  - 已允许 `api_key_ref`、`proxy_credential_ref`、`masked_api_key`、`has_api_key` 等凭据引用和脱敏状态落库，且 Go core 会拒绝未知 `_ref`、非本地 vault scheme 的凭据引用或未脱敏的 `masked_api_key`。
  - 已实现工作区路径绝对路径校验，非法路径返回稳定错误码。
  - 已实现缓存统计只汇总行情、K 线、新闻、图表图片等临时缓存，明确排除报告和配置。
  - 单测覆盖敏感配置拦截、凭据引用放行、非法工作区路径和临时缓存统计。
  - 已在 Go core `apps/sidecar-core/internal/actions/cache` 接入 `POST /api/cache/stats` 和 `POST /api/cache/clean`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - Cache stats 通过 `CacheStatsProvider` 从真实存储层实时统计临时缓存，JSON 字段使用稳定 snake_case，明确排除报告和配置。
  - Cache clean 只把 `settings.FilterCacheCleanupTargets` 过滤后的临时目标交给 `CacheCleaner`，不会把报告或配置传入清理边界。
  - 单测覆盖 cache stats 排除受保护目标、cache clean 只清理临时目标。
  - 已在 Go core `apps/sidecar-core/internal/actions/settings` 接入 `POST /api/settings/get`、`POST /api/settings/set`、`POST /api/workspace/get`、`POST /api/workspace/set`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 已将 Go sidecar 启动参数扩展为 `--workspace=<app_data_dir>`，并在 ready 前初始化 SQLite、执行 GORM migration、注入真实 `dao.Store`。
  - 已在 Rust sidecar 启动链路中从 Tauri app data 目录传入 workspace，runtime token 仍只通过 stdin 握手传递，不进入 argv/env/config。
  - 已新增 `dao.Store.CleanCache`，真实清理 quote、kline、news 临时缓存表，并忽略 report/config 等受保护目标。
  - 已新增 `dao.Store.CacheUsages`，从 quote、kline、news 真实缓存表读取统计，报告和配置不参与缓存统计。
  - Rust `workspace_set` 已在转发前拒绝空路径和相对路径，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`settings_get`、`settings_set`、`workspace_get`、`workspace_set`、`cache_stats`、`cache_clean`，每个 command 固定映射到对应 Go API，禁止通用 path 代理。
  - Rust `settings_set` 会在读写本地 vault 前拒绝同一请求同时写入新代理密码和清理旧代理凭据，避免本地副作用语义冲突。
  - 单测覆盖 settings 持久化读写、敏感 key 拦截、本地 vault 凭据引用白名单、非法工作区路径、workspace 参数生成、sidecar 数据库路径、cache stats 真实 provider、cache cleaner 保护边界、Rust command 固定 path、Rust command 非法 workspace 路径早失败和代理凭据冲突动作早失败。
  - 前端设置中心真实页面、通知和代理密码凭据链路属于 T20/T30/T37 后续任务，不阻塞本 API 基线退出。

---

## 7. P3：股票、行情、K线、指标、新闻基础能力

### T12 股票代码模型和标准化

- 状态：`[x]`
- 依赖：T09
- 交付物：
  - `apps/sidecar-core/internal/service/stock`
  - Symbol parser / validator。
- 执行动作：
  - 支持 `CN:SH:600519`、`CN:SZ:300750`、`HK:00700`、`US:AAPL` 等格式。
  - 错误响应使用稳定错误码。
- 验证：
  - table-driven tests 覆盖合法、非法、边界 symbol。
- 退出条件：
  - 股票代码成为所有后续 API 的统一输入类型。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/stock` 纯模型包，完成 `CN:SH:600519`、`CN:SZ:300750`、`HK:00700`、`US:AAPL` 解析和大小写标准化。
  - 已定义稳定错误码：空输入、格式错误、不支持市场、不支持交易所、代码非法。
  - table-driven tests 已覆盖合法、非法和边界输入。
  - 股票搜索、DAO 缓存写入和 Rust `stock_search(keyword)` 已统一使用标准 symbol，股票代码模型已成为后续 API 的统一输入类型。

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
  - 已新增 `apps/sidecar-core/internal/service/market` 契约包，定义 `MarketProvider`、Provider 状态、股票基础信息、行情快照和 K 线模型。
  - Provider 状态模型强制携带来源、授权边界、频率限制和支持市场描述。
  - 已新增未配置 Provider 的安全状态模型，真实数据源未配置时只返回 `available=false` 和 `source=unconfigured`，不伪造行情、搜索或 K 线能力。
  - Provider 错误会保留 provider/operation 可观测上下文，并复用统一脱敏入口避免泄露授权头和 API Key。
  - 单测已覆盖接口契约、标准 symbol 使用、合规状态描述、未配置状态和错误脱敏。
  - 已新增 `sina-tencent-market` 真实组合 Provider：`SinaSource/SinaProvider` 只负责新浪 suggest/实时行情，`TencentSource/TencentProvider` 只负责腾讯结构化 K 线，`EastMoneySource/EastMoneyProvider` 只负责东方财富 `push2his` K 线兜底，`CompositeMarketProvider` 对外组合成完整 `MarketProvider`；授权和样例验收完成前，生产 `main.go` 仍保持 `UnconfiguredProvider` 安全状态。
  - K 线 Provider 链路为腾讯优先、东财 direct HTTP 兜底；不迁移 chromedp Cookie 抓取逻辑，也不依赖本地浏览器路径。
  - Provider 单测覆盖新浪搜索解析、GB18030 解码、A 股实时行情字段映射、非股票六位代码拒绝、腾讯 K 线解析、不复权参数映射、东财 push2his K 线解析、拆分后的新浪/腾讯/东财职责边界、组合 Provider 委派和东财兜底、异常字段快速失败、状态元信息和未支持市场拒绝。
  - 已新增 `scripts/provider-smoke.mjs`，默认 dry-run 不访问网络；只有执行方显式传入 `--allow-network --confirm-provider-terms` 后，才对新浪 suggest/quote、腾讯 K 线和东财 K 线样例端点做 live smoke 检查；live smoke 会校验端点特征响应体，避免把错误页或空壳 200 当成 Provider 样例通过。
  - 当前 Provider 仅声明支持 `CN` A 股；`HK` / `US`、数据源授权复核、真实外网样例查询和跨平台开发环境验收完成前，T13 仍保持 `[~]`，不能标记为 `[x]`。

### T14 股票搜索和基础信息 API

- 状态：`[~]`
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
- 当前进展：
  - 已在 Go core `apps/sidecar-core/internal/actions/stocks` 接入 `POST /api/stocks/search`。
  - 搜索 API 复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 已实现 keyword 空值校验，空 keyword 返回 400 和稳定错误消息 `invalid_keyword`。
  - 已通过 `MarketProvider.Search` 返回标准字段：`symbol`、`name`、`code`、`market`、`exchange`。
  - Provider 不可用时返回 `market_provider_unavailable`，Provider 调用失败时返回 `market_provider_error` 且日志错误脱敏。
  - 已新增 `dao.Store.UpsertStocks`，搜索成功后按标准 `symbol` 幂等写入或更新 `stocks` 缓存。
  - Rust `stock_search` 已在转发前拒绝空 keyword，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command `stock_search(keyword)`，固定映射到 `POST /api/stocks/search`，禁止通用 path 代理。
  - 单测覆盖空 keyword、有效 keyword 返回标准 symbol、搜索后写入股票缓存、DAO 按 symbol upsert、Rust command 固定 path 和 Rust command 空 keyword 早失败。
  - 受 T13 合规验收和生产注入约束，数据源授权、真实外网样例查询、跨平台开发环境验收完成后再标记为 `[x]`。

### T15 自选股 CRUD

- 状态：`[x]`
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
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/watchlist` 自选股规则模块。
  - 已复用 `stock.ParseSymbol` 作为 symbol 标准化唯一来源，避免自选股另行维护股票代码解析规则。
  - 已实现创建、更新元数据、软删除和 active 列表过滤排序规则。
  - 已实现 active symbol 唯一校验，软删除后同一 symbol 可重新添加。
  - 单测覆盖重复添加、软删除后重新添加、列表过滤 deleted 数据和标签/备注/排序更新。
  - 已在 Go core `apps/sidecar-core/internal/actions/watchlist` 接入 `POST /api/watchlist/list`、`POST /api/watchlist/create`、`POST /api/watchlist/update`、`POST /api/watchlist/delete`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 已接入真实 `dao.Store`，创建和更新支持标签、备注、排序，删除走 soft delete，列表只返回 active 数据。
  - Rust `watchlist_update` 和 `watchlist_delete` 已在转发前校验 `id > 0`，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`watchlist_list`、`watchlist_create`、`watchlist_update`、`watchlist_delete`，每个 command 固定映射到对应 Go API，禁止通用 path 代理。
  - API 单测覆盖创建、更新、列表、删除隐藏和重复 active symbol 稳定错误；Rust 安全测试覆盖 command 注册、固定 path 和非正数 ID 早失败。
  - 自选股页面真实交互已由 T32 接入并覆盖搜索、添加、更新、删除和行内刷新，不再阻塞本 CRUD/API 基线退出。

### T16 行情、K线和缓存

- 状态：`[~]`
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
- 当前进展：
  - 已在 `apps/sidecar-core/internal/service/market` 新增行情短缓存和 K 线缓存规则。
  - 已限制 quote 短缓存 TTL 必须处于 10-60 秒范围内。
  - 已实现 quote 按 symbol 命中和过期 miss 规则。
  - 已实现 K 线按 `symbol + period + adjust` 隔离缓存，并按 `trade_date` 升序返回。
  - 单测覆盖 quote TTL 校验、TTL 命中/过期、K 线排序、adjust 隔离和缓存副本隔离。
  - 已新增 `apps/sidecar-core/internal/actions/market`，接入 `POST /api/market/quote` 和 `POST /api/market/kline`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - `quote` API 会优先读取 30 秒短缓存，缓存 miss 后调用 `MarketProvider.Quote`，成功后写入 `quotes` 缓存。
  - `kline` API 支持 `period=day|week|month`、`adjust=none|qfq|hfq` 和 `limit` 校验，缓存足量时不重复请求 Provider，Provider 返回后按交易日升序响应并写入 `klines` 缓存。
  - 已新增 `dao.Store.SaveQuote`、`LatestQuote`、`SaveKlines`、`ListKlines`，行情缓存读写全部收敛在 GORM dao 层。
  - Rust `market_kline` 已在转发前校验 `limit` 必须为 1-500，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`market_quote`、`market_kline`，固定映射到 `POST /api/market/quote` 和 `POST /api/market/kline`，禁止通用 path 代理。
  - 单测覆盖 quote 返回价格/涨跌幅/时间/provider、quote 缓存命中不重复请求 Provider、K 线有序返回和缓存写入、DAO quote/kline 幂等更新、Rust command 固定 path 和 Rust command 非法 `limit` 早失败。
  - 受 T13/T30 后续接入约束，真实合规 Provider 注入和股票详情页真实展示完成后再标记为 `[x]`。

### T17 技术指标计算

- 状态：`[x]`
- 依赖：T16
- 交付物：
  - `apps/sidecar-core/internal/service/indicator`
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
  - 已新增 `apps/sidecar-core/internal/service/indicator` 纯计算模块。
  - 已实现 MA、EMA、MACD、RSI、KDJ、BOLL、成交量均线、涨跌幅、区间最大回撤、区间波动率。
  - 固定输入输出单测覆盖全部指标，并覆盖数据不足、非法周期、非法输入的稳定错误码。
  - 已在 `apps/sidecar-core/internal/actions/market` 接入 `POST /api/market/indicators`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 技术指标 API 复用 `klines` 缓存，缓存不足时通过 `MarketProvider.Kline` 拉取并写回缓存，不新增独立指标缓存表。
  - 指标 API 支持 `ma`、`ema`、`macd`、`rsi`、`kdj`、`boll`、`volume_ma`、`change_percent`、`max_drawdown`、`volatility` 白名单。
  - 指标序列中的前置空结果以 JSON `null` 返回，避免前端自行推导 NaN 语义。
  - Rust `market_indicators` 已在转发前校验 `limit` 必须为 1-500，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command `market_indicators`，固定映射到 `POST /api/market/indicators`，禁止通用 path 代理。
  - 单测覆盖从缓存 K 线计算 MA/成交量均线/涨跌幅/最大回撤、缓存足量时不重复请求 Provider、缓存不足时拉取 Provider 并写回缓存、Rust command 固定 path 和 Rust command 非法 `limit` 早失败。
  - 个股详情页展示技术指标属于 T33 后续页面任务，不阻塞本指标计算/API 基线退出。

### T18 新闻资讯基础能力

- 状态：`[~]`
- 依赖：T12、T10
- 交付物：
  - `apps/sidecar-core/internal/service/news`
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
  - 已新增 `apps/sidecar-core/internal/service/news` 纯模块，定义新闻条目、个股新闻请求、市场新闻请求和 Provider 契约。
  - 已实现 HTTP(S) URL scheme 白名单校验，禁止 `javascript:`、`file:` 等危险链接进入输出。
  - 已实现 `content_hash` 去重，忽略 URL 和来源以合并多来源转载，并保留首次出现条目。
  - 已实现新闻缓存 TTL 校验，限制在 30-120 分钟范围内。
  - 已实现个股新闻和市场新闻缓存规则，列表按发布时间倒序返回，个股新闻按 symbol 过滤。
  - 已新增未配置新闻 Provider 的安全状态模型，真实新闻源未配置时只返回 `available=false` 和 `source=unconfigured`，不伪造新闻列表或市场新闻。
  - 已新增新闻 Provider 可选 `Status(ctx)` 状态能力，Dashboard 和 Provider 状态接口会优先复用真实新闻源状态，不能因 Provider 非空就假定可用。
  - Provider 错误复用统一脱敏入口，避免授权头和 API Key 泄露。
  - 单测覆盖 URL 校验、标准 symbol 绑定、去重、个股/市场新闻契约、未配置状态、可选状态能力、缓存 TTL、缓存排序、缓存过期、缓存副本隔离和错误脱敏。
  - 已新增 `dao.Store.SaveNewsItems`、`ListNewsBySymbol`、`ListMarketNews`，新闻缓存按 `content_hash` 幂等入库，列表按发布时间倒序返回。
  - 已在 `apps/sidecar-core/internal/actions/news` 接入 `POST /api/news/list` 和 `POST /api/news/market`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 新闻 API 会优先读取 60 分钟缓存；缓存不足时调用 `news.Provider`，输出前执行 URL scheme 校验、content hash 去重和排序，再写入 `news_items`。
  - Rust `news_list` 和 `news_market` 已在转发前校验 `limit` 必须为 1-100，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`news_list`、`news_market`，固定映射到 `POST /api/news/list` 和 `POST /api/news/market`，禁止通用 path 代理。
  - 单测覆盖重复新闻不重复入库、列表按发布时间排序、新闻 URL 输出前 scheme 校验、个股新闻 API 写入缓存、市场新闻缓存命中不重复请求 Provider、Rust command 固定 path 和 Rust command 非法 `limit` 早失败。
  - 已新增 `CailianpressProvider` 和 `SinaLiveProvider` 的 service 层抓取清洗实现，分别清洗财联社电报和新浪财经直播快讯为统一 `news.Item`；尚未注入生产入口。
  - `scripts/provider-smoke.mjs` 已纳入新浪财经直播快讯样例端点；默认 dry-run 不访问网络，联网检查需要显式确认 Provider 条款、授权和频率限制边界。
  - 受真实合规新闻 Provider 数据源授权约束，Provider 注入、Dashboard/资讯中心/分析上下文真实新闻数据闭环完成后再标记为 `[x]`。

### P3 补充：市场资讯扩展 Provider 预研

- 状态：service 层已新增，尚未接入 API、Rust command、SQLite 缓存、前端页面或分析任务上下文。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/marketinfo`，通过腾讯财经公开接口抓取并清洗全球主要指数，支持 Markdown 渲染给后续 AI 上下文使用。
  - 已在 `marketinfo` 新增 `CLSMarketStatisticProvider`，通过财联社 `x-quote` 行情概览接口抓取并清洗全市场上涨/下跌家数、涨停/跌停家数、指数内部涨跌家数、涨跌分布和派生情绪描述。
  - 已在 `marketinfo` 新增 `EastMoneyMutualTop10Provider` 和 `EastMoneyStockScreenerProvider`，分别清洗东财沪深港通/港股通十大成交和东财条件选股列表。
  - 已在 `fundflow` 新增 `EastMoneyStockFundFlowProvider`，清洗东财个股资金流榜单和个股历史资金流序列；不写入本地库。
  - 已新增 `apps/sidecar-core/internal/service/macro`，通过东方财富宏观数据接口抓取并清洗 GDP、CPI、PPI、PMI。
  - 已新增 `apps/sidecar-core/internal/service/calendar`，支持财联社日历公开接口和九阳公社日历凭据注入接口。
  - 已新增 `apps/sidecar-core/internal/service/hotspot`，支持雪球热股凭据注入接口。
  - 需要 Cookie、token 或 API Key 的渠道通过 `ChannelCredential` 注入；缺少凭据时返回 Provider 错误，不使用硬编码 Cookie/token，也不使用 chromedp 获取登录态。
  - 单测覆盖财联社快讯、Sina 直播快讯、腾讯全球指数、财联社市场涨跌统计、东财互联互通十大成交、东财条件选股、东财个股资金流榜单、东财个股历史资金流、东财 GDP、财联社日历、九阳日历凭据校验和雪球热股 Cookie 校验。
- 剩余边界：
  - 当前只迁移抓取和清洗，不迁移原项目中的 DB 写入、定时任务、标签关联、Markdown 业务拼装或页面入口；市场涨跌统计不迁移 `market_statistic` 入库、今日/近 N 日趋势查询；`stock_data_api.go` 中的自选股、交易日志、Wails 方法和 AI Tool 包装不迁移。
  - 研报、公告、龙虎榜、资金流等与 `fundamental`、`fundflow`、`eastmoneyai` 已有职责重叠的能力，不在该预研包重复实现。
  - 各渠道的数据授权、频率限制、Cookie/token 管理、可分发边界和真实外网样例仍需单独验收。

### P3 补充：基本面 / F10 Provider 抽象

- 状态：service 层已新增，尚未接入 API、Rust command、SQLite 缓存、前端页面或分析任务上下文。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/fundamental`，定义基本面 `Provider`、`ProviderStatus`、`ReportKind`、`Dataset`、`Column`、`Row` 和脱敏 `ProviderError`。
  - 已新增 `EastMoneyF10Provider`，通过东方财富 HSF10 direct HTTP JSON 接口获取最新财务、季度财务、机构预测、估值百分位、融资融券、大宗交易、户均持股趋势、龙虎榜和营业部买卖明细等报告。
  - 已新增 `RenderMarkdown`，把结构化 `Dataset` 渲染成 AI 上下文可用的 Markdown，隐藏技术字段并格式化金额、股数、百分比、日期。
  - 单测覆盖 Provider 状态合规元信息、东财 F10 请求参数和解析、Markdown 字段隐藏/格式化、未支持市场拒绝。
- 剩余边界：
  - 未接入首版 API、Rust command、缓存表、Prompt 上下文或 UI。
  - 东方财富 F10 公开网页接口的数据授权、频率限制、可分发边界和真实外网样例仍需单独验收。

### P3 补充：基金 Provider 预研

- 状态：service 层已新增，尚未接入 API、Rust command、SQLite 缓存、前端页面或分析任务上下文。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/fund`，定义基金 `Provider`、`ProviderStatus`、搜索、基础资料、历史净值、排行、十大持仓模型和脱敏 `ProviderError`。
  - 已新增 `EastMoneyProvider`，通过东方财富公开网页接口抓取并清洗基金搜索、基金基础资料、历史净值、基金排行和十大持仓。
  - 迁移范围只包含抓取和清洗，不迁移原项目中的关注基金 CRUD、数据库写入、批量刷新、场内基金 K 线二次封装、持仓股票补行情或页面入口。
  - 单测覆盖搜索响应清洗、基础资料页面解析、历史净值数值转换、基金排行伪 JS 解析、十大持仓 HTML 解析和非法基金代码拒绝。
- 剩余边界：
  - 基金能力不属于当前首版用户可见范围，未接入生产默认入口。
  - 东方财富基金公开网页接口的数据授权、频率限制、可分发边界和真实外网样例仍需单独验收。

### T19 Dashboard summary 和 provider status

- 状态：`[~]`
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
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/dashboard` Dashboard 首版聚合规则模块。
  - 已实现自选股涨跌平分布、最近报告、最近任务、市场新闻、风险提示和 provider status 安全展示模型。
  - 已复用 `report`、`task`、`logger` 既有规则，报告过滤软删除并按 `task_id` 去重，任务按 `updated_at` 倒序，Provider 最近错误统一脱敏。
  - 聚合结果不包含策略、公告、研报、资金流等首版不做字段。
  - 单测覆盖自选涨跌分布、最近报告/任务/新闻排序截断、Provider 错误脱敏和不返回非 MVP 字段。
  - 已在 Go core `apps/sidecar-core/internal/actions/providers` 接入 `POST /api/providers/status`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - Provider status API 复用 Dashboard 安全展示模型，返回 `name`、`source`、`available`、`last_error`，并对最近错误做脱敏；未配置真实 Market/News Provider 时分别返回明确不可用状态而不是 503；新闻 Provider 实现可选 `Status(ctx)` 时会返回 Provider 自身状态。
  - 单测覆盖 provider status API 的安全响应、敏感错误脱敏、未配置 Market/News Provider 状态和新闻 Provider 不可用状态。
  - 已在 Go core `apps/sidecar-core/internal/actions/dashboard` 接入 `POST /api/dashboard/summary`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - Dashboard summary API 已支持生产环境从 `dao.Store` 读取真实 active 自选股、最新行情、最近报告、最近任务和市场新闻缓存，并通过 Market/News Provider 状态或未配置状态聚合数据源状态。
  - 没有注入真实 store 时，测试仍可使用 `Config.DashboardInput` 固定输入；生产 `main.go` 已注入真实 `store`，避免默认返回静态空聚合。
  - 单测覆盖 dashboard summary API 的自选涨跌分布、敏感错误脱敏、不返回非 MVP 字段、未配置 Market/News Provider 状态，以及从真实 SQLite DAO 汇总自选股行情、报告、任务、市场新闻和 Provider 状态。
  - 已新增 Rust 白名单 command：`dashboard_summary`、`providers_status`，固定映射到 `POST /api/dashboard/summary` 和 `POST /api/providers/status`，禁止通用 path 代理。
  - Rust 安全测试覆盖 Dashboard 和 Provider 状态 command 注册和固定 path。
  - Dashboard 前端页面已由 T31 接入并覆盖正常、空数据和 Provider 异常状态；受真实 Market/News Provider 数据源授权约束，完成后再标记为 `[x]`。

---

## 8. P4：AI 配置、凭据、Provider、Prompt 模板

### T20 本地凭据 vault 适配

- 状态：`[x]`
- 依赖：T07
- 交付物：
  - Rust 本地文件 vault 适配。
  - Rust 凭据服务接口。
- 执行动作：
  - 保存、读取、删除 AI Key。
  - 保存、读取、删除代理密码。
  - SQLite 只保存 `api_key_ref`、`proxy_credential_ref`。
- 验证：
  - 本地 vault 保存/读取/删除验收通过。
  - 前端和 Go core 不持久化真实 Key。
- 退出条件：
  - 敏感凭据只存在 Rust 本地 vault 文件和运行期内存。
- 当前进展：
  - 已移除首版对平台凭据服务的强依赖，改为不新增依赖的 Rust 本地文件 vault。
  - 已在 `ai_config_save` 中接收一次性 `api_key`，写入本地 vault 后只向 Go core 转发 `api_key_ref`、`masked_api_key` 和 `has_api_key`。
  - 新建 AI 配置尚无数据库 ID 时，Rust 本地 vault 会生成唯一 `api_key_ref`，避免多个 `id=0` 新配置覆盖同一个密钥文件。
  - 更新 AI Key 时会先校验旧 `api_key_ref` 为合法本地 vault 引用，再写入新密钥并删除被替换的旧文件；删除旧引用失败时会回滚本次新写入文件，避免历史密钥残留或孤儿 secret 文件。
  - 本地 vault 目录和 secret 文件写入已统一收口；Unix/macOS 下目录强制收紧为 `0700`，secret 文件创建和更新后都会强制收紧为 `0600`。
  - 本地 vault 会拒绝清理后为空的 AI Provider、`api_key_ref`、`proxy_credential_ref` 和代理 profile，避免非法引用退化为弱语义文件名或落到隐式 `.secret` 文件。
  - Go core 保存 AI 配置和 settings 前会校验凭据引用 scheme，只接受 `local-vault://ai-config/` 和 `local-vault://proxy/`，避免绕过 Rust 边界写入任意 `_ref`。
  - Go core 会拒绝非空且没有脱敏标记的 `masked_api_key`，避免明文 Key 通过展示字段旁路落库。
  - 已在 `ai_config_delete` 中支持携带 `api_key_ref` 时同步删除本地 vault 文件。
  - `ai_config_delete` 会先校验配置 ID 为正数，再删除本地 vault 文件，避免非法删除请求误删本地凭据。
  - `apps/desktop/src-tauri/src/commands/ai_config.rs` 单测覆盖本地 vault 保存、读取、删除、模型测试读取、新建配置唯一 vault 引用、更新 key 清理旧引用、旧引用非法时不写入新密钥、空 AI Provider 和空/非法清理后的 vault 引用拒绝、Unix/macOS vault 目录 `0700` 和 secret 文件 `0600` 权限、非法删除请求不误删凭据，以及转发保存 payload 不包含明文 `api_key`。
  - Rust `settings_set` 已支持一次性 `proxy_password`，写入本地 vault 后只向 Go core 转发 `proxy_credential_ref`。
  - Rust `settings_set` 已支持 `clear_proxy_credential`，会删除本地 vault 文件并向 Go core 清空 `proxy_credential_ref`。
  - Rust `settings_set` 会在读写本地 vault 前拒绝同一请求同时写入新代理密码和清理旧代理凭据，避免本地副作用语义冲突。
  - `apps/desktop/src-tauri/src/commands/settings.rs` 单测覆盖代理密码写入本地 vault、转发 payload 不包含明文 `proxy_password`、清理代理凭据引用，以及冲突代理凭据动作早失败。
  - `pnpm --dir apps test`、`cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml` 和 `pnpm --dir apps check` 已覆盖前端/shared 不保存敏感字段、Go core 只持久化本地 vault 凭据引用、Rust 本地 vault 读写删除和明文不转发。

### T21 AI 配置 API 和 Rust 内部密钥注入

- 状态：`[x]`
- 依赖：T09、T20
- 交付物：
  - `/api/ai/configs/list`
  - `/api/ai/configs/save`
  - `/api/ai/configs/delete`
  - `/api/ai/configs/test`
  - `ai_config_*` Rust command。
- 执行动作：
  - 前端保存时只把明文 Key 交给 Rust command。
  - Rust 写入本地文件 vault。
  - Go 只保存 `api_key_ref` 和脱敏状态。
  - test 和 analysis 复用 `resolved_api_key` 内部注入协议。
- 验证：
  - list 不返回真实 API Key。
  - delete 同步删除本地 vault 凭据项。
  - test 失败错误不包含 Key、请求头、代理认证。
- 退出条件：
  - 模型配置页可真实保存和测试模型。
- 当前进展：
  - 已在 `apps/sidecar-core/internal/service/ai` 新增 AI 配置安全规则。
  - 已实现配置列表安全展示模型，只返回 `api_key_ref`、`masked_api_key` 和 `has_api_key`，不包含真实 API Key。
  - 已实现 Go core 保存配置规则，拒绝保存请求携带 `raw_api_key`。
  - 已实现模型连通性测试请求的安全日志快照，只记录 `has_api_key`，不记录 `resolved_api_key` 字段名或运行期 Key 明文。
  - 已新增前端/shared 源码安全扫描，禁止 `resolved_api_key`、`raw_api_key` 和 runtime token header 暴露到 renderer 源码。
  - 单测覆盖列表不回显真实 Key、保存拒绝 raw key、保存只保留本地 vault 凭据元数据、非法 `api_key_ref` 拒绝、未脱敏 `masked_api_key` 拒绝和测试请求日志脱敏。
  - 已新增 `apps/sidecar-core/internal/actions/aiconfig`，接入 `POST /api/ai/configs/list`、`POST /api/ai/configs/save`、`POST /api/ai/configs/delete`。
  - AI 配置 API 已接入真实 `dao.Store`，保存时复用 service 规则拒绝 `raw_api_key`，列表只返回安全展示字段。
  - 删除配置使用软删除；Rust command 携带 `api_key_ref` 时会同步删除本地 vault 凭据文件。
  - Rust `ai_config_delete` 和 `ai_config_test` 已在读取或删除本地 vault 前校验 `id > 0`，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`ai_config_list`、`ai_config_save`、`ai_config_delete`、`ai_config_test`，分别固定映射到对应 Go API。
  - 已接入 `POST /api/ai/configs/test`，Go core 根据配置 ID 读取 AI 配置元数据，并只使用 Rust 注入的 `resolved_api_key` 进行模型连通性测试。
  - `ai_config_test` 会从 Rust 本地 vault 读取真实 Key 后转发内部请求，前端 payload 只需要传 `id` 和 `api_key_ref`。
  - `apps/sidecar-core/internal/actions` 单测覆盖保存拒绝 raw API Key、列表不泄露 `raw_api_key` / `resolved_api_key`、删除后列表隐藏、模型测试成功不泄露密钥、Provider 错误响应脱敏；Rust 单测覆盖非正数配置 ID 早失败。
  - 生产 `main.go` 已注入 `OpenAIConfigTester`，避免真实 sidecar 的模型连通性测试接口因为漏配 tester 返回不可用。
  - `apps/sidecar-core/cmd/invest-compass-core` 单测覆盖生产 actions config 必须注入 AI config tester、核心 store 和日志导出源。
  - `apps/desktop/test/security-config.test.mjs` 覆盖 AI 配置 Rust command 注册和固定 path。
  - 已移除平台凭据服务强依赖，`ai_config_save` 改为使用 Rust 本地文件 vault 保存一次性 API Key；新建配置尚无数据库 ID 时会生成唯一 `api_key_ref`，避免同 provider 的多个新配置互相覆盖密钥；更新 key 时会先校验旧引用，清理被替换的旧 vault 引用，并在清理失败时回滚新写入文件。
  - 前端 `/ai-settings` 已接入 `ai_config_list`、`ai_config_save`、`ai_config_delete` 和 `ai_config_test`，保存后只展示脱敏字段并清空明文输入，测试失败错误会二次脱敏。
  - T21 开发闭环已完成；真实外部 Provider 连通性和跨平台桌面凭据验收继续归 T34/T44 收口。

### T22 OpenAI-compatible Provider

- 状态：`[x]`
- 依赖：T21
- 交付物：
  - `apps/sidecar-core/internal/service/ai`
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
  - 已新增 `apps/sidecar-core/internal/service/ai`，实现 OpenAI-compatible `/v1/chat/completions` 标准库 HTTP 客户端。
  - 已支持 base URL、model、temperature、max_tokens、timeout、普通 chat 和 stream chat。
  - 已支持 context cancellation，并将 401、429、5xx、取消/超时映射为稳定错误码。
  - 流式响应支持 `data: ...` chunk 和 `[DONE]` 结束语义。
  - 流式响应会拒绝缺少 `choices` 的异常 chunk，避免异常 Provider payload 被静默吞掉。
  - Provider 错误会统一脱敏，错误文本不泄露 Authorization 或 API Key。
  - mock server 单测覆盖成功、401、429、5xx、context cancellation、流式 chunk 和异常流式 payload。
  - `OpenAIConfigTester` 已复用 OpenAI-compatible 普通 chat completions 执行模型连通性测试，mock server 单测覆盖运行期 Key 注入和安全结果返回。
  - `analysis.Executor` 已复用 OpenAI-compatible Provider 执行分析任务，真实任务 context cancellation 和报告保存链路已由 T26 接入并覆盖单测。

### T23 Prompt 模板 CRUD 和变量白名单

- 状态：`[x]`
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
  - 已新增 `apps/sidecar-core/internal/service/prompt` 纯规则模块，定义首版模板类型和变量白名单。
  - 已限制模板类型只允许 `system`、`stock_full`、`technical`、`custom`。
  - 已限制变量只允许 `stock_name`、`stock_code`、`market`、`quote`、`kline_summary`、`indicators`、`news`、`analysis_language`。
  - 已实现内置模板删除规则、软删除过滤和变量提取去重。
  - 已实现 Prompt 模板创建、更新、删除和列表的纯领域规则，创建/更新时统一固化变量列表。
  - 已限制内置模板更新和删除，允许删除的模板只执行软删除，列表默认不返回已软删除模板。
  - 单测覆盖未支持模板类型、未支持变量、创建固化变量、更新只读内置模板、删除软删除、列表过滤排序和变量提取顺序。
  - 已在 Go core `apps/sidecar-core/internal/actions/prompt` 接入 `POST /api/prompt-templates/list`、`POST /api/prompt-templates/get`、`POST /api/prompt-templates/create`、`POST /api/prompt-templates/update`、`POST /api/prompt-templates/delete`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - Prompt 模板 API 已接入真实 `dao.Store`，创建和更新会固化变量白名单结果，列表和详情不返回已软删除模板。
  - Rust `prompt_templates_get`、`prompt_templates_update` 和 `prompt_templates_delete` 已在转发前校验 `id > 0`，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`prompt_templates_list`、`prompt_templates_get`、`prompt_templates_create`、`prompt_templates_update`、`prompt_templates_delete`，固定映射到对应 Go API，禁止通用 path 代理。
  - 单测覆盖 CRUD、未支持变量保存失败、内置模板更新/删除拒绝、软删除隐藏、Rust command 固定 path 和非正数模板 ID 早失败。
  - Prompt 模板页面真实交互已由 T34 接入并覆盖未支持变量拒绝保存，不再阻塞本 CRUD/API 基线退出。

### T24 Prompt 构建和合规输出约束

- 状态：`[x]`
- 依赖：T17、T18、T23
- 交付物：
  - `apps/sidecar-core/internal/service/prompt`
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
  - 已在 `apps/sidecar-core/internal/service/prompt` 中新增个股综合分析和技术面分析 Prompt builder。
  - 已实现 System、Context、User 三层 Prompt 分离。
  - System Prompt 固定包含“不构成投资建议”、风险、数据时效、事实/推断/观点区分、观察指标和用户自行决策提示。
  - System Prompt 明确禁止“稳赚”“必涨”“买入信号”等诱导表达。
  - 用户一次性持仓输入只进入 User Prompt，不进入 System 或 Context。
  - 单测覆盖缺失数据、含 userPosition、无 userPosition、合规文案和未替换变量残留。
  - `analysis.Executor` 已在任务执行链路中按分析类型调用 Prompt builder，并把 System、Context、User 三层内容传入 OpenAI-compatible Provider。

---

## 9. P5：分析任务、SSE 转发、报告和任务历史

### T25 任务状态机和事件持久化

- 状态：`[x]`
- 依赖：T09、T10
- 交付物：
  - `apps/sidecar-core/internal/service/task`
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
  - 已新增 `apps/sidecar-core/internal/service/task` 纯状态机和事件模型。
  - 已实现 `PENDING`、`RUNNING`、`SUCCESS`、`FAILED`、`CANCELLED` 状态定义和合法流转校验。
  - 已实现 `TASK_CREATED`、`TASK_STARTED`、`TASK_PROGRESS`、`TASK_LOG`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`、`TASK_CANCELLED` 事件类型。
  - 已实现事件 payload 入库前统一脱敏、按 id 递增回放、RUNNING 任务恢复为终态的基础决策。
  - 单测覆盖非法状态迁移、事件状态映射、事件回放排序、RUNNING 恢复和敏感 payload 脱敏。
  - 已在 `dao.Store` 接入 tasks / task_events 真实写入、状态查询、任务列表、任务详情和事件增量回放。
  - `AppendTaskEvent` 在写入数据库前统一脱敏 payload，覆盖 API Key、Authorization、代理密码和用户一次性持仓输入。
  - 单测覆盖任务和事件持久化、事件按 ID 递增回放、任务历史列表/详情读取和敏感 payload 入库前脱敏。

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
  - 已新增 `apps/sidecar-core/internal/service/analysis` 纯规则模块。
  - 已实现 `symbol`、`analysisType`、`aiConfigId`、`promptTemplateId` 创建请求校验和股票代码标准化。
  - 已限制首版分析类型为 `stock_full`、`technical`。
  - 已实现普通日志输入快照，默认只记录 `has_user_position`，不记录一次性持仓明细。
  - 已实现报告输入快照，允许把一次性持仓输入保存到 `analysis_reports.input_snapshot`，但不包含 `resolved_api_key`、`raw_api_key` 或 API Key 字段。
  - 已实现已校验分析请求到 `PENDING` 任务和 `TASK_CREATED` 事件的创建规则，事件 payload 只保留安全摘要。
  - 已实现取消规则：非终态任务可切换为 `CANCELLED` 并生成 `TASK_CANCELLED` 事件，终态任务不可重复取消。
  - 单测覆盖缺少模型配置、缺少模板配置、非法分析类型、日志快照脱敏、报告快照保留一次性持仓、创建任务安全事件和取消状态流转。
  - 已新增 `apps/sidecar-core/internal/actions/analysis`，接入 `POST /api/analysis/tasks` 和 `POST /api/tasks/cancel`。
  - 创建分析任务会校验 symbol、analysisType、aiConfigId、promptTemplateId，并要求 Rust 内部注入 `resolved_api_key`，但不会把真实 Key 或一次性持仓明细写入任务事件。
  - 任务创建会在同一 DAO 事务内写入 `PENDING` 任务和 `TASK_CREATED` 事件；取消会在同一 DAO 事务内写入 `CANCELLED` 任务和 `TASK_CANCELLED` 事件。
  - 已新增 Rust 白名单 command：`analysis_task_create`、`analysis_task_cancel`，分别固定映射到 `POST /api/analysis/tasks` 和 `POST /api/tasks/cancel`。
  - `analysis_task_create` 会先在 Rust 边界校验 `symbol`、`analysis_type` 非空、`api_key_ref` 必须是 `local-vault://ai-config/` 引用，以及 `ai_config_id > 0` 和 `prompt_template_id > 0`，再根据 `api_key_ref` 从 Rust 本地 vault 读取真实 Key，并只在内部请求体中注入 `resolved_api_key`。
  - `analysis_task_cancel` 会在 Rust 边界拒绝空 `task_id`，避免明显非法取消请求进入 Go core。
  - `apps/sidecar-core/internal/actions` 单测覆盖分析任务创建和取消；`apps/desktop/src-tauri` 单测覆盖分析任务内部请求不转发 `api_key_ref`。
  - 已新增 `analysis.Executor`，执行时读取 AI 配置、Prompt 模板、最新行情、K 线缓存和新闻缓存，构建合规 Prompt 后调用 OpenAI-compatible Provider，并按 `task_id` 幂等保存报告。
  - 执行器会写入 `TASK_STARTED`、`TASK_CHUNK`、`TASK_SUCCESS` 或 `TASK_FAILED` 事件；缺少真实行情/K 线时会失败并写失败事件，不伪造上下文数据。
  - `POST /api/analysis/tasks` 创建 PENDING 任务后会异步触发真实执行器，HTTP 请求不等待外部 AI Provider。
  - 已新增进程内运行中任务注册表，`POST /api/tasks/cancel` 在持久化 `CANCELLED` 状态和 `TASK_CANCELLED` 事件后，会按 `task_id` 取消正在执行的 executor context。
  - 分析执行器收到 `context.Canceled` 时直接退出，不再追加 `TASK_FAILED`，避免后台执行覆盖用户取消状态。
  - DAO 已新增 `GetPromptTemplate`，避免分析执行器绕过 dao 层或复用 action 层查询逻辑。
  - `apps/sidecar-core/internal/service/analysis` 单测覆盖成功执行、AI Prompt 上下文、报告保存、报告输入快照保留一次性持仓但不含密钥、缺数据失败和取消错误不覆盖任务状态。
  - `apps/sidecar-core/internal/actions` 单测覆盖取消 API 对运行中 executor context 的传播。
  - 后端/Rust 任务创建、取消、Provider 调用、报告保存和取消传播已接入，前端 `/analysis` 也已覆盖启动和取消触发；受真实 Market/News Provider 数据源授权、外部模型连通和跨平台桌面验收约束，完整主链路完成后再标记为 `[x]`。

### T27 SSE 事件和 Rust 转发

- 状态：`[x]`
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
  - 已在 `apps/sidecar-core/internal/service/task` 新增任务事件 SSE 帧编码基础能力。
  - 已复用 `ReplayEvents` 支持 afterEventID 之后的事件补拉和按事件 ID 递增排序。
  - 已支持多行 payload 按标准 SSE `data:` 行逐行编码，避免流式日志或 chunk 破坏事件帧。
  - SSE 帧包含 `id`、`event`、`data`，转发前统一复用事件 payload 脱敏。
  - 已新增前端/shared 源码安全扫描，禁止 renderer 直接使用浏览器 `EventSource` 或直连 `127.0.0.1` / `localhost` Go core 地址。
  - 单测覆盖 SSE 帧结构、payload 脱敏和 afterEventID 补拉编码顺序。
  - 已在 T29 接入 `POST /api/tasks/events` 增量事件回放和 Rust `task_events` 白名单 command。
  - 已接入 `POST /api/tasks/events/stream`，使用 `text/event-stream` 返回 afterEventID 之后的脱敏 SSE 帧。
  - Go SSE stream 在首次没有新增事件时会保持连接并轮询新增事件，收到 `TASK_SUCCESS`、`TASK_FAILED` 或 `TASK_CANCELLED` 后结束。
  - Go SSE stream 保持首次历史补拉不阻塞：如果首次查询已有待补拉事件，会写出后返回，调用方可用 `last_event_id` 继续订阅。
  - 已新增 Rust 白名单 command `analysis_task_subscribe`，固定映射到 `POST /api/tasks/events/stream`，由 Rust 读取 Go core SSE 帧文本并解析为结构化事件。
  - Rust 订阅 command 会向当前 Tauri window emit `analysis-task-event`，返回 `emitted` 和 `last_event_id`，前端仍不能直连 Go core。
  - Rust SSE 解析已兼容 LF 和 CRLF 帧边界，并保留多行 `data:` 合并解析，避免不同 HTTP 栈换行风格导致任务事件转发失败。
  - Rust 订阅和事件回放 command 会在转发前拒绝空 `task_id`，并继续拒绝负数 `after_event_id`。
  - `apps/sidecar-core/internal/actions` 单测覆盖 SSE 历史补拉脱敏和等待新增终态事件；`apps/desktop/src-tauri` 单测覆盖 Rust SSE 多行 `data:` 和 CRLF 帧边界解析。
  - 前端 `/analysis` 和 `/tasks` 已接入 `analysis_task_subscribe` 与 `task_events`，会先回放持久化事件再订阅新增事件，并展示流式 chunk、终态和失败原因。
  - T27 开发闭环已完成；真实桌面长连接体验和跨平台事件转发验收归 T44 收口。

### T28 报告保存、查询、删除

- 状态：`[x]`
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
  - 报告历史页和分析完成页具备可调用的真实报告 API 和 Rust command。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/report` 报告域规则模块。
  - 已实现同一 `task_id` 报告去重规则，保留 `updated_at` 最新报告，支撑后续按 `task_id` 幂等保存。
  - 已实现软删除过滤规则，列表和详情可复用同一可见报告口径。
  - 已实现 Markdown 导出规则，默认只导出报告元信息、AI 正文和风险摘要，不包含完整 `input_snapshot`。
  - 已支持显式选择时导出 `input_snapshot`，对应用户提示和 UI 入口待前端阶段接入。
  - 单测覆盖 `task_id` 去重、软删除过滤、软删除详情不可见、默认导出不包含 userPosition 和显式导出输入快照。
  - 已新增 `apps/sidecar-core/internal/actions/reports`，接入 `POST /api/reports/list`、`POST /api/reports/get`、`POST /api/reports/delete`。
  - 报告查询和删除已接入真实 `dao.Store`，删除使用软删除，列表和详情默认隐藏已删除报告。
  - 报告 API 默认不返回完整 `input_snapshot`，避免一次性持仓输入在历史查询和默认导出路径中泄露。
  - Rust `report_get` 和 `report_delete` 已在转发前校验 `id > 0`，非法请求不进入 Go core。
  - 已新增 Rust 白名单 command：`report_list`、`report_get`、`report_delete`，分别固定映射到对应 Go API。
  - `apps/sidecar-core/internal/actions` 单测覆盖报告列表、详情、软删除不可见和 `input_snapshot` 隐藏。
  - `apps/desktop/test/security-config.test.mjs` 覆盖报告 Rust command 固定路径映射；Rust 单测覆盖非正数报告 ID 早失败。
  - 真实报告历史页和分析完成页展示接入属于 T36，不在 T28 中冒充完成。

### T29 任务查询、事件回放和 RUNNING 恢复

- 状态：`[x]`
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
  - 已在 `apps/sidecar-core/internal/service/task` 补充任务历史列表排序规则，按 `updated_at` 倒序返回。
  - 已实现批量 `RUNNING` 任务恢复规则，复用单任务恢复逻辑，将悬挂任务恢复为终态并生成恢复事件。
  - 已复用现有 `ReplayEvents` 支持 `afterEventId` 增量事件回放。
  - 单测覆盖任务列表排序、批量 RUNNING 恢复和事件回放增量顺序。
  - Go core 启动时会扫描 `RUNNING` 任务，并在同一事务中保存恢复后的终态任务和 `TASK_FAILED` 恢复事件。
  - 已在 Go core `apps/sidecar-core/internal/actions/tasks` 接入 `POST /api/tasks/list`、`POST /api/tasks/get`、`POST /api/tasks/events`，复用 sidecar ready、runtime token、POST-only 和统一 envelope 安全边界。
  - 任务历史 API 已接入真实 `dao.Store`，列表按 `updated_at` 倒序返回，详情按 `task_id` 读取，事件按 `after_event_id` 增量回放并在输出前二次脱敏。
  - `POST /api/tasks/list` 已在 HTTP 边界限制 `limit` 必须为 1-100，避免外部请求触发 DAO 的无界任务历史查询语义。
  - `POST /api/tasks/events` 和 `POST /api/tasks/events/stream` 已限制 `after_event_id` 不能为负数，保留 `0` 作为从头补拉游标。
  - Rust `task_list`、`task_events` 和 `analysis_task_subscribe` 已在转发 Go core 前复用同样边界校验，非法参数不进入 sidecar。
  - 已新增 Rust 白名单 command：`task_list`、`task_get`、`task_events`，固定映射到对应 Go API，禁止通用 path 代理。
  - 单测覆盖人为制造 RUNNING 任务后的启动恢复、任务列表/详情 API、任务列表非法 `limit` 拒绝、负数事件游标拒绝、事件增量回放、事件脱敏、Rust command 固定 path 和 Rust command 入参早失败。
  - 任务历史真实页面已由 T36 接入并覆盖任务列表、详情、事件回放和详情订阅，不再阻塞本任务查询和恢复基线退出。

---

## 10. P6：前端页面闭环与设置中心

### T30 前端应用框架和 invoke 服务层

- 状态：`[x]`
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
- 当前进展：
  - 已新增前端/shared 源码安全扫描，禁止 renderer 使用 `localStorage`、`sessionStorage`、`indexedDB` 保存敏感状态，禁止直连 Go core 和处理 runtime token。
  - 已新增前端生产源码 mock 数据扫描，禁止 `mock` / `fake` / `dummy` / `fixture` / `demo` / `sample` 业务数据和硬编码投研数据伪装真实能力。
  - 已新增前端/shared 数据源扫描，禁止 renderer 绕过 typed invoke service 直接使用 `fetch`、`XMLHttpRequest`、`axios` 或硬编码 HTTP 数据源取数。
  - 已新增 `apps/frontend/src/services/coreClient.ts` typed invoke service，`coreHealth()` 固定调用 Rust `core_health` command 并解包统一响应。
  - 已将前端首屏接入真实 `core_health` 启动检查，覆盖加载、成功和失败状态，不直连 Go core。
  - 已接入 HashRouter、主导航、应用布局和 `AppErrorBoundary`，为后续页面任务提供基础外壳。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖固定 command 调用和统一错误响应。
  - `apps/frontend/src/app/App.test.tsx` 覆盖启动 health 检查、路由外壳和错误边界。

### T31 Dashboard 页面

- 状态：`[x]`
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
- 当前进展：
  - 已将首页从单一 `core_health` 启动检查扩展为 `core_health` + `dashboard_summary` 并行读取，所有数据仍通过 typed invoke service 调用 Rust 白名单 command。
  - 已展示自选股涨跌平分布、今日热点、最近报告、最近任务、数据源状态和风险提示，不引入策略、公告、研报、资金流等首版未闭环入口。
  - 已覆盖正常数据、无数据和 Provider 异常三类首屏状态；Provider 不可用时展示明确状态和错误原因，不使用假数据兜底。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖 `dashboardSummary()` 固定调用 `dashboard_summary`。
  - `apps/frontend/src/app/App.test.tsx` 覆盖 Dashboard 首屏真实总览展示、空状态和 Provider 异常状态。

### T32 自选股页面

- 状态：`[x]`
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
- 当前进展：
  - 已新增前端 typed service：`stockSearch()`、`marketQuote()`、`watchlistList()`、`watchlistCreate()`、`watchlistUpdate()`、`watchlistDelete()`，均固定调用 Rust 白名单 command，不接触 Go core 地址或 token。
  - 已新增 `/watchlist` 页面和主导航入口，页面读取 `watchlist_list` 后按每个 symbol 调用 `market_quote` 展示真实行情；行情失败时展示错误，不填充假价格。
  - 已支持搜索股票、添加自选、编辑排序/标签/备注、刷新列表和软删除自选；删除后仍可从搜索结果重新添加。
  - 自选股行内已提供单股 quote 刷新入口，调用 `scheduler_refresh_symbol` 并提交 `data_type=quote`，成功后重新读取该 symbol 的行情缓存。
  - 已覆盖重复添加时展示后端业务错误，避免把失败吞掉或显示成功态。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖自选股、股票搜索和行情 command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖自选股列表行情、行内单股刷新、搜索添加、更新、删除后重加和重复添加错误展示。

### T33 个股详情和 K线图页面

- 状态：`[~]`
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
- 当前进展：
  - 已新增前端 typed service：`marketKline()`、`marketIndicators()`、`newsList()`，均固定调用 Rust 白名单 command，不接触 Go core 地址或 token。
  - 已新增 `/stocks/:symbol` 个股详情路由，并从自选股 symbol 进入详情页。
  - 页面已读取真实 `market_quote`、`market_kline`、`market_indicators` 和 `news_list` 数据，展示行情摘要、Lightweight Charts K 线、K 线表格摘要、技术指标摘要和相关新闻。
  - period/adjust 切换会重新读取 K 线和指标，切换时保留旧数据直到新请求返回，避免控件消失。
  - K 线、技术指标和新闻不足时展示明确空状态。
  - 新闻列表已过滤为 HTTPS URL，并通过 Rust `open_external_url` 白名单 command 打开系统浏览器；真实跨平台桌面打开仍待 T44 验收，因此 T33 仍保持 `[~]`。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖 K 线、指标和新闻 command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖详情页数据展示、period/adjust 切换、HTTPS 新闻过滤和空状态。

### T34 模型配置和 Prompt 模板页面

- 状态：`[~]`
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
- 当前进展：
  - 已新增前端 typed service：`aiConfigList()`、`aiConfigSave()`、`aiConfigTest()`、`aiConfigDelete()` 和 `promptTemplates*()`，均固定调用 Rust 白名单 command。
  - 已新增 `/ai-settings` 页面，模型配置表单支持一次性 API Key 保存、脱敏字段展示、编辑、删除和连通性测试。
  - API Key 明文只存在保存表单状态和 `ai_config_save` 一次性 payload 中，保存成功后表单会清空明文输入，列表只展示 `masked_api_key` / `has_api_key`。
  - 连通性测试失败消息会在页面层二次脱敏，避免 Provider 错误文本把 Key 展示给用户。
  - Prompt 模板页面只展示首版类型 `system`、`stock_full`、`technical`、`custom` 和变量白名单，未支持变量会在前端拒绝保存。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖 AI 配置和 Prompt 模板 command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖 API Key 保存后不回显明文、测试连接失败脱敏和未支持变量拒绝保存。
  - 真实外部 Provider 连通性和跨平台桌面凭据验收仍待 T44 发布验收阶段完成。

### T35 AI 分析页面

- 状态：`[~]`
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
- 当前进度：
  - 已新增 `/analysis` 页面，支持选择股票代码、分析类型、已保存 AI 模型和 Prompt 模板创建分析任务。
  - 页面通过 typed invoke service 调用 `analysis_task_create`，只传 `api_key_ref`，不把运行期 Key 注入字段暴露给前端。
  - 创建任务后会调用 `analysis_task_subscribe` 触发 Rust SSE 转发，并通过 `task_events` 读取持久化事件展示 `TASK_CHUNK` 和任务状态。
  - `TASK_SUCCESS` 携带 `report_id` 时会调用 `report_get` 展示最终报告正文和风险摘要。
  - 已支持停止生成按钮，取消后展示 `CANCELLED` 状态。
  - 已支持在分析结果区复制 Markdown 和导出 Markdown，默认只使用报告公开字段，不包含完整 `input_snapshot`。
  - 已支持在 `TASK_FAILED` 事件下直接展示脱敏后的失败原因。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖分析任务、任务历史和报告 command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖创建任务、订阅事件、展示报告、复制 Markdown、导出 Markdown、取消状态和失败原因展示。
  - 真实 Market/News Provider 填充、外部 Provider 真实连通和跨平台桌面剪贴板/下载验收仍待 T44 阶段收口。

### T36 报告历史和任务历史页面

- 状态：`[x]`
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
- 当前进度：
  - 已新增 `/reports` 页面，通过 typed invoke service 调用 `report_list`、`report_get`、`report_delete` 展示报告列表、报告详情和删除操作。
  - 已新增 `/tasks` 页面，通过 typed invoke service 调用 `task_list`、`task_get`、`task_events` 展示任务列表、任务详情、失败原因和持久化事件回放。
  - 任务详情打开时会先通过 `task_events(task_id, 0)` 补拉持久化事件，再以最新事件 ID 调用 `analysis_task_subscribe` 订阅新增事件，并用增量 `task_events` 合并展示新增事件。
  - 报告详情已支持复制 Markdown 和导出 Markdown，默认只使用报告标题、股票代码、分析类型、生成时间、正文和风险摘要，不包含完整 `input_snapshot`。
  - `apps/frontend/src/app/App.test.tsx` 覆盖报告列表/详情/复制 Markdown/导出 Markdown/删除、失败任务详情、事件回放和任务详情订阅新增事件。

### T37 设置中心

- 状态：`[x]`
- 依赖：T11、T20、T30
- 交付物：
  - 基础设置、数据源设置、代理设置、通知设置、工作区设置、缓存管理、开机自启、检查更新、关于应用。
- 执行动作：
  - 设置页读取真实 settings/cache/provider API。
  - 代理密码写入 Rust 本地文件 vault。
  - 授权信息只展示 FREE 状态占位，不提供激活入口。
- 验证：
  - 代理 URL 禁止包含 username/password。
  - 缓存清理不会删除报告和配置。
  - 关于页不展示未实现授权能力。
- 退出条件：
  - 设置页没有假按钮、假状态或半成品入口。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/settings` 设置中心规则模块。
  - 已实现代理 URL 校验，禁止在 URL 中携带 username/password，代理密码必须留给本地 vault 链路处理。
  - 已实现缓存清理目标过滤，确保缓存清理规则不会包含报告和配置。
  - 已实现关于页 FREE 授权占位模型，不提供激活入口或授权 URL。
  - 单测覆盖代理 URL 凭据拒绝、无凭据代理 URL 通过、缓存清理不包含报告/配置和 FREE 占位不暴露激活入口。
  - Rust settings command 已支持代理密码写入/删除本地 vault，并只把 `proxy_credential_ref` 转发给 Go core。
  - 已新增 `/settings` 页面，通过 typed invoke service 读取 settings、workspace、cache、provider status 和 autostart 状态，并支持保存代理设置、保存工作区、关闭到托盘开关、开机自动启动开关、清理允许的缓存目标、检查更新和导出脱敏日志。
  - 设置页会在提交前拒绝带 username/password 的代理 URL，不把代理密码混进 URL。
  - 设置页已支持读取和保存检查更新配置 `update.manifest_url` / `update.allowed_hosts`，并通过真实 `settings_set` 写入 settings。
  - 设置页已支持读取和保存任务成功/失败通知开关 `notifications.task_terminal`，AI 分析终态通知会尊重该设置，缺省保持开启。
  - 设置页日志导出会在空目标目录时早失败，不触发 `export_logs`；有效目录会 trim 后交给 Rust 白名单 command。
  - 关于应用只展示 FREE 占位和投研风险提示，不提供激活入口。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖 settings、workspace、cache、provider status、update 和 log export command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖设置页读取真实 command、代理 URL 凭据拒绝、关闭到托盘开关保存、开机自动启动开关保存、任务成功/失败通知开关保存、检查更新配置保存、缓存清理、检查更新和日志导出目录校验。
  - 开机自启已有设置页入口和 Rust 白名单 command 基线；真实 macOS / Windows 桌面验收继续归 T39/T44，不阻塞 T37 设置中心收口。

### T38 资讯中心页面

- 状态：`[~]`
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
- 当前进展：
  - 已新增 `/news` 资讯中心页面，通过 typed invoke service 调用 `news_market` 读取 CN 市场新闻，并支持输入股票代码后调用 `news_list` 读取个股新闻。
  - 页面只合并展示后端返回的市场新闻和个股新闻，不提供公告、研报、资金流等首版未闭环入口。
  - 页面会过滤非 HTTPS URL，并支持按后端返回的 `tags` 做标签筛选；新闻列表、空状态和错误状态已有前端测试覆盖。
  - `apps/frontend/src/services/coreClient.test.ts` 覆盖 `news_market` 固定 command 契约。
  - `apps/frontend/src/app/App.test.tsx` 覆盖资讯中心导航、市场/个股新闻读取、非 HTTPS 新闻过滤、标签筛选、空状态和错误状态。
  - 新闻外链已通过 Rust `open_external_url` 白名单 command 打开系统浏览器；真实 News Provider 授权和跨平台桌面打开验收仍待 T44 阶段收口，因此 T38 暂保持 `[~]`。

---

## 11. P7：跨平台桌面能力、打包、发布验收

### T39 托盘、窗口状态、通知、开机自启

- 状态：`[~]`
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
- 当前进展：
  - 已新增 Rust 桌面运行期模块，接入主窗口关闭事件，并从 Go core settings 读取 `window.close_to_tray` 作为关闭到托盘开关。
  - 已接入主窗口状态恢复：启动时从 Go core settings 读取 `window.main.x`、`window.main.y`、`window.main.width`、`window.main.height`，关闭主窗口时写回当前外层窗口位置和尺寸。
  - 窗口状态恢复只接受完整且尺寸合理的数据，缺字段、非法数字或过小尺寸时保留 Tauri 默认窗口状态，避免启动时恢复到不可用窗口。
  - 已启用 Tauri `tray-icon` feature，并在 Rust 桌面运行期安装真实托盘菜单；托盘左键点击或“打开工作台”菜单会恢复并聚焦主窗口，“退出”菜单走 Tauri 退出事件统一停止 sidecar。
  - Rust 单测覆盖 `window.close_to_tray` 只接受明确 `true`、settings 响应读取 dedicated key、隐藏窗口必须同时满足托盘可用和用户设置开启、真实托盘恢复入口已启用，以及窗口状态 settings 的固定 key 和恢复边界。
  - 前端设置页已重新暴露关闭到托盘开关，并通过 `settings_set` 保存 `window.close_to_tray`。
  - `apps/desktop/test/security-config.test.mjs` 覆盖 `tray-icon` feature、`TrayIconBuilder` 和托盘菜单固定入口，避免重新退化为无恢复入口的半成品关闭行为。
  - 已接入 Tauri notification 插件，main capability 只开放 `notification:allow-is-permission-granted`、`notification:allow-request-permission`、`notification:allow-notify`，不使用 `notification:default`。
  - 前端 AI 分析页会在 `TASK_SUCCESS` / `TASK_FAILED` 终态事件后触发桌面通知，并受设置项 `notifications.task_terminal` 控制；通知权限拒绝或系统通知失败不会改变分析任务状态。
  - 已接入官方 `tauri-plugin-autostart`，Rust 只暴露 `autostart_get` / `autostart_set` 两个白名单命令，前端设置页可读取和保存“开机自动启动”开关，默认 capability 不开放 `autostart:*` 直接插件权限。
  - 托盘、通知和开机自启仍需 macOS / Windows 真实桌面验收。

### T40 日志导出和二次脱敏

- 状态：`[~]`
- 依赖：T10、T37
- 交付物：
  - `/api/logs/export`
  - `export_logs`
  - 导出目录选择。
- 执行动作：
  - 导出前再次 redaction。
  - 只允许用户选择的导出目标。
  - 日志包含 request_id、trace_id；任务链路日志存在时继续保留 task_id，方便排障。
- 验证：
  - 构造含密钥日志后导出仍为脱敏值。
  - 未授权路径无法写入。
- 退出条件：
  - 用户可以导出可排障且不泄露敏感信息的日志。
- 当前进展：
  - 已在 `apps/sidecar-core/pkg/logger` 新增日志导出文本二次脱敏规则。
  - 已复用统一 `RedactText` 作为唯一脱敏来源，避免日志、错误和导出各自维护敏感字段规则。
  - 已在 `apps/sidecar-core/internal/service/logexport` 新增日志导出包规则，生成稳定文件名、UTC 时间和已脱敏内容，文件写入仍留给 Rust 路径授权层。
  - 导出文本保留 `request_id`、`trace_id`；任务链路日志存在时继续保留 `task_id` 等排障字段。
  - 已新增日志导出请求校验规则，缺少全局 `request_id` 或 `trace_id` 时返回稳定错误码；缺少 `task_id` 不会阻断启动、Provider、设置等非任务场景的日志导出。
  - 已新增 Go core 内存 `slog` 日志源，生产 `main.go` 会注入 `LogExportSource`，让 `/api/logs/export` 从运行期结构化日志快照生成导出包。
  - 内存日志源会保留原有 slog 输出链路，并在采集结构化敏感字段时先脱敏；导出包仍会再次执行二次脱敏。
  - 单测覆盖导出前二次脱敏 Authorization、API Key、代理密码和用户一次性持仓输入、quoted JSON 敏感字段值包含逗号时仍保持完整脱敏且 JSON 有效、排障字段校验、导出包元数据稳定性，以及内存日志源采集结构化 slog 后不泄露敏感字段。
  - 已新增 `apps/sidecar-core/internal/actions/logexport`，接入 `POST /api/logs/export`。
  - 日志导出 API 从注入的日志源生成导出包，Go core 不直接写入用户目录，避免任意路径写入风险。
  - API 返回内容已执行二次脱敏，保留 `request_id`、`trace_id`，并在任务链路日志存在时保留 `task_id` 排障字段。
  - 已新增 Rust 白名单 command `export_logs`，固定映射到 `POST /api/logs/export`，禁止通用 path 代理。
  - Rust `export_logs(target_dir)` 会先校验目标目录存在，非法目录不触发 Go core 日志导出请求。
  - Rust `export_logs(target_dir)` 会把 Go core 返回的脱敏日志包写入调用方指定目录，并返回 `file_path` 和 `file_name`。
  - Rust 文件写入会拒绝无效目标目录、带目录分隔符或路径穿越的导出文件名，且不会覆盖目标目录中已有同名文件；Unix/macOS 导出文件按 `0600` 创建，Go core 仍不接触用户目录。
  - `apps/sidecar-core/internal/actions` 单测覆盖日志导出二次脱敏、排障字段保留和缺少排障字段拒绝。
  - `apps/desktop/src-tauri` 单测覆盖日志导出只写入目标目录、拒绝路径穿越文件名、拒绝覆盖已有同名文件和 Unix/macOS 私有文件权限。
  - `apps/desktop/test/security-config.test.mjs` 覆盖 `export_logs` command 注册和固定 path。
  - 设置中心已接入手动日志导出目录输入和 Tauri dialog 原生目录选择入口，空目录会在前端早失败，不触发 `export_logs`，有效目录会 trim 后传给 Rust 白名单 command 并展示返回文件路径。
  - Tauri main capability 只开放 `dialog:allow-open`，不开放保存文件或消息弹窗权限。
  - macOS / Windows 跨平台手工验收仍待完成；完成前 T40 保持 `[~]`。

### T41 检查更新入口

- 状态：`[x]`
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
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/updatecheck` 检查更新信任边界规则模块。
  - 已实现更新 JSON 解析和必填版本校验，解析后会清理首版支持字段首尾空白。
  - 已实现更新 JSON URL、下载链接和发布说明链接的 HTTPS 与 allowlist 校验。
  - 已实现首版 `PROMPT_ONLY` 结果模型，只表达版本提示或外链，不包含下载、安装或静默升级动作。
  - 已实现简单点分版本比较，支持判断当前版本是否低于 manifest 版本。
  - 单测覆盖非法 JSON、缺少版本、字段清理、HTTP URL 拒绝、非 allowlist 域名拒绝、manifest 链接 allowlist 校验、新版本只提示和当前已是最新。
  - 已新增 `apps/sidecar-core/internal/actions/updatecheck`，接入 `POST /api/update/check`。
  - 更新 API 支持通过注入的 fetcher 拉取远程 manifest，拉取前先校验 manifest URL 必须是 HTTPS 且命中 allowlist。
  - 远程 manifest 响应体上限为 256 KiB，超限会直接失败，不把截断后的内容当成可信更新 JSON。
  - 远程 manifest 拉取不会跟随 HTTP 3xx 重定向，避免初始 allowlist URL 被重定向到未校验来源。
  - 更新 API 会校验 manifest 内下载链接和发布说明链接仍命中 allowlist，只返回 `PROMPT_ONLY` 提示结果。
  - 更新 API 已支持从 settings 表读取 `update.manifest_url` 和逗号分隔的 `update.allowed_hosts`，生产 main 注入默认 HTTP fetcher，避免运行期因为静态配置为空直接不可用。
  - 已新增 Rust 白名单 command `check_update`，固定映射到 `POST /api/update/check`，禁止通用 path 代理。
  - `apps/sidecar-core/internal/actions` 单测覆盖 HTTP manifest 拒绝、settings 配置来源、非 allowlist 链接拒绝、新版本提示结果、manifest 超限拒绝和 manifest 重定向拒绝。
  - `apps/desktop/test/security-config.test.mjs` 覆盖 `check_update` command 注册和固定 path。
  - 设置页已接入检查更新入口，并读取、展示、保存 `update.manifest_url` 和 `update.allowed_hosts`，保存时会 trim 后通过真实 `settings_set` 写入 settings。
  - 前端自动化覆盖更新配置从 settings 读取、编辑后保存，以及检查更新结果展示。

### T42 sidecar 二进制命名和打包

- 状态：`[~]`
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
- 当前进展：
  - 已在 Rust sidecar 模块新增 Tauri sidecar target triple 文件名规则。
  - 已覆盖 `aarch64-apple-darwin`、`x86_64-apple-darwin`、`x86_64-pc-windows-msvc` 的文件名。
  - 当前平台默认 core 二进制路径复用同一命名规则。
  - 单测覆盖三类首版目标平台文件名。
  - 已在 `tauri.conf.json` 配置 `bundle.externalBin = ["binaries/invest-compas-core"]`，按 Tauri sidecar 基名声明随包二进制。
  - 已新增 `scripts/build-sidecar.mjs`，`pnpm --dir apps sidecar:build` 会按当前平台生成 target triple 文件名的 Go sidecar，并保留本地兼容副本。
  - `scripts/build-sidecar.mjs` 已支持 `--target=<triple>` 和 `--all-targets` 目标选择，覆盖 `aarch64-apple-darwin`、`x86_64-apple-darwin`、`x86_64-pc-windows-msvc` 三类首版 sidecar 命名。
  - 已实际生成三类首版 sidecar 产物：Apple Silicon 为 Mach-O arm64、macOS Intel 为 Mach-O x86_64、Windows x64 为 PE32+ x86-64。
  - Windows x64 sidecar 构建已使用 `-ldflags "-H windowsgui"`，产物为 GUI subsystem，降低桌面应用启动 sidecar 时弹出控制台窗口的风险。
  - 已新增 `scripts/verify-sidecar-targets.mjs`、`pnpm --dir apps sidecar:verify-targets` 和 `pnpm --dir apps sidecar:check-targets`，可重复构建并复核三类首版 sidecar target 产物的存在性、非 symlink、非空、macOS 执行位、Mach-O / PE 架构和 Windows GUI subsystem。
  - 本机已通过 `pnpm --dir apps build` 生成 macOS Apple Silicon release 可执行文件，并通过 `tauri build --bundles app` 生成 `投研罗盘.app`。
  - 已确认 `投研罗盘.app/Contents/MacOS/` 内同时包含 `invest-compass-desktop` 和 `invest-compas-core`，包内 sidecar 为 Mach-O arm64。
  - 已用包内 `invest-compas-core` 执行 stdin 握手、ready JSON `protocolVersion=1` 和 `/internal/shutdown` 回环关闭验证；首次沙箱运行因本地监听权限失败，提权后验证通过。
  - 已新增 `scripts/verify-sidecar-runtime.mjs` 和 `pnpm --dir apps sidecar:smoke`，可重复验证 Apple Silicon `.app` 包内 Go core 完成 stdin token 握手、`/internal/health` 和 `/internal/shutdown`，并已接入 `release:check:local`。
  - Rust 运行期已优先解析发布包内 `Contents/MacOS/invest-compas-core`，环境变量覆盖仅用于本地调试，源码目录 `src-tauri/binaries` 只作为开发 fallback。
  - 本机 Apple Silicon 已真实启动 `投研罗盘.app`，进程参数确认 Go core 来自 `.app/Contents/MacOS/invest-compas-core`，退出应用后 `invest-compass-desktop` 和 `invest-compas-core` 均无残留进程。
  - Go core ready JSON 已携带 `protocolVersion`，Rust sidecar ready 解析会拒绝不匹配版本，避免 desktop/core 二进制版本不兼容时继续启动。
  - `pnpm --dir apps test` 已纳入 `scripts/build-sidecar.test.mjs`，覆盖默认当前平台、显式 target、all-targets 目标选择、Windows GUI subsystem 构建参数，以及三类首版 target sidecar 文件名必须匹配 Tauri `externalBin` 基名，且测试 import 脚本不会触发真实构建。
  - 已新增 `scripts/verify-desktop-package.mjs`，用于在生成桌面包后检查 macOS `.app` 或 Windows 解包目录中主程序和 Go sidecar 是否同时存在且非空；该脚本只验证包结构和基础文件有效性，不替代真实目标平台启动验收。
  - `scripts/verify-desktop-package.mjs` 已补充 macOS `.app` 包路径、包名与 Tauri `productName` 一致性、`Info.plist` 的 `CFBundleExecutable`、`CFBundlePackageType=APPL`、`CFBundleDisplayName` / `CFBundleName` 与 Tauri `productName` 一致性、`CFBundleIdentifier` / `CFBundleShortVersionString` / `CFBundleVersion` 与 Tauri 配置一致性、`CFBundleIconFile` 对应 `Contents/Resources` 直接子文件、`Contents/Resources` 非 symlink 和包内二进制执行位校验，避免把普通目录、错误应用名称、缺少可启动元数据、错误 bundle 类型、错误展示名、错误应用身份或版本、缺失声明资源、图标路径逃逸、依赖包外资源目录或不可执行二进制误判为可启动 app bundle；Windows 仍按解包目录验证。
  - `scripts/verify-desktop-package.mjs` 已支持可选 `--target` 参数，可复核 `aarch64-apple-darwin`、`x86_64-apple-darwin` 和 `x86_64-pc-windows-msvc` 包内主程序与 sidecar 的二进制架构；Windows target 还会校验 PE subsystem 为 GUI，避免目标平台包混入错误架构或 console subsystem 产物。
  - `scripts/verify-desktop-package.mjs` 会要求包根路径是目录，并拒绝包根路径、macOS `Contents` / `Contents/MacOS` 关键目录、声明图标时的 `Contents/Resources` 目录、`Info.plist`、包内主程序或 sidecar 使用 symlink 指向包外文件，避免把依赖外部文件的包误判为已随包。
  - `apps/desktop/test/security-config.test.mjs` 已覆盖 `externalBin` 配置和 `sidecar:build` 专用脚本。
  - macOS Intel 包和 Windows 包随包启动验收仍未完成。

### T43 发布文档、风险声明和用户手册

- 状态：`[~]`
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
- 当前进展：
  - 已新增 `docs/2026-06-18-invest-compass-release-user-guide.md`，覆盖首版产品边界、安装启动、模型凭据、数据时效、基本使用流程、发布检查、已知未闭环项和反馈材料。
  - README 已新增首版发布与使用指南入口，并避免继续使用过期的早期仓库状态描述。
  - 文档明确“仅作研究辅助，不构成投资建议”，并说明数据来源、时效、AI 输出限制和敏感凭据存储边界。
  - 用户手册已补充设置中心真实能力、检查更新 manifest / allowlist 配置边界，以及日志导出的前端校验和 Rust 写入边界。
  - 已同步用户手册和验收报告中的调度/设置边界：任务成功/失败通知和开机自启已写入设置中心能力，但开机自启的真实 macOS / Windows 平台验收仍待 T39/T44 收口；不再把股票详情页单股刷新入口列为未闭环项。
  - 文档明确不写授权激活、真实自动更新下载、交易、云同步、移动端等未实现能力。
  - 后续仍需随 T13/T18/T39/T42/T44 等真实 Provider、桌面能力和跨平台验收结果继续更新。

### T44 首版总体验收

- 状态：`[~]`
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
  - 设置中心支持工作区、代理、缓存、数据源状态、任务通知、检查更新和日志导出；托盘、开机自启和跨平台桌面行为不写成已完成入口。
  - 应用有明确风险提示。
  - 数据库升级不会丢失用户已有数据。
  - API Key 存入 Rust 本地文件 vault，配置查询不回显真实 Key。
  - 前端不能直接访问 Go sidecar，Rust command 必须白名单化。
  - 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。
  - macOS / Windows 使用 Rust 本地文件 vault，凭据保存/删除均通过验收。
  - Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。
  - 检查更新链接只允许 HTTPS allowlist 域名。
  - 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。
  - AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和持久化存储。
- 退出条件：
  - 验收报告写明通过项、未执行项、风险和复测命令。
- 已完成：
  - 已新增 `docs/2026-06-18-invest-compass-acceptance-report.md` 作为首版总体验收基线。
  - 验收报告已按技术方案第 19 章逐项映射当前状态，明确区分“部分通过”“未通过”和“未执行”。
  - 验收报告已记录 macOS / Windows 真实验收待办、已知风险、遗留项和复测入口。
  - 已新增前端生产源码 mock 数据扫描，作为“界面绝对不能 mock 数据”的自动化防线；完整 UI 验收仍需页面完成后逐屏执行。
  - 已新增前端生产源码首版未闭环入口扫描，禁止策略观察、授权激活、公告、研报、资金流、券商账户、自动下单、云同步和移动端等入口文案进入 renderer 源码；真实桌面逐屏验收仍需 T44 收口。
  - 已新增前端 `APP_NAV_ITEMS` / `APP_ROUTE_PATHS` 首版导航和路由白名单测试，避免未来把非 MVP 页面入口静默加入主导航或路由集合。
  - 已新增 `pnpm --dir apps acceptance:check` 作为本地发布候选自动化基线入口，聚合 `test`、`check`、`desktop:test` 和 `format:check`；真实 Provider smoke、包结构复核和跨平台人工验收仍需单独执行。
  - 已新增 `pnpm --dir apps release:check:local` 作为本机 Apple Silicon 发布候选聚合入口，串起 `acceptance:check`、SQLite 升级演练、三类 sidecar target 校验、构建、Tauri `.app` 打包、`package:verify`、`sidecar:smoke` 和 `git diff --check`；该命令不触发联网 Provider smoke，也不替代 macOS Intel / Windows 或桌面能力人工验收。
  - Go 测试和 sidecar 构建命令已使用 `-mod=readonly`，避免发布候选检查期间静默改写 `go.mod` / `go.sum`。
  - 已新增 `pnpm --dir apps sqlite:upgrade-rehearsal`，用于发布前重复演练 SQLite 迁移前备份、备份恢复、恢复库再次迁移和用户 settings 数据保留。
  - 已新增 `pnpm --dir apps sidecar:check-targets`，可在进入跨平台打包前先重复生成并校验 Apple Silicon、macOS Intel 和 Windows x64 三类 sidecar target 产物。
- 待完成：
  - 完成真实 macOS / Windows 桌面启动、sidecar、凭据、通知、托盘、打包验收。
  - 完成 T13/T18/T21/T26/T27/T39-T42 等剩余依赖项后，重新执行 `acceptance:check`、发布候选打包验证和真实桌面验收。
  - 前端主链路完成后逐屏验证无 mock 数据、无首版未闭环入口。

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
- T09 GORM dao 和事务层
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

- T20 本地凭据 vault 适配
- T21 AI 配置 API 和 Rust 内部密钥注入
- T22 OpenAI-compatible Provider
- T23 Prompt 模板 CRUD 和变量白名单
- T24 Prompt 构建和合规输出约束

说明：T20/T21 是安全关键路径，首版允许 Rust 本地文件 vault 替代平台凭据服务，但禁止用前端或 SQLite 明文 Key 临时替代。

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
- 不降低 sidecar、Rust 本地 vault、SSE、日志脱敏等安全边界。

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
cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml
cd apps/desktop && pnpm tauri build
```

跨平台和真实桌面能力必须补充手工验收：

```text
macOS：启动、本地凭据 vault、通知、托盘、sidecar、打包、签名/公证方案
Windows：启动、本地凭据 vault、通知、托盘、sidecar、NSIS/MSI、签名方案
```

---

## 15. 当前后端非 UI 停止线

截至 2026-06-18，当前工作树的 Go core、Rust 白名单 command、sidecar
安全启动、SQLite/GORM、任务/报告/SSE、AI Provider、日志导出和检查更新
自动化基线已经推进到“非 UI 可验证”的边界。

继续开发前需要先解决以下非代码或跨层依赖：

- T13 / T18：Market Provider 已新增新浪、腾讯和东财公开网页接口的 service 层代码路径，
  但生产入口仍保持 `UnconfiguredProvider`，需要先确认数据源授权方式、频率限制、
  可分发边界和真实外网样例验收；News Provider 仍需确认真实数据源。
  确认前不能接入假行情、假 K 线或假新闻，也不能把未验收数据源切到生产默认入口。
- 基本面 / F10：已新增 service 层 Provider 抽象和东财 HSF10 代码路径，但仍需
  确认数据授权、频率限制、可分发边界，并完成 API/Rust/cache/Prompt 接入设计
  后才能进入首版用户可见能力。
- T31-T38：均为页面闭环任务；后端和 Rust command 已提供基础能力的任务，
  不能因为页面未完成而标记为整体完成。
- T39：托盘菜单、关闭到托盘设置、窗口状态恢复、任务成功/失败通知和开机自启
  已形成自动化基线；托盘、通知和开机自启仍需 macOS / Windows 真实桌面验收。
- T40 / T41：后端、Rust command 和用户触发链路基线已接入；完整验收仍依赖
  macOS / Windows 真实桌面手工验证。
- T42 / T44：macOS Intel 和 Windows 随包启动、sidecar、本地 vault、托盘、
  通知和安装包验收需要目标平台真实环境。

因此，在未确认 Provider、Tauri 桌面能力配置或进入 UI 开发前，不应继续把
`[~]` 任务批量改成 `[x]`，也不应宣称 RG2、RG5、RG6 完整通过。

---

## 16. Review Gate

### RG1：sidecar 安全启动门禁

- T03-T07 全部完成。
- token 不出现在 argv/env/日志/配置/数据库。
- 前端不能直接访问 Go core。
- Rust 不存在任意路径代理。
- 当前状态：已通过本地自动化验证，后续跨平台打包验收仍归入 RG6。

### RG2：基础数据闭环门禁

- 自动化基线已通过：T08-T12、T15、T17 已完成，股票搜索、自选、行情、K 线、指标、新闻、Dashboard 和 Provider status 都已通过 Rust command 到 Go API，且不依赖假数据。
- 未完全通过：T13、T14、T16、T18、T19 仍受真实合规 Market/News Provider 数据源授权和接入约束。
- 真实 Provider 未确认前，不能把 RG2 宣称为完整通过。

### RG3：AI 和凭据门禁

- 自动化基线已通过：T20、T22、T23、T24 已完成，T21 后端/Rust 基线已完成。
- API Key 和代理密码只在 Rust 本地文件 vault 和运行期内存中出现。
- 配置查询、日志、错误、任务事件、报告快照不泄露真实 Key。
- 未完全通过：真实外部模型连通性和跨平台桌面凭据验收仍归 T34/T44。

### RG4：分析任务门禁

- 自动化基线已通过：T25、T28、T29 已完成；T26 后端/Rust 执行、取消、报告保存已接入；T27 Go SSE 和 Rust 转发已接入。
- 任务事件可回放，SSE 只由 Rust 转发。
- sidecar 重启后 RUNNING 任务不会永久悬挂。
- 报告按 task_id 幂等保存。
- 未完全通过：真实 Market/News Provider 数据填充、外部模型连通和跨平台桌面剪贴板/下载验收仍归 T13/T18/T35/T44。

### RG5：首版页面门禁

- 自动化基线已覆盖 T30-T38 的主要页面主链路；T33/T34/T35/T38 仍受真实 Provider、外部模型连通和跨平台桌面行为验收约束。
- 页面只展示首版真实可执行功能。
- 不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。

### RG6：发布验收门禁

- T39-T44 全部完成。
- macOS 和 Windows 均完成真实安装/启动/主链路验收。
- README、用户手册、风险声明与实际能力一致。
