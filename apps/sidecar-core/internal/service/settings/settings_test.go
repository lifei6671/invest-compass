package settings

import (
	"errors"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestValidateProxyURLRejectsEmbeddedCredentials 验证代理 URL 禁止携带 username/password。
func TestValidateProxyURLRejectsEmbeddedCredentials(t *testing.T) {
	err := ValidateProxyURL("https://user:placeholder@proxy.example:8080")

	assertSettingsErrorCode(t, err, xerr.SettingsProxyCredentialInURL)
}

// TestValidateProxyURLAcceptsCredentialFreeURL 验证不含凭据的代理 URL 可通过。
func TestValidateProxyURLAcceptsCredentialFreeURL(t *testing.T) {
	if err := ValidateProxyURL("socks5://proxy.example:1080"); err != nil {
		t.Fatalf("ValidateProxyURL returned error: %v", err)
	}
}

// TestFilterCacheCleanupTargetsKeepsReportsAndConfigs 验证缓存清理不会删除报告和配置。
func TestFilterCacheCleanupTargetsKeepsReportsAndConfigs(t *testing.T) {
	targets := FilterCacheCleanupTargets([]CacheTarget{
		CacheTargetQuote,
		CacheTargetKline,
		CacheTargetNews,
		CacheTargetChartImage,
		CacheTargetTaskLogs,
		CacheTargetAppLogs,
		CacheTargetReport,
		CacheTargetConfig,
	})

	for _, forbidden := range []CacheTarget{CacheTargetReport, CacheTargetConfig} {
		for _, target := range targets {
			if target == forbidden {
				t.Fatalf("cache cleanup included forbidden target %q: %+v", forbidden, targets)
			}
		}
	}
	if len(targets) != 6 {
		t.Fatalf("expected 6 cache targets, got %+v", targets)
	}
}

// TestValidateSettingRejectsSensitivePlaintext 验证 settings 不能保存敏感明文。
func TestValidateSettingRejectsSensitivePlaintext(t *testing.T) {
	err := ValidateSetting(Setting{Key: "api_key", Value: "plain-value"})

	assertSettingsErrorCode(t, err, xerr.SettingsSensitiveSetting)
}

// TestValidateSettingAcceptsCredentialReference 验证 settings 可以保存本地 vault 引用和脱敏状态。
func TestValidateSettingAcceptsCredentialReference(t *testing.T) {
	for _, setting := range []Setting{
		{Key: "api_key_ref", Value: "local-vault://ai-config/openai-compatible-1"},
		{Key: "proxy_credential_ref", Value: "local-vault://proxy/default"},
		{Key: "masked_api_key", Value: "****1234"},
		{Key: "has_api_key", Value: "true"},
	} {
		if err := ValidateSetting(setting); err != nil {
			t.Fatalf("ValidateSetting(%+v) returned error: %v", setting, err)
		}
	}
}

// TestValidateSettingRejectsInvalidCredentialReference 验证 settings 凭据引用不能用任意 _ref 绕过敏感配置拦截。
func TestValidateSettingRejectsInvalidCredentialReference(t *testing.T) {
	for _, setting := range []Setting{
		{Key: "api_key_ref", Value: "local-vault://proxy/default"},
		{Key: "proxy_credential_ref", Value: "file:///tmp/proxy-secret"},
		{Key: "license_key_ref", Value: "local-vault://license/demo"},
	} {
		err := ValidateSetting(setting)

		assertSettingsErrorCode(t, err, xerr.SettingsInvalidCredentialRef)
	}
}

// TestValidateSettingRejectsUnmaskedAPIKeyDisplay 验证脱敏展示字段不能保存疑似明文 Key。
func TestValidateSettingRejectsUnmaskedAPIKeyDisplay(t *testing.T) {
	err := ValidateSetting(Setting{Key: "masked_api_key", Value: "sk-live-raw-secret-123456"})

	assertSettingsErrorCode(t, err, xerr.SettingsSensitiveSetting)
}

// TestValidateSettingAcceptsKnownSettingValues 验证设置中心已定义 key 的合法值域可保存。
func TestValidateSettingAcceptsKnownSettingValues(t *testing.T) {
	for _, setting := range []Setting{
		{Key: SettingKeyAppTheme, Value: "system"},
		{Key: SettingKeyAppLanguage, Value: DefaultAppLanguage},
		{Key: SettingKeyMarketDefault, Value: DefaultMarket},
		{Key: SettingKeyQuoteRefreshInterval, Value: "manual"},
		{Key: SettingKeyKlineDefaultPeriod, Value: DefaultKlinePeriod},
		{Key: SettingKeyKlineDefaultAdjust, Value: DefaultKlineAdjust},
		{Key: SettingKeyNotificationsInAppEnabled, Value: "true"},
		{Key: SettingKeyNotificationsSystemEnabled, Value: "false"},
		{Key: SettingKeyWindowCloseToTray, Value: "true"},
		{Key: SettingKeyUpdateCheckOnStartup, Value: "false"},
		{Key: SettingKeyDataSourceDefaultMarketSource, Value: DataSourceMarketSourceAutoFallback},
		{Key: SettingKeyDataSourceDefaultMarketSource, Value: DataSourceMarketSourceSina},
		{Key: SettingKeyDataSourceDefaultMarketSource, Value: DataSourceMarketSourceTencent},
		{Key: SettingKeyDataSourceDefaultMarketSource, Value: DataSourceMarketSourceTdx},
	} {
		if err := ValidateSetting(setting); err != nil {
			t.Fatalf("ValidateSetting(%+v) returned error: %v", setting, err)
		}
	}
}

// TestValidateSettingRejectsKnownSettingInvalidValue 验证已定义 key 不接受不可解释的配置值。
func TestValidateSettingRejectsKnownSettingInvalidValue(t *testing.T) {
	for _, setting := range []Setting{
		{Key: SettingKeyAppTheme, Value: "blue"},
		{Key: SettingKeyMarketDefault, Value: "crypto"},
		{Key: SettingKeyQuoteRefreshInterval, Value: "1s"},
		{Key: SettingKeyKlineDefaultAdjust, Value: "forward"},
		{Key: SettingKeyNotificationsTaskSuccess, Value: "yes"},
		{Key: SettingKeyDataSourceDefaultMarketSource, Value: "unknown-provider"},
	} {
		if err := ValidateSetting(setting); err == nil {
			t.Fatalf("ValidateSetting(%+v) should reject invalid value", setting)
		}
	}
}

// TestValidateWorkspacePathRequiresAbsolutePath 验证工作区路径必须是绝对路径。
func TestValidateWorkspacePathRequiresAbsolutePath(t *testing.T) {
	err := ValidateWorkspacePath("relative/workspace")

	assertSettingsErrorCode(t, err, xerr.SettingsInvalidWorkspacePath)
}

// TestBuildCacheStatsOnlyCountsTemporaryTargets 验证缓存统计不把报告和配置算入可清理缓存。
func TestBuildCacheStatsOnlyCountsTemporaryTargets(t *testing.T) {
	stats := BuildCacheStats([]CacheUsage{
		{Target: CacheTargetQuote, Bytes: 10},
		{Target: CacheTargetNews, Bytes: 20},
		{Target: CacheTargetTaskLogs, Bytes: 30},
		{Target: CacheTargetAppLogs, Bytes: 40},
		{Target: CacheTargetReport, Bytes: 300},
		{Target: CacheTargetConfig, Bytes: 400},
	})

	if stats.TotalBytes != 100 {
		t.Fatalf("expected temporary total 100, got %+v", stats)
	}
	if len(stats.Items) != 4 {
		t.Fatalf("expected 4 temporary cache items, got %+v", stats.Items)
	}
}

// TestAboutLicenseViewIsFreePlaceholder 验证关于页只展示 FREE 占位且不提供激活入口。
func TestAboutLicenseViewIsFreePlaceholder(t *testing.T) {
	view := FreeLicenseView()

	if view.Status != LicenseStatusFree {
		t.Fatalf("expected FREE status, got %+v", view)
	}
	if view.CanActivate || view.ActivationURL != "" {
		t.Fatalf("FREE placeholder must not expose activation entry: %+v", view)
	}
}

// assertSettingsErrorCode 校验设置模块错误码稳定。
func assertSettingsErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var settingsError *xerr.Error
	if !errors.As(err, &settingsError) {
		t.Fatalf("expected settings Error, got %T", err)
	}
	if settingsError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, settingsError.Code)
	}
}
