package main

import "testing"

// TestValidateListenHostOnlyAllowsLoopback 验证 sidecar 只能监听本机回环地址。
func TestValidateListenHostOnlyAllowsLoopback(t *testing.T) {
	if err := validateListenHost("127.0.0.1"); err != nil {
		t.Fatalf("expected 127.0.0.1 to be allowed, got %v", err)
	}
	if err := validateListenHost("0.0.0.0"); err == nil {
		t.Fatal("expected 0.0.0.0 to be rejected")
	}
}
