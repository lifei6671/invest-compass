package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Listen 创建只绑定 127.0.0.1 的本地 TCP listener，避免 Go core 暴露到局域网或公网。
func Listen(host string, port string) (net.Listener, error) {
	if err := validateListenHost(host); err != nil {
		return nil, err
	}
	return net.Listen("tcp", net.JoinHostPort(host, port))
}

// NewHTTPServer 创建 Go core 使用的标准库 HTTP server。
func NewHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler}
}

// Serve 启动本地 HTTP server，并在收到 shutdown 信号时优雅关闭。
func Serve(listener net.Listener, handler http.Handler, shutdown <-chan struct{}) error {
	httpServer := NewHTTPServer(handler)
	go shutdownWhenRequested(httpServer, shutdown)

	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// shutdownWhenRequested 等待生命周期信号并给活跃请求一个短暂收尾窗口。
func shutdownWhenRequested(httpServer *http.Server, shutdown <-chan struct{}) {
	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

// validateListenHost 收紧 sidecar 监听地址，防止误配置成公网或局域网地址。
func validateListenHost(host string) error {
	if host != "127.0.0.1" {
		return fmt.Errorf("invalid listen host %q", host)
	}
	return nil
}
