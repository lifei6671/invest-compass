# 投研罗盘首版发布与使用指南

> 面向首版内测用户和开发验收人员。本文只描述首版范围内的能力和风险边界，不把未闭环能力写成已完成。

## 1. 产品边界

投研罗盘是一款本地 AI 投研桌面工作台，用于辅助个人投资者和技术型研究者整理股票行情、K 线、资讯、技术指标和 AI 分析报告。

所有行情、资讯、技术指标和 AI 输出都仅作研究辅助，不构成投资建议。用户必须自行判断信息准确性、数据时效和投资风险。

首版明确不提供以下能力：

- 不接券商账户。
- 不做实盘交易、自动下单或交易托管。
- 不承诺收益，不输出“稳赚”“必涨”等诱导性结论。
- 不做云端同步、多人协作或移动端。
- 不做授权激活闭环。
- 不做真实自动更新下载、安装或静默升级。
- 不单独保存用户持仓信息。

## 2. 首版能力

首版目标能力：

- 本地桌面应用启动和 Go sidecar core 自动启动。
- 股票搜索、自选股、行情、K 线和技术指标。
- 新闻资讯基础聚合。
- OpenAI-compatible 模型配置和连通性测试。
- Prompt 模板和合规输出约束。
- 单个个股 AI 分析任务。
- 任务事件、报告保存、报告历史和任务历史。
- 设置中心基础能力，包括工作区、代理、缓存、数据源状态、检查更新和日志导出入口。
- 数据刷新任务调度，包括交易日定时刷新、启动补偿、手动补偿和单股刷新入口。

以上能力以当前实施清单为准：`docs/2026-06-17-invest-compass-implementation-checklist.md`。

## 3. 安装和启动

内测包应由发布人员从本仓库构建产物生成，不应在运行时下载 Go sidecar。

发布前必须确认：

- 构建输入目录包含当前 target triple 对应的 Go sidecar 文件。
- macOS 发布包运行时包含 `invest-compass-core`。
- Windows x64 发布包运行时包含 `invest-compass-core.exe`。
- Tauri 配置只声明首版需要的权限。
- Go sidecar 只监听 `127.0.0.1`。
- runtime token 不出现在 argv、env、日志、配置文件或数据库中。

内测用户启动应用后，如果 sidecar 启动失败，应保留日志导出包并反馈给开发人员。日志导出包应已经过二次脱敏。

## 4. 模型和凭据

API Key 和代理密码属于敏感凭据。

首版约定：

- API Key 和代理密码保存到 Rust 本地文件 vault。
- macOS / Windows 均使用本地文件 vault，首版不强制依赖平台凭据服务。
- Unix/macOS 下本地 vault 目录应以 `0700` 权限保存，secret 文件应以 `0600` 权限保存。
- AI Provider、本地 vault 引用和代理 profile 必须解析为明确文件名，非法值不会创建弱语义凭据文件或隐式 `.secret` 文件。
- Go core 只接受 `local-vault://ai-config/` 和 `local-vault://proxy/` 这两类凭据引用。
- SQLite 只保存 `api_key_ref`、`proxy_credential_ref`、`has_api_key` 和包含脱敏标记的展示值。
- 前端和日志导出不得包含真实 API Key、代理密码、Authorization 或 Proxy-Authorization。

验收本地 vault 时，只确认引用存在、文件创建/删除符合预期，不使用会打印明文 secret 的命令。

## 5. 数据来源和时效

首版数据源能力用于研究辅助。

用户需要理解：

- 股票行情和新闻资讯可能存在延迟、缺失或第三方服务不可用。
- 技术指标由本地规则计算，结果依赖输入行情和 K 线质量。
- AI 分析依赖模型、Prompt、行情、K 线、资讯和指标上下文。
- AI 输出可能出现事实错误、遗漏、推断偏差或过时信息。
- 报告中的结论不能直接作为买卖依据。

AI 报告必须保留：

- 数据时效说明。
- 风险提示。
- 非投资建议声明。
- 事实、推断和观点的区分。
- 后续观察指标。

## 6. 基本使用流程

首版目标主流程如下。当前仅在 RG5/T31-T38 通过后，才能作为内测用户可执行流程：

1. 启动桌面应用。
2. 在设置中确认工作区和网络代理配置。
3. 配置 OpenAI-compatible 模型。
4. 搜索股票并加入自选。
5. 查看个股行情、K 线、指标和资讯。
6. 选择 Prompt 模板发起个股 AI 分析任务。
7. 查看任务进度和流式输出。
8. 保存分析报告。
9. 在报告历史或任务历史中回看结果。

如果任一步骤显示错误，应优先检查：

