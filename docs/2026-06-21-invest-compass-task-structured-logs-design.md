# 投研罗盘任务结构化日志技术方案

## 1. 背景

投研罗盘当前已经具备任务历史、任务事件、报告历史、日志导出、Go sidecar、SQLite、Rust 白名单 command、统一脱敏等基础能力。项目架构是 React 前端通过 Tauri/Rust 与本地 Go sidecar 通信，Go core 负责行情、新闻、AI 分析、任务、缓存和数据库，SQLite 作为本地持久化层。这个架构适合在本地实现轻量结构化日志，而不需要引入 ELK、Loki、ClickHouse 或 OpenTelemetry Collector 这类服务端日志基础设施。

本方案目标是在「任务历史」页面中实现完整日志查看器，支持：

```text
事件流
执行日志
错误诊断
上下文摘要
原始 JSON 详情
复制当前日志
导出脱敏日志
```

核心原则是：**任务事件给用户看，结构化日志给排障看，脱敏导出给反馈问题用**。

---

## 2. 设计目标

### 2.1 必须实现

```text
1. 每个长任务可查看完整结构化日志。
2. 支持按 task_id 查询日志。
3. 支持按级别、模块、阶段、关键字筛选。
4. 支持运行中任务增量拉取日志。
5. 支持失败任务错误诊断摘要。
6. 支持单条日志展开查看 JSON payload。
7. 支持复制当前可见日志。
8. 支持导出任务级脱敏日志包。
9. 写入 SQLite 前必须脱敏。
10. 导出前必须再次脱敏。
11. 前端不能直连 Go sidecar，只能通过 Rust 白名单 command 调用。
```

### 2.2 不做

```text
1. 不引入远程日志服务。
2. 不上传用户日志。
3. 不做公网 /metrics。
4. 不在前端保存日志原始文件。
5. 不在日志中保存真实 API Key、Authorization、代理密码、用户持仓明细。
6. 不把完整 AI 流式输出重复写入结构化日志。
7. 不把 task_events 和 task_log_entries 混成一张表。
```

---

## 3. 总体架构

```text
React 任务日志抽屉
  ↓ typed invoke service
Tauri / Rust 白名单 command
  ↓ POST /api/tasks/logs/*
Go actions/tasklog
  ↓
service/tasklog
  ├── TaskLogService
  ├── TaskLogWriter
  ├── TaskLogQueryService
  ├── TaskErrorDiagnosisService
  └── TaskLogExportService
        ↓
dao.Store
  ├── task_log_entries
  ├── task_error_diagnoses
  └── task_events / tasks
        ↓
SQLite 本地库

Go slog
  ↓ TaskLogHandler
内存队列
  ↓ batch writer
SQLite + NDJSON 文件日志
```

现有项目已经要求 Rust command 白名单化，禁止任意 path 代理；日志导出也已经要求 Go core 只生成二次脱敏包，由 Rust 写入用户选择目录。结构化任务日志需要沿用这个边界。

---

## 4. 核心概念

## 4.1 task_events：任务事件

任务事件用于展示任务生命周期和用户可理解的进度。

已有事件类型继续保留：

```text
TASK_CREATED
TASK_STARTED
TASK_PROGRESS
TASK_LOG
TASK_CHUNK
TASK_SUCCESS
TASK_FAILED
TASK_CANCELLED
```

用途：

```text
1. 任务历史右侧事件流。
2. AI 分析页进度展示。
3. SSE 断线补拉。
4. sidecar 重启后任务状态恢复。
```

任务事件属于业务状态，不应承载过细的技术日志。现有实施清单已经要求 `task_events` 可按 id 递增回放，并且写入前必须脱敏。

---

## 4.2 task_log_entries：结构化任务日志

结构化日志用于排障，粒度比任务事件更细。

示例：

```text
15:28:40.123 INFO  market    quote_fetch     开始拉取行情
15:28:40.486 INFO  market    quote_fetch     行情缓存命中
15:28:41.022 INFO  kline     kline_fetch     拉取日K线 120 条
15:28:48.211 INFO  ai        prompt_build    Prompt 构建完成
15:28:53.941 WARN  ai        stream_timeout  模型流式响应耗时过长
15:29:46.120 ERROR ai        stream_failed   Provider 响应超时，任务终止
```

用途：

```text
1. 完整日志抽屉的“执行日志”Tab。
2. 失败任务排障。
3. task_id / request_id / trace_id 关联查询。
4. 用户导出脱敏日志包。
```

---

## 4.3 NDJSON 文件日志

SQLite 负责 UI 查询，NDJSON 文件负责完整本地排障和导出来源。

路径建议：

```text
<workspace>/logs/app-2025-05-20.ndjson
<workspace>/logs/app-2025-05-21.ndjson
```

一行一条 JSON：

```json
{"ts":"2025-05-20T15:29:46.120+08:00","level":"ERROR","task_id":"task_20250520_152834_abc123","module":"ai","stage":"stream_failed","message":"Provider 响应超时","trace_id":"trace_91aa..."}
```

首版可以先只实现 SQLite 结构化日志；NDJSON 放第二阶段。

---

## 5. 数据库设计

## 5.1 新增表：task_log_entries

