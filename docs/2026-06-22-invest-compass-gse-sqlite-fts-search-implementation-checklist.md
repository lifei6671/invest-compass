# 投研罗盘范围搜索实施清单

> 来源：`docs/2026-06-21-invest-compass-gse-sqlite-fts-search-design.md`
>
> 目标：把 GSE + SQLite FTS5 范围搜索方案拆成可执行、可推进、可标记、可验收的任务清单。本文只描述搜索专题，不把跨菜单全局搜索、公告研报搜索、资金流搜索或任务日志全文搜索提前混入首批实现。

---

## 1. 范围

### 1.1 首批必须交付

- GSE 中文预分词和 SimpleTokenizer 降级实现。
- 拼音生成和多音字 override。
- SQLite FTS5 编译探针、构建 tag 和跨平台 sidecar 校验。
- `stocks` 搜索字段扩展、`stock_aliases`、`stock_pinyin_overrides`。
- `stock_search_fts`、`search_documents`、`search_documents_fts`。
- `search_index_batches`、`search_index_state`、`search_index_jobs`。
- `StockSearchService`：本地强规则 + FTS + Provider fallback + 强排序。
- 保持现有 `stock_search(keyword)` 响应数组契约不变。
- 顶部搜索框默认只搜股票，点击进入个股详情。
- 菜单范围搜索：报告历史、资讯中心、自选备注。
- 报告搜索只使用 `sanitized_search_summary`，不索引 `input_snapshot` 和未脱敏正文。
- 持久化增量索引 outbox，崩溃后可恢复。
- batch 化无损重建，失败保留旧 active batch。
- Go API、Rust 白名单 command、前端 typed service 和页面入口闭环。
- 设置中心索引状态、手动重建和健康检查入口。

### 1.2 首批明确不做

- 不做 `/api/search/global`。
- 不做独立 `/search` 全局搜索页。
- 不做跨菜单、跨业务对象混合召回。
- 不开放前端传入任意 `doc_types` 组合。
- 不开放 FTS5 MATCH 原生语法给前端。
- 不索引 API Key、代理密码、本地 vault、日志、完整 task payload。
- 不索引 `analysis_reports.input_snapshot`。
- 不索引用户一次性持仓原文、成本、仓位、完整 Prompt 或 Provider 原始响应。
- 不接入公告、研报、资金流、基金中心等首版未闭环入口。
- 不用 GSE 替代 SQLite FTS5；GSE 只做应用层预分词。
- 不写 SQLite 自定义 GSE tokenizer。

---

## 2. 状态标记

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成并通过验证
- `[!]` 阻塞或需要人工确认

---

## 3. 依赖路线

```text
GS0 文档边界和依赖确认
  ↓
GS1 依赖、FTS5 构建和探针
  ↓
GS2 Tokenizer、拼音、词典和 QueryBuilder
  ↓
GS3 SQLite schema、model、DAO 和 outbox
  ↓
GS4 股票搜索增强
  ↓
GS5 菜单范围文档索引和搜索
  ↓
GS6 索引重建、状态和设置中心管理
  ↓
GS7 前端接入和页面体验
  ↓
GS8 验收、发布校验和文档同步
```

并行原则：

- GS1 的依赖和 build tag 是上游门禁，未确认前不要改 `go.mod` / `go.sum` / 构建脚本。
- GS3 的 schema、model、DAO 和 Rust/Go API 契约稳定前，不推进前端真实接入。
- `stock_search(keyword)` 是既有契约，任何增强都不能改变响应根结构。
- 报告脱敏、MATCH 构造、Rust command 白名单属于安全关键路径，不能用临时弱实现绕过。
- 菜单范围搜索可以在报告、新闻、自选备注三个 scope 间并行，但不得新增通用全局入口。

---

## 4. Review Gate

### RGGS1 依赖与构建门禁

- GSE / go-pinyin 依赖已明确确认。
- `sqlite_fts5` 或 `fts5` build tag 在测试、sidecar 构建和发布校验中一致。
- 启动探针能区分 FTS5 `AVAILABLE` / `UNAVAILABLE`。
- macOS Apple Silicon、macOS Intel、Windows x64 sidecar 构建链路覆盖 FTS5。
- 不引入 gojieba 默认依赖。
- 不引入 cgo/C++ 额外强依赖。

### RGGS2 Schema 和索引一致性门禁

- `stocks` 扩展字段迁移成功。
- `stock_aliases`、`stock_pinyin_overrides` 创建成功。
- `stock_search_fts` 创建成功。
- `search_documents`、`search_documents_fts` 创建成功。
- `search_index_batches`、`search_index_state`、`search_index_jobs` 创建成功。
- 所有新增索引元数据表遵守 `created_at` / `updated_at` 规则。
- `search_index_jobs` 与业务写入同事务提交。
- `BUILDING` batch 不参与查询。
- `READY` batch 切换失败时保留旧 active batch。
- FTS5 不可用时状态为 `UNAVAILABLE`，迁移不伪装成功。

### RGGS3 股票搜索门禁

- 代码、symbol、简称、全称、别名、拼音全拼、拼音首字母搜索通过。
- 多音字 override 生效。
- `stock_search(keyword)` data 仍是数组。
- 股票搜索不返回新闻、报告或自选备注。
- 本地无命中时 Provider fallback 符合 T13 边界。
- Provider 未配置时不伪造股票数据。
- 强排序稳定，退市 / 暂停上市降权。

### RGGS4 菜单范围搜索门禁

- `search_reports` 只检索 report。
- `search_news` 只检索 news。
- `search_watchlist_notes` 只检索 watchlist_note。
- 不提供 `/api/search/global`。
- 不提供前端任意 `doc_types` 组合。
- 支持 symbol 过滤和分页。
- 软删除后不可见。
- 报告正文只来自 `sanitized_search_summary`。
- `sanitized_search_summary` 脱敏失败时 fail closed，只索引 `title + risk_summary`。
- 搜索测试证明 `userPosition` / `raw_api_key` / `resolved_api_key` 不进入 FTS。
- `search_index_jobs` 崩溃恢复生效。

### RGGS5 API、Rust 和权限门禁

- Go API 全部 POST。
- Go API 复用 token、ready、`httpx.DecodeJSON`、统一响应和 trace/request ID。
- Rust command 固定 path、固定 request schema、固定 response schema。
- 前端不拼 MATCH。
- 前端不直连 Go sidecar。
- 用户输入不能注入 FTS MATCH。
- 错误响应不泄露 SQL、路径、token、凭据或用户持仓明细。
- 顶部搜索框只调用 `stock_search`。
- `search_rebuild` 只能由设置中心索引管理入口触发。
- `desktop/security-config.test.mjs` 覆盖 search command 注册、固定 path 和无通用代理。

### RGGS6 前端体验和性能门禁

- 顶部搜索框默认股票搜索，不出现“全部结果”入口。
- 报告历史、资讯中心、自选股页只展示当前菜单合法搜索范围。
- 索引构建中、FTS5 不可用、Provider 未配置都有明确空 / 错误态。
- 股票搜索 p95 < 80ms。
- 菜单范围搜索 p95 < 250ms。
- 10 万文档重建 < 120 秒。
- 重建期间 UI 可用。
- 重建失败保留旧索引继续服务。
- 清理 `RETIRED` batch 不阻塞当前搜索。

