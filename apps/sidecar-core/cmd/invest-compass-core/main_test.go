package main

import (
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
)

// TestValidateListenHostOnlyAllowsLoopback 验证 sidecar 只能监听本机回环地址。
func TestValidateListenHostOnlyAllowsLoopback(t *testing.T) {
	listener, err := server.Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatalf("expected 127.0.0.1 to be allowed, got %v", err)
	}
	listener.Close()
	if listener, err := server.Listen("0.0.0.0", "0"); err == nil {
		listener.Close()
		t.Fatal("expected 0.0.0.0 to be rejected")
	}
}
