# 投研罗盘范围搜索技术方案：GSE + SQLite FTS5

## 1. 评审结论

本方案建议将“jieba + SQLite FTS5”调整为：

```text
GSE 预分词 + SQLite FTS5 + 股票专用搜索索引 + 拼音字段独立索引
```

核心结论：

```text
1. GSE 可以替代 gojieba，且更适合 Go sidecar 桌面项目。
2. GSE 只负责中文预分词，不直接替代 SQLite FTS5。
3. SQLite FTS5 继续负责倒排索引、MATCH 查询、prefix 查询、BM25 排序。
4. 股票搜索必须单独设计，不能只依赖普通全文检索。
5. 拼音、股票代码、股票简称、全称、别名要独立建模和排序。
6. 不做跨菜单、跨业务对象混合召回的全局搜索。
7. 顶部搜索框默认就是股票搜索，点击结果进入个股详情。
8. 其他菜单内的搜索必须由当前菜单精确限定范围。
```

原因：当前项目技术栈是 Tauri v2 + Rust 桌面壳 + Go sidecar + SQLite + GORM，且前端必须通过 Rust 白名单 command 访问 Go core，不允许直接访问 sidecar 或 SQLite。搜索能力必须保持这一架构边界，并且不能通过一个“全局搜索”入口把股票、新闻、报告、任务日志等不同业务对象混在一起召回。

SQLite FTS5 官方支持 `unicode61`、`ascii`、`porter`、`trigram` 等 tokenizer，默认 tokenizer 是 `unicode61`；它不内置 GSE 或 jieba 中文分词器，因此本方案采用“应用层 GSE 预分词后写入 FTS5”的方式。([SQLite官网][1])

GSE 本身支持普通模式、搜索引擎模式、全模式、精确模式、HMM、用户词典、停用词、词性标注、多语言等能力，适合承担 Go sidecar 内的中文搜索预分词职责。([Go Packages][2])

---

# 2. 目标与非目标

## 2.1 目标

本期搜索目标分为两类。

### A. 股票搜索增强

必须支持：

```text
股票代码：600519、000001、300750
交易所 + 代码：sh600519、sz300750
标准 symbol：CN:SH:600519
股票简称：贵州茅台、宁德时代
公司全称：贵州茅台酒股份有限公司
股票别名：茅台、宁王
拼音全拼：guizhoumaotai
拼音首字母：gzmt
行业 / 概念弱匹配：光模块、CPO、HBM、PCB、AI服务器
```

### B. 菜单 / 页面内范围搜索

本期不提供独立全局搜索页，也不提供跨业务对象混合召回 API。需要搜索时，由当前菜单或页面精确决定搜索范围：

```text
顶部搜索框：股票搜索，默认走 stock_search(keyword)
自选股菜单：股票搜索 + 当前自选股备注 / 标签搜索
报告历史菜单：仅搜索 AI 分析报告标题 / 正文摘要 / 风险摘要
资讯菜单：仅搜索新闻标题 / 摘要
任务历史菜单：首版不接全文索引，只保留现有任务筛选；后续如接入也只能搜索任务日志摘要，不索引完整 payload
```

后续可扩展：

```text
公告
研报
F10 / 基本面数据
资金流
基金
Prompt 模板
任务日志摘要
```

扩展前提：

```text
1. 必须先有对应菜单或页面。
2. 必须使用固定 Rust command 限定 scope。
3. Go API 不能接受前端传入任意 doc_type 组合来模拟全局搜索。
4. 每个 scope 都要有独立验收：数据来源、脱敏、软删除、跳转目标。
```

## 2.2 非目标

本期不做：

```text
不写 SQLite 自定义 GSE tokenizer
不索引 API Key、代理密码、本地 vault
不索引完整日志
不索引 task_events.payload 全量内容
不索引 analysis_reports.input_snapshot
不索引用户一次性持仓输入
不开放 FTS5 MATCH 原生语法给前端
不让前端直接访问 SQLite 或 Go sidecar
不做独立全局搜索页
不提供 `/api/search/global`
不提供跨 scope 的搜索建议、最近搜索、搜索热词
不接入公告、研报、资金流等首版未闭环入口
```

`analysis_reports.input_snapshot` 可能包含用户一次性持仓输入，当前文档已经要求默认报告查询、复制、导出不包含完整 `input_snapshot`。搜索索引也必须延续这个边界，不索引该字段。

如果 `analysis_reports.content_markdown` 中回显了用户一次性持仓、成本、仓位、API 返回原文或其他敏感上下文，报告范围搜索在入索引前必须先生成脱敏摘要或使用白名单摘要字段，不能把完整正文直接写入 FTS。

---

# 3. 总体架构

```text
React 前端
  ↓ typed invoke service
Tauri Rust command
  ↓ 固定白名单 command
Go actions/stocks 或 actions/search
  ↓
service/search
  ├── QueryNormalizer
  ├── Tokenizer 接口
  │     ├── GSETokenizer       默认实现
  │     └── SimpleTokenizer    降级实现
  ├── PinyinGenerator
  ├── StockSearchService
  ├── ScopedDocumentSearchService
  ├── SearchIndexer
  ├── SearchRebuildManager
  └── SearchRanker
        ↓
dao.Store
        ↓
SQLite
  ├── stocks
  ├── stock_aliases
  ├── stock_pinyin_overrides
  ├── stock_search_fts
  ├── search_documents
  ├── search_documents_fts
  └── search_index_state
```

设计原则：

```text
1. 业务真相仍在 stocks / news_items / analysis_reports / watchlists。
2. FTS 表只是可重建索引，不作为业务真相来源。
3. GSE 只存在于 service/search 内部，不散落到业务模块。
4. 所有搜索 API 只供 Rust 白名单 command 调用。
5. 搜索相关 SQL 必须集中在 dao/search.go。
6. 前端只调用 typed invoke service，不拼接 MATCH 表达式。
7. 除股票搜索外，所有文档搜索必须由菜单 scope 固定范围。
8. 顶部搜索框不做全局检索，默认只调用股票搜索。
```

---

# 4. 技术选型

## 4.1 中文分词：GSE

新增依赖，实施前必须按项目规则确认：

```bash
go get github.com/go-ego/gse
```

使用方式：

```go
seg.CutSearch(text, true)
```

`CutSearch` 是搜索引擎模式，适合构建 FTS5 索引文本。GSE 文档明确列出 `CutSearch(str string, hmm ...bool)` 为 search engine mode。([Go Packages][3])

## 4.2 拼音：go-pinyin

新增依赖，实施前必须按项目规则确认：

```bash
go get github.com/mozillazg/go-pinyin
```

`go-pinyin` 支持普通拼音、首字母、声母、多音字等模式，可用于生成股票拼音字段。([Go Packages][4])

## 4.3 全文索引：SQLite FTS5

FTS5 用于：

