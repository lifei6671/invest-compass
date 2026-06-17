# Invest Compass Agent Guide

> 面向进入本仓库工作的 AI / Agent。目标是快速理解项目边界、优先级、安全约束和验证方式。

---

## 1. 项目速读

项目名：投研罗盘 Invest Compass

定位：面向个人投资者和技术型研究者的本地 AI 投研桌面工作台。

首版目标：完成一个可独立运行、可配置 AI 模型、可查看股票行情和 K 线、可生成个股 AI 分析报告、可保存报告和任务历史的桌面应用。

首版不是：

- 自动交易软件
- 荐股软件
- 券商账户工具
- 云同步或多人协作系统
- 复杂量化回测平台

任何 UI、文档、Prompt、报告输出都必须保留“仅作研究辅助，不构成投资建议”的边界。

---

## 2. 必读文档

开始任何实现前，先读：

1. `docs/2026-06-17-invest-compass-technical-solution.md`
   - 当前权威技术方案。
   - 包含产品范围、架构、安全、API、数据库、页面、测试、验收标准。

2. `docs/2026-06-17-invest-compass-implementation-checklist.md`
   - 当前权威实施任务清单。
   - 使用 `T00-T44` 任务号推进。
   - 使用 `RG1-RG6` 作为阶段 Review Gate。

如果技术方案和 checklist 冲突：

- 先不要自行猜测。
- 优先按更具体、更接近实施的 checklist 推进。
- 同时在最终说明中指出冲突点。
- 涉及安全、数据库、公共 API、权限、依赖时必须请求确认。

---

## 3. 当前首版范围

### 必须交付

- Tauri v2 桌面壳。
- React + TypeScript + Vite 前端。
- Go sidecar core。
- SQLite 本地库。
- `sqlc + database/sql` 数据访问。
- `slog` 结构化日志。
- Rust command 白名单代理。
- Go sidecar stdin token 握手。
- 股票搜索、自选股、行情、K 线、技术指标、新闻资讯。
- OpenAI-compatible AI Provider。
- 模型配置、Prompt 模板、模型连通性测试。
- 系统凭据管理器保存 API Key 和代理密码。
- 个股 AI 分析任务、任务事件、报告保存、报告历史、任务历史。
- 设置中心：工作区、代理、通知、缓存、开机自启、检查更新入口。
- macOS 和 Windows 基础启动、凭据、sidecar、打包验收。

### 明确不做

- 不接券商账户。
- 不做实盘交易和自动下单。
- 不做收益承诺。
- 不做云端同步。
- 不做多人协作。
- 不做移动端。
- 不做策略观察独立页面。
- 不做公告、研报、资金流专用数据源。
- 不做持仓信息落库。
- 不做授权激活闭环。
- 不做真实自动更新下载和安装。

首版 UI 不得出现这些未闭环能力的可点击入口。

---

## 4. 架构边界

目标架构：

```text
React 前端
  ↓ Tauri invoke / event
Rust / Tauri Core
  ↓ localhost HTTP / SSE
Go Sidecar Core
  ↓
SQLite + 外部数据源 + AI Provider
```

职责划分：

- React：页面、图表、表格、设置、任务状态展示。
- Rust：桌面能力、安全代理、sidecar 生命周期、系统凭据、权限收口。
- Go：行情、新闻、指标、Prompt、AI 调用、任务、缓存、数据库。
- SQLite：本地业务数据，不保存真实 API Key 和代理密码。

不要把复杂投研业务塞进 Rust。
不要让前端直接访问 Go sidecar。
不要让 Go core 持久化系统凭据。

---

## 5. 安全硬边界

以下规则不能用临时方案绕过。

### Sidecar

- Go sidecar 只监听 `127.0.0.1`。
- runtime token 每次启动随机生成。
- token 只存在 Rust 内存中。
- token 禁止出现在 argv、env、日志、配置文件、数据库。
- token 通过 sidecar stdin 单行 JSON 握手传递。
- Go core 在 token 安装成功前不得注册业务路由。
- Go API 全部校验 `X-Invest-Compass-Token`。
- Go API 全部只接受 POST。
- 非 POST 请求返回 405。

### Rust command

- 禁止实现 `core_request(method, path, body)` 这类任意路径代理。
- 每个 Rust command 必须固定：
  - Go API path
  - HTTP method
  - request schema
  - response schema
  - 可调用窗口或能力范围
