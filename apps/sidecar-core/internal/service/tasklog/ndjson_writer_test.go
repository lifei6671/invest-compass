package tasklog

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestNDJSONWriterWritesRedactedDailyFile 验证文件日志按天写入且复用脱敏能力。
func TestNDJSONWriterWritesRedactedDailyFile(t *testing.T) {
	now := time.Date(2025, 5, 20, 15, 29, 46, 0, time.UTC)
	writer := newTestNDJSONWriter(t, NDJSONWriterConfig{
		WorkspaceDir: t.TempDir(),
		Now:          func() time.Time { return now },
	})

	err := writer.WriteTaskLog(context.Background(), model.TaskLogEntry{
		TaskID:      "task-1",
		RequestID:   "req-1",
		TraceID:     "trace-1",
		Ts:          now,
		Level:       LevelError,
		Module:      "ai",
		Stage:       "stream_failed",
		Message:     "Authorization: Bearer sk-live-raw-secret-123456",
		PayloadJSON: `{"api_key":"sk-live-raw-secret-123456","proxy_password":"proxy-secret"}`,
	})
	if err != nil {
		t.Fatalf("write ndjson: %v", err)
	}

	content := readFile(t, filepath.Join(writer.config.LogDir, "app-2025-05-20.ndjson"))
	if !strings.Contains(content, `"task_id":"task-1"`) {
		t.Fatalf("expected task id in ndjson line: %s", content)
	}
	for _, forbidden := range []string{"sk-live-raw-secret-123456", "proxy-secret", "Bearer sk-"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("ndjson content leaked %q: %s", forbidden, content)
		}
	}
}

// TestNDJSONWriterRollsBySize 验证单文件超过上限后滚动到序号文件。
func TestNDJSONWriterRollsBySize(t *testing.T) {
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	writer := newTestNDJSONWriter(t, NDJSONWriterConfig{
		WorkspaceDir: t.TempDir(),
		MaxFileBytes: 120,
		Now:          func() time.Time { return now },
	})
	entry := model.TaskLogEntry{TaskID: "task-1", Ts: now, Level: LevelInfo, Module: "market", Stage: "quote_fetch", Message: strings.Repeat("x", 80)}

	if err := writer.WriteTaskLog(context.Background(), entry); err != nil {
		t.Fatalf("write first log: %v", err)
	}
	if err := writer.WriteTaskLog(context.Background(), entry); err != nil {
		t.Fatalf("write second log: %v", err)
	}

	assertFileExists(t, filepath.Join(writer.config.LogDir, "app-2025-05-20.ndjson"))
	assertFileExists(t, filepath.Join(writer.config.LogDir, "app-2025-05-20.1.ndjson"))
}

// TestNDJSONWriterRetentionDeletesExpiredAndOldestFiles 验证文件日志保留策略只删除匹配的旧日志文件。
func TestNDJSONWriterRetentionDeletesExpiredAndOldestFiles(t *testing.T) {
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	logDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	oldPath := filepath.Join(logDir, "app-2025-04-01.ndjson")
	newPath := filepath.Join(logDir, "app-2025-05-20.ndjson")
	ignoredPath := filepath.Join(logDir, "note.txt")
	writeFileWithModTime(t, oldPath, "old\n", now.AddDate(0, 0, -40))
	writeFileWithModTime(t, newPath, strings.Repeat("n", 90), now)
	writeFileWithModTime(t, ignoredPath, "keep\n", now.AddDate(0, 0, -60))

	writer := newTestNDJSONWriter(t, NDJSONWriterConfig{
		LogDir:        logDir,
		MaxTotalBytes: 80,
		RetentionDays: 30,
		Now:           func() time.Time { return now },
	})
	if err := writer.ApplyRetention(context.Background()); err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	assertFileMissing(t, oldPath)
	assertFileMissing(t, newPath)
	assertFileExists(t, ignoredPath)
}

// newTestNDJSONWriter 创建测试用 NDJSON writer 并直接失败非法配置。
func newTestNDJSONWriter(t *testing.T, config NDJSONWriterConfig) *NDJSONWriter {
	t.Helper()
	writer, err := NewNDJSONWriter(config)
	if err != nil {
		t.Fatalf("new ndjson writer: %v", err)
	}
	return writer
}

// readFile 读取测试日志文件内容并保留行尾换行。
func readFile(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file %s: %v", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var builder strings.Builder
	for scanner.Scan() {
		builder.WriteString(scanner.Text())
		builder.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan file %s: %v", path, err)
	}
	return builder.String()
}

// writeFileWithModTime 写入测试文件并设置修改时间。
func writeFileWithModTime(t *testing.T, path string, content string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes file %s: %v", path, err)
	}
}

// assertFileExists 断言指定路径仍存在。
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

// assertFileMissing 断言指定路径已被清理。
func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file %s to be missing, stat err=%v", path, err)
	}
}
