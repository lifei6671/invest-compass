package datasourcecredential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

const (
	testStatusSuccess  = "success"
	testStatusFailed   = "failed"
	testStatusUntested = "untested"
)

// ErrCredentialNotConfigured 表示 Provider 需要凭据但当前没有可用于运行时注入的密文。
var ErrCredentialNotConfigured = errors.New("data source credential not configured")

var providerCatalog = []Config{
	{
		ProviderID:         "eastmoney",
		ProviderName:       "EastMoney",
		Capability:         "行情 / K线",
		AuthType:           AuthTypeNone,
		BaseURL:            "https://quote.eastmoney.com",
		CredentialStatus:   StatusNormal,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 60,
		MaskedCredential:   "无需凭据",
	},
	{
		ProviderID:         "akshare",
		ProviderName:       "AkShare",
		Capability:         "基础数据",
		AuthType:           AuthTypeNone,
		BaseURL:            "https://akshare.akfamily.xyz",
		CredentialStatus:   StatusNormal,
		TimeoutSeconds:     20,
		RateLimitPerMinute: 45,
		MaskedCredential:   "无需凭据",
	},
	{
		ProviderID:         "alpha-vantage",
		ProviderName:       "Alpha Vantage",
		Capability:         "海外行情",
		AuthType:           AuthTypeAPIKey,
		BaseURL:            "https://www.alphavantage.co",
		CredentialStatus:   StatusNotConfigured,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 12,
		MaskedCredential:   "key=****",
	},
	{
		ProviderID:         "cls",
		ProviderName:       "财联社",
		Capability:         "快讯 / 行业事件 / 日历",
		AuthType:           AuthTypeCookie,
		BaseURL:            "https://www.cls.cn",
		CredentialStatus:   StatusNotConfigured,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 30,
		MaskedCredential:   "Cookie=****",
	},
	{
		ProviderID:         "xueqiu",
		ProviderName:       "雪球",
		Capability:         "讨论热度",
		AuthType:           AuthTypeCookie,
		BaseURL:            "https://xueqiu.com",
		CredentialStatus:   StatusNotConfigured,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 20,
		MaskedCredential:   "Cookie=****",
	},
	{
		ProviderID:         "custom-http",
		ProviderName:       "Custom HTTP",
		Capability:         "自定义接口",
		AuthType:           AuthTypeBearerToken,
		BaseURL:            "https://api.example.com",
		CredentialStatus:   StatusNotConfigured,
		TimeoutSeconds:     15,
		RateLimitPerMinute: 30,
		MaskedCredential:   "Bearer ****",
	},
}

var providerIconTypes = map[string]string{
	"eastmoney":     "eastmoney",
	"akshare":       "akshare",
	"alpha-vantage": "alpha",
	"cls":           "cls",
	"xueqiu":        "xueqiu",
	"custom-http":   "custom",
}

// Store 是凭据 service 依赖的 DAO 边界。
type Store interface {
	SaveDataSourceCredential(ctx context.Context, credential *model.DataSourceCredential) error
	ListDataSourceCredentials(ctx context.Context) ([]model.DataSourceCredential, error)
	GetDataSourceCredential(ctx context.Context, providerID string) (model.DataSourceCredential, bool, error)
	ClearDataSourceCredential(ctx context.Context, providerID string) error
}

// Service 管理数据源凭据配置，负责脱敏和加密边界。
type Service struct {
	Store       Store
	KeyProvider KeyProvider
	Clock       Clock
}

// NewService 构造数据源凭据 service。
func NewService(store Store, keyProvider KeyProvider) Service {
	return Service{Store: store, KeyProvider: keyProvider, Clock: SystemClock{}}
}

// List 返回凭据管理页所需的安全展示数据。
func (service Service) List(ctx context.Context) (ListView, error) {
	if service.Store == nil {
		return ListView{}, fmt.Errorf("data source credential store is required")
	}
	stored, err := service.Store.ListDataSourceCredentials(ctx)
	if err != nil {
		return ListView{}, err
	}
	configs := catalogConfigMap()
	for _, item := range stored {
		configs[item.ProviderID] = modelToConfig(item, configs[item.ProviderID])
	}
	providers := providersFromConfigs(configs)
	return ListView{
		Providers:        providers,
		Configs:          configs,
		SelectedProvider: defaultSelectedProvider(configs),
		TestTargets:      defaultTestTargets(),
		TestResult:       defaultTestResult(configs),
		Overview:         overviewFromConfigs(configs, service.now()),
		HealthItems:      healthItemsFromConfigs(configs),
		OperationLogs:    operationLogs(stored),
	}, nil
}

