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

### [ ] S0-01 建立 UI 到后端对接矩阵的执行基线

依赖：两份盘点文档。

执行动作：

- 以 UI 路由为维度列出页面、按钮、展示字段、数据来源、保存方式。
- 以后端能力为维度列出 typed service、Rust command、Go API、SQLite 表。
- 给每个缺口分配本清单中的任务编号。

交付物：

- 本清单保持为唯一推进入口。
- 两份盘点文档只保留方案明细和上下文，不直接当迭代板。

验收标准：

- 任意 UI 功能都能追溯到任务编号或已实现接口。
- 任意缺失接口都能追溯到对应页面和验收数据。

验证方式：

- 人工 review 本文档和两份盘点文档链接是否一致。

### [ ] S0-02 清理或冻结隐藏旧页面入口

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

### [ ] S0-04 定义 UI 禁用、待接入和后端不支持的统一呈现方式

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
- 调度：`QuoteRefreshRunner`、`KlineRefreshRunner`、`NewsRefreshRunner` 均包含写库和 `IngestionWatermark` 更新逻辑。

仍需补齐：

- 股票基础资料的主动刷新或全量种子入库流程。
- 行情快照只保留每个 symbol 最新一条，不满足历史行情统计。
- 市场新闻缓存当前不单独维护 market 字段，`ListMarketNews` 读取的是全量新闻倒序。
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

### [~] S2-03 模型连通性测试接入真实测试接口

依赖：S2-02。

执行动作：

- [x] 测试按钮调用 `aiConfigTest`。
- [x] 测试期间展示 `测试中` 状态。
- [~] 测试结果只展示状态，不展示密钥；耗时和错误摘要展示位置待确认。

交付物：

- [x] 模型测试交互。

验收标准：

- [x] 成功和失败状态都能展示。
- [x] 前端测试调用只传 `id` 与 `api_key_ref`，不传真实 API Key。
- [x] 上游错误提示进入页面前会做敏感字段脱敏。
- [ ] API Key 不进入日志、任务事件或前端类型，需配合端到端日志抽样确认。
- [ ] `AIConfigTestResult` 暂无耗时字段；如需页面展示耗时，需要确认是前端本地计时还是扩展 Rust/Go 返回字段。

验证方式：

- `pnpm --dir apps/frontend exec vitest run src/pages/settings/model-config/ModelConfigPage.test.tsx`。
- `pnpm --dir apps/frontend exec vitest run src/services/coreClient.test.ts src/app/App.test.tsx`。
- `cd apps/sidecar-core && go test -mod=readonly -tags sqlite_fts5 ./internal/service/ai ./internal/actions/aiconfig`。
- `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml commands::ai_config::tests`。
- [ ] 日志脱敏检查。

### [ ] S2-04 Prompt 模板列表和详情接入后端

依赖：S0-02。

执行动作：

- 页面加载调用 `promptTemplatesList`。
- 查看详情调用 `promptTemplatesGet`。
- 只允许 `system`、`stock_full`、`technical`、`custom` 类型。
- UI 中后端不支持的分类能力禁用或改为前端筛选。

交付物：

- Prompt 模板真实列表和详情。

验收标准：

- 刷新后模板仍可见。
- 非白名单 type 被拒绝。
- 变量白名单提示准确。

验证方式：

- Prompt service 测试。
- `pnpm --dir apps build`。

### [ ] S2-05 Prompt 模板新增、编辑、删除

依赖：S2-04。

执行动作：

- 新增调用 `promptTemplatesCreate`。
- 编辑调用 `promptTemplatesUpdate`。
- 删除调用 `promptTemplatesDelete`。
- 保存前校验变量白名单。

交付物：

- Prompt 表单真实保存。

验收标准：

- 新增、编辑、删除刷新后状态一致。
- 非白名单变量不能保存。
- 删除默认模板前有确认或后端保护。

验证方式：

- Prompt service 测试。
- 前端手工验收。

### [ ] S2-06 代理设置保存和脱敏回显

依赖：S0-03。

执行动作：

- 普通代理字段走 `settingsGet/settingsSet`。
- 代理密码走 Rust vault 专用字段，不进入普通 settings items。
- 页面只回显 `has_proxy_password` 和脱敏状态。

交付物：

- 代理配置真实保存。
- 代理密码安全链路。

验收标准：

- 刷新后普通代理配置回显。
- 密码不明文回显。
- SQLite 不保存代理密码明文。