```sql
CREATE TABLE task_log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    task_id TEXT NOT NULL,
    request_id TEXT,
    trace_id TEXT,

    ts DATETIME NOT NULL,
    level TEXT NOT NULL,
    module TEXT NOT NULL,
    stage TEXT NOT NULL,
    message TEXT NOT NULL,

    code TEXT,
    provider TEXT,
    model TEXT,
    symbol TEXT,

    duration_ms INTEGER,
    retryable BOOLEAN NOT NULL DEFAULT 0,

    payload_json TEXT,

    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_task_log_entries_task_id_id
ON task_log_entries(task_id, id);

CREATE INDEX idx_task_log_entries_task_id_level
ON task_log_entries(task_id, level);

CREATE INDEX idx_task_log_entries_task_id_module_stage
ON task_log_entries(task_id, module, stage);

CREATE INDEX idx_task_log_entries_request_id
ON task_log_entries(request_id);

CREATE INDEX idx_task_log_entries_trace_id
ON task_log_entries(trace_id);

CREATE INDEX idx_task_log_entries_ts
ON task_log_entries(ts);
```

### 字段说明

| 字段             | 说明                                                                             |
| -------------- | ------------------------------------------------------------------------------ |
| `task_id`      | 关联任务                                                                           |
| `request_id`   | Rust 转发请求 ID                                                                   |
| `trace_id`     | 链路追踪 ID                                                                        |
| `ts`           | 日志发生时间                                                                         |
| `level`        | `DEBUG / INFO / WARN / ERROR`                                                  |
| `module`       | `market / kline / indicator / news / prompt / ai / report / scheduler / cache` |
| `stage`        | 业务阶段，如 `quote_fetch`、`prompt_build`                                            |
| `message`      | 脱敏后的短消息                                                                        |
| `code`         | 稳定错误码                                                                          |
| `provider`     | 数据源或模型 Provider                                                                |
| `model`        | AI 模型                                                                          |
| `symbol`       | 股票代码                                                                           |
| `duration_ms`  | 阶段耗时                                                                           |
| `retryable`    | 是否建议重试                                                                         |
| `payload_json` | 脱敏 JSON 详情                                                                     |

---

## 5.2 新增表：task_error_diagnoses

用于失败任务的错误诊断 Tab，避免前端从日志里硬解析。

```sql
CREATE TABLE task_error_diagnoses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    task_id TEXT NOT NULL UNIQUE,
    error_code TEXT NOT NULL,
    error_stage TEXT NOT NULL,
    summary TEXT NOT NULL,
    suggestion TEXT,
    retryable BOOLEAN NOT NULL DEFAULT 0,

    request_id TEXT,
    trace_id TEXT,
    source_log_id INTEGER,

    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_task_error_diagnoses_task_id
ON task_error_diagnoses(task_id);
```

示例：

```json
{
  "task_id": "task_20250520_152834_abc123",
  "error_code": "AI_PROVIDER_TIMEOUT",
  "error_stage": "ai.stream_read",
  "summary": "模型服务响应超时",
  "suggestion": "检查代理、API Key、模型服务状态，或更换模型后重试",
  "retryable": true,
  "trace_id": "trace_91aa..."
}
```

---

## 5.3 后续可选：FTS 全文索引

第一版先不做。日志量变大后再加：

```sql
CREATE VIRTUAL TABLE task_log_entries_fts
USING fts5(message, payload_json, content='task_log_entries', content_rowid='id');
```

触发条件：

```text
1. 单任务日志超过 5000 条。
2. LIKE 查询明显卡顿。
3. 用户开始频繁搜索 payload_json。
```

---

## 6. Go 模块设计

新增模块：

```text
apps/sidecar-core/internal/service/tasklog
apps/sidecar-core/internal/actions/tasklog
apps/sidecar-core/internal/model/task_log.go
apps/sidecar-core/internal/dao/task_log_repository.go
```

扩展已有模块：

```text
apps/sidecar-core/pkg/logger
apps/sidecar-core/internal/service/logexport
apps/sidecar-core/internal/actions/router.go
apps/desktop/src-tauri/src/commands/task_logs.rs
apps/frontend/src/services/taskLogs.ts
```

---

## 6.1 model 层

```go
type TaskLogEntry struct {
    ID         int64     `gorm:"primaryKey"`
    TaskID     string    `gorm:"index;not null"`
    RequestID  string    `gorm:"index"`
    TraceID    string    `gorm:"index"`

    TS         time.Time `gorm:"index;not null"`
    Level      string    `gorm:"not null"`
    Module     string    `gorm:"not null"`
    Stage      string    `gorm:"not null"`
    Message    string    `gorm:"not null"`

    Code       string
    Provider   string
    Model      string
    Symbol     string

    DurationMS int64
    Retryable  bool

    PayloadJSON string

    CreatedAt time.Time
    UpdatedAt time.Time
}

type TaskErrorDiagnosis struct {
    ID          int64  `gorm:"primaryKey"`
    TaskID      string `gorm:"uniqueIndex;not null"`
    ErrorCode   string `gorm:"not null"`
    ErrorStage  string `gorm:"not null"`
    Summary     string `gorm:"not null"`
    Suggestion  string
    Retryable   bool

    RequestID   string
    TraceID     string
    SourceLogID int64

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## 6.2 dao 层

新增接口：

```go
type TaskLogQuery struct {
    TaskID    string
    AfterID   int64
    Limit     int
    Level     string
    Module    string
    Stage     string
    Keyword   string
    OnlyError bool
}

