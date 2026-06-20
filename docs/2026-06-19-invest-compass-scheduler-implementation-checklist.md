# 投研罗盘定时任务调度实施清单

> 来源：`docs/2026-06-19-invest-compass-scheduler-design.md`
>
> 目标：把定时任务调度方案拆成可执行、可编辑、可推进、可验证的任务清单。本文只描述调度专题，不把资金流、F10、基金中心或 AI 自动报告提前混入首批实现。

---

## 1. 范围

### 1.1 首批必须交付

- Go sidecar 内部 `SchedulerService`。
- `github.com/go-co-op/gocron/v2` 作为定时任务调度库。
- 进程内轻量 `ExecutionQueue`。
- `scheduler_jobs`、`scheduler_runs`、`ingestion_watermarks` 三张 SQLite 表。
- 启动恢复 enabled jobs。
- 恢复异常退出遗留的 queued / running run。
- 交易日和交易时段判断。
- 自选股 quote 刷新任务。
- 自选股 K 线刷新任务。
- 市场新闻刷新任务。
- 手动单股强制刷新 `refresh-symbol`。
- startup catchup 和 manual backfill。
- Go API、Rust 白名单 command、桌面管理入口的最小闭环。

### 1.2 首批明确不做

- 不引入 Redis、NATS 或外部消息队列。
- 不做多 sidecar 实例抢占同一任务。
- 不开放 HK / US 调度任务，除非对应 Provider 已完成支持和验收。
- 不把 Provider 抓取函数改成直接写库。
- 不把普通 `market_quote` / `market_kline` 页面读取强制改成后台任务。
- 不在首批默认启用任何未完成授权和频率验收的数据源任务。
- 不在首批实现 AI 自动分析报告调度。

---

## 2. 状态标记

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成并通过验证
- `[!]` 阻塞或需要人工确认

---

## 3. 依赖路线

```text
S0 确认边界和文档同步
  ↓
S1 数据库和 DAO 基线
  ↓
S2 Scheduler service 和执行队列
  ↓
S3 Go API 与 Rust command
  ↓
S4 数据刷新任务和补偿任务
  ↓
S5 桌面管理页面
  ↓
S6 验收、文档同步和后续扩展停止线
```

并行原则：

- S1 的 schema / dao 是上游契约，必须先落定。
- S2 可以在 S1 的模型字段稳定后并行拆 `TradingCalendar`、`JobRegistry`、`ExecutionQueue`。
- S3 必须等 Scheduler service 的接口稳定后推进。
- S4 的各 job executor 可并行，但不得同时修改同一个 Provider 文件。
- S5 必须等 Go API 和 Rust command schema 稳定后推进。

---

## 4. Review Gate

### RGS1 数据库和依赖门禁

- `go-co-op/gocron/v2` 依赖已确认并由 `go get` 引入。
- SQLite 新表完成 migration 和 dao 单测。
- 不破坏现有 `tasks` / `task_events` / `settings` 语义。

### RGS2 调度内核门禁

- gocron 只负责定时触发和基础并发保护。
- 所有执行都走 `ExecutionQueue` 和 Runner。
- 同一 job / scope 不重叠执行。
- queued / running run 在启动时恢复为终态。

### RGS3 API 和 Rust 安全门禁

- Go API 全部 POST。
- API 复用 token、ready、`httpx.DecodeJSON`。
- Rust command 固定 path，不提供任意路径代理。
- 非法 ID、非法 symbol、非法 cron 在进入 Go core 前或 handler 边界被拒绝。

### RGS4 数据刷新门禁

- quote、kline、news 刷新都通过 Provider -> service -> dao.Store。
- Provider 不直接写库。
- Provider unavailable 时不写假数据。
- 补偿抓取有 symbol、日期、timeout、并发上限。

### RGS5 桌面管理门禁

- 设置中心可以查看任务、启停任务、立即执行、查看 run 记录。
- 未配置 Provider 的任务展示为不可用或失败原因明确。
- UI 不暴露未闭环的 AI 自动分析、HK/US、资金流专题入口。

---

## 5. S0：确认边界和文档同步

### S00 确认调度专题实施边界

- 状态：`[x]`
- 依赖：无
- 交付物：
  - 本实施清单。
  - 技术方案中定时任务调度章节的同步说明。
- 执行动作：
  - 确认首批只实现数据刷新、补偿抓取和桌面管理。
  - 确认 `analysis_report_schedule` 留作后续阶段。
  - 确认 `fundflow_refresh`、`market_snapshot_refresh` 不随首批默认进入 UI。
- 验证：
  - `git diff --check`
  - 文档没有把后续能力写成已完成。
- 退出条件：
  - 调度专题的首批范围、后续范围和停止线清晰可追踪。

### S01 同步主技术方案和总 checklist 引用

- 状态：`[x]`
- 依赖：S00
- 交付物：
  - `docs/2026-06-17-invest-compass-technical-solution.md`
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
- 执行动作：
  - 在技术方案中补充 Scheduler 模块概览、数据库表、API、Rust command 和安全边界。
  - 在总 checklist 中添加调度专题入口，引用本文，不批量展开所有子任务。
  - 明确调度专题涉及依赖、数据库、公共 API 和 UI 入口，需要确认后实现。
- 验证：
  - `git diff --check`
  - 总 checklist 不标记未实现任务为 `[x]`。
- 退出条件：
  - 主文档能导航到调度专题清单。
- 当前进展：
  - 主技术方案已补充 scheduler 模块概览、Rust command 固定映射、调度表说明和本地 API 边界。
  - 总 checklist 已新增“数据刷新调度专题”入口，明确专题清单路径、首批范围和停止线。
  - 后续 `analysis_report_schedule`、资金流刷新和独立市场快照专题仍不进入当前调度 MVP。

