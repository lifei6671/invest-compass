package model

import (
	"time"

	"gorm.io/gorm"
)

// Stock 是股票基础信息缓存表，symbol 是全局唯一的标准股票代码。
type Stock struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Symbol         string `gorm:"not null;uniqueIndex"`
	Market         string `gorm:"not null"`
	Code           string `gorm:"not null"`
	Name           string `gorm:"not null"`
	Pinyin         string
	Exchange       string
	Industry       string
	Concept        string
	ListDate       string
	Status         string
	FullName       string
	PinyinFull     string
	PinyinInitials string
	SearchName     string
	SearchVersion  int
	IndexedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Watchlist 是自选股表，active symbol 唯一约束只作用于未软删除记录。
type Watchlist struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Symbol    string `gorm:"not null;uniqueIndex:idx_watchlists_symbol_active,where:deleted_at IS NULL"`
	SortOrder int
	Tags      string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// Quote 是行情快照缓存表，保留 Provider 返回的最新可展示行情字段。
type Quote struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	Symbol        string `gorm:"not null;index"`
	Price         float64
	ChangeAmount  float64
	ChangePercent float64
	Open          float64
	High          float64
	Low           float64
	PreClose      float64
	Volume        float64
	Amount        float64
	TurnoverRate  float64
	PE            float64
	PB            float64
	QuoteTime     time.Time
	Provider      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Kline 是 K 线缓存表，同一股票、周期、复权和交易日只能保留一条记录。
type Kline struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Symbol    string `gorm:"not null;uniqueIndex:idx_klines_symbol_period_adjust_trade_date,priority:1"`
	Period    string `gorm:"not null;uniqueIndex:idx_klines_symbol_period_adjust_trade_date,priority:2"`
	Adjust    string `gorm:"not null;uniqueIndex:idx_klines_symbol_period_adjust_trade_date,priority:3"`
	TradeDate string `gorm:"not null;uniqueIndex:idx_klines_symbol_period_adjust_trade_date,priority:4"`
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Amount    float64
	Provider  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewsItem 是新闻资讯缓存表，content_hash 用于跨来源转载去重。
type NewsItem struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	Source      string
	Market      string `gorm:"size:16;index"`
	Title       string `gorm:"not null"`
	URL         string
	Summary     string
	ContentHash string `gorm:"uniqueIndex"`
	Symbols     string
	Tags        string
	PublishedAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

// AIConfig 是 Go core 可持久化的模型配置表，不保存真实 API Key。
type AIConfig struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Name           string `gorm:"not null"`
	Provider       string `gorm:"not null"`
	BaseURL        string
	APIKeyRef      string
	MaskedAPIKey   string
	HasAPIKey      bool
	ModelName      string  `gorm:"not null"`
	Temperature    float64 `gorm:"default:0.7"`
	MaxTokens      int     `gorm:"default:4096"`
	TimeoutSeconds int     `gorm:"default:120"`
	StreamEnabled  bool    `gorm:"default:true"`
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt
}

// DataSourceCredential 是第三方数据源凭据配置表，真实凭据只允许加密后保存。
type DataSourceCredential struct {
	ID                   int64  `gorm:"primaryKey;autoIncrement"`
	ProviderID           string `gorm:"not null;uniqueIndex"`
	ProviderName         string `gorm:"not null"`
	Capability           string
	AuthType             string `gorm:"not null"`
	BaseURL              string
	CredentialStatus     string `gorm:"not null;default:not_configured"`
	ExpiresAt            *time.Time
	TimeoutSeconds       int
	RateLimitPerMinute   int
	EncryptedCredential  string
	CredentialNonce      string
	MaskedCredential     string
	Note                 string
	LastTestStatus       string
	LastTestResponseTime int
	LastTestedAt         *time.Time
	LastTestMessages     string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt
}