type TaskLogStore interface {
    AppendTaskLogs(ctx context.Context, entries []model.TaskLogEntry) error
    ListTaskLogs(ctx context.Context, query TaskLogQuery) ([]model.TaskLogEntry, bool, error)
    GetTaskLog(ctx context.Context, id int64) (*model.TaskLogEntry, error)

    UpsertTaskErrorDiagnosis(ctx context.Context, diagnosis model.TaskErrorDiagnosis) error
    GetTaskErrorDiagnosis(ctx context.Context, taskID string) (*model.TaskErrorDiagnosis, error)

    DeleteTaskLogsBefore(ctx context.Context, before time.Time) error
}
```

要求：

```text
1. 所有写入必须通过 dao.Store。
2. 禁止 service 或 action 层直接使用 GORM 句柄。
3. AppendTaskLogs 必须支持批量写入。
4. ListTaskLogs 默认 limit 不超过 200。
5. payload_json 在 DAO 前已经脱敏，DAO 不负责业务脱敏。
```

现有 checklist 已经对 DAO 边界提出要求：数据库访问入口收敛到 `internal/dao`，业务层不能绕过 GORM dao。新增任务日志也应该遵守这个边界。

---

## 6.3 logger 层：TaskLogHandler

在 `pkg/logger` 中新增 `slog.Handler` 包装器。

```go
type TaskLogHandler struct {
    next   slog.Handler
    writer *TaskLogAsyncWriter
}

func (h *TaskLogHandler) Handle(ctx context.Context, record slog.Record) error {
    _ = h.next.Handle(ctx, record)

    taskID := TaskIDFromContext(ctx)
    if taskID == "" {
        return nil
    }

    entry, ok := BuildTaskLogEntry(ctx, record)
    if !ok {
        return nil
    }

    entry = RedactTaskLogEntry(entry)

    h.writer.TryAppend(entry)

    return nil
}
```

### 关键要求

```text
1. 没有 task_id 的普通应用日志不写 task_log_entries。
2. TaskLogHandler 不同步写 SQLite。
3. 先进入 channel，再由 batch worker 批量写。
4. channel 满时可以丢弃 DEBUG / INFO。
5. WARN / ERROR 尽量不丢，必要时单独保留高优先级队列。
6. 所有 entry 写入前必须 Redact。
```

---

## 6.4 异步批量写入

```go
type TaskLogAsyncWriter struct {
    ch       chan model.TaskLogEntry
    store    TaskLogStore
    interval time.Duration
    batchMax int
}

func (w *TaskLogAsyncWriter) Run(ctx context.Context) {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    batch := make([]model.TaskLogEntry, 0, w.batchMax)

    flush := func() {
        if len(batch) == 0 {
            return
        }

        entries := batch
        batch = make([]model.TaskLogEntry, 0, w.batchMax)

        _ = w.store.AppendTaskLogs(ctx, entries)
    }

    for {
        select {
        case <-ctx.Done():
            flush()
            return

        case entry := <-w.ch:
            batch = append(batch, entry)
            if len(batch) >= w.batchMax {
                flush()
            }

        case <-ticker.C:
            flush()
        }
    }
}
```

建议参数：

```text
channel size：4096
batch max：100
flush interval：100ms
SQLite busy_timeout：5000ms
SQLite journal_mode：WAL
```

---

## 7. 任务上下文传播

任务执行入口统一注入上下文：

```go
type RuntimeTaskContext struct {
    TaskID    string
    RequestID string
    TraceID   string
    Symbol    string
    Provider  string
    Model     string
}

func WithRuntimeTaskContext(ctx context.Context, tc RuntimeTaskContext) context.Context {
    ctx = context.WithValue(ctx, taskIDKey{}, tc.TaskID)
    ctx = context.WithValue(ctx, requestIDKey{}, tc.RequestID)
    ctx = context.WithValue(ctx, traceIDKey{}, tc.TraceID)
    ctx = context.WithValue(ctx, symbolKey{}, tc.Symbol)
    ctx = context.WithValue(ctx, providerKey{}, tc.Provider)
    ctx = context.WithValue(ctx, modelKey{}, tc.Model)
    return ctx
}
```

分析任务启动时：

```go
ctx = logger.WithRuntimeTaskContext(ctx, logger.RuntimeTaskContext{
    TaskID:    task.ID,
    RequestID: requestID,
    TraceID:   traceID,
    Symbol:    req.Symbol,
    Provider:  aiConfig.Provider,
    Model:     aiConfig.ModelName,
})
```

业务代码只负责写结构化字段：

```go
logger.InfoContext(ctx, "开始拉取行情",
    slog.String("module", "market"),
    slog.String("stage", "quote_fetch"),
    slog.String("symbol", symbol),
)
```

`TaskLogHandler` 自动补齐 `task_id / request_id / trace_id / provider / model`。

---

## 8. 阶段日志 helper

新增 `RunStage`，统一记录阶段开始、完成、失败和耗时。

```go
func RunStage(
    ctx context.Context,
    module string,
    stage string,
    fn func(ctx context.Context) error,
) error {
    start := time.Now()

    logger.InfoContext(ctx, "阶段开始",
        slog.String("module", module),
        slog.String("stage", stage),
    )

    err := fn(ctx)
    duration := time.Since(start).Milliseconds()

    if err != nil {
        logger.ErrorContext(ctx, "阶段失败",
            slog.String("module", module),
            slog.String("stage", stage),
            slog.Int64("duration_ms", duration),
            slog.String("error", err.Error()),
        )
        return err
    }

    logger.InfoContext(ctx, "阶段完成",
        slog.String("module", module),
        slog.String("stage", stage),
        slog.Int64("duration_ms", duration),
    )

    return nil
}
```

AI 分析主链路：

```go
err := tasklog.RunStage(ctx, "market", "quote_fetch", func(ctx context.Context) error {
    return s.loadQuote(ctx, symbol)
})

