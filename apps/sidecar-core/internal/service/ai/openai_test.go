package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestChatSendsOpenAICompatibleRequest 验证普通 chat 请求使用 OpenAI-compatible /v1/chat/completions 协议。
func TestChatSendsOpenAICompatibleRequest(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotAuth = request.Header.Get("Authorization")
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "gpt-test" || payload["stream"] == true {
			t.Fatalf("unexpected request payload: %#v", payload)
		}

		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"chatcmpl-test","choices":[{"message":{"content":"分析结果"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(ClientConfig{
		BaseURL: server.URL,
		APIKey:  "dummy-provider-token",
		Timeout: time.Second,
	})
	result, err := client.Chat(context.Background(), ChatRequest{
		Model:       "gpt-test",
		Messages:    []Message{{Role: RoleUser, Content: "hello"}},
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if gotAuth != "Bearer dummy-provider-token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
	if result.Content != "分析结果" {
		t.Fatalf("unexpected content: %+v", result)
	}
}

// TestChatMapsHTTPErrorStatus 验证 401、429、5xx 会映射为稳定错误码且不泄露密钥。
func TestChatMapsHTTPErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   xerr.Code
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, code: xerr.AIUnauthorized},
		{name: "rate limited", status: http.StatusTooManyRequests, code: xerr.AIRateLimited},
		{name: "upstream", status: http.StatusInternalServerError, code: xerr.AIUpstream},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(tt.status)
				_, _ = response.Write([]byte(`{"error":{"message":"Authorization: Bearer dummy-upstream-token"}}`))
			}))
			defer server.Close()

			client := NewOpenAICompatibleClient(ClientConfig{BaseURL: server.URL, APIKey: "dummy-provider-token", Timeout: time.Second})
			_, err := client.Chat(context.Background(), ChatRequest{Model: "gpt-test", Messages: []Message{{Role: RoleUser, Content: "hello"}}})

			assertAIErrorCode(t, err, tt.code)
			if strings.Contains(err.Error(), "dummy-upstream-token") || strings.Contains(err.Error(), "dummy-provider-token") {
				t.Fatalf("error leaked secret: %v", err)
			}
		})
	}
}

// TestChatHonorsContextCancellation 验证请求支持 context cancellation。
func TestChatHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewOpenAICompatibleClient(ClientConfig{BaseURL: server.URL, APIKey: "dummy-provider-token", Timeout: time.Second})
	_, err := client.Chat(ctx, ChatRequest{Model: "gpt-test", Messages: []Message{{Role: RoleUser, Content: "hello"}}})

	assertAIErrorCode(t, err, xerr.AICancelled)
}

// TestStreamChatReadsChunks 验证流式响应可以读取 content delta，直到 DONE 结束。
func TestStreamChatReadsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["stream"] != true {
			t.Fatalf("expected stream=true, got %#v", payload)
		}

		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = response.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n"))
		_, _ = response.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n"))
		_, _ = response.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(ClientConfig{BaseURL: server.URL, APIKey: "dummy-provider-token", Timeout: time.Second})
	chunks, err := client.StreamChat(context.Background(), ChatRequest{Model: "gpt-test", Messages: []Message{{Role: RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}

	var content strings.Builder
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatalf("stream chunk error: %v", chunk.Err)
		}
		content.WriteString(chunk.Content)
	}
	if content.String() != "你好" {
		t.Fatalf("unexpected stream content: %q", content.String())
	}
}

// TestStreamChatRejectsChunkWithoutChoices 验证异常流式 payload 不会被静默吞掉。
func TestStreamChatRejectsChunkWithoutChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = response.Write([]byte("data: {\"id\":\"chunk-without-choices\"}\n\n"))
		_, _ = response.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(ClientConfig{BaseURL: server.URL, APIKey: "dummy-provider-token", Timeout: time.Second})
	chunks, err := client.StreamChat(context.Background(), ChatRequest{Model: "gpt-test", Messages: []Message{{Role: RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}

	var gotErr error
	for chunk := range chunks {
		if chunk.Err != nil {
			gotErr = chunk.Err
			break
		}
	}

	assertAIErrorCode(t, gotErr, xerr.AIUpstream)
}

// assertAIErrorCode 校验 AI Provider 错误码稳定。
func assertAIErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var aiError *xerr.Error
	if !errors.As(err, &aiError) {
		t.Fatalf("expected AI Error, got %T", err)
	}
	if aiError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, aiError.Code)
	}
}
