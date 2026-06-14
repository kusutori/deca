package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kusutori/deca/internal/cache"
	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/install"
)

func TestCacheCommandHelpers(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	oldPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
		os.Setenv("PATH", oldPath)
	})

	c := cache.NewCache()
	src := filepath.Join(tmpHome, "asset.tar.gz")
	if err := os.WriteFile(src, []byte("cache payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Put("owner/repo", "v1.0.0", "asset.tar.gz", src); err != nil {
		t.Fatal(err)
	}

	if got := formatSize(1536); got != "1.5 KB" {
		t.Fatalf("unexpected formatSize: %s", got)
	}
	if got := sizeFromEntries([]cache.Entry{{Size: 1}, {Size: 2}}); got != 3 {
		t.Fatalf("unexpected entry size: %d", got)
	}
	if err := doCacheShow(); err != nil {
		t.Fatalf("doCacheShow failed: %v", err)
	}
	if err := doCacheList(); err != nil {
		t.Fatalf("doCacheList failed: %v", err)
	}
	if err := doCacheSize(); err != nil {
		t.Fatalf("doCacheSize failed: %v", err)
	}
	if err := doCacheClean(false, false); err != nil {
		t.Fatalf("doCacheClean prompt path failed: %v", err)
	}
	if err := doCacheClean(false, true); err != nil {
		t.Fatalf("doCacheClean orphans failed: %v", err)
	}
	if err := doCacheClean(true, false); err != nil {
		t.Fatalf("doCacheClean all failed: %v", err)
	}
	if err := doCacheList(); err != nil {
		t.Fatalf("doCacheList empty failed: %v", err)
	}
}

