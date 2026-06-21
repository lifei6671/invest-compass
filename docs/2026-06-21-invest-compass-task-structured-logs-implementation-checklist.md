# 投研罗盘任务结构化日志实施清单

> 来源：`docs/2026-06-21-invest-compass-task-structured-logs-design.md`
>
> 目标：把任务结构化日志技术方案拆成可执行、可标记、可验收的任务清单。本文只描述任务日志专题，不把远程日志平台、日志上传、复杂统计或完整可观测平台提前混入首批实现。

---

## 1. 范围

### 1.1 首批必须交付

- `task_log_entries` SQLite 表、GORM model、DAO 查询与写入。
- `service/tasklog` 查询、写入、脱敏和分页模型。
- `pkg/logger.TaskLogHandler` 把 Go `slog` 记录转换为任务结构化日志。
- `TaskLogAsyncWriter` 批量写入 SQLite，支持按条数、间隔和 shutdown flush。
- AI 分析任务关键阶段接入 `RunStage`。
- Go API：`/api/tasks/logs/list`、`/api/tasks/logs/get`。
- Rust 白名单 command：`task_logs_list`、`task_log_get`。
- 前端 typed service 和 `TaskLogDrawer` 接入真实结构化日志。
- 执行日志 Tab 支持级别、阶段、关键字、仅错误、本地自动滚动和运行中增量拉取。
- 原始 JSON 详情只展示单条日志的脱敏 payload，不展示密钥、完整请求头或完整 AI 输出。

### 1.2 后续阶段交付

- `task_error_diagnoses` 表和失败任务错误诊断。
- 错误诊断 API、Rust command 和前端 Tab。
- 上下文摘要 API、Rust command 和前端 Tab。
- 脱敏日志导出 API、Rust command 和前端导出交互。
- NDJSON 文件日志 writer、按天滚动、保留策略和缓存清理入口。
- 可选 SQLite FTS5，用于大量日志搜索。

### 1.3 明确不做

- 不引入 ELK、Loki、ClickHouse、OpenTelemetry Collector 等远程日志系统。
- 不上传用户日志。
- 不开放公网 `/metrics`。
- 不让前端直连 Go sidecar。
- 不提供 Rust 任意路径代理。
- 不在日志中保存真实 API Key、`Authorization`、`Proxy-Authorization`、代理密码或用户完整持仓输入。
- 不把完整 AI 流式输出重复写入 `task_log_entries`。
- 不把 `task_events` 和 `task_log_entries` 合并成一张表。
- 不让 Go core 直接写用户选择目录；导出文件必须由 Rust 写入。

---

## 2. 状态标记

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成并通过验证
- `[!]` 阻塞或需要人工确认

---

## 3. 依赖路线

```text
SL0 文档边界和总入口
  ↓
SL1 SQLite schema、model 和 DAO
  ↓
SL2 tasklog service、slog handler、异步写入和 RunStage
  ↓
SL3 Go API 与 Rust 白名单 command
  ↓
SL4 TaskLogDrawer 接真实数据和执行日志交互
  ↓
SL5 错误诊断、上下文摘要和脱敏导出
  ↓
SL6 NDJSON 文件日志、保留策略和容量治理
  ↓
SL7 验收、测试、文档同步和停止线
```

并行原则：

- SL1 是上游契约，schema、索引和 DAO 方法稳定前不要推进 Go API、Rust command 和前端真实接入。
- SL2 可在 SL1 model 稳定后拆成 `TaskLogHandler`、`AsyncWriter`、`RunStage` 三个互不写冲突的子任务。
- SL3 必须等 service 输入输出结构稳定后推进，避免 Rust command schema 反复变更。
- SL4 可在 Rust command 契约稳定后并行拆前端 service、抽屉数据接入和筛选交互。
- SL5 的错误诊断、上下文摘要、导出可以分支并行，但都必须复用统一脱敏能力。
- SL6 是容量治理增强，不阻塞 MVP 执行日志 Tab。