---

## 5. GS0：文档边界和依赖确认

### GS00 固化范围搜索专题实施清单

- 状态：`[x]`
- 依赖：无
- 交付物：
  - `docs/2026-06-22-invest-compass-gse-sqlite-fts-search-implementation-checklist.md`
- 执行动作：
  - 将范围搜索方案拆成阶段、Review Gate、任务项、验证命令和退出条件。
  - 明确不做全局搜索，顶部搜索框默认股票搜索。
  - 明确菜单范围搜索、报告脱敏、outbox 和 batch 重建边界。
- 验证：
  - `git diff --check`
  - 文档没有把未实现能力写成已完成。
- 退出条件：
  - 后续实现可以按任务 ID 独立推进和复盘。

### GS01 同步主 checklist 专题入口

- 状态：`[x]`
- 依赖：GS00
- 交付物：
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
- 执行动作：
  - 在“专题实施清单”章节补充范围搜索专题入口。
  - 引用设计方案和本文。
  - 明确该专题不改变主链路已完成状态。
- 验证：
  - `git diff --check`
  - 总 checklist 不批量展开范围搜索所有子任务。
- 退出条件：
  - 主 checklist 能导航到范围搜索专题清单。

### GS02 确认新增依赖和 build tag

- 状态：`[x]`
- 依赖：GS00
- 交付物：
  - 依赖确认记录。
  - build tag 选择记录。
- 执行动作：
  - 确认是否引入 `github.com/go-ego/gse`。
  - 确认是否引入 `github.com/mozillazg/go-pinyin`。
  - 确认 FTS5 build tag 使用 `sqlite_fts5` 还是 `fts5`。
  - 确认 `go test`、sidecar 构建、release 校验使用同一个 tag。
- 验证：
  - 依赖变更、构建参数变更已获得明确确认。
  - 文档记录确认结果。
- 退出条件：
  - 可以安全进入 `go.mod` / `go.sum` 和构建脚本修改。
- 当前进展：
  - 已采用 `github.com/go-ego/gse`、`github.com/mozillazg/go-pinyin` 和 `sqlite_fts5` build tag。
  - sidecar 测试、SQLite 升级演练和 sidecar 构建脚本已统一使用 `sqlite_fts5`。

---

## 6. GS1：依赖、FTS5 构建和探针

### GS03 引入 GSE 和 go-pinyin 依赖

- 状态：`[x]`
- 依赖：GS02
- 交付物：
  - `apps/sidecar-core/go.mod`
  - `apps/sidecar-core/go.sum`
- 执行动作：
  - 在 `apps/sidecar-core` 执行 `go get github.com/go-ego/gse`。
  - 在 `apps/sidecar-core` 执行 `go get github.com/mozillazg/go-pinyin`。
  - 不引入 gojieba。
  - 不把依赖散落到 actions、dao、Rust 或前端。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./...`
  - `rg "go-ego/gse|mozillazg/go-pinyin|gojieba" apps/sidecar-core`
- 退出条件：
  - 依赖可编译，且只服务于搜索专题。
- 当前进展：
  - `apps/sidecar-core/go.mod` 和 `go.sum` 已包含 GSE、go-pinyin 及 GSE 间接依赖。
  - `go test -tags sqlite_fts5 ./...` 已通过。

### GS04 统一 FTS5 build tag

- 状态：`[x]`
- 依赖：GS02
- 交付物：
  - `apps/package.json`
  - sidecar 构建脚本或 release 校验脚本。
  - `apps/sidecar-core` 测试入口说明。
- 执行动作：
  - 为 Go test 增加 FTS5 build tag。
  - 为 sidecar 本地构建增加同一个 FTS5 build tag。
  - 为 release:check:local 增加 FTS5 二进制探针。
  - 避免测试使用一个 tag、发布使用另一个 tag。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./...`
  - `pnpm --dir apps sidecar:check-targets`
  - `pnpm --dir apps release:check:local`
- 退出条件：
  - 三平台 sidecar 构建链路能稳定启用 FTS5。
- 当前进展：
  - `scripts/build-sidecar.mjs` 已为 sidecar build 增加 `-tags sqlite_fts5`。
  - `scripts/build-sidecar.mjs` 已在 Apple Silicon 本机为 `x86_64-apple-darwin` 目标补充 CGO `-arch x86_64` 参数，避免跨 macOS 架构构建误用 arm64 汇编。
  - `scripts/build-sidecar.mjs` 已为 Windows x64 目标显式选择 `x86_64-w64-mingw32-gcc`，不再误用 Apple clang 构建 Windows CGO sidecar。
  - `apps/package.json` 的 `sidecar:test` 和 `sqlite:upgrade-rehearsal` 已使用同一 tag。
  - `scripts/build-sidecar.test.mjs` 和 `scripts/acceptance-check.test.mjs` 已覆盖该契约。
  - 已通过 `node --test scripts/build-sidecar.test.mjs`。
  - 已通过 `pnpm --dir apps sidecar:build -- --target=x86_64-apple-darwin`。
  - 已通过 `pnpm --dir apps sidecar:check-targets`，确认 `aarch64-apple-darwin`、`x86_64-apple-darwin`、`x86_64-pc-windows-msvc` 三个 sidecar 目标均能构建并通过包体格式校验。

### GS05 实现 FTS5 启动探针

- 状态：`[x]`
- 依赖：GS04
- 交付物：
  - `apps/sidecar-core/internal/dao` 或 `apps/sidecar-core/internal/service/search`
  - 对应单测。
- 执行动作：
  - 启动时执行 `SELECT sqlite_compileoption_used('ENABLE_FTS5')`。
  - 返回 `AVAILABLE` / `UNAVAILABLE` / `UNKNOWN` 状态。
  - FTS5 不可用时不创建伪成功状态。
  - 写入 `search_index_state` 的可观测状态。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao ./internal/service/...`
  - 单测覆盖 FTS5 可用和不可用分支。
- 退出条件：
  - Go core 可以明确判断当前二进制是否支持 FTS5。
- 当前进展：
  - 已新增 `dao.ProbeSQLiteFTS5` 和 `SQLiteFTS5Status`。
  - `TestProbeSQLiteFTS5ReportsAvailable` 已验证当前测试二进制启用 FTS5。
  - 尚未把探针状态写入 `search_index_state` 或启动状态 API。

---

## 7. GS2：Tokenizer、拼音、词典和 QueryBuilder

### GS06 建立 search service 包边界

- 状态：`[x]`
- 依赖：GS03
- 交付物：
  - `apps/sidecar-core/internal/service/search/doc.go`
  - `apps/sidecar-core/internal/service/search/types.go`
- 执行动作：
  - 新建 `internal/service/search`。
  - 用包注释说明该包只承载搜索分词、索引文本构造、排序和查询编排。
  - 明确不得直接注册 HTTP route，不得直接访问 Rust command。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search`
  - `doc.go` 存在且说明职责边界。
- 退出条件：
  - 搜索 service 有清晰落位，不散落到业务 handler。