验证方式：

- settings/Rust command 测试。
- secret 搜索检查。

### [!] S2-07 代理连接测试接口确认

依赖：S2-06。

阻塞原因：

- 当前盘点中未确认真实代理测试后端接口。

推荐方案：

- 新增固定 Rust command `proxyTest({ target_type })`。
- Go 或 Rust 只测试受控目标，不开放任意 URL。
- 返回状态、耗时、错误摘要。

验收标准：

- 不允许用户输入任意测试 URL。
- 不输出 `Proxy-Authorization`。
- 失败可定位到认证失败、连接失败或超时。

---

## 5. RG-S3：数据源与凭据安全

### [ ] S3-01 数据源概览接入真实 Provider 状态

依赖：无。

执行动作：

- 数据源概览调用 `providersStatus`。
- 缓存统计可复用 `cacheStats`。
- 同步/调度状态如已实现则调用 `schedulerStatus`。
- 无数据时展示空态或待配置，不展示假正常。

交付物：

- 数据源概览真实状态卡。

验收标准：

- Provider 状态来自后端。
- Provider 异常时可见错误态。
- 页面不再展示假 Provider 健康数据。

验证方式：

- `pnpm --dir apps build`。
- Provider service 测试。

### [!] S3-02 数据源凭据加密 SQLite schema 确认

依赖：S0-01。

阻塞原因：

- 需要新增数据库表和加密密钥管理，属于数据库和安全边界变更。

推荐 schema：

```text
provider_credentials
  id
  provider_id
  auth_type
  encrypted_credential
  nonce
  key_version
  encryption_alg
  masked_credential
  status
  expires_at
  last_tested_at
  last_test_status
  created_at
  updated_at
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

### [ ] S3-03 数据源凭据 API 和 Rust command 白名单

依赖：S3-02 确认。

执行动作：

- 新增凭据列表、读取脱敏状态、保存、清除、测试接口。
- Rust command 固定 path 和 schema。
- 保存时只接受明文一次，返回脱敏状态。
- 读取接口不返回明文。

交付物：

- `dataSourceCredentialsList/GetMasked/Save/Clear/Test` 或等价命名能力。
- Go service/dao/model。
- Rust command。

验收标准：

- 保存后刷新只回显 masked credential。
- 清除后状态变为未配置。
- 测试连接不输出凭据。

验证方式：

- Go 凭据 service 测试。
- Rust command 测试。
- secret 搜索检查。

### [ ] S3-04 凭据管理页接入真实脱敏状态

依赖：S3-03。

执行动作：

- Provider 列表来自 `providersStatus` 和凭据列表。
- 表单加载脱敏配置。
- 保存调用凭据保存接口。
- 清除调用凭据清除接口。
- 测试调用凭据测试接口。

交付物：

- 凭据管理页真实数据接入。

验收标准：

- 已保存凭据不可通过 eye icon 还原。
- 用户本次输入可显示/隐藏，但保存后只回显脱敏值。
- 不写 localStorage/sessionStorage/IndexedDB。

验证方式：

- 页面手工验收。
- `pnpm --dir apps build`。

### [ ] S3-05 Provider 使用凭据的运行时注入

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

### [ ] S3-06 数据说明页保留静态说明并校准链接

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

- 页面手工验收。

---

## 6. RG-S4：通知与工作区迁移

### [ ] S4-01 通知设置项接入 settings

依赖：S0-03。

执行动作：

- 新增并保存 `notifications.in_app_enabled`。
- 新增并保存 `notifications.system_enabled`。
- 接入任务成功、任务失败、Provider 异常通知开关。
- 可选声音提示首版不做或禁用。

交付物：

- 通知设置真实保存和回显。

验收标准：

- 刷新后开关状态一致。
- 关闭应用内通知后不写入或不展示应用内通知。
- 关闭系统通知后不调用 Tauri 通知。

验证方式：

- settings 测试。
- 前端手工验收。

### [!] S4-02 应用内通知表和 API 确认

依赖：S4-01。

阻塞原因：

- 需要新增 SQLite 表和 API。

推荐 schema：

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
  read_at
```

推荐 API：

```text
notificationsList({ unread_only, limit, offset })
notificationsUnreadCount()
notificationsMarkRead({ ids })
notificationsMarkAllRead()
notificationsClearRead()
```

验收标准：