---

## 6. S1：数据库和 DAO 基线

### S02 引入 gocron 依赖

- 状态：`[x]`
- 依赖：S00
- 交付物：
  - `apps/sidecar-core/go.mod`
  - `apps/sidecar-core/go.sum`
- 执行动作：
  - 在 `apps/sidecar-core` 执行 `go get github.com/go-co-op/gocron/v2`。
  - 保持依赖只由 `internal/service/scheduler` 使用。
- 验证：
  - `cd apps/sidecar-core && go test ./...`
  - `rg "go-co-op/gocron" apps/sidecar-core` 只出现在 scheduler 包和测试中。
- 退出条件：
  - 依赖可编译，且未散落到 actions、dao、Provider 或 Rust 层。

### S03 新增 scheduler schema

- 状态：`[x]`
- 依赖：S02
- 交付物：
  - `apps/sidecar-core/internal/model/schema.go`
  - `apps/sidecar-core/internal/dao/database.go`
  - `apps/sidecar-core/internal/dao/database_test.go`
- 执行动作：
  - 新增 `SchedulerJob`、`SchedulerRun`、`IngestionWatermark` 模型。
  - `SchedulerJob` 使用 `cron_type` 保存任务类型，不使用 `type` 字段。
  - 所有表包含 `created_at`、`updated_at`。
  - `SchedulerJob` 使用软删除。
  - 为 `scheduler_runs.run_key` 建唯一索引。
  - 为 `ingestion_watermarks` 增加 `data_type + scope_key + provider + period` 唯一索引。
  - 把新模型接入 `dao.Migrate`。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TestMigrate`
  - 空库迁移成功。
  - 重复迁移不破坏已有表。
- 退出条件：
  - SQLite schema 可支撑任务配置、运行记录和补偿水位。

### S04 实现 scheduler dao 方法

- 状态：`[x]`
- 依赖：S03
- 交付物：
  - `apps/sidecar-core/internal/dao/store.go`
  - `apps/sidecar-core/internal/dao/repository_test.go`
- 执行动作：
  - 实现 job CRUD、active job 列表、启停、软删除。
  - 实现 run 创建、状态更新、按状态恢复、分页查询。
  - 实现 watermark upsert、读取和按范围查询。
  - 所有写入支持 context。
  - 重要组合写入通过 `WithTransaction` 调用。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run 'Scheduler|Watermark'`
  - 单测覆盖唯一约束、软删除过滤、run 状态流转、watermark upsert。
- 退出条件：
  - service 层不需要直接使用 GORM 就能管理调度数据。

---

## 7. S2：Scheduler service 和执行队列

### S05 建立 scheduler 包边界

- 状态：`[x]`
- 依赖：S04
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/doc.go`
  - `apps/sidecar-core/internal/service/scheduler/types.go`
  - `apps/sidecar-core/internal/service/scheduler/service.go`
  - `apps/sidecar-core/internal/service/scheduler/service_test.go`
- 执行动作：
  - 定义 `Service`、`Store`、`ProviderSet`、`CronType`、`TriggerType`、`RunStatus`。
  - 定义 `RunRequest`、`RunResult`、`JobDefinition`。
  - 保持中文包注释，说明 scheduler 只做调度和执行编排。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/scheduler`
  - 架构扫描不报缺少 `doc.go` 或中文注释。
- 退出条件：
  - 调度 service 边界清晰，不依赖 HTTP handler。

### S06 实现 TradingCalendar

- 状态：`[x]`
- 依赖：S05
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/trading_calendar.go`
  - `apps/sidecar-core/internal/service/scheduler/trading_calendar_test.go`
- 执行动作：
  - 支持 `CN` A 股交易日和交易时段判断。
  - 支持 `trading_time`、`after_close`、`any_time`。
  - 支持判断某个 job 在今天启动前是否已经错过可补偿窗口。
  - 周末返回非交易日。
  - HK / US 返回未支持错误或 skipped 原因。
- 验证：
  - 单测覆盖工作日上午、午休、下午、收盘后、周末、HK/US 未支持。
  - 单测覆盖 09:30 开盘任务在 10:00 启动时被识别为 missed today。
- 退出条件：
  - Runner 可基于统一日历判断是否执行或 skipped。
- 当前进展：
  - 已实现独立 `TradingCalendar`，覆盖 A 股交易时段、午休、收盘后、周末、unsupported market。
  - 已实现固定时分 cron 的交易日 `missed_today` 判断，覆盖 A 股 09:30 任务在 10:00 启动补偿。

### S07 实现 JobRegistry 和参数校验

- 状态：`[x]`
- 依赖：S05
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/registry.go`
  - `apps/sidecar-core/internal/service/scheduler/registry_test.go`
- 执行动作：
  - 注册 `watchlist_quote_refresh`、`watchlist_kline_refresh`、`market_news_refresh`、`symbol_news_refresh`、`manual_symbol_refresh`。
  - 校验 `cron_type` 只允许 JobRegistry 注册过的任务类型。
  - 校验 cron 表达式。
  - 校验 market 只允许 `CN`。
  - 校验 scope 和 params JSON。
  - 提供 job types 元数据给 UI。
- 验证：
  - 单测覆盖非法 cron、未知 `cron_type`、非法 market、非法 params、合法默认配置。
- 退出条件：
  - 用户创建或更新 job 前能在 service 层快速失败。
- 当前进展：
  - 已实现 `DefaultJobRegistry`、`cron_type` 元数据、gocron cron 校验、market/window/scope/params 校验。
  - Go API 保存 job 前会调用注册表校验，管理端可通过 `job-types` 读取类型元数据。

### S08 实现 ExecutionQueue

