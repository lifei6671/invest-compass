package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultClientName   = "crawler"
	defaultTimeout      = 15 * time.Second
	defaultMaxBodyBytes = int64(2 * 1024 * 1024)
)

var (
	// ErrRendererUnavailable 表示调用方请求动态渲染，但当前 Client 没有注入 Renderer。
	ErrRendererUnavailable = errors.New("crawler renderer unavailable")
)

// Config 描述外部数据抓取 Client 的可配置边界。
type Config struct {
	Name         string
	BaseURL      string
	Headers      map[string]string
	Timeout      time.Duration
	MaxBodyBytes int64
	Decoder      BodyDecoder
	HTTPClient   *http.Client
	Renderer     Renderer
}

// BodyDecoder 将响应体字节转换为文本，适合为 GB18030 等非 UTF-8 接口注入解码器。
type BodyDecoder func([]byte) (string, error)

// Request 描述一次外部数据抓取请求。
type Request struct {
	URL           string
	Method        string
	Query         map[string]string
	Headers       map[string]string
	Body          []byte
	Decoder       BodyDecoder
	WaitVisible   string
	RequireRender bool
	Headless      bool
}

// Result 是外部数据抓取后的统一结果。
type Result struct {
	URL        string
	Body       string
	BodyBytes  []byte
	StatusCode int
	Header     http.Header
	Rendered   bool
	FetchedAt  time.Time
}

// Renderer 是动态页面渲染器扩展点，调用方可用 chromedp、Playwright 或其他实现注入。
type Renderer interface {
	RenderHTML(ctx context.Context, request Request) (Result, error)
}

// Client 是可复用的外部数据抓取类库入口。
type Client struct {
	name         string
	baseURL      *url.URL
	headers      map[string]string
	timeout      time.Duration
	maxBodyBytes int64
	decoder      BodyDecoder
	httpClient   *http.Client
	renderer     Renderer
}

// NewClient 创建外部数据抓取 Client，并复制配置避免调用方后续修改影响运行中实例。
func NewClient(config Config) (*Client, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = defaultClientName
	}

	baseURL, err := parseOptionalBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	headers, err := normalizeHeaders(config.Headers)
	if err != nil {
		return nil, err
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("crawler %s invalid timeout", name)
	}

	maxBodyBytes := config.MaxBodyBytes
	if maxBodyBytes == 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	if maxBodyBytes < 0 {
		return nil, fmt.Errorf("crawler %s invalid max body bytes", name)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		name:         name,
		baseURL:      baseURL,
		headers:      headers,
		timeout:      timeout,
		maxBodyBytes: maxBodyBytes,
		decoder:      config.Decoder,
		httpClient:   httpClient,
		renderer:     config.Renderer,
	}, nil
}

// FetchHTML 抓取 HTML 文本，按请求选择标准 HTTP 或注入式动态渲染。
func (client *Client) FetchHTML(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("crawler %s missing context", client.name)
	}

	normalized, err := client.normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	if normalized.RequireRender {
		if client.renderer == nil {
			return Result{}, ErrRendererUnavailable
		}
		result, err := client.renderer.RenderHTML(timeoutCtx, normalized)
		if err != nil {
			return Result{}, fmt.Errorf("crawler %s render html failed: %w", client.name, err)
		}
		result.Rendered = true
		return result, nil
	}

	return client.fetchStaticHTML(timeoutCtx, normalized)
}

// Fetch 执行受控 HTTP 请求，返回文本和原始字节，适合 JSON、JS 包裹体和轻量文本接口。
func (client *Client) Fetch(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("crawler %s missing context", client.name)
	}

	normalized, err := client.normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	return client.fetchStaticHTML(timeoutCtx, normalized)
}

// FetchJSON 执行 HTTP 请求并将响应体按 JSON 解码到 target。
func (client *Client) FetchJSON(ctx context.Context, request Request, target any) (Result, error) {
	if target == nil {
		return Result{}, fmt.Errorf("crawler %s missing json target", client.name)
	}
	result, err := client.Fetch(ctx, request)
	if err != nil {
		return Result{}, err
	}
	if err := json.Unmarshal([]byte(result.Body), target); err != nil {
		return Result{}, fmt.Errorf("crawler %s decode json failed: %w", client.name, err)
	}
	return result, nil
}

// PostJSON 将 payload 编码为 JSON 后 POST，并把 JSON 响应解码到 target。
func (client *Client) PostJSON(ctx context.Context, request Request, payload any, target any) (Result, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("crawler %s encode json failed: %w", client.name, err)
	}
	request.Method = http.MethodPost
	request.Body = body
	if request.Headers == nil {
		request.Headers = make(map[string]string)
	}
	if request.Headers["Content-Type"] == "" && request.Headers["content-type"] == "" {
		request.Headers["Content-Type"] = "application/json"
	}
	if request.Headers["Accept"] == "" && request.Headers["accept"] == "" {
		request.Headers["Accept"] = "application/json"
	}
	return client.FetchJSON(ctx, request, target)
}