// Save 校验并保存凭据配置，明文只在本次调用内用于加密和脱敏。
func (service Service) Save(ctx context.Context, request SaveRequest) (Config, error) {
	if service.Store == nil {
		return Config{}, fmt.Errorf("data source credential store is required")
	}
	config, err := normalizeConfig(request.Config)
	if err != nil {
		return Config{}, err
	}
	var existing model.DataSourceCredential
	var exists bool
	if existing, exists, err = service.Store.GetDataSourceCredential(ctx, config.ProviderID); err != nil {
		return Config{}, err
	}

	credentialText := strings.TrimSpace(request.Credential)
	modelCredential := configToModel(config)
	if credentialText != "" {
		key, err := service.key()
		if err != nil {
			return Config{}, err
		}
		ciphertext, nonce, err := encryptCredential(key, credentialText)
		if err != nil {
			return Config{}, err
		}
		modelCredential.EncryptedCredential = ciphertext
		modelCredential.CredentialNonce = nonce
		modelCredential.MaskedCredential = maskCredential(config.AuthType, credentialText)
		modelCredential.CredentialStatus = string(StatusNormal)
	} else if config.AuthType == AuthTypeNone {
		modelCredential.MaskedCredential = "无需凭据"
		modelCredential.CredentialStatus = string(StatusNormal)
	} else if exists {
		modelCredential.EncryptedCredential = existing.EncryptedCredential
		modelCredential.CredentialNonce = existing.CredentialNonce
		modelCredential.MaskedCredential = existing.MaskedCredential
		modelCredential.CredentialStatus = existing.CredentialStatus
	} else {
		modelCredential.CredentialStatus = string(StatusNotConfigured)
		modelCredential.MaskedCredential = config.MaskedCredential
	}

	if err := service.Store.SaveDataSourceCredential(ctx, &modelCredential); err != nil {
		return Config{}, err
	}
	return modelToConfig(modelCredential, config), nil
}

// Clear 清除指定 Provider 的凭据密文并返回未配置状态。
func (service Service) Clear(ctx context.Context, providerID string) (Config, error) {
	if service.Store == nil {
		return Config{}, fmt.Errorf("data source credential store is required")
	}
	catalogConfig, ok := catalogConfigMap()[strings.TrimSpace(providerID)]
	if !ok {
		return Config{}, fmt.Errorf("unknown data source provider")
	}
	if err := service.Store.ClearDataSourceCredential(ctx, catalogConfig.ProviderID); err != nil {
		return Config{}, err
	}
	catalogConfig.CredentialStatus = StatusNotConfigured
	catalogConfig.MaskedCredential = ""
	return catalogConfig, nil
}

// Test 执行本地预检，不访问真实外部网络。
func (service Service) Test(ctx context.Context, request TestRequest) (TestResult, error) {
	if service.Store == nil {
		return TestResult{}, fmt.Errorf("data source credential store is required")
	}
	providerID := strings.TrimSpace(request.ProviderID)
	catalogConfig, ok := catalogConfigMap()[providerID]
	if !ok {
		return TestResult{}, fmt.Errorf("unknown data source provider")
	}
	if catalogConfig.AuthType == AuthTypeNone {
		return TestResult{
			Status:         testStatusSuccess,
			ResponseTimeMS: 1,
			TestedAt:       formatDisplayTime(service.now()),
			Messages:       []string{"该 Provider 无需凭据，本地预检通过"},
		}, nil
	}
	credential, exists, err := service.Store.GetDataSourceCredential(ctx, providerID)
	if err != nil {
		return TestResult{}, err
	}
	if !exists || strings.TrimSpace(credential.EncryptedCredential) == "" || strings.TrimSpace(credential.CredentialNonce) == "" {
		return TestResult{
			Status:   testStatusUntested,
			Messages: []string{"当前 Provider 尚未配置凭据"},
		}, nil
	}
	result := TestResult{
		Status:         testStatusSuccess,
		ResponseTimeMS: 1,
		TestedAt:       formatDisplayTime(service.now()),
		Messages:       []string{"凭据密文存在", "本地配置预检通过"},
	}
	if err := service.saveTestResult(ctx, credential, result); err != nil {
		return TestResult{}, err
	}
	return result, nil
}

