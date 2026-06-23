# 投研罗盘 UI 与后端对接总账

> 日期：2026-06-22
>
> 目标：重新梳理当前所有 UI 界面、按钮交互、展示数据、数据来源、增删改方式、链接跳转、配置存储与使用、统计展示方式，并对照已实现的前端 typed service、Rust command 和 Go API，整理缺失数据、接口和页面。
>
> 边界：本文是对接盘点文档，不新增产品范围。任何界面功能如果数据库、Go API、Rust command 或后端 service 不支持，后续接入时必须先找用户确认处理方案，不能继续 mock 接口或 mock 数据。
>
> 开发推进清单：`docs/2026-06-22-invest-compass-settings-ui-backend-development-checklist.md`。后续开发以该清单的任务编号、验收标准和 Review Gate 为准，本文作为 UI 页面、按钮、数据来源和后端接口映射的明细依据。

---

## 1. 当前代码入口

### 1.1 前端路由

当前前端入口为 `apps/frontend/src/app/App.tsx`，生效路由如下：

- `/`：总览 `DashboardPage`
- `/watchlist`：自选股 `WatchlistPage`
- `/stocks/:symbol`：个股详情 `StockDetailPage`
- `/analysis`：AI 分析 `AnalysisPage`
- `/analysis/running`：AI 分析运行页 `AnalysisRunningPage`
- `/reports`：报告历史 `ReportHistoryPage`
- `/reports/:reportId`：报告详情 `ReportDetailPage`
- `/news`：资讯中心 `NewsCenterPage`
- `/tasks`：任务历史 `TaskHistoryPage`
- `/settings`：设置中心 `SettingsPage`
- `/scheduler`：任务调度，隐藏导航入口，`AppShell` 中存在 sr-only 链接
- `/ai-settings`：旧版模型与 Prompt 接口验证页，隐藏导航入口

### 1.2 统一调用边界

前端真实接口必须经由：

```text
UI 页面 / 组件
  -> apps/frontend/src/services/coreClient.ts 或 services/scheduler.ts
  -> Tauri invoke 固定 Rust command
  -> Go sidecar 固定 POST API
  -> service / dao / provider / SQLite
```

禁止：

- 页面直接访问 Go sidecar 地址。
- 页面拼接外部数据源密钥。
- 页面把 mock 数据伪装成真实行情、报告、任务、配置或 Provider 状态。
- 页面保存真实 API Key、Cookie、Token、代理密码到 localStorage、sessionStorage 或 IndexedDB。

### 1.3 已实现 Rust command 与 Go API 总览

| 能力 | 前端 service | Rust command | Go API |
| --- | --- | --- | --- |
| 健康检查 | `coreHealth` | `core_health` | `/internal/health` |
| Dashboard 汇总 | `dashboardSummary` | `dashboard_summary` | `/api/dashboard/summary` |
| Provider 状态 | `providersStatus` | `providers_status` | `/api/providers/status` |
| 股票搜索 | `stockSearch` | `stock_search` | `/api/stocks/search` |
| 行情报价 | `marketQuote` | `market_quote` | `/api/market/quote` |
| K 线 | `marketKline` | `market_kline` | `/api/market/kline` |
| 技术指标 | `marketIndicators` | `market_indicators` | `/api/market/indicators` |
| 自选股 | `watchlistList/create/update/delete` | 同名 command | `/api/watchlist/list/create/update/delete` |
| 新闻 | `newsList/newsMarket` | `news_list/news_market` | `/api/news/list/market` |
| 搜索索引 | `searchReports/searchNews/searchWatchlistNotes/searchStatus/searchRebuild` | 同名 command | `/api/search/*` |
| AI 配置 | `aiConfigList/save/test/delete` | 同名 command | `/api/ai/configs/*` |
| Prompt 模板 | `promptTemplatesList/get/create/update/delete` | 同名 command | `/api/prompt-templates/*` |
| AI 分析任务 | `analysisTaskCreate/cancel/subscribe` | 同名 command | `/api/analysis/tasks`、`/api/tasks/cancel`、SSE 转发 |
| 任务列表与事件 | `taskList/taskGet/taskEvents` | 同名 command | `/api/tasks/list/get/events` |
| 任务日志 | `taskLogsList/get/summary/diagnosis/context/export` | 同名 command | `/api/tasks/logs/*` |
| 报告 | `reportList/get/delete` | 同名 command | `/api/reports/list/get/delete` |
| 设置 | `settingsGet/settingsSet` | 同名 command | `/api/settings/get/set` |
| 工作区 | `workspaceGet/workspaceSet` | 同名 command | `/api/workspace/get/set` |
| 缓存 | `cacheStats/cacheClean` | 同名 command | `/api/cache/stats/clean` |
| 开机自启 | `autostartGet/autostartSet` | 同名 command | Tauri 桌面能力 |
| 检查更新 | `checkUpdate` | `check_update` | `/api/update/check` |
| 日志导出 | `exportLogs` | `export_logs` | `/api/logs/export` |
| 外链打开 | `openExternalURL` | `open_external_url` | Tauri 桌面能力 |
| 调度 | `services/scheduler.ts` | `scheduler_*` | `/api/scheduler/*` |

---

## 2. 页面总览矩阵

| 页面 | 当前 UI 状态 | 当前真实接入 | 主要缺口 |
| --- | --- | --- | --- |
| AppShell / TopBar | 已有全局壳层 | 全局股票搜索接 `stockSearch`；刷新按钮调用 `dashboardStore.load()`；状态卡读取 dashboard store | 通知按钮未接应用内通知 Badge/浮层；系统通知 helper 未消费事件；窗口控制未接；Dashboard 页面会覆盖 store 为空状态 |
| 总览 | 静态金融工作台卡片 | 代码中存在 `dashboardStore` 真实编排，但当前页面 mount 时写入空态 | 需要恢复真实 `dashboardSummary/coreHealth/watchlistList/marketQuote/marketKline` 数据流 |
| 自选股 | 表格/卡片和添加弹窗完成 | 添加弹窗搜索接 `stockSearch`；备注搜索接 `searchWatchlistNotes` | 列表、新增、删除、编辑未接 `watchlist*`；行情字段未接 `marketQuote`；刷新未接 |
| 个股详情 | 空态页面完成 | 无 | 应接 `marketQuote/marketKline/marketIndicators/newsList`；缺公司资料、行业、概念、标签备注持久化接口 |
| AI 分析 | 静态配置和上下文预览 | 无 | 应接 AI 配置、Prompt、行情上下文和 `analysisTaskCreate` |
| AI 分析运行页 | 空运行页 | 无 | 应接 `analysisTaskSubscribe/taskEvents/analysisTaskCancel` |
| 报告历史 | 列表、筛选、统计面板完成 | 关键词搜索接 `searchReports`；删除接 `reportDelete` | 默认列表未接 `reportList`；统计面板静态；批量、导出、收藏未接 |
| 报告详情 | 详情结构完成 | `reportGet` | 导出、重新分析、删除、收藏、输入快照未接 |
| 资讯中心 | 列表、筛选、侧栏完成 | 关键词搜索接 `searchNews`；打开原文接 `openExternalURL` | 默认列表未接 `newsMarket/newsList`；侧栏统计、热点、缓存清理、加入上下文未接 |
| 任务历史 | 列表、筛选、详情抽屉完成 | `TaskLogDrawer` 接任务日志接口 | 主列表未接 `taskList/taskGet/taskEvents`；取消、重试、报告跳转未接 |
| 任务调度 | 隐藏页面，可用 | 已接 `scheduler*` 和 `providersStatus` | 未进入主导航；视觉未统一到最新设置中心风格 |
| 设置-基础设置 | 卡片完成 | 缓存统计/清理接 `cacheStats/cacheClean`；搜索索引接 `searchStatus/searchRebuild` | 基础、通知、桌面能力、工作区、代理摘要多数为本地状态 |
| 设置-模型配置 | 新 UI 完成 | 当前 `/settings` 中未接后端 | 需要迁移到 `aiConfigList/save/test/delete`；隐藏 `/ai-settings` 有旧版真实接入，可作为参考 |
| 设置-Prompt 模板 | 新 UI 完成 | 当前 `/settings` 中未接后端 | 需要迁移到 `promptTemplatesList/create/update/delete`；分类能力后端未支持 |
| 设置-数据源概览 | 静态卡片 | 无 | 可接 `providersStatus/cacheStats/schedulerStatus`；缺数据源配置接口 |
| 设置-凭据管理 | 静态 mock 和本地交互 | 无 | 缺第三方数据源凭据 API、加密 SQLite 表和密钥管理设计；必须先确认 |
| 设置-数据说明 | 静态说明页 | 无 | 说明页可保留静态；“查看”类链接需明确跳转或本地提示 |
| 设置-代理 | 静态本地交互 | 无 | 应接 `settingsGet/settingsSet` 和 proxy vault；连接测试后端未实现 |
| 设置-关于 | 静态本地交互 | 无 | 应接 `checkUpdate/exportLogs/openExternalURL`；LICENSE/手册打开方式待定 |