// PromptTemplate 是 Prompt 模板表，variables 保存首版白名单变量的序列化结果。
type PromptTemplate struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"not null"`
	Type        string `gorm:"not null"`
	Description string
	Content     string `gorm:"not null"`
	Variables   string
	IsBuiltin   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

// AnalysisReport 是 AI 分析报告表，按 task_id 支撑幂等保存。
type AnalysisReport struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	TaskID           string `gorm:"uniqueIndex"`
	Symbol           string `gorm:"not null"`
	Title            string `gorm:"not null"`
	AnalysisType     string `gorm:"not null"`
	ModelName        string
	PromptTemplateID int64
	InputSnapshot    string
	ContentMarkdown  string
	RiskSummary      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt
}

// Task 是长任务表，Go core 重启时会根据 status 恢复 RUNNING 任务。
type Task struct {
	ID           string `gorm:"primaryKey"`
	Type         string `gorm:"not null"`
	Status       string `gorm:"not null;index"`
	Title        string
	Progress     int
	ErrorMessage string
	StartedAt    time.Time
	FinishedAt   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TaskEvent 是任务事件表，用于 SSE 断线恢复、任务历史详情和崩溃回放。
type TaskEvent struct {
	ID        int64  `gorm:"primaryKey;autoIncrement;index:idx_task_events_task_id_id,priority:2"`
	TaskID    string `gorm:"not null;index:idx_task_events_task_id_id,priority:1"`
	EventType string `gorm:"not null"`
	Payload   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Setting 是非敏感设置表，真实凭据只允许保存本地 vault 引用。
type Setting struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Notification 是应用内通知表，只保存可展示摘要和源对象引用，不复制任务或报告源数据。
type Notification struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Type       string `gorm:"not null;index"`
	Level      string `gorm:"not null;index"`
	Title      string `gorm:"not null"`
	Content    string
	SourceType string `gorm:"index"`
	SourceID   string `gorm:"index"`
	Route      string
	IsRead     bool `gorm:"not null;default:false;index"`
	ReadAt     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt
}

// SchedulerJob 是桌面可管理的调度任务配置，cron_type 对应调度注册表中的业务任务类型。
type SchedulerJob struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Name           string `gorm:"not null"`
	CronType       string `gorm:"column:cron_type;not null;index"`
	CronExpr       string `gorm:"not null"`
	Enabled        bool   `gorm:"not null;default:false;index"`
	Market         string `gorm:"not null;default:CN"`
	Timezone       string `gorm:"not null;default:Asia/Shanghai"`
	TradeWindow    string `gorm:"not null"`
	ScopeJSON      string
	ParamsJSON     string
	CatchupEnabled bool `gorm:"not null;default:false"`
	CatchupMaxDays int  `gorm:"not null;default:5"`
	TimeoutSeconds int  `gorm:"not null;default:120"`
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	LastStatus     string
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt
}

// SchedulerRun 是调度任务的一次执行记录，run_key 用于避免启动补偿和手动触发重复入队。
type SchedulerRun struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	JobID         int64  `gorm:"not null;index"`
	CronType      string `gorm:"column:cron_type;not null;default:'';index"`
	DataType      string `gorm:"not null;default:'';index"`
	Period        string `gorm:"not null;default:'';index"`
	ParamsJSON    string
	RunKey        string `gorm:"not null;uniqueIndex"`
	TriggerType   string `gorm:"not null;index"`
	Status        string `gorm:"not null;index"`
	Priority      int    `gorm:"not null;default:0"`
	Source        string
	TargetDate    string
	ScopeKey      string `gorm:"index"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
	FetchedCount  int
	WrittenCount  int
	SkippedReason string
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IngestionWatermark 记录每个数据范围的最后成功抓取位置，供补偿任务判定缺口。
type IngestionWatermark struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	DataType      string `gorm:"not null;uniqueIndex:idx_ingestion_watermarks_scope,priority:1"`
	ScopeKey      string `gorm:"not null;uniqueIndex:idx_ingestion_watermarks_scope,priority:2"`
	Provider      string `gorm:"not null;uniqueIndex:idx_ingestion_watermarks_scope,priority:3"`
	Period        string `gorm:"not null;default:'';uniqueIndex:idx_ingestion_watermarks_scope,priority:4"`
	LastSuccessAt *time.Time
	LastTradeDate string
	CursorJSON    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// StockAlias 是股票搜索使用的别名表，别名只影响本地搜索召回和排序。
