# 投研罗盘 Invest Compass 技术方案

## 1. 项目概述

### 1.1 项目名称

中文名：投研罗盘
英文名：Invest Compass
仓库名：`invest-compass`
核心服务名：`invest-compass-core`
桌面端名：`invest-compass-desktop`
应用标识：`com.lifei6671.investcompass`

### 1.2 产品定位

投研罗盘是一款面向个人投资者和技术型研究者的 AI 投研桌面工作台。产品长期方向是聚合行情、K线、公告、研报、资讯、资金流、技术指标和 AI 分析能力，帮助用户完成自选股跟踪、个股研究、市场复盘、策略观察和持仓辅助分析。

首版先收敛到行情、K线、新闻资讯、技术指标和 AI 个股分析闭环，避免同时铺开过多数据源和半成品页面。

产品定位不是自动交易软件，也不是荐股软件。系统输出仅作为研究辅助，不直接给出收益承诺，不执行真实交易，不代替用户做投资决策。

### 1.3 首版目标

首版要完成一个可独立运行、可配置模型、可查看股票、可生成 AI 分析报告、可保存历史记录的桌面应用。

首版核心闭环：

```text
添加自选股
    ↓
查看行情/K线/基本信息
    ↓
聚合行情/K线/新闻资讯/技术指标
    ↓
选择分析模板和 AI 模型
    ↓
生成个股分析报告
    ↓
保存报告、历史任务、个股备注
```

### 1.4 非目标

首版不做以下能力：

```text
不接券商账户
不做自动下单
不做实盘交易
不做收益承诺
不做社交跟单
不做云端同步
不做多人协作
不做复杂量化回测平台
不做移动端
不做策略观察独立页面
不做公告/研报/资金流专用数据源
不做持仓信息落库
不做授权激活闭环
不做真实自动更新安装
```

---

## 2. 总体技术选型

### 2.1 技术栈

桌面壳：Tauri v2
桌面壳语言：Rust
业务内核：Golang
前端框架：React + TypeScript + Vite
UI 组件：Ant Design + Tailwind CSS
图表：Lightweight Charts + ECharts
本地数据库：SQLite
数据库访问：GORM
Go HTTP 框架：Gin
日志：slog
AI 接入：OpenAI-compatible Provider 抽象
构建管理：pnpm workspace + Go module + Cargo
CI/CD：GitHub Actions
安装包：Windows NSIS/MSI、macOS DMG/App Bundle
更新：首版仅做检查更新入口，真实自动更新后续接 Tauri Updater

### 2.2 为什么选择 Tauri + Go Sidecar

Tauri 负责桌面能力和安全边界，Go 负责业务能力。这样可以避免把复杂投研逻辑写进 Rust，也避免把桌面系统能力全部塞进 Go。

核心理由：

```text
1. Tauri 适合窗口、托盘、系统通知、自动更新、权限控制。
2. Go 适合网络抓取、并发任务、数据处理、AI Provider 封装。
3. 投研业务变化快，Go 的开发效率高。
4. 前端保持 Web 工程化，方便快速迭代 UI。
5. Rust 层只做薄壳和安全代理，降低 Rust 业务复杂度。
```

---

## 3. 总体架构

### 3.1 进程架构

```text
┌────────────────────────────────────────────┐
│               React 前端                    │
│  页面、图表、表格、AI 对话、设置、任务状态       │
└─────────────────────┬──────────────────────┘
                      │ invoke / event
                      ▼
┌────────────────────────────────────────────┐
│              Tauri Core / Rust              │
│  窗口、托盘、自动更新、通知、权限、sidecar 管理   │
│  前端请求代理、参数校验、敏感操作收口             │
└─────────────────────┬──────────────────────┘
                      │ localhost HTTP / SSE
                      ▼
┌────────────────────────────────────────────┐
│              Go Sidecar Core                │
│  行情、新闻、AI分析、任务、缓存、数据库             │
└─────────────────────┬──────────────────────┘
                      │
        ┌─────────────┴─────────────┐
        ▼                           ▼
┌──────────────┐            ┌────────────────┐
│ SQLite 本地库 │            │ 外部数据/AI服务 │
└──────────────┘            └────────────────┘
```

### 3.2 关键设计原则

1. 前端不直接保存 API Key。
2. 前端不直接访问外部股票数据源。
3. Go sidecar 只监听 `127.0.0.1`，禁止监听公网地址。
4. Go sidecar 启动时使用随机端口和一次性 token，token 不通过命令行参数传递。
5. 前端所有请求通过 Tauri command 白名单代理，不直接访问 Go sidecar。
6. 所有 AI 输出必须带风险提示。
7. 所有第三方数据源必须做 Provider 抽象，避免被单一数据源锁死。
8. 股票代码、市场、时间周期、复权类型等必须标准化。
9. API Key、代理密码、持仓输入等敏感字段必须统一脱敏，禁止进入普通日志、错误响应和导出文件。
10. macOS 与 Windows 都必须使用系统目录、Rust 本地文件 vault 和平台签名能力，不硬编码平台路径。

---

## 4. 仓库结构

建议采用 monorepo：

```text
invest-compass/
├── apps/
│   ├── package.json
│   ├── pnpm-workspace.yaml
│   ├── desktop/
│   │   ├── src-tauri/
│   │   │   ├── src/
│   │   │   │   ├── main.rs
│   │   │   │   ├── commands/
│   │   │   │   ├── sidecar/
│   │   │   │   ├── security/
│   │   │   │   └── updater/
│   │   │   ├── binaries/
│   │   │   ├── capabilities/
│   │   │   ├── tauri.conf.json
│   │   │   ├── Cargo.toml
│   │   │   └── Cargo.lock
│   │   └── package.json
│   ├── frontend/
│   │   ├── src/
│   │   │   ├── app/
│   │   │   ├── pages/
│   │   │   ├── components/
│   │   │   ├── features/
│   │   │   ├── services/
│   │   │   ├── stores/
│   │   │   ├── types/
│   │   │   └── styles/
│   │   ├── vite.config.ts
│   │   └── package.json
│   └── packages/
│       └── shared/
│           ├── api.schema.json
│           ├── openapi.yaml
│           └── types/
│   └── sidecar-core/
│       ├── cmd/
│       │   └── invest-compass-core/
│       │       └── main.go
│       ├── internal/
│       │   ├── server/
│       │   ├── service/
│       │   │   ├── analysis/
│       │   │   ├── ai/
│       │   │   ├── dashboard/
│       │   │   ├── indicator/
│       │   │   ├── logexport/
│       │   │   ├── market/
│       │   │   ├── news/
│       │   │   ├── prompt/
│       │   │   ├── report/
│       │   │   ├── settings/
│       │   │   ├── sidecar/
│       │   │   ├── stock/
│       │   │   ├── task/
│       │   │   ├── updatecheck/
│       │   │   └── watchlist/
│       │   ├── dao/
│       │   ├── model/
│       │   └── ...
│       ├── pkg/
│       │   ├── constant/
│       │   ├── logger/
│       │   └── xerr/
│       ├── migrations/
│       ├── go.mod
│       └── go.sum
│
├── docs/
│   ├── architecture.md
│   ├── api.md
│   ├── database.md
│   ├── release.md
│   └── compliance.md
│
├── scripts/
│   ├── build-core.ts
│   ├── package-sidecar.ts
│   ├── dev.ts
│   └── release.ts
│
└── README.md
```

---

## 5. 模块设计

## 5.1 桌面壳模块：Tauri Desktop

### 职责

```text
启动和停止 Go sidecar
维护 Go sidecar 健康状态
代理前端请求到 Go core
系统托盘
开机自启
窗口状态恢复
系统通知
检查更新入口
授权状态占位
本地工作区选择
日志导出
```

### Rust Command 设计