---

## 2.1 抓取、清洗与入库链路 review

当前 review 结论：股票和新闻并不是完全只有抓取和清洗，部分链路已经具备写库流程，但写库点分散在 action、search service 和 scheduler runner 中。后续 UI 对接前，应优先按开发清单的 `S5-00A` 到 `S5-00F` 验收入库闭环。

| 数据类型 | 当前入库状态 | 写库入口 | 存储表 | 主要缺口 |
| --- | --- | --- | --- | --- |
| 股票基础信息 | 已有被动入库 | `StockSearchService.searchProvider` 本地/FTS 无命中后调用 Provider，随后 `UpsertStocks` | `stocks` | 缺主动刷新、全量种子和个股详情按 symbol 刷新基础资料流程 |
| 搜索索引 | 已有增量任务 | 股票 Provider fallback 后写 `UpsertSearchIndexJob`，重建任务读取 `stocks` | `search_index_jobs`、`stock_search_fts` | 需要 UI 对接时验收索引任务消费和重建状态 |
| 行情快照 | 已有写库 | `/api/market/quote` 缓存 miss 后抓取 Provider，随后 `SaveQuote` | `quotes` | 当前每个 symbol 只保留最新快照，不支持历史分时统计 |
| K 线 | 已有写库 | `/api/market/kline` 缓存不足后抓取 Provider，随后 `SaveKlines` | `klines` | 需要按周期、复权、limit 验收缓存命中和补齐策略 |
| 技术指标 | 不单独入库 | `/api/market/indicators` 基于 K 线缓存或 Provider 结果计算 | 无 | 指标结果是派生数据，首版不建议单独保存 |
| 个股新闻 | 已有写库 | `/api/news/list` 缓存不足后抓取 Provider，标准化、去重后 `SaveNewsItems` | `news_items` | 需要验收 symbols JSON 查询和 content_hash 去重 |
| 市场新闻 | 已有写库 | `/api/news/market` 缓存不足后抓取 Provider，标准化、去重后 `SaveNewsItems` | `news_items` | 当前 `NewsItem` 无 market 字段，`ListMarketNews` 忽略 market 参数 |
| 调度抓取 | 已有 runner 写库 | `QuoteRefreshRunner`、`KlineRefreshRunner`、`NewsRefreshRunner` | `quotes`、`klines`、`news_items`、`ingestion_watermarks` | 需要 UI 对接前验收 scheduler job/run/watermark 是否形成闭环 |

当前需要补到开发流程中的实现点：

1. UI 页面接真实数据前，先执行 `S5-00A`、`S5-00C`、`S5-00E`，确认已有入库链路真实可用。
2. 个股详情和自选股如果需要公司资料、行业、概念等基础字段，执行 `S5-00B`，补主动刷新或种子入库。
3. 总览、自选股如果要展示历史趋势统计，执行 `S5-00D`，确认是否新增历史行情快照表。
4. 资讯中心如果要按市场筛选市场新闻，执行 `S5-00F`，确认是否给 `news_items` 增加 market 字段。

---

## 3. 页面级对接明细

> 设置中心的配置项字段、保存方式、使用方式、更新方式和专项对接流程，详见 `docs/2026-06-22-invest-compass-settings-integration-inventory.md`。

### 3.1 AppShell / TopBar / Sidebar

展示数据：

- 搜索框：用户输入股票名称、代码、拼音。
- 市场状态与更新时间：来自 `dashboardStore.state` 的行情更新时间。
- 侧栏状态：Go Core、SQLite、数据源和版本号，来自 `dashboardStore.state.health`、`provider_statuses`。

当前接入：

- Enter 搜索股票：`stockSearch(keyword)`，取第一条结果跳转 `/stocks/:symbol`。
- 刷新：调用 `useDashboardStore.load()`。
- 设置按钮：跳转 `/settings`。

缺口：

- 通知按钮只展示，不读取应用内通知列表，也没有未读数量角标。
- 系统级通知 helper 已存在，但未接全局任务/Provider 事件。
- Sidebar 折叠禁用。
- 当前 Dashboard 页面 mount 时写入静态空态，会覆盖 store；应改为统一调用 `dashboardStore.load()`。

验收数据：

- 至少一个可搜索 symbol。
- Go Core health 返回 `status/version`。
- Provider 状态至少能返回空列表或真实状态。

### 3.2 总览页 `/`

展示数据：

- 自选股涨跌分布。
- 热点/市场新闻。
- 最近报告。
- 最近任务。
- 底部风险提示。

后端已支持：

- `dashboardSummary()`：`watchlist`、`recent_reports`、`recent_tasks`、`market_news`、`risk_tips`、`provider_statuses`。
- `watchlistList()`。
- `marketQuote(symbol)`。
- `marketKline({ symbol, period, adjust, limit })`。
- `coreHealth()`。

当前问题：

- `DashboardPage` 当前没有调用 `dashboardStore.load()`，而是写入静态 `dashboardTopBarState`。
- 旧 `DashboardOverview` 组件有真实 store 读取逻辑，但当前页面未使用。

对接方式：

1. 页面 mount 调用 `useDashboardStore.load()`。
2. 卡片统一从 `DashboardViewState` 读取。
3. 指数 mini chart 使用 `marketKline` 的 `items`。
4. 最近报告和任务直接展示 `dashboardSummary` 字段。
5. Provider 不可用时展示错误态，不补假数据。

缺口：

- Dashboard 统计字段是否满足 UI 的全部卡片，需要以后端 `DashboardSummary` 为准；不足字段需列为接口补充项。

### 3.3 自选股 `/watchlist`

展示数据：

- 自选股票列表：名称、代码、市场、价格、涨跌、成交额、换手率、市盈率、标签、备注、更新时间。
- 摘要面板：当前 UI 为空态或本地统计。

后端已支持：

- `stockSearch(keyword)`：搜索候选股票。
- `watchlistList()`：返回 `{ items: WatchlistItem[] }`。
- `watchlistCreate({ symbol, sort_order, tags, note })`。
- `watchlistUpdate({ id, sort_order, tags, note })`。
- `watchlistDelete(id)`。
- `marketQuote(symbol)`：补行情字段。
- `searchWatchlistNotes(payload)`：搜索备注/标签索引。

