package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// TestHTTPFetcherRejectsOversizedManifest 验证远程更新 JSON 超过上限时必须失败，避免截断后误信任 manifest。
func TestHTTPFetcherRejectsOversizedManifest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"version":"0.2.0"}`))
		_, _ = response.Write([]byte(strings.Repeat(" ", maxManifestBytes)))
	}))
	defer server.Close()

	_, err := (HTTPFetcher{Client: server.Client()}).FetchManifest(context.Background(), server.URL)
	if err == nil {
		t.Fatal("oversized manifest should be rejected")
	}
}

// TestHTTPFetcherReadsManifestWithinLimit 验证正常大小的 manifest 可以完整读取。
func TestHTTPFetcherReadsManifestWithinLimit(t *testing.T) {
	t.Parallel()

	expected := []byte(`{"version":"0.2.0"}`)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(expected)
	}))
	defer server.Close()

	content, err := (HTTPFetcher{Client: server.Client()}).FetchManifest(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("manifest should be fetched: %v", err)
	}
	if !reflect.DeepEqual(content, expected) {
		t.Fatalf("expected content %q, got %q", expected, content)
	}
}

// TestHTTPFetcherRejectsManifestRedirect 验证 manifest 拉取不跟随重定向，避免绕过已校验的 HTTPS allowlist URL。
func TestHTTPFetcherRejectsManifestRedirect(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"version":"0.2.0"}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	_, err := (HTTPFetcher{Client: redirector.Client()}).FetchManifest(context.Background(), redirector.URL)
	if err == nil {
		t.Fatal("manifest redirect should be rejected")
	}
}
