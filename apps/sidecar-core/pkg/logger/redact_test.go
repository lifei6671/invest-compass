package logger

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestFieldNamesMatchObservabilityContract 验证核心日志字段名保持稳定，避免模块间各自发明字段。
func TestFieldNamesMatchObservabilityContract(t *testing.T) {
	got := []string{
		FieldRequestID,
		FieldTraceID,
		FieldTaskID,
		FieldProvider,
		FieldSymbol,
	}
	want := []string{
		"request_id",
		"trace_id",
		"task_id",
		"provider",
		"symbol",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected logger fields: got=%v want=%v", got, want)
	}
}

// TestRedactTextMasksKnownSecrets 验证日志脱敏覆盖首版明确禁止输出的敏感字段。
func TestRedactTextMasksKnownSecrets(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer placeholder-auth-secret",
		"Proxy-Authorization: Basic proxy-secret",
		"api_key=placeholder-provider-secret",
		"proxy_password=super-proxy-password",
		"license_key=license-secret",
		"position_snapshot=100股成本价12.34",
	}, "\n")

	redacted := RedactText(input)

	for _, secret := range []string{
		"placeholder-auth-secret",
		"proxy-secret",
		"placeholder-provider-secret",
		"super-proxy-password",
		"license-secret",
		"100股成本价12.34",
	} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("脱敏结果仍包含敏感内容 %q: %s", secret, redacted)
		}
	}
	if strings.Count(redacted, RedactedValue) < 6 {
		t.Fatalf("expected at least 6 redaction markers, got %q", redacted)
	}
}

// TestRedactTextKeepsJSONPayloadValid 验证 JSON payload 脱敏后仍可被任务事件回放解析。
func TestRedactTextKeepsJSONPayloadValid(t *testing.T) {
	input := `{"Authorization":"Bearer placeholder-json-secret","userPosition":"满仓","has_user_position":true}`

	redacted := RedactText(input)

	var payload map[string]any
	if err := json.Unmarshal([]byte(redacted), &payload); err != nil {
		t.Fatalf("redacted JSON should remain valid, got %q: %v", redacted, err)
	}
	if strings.Contains(redacted, "placeholder-json-secret") || strings.Contains(redacted, "满仓") {
		t.Fatalf("redacted JSON leaked secret: %s", redacted)
	}
	if payload["has_user_position"] != true {
		t.Fatalf("redaction should keep safe presence flag, got %#v", payload)
	}
}

// TestRedactTextMasksQuotedJSONValuesWithDelimiters 验证 JSON 字符串字段值包含逗号时仍完整脱敏。
func TestRedactTextMasksQuotedJSONValuesWithDelimiters(t *testing.T) {
	input := `{"Authorization":"Bearer placeholder-json-secret","api_key":"sk,placeholder-json-key","proxy_password":"proxy,secret"}`

	redacted := RedactText(input)

	var payload map[string]string
	if err := json.Unmarshal([]byte(redacted), &payload); err != nil {
		t.Fatalf("redacted JSON should remain valid, got %q: %v", redacted, err)
	}
	for _, secret := range []string{"placeholder-json-secret", "placeholder-json-key", "proxy,secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted JSON leaked %q: %s", secret, redacted)
		}
	}
	for _, key := range []string{"Authorization", "api_key", "proxy_password"} {
		if payload[key] != RedactedValue {
			t.Fatalf("expected %s to be redacted, got %#v in %s", key, payload[key], redacted)
		}
	}
}

// TestRedactErrorMasksSecretInError 验证错误对象进入日志前也会被脱敏。
func TestRedactErrorMasksSecretInError(t *testing.T) {
	err := errors.New("provider failed with Authorization: Bearer placeholder-error-secret")

	redacted := RedactError(err)

	if strings.Contains(redacted, "placeholder-error-secret") {
		t.Fatalf("脱敏错误仍包含密钥: %s", redacted)
	}
	if !strings.Contains(redacted, RedactedValue) {
		t.Fatalf("脱敏错误缺少替换标记: %s", redacted)
	}
}

// TestRedactErrorHandlesNil 验证 nil 错误不会生成误导性日志内容。
func TestRedactErrorHandlesNil(t *testing.T) {
	if got := RedactError(nil); got != "" {
		t.Fatalf("expected empty redacted nil error, got %q", got)
	}
}

// TestExportLogTextRedactsAgain 验证日志导出前会再次脱敏，同时保留排障字段。
func TestExportLogTextRedactsAgain(t *testing.T) {
	lines := []string{
		`{"request_id":"req-1","trace_id":"trace-1","task_id":"task-1","message":"start"}`,
		`Authorization: Bearer placeholder-export-auth`,
		`api_key=placeholder-export-key`,
		`proxy_password=placeholder-export-proxy`,
		`position_input=用户一次性持仓输入`,
	}

	exported := ExportLogText(lines)

	for _, field := range []string{"request_id", "trace_id", "task_id"} {
		if !strings.Contains(exported, field) {
			t.Fatalf("expected exported logs to keep field %q: %s", field, exported)
		}
	}
	for _, secret := range []string{
		"placeholder-export-auth",
		"placeholder-export-key",
		"placeholder-export-proxy",
		"用户一次性持仓输入",
	} {
		if strings.Contains(exported, secret) {
			t.Fatalf("exported logs leaked %q: %s", secret, exported)
		}
	}
	if strings.Count(exported, RedactedValue) < 4 {
		t.Fatalf("expected redaction markers in exported logs: %s", exported)
	}
}