当前接入：

- 添加弹窗的股票搜索已接 `stockSearch`。
- 备注搜索已接 `searchWatchlistNotes`。
- 添加、删除和刷新仍是本地状态。

新增实现方式：

1. 添加弹窗选择股票后调用 `watchlistCreate`。
2. 成功后重新拉取 `watchlistList`。
3. 对列表每个 symbol 调用 `marketQuote` 补充行情展示。
4. 重复添加或 provider 不可用时展示后端错误。

编辑实现方式：

- 标签、备注、排序调用 `watchlistUpdate`。
- 如果 UI 需要编辑行业、分类、星标，但后端模型没有字段，必须先确认是否扩展 `WatchlistItem`。

删除实现方式：

- 删除按钮调用 `watchlistDelete(id)`，成功后更新列表。

链接跳转：

- 查看详情跳转 `/stocks/:symbol`，并携带 `state.from="/watchlist"`。

缺口：

- 当前前端展示的 `price/change/amount/turnoverRate/pe/industry/updatedAt/trend/starred` 不属于 `WatchlistItem` 后端字段，应从 `marketQuote` 或新增接口派生。
- 摘要面板的数据来源需要确定：可从列表和 quote 计算，或由后端汇总。

### 3.4 个股详情 `/stocks/:symbol`

展示数据：

- 头部行情：名称、symbol、价格、涨跌、开高低收、成交额、成交量、更新时间。
- K 线图。
- 技术指标。
- 相关新闻。
- 公司资料、行业、概念。
- 标签和备注。
- AI 研究入口。

后端已支持：

- `marketQuote(symbol)`。
- `marketKline({ symbol, period, adjust, limit })`。
- `marketIndicators({ symbol, period, adjust, indicators })`。
- `newsList({ symbol, limit })`。

当前接入：

- 当前页面为 `emptyStockDetail`，所有数据为空态。

对接方式：

1. 从路由读取 `symbol`。
2. 并行调用 quote、kline、indicators、news。
3. K 线周期、复权方式和指标选择只从后端支持的字段发送。
4. 新闻原文打开使用 `openExternalURL`，不直接在 renderer 打开未知 scheme。
5. 点击 AI 研究入口跳转 `/analysis?symbol=...` 或通过 route state 传递 symbol。

缺口：

- 后端没有公司资料、行业、概念、主营业务等基础资料接口。
- 后端没有个股标签/备注独立读取接口；可以复用 watchlist note，但需要确认 UX。
- 当前页面强制 `min-w-[1140px]`，窄屏可能横向溢出，需要单独 UI 收口。

### 3.5 AI 分析 `/analysis`

展示数据：

- 分析配置：股票、分析类型、模型配置、Prompt 模板。
- 数据上下文预览：行情、K 线、新闻、技术指标。
- 可选持仓上下文。
- 输出预览和操作栏。

后端已支持：

- `aiConfigList()`。
- `promptTemplatesList()`。
- `marketQuote/marketKline/marketIndicators/newsList`。
- `analysisTaskCreate({ symbol, analysis_type, ai_config_id, api_key_ref, prompt_template_id, user_position? })`。

当前接入：

- 页面使用本地 `initialAnalysisConfig`、`emptyContextSummary`，开始生成只跳转运行页。

新增/编辑方式：

- 本页不直接新增模型或 Prompt；跳转设置页管理。
- 可选持仓只作为 `analysisTaskCreate.user_position` 一次性 payload，不落单独持仓表。

按钮对接：

- 返回自选：跳转 `/watchlist`。
- Prompt 模板管理：切到 `/settings` 的 Prompt 模板 tab，或跳转隐藏 `/ai-settings` 前需先统一设计。
- 查看更多资讯：跳转 `/news` 并带 symbol 查询条件。
- 开始生成：校验 symbol、ai_config_id、api_key_ref、prompt_template_id 后调用 `analysisTaskCreate`，成功后带 `task_id` 跳转 `/analysis/running`。
- 停止生成：仅在已有 task_id 时调用 `analysisTaskCancel(task_id)`。
- 保存、复制、导出：只有任务成功并有报告内容后启用。

缺口：

- UI 分析类型、Prompt 类型与后端 `stock_full|technical` 需要收敛。
- 数据上下文预览需要真实字段映射。
- 如果用户希望自定义分析类型，后端当前 `AnalysisTaskCreatePayload` 需要扩展，必须先确认。

### 3.6 AI 分析运行页 `/analysis/running`

展示数据：

- 任务头部：任务 ID、股票、分析类型、状态、耗时、模型。
- 步骤时间线。
- 流式 Markdown 输出。
- 任务日志。
- 底部操作栏。

后端已支持：

- `analysisTaskSubscribe(task_id, after_event_id?)`。
- `taskEvents(task_id, after_event_id?)`。
- `analysisTaskCancel(task_id)`。
- `reportGet/list` 可用于成功后打开报告。

当前接入：

- 页面使用空 `emptyTaskSummary`，没有读取 route state 或 query task_id。

对接方式：

1. 从 route state/query 读取 `task_id`。
2. 首屏调用 `taskGet(task_id)` 和 `taskEvents(task_id)`。
3. 订阅 `analysisTaskSubscribe`，处理 `TASK_STARTED/TASK_PROGRESS/TASK_LOG/TASK_CHUNK/TASK_SUCCESS/TASK_FAILED/TASK_CANCELLED`。
4. `TASK_CHUNK` 追加到 Markdown 输出。
5. `TASK_SUCCESS` 后提供打开报告按钮。
6. 断线后用 `after_event_id` 补拉。

缺口：

- 当前运行页没有 task_id 入口约定。
- 后端事件 data 字段到 UI 步骤模型的映射需要固定。

### 3.7 报告历史 `/reports`

展示数据：

- 报告列表：标题、股票、分析类型、模型、生成时间、风险摘要、状态。
- 过滤：关键词、类型、模型、日期、状态。
- 统计面板。

后端已支持：

- `reportList()`。
- `reportGet(id)`。
- `reportDelete(id)`。
- `searchReports(payload)`。

当前接入：

- 关键词非空时走 `searchReports`。
- 删除搜索结果中的数字 ID 时走 `reportDelete`。
- 默认列表为空，不调用 `reportList`。
- 统计面板为静态展示。

对接方式：

1. 首屏调用 `reportList`。
2. 本地过滤只用于已加载列表；全文关键词检索可继续走 `searchReports`。
3. 刷新按钮重新调用 `reportList`。
4. 删除调用 `reportDelete`。
5. 查看跳转 `/reports/:id`。
6. 新建分析跳转 `/analysis`。

缺口：

- 批量操作、导出报告、收藏状态没有后端字段或接口。
- 统计面板需要从 `reportList` 计算，或新增报告统计接口。
- 搜索索引返回 `ref_id/doc_uid` 与报告数字 ID 的映射需要保证稳定。

### 3.8 报告详情 `/reports/:reportId`

展示数据：

- 报告标题、股票、分析类型、模型、生成时间、任务 ID、更新时间。
- Markdown 正文。
- TOC。
- 输入快照。

后端已支持：

- `reportGet(id)` 返回 `AnalysisReport`：
  - `id`
  - `task_id`
  - `symbol`
  - `title`
  - `analysis_type`
  - `model_name`
  - `prompt_template_id`
  - `content_markdown`
  - `risk_summary`
  - `created_at`
  - `updated_at`

当前接入：

- 详情读取已接 `reportGet`。
- 复制 Markdown 使用剪贴板。