err = tasklog.RunStage(ctx, "kline", "kline_fetch", func(ctx context.Context) error {
    return s.loadKline(ctx, symbol)
})

err = tasklog.RunStage(ctx, "indicator", "calc_indicators", func(ctx context.Context) error {
    return s.calcIndicators(ctx, symbol)
})

err = tasklog.RunStage(ctx, "prompt", "prompt_build", func(ctx context.Context) error {
    return s.buildPrompt(ctx, req)
})

err = tasklog.RunStage(ctx, "ai", "stream_chat", func(ctx context.Context) error {
    return s.streamAI(ctx, req)
})

err = tasklog.RunStage(ctx, "report", "report_save", func(ctx context.Context) error {
    return s.saveReport(ctx, taskID)
})
```

---

## 9. 日志字段规范

### level

```text
DEBUG
INFO
WARN
ERROR
```

首版 UI 默认展示：

```text
INFO / WARN / ERROR
```

DEBUG 可通过开发模式打开。

### module

```text
market
kline
indicator
news
prompt
ai
report
scheduler
cache
settings
provider
task
```

### stage

```text
quote_fetch
quote_cache_hit
quote_cache_miss
kline_fetch
kline_cache_hit
news_fetch
calc_ma
calc_macd
calc_rsi
prompt_build
ai_config_load
stream_start
stream_chunk
stream_timeout
stream_failed
report_save
task_cancel
task_recover
```

阶段名必须作为跨端稳定枚举使用，Go、Rust、TypeScript 三端保持完全一致。

首版 UI 已使用 `stream_timeout` 和 `stream_failed`，因此不再使用
`ai_stream_timeout`、`ai_stream_failed` 这类带模块前缀的 stage 名称。
模块归属统一放在 `module` 字段中表达，例如：

```json
{
  "module": "ai",
  "stage": "stream_failed"
}
```

禁止在不同链路中混用 `stream_failed` 和 `ai_stream_failed`，否则会导致
执行日志 Tab 的阶段筛选无法命中后端数据。

### code

```text
MARKET_PROVIDER_UNAVAILABLE
KLINE_PROVIDER_TIMEOUT
NEWS_PROVIDER_ERROR
AI_CONFIG_NOT_FOUND
AI_PROVIDER_UNAUTHORIZED
AI_PROVIDER_RATE_LIMITED
AI_PROVIDER_TIMEOUT
AI_STREAM_FAILED
PROMPT_TEMPLATE_INVALID
REPORT_SAVE_FAILED
TASK_CANCELLED
```

---

## 10. 脱敏设计

项目现有方案已经要求日志字段统一包含 `request_id`、`trace_id`、`task_id`、`provider`、`symbol`，并要求 secret redaction 覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户持仓输入。结构化任务日志必须复用同一套脱敏能力，不能另起一套规则。

### 10.1 脱敏时机

```text
1. slog Record 转 task_log_entry 时脱敏。
2. payload_json 写入 SQLite 前脱敏。
3. NDJSON 写入文件前脱敏。
4. Go API 返回前再次脱敏。
5. 日志导出前再次脱敏。
```

### 10.2 敏感字段

```text
api_key
raw_api_key
resolved_api_key
authorization
proxy_authorization
proxy_password
token
runtime_token
license_key
user_position
cost_price
shares
position
holding
```

### 10.3 AI chunk 处理

`TASK_CHUNK` 可以保存 AI 输出片段，用于恢复流式显示。

但 `task_log_entries.payload_json` 不保存完整 AI 输出，只保存摘要：

```json
{
  "chunk_index": 12,
  "chunk_size": 358,
  "total_chars": 4201,
  "latency_ms": 812
}
```

避免同一份 AI 输出在任务事件、报告、结构化日志中重复膨胀。

---

## 11. Go API 设计

所有接口仅供 Rust command 调用，前端不能直接访问 Go sidecar。现有方案已经要求所有 Go API 由 Rust 白名单 command 固定映射，且 Go core 只监听 `127.0.0.1`，请求必须带 token。

## 11.1 查询任务日志

```http
POST /api/tasks/logs/list
```

请求：

```json
{
  "task_id": "task_20250520_152834_abc123",
  "after_id": 0,
  "limit": 200,
  "level": "ERROR",
  "module": "ai",
  "stage": "stream_failed",
  "keyword": "超时",
  "only_error": true
}
```

响应：

```json
{
  "items": [
    {
      "id": 1024,
      "timestamp": "2025-05-20T15:29:46.120+08:00",
      "level": "ERROR",
      "module": "ai",
      "stage": "stream_failed",
      "message": "Provider 响应超时，任务终止",
      "code": "AI_PROVIDER_TIMEOUT",
      "provider": "deepseek",
      "model": "DeepSeek-V3",
      "symbol": "CN:SH:600183",
      "duration_ms": 120000,
      "retryable": true,
      "request_id": "req_7d29...",
      "trace_id": "trace_91aa..."
    }
  ],
  "next_after_id": 1024,
  "has_more": false
}
```

校验规则：

```text
task_id：不能为空
after_id：>= 0
limit：1-500，默认 200
level：空值或 DEBUG/INFO/WARN/ERROR
keyword：最长 100 字符
only_error：true 时忽略 level 或强制 level in WARN/ERROR
```

---

## 11.2 获取单条日志详情

```http
POST /api/tasks/logs/get
```

请求：

```json
{
  "id": 1024
}
```

响应：

```json
{
  "id": 1024,
  "task_id": "task_20250520_152834_abc123",
  "timestamp": "2025-05-20T15:29:46.120+08:00",
  "level": "ERROR",
  "module": "ai",
  "stage": "stream_failed",
  "message": "Provider 响应超时，任务终止",
  "payload_json": {
    "provider": "deepseek",
    "model": "DeepSeek-V3",
    "timeout_seconds": 120,
    "request_id": "req_7d29...",
    "trace_id": "trace_91aa..."
  }
}
```

校验规则：

```text
id > 0
返回前再次执行脱敏
```

---

## 11.3 获取错误诊断

```http
POST /api/tasks/logs/diagnosis
```

请求：

```json
{
  "task_id": "task_20250520_152834_abc123"
}
```

响应：

```json
{
  "task_id": "task_20250520_152834_abc123",
  "error_code": "AI_PROVIDER_TIMEOUT",
  "error_stage": "ai.stream_chat",
  "summary": "模型服务响应超时",
  "suggestion": "检查代理、API Key、模型服务状态，或更换模型后重试",
  "causes": [
    "模型服务响应超时",
    "代理配置异常或网络延迟过高",
    "API Key 权限、额度或模型不可用",
    "输出内容过长，超过模型响应时间"
  ],
  "suggestions": [
    "检查代理设置并重新测试连接",
    "更换 AI 模型后重试",
    "降低最大输出 Token",
    "稍后重新发起分析任务"
  ],
  "retryable": true,
  "request_id": "req_7d29...",
  "trace_id": "trace_91aa..."
}
```

字段说明：

```text
summary      用于执行日志 Tab 顶部的红色错误摘要提示条。
suggestion   用于兼容单句建议展示和导出摘要。
causes       用于“错误诊断”Tab 的“可能原因”列表。
suggestions  用于“错误诊断”Tab 的“建议处理”列表。
retryable    用于决定后续是否展示“重新分析 / 重试”动作。
```

---

## 11.4 获取上下文摘要

```http
POST /api/tasks/logs/context
```

请求：

```json
{
  "task_id": "task_20250520_152834_abc123"
}
```

响应：

```json
{
  "task_id": "task_20250520_152834_abc123",
  "stock": "生益科技 CN:SH:600183",
  "analysis_type": "个股综合分析",
  "model": "DeepSeek-V3",
  "prompt_template": "默认个股分析模板",
  "quote_status": "已加载",
  "kline_summary": "120 条",
  "indicators": "MA / MACD / RSI / KDJ / BOLL",
  "news_summary": "36 条",
  "user_position_status": "未提供",
  "data_updated_at": "2025-05-20 15:30:00"
}
```

上下文摘要只返回用于排障展示的短摘要。

禁止返回：

```text
1. 完整 Prompt。
2. 完整 input_snapshot。
3. API Key、Authorization、Proxy-Authorization、代理密码。
4. 用户完整持仓明细、成本价、股数等隐私输入。
```

---

## 11.5 导出任务日志

```http
POST /api/tasks/logs/export
```

请求：

```json
{
  "task_id": "task_20250520_152834_abc123"
}
```

Go core 返回脱敏包内容和建议文件名，不直接写用户目录：

```json
{
  "file_name": "invest-compass-task-log-task_20250520_152834_abc123-20250520.zip",
  "content_base64": "..."
}
```

Rust command 负责写入用户选择目录，并复用现有日志导出边界：

```text
1. target_dir 必须存在。
2. 拒绝路径穿越文件名。
3. 拒绝覆盖同名文件。
4. Unix/macOS 下文件权限 0600。
```

现有发布指南也要求日志导出包必须已二次脱敏，且不要在 issue、截图、日志或聊天中发送真实 API Key、Token、代理密码或持仓明细。

---

## 12. Rust command 设计

新增固定白名单 command：

```text
task_logs_list(payload)
task_log_get(id)
task_log_diagnosis(task_id)
task_log_context(task_id)
task_logs_export(task_id, target_dir)
```

固定映射：

```text
task_logs_list(payload)       -> POST /api/tasks/logs/list
task_log_get(id)              -> POST /api/tasks/logs/get
task_log_diagnosis(task_id)   -> POST /api/tasks/logs/diagnosis
task_log_context(task_id)     -> POST /api/tasks/logs/context
task_logs_export(task_id, target_dir) -> POST /api/tasks/logs/export
```

Rust 参数校验：

```text
task_id 不能为空
id 必须 > 0
limit 必须 1-500
after_id 必须 >= 0
target_dir 必须存在且是目录
target_dir 非法时不得触发 Go core export API
```

导出交互边界：

```text
1. 前端点击“导出脱敏日志”后，必须先通过受控的桌面目录选择能力获取目标目录。
2. 前端不得自行拼接导出文件名。
3. Rust command 调用 Go export API 获取 file_name 和 content_base64。
4. Rust 负责校验 target_dir、拒绝路径穿越、拒绝覆盖同名文件，并写入最终 zip。
5. Go core 不直接写用户目录。
```

不新增：

```text
core_request(method, path, body)
task_log_request(path, body)
```

---

## 13. 前端设计

## 13.1 页面结构

在任务历史页新增完整日志抽屉：

```text
TaskHistoryPage
  ├── TaskTable
  ├── TaskDetailPanel
  └── TaskLogDrawer
        ├── TaskLogSummaryCard
        ├── TaskLogTabs
        │    ├── TaskEventTimeline
        │    ├── TaskExecutionLogTable
        │    ├── TaskErrorDiagnosisPanel
        │    └── TaskContextSummaryPanel
        ├── RawJsonDetailPanel
        └── TaskLogActionBar
