# 投研罗盘设置与 UI 后端对接开发任务清单

> 日期：2026-06-22
>
> 来源：
>
> - `docs/2026-06-22-invest-compass-settings-integration-inventory.md`
> - `docs/2026-06-22-invest-compass-ui-backend-integration-inventory.md`
>
> 目标：把设置中心和全局 UI 后端对接工作拆成可标记、可验收、可推进、可执行的开发任务。
>
> 边界：本文是推进开发的任务清单，不表示任务已经完成。凡涉及数据库 schema、Rust command、Go API、Tauri capability、加密凭据、工作区迁移、代理测试、系统通知的改动，实施前必须按项目规则确认。

---

## 0. 状态标记与推进规则

任务状态：

- `[ ]` 未开始。
- `[~]` 进行中。
- `[x]` 已完成并通过验收。
- `[!]` 阻塞，需要用户或接口方案确认。

推进原则：

- 每次只推进一个 Review Gate 内的任务，完成验收后再进入下一组。
- UI 不得继续用 mock 数据伪装真实能力；后端缺字段、缺接口或缺数据时，标记 `[!]` 并确认处理方案。
- 普通配置优先走 `settingsGet/settingsSet`；敏感配置必须走专用安全链路。
- 所有新增前端入口必须有明确数据来源、保存方式、使用方式和失败态。
- 所有新增后端能力必须补齐 Rust command 白名单、Go API、service/dao/model 分层、必要测试和文档。

通用验收命令：

```bash
git diff --check
pnpm --dir apps build
pnpm --dir apps test
go test ./...
```

说明：

- docs-only 修改至少执行 `git diff --check`。
- Go 验证需在 `apps/sidecar-core` 或仓库约定目录内执行。
- Rust/Tauri command 变更需补充对应 `cargo test` 或 `cargo check`。

---

## 1. Review Gate

### RG-S0：契约与数据基线门禁

通过条件：

- UI 页面、配置项、后端接口、Rust command、SQLite 字段的映射关系明确。
- mock 数据清理策略明确。
- 所有缺失字段和缺失接口都有任务编号。

### RG-S1：设置中心普通配置门禁

通过条件：

- 基础设置、桌面设置、缓存、搜索索引、工作区只读/保存链路接入真实后端。
- 刷新页面后配置可以回显。
- 不支持立即生效的配置有明确 UI 提示或禁用状态。

### RG-S2：模型、Prompt、代理门禁

通过条件：

- 模型配置接入 `aiConfig*`。
- Prompt 模板接入 `promptTemplates*`。
- 代理配置保存、脱敏回显和代理密码安全存储链路明确。

### RG-S3：数据源与凭据安全门禁

通过条件：

- 数据源概览接入真实 Provider 状态或明确空态。
- 数据源凭据具备加密 SQLite 存储方案和安全回显边界。
- 保存、清除、测试连接均不泄露真实 token/cookie/key。

### RG-S4：通知与工作区迁移门禁

通过条件：

- 应用内通知中心具备未读角标、列表浮层、已读/清空能力。
- 系统级通知通过 Tauri/Rust 通知能力触发。
- 工作区切换具备迁移预检、确认、执行、回滚或失败保护。

### RG-S5：全局 UI 页面真实数据门禁

通过条件：

- 总览、自选股、个股详情、AI 分析、报告、资讯、任务历史不再用假数据伪装真实能力。
- 每个按钮点击后要么调用真实能力，要么明确禁用/待接入提示。
- 缺字段场景不会静默补假数据。

### RG-S6：发布验收门禁

通过条件：

- 前端 build/test、Go test、Rust check/test 按变更范围通过。
- secret redaction、凭据回显、安全边界、非投资建议文案通过检查。
- 文档与代码行为一致。

---

## 2. RG-S0：契约与数据基线

### [x] S0-01 建立 UI 到后端对接矩阵的执行基线

依赖：两份盘点文档。

执行动作：

- 以 UI 路由为维度列出页面、按钮、展示字段、数据来源、保存方式。
- 以后端能力为维度列出 typed service、Rust command、Go API、SQLite 表。
- 给每个缺口分配本清单中的任务编号。

交付物：

- 本清单保持为唯一推进入口。
- 两份盘点文档只保留方案明细和上下文，不直接当迭代板。
- 主对接清单第 16 节作为页面、按钮、typed service、Rust command、Go API、SQLite / Provider / vault 数据来源的契约矩阵。

验收标准：

- 任意 UI 功能都能追溯到任务编号或已实现接口。
- 任意缺失接口都能追溯到对应页面和验收数据。

验证方式：

- 人工 review 本文档和两份盘点文档链接是否一致。

执行记录：

- 已在 `docs/2026-06-22-invest-compass-frontend-backend-integration-checklist.md` 第 16 节补齐 FE02 页面契约矩阵。
- 设置中心相关缺口继续使用本清单的 S2-S6 任务编号推进。

### [x] S0-02 清理或冻结隐藏旧页面入口

依赖：无。

执行动作：

- 检查 `/ai-settings`、`/scheduler` 等隐藏路由的产品定位。
- 明确是保留为内部验证页、迁移到设置中心，还是删除入口。
- 不删除真实后端接入参考代码，除非确认不再使用。

交付物：

- 隐藏页面处理决策记录。
- 如果迁移，形成迁移任务到 S2/S5。

验收标准：

- 用户可见导航不会出现未闭环能力。
- 隐藏页面不会和设置中心形成两套可编辑权威状态。

验证方式：

- 前端路由人工检查。
- `pnpm --dir apps build`。

执行记录：

- `/ai-settings` 已迁移为设置中心模型配置入口，复用 `SettingsPage` 的 `model-config` tab，不再渲染旧的独立模型配置页面，避免形成两套可编辑权威状态。
- `/scheduler` 保留为内部验证直达路由，用于调度真实后端链路回归；已移除侧栏中的 `sr-only` 辅助导航链接，首版用户可见导航和辅助导航不暴露任务调度入口。
- 已补充前端路由契约测试，覆盖隐藏验证页不通过导航暴露，以及旧 `/ai-settings` 路由复用设置中心模型配置页。
- 已删除 `App.tsx` 中未挂载的旧 `AISettingsRoute` / `SettingsRoute` 内联实现及其专用 helper，设置中心统一收敛到 `pages/settings/SettingsPage`。

### [x] S0-03 建立 settings key 常量和默认值清单

依赖：无。

执行动作：

- 为普通配置建立统一 key 清单，例如 `app.theme`、`market.default`、`quote.refresh_interval`。
- 明确每个 key 的类型、默认值、可选值、是否立即生效。
- 禁止把 password/token/secret/authorization 等敏感 key 放入普通 settings。

交付物：

- 前端 settings key 常量。
- Go settings service 白名单或校验规则更新。
- 文档同步。

验收标准：

- 前后端使用同一组 key。
- 非法敏感 key 写入被拒绝。
- 默认值不散落在多个页面。

验证方式：

- settings service 单元测试。
- `pnpm --dir apps build`。
- `go test ./...`。

### [x] S0-04 定义 UI 禁用、待接入和后端不支持的统一呈现方式

依赖：S0-01。

执行动作：

- 对后端暂不支持的按钮统一使用禁用态或 `message.info("该能力待接入")`。
- 对需要用户确认的功能标记 `[!]`，不继续 mock 数据。
- 清理页面中容易误导为真实能力的本地假成功提示。

交付物：

- UI 交互边界清单。
- 待接入按钮列表。

验收标准：

- 不存在“点击保存成功但后端没有保存”的配置。
- 不存在“点击测试成功但没有真实测试或明确本地模拟边界”的场景。

验证方式：

- 页面手工验收。
- 搜索 `mock`、`message.success`、`setTimeout` 等高风险位置并复核。

执行记录：

- 代理设置：移除本地 `setTimeout` 伪造的连接成功结果，默认展示“未测试 / 真实代理连接测试待接入”；代理测试、刷新、保存、清空、绕过规则保存均改为 `message.info(...待接入)`，避免误导为已写入后端。
- 数据源概览：行情连接测试、新闻同步、缓存清理、状态检测均改为待接入提示；保留已接真实 `settingsGet/settingsSet` 的数据源基础设置保存成功提示。
- 关于应用：检查更新已接入真实 `checkUpdate` command，展示当前版本、最新版本、更新状态和发布说明可用性；日志导出已接入真实 `exportLogs` command，由用户选择导出目录并写入二次脱敏后的日志包；LICENSE、用户手册和发布说明均使用内置静态页面。
- 已补充前端回归测试覆盖代理设置、数据源概览和关于应用的待接入语义，并复核设置中心剩余 `message.success` 均对应真实保存、删除、测试或缓存后端链路。

### [x] S0-05 复核股票与新闻抓取入库现状

依赖：S0-01。

执行动作：

- 复核股票搜索、行情快照、K 线、新闻资讯和调度 runner 的抓取入库链路。
- 标记哪些能力已经写库，哪些只是抓取和清洗。
- 把后续开发要补的入库闭环拆到 S5-00 到 S5-00E。

当前 review 结论：

- 股票基础信息：`StockSearchService.searchProvider` 在本地/FTS 无命中时调用 Provider，并通过 `UpsertStocks` 写入 `stocks`，同时写入搜索索引 outbox。
- 行情快照：`/api/market/quote` 缓存 miss 后调用 Provider，并通过 `SaveQuote` 写入 `quotes`。
- K 线：`/api/market/kline` 缓存不足时调用 Provider，并通过 `SaveKlines` 写入 `klines`。
- 指标：`/api/market/indicators` 基于 K 线缓存或 Provider 结果计算，指标结果不单独落库。
- 新闻：`/api/news/list` 和 `/api/news/market` 缓存不足时调用 Provider，标准化、去重后通过 `SaveNewsItems` 写入 `news_items`。
- 调度：`StockProfileRefreshRunner`、`QuoteRefreshRunner`、`KlineRefreshRunner`、`NewsRefreshRunner` 均包含写库和 `IngestionWatermark` 更新逻辑。

仍需补齐：

- 股票基础资料已补 active watchlist 主动刷新；全量种子入库流程暂不扩展。
- 行情快照只保留每个 symbol 最新一条，不满足历史行情统计。
- 市场新闻缓存已补 `market` 字段和 `ListMarketNews` 市场过滤；历史数据迁移和前端多市场筛选仍需按需求推进。
- UI 对接前缺少统一的“入库闭环验收任务”，需要在 S5-00 系列执行。

交付物：

- 本清单中的专项任务。
- 后续开发按 S5-00 系列逐项验收入库闭环。

验收标准：

- 文档明确现有写库点和剩余缺口。
- 后续 UI 接入前可以按表定位应该调用哪个读库接口。

验证方式：

- 代码 review。
- `git diff --check`。

---

## 3. RG-S1：设置中心普通配置

### [~] S1-01 基础设置接入真实 settings 存储

依赖：S0-03。

执行动作：

- 页面加载调用 `settingsGet` 回填主题、语言、默认市场、行情刷新间隔、默认 K 线周期、默认复权方式。
- 保存时调用 `settingsSet`。
- 默认 AI 模型不在基础设置维护第二份状态，改为读取模型配置默认项或只读展示。

交付物：

- `SettingsBasicPage` 或对应组件真实读写 settings。
- 缺失配置项的默认值集中化。

验收标准：

- [x] 页面加载调用 `settingsGet` 回填主题、语言、默认市场、行情刷新间隔、默认 K 线周期、默认复权方式。
- [x] 基础设置普通项修改后立即调用 `settingsSet` 保存。
- [x] 默认 AI 模型不在基础设置维护第二份状态，读取并保存模型配置中的 `is_default`。
- [x] Go core 保存默认 AI 模型时保持 `ai_configs.is_default` 单一默认。
- [ ] 保存后刷新页面仍能回显，需要在桌面应用中手工验收。
- [ ] 行情刷新间隔和 K 线默认值被相关页面使用。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend build`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./...`。
- `cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml`。
- 前端手工保存/刷新。

### [~] S1-02 工作区默认目录接入和回显

依赖：S0-03。

执行动作：

- 首次启动按平台生成默认工作区。
- macOS 默认 `~/Documents/Invest Compass`。
- Windows 11 默认 `%USERPROFILE%\Documents\Invest Compass`。
- 页面加载调用 `workspaceGet` 回显。
- 保存调用 `workspaceSet`，但目录切换必须进入迁移流程。

交付物：

- 工作区默认目录策略。
- 工作区页面真实回显。

验收标准：