按钮对接：

- 返回：跳转 `/reports`。
- 复制：复制 `content_markdown`。
- 导出：缺 Markdown 文件导出 command。
- 重新分析：可跳转 `/analysis` 并带 symbol、analysis_type、prompt_template_id。
- 删除：应接 `reportDelete`。
- 收藏：缺持久化字段。
- 复制输入快照：当前 `inputSnapshot` 恒为 null。

缺口：

- `AnalysisReport` 当前没有 `input_snapshot` 前端字段。
- 收藏、导出、重新分析需要明确持久化或命令设计。

### 3.9 资讯中心 `/news`

展示数据：

- 新闻列表：来源、时间、标题、摘要、标签、原文链接。
- 筛选：关键词、股票、来源、行业、时间范围。
- 侧栏：热门关键词、行业热度、提及股票、数据源状态、情绪统计。

后端已支持：

- `newsMarket({ market, limit })`。
- `newsList({ symbol, limit })`。
- `searchNews(payload)`。
- `openExternalURL(url)`。

当前接入：

- 关键词搜索走 `searchNews`。
- 打开原文走 `openExternalURL`。
- 默认列表、侧栏统计为空。

对接方式：

1. 首屏默认调用 `newsMarket({ market: "CN", limit })` 或后端确认 market 枚举。
2. 选择股票后调用 `newsList({ symbol, limit })`。
3. 关键词检索继续走 `searchNews`。
4. 打开原文统一走 `openExternalURL`。
5. 复制摘要使用剪贴板。

缺口：

- 热门关键词、行业热度、提及股票、情绪统计没有后端接口。
- “加入 AI 分析上下文”没有上下文篮子或任务草稿存储。
- “资讯缓存清理”未接 `cacheClean(["news"])`。
- 相关性/热度排序需要搜索或新闻接口支持。

### 3.10 任务历史 `/tasks`

展示数据：

- 任务列表：任务类型、状态、标题、进度、错误摘要、开始/结束时间、耗时。
- 任务统计：运行中、今日成功、失败。
- 任务详情事件。
- 任务日志抽屉。

后端已支持：

- `taskList()`。
- `taskGet(id)`。
- `taskEvents(task_id, after_event_id?)`。
- `analysisTaskCancel(task_id)`。
- `taskLogsList/get/summary/diagnosis/context/export`。

当前接入：

- 主列表使用本地空数组。
- `TaskLogDrawer` 已接真实任务日志接口。
- 取消任务只是本地改状态。

对接方式：

1. 首屏调用 `taskList`。
2. 选择任务调用 `taskGet` 和 `taskEvents`。
3. 取消只对可取消任务调用 `analysisTaskCancel`。
4. 查看报告根据任务 ID 查找报告，当前缺 task -> report 便捷接口，可通过 report list 过滤或新增接口。
5. 重试需要根据任务类型重新构造请求，当前不应假装可用。

缺口：

- UI 的 `duration/stockName/stockCode/errorSummary` 需从后端 `TaskItem` 派生或扩展字段。
- 今日成功统计需要按日期计算或后端提供。
- 重试接口缺失。

### 3.11 任务调度 `/scheduler`

展示数据：

- 调度状态：任务总数、启用任务、排队中、运行中、失败记录。
- 任务类型。
- 调度任务列表。
- 手动补偿。
- 单股刷新。
- 执行记录和详情。
- Provider 状态。

后端已支持：

- `schedulerStatus`。
- `schedulerJobTypes`。
- `schedulerJobsList/get/save/setEnabled/delete/runNow/backfill`。
- `schedulerRunsList/get/trigger`。
- `schedulerRefreshSymbol`。
- `providersStatus`。

当前接入：

- 该页面已通过 `services/scheduler.ts` 接真实接口。
- 路由隐藏，不在主导航展示。

缺口：

- 是否作为首版显式页面，需要产品确认。
- UI 仍是早期 AntD/Tailwind 表单风格，没有按最新设置中心截图重绘。

### 3.12 设置中心 `/settings`

一级 Tab 当前实际可切换：

- 基础设置
- 模型配置
- Prompt 配置
- 数据源设置
- 代理设置
- 关于

当前一级 Tab 中出现但未接入的项，会提示“该设置页待接入”。

#### 基础设置

已接：

- 缓存统计：`cacheStats`。
- 缓存清理：`cacheClean(targets)`。
- 搜索索引状态：`searchStatus`。
- 搜索索引重建：`searchRebuild({ scope, force })`。

未接：

- 应用基础设置持久化。
- 工作区读取/选择/打开/保存：应接 `workspaceGet/workspaceSet/selectDirectory`。
- 工作区默认目录应优先位于用户个人目录：macOS `~/Documents/Invest Compass`，Windows 11 `%USERPROFILE%\\Documents\\Invest Compass`；用户切换目录时必须先迁移数据，再更新 `workspace_path`。
- 通知设置持久化。
- 桌面能力：应接 `autostartGet/autostartSet`，其他能力需确认。
- 代理摘要跳转。
- 其他设置持久化。

#### 模型配置

当前 `/settings` 使用新 UI，但数据是 `initialModelConfigs` 本地状态。

后端已支持：

- `aiConfigList`。
- `aiConfigSave`。
- `aiConfigTest`。
- `aiConfigDelete`。

对接方式：

1. 首屏调用 `aiConfigList`。
2. 新建/编辑调用 `aiConfigSave`。
3. API Key 只在保存 payload 中一次性提交。
4. 保存成功后清空明文输入，列表只展示 `has_api_key/masked_api_key`。
5. 测试调用 `aiConfigTest({ id, api_key_ref })`。
6. 删除调用 `aiConfigDelete({ id })`。

说明：

- 隐藏 `/ai-settings` 中已有一套简易真实接入实现，可作为迁移参考，但不能直接替代当前设置中心视觉页。

#### Prompt 配置

当前 `/settings` 中 Prompt 模板页是本地分类和模板状态。

后端已支持：

- `promptTemplatesList`。
- `promptTemplatesGet`。
- `promptTemplatesCreate`。
- `promptTemplatesUpdate`。
- `promptTemplatesDelete`。

对接方式：

1. 首屏调用 `promptTemplatesList`。
2. 新建模板调用 `promptTemplatesCreate`。
3. 保存模板调用 `promptTemplatesUpdate`。
4. 删除模板调用 `promptTemplatesDelete`，内置模板应禁用删除。
5. 变量列表以技术方案白名单为准。

缺口：

- 当前 UI 有“分类”能力，但后端 `PromptTemplate` 只有 `type`，没有分类 CRUD。
- 自定义分类是否需要持久化必须确认；否则只能映射到已有 `type`。

#### 数据源设置

二级 Tab 当前为：

- 数据源概览
- 凭据管理
- 数据说明

数据源概览当前使用静态数据和本地 message。

可复用后端能力：

- `providersStatus`：Provider 可用性。
- `cacheStats`：本地缓存快照。
- `schedulerStatus`：同步/刷新状态。

缺口：

- 没有数据源基础设置持久化接口。
- 没有第三方数据源 Provider 配置 CRUD。
- 没有同步策略设置接口，当前调度 API 可管理任务，但不是数据源设置专用接口。

凭据管理当前是 mock 数据和本地交互。数据源凭据指已对接 Provider 所需的 token、cookie、API Key 或 Bearer Token，可以加密后保存到 SQLite，但不能明文落库。

必须确认：