- 状态：`[x]`
- 依赖：S04、S05
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/queue.go`
  - `apps/sidecar-core/internal/service/scheduler/queue_test.go`
- 执行动作：
  - 实现内存队列和 worker。
  - 复用 gocron 的全局并发上限和单 job singleton 作为第一层保护。
  - 支持全局并发默认 2。
  - 支持同一 job 不重叠。
  - 支持同一 `data_type + scope_key + period` 不重叠。
  - 支持交互式手动刷新高优先级。
  - 队列入队时创建 `queued` run。
  - 重复 scope 执行时返回已有 run。
- 验证：
  - 单测覆盖优先级、去重、并发上限、context cancel、重复入队返回同一 run。
- 退出条件：
  - cron、manual 和 catchup 都能通过同一个队列执行。
- 当前进展：
  - 已实现进程内队列、run_key 入队去重、dequeue、worker 同步执行和手动触发高优先级。
  - 已实现同一 `data_type + scope_key + period + target_date` 的排队去重；同一 scope 不同交易日的 catchup run 可以排队。
  - 被队列去重拦下的 run 会立即标记为 `skipped`，不会残留永久 `queued`。
  - 已实现 `/api/scheduler/jobs/backfill` 对稳定 `run_key` 的幂等查询；重复补偿请求会返回已有 run，不创建新 run 也不重复入队。
  - 已实现 `/api/scheduler/refresh-symbol` 对 queued/running 的同一 `cron_type + data_type + scope_key + period + target_date` 返回已有 run，不创建 skipped run。
  - 已实现默认全局 worker 数 2，并把 gocron 第一层全局并发上限同步为 2。
  - 已实现队列优先级出队；交互式手动任务可排在低优先级补偿任务前。
  - 已补齐空队列等待出队时的 context cancel 专项测试，确保 worker 可在 shutdown/cancel 时被唤醒退出。
  - 已通过专项测试覆盖 active scope 去重、不同目标日期补偿并行排队、完成后释放 scope、默认 2 worker 并发启动和队列 worker 执行状态流转。

### S09 实现 Runner 和状态流转

- 状态：`[x]`
- 依赖：S06、S07、S08
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/runner.go`
  - `apps/sidecar-core/internal/service/scheduler/runner_test.go`
- 执行动作：
  - Runner 从 `queued` 更新为 `running`。
  - 根据交易窗口返回 `skipped` 或继续执行。
  - Provider unavailable 返回 `failed`，不写假数据。
  - 成功后写 `success`、计数和 watermark。
  - 所有错误通过 `logger.RedactText` 脱敏后写入 run。
- 验证：
  - 单测覆盖 success、failed、skipped、timeout、错误脱敏、watermark 更新。
- 退出条件：
  - 单次 run 的生命周期可观测且可恢复。
- 当前进展：
  - 已实现 queued -> running -> success / failed / skipped 的基础状态流转。
  - 已实现 `cn_a_share_quote_refresh`、`cn_a_share_kline_refresh`、`market_news_refresh`、`symbol_news_refresh` runner，Provider unavailable 会失败且不写假数据。
  - 已补齐 runner 失败写入 `scheduler_runs.error_message` 前的错误脱敏专项测试，避免 Authorization、API Key 等敏感内容落库。
  - 已实现 runner 执行时读取 `params_json.timeout_seconds` 并创建子 context；长期 job 派生的 run 会携带 `SchedulerJob.TimeoutSeconds`。
  - 已补齐 scheduled run 的交易窗口 skipped 矩阵：交易时段外、午休、收盘前 after_close 和非交易日都会落库为 `skipped` 且不进入队列。

### S10 实现 SchedulerService 生命周期