- [x] 默认目录位于用户个人目录下：macOS/Windows 均从系统 Documents 目录派生 `Invest Compass`。
- [x] Tauri 启动 sidecar 和手动 `core_start` 使用同一默认工作区策略。
- [x] 页面加载调用 `workspaceGet` 回显真实工作区路径。
- [x] 读取响应未持久化 `workspace_path` 时，Rust 使用当前平台默认目录回填。
- [x] 空路径、相对路径被 Rust/Go 边界拒绝。
- [x] 不可写路径在迁移预检阶段拒绝。
- [x] 目录切换进入 S1-03 迁移流程，不直接调用 `workspaceSet` 切换。
- [ ] macOS 桌面应用手工刷新回显待验收。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx`。
- `pnpm --dir apps/frontend build`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
- macOS 手工验收。

进展记录：

- 2026-06-24 已将工作区设置从基础设置聚合卡片拆为设置中心一级 Tab，继续复用 `workspaceGet`、`workspaceMigrationPlan`、`workspaceMigrate`、`workspaceOpen` 真实链路。

### [~] S1-03 工作区迁移预检与执行能力

依赖：S1-02。

执行动作：

- [x] 新增 Rust 白名单命令 `workspace_migration_plan({ target_path })`，只做文件系统预检，不修改数据。
- [x] 新增 Rust 白名单命令 `workspace_migrate({ target_path, include_cache, create_backup })`。
- [x] 前端 `workspaceMigrationPlan` / `workspaceMigrate` 使用 typed invoke service 封装。
- [x] 点击“选择目录”进入系统目录选择器，预检通过后展示确认迁移弹窗。
- [x] 确认迁移时默认创建源工作区备份，默认不迁移缓存目录。
- [x] 迁移前停止 Go Core，复制完成后用目标工作区重启并写入 `workspace_path`。
- [x] 迁移失败或目标重启失败时尝试恢复原工作区 Go Core，且不写入新 `workspace_path`。
- [x] “打开目录”按钮接入 Rust 白名单命令 `workspace_open`，路径只来自当前工作区配置或平台默认目录。
- [x] 迁移预检读取目标目录所在磁盘可用空间，并在弹窗中展示预检状态、迁移数据量和可用空间。

交付物：

- [x] Rust command 白名单。
- [x] Rust 文件迁移实现。
- [x] 前端迁移确认弹窗。
- [x] 工作区目录打开命令。
- [x] 设置页交互测试。

验收标准：

- [x] 目标目录空间不足时禁止迁移：当前不新增依赖，macOS/Linux 使用 `df -Pk`、Windows 使用 PowerShell 读取目标卷可用空间。
- [x] 前端显示迁移预检结果；预检不通过时保留弹窗但禁用“确认迁移”按钮。
- [x] 目标目录已有内容时默认取消，不提供覆盖/强制迁移。
- [x] 目标目录位于当前工作区内部时默认取消，避免递归复制污染源目录。
- [x] 默认迁移 `invest-compass.sqlite3`、`invest-compass.sqlite3-wal`、`invest-compass.sqlite3-shm`、`logs/`。
- [x] `backups/` 默认不迁移，`cache/` 仅在用户勾选后迁移。
- [ ] 迁移成功后报告、必要数据库、索引和配置快照可读，需要 macOS 桌面端实际迁移验收。
- [ ] Windows 11 迁移路径、权限和重启验收待后续人工验证。
- [x] 迁移失败保留原工作区可用的 Rust 单元测试覆盖核心文件复制边界。

验证方式：

- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::settings::tests`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx`。
- `pnpm --dir apps/frontend build`。
- macOS 临时目录手工迁移。
- Windows 11 后续人工验收。

### [~] S1-04 桌面设置接入开机自启和关闭行为

依赖：S0-03。

执行动作：

- [x] 开机自启接入 `autostartGet/autostartSet`。
- [x] 页面加载读取 `autostart_get` 并回显系统开机自启状态。
- [x] 开机自启切换后调用 `autostart_set`，失败时回滚 UI 状态。
- [x] 关闭到托盘读取 `window.close_to_tray` settings 并回显。
- [x] 关闭到托盘切换后调用 `settings_set(window.close_to_tray)`，失败时回滚 UI 状态。
- [x] Rust runtime 已按 `window.close_to_tray` 控制关闭窗口时隐藏到托盘或退出。
- [ ] “启动时最小化”当前基础设置页没有 UI 项，未新增入口；如后续需要需先确认页面设计和 Rust runtime 契约。

交付物：

- [x] 桌面设置真实读写。
- [x] 不支持项处理记录。
- [x] 设置页交互测试和 App 路由测试。

验收标准：

- [x] 开机自启刷新后状态一致：页面重新加载调用 `autostart_get`。
- [x] 关闭到托盘刷新后状态一致：页面重新加载调用 `settings_get(window.close_to_tray)`。
- [x] 不支持的桌面行为不会保存假状态：未新增“启动时最小化”入口。
- [ ] macOS 开机自启实际系统项写入/取消需要桌面端手工验收。
- [ ] macOS 关闭到托盘实际行为需要桌面端手工验收。
- [ ] Windows 11 开机自启和关闭到托盘行为需要后续人工验收。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx`。
- `pnpm --dir apps/frontend build`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
- macOS 手工验收。
- Windows 11 后续人工验收。

### [~] S1-05 缓存管理接入真实缓存统计和清理

依赖：无。

执行动作：

- [x] 页面加载调用 `cacheStats`。
- [x] 清理按钮先展示确认弹窗，不直接删除。
- [x] 确认后调用 `cacheClean`。
- [x] 清理完成后重新拉取统计。
- [x] 前端只提交后端标记为 `cleanable` 的缓存目标。
- [x] 弹窗明确不清理报告、配置、凭据。

交付物：

- [x] 缓存统计卡真实数据。
- [x] 清理确认弹窗。

验收标准：

- [x] 清理前后统计变化可见：前端测试覆盖清理后重新读取统计。
- [x] 清理失败展示错误信息：保留 `cache_clean` 异常提示。
- [x] 不删除业务数据：Go `cache_clean` 仅允许清理缓存目标，前端弹窗同步提示边界。
- [ ] macOS 桌面应用中清理缓存前后统计变化待手工验收。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx`。
- `pnpm --dir apps/frontend build`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./...`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
- `git diff --check`。
- 前端手工验收。

进展记录：

- 2026-06-24 已将缓存管理从基础设置聚合卡片拆为设置中心一级 Tab，继续复用 `cacheStats`、`cacheClean` 真实链路和清理确认弹窗。

### [~] S1-06 搜索索引状态和重建接入真实任务

依赖：无。

执行动作：

- [x] 页面加载调用 `searchStatus`。
- [x] 重建按钮调用 `searchRebuild`。
- [x] 返回 `task_id` 后提示可在任务历史查看。

交付物：

- [x] 搜索索引卡真实状态。
- [x] 重建任务提示。

验收标准：

- [x] 索引状态来自后端：页面初始化调用 `search_status` 并展示 FTS5、索引数量、批次和运行任务。
- [x] 重建失败不展示成功：`searchRebuild` 异常路径只展示错误提示。
- [x] 重建请求只使用固定 scope，不调用旧的全局搜索入口。
- [x] 重建成功提示任务 ID 和“可在任务历史查看”。
- [ ] 任务 ID 在任务历史页面可追踪到对应任务，需要桌面端端到端验收。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/actions/search ./internal/service/search`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::search::tests`。
- 前端手工验收。

---

## 4. RG-S2：模型、Prompt、代理

### [~] S2-01 模型配置页接入 `aiConfigList`

依赖：S0-02。

执行动作：

- [x] 设置中心模型配置页加载时调用 `aiConfigList`。
- [x] 列表展示 provider、model、base_url、masked_api_key、is_default、stream_enabled。
- [x] 空列表展示 AntD 空态，不造假配置。
- [x] 新增、编辑、删除、默认、测试连接暂不做本地假保存，统一提示待接入真实保存。

交付物：

- [x] 设置中心模型配置真实列表。
- [x] 编辑类操作的待接入边界。

验收标准：

- [x] 有配置时完整展示。
- [x] 无配置时展示空态，不注入默认假配置。
- [x] API Key 只展示脱敏值。
- [x] 编辑类操作不会触发本地假保存或假删除。
- [ ] 桌面端刷新设置中心模型配置页后仍可回显，需要手工验收。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/model-config/ModelConfigPage.test.tsx`。
- `pnpm --dir apps/frontend build`。
- Go AI config 测试。

### [~] S2-02 模型配置新增、编辑、删除和默认模型

依赖：S2-01。

执行动作：

- [x] 新增/编辑调用 `aiConfigSave`。
- [x] 删除调用 `aiConfigDelete`。
- [x] 默认模型通过 `is_default` 维护。
- [x] 流式输出切换调用 `aiConfigSave`，不做本地假保存。
- [x] 明文 API Key 只提交一次到 Rust vault，保存前先从前端表单 state 清空，不进入前端持久化。

交付物：

- [x] 模型配置表单真实保存。
- [x] 删除确认弹窗。

验收标准：

- [x] 新增后列表使用后端返回的脱敏 `masked_api_key` 回显。
- [x] 默认模型保存调用 `aiConfigSave({ is_default: true })`。
- [x] 删除确认后调用 `aiConfigDelete` 并更新列表。
- [x] 前端不持久化真实 API Key，保存后页面不展示本次输入的明文 Key。
- [ ] 刷新后配置回显需要桌面端手工验收。
- [ ] 默认模型唯一性由 Go core 保证，需配合 Go AI config 测试确认。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/model-config/ModelConfigPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx`。
- `pnpm --dir apps/frontend build`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/service/ai ./internal/actions/aiconfig`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::ai_config::tests`。
- secret 搜索检查。

### [x] S2-03 模型连通性测试接入真实测试接口

依赖：S2-02。

执行动作：

- [x] 测试按钮调用 `aiConfigTest`。
- [x] 测试期间展示 `测试中` 状态。
- [x] 测试结果只展示状态和后端返回耗时，不展示密钥；错误摘要进入页面前脱敏。

交付物：

- [x] 模型测试交互。

验收标准：

- [x] 成功和失败状态都能展示。
- [x] 前端测试调用只传 `id` 与 `api_key_ref`，不传真实 API Key。
- [x] 上游错误提示进入页面前会做敏感字段脱敏。
- [x] API Key 不进入日志、任务事件或前端类型；已通过 handler 日志捕获测试和源码搜索抽样确认。
- [x] `AIConfigTestResult` 已扩展 `duration_ms` 字段，由 Go core 连通性测试返回，Rust 透明转发，前端在连接状态列展示。

实现记录：

- 2026-06-24：`ai.TestResult` 新增 `duration_ms`，前端连接状态缓存同步保存测试耗时；模型配置表在状态标签下方显示 `xxx ms`。
- 2026-06-24：补充 `TestAIConfigTestRedactsProviderErrorFromLogs`，验证 Provider 错误日志和响应都不会泄露 `resolved_api_key`；`ai_config_test` 不写任务事件，前端 `AIConfigTestResult` 不包含密钥字段。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/model-config/ModelConfigPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts src/app/App.test.tsx`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/service/ai ./internal/actions/aiconfig`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::ai_config::tests`。
- [x] 日志脱敏检查：`go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run TestAIConfigTestRedactsProviderErrorFromLogs -count=1`。

### [x] S2-04 Prompt 模板列表和详情接入后端

依赖：S0-02。

执行动作：

- [x] 页面加载调用 `promptTemplatesList`。
- [x] 查看详情调用 `promptTemplatesGet`。
- [x] 只允许 `system`、`stock_full`、`technical`、`fundamental`、`news`、`custom` 类型。
- [x] UI 中后端不支持的分类能力禁用或改为前端筛选。

交付物：

- [x] Prompt 模板真实列表和详情。

验收标准：

- [x] 刷新后模板仍可见：页面初始化固定调用后端列表接口。
- [x] 非白名单 type 被拒绝：前端分类收敛为当前六类，后端继续保留校验。
- [x] 变量白名单提示准确：变量说明和保存前校验均使用当前 Prompt service 白名单。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/prompt-template/PromptTemplatePage.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts --reporter=basic`。
- `pnpm --dir apps/frontend check`。
- `pnpm --dir apps/frontend build`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/service/prompt ./internal/actions/prompt`。

执行记录：

- 2026-06-24 已新增 `prompt_templates.key/version/checksum/builtin_locked/source` 字段，API 列表和详情返回内置模板元数据。
- 2026-06-24 已通过 Go `embed` 打包 5 个内置 Prompt Markdown，并在 Go core 启动迁移后 seed 到 SQLite。
- 2026-06-24 内置模板按 `builtin_locked=true` 只读，前端禁用编辑、保存、删除，保留创建自定义副本入口。

### [x] S2-05 Prompt 模板新增、编辑、删除

依赖：S2-04。

执行动作：

- [x] 新增调用 `promptTemplatesCreate`。
- [x] 编辑调用 `promptTemplatesUpdate`。
- [x] 删除调用 `promptTemplatesDelete`。
- [x] 保存前校验变量白名单。

交付物：

- [x] Prompt 表单真实保存。

验收标准：

- [x] 新增、编辑、删除刷新后状态一致：页面状态以后端返回为准，重新加载仍调用列表接口。
- [x] 非白名单变量不能保存：前端保存前阻断，后端服务继续校验。
- [x] 删除默认模板前有确认或后端保护：前端阻断内置模板删除，后端 CRUD API 继续通过 `builtin_locked` 保护内置模板。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/prompt-template/PromptTemplatePage.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend check`。
- `pnpm --dir apps/frontend build`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/service/prompt ./internal/actions/prompt`。

### [x] S2-06 代理设置保存和脱敏回显

依赖：S0-03。

执行动作：

- [x] 普通代理字段走 `settingsGet/settingsSet`。
- [x] 首版运行时只支持无认证手动代理，页面不展示认证代理保存入口。
- [x] 历史代理密码引用不进入普通 settings items，保存无认证代理时清理旧 `proxy_credential_ref`。

交付物：

- [x] 无认证代理配置真实保存。
- [x] 代理密码 vault 能力保留，但运行时认证代理入口关闭，避免半成品生效状态。

验收标准：

- [x] 刷新后普通代理配置回显。
- [x] 密码不明文回显，且前端不展示认证代理输入框。
- [x] SQLite 不保存代理密码明文；保存无认证代理会通过 `clear_proxy_credential` 清理旧代理凭据引用。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/proxy/ProxySettingsPage.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend check`。
- `pnpm --dir apps/frontend build`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::settings::tests::build_settings_forward_plan`。
- `git diff --check`。
- `rg -n "proxy_password|proxy_credential_ref|local-vault://proxy|new-proxy-secret|Proxy-Authorization" apps/frontend/src/pages/settings/proxy apps/frontend/src/app/App.test.tsx apps/desktop/src-tauri/src docs/2026-06-22-invest-compass-settings-ui-backend-development-checklist.md`。

执行记录：

- 代理页初始化读取 `proxy.mode`、`proxy.http_url`、`proxy.socks5_url`、`proxy.no_proxy`、`proxy.username`、`proxy_credential_ref`，刷新后可从真实 settings 回显普通代理配置。
- HTTP / SOCKS5 保存只把无认证普通字段放入 `items`，并携带 `clear_proxy_credential` 清理旧代理凭据引用。
- 页面检测到已有 `proxy_credential_ref` 时不回显 vault 引用，也不展示认证代理输入框；页面提示首版仅支持无认证代理。
- Rust settings command 的 vault 转发能力保留为安全边界基线；认证代理要真正进入运行时前，必须另行补 Rust 解密并注入 Go 内存凭据的协议。

### [x] S2-07 代理连接测试接口确认

依赖：S2-06。

执行动作：

- [x] 新增固定 Rust command `proxy_connection_test({ target })`，只转发到 Go core `/api/proxy/test`。
- [x] Go core 只接受 `baidu`、`google`、`openai`、`deepseek` 这类受控目标，不开放任意 URL。
- [x] 返回状态、HTTP 状态码、耗时、检查时间和脱敏错误摘要。
- [x] 代理设置页测试连接按钮调用真实后端命令，不再展示本地伪造结果。
- [x] 外部数据 Provider、数据源凭据校验、AI 模型测试和 AI 分析任务共用动态代理 HTTP client，每次请求按最新 settings 解析 `system` / `none` / `custom`。

验收标准：

- [x] 不允许用户输入任意测试 URL。
- [x] 不输出 `Proxy-Authorization`。
- [x] 失败可定位到连接失败、上游错误或超时，不输出代理凭据。
- [x] 用户选择“不使用代理”后，外部数据和 AI 请求不使用系统代理或手动代理。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/proxy/ProxySettingsPage.test.tsx src/services/coreClient.test.ts --passWithNoTests`。
- `go test ./cmd/invest-compass-core ./internal/service/netproxy ./internal/service/ai ./internal/service/analysis ./internal/actions -run 'TestDynamic|TestOpenAIConfigTesterUsesInjectedHTTPClient|TestExecutorDefaultChatClientUsesInjectedHTTPClient|TestProxy|Proxy|^$' -count=1`。
- `cargo test --lib commands::settings::tests::validate_proxy_test_target_rejects_arbitrary_url`。