- 第三方数据源凭据是否进入首版真实闭环。
- SQLite 密文字段、nonce、key version 和脱敏展示字段。
- 加密密钥由 Rust 管理还是 Go 管理，且密钥不能保存到 SQLite。
- Go core 如何在 Provider 请求执行期解密并注入凭据。
- 页面是否允许管理 Cookie / API Key / Bearer Token。

在确认前：

- 不接真实接口。
- 不保存真实凭据。
- 不把 maskedCredential 当真实值。

数据说明页是说明性静态页面，可以保留静态内容；按钮如“查看数据源概览”应切换本地二级 Tab，而不是跳不存在页面。

#### 代理设置

当前使用本地 state，连接测试为 600ms 定时模拟。

后端已支持：

- `settingsGet(keys)`。
- `settingsSet({ items, proxy_password?, proxy_credential_ref?, clear_proxy_credential? })`。

对接方式：

1. 首屏读取 `proxy_url/proxy_credential_ref` 等设置。
2. 保存 HTTP/SOCKS5 配置调用 `settingsSet`。
3. 代理密码只在保存 payload 中一次性传入。
4. 保存成功后前端清空明文密码，只展示脱敏状态。

缺口：

- 代理连通性测试接口未实现。
- bypass rules 的后端 key 需要确认。
- 系统代理状态读取是否需要 Rust 桌面能力，需要确认。

#### 关于

当前为本地定时模拟。

后端已支持：

- `checkUpdate`。
- `exportLogs`。
- `openExternalURL`。

对接方式：

- 检查更新调用 `checkUpdate`。
- 导出日志调用 `exportLogs`。
- LICENSE、用户手册、发布说明可用 `openExternalURL` 或内置页面，需确认 URL/文件路径。

缺口：

- 授权状态卡不属于首版闭环，若没有真实能力应移除或改成开源许可说明。

---

## 4. 字段对接矩阵

### 4.1 股票与行情

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| 股票名称 | `StockSearchResult.name` | `stockSearch` | 搜索候选使用 |
| 标准 symbol | `StockSearchResult.symbol` | `stockSearch` | 传给 quote/kline/news |
| 代码 | `StockSearchResult.code` | `stockSearch` | UI 展示 |
| 市场/交易所 | `market/exchange` | `stockSearch` | UI tag |
| 现价 | `MarketQuote.price` | `marketQuote` | 自选股、详情 |
| 涨跌额 | `change_amount` | `marketQuote` | 自选股、详情 |
| 涨跌幅 | `change_percent` | `marketQuote` | 自选股、详情 |
| 开高低昨收 | `open/high/low/pre_close` | `marketQuote` | 详情头部 |
| 成交量/成交额 | `volume/amount` | `marketQuote` | 自选股、详情 |
| 换手率/PE/PB | `turnover_rate/pe/pb` | `marketQuote` | 部分 Provider 可能缺失，缺失展示空态 |
| 更新时间 | `quote_time` | `marketQuote` | TopBar 和页面 |
| K 线 | `MarketKlineItem[]` | `marketKline` | 图表和表格 |
| 技术指标 | `indicators` | `marketIndicators` | key/value 展示 |

### 4.2 自选股

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| ID | `WatchlistItem.id` | `watchlistList` | 更新/删除主键 |
| symbol | `symbol` | `watchlistList/create` | 权威股票标识 |
| 排序 | `sort_order` | `watchlistList/update` | 拖拽排序可用 |
| 标签 | `tags` | `watchlistList/update` | 数组 |
| 备注 | `note` | `watchlistList/update` | 搜索索引来源 |
| 行情展示字段 | `MarketQuote` | `marketQuote` | 不应落入 watchlist 模型 |

### 4.3 新闻与搜索

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| 新闻 ID | `NewsItem.id` | `newsList/newsMarket` | 列表 key |
| 来源 | `source` | `news*` / `searchNews` | |
| 标题 | `title` | 同上 | |
| 摘要 | `summary` | 同上 | |
| 原文链接 | `url` 或搜索 `ref_id` | `news*` / `searchNews` | 打开必须走 `openExternalURL` |
| 发布时间 | `published_at` / `source_time` | 同上 | |
| tags/symbols | `tags/symbols/highlights` | 同上 | |

### 4.4 AI 配置与 Prompt

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| 配置名 | `AIConfig.name` | `aiConfigList/save` | |
| Provider | `provider` | 同上 | 当前只支持 OpenAI-compatible |
| Base URL | `base_url` | 同上 | |
| 模型名 | `model_name` | 同上 | |
| 密钥状态 | `has_api_key/masked_api_key/api_key_ref` | 同上 | 明文不回显 |
| 温度 | `temperature` | 同上 | |
| Max Tokens | `max_tokens` | 同上 | |
| 超时 | `timeout_seconds` | 同上 | |
| 流式 | `stream_enabled` | 同上 | |
| 默认 | `is_default` | 同上 | |
| Prompt 名称 | `PromptTemplate.name` | `promptTemplates*` | |
| Prompt 类型 | `type` | 同上 | 只允许首版白名单 |
| Prompt 内容 | `content` | 同上 | |
| 变量 | `variables` | 后端解析返回 | UI 只展示白名单 |

### 4.5 任务与报告

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| 任务 ID | `TaskItem.id` | `taskList/taskGet` | |
| 任务类型 | `type` | 同上 | |
| 状态 | `status` | 同上 | |
| 标题 | `title` | 同上 | |
| 进度 | `progress` | 同上 | |
| 错误 | `error_message` | 同上 | 必须脱敏 |
| 时间 | `started_at/finished_at/created_at/updated_at` | 同上 | |
| 事件 | `TaskEventItem.event/data/created_at` | `taskEvents/subscribe` | |
| 报告 ID | `AnalysisReport.id` | `reportList/get` | |
| 报告内容 | `content_markdown` | `reportGet` | Markdown 渲染需安全白名单 |
| 风险摘要 | `risk_summary` | `reportGet/list` | |

### 4.6 设置、缓存与调度

| UI 字段 | 后端字段 | 来源接口 | 说明 |
| --- | --- | --- | --- |
| 设置项 | `SettingItem.key/value` | `settingsGet/set` | key 需要统一登记 |
| 工作区 | `WorkspaceResult.path` | `workspaceGet/set` | 目录选择走 Tauri |
| 缓存大小 | `CacheStatsResult.total_bytes/items` | `cacheStats` | |
| 缓存清理 | `targets` | `cacheClean` | 不清理报告和配置 |
| 索引状态 | `SearchIndexStatus` | `searchStatus` | |
| 索引重建 | `scope/force` | `searchRebuild` | 返回 task_id |
| 调度状态 | `SchedulerStatus` | `schedulerStatus` | |
| 调度任务 | `SchedulerJob` | `schedulerJobs*` | |
| 调度执行 | `SchedulerRun` | `schedulerRuns*` | |

---

## 5. 缺失接口、数据和页面清单

### 5.1 必须补接口或确认的能力

1. 个股基础资料接口
   - 缺字段：公司资料、行业、细分行业、概念。
   - 影响页面：个股详情。
   - 处理建议：确认是否首版需要；需要则新增只读接口。

2. 自选股行情聚合接口或前端编排规则
   - 当前可以 `watchlistList + marketQuote` 前端编排。
   - 如果列表数量变大，建议后端提供 watchlist quote 汇总接口。

3. 报告统计接口或前端计算规则
   - 当前报告统计面板无后端数据。
   - 可先用 `reportList` 前端计算；复杂统计再加接口。

4. 任务统计接口或前端计算规则
   - 当前任务历史统计本地计算空数组。
   - 可先用 `taskList` 前端计算。

5. 新闻侧栏统计接口
   - 缺热门关键词、行业热度、提及股票、情绪统计、数据源状态。
   - 需要确认是否首版显示，或改为空态/移除。