- 未读数量可查询。
- 通知列表可分页。
- 点击通知可按 route 跳转。
- 清理已读不影响任务、报告等源数据。

### [ ] S4-03 TopBar 通知角标和浮层

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
- 点击通知可跳到报告、任务或设置相关页面。
- 浮层关闭后状态不丢失。

验证方式：

- 前端手工验收。
- `pnpm --dir apps build`。

### [ ] S4-04 系统级通知接入 Tauri 通知插件

依赖：S4-01。

执行动作：

- 复用 `@tauri-apps/plugin-notification` 和 Rust `tauri-plugin-notification`。
- 首次使用前检查并请求权限。
- 任务成功、任务失败、Provider 异常按设置触发系统通知。
- 权限被拒绝时只保留应用内通知。

交付物：

- 系统通知触发服务。
- 权限降级逻辑。

验收标准：

- macOS 能看到系统通知。
- 权限拒绝不会报错中断任务。
- 通知内容不包含敏感凭据。

验证方式：

- macOS 手工验收。
- Tauri permission 检查。

### [ ] S4-05 任务和 Provider 事件生成通知

依赖：S4-02、S4-04。

执行动作：

- 监听任务事件 `TASK_SUCCESS`、`TASK_FAILED`。
- Provider 状态从正常变异常时生成通知。
- 通知写入应用内表，并按开关触发系统通知。

交付物：

- 通知事件生产链路。

验收标准：

- 同一任务完成不会重复生成多条通知。
- Provider 异常通知有去重或频率限制。
- 通知 route 可跳转到源页面。

验证方式：

- Go service 测试。
- 前端手工验收。

---

## 7. RG-S5：全局 UI 页面真实数据对接

### [ ] S5-00A 验收股票基础信息搜索入库闭环

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

- `go test ./...`，重点覆盖 `TestStockSearchUpsertsStockCache` 和 `TestStockSearchServiceFallsBackToProviderAndEnqueuesIndexJobs`。
- 前端全局搜索手工验收。

### [!] S5-00B 补齐股票基础资料主动刷新或种子入库流程

依赖：S5-00A。

阻塞原因：

- 当前股票基础信息主要在搜索 fallback 时被动写入；如果用户没有搜索过某只股票，`stocks` 可能没有完整基础资料。
- 个股详情需要的公司资料、行业、概念、上市日期、状态等字段不能只依赖搜索触发。

推荐方案：

- 新增 `StockProfileProvider` 或复用已有 fundamental provider，按 symbol 拉取基础资料。
- 新增固定接口 `stockProfile({ symbol })`，缓存 miss 时抓取并 `UpsertStocks`。
- 新增调度任务 `stock_profile_refresh`，对 active watchlist 做增量刷新。
- 搜索索引重建读取 `stocks`，不直接访问 Provider。

验收标准：

- 个股详情进入时即使未搜索过，也能按 symbol 获取基础资料或明确返回未配置错误。
- 基础资料刷新后 `stocks` 字段更新，搜索索引任务入队。
- 后端缺 Provider 时页面展示空态，不填假行业、假概念。

### [ ] S5-00C 验收行情快照和 K 线入库闭环

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

- `go test ./...`，重点覆盖 market action、DAO repository、scheduler runner。
- 个股详情和自选股手工验收。

### [!] S5-00D 明确行情历史统计是否需要新增表

依赖：S5-00C。

阻塞原因：

- 当前 `quotes` 表按 symbol 覆盖保存最新行情快照，不能用于历史分时、涨跌分布历史、轮询趋势统计。

推荐方案：

- 首版自选股和总览只展示最新 quote，统计从当前 watchlist quote 计算。
- 如需要历史行情快照，新增 `quote_snapshots` 或分时表，按 `symbol + quote_time` 幂等保存，并设置保留周期。
- 不要把当前 `quotes` 改成无限追加表，避免破坏现有最新快照读取语义。

验收标准：

- UI 统计口径明确：当前快照统计或历史快照统计。
- 没有历史表前，不展示需要历史数据支撑的趋势结论。

### [ ] S5-00E 验收新闻资讯入库闭环

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

- `go test ./...`，重点覆盖 news action、DAO repository、scheduler news runner。
- 资讯中心和 Dashboard 手工验收。

### [!] S5-00F 补齐市场新闻 market 维度

依赖：S5-00E。

阻塞原因：

- 当前 `NewsItem` 没有 market 字段；`ListMarketNews(ctx, market, ...)` 目前忽略 market 参数，按全量新闻倒序读取。

