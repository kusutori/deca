package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

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

func TestFindMatchingAsset(t *testing.T) {
	release := &ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "tool-linux-amd64.tar.gz", DownloadURL: "http://example.com/linux-amd64.tar.gz"},
			{Name: "tool-linux-arm64.tar.gz", DownloadURL: "http://example.com/linux-arm64.tar.gz"},
			{Name: "tool-darwin-amd64.tar.gz", DownloadURL: "http://example.com/darwin-amd64.tar.gz"},
			{Name: "tool-windows-amd64.zip", DownloadURL: "http://example.com/windows-amd64.zip"},
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