- 状态：`[x]`
- 依赖：S08、S09
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/service.go`
  - `apps/sidecar-core/internal/service/scheduler/service_test.go`
- 执行动作：
  - 初始化 gocron scheduler。
  - 加载 enabled jobs。
  - 注册、移除、重新注册 cron entry。
  - 支持 `Start`、`Stop`、`ReloadJob`、`RunNow`、`Backfill`。
  - `Stop` 取消队列并等待运行中任务退出或超时。
- 验证：
  - 单测覆盖启动加载、启用、禁用、删除、重新注册、停止取消。
- 退出条件：
  - main 可以把 SchedulerService 当成 sidecar 生命周期组件管理。
- 当前进展：
  - 已实现 gocron 初始化、enabled jobs 加载、Start、Shutdown、启动补偿和 worker。
  - 已实现 Provider 明确时基于 `ingestion_watermarks.last_trade_date` 的 startup `catchup_gap`，今天之前的缺口按 `catchup_max_days` 生成。
  - 已实现 `missed_today` 对固定时分 cron 和 interval cron 的启动补偿；用户 10:00 打开软件时，可补偿当天 09:30 后已经错过的最近一次计划窗口。
  - 已补齐 `missed_today` 的 `trade_window` 校验：只补偿原计划触发时间本身满足任务交易窗口的执行，避免把本来应跳过的窗口在启动恢复时重新入队。
  - 已实现 service 层 `RunNow`，`/api/scheduler/jobs/run-now` 在生产注入 service 时复用该方法创建 `user_request` run 并入队。
  - 已实现 service 层 `Backfill`，`/api/scheduler/jobs/backfill` 在生产注入 service 时复用该方法创建或复用 `catchup_gap` run 并入队。
  - 已实现 service 层 `ReloadJob`，保存、启停、删除长期任务后会移除旧 gocron entry 并按最新 enabled 状态重新注册。

### S11 接入 sidecar 启动恢复和关闭

- 状态：`[x]`
- 依赖：S10
- 交付物：
  - `apps/sidecar-core/cmd/invest-compass-core/main.go`
  - `apps/sidecar-core/cmd/invest-compass-core/main_test.go`
- 执行动作：
  - 在数据库迁移和 `recoverRunningTasksOnStartup` 后初始化 SchedulerService。
  - 启动时恢复 enabled jobs。
  - 将遗留 queued run 标记为 cancelled。
  - 将遗留 running run 标记为 failed。
  - 启动后触发 startup catchup，包含今天已错过窗口的 `missed_today` run。
  - shutdown 时停止 scheduler，再关闭 HTTP server。
- 验证：
  - `cd apps/sidecar-core && go test ./cmd/invest-compass-core`
  - 单测覆盖生产 config 注入 scheduler、恢复遗留 run、启动补偿入口、shutdown 调用 Stop。
- 退出条件：
  - sidecar 重启不会留下永久 running 的 scheduler run。
- 当前进展：
  - 已在 Go sidecar main 中接入 SchedulerService 启动和 shutdown。
  - 已将异常退出遗留的 queued run 标记为 `cancelled`，并记录 `sidecar restarted before queued run started`。
  - 已将异常退出遗留的 running run 标记为 `failed`，并记录 `sidecar restarted before scheduler run finished`。
  - 已实现 `missed_today` 启动补偿入口，覆盖固定时分 cron 和交易时段 interval cron；Provider 明确时也会执行 startup `catchup_gap`。
  - 已补 `cmd/invest-compass-core` 层生产 scheduler 组装、A 股 09:30 任务在 10:00 启动补偿，以及先停 scheduler 再通知 HTTP server 关闭的专项单测。

---

## 8. S3：Go API 与 Rust command

### S12 新增 scheduler Go API

- 状态：`[x]`
- 依赖：S10
- 交付物：
  - `apps/sidecar-core/internal/actions/scheduler/doc.go`
  - `apps/sidecar-core/internal/actions/scheduler/scheduler.go`
  - `apps/sidecar-core/internal/actions/scheduler/scheduler_test.go`
  - `apps/sidecar-core/internal/actions/router.go`
- 执行动作：
  - 实现 `/api/scheduler/jobs/list`。
  - 实现 `/api/scheduler/jobs/get`。
  - 实现 `/api/scheduler/jobs/save`。
  - 实现 `/api/scheduler/jobs/set-enabled`。
  - 实现 `/api/scheduler/jobs/delete`。
  - 实现 `/api/scheduler/jobs/run-now`。
  - 实现 `/api/scheduler/jobs/backfill`。
  - 实现 `/api/scheduler/refresh-symbol`。
  - 实现 `/api/scheduler/runs/list`。
  - 实现 `/api/scheduler/runs/get`。
  - 实现 `/api/scheduler/status`。
  - 实现 `/api/scheduler/job-types`。
- 验证：
  - API 单测覆盖 200、400、404、503。
  - 非 POST、无 token、未 ready 使用既有安全边界。
  - 请求体未知字段被拒绝。
- 退出条件：
  - Go API 能完整管理 job、run、backfill 和单股强制刷新。
- 当前进展：
  - 已实现 `/api/scheduler/job-types`、`/api/scheduler/jobs/list`、`/api/scheduler/jobs/get`、`/api/scheduler/jobs/save`、`/api/scheduler/jobs/set-enabled`、`/api/scheduler/jobs/delete`、`/api/scheduler/jobs/run-now`、`/api/scheduler/jobs/backfill`、`/api/scheduler/runs/list`、`/api/scheduler/runs/get`、`/api/scheduler/runs/trigger`、`/api/scheduler/refresh-symbol`、`/api/scheduler/status`。
  - 响应使用 `cron_type`，手动触发使用 `trigger_type=user_request` 并复用执行队列。
  - `jobs/backfill` 已校验 30 天范围和 30 个 symbol 上限，使用 `dateFrom/dateTo` 字段，只为交易日生成 `catchup_gap` run。
  - `jobs/run-now` 默认遵守任务交易窗口，只有显式 `ignore_trade_window=true` 时才允许绕过。
  - 已覆盖请求体未知字段拒绝、任务/执行记录 404、执行队列不可用 503 分支。

### S13 新增 Rust 白名单 command

- 状态：`[x]`
- 依赖：S12
- 交付物：
  - `apps/desktop/src-tauri/src/commands/scheduler.rs`
  - `apps/desktop/src-tauri/src/commands/mod.rs`
  - `apps/desktop/src-tauri/src/lib.rs`
  - `apps/desktop/test/security-config.test.mjs`
- 执行动作：
  - 新增 `scheduler_job_list`。
  - 新增 `scheduler_job_get`。
  - 新增 `scheduler_job_create`。
  - 新增 `scheduler_job_update`。
  - 新增 `scheduler_job_enable`。
  - 新增 `scheduler_job_delete`。
  - 新增 `scheduler_job_run_now`。
  - 新增 `scheduler_job_backfill`。
  - 新增 `scheduler_refresh_symbol`。
  - 新增 `scheduler_run_list`。
  - 新增 `scheduler_run_get`。
  - 新增 `scheduler_status`。
  - 新增 `scheduler_job_types`。
  - 每个 command 固定映射到 Go API path。
  - Rust 侧校验正数 ID、非空 symbol 和必要 payload。
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
  - `pnpm --dir apps test`
  - 安全测试覆盖 command 注册和固定 path。
- 退出条件：
  - 前端只能通过白名单 command 访问 scheduler API。
- 当前进展：
  - 已新增 `scheduler_job_types`、`scheduler_jobs_list`、`scheduler_jobs_get`、`scheduler_jobs_save`、`scheduler_jobs_set_enabled`、`scheduler_jobs_delete`、`scheduler_jobs_run_now`、`scheduler_jobs_backfill`、`scheduler_runs_list`、`scheduler_runs_get`、`scheduler_runs_trigger`、`scheduler_refresh_symbol`、`scheduler_status`。
  - 每个 command 固定映射到 `/api/scheduler/...` path。
  - 已在 `apps/desktop/src-tauri/src/lib.rs` 注册全部 scheduler 白名单 command。
  - 已补安全配置专项测试覆盖 command 注册和固定 path。
  - 已补 Rust 侧正数 ID、本地单股刷新 `symbol` / `data_type` / `target_date` 必填的快速失败测试和校验。
  - 已补 `scheduler_runs_list` 的非负 `job_id` 和 `limit=1..100` 校验。
  - 已补 `scheduler_jobs_backfill` 的正数 `id`、非空 `dateFrom/dateTo` 和非空 symbol 校验。
  - 已补 `scheduler_runs_trigger` 的非负 `job_id`、非空 `cron_type/scope_key/target_date` 校验。
  - 已补 Rust 侧 `scheduler_jobs_backfill` 的 `YYYY-MM-DD` 日期格式、`dateFrom <= dateTo`、30 个自然日范围和 1..30 个 symbol 上限校验。
  - S13 仍需随前端接入继续复核 command 注册、固定 path 安全测试和 UI 调用链覆盖。

### S14 新增前端 typed service

- 状态：`[x]`
- 依赖：S13
- 交付物：
  - `apps/frontend/src/services/scheduler.ts`
  - `apps/frontend/src/services/scheduler.test.ts`
  - 如当前前端目录结构不同，以现有 typed invoke service 目录为准。
- 执行动作：
  - 封装 job list、create、update、enable、delete、run-now、backfill。
  - 封装 refresh-symbol、run list、status、job types。
  - 不在 service 中拼接外部数据源。
  - 不保存任何凭据或 token。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
  - 单测覆盖 payload 转换和错误透传。
- 退出条件：
  - UI 可以通过单一 typed service 访问调度能力。
- 当前进展：
  - 已在现有 `coreClient.ts` 中新增 job list/get/save/set-enabled/delete/run-now/backfill、run list/get/trigger、refresh-symbol、status、job-types typed 方法。
  - 已有单测覆盖固定 Tauri command 和错误 envelope 解包。
  - 已拆出 `apps/frontend/src/services/scheduler.ts`，作为调度 UI 的单一 typed service 入口，不保存凭据、不拼接外部数据源。
  - 已补 `apps/frontend/src/services/scheduler.test.ts`，覆盖任务列表/状态读取、任务保存、手动补偿 payload 透传和统一错误响应透传。

---

## 9. S4：数据刷新任务和补偿任务

### S15 实现 watchlist_quote_refresh

- 状态：`[x]`
- 依赖：S09
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/jobs_quote.go`
  - `apps/sidecar-core/internal/service/scheduler/jobs_quote_test.go`
