package news

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// Provider 定义首版新闻数据源必须实现的能力边界。
type Provider interface {
	Name() string
	List(ctx context.Context, request ListRequest) ([]Item, error)
	Market(ctx context.Context, request MarketRequest) ([]Item, error)
}

// ListRequest 是个股新闻查询请求。
type ListRequest struct {
	Symbol stock.Symbol
	Limit  int
}

// MarketRequest 是市场新闻查询请求。
type MarketRequest struct {
	Market string
	Limit  int
}

// Item 是新闻资讯的统一模型。
type Item struct {
	ID          string
	Source      string
	Title       string
	URL         string
	Summary     string
	ContentHash string
	PublishedAt time.Time
	Symbols     []stock.Symbol
	Tags        []string
}

// ProviderError 是新闻 Provider 调用失败时的可观测错误。
type ProviderError struct {
	Provider  string
	Operation string
	Cause     error
}

// Error 返回脱敏后的 Provider 错误，避免请求头或密钥泄露进日志和响应。
func (err *ProviderError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf(
		"news provider %s %s failed: %s",
		err.Provider,
		err.Operation,
		logger.RedactError(err.Cause),
	)
}

// Unwrap 返回原始错误，供内部错误分类使用。
func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewProviderError 创建会自动脱敏输出的新闻 Provider 错误。
func NewProviderError(provider string, operation string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Operation: operation,
		Cause:     cause,
	}
}

// NormalizeItems 批量标准化新闻条目，任何条目非法都会直接返回错误。
func NormalizeItems(items []Item) ([]Item, error) {
	normalized := make([]Item, 0, len(items))
	for _, item := range items {
		next, err := NormalizeItem(item)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, next)
	}
	return normalized, nil
}

// NormalizeItem 标准化单条新闻，生成稳定 ID、内容 hash，并校验 URL scheme。
func NormalizeItem(item Item) (Item, error) {
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" {
		return Item{}, &xerr.Error{Code: xerr.NewsMissingTitle}
	}

	normalizedURL, err := normalizeURL(item.URL)
	if err != nil {
		return Item{}, err
	}
	item.URL = normalizedURL
	item.ContentHash = contentHash(item)
	if item.ID == "" {
		item.ID = item.ContentHash
	}
	return item, nil
}

// Deduplicate 按内容 hash 去重，并保留首次出现的新闻顺序。
func Deduplicate(items []Item) []Item {
	seen := make(map[string]struct{}, len(items))
	deduped := make([]Item, 0, len(items))
	for _, item := range items {
		key := item.ContentHash
		if key == "" {
			key = contentHash(item)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, item)
	}
	return deduped
}

// normalizeURL 只允许 HTTP(S) 新闻链接，禁止 javascript/file 等危险 scheme。
func normalizeURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" {
		return "", &xerr.Error{Code: xerr.NewsUnsafeURL}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", &xerr.Error{Code: xerr.NewsUnsafeURL}
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

// contentHash 生成新闻内容去重 hash，忽略 URL 以合并多来源转载。
func contentHash(item Item) string {
	symbols := make([]string, 0, len(item.Symbols))
	for _, symbol := range item.Symbols {
		symbols = append(symbols, symbol.String())
	}
	sort.Strings(symbols)

	parts := []string{
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Summary),
		item.PublishedAt.UTC().Format(time.RFC3339Nano),
		strings.Join(symbols, ","),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}
