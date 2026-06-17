package settings

import (
	"errors"
	"testing"
)

// TestValidateProxyURLRejectsEmbeddedCredentials 验证代理 URL 禁止携带 username/password。
func TestValidateProxyURLRejectsEmbeddedCredentials(t *testing.T) {
	err := ValidateProxyURL("https://user:placeholder@proxy.example:8080")

	assertSettingsErrorCode(t, err, ErrorProxyCredentialInURL)
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
	if len(targets) != 4 {
		t.Fatalf("expected 4 cache targets, got %+v", targets)
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
func assertSettingsErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var settingsError *Error
	if !errors.As(err, &settingsError) {
		t.Fatalf("expected settings Error, got %T", err)
	}
	if settingsError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, settingsError.Code)
	}
}