- 执行动作：
  - 读取 active watchlist。
  - 对 CN symbol 调用 `MarketProvider.Quote`。
  - 转换为 `model.Quote`。
  - 通过 `dao.Store.SaveQuote` 幂等写入。
  - Provider 不可用时 run failed。
  - 非交易时段按 trade window skipped。
- 验证：
  - 单测覆盖空自选、单只成功、多只部分失败、Provider unavailable、非 CN skipped。
- 退出条件：
  - 自选股 quote 可以由后台刷新写入缓存。
- 当前进展：
  - 已实现 `QuoteRefreshRunner`，可按 run 的 `scope_key` 抓取标准 symbol quote、写 `quotes`、更新 `ingestion_watermarks`。
  - 已支持市场范围任务读取 active watchlist，并只刷新其中的 CN 标的。
  - 已实现多只自选股部分失败策略：单只 Provider 抓取失败会继续处理后续 symbol，成功项仍写库，最终 run 以失败错误暴露部分失败。
  - 已实现 scheduled run 的交易窗口拦截：不满足 `trade_window` 时落库为 `skipped`，不进入执行队列。

### S16 实现 watchlist_kline_refresh

- 状态：`[x]`
- 依赖：S09
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/jobs_kline.go`
  - `apps/sidecar-core/internal/service/scheduler/jobs_kline_test.go`
- 执行动作：
  - 读取 active watchlist。
  - 按 params 中的 period、adjust、limit 调用 `MarketProvider.Kline`。
  - 转换为 `model.Kline`。
  - 通过 `dao.Store.SaveKlines` 幂等写入。
  - 成功后更新 watermark。
- 验证：
  - 单测覆盖日 K 成功、空 K 线失败、坏 period 拒绝、watermark 更新、重复写入幂等。
- 退出条件：
  - 自选股 K 线可在收盘后刷新并补偿。
- 当前进展：
  - 已实现 `KlineRefreshRunner`，可按 run 的 `scope_key` 和 `params_json` 抓取 K 线、写 `klines`、更新 `ingestion_watermarks`。
  - 已支持市场范围任务读取 active watchlist，并只刷新其中的 CN 标的。
  - 已覆盖日 K 成功、市场范围 active watchlist、空 K 线失败、坏 period 拒绝、watermark 更新。
  - 重复写入幂等由 dao `SaveKlines` 测试覆盖，runner 只负责把转换后的 K 线交给 dao。
  - scheduled run 的交易窗口 skipped 已由 `SchedulerService.EnqueueScheduledRun` 统一覆盖。

### S17 实现 market_news_refresh

- 状态：`[x]`
- 依赖：S09
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/jobs_news.go`
  - `apps/sidecar-core/internal/service/scheduler/jobs_news_test.go`