func TestMirrorCommandHelpers(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	if err := doMirrorShow(); err != nil {
		t.Fatalf("doMirrorShow failed: %v", err)
	}
	if err := doMirrorList(); err != nil {
		t.Fatalf("doMirrorList failed: %v", err)
	}
	if err := doMirrorSelect(); err != nil {
		t.Fatalf("doMirrorSelect non-interactive failed: %v", err)
	}
	if err := doMirrorAdd("Test Mirror", "https://mirror.example.com"); err != nil {
		t.Fatalf("doMirrorAdd failed: %v", err)
	}
	if err := doMirrorAdd("Test Mirror", "https://mirror.example.com"); err == nil {
		t.Fatal("expected duplicate mirror error")
	}
	if err := doMirrorRemove("GitHub (Official)"); err == nil {
		t.Fatal("expected default mirror removal error")
	}
	if err := doMirrorRemove("missing"); err == nil {
		t.Fatal("expected missing mirror error")
	}
	if err := doMirrorRemove("Test Mirror"); err != nil {
		t.Fatalf("doMirrorRemove failed: %v", err)
	}

	for _, tc := range []struct {
		name string
		url  string
	}{
		{"Jihulab", "https://jihulab.com"},
		{"FastGit", "https://fastgit.org"},
		{"GhFast", "https://ghfast.top"},
		{"GhProxy", "https://ghproxy.example.com"},
	} {
		if err := doMirrorAdd(tc.name, tc.url); err != nil {
			t.Fatalf("doMirrorAdd %s failed: %v", tc.name, err)
		}
	}

	apiURL, downloadURL := GetMirrorURLs()
	if apiURL == "" || downloadURL == "" {
		t.Fatalf("expected mirror urls, got %q %q", apiURL, downloadURL)
	}
	_ = GetCurrentMirror()
	_ = IsUsingMirror()

	mirror := &config.Mirror{DownloadURL: "https://example.com/{owner}/{repo}/{tag}/{asset}"}
	if got := buildDownloadTestURL(mirror); !strings.Contains(got, "gh_2.0.0_linux_amd64.tar.gz") {
		t.Fatalf("unexpected download test url: %s", got)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = time.Second
	runMirrorProbe(client, "API", server.URL, "GET")
	runMirrorProbe(client, "bad", "://bad-url", "GET")

	mirrorCfg := &config.MirrorConfig{
		CurrentName: "Local",
		Mirrors: []config.Mirror{{
			Name:        "Local",
			URL:         server.URL,
			APIURL:      server.URL,
			DownloadURL: server.URL + "/{owner}/{repo}/{tag}/{asset}",
		}},
	}
	if err := config.SaveMirrorConfig(config.GetMirrorPath(), mirrorCfg); err != nil {
		t.Fatal(err)
	}
	if err := doMirrorTest(); err != nil {
		t.Fatalf("doMirrorTest failed: %v", err)
	}
}

func TestDesktopCommandHelpers(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	oldPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
		os.Setenv("PATH", oldPath)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	origConfigPath := configPath
	configPath = cfgPath
	t.Cleanup(func() { configPath = origConfigPath })

	cfg := &config.Config{
		BinDir: filepath.Join(tmpHome, "bin"),
		OS:     "linux",
		Arch:   "amd64",
		Packages: map[string]config.Package{
			"tool": {
				Repo: "owner/tool",
				Desktop: &config.DesktopConfig{
					Name:       "Tool",
					Comment:    "Useful tool",
					Icon:       "tool",
					Terminal:   true,
					Categories: "Utility",
					MimeTypes:  "text/plain",
				},
			},
		},
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	resetDesktopFlags := func() {
		execPath, icon, comment, categories, mimeTypes, name = "", "", "", "", "", ""
		terminal, remove, dryRun = false, false, false
	}
	resetDesktopFlags()
	t.Cleanup(resetDesktopFlags)

	if got := expandPath(`"$HOME/bin/tool"`); !strings.Contains(filepath.ToSlash(got), filepath.ToSlash(filepath.Join(tmpHome, "bin", "tool"))) {
		t.Fatalf("unexpected expanded path: %s", got)
	}
	desktop := generateDesktopFile("Tool", "/bin/tool", "Comment", "icon", true, "Utility", "text/plain")
	for _, want := range []string{"Name=Tool", "Comment=Comment", "Exec=/bin/tool %U", "Icon=icon", "Terminal=true", "Categories=Utility;", "MimeType=text/plain;"} {
		if !strings.Contains(desktop, want) {
			t.Fatalf("desktop content missing %q:\n%s", want, desktop)
		}
	}
	if err := generateDesktopEntry("missing"); err == nil {
		t.Fatal("expected missing package error")
	}
	if err := generateDesktopEntry("tool"); err != nil {
		t.Fatalf("generateDesktopEntry failed: %v", err)
	}
	if err := removeDesktopEntry("tool"); err != nil {
		t.Fatalf("removeDesktopEntry failed: %v", err)
	}
	if err := removeDesktopEntry("tool"); err != nil {
		t.Fatalf("removeDesktopEntry missing should succeed: %v", err)
	}
	dryRun = true
	if err := generateDesktopEntry("tool"); err != nil {
		t.Fatalf("generateDesktopEntry dry-run failed: %v", err)
	}
	if err := removeDesktopEntry("tool"); err != nil {
		t.Fatalf("removeDesktopEntry dry-run missing failed: %v", err)
	}
}

func TestConfigAndSearchHelpers(t *testing.T) {
	oldEditor := os.Getenv("EDITOR")
	oldVisual := os.Getenv("VISUAL")
	os.Setenv("EDITOR", "editor-cmd")
	os.Setenv("VISUAL", "visual-cmd")
	t.Cleanup(func() {
		os.Setenv("EDITOR", oldEditor)
		os.Setenv("VISUAL", oldVisual)
	})
	if got := getEditor(); got != "editor-cmd" {
		t.Fatalf("expected EDITOR, got %q", got)
	}
	os.Unsetenv("EDITOR")
	if got := getEditor(); got != "visual-cmd" {
		t.Fatalf("expected VISUAL, got %q", got)
	}

	if got := truncate("short", 10); got != "short" {
		t.Fatalf("unexpected short truncate: %q", got)
	}
	if got := truncate("abcdefghijklmnopqrstuvwxyz", 10); got != "abcdefg..." {
		t.Fatalf("unexpected long truncate: %q", got)
	}
}

func TestLocalStateCommands(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	oldPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
		os.Setenv("PATH", oldPath)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	binDir := filepath.Join(tmpHome, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	binName := "tool"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(binDir, binName)
	if err := os.WriteFile(binPath, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BinDir: binDir,
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Packages: map[string]config.Package{
			"tool":  {Repo: "owner/tool", Asset: "*tool*", OS: runtime.GOOS, Arch: runtime.GOARCH},
			"extra": {Repo: "owner/extra"},
		},
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	state := &config.State{Packages: map[string]config.InstalledPackage{
		"tool": {
			Repo:        "owner/tool",
			Version:     "v1.0.0",
			AssetName:   "tool.zip",
			InstallType: config.InstallTypeBinary,
			ExposedPath: binPath,
		},
		"old": {
			Repo:    "owner/old",
			Version: "v0.1.0",
		},
	}}
	if err := state.SaveState(config.DefaultStatePath()); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, string, error) {
		return runCmd(t, append([]string{"--config", cfgPath}, args...)...)
	}

	if out, errOut, err := run("config", "path"); err != nil || !strings.Contains(out+errOut, cfgPath) {
		t.Fatalf("config path failed: out=%q errOut=%q err=%v", out, errOut, err)
	}
	if _, _, err := run("config", "show"); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	if _, _, err := run("config", "diff"); err != nil {
		t.Fatalf("config diff failed: %v", err)
	}
	if _, _, err := run("list", "--verbose"); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if _, _, err := run("remove", "tool", "--keep-installed"); err != nil {
		t.Fatalf("remove keep-installed failed: %v", err)
	}
	if _, _, err := run("remove", "old"); err != nil {
		t.Fatalf("remove installed-only failed: %v", err)
	}
}

func TestSchemaInitDoctorAndInteractiveHelpers(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	oldPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
		os.Setenv("PATH", oldPath)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	schemaPath := filepath.Join(tmpHome, "schema.json")
	binDir := filepath.Join(tmpHome, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	cfg := &config.Config{
		BinDir: binDir,
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Packages: map[string]config.Package{
			"tool": {Repo: "owner/tool"},
		},
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	state := &config.State{Packages: map[string]config.InstalledPackage{
		"tool": {Repo: "owner/tool", Version: "v1.0.0"},
	}}
	if err := state.SaveState(config.DefaultStatePath()); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, string, error) {
		return runCmd(t, append([]string{"--config", cfgPath}, args...)...)
	}

	if _, _, err := run("schema", "--output", schemaPath, "--inject"); err != nil {
		t.Fatalf("schema output/inject failed: %v", err)
	}
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if _, _, err := run("init"); err != nil {
		t.Fatalf("init existing config failed: %v", err)
	}

	apiState := &apiServerState{
		tag:       "v1.0.0",
		assetName: "tool-linux-amd64.tar.gz",
		assetData: []byte("asset"),
	}
	apiServer := newAPIServer(t, apiState)
	defer apiServer.Close()
	apiURL, err := url.Parse(apiServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: oldTransport, apiURL: apiURL}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	if _, _, err := run("doctor"); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}

func TestSearchCommandWithMockGitHub(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	if err := config.Save(&config.Config{Packages: map[string]config.Package{}}, cfgPath); err != nil {
		t.Fatal(err)
	}

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"items": []map[string]any{
				{
					"full_name":        "owner/tool",
					"description":      strings.Repeat("useful ", 20),
					"stargazers_count": 42,
					"updated_at":       "2026-01-02T03:04:05Z",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer searchServer.Close()

	apiURL, err := url.Parse(searchServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: oldTransport, apiURL: apiURL}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	out, errOut, err := runCmd(t, "--config", cfgPath, "search", "tool")
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, errOut)
	}
	if !strings.Contains(out+errOut, "owner/tool") {
		t.Fatalf("search output missing repository:\n%s\n%s", out, errOut)
	}
}

func TestStatusCommandAllUpToDate(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	cfg := &config.Config{
		BinDir: filepath.Join(tmpHome, "bin"),
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Packages: map[string]config.Package{
			"tool": {Repo: "owner/tool"},
		},
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	state := &config.State{Packages: map[string]config.InstalledPackage{
		"tool": {Repo: "owner/tool", Version: "v1.0.0"},
	}}
	if err := state.SaveState(config.DefaultStatePath()); err != nil {
		t.Fatal(err)
	}

	apiState := &apiServerState{
		tag:       "v1.0.0",
		assetName: "tool-linux-amd64.tar.gz",
		assetData: []byte("asset"),
	}
	apiServer := newAPIServer(t, apiState)
	defer apiServer.Close()
	apiURL, err := url.Parse(apiServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: oldTransport, apiURL: apiURL}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	out, errOut, err := runCmd(t, "--config", cfgPath, "status")
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, errOut)
	}
	if !strings.Contains(out+errOut, "up to date") {
		t.Fatalf("status output missing up-to-date message:\n%s\n%s", out, errOut)
	}
}

func TestUpdateCommandLocalBranches(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	})

	cfgPath := filepath.Join(tmpHome, "deca.toml")
	cfg := &config.Config{
		BinDir: filepath.Join(tmpHome, "bin"),
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Packages: map[string]config.Package{
			"skip": {Repo: "owner/skip", OS: "definitely-not-" + runtime.GOOS},
		},
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	state := &config.State{Packages: map[string]config.InstalledPackage{}}
	if err := state.SaveState(config.DefaultStatePath()); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, "--config", cfgPath, "update", "missing"); err == nil {
		t.Fatal("expected missing package update error")
	}
	if _, _, err := runCmd(t, "--config", cfgPath, "update"); err != nil {
		t.Fatalf("update skipped package failed: %v", err)
	}
}

func TestReleaseForPackagePinnedVersions(t *testing.T) {
	apiState := &apiServerState{
		tag:       "v1.2.3",
		assetName: "tool-linux-amd64.tar.gz",
		assetData: []byte("asset"),
	}
	apiServer := newAPIServer(t, apiState)
	defer apiServer.Close()
	apiURL, err := url.Parse(apiServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: oldTransport, apiURL: apiURL}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	ghClient := github.NewClient()
	release, err := releaseForPackage(context.Background(), ghClient, "owner", "tool", &config.Package{Version: "v1.2.3"}, false)
	if err != nil {
		t.Fatalf("releaseForPackage v tag failed: %v", err)
	}
	if release.TagName != "v1.2.3" {
		t.Fatalf("unexpected tag: %s", release.TagName)
	}

	release, err = releaseForPackage(context.Background(), ghClient, "owner", "tool", &config.Package{Version: "1.2.3"}, false)
	if err != nil {
		t.Fatalf("releaseForPackage fallback tag failed: %v", err)
	}
	if release.TagName != "1.2.3" {
		t.Fatalf("unexpected fallback tag: %s", release.TagName)
	}
	if tags := candidateTags(" "); tags != nil {
		t.Fatalf("expected nil tags for blank version, got %+v", tags)
	}
}

func TestInstalledPackageFromResultCarriesWindowsMetadata(t *testing.T) {
	pkg := &config.Package{
		Repo:        "owner/tool",
		InstallType: "portable",
	}
	installer := install.NewInstaller(t.TempDir())
	release := &github.ReleaseInfo{TagName: "v2.0.0"}
	result := &install.InstallResult{
		AssetName:   "tool.exe",
		InstallType: config.InstallTypeWindowsMSI,
		InstallRoot: filepath.Join(t.TempDir(), "packages", "tool", "v2.0.0"),
		ExposedPath: filepath.Join(t.TempDir(), "bin", "tool.exe"),
		ProductCode: "{PRODUCT-CODE}",
		LinkType:    "copy",
	}

	installed := installedPackageFromResult("tool", pkg, installer, release, result)
	if installed.Version != "v2.0.0" ||
		installed.AssetName != result.AssetName ||
		installed.InstallType != result.InstallType ||
		installed.InstallRoot != result.InstallRoot ||
		installed.ExposedPath != result.ExposedPath ||
		installed.ProductCode != result.ProductCode ||
		installed.LinkType != result.LinkType {
		t.Fatalf("metadata not preserved: %+v", installed)
	}
}