- `/internal/*` 只能由 Rust 内部生命周期管理调用。
- SSE 只能由 Rust 订阅 Go core，再转发为 Tauri event / channel。
- 前端不得直接使用浏览器 `EventSource` 连接 Go core。

### 凭据

- API Key 保存到系统凭据管理器：
  - macOS：Keychain
  - Windows：Credential Manager
- 代理密码也保存到系统凭据管理器。
- SQLite 只保存 `api_key_ref`、`proxy_credential_ref`、`has_api_key`、`masked_api_key`。
- 前端到 Rust 可以一次性提交明文 API Key。
- Go core 只能在当前请求或任务内存中使用 `resolved_api_key`。
- `resolved_api_key` 不得进入前端类型、OpenAPI、SQLite、任务事件、报告快照、日志。

### 日志和导出

secret redaction 必须覆盖：

- API Key
- `Authorization`
- `Proxy-Authorization`
- 代理密码
- license key
- 用户一次性持仓输入

日志导出前必须二次脱敏。

---

## 6. 数据库规则

本地数据库使用 SQLite。

访问方式：

- 使用 `sqlc + database/sql`。
- 不要在业务 handler 中手拼 SQL。
- 重要写操作使用事务。

表设计原则：

- 所有业务表必须包含 `created_at`、`updated_at`。
- 重要业务表使用软删除。
- migration 必须可重复验证。
- 已发布 migration 不得随意改写。
- 升级前需要备份策略。

首版核心表：

- `stocks`
- `watchlists`
- `quotes`
- `klines`
- `news_items`
- `ai_configs`
- `prompt_templates`
- `analysis_reports`
- `tasks`
- `task_events`
- `settings`

任务恢复规则：

- Go core 启动时必须扫描 `RUNNING` 任务。
- 按任务类型标记为 `FAILED` 或 `CANCELLED`。
- 禁止让任务永久停在 `RUNNING`。
- 报告保存必须按 `task_id` 幂等。

---

## 7. API 和事件规则

Go core API 只供 Rust 层调用，前端不得直接调用。

统一响应结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

错误响应必须包含：

- `code`
- `message`
- `traceId`
- `requestId`

任务事件必须支持：

- `TASK_CREATED`
- `TASK_STARTED`
- `TASK_PROGRESS`
- `TASK_LOG`
- `TASK_CHUNK`
- `TASK_SUCCESS`
- `TASK_FAILED`
- `TASK_CANCELLED`

任务事件必须持久化到 `task_events`，用于：

- SSE 断线恢复
- 任务历史详情
- 崩溃后的状态回放

---

## 8. 前端规则

技术栈：

- React
- TypeScript
- Vite
- Ant Design
- Tailwind CSS
- Lightweight Charts
- ECharts

页面必须绑定真实 command。

允许：

- 空状态
- 加载态
- 错误态
- 明确的“暂未实现”静态说明

禁止：

- 假数据伪装真实能力
- 假按钮
- 保存了但运行时不生效的配置
- 首版非目标能力入口
- 在 localStorage / sessionStorage / IndexedDB 保存 API Key
- 前端拼接外部数据源密钥

Markdown 渲染：

- 必须做标签白名单。
- 必须做 URL scheme 白名单。
- 外部链接统一通过系统浏览器打开。

---

## 9. AI 和投研合规规则

AI 输出必须包含：

- 数据时效说明
- 风险提示
- 非投资建议声明
- 事实、推断、观点的区分
- 后续观察指标

AI 输出禁止：

- 承诺收益
- 直接替用户做买卖决策
- 使用“必涨”“稳赚”“买入信号”等诱导表达

一次性持仓输入：

- 只允许通过分析任务请求传入。
- 只用于本次分析上下文和报告输入快照。
- 不作为持仓数据单独落库。
- 不进入普通日志。
- 默认 Markdown 导出不包含完整 `input_snapshot`。

Prompt 模板首版只允许：

- `system`
- `stock_full`
- `technical`
- `custom`

首版模板变量只允许技术方案中列出的白名单变量。

---

## 10. 推荐开发顺序

严格优先打通主链路：

1. Tauri + Go sidecar 跑通。
2. React 调用 health 跑通。
3. SQLite 初始化和 migration 跑通。
4. 股票搜索跑通。
5. 自选股跑通。
6. K 线图跑通。
7. AI Provider 跑通。
8. AI 分析任务跑通。
9. 报告保存跑通。
10. 设置中心补齐。
11. 桌面能力补齐。
12. 打包发布补齐。

