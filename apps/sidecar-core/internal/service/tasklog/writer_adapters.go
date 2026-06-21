package tasklog

import (
	"context"
	"errors"
	"fmt"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// WriterAppender 将单条日志 writer 适配为批量 appender，用于复用 AsyncWriter。
type WriterAppender struct {
	Writer StageWriter
}

// Append 逐条写入批量日志；任意写入失败都会返回聚合错误，避免静默丢日志。
func (appender WriterAppender) Append(ctx context.Context, entries []model.TaskLogEntry) error {
	if appender.Writer == nil {
		return fmt.Errorf("task log writer is required")
	}
	var joined error
	for _, entry := range entries {
		if err := appender.Writer.WriteTaskLog(ctx, entry); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// MultiAppender 将同一批任务日志写入多个后端，例如 SQLite 和本地 NDJSON。
type MultiAppender struct {
	appenders []BatchAppender
}

// NewMultiAppender 创建批量日志组合写入器，nil appender 会被忽略。
func NewMultiAppender(appenders ...BatchAppender) MultiAppender {
	filtered := make([]BatchAppender, 0, len(appenders))
	for _, appender := range appenders {
		if appender != nil {
			filtered = append(filtered, appender)
		}
	}
	return MultiAppender{appenders: filtered}
}

// Append 将日志批量写入所有后端；返回聚合错误供生命周期观测。
func (appender MultiAppender) Append(ctx context.Context, entries []model.TaskLogEntry) error {
	if len(appender.appenders) == 0 {
		return fmt.Errorf("task log appender is required")
	}
	var joined error
	for _, target := range appender.appenders {
		if err := target.Append(ctx, entries); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}
