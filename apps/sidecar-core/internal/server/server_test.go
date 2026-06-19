package server

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
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

// TestNewHTTPServerConfiguresTimeouts 验证本地 HTTP server 带连接超时，避免异常连接长期占用 sidecar。
func TestNewHTTPServerConfiguresTimeouts(t *testing.T) {
	httpServer := NewHTTPServer(&recordingHandler{})

	if httpServer.ReadHeaderTimeout <= 0 ||
		httpServer.ReadTimeout <= 0 ||
		httpServer.WriteTimeout <= 0 ||
		httpServer.IdleTimeout <= 0 {
		t.Fatalf("expected http server timeouts to be configured: %+v", httpServer)
	}
}

// TestServeReturnsShutdownTimeout 验证优雅关闭超时时会把错误返回给调用方，
// 避免 sidecar 生命周期层误以为服务已经干净退出。
func TestServeReturnsShutdownTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	handlerEntered := make(chan struct{})
	releaseHandler := make(chan struct{})
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		close(handlerEntered)
		<-releaseHandler
		_, _ = response.Write([]byte("ok"))
	})

	shutdown := make(chan struct{})
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- serveWithShutdownTimeout(listener, handler, shutdown, time.Millisecond)
	}()

	requestDone := make(chan struct{})
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			_, _ = io.ReadAll(response.Body)
			_ = response.Body.Close()
		}
		close(requestDone)
	}()

	<-handlerEntered
	close(shutdown)

	err = <-serveDone
	close(releaseHandler)
	<-requestDone

	if err == nil {
		t.Fatal("expected shutdown timeout error")
	}
}

type recordingHandler struct{}

// ServeHTTP 实现 http.Handler，供 server 生命周期测试注入。
func (handler *recordingHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