6. AI 分析上下文篮子
   - “加入 AI 分析上下文”没有持久化位置和任务草稿模型。
   - 需要确认是否使用 route state、临时 store，还是后端草稿接口。

7. 报告导出接口
   - 报告详情和报告历史都有导出入口。
   - 需要 Rust command 写文件或调用系统保存对话框。

8. 报告收藏字段
   - 当前 UI 本地切换 favorite。
   - 后端 `AnalysisReport` 没有收藏字段。

9. Prompt 分类持久化
   - 当前 UI 支持分类新增。
   - 后端只支持模板 `type`，没有 category 表。

10. 第三方数据源配置与凭据管理
    - 凭据管理页当前全 mock。
    - 缺 Provider 配置、加密凭据表、过期时间、测试结果、操作日志等后端模型。
    - 涉及凭据边界，必须单独确认方案。

11. 代理连通性测试
    - 代理配置可用 `settingsSet` 保存。
    - 但连接测试没有 Go/Rust command。

12. 通知列表与通知设置持久化
    - TopBar 通知按钮未接。
    - 设置页通知卡片未保存。
    - 缺应用内通知 `notifications` 表、未读数量接口和通知列表浮层。
    - 系统级通知已有 Tauri 插件基础，但未接事件触发和权限降级提示。

13. 关于页资源打开
    - LICENSE、用户手册、发布说明需要确定是内置页面、文件打开还是外链。

14. 工作区迁移能力
    - 当前只有 `workspaceGet/workspaceSet`，只能保存路径。
    - 用户切换目录时需要迁移 SQLite、报告导出、索引、配置快照和必要业务文件。
    - 需要新增迁移预检、迁移执行、备份、校验和回滚能力。

### 5.2 页面缺失或入口状态

- 任务调度页存在但隐藏，是否进入首版主导航待确认。
- `/ai-settings` 旧版真实接入页存在但隐藏，当前设置中心新 UI 未复用该真实链路。
- 设置中心一级 Tab 中通知设置、工作区设置、缓存管理等在新设置页中不是独立页面，而是基础设置卡片；如果要按截图拆独立页，需要新增页面或调整 Tab 行为。
- 数据源设置已移除 Provider 配置和同步策略二级 Tab；对应能力如果后续需要，应重新确认页面定位和后端支持。

### 5.3 推荐补齐方案

推荐按“先不改 schema 的编排补齐，再做低风险字段扩展，最后处理凭据和跨桌面能力”的顺序推进。涉及数据库、Rust command、Go API 或凭据边界的方案，实施前需要单独确认。

#### P0：无需新增后端能力，优先接通

1. 总览真实数据恢复
   - 推荐方案：恢复 `DashboardPage -> useDashboardStore.load()`，复用现有 `dashboardSummary/coreHealth/watchlistList/marketQuote/marketKline`。
   - 后端变化：无。
   - 前端变化：删除页面写入静态 `dashboardTopBarState` 的逻辑，复用或迁移 `DashboardOverview` 的真实 store 展示。
   - 验收：Go Core 未启动时展示错误态；Provider 未配置时展示不可用态；不出现静态假数据。

2. 自选股列表和写操作
   - 推荐方案：使用 `watchlistList/create/update/delete` 作为权威状态，用 `marketQuote` 补展示行情。
   - 后端变化：无。
   - 前端变化：新增列表加载态、写入后重拉、quote 失败单行降级。
   - 验收：新增、编辑标签备注、删除均能反映到 SQLite；行情失败不影响自选股基础列表。

3. 个股详情基础行情链路
   - 推荐方案：先只接现有 `marketQuote/marketKline/marketIndicators/newsList`，公司资料、概念等缺失字段展示空态。
   - 后端变化：无。
   - 前端变化：路由 symbol 驱动并行加载，页面字段以现有接口为准。
   - 验收：任意可搜索股票可进入详情；K 线和新闻无数据时为空态；不填假行业或假概念。

4. 报告历史默认列表
   - 推荐方案：首屏和刷新按钮接 `reportList`，关键词全文检索继续用 `searchReports`。
   - 后端变化：无。
   - 前端变化：列表数据统一从 `AnalysisReport` 映射；筛选先本地执行。
   - 验收：无关键词时能显示真实报告列表；删除后列表同步更新。

5. 资讯中心默认列表
   - 推荐方案：首屏接 `newsMarket({ market: "CN", limit })`，股票筛选接 `newsList({ symbol, limit })`，关键词搜索保留 `searchNews`。
   - 后端变化：无。
   - 前端变化：空侧栏保留空态；资讯缓存清理接 `cacheClean(["news"])`。
   - 验收：无关键词也能拉取真实市场资讯；无 Provider 时显示不可用态。

6. 任务历史主列表
   - 推荐方案：主列表接 `taskList`，详情抽屉接 `taskGet + taskEvents`，日志抽屉保留现有 `taskLogs*`。
   - 后端变化：无。
   - 前端变化：将 UI 字段从 `TaskItem` 派生；无法派生字段不展示或标注暂无。
   - 验收：任务列表、事件、日志三者能按 task_id 关联。

7. 设置中心模型配置真实接入
   - 推荐方案：把隐藏 `/ai-settings` 中的真实调用迁移到当前新视觉页。
   - 后端变化：无。
   - 前端变化：`ModelConfigPage` 接 `aiConfigList/save/test/delete`；API Key 保存成功后清空明文。
   - 验收：列表只展示 `masked_api_key`；测试失败不泄露密钥。

8. 设置中心 Prompt 模板真实接入
   - 推荐方案：先取消“自定义分类持久化”，把当前分类 UI 映射为后端 `type` 分组。
   - 后端变化：无。
   - 前端变化：接 `promptTemplatesList/create/update/delete`；新建分类按钮改为待接入或隐藏。
   - 验收：内置模板不可删除；变量只显示后端返回和白名单允许的变量。

9. 关于页基础能力
   - 推荐方案：检查更新接 `checkUpdate`，日志导出接 `exportLogs`。
   - 后端变化：无。
   - 前端变化：LICENSE 和用户手册先用内置说明或打开仓库文件，避免外链不确定。
   - 验收：日志导出路径展示；检查更新展示后端返回状态。

#### P1：建议补低风险接口或字段

1. 个股基础资料只读接口
   - 推荐 API：`stockProfile({ symbol }) -> StockProfile`。
   - 推荐 Go path：`/api/stocks/profile`。
   - 推荐 Rust command：`stock_profile`。
   - 推荐字段：
     - `symbol`
     - `name`
     - `industry`
     - `sub_industry`
     - `concepts: string[]`
     - `exchange`
     - `market`
     - `updated_at`
   - 数据来源：优先 provider 公开基础资料；没有 Provider 支持时返回空字段和明确 provider error。
   - 说明：先做只读，不做公司资料编辑。

2. 报告统计汇总
   - 推荐方案：短期前端用 `reportList` 计算；如果列表增长或统计口径复杂，再补 `reportSummary()`。
   - 推荐 API：`reportSummary({ range? }) -> ReportSummary`。
   - 推荐字段：
     - `total_count`
     - `success_count`
     - `failed_count`
     - `running_count`
     - `by_analysis_type`
     - `by_model`
     - `latest_generated_at`
   - 说明：统计来自本地报告表，不引入假趋势。

3. 任务统计汇总
   - 推荐方案：短期前端用 `taskList` 计算；后续补 `taskSummary()`。
   - 推荐 API：`taskSummary({ range? }) -> TaskSummary`。
   - 推荐字段：
     - `running_count`
     - `success_today_count`
     - `failed_count`
     - `cancelled_count`
     - `avg_duration_seconds`
   - 说明：只统计本地任务表，不读取外部服务。

