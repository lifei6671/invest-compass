# 投研罗盘设置中心配置项对接明细

> 日期：2026-06-22
>
> 目标：重点梳理设置中心各功能配置项、后端字段、保存方式、使用方式、更新方式和对接流程。
>
> 边界：本文只描述对接方案，不表示这些能力已经全部实现。凡涉及数据库 schema、Rust command、凭据 vault、代理测试或数据源凭据的新增能力，实施前必须单独确认。
>
> 开发推进清单：`docs/2026-06-22-invest-compass-settings-ui-backend-development-checklist.md`。后续开发以该清单的任务编号、验收标准和 Review Gate 为准，本文作为设置项字段、保存方式、使用方式和更新方式的明细依据。

---

## 1. 存储分层

| 配置类型 | 存储位置 | 已有后端能力 | 说明 |
| --- | --- | --- | --- |
| 普通非敏感设置 | SQLite `settings` key/value | `settingsGet/settingsSet` | 只保存字符串；Go 会拒绝 password/token/secret/authorization 等敏感 key |
| 工作区路径 | SQLite `settings.workspace_path` | `workspaceGet/workspaceSet`；迁移能力待补 | 默认优先保存到用户个人目录下；切换目录必须支持数据迁移 |
| 开机自启 | OS / Tauri 插件 | `autostartGet/autostartSet` | 不进 SQLite |
| 缓存统计/清理 | Go core 缓存服务 | `cacheStats/cacheClean` | 只清临时缓存，不清报告和配置 |
| 搜索索引 | Go core search service | `searchStatus/searchRebuild` | 重建返回 `task_id` |
| AI 模型配置 | SQLite `ai_configs` + Rust vault | `aiConfigList/save/test/delete` | API Key 明文只进 Rust vault 一次 |
| Prompt 模板 | SQLite `prompt_templates` | `promptTemplates*` | type 和变量有白名单 |
| 代理密码 | Rust vault + SQLite `proxy_credential_ref` | `settingsSet` 专用字段 | Renderer 不能直接把 `proxy_credential_ref` 放进普通 items 写入 |
| 数据源凭据 | SQLite 密文表 + 本地加密密钥 | 暂无 | token/cookie 可加密后存 SQLite；加密密钥不能存 SQLite，前端不能拿明文 |
| 关于页动作 | Go core + Rust 桌面能力 | `checkUpdate/exportLogs` | LICENSE/手册打开方式待确认 |

---

## 2. 基础设置

当前 UI：`apps/frontend/src/pages/settings/basic/SettingsBasicPage.tsx`

当前类型：`BasicSettingsState`

| UI 配置项 | UI 字段 | 推荐后端 key / 字段 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 主题 | `theme` | `app.theme` | `settingsSet([{ key: "app.theme", value }])` | 前端主题初始化；当前不改全局主题时只保存不立即生效 | 页面加载 `settingsGet` 回填；修改后保存并更新本地 state |
| 语言 | `language` | `app.language` | `settingsSet` | AntD locale / i18n；当前只有中文，建议先保存不切换 | 回填；无 i18n 前可禁用切换 |
| 默认市场 | `defaultMarket` | `market.default` | `settingsSet` | 股票搜索、资讯、数据源默认市场 | 保存后后续请求生效 |
| 默认 AI 模型 | `defaultAIModel` | 不建议保存名称；推荐 `ai_configs.is_default` | 由模型配置页 `aiConfigSave({ is_default: true })` 维护 | AI 分析页默认模型 | 从基础设置移除或只读展示 |
| 行情刷新间隔 | `quoteRefreshInterval` | `quote.refresh_interval` | `settingsSet` | Dashboard、自选股、详情页刷新轮询 | 保存后重建页面 timer/store |
| 默认 K 线周期 | `defaultKlinePeriod` | `kline.default_period` | `settingsSet` | 个股详情 `marketKline.period` 默认值 | 新进入详情页生效 |
| 默认复权方式 | `defaultAdjustType` | `kline.default_adjust` | `settingsSet` | `marketKline/marketIndicators.adjust` 默认值 | 新请求生效 |

推荐读取：

```text
settingsGet([
  "app.theme",
  "app.language",
  "market.default",
  "quote.refresh_interval",
  "kline.default_period",
  "kline.default_adjust"
])
```

验收标准：

- 刷新页面后可以从 SQLite 回显配置。
- `defaultAIModel` 不再和模型配置页维护第二份默认模型。
- 不支持立即生效的设置需要明确为“下次进入页面生效”。

---

## 3. 工作区设置

当前类型：`WorkspaceSettingsState`