```text
core_start()
core_health()
stock_search(keyword)
market_quote(symbol)
market_kline(symbol, period, adjust, limit)
market_indicators(symbol, period, adjust, limit, indicators)
news_list(symbol, limit)
news_market(market, limit)
watchlist_list()
watchlist_create(payload)
watchlist_update(id, payload)
watchlist_delete(id)
ai_config_list()
ai_config_save(payload)
ai_config_test({ id, api_key_ref })
prompt_templates_list()
prompt_templates_get(id)
prompt_templates_create(payload)
prompt_templates_update(payload)
prompt_templates_delete(id)
analysis_task_create(payload)
task_list(filters)
task_get(task_id)
task_events(task_id, after_event_id)
analysis_task_cancel(task_id)
analysis_task_subscribe(task_id)
report_list(filters)
report_get(id)
report_delete(id)
dashboard_summary()
providers_status()
settings_get(keys)
settings_set(payload)
workspace_get()
workspace_set(payload)
cache_stats()
cache_clean(scope)
export_logs()
provider_status()
open_workspace()
select_directory()
show_notification(title, body)
set_autostart(enabled)
get_autostart()
check_update()
license_status()
```

Rust 层不提供 `core_request(method, path, body)` 这类任意路径代理。每个 command 必须固定允许访问的 Go API、HTTP 方法、请求 schema 和响应字段。`/internal/*`、shutdown、日志导出、本地 vault 写入等敏感能力只能由对应的白名单 command 调用。

报告相关 command 必须固定映射：

- `report_list(filters)` -> `POST /api/reports/list`
- `report_get(id)` -> `POST /api/reports/get`
- `report_delete(id)` -> `POST /api/reports/delete`

报告查询结果默认不返回完整 `input_snapshot`。一次性持仓输入只允许作为任务输入快照的内部审计材料，默认复制、导出和历史查询链路都不得包含完整 `userPosition`。

检查更新 command 必须固定映射：

- `check_update()` -> `POST /api/update/check`

日志导出 command 必须固定映射：

- `export_logs(target_dir)` -> `POST /api/logs/export`

日志导出 API 只生成已二次脱敏的导出包，不直接写入用户目录。真实文件写入必须由 Rust 在用户选择的授权目录内完成。Rust `export_logs(target_dir)` 必须先校验 `target_dir` 是已存在目录，非法目录不得触发 Go core 导出请求；写入时必须拒绝带目录分隔符或路径穿越的文件名，不得覆盖目标目录中已有同名文件，Unix/macOS 下导出文件权限必须为 `0600`，并返回实际写入的 `file_path` 和 `file_name`。

Tauri v2 capability 原则：

```text
1. 默认只给 main 窗口 `core:default`、`core:window:default`、`core:event:default`。
2. 文件、通知、剪贴板、shell、updater 等插件权限必须按功能单独声明。
3. 文件访问 scope 只允许应用数据目录、用户选择的工作区和日志导出目标。
4. `ai_config_save`、`export_logs`、`check_update`、`set_autostart` 等敏感 command 只能由 main/settings 窗口调用。
5. 禁止远程 URL 获得本地 command 权限。
6. CSP 默认使用 `default-src 'self'`，外部链接只允许 HTTPS 且通过系统浏览器打开。
7. macOS 和 Windows capability 文件使用同一最小权限基线，平台特有权限单独拆分。
```

### Sidecar 启动流程

```text
1. Tauri 启动。
2. 读取应用数据目录。
3. 随机生成 core token。
4. 分配本地随机端口。
5. 启动 invest-compass-core sidecar。
6. 传入参数：
   --host=127.0.0.1
   --port=0 或指定随机端口
   --workspace=<app_data_dir>
7. Tauri 通过 sidecar stdin 完成一次性握手，传入 runtime token。
8. Go core 初始化数据库、配置、缓存。
9. Go core 输出 ready JSON：
   {"status":"ready","port":xxxxx,"pid":xxxxx,"protocolVersion":"1"}
10. Tauri 仅在内存保存 port/token。
11. 前端进入主界面。
```

runtime token 禁止出现在命令行参数、环境变量、日志、配置文件和数据库中。sidecar 每次启动或重启都必须生成新的 token。

握手协议：

```text
1. Tauri 使用 sidecar stdin 写入单行 JSON：{"token":"...","protocolVersion":"1"}，随后关闭 stdin。
2. Go core 必须在 5 秒内从 stdin 读完并安装 token，超时或 JSON 非法则退出。
3. Go core 在握手完成前不得注册业务路由；即使端口已分配，也必须拒绝所有请求。
4. Go core 只在 token 安装成功后输出携带 `protocolVersion` 的 ready JSON。
5. macOS 和 Windows 都使用 stdin 语义，不依赖平台特有 FD/HANDLE 继承。
6. 如果 Tauri sidecar API 无法满足 stdin 握手，才允许降级为 Rust `std::process::Command` 自管子进程；降级方案必须同时覆盖 macOS 和 Windows。
```

### 退出流程

```text
1. 用户点击退出。
2. Tauri 调用 Go core /internal/shutdown。
3. Go core 停止任务、落库、flush 日志。
4. Tauri kill sidecar 兜底。
5. 应用退出。
```

---

## 5.2 Go Core 模块

### 5.2.1 server 模块

只负责本地 HTTP server 生命周期。

核心能力：

```text
创建本地 listener
初始化标准库 http.Server
启动本地 HTTP 服务
接收关闭信号并优雅关闭
拒绝非 127.0.0.1 监听地址
配置 ReadHeaderTimeout、ReadTimeout、WriteTimeout、IdleTimeout，避免异常连接长期占用 sidecar
```

### 5.2.1.1 actions 模块

负责本地 HTTP API 的路由注册和 handler 分发。

```text
actions/router.go：唯一 Gin 路由注册入口。
actions/<module>：业务 handler 子包，对外提供自身路由定义，不直接依赖 Gin 注册 API。
actions/httpx：统一响应、request_id/trace_id、POST-only、token 校验、JSON 请求体上限、panic recovery 辅助能力。
```

统一响应结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

错误响应：

```json
{
  "code": 40001,
  "message": "invalid stock code",
  "data": null,
  "traceId": "...",
  "requestId": "..."
}
```

`requestId` / `traceId` 可从 Rust 转发请求头透传，但 Go core 只接受 1-128 位 ASCII 字母、数字、`-`、`_`、`.`。缺失、超长或包含换行/空格等异常字符时必须重新生成本地 ID，避免异常头值进入统一响应、日志和导出链路。

Go core 所有业务 handler 必须通过 `actions/httpx.DecodeJSON` 解码 JSON 请求体，单个请求体上限为 1 MiB，且请求体只允许包含一个 JSON object 文档。超过上限时返回 HTTP 413 和稳定错误码 `41300/request_body_too_large`；`null`、数组、合法 JSON 后追加额外内容或包含未知字段时返回 `40001/invalid_json`，避免异常本地请求占用过多内存、拼写错误、零值请求或未知字段被业务层忽略。

### 5.2.2 stock 模块

负责股票基础数据、搜索、自选股、个股详情。

核心能力：

```text
股票搜索
股票基础信息
自选股列表
添加自选股
删除自选股
股票标签
用户备注
```

股票标准编码：

```text
A股：CN:SH:600519
A股：CN:SZ:300750
港股：HK:00700
美股：US:AAPL
指数：INDEX:CN:000001
基金：FUND:CN:xxxxxx
```

### 5.2.3 market 模块

负责行情数据。

核心能力：

```text
实时行情
分时数据
日K/周K/月K
前复权/后复权/不复权
成交量
换手率
涨跌幅
市盈率/市净率/市销率
行业板块
概念板块
```

行情 Provider 抽象：

```go
type MarketProvider interface {
    Name() string
    Search(keyword string) ([]StockBasic, error)
    Quote(symbol Symbol) (*Quote, error)
    Kline(req KlineRequest) ([]KlineBar, error)
    Minute(req MinuteRequest) ([]MinuteBar, error)
}
```

首版至少实现一个合规可用的数据 Provider，同时保留可扩展接口。数据源必须明确来源、授权边界和访问频率限制。后续可以接入付费数据源或用户自己的数据源。

### 5.2.4 news 模块

负责新闻资讯。公告、研报、财经日历属于后续版本能力，首版不提供专用数据源和独立 API。

核心能力：

```text
个股新闻
市场新闻
行业新闻
重大事件
新闻去重
新闻缓存
```

数据结构：

```go
type NewsItem struct {
    ID          string
    Source      string
    Title       string
    URL         string
    Summary     string
    PublishedAt time.Time
    Symbols     []string
    Tags        []string
}
```

### 5.2.5 indicator 模块

负责技术指标计算。

首版指标：

