package main

import (
	"encoding/json"
	"strings"
	"testing"

	credentialservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
)

// TestParseSmokeOptionsRequiresExplicitNetworkConsent 验证真实 Provider smoke 必须显式确认联网和授权边界。
func TestParseSmokeOptionsRequiresExplicitNetworkConsent(t *testing.T) {
	_, err := parseSmokeOptions([]string{"--allow-network"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("expected missing provider terms confirmation to fail")
	}

	options, err := parseSmokeOptions([]string{
		"--workspace", "/tmp/invest-compass",
		"--providers", "cls,xueqiu",
		"--allow-network",
		"--confirm-provider-terms",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if options.Workspace != "/tmp/invest-compass" || options.Providers != "cls,xueqiu" {
		t.Fatalf("options = %#v", options)
	}
	if !options.AllowNetwork || !options.ConfirmProviderTerms {
		t.Fatalf("network confirmation flags missing: %#v", options)
	}
}

// TestParseSmokeOptionsIgnoresPackageManagerSeparator 验证 pnpm 透传的分隔符不会吞掉后续参数。
func TestParseSmokeOptionsIgnoresPackageManagerSeparator(t *testing.T) {
	options, err := parseSmokeOptions([]string{
		"--",
		"--workspace", "/tmp/invest-compass",
		"--allow-network",
		"--confirm-provider-terms",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parse options with separator: %v", err)
	}
	if !options.AllowNetwork || !options.ConfirmProviderTerms {
		t.Fatalf("network flags not parsed: %#v", options)
	}
}

// TestSelectProviderIDsRespectsAllConfiguredAndExplicitModes 验证 smoke 运行范围可覆盖全部、已配置或指定 Provider。
func TestSelectProviderIDsRespectsAllConfiguredAndExplicitModes(t *testing.T) {
	view := credentialservice.ListView{
		Providers: []credentialservice.Provider{
			{ID: "sina", Status: credentialservice.StatusNormal, AuthType: credentialservice.AuthTypeNone},
			{ID: "cls", Status: credentialservice.StatusNormal, AuthType: credentialservice.AuthTypeCookie},
			{ID: "xueqiu", Status: credentialservice.StatusNotConfigured, AuthType: credentialservice.AuthTypeCookie},
		},
	}

	all, err := selectProviderIDs(view, "all")
	if err != nil {
		t.Fatalf("select all: %v", err)
	}
	if got := joinProviderIDs(all); got != "sina,cls,xueqiu" {
		t.Fatalf("all provider ids = %s", got)
	}

	configured, err := selectProviderIDs(view, "configured")
	if err != nil {
		t.Fatalf("select configured: %v", err)
	}
	if got := joinProviderIDs(configured); got != "sina,cls" {
		t.Fatalf("configured provider ids = %s", got)
	}

	explicit, err := selectProviderIDs(view, "cls,xueqiu")
	if err != nil {
		t.Fatalf("select explicit: %v", err)
	}
	if got := joinProviderIDs(explicit); got != "cls,xueqiu" {
		t.Fatalf("explicit provider ids = %s", got)
	}
}

// TestSelectProviderIDsRejectsUnknownProvider 验证显式指定未知 Provider 时快速失败。
func TestSelectProviderIDsRejectsUnknownProvider(t *testing.T) {
	_, err := selectProviderIDs(credentialservice.ListView{
		Providers: []credentialservice.Provider{{ID: "sina"}},
	}, "missing")
	if err == nil {
		t.Fatal("expected unknown provider to fail")
	}
}

// TestSmokeOutputRequiresTestedProvidersInLiveMode 验证联网 smoke 中未执行测试的 Provider 不能让命令成功退出。
func TestSmokeOutputRequiresTestedProvidersInLiveMode(t *testing.T) {
	output := smokeOutput{
		NetworkEnabled: true,
		Checks: []smokeCheck{{
			ProviderID: "alpha-vantage",
			Status:     "untested",
			Messages:   []string{"当前 Provider 尚未配置凭据"},
		}},
	}

	if !shouldExitUnsuccessful(output) {
		t.Fatal("expected live smoke with untested provider to exit unsuccessfully")
	}
}

// TestSmokeOutputOmitsWorkspacePath 验证 smoke JSON 输出不暴露本机 workspace 路径。
func TestSmokeOutputOmitsWorkspacePath(t *testing.T) {
	encoded, err := json.Marshal(smokeOutput{
		NetworkEnabled: false,
		Checks:         []smokeCheck{},
	})
	if err != nil {
		t.Fatalf("marshal smoke output: %v", err)
	}
	if strings.Contains(string(encoded), "workspace") {
		t.Fatalf("smoke output must not include workspace path: %s", encoded)
	}
}
