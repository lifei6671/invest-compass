package model

import (
	"time"

	"gorm.io/gorm"
)

// Stock 是股票基础信息缓存表，symbol 是全局唯一的标准股票代码。
type Stock struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Symbol    string `gorm:"not null;uniqueIndex"`
	Market    string `gorm:"not null"`
	Code      string `gorm:"not null"`
	Name      string `gorm:"not null"`
	Pinyin    string
	Exchange  string
	Industry  string
	Concept   string
	ListDate  string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
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