```text
MA
EMA
MACD
RSI
KDJ
BOLL
成交量均线
涨跌幅
区间最大回撤
区间波动率
```

指标计算原则：

```text
1. 不依赖前端计算。
2. Go core 统一计算，前端只展示。
3. 指标结果可缓存。
4. 指标参数可配置。
```

### 5.2.6 ai 模块

负责 AI Provider、模型配置、Prompt 构建、AI 调用、流式输出。

Provider 类型：

```text
OpenAI Compatible
DeepSeek
Qwen
Doubao / Ark
Gemini
Claude
Ollama
LM Studio
Custom HTTP Provider
```

模型配置：

```text
Provider 名称
Base URL
API Key
模型名
温度
最大输出 Token
超时时间
是否启用流式输出
是否默认模型
```

AI 调用接口：

```go
type AIProvider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}
```

### 5.2.7 analysis 模块

负责投研分析任务。

首版分析类型：

```text
个股综合分析
技术面分析
```

财务质量、行业地位、持仓辅助、市场复盘等分析类型依赖更完整的数据源和持仓模型，放到后续版本。

个股综合分析流程：

```text
1. 校验股票代码。
2. 拉取基础信息。
3. 拉取行情和K线。
4. 计算技术指标。
5. 拉取新闻资讯。
6. 整理数据上下文。
7. 根据模板构建 Prompt。
8. 调用 AI。
9. 流式返回结果。
10. 保存分析报告。
11. 生成风险提示。
```

输出报告结构：

```text
1. 公司/股票概况
2. 当前行情状态
3. 技术面观察
4. 消息面观察
5. 行业与竞争格局
6. 主要风险
7. 观察指标
8. 非投资建议声明
```

### 5.2.8 prompt 模块

负责 Prompt 模板管理。

模板类型：

```text
系统模板
个股分析模板
技术分析模板
自定义模板
```

财务分析模板、持仓分析模板、市场复盘模板放到后续版本。首版模板页不得出现不可执行的模板类型。

Prompt 变量：

```text
{{stock_name}}
{{stock_code}}
{{market}}
{{quote}}
{{kline_summary}}
{{indicators}}
{{news}}
{{analysis_language}}
```

一次性持仓输入只允许通过分析任务请求传入，不作为通用模板变量持久化。

### 5.2.9 task 模块

负责长任务。

任务类型：

```text
AI_ANALYSIS
CACHE_CLEAN
```

`NEWS_SYNC`、`REPORT_SYNC`、`MARKET_REFRESH`、`DATA_IMPORT` 放到后续版本；首版不保留不可执行任务类型。

任务状态：

```text
PENDING
RUNNING
SUCCESS
FAILED
CANCELLED
```

任务事件：

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

### 5.2.10 Go core 分层

Go core 按 server、actions、service、dao、model、pkg/constant、pkg/xerr 分层组织。

职责：

```text
server：HTTP server 监听、初始化、启动和优雅关闭，不注册业务路由。
actions/router.go：集中注册 Gin 路由，组合各 action 子包提供的路由定义。
actions/<module>：业务 handler 子包，负责自己的业务 HTTP 处理，但不直接依赖 Gin 路由注册。
actions/httpx：actions 层共享的统一响应、追踪 ID 和 token 安全边界。
service：业务编排和核心业务逻辑，组合 provider、dao、prompt、AI、task 等能力。
dao：GORM 数据库访问、事务、迁移和升级备份。
model：service 和 dao 共享的结构体、持久化模型和跨层数据模型。
pkg/constant：跨包共享的非错误类常量。
pkg/xerr：跨包共享错误码和通用错误类型，service 层不得重复定义通用错误结构。
```

数据库层原则：

原则：

```text
1. 本地优先。
2. 数据库文件放在用户工作区。
3. 每次版本升级前自动备份。
4. 所有表必须有 created_at、updated_at。
5. 重要业务表使用软删除。
6. 运行中任务恢复必须可判断，避免 sidecar 重启后任务长期停在 RUNNING。
7. 非 dao 层禁止直接使用 database/sql、GORM 数据库句柄或手写 SQL。
```

Go sidecar 生产启动时，在 `dao.Open` 和 `dao.Migrate` 前执行迁移前备份：已有 SQLite 主库复制到同工作区 `backups/` 目录，首次启动缺失库跳过，同名备份拒绝覆盖。真实发布升级仍需在 macOS / Windows 安装包中做恢复演练。

### 5.2.11 config 模块

负责应用配置。

配置项：

```text
主题
语言
工作区路径
代理设置
默认市场
默认 AI 模型
行情刷新频率
通知设置
开机自启
关闭后最小化到托盘
主窗口状态：window.main.x / window.main.y / window.main.width / window.main.height
缓存大小上限
```

Rust 桌面运行期启动时从 Go core settings 读取主窗口状态并恢复位置和尺寸；关闭主窗口时写回当前外层窗口位置和尺寸。恢复只接受完整且尺寸合理的数据，缺字段、非法数字或过小尺寸时保留 Tauri 默认窗口状态。

### 5.2.12 license 模块

首版不实现商业授权闭环，只在设置和关于页面展示 `FREE` 状态占位。授权激活、授权服务、离线宽限、设备迁移放到商业化阶段。

首版状态：

```text
FREE
```

后续授权策略：

授权策略：

```text
1. 用户输入 license key。
2. 应用向授权服务校验。
3. 服务返回签名后的授权信息。
4. 本地缓存授权文件。
5. 离线状态下允许短期宽限。
6. 授权绑定 machine id，但允许有限次数设备迁移。
```

授权状态：

```text
FREE
TRIAL
PRO
EXPIRED
INVALID
OFFLINE_GRACE
```

---

## 6. 前端页面设计

### 6.1 页面列表

首版页面：

```text
1. 总览页 Dashboard
2. 自选股 Watchlist
3. 个股详情 Stock Detail
4. AI 分析 Analysis
5. 分析报告历史 Reports
6. 资讯中心 News Center
7. 任务历史 Tasks
8. 设置中心 Settings
```

### 6.2 总览页

内容：

```text
市场指数卡片
自选股涨跌分布
今日热点
最近分析报告
最近任务状态
风险提示区
快捷入口
```

### 6.3 自选股页

功能：

```text
搜索股票
添加自选
删除自选
标签
备注
批量刷新
排序
```

字段：

```text
股票名称
代码
市场
现价
涨跌幅
成交额
换手率
市盈率
所属行业
最后更新时间
操作
```

### 6.4 个股详情页

区域：

```text
顶部：股票名称、代码、价格、涨跌幅、更新时间
左侧：K线图、分时图、技术指标
右侧：基础信息、估值、行业、标签、备注
下方：新闻资讯、AI分析入口
```

### 6.5 AI 分析页

功能：

```text
选择股票
选择分析类型
选择 AI 模型
选择 Prompt 模板
输入个人持仓信息，可选，仅作为本次分析上下文，不落库
开始分析
流式输出
停止生成
保存报告
复制 Markdown
导出 Markdown
```

### 6.6 设置中心

设置分组：

```text
基础设置
模型设置
数据源设置
代理设置
通知设置
工作区设置
缓存管理
开机自启
检查更新
关于应用
```

检查更新在首版仅提供版本提示，不执行下载和安装。授权信息仅展示当前 `FREE` 状态占位，不提供激活入口。

总览页和资讯中心只使用 `dashboard/summary`、`news`、`news/market`、报告列表和任务列表等首版已有 API，不接入策略、公告、研报、资金流数据。

---

## 7. 数据库设计

### 7.1 stocks 股票基础表

```sql
CREATE TABLE stocks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL UNIQUE,
    market TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    pinyin TEXT,
    exchange TEXT,
    industry TEXT,
    concept TEXT,
    list_date TEXT,
    status TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
```

### 7.2 watchlists 自选股表

```sql
CREATE TABLE watchlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    sort_order INTEGER DEFAULT 0,
    tags TEXT,
    note TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX idx_watchlists_symbol_active
ON watchlists(symbol)
WHERE deleted_at IS NULL;
```

### 7.3 quotes 行情快照表

```sql
CREATE TABLE quotes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    price REAL,
    change_amount REAL,
    change_percent REAL,
    open REAL,
    high REAL,
    low REAL,
    pre_close REAL,
    volume REAL,
    amount REAL,
    turnover_rate REAL,
    pe REAL,
    pb REAL,
    quote_time DATETIME,
    provider TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
```