| UI 配置项 | UI 字段 | 后端字段/接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 工作区路径 | `workspacePath` | `workspace_path` / `workspaceGet/workspaceSet` | 初次使用写入默认个人目录；用户选择目录后走迁移流程，迁移成功再 `workspaceSet(path)` | SQLite、报告导出、日志导出、后续文件型产物默认目录 | 页面加载 `workspaceGet()`；选择目录用 `selectDirectory()`；迁移成功后回填 |

### 3.1 默认目录规则

工作区默认目录必须优先落在用户个人目录下，不能放在应用安装目录、仓库目录、系统根目录或需要管理员权限的位置。

推荐默认值：

| 平台 | 默认工作区 | 用途 |
| --- | --- | --- |
| macOS | `~/Documents/Invest Compass` | 用户可见的工作区、导出报告、可迁移业务文件 |
| Windows 11 | `%USERPROFILE%\\Documents\\Invest Compass` | 用户可见的工作区、导出报告、可迁移业务文件 |

应用内部数据如必须拆分保存，建议遵循系统惯例：

| 数据类型 | macOS 建议目录 | Windows 11 建议目录 |
| --- | --- | --- |
| 应用支持数据 | `~/Library/Application Support/Invest Compass` | `%LOCALAPPDATA%\\Invest Compass` |
| 缓存 | `~/Library/Caches/Invest Compass` | `%LOCALAPPDATA%\\Invest Compass\\Cache` |
| 日志 | `~/Library/Logs/Invest Compass` | `%LOCALAPPDATA%\\Invest Compass\\Logs` |
| 用户导出文件 | `~/Documents/Invest Compass` | `%USERPROFILE%\\Documents\\Invest Compass` |

推荐边界：

- `workspace_path` 面向用户可见、可迁移的数据根目录。
- SQLite 如果被定义为工作区数据，应随工作区迁移。
- 缓存可迁移但不是强制；迁移失败时可重新生成。
- 日志默认留在系统日志目录；用户执行日志导出时复制到工作区或用户选择目录。
- 加密密钥不放在工作区，避免用户复制工作区时把密钥和密文一起带走。

推荐流程：

1. 页面加载调用 `workspaceGet()`。
2. 如果没有已保存路径，按平台生成默认个人目录，并初始化目录结构。
3. “选择目录”调用 `selectDirectory()`。
4. 如果目标目录和当前目录不同，先进入迁移确认流程。
5. 迁移成功后调用 `workspaceSet(path)`。
6. “打开目录”如无 Rust open-path command，保留待接入，不假装打开。

### 3.2 切换工作区迁移能力

用户切换工作区目录时必须具备迁移能力。不能只改 `workspace_path`，否则会造成报告、数据库、索引或导出文件丢失感。

推荐新增能力：

```text
workspaceMigrationPlan({ target_path }) -> WorkspaceMigrationPlan
workspaceMigrate({ target_path, include_cache, create_backup }) -> WorkspaceMigrationResult
```

推荐 `WorkspaceMigrationPlan` 字段：

```text
current_path
target_path
items:
  - name
  - source_path
  - target_path
  - bytes
  - required
  - exists_in_target
  - conflict_strategy
required_bytes
available_bytes
warnings
```

推荐 `WorkspaceMigrationResult` 字段：

```text
from_path
to_path
backup_path
migrated_items
skipped_items
failed_items
workspace_path
```

迁移流程：

```text
用户选择新目录
  -> 校验目标目录为绝对路径且位于用户可写位置
  -> 生成迁移计划：数据库、报告导出、索引、配置快照、可选缓存
  -> 展示迁移内容、大小、冲突和备份位置
  -> 用户确认
  -> 暂停写入任务和调度，避免迁移中产生新文件
  -> 创建目标目录和备份目录
  -> 复制必需数据，校验文件大小/校验和
  -> 切换 workspace_path
  -> 恢复任务和调度
  -> 成功后保留旧目录备份；失败则回滚 workspace_path
```

冲突处理：

- 目标目录为空：直接迁移。
- 目标目录已有 Invest Compass 工作区：提示覆盖、合并或取消，默认取消。
- 目标目录空间不足：禁止迁移。
- 迁移中失败：保留当前工作区不变，展示失败项。

验收标准：

- 空路径和相对路径被后端拒绝。
- 保存后刷新页面回显同一路径。
- 默认路径位于用户个人目录。
- 切换目录前有迁移预检和确认。
- 迁移失败不会破坏原工作区。
- 迁移成功后新工作区可读取原报告、配置、索引和必要数据库。

---

## 4. 通知设置

当前类型：`NotificationSettingsState`

通知功能拆成两块：

1. 应用内通知：应用全局右上角通知区域展示未读数量角标，点击通知按钮弹出通知列表浮层。
2. 系统级通知：使用 Rust/Tauri notification 插件触发系统通知。

当前仓库已具备系统级通知基础能力：

