package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	gh "github.com/google/go-github/v60/github"
	"github.com/kusutori/deca/internal/config"
)

func TestGetLatestReleaseWithOptions_IncludePrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"tag_name":   "v2.0.0-rc1",
				"prerelease": true,
				"draft":      false,
				"assets":     []map[string]any{},
				"html_url":   "https://example.com/release/prerelease",
			},
			{
				"tag_name":   "v1.9.0",
				"prerelease": false,
				"draft":      false,
				"assets":     []map[string]any{},
				"html_url":   "https://example.com/release/stable",
			},
		})
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")

	release, err := client.GetLatestReleaseWithOptions(context.Background(), "owner", "repo", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v2.0.0-rc1" {
		t.Fatalf("expected prerelease tag, got %s", release.TagName)
	}
}

func TestReleaseAPIsAndSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name":     "v1.0.0",
				"html_url":     "https://example.com/v1",
				"published_at": "2024-01-02T03:04:05Z",
				"assets": []map[string]any{{
					"name":                 "tool-linux-amd64.tar.gz",
					"browser_download_url": "https://github.com/owner/repo/releases/download/v1.0.0/tool-linux-amd64.tar.gz",
					"size":                 123,
				}},
			})
		case "/repos/owner/repo/releases/tags/v1.0.0":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v1.0.0",
				"assets":   []map[string]any{},
			})
		case "/search/repositories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"full_name":        "owner/repo",
					"name":             "repo",
					"description":      "desc",
					"stargazers_count": 42,
					"updated_at":       "2024-01-02T03:04:05Z",
				}},
			})
		default:
			dump, _ := httputil.DumpRequest(r, false)
			t.Fatalf("unexpected request:\n%s", dump)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")
	client.client.UploadURL = mustParseURL(t, server.URL+"/")

	latest, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("GetLatestRelease failed: %v", err)
	}
	if latest.TagName != "v1.0.0" || latest.Assets[0].Name == "" || latest.Published == "" {
		t.Fatalf("unexpected latest release: %+v", latest)
	}

	byTag, err := client.GetReleaseByTag(context.Background(), "owner", "repo", "v1.0.0")
	if err != nil {
		t.Fatalf("GetReleaseByTag failed: %v", err)
	}
	if byTag.TagName != "v1.0.0" {
		t.Fatalf("unexpected tag release: %+v", byTag)
	}

	results, err := client.SearchRepositories(context.Background(), "repo")
	if err != nil {
		t.Fatalf("SearchRepositories failed: %v", err)
	}
	if len(results) != 1 || results[0].FullName != "owner/repo" || results[0].UpdatedAt != "2024-01-02" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse url %q: %v", raw, err)
	}
	return parsed
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"sharkdp/bat", "sharkdp", "bat", false},
		{"eza-community/eza", "eza-community", "eza", false},
		{"owner", "", "", true},
		{"owner/multi/name", "owner", "multi/name", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, name, err := ParseRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("expected owner '%s', got '%s'", tt.wantOwner, owner)
			}
			if name != tt.wantName {
				t.Errorf("expected name '%s', got '%s'", tt.wantName, name)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.tar.gz", "file.tar.gz", true},
		{"*.tar.gz", "file.tar.xz", false},
		{"*.exe", "app.exe", true},
		{"bat*", "bat-v0.24.0.tar.gz", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.name, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.name)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

func TestMatchesOSArch(t *testing.T) {
	tests := []struct {
		name string
		os   string
		arch string
		want bool
	}{
		// Linux tests
		{"tool-linux-amd64.tar.gz", "linux", "amd64", true},
		{"tool-linux-x86_64.tar.gz", "linux", "amd64", true},
		{"tool-linux-arm64.tar.gz", "linux", "arm64", true},
		{"tool-linux-amd64.tar.gz", "darwin", "amd64", false},
		{"tool-linux-amd64.tar.gz", "linux", "arm64", false},

		// macOS tests
		{"tool-darwin-amd64.tar.gz", "darwin", "amd64", true},
		{"tool-darwin-arm64.tar.gz", "darwin", "arm64", true},
		{"tool-darwin-amd64.tar.gz", "linux", "amd64", false},

		// Windows tests - .exe without arch should match if os is windows
		{"tool-windows.exe", "windows", "amd64", true},
		{"tool-windows.exe", "windows", "", true},
		{"tool-windows-amd64.zip", "windows", "amd64", true},
		{"ToolSetup.msi", "windows", "amd64", true},
		{"tool-windows.exe", "linux", "amd64", false},

		// No constraints
		{"any-file.tar.gz", "", "", true},
		{"any-file.tar.gz", "linux", "", true},

		// Architecture variants
		{"tool-x86_64.tar.gz", "linux", "amd64", true},
		{"tool-amd64.tar.gz", "linux", "x86_64", true},
		{"tool-arm64.tar.gz", "linux", "aarch64", true},
		{"tool-aarch64.tar.gz", "linux", "arm64", true},

		// x64 shorthand (common in Electron/AppImage apps)
		{"Craft-Agents-0.4.8-linux-x64.AppImage", "linux", "amd64", true},
		{"app-linux-x64.AppImage", "linux", "amd64", true},
		{"app-mac-arm64.dmg", "linux", "amd64", false},
		{"app-linux-arm64.AppImage", "linux", "amd64", false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.os+"_"+tt.arch, func(t *testing.T) {
			got := matchesOSArch(tt.name, tt.os, tt.arch)
			if got != tt.want {
				t.Errorf("matchesOSArch(%q, %q, %q) = %v, want %v", tt.name, tt.os, tt.arch, got, tt.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s    string
		subs []string
		want bool
	}{
		{"hello world", []string{"hello"}, true},
		{"hello world", []string{"foo"}, false},
		{"hello world", []string{"foo", "world"}, true},
		{"hello", []string{"a", "b", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := containsAny(tt.s, tt.subs...)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.subs, got, tt.want)
			}
		})
	}
}

func TestGetLatestReleaseWithOptions_NoReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")
	if _, err := client.GetLatestReleaseWithOptions(context.Background(), "owner", "repo", true); err == nil {
		t.Fatal("expected no releases error")
	}
}

func TestGetLatestReleaseWithOptions_ListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")
	if _, err := client.GetLatestReleaseWithOptions(context.Background(), "owner", "repo", true); err == nil {
		t.Fatal("expected list releases error")
	}
}

func TestFindMatchingAsset(t *testing.T) {
	release := &ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "tool-linux-amd64.tar.gz", DownloadURL: "http://example.com/linux-amd64.tar.gz"},
			{Name: "tool-linux-arm64.tar.gz", DownloadURL: "http://example.com/linux-arm64.tar.gz"},
			{Name: "tool-darwin-amd64.tar.gz", DownloadURL: "http://example.com/darwin-amd64.tar.gz"},
			{Name: "tool-windows-amd64.zip", DownloadURL: "http://example.com/windows-amd64.zip"},
			{Name: "ToolSetup.msi", DownloadURL: "http://example.com/tool.msi"},
		},
	}

	tests := []struct {
		name    string
		pattern string
		os      string
		arch    string
		want    string
		wantErr bool
	}{
		{"linux amd64", "", "linux", "amd64", "tool-linux-amd64.tar.gz", false},
		{"linux arm64", "", "linux", "arm64", "tool-linux-arm64.tar.gz", false},
		{"darwin amd64", "", "darwin", "amd64", "tool-darwin-amd64.tar.gz", false},
		{"windows", "", "windows", "amd64", "tool-windows-amd64.zip", false},
		{"windows msi", "*.msi", "windows", "amd64", "ToolSetup.msi", false},
		{"no match", "", "freebsd", "amd64", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := FindMatchingAsset(release, tt.pattern, tt.os, tt.arch)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if asset.Name != tt.want {
				t.Errorf("expected asset '%s', got '%s'", tt.want, asset.Name)
			}
		})
	}
}

