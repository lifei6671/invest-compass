package crawler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClientFetchHTMLUsesHeadersAndReturnsBody 验证标准 HTTP 抓取会带上默认头并返回 HTML 正文。
func TestClientFetchHTMLUsesHeadersAndReturnsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "InvestCompassBot/1.0" {
			t.Fatalf("unexpected user agent: %s", request.Header.Get("User-Agent"))
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Name:    "test-crawler",
		Headers: map[string]string{"User-Agent": "InvestCompassBot/1.0"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	result, err := client.FetchHTML(context.Background(), Request{URL: server.URL})
	if err != nil {
		t.Fatalf("FetchHTML returned error: %v", err)
	}
	if result.Body != "<html><body>ok</body></html>" || result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestClientFetchHTMLRejectsUnsafeURL 验证类库只允许 HTTP(S)，避免 file/javascript 等危险 scheme。
func TestClientFetchHTMLRejectsUnsafeURL(t *testing.T) {
	client, err := NewClient(Config{Name: "test-crawler"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchHTML(context.Background(), Request{URL: "file:///etc/passwd"})
	if err == nil {
		t.Fatal("expected unsafe URL error")
	}
	if !strings.Contains(err.Error(), "unsupported url scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientFetchHTMLFailsOnUnexpectedStatus 验证非 2xx 响应会显式失败而不是返回空正文和 bool。
func TestClientFetchHTMLFailsOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "blocked", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "test-crawler", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchHTML(context.Background(), Request{URL: server.URL})
	if err == nil {
		t.Fatal("expected status error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected status code in error, got: %v", err)
	}
}

// TestClientFetchHTMLHonorsMaxBodyBytes 验证最大响应体限制会阻止异常大页面进入内存。
func TestClientFetchHTMLHonorsMaxBodyBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("0123456789abcdef"))
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "test-crawler", Timeout: time.Second, MaxBodyBytes: 8})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchHTML(context.Background(), Request{URL: server.URL})
	if err == nil {
		t.Fatal("expected body limit error")
	}
	if !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientFetchHTMLUsesRendererWhenRequested 验证动态页面抓取通过注入式 Renderer 扩展，不强绑定浏览器依赖。
func TestClientFetchHTMLUsesRendererWhenRequested(t *testing.T) {
	renderer := &recordingRenderer{body: "<html><body>rendered</body></html>"}
	client, err := NewClient(Config{Name: "test-crawler", Renderer: renderer})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	result, err := client.FetchHTML(context.Background(), Request{
		URL:           "https://example.com/page",
		WaitVisible:   "#app",
		RequireRender: true,
		Headless:      true,
	})
	if err != nil {
		t.Fatalf("FetchHTML returned error: %v", err)
	}
	if result.Body != renderer.body || renderer.request.WaitVisible != "#app" || !renderer.request.Headless {
		t.Fatalf("renderer request was not preserved: result=%+v request=%+v", result, renderer.request)
	}
}

// TestClientFetchHTMLReturnsRendererMissing 验证请求动态渲染但未注入 Renderer 时会快速失败。
func TestClientFetchHTMLReturnsRendererMissing(t *testing.T) {
	client, err := NewClient(Config{Name: "test-crawler"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchHTML(context.Background(), Request{
		URL:           "https://example.com/page",
		RequireRender: true,
	})
	if err == nil {
		t.Fatal("expected renderer missing error")
	}
	if !errors.Is(err, ErrRendererUnavailable) {
		t.Fatalf("expected ErrRendererUnavailable, got: %v", err)
	}
}

// TestClientFetchJSONPostsStructuredBody 验证 JSON POST 能覆盖通用外部数据接口。
func TestClientFetchJSONPostsStructuredBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type: %s", request.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"operation":"list"`) {
			t.Fatalf("unexpected request body: %s", body)
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"items":[{"id":"row-1","name":"example"}]}}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "json-api", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]string `json:"items"`
		} `json:"data"`
	}
	result, err := client.PostJSON(context.Background(), Request{URL: server.URL}, map[string]any{
		"operation": "list",
	}, &response)
	if err != nil {
		t.Fatalf("PostJSON returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK || response.Code != 0 || response.Data.Items[0]["name"] != "example" {
		t.Fatalf("unexpected json response: result=%+v response=%+v", result, response)
	}
}

// TestClientFetchJSONUsesDecodedBody 验证 JSON 解码使用请求解码器产出的文本，覆盖非 UTF-8 接口。
func TestClientFetchJSONUsesDecodedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte{0xff, 0xfe})
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "json-decoder-api", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	var response struct {
		Name string `json:"name"`
	}
	_, err = client.FetchJSON(context.Background(), Request{
		URL: server.URL,
		Decoder: func(body []byte) (string, error) {
			if len(body) != 2 {
				t.Fatalf("unexpected raw body length: %d", len(body))
			}
			return `{"name":"已解码"}`, nil
		},
	}, &response)
	if err != nil {
		t.Fatalf("FetchJSON returned error: %v", err)
	}
	if response.Name != "已解码" {
		t.Fatalf("unexpected decoded json response: %+v", response)
	}
}

// TestClientFetchMergesBaseURLAndQuery 验证请求可以基于 BaseURL 拼接路径和 query。
func TestClientFetchMergesBaseURLAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/quote" || request.URL.Query().Get("symbol") != "CN:SH:600519" {
			t.Fatalf("unexpected url: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "query-api", BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	result, err := client.Fetch(context.Background(), Request{
		URL:   "/api/quote",
		Query: map[string]string{"symbol": "CN:SH:600519"},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if result.Body != "ok" {
		t.Fatalf("unexpected response body: %s", result.Body)
	}
}

// TestClientFetchUsesRequestDecoder 验证调用方可以为 GB18030 等非 UTF-8 响应注入文本解码器。
func TestClientFetchUsesRequestDecoder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte{0x01, 0x02})
	}))
	defer server.Close()

	client, err := NewClient(Config{Name: "decoder-api", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	result, err := client.Fetch(context.Background(), Request{
		URL: server.URL,
		Decoder: func(body []byte) (string, error) {
			return "decoded:" + string(rune('0'+len(body))), nil
		},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if result.Body != "decoded:2" {
		t.Fatalf("unexpected decoded body: %s", result.Body)
	}
}

type recordingRenderer struct {
	body    string
	request Request
}

// RenderHTML 记录动态渲染请求并返回固定 HTML，用于验证 Renderer 扩展契约。
func (renderer *recordingRenderer) RenderHTML(_ context.Context, request Request) (Result, error) {
	renderer.request = request
	return Result{
		URL:        request.URL,
		Body:       renderer.body,
		Rendered:   true,
		FetchedAt:  time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		StatusCode: http.StatusOK,
	}, nil
}