- 前端依赖 `@tauri-apps/plugin-notification`。
- Rust 依赖 `tauri-plugin-notification`。
- Tauri capability 已允许 `notification:allow-is-permission-granted`、`notification:allow-request-permission`、`notification:allow-notify`。
- 前端已有 `sendDesktopNotification` helper。

### 4.1 设置项

| UI 配置项 | UI 字段 | 推荐 settings key | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 应用内通知总开关 | 新增 `inAppEnabled` | `notifications.in_app_enabled` | `settingsSet` 保存 `"true"` / `"false"` | 控制是否写入和展示应用内通知 | 修改后保存并更新通知 store |
| 系统级通知总开关 | 新增 `systemEnabled` | `notifications.system_enabled` | `settingsSet` | 控制是否调用 Tauri notification | 修改后保存；关闭后不弹系统通知 |
| 任务成功通知 | `taskSuccessNotification` | `notifications.task_success` | `settingsSet` | `TASK_SUCCESS` 生成应用内通知；如系统开关开启则弹系统通知 | 修改后保存并更新 state |
| 任务失败通知 | `taskFailedNotification` | `notifications.task_failed` | `settingsSet` | `TASK_FAILED` 生成通知 | 同上 |
| Provider 异常通知 | 新增 `providerErrorNotification` | `notifications.provider_error` | `settingsSet` | Provider 不可用或凭据失效时通知 | 后续 Provider 状态接入后更新 |
| 声音提示 | 可选 `soundEnabled` | `notifications.sound_enabled` | 暂缓 | 控制系统通知声音 | 首版可不做 UI |

### 4.2 应用内通知中心

推荐新增 SQLite 表：

```text
notifications
  id
  type
  level
  title
  content
  source_type
  source_id
  source_event_id
  route
  read_at
  created_at
  updated_at
```

字段说明：

- `type`：`task` / `provider` / `system` / `search_index`
- `level`：`info` / `success` / `warning` / `error`
- `source_type`：来源类型，例如 `task`、`provider`、`scheduler`
- `source_id`：来源 ID，例如 `task_id`、`provider_id`
- `source_event_id`：任务事件 ID，用于幂等去重
- `route`：点击通知后的应用内跳转路径，例如 `/tasks`、`/reports/123`
- `read_at`：为空表示未读

推荐 API：

```text
notificationsList({ unread_only?, limit?, offset? })
notificationsUnreadCount()
notificationsMarkRead({ ids })
notificationsMarkAllRead()
notificationsDelete({ ids })
```

推荐 Rust command：

```text
notifications_list
notifications_unread_count
notifications_mark_read
notifications_mark_all_read
notifications_delete
```

TopBar 交互：

```text
应用启动 / TopBar mount
  -> notificationsUnreadCount()
  -> Bell 按钮外层展示 Badge count
  -> 点击 Bell
  -> Popover/Dropdown 打开通知列表
  -> notificationsList({ limit: 20 })
  -> 点击单条通知：markRead + navigate(route)
  -> 点击全部已读：notificationsMarkAllRead()
```

浮层要求：

- 有未读角标。
- 有加载态、空态、错误态。
- 每条通知展示标题、正文摘要、时间、状态色。
- 支持“全部已读”。
- 通知跳转只允许应用内路由，不打开任意外链。

### 4.3 系统级通知

推荐触发策略：

```text
任务/Provider/系统事件
  -> 后端生成应用内通知
  -> 前端全局通知 store 读取到新增通知
  -> 检查 settings 中 notifications.system_enabled 和具体类型开关
  -> 调用 sendDesktopNotification({ title, body })
```

系统级通知不单独持久化，持久化记录以应用内 `notifications` 表为准。

系统通知触发范围建议：

- `TASK_SUCCESS`
- `TASK_FAILED`
- `TASK_CANCELLED` 可选
- Provider 凭据失效
- 数据源连续失败
- 搜索索引重建完成/失败

权限处理：

- 首次发送前调用 `isPermissionGranted`。
- 未授权时调用 `requestPermission`。
- 用户拒绝后不反复弹权限请求，只在设置页提示系统通知权限未授予。

### 4.4 通知生成规则

推荐从统一事件入口生成通知，不让页面各自生成：

```text
Go task/provider/system event
  -> notification service 判断设置和去重
  -> 写 notifications 表
  -> 前端 TopBar 定期或事件驱动刷新未读数
```

第一批事件来源：

- `TASK_SUCCESS`
- `TASK_FAILED`
- `TASK_CANCELLED`
- `scheduler run failed`
- `provider credential expired`

去重规则：

- 同一 `source_type + source_id + source_event_id` 只能生成一条通知。
- 没有 `source_event_id` 的 Provider 状态类通知，按 `source_type + source_id + type + 日期小时` 做粗粒度去重。

