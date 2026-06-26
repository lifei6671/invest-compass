package datasourcecredential

import (
	"net/http"
	"time"
)

// AuthType 表示数据源凭据认证方式。
type AuthType string

const (
	// AuthTypeNone 表示 Provider 不需要凭据。
	AuthTypeNone AuthType = "none"
	// AuthTypeAPIKey 表示 Provider 使用 API Key。
	AuthTypeAPIKey AuthType = "api_key"
	// AuthTypeCookie 表示 Provider 使用 Cookie。
	AuthTypeCookie AuthType = "cookie"
	// AuthTypeBearerToken 表示 Provider 使用 Bearer Token。
	AuthTypeBearerToken AuthType = "bearer_token"
	// AuthTypeCustomHeader 表示 Provider 使用自定义 Header。
	AuthTypeCustomHeader AuthType = "custom_header"
)

// Status 表示凭据配置和健康状态。
type Status string

const (
	// StatusNormal 表示凭据已配置且真实连接测试正常。
	StatusNormal Status = "normal"
	// StatusNotConfigured 表示凭据尚未配置。
	StatusNotConfigured Status = "not_configured"
	// StatusExpired 表示凭据已过期。
	StatusExpired Status = "expired"
	// StatusExpiring 表示凭据即将过期。
	StatusExpiring Status = "expiring"
	// StatusLimited 表示 Provider 部分能力可用但存在已知受限项。
	StatusLimited Status = "limited"
	// StatusFailed 表示凭据配置或真实连接测试失败。
	StatusFailed Status = "failed"
)

// Provider 描述凭据管理页展示的数据源目录项。
type Provider struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Capability string   `json:"capability"`
	Status     Status   `json:"status"`
	AuthType   AuthType `json:"authType"`
	IconType   string   `json:"iconType"`
}

// Config 描述单个 Provider 的凭据配置展示模型，不包含真实凭据明文。
type Config struct {
	ProviderID         string     `json:"providerId"`
	ProviderName       string     `json:"providerName"`
	Capability         string     `json:"capability"`
	AuthType           AuthType   `json:"authType"`
	BaseURL            string     `json:"baseUrl"`
	CredentialStatus   Status     `json:"credentialStatus"`
	ExpiresAt          string     `json:"expiresAt"`
	TimeoutSeconds     int        `json:"timeoutSeconds"`
	RateLimitPerMinute int        `json:"rateLimitPerMinute"`
	MaskedCredential   string     `json:"maskedCredential"`
	Note               string     `json:"note"`
	LastTestResult     TestResult `json:"lastTestResult"`
}

// SaveRequest 是保存凭据配置的 service 输入；Credential 只允许用于本次保存。
type SaveRequest struct {
	Config     Config `json:"config"`
	Credential string `json:"credential"`
}

// RuntimeCredential 是运行时注入 Provider 的凭据载体，只能在 Go core 内部短暂使用。
type RuntimeCredential struct {
	ProviderID  string
	AuthType    AuthType
	Cookie      string
	APIKey      string
	BearerToken string
	HeaderName  string
	HeaderValue string
}

// TestRequest 是真实连接测试输入，会向 Provider 发起受控 HTTP 预检。
type TestRequest struct {
	ProviderID string `json:"providerId"`
	Target     string `json:"target"`
}

// TestResult 描述凭据真实连接测试结果。
type TestResult struct {
	Status         string   `json:"status"`
	ResponseTimeMS int      `json:"responseTimeMs,omitempty"`
	TestedAt       string   `json:"testedAt,omitempty"`
	Messages       []string `json:"messages"`
}

// Overview 汇总凭据配置状态。
type Overview struct {
	ConfiguredCount   int `json:"configuredCount"`
	ExpiringSoonCount int `json:"expiringSoonCount"`
	ExpiredCount      int `json:"expiredCount"`
}

// HealthItem 描述凭据页底部的调用限制和健康状态。
type HealthItem struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	RateLimitText string `json:"rateLimitText"`
}

// OperationLog 描述凭据操作日志展示项，日志不包含任何真实凭据内容。
type OperationLog struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Status string `json:"status"`
	Time   string `json:"time"`
}

// ListView 是凭据管理页一次性加载所需的全部安全展示数据。
type ListView struct {
	Providers        []Provider        `json:"providers"`
	Configs          map[string]Config `json:"configs"`
	SelectedProvider string            `json:"selectedProviderId"`
	TestTargets      []TestTarget      `json:"testTargets"`
	TestResult       TestResult        `json:"testResult"`
	Overview         Overview          `json:"overview"`
	HealthItems      []HealthItem      `json:"healthItems"`
	OperationLogs    []OperationLog    `json:"operationLogs"`
}

// TestTarget 描述凭据页可选的真实连接测试目标。
type TestTarget struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Clock 为测试提供可控时间来源。
type Clock interface {
	Now() time.Time
}

// SystemClock 使用当前系统时间。
type SystemClock struct{}

// Now 返回当前 UTC 时间，避免展示和持久化混用本地时区。
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// HTTPDoer 是真实连接测试依赖的最小 HTTP client 边界，便于单元测试替换。
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}