```

命名约定：

```text
组件名统一使用 TaskLogDrawer。
文件名统一使用 TaskLogDrawer.tsx。
测试名统一使用 TaskLogDrawer ...
```

不要再新增 `TaskFullLogDrawer`，避免后续 mock UI 和真实接入出现两套抽屉。

---

## 13.2 数据加载

打开日志抽屉时并行请求：

```text
task_get(task_id)                  → 顶部任务概要
task_events(task_id, 0)            → 事件流
task_logs_list({ task_id })        → 执行日志
task_log_diagnosis(task_id)        → 错误诊断
task_log_context(task_id)          → 上下文摘要
```

前端可以在 service 层封装一个聚合函数，用多个 Rust 白名单 command 组合出
抽屉需要的数据，但不要新增任意 path 代理：

```ts
async function loadTaskLogDrawerData(taskId: string): Promise<TaskLogDrawerData> {
  const [summary, events, logs, diagnosis, contextSummary] = await Promise.all([
    taskGet(taskId),
    taskEventsList(taskId, 0),
    taskLogsList({ taskId, limit: 200 }),
    taskLogDiagnosis(taskId),
    taskLogContext(taskId),
  ]);

  return mapTaskLogDrawerData(summary, events, logs, diagnosis, contextSummary);
}
```

运行中任务增量拉取：

```ts
const timer = window.setInterval(async () => {
  const result = await taskLogsList({
    task_id: taskId,
    after_id: lastLogId,
    limit: 200,
    ...filters,
  });

  appendLogs(result.items);
  setLastLogId(result.next_after_id);

  if (isTerminalTaskStatus(task.status)) {
    clearInterval(timer);
  }
}, 1000);
```

第一版用轮询，不做日志 SSE。原因：

```text
1. 实现简单。
2. 与 task_events 的 SSE 转发解耦。
3. 日志量大时前端可控。
4. 任务终态后自动停止。
```

---

## 13.3 前端 DTO 契约

真实接口接入时，前端抽屉统一消费以下 DTO。Rust command 的返回值可以
保持 snake_case，但进入 React 页面前必须在 typed service 中转换为 camelCase。

```ts
type TaskStatus = "RUNNING" | "SUCCESS" | "FAILED" | "CANCELLED";