// Resolve 解密并返回运行时 Provider 凭据，调用方不得记录或返回该结构。
func (service Service) Resolve(ctx context.Context, providerID string) (RuntimeCredential, error) {
	if service.Store == nil {
		return RuntimeCredential{}, fmt.Errorf("data source credential store is required")
	}
	trimmedProviderID := strings.TrimSpace(providerID)
	catalogConfig, ok := catalogConfigMap()[trimmedProviderID]
	if !ok {
		return RuntimeCredential{}, fmt.Errorf("unknown data source provider")
	}
	if catalogConfig.AuthType == AuthTypeNone {
		return RuntimeCredential{ProviderID: catalogConfig.ProviderID, AuthType: AuthTypeNone}, nil
	}
	credential, exists, err := service.Store.GetDataSourceCredential(ctx, trimmedProviderID)
	if err != nil {
		return RuntimeCredential{}, err
	}
	if !exists || !credentialUsableForRuntime(credential) {
		return RuntimeCredential{}, ErrCredentialNotConfigured
	}
	key, err := service.key()
	if err != nil {
		return RuntimeCredential{}, err
	}
	plaintext, err := decryptCredential(key, credential.EncryptedCredential, credential.CredentialNonce)
	if err != nil {
		return RuntimeCredential{}, err
	}
	return runtimeCredentialFromPlaintext(catalogConfig.ProviderID, catalogConfig.AuthType, plaintext), nil
}

// key 读取本地加密密钥。
func (service Service) key() ([]byte, error) {
	if service.KeyProvider == nil {
		return nil, fmt.Errorf("data source credential key provider is required")
	}
	return service.KeyProvider.Key()
}

// now 返回当前时间；测试未注入时使用系统时间。
func (service Service) now() time.Time {
	if service.Clock != nil {
		return service.Clock.Now()
	}
	return time.Now().UTC()
}

// saveTestResult 将最近一次本地预检状态写回 SQLite。
func (service Service) saveTestResult(ctx context.Context, credential model.DataSourceCredential, result TestResult) error {
	messages, err := json.Marshal(result.Messages)
	if err != nil {
		return err
	}
	testedAt := service.now()
	credential.LastTestStatus = result.Status
	credential.LastTestResponseTime = result.ResponseTimeMS
	credential.LastTestedAt = &testedAt
	credential.LastTestMessages = string(messages)
	return service.Store.SaveDataSourceCredential(ctx, &credential)
}

// credentialUsableForRuntime 判断保存的密文是否允许进入运行时注入链路。
func credentialUsableForRuntime(credential model.DataSourceCredential) bool {
	status := Status(strings.TrimSpace(credential.CredentialStatus))
	if status != StatusNormal && status != StatusExpiring {
		return false
	}
	return strings.TrimSpace(credential.EncryptedCredential) != "" && strings.TrimSpace(credential.CredentialNonce) != ""
}

// runtimeCredentialFromPlaintext 按认证方式把明文放入唯一对应的运行时字段。
func runtimeCredentialFromPlaintext(providerID string, authType AuthType, plaintext string) RuntimeCredential {
	credential := RuntimeCredential{
		ProviderID: providerID,
		AuthType:   authType,
	}
	switch authType {
	case AuthTypeAPIKey:
		credential.APIKey = plaintext
	case AuthTypeCookie:
		credential.Cookie = plaintext
	case AuthTypeBearerToken:
		credential.BearerToken = plaintext
	case AuthTypeCustomHeader:
		credential.HeaderValue = plaintext
	}
	return credential
}

// normalizeConfig 校验并补齐保存请求中的配置字段。
func normalizeConfig(input Config) (Config, error) {
	providerID := strings.TrimSpace(input.ProviderID)
	catalogConfig, ok := catalogConfigMap()[providerID]
	if !ok {
		return Config{}, fmt.Errorf("unknown data source provider")
	}
	input.ProviderID = providerID
	input.ProviderName = strings.TrimSpace(input.ProviderName)
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	if input.ProviderName == "" || input.BaseURL == "" || input.AuthType == "" {
		return Config{}, fmt.Errorf("provider name, base url and auth type are required")
	}
	if _, err := url.ParseRequestURI(input.BaseURL); err != nil {
		return Config{}, fmt.Errorf("base url is invalid")
	}
	if !isAllowedAuthType(input.AuthType) {
		return Config{}, fmt.Errorf("auth type is invalid")
	}
	if input.Capability == "" {
		input.Capability = catalogConfig.Capability
	}
	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = catalogConfig.TimeoutSeconds
	}
	if input.RateLimitPerMinute <= 0 {
		input.RateLimitPerMinute = catalogConfig.RateLimitPerMinute
	}
	if input.CredentialStatus == "" {
		input.CredentialStatus = catalogConfig.CredentialStatus
	}
	return input, nil
}