- 当前进展：
  - 已新增 `internal/service/search` 包和 `doc.go` 包注释。
  - 包边界明确只承载分词、索引文本构造、排序和查询编排，不注册 HTTP route，不直接访问 Rust command。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search`。

### GS07 实现 Tokenizer 接口和 SimpleTokenizer

- 状态：`[x]`
- 依赖：GS06
- 交付物：
  - `apps/sidecar-core/internal/service/search/tokenizer.go`
  - `apps/sidecar-core/internal/service/search/tokenizer_test.go`
- 执行动作：
  - 定义 `Tokenizer` 接口。
  - 实现 `SimpleTokenizer`，支持中文原词、英文数字小写、空白分割。
  - 空输入返回空 token，不报 panic。
  - token 去重并保持稳定顺序。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run Tokenizer`
  - 覆盖中文、英文、数字、混合输入和空输入。
- 退出条件：
  - 即使 GSE 初始化失败，也有可预测降级分词能力。
- 当前进展：
  - 已定义 `Tokenizer` 接口和 `SimpleTokenizer`。
  - 已覆盖中文原词、英文、数字、混合输入、空输入、去重和稳定顺序。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run Tokenizer`。

### GS08 实现 GSETokenizer

- 状态：`[x]`
- 依赖：GS07
- 交付物：
  - `apps/sidecar-core/internal/service/search/gse_tokenizer.go`
  - `apps/sidecar-core/internal/service/search/gse_tokenizer_test.go`
- 执行动作：
  - 用 GSE `CutSearch` 构建搜索引擎模式分词。
  - 支持初始化投研词典。
  - 支持从股票名称、全称、别名动态 AddWord。
  - 初始化失败时返回清晰错误，由上层决定是否降级 SimpleTokenizer。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run GSE`
  - 测试覆盖“贵州茅台”“光模块”“AI服务器”“CPO”等词。
- 退出条件：
  - 中文搜索索引文本可由 GSE 稳定生成。
- 当前进展：
  - 已实现 GSE `CutSearch` 搜索模式分词。
  - 已支持动态 `AddWords`，用于把股票名称、全称、别名和投研领域词加入词典。
  - 已覆盖“贵州茅台 / 光模块 / AI服务器 / CPO”等领域词。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run GSE`。

### GS09 实现拼音生成和多音字 override

- 状态：`[x]`
- 依赖：GS03、GS06
- 交付物：
  - `apps/sidecar-core/internal/service/search/pinyin.go`
  - `apps/sidecar-core/internal/service/search/pinyin_test.go`
- 执行动作：
  - 生成 `pinyin_full`。
  - 生成 `pinyin_initials`。
  - 支持 `stock_pinyin_overrides` 覆盖多音字。
  - 输出统一小写，去掉空格和特殊符号。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run Pinyin`
  - 覆盖“贵州茅台 / gzmt”“重庆啤酒 / cqpj”“长城汽车 / ccqc”“兴业银行 / xyyh”。
- 退出条件：
  - 股票名称、全称、别名都能生成稳定拼音字段。
- 当前进展：
  - 已实现 `BuildPinyinText`，输出 `pinyin_full` 和股票搜索习惯的首字母。
  - 已支持调用方 override，并内置首批常见股票多音字 override。
  - 已覆盖“贵州茅台 / gzmt”“重庆啤酒 / cqpj”“长城汽车 / ccqc”“兴业银行 / xyyh”。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run Pinyin`。

### GS10 实现 QueryNormalizer 和 FTSQueryBuilder

- 状态：`[x]`
- 依赖：GS07
- 交付物：
  - `apps/sidecar-core/internal/service/search/query.go`
  - `apps/sidecar-core/internal/service/search/query_test.go`
- 执行动作：
  - Normalize 用户输入：trim、大小写归一、全角半角归一、空白合并。
  - 识别 code、symbol、exchange_code、pinyin、pinyin_initials、chinese、mixed。
  - FTSQueryBuilder 只允许生成安全 token、`token*`、`column:token*`、`AND` / `OR`。
  - 禁止用户输入直接进入 MATCH 表达式。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run 'Query|Normalize|MATCH'`
  - 覆盖 `茅台 OR 1=1`、`NEAR(茅台,10)`、`symbol:CN:SH:600519`、`"` 等危险输入。
- 退出条件：
  - MATCH 查询构造不接受原始用户语法注入。
- 当前进展：
  - 已实现 `NormalizeQueryInput`，支持 trim、大小写归一、全角半角归一和空白合并。
  - 已识别 code、symbol、exchange_code、pinyin、pinyin_initials、chinese、mixed。
  - 已实现 `BuildFTSMatch`，仅由安全 token、内部允许列、`AND` / `OR` 组合 MATCH。
  - 已覆盖 `茅台 OR 1=1`、`NEAR(茅台,10)`、`symbol:CN:SH:600519`、引号输入等危险语法。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run 'Query|Normalize|MATCH'`。

---

## 8. GS3：SQLite schema、model、DAO 和 outbox

### GS11 扩展 stocks 模型和迁移

- 状态：`[x]`
- 依赖：GS05
- 交付物：
  - `apps/sidecar-core/internal/model/schema.go`
  - `apps/sidecar-core/internal/dao/database.go`
  - `apps/sidecar-core/internal/dao/database_test.go`
- 执行动作：
  - 为 `Stock` 增加 `full_name`、`pinyin_full`、`pinyin_initials`、`search_name`、`search_version`、`indexed_at`。
  - 接入 `dao.Migrate`。
  - 保持已有 `symbol` 唯一约束不变。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TestMigrate`
  - 空库迁移和重复迁移成功。
- 退出条件：
  - `stocks` 能承载股票搜索增强字段。
- 当前进展：
  - `model.Stock` 已新增搜索扩展字段。
  - migration 测试已验证 `stocks` 包含 `full_name`、`pinyin_full`、`pinyin_initials`、`search_name`、`search_version`、`indexed_at`。

### GS12 新增搜索元数据表模型

- 状态：`[x]`
- 依赖：GS11
- 交付物：
  - `apps/sidecar-core/internal/model/search.go`
  - `apps/sidecar-core/internal/dao/database.go`
  - DAO migration 测试。
- 执行动作：
  - 新增 `StockAlias`。
  - 新增 `StockPinyinOverride`。
  - 新增 `SearchDocument`。
  - 新增 `SearchIndexBatch`。
  - 新增 `SearchIndexState`。
  - 新增 `SearchIndexJob`。
  - 所有非 FTS 元数据表包含 `created_at`、`updated_at`。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run TestMigrate`
  - 表、唯一约束和索引创建成功。
- 退出条件：
  - 搜索元数据 schema 可重复迁移。
- 当前进展：
  - 已新增 `StockAlias`、`StockPinyinOverride`、`SearchDocument`、`SearchIndexBatch`、`SearchIndexState`、`SearchIndexJob` 模型。
  - migration 测试已验证普通搜索元数据表创建成功。

### GS13 创建 FTS5 虚表

- 状态：`[x]`
- 依赖：GS05、GS12
- 交付物：
  - `apps/sidecar-core/internal/dao/search_migration.go`
  - `apps/sidecar-core/internal/dao/search_migration_test.go`
- 执行动作：
  - 创建 `stock_search_fts`。
  - 创建 `search_documents_fts`。
  - `stock_search_fts` 包含 `batch_id`。
  - `search_documents_fts` 包含 `batch_id`、`doc_type`、`symbol`。
  - FTS5 不可用时写 `UNAVAILABLE`，不伪装成功。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -run FTS`
  - 临时 SQLite 能创建虚表并插入查询。
