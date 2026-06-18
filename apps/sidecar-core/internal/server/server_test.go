package server

import (
	"net/http"
	"testing"
)

// TestListenRejectsNonLocalhost 验证 server 层拒绝非本机回环监听地址。
func TestListenRejectsNonLocalhost(t *testing.T) {
	listener, err := Listen("0.0.0.0", "0")
	if err == nil {
		listener.Close()
		t.Fatal("expected Listen to reject non-localhost host")
	}
}

// TestNewHTTPServerUsesInjectedHandler 验证 server 层只接收外部注入 handler，不创建业务路由。
func TestNewHTTPServerUsesInjectedHandler(t *testing.T) {
	handler := &recordingHandler{}
	httpServer := NewHTTPServer(handler)

	if httpServer.Handler != handler {
		t.Fatal("expected server to use injected handler")
	}
}

type recordingHandler struct{}

// ServeHTTP 实现 http.Handler，供 server 生命周期测试注入。
func (handler *recordingHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
