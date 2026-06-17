package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHealthRejectsNonPost 验证本地 core API 默认拒绝非 POST 请求。
func TestHealthRejectsNonPost(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "method_not_allowed")
}

// TestHealthRequiresReadyToken 验证 sidecar 未完成握手前健康检查不可用。
func TestHealthRequiresReadyToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "",
		DBStatus: "not_configured",
		Ready:    false,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "core_not_ready")
}

// TestHealthRejectsInvalidToken 验证错误 runtime token 不能访问健康检查。
func TestHealthRejectsInvalidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)
	request.Header.Set("X-Invest-Compass-Token", "wrong-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "unauthorized")
}

// TestShutdownRequiresValidToken 验证关闭接口必须经过 runtime token 校验。
func TestShutdownRequiresValidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		OnShutdown: func() {
			t.Fatal("shutdown callback must not run for invalid token")
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/shutdown", nil)
	request.Header.Set("X-Invest-Compass-Token", "wrong-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	assertErrorEnvelope(t, recorder.Body.String(), "unauthorized")
}

// TestShutdownCallsConfiguredCallback 验证关闭接口通过后会触发进程生命周期清理回调。
func TestShutdownCallsConfiguredCallback(t *testing.T) {
	called := false
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
		OnShutdown: func() {
			called = true
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/shutdown", nil)
	request.Header.Set("X-Invest-Compass-Token", "test-token")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !called {
		t.Fatal("expected shutdown callback to run")
	}
}

// TestRecoverHTTPRedactsPanicError 验证 panic 会转成统一错误响应，且不会泄露异常中的密钥。
func TestRecoverHTTPRedactsPanicError(t *testing.T) {
	handler := recoverHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(errors.New("provider failed with Authorization: Bearer sk-panic-secret"))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", nil)
	request.Header.Set("X-Request-Id", "req-panic")
	request.Header.Set("X-Trace-Id", "trace-panic")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "sk-panic-secret") {
		t.Fatalf("panic response leaked secret: %s", recorder.Body.String())
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal panic response: %v", err)
	}
	if response.Code != 50000 || response.Message != "internal_error" {
		t.Fatalf("unexpected panic envelope: %+v", response)
	}
	if response.RequestID != "req-panic" || response.TraceID != "trace-panic" {
		t.Fatalf("expected propagated ids, got requestId=%q traceId=%q", response.RequestID, response.TraceID)
	}
}

// TestHealthReturnsVersionAndDBStatusWithValidToken 验证合法 token 可获取最小健康信息。
func TestHealthReturnsVersionAndDBStatusWithValidToken(t *testing.T) {
	handler := NewHandler(Config{
		Version:  "0.1.0",
		Token:    "test-token",
		DBStatus: "not_configured",
		Ready:    true,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/health", strings.NewReader("{}"))
	request.Header.Set("X-Invest-Compass-Token", "test-token")
	request.Header.Set("X-Request-Id", "req-test")
	request.Header.Set("X-Trace-Id", "trace-test")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if response.Code != 0 || response.Message != "ok" {
		t.Fatalf("unexpected response envelope: %+v", response)
	}
	if response.RequestID != "req-test" || response.TraceID != "trace-test" {
		t.Fatalf("expected propagated ids, got requestId=%q traceId=%q", response.RequestID, response.TraceID)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected health data object, got %T", response.Data)
	}
	if data["version"] != "0.1.0" || data["dbStatus"] != "not_configured" {
		t.Fatalf("unexpected health data: %#v", data)
	}
}

// assertErrorEnvelope 校验错误响应必须包含统一 envelope 和追踪 ID。
func assertErrorEnvelope(t *testing.T, body string, message string) {
	t.Helper()

	var response apiResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if response.Code == 0 {
		t.Fatalf("expected non-zero error code, got response %+v", response)
	}
	if response.Message != message {
		t.Fatalf("expected message %q, got %q", message, response.Message)
	}
	if response.RequestID == "" || response.TraceID == "" {
		t.Fatalf("expected requestId and traceId, got response %+v", response)
	}
}