缺口：

- 没有通知列表表。
- 没有通知 CRUD API。
- TopBar 只有静态通知按钮，没有 Badge 和浮层。
- `sendDesktopNotification` helper 未接全局事件。
- 设置页通知开关还未接 `settingsGet/settingsSet`。
- 系统通知权限拒绝后的 UI 降级提示未实现。

---

## 5. 桌面能力设置

当前类型：`DesktopSettingsState`

| UI 配置项 | UI 字段 | 后端字段/接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 开机启动 | `autostart` | `autostartGet/autostartSet` | `autostartSet(enabled)` | OS 登录项 | 页面加载 `autostartGet()`；切换后调用 `autostartSet` |
| 关闭到托盘 | `closeToTray` | `window.close_to_tray` | `settingsSet` | Rust window close handler 读取 | 保存后写 settings；若 Rust 未读取该 key，UI 只能标待接入 |

推荐方案：

- `autostart` 不进入 SQLite。
- `closeToTray` 只有在 Rust 关闭窗口逻辑读取该 key 后才能宣称生效。

---

## 6. 缓存管理

当前类型：`CacheSummary`

| UI 配置项 | 后端字段/接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- |
| 缓存总量 | `CacheStatsResult.total_bytes` / `cacheStats` | 不保存 | 展示当前缓存体积 | 页面加载和清理后读取 |
| 缓存项 | `CacheStatsResult.items` | 不保存 | 展示 target、size、cleanable | 后端返回为准 |
| 清理缓存 | `cacheClean(targets)` | Go core 执行清理 | 释放临时缓存 | 成功后重新 `cacheStats()` |

允许清理：

- `quote`
- `kline`
- `news`
- `chart_image`
- `task_logs`
- `app_logs`

禁止清理：

- `report`
- `config`

验收标准：

- 清理不删除报告和配置。
- 清理失败展示后端错误。

---

## 7. 搜索索引管理

| UI 配置项 | 后端字段/接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- |
| 索引状态 | `searchStatus()` | 不保存 | 展示索引状态和统计 | 页面加载、重建后刷新 |
| 重建范围 | `SearchRebuildPayload.scope` | 不保存 | 触发报告/资讯/自选备注等索引重建 | `searchRebuild({ scope, force: false })` |
| 重建任务 | `SearchRebuildResult.task_id` | 由任务系统保存 | 去任务历史追踪 | 返回后展示 task_id |

推荐方案：

- `force` 默认 false。
- 强制重建需要二次确认。

---

## 8. 其他设置

当前类型：`OtherSettingsState`

| UI 配置项 | UI 字段 | 推荐处理 |
| --- | --- | --- |
| 启动时检查更新 | `checkUpdateOnStartup` | 保存到 `update.check_on_startup`；启动后如为 true 调用 `checkUpdate` |
| 匿名使用统计 | `anonymousUsageStats` | 当前项目无遥测边界，不建议实现；建议从 UI 移除或固定 false |

---

## 9. 模型配置

当前 UI：`apps/frontend/src/pages/settings/model-config/ModelConfigPage.tsx`

当前后端：`ai_configs` 表 + Rust 本地 vault。

| UI 配置项 | UI 字段 | 后端字段 | 接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- | --- |
| 配置 ID | `id` | `AIConfig.id` | `aiConfigList/save/delete/test` | 新建后由后端生成；编辑带 id | AI 分析引用 `ai_config_id` | 保存返回值覆盖本地 |
| 配置名称 | `name` | `name` | `aiConfigSave` | 普通字段保存 | UI 展示和选择 | 可编辑 |
| Provider | `provider` | `provider` | `aiConfigSave` | 普通字段保存 | AI 调用适配 | 当前建议只启用 OpenAI-compatible，其他 Provider 标待接入 |
| Base URL | `baseUrl` | `base_url` | `aiConfigSave` | 普通字段保存 | AI endpoint | 可编辑，应校验 URL |
| 模型名称 | `modelName` | `model_name` | `aiConfigSave` | 普通字段保存 | AI 请求 model | 可编辑 |
| API Key | `apiKeyInput` | `api_key_ref/masked_api_key/has_api_key` | `aiConfigSave` | 明文只作为 `api_key` 一次性给 Rust vault；Go 只收 ref/masked 状态 | 测试和分析任务由 Rust 解析注入 | 保存成功清空输入，不回显明文 |
| Temperature | `temperature` | `temperature` | `aiConfigSave` | 普通字段保存 | AI 请求参数 | 可编辑 |
| Max Tokens | `maxTokens` | `max_tokens` | `aiConfigSave` | 普通字段保存 | AI 请求参数 | 可编辑 |
| 超时秒数 | `timeoutSeconds` | `timeout_seconds` | `aiConfigSave` | 普通字段保存 | AI 请求 timeout | 可编辑 |
| 流式输出 | `streamEnabled` | `stream_enabled` | `aiConfigSave` | 普通字段保存 | 分析任务是否流式 | 可切换 |
| 默认配置 | `isDefault` | `is_default` | `aiConfigSave` | 普通字段保存 | AI 分析页默认模型 | 设置一个默认时其他配置应取消默认 |
| 连接状态 | `connectionStatus` | 当前无持久字段 | `aiConfigTest` | 不建议持久化 | 临时展示最近测试结果 | 测试后本地更新 |