- 网络和代理是否可用。
- 模型配置是否完整。
- API Key 或代理密码是否已写入 Rust 本地文件 vault。
- 数据源是否可用。
- sidecar 是否正常启动。

## 7. 搜索范围和索引状态

首版不提供跨菜单全局搜索，也不提供独立 `/search` 全局搜索页。

搜索入口约定：

- 顶部搜索框默认只搜索股票，点击结果进入个股详情。
- 报告历史页只搜索报告历史范围。
- 资讯中心页只搜索资讯范围。
- 自选股页的添加弹窗继续搜索股票；自选备注和标签搜索只搜索当前自选备注范围。
- 任务历史首版只保留筛选能力，不接入任务日志全文搜索。

设置中心的“搜索索引”卡片用于查看本地索引健康状态和触发手动重建。它会展示 FTS5 状态、GSE / 降级分词状态、股票 / 报告 / 新闻 / 自选备注索引数量、最后重建时间、词典版本和当前 active batch。

如果 FTS5 不可用，股票搜索会尽量退回到本地 code / name / pinyin 查询和 Provider fallback；报告、资讯、自选备注这类菜单范围全文搜索会保持不可用提示，不会伪造结果。

## 8. 设置中心

设置中心用于管理本地运行参数，不承载交易、授权激活或自动升级能力。

当前设置能力包括：

- 保存工作区路径。
- 保存不含用户名和密码的代理 URL。
- 将代理密码写入 Rust 本地文件 vault，前端和 Go core 只使用凭据引用。
- 查看缓存占用并清理允许的缓存目标。
- 查看数据源状态和不可用原因。
- 开关 AI 分析任务成功/失败后的系统通知；通知权限拒绝或系统通知失败不会改变任务状态。
- 开关系统开机自动启动；该能力仍需在目标 macOS / Windows 安装环境做真实验收。
- 检查更新并展示版本提示。
- 导出已二次脱敏的日志包。
- 查看当前 `FREE` 状态和投研风险提示。

检查更新配置：

- `update.manifest_url` 必须使用 HTTPS，并命中允许域名。
- `update.allowed_hosts` 使用逗号分隔的域名列表。
- manifest 内下载链接和发布说明链接也必须命中 allowlist。
- 首版只提示版本，不下载、不安装、不静默升级。

日志导出边界：

- 目标目录不能为空，且必须是用户指定的本地目录。
- Go core 只生成已脱敏日志包，不直接写入用户目录。
- Rust 写入导出文件时会拒绝路径穿越文件名和同名覆盖。

## 9. 数据刷新调度

任务调度用于刷新本地研究数据，不代表交易信号，也不会连接券商账户或自动下单。

当前调度能力包括：

- 查看调度任务总数、启用任务、排队中、运行中和失败记录。
- 创建、编辑、启用、停用、删除长期数据刷新任务。
- 对已有任务立即执行一次。
- 在有边界的日期范围内发起手动补偿抓取。
- 当用户晚于交易日调度窗口启动软件时，由 Go core 计算需要补偿的 `missed_today` run；例如 A 股 09:30 已开盘，用户 10:00 才启动应用，会补一次今天已经错过且仍有价值的刷新窗口。
- 手动刷新某只股票的行情、K 线、新闻或全部数据，并复用统一执行队列。

使用边界：

- 单次手动补偿最多 30 个 symbol、30 个自然日。
- 非交易日不会生成交易日补偿 run。
- 单股刷新当前在任务调度页、股票详情页和自选股行内提供真实入口。
- Provider 未授权、频率限制或网络错误会导致 run 失败，失败原因以脱敏摘要展示。

## 10. 发布检查

发布前至少运行：

```bash
pnpm --dir apps acceptance:check
pnpm --dir apps sqlite:upgrade-rehearsal
pnpm --dir apps sidecar:check-targets
pnpm --dir apps build
git diff --check
```

`acceptance:check` 会复跑本地测试、类型/编译检查、Rust 桌面单元测试和格式检查；它不会触网，也不会替代真实 Provider smoke、包结构复核或 macOS / Windows 人工验收。

`sqlite:upgrade-rehearsal` 会用临时 SQLite 创建用户数据、执行迁移前备份、恢复备份、再次迁移，并确认已有 settings 数据仍可读取；它用于发布前自动化演练，不替代真实安装包升级已有用户 profile 的手工验收。

本机 Apple Silicon 发布候选可使用聚合命令复跑本地验收、SQLite 升级演练、三类 sidecar target 校验、构建、生成 `.app`、复核包结构并检查 diff 空白：

```bash
pnpm --dir apps release:check:local
```