- 退出条件：
  - FTS5 表可由 migration 稳定创建和探测。
- 当前进展：
  - `dao.Migrate` 已创建 `stock_search_fts` 和 `search_documents_fts`。
  - FTS5 集成测试已验证 `stock_search_fts` 可插入并 MATCH 查询。
  - FTS5 不可用时的 `UNAVAILABLE` 状态写入仍待接入 `search_index_state`。

### GS14 实现 search DAO

- 状态：`[x]`
- 依赖：GS12、GS13
- 交付物：
  - `apps/sidecar-core/internal/dao/search_repository.go`
  - `apps/sidecar-core/internal/dao/repository_test.go`
- 执行动作：
  - 实现 active batch 读写。
  - 实现 batch 创建、标记 READY / FAILED / RETIRED。
  - 实现 stock FTS delete + insert。
  - 实现 search_documents upsert / soft delete。
  - 实现 search_documents_fts delete + insert。
  - 实现 `search_index_state` get / set。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -run Search`
  - 覆盖 active batch 查询、BUILDING batch 不可见、READY 切换失败保留旧 batch。
- 退出条件：
  - service 层不需要手写散落 SQL 就能访问搜索索引。
- 当前进展：
  - 已实现 `search_index_state` get / set、batch upsert、股票 FTS batch 替换和 batch 内查询。
  - 已实现 `search_documents` upsert / soft delete，以及 `search_documents_fts` 单文档替换和范围查询。
  - 测试已覆盖 active batch 状态、失败重建保留旧 batch、BUILDING batch 不被指定 active batch 查询命中、文档软删除后移除 FTS。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -count=1`。

### GS15 实现 search_index_jobs outbox DAO

- 状态：`[x]`
- 依赖：GS12
- 交付物：
  - `apps/sidecar-core/internal/dao/search_repository.go`
  - `apps/sidecar-core/internal/dao/repository_test.go`
- 执行动作：
  - 实现同事务 upsert job。
  - 实现拉取 PENDING / FAILED_RETRYABLE job。
  - 实现 RUNNING 恢复为可重试。
  - 实现 DONE / FAILED_FINAL 状态更新。
  - last_error 只保存脱敏摘要。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/dao -run SearchIndexJob`
  - 覆盖重复 job 合并、崩溃恢复、失败重试次数。
- 退出条件：
  - 增量索引任务不会因为进程崩溃静默丢失。
- 当前进展：
  - 已实现同一 `doc_type + ref_id + operation` 的 outbox job 合并。
  - 已实现 `PENDING` / `FAILED_RETRYABLE` 拉取、`RUNNING` 恢复为 `FAILED_RETRYABLE`。
  - 已实现 outbox job 状态更新，`last_error` 入库前统一脱敏。
  - 测试已覆盖重复 job 合并、崩溃恢复和终态错误脱敏。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -count=1`。

---

## 9. GS4：股票搜索增强

### GS16 实现股票索引文本构造器

- 状态：`[x]`
- 依赖：GS08、GS09、GS14
- 交付物：
  - `apps/sidecar-core/internal/service/search/stock_indexer.go`
  - `apps/sidecar-core/internal/service/search/stock_indexer_test.go`
- 执行动作：
  - 从 `stocks + stock_aliases + stock_pinyin_overrides` 构造索引文本。
  - 股票简称、全称、别名必须追加原词。
  - 生成 code、code_prefix、name_index、full_name_index、alias_index、pinyin_full、pinyin_initials、industry_index、concept_index。
  - 写入当前 `active_stock_batch_id`。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run StockIndex`
  - 覆盖“贵州茅台 / 茅台 / gzmt / guizhoumaotai / sh600519”。
- 退出条件：
  - 股票 FTS 索引内容完整、可重建、可增量更新。
- 当前进展：
  - 已实现 `BuildStockSearchFTSRow`，从 `stocks + stock_aliases + stock_pinyin_overrides` 构造 DAO 可写入的股票 FTS 行。
  - 已生成 code、code_prefix、name_index、full_name_index、alias_index、pinyin_full、pinyin_initials、industry_index、concept_index。
  - 已覆盖“贵州茅台 / 茅台 / gzmt / guizhoumaotai / sh600519”。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run StockIndex`。

### GS17 实现 StockRanker

- 状态：`[x]`
- 依赖：GS16
- 交付物：
  - `apps/sidecar-core/internal/service/search/stock_ranker.go`
  - `apps/sidecar-core/internal/service/search/stock_ranker_test.go`
- 执行动作：
  - 实现 symbol 精确、code 精确、name 精确、alias 精确、拼音匹配、GSE token、行业概念弱匹配评分。
  - 自选股加权。
  - 正常上市加权，退市 / 暂停上市降权。
  - 排序稳定：score DESC、rank ASC、code ASC。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run StockRanker`
  - 覆盖精确匹配优先于概念弱匹配。
- 退出条件：
  - 股票搜索排序符合业务强规则。
- 当前进展：
  - 已实现 `RankStocks`、`StockCandidate`、`RankedStock`。
  - 已覆盖 symbol/code/name/alias/pinyin/行业概念分层评分、自选股加权、退市降权。
  - 已验证精确匹配优先于概念弱匹配，并按 score DESC、rank ASC、code ASC 稳定排序。
  - 已通过 `cd apps/sidecar-core && go test ./internal/service/search -run StockRanker`。

### GS18 实现 StockSearchService

- 状态：`[x]`
- 依赖：GS10、GS14、GS17
- 交付物：
  - `apps/sidecar-core/internal/service/search/stock_search.go`
  - `apps/sidecar-core/internal/service/search/stock_search_test.go`
- 执行动作：
  - Normalize keyword。
  - 执行本地 stocks 强规则查询。
  - 执行 `stock_search_fts` MATCH 召回。
  - 本地无命中且 Provider 可用时调用 `MarketProvider.Search`。
  - Provider 返回结果写入 `stocks` 并通过 outbox 增量索引。
  - Provider 未配置时不伪造数据。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run StockSearch`
  - 覆盖本地命中、Provider fallback、Provider unavailable、FTS unavailable 降级。
- 退出条件：
  - 股票搜索 service 可替代 handler 直接调用 Provider 的旧路径。
- 当前进展：
  - 已实现 `StockSearchService`，按本地强规则候选、active FTS batch、Provider fallback 顺序搜索。
  - Provider fallback 返回结果会写入 `stocks`，并通过 `search_index_jobs` 追加 `stock/upsert` 增量索引任务。
  - 已实现 DAO 支撑方法 `ListStocksForSearch`、`ListStockAliasesBySymbols`，service 层不手写散落 SQL。
  - 已覆盖本地命中不调用 Provider、FTS active batch 召回、Provider fallback 缓存与 outbox、Provider 未配置不伪造数据。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run StockSearch -count=1`。

### GS19 改造 `/api/stocks/search` handler

- 状态：`[x]`
- 依赖：GS18
- 交付物：
  - `apps/sidecar-core/internal/actions/stocks/stocks.go`
  - `apps/sidecar-core/internal/actions/stocks/stocks_test.go`
- 执行动作：
  - handler 改为调用 `StockSearchService`。
  - 保持请求体 `{ "keyword": "..." }`。
  - 保持响应 data 根结构为数组。
  - 可选字段只能作为 item 扩展，不改变既有字段。
  - 继续复用 token、ready、`httpx.DecodeJSON`。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/stocks`
  - `apps/frontend/src/services/coreClient.test.ts` 中 `stockSearch()` 契约不变。