保存流程：

```text
用户点击保存
  -> 校验 name/base_url/model_name/timeout/token 参数
  -> aiConfigSave(payload)
  -> Rust 如有 api_key，写入本地 vault，生成 api_key_ref 和 masked_api_key
  -> Rust 固定调用 Go /api/ai/configs/save
  -> Go 保存 ai_configs 元数据
  -> 前端用返回 config 刷新列表，清空 apiKeyInput
```

使用流程：

```text
AI 分析页读取 aiConfigList
  -> 选择 is_default 或用户指定配置
  -> analysisTaskCreate 携带 ai_config_id 和 api_key_ref
  -> Rust 从 vault 解析 API Key
  -> Go core 只在任务内存中使用 resolved_api_key
```

验收标准：

- SQLite 只保存 `api_key_ref/masked_api_key/has_api_key`。
- 前端列表和错误态不泄露真实 API Key。

---

## 10. Prompt 配置

当前 UI：`apps/frontend/src/pages/prompt-template/PromptTemplatePage.tsx`

当前后端：`prompt_templates` 表。

| UI 配置项 | UI 字段 | 后端字段 | 接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- | --- |
| 模板 ID | `id` | `id` | `promptTemplates*` | 后端生成 | AI 分析引用 `prompt_template_id` | 返回值覆盖 |
| 模板名称 | `templateName` | `name` | create/update | 普通字段保存 | UI 列表和任务记录 | 可编辑 |
| 模板类型 | `templateType` | `type` | create/update | 必须在后端白名单 | 匹配分析类型 | 可编辑但限制选项 |
| 模板说明 | `templateDescription` | `description` | create/update | 普通字段保存 | UI 展示 | 可编辑 |
| Prompt 内容 | `promptContent` | `content` | create/update | 普通字段保存 | AI prompt 构建 | 可编辑 |
| 变量列表 | 当前静态 `promptVariables` | `variables` | 后端解析返回 | 不由前端直接保存 | 展示变量命中 | 保存后以后端返回为准 |
| 是否内置 | `isBuiltin` | `is_builtin` | list/get | 后端字段 | 禁止删除 | 不允许前端改 |
| 分类 | `selectedCategoryId` | 后端无字段 | 无 | 不建议持久化 | 仅 UI 分组 | 映射到 `type` |

后端允许类型：

- `system`
- `stock_full`
- `technical`
- `custom`

当前 UI 分类和后端白名单不一致。推荐处理：

1. UI 分类改为后端 `type` 分组。
2. `financial_analysis`、`position_analysis`、`market_review` 暂不作为真实类型保存。
3. 新建分类按钮隐藏或提示待接入。
4. 保存时只提交 `name/type/description/content`。
5. 保存成功后以后端返回的 `variables` 更新变量提示。

---

## 11. 数据源设置

当前 UI：`apps/frontend/src/pages/settings/data-source/DataSourceSettingsPage.tsx`

当前二级 Tab：

- 数据源概览
- 凭据管理
- 数据说明

### 11.1 数据源概览

| UI 配置项 | UI 字段 | 推荐后端字段/接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 默认行情源 | `defaultMarketSource` | 暂无；推荐 `data_source.market_source` | 当前不保存 | Provider 选择策略 | 需先确认 Provider 配置模型 |
| 默认资讯源 | `defaultNewsSource` | 暂无；推荐 `data_source.news_source` | 当前不保存 | 新闻 Provider 选择 | 同上 |
| 默认市场范围 | `defaultMarketScope` | 可复用 `market.default` | `settingsSet` | 默认新闻/行情范围 | 后续请求生效 |
| K 线范围 | `klineRange` | `kline.default_range` | `settingsSet` | K 线默认范围 | 后续请求生效 |
| 行情刷新间隔 | `quoteRefreshInterval` | `quote.refresh_interval` | `settingsSet` | 页面轮询 | 与基础设置共用 |
| 新闻同步间隔 | `newsSyncInterval` | 推荐 scheduler job cron | `schedulerJobsSave` | 新闻同步调度 | 复用调度系统 |
| 启动同步 | `syncOnStartup` | 推荐 scheduler catchup/startup params | `schedulerJobsSave` 或待确认 | 启动补偿抓取 | 复用调度系统 |
| 非交易时段降频 | `reduceFrequencyOutsideTradingHours` | 推荐 scheduler params | `schedulerJobsSave` 或待确认 | 调度策略 | 复用调度系统 |