推荐方案：

- 如果首版只支持 A 股市场新闻，可在 UI 和文档中明确 `market=CN`，暂不扩表。
- 如果要支持多市场，给 `news_items` 增加 `market` 字段，Provider 写入时带市场，`ListMarketNews` 按 market 过滤。
- 搜索索引重建和新闻统计同步使用 market 字段。

验收标准：

- 市场新闻筛选口径明确。
- 多市场切换时不会混入其他市场新闻。
- 未扩表前不展示多市场新闻筛选。

### [ ] S5-01 AppShell / TopBar 全局状态接入

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

### [ ] S5-02 总览页恢复真实 dashboard 数据流

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

### [ ] S5-03 自选股列表、增删改和行情补全

依赖：S5-01。

执行动作：

- 列表调用 `watchlistList`。
- 添加调用 `watchlistCreate`。
- 编辑备注、标签、排序调用 `watchlistUpdate`。
- 删除调用 `watchlistDelete`。
- 行情字段调用 `marketQuote` 补全。

交付物：

- 自选股页面真实 CRUD。

验收标准：

- 添加后刷新仍存在。
- 删除后刷新不出现。
- 行情不可用时展示错误或空值，不造假涨跌。

验证方式：

- Go watchlist 测试。
- 前端手工验收。

### [!] S5-04 自选股扩展字段确认

依赖：S5-03。

阻塞原因：

- UI 中的 `industry`、`trend`、`starred`、部分统计字段不一定存在于后端模型。

推荐方案：

- `industry` 从股票基础资料或 quote 派生。
- `trend` 从近期 K 线计算。
- `starred` 如为业务字段，扩展 watchlist schema。
- 统计面板优先前端从真实列表和 quote 计算。

验收标准：

- 所有扩展字段都有真实来源。
- 无来源字段不显示或标记待接入。

### [ ] S5-05 个股详情行情、K 线、指标、新闻接入

依赖：S5-01。

执行动作：

- 进入页面调用 `marketQuote(symbol)`。
- 图表调用 `marketKline({ symbol, period, adjust, limit })`。
- 指标调用 `marketIndicators`。
- 相关新闻调用 `newsList`。
- 配置默认周期和复权读取基础设置。

交付物：

- 个股详情真实数据。

验收标准：

- 无 symbol 或查无股票时展示错误态。
- 切换周期会重新拉取 K 线。
- 新闻为空时展示空态。

验证方式：

- market provider 测试。
- 前端手工验收。

### [!] S5-06 个股详情公司资料和标签接口确认

依赖：S5-05。

阻塞原因：

- 公司资料、行业、概念、页面标签备注等字段当前接口可能不足。

推荐方案：

- 新增 `stockProfile({ symbol })` 返回公司资料、行业、概念。
- 页面级用户备注复用 watchlist note，或新增 stock note schema。

验收标准：

- 公司资料字段有稳定来源。
- 用户备注保存后刷新可回显。

### [ ] S5-07 AI 分析页接入模型、Prompt 和任务创建

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

### [ ] S5-08 AI 分析运行页接入任务事件和取消

依赖：S5-07。

执行动作：

- 运行页订阅 `analysisTaskSubscribe`。
- 支持 `TASK_PROGRESS`、`TASK_LOG`、`TASK_CHUNK`、`TASK_SUCCESS`、`TASK_FAILED`。
- 取消按钮调用 `analysisTaskCancel`。

交付物：

- 任务运行实时状态。

验收标准：

- 断线后可从 `taskEvents` 恢复。
- 取消后任务状态正确。
- 成功后可跳转报告详情。

验证方式：

- task event 测试。
- 前端手工验收。

### [ ] S5-09 报告历史默认列表、筛选和删除接入

依赖：S5-08。

执行动作：

- 页面加载调用 `reportList`。
- 筛选条件映射到后端支持字段。
- 搜索继续使用 `searchReports` 或和 `reportList` 统一。
- 删除调用 `reportDelete`。

交付物：

- 报告历史真实列表。

验收标准：

- 删除后刷新不出现。
- 筛选条件后端不支持时禁用或待接入。
- 统计不再使用静态数据。

验证方式：

- report service 测试。
- 前端手工验收。

### [!] S5-10 报告收藏、批量、导出、统计接口确认

依赖：S5-09。

阻塞原因：

- 收藏、批量操作、导出格式和统计口径需要后端接口或产品确认。