### 7.4 klines K线表

```sql
CREATE TABLE klines (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    adjust TEXT NOT NULL,
    trade_date TEXT NOT NULL,
    open REAL,
    high REAL,
    low REAL,
    close REAL,
    volume REAL,
    amount REAL,
    provider TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(symbol, period, adjust, trade_date)
);
```

### 7.5 news_items 新闻表

```sql
CREATE TABLE news_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT,
    title TEXT NOT NULL,
    url TEXT,
    summary TEXT,
    content_hash TEXT UNIQUE,
    symbols TEXT,
    tags TEXT,
    published_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);
```

### 7.6 ai_configs 模型配置表

```sql
CREATE TABLE ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    base_url TEXT,
    api_key_ref TEXT,
    masked_api_key TEXT,
    has_api_key BOOLEAN DEFAULT 0,
    model_name TEXT NOT NULL,
    temperature REAL DEFAULT 0.7,
    max_tokens INTEGER DEFAULT 4096,
    timeout_seconds INTEGER DEFAULT 120,
    stream_enabled BOOLEAN DEFAULT 1,
    is_default BOOLEAN DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);
```

### 7.7 prompt_templates 模板表

```sql
CREATE TABLE prompt_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    content TEXT NOT NULL,
    variables TEXT,
    is_builtin BOOLEAN DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);
```

### 7.8 analysis_reports 分析报告表

```sql
CREATE TABLE analysis_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT UNIQUE,
    symbol TEXT NOT NULL,
    title TEXT NOT NULL,
    analysis_type TEXT NOT NULL,
    model_name TEXT,
    prompt_template_id INTEGER,
    input_snapshot TEXT,
    content_markdown TEXT,
    risk_summary TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);
```

`input_snapshot` 属于敏感数据，可能包含用户一次性持仓输入。报告复制和 Markdown 导出默认包含 AI 输出正文，不默认包含完整 `input_snapshot`；用户显式选择“导出输入快照”时才允许导出，并必须提示其中可能包含个人持仓信息。

### 7.9 tasks 任务表

```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    title TEXT,
    progress INTEGER DEFAULT 0,
    error_message TEXT,
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);
```

### 7.10 task_events 任务事件表

```sql
CREATE TABLE task_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX idx_task_events_task_id_id
ON task_events(task_id, id);
```

任务事件用于 SSE 断线恢复和崩溃后的状态回放。Go core 重启时必须扫描 `RUNNING` 任务，并按任务类型标记为 `FAILED` 或 `CANCELLED`，避免任务永久悬挂。

### 7.11 settings 设置表

```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
```

---

## 8. 本地 API 设计

所有 Go core API 仅供 Tauri Rust 层调用，前端不得直接访问 `127.0.0.1:<core_port>`。

API 边界：