- 执行动作：
  - 调用 `NewsProvider` 获取市场新闻。
  - 转换为 `model.NewsItem`。
  - 通过 `dao.Store.SaveNewsItems` 按 `content_hash` 幂等写入。
  - Provider unavailable 时不写假数据。
- 验证：
  - 单测覆盖成功、重复新闻去重、Provider unavailable、错误脱敏。
- 退出条件：
  - 市场新闻可以由后台刷新写入缓存。
- 当前进展：
  - 已实现 `NewsRefreshRunner` 的市场新闻分支，调用新闻 Provider、标准化、按 `content_hash` 去重写入并更新 watermark。
  - 已覆盖市场新闻成功、错误脱敏、非 CN 市场跳过、个股新闻、市场范围读取 active watchlist、个股数量上限和非 CN 个股跳过。
  - 已覆盖未配置新闻 Provider 快速失败，确认不会写入假新闻或水位。
  - 已覆盖市场新闻 Provider 错误脱敏，避免认证信息进入 run 错误。
  - 已覆盖非 CN 市场范围，返回 `non_cn_scope` 且不调用新闻 Provider，避免后台任务误抓未支持市场。
  - 桌面 Provider 可用性展示由 S21 调度管理页面覆盖。

### S18 实现 symbol_news_refresh

- 状态：`[x]`
- 依赖：S17
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/jobs_news.go`
  - `apps/sidecar-core/internal/service/scheduler/jobs_news_test.go`
- 执行动作：
  - 从 scope 中读取 symbol 或 watchlist。
  - 对每个 symbol 调用新闻 Provider。
  - 写入 `news_items`。
  - 单次 symbol 数量受硬上限约束。
- 验证：
  - 单测覆盖单股新闻、自选股范围、超过 symbol 上限、非 CN skipped。
- 退出条件：
  - 个股新闻可以被后台或补偿任务刷新。
- 当前进展：
  - 已实现 `NewsRefreshRunner` 的个股新闻分支，支持从 run `scope_key` 读取一个或多个 symbol。
  - 已支持市场范围任务读取 active watchlist，并只刷新其中的 CN 标的。
  - 已实现 30 个 symbol 硬上限，超过上限时在调用 Provider 前失败。
  - 已实现非 CN 标的 skipped，返回 `non_cn_scope` 且不调用新闻 Provider。

### S19 实现 refresh-symbol 一次性强制刷新

- 状态：`[x]`
- 依赖：S12、S15、S16、S17
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/manual_refresh.go`
  - `apps/sidecar-core/internal/service/scheduler/manual_refresh_test.go`
  - `apps/sidecar-core/internal/actions/scheduler/scheduler_test.go`
- 执行动作：
  - 支持 `data_type=quote|kline|news|all`。
  - 不创建长期 scheduler job。
  - 生成 `trigger_type=user_request` run。
  - 同一 `data_type + symbol + period` 正在运行时返回已有 run。
  - 默认 timeout 30 秒。
- 验证：
  - 单测覆盖刷新 quote、刷新 K 线、刷新 news、all、多次点击去重、Provider unavailable。
- 退出条件：
  - 用户点击单股强制刷新时可复用执行队列。
- 当前进展：
  - 已实现 `/api/scheduler/refresh-symbol`、Rust `scheduler_refresh_symbol`、前端 `schedulerRefreshSymbol`。
  - 支持 `data_type=quote|kline|news|all`，生成 `trigger_type=user_request` run 并进入统一队列。
  - 重复同一 symbol、data_type、period、target_date 且已有 queued/running run 时，API 会返回已有 run，不创建 skipped run。
  - `/scheduler` 页面提供真实“单股刷新”入口，调用 `scheduler_refresh_symbol` 并复用统一执行队列。
  - `/stocks/:symbol` 股票详情页提供真实“刷新单股”入口，提交 `data_type=all` 并在 success run 返回后重新读取当前缓存。
  - 自选股行内刷新提交 `data_type=quote`，success run 返回后重新读取该 symbol 的行情缓存。
  - 前端测试覆盖自选股行内刷新、股票详情页 queued/running 提示和 success 后缓存重读。

### S20 实现 CatchupPlanner 和 backfill

- 状态：`[x]`
- 依赖：S06、S08、S16、S17
- 交付物：
  - `apps/sidecar-core/internal/service/scheduler/catchup.go`
  - `apps/sidecar-core/internal/service/scheduler/catchup_test.go`
- 执行动作：
  - 根据 watermark 计算缺失交易日。
  - startup catchup 只为 `catchup_enabled=true` 的 job 生成 run。
  - startup catchup 同时计算今天启动前已经错过的调度窗口，生成 `trigger_type=missed_today` 的 run。
  - `missed_today` 只补今天最近一次错过且仍有价值的窗口，不补未来窗口。
  - 对同一 `run_key` 已存在 success / queued / running 的 missed today run 不重复生成。
  - manual backfill 校验 symbol 数量、日期范围和数据类型。
  - 单次最多 30 个 symbol、30 个交易日。
  - 生成低优先级 run request。
- 验证：
  - 单测覆盖无缺口、有缺口、超过范围、非交易日跳过、startup catchup 限制。
  - 单测覆盖 A 股 09:30 开盘任务在 10:00 启动时生成 `missed_today`。
  - 单测覆盖同一 `run_key` 已成功时不重复生成 missed today run。
- 退出条件：
  - sidecar 离线或休眠后能有边界地补齐数据。