```text
MATCH 查询
prefix 查询
BM25 排序
snippet / highlight，后续可选
```

FTS5 支持 prefix index；本方案用 `prefix = '1 2 3 4 5 6'` 支撑股票代码、拼音、首字母前缀查询。FTS5 官方也提供 `bm25()` 排序函数，且可以给不同列传入权重。([SQLite官网][1])

当前 Go sidecar 通过 `gorm.io/driver/sqlite` 间接使用 `github.com/mattn/go-sqlite3`。FTS5 不是只靠运行时检查即可保证的能力，构建链路必须显式带上 `sqlite_fts5` 或 `fts5` build tag。

必须补齐：

```text
1. apps/sidecar-core 的本地 go test 命令带 FTS5 build tag。
2. sidecar:check-targets 覆盖 macOS Apple Silicon、macOS Intel、Windows x64。
3. release:check:local 能验证实际 sidecar 二进制包含 FTS5。
4. 启动时执行 SELECT sqlite_compileoption_used('ENABLE_FTS5')。
5. 如果 FTS5 不可用，股票搜索退回 code/name/pinyin LIKE，菜单范围文档搜索禁用并提示。
```

建议统一命令入口：

```bash
go test -tags sqlite_fts5 ./...
```

如果后续构建脚本选择 `fts5` 而不是 `sqlite_fts5`，必须在 sidecar 构建、测试和发布脚本中保持同一个 tag，不能本地测试和发布二进制使用不同编译选项。

## 4.4 依赖确认点

本方案涉及新增 Go 依赖和构建参数变化，属于项目规则中的 Ask First 范围。

实施前需要确认：

```text
操作类型：新增 Go 依赖 + 修改 sidecar 构建 tag
影响范围：apps/sidecar-core/go.mod、go.sum、sidecar 构建脚本、发布校验脚本
风险评估：依赖体积、跨平台编译、SQLite FTS5 是否随二进制稳定启用
```

---

# 5. 模块设计

## 5.1 search service 包结构

新增目录：

```text
apps/sidecar-core/internal/service/search/
├── doc.go
├── types.go
├── normalizer.go
├── tokenizer.go
├── tokenizer_gse.go
├── tokenizer_simple.go
├── pinyin.go
├── dictionary.go
├── stock_indexer.go
├── document_indexer.go
├── query_builder.go
├── ranker.go
├── service.go
├── rebuild.go
└── *_test.go
```

新增 actions：

```text
apps/sidecar-core/internal/actions/search/
├── doc.go
├── search.go
└── search_test.go
```

新增 dao：

```text
apps/sidecar-core/internal/dao/search.go
apps/sidecar-core/internal/dao/search_test.go
```

新增 Rust command：

```text
apps/desktop/src-tauri/src/commands/search.rs
```

新增前端 service：

```text
apps/frontend/src/services/search.ts
apps/frontend/src/services/search.test.ts
```

注意：

```text
1. 现有 stock_search(keyword) 仍属于 market.rs 的股票搜索 command。
2. search.rs 只承载菜单范围文档搜索、索引状态和重建能力。
3. 不新增 search_global / globalSearch。
4. 不新增可由前端自由传 doc_types 的通用搜索 command。
```

## 5.2 Tokenizer 接口

```go
package search

type Tokenizer interface {
    Name() string
    SegmentForIndex(text string) []string
    SegmentForQuery(text string) []string
    AddWord(word string, weight int) error
    Close() error
}
```

默认实现：

```go
type GSETokenizer struct {
    seg *gse.Segmenter
}
```

降级实现：

```go
type SimpleTokenizer struct {}
```

降级实现只处理：

```text
英文
数字
股票代码
按 rune 切中文 bigram，可选
```

降级触发条件：

```text
GSE 初始化失败
词典文件损坏
开发测试环境禁用 GSE
```

---

# 6. 数据库设计

## 6.1 扩展 stocks 表

当前 `stocks` 表已有 `symbol`、`market`、`code`、`name`、`pinyin`、`exchange`、`industry`、`concept` 等字段。

新增字段：

```sql
ALTER TABLE stocks ADD COLUMN full_name TEXT DEFAULT '';
ALTER TABLE stocks ADD COLUMN pinyin_full TEXT DEFAULT '';
ALTER TABLE stocks ADD COLUMN pinyin_initials TEXT DEFAULT '';
ALTER TABLE stocks ADD COLUMN search_name TEXT DEFAULT '';
ALTER TABLE stocks ADD COLUMN search_version INTEGER DEFAULT 0;
ALTER TABLE stocks ADD COLUMN indexed_at DATETIME;
```

字段说明：

```text
full_name：公司全称
pinyin_full：股票名称全拼，例如 guizhoumaotai
pinyin_initials：首字母，例如 gzmt
search_name：简称、全称、别名聚合展示字段
search_version：搜索索引版本
indexed_at：最后索引时间
```

## 6.2 新增 stock_aliases

```sql
CREATE TABLE stock_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    alias TEXT NOT NULL,
    alias_type TEXT NOT NULL DEFAULT 'manual',
    pinyin_full TEXT DEFAULT '',
    pinyin_initials TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME,
    UNIQUE(symbol, alias)
);

CREATE INDEX idx_stock_aliases_symbol_active
ON stock_aliases(symbol)
WHERE deleted_at IS NULL;
```

示例：

```text
CN:SH:600519 -> 茅台
CN:SZ:300750 -> 宁王
CN:SZ:300502 -> 易盛，可选
```

## 6.3 新增 stock_pinyin_overrides

```sql
CREATE TABLE stock_pinyin_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL UNIQUE,
    pinyin_full TEXT NOT NULL,
    pinyin_initials TEXT NOT NULL,
    reason TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

用于解决多音字：

```text
重庆啤酒 -> chongqingpijiu / cqpj
长城汽车 -> changchengqiche / ccqc
兴业银行 -> xingyeyinhang / xyyh
```

## 6.4 新增 stock_search_fts

```sql
CREATE VIRTUAL TABLE stock_search_fts USING fts5(
    batch_id UNINDEXED,
    symbol UNINDEXED,
    market UNINDEXED,
    exchange UNINDEXED,
    code,
    code_prefix,
    name_index,
    full_name_index,
    alias_index,
    pinyin_full,
    pinyin_initials,
    industry_index,
    concept_index,
    tokenize = 'unicode61 remove_diacritics 2 tokenchars ''._-:''',
    prefix = '1 2 3 4 5 6'
);
```

字段说明：

```text
batch_id：当前可用索引批次，查询必须绑定 search_index_state.active_stock_batch_id
symbol：CN:SH:600519，不参与全文排序，仅返回
market：CN，不参与全文排序
exchange：SH/SZ，不参与全文排序
code：600519
code_prefix：sh600519 sz600519 cnsh600519
name_index：GSE 分词后的股票简称 + 原词
full_name_index：GSE 分词后的公司全称 + 原词
alias_index：别名分词 + 原词
pinyin_full：guizhoumaotai gui zhou mao tai
pinyin_initials：gzmt g z m t
industry_index：行业词
concept_index：概念词
```

## 6.5 新增 search_documents

```sql
CREATE TABLE search_documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT NOT NULL,
    doc_uid TEXT NOT NULL,
    doc_type TEXT NOT NULL,
    ref_table TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    symbol TEXT,
    title TEXT NOT NULL,
    summary TEXT DEFAULT '',
    source TEXT DEFAULT '',
    source_time DATETIME,
    indexed_at DATETIME NOT NULL,
    index_version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME,
    UNIQUE(batch_id, doc_uid)
);