type TaskLogLevel = "INFO" | "WARN" | "ERROR";

type TaskLogDrawerData = {
  summary: TaskLogSummary;
  events: TaskLogEvent[];
  logs: TaskLogRecord[];
  diagnosis?: TaskLogDiagnosis;
  contextSummary?: TaskLogContextSummary;
};

type TaskLogSummary = {
  title: string;
  taskId: string;
  taskType: string;
  stock?: string;
  model?: string;
  startedAt: string;
  duration: string;
  requestId: string;
  traceId: string;
  status: TaskStatus;
};

type TaskLogRecord = {
  id: string;
  time: string;
  timestamp: string;
  level: TaskLogLevel;
  module: string;
  stage: string;
  message: string;
  code?: string;
  provider?: string;
  model?: string;
  symbol?: string;
  durationMs?: number;
  retryable?: boolean;
};

type TaskLogEvent = {
  id: string;
  time: string;
  eventType:
    | "TASK_CREATED"
    | "TASK_STARTED"
    | "TASK_PROGRESS"
    | "TASK_CHUNK"
    | "TASK_SUCCESS"
    | "TASK_FAILED"
    | "TASK_CANCELLED";
  description: string;
};

type TaskLogDiagnosis = {
  errorCode: string;
  errorStage: string;
  summary: string;
  suggestion: string;
  causes: string[];
  suggestions: string[];
  retryable: boolean;
  requestId?: string;
  traceId?: string;
};