---

## 4. Review Gate

### RGSL1 数据库和 DAO 门禁

- `task_log_entries` migration 可重复执行。
- 索引覆盖 `task_id + id`、`task_id + level`、`task_id + module + stage`、`request_id`、`trace_id`、`ts`。
- DAO 支持 append、list、get、分页和筛选。
- 不破坏现有 `tasks`、`task_events`、`analysis_reports` 语义。

### RGSL2 日志写入和脱敏门禁

- `slog.Record` 转 `TaskLogEntry` 前完成脱敏。
- SQLite 入库前不包含 API Key、Authorization、代理密码、用户完整持仓输入。
- 没有 `task_id` 的普通应用日志不写入 `task_log_entries`。
- AI 流式输出只记录摘要，不重复保存完整报告正文。
- AsyncWriter 在 shutdown 时可 flush，不静默丢弃已入队日志。

### RGSL3 API 和 Rust 安全门禁

- Go API 全部为 POST。
- Go API 复用 token、ready、`httpx.DecodeJSON`、统一响应和 trace/request ID。
- Rust command 固定 path、固定 request/response schema，不提供任意 task log path 代理。
- `task_log_get` 拒绝非正数 id。
- `task_logs_export` 拒绝空目录、缺失目录、路径穿越和覆盖已有文件。

### RGSL4 前端抽屉门禁

- 点击任务历史“日志”打开右侧 `TaskLogDrawer`。
- 宽屏抽屉无遮罩，窄屏全屏抽屉不横向溢出。
- 执行日志 Tab 从真实 command 加载数据。
- 搜索、级别、阶段、仅错误、自动滚动、本地轮询可用。
- 原始 JSON 详情按选中日志加载，不展示整页假数据。

### RGSL5 诊断、上下文和导出门禁

- FAILED 任务可展示错误摘要、原因列表、建议处理和 retryable。
- 上下文摘要只展示股票、分析类型、模型、模板、数据加载状态等必要字段。
- 导出包二次脱敏，不包含密钥、完整请求头、代理密码或完整用户隐私输入。
- Rust 写入用户目录，Go core 只返回脱敏包内容或临时安全结果。

### RGSL6 容量治理门禁

- NDJSON 文件按天滚动。
- 保留策略只清理日志，不删除任务、事件、报告和设置。
- 缓存清理能区分 `task_logs` 和 `app_logs`。
- FTS5 如未进入首批，必须保持清晰停止线，不影响基础查询。

---

## 5. SL0：文档边界和总入口

### SL00 固化结构化日志专题实施清单

- 状态：`[x]`
- 依赖：无
- 交付物：
  - `docs/2026-06-21-invest-compass-task-structured-logs-implementation-checklist.md`
- 执行动作：
  - 将技术方案拆成阶段、Review Gate、任务项、验证命令和退出条件。
  - 明确 MVP、后续阶段和禁止事项。
  - 保持任务 ID 可勾选、可引用、可验收。
- 验证：
  - `git diff --check`
  - 文档没有把未实现能力写成已完成。
- 退出条件：
  - 后续实现可以按任务 ID 独立推进和复盘。

### SL01 同步主 checklist 专题入口

- 状态：`[x]`
- 依赖：SL00
- 交付物：
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
- 执行动作：
  - 在“专题实施清单”章节补充结构化日志专题入口。
  - 引用本文和设计方案。
  - 明确该专题不改变主链路已完成状态。
- 验证：
  - `git diff --check`
  - 总 checklist 不批量展开结构化日志所有子任务。
- 退出条件：
  - 主 checklist 能导航到结构化日志专题清单。

---

## 6. SL1：SQLite schema、model 和 DAO

### SL02 新增 task_log_entries schema

- 状态：`[ ]`
- 依赖：SL00
- 交付物：
  - `apps/sidecar-core/internal/model/task_log.go`
  - `apps/sidecar-core/internal/dao/database.go`
  - migration 相关测试
