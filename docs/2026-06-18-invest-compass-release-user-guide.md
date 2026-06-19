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
- 设置中心基础能力，包括工作区、代理、缓存、通知、开机自启和检查更新入口。

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

## 7. 发布检查

发布前至少运行：

```bash
pnpm --dir apps format:check
pnpm --dir apps check
pnpm --dir apps test
pnpm --dir apps build
(cd apps/sidecar-core && go test ./...)
cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
git diff --check
```

跨平台发布还需要人工确认：

- macOS Apple Silicon 启动、sidecar、本地 vault、通知、托盘和打包产物。
- macOS Intel 启动、sidecar、本地 vault、通知、托盘和打包产物。
- Windows x64 启动、sidecar、本地 vault、通知、托盘和安装包。

## 8. 已知未闭环项

以下事项不能在文档或 UI 中写成已完成：

- 真实自动更新下载和安装。
- 授权激活闭环。
- 券商账户、交易、自动下单。
- 云同步和多人协作。
- 移动端。
- 持仓数据独立落库。
- 非首版数据源，如公告、研报、资金流专用数据源。

## 9. 反馈材料

反馈问题时建议提供：

- 操作系统和 CPU 架构。
- 应用版本。
- 触发问题的操作步骤。
- 脱敏后的日志导出包。
- 是否配置代理。
- 是否使用真实 Provider。

不要在 issue、截图、日志或聊天中发送真实 API Key、Token、代理密码或持仓明细。