推荐方案：

- 数据源概览先接 `providersStatus + schedulerStatus + cacheStats`。
- 同步策略复用 `/scheduler`，不新造第二套任务系统。

### 11.2 凭据管理

数据源凭据指已对接 Provider 访问真实数据时需要的 token、cookie、API Key 或 Bearer Token。该类凭据可以加密后保存到 SQLite，但必须满足：

- SQLite 只保存密文、nonce、算法、key version、脱敏展示值和状态元数据。
- 加密密钥不保存到 SQLite，建议由 Rust 桌面层管理，来源可以是本地安全文件密钥或后续平台 Keychain。
- 前端只提交用户本次输入的明文，保存成功后立即清空，不回显已保存明文。
- Go core 只在 Provider 请求执行期解密并使用凭据，不写日志、不进入任务事件、不进入导出文件。

| UI 配置项 | UI 字段 | 推荐后端字段 | 推荐保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| Provider | `providerId/providerName` | `provider_id` | SQLite 元数据 | 选择数据源凭据 | Provider 列表读取 |
| 认证方式 | `authType` | `auth_type` | SQLite 元数据 | 请求构造策略 | 保存后生效 |
| Base URL | `baseUrl` | `base_url` | SQLite 元数据或 Provider 配置 | Provider endpoint | 保存后生效 |
| 凭据状态 | `credentialStatus` | `status` | SQLite 元数据 | UI 状态 | 测试/清除后更新 |
| 过期时间 | `expiresAt` | `expires_at` | SQLite 元数据 | 到期提醒 | 保存后回显 |
| 请求超时 | `timeoutSeconds` | `timeout_seconds` | SQLite 元数据 | Provider timeout | 保存后生效 |
| 频率限制 | `rateLimitPerMinute` | `rate_limit_per_minute` | SQLite 元数据 | Provider 限流 | 保存后生效 |
| 凭据内容 | `maskedCredential` + 新输入 | `encrypted_credential/nonce/key_version/masked_credential` | Rust 或 Go service 加密后写 SQLite 密文 | Provider 请求时解密并注入 | 保存成功清空明文 |
| 备注 | `note` | `note` | SQLite 元数据 | UI 展示 | 保存后回显 |
| 操作日志 | `CredentialOperationLog` | `data_source_credential_logs` | Go 记录脱敏操作日志 | 审计展示 | 保存/测试/清除后追加 |

推荐表结构：

```text
data_source_credentials
  id
  provider_id
  provider_name
  auth_type
  base_url
  encrypted_credential
  nonce
  key_version
  encryption_alg
  masked_credential
  status
  expires_at
  timeout_seconds
  rate_limit_per_minute
  note
  last_tested_at
  last_test_status
  created_at
  updated_at
  deleted_at

data_source_credential_logs
  id
  provider_id
  action
  status
  message
  created_at
```

推荐接口：

```text
dataSourceCredentialsList()
dataSourceCredentialGet(provider_id)
dataSourceCredentialSave(payload)
dataSourceCredentialClear(provider_id)
dataSourceCredentialTest(provider_id, target)
dataSourceCredentialLogs(provider_id?)
```

保存流程：

```text
用户输入 token/cookie
  -> 前端提交本次输入明文到固定 Rust command
  -> Rust command 校验 provider_id/auth_type/request schema
  -> Rust 调 Go API 或本地加密模块，加密明文得到 encrypted_credential + nonce + key_version
  -> SQLite 保存密文和脱敏值
  -> 前端清空明文输入，只显示 masked_credential 和状态
```

使用流程：

```text
Provider 发起真实请求
  -> Go service 读取 data_source_credentials
  -> 根据 key_version 取本地加密密钥
  -> 解密得到运行期凭据
  -> 仅在当前请求内注入 Cookie/API Key/Bearer Token
  -> 请求结束后丢弃明文
```

安全要求：

- 真实接入前必须确认 DB migration、加密密钥管理方式、解密发生在 Rust 还是 Go、Provider 注入方式和 Cookie 是否允许。
- 建议使用 AEAD，例如 AES-GCM 或 ChaCha20-Poly1305；每条凭据使用独立 nonce。
- `masked_credential` 必须由后端生成，不能由前端传入作为权威展示值。
- `encrypted_credential`、`nonce`、`key_version` 不进入日志、导出和任务事件。
- 清除凭据执行软删除或清空密文字段，并追加脱敏操作日志。
- 测试连接必须是白名单目标，不能做任意 URL 网络诊断。

### 11.3 数据说明

