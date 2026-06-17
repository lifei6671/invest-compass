package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestReadHandshakeAcceptsValidJSONLine 验证合法 stdin 握手能提取 token 和协议版本。
func TestReadHandshakeAcceptsValidJSONLine(t *testing.T) {
	handshake, err := ReadHandshake(context.Background(), strings.NewReader(`{"token":"test-token","protocolVersion":"1"}`+"\n"))
	if err != nil {
		t.Fatalf("expected valid handshake, got %v", err)
	}
	if handshake.Token != "test-token" || handshake.ProtocolVersion != "1" {
		t.Fatalf("unexpected handshake: %+v", handshake)
	}
}

// TestReadHandshakeRejectsInvalidJSON 验证非法 JSON 不会被当作有效握手。
func TestReadHandshakeRejectsInvalidJSON(t *testing.T) {
	_, err := ReadHandshake(context.Background(), strings.NewReader(`{"token":"test-token"`+"\n"))
	if err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

// TestReadHandshakeRejectsMissingToken 验证缺少 token 的握手会失败。
func TestReadHandshakeRejectsMissingToken(t *testing.T) {
	_, err := ReadHandshake(context.Background(), strings.NewReader(`{"protocolVersion":"1"}`+"\n"))
	if !errors.Is(err, ErrInvalidHandshake) {
		t.Fatalf("expected ErrInvalidHandshake, got %v", err)
	}
}

// TestReadHandshakeRejectsUnsupportedProtocol 验证不兼容协议版本会被拒绝。
func TestReadHandshakeRejectsUnsupportedProtocol(t *testing.T) {
	_, err := ReadHandshake(context.Background(), strings.NewReader(`{"token":"test-token","protocolVersion":"2"}`+"\n"))
	if !errors.Is(err, ErrInvalidHandshake) {
		t.Fatalf("expected ErrInvalidHandshake, got %v", err)
	}
}

// TestReadHandshakeReturnsContextErrorWhenNoInputArrives 验证 stdin 长时间无输入时按超时失败。
func TestReadHandshakeReturnsContextErrorWhenNoInputArrives(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := ReadHandshake(ctx, reader)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
}

// TestReadyMessageUsesStableJSONFields 验证 ready JSON 字段稳定，方便 Rust 层解析。
func TestReadyMessageUsesStableJSONFields(t *testing.T) {
	message := ReadyMessage{
		Status: "ready",
		Port:   5432,
		PID:    100,
	}

	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal ready message: %v", err)
	}

	if string(payload) != `{"status":"ready","port":5432,"pid":100}` {
		t.Fatalf("unexpected ready payload: %s", payload)
	}
}
