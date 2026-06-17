package updatecheck

import (
	"errors"
	"testing"
)

// TestValidateManifestURLRejectsHTTP 验证更新 JSON 只能使用 HTTPS。
func TestValidateManifestURLRejectsHTTP(t *testing.T) {
	err := ValidateManifestURL("http://updates.invest-compass.example/latest.json", []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, ErrorInsecureURL)
}

// TestValidateManifestURLRejectsNonAllowlistedHost 验证更新 JSON 域名必须命中 allowlist。
func TestValidateManifestURLRejectsNonAllowlistedHost(t *testing.T) {
	err := ValidateManifestURL("https://evil.example/latest.json", []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, ErrorHostNotAllowed)
}

// TestValidateManifestRejectsNonAllowlistedLinks 验证下载页和发布说明链接也必须命中 allowlist。
func TestValidateManifestRejectsNonAllowlistedLinks(t *testing.T) {
	manifest := Manifest{
		Version:         "0.2.0",
		DownloadURL:     "https://evil.example/download",
		ReleaseNotesURL: "https://updates.invest-compass.example/releases/0.2.0",
	}

	err := ValidateManifest(manifest, []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, ErrorHostNotAllowed)
}

// TestCheckResultOnlyPromptsForNewVersion 验证首版只提示新版本，不产生下载或安装动作。
func TestCheckResultOnlyPromptsForNewVersion(t *testing.T) {
	manifest := Manifest{
		Version:         "0.2.0",
		DownloadURL:     "https://updates.invest-compass.example/download",
		ReleaseNotesURL: "https://updates.invest-compass.example/releases/0.2.0",
	}

	result, err := BuildResult("0.1.0", manifest, []string{"updates.invest-compass.example"})
	if err != nil {
		t.Fatalf("BuildResult returned error: %v", err)
	}

	if !result.HasNewVersion {
		t.Fatalf("expected new version result: %+v", result)
	}
	if result.Action != ActionPromptOnly {
		t.Fatalf("expected prompt-only action, got %q", result.Action)
	}
	if result.DownloadURL != manifest.DownloadURL || result.ReleaseNotesURL != manifest.ReleaseNotesURL {
		t.Fatalf("unexpected result links: %+v", result)
	}
}

// TestCheckResultReportsLatestVersion 验证当前版本不低于 manifest 时只提示已是最新。
func TestCheckResultReportsLatestVersion(t *testing.T) {
	result, err := BuildResult("0.2.0", Manifest{Version: "0.2.0"}, []string{"updates.invest-compass.example"})
	if err != nil {
		t.Fatalf("BuildResult returned error: %v", err)
	}

	if result.HasNewVersion {
		t.Fatalf("expected latest version result: %+v", result)
	}
	if result.Action != ActionPromptOnly {
		t.Fatalf("expected prompt-only action, got %q", result.Action)
	}
}

// assertUpdateErrorCode 校验检查更新错误码稳定。
func assertUpdateErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var updateError *Error
	if !errors.As(err, &updateError) {
		t.Fatalf("expected update Error, got %T", err)
	}
	if updateError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, updateError.Code)
	}
}
