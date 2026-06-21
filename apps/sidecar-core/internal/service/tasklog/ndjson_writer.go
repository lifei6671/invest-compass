package tasklog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	defaultNDJSONMaxFileBytes  int64 = 20 * 1024 * 1024
	defaultNDJSONMaxTotalBytes int64 = 500 * 1024 * 1024
	defaultNDJSONRetentionDays       = 30
)

// NDJSONWriterConfig 描述本地 NDJSON 文件日志写入和保留策略。
type NDJSONWriterConfig struct {
	WorkspaceDir  string
	LogDir        string
	MaxFileBytes  int64
	MaxTotalBytes int64
	RetentionDays int
	Now           func() time.Time
}

// NDJSONWriter 将任务结构化日志追加为本地脱敏 NDJSON 文件。
type NDJSONWriter struct {
	config NDJSONWriterConfig
}

// NewNDJSONWriter 创建按天滚动的本地文件日志 writer。
func NewNDJSONWriter(config NDJSONWriterConfig) (*NDJSONWriter, error) {
	logDir := strings.TrimSpace(config.LogDir)
	if logDir == "" {
		workspaceDir := strings.TrimSpace(config.WorkspaceDir)
		if workspaceDir == "" {
			return nil, fmt.Errorf("workspace dir is required")
		}
		logDir = filepath.Join(workspaceDir, "logs")
	}
	if !filepath.IsAbs(logDir) {
		return nil, fmt.Errorf("log dir must be absolute")
	}
	if config.MaxFileBytes <= 0 {
		config.MaxFileBytes = defaultNDJSONMaxFileBytes
	}
	if config.MaxTotalBytes <= 0 {
		config.MaxTotalBytes = defaultNDJSONMaxTotalBytes
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = defaultNDJSONRetentionDays
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	config.LogDir = logDir
	return &NDJSONWriter{config: config}, nil
}

// WriteTaskLog 追加一条脱敏任务日志到当天 NDJSON 文件。
func (writer *NDJSONWriter) WriteTaskLog(ctx context.Context, entry model.TaskLogEntry) error {
	if writer == nil {
		return fmt.Errorf("ndjson writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(entry.TaskID) == "" {
		return fmt.Errorf("task_id is required")
	}
	if err := os.MkdirAll(writer.config.LogDir, 0o700); err != nil {
		return err
	}

	line, err := json.Marshal(writer.ndjsonRecord(entry))
	if err != nil {
		return err
	}
	line = append(line, '\n')

	path, err := writer.nextWritablePath(entry.Ts, int64(len(line)))
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(line); err != nil {
		return err
	}
	return writer.ApplyRetention(ctx)
}

// ApplyRetention 删除超出保留天数或总容量上限的 NDJSON 文件。
func (writer *NDJSONWriter) ApplyRetention(ctx context.Context) error {
	if writer == nil {
		return fmt.Errorf("ndjson writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	files, err := writer.logFiles()
	if err != nil {
		return err
	}
	cutoff := writer.config.Now().AddDate(0, 0, -writer.config.RetentionDays)
	kept := make([]ndjsonLogFile, 0, len(files))
	for _, file := range files {
		if file.ModTime.Before(cutoff) {
			if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		kept = append(kept, file)
	}

	var total int64
	for _, file := range kept {
		total += file.Size
	}
	if total <= writer.config.MaxTotalBytes {
		return nil
	}
	sort.Slice(kept, func(left, right int) bool {
		if kept[left].ModTime.Equal(kept[right].ModTime) {
			return kept[left].Path < kept[right].Path
		}
		return kept[left].ModTime.Before(kept[right].ModTime)
	})
	for _, file := range kept {
		if total <= writer.config.MaxTotalBytes {
			return nil
		}
		if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		total -= file.Size
	}
	return nil
}

func (writer *NDJSONWriter) nextWritablePath(value time.Time, incomingBytes int64) (string, error) {
	date := value
	if date.IsZero() {
		date = writer.config.Now()
	}
	base := filepath.Join(writer.config.LogDir, fmt.Sprintf("app-%s.ndjson", date.Format("2006-01-02")))
	for index := 0; ; index++ {
		path := base
		if index > 0 {
			path = filepath.Join(writer.config.LogDir, fmt.Sprintf("app-%s.%d.ndjson", date.Format("2006-01-02"), index))
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return path, nil
			}
			return "", err
		}
		if info.Size()+incomingBytes <= writer.config.MaxFileBytes {
			return path, nil
		}
	}
}

func (writer *NDJSONWriter) ndjsonRecord(entry model.TaskLogEntry) map[string]any {
	timestamp := entry.Ts
	if timestamp.IsZero() {
		timestamp = writer.config.Now()
	}
	record := map[string]any{
		"timestamp":   formatTime(timestamp),
		"task_id":     logger.RedactText(strings.TrimSpace(entry.TaskID)),
		"level":       logger.RedactText(strings.TrimSpace(entry.Level)),
		"module":      logger.RedactText(strings.TrimSpace(entry.Module)),
		"stage":       logger.RedactText(strings.TrimSpace(entry.Stage)),
		"message":     logger.RedactText(strings.TrimSpace(entry.Message)),
		"code":        logger.RedactText(strings.TrimSpace(entry.Code)),
		"provider":    logger.RedactText(strings.TrimSpace(entry.Provider)),
		"model":       logger.RedactText(strings.TrimSpace(entry.Model)),
		"symbol":      logger.RedactText(strings.TrimSpace(entry.Symbol)),
		"duration_ms": entry.DurationMS,
		"retryable":   entry.Retryable,
		"request_id":  logger.RedactText(strings.TrimSpace(entry.RequestID)),
		"trace_id":    logger.RedactText(strings.TrimSpace(entry.TraceID)),
	}
	if payload := sanitizeJSON(entry.PayloadJSON); payload != "" {
		var decoded any
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			record["payload"] = decoded
		} else {
			record["payload_text"] = payload
		}
	}
	return record
}

func (writer *NDJSONWriter) logFiles() ([]ndjsonLogFile, error) {
	entries, err := os.ReadDir(writer.config.LogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	files := make([]ndjsonLogFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "app-") || !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, ndjsonLogFile{
			Path:    filepath.Join(writer.config.LogDir, name),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	return files, nil
}

type ndjsonLogFile struct {
	Path    string
	Size    int64
	ModTime time.Time
}
