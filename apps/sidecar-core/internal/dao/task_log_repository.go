package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultTaskLogLimit = 200
	maxTaskLogLimit     = 500
)

// TaskLogQuery 描述任务结构化日志列表查询条件。
type TaskLogQuery struct {
	TaskID    string
	Level     string
	Module    string
	Stage     string
	Keyword   string
	OnlyError bool
	AfterID   int64
	Limit     int
}

// TaskLogListResult 是任务日志分页查询结果。
type TaskLogListResult struct {
	Entries []model.TaskLogEntry
	HasMore bool
	NextID  int64
}

// TaskLogRetentionOptions 描述 SQLite 任务日志容量治理规则。
type TaskLogRetentionOptions struct {
	Before       time.Time
	PerTaskLimit int
	TotalLimit   int
}

// TaskLogRetentionResult 记录本次容量治理实际删除的日志数量。
type TaskLogRetentionResult struct {
	DeletedExpired         int64
	DeletedPerTaskOverflow int64
	DeletedTotalOverflow   int64
}

// AppendTaskLogs 批量写入已经脱敏的任务结构化日志。
func (store *Store) AppendTaskLogs(ctx context.Context, entries []model.TaskLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	for index := range entries {
		if strings.TrimSpace(entries[index].TaskID) == "" {
			return fmt.Errorf("task log task_id is required")
		}
		if strings.TrimSpace(entries[index].Level) == "" {
			return fmt.Errorf("task log level is required")
		}
		if strings.TrimSpace(entries[index].Module) == "" {
			return fmt.Errorf("task log module is required")
		}
		if strings.TrimSpace(entries[index].Stage) == "" {
			return fmt.Errorf("task log stage is required")
		}
	}
	return store.db.WithContext(ctx).Create(&entries).Error
}

// ListTaskLogs 按 task_id 和可选筛选条件返回结构化日志，按 id 升序用于增量拉取。
func (store *Store) ListTaskLogs(ctx context.Context, query TaskLogQuery) (TaskLogListResult, error) {
	taskID := strings.TrimSpace(query.TaskID)
	if taskID == "" {
		return TaskLogListResult{}, fmt.Errorf("task_id is required")
	}
	limit, err := normalizeTaskLogLimit(query.Limit)
	if err != nil {
		return TaskLogListResult{}, err
	}

	db := store.db.WithContext(ctx).Where("task_id = ? AND id > ?", taskID, query.AfterID)
	if level := strings.TrimSpace(query.Level); level != "" {
		db = db.Where("level = ?", level)
	}
	if module := strings.TrimSpace(query.Module); module != "" {
		db = db.Where("module = ?", module)
	}
	if stage := strings.TrimSpace(query.Stage); stage != "" {
		db = db.Where("stage = ?", stage)
	}
	if query.OnlyError {
		db = db.Where("level = ?", "ERROR")
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("message LIKE ? OR module LIKE ? OR stage LIKE ? OR payload_json LIKE ?", like, like, like, like)
	}

	var entries []model.TaskLogEntry
	if err := db.Order("id ASC").Limit(limit + 1).Find(&entries).Error; err != nil {
		return TaskLogListResult{}, err
	}
	result := TaskLogListResult{Entries: entries}
	if len(entries) > limit {
		result.HasMore = true
		result.Entries = entries[:limit]
	}
	if len(result.Entries) > 0 {
		result.NextID = result.Entries[len(result.Entries)-1].ID
	}
	return result, nil
}

// GetTaskLog 按自增 ID 读取单条结构化日志。
func (store *Store) GetTaskLog(ctx context.Context, id int64) (model.TaskLogEntry, bool, error) {
	if id <= 0 {
		return model.TaskLogEntry{}, false, fmt.Errorf("task log id must be positive")
	}
	var entry model.TaskLogEntry
	err := store.db.WithContext(ctx).First(&entry, "id = ?", id).Error
	if err == nil {
		return entry, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return model.TaskLogEntry{}, false, nil
	}
	return model.TaskLogEntry{}, false, err
}