`release:check:local` 只覆盖当前仓库本机 macOS Apple Silicon 发布候选自动化检查，会校验三类 sidecar target 产物，并对包内 Go core 执行 stdin token 握手、`/internal/health` 和 `/internal/shutdown` smoke；它不触发联网 Provider smoke，也不替代 macOS Intel、Windows 或桌面能力人工验收。

跨平台打包前，可先重复生成并校验三类首版 sidecar target 产物：

```bash
pnpm --dir apps sidecar:check-targets
```

该命令会构建并复核 Apple Silicon、macOS Intel 和 Windows x64 sidecar，校验文件存在性、非 symlink、非空、macOS 执行位、Mach-O / PE 架构，以及 Windows GUI subsystem。它只证明 target sidecar 产物本身符合打包输入要求，不替代 macOS Intel 或 Windows 安装包真实启动验收。

真实 Provider 授权和频率限制确认完成后，可执行外网 smoke：

```bash
pnpm --dir apps provider:smoke -- --allow-network --confirm-provider-terms
```

不带参数的 `pnpm --dir apps provider:smoke` 只输出 dry-run 计划，不访问网络。联网 smoke 只证明样例端点当次可访问，不等同于授权、可分发边界或生产可用性已经通过。

生成桌面包后，可先用包结构脚本确认主程序和 Go sidecar 是否随包存在且非空：

```bash
pnpm --dir apps package:verify -- --platform=darwin --target=aarch64-apple-darwin "desktop/src-tauri/target/release/bundle/macos/投研罗盘.app"
pnpm --dir apps package:verify -- --platform=darwin --target=x86_64-apple-darwin "path/to/intel/投研罗盘.app"
pnpm --dir apps package:verify -- --platform=win32 --target=x86_64-pc-windows-msvc "path/to/unpacked/windows/package"
```

macOS 参数必须指向 `.app` 包目录，并会检查包名是否匹配 Tauri `productName`、包根是目录、包根、`Contents` / `Contents/MacOS` 关键目录、声明图标时的 `Contents/Resources` 目录、`Info.plist` 和包内二进制不是 symlink、包内主程序和 Go sidecar 是否具备执行位、`Info.plist` 的 `CFBundleExecutable` 是否指向主程序、`CFBundlePackageType` 是否为 `APPL`、`CFBundleDisplayName` / `CFBundleName` 是否匹配 Tauri `productName`、`CFBundleIdentifier` / `CFBundleShortVersionString` / `CFBundleVersion` 是否与 Tauri 配置一致，以及 `CFBundleIconFile` 声明的图标资源是否为 `Contents/Resources` 直接子文件；Windows 参数指向解包后的安装目录。传入 `--target` 时，脚本还会校验包内主程序和 sidecar 的二进制架构是否匹配目标平台；Windows target 校验还会拒绝 console subsystem 的 PE 文件，避免 sidecar 打包后弹出控制台窗口。该脚本只检查包结构和基础文件有效性，不替代真实目标平台启动、凭据、通知、托盘和开机自启验收。

Apple Silicon `.app` 生成后，也可以单独验证包内 Go core runtime：

```bash
pnpm --dir apps sidecar:smoke
```

该命令会启动包内 `invest-compass-core`，完成 stdin token 握手、健康检查和内部 shutdown；它不打开桌面 UI，也不替代完整桌面验收。

跨平台发布还需要人工确认：

- macOS Apple Silicon 启动、sidecar、本地 vault、通知、托盘和打包产物。
- macOS Intel 启动、sidecar、本地 vault、通知、托盘和打包产物。
- Windows x64 启动、sidecar、本地 vault、通知、托盘和安装包。
- 开机自启已接入设置入口和 Rust 白名单命令，但仍需按平台能力单独验收，未验收前不能写成目标平台已通过。

## 11. 已知未闭环项

以下事项不能在文档或 UI 中写成已完成：

- 真实自动更新下载和安装。
- 授权激活闭环。
- 券商账户、交易、自动下单。
- 云同步和多人协作。
- 移动端。
- 持仓数据独立落库。
- 非首版数据源，如公告、研报、资金流专用数据源。
- 开机自启。
- 调度专题中的真实 Provider 授权、频率限制验收和跨平台休眠恢复 / 退出恢复验收。

## 12. 反馈材料

反馈问题时建议提供：

- 操作系统和 CPU 架构。
- 应用版本。
- 触发问题的操作步骤。
- 脱敏后的日志导出包。
- 是否配置代理。
- 是否使用真实 Provider。

不要在 issue、截图、日志或聊天中发送真实 API Key、Token、代理密码或持仓明细。