// isAllowedAuthType 判断认证方式是否在首版白名单内。
func isAllowedAuthType(authType AuthType) bool {
	switch authType {
	case AuthTypeNone, AuthTypeAPIKey, AuthTypeCookie, AuthTypeBearerToken, AuthTypeCustomHeader:
		return true
	default:
		return false
	}
}

// catalogConfigMap 返回 Provider 默认配置索引。
func catalogConfigMap() map[string]Config {
	configs := make(map[string]Config, len(providerCatalog))
	for _, item := range providerCatalog {
		configs[item.ProviderID] = item
	}
	return configs
}

// providersFromConfigs 根据配置构造左侧 Provider 列表。
func providersFromConfigs(configs map[string]Config) []Provider {
	providers := make([]Provider, 0, len(providerCatalog))
	for _, catalogItem := range providerCatalog {
		config := configs[catalogItem.ProviderID]
		providers = append(providers, Provider{
			ID:         config.ProviderID,
			Name:       config.ProviderName,
			Capability: catalogItem.Capability,
			Status:     config.CredentialStatus,
			AuthType:   config.AuthType,
			IconType:   providerIconTypes[config.ProviderID],
		})
	}
	return providers
}

// defaultSelectedProvider 返回凭据管理页默认选中的 Provider。
func defaultSelectedProvider(configs map[string]Config) string {
	if _, ok := configs["cls"]; ok {
		return "cls"
	}
	if len(providerCatalog) > 0 {
		return providerCatalog[0].ProviderID
	}
	return ""
}

// defaultTestTargets 返回首版凭据本地预检目标。
func defaultTestTargets() []TestTarget {
	return []TestTarget{
		{Label: "快讯接口（/api/flash）", Value: "flash"},
		{Label: "日历接口（/api/calendar）", Value: "calendar"},
		{Label: "行业事件接口（/api/events）", Value: "events"},
	}
}

// defaultTestResult 返回默认未测试状态。
func defaultTestResult(configs map[string]Config) TestResult {
	selected := configs[defaultSelectedProvider(configs)]
	if selected.CredentialStatus == StatusNormal {
		return TestResult{Status: testStatusUntested, Messages: []string{"尚未执行本地预检"}}
	}
	return TestResult{Status: testStatusUntested, Messages: []string{"当前 Provider 尚未配置凭据"}}
}

// overviewFromConfigs 汇总凭据配置数量和过期风险。
func overviewFromConfigs(configs map[string]Config, now time.Time) Overview {
	var overview Overview
	for _, config := range configs {
		switch config.CredentialStatus {
		case StatusNormal:
			overview.ConfiguredCount++
		case StatusExpired:
			overview.ExpiredCount++
		case StatusExpiring:
			overview.ExpiringSoonCount++
		}
		if config.CredentialStatus == StatusNormal && config.ExpiresAt != "" {
			if expiresAt, err := parseDisplayTime(config.ExpiresAt); err == nil {
				if expiresAt.Before(now) {
					overview.ExpiredCount++
				} else if expiresAt.Sub(now) <= 7*24*time.Hour {
					overview.ExpiringSoonCount++
				}
			}
		}
	}
	return overview
}

// healthItemsFromConfigs 构造调用限制和健康状态展示项。
func healthItemsFromConfigs(configs map[string]Config) []HealthItem {
	return []HealthItem{
		{Name: "行情源", Status: healthStatus(configs["eastmoney"]), RateLimitText: rateLimitText(configs["eastmoney"])},
		{Name: "新闻源", Status: healthStatus(configs["cls"]), RateLimitText: rateLimitText(configs["cls"])},
		{Name: "海外源", Status: healthStatus(configs["alpha-vantage"]), RateLimitText: rateLimitText(configs["alpha-vantage"])},
	}
}

// healthStatus 将凭据状态映射到底部健康状态。
func healthStatus(config Config) string {
	switch config.CredentialStatus {
	case StatusNormal:
		return "normal"
	case StatusFailed, StatusExpired:
		return "failed"
	default:
		return "limited"
	}
}

