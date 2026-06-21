package tasklog

import (
	"context"
	"fmt"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
)

const (
	// DefaultRetentionDays 是任务结构化日志默认保留天数。
	DefaultRetentionDays = 30
	// DefaultMaxEntriesPerTask 是单个任务默认最多保留日志条数。
	DefaultMaxEntriesPerTask = 5000
	// DefaultMaxTotalEntries 是全库默认最多保留日志条数。
	DefaultMaxTotalEntries = 500000
)

// RetentionStore 是日志保留策略依赖的数据访问边界。
type RetentionStore interface {
	PruneTaskLogs(ctx context.Context, options dao.TaskLogRetentionOptions) (dao.TaskLogRetentionResult, error)
}

// RetentionConfig 描述任务结构化日志保留策略。
type RetentionConfig struct {
	Store             RetentionStore
	Now               func() time.Time
	RetentionDays     int
	MaxEntriesPerTask int
	MaxTotalEntries   int
}

// RetentionManager 负责把配置转成 DAO 可执行的日志剪枝任务。
type RetentionManager struct {
	config RetentionConfig
}

// NewRetentionManager 创建任务日志保留策略执行器。
func NewRetentionManager(config RetentionConfig) (*RetentionManager, error) {
	if config.Store == nil {
		return nil, fmt.Errorf("task log retention store is required")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = DefaultRetentionDays
	}
	if config.MaxEntriesPerTask <= 0 {
		config.MaxEntriesPerTask = DefaultMaxEntriesPerTask
	}
	if config.MaxTotalEntries <= 0 {
		config.MaxTotalEntries = DefaultMaxTotalEntries
	}
	return &RetentionManager{config: config}, nil
}

// Apply 执行默认保留策略，只删除 task_log_entries 中的旧日志。
func (manager *RetentionManager) Apply(ctx context.Context) (dao.TaskLogRetentionResult, error) {
	if manager == nil {
		return dao.TaskLogRetentionResult{}, fmt.Errorf("task log retention manager is nil")
	}
	cutoff := manager.config.Now().AddDate(0, 0, -manager.config.RetentionDays)
	return manager.config.Store.PruneTaskLogs(ctx, dao.TaskLogRetentionOptions{
		Before:       cutoff,
		PerTaskLimit: manager.config.MaxEntriesPerTask,
		TotalLimit:   manager.config.MaxTotalEntries,
	})
}
