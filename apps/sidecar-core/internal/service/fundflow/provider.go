package fundflow

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

// Scope 表示资金流数据覆盖的板块范围。
type Scope string

const (
	// ScopeConcept 表示概念板块资金流。
	ScopeConcept Scope = "concept"
	// ScopeIndustry 表示行业板块资金流，当前概念 Provider 不支持该范围。
	ScopeIndustry Scope = "industry"
	// ScopeStock 表示个股资金流。
	ScopeStock Scope = "stock"
)

// Provider 定义资金流数据源的最小能力边界。
type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	FetchSnapshot(ctx context.Context, request Request) (Snapshot, error)
}

// ProviderStatus 描述资金流数据源来源、授权边界、频率限制和可用性。
type ProviderStatus struct {
	Name            string
	Source          string
	License         string
	RateLimit       string
	Available       bool
	LastCheckedAt   time.Time
	LastError       string
	SupportedScopes []Scope
}

// Request 是一次资金流快照查询请求。
type Request struct {
	Scope Scope
	Limit int
}

// Snapshot 表示某一时刻的板块资金流快照。
type Snapshot struct {
	Scope     Scope
	Total     int
	Items     []Item
	Provider  string
	Source    string
	FetchedAt time.Time
}

// TopN 返回按主力净流入降序排列的前 n 个条目，便于后续摘要或排行榜展示。
func (snapshot Snapshot) TopN(n int) []Item {
	if n <= 0 || len(snapshot.Items) == 0 {
		return nil
	}
	items := make([]Item, len(snapshot.Items))
	copy(items, snapshot.Items)
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].NetInflow > items[right].NetInflow
	})
	if n > len(items) {
		n = len(items)
	}
	return items[:n]
}

// Item 表示单个概念或行业板块的资金流条目。
type Item struct {
	Code      string
	Name      string
	MarketID  int
	NetInflow float64
}

// ProviderError 是资金流 Provider 调用失败时的可观测错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏后的 Provider 错误，避免远端响应或请求信息污染日志。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf(
		"fundflow provider %s %s failed: %s",
		err.Provider,
		err.Operation,
		logger.RedactError(err.Cause),
	)
}

// Unwrap 返回原始错误，供调用方进行错误分类。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建会自动脱敏输出的资金流 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}