执行记录：

- 2026-06-24：代理页已合并 HTTP / SOCKS 为“手动代理”，新增“不使用代理”，并保留旧 `proxy.mode=http/socks5` 的读取兼容。
- 2026-06-24：Go `netproxy.DynamicClientForSettings` 会在每次外部请求前读取最新代理 settings；回归测试覆盖从手动代理切到 `none` 后同一个 client 的下一次请求直连。
- 2026-06-24：AI 配置连通性测试和 AI 分析执行器已支持注入运行时 HTTP client，生产组装统一注入动态代理 client。

---

## 5. RG-S3：数据源与凭据安全

### [x] S3-01 数据源概览接入真实 Provider 状态

依赖：无。

执行动作：

- [x] 数据源概览调用 `providersStatus`。
- [x] 缓存统计复用 `cacheStats`。
- [x] 同步/调度状态调用已实现的 `schedulerStatus`。
- [x] 无数据时展示空态或待配置，不展示假正常。

交付物：

- [x] 数据源概览真实状态卡。

验收标准：

- [x] Provider 状态来自后端。
- [x] Provider 异常时可见错误态。
- [x] 页面不再展示假 Provider 健康数据。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/data-source/DataSourceSettingsPage.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend check`。
- `pnpm --dir apps/frontend build`。
- `git diff --check`。

执行记录：

- 数据源概览加载时并行读取 `providersStatus()`、`cacheStats()`、`schedulerStatus()`，不再使用静态 Provider 健康状态、静态缓存大小或静态同步策略。
- 行情源、新闻源和状态摘要均由 `ProviderStatusItem.name/source/available/last_error` 派生；`available=false` 显示异常或未配置，不回退到假正常。
- 本地缓存卡片展示 `cacheStats` 返回的行情缓存、新闻缓存和总量；后端未提供的快照时间明确显示“后端未提供”。
- 同步任务卡片展示 `scheduler_status` 的启用任务数、排队/运行数和失败数，不再展示后端未提供的预热、重试次数、同步间隔等静态策略。
- `重新检测` 改为重新拉取真实概览状态；连接测试、新闻同步、日志、调度配置、缓存清理仍保持待接入提示，避免假成功。

### [x] S3-02 数据源凭据加密 SQLite schema 确认

依赖：S0-01。

执行结果：

- 2026-06-23 已新增 `DataSourceCredential` 模型并纳入 GORM migration。
- SQLite 表保存 Provider 配置、脱敏值、AES-GCM 密文、nonce 和最近真实连接测试状态。
- 加密密钥保存在工作区 `credentials/data-source.key`，不写入 SQLite。

已实现 schema：

```text
data_source_credentials
  id
  provider_id
  provider_name
  capability
  auth_type
  base_url
  credential_status
  encrypted_credential
  credential_nonce
  masked_credential
  note
  expires_at
  last_tested_at
  last_test_status
  last_test_response_time
  last_test_messages
  created_at
  updated_at
  deleted_at
```

推荐边界：

- token/cookie/key 加密后可存 SQLite。
- 加密密钥不得存 SQLite。
- 前端不得读取已保存明文。
- Go 使用凭据时只能在内存中短暂解密。

验收标准：

- SQLite 中没有明文 token/cookie/key。
- 加密密钥和密文不在同一存储层。
- 导出日志会二次脱敏。

验证结果：

- `go test ./internal/service/datasourcecredential ./internal/actions/datasourcecredential`
- `pnpm --dir apps check`

### [x] S3-03 数据源凭据 API 和 Rust command 白名单

依赖：S3-02 确认。

执行动作：

- 新增凭据列表、读取脱敏状态、保存、清除、测试接口。
- Rust command 固定 path 和 schema。
- 保存时只接受明文一次，返回脱敏状态。
- 读取接口不返回明文。

交付物：

- `dataSourceCredentialsList/Save/Clear/Test`。
- `data_source_credentials_list/save/clear/test`。
- `POST /api/data-source/credentials/list/save/clear/test`。
- Go service/dao/model/action。
- Rust command 固定 path 安全护栏。

验收标准：

- 保存后刷新只回显 masked credential。
- 清除后状态变为未配置。
- 测试连接不输出凭据。

验证方式：

- Go 凭据 service 测试。
- Rust command 测试。
- secret 搜索检查。

验证结果：

- `node --test apps/desktop/test/security-config.test.mjs`
- `pnpm --dir apps --filter @invest-compass/desktop test`
- `pnpm --dir apps check`

### [~] S3-04 凭据管理页接入真实脱敏状态

依赖：S3-03。

执行动作：

- Provider 列表来自 `providersStatus` 和凭据列表。
- 表单加载脱敏配置。
- 保存调用凭据保存接口。
- 清除调用凭据清除接口。
- 测试调用凭据测试接口。

交付物：

- 凭据管理页真实数据接入。
- 已删除 `credentials/mock.ts`。
- `DataSourceCredentialPage` 改为调用 typed service。

验收标准：

- 已保存凭据不可通过 eye icon 还原。
- 用户本次输入可显示/隐藏，但保存后只回显脱敏值。
- 不写 localStorage/sessionStorage/IndexedDB。

验证方式：

- 页面手工验收。
- `pnpm --dir apps build`。

阻塞记录：

- 2026-06-23 执行 FE03 桌面安全护栏时，`DataSourceCredentialPage` 仍命中 mock 数据扫描。
- 当前页面只能在 S3-02 和 S3-03 确认后接入真实脱敏状态；确认前不能继续保留假 Provider 凭据状态作为真实能力。

推进记录：

- 2026-06-23 已接入真实脱敏状态、保存、清除、真实连接测试。
- 2026-06-23 已将股票数据源拆分为无需凭据的 `sina` 与 `tencent` 两个 Provider；凭据页分别展示“新浪财经”和“腾讯财经”，真实连接测试目标分别为新浪行情 `/list=sh000001` 与腾讯 K 线 `/appstock/app/fqkline/get?param=sh000001,day,,,2,qfq`。
- 2026-06-23 数据源概览“默认行情源”已补充“自动降级”选项；Go runtime 已读取 `data_source.default_market_source`，并沿用当前新浪搜索/实时行情、腾讯 K 线、东财 K 线兜底的生产 Provider 链路。
- 显式选择单一行情源后的完整按源路由策略仍需单独确认；本期只验收“自动降级”进入运行时读取链路。
- 仍需在运行中页面手工验收 1440x900 布局、保存后刷新回显、清除后状态和错误提示。
- 2026-06-24 补充前端自动化验收：凭据管理页保存时只把本次输入明文作为一次性 `credential` 传给 typed service，保存后输入框只回显后端返回的脱敏值，并断言不写 `localStorage` / `sessionStorage` / `IndexedDB`。

### [x] S3-05 Provider 使用凭据的运行时注入

依赖：S3-03。

执行动作：

- Provider 调用前按 provider_id 获取并解密凭据。
- 凭据只在请求生命周期或受控任务内存中使用。
- Provider 错误映射为可展示状态，不泄露 header。

交付物：

- Provider credential resolver。
- Provider 调用测试。

验收标准：

- 未配置凭据的数据源不返回假数据。
- 凭据失效能生成 Provider 异常状态。
- 日志不包含 Cookie、Authorization、Proxy-Authorization。

验证方式：

- Provider 单元测试。
- 日志脱敏测试。

进展记录：

- 2026-06-23 已新增 `datasourcecredential.Service.Resolve`，可按 `provider_id` 解密并生成仅供 Go core 内部短暂使用的运行时凭据结构。
- 2026-06-23 已覆盖无需凭据 Provider、雪球 Cookie 解密、缺失凭据失败路径；雪球热点 Provider 已有缺 Cookie 不抓取、注入 Cookie 后请求携带 Cookie 的离线单测。
- 生产请求是否切换到雪球热点、财联社、Alpha Vantage 或其他真实 Provider，需要结合页面入口和数据授权边界确认；确认前不把 `UnconfiguredProvider` 静默替换为真实外部访问。
- 2026-06-23 用户确认先不切生产 Provider，只保留 resolver 和离线测试；真实外部访问接入需后续按 Provider 授权边界单独确认。
- 2026-06-24 用户确认改为激进方案，要求真实 Provider 能测试可用；已补 `provider:credential-smoke`，命令会读取真实工作区 SQLite 与 data-source vault key，复用动态代理 HTTP client 和 `datasourcecredential.Service.Test`，只输出 provider、target、状态、耗时和脱敏消息。
- 2026-06-24 已修复凭据页真实连接测试目标：`eastmoney` 命中东财基础证券列表接口、`sina` 带来源头命中行情接口、`tencent` 命中 K 线接口、`alpha-vantage` 使用 `GLOBAL_QUOTE` 并把 API Key 注入 `apikey` 查询参数、`xueqiu` 命中热股榜；东财 K 线仍按 `limited` 能力处理，不能用基础列表连通冒充 K 线验收通过。
- 2026-06-24 用户补齐密钥后重新执行全量真实 smoke：`sina`、`tencent`、`akshare` 基础连通、`alpha-vantage`、`cls`、`xueqiu` 通过；`eastmoney` 的 `push2his` K 线 API 在当前网络返回 EOF。此前按严格 K 线口径保持阻塞，不能用东财网页 200 或无关接口冒充 K 线通过。
- 2026-06-24 用户确认本期隐藏数据源 `custom-http`；后端 Provider catalog、凭据列表和真实 smoke 枚举均不再暴露该 Provider，旧数据库残留配置也不会重新显示。
- 2026-06-24 已按严格口径处理 EastMoney：凭据目录默认状态改为 `limited`，能力说明收窄为 `基础证券列表可访问 / K线受限`；旧存量配置或保存操作不能把它重新展示为 `normal`。
- 2026-06-24 降级东财验收口径时曾尝试 `push2` 行情查询接口；复测发现 `clist` 和 `ulist.np/get` 都存在间歇性 EOF，不能作为“全量 smoke 必须通过”的稳定目标。
- 2026-06-24 用户确认采用推荐方案后，东财 smoke 目标最终改为 `datacenter-web.eastmoney.com/api/data/v1/get` 基础证券列表；能力文案收窄为 `基础证券列表可访问 / K线受限`。K 线主源仍由腾讯承担，东财 K 线兜底不作为本期通过条件。
- 2026-06-24 最终全量真实联网 smoke 通过：`eastmoney security_list`、`sina quote`、`tencent kline`、`akshare connectivity`、`alpha-vantage quote`、`cls flash`、`xueqiu hot_stock` 均返回 HTTP 200 且响应可读取。
- 2026-06-24 补充验证：
  - `go test -mod=readonly ./internal/service/datasourcecredential -run 'TestPreflightURLSupportsCredentialedProviderTargets|TestListMarksEastMoneyCapabilitiesAsLimited'`
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "数据源设置凭据管理页展示脱敏凭据并仅使用本地交互" --reporter=basic`
  - `pnpm --dir apps provider:credential-smoke -- --workspace "$HOME/Documents/Invest Compass" --providers all --allow-network --confirm-provider-terms`

### [x] S3-06 数据说明页保留静态说明并校准链接

依赖：S0-04。

执行动作：

- 保留数据说明页静态解释内容。
- “查看数据源概览”“查看凭据管理”等链接只跳转到已存在子 tab。
- 不存在页面的链接使用待接入提示。

交付物：

- 数据说明页链接策略。

验收标准：

- 不打开外链。
- 不出现交易、下单、券商账户等未闭环能力。

验证方式：

- `pnpm --filter @invest-compass/frontend test -- DataSourceSettingsPage`。

进展记录：

