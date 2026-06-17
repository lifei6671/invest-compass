package logexport

import (
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
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
