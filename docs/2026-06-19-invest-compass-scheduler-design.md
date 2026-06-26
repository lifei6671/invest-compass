# 投研罗盘定时任务调度方案

> 日期：2026-06-19
> 状态：设计草案
> 适用范围：Go sidecar 内部数据刷新、补偿抓取、桌面端任务管理接口

---

## 1. 背景和目标

当前项目已经具备本地 SQLite、GORM dao、行情 Provider、新闻 Provider、任务事件、报告历史、Rust 白名单 command 和设置中心基础能力。

后续需要一套适合桌面应用的定时任务能力，用于：

- 程序启动后恢复已启用的定时任务。
- 交易日和交易时段内定时抓取行情、新闻、K 线等数据。
- 程序离线、休眠或异常退出后执行有边界的补偿抓取。
- 通过桌面端管理任务启停、立即执行、查看运行记录和错误。

本方案参考 `go-stock` 的触发场景，但不继承其抓取、解析和数据库写入混在同一个函数里的实现方式。

核心原则：

```text
gocron 负责定时触发和基础并发保护。
执行队列负责业务优先级、scope 去重、取消和运行记录。
Provider 只负责抓取和清洗。
dao.Store 只负责持久化。
Rust command 只提供固定白名单接口。
```

---

## 2. 是否需要任务执行队列

需要，但首版只需要一层**进程内轻量执行队列**，不需要引入外部消息队列，也不需要实现复杂分布式调度。

原因：

1. `go-co-op/gocron/v2` 可以承载 crontab、scheduler 生命周期、全局并发上限和单 job singleton 这类基础调度能力。
2. 桌面应用还需要业务级优先级、`data_type + scope_key + period` 去重、run 落库、补偿排队和手动单股刷新复用，这些不应散落在各个 Provider 或 handler 中。
3. 行情、新闻、K 线和补偿抓取共享 Provider 和 SQLite，需要统一限制并发并把错误脱敏后记录到 `scheduler_runs`。
4. 运行记录、取消、失败恢复和桌面状态展示都需要一个统一执行入口。

首版执行队列定位：

```text
内存队列 + SQLite run 记录
```

内存队列负责当前进程内的排队和 worker 执行；SQLite 的 `scheduler_runs` 负责可观测性和崩溃恢复。sidecar 重启后不恢复内存队列本身，而是根据 `scheduler_jobs`、`scheduler_runs` 和 `ingestion_watermarks` 重新计算需要补偿的任务。

不做：

- 不引入 Redis、NATS、SQLite polling queue 等外部队列。
- 不支持多 sidecar 实例抢占同一任务。
- 不把 AI 分析任务和数据刷新任务混成同一套复杂工作流引擎。

---

## 3. 总体架构

```text
Tauri 前端
  ↓ 固定 Rust command
Rust 白名单命令
  ↓ POST /api/scheduler/*
Go actions/scheduler
  ↓
service/scheduler
  ├── go-co-op/gocron/v2 调度器
  ├── JobRegistry 任务类型注册表
  ├── ExecutionQueue 进程内执行队列
  ├── TradingCalendar 交易日和交易时段判断
  ├── CatchupPlanner 补偿计划
  └── Runner 任务执行器
        ↓
market/news/fundflow/marketinfo Provider
        ↓
dao.Store
        ↓
SQLite
```

`github.com/go-co-op/gocron/v2` 只允许出现在 `internal/service/scheduler` 内部。handler、Provider、dao 和 Rust 层不得直接依赖 gocron。

---

## 4. 模块边界

### 4.1 actions/scheduler

职责：

- 解码和校验本地 HTTP 请求。
- 调用 scheduler service。
- 返回统一 envelope。
- 复用 token、ready、POST-only 和 `httpx.DecodeJSON` 安全边界。

不做：

- 不直接注册 cron。
- 不直接调用 Provider。
- 不直接访问 GORM。

### 4.2 service/scheduler

职责：

- 初始化 gocron scheduler。
- 加载和恢复已启用任务。
- 管理任务注册、启停、立即执行、补偿执行。
- 管理执行队列和运行状态。
- 根据交易日、交易时段、水位和任务配置决定是否执行。

### 4.3 JobRegistry

职责：

- 按任务类型注册执行器。
- 校验任务参数 schema。
- 给 UI 返回可创建任务类型和默认配置。

首版内置任务类型：

```text
watchlist_quote_refresh
watchlist_kline_refresh
market_news_refresh
symbol_news_refresh
data_catchup
```

后续可扩展：