数据说明页是静态说明内容，不需要后端保存。

更新方式：

- Provider、数据范围或同步策略真实变更时，同步更新文案。
- “查看数据源概览”等按钮切本地二级 Tab，不跳不存在页面。

---

## 12. 代理设置

当前 UI：`apps/frontend/src/pages/settings/proxy/ProxySettingsPage.tsx`

后端已有安全保存能力：`settingsSet` 的 `proxy_password`、`proxy_credential_ref`、`clear_proxy_credential` 专用字段。

| UI 配置项 | UI 字段 | 推荐 settings key / 字段 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 代理模式 | `proxyMode` | `proxy.mode` | `settingsSet` | 外部数据源和 AI 请求是否走代理 | 保存后后续请求生效 |
| HTTP host/port/protocol | `httpProxyConfig` | `proxy.http_url` | `settingsSet` | HTTP/HTTPS 代理地址 | URL 禁止 username/password |
| SOCKS host/port/version | `socks5ProxyConfig` | `proxy.socks5_url` | `settingsSet` | SOCKS 代理地址 | URL 禁止 username/password |
| 认证开关 | `authenticationEnabled` | `proxy.auth_enabled` | `settingsSet` | 判断是否读取代理凭据 | 保存后生效 |
| 用户名 | `username` | `proxy.username` | `settingsSet` | 代理认证用户名 | 可保存，不能含密码 |
| 密码 | `password` | Rust vault + `proxy_credential_ref` | `settingsSet({ proxy_password })` | 请求时通过 ref 解析 | 保存成功清空明文；清除走 `clear_proxy_credential` |
| 超时 | `timeoutSeconds` | `proxy.timeout_seconds` | `settingsSet` | 外部请求 timeout | 保存后生效 |
| 绕过规则 | `bypassRules` | `proxy.bypass_rules` | `settingsSet` | 代理绕过 | 保存后生效 |
| 测试目标 | `testTarget` | 不建议保存 | 暂不保存 | 只用于本次测试 | 本地 state |
| 测试结果 | `testResult` | 当前无后端字段 | 不保存 | 临时 UI 展示 | 测试后本地更新 |

保存流程：

```text
用户保存代理配置
  -> 前端组装非敏感 settings items
  -> 如果输入新密码，放入 SettingsSetPayload.proxy_password
  -> 如果清除密码，设置 clear_proxy_credential=true
  -> settingsSet(payload)
  -> Rust 将 proxy_password 写入 vault，生成 local-vault://proxy/... 引用
  -> Rust 把 proxy_credential_ref 合并进 settings items
  -> Go 校验 proxy URL 不含用户名/密码，保存 settings
  -> 成功后 Rust 清理旧 vault 引用，前端清空 password 输入
```

缺口：

- 代理连通性测试 command 未实现。
- 系统代理状态读取未接真实 OS 能力。

---

## 13. 关于应用

当前 UI：`apps/frontend/src/pages/settings/about/AboutAppPage.tsx`

| UI 配置项/动作 | 当前字段 | 后端接口 | 保存方式 | 使用方式 | 更新方式 |
| --- | --- | --- | --- | --- | --- |
| 当前版本 | `updateInfo.currentVersion` | `coreHealth.version` 或 app package version | 不保存 | 关于页展示 | 打开页面读取 |
| 检查更新 | `updateInfo` | `checkUpdate` | 不保存 | 展示最新版本、状态和说明 | 点击按钮调用 |
| 授权状态 | `licenseInfo` | 当前仅 Free 占位 | 不保存 | 展示开源/免费状态 | 不做激活 |
| LICENSE | `resourceLinks.license` | 本地 LICENSE 文件或内置页面 | 不保存 | 打开许可说明 | 待接 open file 或内置路由 |
| 用户手册 | `resourceLinks.manual` | 暂无 | 不保存 | 打开文档 | 无正式手册前保持待接入 |
| 日志导出 | `resourceLinks.logs` | `exportLogs` | 导出文件到工作区或系统路径 | 问题排查 | 点击后调用并展示 `file_path` |

推荐处理：

- 授权状态不要出现“激活”入口。
- 日志导出必须走脱敏导出链路。
- 发布说明如果无后端字段，先由 `checkUpdate` 返回 message 或本地 release notes 文档提供。

---

## 14. 设置中心对接流程

### 步骤 1：建立配置 key 清单

任务：

- 新增前端常量文件，例如 `apps/frontend/src/pages/settings/settingsKeys.ts`。
- 统一登记所有 `settings` key，禁止页面散落字符串。

建议初始 key：

```text
app.theme
app.language
market.default
quote.refresh_interval
kline.default_period
kline.default_adjust
notifications.task_success
notifications.task_failed
window.close_to_tray
update.check_on_startup
proxy.mode
proxy.http_url
proxy.socks5_url
proxy.auth_enabled
proxy.username
proxy.timeout_seconds
proxy.bypass_rules
proxy_credential_ref
```

