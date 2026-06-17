package logexport

import (
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
)

// ErrorCode 是日志导出模块对外稳定的错误码。
type ErrorCode string

const (
	// ErrorMissingTroubleshootingField 表示导出日志缺少核心排障字段。
	ErrorMissingTroubleshootingField ErrorCode = "missing_log_troubleshooting_field"
)

// Error 表示日志导出规则错误。
type Error struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把原始日志内容写入错误文本。
func (err *Error) Error() string {
	return string(err.Code)
}

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

// ValidateRequest 校验导出日志是否包含定位问题所需的核心链路字段。
func ValidateRequest(request Request) error {
	content := strings.Join(request.Lines, "\n")
	for _, field := range []string{logger.FieldRequestID, logger.FieldTraceID, logger.FieldTaskID} {
		if !strings.Contains(content, field) {
			return &Error{Code: ErrorMissingTroubleshootingField}
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