- 执行动作：
  - 新增 `TaskLogEntry` GORM model。
  - 字段包含 `task_id`、`request_id`、`trace_id`、`ts`、`level`、`module`、`stage`、`message`、`code`、`provider`、`model`、`symbol`、`duration_ms`、`retryable`、`payload_json`、`created_at`、`updated_at`。
  - 增加技术方案要求的索引。
  - 接入 `dao.Migrate`。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TestMigrate`
  - 空库迁移成功。
  - 重复迁移不破坏已有表。
- 退出条件：
  - SQLite schema 能支撑任务级结构化日志查询。

### SL03 实现 task log DAO

- 状态：`[ ]`
- 依赖：SL02
- 交付物：
  - `apps/sidecar-core/internal/dao/task_log_repository.go`
  - `apps/sidecar-core/internal/dao/repository_test.go`
- 执行动作：
  - 实现 `AppendTaskLogs(ctx, entries)`。
  - 实现 `ListTaskLogs(ctx, query)`。
  - 实现 `GetTaskLog(ctx, id)`。
  - 支持 `task_id`、`level`、`module`、`stage`、`keyword`、`only_error`、`after_id`、`limit`。
  - `limit` 必须有上限，避免无界查询。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TaskLog`
  - 覆盖按 task、level、module、stage、keyword、after_id 查询。
- 退出条件：
  - service 层不需要直接使用 GORM 就能查询和写入任务日志。

### SL04 补充 DAO 边界测试

- 状态：`[ ]`
- 依赖：SL03
- 交付物：
  - DAO 单测
- 执行动作：
  - 覆盖空 task_id、非法 limit、缺失日志、分页边界。
  - 覆盖同一 task_id 多级别日志排序。
  - 覆盖 payload_json 存取和 JSON 字符串稳定性。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TaskLog`
- 退出条件：
  - DAO 关键边界有明确失败或返回语义。

---

## 7. SL2：tasklog service、slog handler 和任务接入

### SL05 建立 service/tasklog 包边界

- 状态：`[ ]`
- 依赖：SL03
- 交付物：
  - `apps/sidecar-core/internal/service/tasklog/doc.go`
  - `apps/sidecar-core/internal/service/tasklog/service.go`
  - `apps/sidecar-core/internal/service/tasklog/types.go`
- 执行动作：
  - 定义 query、response、summary、raw detail 类型。
  - service 只依赖 dao.Store，不直接依赖 Gin 或 Rust。
  - 在包注释中说明该包只负责任务结构化日志，不承载普通应用日志平台能力。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog`
- 退出条件：
  - 后续 handler、API 和 export 能复用同一 service 契约。

### SL06 实现统一脱敏能力

- 状态：`[ ]`
- 依赖：SL05
- 交付物：
  - `apps/sidecar-core/pkg/logger` 或现有脱敏包
  - 单测
- 执行动作：
  - 覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户一次性持仓输入。
  - 对 message、payload_json、raw attributes 统一处理。
  - 保持 request_id、trace_id、task_id、provider、symbol 等可观测字段不被误删。
- 验证：
  - `cd apps/sidecar-core && go test ./... -run Redact`
- 退出条件：
  - 入库前和导出前都可以调用同一套脱敏能力。

### SL07 实现 pkg/logger.TaskLogHandler

- 状态：`[ ]`
- 依赖：SL05、SL06
- 交付物：
  - `apps/sidecar-core/pkg/logger/task_log_handler.go`
  - `apps/sidecar-core/pkg/logger/task_log_handler_test.go`
- 执行动作：
  - 从 `slog.Record` 提取 `task_id`、`request_id`、`trace_id`、`module`、`stage`、`provider`、`model`、`symbol`、`duration_ms`、`retryable`。
  - 没有 `task_id` 时跳过写入 `task_log_entries`。
  - 转换前完成脱敏。
  - 对缺失 module/stage 做清晰失败或拒绝写入，不静默写成无意义日志。