type StockAlias struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Symbol         string `gorm:"not null;uniqueIndex:idx_stock_aliases_symbol_alias,priority:1;index:idx_stock_aliases_symbol_active,where:deleted_at IS NULL"`
	Alias          string `gorm:"not null;uniqueIndex:idx_stock_aliases_symbol_alias,priority:2"`
	AliasType      string `gorm:"not null;default:manual"`
	PinyinFull     string
	PinyinInitials string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt
}

// StockPinyinOverride 保存股票多音字拼音修正规则。
type StockPinyinOverride struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Symbol         string `gorm:"not null;uniqueIndex"`
	PinyinFull     string `gorm:"not null"`
	PinyinInitials string `gorm:"not null"`
	Reason         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SearchDocument 是菜单范围搜索索引的元数据表，真实内容仍以业务源表为准。
type SearchDocument struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	BatchID      string `gorm:"column:batch_id;not null;uniqueIndex:idx_search_documents_batch_doc_uid,priority:1;index:idx_search_documents_batch_doc_type;index:idx_search_documents_batch_symbol;index:idx_search_documents_batch_ref,priority:1;index:idx_search_documents_batch_source_time"`
	DocUID       string `gorm:"not null;uniqueIndex:idx_search_documents_batch_doc_uid,priority:2"`
	DocType      string `gorm:"not null;index:idx_search_documents_batch_doc_type"`
	RefTable     string `gorm:"not null;index:idx_search_documents_batch_ref,priority:2"`
	RefID        string `gorm:"not null;index:idx_search_documents_batch_ref,priority:3"`
	Symbol       string `gorm:"index:idx_search_documents_batch_symbol"`
	Title        string `gorm:"not null"`
	Summary      string
	Source       string
	SourceTime   time.Time `gorm:"index:idx_search_documents_batch_source_time"`
	IndexedAt    time.Time `gorm:"not null"`
	IndexVersion int       `gorm:"not null;default:1"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt
}

// SearchIndexBatch 记录一次可切换的搜索索引重建批次。
type SearchIndexBatch struct {
	BatchID             string `gorm:"column:batch_id;primaryKey"`
	Scope               string `gorm:"not null;index:idx_search_index_batches_scope_status,priority:1"`
	Status              string `gorm:"not null;index:idx_search_index_batches_scope_status,priority:2"`
	SourceSchemaVersion int    `gorm:"not null;default:1"`
	TokenizerName       string `gorm:"not null"`
	TokenizerVersion    string `gorm:"not null"`
	DictionaryHash      string `gorm:"not null"`
	StartedAt           time.Time
	FinishedAt          *time.Time
	ErrorMessage        string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// SearchIndexState 保存搜索索引的运行期状态和 active batch 指针。
type SearchIndexState struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName 固定搜索状态表名为单数，便于作为 key/value 状态表使用。
func (SearchIndexState) TableName() string {
	return "search_index_state"
}

// SearchIndexJob 是搜索增量索引的持久化 outbox，避免进程崩溃丢任务。
type SearchIndexJob struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	DocType   string `gorm:"not null;uniqueIndex:idx_search_index_jobs_unique,priority:1"`
	RefID     string `gorm:"not null;uniqueIndex:idx_search_index_jobs_unique,priority:2"`
	Operation string `gorm:"not null;uniqueIndex:idx_search_index_jobs_unique,priority:3"`
	Status    string `gorm:"not null;index:idx_search_index_jobs_status"`
	Attempts  int    `gorm:"not null;default:0"`
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}