- 2026-06-23 已将顶部“查看数据源概览”和 A 卡片“查看数据源”接入到已存在的“数据源概览”子 Tab。
- 2026-06-23 已将“查看凭据管理”接入到已存在的“凭据管理”子 Tab。
- 字段说明、FAQ 详情和更多 FAQ 仍为待接入提示，不打开不存在页面或外链。
- 2026-06-24 补充页面级自动化验收：数据说明页按钮只在已有子 Tab 间跳转，字段说明/FAQ 按钮保留当前页并显示待接入提示，不打开外链，不暴露下单、券商账户或自动交易入口。

---

## 6. RG-S4：通知与工作区迁移

### [x] S4-01 通知设置项接入 settings

依赖：S0-03。

执行动作：

- [x] 新增并保存 `notifications.in_app_enabled`。
- [x] 新增并保存 `notifications.system_enabled`。
- [x] 接入任务成功、任务失败、Provider 异常通知开关。
- [x] 可选声音提示首版不做或禁用。

交付物：

- [x] 通知设置真实保存和回显。

验收标准：

- [x] 刷新后开关状态一致。
- [x] 关闭应用内通知后不展示应用内通知：设置项已接入；通知生产和浮层消费归属 S4-02 至 S4-04。
- [x] 关闭系统通知后不调用 Tauri 通知：设置项已接入；系统通知触发归属 S4-04。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx --reporter=basic`。
- `pnpm --dir apps/frontend check`。
- `pnpm --dir apps/frontend build`。

进展记录：

- 2026-06-23 已在基础设置页接入 `notifications.in_app_enabled`、`notifications.system_enabled`、`notifications.task_success`、`notifications.task_failed`、`notifications.provider_error` 的真实 `settings_get/settings_set` 读写。
- 2026-06-23 已补充前端测试覆盖通知设置读取、逐项保存和 App 级基础设置展示。
- 2026-06-24 已将通知设置从基础设置聚合卡片拆为设置中心一级 Tab，一级入口继续复用同一组真实 settings 读写。
- 应用内通知表/API、TopBar 未读角标/列表浮层、系统级通知实际触发仍属于 S4-02 至 S4-04，未在本任务中实现。

### [x] S4-02 应用内通知表和 API

依赖：S4-01。

执行动作：

- [x] 新增 SQLite `notifications` 表，保存通知摘要、源对象引用、路由、已读状态和时间戳。
- [x] 新增 Go service/dao/action：分页列表、未读数量、标记已读、全部已读、清理已读。
- [x] 新增 Rust 白名单命令：`notifications_list`、`notifications_unread_count`、`notifications_mark_read`、`notifications_mark_all_read`、`notifications_clear_read`。
- [x] 新增前端 typed invoke service 封装，后续 TopBar 只允许从 `coreClient.ts` 取数。
- [x] 通知标题和正文在写入和响应边界做脱敏处理。
- [x] TopBar 角标和浮层接入，归属 S4-03。
- [x] 任务和 Provider 事件生产通知，归属 S4-05。

已落地 schema：

```text
notifications
  id
  type
  level
  title
  content
  source_type
  source_id
  route
  is_read
  created_at
  updated_at
  read_at
  deleted_at
```

已落地 API：

```text
notificationsList({ unread_only, limit, offset })
notificationsUnreadCount()
notificationsMarkRead({ ids })
notificationsMarkAllRead()
notificationsClearRead()
```

验收标准：

- [x] 未读数量可查询。
- [x] 通知列表可分页。
- [x] 点击通知可按 route 跳转，已由 S4-03 TopBar 浮层接入后验收。
- [x] 清理已读不影响任务、报告等源数据，仅清理通知表已读记录。
- [x] Rust command 安全扫描确认通知 command 固定映射，不存在通用代理。

验证方式：

- [x] `go test ./internal/service/notification ./internal/actions/notification`
- [x] `pnpm --dir apps --filter @invest-compass/frontend test -- src/services/coreClient.test.ts`
- [x] `node apps/desktop/test/security-config.test.mjs`

### [x] S4-03 TopBar 通知角标和浮层

依赖：S4-02 确认并实现。

执行动作：

- TopBar 通知按钮包裹 AntD `Badge`。
- 点击弹出通知列表浮层。
- 支持标记已读、全部已读、跳转源页面。
- 空列表显示空态。

交付物：

- 全局通知入口真实接入。

验收标准：

- 未读数量实时更新。
- 点击通知可跳到报告、任务、设置等白名单页面；未知 route 只标记已读，不跳转。
- 浮层关闭后状态不丢失。

验证方式：

- [x] 前端自动化覆盖 TopBar Badge、Popover、单条已读和白名单 route 跳转。
- [ ] 前端手工验收。
- [x] `pnpm --dir apps --filter @invest-compass/frontend test -- src/app/App.test.tsx -t "TopBar"`
- [x] `pnpm --dir apps --filter @invest-compass/frontend check`
- [x] `pnpm --dir apps --filter @invest-compass/frontend build`

### [!] S4-04 系统级通知接入 Tauri 通知插件

依赖：S4-01。

执行动作：

- [x] 复用 `@tauri-apps/plugin-notification` 和 Rust `tauri-plugin-notification`。
- [x] 首次使用前检查并请求权限。
- [x] 系统通知触发服务按任务成功、任务失败、Provider 异常开关决定是否触发。
- [x] 权限被拒绝时只保留应用内通知链路，不抛错中断业务。
- [x] 接入真实任务和 Provider 事件后触发系统通知，归属 S4-05。

交付物：

- [x] 系统通知触发服务。
- [x] 权限降级逻辑。
- [x] 敏感字段脱敏逻辑。

验收标准：

- [ ] macOS 能看到系统通知。
- [x] 权限拒绝不会报错中断任务。
- [x] 通知内容不包含敏感凭据。

验证方式：

- [ ] macOS 手工验收。
- [x] Tauri permission 检查。
- [x] `pnpm --dir apps --filter @invest-compass/frontend test -- src/services/desktopNotification.test.ts`
- [x] `pnpm --dir apps --filter @invest-compass/frontend check`
- [x] `pnpm --dir apps --filter @invest-compass/frontend build`
- [x] `node apps/desktop/test/security-config.test.mjs`

阻塞记录：

- 2026-06-23 自动化验证已通过；剩余 `macOS 能看到系统通知` 需要在真实 Tauri 桌面运行态由用户手工验收，当前不冒充完成。

### [!] S4-05 任务和 Provider 事件生成通知

依赖：S4-02、S4-04。

执行动作：

- [x] 分析任务 `SUCCESS` / `FAILED` 终态写入应用内通知。
- [x] Provider 状态接口发现已配置 Provider 不可用且有 `LastError` 时写入应用内通知。
- [x] Provider 未配置空态不生成异常通知，避免启动后刷无效通知。
- [x] 系统通知消费新增应用内通知，并按设置开关触发系统通知。

交付物：

- 应用内通知事件生产链路。
- 系统通知事件消费链路。

验收标准：

- [x] 同一任务完成不会重复生成多条通知。
- [x] Provider 异常通知按 `source_type + source_id + type` 去重。
- [x] 任务成功优先跳转报告详情；无报告时跳转任务历史。
- [x] 任务失败跳转任务历史。
- [x] Provider 异常跳转基础设置页。
- [x] 系统级通知自动触发逻辑。
- [ ] macOS 系统通知手工验收。

验证方式：

- [x] `GOCACHE=/private/tmp/invest-compass-go-cache go test ./internal/service/notification ./internal/service/analysis ./internal/actions/providers`
- [x] `pnpm --dir apps --filter @invest-compass/frontend test -- src/app/App.test.tsx -t "TopBar"`
- [x] `pnpm --dir apps --filter @invest-compass/frontend check`
- [ ] 前端手工验收：生成任务成功/失败通知、点击通知跳转。
- [ ] 前端手工验收：Provider 异常通知点击后跳转基础设置页。
- [ ] macOS 手工验收：系统通知中心可见任务成功、任务失败和 Provider 异常通知。

进展记录：

- 2026-06-23 已接入 Go `notification.Service.NotifyTaskTerminal` / `NotifyProviderError`。
- 2026-06-23 已在分析任务执行器成功/失败终态后生成应用内通知；通知失败不反向改变任务终态。
- 2026-06-23 已在 Provider 状态接口对真实异常生成应用内通知，并跳过未配置 Provider 空态。
- 2026-06-23 已在 TopBar 运行期未读数增加时消费新增应用内通知，并按系统通知设置触发 Tauri 系统通知；首次加载不弹历史未读通知。
- 受限：完整 `internal/actions` / `cmd/invest-compass-core` 测试在当前环境受 SQLite FTS5 和监听权限限制，已通过受影响包窄范围测试。
- 2026-06-23 自动化验证已通过；剩余任务成功/失败、Provider 异常通知点击跳转和 macOS 系统通知中心可见性需要真实桌面运行态手工验收。

---

## 7. RG-S5：全局 UI 页面真实数据对接

### [x] S5-00A 验收股票基础信息搜索入库闭环

依赖：S0-05。

执行动作：

- 使用 `/api/stocks/search` 触发本地缓存、FTS、Provider fallback 三段搜索。
- 本地和 FTS 无命中时，Provider 返回的股票基础信息必须写入 `stocks`。
- 同步写入搜索索引 outbox，后续由搜索索引任务消费。
- Provider 未配置时返回错误，不写假股票。

当前实现位置：

- Action：`apps/sidecar-core/internal/actions/stocks/stocks.go`
- Service：`apps/sidecar-core/internal/service/search/stock_search.go`
- DAO：`apps/sidecar-core/internal/dao/store.go` 的 `UpsertStocks`
- Model：`apps/sidecar-core/internal/model/schema.go` 的 `Stock`

交付物：

- 股票搜索入库链路测试或手工验收记录。
- UI 自选股添加和全局搜索复用该链路。

验收标准：

- 第一次搜索 Provider 返回结果后，`stocks` 有对应 symbol。
- 第二次搜索优先命中本地或 FTS。
- 新股票写入后存在 pending 搜索索引任务。
- Provider 未配置时不返回假数据。

验证方式：

- `go test -tags sqlite_fts5 ./internal/service/search ./internal/actions/stocks ./internal/dao`，重点覆盖 `TestStockSearchServiceFallsBackToProviderAndEnqueuesIndexJobs`。
- `pnpm --filter @invest-compass/frontend test -- WatchlistPage`。
- `pnpm --filter @invest-compass/frontend test -- App -t "顶部搜索通过后端股票搜索跳转到首个真实结果"`。

进展记录：

- 2026-06-23 已通过后端自动化验证：
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/service/search ./internal/actions/stocks ./internal/dao -run 'TestStockSearch|TestSearchMigrationCreatesIndexSchema|TestProbeSQLiteFTS5ReportsAvailable'`
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/service/search ./internal/actions/stocks ./internal/dao`
- 2026-06-23 结论：Provider fallback 写入 `stocks`、搜索索引 outbox、FTS5 schema 探测和 Provider 未配置错误边界已有测试覆盖。
- 2026-06-24 已补充前端自动化验收：TopBar 全局搜索调用 `stock_search` 后跳转首个真实结果；自选股添加弹窗通过 `stock_search` 搜索 Provider 结果，再调用 `watchlist_create` 新增自选股。

### [x] S5-00B 补齐股票基础资料主动刷新或种子入库流程

依赖：S5-00A。

背景问题：

- 当前股票基础信息主要在搜索 fallback 时被动写入；如果用户没有搜索过某只股票，`stocks` 可能没有完整基础资料。
- 个股详情需要的公司资料、行业、概念、上市日期、状态等字段不能只依赖搜索触发。

执行方案：

- Go core 启动迁移完成后，通过 `go:embed` 打包的 `stock_basic.json` 幂等 seed 沪深 A 股基础资料到 `stocks`。
- 北交所 `BSE/BJ` 数据暂不导入；当前 `ParseSymbol` 尚未定义 `CN:BJ:*` 标准 symbol，避免引入不可解析代码。
- 内置 seed 使用 `UpsertBuiltinStocks` 写入全称、拼音、行业、上市日期和状态；远端搜索 fallback 仍使用 `UpsertStocks`，避免简版远端结果清空内置详情字段。
- 复用现有 `MarketProvider.Search` 能力，按 symbol code 拉取基础资料。
- 新增调度任务 `stock_profile_refresh`，对 active watchlist 做增量刷新。
- 刷新结果通过 `UpsertStocks` 写入 `stocks`，并写入搜索索引 outbox。
- 搜索索引重建读取 `stocks`，不直接访问 Provider。

验收标准：

- 首次启动后，本地 `stocks` 表已有沪深 A 股基础股票池；用户添加自选或打开详情时不再依赖“先搜索过”。
- 个股详情进入时即使未搜索过，也能按 symbol 获取基础资料或明确返回未配置错误。
- 基础资料刷新后 `stocks` 字段更新，搜索索引任务入队。
- 后端缺 Provider 时页面展示空态，不填假行业、假概念。

进展记录：

- 2026-06-23 已落地后端主动刷新：
  - `StockProfileRefreshRunner` 支持 `stock_profile_refresh`。
  - `DefaultJobRegistry` 已注册“股票基础资料刷新”任务类型。
  - 生产 scheduler 已挂载对应 runner。
- 2026-06-23 已通过后端自动化验证：
  - `go test -tags sqlite_fts5 ./internal/service/scheduler ./internal/dao -run 'TestStockProfileRefreshRunner|TestDefaultJobRegistryIncludesStockProfileRefresh|TestNewsRepositoryFiltersMarketNewsByMarket'`
- 2026-06-24 复核收口：`stock_profile_refresh` 已进入默认调度任务注册，详情页已通过固定 `stock_profile` command/API 读取 `stocks` 表；缓存 miss 明确返回 `stock_profile_not_found`，不伪造公司资料。
- 2026-06-24 通过自动化验证：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/service/scheduler ./internal/actions/stocks ./internal/actions/watchlist ./internal/dao -run 'TestStockProfileRefreshRunner|TestDefaultJobRegistryIncludesStockProfileRefresh|TestHandleProfile|TestHandleListIncludesStockProfileFields|TestStockRepositoryUpsertsBySymbol|TestStockSearchRepositoryListsAllStocksForRebuild'`
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "个股详情"`
  - `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts -t "stockProfile"`
- 2026-06-24 已补启动内置基础股票池 seed：
  - `stock_basic.json` 随 Go sidecar 打包，启动时 seed 5,151 条沪深 A 股基础资料；267 条北交所记录因标准 symbol 未支持而显式跳过。
  - `UpsertBuiltinStocks` 与远端 `UpsertStocks` 分离，避免远端简版搜索结果覆盖内置 `full_name`、`pinyin_initials` 等字段。
  - 已通过验证：`go test -mod=readonly ./internal/service/stockseed`、`go test -mod=readonly -tags sqlite_fts5 ./internal/service/stockseed ./internal/dao`、`go test -mod=readonly -tags sqlite_fts5 ./cmd/invest-compass-core`。

### [x] S5-00C 验收行情快照和 K 线入库闭环

依赖：S0-05。

执行动作：

- `/api/market/quote` 先读 `LatestQuote`，缓存过期后抓取 Provider 并 `SaveQuote`。
- `/api/market/kline` 先读 `ListKlines`，缓存不足后抓取 Provider 并 `SaveKlines`。
- `/api/market/indicators` 只计算指标，不保存指标结果。
- 调度 runner 写入 quote/kline 并更新 `IngestionWatermark`。

当前实现位置：

- Action：`apps/sidecar-core/internal/actions/market/market.go`
- Scheduler：`apps/sidecar-core/internal/service/scheduler/quote_runner.go`、`kline_runner.go`
- DAO：`apps/sidecar-core/internal/dao/store.go` 的 `SaveQuote`、`SaveKlines`
- Model：`Quote`、`Kline`、`IngestionWatermark`

交付物：

- 行情和 K 线缓存写入验收。
- UI 只读取真实 quote/kline，缺失时展示空态或错误态。

验收标准：

- quote 每个 symbol 保留最新快照。
- K 线按 `symbol + period + adjust + trade_date` 幂等 upsert。
- K 线缓存命中时不重复访问 Provider。
- 调度执行成功后更新水位。

验证方式：

- `go test -tags sqlite_fts5 ./internal/actions/market ./internal/service/scheduler ./internal/dao`，重点覆盖 market action、DAO repository、scheduler runner。
- 个股详情和自选股手工验收。

进展记录：

- 2026-06-23 已通过后端自动化验证：
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/actions/market ./internal/service/scheduler ./internal/dao`
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/actions -run 'TestMarketQuote|TestMarketKline|TestMarketIndicators|TestRefreshSymbolAllCreatesQuoteKlineAndNewsRuns|Test.*QuoteRefresh|Test.*KlineRefresh'`
- 2026-06-23 结论：quote 最新快照写入、K 线幂等缓存、指标按缓存计算、缓存不足回源写入、调度 quote/kline 写入和水位更新已有测试覆盖。
- 2026-06-24 已补齐前端自动化链路复核：
  - `pnpm --filter @invest-compass/frontend test -- App -t "个股详情页进入后读取真实行情、K线、指标和新闻|个股详情页切换周期会按真实周期重新读取K线和指标|顶部搜索通过后端股票搜索跳转到首个真实结果"`
  - `App.test.tsx` 已验证个股详情进入后调用 `market_quote`、`market_kline`、`market_indicators`，切换周期后按真实周期重新读取 K 线和指标。
  - `WatchlistPage.test.tsx` 已覆盖自选股列表按真实 `market_quote` 补全行情，不展示伪造自选数据。
- 2026-06-24 已复跑后端闭环：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions/market ./internal/actions/news ./internal/service/scheduler ./internal/dao`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run 'TestMarketQuote|TestMarketKline|TestMarketIndicators|TestNews|Test.*News|TestMarketNews|TestSymbolNews|TestDashboardSummaryUsesRealStoreData'`

### [x] S5-00D 明确行情历史统计是否需要新增表

依赖：S5-00C。

阻塞原因：

- 当前 `quotes` 表按 symbol 覆盖保存最新行情快照，不能用于历史分时、涨跌分布历史、轮询趋势统计。

确认方案：

- 首版自选股和总览只展示最新 quote，统计从当前 watchlist quote 计算。
- 本期不新增 `quote_snapshots` 或分时表；历史分时、刷新趋势、涨跌分布历史统计放到第二期。
- 不要把当前 `quotes` 改成无限追加表，避免破坏现有最新快照读取语义。

概念说明：

- “当前行情快照”指 `quotes` 表中每个 symbol 的最新一条行情，适合展示现价、涨跌幅、成交额等当前状态。
- “历史行情快照”指把每次轮询或刷新得到的 quote 按时间追加保存，例如 `symbol + quote_time` 一条，用于盘中变化趋势、刷新历史、涨跌分布历史等统计。
- 历史行情快照不同于 K 线：K 线是固定周期 OHLCV 聚合数据；历史 quote snapshot 是原始轮询点位。

验收标准：

- [x] UI 统计口径明确：当前快照统计。
- [x] 没有历史表前，不展示需要历史数据支撑的趋势结论。

进展记录：

- 2026-06-24 用户确认采用推荐口径：MVP 不新增行情历史快照表；总览和自选股只基于 `quotes` 最新快照做当前状态展示和当前分布统计。
- 2026-06-24 已确认当前代码不迁移 `market_statistic` 入库、今日/近 N 日趋势查询等历史统计能力；后续如要支持盘中刷新趋势，再另行设计 `quote_snapshots` 或分时表。

### [x] S5-00E 验收新闻资讯入库闭环

依赖：S0-05。

执行动作：

- `/api/news/list` 先读 `ListNewsBySymbol`，缓存不足时抓取 Provider。
- `/api/news/market` 先读 `ListMarketNews`，缓存不足时抓取 Provider。
- Provider 返回结果经 `NormalizeItems`、`Deduplicate` 后通过 `SaveNewsItems` 写入 `news_items`。
- 新闻调度 runner 写入新闻并更新 `IngestionWatermark`。

当前实现位置：

- Action：`apps/sidecar-core/internal/actions/news/news.go`
- Scheduler：`apps/sidecar-core/internal/service/scheduler/news_runner.go`
- DAO：`apps/sidecar-core/internal/dao/store.go` 的 `SaveNewsItems`
- Model：`NewsItem`、`IngestionWatermark`

交付物：

- 新闻列表和市场新闻真实缓存验收。
- 资讯中心和 Dashboard 新闻模块只读真实 `news_items`。

验收标准：

- 同一 `content_hash` 新闻幂等 upsert。
- 个股新闻按 symbols 查询。
- 新闻缓存不足时才访问 Provider。
- Provider 未配置或失败时不返回假新闻。

验证方式：

- `go test -tags sqlite_fts5 ./internal/actions/news ./internal/service/scheduler ./internal/dao`，重点覆盖 news action、DAO repository、scheduler news runner。
- 资讯中心和 Dashboard 手工验收。

进展记录：

- 2026-06-23 已通过后端自动化验证：
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/actions/news ./internal/service/scheduler ./internal/dao`
  - `GOCACHE=/private/tmp/invest-compass-go-cache go test -tags sqlite_fts5 ./internal/actions -run 'TestNews|Test.*News|TestMarketNews|TestSymbolNews|TestDashboardSummaryUsesRealStoreData'`