```text
market_snapshot_refresh
fundflow_refresh
analysis_report_schedule
cache_cleanup
```

### 4.4 ExecutionQueue

职责：

- 接收 cron、手动执行和补偿计划生成的 run request。
- 控制全局并发和任务类型并发。
- 防止同一 job 或同一 scope 重叠执行。
- 支持取消排队中任务和运行中任务。
- 把排队、开始、完成、失败、跳过状态写入 `scheduler_runs`。

gocron 自带的 `WithLimitConcurrentJobs` 和 `WithSingletonMode` 用于第一层保护；`ExecutionQueue` 只承载业务级排队、优先级、scope 去重、run 落库和补偿协调。

首版建议：

```text
全局 worker 数：2
单个 job 同时运行数：1
同一 data_type + scope_key 同时运行数：1
交互式手动任务优先级高于交易时段实时刷新
交易时段实时刷新优先级高于补偿任务
补偿任务优先级最低
```

### 4.5 TradingCalendar

职责：

- 判断交易日。
- 判断交易时段。
- 给补偿计划返回最近 N 个交易日。

首版只支持 CN A 股：

```text
交易时段：
09:15-11:30
13:00-15:00

收盘后窗口：
15:05 之后允许日 K、新闻和补偿任务
```

HK / US 在 Provider 未正式支持前不得伪装可用，相关任务应返回 skipped。

### 4.6 手动单股拉取

用户在股票详情页或自选股页手动触发“刷新某只股票数据”时，可以复用 `ExecutionQueue`，但必须区分两类场景：

```text
即时读取：
用户只是打开详情页、查看当前 quote 或 K 线。
这类请求继续走 market_quote / market_kline 现有 API。
API 可以按现有缓存策略读取缓存，缓存 miss 后同步调用 Provider 并写回缓存。
不强制进入调度队列，避免普通页面读取被后台任务状态复杂化。

强制刷新：
用户明确点击刷新、补齐、重新拉取、更新本地缓存。
这类请求进入 ExecutionQueue，记录 scheduler_runs。
```

手动单股拉取进入队列时使用 `manual_symbol_refresh` 语义，不需要先创建一条长期 `scheduler_jobs`。它属于一次性 run request：

```text
trigger_type: user_request
priority: interactive
scope_key: symbol，例如 CN:SH:600519
data_type: quote / kline / news / all
timeout_seconds: 默认 30
```

队列规则：

- 同一 `data_type + symbol + period` 正在执行时，不再启动第二个 Provider 请求。
- 如果已有同 scope 的运行中 run，直接返回该 run 的 `run_id` 和当前状态。
- 交互式手动任务可以排在补偿任务前面，但不能打断正在运行的 Provider 调用。
- 单次手动刷新默认只允许 1 个 symbol，批量刷新应走普通 scheduler job 或 backfill。
- 手动刷新仍必须校验 Provider status；Provider 未配置时返回明确错误，不写假数据。

这层设计让“用户点刷新”复用同一套并发控制、运行记录和错误展示，同时保留普通 quote/K 线读取路径的低延迟体验。

---

## 5. 数据库设计

新增三张表：

```text
scheduler_jobs
scheduler_runs
ingestion_watermarks
```

### 5.1 scheduler_jobs

保存桌面可管理的任务配置。

字段建议：

```text
id                  integer primary key
name                text not null
cron_type           text not null
cron_expr           text not null
enabled             boolean not null
market              text not null default 'CN'
timezone            text not null default 'Asia/Shanghai'
trade_window        text not null
scope_json          text
params_json         text
catchup_enabled     boolean not null default false
catchup_max_days    integer not null default 5
timeout_seconds     integer not null default 120
last_run_at         datetime
next_run_at         datetime
last_status         text
last_error          text
created_at          datetime not null
updated_at          datetime not null
deleted_at          datetime
```

`cron_type` 只允许 JobRegistry 注册过的任务类型。API payload、Rust command payload 和前端表单统一使用 `cron_type`，不再使用容易和语言关键字混淆的 `type` 字段。

`trade_window` 取值：

```text
trading_time
after_close
any_time
```

### 5.2 scheduler_runs

记录每次执行，供桌面查看和崩溃恢复。

字段建议：

```text
id                  integer primary key
job_id              integer not null
run_key             text not null unique
trigger_type        text not null
status              text not null
priority            integer not null default 0
source              text
target_date         text
scope_key           text
started_at          datetime
finished_at         datetime
fetched_count       integer not null default 0
written_count       integer not null default 0
skipped_reason      text
error_message       text
created_at          datetime not null
updated_at          datetime not null
```

