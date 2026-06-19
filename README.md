# 投研罗盘 Invest Compass

投研罗盘是一款面向个人投资者和技术型研究者的本地 AI 投研桌面工作台。

首版目标是完成一个可独立运行、可配置 AI 模型、可查看股票行情和 K 线、可生成个股 AI 分析报告、可保存报告和任务历史的桌面应用。所有分析输出仅作研究辅助，不构成投资建议。

---

## 首版 MVP 范围

必须交付：

- Tauri v2 桌面壳、React + TypeScript + Vite 前端、Go sidecar core。
- SQLite 本地库，使用 `GORM` 访问。
- Go core 按 `server` / `actions` / `service` / `dao` / `model` / `pkg/constant` / `pkg/xerr` 分层组织。
- Rust command 白名单代理，前端不得直接访问 Go sidecar。
- Go sidecar 只监听 `127.0.0.1`，runtime token 通过 stdin 握手传递。
- 股票搜索、自选股、行情、K 线、技术指标、新闻资讯。
- OpenAI-compatible AI Provider、模型配置、Prompt 模板、模型连通性测试。
- API Key 和代理密码保存到 Rust 管理的本地文件 vault，SQLite 只保存引用标识和脱敏状态。
- 个股 AI 分析任务、任务事件、报告保存、报告历史、任务历史。
- 设置中心覆盖工作区、代理、通知、缓存、开机自启和检查更新入口。

明确不做：

- 不接券商账户，不做实盘交易、自动下单或收益承诺。
- 不做云端同步、多人协作、移动端。
- 不做策略观察独立页面。
- 不做公告、研报、资金流专用数据源。
- 不做持仓信息落库。
- 不做授权激活闭环。
- 不做真实自动更新下载和安装。

---

## 当前仓库状态

当前仓库处于首版实现推进阶段。任务状态以实施清单为准，不要从 README 推断阶段完成度。

已存在：

- 技术方案：`docs/2026-06-17-invest-compass-technical-solution.md`
- 实施清单：`docs/2026-06-17-invest-compass-implementation-checklist.md`
- 首版发布与使用指南：`docs/2026-06-18-invest-compass-release-user-guide.md`
- 首版总体验收报告：`docs/2026-06-18-invest-compass-acceptance-report.md`
- Agent 开发规则：`AGENTS.md`
- pnpm workspace：`apps/package.json`、`apps/pnpm-workspace.yaml`
- 前端应用：`apps/frontend/`
- 桌面壳：`apps/desktop/`
- Go sidecar：`apps/sidecar-core/`
- 共享契约目录：`apps/packages/shared/`
- 自动化脚本目录：`scripts/`

---

## 本地开发入口

docs-only 修改至少运行：

```bash
git diff --check
```

工程骨架相关修改优先运行：

```bash
cd apps && pnpm install
cd apps && pnpm build
cd apps && pnpm test
cd apps && pnpm check
cd apps/sidecar-core && go test ./...
cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml
git diff --check
```

具体推进顺序以实施清单的 `T00-T44` 和 `RG1-RG6` 为准。

---

## 合规边界

Invest Compass 不是自动交易软件，也不是荐股软件。行情、新闻、技术指标和 AI 输出均用于研究辅助，不能替代用户独立判断。任何 UI、Prompt、报告和文档都必须保留风险提示和非投资建议声明。

面向内测用户的安装、配置、使用、风险声明和发布检查见：

```text
docs/2026-06-18-invest-compass-release-user-guide.md
```