CREATE INDEX idx_search_documents_doc_type
ON search_documents(batch_id, doc_type);

CREATE INDEX idx_search_documents_symbol
ON search_documents(batch_id, symbol);

CREATE INDEX idx_search_documents_ref
ON search_documents(batch_id, ref_table, ref_id);

CREATE INDEX idx_search_documents_source_time
ON search_documents(batch_id, source_time);
```

`search_documents` 是菜单范围文档索引的元数据表，不是全局搜索结果表。

查询时必须附带固定 scope：

```text
report_history -> doc_type = report
news_center -> doc_type = news
watchlist -> doc_type = watchlist_note
```

禁止一个请求同时传入多个 doc_type 来模拟跨菜单全局搜索。

`doc_uid` 规则：

```text
stock:CN:SH:600519
news:<news_items.id>
report:<analysis_reports.id>
watchlist_note:<watchlists.id>
prompt:<prompt_templates.id>      后续 Prompt 模板菜单需要搜索时再启用
```

## 6.6 新增 search_documents_fts

```sql
CREATE VIRTUAL TABLE search_documents_fts USING fts5(
    batch_id UNINDEXED,
    doc_uid UNINDEXED,
    doc_type UNINDEXED,
    symbol UNINDEXED,
    title_index,
    body_index,
    tag_index,
    pinyin_index,
    tokenize = 'unicode61 remove_diacritics 2 tokenchars ''._-:/#''',
    prefix = '2 3 4 5'
);
```

该 FTS 表可复用同一套物理结构，但查询入口必须是菜单范围 API。`doc_type` 只是后端强约束字段，不能暴露成前端可自由组合的全局过滤器。

查询必须同时绑定：

```text
batch_id = active_document_batch_id
doc_type = 固定菜单 scope 对应类型
```

`batch_id` 是无损重建边界。重建时写入新 batch，校验成功后只切换 `search_index_state.active_document_batch_id`，不得先清空当前可用 batch。

## 6.7 新增 search_index_batches

```sql
CREATE TABLE search_index_batches (
    batch_id TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    status TEXT NOT NULL,
    source_schema_version INTEGER NOT NULL DEFAULT 1,
    tokenizer_name TEXT NOT NULL,
    tokenizer_version TEXT NOT NULL,
    dictionary_hash TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    error_message TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_search_index_batches_scope_status
ON search_index_batches(scope, status);
```

`scope` 只能取：

```text
stock
reports
news
watchlist_notes
all
```

`status` 只能取：

```text
BUILDING
READY
FAILED
RETIRED
```

`error_message` 必须写入脱敏后的错误摘要，不得包含 SQL 原文、文件路径、token、凭据或用户持仓明细。

## 6.8 新增 search_index_state

```sql
CREATE TABLE search_index_state (
    key TEXT PRIMARY KEY,
    value TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

状态 key：

```text
schema_version
tokenizer_name
tokenizer_version
dictionary_hash
last_full_rebuild_at
last_rebuild_status
last_rebuild_error
active_stock_batch_id
active_document_batch_id
stock_count
document_count
```

`active_stock_batch_id` 和 `active_document_batch_id` 是查询期唯一可信的当前索引批次。增量索引和搜索查询都必须读取 active batch，避免重建中的半成品进入用户可见结果。

## 6.9 新增 search_index_jobs

```sql
CREATE TABLE search_index_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_type TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    operation TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE(doc_type, ref_id, operation)
);

CREATE INDEX idx_search_index_jobs_status
ON search_index_jobs(status, updated_at);
```

`search_index_jobs` 是增量索引 outbox。业务表写入成功时，必须在同一个数据库事务内写入或合并对应 job；后台 worker 只消费 `PENDING` / `FAILED_RETRYABLE` job。禁止仅在事务提交后写内存队列，否则进程崩溃会丢失索引任务。

字段约束：

```text
operation：upsert / delete
status：PENDING / RUNNING / DONE / FAILED_RETRYABLE / FAILED_FINAL
last_error：仅保存脱敏摘要
```

---

# 7. 索引内容范围

## 7.1 股票索引

来源：

```text
stocks
stock_aliases
stock_pinyin_overrides
```

索引字段：

```text
code
code_prefix
name_index
full_name_index
alias_index
pinyin_full
pinyin_initials
industry_index
concept_index
```

构造示例：

```text
symbol: CN:SH:600519
code: 600519
code_prefix: sh600519 cnsh600519
name_index: 贵州 茅台 贵州茅台
full_name_index: 贵州 茅台 酒 股份 有限公司 贵州茅台酒股份有限公司
alias_index: 茅台
pinyin_full: guizhoumaotai gui zhou mao tai
pinyin_initials: gzmt g z m t
industry_index: 白酒 消费
concept_index: 高端白酒 MSCI
```

注意：股票简称、全称、别名必须追加原词，不能只放 GSE 分词结果。

## 7.2 新闻索引

当前 `news_items` 表已有 `title`、`summary`、`content_hash`、`symbols`、`tags`、`published_at` 等字段。

索引字段：

```text
title_index：title 的 GSE 搜索模式分词 + 原标题
body_index：summary 的 GSE 搜索模式分词
tag_index：tags
symbol：关联 symbol
source_time：published_at
```

不索引：

```text
URL 中的 token 参数
未清洗 HTML
外部网页全文
```

## 7.3 AI 报告索引

来源：

```text
analysis_reports.title
analysis_reports.content_markdown 脱敏摘要或白名单片段
analysis_reports.risk_summary
analysis_reports.symbol
analysis_reports.created_at
```

索引字段：

```text
title_index：报告标题
body_index：脱敏后的 content_markdown 摘要 + risk_summary
symbol：报告关联股票
source_time：created_at
```

不索引：

```text
input_snapshot
userPosition
api_key_ref
resolved_api_key
raw_api_key
```

报告索引的强制规则：

```text
1. 不读取 input_snapshot。
2. 不索引用户一次性持仓原文。
3. 不索引模型请求原文、Provider 原始响应和内部调试上下文。
4. content_markdown 入索引前必须经过敏感信息脱敏。
5. 如果无法证明正文已脱敏，只能索引 title + risk_summary + 结构化摘要。
```

## 7.4 自选股备注索引

来源：

```text
watchlists.tags
watchlists.note
```

索引字段：

```text
title_index：股票名称 + symbol
body_index：note
tag_index：tags
```

---

# 8. 分词、拼音、词典策略

## 8.1 QueryNormalizer

统一归一化：

```text
去首尾空白
全角转半角
大小写归一
中文标点转空格
多个空白合并
移除不可见字符
保留中文、英文、数字、下划线、短横线、冒号、点号
```

示例：

```text
" SH 600519 "       -> "sh600519"
"CN：SH：600519"    -> "cn:sh:600519"
" 贵州  茅台 "       -> "贵州 茅台"
"g z m t"           -> "gzmt"
```

## 8.2 GSE 分词策略

索引时：

```go
func SegmentForIndex(text string) []string {
    normalized := NormalizeText(text)
    tokens := seg.CutSearch(normalized, true)
    tokens = append(tokens, normalized)
    return NormalizeTokens(tokens)
}
```

查询时：

```go
func SegmentForQuery(text string) []string {
    normalized := NormalizeText(text)

    if IsCodeLike(normalized) || IsSymbolLike(normalized) || IsPinyinLike(normalized) {
        return []string{normalized}
    }

    tokens := seg.CutSearch(normalized, true)
    tokens = append(tokens, normalized)
    return NormalizeTokens(tokens)
}
```

## 8.3 投研词典

必须维护内置词典：

```text
股票简称
股票全称
股票别名
行业名称
概念名称
投研高频词
AI 硬件词
半导体词
算力词
宏观词
```

首批词典例子：

```text
贵州茅台
宁德时代
中际旭创
新易盛
天孚通信
寒武纪
胜宏科技
生益科技
雅克科技
泰晶科技
光模块
CPO
800G
1.6T
HBM
AI服务器
液冷
PCB
覆铜板
半导体设备
先进封装
Chiplet
算力
国产替代
```

词典加载规则：

```text
1. 启动时加载内置词典。
2. 股票基础数据刷新后动态 AddWord。
3. 用户新增别名后动态 AddWord。
4. dictionary_hash 变化后标记需要重建索引。
```

## 8.4 拼音生成策略

对股票简称、全称、别名生成：

```text
pinyin_full：guizhoumaotai
pinyin_tokens：gui zhou mao tai
pinyin_initials：gzmt
pinyin_initial_tokens：g z m t
```

入库文本：

```text
pinyin_full = "guizhoumaotai gui zhou mao tai"
pinyin_initials = "gzmt g z m t"
```

拼音优先级：

```text
1. stock_pinyin_overrides
2. go-pinyin 默认转换
3. 多音字 fallback
```

---

# 9. 查询设计

## 9.1 股票搜索 API

保留现有入口：

```http
POST /api/stocks/search
```

当前兼容请求：

```json
{
  "keyword": "gzmt"
}
```

兼容响应：

```json
[
  {
    "symbol": "CN:SH:600519",
    "code": "600519",
    "name": "贵州茅台",
    "market": "CN",
    "exchange": "SH"
  }
]
```

兼容策略：

```text
1. `stock_search(keyword)` 的 data 必须继续是数组，不能改成 `{ items: [] }`。
2. symbol/name/code/market/exchange 字段必须保持稳定。
3. full_name、industry、match_type、score、highlight 等字段只能作为 item 的可选扩展字段。
4. 如需返回 `{ items, has_more }`，必须新增 command 或 endpoint，不能静默改现有契约。
```

## 9.2 股票搜索流程

```text
1. Normalize keyword
2. 判断 keyword 类型
3. 执行强规则精确匹配
4. 执行本地 stocks 强规则查询
5. 执行 stock_search_fts MATCH 召回
6. 本地无命中且 MarketProvider 可用时，调用 Provider 搜索
7. Provider 返回结果写入 stocks 并增量索引
8. 合并去重
9. 计算业务 score
10. 按 score DESC、rank ASC 排序
11. 返回 limit 条
```

冷启动和 Provider 边界：

```text
1. 新库或索引未完成时，本地索引可能没有股票基础数据。
2. 本地无命中时允许走 MarketProvider.Search，但必须保持 T13 授权和可用性边界。
3. Provider 未配置时不得伪造股票，只能返回本地缓存命中；无缓存命中则返回空结果或稳定不可用错误。
4. Provider 调用失败时错误必须脱敏，不能泄露外部请求 URL 中的敏感参数。
5. 搜索成功后仍按标准 symbol 幂等写入 stocks 缓存。
```

keyword 类型：

```text
code_exact：600519
code_prefix：6005
symbol：CN:SH:600519
exchange_code：sh600519
pinyin：guizhoumaotai
pinyin_initials：gzmt
chinese：贵州茅台
mixed：茅台600519
concept：光模块
```

## 9.3 股票排序规则

建议分值：

```text
symbol 精确匹配：1200
code 精确匹配：1100
exchange + code 精确匹配：1050
name 精确匹配：1000
full_name 精确匹配：950
alias 精确匹配：900
pinyin_initials 精确匹配：850
pinyin_full 精确匹配：820
code 前缀匹配：750
name 前缀匹配：700
pinyin 前缀匹配：650
GSE token 命中：500
行业 / 概念命中：100-200
```

最终：

```text
final_score = business_score + fts_rank_score + market_boost + active_boost
```

`active_boost`：

```text
用户自选股 +50
正常上市 +20
退市 / 暂停上市 -200
```

## 9.4 菜单范围搜索 API

不新增 `/api/search/global`。

允许新增的 API 必须由菜单范围固定：

```http
POST /api/search/reports
POST /api/search/news
POST /api/search/watchlist-notes
```

报告历史搜索请求：

```json
{
  "keyword": "光模块 算力",
  "symbols": ["CN:SZ:300502"],
  "limit": 20,
  "offset": 0,
  "sort": "relevance"
}
```

响应：

```json
{
  "items": [
    {
      "doc_uid": "report:123",
      "doc_type": "report",
      "ref_id": "123",
      "symbol": "CN:SZ:300502",
      "title": "新易盛 投研分析",
      "summary": "报告中提到光模块、800G、AI 算力需求...",
      "source": "analysis_report",
      "source_time": "2026-06-21T10:00:00Z",
      "score": 0.93,
      "highlights": {
        "title": "新易盛 投研分析",
        "summary": "...光模块..."
      }
    }
  ],
  "has_more": true
}
```

固定范围规则：

```text
1. `/api/search/reports` 只能查询 doc_type = report。
2. `/api/search/news` 只能查询 doc_type = news。
3. `/api/search/watchlist-notes` 只能查询 doc_type = watchlist_note。
4. Go handler 可以复用内部实现，但 scope 必须由路由固定，不能由前端传入任意 doc_types。
5. 搜索结果点击目标必须和菜单一致：report -> 报告详情，news -> 新闻详情或外链，watchlist_note -> 自选股页定位。
```

## 9.5 顶部搜索框

顶部搜索框的交互语义：

```text
1. 默认就是股票搜索。
2. 只调用 `stock_search(keyword)`。
3. 输入代码、symbol、简称、别名、拼音时返回股票候选。
4. 点击股票候选进入 `/stocks/:symbol`。
5. 不召回新闻、报告、自选备注、任务日志。
6. 不展示“全部结果”或跨菜单搜索页入口。
```

如果后续需要顶部搜索框显示候选建议，也必须复用股票搜索结果，不新增跨 scope 的 `search_suggest`。


---

# 10. FTS MATCH 安全设计

用户输入禁止直接拼到 MATCH。

禁止：

```go
sql := "WHERE search_documents_fts MATCH '" + keyword + "'"
```

必须：

```go
matchExpr := queryBuilder.Build(tokens)
rows, err := db.QueryContext(ctx, sqlText, matchExpr, limit)
```

`QueryBuilder` 只允许生成：

```text
token*
column:token*
token AND token
token OR token
```

禁止用户输入直接使用：

```text
NEAR
NOT
OR
AND
()
:
^
*
"
```

处理规则：

```text
1. 用户原始 keyword 先 Normalize。
2. GSE / 拼音 / 代码规则生成安全 token。
3. 每个 token 只允许安全字符集。
4. 由 QueryBuilder 生成 MATCH 表达式。
5. SQL 使用参数绑定。
```

危险输入回归：

```text
"茅台 OR 1=1"
"NEAR(茅台,10)"
"茅台*"
"symbol:CN:SH:600519"
"\""
"贵州\n茅台"
```

---

# 11. 索引写入流程

## 11.1 统一索引器

新增：

```go
type SearchIndexer interface {
    IndexStock(ctx context.Context, symbol string) error
    IndexNews(ctx context.Context, id int64) error
    IndexReport(ctx context.Context, id int64) error
    IndexWatchlistNote(ctx context.Context, id int64) error
    DeleteDocument(ctx context.Context, docUID string) error
    Rebuild(ctx context.Context, req RebuildRequest) error
}
```

## 11.2 写入后增量索引

```text
业务写入成功
  ↓
同一 dao.Store transaction 内 upsert search_index_jobs
  ↓
dao.Store transaction commit
  ↓
后台轻量 worker 拉取 PENDING / FAILED_RETRYABLE job
  ↓
读取源表
  ↓
生成索引文本
  ↓
upsert search_documents
  ↓
delete + insert FTS row
  ↓
标记 search_index_jobs DONE
```

增量索引必须使用 `search_index_jobs` 作为持久化 outbox：

```text
1. 业务写入和 job 写入必须在同一个事务内完成。
2. 同一 doc_type/ref_id/operation 重复触发时合并为一个 PENDING job。
3. 进程重启后扫描 RUNNING / FAILED_RETRYABLE job 并恢复处理。
4. job 失败只记录脱敏后的 last_error。
5. worker 写索引时必须绑定当前 active batch。
```

禁止做法：

```text
业务事务提交后只写内存队列。
业务事务提交后异步 best-effort Enqueue 但没有持久化补偿。
索引失败后吞掉错误且不留下可恢复 job。
```

## 11.3 股票更新索引

触发时机：

```text
股票搜索 Provider 返回新股票
股票基础数据导入完成
用户新增 / 修改别名
拼音 override 修改
行业 / 概念更新
搜索索引版本变化
```

流程：

```text
1. 读取 active_stock_batch_id。
2. 读取 stocks + aliases + overrides。
3. 生成 code/name/full_name/alias/pinyin/industry/concept。
4. DELETE FROM stock_search_fts WHERE batch_id = ? AND symbol = ?。
5. INSERT INTO stock_search_fts(batch_id, ...) VALUES(...)。
6. 更新 stocks.indexed_at / search_version。
```

## 11.4 新闻更新索引

触发时机：

```text
news_items 新增
news_items 更新
news_items 软删除
```

流程：

```text
1. 读取 news_items。
2. title / summary 走 GSE CutSearch。
3. tags 轻分词。
4. 读取 active_document_batch_id。
5. 写 search_documents。
6. 写 search_documents_fts。
```

## 11.5 报告更新索引

触发时机：

```text
AI 报告保存成功
报告删除
报告重新生成
```

流程：

```text
1. 读取 analysis_reports。
2. 不读取 input_snapshot。
3. 生成 sanitized_search_summary。
4. title / sanitized_search_summary / risk_summary 入索引。
5. 读取 active_document_batch_id。
6. 写 search_documents。
7. 写 search_documents_fts。
```

`sanitized_search_summary` 是报告搜索唯一允许使用的正文来源：

```text
1. 来源可以是 content_markdown 的脱敏摘要或报告保存时生成的白名单结构化摘要。
2. 必须复用日志 / 导出链路的敏感信息脱敏规则。
3. 必须删除用户一次性持仓原文、成本、仓位、完整 Prompt、Provider 原始响应、API Key、代理密码和内部调试上下文。
4. 如果无法证明摘要已脱敏，必须 fail closed：只索引 title + risk_summary，不索引正文。
5. 单测必须覆盖包含 userPosition / raw_api_key / resolved_api_key / Authorization / Proxy-Authorization 的报告内容不会进入 FTS。
```

报告删除：

```text
1. analysis_reports soft delete。
2. search_documents soft delete。
3. 在 active_document_batch_id 内删除 search_documents_fts row。
```

---

# 12. 重建索引设计

## 12.1 触发入口

```text
首次安装
数据库升级
search schema version 变化
GSE 词典 hash 变化
股票基础库重新导入
用户手动点击“重建搜索索引”
索引健康检查失败
```

## 12.2 重建 API

新增：

```http
POST /api/search/rebuild
POST /api/search/status
```

`/api/search/rebuild` 请求：

```json
{
  "scope": "all",
  "force": false
}
```

`scope` 只能取：

```text
all
stock
reports
news
watchlist_notes
```

该接口不接受 `doc_types` 数组。

返回：

```json
{
  "task_id": "task_search_rebuild_xxx"
}
```

`/api/search/status` 响应必须来自 active batch 和现有索引表，不允许前端伪造：

```json
{
  "fts5_status": "AVAILABLE",
  "gse_status": "FALLBACK",
  "search_status": "READY",
  "active_stock_batch_id": "stock-ready-1",
  "active_document_batch_id": "document-ready-1",
  "running_rebuild_task_id": "",
  "stock_index_count": 5000,
  "report_index_count": 20,
  "news_index_count": 1000,
  "watchlist_note_index_count": 12,
  "last_rebuild_at": "2026-06-22T13:30:00Z",
  "tokenizer_name": "simple",
  "tokenizer_version": "1",
  "dictionary_hash": "builtin"
}
```

字段来源：

```text
stock_index_count -> stock_search_fts 当前 active stock batch
report/news/watchlist_note_index_count -> search_documents 当前 active document batch
last_rebuild_at / tokenizer / dictionary -> search_index_batches 当前 active batch
gse_status -> 根据 active batch tokenizer_name 推导，simple 表示 FALLBACK
```

## 12.3 重建流程

```text
1. 创建 SEARCH_INDEX_REBUILD 任务。
2. 写 TASK_CREATED。
3. 创建 rebuild batch_id。
4. search_index_batches 写入 BUILDING。
5. 批量重建股票索引。
6. 批量重建新闻索引。
7. 批量重建报告索引。
8. 批量重建自选股备注索引。
9. 校验新索引完整性。
10. 在短事务内切换 active_stock_batch_id / active_document_batch_id。
11. 将旧 batch 标记 RETIRED。
12. 执行 FTS optimize。
13. 更新 search_index_state。
14. 写 TASK_SUCCESS。
```

FTS5 支持 `optimize` 和 `integrity-check` 等维护命令，重建后建议执行 optimize，设置中心可以提供索引健康检查。([SQLite官网][1])

批次写入规则：

```text
1. 重建期间所有新索引行都写入 rebuild batch_id。
2. 搜索查询继续读取 active batch，不读取 BUILDING batch。
3. 校验失败时将 rebuild batch 标记 FAILED，并保留旧 active batch。
4. active batch 切换必须在一个短事务内完成，只更新 search_index_state 和 batch 状态。
5. 清理 RETIRED batch 必须异步执行，清理失败不影响当前搜索。
```

重建安全规则：

```text
1. 不能先清空线上可用索引再开始慢速重建。
2. 如果已有可用索引，重建失败必须保留旧索引继续服务。
3. 如果是首次安装且没有旧索引，搜索状态显示 NEED_REBUILD / BUILDING。
4. 重建错误写入 task_events 前必须脱敏。
5. 用户再次点击重建时，同一 scope 只能有一个 RUNNING 重建任务。
6. 重建任务不得读取 analysis_reports.input_snapshot。
7. 报告重建无法生成脱敏摘要时，必须只索引 title + risk_summary。
```

## 12.4 是否接入 scheduler

不建议把普通索引任务放到交易日 scheduler。已有 scheduler 的职责是行情、K 线、新闻刷新、启动补偿、手动补偿和单股刷新，并通过 ExecutionQueue 管理数据任务。搜索重建属于本地索引维护任务，不应该混入交易日数据刷新队列。

建议：

```text
增量索引：SearchIndexer 内部轻量队列
全量重建：复用 task / task_events 长任务系统
不接入 scheduler_jobs
```

---

# 13. Go API 与 Rust command

## 13.1 新增 Go API

```text
POST /api/search/reports
POST /api/search/news
POST /api/search/watchlist-notes
POST /api/search/rebuild
POST /api/search/status
```

股票搜索继续使用：

```text
POST /api/stocks/search
```

但底层改为：

```text
StockSearchService -> 本地 stocks 强规则 + stock_search_fts + Provider fallback + 强规则排序
```

## 13.2 新增 Rust command

```text
search_reports(payload)
search_news(payload)
search_watchlist_notes(payload)
search_rebuild(payload)
search_status()
```

固定映射：

```text
search_reports(payload)         -> POST /api/search/reports
search_news(payload)            -> POST /api/search/news
search_watchlist_notes(payload) -> POST /api/search/watchlist-notes
search_rebuild(payload)         -> POST /api/search/rebuild
search_status()                 -> POST /api/search/status
```

Rust 校验：

```text
keyword trim 后不能为空
limit 必须 1-100
offset 必须 >= 0
symbols 必须符合标准 symbol 格式
rebuild scope 只能是 all / stock / reports / news / watchlist_notes
不能接收 doc_types 数组
```

权限边界：

```text
1. search_reports / search_news / search_watchlist_notes 只允许主窗口通过对应菜单页面调用。
2. search_status 允许主窗口和设置中心读取。
3. search_rebuild 会启动 SEARCH_INDEX_REBUILD 后台任务，只能由设置中心索引管理入口触发。
4. capabilities/default.json 不新增文件系统、shell、网络或插件权限。
5. desktop security-config.test.mjs 必须覆盖 search_* command 已注册、固定 path、无通用代理，以及 search_rebuild 不能暴露为任意 path / 任意 scope。
6. 如果后续新增多窗口，search_rebuild 必须继续限制在 settings/main 能力范围，远程 URL 或非受信窗口不得调用。
```

## 13.3 前端 typed service

```ts
export async function stockSearch(
  keyword: string
): Promise<StockSearchResult[]>

export async function searchReports(
  payload: ScopedSearchRequest
): Promise<ScopedSearchResponse>

export async function searchNews(
  payload: ScopedSearchRequest
): Promise<ScopedSearchResponse>

export async function searchWatchlistNotes(
  payload: ScopedSearchRequest
): Promise<ScopedSearchResponse>

export async function searchStatus(): Promise<SearchStatusResponse>

export async function rebuildSearchIndex(
  payload: RebuildSearchIndexRequest
): Promise<RebuildSearchIndexResponse>
```

前端禁止：

```text
拼接 MATCH
保存 Go core port/token
访问 127.0.0.1
直接访问 SQLite
保存敏感搜索上下文
传入任意 doc_types 组合
```

---

# 14. 前端页面设计

## 14.1 顶部股票搜索框

位置：

```text
AppShell 顶部搜索框
```

语义：

```text
1. 默认搜索股票。
2. 调用 stockSearch(keyword)。
3. 结果只展示股票。
4. 点击结果进入 /stocks/:symbol。
5. 不展示新闻、报告、自选备注和任务日志。
6. 不提供“查看全部搜索结果”入口。
```

展示字段：

```text
股票名称
代码
市场 / 交易所
行业，可选
匹配类型，可选
是否已加入自选，可选
```

## 14.2 股票搜索弹窗增强

位置：

```text
自选股页
AI 分析页股票选择器
```

展示字段：

```text
股票名称
代码
市场 / 交易所
行业
匹配类型
是否已加入自选
```

匹配类型：

```text
代码匹配
简称匹配
全称匹配
别名匹配
拼音匹配
概念匹配
```

## 14.3 菜单范围搜索

不新增 `/search` 全局搜索页。

各菜单搜索范围：

```text
报告历史页：searchReports，只搜索 report
资讯中心页：searchNews，只搜索 news
自选股页：stockSearch + searchWatchlistNotes
任务历史页：首版保留结构化筛选，不接全文搜索
设置中心：只展示搜索索引状态，不搜索业务数据
```

页面交互：

```text
搜索框位于当前菜单页面内
筛选项只展示当前菜单合法字段
结果点击只跳转当前菜单支持的目标
空状态必须说明当前搜索范围
```

禁止：

```text
跨菜单混合结果
跨菜单“全部”过滤
最近搜索跨菜单共享
搜索热词跨菜单共享
前端传 doc_types 自由组合
```

结果点击：

```text
news -> 打开新闻详情或外链
report -> /reports/:id
watchlist_note -> /watchlist?symbol=...
```

## 14.4 设置中心新增索引管理

位置：

```text
设置中心 -> 数据与缓存 -> 搜索索引
```

显示：

```text
索引状态
股票索引数量
报告索引数量
新闻索引数量
自选备注索引数量
最后重建时间
词典版本
GSE tokenizer 状态
重建按钮
健康检查按钮
```

---

# 15. 性能目标

## 15.1 数据规模假设

首版：

```text
A 股股票：5000-10000
新闻：10 万以内
报告：1 万以内
自选备注：1000 以内
```

## 15.2 目标指标

```text
股票搜索：
p50 < 30ms
p95 < 80ms

菜单范围搜索：
p50 < 80ms
p95 < 250ms

索引重建：
10 万文档 < 120 秒
重建过程中 UI 不假死
增量索引单条 < 50ms
```

## 15.3 SQLite 配置

启动时检查：

```sql
SELECT sqlite_compileoption_used('ENABLE_FTS5');
```

建议 PRAGMA：

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA temp_store = MEMORY;
```

FTS 重建后：

```sql
INSERT INTO stock_search_fts(stock_search_fts) VALUES('optimize');
INSERT INTO search_documents_fts(search_documents_fts) VALUES('optimize');
```

---

# 16. 迁移方案

## 16.1 Migration 步骤

```text
1. 迁移前备份 SQLite。
2. 扩展 stocks 字段。
3. 创建 stock_aliases。
4. 创建 stock_pinyin_overrides。
5. 创建 stock_search_fts。
6. 创建 search_documents。
7. 创建 search_documents_fts。
8. 创建 search_index_batches。
9. 创建 search_index_state。
10. 创建 search_index_jobs。
11. 写入 search schema_version。
12. 标记 search_status = NEED_REBUILD。
```

当前项目已经要求 Go sidecar 生产启动时在 `dao.Open` 和 `dao.Migrate` 前执行迁移前备份。搜索索引新增表也必须遵守该流程。

## 16.2 首次启动策略

```text
首次启动不阻塞进入主界面。
后台启动 SEARCH_INDEX_REBUILD。
股票搜索在索引未完成前只基于已缓存 stocks 降级为 code/name LIKE；本地无命中时仍按 Provider fallback 边界处理。
菜单范围搜索在索引未完成前展示“索引构建中”。
SEARCH_INDEX_REBUILD 成功前不得把 BUILDING batch 暴露给搜索查询。
```

## 16.3 回滚策略

如果 FTS5 不可用：

```text
search_status = UNAVAILABLE
股票搜索降级为 code/name/pinyin LIKE
菜单范围搜索禁用并提示当前 SQLite 不支持 FTS5
设置页显示修复建议
search_index_jobs 保留，待 FTS5 恢复后重新消费或触发全量重建
```

---

# 17. 测试方案

## 17.1 Go 单元测试

覆盖：

```text
QueryNormalizer
GSETokenizer
SimpleTokenizer
PinyinGenerator
PinyinOverride
DictionaryLoader
StockIndexBuilder
DocumentIndexBuilder
FTSQueryBuilder
StockRanker
ScopedSearchRanker
危险 MATCH 输入转义
软删除后索引不可见
索引重建幂等
search_index_jobs 崩溃恢复
active batch 切换失败时保留旧索引
报告 sanitized_search_summary 脱敏失败时 fail closed
```

## 17.2 SQLite 集成测试

使用临时 SQLite：

```text
创建 FTS5 表
插入股票索引
插入新闻索引
插入报告索引
执行 code/name/pinyin/MATCH 查询
验证排序
验证 doc_type 过滤
验证 symbol 过滤
验证 delete + insert 更新
验证 search_index_batches 只切换 READY batch
验证 BUILDING batch 不参与查询
验证 search_index_jobs 与业务写入同事务提交
验证 optimize 不报错
```

## 17.3 搜索质量回归集

必须覆盖：

```text
600519 -> 贵州茅台
sh600519 -> 贵州茅台
CN:SH:600519 -> 贵州茅台
贵州茅台 -> 贵州茅台
茅台 -> 贵州茅台
贵州茅台酒股份有限公司 -> 贵州茅台
guizhoumaotai -> 贵州茅台
gzmt -> 贵州茅台
宁王 -> 宁德时代
ningdeshidai -> 宁德时代
ndsd -> 宁德时代
重庆啤酒 -> 重庆啤酒
chongqingpijiu -> 重庆啤酒
cqpj -> 重庆啤酒
光模块 -> 相关股票 / 新闻 / 报告
CPO -> 相关股票 / 新闻 / 报告
AI服务器 -> 相关股票 / 新闻 / 报告
```

说明：

```text
股票相关断言在 stock_search 中验证。
新闻相关断言在 searchNews 中验证。
报告相关断言在 searchReports 中验证。
不得用一个全局搜索 case 同时断言三类结果。
```

## 17.4 前端测试

```text
股票搜索 loading / empty / error / success
拼音搜索成功
代码搜索成功
报告历史菜单范围搜索结果展示
资讯菜单范围搜索结果展示
自选备注菜单范围搜索结果展示
symbol 过滤
点击结果跳转
索引构建中提示
重建索引按钮
```

## 17.5 跨平台构建验证

必须执行：

```bash
cd apps/sidecar-core && go test -tags sqlite_fts5 ./...
pnpm --dir apps test
pnpm --dir apps check
pnpm --dir apps sidecar:check-targets
pnpm --dir apps release:check:local
```

重点验证：

```text
macOS Apple Silicon sidecar 可构建
macOS Intel sidecar 可构建
Windows x64 sidecar 可构建
构建脚本和测试脚本使用同一 FTS5 build tag
GSE 词典文件能被正确打包或嵌入
启动后 GSE 初始化成功
SELECT sqlite_compileoption_used('ENABLE_FTS5') 返回 1
```

---

# 18. Review Gate

## RGF1：依赖与构建门禁

```text
GSE 依赖已引入。
go-pinyin 依赖已引入。
FTS5 build tag 已进入 Go 测试、sidecar 构建和发布验证。
三平台 sidecar 构建通过。
不引入 gojieba 默认依赖。
不引入 cgo/C++ 强依赖。
```

## RGF2：Schema 门禁

```text
stocks 扩展字段迁移成功。
stock_aliases / stock_pinyin_overrides 创建成功。
stock_search_fts 创建成功。
search_documents / search_documents_fts 创建成功。
search_index_batches 创建成功。
search_index_state 创建成功。
search_index_jobs 创建成功。
所有新增索引元数据表遵守 created_at / updated_at 规则，明确例外不得隐含。
重复 migration 不破坏旧库。
迁移前备份生效。
FTS5 不可用时状态为 UNAVAILABLE，迁移不伪装成功。
BUILDING batch 不参与查询。
READY batch 切换失败时保留旧 active batch。
```

## RGF3：股票搜索门禁

```text
代码搜索通过。
symbol 搜索通过。
简称搜索通过。
全称搜索通过。
别名搜索通过。
拼音全拼搜索通过。
拼音首字母搜索通过。
多音字 override 生效。
股票搜索不返回新闻 / 报告。
本地无命中时 Provider fallback 边界符合 T13。
Provider 未配置时不伪造股票数据。
现有 stock_search(keyword) 仍返回数组。
排序符合强规则。
```

## RGF4：菜单范围搜索门禁

```text
search_reports 只检索 report。
search_news 只检索 news。
search_watchlist_notes 只检索 watchlist_note。
不提供 /api/search/global。
不提供前端任意 doc_types 组合。
支持 symbol 过滤。
软删除后不可见。
不索引 input_snapshot。
不索引 task_events payload。
不索引日志和凭据。
报告正文只来自 sanitized_search_summary。
sanitized_search_summary 脱敏失败时 fail closed，只索引 title + risk_summary。
报告搜索测试证明 userPosition / raw_api_key / resolved_api_key 不进入 FTS。
search_index_jobs 崩溃恢复生效。
```

## RGF5：安全门禁

```text
Rust command 固定 path。
Go API 全 POST。
Go API 复用 token / ready / DecodeJSON。
前端不拼 MATCH。
前端不直连 Go sidecar。
用户输入不能注入 FTS MATCH。
错误响应不泄露 SQL、路径、token。
顶部搜索框只调用 stock_search。
search_rebuild 只能由设置中心索引管理入口触发。
desktop security-config.test.mjs 覆盖 search_* command 注册、固定 path 和无通用代理。
search_rebuild 不接受任意 doc_types / 任意 path / 任意 scope。
```

## RGF6：性能门禁

```text
股票搜索 p95 < 80ms。
菜单范围搜索 p95 < 250ms。
10 万文档重建 < 120 秒。
重建期间 UI 可用。
增量索引不阻塞主业务写入。
重建失败时保留旧索引继续服务。
清理 RETIRED batch 不阻塞当前搜索。
```

---

# 19. 风险与应对

## 19.1 GSE 分词效果不稳定

风险：

```text
金融、股票、科技专有名词切词错误。
```

应对：

```text
内置投研词典。
股票基础数据动态 AddWord。
别名动态 AddWord。
搜索质量回归集。
Tokenizer 接口保留替换空间。
```

## 19.2 拼音多音字错误

风险：

```text
重庆、长城、兴业等股票拼音错误。
```

应对：

```text
stock_pinyin_overrides。
内置高频多音字股票修正规则。
搜索质量回归。
```

## 19.3 MATCH 查询注入

风险：

```text
用户输入构造 FTS5 特殊语法导致语法错误或异常召回。
```

应对：

```text
用户输入不直接拼 MATCH。
QueryBuilder 只接受安全 token。
SQL 参数化。
危险输入回归测试。
```

## 19.4 索引与源表不一致

风险：

```text
业务表更新后索引未更新。
```

应对：

```text
source updated_at > indexed_at 启动补偿。
手动重建索引。
search_index_state 记录版本和状态。
删除源数据时同步删除 FTS row。
```

## 19.5 索引体积膨胀

风险：

```text
prefix 过多、文档过长导致 SQLite 文件变大。
```

应对：

```text
股票索引和通用文档索引拆分。
不索引 HTML 全文。
prefix 控制长度。
定期 optimize。
设置中心展示索引大小。
```

---

# 20. 分阶段实施计划

## Phase 0：依赖、构建与 FTS5 探针

交付：

```text
GSE / go-pinyin 依赖确认
sqlite_fts5 或 fts5 build tag 统一配置
FTS5 编译能力探针
临时 SQLite FTS5 集成测试
sidecar:check-targets 覆盖三平台
```

验收：

```text
go test -tags sqlite_fts5 ./... 通过。
三平台 sidecar 构建链路使用同一 FTS5 tag。
启动探针能区分 AVAILABLE / UNAVAILABLE。
```

## Phase 1：Tokenizer 与股票搜索增强

交付：

```text
Tokenizer 接口
GSETokenizer
SimpleTokenizer
PinyinGenerator
stock_aliases
stock_pinyin_overrides
stock_search_fts
active_stock_batch_id
/api/stocks/search 增强为本地索引 + Provider fallback
顶部搜索框默认股票搜索
```

验收：

```text
代码、简称、全称、别名、拼音、首字母全部通过。
现有 stock_search(keyword) 响应数组契约不变。
Provider 未配置时不伪造数据。
```

## Phase 2：菜单范围搜索

交付：

```text
search_documents
search_documents_fts
search_index_batches
search_index_jobs
SearchIndexer
/api/search/reports
/api/search/news
/api/search/watchlist-notes
search_reports / search_news / search_watchlist_notes Rust command
报告历史、资讯中心、自选股页内搜索
```

验收：

```text
新闻、报告、自选备注分别在对应菜单内可搜索。
不存在 /search 全局页。
不存在跨 scope 搜索结果。
报告搜索不索引 input_snapshot 和未脱敏正文。
增量索引通过 search_index_jobs 崩溃恢复。
查询只读取 active_document_batch_id。
```

## Phase 3：索引管理

交付：

```text
search_index_state
search_index_batches
/api/search/status
/api/search/rebuild
设置中心索引管理
重建进度
FTS optimize
```

验收：

```text
可查看索引状态。
可手动重建。
重建失败可看到脱敏错误。
重建失败保留旧 active batch。
search_rebuild 只允许设置中心索引管理入口触发。
```

## Phase 4：搜索体验增强

交付：

```text
股票候选高亮摘要
菜单内结果高亮摘要
同义词词典
可选 trigram 子串召回
```

验收：

```text
结果摘要高亮。
顶部候选仍只返回股票。
菜单内增强不跨 scope。
不会引入明显噪声。
```

---

# 21. 最终建议

推荐落地方案：

```text
默认 tokenizer：GSE
备选 tokenizer：SimpleTokenizer
不默认引入 gojieba
SQLite FTS5 只做倒排和排序
股票搜索独立强排序
拼音字段独立索引
搜索索引可重建
敏感字段不索引
不做跨菜单全局搜索
```

最关键的工程原则：

```text
不要把“全文检索”做成一个散落在 stock/news/report 各模块里的工具函数；
必须独立成 service/search，并通过 dao/search.go 统一管理 FTS 表。
```

最推荐的开发顺序：

```text
1. 先做 GSE Tokenizer 抽象。
2. 再做 stock_search_fts。
3. 先增强 /api/stocks/search，并保持现有响应数组契约。
4. 再做 search_documents_fts。
5. 最后做菜单范围搜索页面和索引管理。
```