type TaskLogContextSummary = {
  stock: string;
  analysisType: string;
  model: string;
  promptTemplate: string;
  quoteStatus: string;
  klineSummary: string;
  indicators: string;
  newsSummary: string;
  userPositionStatus: string;
  dataUpdatedAt: string;
};
```

字段对接关系：

```text
TaskLogSummary      → 抽屉头部和任务基础信息卡。
TaskLogEvent[]      → “事件流”Tab。
TaskLogRecord[]     → “执行日志”表格。
TaskLogDiagnosis    → FAILED 错误摘要提示条和“错误诊断”Tab。
TaskLogContextSummary → “上下文摘要”Tab。
```

---

## 13.4 表格性能

```text
默认加载最近 200 条
点击“加载更多”再取 200 条
运行中自动追加
切换筛选条件时重新查询
超过 1000 条启用虚拟滚动
```

Ant Design Table 可以先使用分页；日志量变大后换成虚拟列表。

---

## 13.5 筛选控件

```text
搜索日志关键字
全部级别
全部阶段
仅看错误
自动滚动
暂停滚动
```

首版抽屉不展示“模块”下拉框。关键字搜索需要同时匹配
`message / module / stage`，因此用户仍然可以通过输入模块名定位日志。

后端 API 仍保留 `module` 查询参数，供后续增加模块筛选或导出过滤使用。

筛选逻辑由后端执行。前端只在 mock 阶段允许本地过滤，真实接入后切换筛选条件
必须重新调用 `task_logs_list`。

---

## 13.6 原始 JSON 详情

单击日志行后调用：

```text
task_log_get(id)
```

展开：

```json
{
  "timestamp": "2025-05-20T15:29:46.120+08:00",
  "level": "ERROR",
  "module": "ai",
  "stage": "stream_failed",
  "message": "Provider 响应超时，任务终止",
  "request_id": "req_7d29...",
  "trace_id": "trace_91aa..."
}
```

前端只展示 Go core 返回的脱敏 JSON，不做自行脱敏作为唯一防线。

交互规则：

```text
1. 打开抽屉后，如果任务失败，默认选中最近一条 ERROR 日志。
2. 如果没有 ERROR 日志，默认选中第一条日志。
3. 点击日志表格行时更新 selectedLogId。
4. selectedLogId 变化后调用 task_log_get(id)。
5. 加载中展示 JSON 面板 skeleton 或“加载中”文案。
6. payload_json 为空时展示“暂无 JSON 详情”。
7. JSON 面板中的复制按钮只复制当前选中日志的脱敏 JSON。
```

---

## 13.7 导出脱敏日志交互

抽屉底部“导出脱敏日志”按钮必须走固定流程：

```text
1. 用户点击按钮。
2. 前端触发受控目录选择能力。
3. 用户选择目录后调用 task_logs_export(task_id, target_dir)。
4. Rust 校验目录和最终文件写入规则。
5. Go core 只返回脱敏 zip 内容，不接触用户目录。
6. 成功后提示“脱敏日志已导出”。
```

取消目录选择时只关闭流程，不调用 Go core。

导出失败时必须展示错误提示，但不得把 API Key、代理密码、Authorization、
Proxy-Authorization 或完整用户持仓输入带到错误信息中。

---

## 14. 错误诊断生成

失败任务终态时生成 `task_error_diagnoses`。

生成来源优先级：

```text
1. 明确业务错误码 xerr.Code。
2. 最后一条 ERROR 级别 task_log_entries。
3. TASK_FAILED 事件 payload。
4. fallback：任务 error_message。
```

错误码映射：

```go
var diagnosisRules = map[string]DiagnosisRule{
    "AI_PROVIDER_TIMEOUT": {
        Summary:    "模型服务响应超时",
        Suggestion: "检查代理、API Key、模型服务状态，或更换模型后重试",
        Retryable:  true,
    },
    "AI_PROVIDER_UNAUTHORIZED": {
        Summary:    "模型服务认证失败",
        Suggestion: "检查 API Key 是否有效，或重新保存模型配置",
        Retryable:  true,
    },
    "MARKET_PROVIDER_UNAVAILABLE": {
        Summary:    "行情数据源不可用",
        Suggestion: "检查数据源状态、网络代理或稍后重试",
        Retryable:  true,
    },
}
```

---

## 15. 日志容量控制

## 15.1 SQLite 日志保留

默认策略：

```text
保留最近 30 天 task_log_entries
单任务最多保留 5000 条结构化日志
全库 task_log_entries 最大 50 万条
超过后删除最旧数据
```

可配置项：

```text
logs.task_retention_days = 30
logs.max_entries_per_task = 5000
logs.max_total_entries = 500000
logs.debug_enabled = false
```

## 15.2 清理任务

新增缓存清理范围：

```text
task_logs
app_logs
```

但清理时必须提示：

```text
清理任务日志不会删除任务记录、任务事件和分析报告。
```

不要让缓存清理删除：

```text
tasks
task_events
analysis_reports
ai_configs
settings
```

---

## 16. NDJSON 文件日志设计

第二阶段实现。

路径：

```text
<workspace>/logs/app-YYYY-MM-DD.ndjson
```

规则：

```text
1. 按天切分。
2. 单文件超过 20MB 后滚动为 app-YYYY-MM-DD.1.ndjson。
3. 总大小超过 500MB 删除最旧文件。
4. 文件写入前必须脱敏。
5. 导出任务日志时可合并 SQLite task_log_entries 和 NDJSON 中对应 task_id 内容。
```

---

## 17. 安全边界

必须满足：

```text
1. 前端不能拿到 Go core port/token。
2. 前端不能直连 127.0.0.1。
3. 所有日志查询通过 Rust command。
4. 所有 Go API 只接受 POST。
5. Go API 必须校验 X-Invest-Compass-Token。
6. task_logs_export 不能让 Go core 写用户目录。
7. Rust 写文件必须校验目录存在、拒绝路径穿越、拒绝覆盖。
8. API Key、代理密码、Authorization、Proxy-Authorization、用户持仓输入不得写入日志。
9. 日志导出前必须二次脱敏。
10. 完整 input_snapshot 默认不进入日志 UI。
```

项目当前安全设计已经明确：前端所有请求通过 Tauri command 白名单代理，Go sidecar 只监听 `127.0.0.1`，token 只存在 Tauri 内存中，不持久化；可观测字段需要保留 request_id、trace_id、task_id、provider、symbol，同时不记录密钥和完整请求头。结构化日志必须完全沿用该边界。

---

## 18. 实施步骤

### 阶段一：MVP

交付目标：UI 图中的执行日志 Tab 可用。

```text
L01 新增 task_log_entries GORM model 和 migration
L02 新增 dao.Store.AppendTaskLogs / ListTaskLogs / GetTaskLog
L03 新增 service/tasklog 查询和写入模型
L04 新增 pkg/logger.TaskLogHandler
L05 新增 TaskLogAsyncWriter 批量写 SQLite
L06 分析任务关键阶段接入 RunStage
L07 新增 Go API：/api/tasks/logs/list、/api/tasks/logs/get
L08 新增 Rust command：task_logs_list、task_log_get
L09 新增前端 TaskLogDrawer
L10 执行日志 Tab 支持筛选、分页、自动滚动
```

验收：

```text
1. AI 分析任务执行后可看到结构化日志。
2. FAILED 任务可看到 ERROR 日志。
3. task_id / request_id / trace_id 能对应上。
4. API Key、Authorization、代理密码不会进入 SQLite。
5. 运行中任务可每秒增量追加日志。
```

---

### 阶段二：错误诊断和导出

```text
L11 新增 task_error_diagnoses 表
L12 新增 TaskErrorDiagnosisService
L13 任务失败时生成错误诊断
L14 新增 /api/tasks/logs/diagnosis
L15 前端错误诊断 Tab 接入
L16 新增 /api/tasks/logs/context
L17 前端上下文摘要 Tab 接入
L18 新增 /api/tasks/logs/export
L19 新增 Rust task_logs_export
L20 任务级脱敏日志 zip 导出
```

验收：

```text
1. FAILED 任务默认展示错误摘要。
2. 错误诊断包含 error_code、error_stage、summary、suggestion、retryable。
3. 导出包不包含真实 API Key、代理密码、Authorization。
4. Rust 写入目标目录，不由 Go core 写用户目录。
```

---

### 阶段三：文件日志和容量治理

```text
L21 新增 NDJSON 文件日志 writer
L22 增加日志按天滚动
L23 增加日志保留策略
L24 设置中心增加日志保留配置
L25 缓存清理支持 task_logs / app_logs
L26 可选接入 SQLite FTS5
```

验收：

```text
1. 日志文件按天生成。
2. 超过容量后清理最旧文件。
3. 清理日志不删除任务、事件、报告、设置。
4. 搜索大量日志时性能可接受。
```

---

## 19. 测试方案

## 19.1 Go 单测

```text
TestTaskLogMigration
TestAppendTaskLogsBatch
TestListTaskLogsByTaskID
TestListTaskLogsByLevel
TestListTaskLogsByModuleStage
TestListTaskLogsByKeyword
TestGetTaskLog
TestTaskLogRedactionBeforePersist
TestTaskLogAsyncWriterFlushBySize
TestTaskLogAsyncWriterFlushByInterval
TestTaskLogAsyncWriterFlushOnShutdown
TestRunStageSuccess
TestRunStageFailed
TestTaskErrorDiagnosisFromXerr
TestTaskLogContextSummaryRedacted
TestTaskLogsExportRedacted
```

---

## 19.2 Rust 单测

```text
task_logs_list_fixed_path
task_log_get_rejects_non_positive_id
task_log_context_rejects_empty_task_id
task_logs_export_rejects_empty_target_dir
task_logs_export_rejects_missing_target_dir
task_logs_export_rejects_path_traversal_filename
task_logs_export_does_not_overwrite_existing_file
task_logs_export_creates_private_file_on_unix
```

---

## 19.3 前端测试

```text
TaskLogDrawer renders summary
TaskLogDrawer loads log table
TaskLogDrawer filters by level
TaskLogDrawer filters only error
TaskLogDrawer appends logs while running
TaskLogDrawer stops polling after terminal status
TaskLogDrawer opens raw JSON detail
TaskLogDrawer shows diagnosis for failed task
TaskLogDrawer renders context summary
TaskLogDrawer exports redacted logs
```

---

## 19.4 集成验收

```text
1. 创建一个 AI 分析任务。
2. 任务执行时打开完整日志抽屉。
3. 执行日志持续追加。
4. 任务成功后停止自动拉取。
5. 创建一个故意失败的任务，比如错误 API Key。
6. 错误诊断显示认证失败。
7. 导出任务日志。
8. 检查导出包无 API Key、Authorization、代理密码、用户持仓明细。
```

---

## 20. 推荐开发顺序

优先做最短闭环：

```text
task_log_entries 表
  ↓
TaskLogHandler + AsyncWriter
  ↓
分析任务 RunStage
  ↓
task_logs_list API
  ↓
Rust task_logs_list command
  ↓
前端完整日志抽屉
```

不要先做：

```text
FTS
日志瀑布图
远程日志
复杂统计
日志上传
```

---

## 21. 结论

投研罗盘的结构化日志不应该做成复杂的服务端日志平台，而应该做成桌面本地轻量可观测系统：

```text
task_events：用户可读事件流
task_log_entries：任务级结构化执行日志
task_error_diagnoses：失败任务诊断摘要
NDJSON：本地完整日志文件
task_logs_export：任务级脱敏导出
React Drawer：日志查询、筛选、展示、复制、导出
```

这套方案的关键是三点：

```text
1. slog 自定义 Handler，把任务日志自动落库。
2. SQLite 批量写入，保证桌面端性能稳定。
3. 统一脱敏边界，确保日志、错误、导出都不泄露敏感信息。
```
