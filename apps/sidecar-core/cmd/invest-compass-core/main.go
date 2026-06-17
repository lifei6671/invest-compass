package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/server"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/sidecar"
)

const version = "0.1.0"

// main 启动 Go sidecar，完成本地监听、stdin token 握手和 ready JSON 输出。
func main() {
	host := flag.String("host", "127.0.0.1", "local listen host")
	port := flag.String("port", "0", "local listen port")
	flag.Parse()

	if err := validateListenHost(*host); err != nil {
		slog.Error("Go core 只允许监听 127.0.0.1", "host", *host)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(*host, *port))
	if err != nil {
		slog.Error("启动本地 HTTP server 失败", "error", err)
		os.Exit(1)
	}
	defer listener.Close()

	_, listenPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		slog.Error("解析本地监听端口失败", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// runtime token 只能通过 stdin 握手进入 Go core，不能写入 argv、env、日志或配置。
	handshake, err := sidecar.ReadHandshake(ctx, os.Stdin)
	if err != nil {
		slog.Error("sidecar stdin 握手失败", "error", "invalid_or_timeout")
		os.Exit(1)
	}

	var readyPort int
	if _, err := fmt.Sscanf(listenPort, "%d", &readyPort); err != nil {
		slog.Error("解析本地监听端口失败", "error", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(sidecar.ReadyMessage{
		Status: "ready",
		Port:   readyPort,
		PID:    os.Getpid(),
	}); err != nil {
		slog.Error("输出 ready JSON 失败", "error", err)
		os.Exit(1)
	}

	shutdownRequested := make(chan struct{}, 1)
	httpServer := &http.Server{}
	handler := server.NewHandler(server.Config{
		Version:  version,
		Token:    handshake.Token,
		DBStatus: "not_configured",
		Ready:    true,
		OnShutdown: func() {
			select {
			case shutdownRequested <- struct{}{}:
			default:
			}
		},
	})
	httpServer.Handler = handler

	go func() {
		<-shutdownRequested
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("关闭本地 HTTP server 失败", "error", err)
		}
	}()

	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("本地 HTTP server 异常退出", "error", err)
		os.Exit(1)
	}
}

// validateListenHost 收紧 sidecar 监听地址，防止误配置成公网或局域网地址。
func validateListenHost(host string) error {
	if host != "127.0.0.1" {
		return fmt.Errorf("invalid listen host %q", host)
	}
	return nil
}