4. 报告删除和收藏
   - 推荐方案：删除复用现有 `reportDelete`；收藏不建议在当前阶段做本地假状态。
   - 推荐 DB 字段：`analysis_reports.favorite boolean default false`。
   - 推荐 API：`reportUpdate({ id, favorite? }) -> AnalysisReport`。
   - 推荐 Rust command：`report_update`。
   - 说明：字段变更需要 migration，实施前确认。

5. 报告导出
   - 推荐方案：由 Rust command 负责系统保存路径或导出到工作区，Go 只提供报告内容。
   - 推荐 Rust command：`export_report_markdown({ report_id, target_path? }) -> { file_path }`。
   - 推荐行为：
     - 默认导出到工作区 `exports/reports/`。
     - 文件名使用报告标题和时间生成。
     - 导出前做 Markdown 内容脱敏。
   - 说明：不要让前端拼文件路径写入磁盘。

6. 新闻侧栏轻量统计
   - 推荐方案：先由 Go 基于 `news_items` 本地表计算，不引入外部情绪模型。
   - 推荐 API：`newsSummary({ market?, symbol?, range? }) -> NewsSummary`。
   - 推荐字段：
     - `hot_keywords`
     - `hot_industries`
     - `mentioned_stocks`
     - `source_statuses`
     - `sentiment_summary`
   - 情绪统计建议：
     - 首版仅支持 `positive/neutral/negative` 的简单规则或空态。
     - 如果没有可靠算法，字段返回空，UI 展示“暂无资讯情绪统计”。

7. AI 分析 task -> report 关联查询
   - 推荐 API：`reportGetByTask({ task_id }) -> AnalysisReport | null`。
   - 推荐 Rust command：`report_get_by_task`。
   - 用途：运行页 `TASK_SUCCESS` 后打开最终报告，任务历史“查看报告”按钮。

8. 设置项 key 清单
   - 推荐方案：建立前端常量和文档表，所有 `settingsGet/settingsSet` key 统一登记。
   - 初始 key：
     - `app.language`
     - `app.theme`
     - `notifications.task_terminal`
     - `notifications.in_app_enabled`
     - `notifications.system_enabled`
     - `notifications.task_success`
     - `notifications.task_failed`
     - `notifications.provider_error`
     - `window.close_to_tray`
     - `proxy.mode`
     - `proxy.url`
     - `proxy.bypass_rules`
     - `proxy_credential_ref`
   - 说明：key 增删属于配置契约变更，实施前确认。

9. 工作区默认路径和迁移
   - 推荐默认路径：
     - macOS：`~/Documents/Invest Compass`
     - Windows 11：`%USERPROFILE%\\Documents\\Invest Compass`
   - 推荐内部目录：
     - macOS 应用支持：`~/Library/Application Support/Invest Compass`
     - macOS 缓存：`~/Library/Caches/Invest Compass`
     - macOS 日志：`~/Library/Logs/Invest Compass`
     - Windows 应用数据：`%LOCALAPPDATA%\\Invest Compass`
     - Windows 缓存：`%LOCALAPPDATA%\\Invest Compass\\Cache`
     - Windows 日志：`%LOCALAPPDATA%\\Invest Compass\\Logs`
   - 推荐 API：
     - `workspaceMigrationPlan({ target_path }) -> WorkspaceMigrationPlan`
     - `workspaceMigrate({ target_path, include_cache, create_backup }) -> WorkspaceMigrationResult`
   - 推荐规则：
     - `workspace_path` 面向用户可见和可迁移数据。
     - 加密密钥不放在工作区，避免密钥和密文一起被复制。
     - 切换目录必须先预检空间和冲突，迁移成功后再 `workspaceSet(path)`。
     - 迁移失败必须回滚，不能破坏原工作区。

#### P2：涉及凭据、桌面能力或产品口径，先确认再做

1. 数据源凭据管理
   - 推荐结论：不要直接把当前 mock 页接真实保存；先设计 encrypted SQLite 安全契约。
   - 推荐模型：
     - `data_source_credentials`
       - `id`
       - `provider_id`
       - `auth_type`
       - `base_url`
       - `encrypted_credential`
       - `nonce`
       - `key_version`
       - `encryption_alg`
       - `masked_credential`
       - `status`
       - `expires_at`
       - `timeout_seconds`
       - `rate_limit_per_minute`
       - `note`
       - `last_tested_at`
       - `last_test_status`
       - `created_at`
       - `updated_at`
       - `deleted_at`
     - `data_source_credential_logs`
       - `id`
       - `provider_id`
       - `action`
       - `status`
       - `message`
       - `created_at`
   - 推荐 API：
     - `dataSourceCredentialsList()`
     - `dataSourceCredentialGet(provider_id)`
     - `dataSourceCredentialSave(payload)`
     - `dataSourceCredentialClear(provider_id)`
     - `dataSourceCredentialTest(provider_id, target)`
     - `dataSourceCredentialLogs(provider_id?)`
   - 推荐 Rust command：
     - `data_source_credentials_list`
     - `data_source_credential_get`
     - `data_source_credential_save`
     - `data_source_credential_clear`
     - `data_source_credential_test`
     - `data_source_credential_logs`
   - 安全规则：
     - SQLite 只存密文、nonce、key version、算法和脱敏展示值。
     - 加密密钥不能存 SQLite，建议由 Rust 桌面层管理或后续接平台 Keychain。
     - 明文只在保存请求和 Provider 当前请求内短暂存在。
     - Go core 只在单次 Provider 请求或任务内解密并使用凭据。
     - 测试接口只返回脱敏状态、响应耗时和错误码。
   - 确认点：是否允许 Cookie 类型凭据进入首版。

2. 数据源配置与同步策略
   - 推荐结论：当前已移除 Provider 配置和同步策略二级 Tab，不建议马上恢复页面。
   - 推荐方案：把“调度任务”作为同步策略的真实实现入口，数据源概览只展示 `providersStatus + schedulerStatus`。
   - 如果后续恢复：
     - Provider 配置只做启用状态、优先级、超时、限流。
     - 同步策略复用 `/api/scheduler/*`，不新造第二套任务系统。

3. 代理连通性测试
   - 推荐 API：`proxyTest({ target, proxy_mode?, proxy_url?, proxy_credential_ref? }) -> ProxyTestResult`。
   - 推荐 Rust command：`proxy_test_connection`。
   - 推荐返回：
     - `status`
     - `response_time_ms`
     - `checked_at`
     - `message`
   - 安全规则：不返回代理密码，不记录 `Proxy-Authorization`。
   - 确认点：测试目标是否固定为后端白名单，避免任意 URL 探测工具化。