推荐方案：

- 收藏新增 `reportUpdate({ id, favorite })`。
- 批量删除新增 `reportBatchDelete(ids)`。
- 导出由 Rust 固定 command 处理文件选择和写入。
- 统计由 `reportStats` 返回，或前端基于当前筛选列表计算。

验收标准：

- 每个按钮都有真实能力或禁用态。
- 导出文件不包含完整敏感输入快照。

### [ ] S5-11 报告详情动作接入

依赖：S5-09。

执行动作：

- 详情读取 `reportGet`。
- 删除调用 `reportDelete`。
- 重新分析调用 `analysisTaskCreate`，使用报告源 symbol 和模板。
- 导出按 S5-10 方案实现或禁用。

交付物：

- 报告详情真实动作。

验收标准：

- 报告不存在展示 404/空态。
- 重新分析创建新任务，不覆盖旧报告。
- 删除后返回报告列表。

验证方式：

- 前端手工验收。

### [ ] S5-12 资讯中心默认列表和筛选接入

依赖：S5-01。

执行动作：

- 默认列表调用 `newsMarket` 或 `newsList`。
- 关键词搜索调用 `searchNews`。
- 打开原文继续调用 `openExternalURL`。
- 加入 AI 上下文如无后端支持先禁用。

交付物：

- 资讯中心真实新闻列表。

验收标准：

- 新闻为空时展示空态。
- 外链通过系统浏览器打开。
- 不展示假热点和假统计。

验证方式：

- news service 测试。
- 前端手工验收。

### [!] S5-13 资讯侧栏统计、热点和加入上下文接口确认

依赖：S5-12。

阻塞原因：

- 侧栏统计、热点榜、加入 AI 上下文需要明确数据来源和保存方式。

推荐方案：

- 统计由 `newsStats` 返回。
- 热点由 `newsHotTopics` 返回。
- 加入上下文先作为 AI 分析页临时输入，不单独落库。

验收标准：

- 统计口径可解释。
- 上下文内容不包含外部网页未授权全文。

### [ ] S5-14 任务历史主列表和事件详情接入

依赖：S5-08。

执行动作：

- 页面加载调用 `taskList`。
- 详情调用 `taskGet` 和 `taskEvents`。
- 任务日志抽屉继续调用 `taskLogs*`。
- 取消调用 `analysisTaskCancel` 或 `task cancel` 固定接口。

交付物：

- 任务历史真实列表和详情。

验收标准：

- 任务状态刷新后不丢。
- 失败任务展示错误摘要。
- 任务事件可回放。

验证方式：

- task service 测试。
- 前端手工验收。

### [!] S5-15 任务重试和报告跳转规则确认

依赖：S5-14。

阻塞原因：

- 重试是否复用原输入、是否生成新 task_id、报告跳转字段来源需要确认。

推荐方案：

- 重试生成新任务，引用 `retry_of_task_id`。
- 报告跳转依赖任务成功后保存的 `report_id`。

验收标准：

- 重试不覆盖原任务事件。
- 成功任务能稳定跳转报告。

---

## 8. RG-S6：关于、日志、更新和发布验收

### [ ] S6-01 关于页检查更新接入

依赖：无。

执行动作：

- 检查更新按钮调用 `checkUpdate`。
- 不实现真实下载安装。
- 返回当前版本、最新版本、是否有更新、说明摘要。

交付物：

- 关于页检查更新真实结果。

验收标准：

- 网络或后端失败时展示错误。
- 不出现授权激活或自动安装入口。

验证方式：

- update service 测试或手工验收。

### [ ] S6-02 日志导出接入并二次脱敏

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

### [!] S6-03 LICENSE、用户手册、发布说明打开方式确认

依赖：无。

阻塞原因：

- 需要确认是内置静态页面、本地文件，还是外部链接。

推荐方案：

- LICENSE 和发布说明使用内置静态页面或本地打包文件。
- 用户手册如打开外链，必须走 `openExternalURL` 并限制 URL 白名单。

验收标准：

- 不直接在前端 `window.open` 外链。
- 外链打开失败有提示。

### [ ] S6-04 全量 secret 和禁用词检查

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

### [ ] S6-05 按变更范围执行最终验证

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
- S5-13：资讯侧栏统计、热点和加入上下文。
- S5-15：任务重试和报告跳转规则。
- S6-03：LICENSE、用户手册、发布说明打开方式。
