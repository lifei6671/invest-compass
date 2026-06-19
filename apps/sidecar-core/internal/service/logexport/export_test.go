package logexport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestBuildBundleRedactsLogContent 验证日志导出包会复用统一脱敏规则，避免敏感信息进入导出文件。
func TestBuildBundleRedactsLogContent(t *testing.T) {
	bundle := BuildBundle(Request{
		CreatedAt: time.Date(2026, 6, 17, 9, 8, 7, 0, time.FixedZone("CST", 8*60*60)),
		Lines: []string{
			`{"request_id":"req-1","trace_id":"trace-1","task_id":"task-1","message":"start"}`,
			`Authorization: Bearer placeholder-export-auth`,
			`api_key=placeholder-export-key`,
			`proxy_password=placeholder-export-proxy`,
			`position_input=用户一次性持仓输入`,
		},
	})

	for _, field := range []string{"request_id", "trace_id", "task_id"} {
		if !strings.Contains(bundle.Content, field) {
			t.Fatalf("expected exported content to keep field %q: %s", field, bundle.Content)
		}
	}

	for _, secret := range []string{
		"placeholder-export-auth",
		"placeholder-export-key",
		"placeholder-export-proxy",
		"用户一次性持仓输入",
	} {
		if strings.Contains(bundle.Content, secret) {
			t.Fatalf("export bundle leaked %q: %s", secret, bundle.Content)
		}
	}

	if strings.Count(bundle.Content, logger.RedactedValue) < 4 {
		t.Fatalf("expected redaction markers in export bundle: %s", bundle.Content)
	}
}

// TestBuildBundleUsesStableMetadata 验证导出包元数据稳定，方便 Rust 层后续做目录授权和写文件。
func TestBuildBundleUsesStableMetadata(t *testing.T) {
	bundle := BuildBundle(Request{
		CreatedAt: time.Date(2026, 6, 17, 9, 8, 7, 0, time.FixedZone("CST", 8*60*60)),
		Lines:     []string{"request_id=req-1"},
	})

	if bundle.FileName != "invest-compass-logs-20260617-010807.txt" {
		t.Fatalf("unexpected file name: %s", bundle.FileName)
	}
	if bundle.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected UTC timestamp, got %s", bundle.CreatedAt.Location())
	}
	if !bundle.CreatedAt.Equal(time.Date(2026, 6, 17, 1, 8, 7, 0, time.UTC)) {
		t.Fatalf("unexpected created_at: %s", bundle.CreatedAt)
	}
}

// TestValidateRequestRequiresTroubleshootingFields 验证日志导出前必须具备核心排障字段。
func TestValidateRequestRequiresTroubleshootingFields(t *testing.T) {
	err := ValidateRequest(Request{
		Lines: []string{
			`{"request_id":"req-1","task_id":"task-1","message":"missing trace"}`,
		},
	})

	assertLogExportErrorCode(t, err, xerr.LogExportMissingTroubleshootingField)
}

// TestValidateRequestAcceptsTroubleshootingFields 验证 request_id、trace_id、task_id 同时存在时允许导出。
func TestValidateRequestAcceptsTroubleshootingFields(t *testing.T) {
	err := ValidateRequest(Request{
		Lines: []string{
			`{"request_id":"req-1","trace_id":"trace-1","task_id":"task-1","message":"ok"}`,
		},
	})
	if err != nil {
		t.Fatalf("ValidateRequest returned error: %v", err)
	}
}

// TestValidateRequestAllowsLogsBeforeAnyTask 验证日志导出可用于排查启动、Provider 和配置类问题，
// 即使当前日志快照还没有任务链路，也不能因为缺少 task_id 阻断导出。
func TestValidateRequestAllowsLogsBeforeAnyTask(t *testing.T) {
	err := ValidateRequest(Request{
		Lines: []string{
			`{"request_id":"req-1","trace_id":"trace-1","message":"provider config failed"}`,
		},
	})
	if err != nil {
		t.Fatalf("ValidateRequest should allow non-task troubleshooting logs: %v", err)
	}
}