- 退出条件：
  - 旧前端不需要改响应解析即可继续使用股票搜索。
- 当前进展：
  - `stocks` action 已改为只调用 `StockSearchService`，不再直接调用 `MarketProvider.Search`。
  - 请求体仍为 `{ "keyword": "..." }`，响应 `data` 根结构仍为数组，既有 `symbol/name/code/market/exchange` 字段不变。
  - root router 默认使用 `StockStore + MarketProvider` 构造 `StockSearchService`，也支持测试或特殊场景显式注入 service。
  - 路由测试已验证 Provider fallback 后会写入 stocks 缓存和 `stock/upsert` outbox job。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/stocks ./internal/actions -run 'TestHandleSearch|TestStockSearch' -count=1`。
  - 已通过 `pnpm --dir apps --filter @invest-compass/frontend test -- src/services/coreClient.test.ts`，确认前端 `stockSearch()` typed command 和响应数组契约不变。

### GS20 更新 Rust `stock_search` 边界测试

- 状态：`[x]`
- 依赖：GS19
- 交付物：
  - `apps/desktop/src-tauri/src/commands/market.rs`
  - `apps/desktop/test/security-config.test.mjs`
- 执行动作：
  - 保持 `stock_search(keyword)` 固定映射 `/api/stocks/search`。
  - 继续拒绝空关键词。
  - 不把股票搜索迁到 `search.rs`。
  - 安全测试确认没有新增通用搜索代理。
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
  - `pnpm --dir apps test -- security-config.test.mjs`
- 退出条件：
  - Rust 股票搜索 command 仍是固定白名单路径。
- 当前进展：
  - `apps/desktop/src-tauri/src/commands/market.rs` 已保持 `stock_search(keyword)` 固定映射 `/api/stocks/search`，并继续拒绝空关键词。
  - `apps/desktop/test/security-config.test.mjs` 已包含 `stock_search` 显式声明和 `/api/stocks/search` 固定路径断言。
  - 已通过 `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
  - 已验证 `node --test apps/desktop/test/security-config.test.mjs` 中“股票搜索 Rust command 必须固定映射到 Go API”子测试通过。
  - 已通过完整 `node --test apps/desktop/test/security-config.test.mjs`，确认股票搜索固定白名单路径、菜单范围搜索 command、无通用搜索入口、无生产伪数据等安全门禁均通过。

---

## 10. GS5：菜单范围文档索引和搜索

### GS21 实现文档索引构造器

- 状态：`[x]`
- 依赖：GS08、GS14、GS15
- 交付物：
  - `apps/sidecar-core/internal/service/search/document_indexer.go`
  - `apps/sidecar-core/internal/service/search/document_indexer_test.go`
- 执行动作：
  - 为 news 构造 title、summary、tags、symbol、source_time。
  - 为 report 构造 title、`sanitized_search_summary`、risk_summary、symbol、source_time。
  - 为 watchlist_note 构造 title、note、tags、symbol。
  - 软删除时删除 FTS row 并 soft delete `search_documents`。
  - 所有写入绑定 `active_document_batch_id`。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run DocumentIndex`
  - 覆盖 report/news/watchlist_note 三类索引。
- 退出条件：
  - 菜单范围文档可以被统一索引，但查询入口仍按 scope 固定。
- 当前进展：
  - 已新增 `DocumentIndexer`，通过 `active_document_batch_id` 绑定写入 batch。
  - 已支持 `news`、`report`、`watchlist_note` 三类文档构造和 FTS 行写入。
  - 报告索引输入使用 `ReportSearchSource.SanitizedSearchSummary`，不会读取 `InputSnapshot` 或 `ContentMarkdown`。
  - 删除路径调用 `SoftDeleteSearchDocument`，由 DAO 同步软删除 `search_documents` 并删除 FTS row。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run DocumentIndexer -count=1`。

### GS22 实现报告搜索摘要脱敏

- 状态：`[x]`
- 依赖：GS21
- 交付物：
  - `apps/sidecar-core/internal/service/search/report_sanitizer.go`
  - `apps/sidecar-core/internal/service/search/report_sanitizer_test.go`
- 执行动作：
  - 生成 `sanitized_search_summary`。
  - 复用日志 / 导出链路的脱敏规则。
  - 删除 userPosition、成本、仓位、完整 Prompt、Provider 原始响应、Authorization、Proxy-Authorization、API Key、代理密码。
  - 脱敏失败时 fail closed：只返回空摘要，由索引器只索引 title + risk_summary。
- 验证：
  - `cd apps/sidecar-core && go test ./internal/service/search -run ReportSanitizer`
  - 测试确认敏感字段不会进入输出。
- 退出条件：
  - 报告正文搜索不会泄露用户持仓或凭据上下文。
- 当前进展：
  - 已新增 `BuildReportSearchSummary(report)`，从 `ContentMarkdown` 生成有限长度搜索摘要。
  - 生成过程忽略 `InputSnapshot`，并按 Markdown 段落删除持仓、Prompt、Provider 原始响应和凭据相关内容。
  - 已复用 `logger.RedactText` 做二次脱敏；脱敏后仍检测到敏感片段时返回空摘要，交由索引器只索引 `title + risk_summary`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run ReportSearchSummary -count=1`。

### GS23 实现 ScopedDocumentSearchService

- 状态：`[x]`
- 依赖：GS10、GS14、GS21
- 交付物：
  - `apps/sidecar-core/internal/service/search/document_search.go`
  - `apps/sidecar-core/internal/service/search/document_search_test.go`
- 执行动作：
  - 支持 report、news、watchlist_note 三个固定 scope。
  - 支持 keyword、symbols、limit、offset、sort。
  - 查询必须绑定 `active_document_batch_id`。
  - 查询必须绑定单一 doc_type。
  - 不接受多个 doc_type。
  - 结果包含 doc_uid、doc_type、ref_id、symbol、title、summary、source、source_time、score、highlights。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run DocumentSearch`
  - 覆盖每个 scope 只能返回自己的 doc_type。
- 退出条件：
  - service 层可以支撑三个菜单范围搜索 API。
- 当前进展：
  - 已新增 `ScopedDocumentSearchService`，只暴露 `SearchReports`、`SearchNews`、`SearchWatchlistNotes` 三个固定入口。
  - 搜索绑定 `active_document_batch_id` 和单一内部 `doc_type`，不接收前端传入的任意 `doc_types` 组合。
  - 已支持 keyword 安全 MATCH 构造、symbol 过滤、limit/offset 分页和结果元数据回表。
  - DAO 已新增 `ListSearchDocumentsByUIDs`，按 FTS 命中 doc_uid 顺序回表并排除软删除文档。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run ScopedDocumentSearch -count=1`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -run SearchDocumentRepository -count=1`。

