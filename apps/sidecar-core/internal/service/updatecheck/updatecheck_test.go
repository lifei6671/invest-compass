package updatecheck

import (
	"errors"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestParseManifestRejectsInvalidJSON 验证更新 JSON 格式错误时返回稳定错误码。
func TestParseManifestRejectsInvalidJSON(t *testing.T) {
	_, err := ParseManifest([]byte(`{"version":`))

	assertUpdateErrorCode(t, err, xerr.UpdateInvalidManifest)
}

// TestParseManifestRequiresVersion 验证更新 JSON 必须包含版本号。
func TestParseManifestRequiresVersion(t *testing.T) {
	_, err := ParseManifest([]byte(`{"download_url":"https://updates.invest-compass.example/download"}`))

	assertUpdateErrorCode(t, err, xerr.UpdateInvalidManifest)
}

// TestParseManifestTrimsFields 验证更新 JSON 解析后会清理首尾空白，避免后续 allowlist 校验误判。
func TestParseManifestTrimsFields(t *testing.T) {
	manifest, err := ParseManifest([]byte(`{
		"version":" 0.2.0 ",
		"download_url":" https://updates.invest-compass.example/download ",
		"release_notes_url":" https://updates.invest-compass.example/releases/0.2.0 "
	}`))
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}

	if manifest.Version != "0.2.0" {
		t.Fatalf("unexpected version: %q", manifest.Version)
	}
	if manifest.DownloadURL != "https://updates.invest-compass.example/download" {
		t.Fatalf("unexpected download URL: %q", manifest.DownloadURL)
	}
	if manifest.ReleaseNotesURL != "https://updates.invest-compass.example/releases/0.2.0" {
		t.Fatalf("unexpected release notes URL: %q", manifest.ReleaseNotesURL)
	}
}

// TestValidateManifestURLRejectsHTTP 验证更新 JSON 只能使用 HTTPS。
func TestValidateManifestURLRejectsHTTP(t *testing.T) {
	err := ValidateManifestURL("http://updates.invest-compass.example/latest.json", []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, xerr.UpdateInsecureURL)
}

// TestValidateManifestURLRejectsNonAllowlistedHost 验证更新 JSON 域名必须命中 allowlist。
func TestValidateManifestURLRejectsNonAllowlistedHost(t *testing.T) {
	err := ValidateManifestURL("https://evil.example/latest.json", []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, xerr.UpdateHostNotAllowed)
}

// TestValidateManifestRejectsNonAllowlistedLinks 验证下载页和发布说明链接也必须命中 allowlist。
func TestValidateManifestRejectsNonAllowlistedLinks(t *testing.T) {
	manifest := Manifest{
		Version:         "0.2.0",
		DownloadURL:     "https://evil.example/download",
		ReleaseNotesURL: "https://updates.invest-compass.example/releases/0.2.0",
	}

	err := ValidateManifest(manifest, []string{"updates.invest-compass.example"})

	assertUpdateErrorCode(t, err, xerr.UpdateHostNotAllowed)
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
func assertUpdateErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var updateError *xerr.Error
	if !errors.As(err, &updateError) {
		t.Fatalf("expected update Error, got %T", err)
	}
	if updateError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, updateError.Code)
	}
}
