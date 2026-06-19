package httpx

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// TokenHeader 是 Rust 代理访问 Go core 时必须携带的运行期 token header。
const TokenHeader = "X-Invest-Compass-Token"

// MaxJSONBodyBytes 是 Go core 本地 API 单个 JSON 请求体上限，避免异常请求占用过多内存。
const MaxJSONBodyBytes = 1 << 20

// Route 描述一个 action 子包拥有的 HTTP 路由，不绑定具体 Gin 注册 API。
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// SecurityConfig 是所有本地 Go API 共用的 ready 和 runtime token 边界。
type SecurityConfig struct {
	Token string
	Ready bool
}

// Response 是 Go core 统一 JSON envelope。
type Response struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

// RequestContext 保存单次请求的可观测追踪字段。
type RequestContext struct {
	RequestID string
	TraceID   string
}

// ContextFrom 从请求头提取追踪 ID，缺失时生成本地 ID 方便排障关联。
func ContextFrom(request *http.Request) RequestContext {
	return RequestContext{
		RequestID: traceHeaderOrNewID(request.Header.Get("X-Request-Id")),
		TraceID:   traceHeaderOrNewID(request.Header.Get("X-Trace-Id")),
	}
}

// RequireReadyToken 校验业务 API 必须在 sidecar ready 后携带正确 runtime token。
func RequireReadyToken(response http.ResponseWriter, request *http.Request, security SecurityConfig, context RequestContext) bool {
	if !security.Ready || security.Token == "" {
		WriteError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return false
	}
	if !tokenMatches(request.Header.Get(TokenHeader), security.Token) {
		WriteError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return false
	}
	return true
}

// WriteOK 用统一成功 envelope 写出 action 响应。
func WriteOK(response http.ResponseWriter, data any, context RequestContext) {
	WriteJSON(response, http.StatusOK, Response{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: context.RequestID,
		TraceID:   context.TraceID,
	})
}

// WriteError 用统一错误结构返回失败原因，避免各 handler 自行拼响应。
func WriteError(response http.ResponseWriter, status int, code int, message string, context RequestContext) {
	WriteJSON(response, status, Response{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: context.RequestID,
		TraceID:   context.TraceID,
	})
}

// DecodeJSON 统一限制并解析本地 API JSON 请求体，所有 handler 都应通过它进入业务校验。
func DecodeJSON(response http.ResponseWriter, request *http.Request, context RequestContext, payload any) bool {
	request.Body = http.MaxBytesReader(response, request.Body, MaxJSONBodyBytes)
	decoder := json.NewDecoder(request.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(response, http.StatusRequestEntityTooLarge, 41300, "request_body_too_large", context)
			return false
		}
		WriteError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(response, http.StatusRequestEntityTooLarge, 41300, "request_body_too_large", context)
			return false
		}
		WriteError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return false
	}
	if len(raw) == 0 || raw[0] != '{' {
		WriteError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return false
	}
	bodyDecoder := json.NewDecoder(bytes.NewReader(raw))
	bodyDecoder.DisallowUnknownFields()
	if err := bodyDecoder.Decode(payload); err != nil {
		WriteError(response, http.StatusBadRequest, 40001, "invalid_json", context)
		return false
	}
	return true
}

// WriteJSON 写入统一 JSON 响应，保证 Content-Type 和 envelope 结构一致。
func WriteJSON(response http.ResponseWriter, status int, payload Response) {
	body, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		status = http.StatusInternalServerError
		body = mustMarshalResponse(Response{
			Code:      50000,
			Message:   "internal_error",
			Data:      nil,
			RequestID: payload.RequestID,
			TraceID:   payload.TraceID,
		})
	}

	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_, _ = response.Write(append(body, '\n'))
}

// mustMarshalResponse 只用于内部固定 envelope；固定字段不可编码时应尽早暴露开发错误。
func mustMarshalResponse(payload Response) []byte {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return body
}

// tokenMatches 使用常量时间比较校验 runtime token，避免在安全边界上使用普通字符串比较。
func tokenMatches(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

// traceHeaderOrNewID 只接受短 ASCII 追踪字段，避免异常头值进入响应和日志。
func traceHeaderOrNewID(value string) string {
	if isSafeTraceHeader(value) {
		return value
	}
	return newID()
}

// isSafeTraceHeader 校验 requestId/traceId 的可回显字符集和长度。
func isSafeTraceHeader(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, item := range value {
		if item >= 'a' && item <= 'z' ||
			item >= 'A' && item <= 'Z' ||
			item >= '0' && item <= '9' ||
			item == '-' ||
			item == '_' ||
			item == '.' {
			continue
		}
		return false
	}
	return true
}

// newID 生成 requestId/traceId，在随机源异常时退化为时间戳编码以保留可观测性。
func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}

	// 系统随机源异常时仍返回可追踪 ID，避免错误响应缺少 requestId/traceId。
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}