- 2026-06-23 结论：新闻缓存读取、Provider 回源、标准化去重、`content_hash` 幂等写入、个股新闻 symbol 查询、市场新闻缓存读取、新闻调度写入和水位更新已有测试覆盖。
- 2026-06-24 已补齐前端自动化链路复核：
  - `pnpm --filter @invest-compass/frontend test -- App -t "个股详情页进入后读取真实行情、K线、指标和新闻|个股详情页切换周期会按真实周期重新读取K线和指标|顶部搜索通过后端股票搜索跳转到首个真实结果"`
  - `NewsCenterPage.test.tsx` 已验证资讯中心只调用 `search_news` 查询资讯范围，并通过 `open_external_url` 打开真实原文链接。
  - `DashboardOverview.test.tsx` 已验证 Dashboard 只展示后端返回的 `market_news`，不展示未闭环热点入口或硬编码统计占位。
- 2026-06-24 已复跑后端闭环：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions/market ./internal/actions/news ./internal/service/scheduler ./internal/dao`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run 'TestMarketQuote|TestMarketKline|TestMarketIndicators|TestNews|Test.*News|TestMarketNews|TestSymbolNews|TestDashboardSummaryUsesRealStoreData'`

### [x] S5-00F 补齐市场新闻 market 维度

依赖：S5-00E。

原阻塞原因：

- 当前 `NewsItem` 没有 market 字段；`ListMarketNews(ctx, market, ...)` 目前忽略 market 参数，按全量新闻倒序读取。

执行方案：

- `news_items` 增加 `market` 字段。
- `/api/news/market` 和市场新闻调度写入时保存请求 market。
- `/api/news/list` 和个股新闻调度写入时保存 symbol 所属 market。
- `ListMarketNews(ctx, market, ...)` 在 market 非空时按 market 过滤；market 为空时仍用于全量摘要读取。

验收标准：

- 市场新闻筛选口径明确。
- 多市场切换时不会混入其他市场新闻。
- 旧数据没有 market 时不会被误判为指定市场新闻。

进展记录：

- 2026-06-23 已落地后端字段、DAO 和写库链路。
- 2026-06-23 已通过后端自动化验证：
  - `go test -tags sqlite_fts5 ./internal/service/scheduler ./internal/dao -run 'TestStockProfileRefreshRunner|TestDefaultJobRegistryIncludesStockProfileRefresh|TestNewsRepositoryFiltersMarketNewsByMarket'`
- 2026-06-24 复核收口：`news_items.market` 已存在；`/api/news/market` 和市场新闻调度写入时保存请求 market；`ListMarketNews(ctx, market, ...)` 在 market 非空时按 market 过滤，market 为空时用于全量摘要。
- 旧数据 market 为空的展示策略：只进入首页/摘要等全量读取，不参与指定市场筛选，避免混入其他市场新闻。
- 本期资讯中心先固定读取 `CN` 市场；多市场筛选 UI 不在本项扩展。
- 2026-06-24 通过自动化验证：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions/news ./internal/service/scheduler ./internal/dao -run 'TestMarketNews|Test.*News|TestNewsRepositoryFiltersMarketNewsByMarket|TestNewsRefreshRunner|TestDefaultJobRegistryIncludes'`
  - `pnpm --dir apps/frontend exec vitest run src/pages/news/NewsCenterPage.test.tsx`
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "资讯中心"`

### [x] S5-01 AppShell / TopBar 全局状态接入

依赖：S4-03、S5-00A、S5-00C、S5-00E。

执行动作：

- 市场状态、数据更新时间从 dashboard store 或统一 app status store 读取。
- 刷新按钮调用当前页面或全局数据刷新能力。
- 搜索保持 `stockSearch`，无结果展示错误态。
- 通知按钮接入 S4-03。

交付物：

- TopBar 真实状态。

验收标准：

- 搜索可跳转到个股详情。
- 刷新不会覆盖页面真实数据为空态。
- 通知角标可见。

验证方式：

- 前端手工验收。
- `pnpm --dir apps build`。

进展记录：

- 2026-06-23 已确认 TopBar 使用真实 `dashboardStore` / `stockSearch` / `notifications_*` command：
  - 市场状态和数据更新时间从 dashboard quote 状态推导。
  - 搜索通过 `stockSearch` 跳转到个股详情。
  - 刷新按钮调用 `dashboardStore.load()`。
  - 通知角标、通知浮层、标记已读和清理已读接入应用内通知接口。
- 2026-06-23 修复 Dashboard / TopBar 数字边界：
  - 自选股涨跌分布缺失计数字段时不再渲染 `NaN`。
  - 通知未读数异常响应时角标按 0 处理，不渲染 `NaN`。
- 2026-06-23 已通过自动化验证：
  - `pnpm --dir apps --filter @invest-compass/frontend test -- src/components/dashboard/DashboardOverview.test.tsx src/app/App.test.tsx -t 'Dashboard|TopBar|通知|搜索|刷新'`
  - `pnpm --dir apps --filter @invest-compass/frontend check`
  - `pnpm --dir apps --filter @invest-compass/frontend build`
- 2026-06-24 已复跑前端自动化验收：
  - `pnpm --filter @invest-compass/frontend test -- App -t "个股详情页进入后读取真实行情、K线、指标和新闻|个股详情页切换周期会按真实周期重新读取K线和指标|顶部搜索通过后端股票搜索跳转到首个真实结果"`
  - 实际执行覆盖全部 22 个前端测试文件、154 个用例。
  - 覆盖点：TopBar 搜索调用 `stock_search` 并跳转个股详情；刷新仍走 `dashboardStore.load()`；通知角标和浮层走 `notifications_*`；初始化完成后工作台 TopBar 不展示硬编码行情状态。
- 2026-06-25 已修复首页刷新与状态展示：
  - TopBar 交易中状态使用绿色状态背景。
  - 总览页初次加载、自动刷新和右上角刷新调用 `dashboardStore.load({ forceRefresh: true })`。
  - `market_quote` / `/api/market/quote` 新增可选 `force_refresh`，用于首页刷新绕过短缓存读取 Provider。
  - 自选股涨跌分布右侧指标列增加竖线两侧留白，避免文字贴线。

### [x] S5-02 总览页恢复真实 dashboard 数据流