// TestMemorySourceCapturesStructuredSlog 验证生产日志源能采集结构化日志并在导出前脱敏。
func TestMemorySourceCapturesStructuredSlog(t *testing.T) {
	source := NewMemorySource(slog.NewTextHandler(io.Discard, nil), 10)
	slogLogger := slog.New(source)

	slogLogger.Info(
		"分析任务完成",
		logger.FieldRequestID, "req-log",
		logger.FieldTraceID, "trace-log",
		logger.FieldTaskID, "task-log",
		"Authorization", "Bearer placeholder-log-secret",
	)

	request, err := source.ExportLogRequest(context.Background())
	if err != nil {
		t.Fatalf("export log request: %v", err)
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("memory source should include troubleshooting fields: %v", err)
	}
	bundle := BuildBundle(request)
	for _, field := range []string{logger.FieldRequestID, logger.FieldTraceID, logger.FieldTaskID} {
		if !strings.Contains(bundle.Content, field) {
			t.Fatalf("expected exported logs to keep %s: %s", field, bundle.Content)
		}
	}
	if strings.Contains(bundle.Content, "placeholder-log-secret") {
		t.Fatalf("memory source leaked secret: %s", bundle.Content)
	}
}

// TestMemorySourceDerivedHandlersShareBuffer 验证 slog.With/WithGroup 派生 logger 的日志仍进入同一导出缓冲区。
func TestMemorySourceDerivedHandlersShareBuffer(t *testing.T) {
	source := NewMemorySource(slog.NewTextHandler(io.Discard, nil), 10)
	slogLogger := slog.New(source).With(logger.FieldRequestID, "req-derived").WithGroup("ai")

	slogLogger.Warn(
		"派生日志",
		logger.FieldTraceID, "trace-derived",
		logger.FieldTaskID, "task-derived",
		"api_key", "placeholder-derived-secret",
	)

	request, err := source.ExportLogRequest(context.Background())
	if err != nil {
		t.Fatalf("export log request: %v", err)
	}
	bundle := BuildBundle(request)
	for _, text := range []string{"派生日志", "req-derived", "trace-derived", "task-derived"} {
		if !strings.Contains(bundle.Content, text) {
			t.Fatalf("expected derived logger content %q in root export: %s", text, bundle.Content)
		}
	}
	if strings.Contains(bundle.Content, "placeholder-derived-secret") {
		t.Fatalf("derived logger leaked secret: %s", bundle.Content)
	}
}

// TestMemorySourceRedactsRecordBeforeForwarding 验证下游日志 handler 不会收到原始敏感字段。
func TestMemorySourceRedactsRecordBeforeForwarding(t *testing.T) {
	var output bytes.Buffer
	source := NewMemorySource(slog.NewTextHandler(&output, nil), 10)
	slogLogger := slog.New(source)

	slogLogger.Info(
		"测试下游脱敏",
		logger.FieldRequestID, "req-next",
		logger.FieldTraceID, "trace-next",
		logger.FieldTaskID, "task-next",
		"api_key", "placeholder-next-secret",
	)

	if strings.Contains(output.String(), "placeholder-next-secret") {
		t.Fatalf("downstream handler leaked secret: %s", output.String())
	}
	if !strings.Contains(output.String(), logger.RedactedValue) {
		t.Fatalf("downstream handler should contain redaction marker: %s", output.String())
	}
}

// TestMemorySourceRedactsAnyErrorBeforeForwarding 验证下游日志 handler 不会收到 error/Any 字段中的敏感文本。
func TestMemorySourceRedactsAnyErrorBeforeForwarding(t *testing.T) {
	var output bytes.Buffer
	source := NewMemorySource(slog.NewTextHandler(&output, nil), 10)
	slogLogger := slog.New(source)

	slogLogger.Warn(
		"测试下游 error 脱敏",
		logger.FieldRequestID, "req-any",
		logger.FieldTraceID, "trace-any",
		logger.FieldTaskID, "task-any",
		"error", errors.New("provider failed with Authorization: Bearer placeholder-any-secret"),
	)

	if strings.Contains(output.String(), "placeholder-any-secret") {
		t.Fatalf("downstream handler leaked error secret: %s", output.String())
	}
	if !strings.Contains(output.String(), logger.RedactedValue) {
		t.Fatalf("downstream handler should contain redaction marker: %s", output.String())
	}
}

// assertLogExportErrorCode 校验日志导出错误码稳定。
func assertLogExportErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var exportError *xerr.Error
	if !errors.As(err, &exportError) {
		t.Fatalf("expected logexport Error, got %T", err)
	}
	if exportError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, exportError.Code)
	}
}