不要先做复杂 UI。
不要先接全部数据源。
不要先做授权、自动更新、策略观察等后续能力。

---

## 11. Checklist 工作流

实施必须对齐：

`docs/2026-06-17-invest-compass-implementation-checklist.md`

推进规则：

- 每次只勾选已经实现且验证通过的任务。
- 不要批量勾选依赖任务。
- 任务状态使用 checklist 中定义的 `[ ]`、`[~]`、`[x]`、`[!]`。
- 修改公共 API、数据库、权限、安全边界时，同步更新技术方案或 checklist。
- 如果实现发现方案有问题，先记录冲突和建议，不要静默改方向。

Review Gate：

- RG1：sidecar 安全启动门禁
- RG2：基础数据闭环门禁
- RG3：AI 和凭据门禁
- RG4：分析任务门禁
- RG5：首版页面门禁
- RG6：发布验收门禁

未通过对应 Review Gate 前，不要宣称该阶段完成。

---

## 12. 并行工作规则

适合并行：

- Go core 独立模块实现
- Rust command 白名单映射
- 前端页面在接口契约稳定后的实现
- Provider mock 和单元测试
- 文档和验收报告整理

不适合并行：

- 同一 migration 文件多代理同时修改
- 同一 Rust command 契约多代理同时修改
- 同一 API schema 多代理同时修改
- sidecar token、SSE、凭据注入、安全脱敏等关键链路拆成互相猜测的实现

下发子任务时必须说明：

- 代理名称
- 任务定义
- 执行动作
- 预期结果
- 可修改文件范围
- 不可触碰边界
- 验证命令

---

## 13. 验证命令

仓库骨架落地前，docs-only 修改至少运行：

```bash
git diff --check
```

仓库骨架落地后，不要凭空发明命令。按以下顺序找：

1. 当前目录或更近目录的 `AGENTS.md` / `AGENTS.override.md`
2. `Makefile`
3. `package.json`
4. `go.mod`
5. `Cargo.toml`
6. README 或 docs 中明确的命令

初始建议命令，实际以仓库文件为准：

```bash
pnpm install
pnpm build
pnpm test
go test ./...
cargo check
pnpm tauri build
```

长时间测试和构建需要设置合理超时。

跨平台能力必须做真实验收：

- macOS：启动、Keychain、通知、托盘、sidecar、打包、签名/公证方案。
- Windows：启动、Credential Manager、通知、托盘、sidecar、NSIS/MSI、签名方案。

不能完成验证时，最终回复必须说明：

- 未执行的命令
- 未执行原因
- 影响范围
- 剩余风险

---

## 14. 文档同步规则

需要同步文档的情况：

- MVP 范围变化。
- Rust command、Go API、SSE 事件、错误结构变化。
- SQLite schema、migration、任务恢复策略变化。
- API Key、代理密码、凭据、日志脱敏规则变化。
- 前端页面新增或移除入口。
- 打包、发布、签名、更新策略变化。
- 验证命令或 Review Gate 变化。

同步目标：

- 技术方案：`docs/2026-06-17-invest-compass-technical-solution.md`
- 实施清单：`docs/2026-06-17-invest-compass-implementation-checklist.md`
- README：用户或开发入口变化时更新。
- AGENTS.md：长期项目规则变化时更新。

不要把未实现能力写成已完成。
不要把手工验收缺口写成自动化已覆盖。

---

## 15. 提交前检查

提交或交付前至少确认：

- 没有真实密钥、Token、密码。
- 没有明文 API Key 示例。
- 没有弱化 sidecar token、Rust 白名单、SSE 转发、凭据存储边界。
- 没有首版非目标入口。
- checklist 只勾选已验证任务。
- 文档与代码行为一致。
- `git diff --check` 通过。
- 相关语言栈测试或构建已运行，或明确说明未运行原因。

---

## 16. 当前仓库状态提示

当前仓库处于项目早期规划阶段。

已存在：

- README
- 技术方案
- 实施任务清单

尚未落地完整代码骨架时：

- 不要假设 `package.json`、`go.mod`、`Cargo.toml` 已存在。
- 不要直接运行不存在的构建命令。
- docs-only 变更使用 `git diff --check` 做最小验证。

后续一旦完成 T01，需要回到本文件更新实际命令和目录结构。