- 验证：
  - `cd apps/sidecar-core && go test ./pkg/logger -run TaskLogHandler`
- 退出条件：
  - Go 业务日志可以稳定转换为任务结构化日志。

### SL08 实现 TaskLogAsyncWriter

- 状态：`[ ]`
- 依赖：SL03、SL07
- 交付物：
  - `apps/sidecar-core/internal/service/tasklog/async_writer.go`
  - 单测
- 执行动作：
  - 支持内存队列、批量写入、按条数 flush、按间隔 flush、shutdown flush。
  - 写入失败必须可见，至少写入普通应用日志并保留错误返回路径。
  - 不因为单条日志失败阻塞所有任务。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run AsyncWriter`
- 退出条件：
  - 任务日志写入不会明显拖慢任务主链路。

### SL09 实现 RunStage helper

- 状态：`[ ]`
- 依赖：SL07、SL08
- 交付物：
  - `apps/sidecar-core/internal/service/tasklog/run_stage.go`
  - 单测
- 执行动作：
  - 定义 `RunStage(ctx, meta, stage, fn)`。
  - success 时写 INFO。
  - error 时写 ERROR，并附带 code、duration_ms、retryable 等字段。
  - 阶段名使用技术方案统一枚举，不引入 `ai_stream_*` 旧命名。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run RunStage`
- 退出条件：
  - 长任务阶段可以用统一方式产生日志。

### SL10 AI 分析任务接入关键阶段日志

- 状态：`[ ]`
- 依赖：SL09
- 交付物：
  - `apps/sidecar-core/internal/service/analysis` 相关文件
  - 单测或集成测试
- 执行动作：
  - 接入 `quote_fetch`、`kline_fetch`、`calc_macd`、`prompt_build`、`stream_start`、`stream_chunk`、`stream_timeout`、`stream_failed`。
  - `TASK_CHUNK` 仍写入 `task_events`，结构化日志只写摘要。
  - 失败时保留 error_code、stage、retryable。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/analysis ./internal/service/tasklog`
  - 人工或集成用例可在失败任务中看到 ERROR 日志。
- 退出条件：
  - AI 分析任务能产生完整执行日志链路。

---

## 8. SL3：Go API 与 Rust 白名单 command

### SL11 新增 task logs Go API

- 状态：`[ ]`
- 依赖：SL05、SL10
- 交付物：
  - `apps/sidecar-core/internal/actions/tasklog`
  - `apps/sidecar-core/internal/actions/router.go`
- 执行动作：
  - 新增 `POST /api/tasks/logs/list`。
  - 新增 `POST /api/tasks/logs/get`。
  - 复用 token、ready、POST-only、统一响应、request_id/trace_id。
  - list 返回 rows、next_after_id、has_more。
  - get 返回单条日志脱敏 raw JSON。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/actions/... -run TaskLog`
- 退出条件：
  - Rust 可以通过固定 API 获取日志列表和详情。

### SL12 加强 Go API 入参和安全测试

- 状态：`[ ]`
- 依赖：SL11
- 交付物：
  - action 层测试
- 执行动作：
  - 拒绝空 task_id。
  - 拒绝非法 id。
  - 限制 limit 上限。
  - 验证非 POST 返回 405。
  - 验证未 ready 或缺 token 不能访问。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/actions/... -run TaskLog`
- 退出条件：
  - HTTP 边界不会绕过 sidecar 安全约束。

### SL13 新增 Rust task log command

- 状态：`[ ]`
- 依赖：SL11
- 交付物：
  - `apps/desktop/src-tauri/src/commands/task_logs.rs`
  - command 注册与权限配置
  - Rust 单测
- 执行动作：
  - 新增 `task_logs_list(payload)` 固定映射 `/api/tasks/logs/list`。
  - 新增 `task_log_get(id)` 固定映射 `/api/tasks/logs/get`。
  - request/response schema 固定，不提供任意 path 代理。
  - `task_log_get` 拒绝非正数 id。
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml task_log`
- 退出条件：
  - 前端只能通过白名单 command 访问任务日志。

