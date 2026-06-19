package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteJSONHandlesMarshalFailureWithoutPanic 验证 HTTP 边界遇到不可编码响应时返回稳定 500，
// 避免单个 handler 响应异常击穿整个请求链路。
func TestWriteJSONHandlesMarshalFailureWithoutPanic(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}

	assertNotPanics(t, func() {
		WriteOK(recorder, map[string]any{"bad": func() {}}, context)
	})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response must be valid json: %v", err)
	}
	if response.Code != 50000 || response.Message != "internal_error" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.RequestID != "req-1" || response.TraceID != "trace-1" {
		t.Fatalf("trace fields must be preserved: %+v", response)
	}
}

// TestDecodeJSONAcceptsSmallBody 验证统一请求体解码入口仍接受正常 JSON 负载。
func TestDecodeJSONAcceptsSmallBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"name":"demo"}`))
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}
	var payload struct {
		Name string `json:"name"`
	}

	if !DecodeJSON(recorder, request, context, &payload) {
		t.Fatalf("expected small body to decode successfully")
	}
	if payload.Name != "demo" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

// TestDecodeJSONRejectsOversizedBody 验证本地 API 在 JSON 解码前限制请求体大小，避免异常请求占用内存。
func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"name":"`+strings.Repeat("a", MaxJSONBodyBytes)+`"}`))
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}
	var payload struct {
		Name string `json:"name"`
	}

	if DecodeJSON(recorder, request, context, &payload) {
		t.Fatalf("expected oversized body to be rejected")
	}
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response must be valid json: %v", err)
	}
	if response.Code != 41300 || response.Message != "request_body_too_large" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

// TestDecodeJSONRejectsTrailingContent 验证请求体只能包含一个 JSON 文档，避免拼接垃圾内容被业务层忽略。
func TestDecodeJSONRejectsTrailingContent(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"name":"demo"}{"extra":true}`))
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}
	var payload struct {
		Name string `json:"name"`
	}

	if DecodeJSON(recorder, request, context, &payload) {
		t.Fatalf("expected trailing content to be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response must be valid json: %v", err)
	}
	if response.Code != 40001 || response.Message != "invalid_json" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

// TestDecodeJSONRejectsUnknownFields 验证内部 API 固定 schema 不接受未知字段，避免请求拼写错误被静默忽略。
func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"name":"demo","unknown":true}`))
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}
	var payload struct {
		Name string `json:"name"`
	}

	if DecodeJSON(recorder, request, context, &payload) {
		t.Fatalf("expected unknown field to be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response must be valid json: %v", err)
	}
	if response.Code != 40001 || response.Message != "invalid_json" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

// TestDecodeJSONRejectsNullDocument 验证内部 API 只接受对象请求体，避免 null 被解码成零值结构体。
func TestDecodeJSONRejectsNullDocument(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`null`))
	recorder := httptest.NewRecorder()
	context := RequestContext{RequestID: "req-1", TraceID: "trace-1"}
	var payload struct {
		Name string `json:"name"`
	}

	if DecodeJSON(recorder, request, context, &payload) {
		t.Fatalf("expected null document to be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response must be valid json: %v", err)
	}
	if response.Code != 40001 || response.Message != "invalid_json" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

// TestContextFromSanitizesTraceHeaders 验证请求追踪字段只接受短横线安全字符，避免响应和日志回显异常头值。
func TestContextFromSanitizesTraceHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	request.Header.Set("X-Request-Id", "req-123_ABC")
	request.Header.Set("X-Trace-Id", "trace\nwith secret")

	context := ContextFrom(request)

	if context.RequestID != "req-123_ABC" {
		t.Fatalf("expected valid request id to be preserved, got %q", context.RequestID)
	}
	if context.TraceID == "trace\nwith secret" ||
		strings.Contains(context.TraceID, "\n") ||
		strings.Contains(context.TraceID, "secret") {
		t.Fatalf("expected invalid trace id to be regenerated, got %q", context.TraceID)
	}
}

// TestContextFromRejectsOversizedTraceHeaders 验证过长追踪头不会进入统一响应 envelope。
func TestContextFromRejectsOversizedTraceHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	request.Header.Set("X-Request-Id", strings.Repeat("a", 129))
	request.Header.Set("X-Trace-Id", "trace-ok")

	context := ContextFrom(request)

	if context.RequestID == strings.Repeat("a", 129) || len(context.RequestID) > 128 {
		t.Fatalf("expected oversized request id to be regenerated, got %q", context.RequestID)
	}
	if context.TraceID != "trace-ok" {
		t.Fatalf("expected valid trace id to be preserved, got %q", context.TraceID)
	}
}

// assertNotPanics 捕获 panic，使测试能明确表达 HTTP 边界不能崩溃。
func assertNotPanics(t *testing.T, run func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	run()
}
