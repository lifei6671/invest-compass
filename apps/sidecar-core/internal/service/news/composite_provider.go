package news

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// CompositeProvider 将多个市场资讯源聚合成一个 Provider，单个来源失败不阻断其它来源。
type CompositeProvider struct {
	name      string
	providers []Provider
}

// NewCompositeProvider 创建聚合资讯 Provider，调用方必须传入至少一个真实来源。
func NewCompositeProvider(name string, providers []Provider) *CompositeProvider {
	filtered := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if provider != nil {
			filtered = append(filtered, provider)
		}
	}
	if strings.TrimSpace(name) == "" {
		name = "composite-news"
	}
	return &CompositeProvider{name: name, providers: filtered}
}

// Name 返回聚合 Provider 稳定名称。
func (provider *CompositeProvider) Name() string {
	return provider.name
}

// Status 汇总子 Provider 状态，只要有一个来源可用就认为聚合新闻源可用。
func (provider *CompositeProvider) Status(ctx context.Context) ProviderStatus {
	if provider == nil || len(provider.providers) == 0 {
		return UnconfiguredProviderStatus()
	}
	sources := make([]string, 0, len(provider.providers))
	errors := make([]string, 0, len(provider.providers))
	available := false
	for _, child := range provider.providers {
		status := ProviderStatusFromProvider(ctx, child)
		source := strings.TrimSpace(status.Source)
		if source == "" {
			source = child.Name()
		}
		sources = append(sources, source)
		if status.Available {
			available = true
			continue
		}
		if status.LastError != "" {
			errors = append(errors, fmt.Sprintf("%s:%s", child.Name(), status.LastError))
		}
	}
	return ProviderStatus{
		Name:      provider.Name(),
		Source:    strings.Join(sources, ", "),
		Available: available,
		LastError: strings.Join(errors, "; "),
	}
}

// List 聚合支持个股新闻的子 Provider；当前市场快讯源不支持时会继续尝试其它来源。
func (provider *CompositeProvider) List(ctx context.Context, request ListRequest) ([]Item, error) {
	items, err := provider.collect(func(child Provider) ([]Item, error) {
		return child.List(ctx, request)
	})
	if err != nil {
		return nil, err
	}
	return sortedLimitedItems(items, request.Limit)
}

// Market 聚合市场新闻并按发布时间倒序返回。
func (provider *CompositeProvider) Market(ctx context.Context, request MarketRequest) ([]Item, error) {
	items, err := provider.collect(func(child Provider) ([]Item, error) {
		return child.Market(ctx, request)
	})
	if err != nil {
		return nil, err
	}
	return sortedLimitedItems(items, request.Limit)
}

func (provider *CompositeProvider) collect(fetch func(Provider) ([]Item, error)) ([]Item, error) {
	if provider == nil || len(provider.providers) == 0 {
		return nil, &ProviderError{Provider: "composite-news", Operation: "provider_unconfigured", Cause: fmt.Errorf("empty news providers")}
	}
	type providerResult struct {
		items []Item
		err   error
	}
	resultCh := make(chan providerResult, len(provider.providers))
	var wg sync.WaitGroup
	for _, child := range provider.providers {
		wg.Add(1)
		go func(child Provider) {
			defer wg.Done()
			items, err := fetch(child)
			resultCh <- providerResult{items: items, err: err}
		}(child)
	}
	wg.Wait()
	close(resultCh)

	var collected []Item
	var firstErr error
	succeeded := false
	for result := range resultCh {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		succeeded = true
		collected = append(collected, result.items...)
	}
	if !succeeded && firstErr != nil {
		return nil, firstErr
	}
	return collected, nil
}

func sortedLimitedItems(items []Item, limit int) ([]Item, error) {
	normalized, err := NormalizeItems(items)
	if err != nil {
		return nil, err
	}
	deduped := Deduplicate(normalized)
	sort.SliceStable(deduped, func(left int, right int) bool {
		return deduped[left].PublishedAt.After(deduped[right].PublishedAt)
	})
	if limit > 0 && limit < len(deduped) {
		return deduped[:limit], nil
	}
	return deduped, nil
}