// rateLimitText 格式化调用限制展示值。
func rateLimitText(config Config) string {
	if config.RateLimitPerMinute <= 0 {
		return "未配置"
	}
	return fmt.Sprintf("%d 次/分钟", config.RateLimitPerMinute)
}

// operationLogs 从已保存配置派生脱敏操作日志。
func operationLogs(credentials []model.DataSourceCredential) []OperationLog {
	logs := make([]OperationLog, 0, len(credentials))
	sort.SliceStable(credentials, func(left int, right int) bool {
		return credentials[left].UpdatedAt.After(credentials[right].UpdatedAt)
	})
	for index, credential := range credentials {
		if index >= 3 {
			break
		}
		logs = append(logs, OperationLog{
			ID:     fmt.Sprintf("%d", credential.ID),
			Action: fmt.Sprintf("更新%s凭据", credential.ProviderName),
			Status: "success",
			Time:   credential.UpdatedAt.Local().Format("15:04:05"),
		})
	}
	return logs
}

// modelToConfig 将持久化模型转换为前端安全展示配置。
func modelToConfig(item model.DataSourceCredential, fallback Config) Config {
	expiresAt := ""
	if item.ExpiresAt != nil {
		expiresAt = formatDisplayTime(*item.ExpiresAt)
	}
	return Config{
		ProviderID:         nonEmpty(item.ProviderID, fallback.ProviderID),
		ProviderName:       nonEmpty(item.ProviderName, fallback.ProviderName),
		Capability:         nonEmpty(item.Capability, fallback.Capability),
		AuthType:           AuthType(nonEmpty(item.AuthType, string(fallback.AuthType))),
		BaseURL:            nonEmpty(item.BaseURL, fallback.BaseURL),
		CredentialStatus:   Status(nonEmpty(item.CredentialStatus, string(fallback.CredentialStatus))),
		ExpiresAt:          expiresAt,
		TimeoutSeconds:     nonZero(item.TimeoutSeconds, fallback.TimeoutSeconds),
		RateLimitPerMinute: nonZero(item.RateLimitPerMinute, fallback.RateLimitPerMinute),
		MaskedCredential:   nonEmpty(item.MaskedCredential, fallback.MaskedCredential),
		Note:               item.Note,
	}
}

// configToModel 将 service 配置转换为数据库模型。
func configToModel(config Config) model.DataSourceCredential {
	var expiresAt *time.Time
	if parsed, err := parseDisplayTime(config.ExpiresAt); err == nil && !parsed.IsZero() {
		expiresAt = &parsed
	}
	return model.DataSourceCredential{
		ProviderID:         config.ProviderID,
		ProviderName:       config.ProviderName,
		Capability:         config.Capability,
		AuthType:           string(config.AuthType),
		BaseURL:            config.BaseURL,
		CredentialStatus:   string(config.CredentialStatus),
		ExpiresAt:          expiresAt,
		TimeoutSeconds:     config.TimeoutSeconds,
		RateLimitPerMinute: config.RateLimitPerMinute,
		MaskedCredential:   config.MaskedCredential,
		Note:               config.Note,
		LastTestStatus:     testStatusUntested,
	}
}

// maskCredential 根据认证方式生成安全脱敏展示值。
func maskCredential(authType AuthType, secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	switch authType {
	case AuthTypeNone:
		return "无需凭据"
	case AuthTypeCookie:
		parts := strings.Split(trimmed, ";")
		masked := make([]string, 0, len(parts))
		for _, part := range parts {
			name, _, found := strings.Cut(strings.TrimSpace(part), "=")
			if found && name != "" {
				masked = append(masked, name+"=****")
			}
		}
		if len(masked) > 0 {
			return strings.Join(masked, "; ")
		}
		return "Cookie=****"
	case AuthTypeAPIKey:
		return "key=" + tailMask(trimmed)
	case AuthTypeBearerToken:
		return "Bearer " + tailMask(trimmed)
	default:
		return "credential=" + tailMask(trimmed)
	}
}

// tailMask 只保留末尾四位用于用户识别，避免泄露完整凭据。
func tailMask(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}

// formatDisplayTime 输出前端 DatePicker 使用的分钟级时间文本。
func formatDisplayTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04")
}

// parseDisplayTime 解析前端 DatePicker 提交的时间文本。
func parseDisplayTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	return time.ParseInLocation("2006-01-02 15:04", trimmed, time.Local)
}

// nonEmpty 返回首个非空字符串。
func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// nonZero 返回首个非零整数。
func nonZero(value int, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}