4. 通知中心
   - 推荐结论：拆成应用内通知和系统级通知两块。
   - 应用内通知：
     - 新增 `notifications` SQLite 表。
     - TopBar Bell 使用 `Badge count={unreadCount}`。
     - 点击 Bell 打开 Popover/Dropdown 通知列表浮层。
     - 支持单条已读、全部已读、点击通知跳转应用内 route。
   - 系统级通知：
     - 复用已有 `@tauri-apps/plugin-notification` 和 Rust `tauri-plugin-notification`。
     - 由全局通知服务消费任务/Provider 事件后调用 `sendDesktopNotification`。
     - 受 `notifications.system_enabled` 和具体类型开关控制。
   - 推荐表字段：
     - `id`
     - `type`
     - `level`
     - `title`
     - `content`
     - `source_type`
     - `source_id`
     - `source_event_id`
     - `route`
     - `read_at`
     - `created_at`
     - `updated_at`
   - 推荐 API：
     - `notificationsList({ unread_only?, limit?, offset? })`
     - `notificationsUnreadCount()`
     - `notificationsMarkRead({ ids })`
     - `notificationsMarkAllRead()`
     - `notificationsDelete({ ids })`
   - 推荐 Rust command：
     - `notifications_list`
     - `notifications_unread_count`
     - `notifications_mark_read`
     - `notifications_mark_all_read`
     - `notifications_delete`
   - 首批事件来源：
     - `TASK_SUCCESS`
     - `TASK_FAILED`
     - `TASK_CANCELLED`
     - `scheduler run failed`
     - `provider credential expired`
   - 去重规则：
     - 同一 `source_type + source_id + source_event_id` 只能生成一条通知。
     - Provider 状态类通知按 `source_type + source_id + type + 日期小时` 粗粒度去重。

5. Prompt 自定义分类
   - 推荐结论：首版不做持久化分类，避免新增一套分类模型。
   - 推荐 UI：用后端 `PromptTemplate.type` 分组；自定义分类入口改为“待接入”或移除。
   - 如果必须做：
     - 新增 `prompt_template_categories` 表。
     - 模板增加 `category_id`。
     - 迁移内置类型为默认分类。

6. AI 分析上下文篮子
   - 推荐结论：首版使用前端内存态或 route state，不落库。
   - 推荐行为：
     - 资讯“加入 AI 分析上下文”只在当前会话保存新闻 ID 和摘要。
     - 进入 `/analysis` 时作为 `user_position` 之外的 `context_items` 预览。
   - 如果需要跨页面/重启保留，需要新增 `analysis_drafts`，这属于新产品能力，应确认。

7. 关于页 LICENSE / 手册 / 发布说明
   - 推荐方案：
     - LICENSE：打开仓库根目录 `LICENSE` 或内置文本页。
     - 用户手册：如果暂无正式手册，按钮改为“用户手册待接入”。
     - 发布说明：读取本地 release notes 文档或 `checkUpdate` 返回说明。
   - 不建议：直接打开不确定外链。

8. 任务调度入口
   - 推荐结论：保留隐藏路由，不加入主导航。
   - 推荐入口：数据源概览卡片“任务调度配置”跳转 `/scheduler`，但仅在用户确认调度页进入首版后启用。
   - 说明：页面真实接口完整，但视觉和信息密度与最新设置中心不一致。

#### 推荐任务拆分

第一批：

- 恢复总览真实 store。
- 自选股接真实列表和 CRUD。
- 报告历史默认列表接入。
- 资讯中心默认列表接入。
- 任务历史主列表接入。

第二批：

- 个股详情接行情/K 线/指标/新闻。
- AI 分析创建任务。
- AI 运行页订阅事件。
- 模型配置和 Prompt 模板迁移真实 service。

第三批：

- 报告导出。
- 报告收藏。
- 新闻侧栏统计。
- 个股基础资料。
- task -> report 查询。

第四批：

- 数据源凭据管理。
- 代理连通性测试。
- 通知中心。
- Prompt 自定义分类。

每批验收前都要确认：

- 没有新增真实凭据明文回显。
- 没有新增交易、券商账户、授权激活等非首版入口。
- UI 不再使用 mock 数据伪装真实能力。
- `git diff --check` 和相关前端/后端验证通过。

---

## 6. 推荐对接顺序

### 阶段 A：先拉真实数据基线

任务：

- 确认 Go Core 启动和 `coreHealth`。
- 拉 `providersStatus`。
- 拉 `stockSearch` 最小样本。
- 拉 `watchlistList`。
- 拉 `dashboardSummary`。
- 拉 `reportList`、`taskList`、`newsMarket`。

验收标准：

- 每个页面有真实空态、错误态或真实数据样本。
- 无需 mock 数据也能判断页面是否可验收。
- 缺失 Provider 或数据库数据时记录原因。

### 阶段 B：只读页面接入

任务：

- 总览接 `dashboardStore.load()`。
- 自选股接 `watchlistList + marketQuote`。
- 个股详情接 quote/kline/indicator/news。
- 报告历史接 `reportList`。
- 资讯中心接 `newsMarket/newsList`。
- 任务历史接 `taskList/taskGet/taskEvents`。

验收标准：

- 页面刷新后能从真实 command 获取数据。
- Provider 不可用时展示错误态，不出现假数据。
- 所有列表均支持空态。

### 阶段 C：写操作接入

任务：

- 自选股新增、编辑、删除。
- 模型配置 CRUD 和测试。
- Prompt 模板 CRUD。
- 设置保存、工作区、开机自启、缓存清理、索引重建。
- 报告删除。

验收标准：

- 写操作成功后重新读取权威数据。
- 敏感字段保存后不回显明文。
- 删除操作有确认和错误态。

### 阶段 D：任务和长连接

任务：

- AI 分析创建任务。
- 运行页订阅事件。
- 任务历史读取事件和日志。
- 取消任务。
- 成功后跳转报告详情。

验收标准：

- `TASK_CHUNK` 可流式展示。
- `TASK_FAILED` 显示脱敏错误。
- 断线可补拉事件。
- 任务和报告可追溯。

### 阶段 E：处理阻塞项

任务：

- 对数据源凭据管理、新闻统计、报告导出、收藏、Prompt 分类、代理测试等缺口逐项找用户确认。
- 已确认做的能力再补 API/DB/Rust command。
- 不做的能力改为空态、移除入口或明确“待接入”。

验收标准：

- 没有 UI 假装后端不支持的能力可用。
- 所有“待接入”都有对应任务或被移除。

---

## 7. 待用户确认项

1. 数据源凭据管理是否进入当前对接阶段？
   - 如果进入，需要先设计 encrypted SQLite 表、加密密钥管理、Go/Rust 解密边界和 API。
   - 如果不进入，页面保持静态说明或明确“待接入”，不能保存真实凭据。

2. 个股详情是否需要公司资料、行业、概念等基础资料？
   - 需要则补后端接口。
   - 不需要则从 UI 移除或展示明确空态。

3. 新闻中心侧栏统计是否首版必须真实展示？
   - 需要则补统计接口。
   - 不需要则展示空态，不使用静态假统计。

4. 报告收藏、批量操作、导出是否进入首版？
   - 收藏需要 DB 字段。
   - 导出需要 Rust 文件能力。
   - 批量操作需要明确动作集合。

5. Prompt 分类是否需要持久化？
   - 需要则补 category 模型。
   - 不需要则 UI 改为模板类型分组。

6. `/scheduler` 是否要进入主导航或设置中心？
   - 当前接口完整，但页面隐藏且视觉未统一。

7. 设置中心新模型配置页是否以隐藏 `/ai-settings` 真实接入逻辑为迁移基准？
   - 推荐迁移真实调用，不保留两套体验。

8. 代理连接测试是否需要真实实现？
   - 需要则补 Rust/Go 测试接口。
   - 不需要则移除“测试连接”按钮或明确待接入。

---

## 8. 文档维护规则

- 每完成一个页面真实接入，要更新本文对应页面状态。
- 新增、删除、改名 Rust command 或 Go API 时，要同步更新第 1.3 节和字段矩阵。
- UI 上出现新的按钮、链接、统计卡或编辑入口时，要先写明数据来源和后端支持情况。
- 发现页面功能后端不支持时，必须把该项加入第 7 节确认项或第 5 节缺口清单。
- 不把未验证能力写成已完成。
