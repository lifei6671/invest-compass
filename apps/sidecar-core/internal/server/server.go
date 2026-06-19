package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
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
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// Serve 启动本地 HTTP server，并在收到 shutdown 信号时优雅关闭。
func Serve(listener net.Listener, handler http.Handler, shutdown <-chan struct{}) error {
	return serveWithShutdownTimeout(listener, handler, shutdown, 3*time.Second)
}

// serveWithShutdownTimeout 启动 HTTP server，并把优雅关闭错误传播给调用方。
func serveWithShutdownTimeout(listener net.Listener, handler http.Handler, shutdown <-chan struct{}, timeout time.Duration) error {
	httpServer := NewHTTPServer(handler)
	shutdownResult := make(chan error, 1)
	go shutdownWhenRequested(httpServer, shutdown, timeout, shutdownResult)

	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if err := <-shutdownResult; err != nil {
		return err
	}
	return nil
}

// shutdownWhenRequested 等待生命周期信号并给活跃请求一个短暂收尾窗口。
func shutdownWhenRequested(httpServer *http.Server, shutdown <-chan struct{}, timeout time.Duration, result chan<- error) {
	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result <- httpServer.Shutdown(ctx)
}

// validateListenHost 收紧 sidecar 监听地址，防止误配置成公网或局域网地址。
func validateListenHost(host string) error {
	if host != "127.0.0.1" {
		return fmt.Errorf("invalid listen host %q", host)
	}
	return nil
}
