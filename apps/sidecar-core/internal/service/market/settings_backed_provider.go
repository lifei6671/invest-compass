package market

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	settingsservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/settings"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/stock"
)

// SettingsStore 是行情 Provider 读取运行期数据源配置所需的最小 settings 边界。
type SettingsStore interface {
	GetSettings(ctx context.Context, keys []string) ([]model.Setting, error)
}

// SettingsBackedMarketProvider 在调用真实行情 Provider 前读取数据源配置。
// 当前生产 Provider 内部已按能力拆分为新浪搜索/实时行情、腾讯 K 线和东财 K 线兜底；
// 这里负责把设置中心保存的默认行情源纳入运行时链路，避免 UI 保存后完全不生效。
type SettingsBackedMarketProvider struct {
	store    SettingsStore
	provider MarketProvider
}

type sourceSelectableKlineProvider interface {
	klineWithSourcePreference(ctx context.Context, source string, request KlineRequest) ([]KlineBar, error)
}

// NewSettingsBackedMarketProvider 创建读取 settings 的行情 Provider 包装器。
func NewSettingsBackedMarketProvider(store SettingsStore, provider MarketProvider) MarketProvider {
	if provider == nil {
		return UnconfiguredProvider{}
	}
	if store == nil {
		return provider
	}
	return &SettingsBackedMarketProvider{store: store, provider: provider}
}

// Name 返回底层行情 Provider 名称。
func (provider *SettingsBackedMarketProvider) Name() string {
	return provider.provider.Name()
}

// Status 返回底层行情 Provider 状态；状态读取不阻塞真实调用链路。
func (provider *SettingsBackedMarketProvider) Status(ctx context.Context) ProviderStatus {
	return provider.provider.Status(ctx)
}

// Search 读取默认行情源配置后执行股票搜索。
func (provider *SettingsBackedMarketProvider) Search(ctx context.Context, keyword string) ([]StockBasic, error) {
	if _, err := provider.defaultMarketSource(ctx); err != nil {
		return nil, err
	}
	return provider.provider.Search(ctx, keyword)
}

// Quote 读取默认行情源配置后执行实时行情查询。
func (provider *SettingsBackedMarketProvider) Quote(ctx context.Context, symbol stock.Symbol) (Quote, error) {
	if _, err := provider.defaultMarketSource(ctx); err != nil {
		return Quote{}, err
	}
	return provider.provider.Quote(ctx, symbol)
}

// Kline 读取默认行情源配置后执行 K 线查询，显式支持的 K 线源必须真实命中对应 Provider。
func (provider *SettingsBackedMarketProvider) Kline(ctx context.Context, request KlineRequest) ([]KlineBar, error) {
	source, err := provider.defaultMarketSource(ctx)
	if err != nil {
		return nil, err
	}
	if preferredProvider, ok := provider.provider.(sourceSelectableKlineProvider); ok {
		return preferredProvider.klineWithSourcePreference(ctx, source, request)
	}
	if source == settingsservice.DataSourceMarketSourceTdx {
		if preferredProvider, ok := provider.provider.(interface {
			klineWithTdxPreference(context.Context, KlineRequest) ([]KlineBar, error)
		}); ok {
			return preferredProvider.klineWithTdxPreference(ctx, request)
		}
	}
	return provider.provider.Kline(ctx, request)
}

// defaultMarketSource 读取并校验数据源默认行情源；空配置按自动降级处理。
func (provider *SettingsBackedMarketProvider) defaultMarketSource(ctx context.Context) (string, error) {
	settings, err := provider.store.GetSettings(ctx, []string{settingsservice.SettingKeyDataSourceDefaultMarketSource})
	if err != nil {
		return "", fmt.Errorf("read default market source setting: %w", err)
	}
	if len(settings) == 0 || strings.TrimSpace(settings[0].Value) == "" {
		return settingsservice.DataSourceMarketSourceAutoFallback, nil
	}

	value := strings.TrimSpace(settings[0].Value)
	if err := settingsservice.ValidateSetting(settingsservice.Setting{
		Key:   settingsservice.SettingKeyDataSourceDefaultMarketSource,
		Value: value,
	}); err != nil {
		return "", err
	}
	return value, nil
}
