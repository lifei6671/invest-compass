package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/logger"
)

// ErrorCode 是 AI Provider 对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidRequest 表示请求参数不足。
	ErrorInvalidRequest ErrorCode = "invalid_ai_request"
	// ErrorUnauthorized 表示 Provider 返回认证失败。
	ErrorUnauthorized ErrorCode = "ai_unauthorized"
	// ErrorRateLimited 表示 Provider 返回限流。
	ErrorRateLimited ErrorCode = "ai_rate_limited"
	// ErrorUpstream 表示 Provider 返回 5xx 或不可识别错误。
	ErrorUpstream ErrorCode = "ai_upstream_error"
	// ErrorCancelled 表示上下文取消或超时。
	ErrorCancelled ErrorCode = "ai_request_cancelled"
)

// Error 表示 AI Provider 调用错误，错误文本必须脱敏。
type Error struct {
	Code    ErrorCode
	Message string
}

// Error 返回脱敏后的错误文本。
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Message == "" {
		return string(err.Code)
	}
	return string(err.Code) + ": " + logger.RedactText(err.Message)
}

// Role 是 OpenAI-compatible chat message 的角色。
type Role string

const (
	// RoleSystem 表示系统消息。
	RoleSystem Role = "system"
	// RoleUser 表示用户消息。
	RoleUser Role = "user"
	// RoleAssistant 表示助手消息。
	RoleAssistant Role = "assistant"
)

// Message 是 OpenAI-compatible chat message。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 是普通和流式 chat 共用请求。
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// ChatResponse 是普通 chat 响应。
type ChatResponse struct {
	ID      string
	Content string
}

// ChatChunk 是流式 chat 输出片段。
type ChatChunk struct {
	Content string
	Err     error
}

// ClientConfig 是 OpenAI-compatible Provider 客户端配置。
type ClientConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Client  *http.Client
}

// OpenAICompatibleClient 是基于 OpenAI-compatible chat completions 协议的客户端。
type OpenAICompatibleClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewOpenAICompatibleClient 创建 OpenAI-compatible Provider 客户端。
func NewOpenAICompatibleClient(config ClientConfig) OpenAICompatibleClient {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return OpenAICompatibleClient{
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		apiKey:  config.APIKey,
		client:  client,
	}
}

// Chat 调用 OpenAI-compatible 普通 chat completions 接口。
func (client OpenAICompatibleClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	if err := validateChatRequest(request); err != nil {
		return ChatResponse{}, err
	}
	httpRequest, err := client.newRequest(ctx, request, false)
	if err != nil {
		return ChatResponse{}, err
	}

	httpResponse, err := client.client.Do(httpRequest)
	if err != nil {
		return ChatResponse{}, mapTransportError(err)
	}
	defer httpResponse.Body.Close()

	if err := mapHTTPStatus(httpResponse); err != nil {
		return ChatResponse{}, err
	}

	var payload chatCompletionResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&payload); err != nil {
		return ChatResponse{}, &Error{Code: ErrorUpstream, Message: err.Error()}
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, &Error{Code: ErrorUpstream, Message: "missing choices"}
	}
	return ChatResponse{
		ID:      payload.ID,
		Content: payload.Choices[0].Message.Content,
	}, nil
}

// StreamChat 调用 OpenAI-compatible 流式 chat completions 接口。
func (client OpenAICompatibleClient) StreamChat(ctx context.Context, request ChatRequest) (<-chan ChatChunk, error) {
	if err := validateChatRequest(request); err != nil {
		return nil, err
	}
	httpRequest, err := client.newRequest(ctx, request, true)
	if err != nil {
		return nil, err
	}

	httpResponse, err := client.client.Do(httpRequest)
	if err != nil {
		return nil, mapTransportError(err)
	}
	if err := mapHTTPStatus(httpResponse); err != nil {
		httpResponse.Body.Close()
		return nil, err
	}

	chunks := make(chan ChatChunk)
	go func() {
		defer close(chunks)
		defer httpResponse.Body.Close()
		client.readStream(httpResponse.Body, chunks)
	}()
	return chunks, nil
}

// newRequest 构造 OpenAI-compatible chat completions HTTP 请求。
func (client OpenAICompatibleClient) newRequest(ctx context.Context, request ChatRequest, stream bool) (*http.Request, error) {
	body := chatCompletionRequest{
		Model:       request.Model,
		Messages:    request.Messages,
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
		Stream:      stream,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Code: ErrorInvalidRequest, Message: err.Error()}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return nil, &Error{Code: ErrorInvalidRequest, Message: err.Error()}
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if client.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+client.apiKey)
	}
	return httpRequest, nil
}

// readStream 读取 OpenAI-compatible SSE 数据流。
func (client OpenAICompatibleClient) readStream(body io.Reader, chunks chan<- ChatChunk) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return
		}

		var payload chatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			chunks <- ChatChunk{Err: &Error{Code: ErrorUpstream, Message: err.Error()}}
			return
		}
		for _, choice := range payload.Choices {
			if choice.Delta.Content != "" {
				chunks <- ChatChunk{Content: choice.Delta.Content}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		chunks <- ChatChunk{Err: mapTransportError(err)}
	}
}

// validateChatRequest 校验 chat 请求的必要字段。
func validateChatRequest(request ChatRequest) error {
	if strings.TrimSpace(request.Model) == "" || len(request.Messages) == 0 {
		return &Error{Code: ErrorInvalidRequest}
	}
	return nil
}

// mapHTTPStatus 将 Provider HTTP 状态码映射为稳定错误码。
func mapHTTPStatus(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(response.Body)
	message := string(body)
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return &Error{Code: ErrorUnauthorized, Message: message}
	case http.StatusTooManyRequests:
		return &Error{Code: ErrorRateLimited, Message: message}
	default:
		return &Error{Code: ErrorUpstream, Message: message}
	}
}

// mapTransportError 将网络、超时和取消错误映射为稳定错误码。
func mapTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: ErrorCancelled, Message: err.Error()}
	}
	return &Error{Code: ErrorUpstream, Message: fmt.Sprint(err)}
}

type chatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type chatCompletionStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}