---

## 9. SL4：TaskLogDrawer 接真实数据

### SL14 新增前端 task logs typed service

- 状态：`[ ]`
- 依赖：SL13
- 交付物：
  - `apps/frontend/src/services/taskLogs.ts`
  - `apps/frontend/src/pages/tasks/taskLogTypes.ts`
- 执行动作：
  - 定义 `TaskLogRow`、`TaskLogDetail`、`TaskLogListRequest`、`TaskLogListResponse`。
  - 封装 `task_logs_list` 和 `task_log_get`。
  - 前端 DTO 不包含密钥、完整请求头和代理密码字段。
- 验证：
  - `pnpm --dir apps check`
  - `pnpm --dir apps test -- taskLogs`
- 退出条件：
  - 页面不直接调用裸 invoke 字符串。

### SL15 TaskLogDrawer 接入真实日志列表

- 状态：`[ ]`
- 依赖：SL14
- 交付物：
  - `apps/frontend/src/pages/tasks/components/TaskLogDrawer.tsx`
  - `apps/frontend/src/pages/tasks/components/ExecutionLogTable.tsx`
- 执行动作：
  - 点击任务列表“日志”时传入 selectedTask。
  - 抽屉打开后按 task_id 加载执行日志。
  - 保留宽屏无遮罩、窄屏全屏行为。
  - 加载态、空态、错误态都必须明确展示。
- 验证：
  - `pnpm --dir apps check`
  - `pnpm --dir apps test -- TaskLogDrawer`
- 退出条件：
  - 抽屉不再依赖静态 mock 日志。

### SL16 执行日志筛选和运行中增量拉取

- 状态：`[ ]`
- 依赖：SL15
- 交付物：
  - `TaskLogTabs` / `ExecutionLogTable` 相关实现
- 执行动作：
  - 支持 keyword、level、stage、onlyError。
  - RUNNING 任务按 `after_id` 增量拉取。
  - 终态任务停止轮询。
  - 自动滚动和暂停滚动只影响本地展示，不改后端状态。
- 验证：
  - `pnpm --dir apps test -- TaskLogDrawer`
  - 手工验证 RUNNING、SUCCESS、FAILED 三类任务。
- 退出条件：
  - UI 图中的执行日志 Tab 行为可用。

### SL17 原始 JSON 详情接真实单条日志

- 状态：`[ ]`
- 依赖：SL15
- 交付物：
  - `apps/frontend/src/pages/tasks/components/RawJsonPanel.tsx`
- 执行动作：
  - 用户点击表格行后调用 `task_log_get(id)`。
  - 默认展示最新 ERROR；无 ERROR 时展示最近一条；无日志时展示空态。
  - JSON copy 只复制脱敏后的单条 raw。
- 验证：
  - `pnpm --dir apps test -- RawJsonPanel`
- 退出条件：
  - 原始 JSON 区和表格选中行保持一致。

---

## 10. SL5：错误诊断、上下文摘要和脱敏导出

### SL18 新增 task_error_diagnoses schema 和 DAO

- 状态：`[ ]`
- 依赖：RGSL4
- 交付物：
  - `apps/sidecar-core/internal/model/task_log.go`
  - `apps/sidecar-core/internal/dao/task_log_repository.go`
  - DAO 单测