依赖：S5-01。

执行动作：

- 页面 mount 调用 `dashboardSummary` 或 `dashboardStore.load()`。
- 最近报告、最近任务、市场新闻、Provider 状态来自后端。
- K 线 mini chart 使用 `marketKline`。

交付物：

- 总览页真实数据接入。

验收标准：

- 后端为空时展示空态。
- Provider 异常时展示错误态。
- 不再写入静态空状态覆盖 store。

验证方式：

- 前端手工验收。
- `pnpm --dir apps build`。

执行记录：

- 2026-06-23 已补齐 Dashboard 总览页数据源状态展示，直接使用 `dashboard_summary.provider_statuses`，空数据展示空态，不在前端构造 Provider 状态。
- 2026-06-23 已补充自动化覆盖：
  - Dashboard 页面展示后端返回的数据源状态。
  - Dashboard store 保留 `dashboard_summary` 的最近报告、最近任务、市场新闻、Provider 状态。
  - 指数迷你走势固定通过 `market_kline` 使用 `{ period: "day", adjust: "qfq", limit: 40 }` 拉取。
- 2026-06-23 已移除首页热点区未闭环的行业热点、概念热点、重点观察入口；涨跌分布卡不再展示硬编码“暂未接入”统计占位。
- 2026-06-23 已通过：
  - `pnpm --dir apps --filter @invest-compass/frontend test -- src/stores/dashboardStore.test.ts src/components/dashboard/DashboardOverview.test.tsx -t Dashboard`
- 2026-06-24 已复跑前端自动化验收：
  - `pnpm --filter @invest-compass/frontend test -- dashboardStore DashboardOverview App -t "Dashboard|首页|TopBar|刷新"`
  - 实际执行覆盖全部 22 个前端测试文件、154 个用例。
  - 覆盖点：Dashboard mount 后走 `dashboardStore.load()`；store 保留后端最近报告、最近任务、市场新闻和 Provider 状态；指数 mini chart 走固定 `market_kline`；后端为空时展示空态；Provider 异常态来自后端字段。
- 2026-06-24 已复跑后端 Dashboard summary 验收：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run 'TestDashboardSummaryReturnsInjectedData|TestDashboardSummaryReadsRealStoreData|TestDashboardSummaryIncludesUnconfiguredProviderStatus'`

### [x] S5-03 自选股列表、增删改和行情补全

依赖：S5-01。

执行动作：

- 列表调用 `watchlistList`。
- 添加调用 `watchlistCreate`。
- 编辑备注、标签、排序调用 `watchlistUpdate`。
- 删除调用 `watchlistDelete`。
- 行情和迷你走势由 `watchlistList` 返回的本地缓存 `quote`、`trend_points` 补全；批量刷新调用 `watchlistRefresh`，不在前端逐股直打远端行情接口。

交付物：

- 自选股页面真实 CRUD。

验收标准：

- 添加后刷新仍存在。
- 删除后刷新不出现。
- 行情不可用时展示错误或空值，不造假涨跌。

验证方式：

- Go watchlist 测试。
- 前端手工验收。

执行记录：

- 2026-06-23 已将自选股页面 mount 接入 `watchlist_list`，批量刷新接入 `watchlist_refresh`；行情展示以列表返回的本地缓存 `quote` 为准。
- 2026-06-23 已将新增、编辑、删除接入 `watchlist_create`、`watchlist_update`、`watchlist_delete`，新增失败时不会清空弹窗输入。
- 2026-06-23 已补充自动化覆盖：
  - 默认空态来自空后端列表，不展示伪造自选数据。
  - 新增自选股必须先 `stock_search`，再调用 `watchlist_create`，并通过重新加载 `watchlist_list` 读取缓存行情。
  - 编辑备注和标签调用 `watchlist_update`。
  - 删除调用 `watchlist_delete`。
  - 市场和标签筛选只基于已加载自选列表本地过滤，不触发未定义后端接口。
- 2026-06-23 已移除卡片视图中“筛选功能待接入”的假交互；当前市场、标签筛选使用真实列表字段，默认排序暂保留只读显示。
- 2026-06-23 已通过：
  - `pnpm --dir apps --filter @invest-compass/frontend test -- WatchlistPage.test.tsx`
- 2026-06-24 已补 `watchlist_list` 关联 `stocks` 表返回 `name`、`code`、`market`、`exchange`、`industry`、`concepts`、`list_date`、`status`、`full_name`，自选股页面重载后可显示真实股票名称和行业。
- 2026-06-24 已复跑前端自动化验收：
  - `pnpm --filter @invest-compass/frontend test -- WatchlistPage`
  - 实际执行覆盖全部 22 个前端测试文件、154 个用例。
  - 覆盖点：空态不展示伪造自选数据；新增先 `stock_search` 再 `watchlist_create`；编辑走 `watchlist_update`；删除走 `watchlist_delete`；列表和新增后均通过 `watchlist_list` 返回的缓存字段展示行情；筛选仅基于已加载真实列表字段。
- 2026-06-24 已复跑 Go 自动化验收：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions ./internal/actions/watchlist ./internal/dao -run 'TestWatchlist|Test.*Watchlist|TestDashboardSummaryReadsRealStoreData'`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions/watchlist -run TestHandleListIncludesStockProfileFields`

### [x] S5-04 自选股扩展字段确认

依赖：S5-03。

字段来源：

- `industry` 已由 `watchlist_list` 关联 `stocks` 表返回。
- `starred` 首版不新增独立业务字段；在自选股列表中表示“当前股票已加入自选”，来源即 `watchlists` 记录本身。
- `trend` 首版不单独持久化、不伪造历史曲线；自选股卡片从 `watchlist_list` 返回的真实缓存 `trend_points` 绘制，缺少真实点位时展示“无走势数据”空态。
- 统计面板从真实 `watchlist_list` 返回值和缓存行情计算，不补假字段。

实施口径：

- `industry` 从股票基础资料返回。
- 近期 K 线迷你走势只使用 Go core 本地缓存的分时 K 收盘价；缓存缺失时不额外造点、不前端直连远端 Provider。
- 自选股 AI 分析按钮不再提示待接入，改为跳转 `/analysis?symbol=...`，分析页通过真实 `stock_search` 预选股票。

验收标准：

- 所有扩展字段都有真实来源。
- 无来源字段不显示或标记待接入。
- 自选股卡片不绘制假走势。
- 自选股 AI 分析入口进入真实分析页，不保留假按钮。

进展记录：

- 2026-06-24：`industry`、股票名称、代码、市场、交易所、概念、上市日期、状态、公司全称已由 Go `watchlist_list` 通过 `stocks` 表补充返回；前端自选股列表优先使用这些真实字段。
- 2026-06-24：按首版推荐口径确认不新增 `starred` 字段、不伪造 `trend`；有本地分时 K 缓存时通过 `watchlist_list` 返回 `trend_points`，无真实走势点位时显示空态。
- 2026-06-24：已清理自选股 AI 分析待接入提示，卡片和表格的 AI 分析入口跳转真实分析页，分析页按 query symbol 调用 `stock_search` 预选股票。
- 2026-06-24 验证：
  - `pnpm --dir apps/frontend exec vitest run src/components/watchlist/WatchlistPage.test.tsx`
  - `pnpm --dir apps/frontend check`

### [x] S5-05 个股详情行情、K 线、指标、新闻接入

依赖：S5-01。

执行动作：

- [x] 进入页面调用 `marketQuote(symbol)`。
- [x] 公司资料调用 `stockProfile(symbol)`。
- [x] 图表调用 `marketKline({ symbol, period, adjust, limit })`。
- [x] 指标调用 `marketIndicators`。
- [x] 相关新闻调用 `newsList`。
- [x] 配置默认周期和复权读取基础设置。

交付物：

- [x] 个股详情真实数据。
- [x] 清理详情页标签备注固定假数据，避免未接后端字段伪装真实能力。

验收标准：

- [x] 无 symbol 或查无股票时展示错误态。
- [x] 切换周期会重新拉取 K 线。
- [x] 新闻为空时展示空态。

验证方式：

- [x] `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "个股详情页" --reporter=basic`
- [x] `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx --reporter=basic`
- [x] `pnpm --dir apps/frontend check`
- [x] `pnpm --dir apps/frontend build`
- [x] `node apps/desktop/test/security-config.test.mjs`
- [x] `git diff --check`
- [x] 前端自动化验收。

进展记录：

- 2026-06-23：详情页已接入 `settings_get`、`market_quote`、`market_kline`、`market_indicators`、`news_list`；`minute` 默认周期因 Go Provider 暂不支持，在详情页按 `day` 加载，不修改用户设置。公司资料、行业、概念、标签备注仍按 S5-06/S5-04 阻塞项处理，不在本任务中擅自新增接口或字段。
- 2026-06-24：详情页已新增固定 Rust command `stock_profile` 和 Go `/api/stocks/profile`，从 `stocks` 表读取公司名称、行业、概念、上市日期、状态、公司全称；页面标题、行业、概念和基础信息已改用真实资料字段。
- 2026-06-24 已复跑详情页自动化验收：
  - `pnpm --filter @invest-compass/frontend test -- App -t "个股详情页进入后读取真实行情、K线、指标和新闻|个股详情页切换周期会按真实周期重新读取K线和指标|顶部搜索通过后端股票搜索跳转到首个真实结果"`
  - 覆盖点：详情页进入后调用 `settings_get`、`market_quote`、`stock_profile`、`market_kline`、`market_indicators`、`news_list`；切换周期会按真实周期重新拉取 K 线和指标；新闻为空展示空态；页面不暴露买卖、下单、券商账户等未闭环能力。
- 2026-06-24 已复跑安全白名单：
  - `node apps/desktop/test/security-config.test.mjs`
- 2026-06-24：个股详情剩余假入口已收口。顶部“刷新行情”会重新读取 `market_quote`、`stock_profile`、`market_kline`、`market_indicators`、`news_list` 和 `watchlist_list`；“发起 AI 分析”和研究快捷入口跳转 `/analysis?symbol=...`，技术面入口附带 `analysisType=technical`；新闻“查看更多”跳转资讯中心；未实现的 K 线工具按钮、AI 摘要 Tab、历史报告 Tab 已隐藏，不再保留待接入假按钮。
- 2026-06-24 验证：
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "个股详情页" --reporter=basic`
  - `pnpm --dir apps/frontend check`
  - `git diff --check`

### [x] S5-06 个股详情公司资料和标签接口确认

依赖：S5-05。

实现口径：

- 公司资料、行业、概念接口已补齐。
- 页面级用户标签与备注首版复用 `watchlists.tags` / `watchlists.note`，不新增独立 stock note schema。

数据来源：

- 已新增 `stockProfile({ symbol })` 返回公司资料、行业、概念。
- 详情页读取 `watchlist_list`，按股票代码匹配当前详情页股票。
- 当前股票已加入自选时，展示并允许编辑 `tags/note`，保存调用 `watchlist_update`。
- 当前股票未加入自选时，仅展示空态和“加入自选后可编辑”，不提供假保存入口。

验收标准：

- [x] 公司资料字段有稳定来源。
- [x] 用户备注保存后刷新可回显。
- [x] 未加入自选的股票不显示假编辑入口。

进展记录：

- 2026-06-24：完成 `stock_profile` typed service、Rust 白名单 command、Go `/api/stocks/profile`、DAO `GetStockBySymbol`，详情页已消费 profile 字段。
- 2026-06-24：详情页“我的标签与备注”已复用自选股记录；读取 `watchlist_list` 后匹配当前股票，编辑保存调用 `watchlist_update` 并回显。
- 2026-06-24 验证：
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "个股详情页" --reporter=basic`
  - `pnpm --dir apps/frontend check`

### [x] S5-07 AI 分析页接入模型、Prompt 和任务创建

依赖：S2-01、S2-04、S5-05。

执行动作：

- 模型下拉读取 `aiConfigList`。
- Prompt 模板读取 `promptTemplatesList`。
- 股票上下文来自行情、K 线、新闻真实数据。
- 生成按钮调用 `analysisTaskCreate`。

交付物：

- AI 分析创建链路。

验收标准：

- 未配置模型时不能创建任务。
- 缺行情上下文时给出明确错误。
- 创建成功跳转运行页。

验证方式：

- analysis task 测试。
- 前端手工验收。

进展记录：

- 2026-06-24 已完成前端真实创建链路：
  - 模型下拉读取 `ai_config_list`，仅已配置 `api_key_ref` 的模型可用于创建任务。
  - Prompt 模板读取 `prompt_templates_list`，首版创建链路只开放 `stock_full` / `technical` 类型。
  - 股票输入通过 `stock_search` 查询，选中后读取 `stock_profile`、`market_quote`、`market_kline`、`market_indicators`、`news_list` 构建上下文预览。
  - 开始分析调用 `analysis_task_create`，成功后跳转 `/analysis/running?taskId=...`。
  - 未配置模型时“开始分析”禁用，不会触发任务创建。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend test -- App -t "AI 分析页"`
  - `pnpm --filter @invest-compass/frontend check`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions ./internal/actions/analysis ./internal/service/analysis -run 'TestAnalysisTask|Test.*Analysis|TestCreate|TestValidate'`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::tasks::tests -- --test-threads=1`
- 2026-06-24：AI 分析页输出预览全屏按钮已改为真实弹窗，弹窗内容区独立滚动；未生成输出前，“保存报告”“复制 Markdown”“导出 Markdown”禁用，不再弹出待接入提示。导出仍以生成后的报告详情页 `reportExport` 为真实保存链路。
- 2026-06-24 补充验证：
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "AI 分析页读取真实模型" --reporter=basic`
  - `pnpm --dir apps/frontend check`

### [x] S5-08 AI 分析运行页接入任务事件和取消

依赖：S5-07。

执行动作：

- [x] 运行页订阅 `analysisTaskSubscribe`。
- [x] 运行页通过 `taskEvents` 恢复历史事件。
- [x] 支持 `TASK_PROGRESS`、`TASK_LOG`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`、`TASK_CANCELLED`。
- [x] 取消按钮调用 `analysisTaskCancel`。
- [x] 成功后读取 `task_get.report_id`，有真实报告 ID 时显示“查看报告”并跳转报告详情。