- 当前进展：
  - 已实现 `missed_today`：例如 A 股 09:30 开盘任务，用户 10:00 启动软件会生成当天一次性补偿 run。
  - 已实现 missed today 创建前的显式 `run_key` 查询，同一 `run_key` 已存在 success / queued / running 等执行记录时不重复生成。
  - 已实现 Provider 明确时基于 `ingestion_watermarks.last_trade_date` 的 startup `catchup_gap`，只补今天之前的交易日缺口，并受 `catchup_max_days` 限制。
  - 已实现 `/api/scheduler/jobs/backfill`、Rust `scheduler_jobs_backfill`、前端 `schedulerJobsBackfill`，支持 30 symbol / 30 天边界，只为交易日生成低优先级 `catchup_gap` run。
  - 已实现手动 backfill 的 `run_key` 幂等返回；重复请求返回已存在的 `catchup_gap` run，避免补偿重试变成 500。
  - 已覆盖跨周末历史缺口补偿矩阵，确认非交易日不会生成 `catchup_gap` run。

---

## 10. S5：桌面管理页面

### S21 新增任务调度设置页入口

- 状态：`[x]`
- 依赖：S14
- 交付物：
  - `apps/frontend/src/pages/settings/SchedulerSettingsPage.tsx`
  - 设置中心路由或 tab 配置文件。
- 执行动作：
  - 增加“数据刷新”或“任务调度”入口。
  - 展示 Provider 未配置状态。
  - 不展示 AI 自动分析、HK/US、资金流专题入口。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
  - 页面空状态、加载态、错误态可见。
- 退出条件：
  - 用户能进入任务调度管理页面。
- 当前进展：
  - 已在现有 `App.tsx` 中新增 `/scheduler` 路由和“任务调度”导航入口。
  - 设置中心已新增“数据刷新”区块，提供“进入任务调度”链接到 `/scheduler`。
  - 页面已读取真实 `scheduler_status`、job list、run list 和 job types。
  - 已读取 `providers_status` 并在任务调度页展示 Provider 未配置或不可用原因。
  - 当前沿用 `App.tsx` 路由实现，不额外拆分设置中心独立页面，避免为单入口引入不必要抽象。

### S22 实现 job 列表和任务操作

- 状态：`[x]`
- 依赖：S21
- 交付物：
  - `apps/frontend/src/components/scheduler/SchedulerJobTable.tsx`
  - `apps/frontend/src/components/scheduler/SchedulerJobEditor.tsx`
  - 对应测试文件。
- 执行动作：
  - 展示名称、类型、启用状态、cron、下次运行、上次结果、最近错误。
  - 支持启用、禁用、立即执行、删除。
  - 支持创建和编辑任务。
  - 未配置 Provider 的任务禁用立即执行或展示明确错误。
- 验证：
  - 单测覆盖启停、立即执行、表单校验、Provider 不可用状态。
- 退出条件：
  - 桌面端可管理长期 scheduler job。
- 当前进展：
  - 已在 `/scheduler` 页面展示 job 名称、`cron_type`、启用状态和 cron。
  - 已接入 `scheduler_jobs_run_now`，可对单个 job 生成当天手动执行 run。
  - 已接入 `scheduler_jobs_set_enabled` 和 `scheduler_jobs_delete`，支持启停与删除长期 job。
  - 已接入 `scheduler_jobs_save`，支持创建任务和把现有 job 填回表单编辑。
  - 已按任务类型映射 `market-provider` / `news-provider`，Provider 不可用时禁用立即执行并展示原因。
  - 已在 `dao.Store.UpdateSchedulerRun` 反写 job 最近状态、最近错误和最近运行时间，`/scheduler` 页面展示最近结果与最近错误列。
  - 已在 `/scheduler` job 表根据五段 `cron_expr` 显示下次运行时间，覆盖数字、星号、逗号、范围和步长。
  - 已抽取 `SchedulerJobTable` 并补组件测试，覆盖下次运行、Provider 不可用禁用立即执行和操作回调。
  - 已抽取 `SchedulerJobEditor` 并补组件测试，保持受控表单渲染、类型切换、字段编辑、提交和重置行为。

### S23 实现运行记录和手动补偿 UI

- 状态：`[x]`
- 依赖：S22
- 交付物：
  - `apps/frontend/src/components/scheduler/SchedulerRunList.tsx`
  - `apps/frontend/src/components/scheduler/SchedulerBackfillDialog.tsx`
  - 对应测试文件。
- 执行动作：
  - 展示 run 状态、开始/结束时间、抓取条数、写入条数、失败原因。
  - 支持按 job、status、trigger 过滤。
  - 支持手动 backfill。
  - backfill 表单限制 symbol 和日期范围。
- 验证：
  - 单测覆盖过滤、错误展示、超范围校验、提交成功状态。
- 退出条件：
  - 用户可以追踪调度运行情况并触发有边界的补偿。
- 当前进展：
  - 已在 `/scheduler` 页面展示最近 run 的状态、触发类型、目标日期、开始/结束时间和错误摘要。
  - 已支持按 job 重新拉取运行记录，并在当前记录内按 status、trigger 过滤。
  - 已实现桌面 backfill 表单，前端限制日期范围顺序、30 个自然日和 30 个 symbol。
  - 已实现 run 详情查看，点击最近执行记录可通过 `scheduler_runs_get` 读取单条执行详情，并展示来源、scope、开始/结束时间、抓取/写入统计、跳过原因和错误信息。
  - 已补状态和触发类型过滤前端单测，并为运行记录过滤控件增加可访问名称，便于桌面自动化验收。
  - 已补 backfill 超范围校验前端单测，超出 30 个自然日时不会提交 `scheduler_jobs_backfill`，并会清理上一条成功提示。
  - 已补执行详情读取失败的前端单测，`scheduler_runs_get` 返回错误时清理旧详情并展示错误提示。
  - 已抽取 `SchedulerRunList` 并补组件测试，覆盖过滤控件、执行记录、空状态、详情展示和详情事件分发。
  - 已抽取 `SchedulerBackfillDialog` 并补组件测试，保持受控表单渲染、字段编辑和提交行为。