```text
1. Go API 全部校验 X-Invest-Compass-Token。
2. Rust command 使用白名单映射到固定 Go API，禁止任意 path 代理。
3. `/internal/*` 仅允许 Rust 内部生命周期管理调用。
4. 所有响应错误必须包含 requestId/traceId。
5. Rust 到 Go sidecar 的所有 HTTP 请求只允许 POST，禁止 GET/PATCH/DELETE/PUT。
6. Go sidecar 对非 POST 请求统一返回 405，并记录脱敏后的安全日志。
7. 读取、查询、删除、更新等语义都通过 POST 路径和 JSON body 表达。
8. API Key、代理密码、用户持仓输入等敏感字段不得出现在响应、日志、错误详情和导出文件中。
9. Go sidecar 统一限制 JSON 请求体大小，只接受 JSON object，并拒绝尾随内容和未知字段，超限或非法 JSON 请求不得进入业务 service 或 dao 层。
```

### 8.1 健康检查

```http
POST /internal/health
```

返回：

```json
{
  "status": "ok",
  "version": "0.1.0",
  "db": "ok"
}
```

### 8.2 股票搜索

```http
POST /api/stocks/search
```

请求：

```json
{
  "keyword": "茅台"
}
```

`keyword` 去除首尾空白后不能为空。Rust `stock_search` 必须在转发前做同样校验，非法参数不得进入 Go core。

返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "symbol": "CN:SH:600519",
      "name": "贵州茅台",
      "code": "600519",
      "market": "CN",
      "exchange": "SH"
    }
  ],
  "traceId": "...",
  "requestId": "..."
}
```

### 8.3 获取行情

```http
POST /api/market/quote
```

请求：

```json
{
  "symbol": "CN:SH:600519"
}
```

响应字段至少包含：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "symbol": "CN:SH:600519",
    "price": 1688.5,
    "change_amount": 12.3,
    "change_percent": 0.73,
    "quote_time": "2026-06-18T10:30:00Z",
    "provider": "provider-name"
  },
  "traceId": "...",
  "requestId": "..."
}
```

Go core 先读取 10-60 秒 quote 短缓存；缓存未命中时才调用 `MarketProvider.Quote`，成功后写入 `quotes`。

### 8.4 获取 K线

```http
POST /api/market/kline
```

请求：

```json
{
  "symbol": "CN:SH:600519",
  "period": "day",
  "adjust": "qfq",
  "limit": 120
}
```

`period` 首版只允许 `day`、`week`、`month`；`adjust` 首版只允许 `none`、`qfq`、`hfq`；`limit` 必须为 1-500。Rust `market_kline` 必须在转发前做同样校验，非法参数不得进入 Go core。

Go core 先读取 `symbol + period + adjust` 对应的 K 线缓存；缓存足量时不重复调用 Provider。返回数组必须按 `trade_date` 升序排列，Provider 返回后按交易日写入 `klines`。

### 8.5 获取技术指标

```http
POST /api/market/indicators
```

请求：

```json
{
  "symbol": "CN:SH:600519",
  "period": "day",
  "adjust": "qfq",
  "limit": 120,
  "indicators": ["ma", "macd", "rsi"]
}
```

`indicators` 首版只允许 `ma`、`ema`、`macd`、`rsi`、`kdj`、`boll`、`volume_ma`、`change_percent`、`max_drawdown`、`volatility`。`limit` 必须为 1-500，Rust `market_indicators` 必须在转发前做同样校验，非法参数不得进入 Go core。

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "symbol": "CN:SH:600519",
    "period": "day",
    "adjust": "qfq",
    "indicators": {
      "ma": {
        "ma5": [null, null, null, null, 102.4]
      },
      "max_drawdown": 2.1
    }
  },
  "traceId": "...",
  "requestId": "..."
}
```

指标由 Go core 基于 K线统一计算。首版不单独落指标缓存表，允许复用 K线缓存；缓存不足时由 Go core 通过行情 Provider 拉取 K 线并写回 `klines`。序列前置空结果使用 JSON `null`，前端不得自行计算业务指标。

### 8.6 自选股

```http
POST /api/watchlist/list
POST /api/watchlist/create
POST /api/watchlist/update
POST /api/watchlist/delete
```

Rust `watchlist_update` 和 `watchlist_delete` 必须在转发前校验 `id > 0`，非法参数不得进入 Go core。

### 8.7 新闻资讯

```http
POST /api/news/list
POST /api/news/market
```

个股新闻请求：

```json
{
  "symbol": "CN:SH:600519",
  "limit": 20
}
```

市场新闻请求：

```json
{
  "market": "CN",
  "limit": 20
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "id": "content-hash",
        "source": "provider-name",
        "title": "新闻标题",
        "url": "https://example.com/news",
        "summary": "摘要",
        "content_hash": "content-hash",
        "published_at": "2026-06-18T09:30:00Z",
        "symbols": ["CN:SH:600519"],
        "tags": ["company"]
      }
    ]
  },
  "traceId": "...",
  "requestId": "..."
}
```

新闻 `limit` 必须为 1-100。Rust `news_list` 和 `news_market` 必须在转发前做同样校验，非法参数不得进入 Go core。

Go core 输出新闻前必须执行 HTTP(S) URL scheme 校验和 `content_hash` 去重。新闻缓存写入 `news_items`，按 30-120 分钟缓存策略读取；缓存不足时通过合规 `news.Provider` 拉取。首版不接公告、研报、资金流专用数据源。

### 8.8 AI 配置

```http
POST /api/ai/configs/list
POST /api/ai/configs/save
POST /api/ai/configs/delete
POST /api/ai/configs/test
```

AI 配置密钥规则：

```text
1. 前端到 Rust command 的 `ai_config_save(payload)` 可以接收一次性明文 API Key。
2. Rust/Tauri 负责写入本地文件 vault，并向 Go core 保存 api_key_ref。
3. `/api/ai/configs/list` 只返回 has_api_key、masked_api_key、api_key_ref，不返回真实 Key。
4. 测试连接失败时不得把请求头、Key、代理认证信息写入错误消息。
5. 日志、trace、导出配置必须经过统一 secret redaction。
6. Go core 不直接写本地 vault，不持久化真实 Key。
7. 新建 AI 配置尚未获得数据库 ID 时，Rust 写入本地 vault 必须生成唯一 api_key_ref，禁止多个新配置因 `id=0` 覆盖同一个密钥文件。
8. 更新 AI Key 时，Rust 必须先校验旧 api_key_ref 是合法本地 vault 引用，再写入新密钥并删除被替换的旧 api_key_ref；如果删除旧引用失败，必须回滚本次新写入的密钥文件，避免历史密钥残留或孤儿 secret 文件。
9. Unix/macOS 下本地 vault 目录必须收紧为 `0700`，secret 文件创建和更新后必须收紧为 `0600`，避免被同机其他用户枚举或读取。
10. AI Provider、本地 vault 引用和代理 profile 清理后的文件名片段不能为空，非法值必须在 Rust 边界拒绝，禁止退化为弱语义文件名或落到 `.secret` 这类隐式文件。
11. Go core 保存 AI 配置和 settings 前必须再次校验凭据引用 scheme，只允许 `local-vault://ai-config/` 和 `local-vault://proxy/`，禁止任意 `_ref` 绕过敏感配置拦截。
12. `masked_api_key` 只能保存空值或包含 `*` / `...` 的脱敏展示文本；Go core 必须拒绝没有脱敏标记的非空值，避免明文 Key 通过展示字段旁路落库。
```

Rust command `ai_config_list()`、`ai_config_save(payload)`、`ai_config_delete(payload)`、`ai_config_test({ id, api_key_ref })` 分别固定映射到 `POST /api/ai/configs/list`、`POST /api/ai/configs/save`、`POST /api/ai/configs/delete`、`POST /api/ai/configs/test`。`ai_config_delete` 和 `ai_config_test` 必须在读取或删除本地 vault 前校验 `id > 0`，非法参数不得误删本地凭据或进入 Go core。Go core 保存接口必须拒绝 `raw_api_key`，真实明文 Key 只能由 Rust command 一次性接收并写入本地文件 vault。

Go core 生产启动时必须注入 OpenAI-compatible 模型连通性 tester。若 tester 未注入，视为后端配置错误，不能把模型测试能力写成可用。

跨平台凭据实现：

```text
macOS / Windows：Rust/Tauri 写入本地文件 vault。
默认目录：macOS 使用 ~/Library/Application Support/Invest Compass/credentials，Windows 使用 %APPDATA%/Invest Compass/credentials。
开发和测试可用 INVEST_COMPASS_CREDENTIAL_DIR 覆盖本地 vault 目录。
SQLite：只保存 api_key_ref、has_api_key、masked_api_key。
模型测试/AI 分析：Rust 从本地 vault 读取真实 Key，通过本次白名单请求传给 Go core；Go 仅在内存中使用，禁止写入日志、数据库和任务事件。
```

Rust 到 Go 的内部密钥注入协议：

```text
1. 前端请求只允许携带 aiConfigId 和可落库的 api_key_ref，不允许携带真实 API Key。
2. Rust command 根据 api_key_ref 从本地 vault 读取真实 Key。
3. Rust 转发给 Go core 时，在内部请求体加入 resolved_api_key。
4. resolved_api_key 不进入前端类型、OpenAPI 对外文档、SQLite、任务事件、报告快照和日志。
5. Go core 收到 resolved_api_key 后只保存在当前请求/任务内存中，AI 调用结束后释放。
6. ai_config_test(id) 和 analysis_task_create(payload) 必须复用同一套密钥注入与脱敏逻辑。
```

### 8.9 Prompt 模板

```http
POST /api/prompt-templates/list
POST /api/prompt-templates/get
POST /api/prompt-templates/create
POST /api/prompt-templates/update
POST /api/prompt-templates/delete
```

首版模板类型只允许 `system`、`stock_full`、`technical`、`custom`。模板保存时必须校验变量白名单，不允许首版未支持的 `announcements`、`reports`、`portfolio` 等变量。

Rust command `prompt_templates_list()`、`prompt_templates_get(id)`、`prompt_templates_create(payload)`、`prompt_templates_update(payload)`、`prompt_templates_delete(id)` 分别固定映射到上述 Go API，禁止通过通用 path 代理调用。涉及模板 ID 的 command 必须在转发前校验 `id > 0`。

### 8.10 分析任务

创建任务：

```http
POST /api/analysis/tasks
```

请求：

```json
{
  "symbol": "CN:SH:600519",
  "analysis_type": "stock_full",
  "ai_config_id": 1,
  "api_key_ref": "local-vault://ai-config/openai-compatible-1",
  "prompt_template_id": 2,
  "user_position": {
    "cost_price": 1680,
    "shares": 100,
    "risk_level": "medium"
  }
}
```

`user_position` 仅用于本次分析上下文和报告输入快照，不作为持仓数据单独落库，也不进入普通日志。Go core 普通日志和任务事件只记录 `has_user_position`；`analysis_reports.input_snapshot` 可以保存本次 `user_position` 详情，但不得包含 `resolved_api_key`、`raw_api_key` 或其他 API Key 字段。`api_key_ref` 是可落库的本地 vault 引用，不是真实 API Key。

以上请求结构是前端到 Rust command 的输入。Rust 转发给 Go core 时会补充内部字段 `resolved_api_key`，该字段不得暴露给前端。`analysis_task_create(payload)` 必须在读取本地 vault 前校验 `symbol`、`analysis_type` 非空、`api_key_ref` 必须是 `local-vault://ai-config/` 引用，并校验 `ai_config_id > 0` 和 `prompt_template_id > 0`；非法请求不得触发凭据读取或进入 Go core。

Go core 创建 `PENDING` 任务和 `TASK_CREATED` 事件后异步启动分析执行器。执行器只能读取已落库的真实行情、K 线、新闻缓存和 Prompt 模板；缺少必要上下文时必须把任务标记为 `FAILED`，不得生成占位行情或假新闻。执行成功后按 `task_id` 幂等保存报告，并写入 `TASK_STARTED`、`TASK_CHUNK`、`TASK_SUCCESS` 等事件。取消任务必须先持久化 `CANCELLED` 状态和 `TASK_CANCELLED` 事件，再取消当前进程内运行中的 executor context；执行器收到 `context.Canceled` 时直接退出，不追加 `TASK_FAILED`。

返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "taskId": "task_abc123"
  },
  "traceId": "...",
  "requestId": "..."
}
```

订阅任务事件：

```http
POST /api/tasks/events/stream
```

该 SSE 接口仅供 Rust 层订阅。前端通过 `analysis_task_subscribe(task_id)` 接收 Tauri Channel 或 event，不直接使用浏览器 `EventSource` 连接 Go core。Go core 首次查询已有待补拉事件时会写出后返回；首次没有新增事件时会保持连接等待新事件，并在 `TASK_SUCCESS`、`TASK_FAILED` 或 `TASK_CANCELLED` 后结束。Rust 订阅 command 必须解析 Go core SSE 帧，兼容 LF/CRLF 帧边界和多行 `data:` 合并语义，并以 `analysis-task-event` 事件名向当前窗口分发结构化 payload，返回本轮分发数量和 `last_event_id` 便于后续补拉。

任务查询与事件回放：

```http
POST /api/tasks/list
POST /api/tasks/get
POST /api/tasks/events
```

`task_list(limit)` 的 `limit` 必须为 1-100，Go core 在 HTTP 边界拒绝无界任务历史查询。`afterEventId` 用于 SSE 断线后补拉事件，必须大于等于 0，其中 0 表示从头补拉。任务历史页必须优先读取任务详情和事件回放，再恢复订阅。

Rust command `task_list(limit)`、`task_get(task_id)`、`task_events(task_id, after_event_id)` 分别固定映射到上述任务查询与事件回放 Go API，禁止通过通用 path 代理调用。`task_get`、`task_events`、`analysis_task_cancel` 和 `analysis_task_subscribe` 必须在 Rust 边界拒绝空 `task_id`；`task_list`、`task_events` 和 `analysis_task_subscribe` 必须在 Rust 边界复用同样的 `limit` / `after_event_id` 校验，非法参数不得转发到 Go core。

Go core 启动时必须扫描 `RUNNING` 任务，并在同一事务中写入恢复后的终态任务和恢复事件，避免 sidecar 崩溃或重启后任务永久悬挂。恢复事件 payload 写入 `task_events` 前必须复用统一 secret redaction。

取消任务：

```http
POST /api/tasks/cancel
```

获取报告：

```http
POST /api/reports/get
POST /api/reports/list
POST /api/reports/delete
```

报告删除使用软删除。`list` 和 `get` 只返回未删除报告，已删除报告详情按未找到处理。
报告历史查询默认不返回完整 `input_snapshot`，避免一次性持仓输入进入普通历史浏览、复制和默认导出路径。
Rust `report_get` 和 `report_delete` 必须在转发前校验 `id > 0`，非法参数不得进入 Go core。

检查更新：

```http
POST /api/update/check
```

检查更新 API 只返回 `PROMPT_ONLY` 版本提示结果，不返回下载、安装或静默升级动作。
manifest URL、下载链接和发布说明链接都必须使用 HTTPS 且命中 allowlist。

### 8.11 Dashboard 聚合

```http
POST /api/dashboard/summary
```

首版 dashboard 只聚合已有能力：

```text
自选股摘要
最近分析报告
最近任务状态
市场新闻摘要
风险提示
```

不做策略、资金流、公告、研报聚合。

Go core 生产环境的 `dashboard/summary` 必须从统一 `dao.Store` 聚合真实本地数据：

- active 自选股列表。
- active 自选股对应的最新 quote 缓存。
- 未软删除的最近分析报告。
- 最近任务状态。
- 市场新闻缓存。
- `MarketProvider.Status` 和新闻 Provider 可选 `Status` 能力返回的数据源可用性。

如果缓存缺失，接口返回空集合或 0 统计，不生成假行情、假新闻或假报告。真实 Market/News Provider 未配置时，`providers/status` 和 `dashboard/summary` 只能分别返回 `available=false`、`source=unconfigured` 的不可用状态；新闻 Provider 如果实现可选 `Status(ctx)`，Dashboard 和 Provider 状态接口必须复用该真实状态，不能因 Provider 非空就假定可用；搜索、行情、K 线和新闻接口仍不得伪造数据。

Rust command `dashboard_summary()` 固定映射到 `POST /api/dashboard/summary`，不得通过通用 path 代理调用。

### 8.12 设置、缓存、日志和工作区

```http
POST /api/settings/get
POST /api/settings/set
POST /api/cache/stats
POST /api/cache/clean
POST /api/logs/export
POST /api/workspace/get
POST /api/workspace/set
POST /api/providers/status
```

这些接口同样只供 Rust 白名单 command 调用。其中 `providers_status()` 固定映射到 `POST /api/providers/status`；`workspace_set(payload)` 必须在 Rust 边界拒绝空路径和相对路径；`logs/export` 必须先脱敏 API Key、代理密码、持仓输入和授权信息。
`POST /api/logs/export` 从 Go core 运行期内存 `slog` 快照生成已二次脱敏的导出包，必须保留 `request_id`、`trace_id`，任务链路日志存在时必须继续保留 `task_id` 方便排障；Go core 不直接写入用户目录。Rust `export_logs(target_dir)` 在调用该 API 前先校验目标目录存在，避免无效目标触发日志包生成；写入导出文件时必须使用不覆盖已有文件的方式，防止用户目录中同名文件被替换；Unix/macOS 下必须使用当前用户私有读写权限创建导出文件。

---

## 9. AI 分析设计

### 9.1 Prompt 分层

每次 AI 分析由三层 Prompt 组成：

```text
System Prompt：角色、边界、风险声明、输出格式
Context Prompt：股票数据、行情、K线、新闻资讯、指标
User Prompt：用户问题、一次性持仓输入、分析目标
```

### 9.2 输出约束

AI 输出必须遵守：

```text
不承诺收益
不直接替用户做买卖决策
必须列出风险点
必须说明数据时效
必须区分事实、推断和观点
必须给出观察指标
必须提醒用户自行决策
```

### 9.3 分析报告模板

```markdown
# {{stock_name}} {{stock_code}} 投研分析

## 1. 核心结论

## 2. 当前行情状态

## 3. 技术面观察

## 4. 基本面观察

## 5. 消息面观察

## 6. 行业与竞争格局

## 7. 风险点

## 8. 后续观察指标

## 9. 说明

本文仅为研究辅助，不构成投资建议。
```

---

## 10. 缓存设计

### 10.1 缓存类型

```text
行情短缓存：10-60 秒
K线缓存：按交易日缓存
新闻缓存：30-120 分钟
AI 报告：永久保存，用户可删除
图片/图表缓存：本地文件缓存
```

### 10.2 缓存清理

清理策略：

```text
按大小清理
按时间清理
手动清理
仅清理临时缓存
保留用户报告和配置
```

---

## 11. 设置与代理

### 11.1 代理设置

支持：

```text
不使用代理
系统代理
HTTP 代理
SOCKS5 代理
按 Provider 单独配置代理
```

配置项：

```text
proxy.mode
proxy.http_url
proxy.socks5_url
proxy.no_proxy
proxy.username
proxy_credential_ref
```

代理认证规则：

```text
1. proxy.http_url / proxy.socks5_url 禁止包含 username/password。
2. 代理用户名可存 SQLite settings。
3. 代理密码必须走 Rust 本地文件 vault，SQLite 只保存 proxy_credential_ref。
4. Rust `settings_set` 可以接收一次性 proxy_password，写入本地 vault 后只向 Go core 转发 proxy_credential_ref。
5. Rust `settings_set` 清理代理凭据时必须删除本地 vault 文件，并向 Go core 清空 proxy_credential_ref。
6. 同一次 `settings_set` 请求不得同时携带新 proxy_password 和 clear_proxy_credential，Rust 必须在读写本地 vault 前拒绝这种冲突请求。
7. macOS / Windows 都使用 Rust 本地文件 vault。
8. 日志和导出配置不得包含代理认证明文。
```

### 11.2 API Key 保存

API Key 不直接明文暴露给前端。

首版固定方案：

```text
1. 使用 Rust 本地文件 vault 保存真实 API Key。
2. SQLite 的 ai_configs 只保存 api_key_ref。
3. 前端只展示 has_api_key 和 masked_api_key。
4. 删除模型配置时同步删除对应凭据项。
```

跨平台实现：

```text
macOS：本地文件 vault。
Windows：本地文件 vault。
```

首版本地文件 vault 不做强加密，安全性低于平台凭据服务；它的边界是避免前端、Go core、SQLite、日志和报告直接接触真实 Key。如果后续需要更强安全性，可以评估 Tauri Stronghold、keyring crate 或平台凭据服务。

---

## 12. 安全设计

### 12.1 本地服务安全

```text
1. Go core 只监听 127.0.0.1。
2. 启动时生成随机 token。
3. 所有请求必须带 X-Invest-Compass-Token。
4. token 只存在 Tauri 内存中，不持久化。
5. 禁止前端直接读取 token。
6. 禁止 Go core 开启任意跨域。
7. 退出应用时销毁 sidecar。
8. token 通过 sidecar stdin 握手传递，禁止通过 argv/env 传递。
9. Rust command 必须白名单化，禁止任意 path 代理。
10. SSE 只能由 Rust 订阅 Go core，再转发为 Tauri event。
```

### 12.2 前端安全

```text
1. 不在 localStorage 存 API Key。
2. 不在前端拼接外部数据源密钥。
3. 不直接执行远程脚本。
4. Markdown 渲染必须做 XSS 清理。
5. 外部链接统一通过系统浏览器打开。
6. 渲染 AI Markdown 前必须同时做标签白名单和 URL scheme 白名单。
```

### 12.3 可观测性与脱敏

```text
1. Rust 为每个前端请求生成 request_id，并透传给 Go core。
2. Go core 为任务生成 task_id，任务日志、SSE 事件、报告快照都必须关联 task_id。
3. 对外部 Provider 调用记录 provider、latency、status、trace_id，不记录密钥和完整请求头。
4. 日志字段统一使用 request_id、trace_id、task_id、provider、symbol。
5. Go core 只回显规范化后的 request_id / trace_id，拒绝把超长、换行或空格等异常头值写入响应和日志。
6. secret redaction 覆盖 API Key、Authorization、Proxy-Authorization、代理密码、license key、用户持仓输入。
7. 首版不暴露公网 `/metrics`；本地调试可以在开发模式输出 Provider latency、error_count、SSE reconnect_count。
8. Rust command、Go handler、Provider 调用的 span 命名分别使用 desktop.<command>、core.<handler>、provider.<name>。
```

### 12.4 数据合规

```text
1. 明确标注数据来源。
2. 不绕过需要授权的数据服务。
3. 对抓取频率做限制。
4. 对用户声明数据仅供研究。
5. 不提供规避数据源限制的能力。
```

### 12.5 投资合规边界

```text
1. 应用定位为投研辅助工具。
2. 输出不构成投资建议。
3. 不做收益承诺。
4. 不展示“必涨”“稳赚”“买入信号”等诱导表达。
5. 不接交易账户。
6. 不自动下单。
```

---

## 13. 自动更新与发布

### 13.1 更新策略

首版仅实现手动检查更新和版本提示，不执行下载、安装和静默升级。真实自动更新放到 `0.5.0`。

首版检查更新流程：

```text
1. 用户点击检查更新后读取更新配置。
2. Rust `check_update()` 固定调用 Go core `POST /api/update/check`。
3. Go core 优先使用静态配置；静态配置为空时从 settings 表读取 `update.manifest_url` 和逗号分隔的 `update.allowed_hosts`。
4. Go core 校验更新 JSON URL 的 HTTPS 和 allowlist 后拉取 manifest。
5. Go core 拉取 manifest 时拒绝 HTTP 3xx 重定向，避免跳转到未校验来源。
6. Go core 拒绝超过 256 KiB 的 manifest 响应体，不接受截断后的更新 JSON。
7. Go core 校验 manifest 内下载链接和发布说明链接仍命中 allowlist。
8. 发现新版本后只提示用户，并展示下载页面或发布说明链接。
```

首版检查更新信任边界：

```text
1. 更新 JSON 必须来自 HTTPS。
2. 域名必须在内置或 settings 配置的 allowlist 中。
3. 展示的下载链接和发布说明链接也必须匹配 allowlist。
4. manifest 拉取不跟随 HTTP 3xx 重定向。
5. manifest 响应体最大 256 KiB，超限直接返回失败。
6. 外链只允许 https scheme，并通过系统浏览器打开。
7. 不允许从更新 JSON 执行脚本、命令或自定义 URL scheme。
```

后续自动更新流程：

```text
1. 固定 Tauri updater public key。
2. 请求签名后的更新 manifest。
3. 校验 manifest 签名。
4. 下载签名后的安装包。
5. 校验安装包签名和 sidecar hash。
6. 检查 desktop/core 协议版本兼容性。
7. 迁移前备份数据库。
8. 安装更新。
9. 失败时回滚或提示用户重新安装。
```

### 13.2 版本规范

```text
0.1.0：MVP
0.2.0：AI 分析增强
0.3.0：资讯/研报增强
0.4.0：授权激活
0.5.0：自动更新
1.0.0：正式商业版本
```

### 13.3 打包目标

```text
Windows x64
macOS Apple Silicon
macOS Intel
macOS Universal，可选
```

跨平台打包要求：

```text
1. macOS 分别产出 aarch64-apple-darwin 与 x86_64-apple-darwin sidecar；Universal 包可选。
2. Windows 产出 x86_64-pc-windows-msvc sidecar，文件名带 .exe。
3. sidecar 按 Tauri externalBin 规则随包分发，不从运行时下载。
4. `scripts/build-sidecar.mjs` 默认构建当前平台 sidecar，也必须支持 `--target=<triple>` 和 `--all-targets` 选择首版三类目标。
5. desktop 与 core 必须带 protocolVersion，启动时校验兼容性。
6. Windows sidecar 必须使用 GUI subsystem 构建，避免桌面应用启动 sidecar 时弹出控制台窗口。
7. 发布包运行时优先解析主程序同目录的 `invest-compass-core` / `invest-compass-core.exe`；环境变量覆盖仅用于本地调试，源码目录 `src-tauri/binaries` 只作为开发 fallback。
8. 应用级退出事件必须显式停止 Go sidecar，不能依赖进程退出后的 Drop 清理。
9. 打包前必须实际生成 Apple Silicon、macOS Intel 和 Windows x64 三类 sidecar 产物；安装包启动验收仍必须在目标平台完成。
10. macOS 发布需要 Developer ID 签名和 notarization。
11. Windows 发布需要代码签名证书；未签名包只允许开发/内部测试。
12. Windows 安装包优先 NSIS，MSI 作为后续可选。
```

sidecar 命名：

```text
invest-compass-core-aarch64-apple-darwin
invest-compass-core-x86_64-apple-darwin
invest-compass-core-x86_64-pc-windows-msvc.exe
```

---

## 14. 开发环境

### 14.1 基础工具

```text
Node.js
pnpm
Rust toolchain
Tauri CLI
Go toolchain
SQLite
Git
```

### 14.2 本地开发命令

```bash
cd apps && pnpm install
cd apps && pnpm dev
cd apps && pnpm build
cd apps/desktop && pnpm tauri dev
cd apps/desktop && pnpm tauri build
```

Go sidecar 单独运行：

```bash
cd apps/sidecar-core
go run ./cmd/invest-compass-core \
  --host=127.0.0.1 \
  --port=18888 \
  --workspace=../../.data \
  --auth-stdin
```

本地调试时从 stdin 输入一次性握手 JSON，例如 `{"token":"<dev-token>"}`。不要把真实 token 写进命令行参数、环境变量或脚本。

---

## 15. 测试方案

### 15.1 Go 单元测试

覆盖：

```text
股票代码解析
Provider 适配
K线数据解析
技术指标计算
Prompt 构建
AI Provider Mock
任务状态流转
数据库迁移
缓存清理
```

### 15.2 前端测试

覆盖：

```text
页面渲染
表格筛选
股票搜索
AI 分析任务状态
设置保存
错误提示
Markdown 渲染
```

### 15.3 集成测试

覆盖：

```text
Tauri 启动 sidecar
sidecar ready 检测
前端 invoke 到 Rust
Rust 代理到 Go
Go 返回结果
任务 SSE 事件
应用退出时清理 sidecar
Rust 本地文件 vault 凭据保存与删除
macOS / Windows sidecar 启动握手
Tauri capability 权限拒绝用例
```

### 15.4 手工验收

验收场景：

```text
首次启动
设置工作区
配置 AI 模型
测试模型连接
搜索股票
添加自选股
查看个股详情
生成 AI 分析
保存报告
查看任务历史
清理缓存
关闭到托盘
重新打开
检查更新
```

---

## 16. 首版里程碑

### Milestone 0：项目骨架

交付物：

```text
monorepo 初始化
Tauri 桌面壳
React 前端
Go core 服务
sidecar 启动
健康检查
统一日志
sidecar stdin token 握手
Rust command 白名单
Tauri v2 capabilities / CSP 基线
macOS / Windows sidecar 二进制命名
```

验收标准：

```text
应用可启动
Go core 可随 Tauri 启动
前端可调用 health
退出时 sidecar 被关闭
token 不出现在 argv/env/日志
macOS 和 Windows 均可完成 sidecar 握手
```

### Milestone 1：股票基础能力

交付物：

```text
股票搜索
股票基础表
自选股
行情快照
K线接口
技术指标接口
K线图展示
```

验收标准：

```text
可以搜索股票
可以添加自选
可以查看价格和K线
可以查看首版技术指标
数据可以缓存和刷新
```

### Milestone 2：AI 模型与 Prompt

交付物：

```text
AI Provider 抽象
OpenAI-compatible 接入
模型配置页
Prompt 模板页
模型连通性测试
Rust 本地文件 vault 保存 API Key
```

验收标准：

```text
用户可配置 API Key
可测试模型
可创建 Prompt 模板
可调用 AI 返回文本
配置查询不会回显真实 API Key
Prompt 模板 CRUD 可用且变量校验生效
```

### Milestone 3：个股 AI 分析闭环

交付物：

```text
分析任务
流式输出
分析报告保存
报告历史
任务历史
任务取消
SSE 断线后可重新获取任务状态
报告列表/详情/删除
```

验收标准：

```text
用户选择股票后可以生成分析报告
生成过程可见
失败原因可见
报告可保存、查看、复制
sidecar 重启后 RUNNING 任务不会永久悬挂
任务历史可查询详情和事件回放
```

### Milestone 4：设置中心和桌面能力

交付物：

```text
开机自启
关闭到托盘
系统通知
工作区设置
缓存管理
代理设置
日志导出
检查更新入口
```

验收标准：

```text
设置可保存
开机自启可开关
任务成功/失败有通知
缓存可查看和清理
代理可配置
日志导出会脱敏
检查更新只提示版本，不执行安装
更新链接只允许 HTTPS allowlist 域名
```

### Milestone 5：发布准备

交付物：

```text
Windows 安装包
macOS 安装包
检查更新入口
风险声明
关于页面
README
用户手册
```

验收标准：

```text
安装包可安装
检查更新可提示新版本或当前已是最新
用户能看懂风险说明
README 可以指导开发和使用
macOS 包完成签名/公证要求，Windows 包具备签名方案
```

---

## 17. 任务拆分

### 17.1 桌面壳任务

```text
desktop-001 初始化 Tauri 项目
desktop-002 实现 sidecar 启动
desktop-003 实现 sidecar 健康检查
desktop-004 实现 sidecar stdin token 握手
desktop-005 实现 Rust command 白名单代理
desktop-006 实现任务事件转发
desktop-007 实现托盘菜单
desktop-008 实现窗口状态恢复
desktop-009 实现开机自启
desktop-010 实现系统通知
desktop-011 实现检查更新入口
desktop-012 实现日志导出脱敏
desktop-013 实现 Tauri capabilities / CSP 基线
desktop-014 实现 Rust 本地文件 vault 适配
desktop-015 实现 AI Key 内部注入协议
```

### 17.2 Go core 任务

```text
core-001 初始化 Go 项目
core-002 实现 HTTP server
core-003 实现 token 中间件
core-004 实现 SQLite 初始化
core-005 实现数据库迁移
core-006 实现股票代码模型
core-007 实现股票搜索 Provider
core-008 实现行情 Provider
core-009 实现 K线 Provider
core-010 实现自选股 CRUD
core-011 实现技术指标
core-012 实现 AI Provider 抽象
core-013 实现 OpenAI-compatible Provider
core-014 实现 Prompt 模板
core-015 实现分析任务
core-016 实现任务事件持久化
core-017 实现报告保存
core-018 实现设置中心接口
core-019 实现缓存管理
core-020 实现日志导出
core-021 实现 SSE 事件
core-022 实现启动时 RUNNING 任务恢复
core-023 实现 secret redaction
core-024 实现 Prompt 模板 CRUD
core-025 实现任务查询和事件回放
core-026 实现报告查询和删除
core-027 实现 dashboard summary
core-028 实现新闻列表/市场新闻查询
core-029 实现 provider status
```

### 17.3 前端任务

```text
ui-001 初始化 React 项目
ui-002 实现布局框架
ui-003 实现总览页
ui-004 实现自选股页
ui-005 实现个股详情页
ui-006 实现 K线图
ui-007 实现 AI 分析页
ui-008 实现模型配置页
ui-009 实现 Prompt 模板页
ui-010 实现报告历史页
ui-011 实现任务历史页
ui-012 实现设置中心
ui-013 实现错误页和加载态
ui-014 实现关于页和版本检查入口
```

---

## 18. 风险与应对

### 18.1 数据源不稳定

风险：免费行情、新闻数据源可能变动。
应对：

```text
Provider 抽象
后续预留备用 Provider
缓存兜底
错误降级
数据源状态页
```

### 18.2 AI 输出不稳定

风险：AI 可能幻觉、误判、输出过度武断。
应对：

```text
固定输出格式
加入事实/推断分区
保留输入数据快照
强制风险提示
允许用户查看引用数据
```

### 18.3 Sidecar 生命周期异常

风险：Go core 崩溃、端口占用、僵尸进程。
应对：

```text
随机端口
health check
崩溃自动重启
启动时扫描 RUNNING 任务并标记为 FAILED/CANCELLED
任务事件持久化
报告保存按 task_id 幂等
SSE 断线后重新拉取任务状态
退出 kill 兜底
日志保留
```

### 18.4 跨平台差异

风险：Windows/macOS 在托盘、通知、签名、路径权限上差异明显。
应对：

```text
平台适配层
CI 多平台构建
安装包手工验收
路径全部通过系统目录获取
macOS 与 Windows 本地文件 vault 分别验收
sidecar 二进制按 target triple 命名并随包分发
```

### 18.5 GPL 许可证风险

风险：直接复制 GPLv3 项目代码可能污染商业闭源项目。
应对：

```text
只参考产品思路和架构
不复制 go-stock 源码
核心模块自行实现
第三方依赖逐个检查 License
保留依赖 License 清单
```

### 18.6 本地密钥泄露风险

风险：API Key、代理密码、license key 或用户持仓输入可能通过日志、错误响应、导出文件泄露。
应对：

```text
真实 API Key 存 Rust 本地文件 vault
配置查询接口只返回脱敏状态
统一 secret redaction
日志导出前二次脱敏
测试连接错误不包含请求头和密钥
```

### 18.7 WebView 权限滥用风险

风险：如果 WebView 出现 XSS，攻击者可能调用高权限 Tauri command。
应对：

```text
Tauri v2 capabilities 最小授权
敏感 command 只允许 main/settings 窗口调用
CSP 禁止远程脚本
外链 scheme/host allowlist
插件权限按功能拆分
```

---

## 19. 首版验收标准

首版达到以下标准才算可发布内部测试：

```text
1. 应用可在 Windows 和 macOS 启动。
2. Go sidecar 可自动启动和退出。
3. 用户可以搜索并添加自选股。
4. 用户可以查看个股行情和 K线。
5. 用户可以配置至少一个 AI 模型。
6. 用户可以基于股票生成 AI 分析报告。
7. 分析过程支持流式输出或进度展示。
8. 报告可以保存、查看、复制。
9. 任务失败后可以看到错误原因。
10. 设置中心支持工作区、代理、通知、缓存。
11. 应用有明确风险提示。
12. 数据库升级不会丢失用户已有数据。
13. API Key 存入 Rust 本地文件 vault，配置查询不回显真实 Key。
14. 前端不能直接访问 Go sidecar，Rust command 必须白名单化。
15. 首版不出现策略观察、授权激活、公告/研报/资金流等未闭环入口。
16. macOS / Windows 使用 Rust 本地文件 vault，凭据保存/删除均通过验收。
17. Prompt 模板、任务历史、报告历史、技术指标均有 Rust command 和 Go API 闭环。
18. 检查更新链接只允许 HTTPS allowlist 域名。
19. 总览页、资讯中心和数据源状态均有 Rust command 和 Go API 闭环。
20. AI Key 只通过 Rust 内部注入字段传给 Go core，不暴露给前端类型和持久化存储。
```

---

## 20. 推荐开发顺序

不要先做复杂 UI，也不要先做全部数据源。最合理的顺序是：

```text
1. Tauri + Go sidecar 跑通。
2. React 调用 health 跑通。
3. SQLite 初始化和迁移跑通。
4. 股票搜索跑通。
5. 自选股跑通。
6. K线图跑通。
7. AI Provider 跑通。
8. AI 分析任务跑通。
9. 报告保存跑通。
10. 设置中心补齐。
11. 桌面能力补齐。
12. 打包发布补齐。
```

核心原则：先打通主链路，再扩展数据源和分析能力。