验收标准：

- 页面不再手写重复 key。
- `proxy_credential_ref` 只能被读取，不能作为普通 `items` 写入。

### 步骤 2：基础设置真实读取和保存

任务：

- `SettingsBasicPage` mount 时调用 `settingsGet`、`workspaceGet`、`autostartGet`。
- 基础偏好、通知、关闭到托盘、启动检查更新用 `settingsSet`。
- 工作区如果为空，按平台初始化到用户个人目录：macOS `~/Documents/Invest Compass`，Windows 11 `%USERPROFILE%\\Documents\\Invest Compass`。
- 工作区切换时先调用迁移预检和迁移执行，迁移成功后再 `workspaceSet`。
- 开机自启用 `autostartSet`。
- 通知开关用 `settingsSet` 保存，包括应用内通知、系统级通知、任务成功/失败、Provider 异常。

验收标准：

- 刷新页面后配置回显。
- 相对工作区路径保存失败。
- 默认工作区位于用户个人目录。
- 切换工作区失败时原目录仍可用。
- 敏感 key 保存失败。

### 步骤 2.5：通知中心和系统通知接入

任务：

- 新增 `notifications` 表和通知 CRUD API。
- 新增 `notificationsUnreadCount` 读取未读数量。
- TopBar Bell 增加 Badge 角标。
- 点击 Bell 弹出通知列表浮层。
- 任务终态事件和 Provider 异常生成应用内通知。
- 系统级通知复用 Tauri notification 插件，并受设置项控制。

验收标准：

- `TASK_SUCCESS` / `TASK_FAILED` 只生成一条应用内通知。
- TopBar 未读数刷新正确。
- 点击通知后标记已读并跳转应用内 route。
- 系统通知权限未授予时不崩溃，并在设置页提示。
- 关闭系统级通知后不再调用 `sendDesktopNotification`。

### 步骤 3：缓存和索引保持现有真实接口

任务：

- 保留 `cacheStats/cacheClean`。
- 保留 `searchStatus/searchRebuild`。
- 清理后重新读取缓存统计。
- 重建后展示 `task_id`。

验收标准：

- 不清理 report/config。
- 索引重建失败展示错误。

### 步骤 4：模型配置迁移真实 service

任务：

- `ModelConfigPage` 接 `aiConfigList/save/test/delete`。
- UI provider 列表先只启用 OpenAI-compatible，其他 Provider 标记待接入或禁用。
- 保存 API Key 后清空明文输入。

验收标准：

- SQLite 只保存 `api_key_ref/masked_api_key/has_api_key`。
- 列表和测试错误不泄露真实 Key。

### 步骤 5：Prompt 配置迁移真实 service

任务：

- `PromptTemplatePage` 接 `promptTemplatesList/create/update/delete`。
- 分类改为后端 `type` 分组。
- 新建分类能力隐藏或待接入。

验收标准：

- 非白名单 type 无法保存。
- 内置模板不可删除。
- 变量列表以后端返回为准。

### 步骤 6：代理设置接入 vault 安全链路

任务：

- 页面加载读取 `proxy.*` 和 `proxy_credential_ref`。
- 保存时通过专用 `proxy_password/clear_proxy_credential` 字段处理密码。
- URL 中禁止带用户名或密码。

验收标准：

- 密码不进入普通 settings items。
- 保存成功后前端清空明文密码。
- 清除凭据后 `proxy_credential_ref` 为空。

### 步骤 7：数据源设置分阶段处理

任务：

- 概览页先接 `providersStatus/cacheStats/schedulerStatus`。
- 数据说明保持静态。
- 凭据管理保持待接入，直到确认 encrypted SQLite、加密密钥管理、API 和 DB migration 方案。

验收标准：

- 数据源概览不再展示 mock 健康状态。
- 凭据管理在方案落地前不保存真实凭据；方案落地后只能保存密文，不能保存明文。

### 步骤 8：关于页真实动作

任务：

- 检查更新接 `checkUpdate`。
- 日志导出接 `exportLogs`。
- LICENSE/手册按本地文件或内置说明处理。

验收标准：

- 日志导出路径可展示。
- 不出现授权激活入口。

### 步骤 9：验证和回归

任务：

- 前端补设置页 service mock 测试。
- Rust 补代理 vault 边界测试。
- Go 补 settings key 校验测试。
- 跑 `git diff --check` 和相关前端/后端验证。

验收标准：

- 所有保存接口失败时 UI 有错误态。
- 所有敏感字段不回显、不入日志、不入 SQLite 明文字段。
- 设置页刷新后可从真实后端恢复状态。