### S24 接入单股强制刷新入口

- 状态：`[x]`
- 依赖：S19、S14
- 交付物：
  - 股票详情页或自选股行操作组件。
  - 对应测试文件。
- 执行动作：
  - 普通进入详情页仍走 `market_quote` / `market_kline`。
  - 用户点击强制刷新时调用 `scheduler_refresh_symbol`。
  - 如果返回已有 run，展示“正在刷新”状态。
  - 刷新完成后重新读取缓存数据。
- 验证：
  - 单测覆盖普通读取不入队、点击刷新入队、重复点击复用 run、Provider 不可用提示。
- 退出条件：
  - 手动单股刷新复用队列但不拖慢普通页面读取。
- 当前进展：
  - 已在 `/scheduler` 页面提供真实“单股刷新”入口，调用 `scheduler_refresh_symbol` 并复用统一执行队列。
  - 已在 `/stocks/:symbol` 股票详情页提供真实“刷新单股”入口，普通详情读取仍只调用 `market_quote` / `market_kline` / `market_indicators` / `news_list`。
  - 股票详情页刷新会调用 `scheduler_refresh_symbol` 并提交 `data_type=all`，复用统一执行队列刷新当前页展示的行情、K 线和新闻数据。
  - 自选股行内刷新会调用 `scheduler_refresh_symbol` 并提交 `data_type=quote`，成功后重新读取该 symbol 的行情缓存。
  - 已覆盖 queued/running run 返回时展示“正在刷新当前股票数据”，以及 success run 返回后重新读取当前缓存。

---

## 11. S6：验收、文档同步和停止线

### S25 增加端到端安全和架构护栏

- 状态：`[x]`
- 依赖：S13
- 交付物：
  - `apps/desktop/test/security-config.test.mjs`
  - 必要的 Go 架构扫描测试。
- 执行动作：
  - 校验 Rust scheduler command 全部固定 path。
  - 校验 Go handler 只通过 router 注册。
  - 校验 `go-co-op/gocron` 不出现在非 scheduler 包。
  - 校验 Provider 不直接写 scheduler 表。
- 验证：
  - `pnpm --dir apps test`
  - `cd apps/sidecar-core && go test ./...`
- 退出条件：
  - 调度能力不破坏现有安全边界。
- 当前进展：
  - 已在 `security-config.test.mjs` 中校验 scheduler Rust command 固定 path。
  - 已增加 `gocron` 使用范围扫描，限制其只出现在 `internal/service/scheduler`。
  - 已增加非 scheduler service 直接引用调度表名和调度模型的扫描。

### S26 同步验收报告和用户指南

- 状态：`[x]`
- 依赖：S21、S22、S23、S24
- 交付物：
  - `docs/2026-06-18-invest-compass-acceptance-report.md`
  - 必要时更新 `docs/2026-06-18-invest-compass-release-user-guide.md`
- 执行动作：
  - 写实记录已通过项、未执行项、剩余风险。
  - 明确 Provider 授权、频率限制和真实样例验收状态。
  - 不把未完成的 AI 自动分析或资金流专题写成已完成。
- 验证：
  - `git diff --check`
  - 文档不承诺未实现能力。
- 退出条件：
  - 调度专题具备可交付验收记录。
- 当前进展：
  - 已在 `docs/2026-06-18-invest-compass-acceptance-report.md` 新增调度专题验收记录。
  - 已在 `docs/2026-06-18-invest-compass-release-user-guide.md` 增加数据刷新调度使用边界。

### S27 后续扩展停止线审计

- 状态：`[x]`
- 依赖：S26
- 交付物：
  - 本清单的后续范围审计说明。
- 执行动作：
  - 判断 `market_snapshot_refresh` 是否具备缓存表和 UI 入口条件。
  - 判断 `fundflow_refresh` 是否已经完成数据源授权和频率验收。
  - 判断 `analysis_report_schedule` 是否可以进入独立专题。
  - 未满足条件的能力保持后续范围，不进入首批完成声明。
- 验证：
  - `git diff --check`
  - 总 checklist 和验收报告一致。
- 退出条件：
  - 首批调度专题可以收口，后续能力有明确进入条件。
- 当前进展：
  - `market_snapshot_refresh` 缺少独立缓存表和前端入口，保持后续范围。
  - `fundflow_refresh` 依赖资金流数据源授权、频率验收和首版范围确认，保持后续范围。
  - `analysis_report_schedule` 会引入 AI 成本、凭据、Prompt 和合规审核，不进入当前数据刷新调度 MVP。
  - 当前已交付范围限定为数据刷新、启动补偿、手动补偿、单股刷新和桌面管理接口。
  - 后续能力只有在完成独立数据模型、Provider 授权/频率验收、前端真实入口和合规边界复核后，才能另起专题进入实施清单。

---

## 12. 推荐验证命令

后端：

```bash
cd apps/sidecar-core && go test ./...
```

前端和 Rust：

```bash
pnpm --dir apps test
pnpm --dir apps check
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
```

文档和空白字符：

```bash
git diff --check
```

涉及新增未跟踪文档时：

```bash
git diff --no-index --check /dev/null <new-doc-path>
```

---

## 13. 实施前需要确认的操作

以下操作必须在执行前得到明确确认：

1. 新增依赖 `github.com/go-co-op/gocron/v2`。
2. 新增 SQLite 表 `scheduler_jobs`、`scheduler_runs`、`ingestion_watermarks`。
3. 新增 Go API `/api/scheduler/*`。
4. 新增 Rust scheduler command 白名单。
5. 新增设置中心任务调度页面入口。
