package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

const tokenHeader = "X-Invest-Compass-Token"

type Config struct {
	Version    string
	Token      string
	DBStatus   string
	Ready      bool
	OnShutdown func()
}

type apiResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

type healthData struct {
	Version  string `json:"version"`
	DBStatus string `json:"dbStatus"`
}

type appHandler struct {
	config Config
}

// NewHandler 创建本地 HTTP handler，并把当前进程的 ready 状态和 token 边界注入进去。
func NewHandler(config Config) http.Handler {
	return appHandler{config: config}
}

// ServeHTTP 统一处理本地 core API 的入口约束，确保所有请求先经过 POST 限制。
func (handler appHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	context := requestContextFrom(request)

	// 本地 core API 统一只接受 POST，避免浏览器或外部工具通过 GET 探测业务接口。
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, 40500, "method_not_allowed", context)
		return
	}

	switch request.URL.Path {
	case "/internal/health":
		handler.handleHealth(response, request, context)
	case "/internal/shutdown":
		handler.handleShutdown(response, request, context)
	default:
		writeError(response, http.StatusNotFound, 40400, "not_found", context)
	}
}

// handleHealth 返回最小健康信息，只在 sidecar 握手完成且 token 正确时可用。
func (handler appHandler) handleHealth(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.config.Ready || handler.config.Token == "" {
		writeError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return
	}
	if !tokenMatches(request.Header.Get(tokenHeader), handler.config.Token) {
		writeError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:    0,
		Message: "ok",
		Data: healthData{
			Version:  handler.config.Version,
			DBStatus: handler.config.DBStatus,
		},
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// handleShutdown 处理 Rust 生命周期管理层发起的关闭请求，并复用 runtime token 安全边界。
func (handler appHandler) handleShutdown(response http.ResponseWriter, request *http.Request, context requestContext) {
	if !handler.config.Ready || handler.config.Token == "" {
		writeError(response, http.StatusServiceUnavailable, 50300, "core_not_ready", context)
		return
	}
	if !tokenMatches(request.Header.Get(tokenHeader), handler.config.Token) {
		writeError(response, http.StatusUnauthorized, 40100, "unauthorized", context)
		return
	}

	writeJSON(response, http.StatusOK, apiResponse{
		Code:      0,
		Message:   "ok",
		Data:      map[string]string{"status": "shutting_down"},
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})

	if handler.config.OnShutdown != nil {
		handler.config.OnShutdown()
	}
}

type requestContext struct {
	requestID string
	traceID   string
}

// requestContextFrom 从请求头提取追踪 ID，缺失时生成本地 ID 方便排障关联。
func requestContextFrom(request *http.Request) requestContext {
	requestID := request.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = newID()
	}

	traceID := request.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = newID()
	}

	return requestContext{
		requestID: requestID,
		traceID:   traceID,
	}
}

// writeError 用统一错误结构返回失败原因，避免各 handler 自行拼响应。
func writeError(response http.ResponseWriter, status int, code int, message string, context requestContext) {
	writeJSON(response, status, apiResponse{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: context.requestID,
		TraceID:   context.traceID,
	})
}

// writeJSON 写入统一 JSON 响应，保证 Content-Type 和 envelope 结构一致。
func writeJSON(response http.ResponseWriter, status int, payload apiResponse) {
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