### GS24 实现菜单范围 Go API

- 状态：`[x]`
- 依赖：GS23
- 交付物：
  - `apps/sidecar-core/internal/actions/search/doc.go`
  - `apps/sidecar-core/internal/actions/search/search.go`
  - `apps/sidecar-core/internal/actions/router.go`
  - `apps/sidecar-core/internal/actions/search/search_test.go`
- 执行动作：
  - 新增 `POST /api/search/reports`。
  - 新增 `POST /api/search/news`。
  - 新增 `POST /api/search/watchlist-notes`。
  - 每个路由固定 scope，不从前端读取 doc_types。
  - 复用 token、ready、`httpx.DecodeJSON`。
  - limit 限制 1-100，offset >= 0。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search ./internal/actions`
  - 非 POST 返回 405。
  - 传 doc_types 返回 invalid_json 或 invalid_request。
- 退出条件：
  - Go API 不提供任何全局搜索路径。
- 当前进展：
  - 已新增 `POST /api/search/reports`、`POST /api/search/news`、`POST /api/search/watchlist-notes`。
  - 三个路由通过内部固定方法调用 report/news/watchlist_note 范围，不从请求读取 `doc_types`。
  - 请求复用 `httpx.RequireReadyToken`、`httpx.DecodeJSON` 和统一 envelope；未知字段 `doc_types` 会被拒绝。
  - 已在 root router 注册菜单搜索路由，并验证 `POST /api/search/global` 返回 404。
  - 真实启动配置已注入 `DocumentSearchStore: store`，默认构造 `ScopedDocumentSearchService`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search ./internal/actions -run 'TestSearch|TestDocumentSearchRoutesAreScoped' -count=1`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search ./internal/actions ./internal/service/search ./internal/dao ./cmd/invest-compass-core -count=1`。

---

## 11. GS6：索引重建、状态和设置中心管理

### GS25 实现 SearchRebuildManager

- 状态：`[x]`
- 依赖：GS14、GS21
- 交付物：
  - `apps/sidecar-core/internal/service/search/rebuild.go`
  - `apps/sidecar-core/internal/service/search/rebuild_test.go`
- 执行动作：
  - 创建 `SEARCH_INDEX_REBUILD` task。
  - 创建 rebuild batch。
  - 重建股票、新闻、报告、自选备注索引。
  - 校验 READY 后短事务切换 active batch。
  - 失败时标记 FAILED，保留旧 active batch。
  - 清理 RETIRED batch 不阻塞搜索。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run Rebuild`
  - 覆盖成功切换、失败保留旧 batch、首次无旧 batch。
- 退出条件：
  - 搜索索引可全量重建且不破坏线上可用索引。
- 当前进展：
  - 已新增 `SearchRebuildManager`，同步执行 `all` 范围全量重建；GS26 再接入安全 Go API。
  - 重建会创建 `SEARCH_INDEX_REBUILD` task，创建 stock/document 两个 BUILDING batch。
  - 已支持全量重建股票 FTS，以及 news/report/watchlist_note 文档索引；报告正文只使用 `BuildReportSearchSummary` 的脱敏摘要。
  - DAO 已新增 `ListAllStocksForSearch` 和 `ActivateSearchIndexBatches`，后者在短事务内校验 READY batch、切换 active 指针并退休旧 batch。
  - 失败时会把本次 stock/document batch 标记为 FAILED，不调用 active batch 切换，旧索引继续可用。
  - 已覆盖成功切换、失败保留旧 batch、首次无旧 batch。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run SearchRebuildManager -count=1`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/dao -run 'List.*StocksForRebuild|ActivateSearchIndexBatches|NonReadyActivation' -count=1`。

### GS26 实现搜索状态和重建 Go API

- 状态：`[x]`
- 依赖：GS25
- 交付物：
  - `apps/sidecar-core/internal/actions/search/search.go`
  - `apps/sidecar-core/internal/actions/search/search_test.go`
- 执行动作：
  - 新增 `POST /api/search/status`。
  - 新增 `POST /api/search/rebuild`。
  - rebuild scope 只能是 all、stock、reports、news、watchlist_notes。
  - 同一 scope 同时只能有一个 RUNNING rebuild。
  - task_events 中的错误必须脱敏。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search`
  - 覆盖非法 scope、重复 rebuild、FTS5 unavailable。
- 退出条件：
  - 设置中心能通过安全 API 读取状态和触发重建。
- 当前进展：
  - 已新增 `POST /api/search/status`，返回 FTS5 状态、搜索状态、active stock/document batch 和运行中的重建任务 ID。
  - 已新增 `POST /api/search/rebuild`，支持触发 `all`、`stock`、`reports`、`news`、`watchlist_notes` 范围重建并返回 task/batch 摘要。
  - 已覆盖非法 scope、重复 rebuild、FTS5 unavailable 的 action 错误映射。
  - 默认 `ScopedDocumentSearchService` 已实现 `Status` 和 `Rebuild`；真实启动路径会通过 DAO store 调用 `SearchRebuildManager`。
  - `stock` 范围只重建股票 batch，并复用当前 document batch。
  - `reports`、`news`、`watchlist_notes` 任一文档范围会重建完整 document batch，并复用当前 stock batch，避免切换后丢失其他菜单索引。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search -run 'SearchStatus|SearchRebuild' -count=1`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run 'RebuildsStockScopeOnly|RebuildsDocumentScopesSafely|SearchRebuildManager' -count=1`。
  - 已通过 `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/actions/search ./internal/actions ./internal/service/search ./internal/dao ./cmd/invest-compass-core -count=1`。

### GS27 实现 Rust search commands

- 状态：`[x]`
- 依赖：GS24、GS26
- 交付物：
  - `apps/desktop/src-tauri/src/commands/search.rs`
  - `apps/desktop/src-tauri/src/commands/mod.rs`
  - `apps/desktop/src-tauri/src/lib.rs`
  - `apps/desktop/test/security-config.test.mjs`
- 执行动作：
  - 新增 `search_reports(payload)`。
  - 新增 `search_news(payload)`。
  - 新增 `search_watchlist_notes(payload)`。
  - 新增 `search_status()`。
  - 新增 `search_rebuild(payload)`。
  - 每个 command 固定 Go API path。
  - Rust 边界校验 keyword、limit、offset、symbols、rebuild scope。
  - 不提供 `search_global` 或通用 path 代理。
- 验证：
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
  - `pnpm --dir apps test -- security-config.test.mjs`
- 退出条件：
  - 前端只能通过固定 command 调用菜单范围搜索和索引管理。
- 当前进展：
  - 已新增 `commands/search.rs`，提供 `search_reports`、`search_news`、`search_watchlist_notes`、`search_status`、`search_rebuild` 五个白名单 command。
  - 每个 command 固定映射到 `/api/search/reports`、`/api/search/news`、`/api/search/watchlist-notes`、`/api/search/status`、`/api/search/rebuild`，未提供 `search_global` 或通用 `search_request`。
  - Rust 边界已校验 `keyword`、`limit`、`offset`、`symbols` 和 rebuild `scope`。
  - 已注册到 `invoke_handler`，并在 `security-config.test.mjs` 增加菜单范围搜索 command 固定 path 安全断言。
  - 已通过 `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
  - 已通过 `node --test --test-name-pattern "菜单范围搜索 Rust command" apps/desktop/test/security-config.test.mjs`。
  - 已通过完整 `node --test apps/desktop/test/security-config.test.mjs`，确认前端只能通过固定 command 调用菜单范围搜索和索引管理，且不存在 `search_global` 或任意路径代理。