交付物：

- [x] 任务运行实时状态。

验收标准：

- [x] 断线后可从 `taskEvents` 恢复。
- [x] 取消后任务状态正确。
- [x] 成功后可跳转报告详情：运行页复用 S5-15 已补齐的 `task_get.report_id`，不新增任务表字段或额外查询接口。

验证方式：

- `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "AI 分析运行页恢复任务事件、订阅增量事件并支持取消|AI 分析页读取真实模型、Prompt 和股票上下文后创建分析任务"`。
- `pnpm --filter @invest-compass/frontend exec vitest run src/services/coreClient.test.ts`。
- `pnpm --filter @invest-compass/frontend check`。
- 前端手工验收。

进展记录：

- 2026-06-24 已将运行页从本地空态改为真实任务事件链路：
  - 页面读取 URL `taskId` 后调用 `task_get` 建立任务摘要。
  - 页面先用 `task_events(taskId, 0)` 恢复历史事件，再调用 `analysis_task_subscribe(taskId, lastEventId)` 订阅增量事件。
  - 页面监听 Rust 转发的 `analysis-task-event`，将事件映射为任务步骤、任务日志和流式 Markdown 输出。
  - “停止生成”调用 `analysis_task_cancel`，成功后本地状态切换为 `CANCELLED`。
  - `task_events` typed service 已兼容 Go API 真实返回的 `event_type + payload` 字符串，并归一化为前端消费的 `event + data`。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "AI 分析运行页恢复任务事件、订阅增量事件并支持取消|AI 分析页读取真实模型、Prompt 和股票上下文后创建分析任务"`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/services/coreClient.test.ts`
  - `pnpm --filter @invest-compass/frontend check`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx`
- 2026-06-24：S5-15 已为 `task_get/task_list` 返回 `report_id`。运行页现在在终态任务恢复时读取 `task_get.report_id`，显示真实“查看报告”按钮并跳转 `/reports/:reportId`；实时成功事件如果带 `report_id` 也会同步更新入口。
- 2026-06-24 补充验证：
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "AI 分析运行页成功任务可跳转已生成报告" --reporter=basic`

### [~] S5-09 报告历史默认列表、筛选和删除接入

依赖：S5-08。

执行动作：

- [x] 页面加载调用 `reportList`。
- [x] 筛选条件基于 `reportList` 返回的真实列表做本地二次筛选；后端当前未提供列表筛选参数，未新增接口。
- [x] 搜索继续使用固定 `searchReports` 报告范围命令。
- [x] 删除调用 `reportDelete`，删除后从当前列表移除，重置/刷新会重新读取真实列表。
- [x] 右侧统计、常用模型和分析类型分布由 S5-10 `reportStats` 接口返回，不再使用静态 156 或静态统计数组。
- [x] 批量删除由 S5-10 `reportBatchDelete(ids)` 接入真实后端。
- [x] 收藏由 S5-10 `reportUpdate(id, favorite)` 接入真实后端。
- [x] 单条导出由 S5-10 `reportExport(id)` 接入 Rust 保存对话框。
- [!] 批量导出保存策略放到第二期，入口继续禁用，不伪装成功。

交付物：

- 报告历史真实列表。

验收标准：

- [x] 删除后刷新不出现：后端 `reportDelete` 已软删除，页面重置/刷新走 `reportList`。
- [x] 筛选条件后端不支持时未新增后端参数，当前对真实返回列表做本地筛选。
- [x] 统计不再使用静态数据。
- [ ] 桌面端手工验收：真实报告列表、删除后刷新、关键词搜索和统计展示。

验证方式：

- report service 测试。
- 前端手工验收。

进展记录：

- 2026-06-24：报告历史页已接入 `reportList` 默认列表、`searchReports` 报告范围搜索和 `reportDelete` 删除；表格总数、分页总数来自当前真实列表。
- 2026-06-24：右侧报告统计、常用模型 TOP 5、分析类型分布已改为读取 S5-10 `reportStats`；批量删除已改为调用 `reportBatchDelete(ids)`。
- 2026-06-24：用户确认新增收藏字段和导出默认选择保存位置后，报告历史页已接入 `reportUpdate(id, favorite)` 和 `reportExport(id)`；批量导出仍禁用，等待第二期多文件保存策略。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend exec vitest run src/pages/reports/ReportHistoryPage.test.tsx`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "分析报告历史页面展示空态并按报告范围搜索"`

### [~] S5-10 报告收藏、批量、导出、统计接口确认

依赖：S5-09。

执行动作：

- [x] 新增 Go `/api/reports/stats`，只从可见报告元数据聚合统计，不返回正文或输入快照。
- [x] 新增 Go `/api/reports/batch-delete`，批量软删除报告。
- [x] 新增 Rust `report_stats` 和 `report_batch_delete` 白名单 command。
- [x] 新增前端 `reportStats` 和 `reportBatchDelete` typed service。
- [x] 报告历史页右侧统计读取 `reportStats`。
- [x] 报告历史页批量删除选中报告调用 `reportBatchDelete(ids)`。
- [x] 新增 `analysis_reports.favorite` 字段，报告收藏状态随列表和详情返回。
- [x] 新增 Go `/api/reports/update`，仅允许更新收藏状态，不通过该接口修改正文或输入快照。
- [x] 新增 Go `/api/reports/export`，返回安全建议文件名和默认不含完整 `input_snapshot` 的 Markdown 内容。
- [x] 新增 Rust `report_update` 和 `report_export` 白名单 command；`report_export` 通过系统保存对话框选择写入位置，前端不传任意路径。
- [x] 新增前端 `reportUpdate` 和 `reportExport` typed service，报告历史和详情页单条收藏/导出均已接入。
- [!] 批量导出继续禁用，第二期确认多报告保存为目录、压缩包或逐个保存后再接入。

推荐方案：

- 收藏新增 `reportUpdate(id, favorite)`，当前采用 `analysis_reports.favorite` 字段。
- 批量删除已新增 `reportBatchDelete(ids)`。
- 单条导出由 Rust 固定 command 处理文件选择和写入，前端不传保存路径。
- 统计已由 `reportStats` 返回，前端不再伪造周环比、成功率等无来源数据。

验收标准：

- [x] 每个按钮都有真实能力或禁用态。
- [x] 统计响应不包含完整敏感输入快照。
- [x] 批量删除不覆盖报告正文或任务事件，只做报告软删除。
- [x] 导出文件不包含完整敏感输入快照。
- [ ] 桌面端手工验收：统计、批量删除、收藏、单条导出保存位置选择。

进展记录：

- 2026-06-24：`reportStats` 返回报告总数、覆盖股票数、最新报告时间、分析类型分布和常用模型 TOP；Go action 只使用元数据字段，测试覆盖不泄露正文和输入快照。
- 2026-06-24：`reportBatchDelete(ids)` 已接通 Go/Rust/前端，批量删除前会校验 ID，前端删除后刷新统计并清空选中项。
- 2026-06-24：收藏已接通 Go `/api/reports/update`、Rust `report_update`、前端 `reportUpdate` 和 `analysis_reports.favorite` 字段；报告 upsert 不覆盖用户收藏状态。
- 2026-06-24：单条导出已接通 Go `/api/reports/export`、Rust `report_export` 和前端 `reportExport`；导出内容默认不含完整 `input_snapshot`，保存位置由系统保存对话框选择。批量导出仍禁用，放到第二期。
- 2026-06-24 已通过自动化验证：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run "TestReportsAPIListsGetsAndSoftDeletes|TestReportsAPIStatsAndBatchDelete"`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run 'TestReportsAPI(UpdatesFavorite|ExportsMarkdownWithoutInputSnapshot)' -count=1`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/dao -run 'Test(MigrateCreatesInitialSchema|ReportRepositoryUpsertsByTaskID)' -count=1`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::reports::tests -- --test-threads=1`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/pages/reports/ReportHistoryPage.test.tsx`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/services/coreClient.test.ts -t "任务和报告方法"`
  - `pnpm --filter @invest-compass/frontend check`
  - `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts src/pages/reports/ReportHistoryPage.test.tsx src/app/App.test.tsx`

### [~] S5-11 报告详情动作接入

依赖：S5-09。

执行动作：

- [x] 详情读取 `reportGet`。
- [x] 删除调用 `reportDelete`，删除成功后返回报告列表。
- [x] 重新分析调用 `analysisTaskCreate`，使用报告源 symbol、报告 analysis type、报告 `prompt_template_id` 和当前默认可用 AI 配置创建新任务。
- [x] 导出按 S5-10 方案实现：调用 `reportExport(id)` 并由系统保存对话框选择位置。
- [x] 收藏按 S5-10 方案实现：调用 `reportUpdate(id, favorite)`。

交付物：

- 报告详情真实动作。

验收标准：

- [x] 报告不存在展示 404/空态。
- [x] 重新分析创建新任务，不覆盖旧报告。
- [x] 删除后返回报告列表。
- [ ] 桌面端手工验收：详情页删除、重新分析跳转运行页、收藏和单条导出保存位置选择。

验证方式：

- 前端手工验收。

进展记录：

- 2026-06-24：报告详情页已接入真实 `reportGet`、`reportDelete` 和 `analysisTaskCreate`。重新分析不复用不可见的旧 API Key 明文；页面读取当前默认可用 AI 配置，只向任务创建传递 `api_key_ref`。当前 `reportGet` 未返回旧报告的原始 `ai_config_id`，因此重新分析不强行猜测旧模型配置。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "分析报告详情页面重新分析使用报告来源创建新任务|分析报告详情页面删除报告后返回真实报告列表"`

### [~] S5-12 资讯中心默认列表和筛选接入

依赖：S5-01。

执行动作：

- [x] 默认列表调用 `newsMarket` 或 `newsList`。
- [x] 关键词搜索调用 `searchNews`。
- [x] 打开原文继续调用 `openExternalURL`。
- [x] 加入 AI 上下文如无后端支持先禁用。

交付物：

- [x] 资讯中心真实新闻列表。

验收标准：

- [x] 新闻为空时展示空态。
- [x] 外链通过系统浏览器打开。
- [x] 不展示假热点和假统计。
- [ ] 桌面端手工验收：默认市场资讯列表、关键词搜索、打开原文、空态和待接入提示。

验证方式：

- news service 测试。
- 前端手工验收。

进展记录：

- 2026-06-24：资讯中心默认列表已接入 `newsMarket({ market: "CN", limit: 20 })`，关键词搜索继续调用 `searchNews`，打开原文继续调用 `openExternalURL`。筛选条件当前只对已加载真实列表做本地二次筛选，不新增后端未支持参数。
- 2026-06-24：`加入 AI 上下文` 已隐藏并放入第二期；资讯侧栏热点和统计改为读取本地新闻缓存，不再展示固定更新时间或虚构缓存容量。资讯缓存清理入口保持禁用，等待后续明确缓存命令后再开放。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "资讯中心页面加载真实市场新闻并按资讯范围搜索"`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx src/services/coreClient.test.ts`
  - `pnpm --filter @invest-compass/frontend check`

### [~] S5-13 资讯侧栏统计、热点和加入上下文接口确认

依赖：S5-12。

执行动作：

- [x] 统计由 `newsStats` 返回。
- [x] 热点由 `newsHotTopics` 返回。
- [x] 资讯侧栏统计和热点只读取本地 `news_items` 缓存，不触发 Provider 回源。
- [x] 未接入情绪分类前，不展示伪造利好/利空比例。
- [x] 加入上下文入口本期隐藏，放到第二期开发。

交付物：

- [x] Go core `/api/news/stats` 和 `/api/news/hot-topics`。
- [x] Rust `news_stats` 和 `news_hot_topics` 白名单 command。
- [x] 前端 `newsStats`、`newsHotTopics` typed service 和资讯侧栏真实展示。

验收标准：

- [x] 统计口径可解释。
- [x] 上下文内容不包含外部网页未授权全文。
- [x] 不展示假热点、固定更新时间、虚构缓存容量或未闭环加入上下文按钮。
- [ ] 桌面端手工验收：资讯侧栏热点标签、提及股票、统计说明和空态。

进展记录：

- 2026-06-24：`newsStats` 返回缓存新闻总数、来源数、最新发布时间和“情绪分类未接入”的说明；`newsHotTopics` 基于缓存新闻 tags 和 symbols 汇总热点标签与高频提及股票。前端将 count 归一化为热度条，仅用于展示相对强弱，不代表外部热度排行。
- 2026-06-24：`加入上下文` 按确认隐藏，放到第二期；资讯侧栏不再显示 `15:30 更新` 和 `缓存总量：312 MB`。
- 2026-06-24 已通过自动化验证：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions -run "TestNewsStatsAndHotTopicsUseCachedNews|TestNewsMarketReturnsSortedCachedItems"`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::news::tests -- --test-threads=1`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "资讯中心页面加载真实市场新闻并按资讯范围搜索"`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/services/coreClient.test.ts -t "新闻统计"`

### [~] S5-14 任务历史主列表和事件详情接入

依赖：S5-08。

执行动作：

- [x] 页面加载调用 `taskList`。
- [x] 详情调用 `taskGet` 和 `taskEvents`。
- [x] 任务日志抽屉继续调用 `taskLogs*`。
- [x] 取消调用 `analysisTaskCancel` 或 `task cancel` 固定接口。

交付物：

- [x] 任务历史真实列表和详情。

验收标准：

- [x] 任务状态刷新后不丢。
- [x] 失败任务展示错误摘要。
- [x] 任务事件可回放。
- [x] 成功任务跳转报告、失败任务重试归属 S5-15；S5-14 只保留列表、详情和取消主链路。
- [ ] 桌面端手工验收：真实任务列表、详情事件、日志抽屉、取消运行中任务。

验证方式：

- task service 测试。
- 前端手工验收。

进展记录：

- 2026-06-24：任务历史页已从本地空数组改为默认调用 `taskList(100)`；页面将 Go/Rust 返回的 `TaskItem` 归一化为 UI 任务类型、状态、进度、开始/结束时间和耗时。
- 2026-06-24：点击任务行会调用 `taskGet(taskId)` 和 `taskEvents(taskId, 0)`，详情抽屉展示真实事件流；取消运行中任务调用 `analysisTaskCancel(taskId)`，成功后本地任务状态切换为 `CANCELLED`。
- 2026-06-24：旧详情面板不再硬编码 `2025-05-20`，开始时间直接展示真实任务时间字段。
- 2026-06-24：修复搜索索引重建任务被前端兜底标记为 `AI 分析` 的问题；`SEARCH_REBUILD` / `search-rebuild-*` 归类为 `数据重建`，此类非报告任务不显示报告入口。
- 2026-06-24 已通过自动化验证：
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "任务历史页面读取真实任务列表、事件详情并支持取消"`
  - `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "任务历史搜索索引重建任务展示为数据重建且不显示报告入口"`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx src/services/coreClient.test.ts`
  - `pnpm --filter @invest-compass/frontend check`

### [~] S5-15 任务重试和报告跳转规则确认

依赖：S5-14。

执行动作：

- [x] 任务列表和详情返回 `report_id`，来源为已保存报告的 `task_id -> report_id` 反查，不新增任务表字段。
- [x] 成功 AI 分析任务点击“报告”时跳转 `/reports/:reportId`。
- [x] 失败 AI 分析任务点击“重试”时读取原任务 `TASK_CREATED` 事件，并创建新任务。
- [x] 重试任务通过 `retry_of_task_id` 引用原任务，不覆盖原任务事件。
- [x] 原任务包含一次性持仓输入时拒绝自动重试，提示用户重新创建分析任务。
- [x] 前端重试只从 AI 配置列表读取 `api_key_ref`，不从任务事件或日志恢复明文凭据。
- [x] Rust command 转发 `retry_of_task_id`，并继续阻止 `api_key_ref` 进入 Go core 请求体。

交付物：

- [x] Go `task_list/task_get` 增加 `report_id` 响应字段。
- [x] Go `analysis_task_create` 接收并记录 `retry_of_task_id`。
- [x] Rust `analysis_task_create` 白名单 payload 支持 `retry_of_task_id`。
- [x] 前端任务历史“报告”和“重试”按钮接入真实能力。

验收标准：

- [x] 重试不覆盖原任务事件。
- [x] 成功任务能稳定跳转报告。
- [x] 不从历史任务恢复一次性持仓明细或明文 API Key。
- [ ] 桌面端手工验收：成功任务报告跳转、失败任务重试创建新任务、含持仓任务重试拒绝提示。

进展记录：

- 2026-06-24：S5-15 按确认方案实现。报告跳转不改 DB schema，直接使用报告表中已有 `task_id` 关系反查 `report_id`；任务重试生成新任务，并在 `TASK_CREATED` 事件安全 payload 中记录 `retry_of_task_id`。
- 2026-06-24：原任务如果含一次性持仓输入，任务历史页不会从事件中恢复持仓详情，改为提示用户重新创建分析任务，避免持仓输入被历史事件反推。
- 2026-06-24 已通过自动化验证：
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/actions ./internal/service/analysis -run "TestTasksAPIListsGetsAndReplaysEvents|TestCreateTaskBuildsPendingTaskAndSafeCreatedEvent"`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::tasks::tests -- --test-threads=1`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/app/App.test.tsx -t "任务历史成功分析任务可跳转已生成报告|任务历史失败分析任务可按原创建事件生成重试任务|任务历史页面读取真实任务列表、事件详情并支持取消"`
  - `pnpm --filter @invest-compass/frontend exec vitest run src/services/coreClient.test.ts -t "分析任务"`
  - `pnpm --filter @invest-compass/frontend check`