// UpsertTaskErrorDiagnosis 按 task_id 幂等保存失败任务诊断摘要。
func (store *Store) UpsertTaskErrorDiagnosis(ctx context.Context, diagnosis model.TaskErrorDiagnosis) error {
	if strings.TrimSpace(diagnosis.TaskID) == "" {
		return fmt.Errorf("task diagnosis task_id is required")
	}
	if strings.TrimSpace(diagnosis.ErrorCode) == "" {
		return fmt.Errorf("task diagnosis error_code is required")
	}
	if strings.TrimSpace(diagnosis.ErrorStage) == "" {
		return fmt.Errorf("task diagnosis error_stage is required")
	}
	if strings.TrimSpace(diagnosis.Summary) == "" {
		return fmt.Errorf("task diagnosis summary is required")
	}
	return store.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"error_code",
			"error_stage",
			"summary",
			"causes_json",
			"suggestions_json",
			"retryable",
			"request_id",
			"trace_id",
			"source_log_id",
			"updated_at",
		}),
	}).Create(&diagnosis).Error
}

// GetTaskErrorDiagnosis 按 task_id 读取失败任务诊断摘要。
func (store *Store) GetTaskErrorDiagnosis(ctx context.Context, taskID string) (model.TaskErrorDiagnosis, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return model.TaskErrorDiagnosis{}, false, fmt.Errorf("task_id is required")
	}
	var diagnosis model.TaskErrorDiagnosis
	err := store.db.WithContext(ctx).Where("task_id = ?", taskID).First(&diagnosis).Error
	if err == nil {
		return diagnosis, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return model.TaskErrorDiagnosis{}, false, nil
	}
	return model.TaskErrorDiagnosis{}, false, err
}

// PruneTaskLogs 只裁剪 task_log_entries，避免误删任务、事件、报告和设置。
func (store *Store) PruneTaskLogs(ctx context.Context, options TaskLogRetentionOptions) (TaskLogRetentionResult, error) {
	if options.PerTaskLimit < 0 {
		return TaskLogRetentionResult{}, fmt.Errorf("task log per task limit must be non-negative")
	}
	if options.TotalLimit < 0 {
		return TaskLogRetentionResult{}, fmt.Errorf("task log total limit must be non-negative")
	}

	var result TaskLogRetentionResult
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !options.Before.IsZero() {
			deleted := tx.Where("ts < ?", options.Before).Delete(&model.TaskLogEntry{})
			if deleted.Error != nil {
				return deleted.Error
			}
			result.DeletedExpired = deleted.RowsAffected
		}

		if options.PerTaskLimit > 0 {
			deleted, err := pruneTaskLogPerTaskOverflow(tx, options.PerTaskLimit)
			if err != nil {
				return err
			}
			result.DeletedPerTaskOverflow = deleted
		}

		if options.TotalLimit > 0 {
			deleted, err := pruneTaskLogTotalOverflow(tx, options.TotalLimit)
			if err != nil {
				return err
			}
			result.DeletedTotalOverflow = deleted
		}
		return nil
	})
	return result, err
}

func normalizeTaskLogLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultTaskLogLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("task log limit must be positive")
	}
	if limit > maxTaskLogLimit {
		return 0, fmt.Errorf("task log limit exceeds %d", maxTaskLogLimit)
	}
	return limit, nil
}

func pruneTaskLogPerTaskOverflow(tx *gorm.DB, limit int) (int64, error) {
	var taskIDs []string
	if err := tx.Model(&model.TaskLogEntry{}).Distinct("task_id").Pluck("task_id", &taskIDs).Error; err != nil {
		return 0, err
	}

	var deleted int64
	for _, taskID := range taskIDs {
		var count int64
		if err := tx.Model(&model.TaskLogEntry{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
			return deleted, err
		}
		overflow := int(count) - limit
		if overflow <= 0 {
			continue
		}

		var ids []int64
		if err := tx.Model(&model.TaskLogEntry{}).
			Where("task_id = ?", taskID).
			Order("ts ASC, id ASC").
			Limit(overflow).
			Pluck("id", &ids).Error; err != nil {
			return deleted, err
		}
		if len(ids) == 0 {
			continue
		}
		result := tx.Where("id IN ?", ids).Delete(&model.TaskLogEntry{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	return deleted, nil
}

func pruneTaskLogTotalOverflow(tx *gorm.DB, limit int) (int64, error) {
	var count int64
	if err := tx.Model(&model.TaskLogEntry{}).Count(&count).Error; err != nil {
		return 0, err
	}
	overflow := int(count) - limit
	if overflow <= 0 {
		return 0, nil
	}

	var ids []int64
	if err := tx.Model(&model.TaskLogEntry{}).
		Order("ts ASC, id ASC").
		Limit(overflow).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := tx.Where("id IN ?", ids).Delete(&model.TaskLogEntry{})
	return result.RowsAffected, result.Error
}