### GS28 接入设置中心索引管理

- 状态：`[x]`
- 依赖：GS27
- 交付物：
  - `apps/frontend/src/pages/settings/...`
  - `apps/frontend/src/services/search.ts`
  - 前端测试。
- 执行动作：
  - 设置中心新增搜索索引状态展示。
  - 展示股票、报告、新闻、自选备注索引数量。
  - 展示最后重建时间、词典版本、GSE 状态、FTS5 状态。
  - 提供健康检查和重建按钮。
  - `search_rebuild` 只从设置中心索引管理入口触发。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
  - 前端测试覆盖重建按钮、loading、error、success。
- 退出条件：
  - 用户可以看到索引健康状态并手动触发重建。
- 当前进展：
  - 已新增 `searchStatus()`、`searchRebuild(payload)` typed invoke service，只调用固定 Rust command `search_status` 和 `search_rebuild`。
  - Go `/api/search/status` 已通过 `SearchIndexOverview` 返回 active batch 的股票、报告、新闻、自选备注索引数量、最后重建时间、分词器版本、词典版本、GSE 状态和 FTS5 状态；统计来源为现有 `stock_search_fts`、`search_documents`、`search_index_batches`，未新增 schema。
  - 基础设置页已新增“搜索索引”卡片，展示 `FTS5`、`GSE`、`search_status`、四类索引数量、最后重建时间、分词器、词典版本、active stock/document batch 和运行中的重建任务 ID。
  - 已提供 `all`、`stock`、`reports`、`news`、`watchlist_notes` 五个固定 scope 重建按钮，不提供全局搜索或任意 doc_types 输入。
  - 已补充 `coreClient.test.ts`、`SettingsBasicPage.test.tsx`、App 设置页集成测试、DAO 概览测试和 search service 状态测试。
  - 已通过 `go test -tags sqlite_fts5 ./internal/dao ./internal/service/search`。
  - 已通过 `pnpm --dir apps/frontend exec vitest run src/pages/settings/basic/SettingsBasicPage.test.tsx src/app/App.test.tsx --reporter=basic`。
  - 已通过 `pnpm --dir apps/frontend test`。
  - 已通过 `pnpm --dir apps/frontend check`。

---

## 12. GS7：前端接入和页面体验

### GS29 增强顶部股票搜索框

- 状态：`[x]`
- 依赖：GS19、GS20
- 交付物：
  - `apps/frontend/src/app` 或现有 TopBar 相关文件。
  - `apps/frontend/src/services/coreClient.ts`
  - 前端测试。
- 执行动作：
  - 顶部搜索框默认调用 `stockSearch(keyword)`。
  - 结果只展示股票。
  - 点击进入 `/stocks/:symbol`。
  - 不展示新闻、报告、自选备注、任务日志。
  - 不提供“查看全部搜索结果”入口。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
  - 测试覆盖顶部搜索只调用 `stock_search`。
- 退出条件：
  - 顶部搜索语义和用户要求一致。
- 当前进展：
  - 当前 TopBar 已默认调用 `stockSearch(keyword)`，仅触发 Rust command `stock_search`。
  - 搜索成功后只取股票结果并进入 `/stocks/:symbol`。
  - 未展示新闻、报告、自选备注、任务日志结果，也未提供“查看全部搜索结果”入口。
  - 已加强 App 测试，断言顶部搜索不会调用 `search_reports`、`search_news`、`search_watchlist_notes`、`search_global`。
  - 已通过 `pnpm --dir apps/frontend exec vitest run src/app/App.test.tsx -t "顶部搜索"`。

### GS30 接入报告历史菜单范围搜索

- 状态：`[x]`
- 依赖：GS24、GS27
- 交付物：
  - `apps/frontend/src/pages/reports/...`
  - `apps/frontend/src/services/search.ts`
  - 前端测试。
- 执行动作：
  - 报告历史页搜索调用 `searchReports`。
  - 支持 keyword、symbol、limit、offset。
  - 空状态说明“仅搜索报告历史”。
  - 点击结果进入报告详情。
  - 不显示新闻或自选备注结果。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
- 退出条件：
  - 报告历史页可以按当前菜单范围搜索报告。
- 当前进展：
  - 已新增 `searchReports(payload)` typed invoke service，固定调用 Rust command `search_reports`，不接受任意 `doc_type`。
  - 报告历史页点击“查询报告”时调用 `searchReports`，传入 `keyword`、`symbols`、`limit`、`offset`、`sort`。
  - 搜索结果只接收并展示 `doc_type = report` 的结果，点击标题继续进入 `/reports/:reportId`。
  - 已补充 `coreClient.test.ts`、`ReportHistoryPage.test.tsx` 和 App 报告历史集成测试，覆盖不调用 `search_news`、`search_watchlist_notes`、`search_global`。
  - 已通过 `pnpm --dir apps/frontend test`。
  - 已通过 `pnpm --dir apps/frontend check`。
  - 已通过 `git diff --check`。

### GS31 接入资讯中心菜单范围搜索

- 状态：`[x]`
- 依赖：GS24、GS27
- 交付物：
  - `apps/frontend/src/pages/news/...`
  - `apps/frontend/src/services/coreClient.ts`
  - 前端测试。
- 执行动作：
  - 资讯中心搜索调用 `searchNews`。
  - 只展示新闻结果。
  - 支持 symbol 过滤。
  - 点击结果进入新闻详情或通过外链白名单打开系统浏览器。
  - 空状态说明“仅搜索资讯中心”。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
- 退出条件：
  - 资讯中心搜索不越界召回报告或股票详情。
- 当前进展：
  - 已新增 `searchNews(payload)` typed invoke service，固定调用 Rust command `search_news`，不接受任意 `doc_type`。
  - 资讯中心页点击“刷新资讯”时按当前菜单范围调用 `searchNews`，传入 `keyword`、`symbols`、`limit`、`offset`、`sort`。
  - 搜索结果只接收并展示 `doc_type = news` 的结果；`ref_id` 为 HTTPS 外链时，通过固定 `open_external_url` command 打开系统浏览器。
  - 搜索无结果时展示“仅搜索资讯中心，暂无匹配资讯”。
  - 已补充 `coreClient.test.ts`、`NewsCenterPage.test.tsx` 和 App 资讯中心集成测试，覆盖不调用 `search_reports`、`search_watchlist_notes`、`search_global`。
  - 已通过 `pnpm --dir apps/frontend test`。
  - 已通过 `pnpm --dir apps/frontend check`。
  - 已通过 `git diff --check`。

### GS32 接入自选股备注范围搜索

- 状态：`[x]`
- 依赖：GS24、GS27
- 交付物：
  - `apps/frontend/src/components/watchlist/...`
  - `apps/frontend/src/services/coreClient.ts`
  - 前端测试。