`trigger_type` 取值：

```text
scheduled
user_request
missed_today
catchup_gap
```

`scheduled` 表示 gocron 按 scheduler job 的 cron 表达式触发。
`user_request` 表示用户对某个已存在 scheduler job 执行“立即运行”，或在业务页面触发的一次性数据刷新。
`missed_today` 表示用户当天晚于某个调度窗口启动软件时，对今天已经错过但仍有补偿价值的调度点做一次性补偿。
`catchup_gap` 表示根据抓取水位或手动 backfill 生成的历史缺口补偿。

`status` 取值：

```text
queued
running
success
failed
skipped
cancelled
```

### 5.3 ingestion_watermarks

记录每类数据的最近成功抓取边界，用于补偿。

字段建议：

```text
id                  integer primary key
data_type           text not null
scope_key           text not null
provider            text not null
period              text
last_success_at     datetime
last_trade_date     text
cursor_json         text
created_at          datetime not null
updated_at          datetime not null
```

建议唯一索引：

```text
data_type + scope_key + provider + period
```

---

## 6. 启动恢复流程

Go core 启动顺序建议：

```text
1. stdin token 握手。
2. 打开 SQLite。
3. dao.Migrate。
4. 恢复 RUNNING analysis tasks。
5. 初始化 SchedulerService。
6. 将上次异常退出遗留的 queued/running scheduler_runs 标记为 cancelled 或 failed。
7. 加载 enabled scheduler_jobs 并注册 cron entry。
8. 输出 ready JSON。
9. 启动 scheduler。
10. 执行 startup catchup 规划。
```

恢复规则：

- `running` run 标记为 `failed`，错误为 `sidecar restarted before scheduler run finished`。
- `queued` run 标记为 `cancelled`，原因是 `sidecar restarted before queued run started`。
- 不直接重放旧 run。需要补偿的缺口由 `CatchupPlanner` 根据 watermark 重新生成。
- 如果今天是交易日，且当前时间已经晚于某个 enabled job 的当日首次计划执行时间，`CatchupPlanner` 必须为该 job 生成 `missed_today` run。
- 对于交易时段内的 interval cron，例如 `*/5 9-11,13-14 * * 1-5`，如果用户 10:00 才打开软件，`CatchupPlanner` 应按当天最近一次已经到达的计划时间生成一次 `missed_today` run，而不是因为 09:30 的首个窗口已经过去就跳过整天。
- `missed_today` 只补今天启动前已经错过的调度点，不补未来调度点，也不重复补已经有 success / running / queued run 的同一 `run_key`。
- `missed_today` 必须按原计划触发时间校验 `trade_window`；例如 `after_close` 任务如果 cron 落在收盘前，即使用户收盘后启动，也不能补偿这次本来应被跳过的触发点。
- 首批 `missed_today` cron 解析只覆盖五段 cron 的数字、星号、逗号、范围和步长；不支持 `L`、`W`、`#` 等扩展语法。

启动补偿判定步骤：

```text
1. 读取所有 enabled scheduler_jobs。
2. 跳过 catchup_enabled=false 的 job。
3. 使用 TradingCalendar 判断今天是否为目标市场交易日。
4. 使用 cron 表达式和 trade_window 计算今天启动前已经到达的计划窗口。
5. 只取最近一个仍有业务价值的计划窗口，构造 trigger_type=missed_today 的 run。
6. 用 cron_type + data_type + scope_key + period + target_date + trigger_type 生成稳定 run_key。
7. 如果同一 run_key 已存在 success / running / queued run，则不重复入队。
8. 将 missed_today run 写入 scheduler_runs 并交给 ExecutionQueue 执行。
```

示例：A 股自选股行情任务配置为交易日 `09:30` 刷新，用户在当天 `10:00` 才启动软件。
启动恢复不会等待下一次 cron，也不会重放旧的进程内队列，而是生成一条当天的 `missed_today` run，用来补今天已经错过的 `09:30` 刷新窗口。
如果用户随后重启应用，且这条 run 已经处于 `queued`、`running` 或 `success`，则不会重复补偿。

---

## 7. 执行流程

统一执行链路：