- 执行动作：
  - 新增 `task_error_diagnoses` 表。
  - 字段包含 `task_id`、`error_code`、`error_stage`、`summary`、`causes_json`、`suggestions_json`、`retryable`、`source_log_id`、`created_at`、`updated_at`。
  - 支持按 task_id upsert 和读取。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run Diagnosis`
- 退出条件：
  - 失败任务诊断可持久化和回放。

### SL19 实现 TaskErrorDiagnosisService

- 状态：`[ ]`
- 依赖：SL18
- 交付物：
  - `apps/sidecar-core/internal/service/tasklog/diagnosis.go`
  - 单测
- 执行动作：
  - 根据 xerr code、最后一条 ERROR、stage 生成诊断。
  - 输出 summary、possible causes、suggestions、retryable。
  - 不把完整 Provider request、API Key、代理信息写入诊断。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run Diagnosis`
- 退出条件：
  - 常见认证失败、网络超时、模型不可用有可读诊断。

### SL20 接入 diagnosis API、Rust command 和前端 Tab

- 状态：`[ ]`
- 依赖：SL19
- 交付物：
  - `POST /api/tasks/logs/diagnosis`
  - `task_log_diagnosis(task_id)`
  - 错误诊断 Tab
- 执行动作：
  - Go API 拒绝空 task_id。
  - Rust command 固定 path。
  - FAILED 任务默认显示错误摘要。
  - 非 FAILED 任务展示无诊断或正常说明。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/actions/... -run Diagnosis`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml task_log`
  - `pnpm --dir apps test -- TaskLogDrawer`
- 退出条件：
  - 抽屉错误诊断 Tab 与失败任务状态一致。

### SL21 实现上下文摘要 API、Rust command 和前端 Tab

- 状态：`[ ]`
- 依赖：RGSL4
- 交付物：
  - `POST /api/tasks/logs/context`
  - `task_log_context(task_id)`
  - 上下文摘要 Tab
- 执行动作：
  - 展示股票、分析类型、模型、Prompt 模板、行情/K线/指标/新闻加载摘要。
  - 用户持仓只展示“未提供”或脱敏摘要。
  - 不展示完整 Prompt 和完整用户隐私输入。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run Context`
  - `pnpm --dir apps test -- TaskLogDrawer`
- 退出条件：
  - 上下文摘要可辅助排障，但不泄漏敏感输入。

### SL22 实现脱敏日志导出 Go API

- 状态：`[ ]`
- 依赖：SL20、SL21
- 交付物：
  - `POST /api/tasks/logs/export`
  - `TaskLogExportService`
- 执行动作：
  - 生成任务级脱敏日志包内容。
  - 包含 summary、events、logs、diagnosis、context、raw-json-sample。
  - 导出前二次脱敏。
  - Go core 不直接写用户目录。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run Export`
- 退出条件：
  - 导出包内容完整且无密钥字段。

### SL23 实现 Rust task_logs_export 和前端导出交互

- 状态：`[ ]`
- 依赖：SL22
- 交付物：
  - `task_logs_export(task_id, target_dir)`
  - 前端“导出脱敏日志”按钮
- 执行动作：
  - Rust 校验目标目录存在。
  - 拒绝路径穿越和覆盖已有文件。
  - Unix 下创建私有权限文件。
  - 前端点击只触发白名单 command，不直接写文件。
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml task_logs_export`
  - `pnpm --dir apps test -- TaskLogDrawer`
- 退出条件：
  - 用户可以导出脱敏日志包，且写入边界由 Rust 控制。

---

## 11. SL6：NDJSON 文件日志和容量治理

### SL24 新增 NDJSON 文件日志 writer

- 状态：`[ ]`
- 依赖：RGSL5
- 交付物：
  - Go 文件日志 writer
  - 单测
- 执行动作：
  - 写入 `<workspace>/logs/app-YYYY-MM-DD.ndjson`。
  - 一行一条脱敏 JSON。
  - 和 SQLite 写入共享脱敏逻辑。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run NDJSON`
- 退出条件：
  - 文件日志可作为本地排障和导出补充来源。

### SL25 实现日志按天滚动和保留策略

- 状态：`[ ]`
- 依赖：SL24
- 交付物：
  - rolling/retention 实现
  - 单测
