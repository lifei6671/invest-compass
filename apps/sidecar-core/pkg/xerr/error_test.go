package xerr

import (
	"strings"
	"testing"
)

// TestErrorReturnsCodeAndRedactedMessage 验证通用错误会返回稳定错误码并脱敏消息。
func TestErrorReturnsCodeAndRedactedMessage(t *testing.T) {
	err := &Error{
		Code:    AIUnauthorized,
		Message: "Authorization: Bearer demo-sensitive-value",
	}

	text := err.Error()
	if !strings.Contains(text, string(AIUnauthorized)) {
		t.Fatalf("expected error text to contain code, got %q", text)
	}
	if strings.Contains(text, "demo-sensitive-value") {
		t.Fatalf("expected error text to redact secret, got %q", text)
	}
}