// fetchStaticHTML 通过标准 HTTP 抓取文本或 JSON 数据。
func (client *Client) fetchStaticHTML(ctx context.Context, request Request) (Result, error) {
	method := request.Method
	if method == "" {
		method = http.MethodGet
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, request.URL, bytes.NewReader(request.Body))
	if err != nil {
		return Result{}, fmt.Errorf("crawler %s build request failed: %w", client.name, err)
	}
	for key, value := range request.Headers {
		httpRequest.Header.Set(key, value)
	}

	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return Result{}, fmt.Errorf("crawler %s fetch html failed: %w", client.name, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("crawler %s unexpected status code %d", client.name, response.StatusCode)
	}

	body, err := readLimitedBody(response.Body, client.maxBodyBytes)
	if err != nil {
		return Result{}, fmt.Errorf("crawler %s read html failed: %w", client.name, err)
	}
	bodyText, err := client.decodeBody(request, body)
	if err != nil {
		return Result{}, err
	}

	return Result{
		URL:        request.URL,
		Body:       bodyText,
		BodyBytes:  body,
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		FetchedAt:  time.Now().UTC(),
	}, nil
}

// normalizeRequest 解析 URL 并合并默认请求头和本次请求头。
func (client *Client) normalizeRequest(request Request) (Request, error) {
	normalizedURL, err := client.resolveURL(request.URL)
	if err != nil {
		return Request{}, err
	}
	normalizedURL, err = applyQuery(normalizedURL, request.Query)
	if err != nil {
		return Request{}, fmt.Errorf("crawler %s invalid query url", client.name)
	}
	headers, err := normalizeHeaders(request.Headers)
	if err != nil {
		return Request{}, err
	}

	mergedHeaders := make(map[string]string, len(client.headers)+len(headers))
	for key, value := range client.headers {
		mergedHeaders[key] = value
	}
	for key, value := range headers {
		mergedHeaders[key] = value
	}

	request.URL = normalizedURL
	request.Headers = mergedHeaders
	return request, nil
}

// decodeBody 使用请求级或客户端级解码器生成文本正文，默认按 UTF-8 字节转换。
func (client *Client) decodeBody(request Request, body []byte) (string, error) {
	decoder := request.Decoder
	if decoder == nil {
		decoder = client.decoder
	}
	if decoder == nil {
		return string(body), nil
	}
	decoded, err := decoder(body)
	if err != nil {
		return "", fmt.Errorf("crawler %s decode body failed: %w", client.name, err)
	}
	return decoded, nil
}

// resolveURL 将请求 URL 解析为绝对 HTTP(S) URL，必要时基于 BaseURL 解析相对路径。
func (client *Client) resolveURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("crawler %s empty url", client.name)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("crawler %s invalid url", client.name)
	}
	if !parsed.IsAbs() {
		if client.baseURL == nil {
			return "", fmt.Errorf("crawler %s relative url requires base url", client.name)
		}
		parsed = client.baseURL.ResolveReference(parsed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("crawler %s unsupported url scheme %q", client.name, parsed.Scheme)
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

// applyQuery 将 query 参数写入 URL，保留原始 URL 上已有参数。
func applyQuery(rawURL string, queryValues map[string]string) (string, error) {
	if len(queryValues) == 0 {
		return rawURL, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range queryValues {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// parseOptionalBaseURL 解析可选 BaseURL，并限制为 HTTP(S) 根地址。
func parseOptionalBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("crawler invalid base url")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if !parsed.IsAbs() || (scheme != "http" && scheme != "https") {
		return nil, fmt.Errorf("crawler base url must use http or https")
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed, nil
}

// normalizeHeaders 复制并校验请求头，阻止 CRLF 注入和空 Header 名。
func normalizeHeaders(headers map[string]string) (map[string]string, error) {
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		headerName := strings.TrimSpace(key)
		if headerName == "" {
			return nil, errors.New("crawler header name is empty")
		}
		if containsLineBreak(headerName) || containsLineBreak(value) {
			return nil, errors.New("crawler header contains line break")
		}
		normalized[http.CanonicalHeaderKey(headerName)] = strings.TrimSpace(value)
	}
	return normalized, nil
}

// containsLineBreak 判断字符串是否包含可能造成 Header 注入的换行字符。
func containsLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

// readLimitedBody 读取响应体，并在超过上限时显式返回错误。
func readLimitedBody(reader io.Reader, maxBodyBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBodyBytes)
	}
	return body, nil
}