- 执行动作：
  - 默认保留最近 30 天。
  - 单任务最多保留 5000 条结构化日志。
  - 全库 `task_log_entries` 最大 50 万条。
  - 超限时清理最旧日志，不删除任务、事件、报告和设置。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/tasklog -run Retention`
- 退出条件：
  - 日志容量可控且不会误删业务数据。

### SL26 设置中心和缓存清理接入日志治理

- 状态：`[ ]`
- 依赖：SL25
- 交付物：
  - 设置中心日志保留配置入口
  - 缓存清理支持 `task_logs` / `app_logs`
- 执行动作：
  - 只展示已闭环的日志保留和清理能力。
  - 清理动作必须走 Rust command -> Go API 白名单链路。
  - 不在 UI 中伪造清理成功。
- 验证：
  - `pnpm --dir apps check`
  - `cd apps/sidecar-core && go test ./internal/service/...`
- 退出条件：
  - 用户能在设置中心管理日志容量。

### SL27 FTS5 搜索停止线

- 状态：`[ ]`
- 依赖：RGSL6
- 交付物：
  - 是否启用 FTS5 的决策记录
- 执行动作：
  - 日志量未达到性能瓶颈前不默认引入 FTS5。
  - 如果启用，补充 migration、触发器、重建策略和回滚说明。
  - 不影响基础 `LIKE` 搜索能力。
- 验证：
  - 性能测试或明确延期说明。
- 退出条件：
  - 搜索增强有清晰进入条件和停止线。

---

## 12. SL7：测试、验收和文档同步

### SL28 补齐 Go 单测

- 状态：`[ ]`
- 依赖：RGSL5
- 交付物：
  - Go 单测
- 执行动作：
  - 覆盖设计方案中的 Go 测试清单：
    - `TestTaskLogMigration`
    - `TestAppendTaskLogsBatch`
    - `TestListTaskLogsByTaskID`
    - `TestListTaskLogsByLevel`
    - `TestListTaskLogsByModuleStage`
    - `TestListTaskLogsByKeyword`
    - `TestGetTaskLog`
    - `TestTaskLogRedactionBeforePersist`
    - `TestTaskLogAsyncWriterFlushBySize`
    - `TestTaskLogAsyncWriterFlushByInterval`
    - `TestTaskLogAsyncWriterFlushOnShutdown`
    - `TestRunStageSuccess`
    - `TestRunStageFailed`
    - `TestTaskErrorDiagnosisFromXerr`
    - `TestTaskLogContextSummaryRedacted`
    - `TestTaskLogsExportRedacted`
- 验证：
  - `cd apps/sidecar-core && go test ./...`
- 退出条件：
  - Go 日志链路关键行为有自动化覆盖。

### SL29 补齐 Rust command 单测

- 状态：`[ ]`
- 依赖：SL13、SL23
- 交付物：
  - Rust 单测
- 执行动作：
  - 覆盖设计方案中的 Rust 测试清单：
    - `task_logs_list_fixed_path`
    - `task_log_get_rejects_non_positive_id`
    - `task_log_context_rejects_empty_task_id`
    - `task_logs_export_rejects_empty_target_dir`
    - `task_logs_export_rejects_missing_target_dir`
    - `task_logs_export_rejects_path_traversal_filename`
    - `task_logs_export_does_not_overwrite_existing_file`
    - `task_logs_export_creates_private_file_on_unix`
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml task_log`
- 退出条件：
  - Rust 白名单 command 和导出安全边界有自动化覆盖。

### SL30 补齐前端测试和交互验收

- 状态：`[ ]`
- 依赖：SL17、SL20、SL21、SL23
- 交付物：
  - 前端测试
  - 手工验收记录