```text
gocron/manual/catchup trigger
  ↓
SchedulerService.Enqueue
  ↓
ExecutionQueue 去重、限流、写 queued run
  ↓
Runner 开始执行，写 running
  ↓
校验交易日和交易时段
  ↓
校验 Provider status
  ↓
调用对应 Provider
  ↓
转换为 model.Quote / model.Kline / model.NewsItem
  ↓
dao.Store 幂等写入
  ↓
更新 watermark
  ↓
写 success / skipped / failed
  ↓
更新 job last_run_at / next_run_at / last_status / last_error
```

所有 Provider 错误进入日志和 run 前必须脱敏。

---

## 8. 补偿抓取策略

补偿来源：

```text
startup catchup
sidecar 启动后自动检查 watermark，并检查今天已错过的调度窗口。
历史 watermark 补偿只在 job params 明确给出 provider 时启用，因为水位按 data_type + scope_key + provider + period 定位。

missed-today catchup
例如 A 股 09:30 已开盘，但用户 10:00 才打开软件。
如果自选股实时行情刷新在 09:30-10:00 之间本应执行，则启动后生成一次 missed_today run。
如果任务是交易时段内每 5 分钟执行一次，则补偿启动前最近一次已经到达的执行窗口。
该 run 只用于补今天已经错过且仍有价值的刷新，不替代后续正常 cron。
missed_today 属于启动恢复补偿，不等同于历史 backfill；它只处理今天启动前已经过期的调度窗口，历史缺口仍由 watermark 驱动的 catchup_gap 负责。

manual catchup
用户在桌面端选择任务、日期和范围后立即触发。

scheduled catchup
收盘后定时补当天 K 线、新闻或市场快照。
```

硬边界：

```text
单次最多 30 个 symbol。
单次最多 30 个交易日。
默认 timeout 120 秒。
默认全局并发 2。
默认 catchup_max_days 为 5。
missed_today 默认只补当日最近一次应执行窗口。
```

超过边界直接返回 400，不进入队列。

`missed_today` 的执行优先级低于交互式手动刷新，高于历史 backfill。实时行情类任务补最近一次错过的窗口即可；K 线、新闻等幂等缓存类任务可以按同一 `run_key` 去重后补一次。

当前实现中，`ExecutionQueue` 按 `data_type + scope_key + period + target_date` 做排队去重：

- 同一股票、同一数据类型、同一交易日的重复手动刷新不会重复执行。
- 不同交易日的历史补偿 run 可以同时排队，避免 09:30 当天补偿和前一交易日 backfill 互相挤掉。
- 被去重拦截的 run 会落为 `skipped`，不会长期停留在 `queued`。

---

## 9. Go API 设计

新增 `internal/actions/scheduler`。

API 全部使用 POST：

```text
POST /api/scheduler/jobs/list
POST /api/scheduler/jobs/get
POST /api/scheduler/jobs/create
POST /api/scheduler/jobs/update
POST /api/scheduler/jobs/enable
POST /api/scheduler/jobs/delete
POST /api/scheduler/jobs/run-now
POST /api/scheduler/jobs/backfill
POST /api/scheduler/refresh-symbol
POST /api/scheduler/runs/list
POST /api/scheduler/runs/get
POST /api/scheduler/status
POST /api/scheduler/job-types
```

`delete` 使用软删除，并从 cron 移除。

`run-now` 默认仍遵守交易日和交易时段。只有请求明确携带 `ignore_trade_window=true` 时才允许跳过交易窗口，但仍必须记录 trigger 和参数。

`refresh-symbol` 用于业务页面触发单股强制刷新。它不创建长期 job，只提交一次性 run request 到 `ExecutionQueue`，并返回 `run_id`、`status` 和可选的最新缓存摘要。

当前首批已落地的 Go API 使用现有保存/启停命名：

```text
POST /api/scheduler/job-types
POST /api/scheduler/jobs/list
POST /api/scheduler/jobs/get
POST /api/scheduler/jobs/save
POST /api/scheduler/jobs/set-enabled
POST /api/scheduler/jobs/delete
POST /api/scheduler/jobs/run-now
POST /api/scheduler/jobs/backfill
POST /api/scheduler/runs/list
POST /api/scheduler/runs/get
POST /api/scheduler/runs/trigger
POST /api/scheduler/refresh-symbol
POST /api/scheduler/status
```

`refresh-symbol` 支持 `data_type=quote|kline|news|all`。`all` 会拆分为 quote、kline、news 三类一次性 run，并全部复用统一执行队列。

`jobs/backfill` 使用 `dateFrom`、`dateTo` 和 `symbols` 作为请求字段，单次最多 30 个 symbol、30 个自然日范围，并只为交易日生成 `catchup_gap` run。

---

## 10. Rust Command 白名单

