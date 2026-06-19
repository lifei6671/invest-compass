package logexport

import (
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Request 是构建日志导出包所需的输入。
type Request struct {
	Lines     []string
	CreatedAt time.Time
}

// Bundle 是交给 Rust 白名单命令写入用户授权目录的安全日志导出内容。
type Bundle struct {
	FileName  string    `json:"file_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidateRequest 校验导出日志是否包含全局排障必需字段；task_id 只在任务链路出现时保留。
func ValidateRequest(request Request) error {
	content := strings.Join(request.Lines, "\n")
	for _, field := range []string{logger.FieldRequestID, logger.FieldTraceID} {
		if !strings.Contains(content, field) {
			return &xerr.Error{Code: xerr.LogExportMissingTroubleshootingField}
		}
	}
	return nil
}

// BuildBundle 生成已经二次脱敏的日志导出包，不负责文件系统写入。
func BuildBundle(request Request) Bundle {
	createdAt := request.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	createdAt = createdAt.UTC()

	return Bundle{
		FileName:  "invest-compass-logs-" + createdAt.Format("20060102-150405") + ".txt",
		Content:   logger.ExportLogText(request.Lines),
		CreatedAt: createdAt,
	}
}