---

## 8. RG-S6：关于、日志、更新和发布验收

### [x] S6-01 关于页检查更新接入

依赖：无。

执行动作：

- 检查更新按钮调用 `checkUpdate`。
- 不实现真实下载安装。
- 返回当前版本、最新版本、是否有更新、说明摘要。

交付物：

- 关于页检查更新真实结果。
- 发布说明只展示可用性，不打开外链或本地文件；打开方式仍归 S6-03。

验收标准：

- 网络或后端失败时展示错误。
- 不出现授权激活或自动安装入口。

验证方式：

- update service 测试或手工验收。

进展记录：

- 2026-06-24：S6-01 已接入。关于页检查更新按钮调用真实 `check_update` 白名单 command，并展示当前版本、最新版本、`发现新版本` / `当前已是最新版本` / `检查更新失败` 状态，以及发布说明可用性；不提供下载、安装或授权激活入口。
- 2026-06-24：发布说明打开仍保留为 S6-03 决策项，本次不直接打开外链或本地文件。
- 2026-06-24 已通过自动化验证：
  - `pnpm --dir apps/frontend exec vitest run src/pages/settings/about/AboutAppPage.test.tsx src/services/coreClient.test.ts src/app/App.test.tsx`
  - `pnpm --filter @invest-compass/frontend check`

### [x] S6-02 日志导出接入并二次脱敏

依赖：S3-03、S4-02。

执行动作：

- 日志导出按钮调用 `exportLogs`。
- 导出前覆盖 API Key、Cookie、Authorization、Proxy-Authorization、代理密码脱敏。
- 导出位置默认工作区或用户选择目录。

交付物：

- 日志导出真实能力。

验收标准：

- 导出日志不含敏感明文。
- 导出失败展示错误。

验证方式：

- 日志脱敏测试。
- 手工导出检查。

进展记录：

- 2026-06-24：S6-02 已接入。关于页日志导出按钮先打开系统目录选择器；用户取消时不调用后端，选择目录后调用固定 `export_logs` command，由 Rust 请求 Go `/api/logs/export` 获取已脱敏日志包并写入用户授权目录。
- 2026-06-24：保留现有 Go `logexport` 二次脱敏规则和 Rust 目录/文件名校验边界；前端不传文件名，不直接构造日志内容。
- 2026-06-24 已通过自动化验证：
  - `pnpm --dir apps/frontend exec vitest run src/pages/settings/about/AboutAppPage.test.tsx src/services/coreClient.test.ts src/app/App.test.tsx`
  - `pnpm --filter @invest-compass/frontend check`
  - `go test -mod=readonly -tags sqlite_fts5 ./internal/service/logexport ./internal/actions/logexport`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::logs::tests -- --test-threads=1`

### [x] S6-03 LICENSE、用户手册、发布说明打开方式确认

依赖：无。

确认方案：

- 暂时使用内置静态页面，不打开外链，不读取本地打包文件。
- LICENSE 展示 GPL v3.0 摘要、无担保声明和第三方组件说明。
- 用户手册展示首版使用流程、风险边界和排障建议。
- 发布说明展示 v0.1.0 内测能力、已知边界和合规提示。

验收标准：

- 不直接在前端 `window.open` 外链。
- 不调用 `openExternalURL` 或其他外部打开命令。
- 静态页面在弹窗内滚动展示，关闭后不影响关于页状态。

进展记录：

- 2026-06-24：S6-03 已按确认方案实现为内置静态页面。关于页 `查看 LICENSE`、`打开用户手册`、`查看发布说明` 均打开应用内弹窗，不访问外链，不增加 Rust/Go command。
- 2026-06-24 已通过自动化验证：
  - `pnpm --dir apps/frontend exec vitest run src/pages/settings/about/AboutAppPage.test.tsx src/services/coreClient.test.ts src/app/App.test.tsx`
  - `pnpm --filter @invest-compass/frontend check`

### [x] S6-04 全量 secret 和禁用词检查

依赖：S2、S3、S4、S5。

执行动作：

- 搜索真实或示例 API Key、Cookie、Token、Authorization、Proxy-Authorization。
- 搜索交易、下单、券商账户、收益承诺等未闭环能力入口。
- 修复或删除违规文案和示例。

交付物：

- secret/禁用词检查记录。

验收标准：

- 仓库不含真实密钥。
- UI 不含未闭环交易能力入口。

验证方式：

- `rg` 检查。
- 人工 review。

进展记录：

- 2026-06-24：已修正关于页内置静态文档中的首版禁用入口字面量，避免前端生产源码出现容易被误判为交易闭环入口的文案。
- 2026-06-24：生产源码禁用词扫描仅剩 `apps/sidecar-core/internal/service/prompt/builder.go` 中的合规约束文案，用于禁止模型输出诱导表达，不是用户可点击入口。
- 2026-06-24：secret 扫描命中项已人工分类，剩余命中均为凭据处理字段、脱敏正则、真实请求头设置点或 Rust 内联测试夹具；未发现真实密钥、真实 Cookie、真实 Token 或真实代理密码落入仓库。
- 2026-06-24 已通过自动化验证：
  - `rg -n --glob '!**/*.test.*' --glob '!**/*_test.go' --glob '!**/test/**' --glob '!**/*.md' "(下单|自动下单|实盘|交易托管|券商账户|稳赚|必涨|收益承诺|保证收益|买入信号|卖出信号|立即买入|立即卖出|目标价|满仓|清仓|加仓|减仓|推荐买入|推荐卖出|荐股)" apps/frontend/src apps/desktop/src-tauri/src apps/sidecar-core/internal apps/sidecar-core/pkg`
  - `rg -n --glob '!**/*.test.*' --glob '!**/*_test.go' --glob '!**/test/**' --glob '!**/*.md' "(sk-[A-Za-z0-9_\\-]{8,}|Authorization|Proxy-Authorization|api[_-]?key\\s*[:=]\\s*[\\\"']?[^\\s\\\"']+|proxy_password\\s*[:=]|Cookie:\\s*|token=|Bearer\\s+[A-Za-z0-9._\\-]{8,}|secret\\s*[:=])" apps/frontend/src apps/desktop/src-tauri/src apps/sidecar-core/internal apps/sidecar-core/pkg`
  - `node --test apps/desktop/test/security-config.test.mjs --test-name-pattern "前端生产源码禁止首版未闭环入口文案|前端和共享契约禁止暴露 Go core、runtime token 或内部密钥字段"`
  - `pnpm --dir apps/frontend exec vitest run src/pages/settings/about/AboutAppPage.test.tsx src/services/coreClient.test.ts src/app/App.test.tsx`

### [x] S6-05 按变更范围执行最终验证

依赖：全部开发任务。

执行动作：

- docs-only：执行 `git diff --check`。
- 前端变更：执行 `pnpm --dir apps build` 和 `pnpm --dir apps test`。
- Go 变更：在 Go module 执行 `go test ./...`。
- Rust/Tauri 变更：执行对应 `cargo check` 或 `cargo test`。

交付物：

- 验证命令和结果记录。

验收标准：

- 所有必需验证通过。
- 无法执行的验证说明原因、影响范围和剩余风险。

进展记录：

- 2026-06-24：首次在沙箱内执行 `pnpm --dir apps build` 时被 Go build cache 写入 `~/Library/Caches/go-build` 权限拦截；使用升级权限重跑同一命令后通过。
- 2026-06-24：`pnpm --dir apps test` 首轮发现 Go 测试辅助函数缺少中文注释，已补齐 Prompt、AI、分析执行器和代理测试中的辅助函数注释并重新格式化。
- 2026-06-24：`pnpm --dir apps test` 后续发现资讯中心单测仍按旧假设只允许 `search_news`，已更新测试夹具，先接受页面默认读取 `news_market` / `news_stats` / `news_hot_topics`，再验证关键词刷新调用 `search_news`。
- 2026-06-24：`pnpm --dir apps test` 后续发现调度设计文档默认任务示例漏掉 `stock_profile_refresh`，已同步 `docs/2026-06-19-invest-compass-scheduler-design.md`。
- 2026-06-24 已通过自动化验证：
  - `pnpm --dir apps build`
  - `pnpm --dir apps test`
  - `cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml`
  - `git diff --check`

---

## 9. 优先推进顺序

推荐第一批：

1. S0-01、S0-03、S0-04、S0-05。
2. S1-01、S1-05、S1-06。
3. S2-01、S2-02、S2-04、S2-05。

推荐第二批：

1. S5-00A、S5-00C、S5-00E。
2. S5-01、S5-02、S5-03、S5-05。
3. S5-09、S5-12、S5-14。
4. S4-01。

推荐第三批：

1. S3-02、S3-03、S3-04、S3-05。
2. S4-02、S4-03、S4-04、S4-05。
3. S1-03。

推荐第四批：

1. S5-00B、S5-00D、S5-00F。
2. S5-04、S5-06、S5-10、S5-13、S5-15。
3. S6-01、S6-02、S6-03、S6-04、S6-05。

需要优先确认的阻塞项：

- S2-07：代理连接测试接口。
- S3-02：数据源凭据加密 SQLite schema 和密钥管理。
- S4-02：应用内通知表和 API。
- S5-00B：股票基础资料主动刷新或种子入库流程。
- S5-00D：行情历史统计是否需要新增历史快照表。
- S5-00F：市场新闻是否需要新增 market 字段。
- S5-04：自选股扩展字段来源。
- S5-06：个股详情公司资料和用户备注接口。
- S5-10：报告收藏、批量、导出、统计接口。
- S6-03：LICENSE、用户手册、发布说明打开方式。