- 执行动作：
  - 自选股页保留股票搜索用于添加自选。
  - 自选备注 / 标签搜索调用 `searchWatchlistNotes`。
  - 结果点击定位到自选股行或个股详情。
  - 空状态说明“仅搜索自选备注和标签”。
- 验证：
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
- 退出条件：
  - 自选股搜索和自选备注搜索语义清晰，不混成全局搜索。
- 当前进展：
  - 已新增 `searchWatchlistNotes(payload)` typed invoke service，固定调用 Rust command `search_watchlist_notes`，不接受任意 `doc_type`。
  - 自选股页保留添加自选弹窗里的股票搜索语义；主搜索框输入时仍保留本地筛选，按回车后调用 `searchWatchlistNotes` 搜索自选备注和标签。
  - 搜索结果只接收并展示 `doc_type = watchlist_note` 的结果，点击“查看”继续进入对应个股详情。
  - 搜索无结果时展示“仅搜索自选备注和标签，暂无匹配自选项”。
  - 已补充 `coreClient.test.ts` 和 `WatchlistPage.test.tsx`，覆盖不调用 `search_reports`、`search_news`、`search_global`。
  - 已通过 `pnpm --dir apps/frontend test`。
  - 已通过 `pnpm --dir apps/frontend check`。
  - 已通过 `git diff --check`。

---

## 13. GS8：验收、发布校验和文档同步

### GS33 补齐搜索质量回归集

- 状态：`[x]`
- 依赖：GS19、GS24
- 交付物：
  - `apps/sidecar-core/internal/service/search/search_quality_test.go`
  - 前端或 shared 契约测试。
- 执行动作：
  - 覆盖 `600519`、`sh600519`、`CN:SH:600519`、`贵州茅台`、`茅台`、`gzmt`。
  - 覆盖 `宁王`、`ningdeshidai`、`ndsd`。
  - 覆盖 `重庆啤酒`、`chongqingpijiu`、`cqpj`。
  - 覆盖 `光模块`、`CPO`、`AI服务器` 在不同 scope 下分别断言。
  - 不用一个全局搜索 case 同时断言三类结果。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./internal/service/search -run Quality`
  - `pnpm --dir apps test`
- 退出条件：
  - 搜索质量有稳定回归保护。
- 当前进展：
  - 已新增 `apps/sidecar-core/internal/service/search/search_quality_test.go`。
  - 已覆盖股票搜索输入形态：`600519`、`sh600519`、`CN:SH:600519`、`贵州茅台`、`茅台`、`gzmt`、`宁王`、`ningdeshidai`、`ndsd`、`重庆啤酒`、`chongqingpijiu`、`cqpj`。
  - 已覆盖 `光模块`、`CPO`、`AI服务器` 在 report、news、watchlist_note 三个菜单范围下分别断言，不使用全局搜索 case 混合断言。
  - 已覆盖 FTS 用户语法降级和 DAO 混合命中时的 doc_type 隔离。
  - 已通过 `go test ./internal/service/search`。
  - 已通过 `go test -tags sqlite_fts5 ./internal/service/search -run Quality`。
  - 前端在 GS32 后已通过 `pnpm --dir apps/frontend test` 和 `pnpm --dir apps/frontend check`，GS33 未改动前端运行时代码。

### GS34 执行专题总体验收

- 状态：`[x]`
- 依赖：GS03-GS33
- 交付物：
  - 验收记录。
  - 必要的文档同步。
- 执行动作：
  - 运行 Go、Rust、前端、release 本地校验。
  - 检查没有 `/api/search/global`、`search_global`、跨菜单搜索页。
  - 检查报告 FTS 中没有敏感字段。
  - 检查重建失败保留旧 active batch。
  - 检查 FTS5 unavailable 降级路径。
- 验证：
  - `cd apps/sidecar-core && go test -tags sqlite_fts5 ./...`
  - `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`
  - `pnpm --dir apps test`
  - `pnpm --dir apps check`
  - `pnpm --dir apps sidecar:check-targets`
  - `pnpm --dir apps release:check:local`
  - `git diff --check`
- 退出条件：
  - RGGS1-RGGS6 全部通过，未完成能力仍保持停止线。
- 当前进展：
  - 已通过 `go test -tags sqlite_fts5 ./...`。
  - 已通过 `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`。
  - 已通过 `pnpm --dir apps test`，包含 desktop 安全扫描、frontend、shared、scripts 和 Go sidecar 测试。
  - 已通过 `pnpm --dir apps check`。
  - 已通过 `pnpm --dir apps sidecar:check-targets`，确认 macOS arm64、macOS x86_64、Windows x64 sidecar 目标均构建并通过格式校验。
  - 已通过 `pnpm --dir apps release:check:local`，覆盖 acceptance、SQLite 升级演练、多目标 sidecar、前端 build、Tauri app bundle、包体校验、sidecar runtime smoke 和最终 `git diff --check`。
  - 已通过 `git diff --check`。
  - 已检查 `search_global`、`/api/search/global`、`SearchGlobal`、`searchGlobal`：命中仅存在于文档和测试断言，不存在实现入口。
  - 已检查搜索索引相关目录中的 `resolved_api_key`、`api_key`、`proxy_password`、`Authorization`、`Proxy-Authorization`：命中集中在 sanitizer 实现和测试，不是报告 FTS 持久化字段。
  - 已修复 macOS Intel 目标跨架构 CGO 参数，并将 Windows 目标 CGO 编译器环境固定在构建脚本中；最终本机多目标 sidecar 和 release 本地验收均已通过。
  - 2026-06-22 复验通过：`go test -tags sqlite_fts5 ./...`、`pnpm --dir apps test`、`pnpm --dir apps check`、`cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`、`pnpm --dir apps release:check:local`。

### GS35 同步设计方案、主 checklist 和用户文档

- 状态：`[x]`
- 依赖：GS34
- 交付物：
  - `docs/2026-06-21-invest-compass-gse-sqlite-fts-search-design.md`
  - `docs/2026-06-17-invest-compass-implementation-checklist.md`
  - 用户手册或发布说明中与搜索相关的段落。
- 执行动作：
  - 将实现结果同步回设计方案。
  - 只勾选已经验证通过的任务。
  - 用户文档说明顶部搜索只搜股票，菜单搜索只搜当前菜单范围。
  - 记录 FTS5 不可用时的降级提示。
- 验证：
  - `git diff --check`
  - 文档没有把未实现能力写成已完成。
- 退出条件：
  - 代码行为、设计方案、实施清单和用户说明一致。
- 当前进展：
  - 已在设计方案中补齐 `/api/search/status` 响应字段、字段来源和 GSE fallback 推导规则。
  - 已在主 checklist 中记录范围搜索专题 GS00-GS35 已完成，并保留不做全局搜索、独立搜索页和任意 `doc_types` 的停止线。
  - 已在发布用户指南中新增“搜索范围和索引状态”，说明顶部搜索只搜股票、菜单搜索只搜当前范围，以及 FTS5 不可用时的降级/不可用提示。
  - 已通过 `pnpm --dir apps release:check:local` 中的最终 `git diff --check`。