func TestAssetSelectionHelpers(t *testing.T) {
	windowsAssets := []*AssetInfo{
		{Name: "ToolSetup.msi"},
		{Name: "tool-windows.zip"},
		{Name: "tool.exe"},
	}
	if got := selectBestAsset(windowsAssets, "windows"); got.Name != "tool.exe" {
		t.Fatalf("expected exe priority on windows, got %s", got.Name)
	}

	linuxAssets := []*AssetInfo{
		{Name: "tool.rpm"},
		{Name: "tool.AppImage"},
		{Name: "tool.deb"},
		{Name: "tool-linux-amd64.tar.gz"},
	}
	if got := selectBestAsset(linuxAssets, "linux"); got.Name != "tool-linux-amd64.tar.gz" {
		t.Fatalf("expected archive priority on linux, got %s", got.Name)
	}

	if !hasArchiveSuffix("tool.msi") || !hasArchiveSuffix("tool.exe") || hasArchiveSuffix("tool") {
		t.Fatal("unexpected archive suffix detection")
	}
	if !matchesAsset(&AssetInfo{Name: "tool-windows.exe"}, "*.exe", "windows", "amd64") {
		t.Fatal("expected matchesAsset true")
	}
	if matchesAsset(&AssetInfo{Name: "tool-linux.tar.gz"}, "*.exe", "windows", "amd64") {
		t.Fatal("expected matchesAsset false")
	}

	patterns := map[string]string{
		"windows/amd64": guessAssetPattern("windows", "amd64"),
		"linux/amd64":   guessAssetPattern("linux", "amd64"),
		"darwin/arm64":  guessAssetPattern("darwin", "arm64"),
		"unknown":       guessAssetPattern("", ""),
	}
	if patterns["windows/amd64"] != "windows" || patterns["unknown"] != "" {
		t.Fatalf("unexpected guessed patterns: %+v", patterns)
	}
	if got := GetAssetDownloadURL("asset.zip", "owner", "repo", "v1"); got == "" {
		t.Fatal("expected download URL")
	}
}

