package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSimpleConfig(t *testing.T) {
	content := `
bin_dir = "/test/bin"

[packages]
eza = "eza-community/eza"
bat = "sharkdp/bat"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "deca.toml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.BinDir != "/test/bin" {
		t.Errorf("expected bin_dir '/test/bin', got '%s'", cfg.BinDir)
	}

	if len(cfg.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(cfg.Packages))
	}

	if pkg, ok := cfg.Packages["eza"]; !ok || pkg.Repo != "eza-community/eza" {
		t.Errorf("expected eza package to point to 'eza-community/eza'")
	}

	if pkg, ok := cfg.Packages["bat"]; !ok || pkg.Repo != "sharkdp/bat" {
		t.Errorf("expected bat package to point to 'sharkdp/bat'")
	}
}

func TestLoadFullConfig(t *testing.T) {
	content := `
bin_dir = "/usr/local/bin"
os = "linux"
arch = "amd64"

[packages]
zellij = { repo = "zellij-org/zellij", asset = "zellij.*x86_64" }
neovim = { repo = "neovim/neovim", version = "0.9.5", os = "linux", arch = "amd64" }
wezterm = { repo = "wez/wezterm", prerelease = true }

[settings]
auto_update = true
check_interval = "24h"

[system_info]
os = "linux"
arch = "amd64"
distribution = "ubuntu"
package_manager = "apt"
bin_dir = "/usr/local/bin"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "deca.toml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.BinDir != "/usr/local/bin" {
		t.Errorf("expected bin_dir '/usr/local/bin', got '%s'", cfg.BinDir)
	}

	if !cfg.Settings.AutoUpdate {
		t.Error("expected auto_update to be true")
	}

	if cfg.Settings.CheckInterval != "24h" {
		t.Errorf("expected check_interval '24h', got '%s'", cfg.Settings.CheckInterval)
	}

	if cfg.SystemInfo == nil {
		t.Fatal("expected system_info to be loaded")
	}
	if cfg.SystemInfo.Distribution != "ubuntu" {
		t.Errorf("expected system_info distribution 'ubuntu', got '%s'", cfg.SystemInfo.Distribution)
	}
	if cfg.SystemInfo.PackageManager != "apt" {
		t.Errorf("expected system_info package_manager 'apt', got '%s'", cfg.SystemInfo.PackageManager)
	}

	zellij, ok := cfg.Packages["zellij"]
	if !ok {
		t.Fatal("zellij package not found")
	}
	if zellij.Repo != "zellij-org/zellij" {
		t.Errorf("expected zellij repo 'zellij-org/zellij', got '%s'", zellij.Repo)
	}
	if zellij.Asset != "zellij.*x86_64" {
		t.Errorf("expected zellij asset 'zellij.*x86_64', got '%s'", zellij.Asset)
	}

	neovim, ok := cfg.Packages["neovim"]
	if !ok {
		t.Fatal("neovim package not found")
	}
	if neovim.Version != "0.9.5" {
		t.Errorf("expected neovim version '0.9.5', got '%s'", neovim.Version)
	}
	if neovim.OS != "linux" {
		t.Errorf("expected neovim os 'linux', got '%s'", neovim.OS)
	}
	if neovim.Arch != "amd64" {
		t.Errorf("expected neovim arch 'amd64', got '%s'", neovim.Arch)
	}

	wezterm, ok := cfg.Packages["wezterm"]
	if !ok {
		t.Fatal("wezterm package not found")
	}
	if !wezterm.Prerelease {
		t.Error("expected wezterm prerelease to be true")
	}
}

func TestDefaultBinDir(t *testing.T) {
	// Just verify it returns a non-empty string
	dir := DefaultBinDir()
	if dir == "" {
		t.Error("expected non-empty bin dir")
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/deca.toml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestPackageRef(t *testing.T) {
	pkg := Package{Repo: "owner/repo"}
	if ref := pkg.PackageRef(); ref != "owner/repo" {
		t.Errorf("expected 'owner/repo', got '%s'", ref)
	}

	emptyPkg := Package{}
	if ref := emptyPkg.PackageRef(); ref != "" {
		t.Errorf("expected empty string, got '%s'", ref)
	}
}

func TestDefaultDirsAndSaveLoadDefault(t *testing.T) {
	oldHome := getenvFunc("HOME")
	oldUserProfile := getenvFunc("USERPROFILE")
	oldLocalAppData := getenvFunc("LOCALAPPDATA")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "LocalAppData"))
	t.Cleanup(func() {
		t.Setenv("HOME", oldHome)
		t.Setenv("USERPROFILE", oldUserProfile)
		t.Setenv("LOCALAPPDATA", oldLocalAppData)
	})

	if got := DefaultConfigDir(); got != filepath.Join(tmpDir, ".config", "deca") {
		t.Fatalf("DefaultConfigDir = %s", got)
	}
	if got := DefaultDesktopDir(); got != filepath.Join(tmpDir, ".local", "share", "applications") {
		t.Fatalf("DefaultDesktopDir = %s", got)
	}
	if got := DesktopEntryPath("tool"); got != filepath.Join(tmpDir, ".local", "share", "applications", "tool.desktop") {
		t.Fatalf("DesktopEntryPath = %s", got)
	}

	cfg := &Config{
		BinDir:   filepath.Join(tmpDir, "bin"),
		OS:       "linux",
		Arch:     "amd64",
		Packages: map[string]Package{"tool": {Repo: "owner/tool"}},
	}
	path := filepath.Join(DefaultConfigDir(), "deca.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}
	if loaded.Packages["tool"].Repo != "owner/tool" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "broken.toml")
	if err := os.WriteFile(configPath, []byte("[packages\ninvalid"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