- 执行动作：
  - 覆盖设计方案中的前端测试清单：
    - `TaskLogDrawer renders summary`
    - `TaskLogDrawer loads log table`
    - `TaskLogDrawer filters by level`
    - `TaskLogDrawer filters only error`
    - `TaskLogDrawer appends logs while running`
    - `TaskLogDrawer stops polling after terminal status`
    - `TaskLogDrawer opens raw JSON detail`
    - `TaskLogDrawer shows diagnosis for failed task`
    - `TaskLogDrawer renders context summary`
    - `TaskLogDrawer exports redacted logs`
  - 手工验收 RUNNING、SUCCESS、FAILED、CANCELLED 四种状态。
- 验证：
  - `pnpm --dir apps check`
  - `pnpm --dir apps test -- TaskLogDrawer`
- 退出条件：
  - 抽屉 UI 行为与设计图和安全边界一致。

### SL31 集成验收

- 状态：`[ ]`
- 依赖：SL28、SL29、SL30
- 交付物：
  - 集成验收记录
- 执行动作：
  - 创建一个 AI 分析任务。
  - 任务执行时打开完整日志抽屉。
  - 验证执行日志持续追加。
  - 任务成功后停止自动拉取。
  - 创建一个故意失败的任务，例如错误 API Key。
  - 验证错误诊断显示认证失败或超时原因。
  - 导出任务日志。
  - 检查导出包无 API Key、Authorization、代理密码、用户持仓明细。
- 验证：
  - `cd apps/sidecar-core && go test ./...`
  - `pnpm --dir apps check`
  - `pnpm --dir apps test`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
- 退出条件：
  - 任务日志从 Go 写入、Rust 白名单、前端抽屉到脱敏导出形成闭环。

### SL32 文档同步和停止线复核

- 状态：`[ ]`
- 依赖：SL31
- 交付物：
  - `docs/2026-06-17-invest-compass-technical-solution.md`
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
  - 本专题清单
- 执行动作：
  - 同步已实现 API、Rust command、数据库 schema、安全边界和验证结果。
  - 只勾选已实现且验证通过的任务。
  - 明确未做事项：远程日志、日志上传、复杂统计、FTS5 如延期则不写成已完成。
- 验证：
  - `git diff --check`
- 退出条件：
  - 文档和代码行为一致，专题能力可进入后续 Review Gate。

---

## 13. 推荐推进批次

### Batch A：MVP 后端契约

- SL02
- SL03
- SL04
- SL05
- SL06

退出条件：`task_log_entries` 和查询写入契约稳定。

### Batch B：写入链路

- SL07
- SL08
- SL09
- SL10

退出条件：AI 分析任务能产生脱敏结构化日志。

### Batch C：API、Rust 和前端执行日志

- SL11
- SL12
- SL13
- SL14
- SL15
- SL16
- SL17

退出条件：任务历史“日志”抽屉执行日志 Tab 可用。

### Batch D：诊断、上下文和导出

- SL18
- SL19
- SL20
- SL21
- SL22
- SL23

退出条件：FAILED 任务排障和脱敏导出闭环。

### Batch E：容量治理和最终验收

- SL24
- SL25
- SL26
- SL27
- SL28
- SL29
- SL30
- SL31
- SL32

退出条件：本地日志容量可治理，自动化测试和文档同步完成。

---

## 14. 最小验收命令

按实现阶段逐步执行，不要求 docs-only 阶段运行全部命令。

```bash
git diff --check
cd apps/sidecar-core && go test ./...
pnpm --dir apps check
pnpm --dir apps test
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
```

无法运行某条命令时，必须在交付说明中写明：

- 未运行命令
- 未运行原因
- 影响范围
- 剩余风险

---

## 15. 专题停止线

以下能力不随结构化日志 MVP 自动进入实现：

- 远程日志平台。
- 日志上传。
- 公网指标服务。
- 复杂日志统计图表。
- 日志瀑布图。
- FTS5 全文索引。
- 自动向外部 issue 系统提交日志。
- 将完整 AI 输出或用户隐私输入写入日志。