新增固定 command：

```text
scheduler_jobs_list()
scheduler_jobs_get(id)
scheduler_jobs_save(payload)
scheduler_jobs_set_enabled(id, enabled)
scheduler_jobs_delete(id)
scheduler_jobs_run_now(payload)
scheduler_jobs_backfill(payload)
scheduler_refresh_symbol(payload)
scheduler_runs_list(job_id, limit)
scheduler_runs_get(id)
scheduler_status()
scheduler_job_types()
```

Rust 层职责：

- 校验 ID 必须为正数。
- 固定映射 Go API path。
- 不接受任意 path。
- 不解析 cron。
- 不直接操作本地数据库。

---

## 11. 桌面管理界面

设置中心新增“数据刷新”或“任务调度”页。

首版页面信息：

```text
任务列表：
名称、类型、启用状态、cron、下次运行、上次结果、最近错误。

任务详情：
基础信息、触发规则、交易时段、抓取范围、补偿设置。

运行记录：
每次 run 的状态、开始/结束时间、抓取条数、写入条数、失败原因。

操作：
启用/禁用、立即执行、补偿抓取、查看错误、删除。
```

UI 必须明确展示数据来源和频率限制，不得把未配置 Provider 的任务展示为可正常执行。

---

## 12. 默认任务建议

首版默认创建但不自动启用：

```text
股票基础资料刷新
cron_type: stock_profile_refresh
cron: 0 8 * * 1-5
trade_window: any_time
catchup_enabled: false

自选股实时行情刷新
cron_type: cn_a_share_quote_refresh
cron: 30 9 * * 1-5
trade_window: trading_time
catchup_enabled: false

自选股日 K 刷新
cron_type: cn_a_share_kline_refresh
cron: 30 15 * * 1-5
trade_window: after_close
catchup_enabled: true
catchup_max_days: 5

市场新闻刷新
cron_type: market_news_refresh
cron: 0 */2 * * 1-5
trade_window: any_time
catchup_enabled: false

个股新闻刷新
cron_type: symbol_news_refresh
cron: 15 */2 * * 1-5
trade_window: any_time
catchup_enabled: true
catchup_max_days: 3
```

是否启用由用户在桌面设置页决定。Provider 未完成授权、频率和真实样例验收前，生产默认不自动开启。

---

## 13. 分阶段实施

### 阶段一：调度基础设施

- 引入 `github.com/go-co-op/gocron/v2`。
- 新增 scheduler schema 和 dao 方法。
- 实现 `SchedulerService` 生命周期。
- 实现 `ExecutionQueue`、Runner 和恢复逻辑。
- 实现任务列表、启停、立即执行、运行记录 API。
- 新增 Rust command 白名单。

### 阶段二：真实数据刷新任务

- 实现 `watchlist_quote_refresh`。
- 实现 `watchlist_kline_refresh`。
- 实现 `market_news_refresh`。
- 实现 startup catchup 和 manual backfill。

### 阶段三：扩展数据任务

- 接入 `market_snapshot_refresh`。
- 接入 `fundflow_refresh`。
- 增加 run 历史清理任务。

### 阶段四：AI 自动分析任务

- 接入 `analysis_report_schedule`。
- 复用现有 analysis executor、task_events 和 report 保存链路。
- 该阶段不应与首批数据调度混在一个 PR 中实现。

---

## 14. 验证项

Go：

```bash
cd apps/sidecar-core && go test ./...
```

Rust / 前端：

```bash
pnpm --dir apps test
pnpm --dir apps check
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
```

通用：

```bash
git diff --check
```

必须覆盖的测试场景：

- cron 注册、启用、禁用和删除。
- sidecar 启动后恢复 enabled jobs。
- 遗留 queued/running run 被恢复为终态。
- 非交易时段任务返回 skipped。
- Provider unavailable 不写假数据。
- 同一 job 不重叠执行。
- catchup 根据 watermark 生成缺口。
- quote、kline、news 幂等写入。
- Rust command 固定 path。
- 非 POST、无 token、未 ready 被拒绝。

---

## 15. 需要确认的变更

实现前必须明确确认：

1. 新增依赖 `github.com/go-co-op/gocron/v2`。
2. 新增 SQLite 表 `scheduler_jobs`、`scheduler_runs`、`ingestion_watermarks`。
3. 新增 Rust command 白名单和 Tauri command 注册。
4. 新增设置中心任务调度页面入口。

这些变更涉及依赖、数据库、公共 API 和桌面能力入口，不能静默实现。
