package httpx

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// TokenHeader 是 Rust 代理访问 Go core 时必须携带的运行期 token header。
const TokenHeader = "X-Invest-Compass-Token"

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
	requestID := request.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = newID()
	}

	traceID := request.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = newID()
	}

	return RequestContext{
		RequestID: requestID,
		TraceID:   traceID,
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

// WriteJSON 写入统一 JSON 响应，保证 Content-Type 和 envelope 结构一致。
func WriteJSON(response http.ResponseWriter, status int, payload Response) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		panic(err)
	}
}

// tokenMatches 使用常量时间比较校验 runtime token，避免在安全边界上使用普通字符串比较。
func tokenMatches(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
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