func TestTokenTransportAddsAuthorization(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := http.Client{Transport: &tokenTransport{token: "secret"}}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if gotAuth != "Bearer secret" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
}

func TestNewClientWithToken(t *testing.T) {
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", "secret")
	t.Cleanup(func() { os.Setenv("GITHUB_TOKEN", oldToken) })

	client := NewClient()
	if client.token != "secret" {
		t.Fatalf("expected token to be captured, got %q", client.token)
	}
	if client.client == nil {
		t.Fatal("expected github client")
	}
}

func TestFetchMultipleReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/bad/repo/releases/latest" {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"assets":   []map[string]any{},
		})
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")
	results, errs := client.FetchMultipleReleases(context.Background(), []struct{ Owner, Repo string }{
		{"owner", "repo"},
		{"bad", "repo"},
	})
	if len(results) != 2 || results[0] == nil || results[0].TagName != "v1.0.0" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %+v", errs)
	}
}

func TestTransformDownloadURL_UsesMirrorConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg := &config.MirrorConfig{
		Mirrors: []config.Mirror{
			{
				Name:        "GitHub (Official)",
				URL:         "https://github.com",
				APIURL:      "https://api.github.com",
				DownloadURL: "https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}",
			},
			{
				Name:        "Test Mirror",
				URL:         "https://mirror.example.com",
				APIURL:      "https://mirror.example.com",
				DownloadURL: "https://mirror.example.com/{owner}/{repo}/releases/download/{tag}/{asset}",
			},
		},
		CurrentName: "Test Mirror",
	}

	path := filepath.Join(tmpDir, ".config", "deca", "mirrors.toml")
	if err := config.SaveMirrorConfig(path, cfg); err != nil {
		t.Fatalf("failed to save mirror config: %v", err)
	}

	original := "https://github.com/owner/repo/releases/download/v1.0.0/asset.tar.gz"
	got := TransformDownloadURL(original, "owner", "repo", "v1.0.0")
	want := "https://mirror.example.com/owner/repo/releases/download/v1.0.0/asset.tar.gz"
	if got != want {
		t.Fatalf("expected mirror url %s, got %s", want, got)
	}
}

func TestGetLatestRelease_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "9999999999")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	client := NewClient()
	client.client.BaseURL = mustParseURL(t, server.URL+"/")

	_, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatal("expected error")
	}

	var rl *gh.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected wrapped RateLimitError, got %T %v", err, err)
	}
}

type assetDigestRewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t *assetDigestRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.github.com" {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = t.target.Scheme
		clone.URL.Host = t.target.Host
		clone.Host = t.target.Host
		req = clone
	}
	return t.base.RoundTrip(req)
}

func TestGetAssetDigest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/assets/1":
			_ = json.NewEncoder(w).Encode(map[string]string{"digest": "sha256:abc"})
		case "/repos/owner/repo/releases/assets/2":
			http.Error(w, "missing", http.StatusNotFound)
		case "/repos/owner/repo/releases/assets/3":
			_, _ = w.Write([]byte("{bad json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultClient.Transport
	base := oldTransport
	if base == nil {
		base = http.DefaultTransport
	}
	http.DefaultClient.Transport = &assetDigestRewriteTransport{base: base, target: target}
	t.Cleanup(func() { http.DefaultClient.Transport = oldTransport })

	client := NewClient()
	if got := client.GetAssetDigest(context.Background(), "owner", "repo", 1); got != "sha256:abc" {
		t.Fatalf("unexpected digest: %q", got)
	}
	if got := client.GetAssetDigest(context.Background(), "owner", "repo", 2); got != "" {
		t.Fatalf("expected empty digest for non-200, got %q", got)
	}
	if got := client.GetAssetDigest(context.Background(), "owner", "repo", 3); got != "" {
		t.Fatalf("expected empty digest for invalid JSON, got %q", got)
	}
}
