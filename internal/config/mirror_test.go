package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMirrors(t *testing.T) {
	mirrors := DefaultMirrors()

	if len(mirrors) == 0 {
		t.Error("expected default mirrors to not be empty")
	}

	// Check that GitHub (Official) is first
	if len(mirrors) > 0 && mirrors[0].Name != "GitHub (Official)" {
		t.Errorf("expected first mirror to be GitHub (Official), got %s", mirrors[0].Name)
	}
}

func TestGetMirrorByName(t *testing.T) {
	mirrors := DefaultMirrors()

	// Test finding existing mirror
	mirror := GetMirrorByName("GitHub Fast (China)", mirrors)
	if mirror == nil {
		t.Error("expected to find GitHub Fast (China) mirror")
	}
	if mirror.URL != "https://ghfast.top" {
		t.Errorf("expected URL https://ghfast.top, got %s", mirror.URL)
	}
	if mirror.DownloadURL != "https://ghfast.top/https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}" {
		t.Errorf("unexpected ghfast download URL: %s", mirror.DownloadURL)
	}

	// Test non-existing mirror
	mirror = GetMirrorByName("NonExisting", mirrors)
	if mirror != nil {
		t.Error("expected nil for non-existing mirror")
	}
}

func TestDefaultMirrorConfig(t *testing.T) {
	cfg := DefaultMirrorConfig()

	if cfg.CurrentName != "GitHub (Official)" {
		t.Errorf("expected current to be GitHub (Official), got %s", cfg.CurrentName)
	}

	if len(cfg.Mirrors) == 0 {
		t.Error("expected mirrors to not be empty")
	}
}

func TestGetCurrentMirror(t *testing.T) {
	cfg := DefaultMirrorConfig()

	current := cfg.GetCurrentMirror()
	if current == nil {
		t.Error("expected current mirror to not be nil")
	}
	if current.Name != "GitHub (Official)" {
		t.Errorf("expected GitHub (Official), got %s", current.Name)
	}

	// Test with different current
	cfg.CurrentName = "GitHub Fast (China)"
	current = cfg.GetCurrentMirror()
	if current == nil {
		t.Error("expected current mirror to not be nil")
	}
	if current.Name != "GitHub Fast (China)" {
		t.Errorf("expected GitHub Fast (China), got %s", current.Name)
	}
}

func TestGetMirrorPath(t *testing.T) {
	oldHome := os.Getenv("HOME")
	testHome := filepath.Join(string(filepath.Separator), "test", "home")
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	path := GetMirrorPath()
	expected := filepath.Join(testHome, ".config", "deca", "mirrors.toml")

	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestSaveAndLoadMirrorConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mirrors.toml")

	// Create a test config
	cfg := &MirrorConfig{
		Mirrors: []Mirror{
			{
				Name:        "Test Mirror",
				URL:         "https://test.example.com",
				APIURL:      "https://test.example.com/api",
				DownloadURL: "https://test.example.com/{owner}/{repo}/releases/download/{tag}/{asset}",
			},
		},
		CurrentName: "Test Mirror",
	}

	// Save
	if err := SaveMirrorConfig(path, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists (LoadMirrorConfig currently returns default)
	if _, err := os.Stat(path); err != nil {
		t.Error("config file was not created")
	}
}

func TestLoadMirrorConfigFallbacksAndErrors(t *testing.T) {
	tmpDir := t.TempDir()

	missing, err := LoadMirrorConfig(filepath.Join(tmpDir, "missing.toml"))
	if err != nil {
		t.Fatalf("missing config should use defaults: %v", err)
	}
	if missing.CurrentName != "GitHub (Official)" || len(missing.Mirrors) == 0 {
		t.Fatalf("unexpected default mirror config: %+v", missing)
	}

	invalidPath := filepath.Join(tmpDir, "invalid.toml")
	if err := os.WriteFile(invalidPath, []byte("not = [valid"), 0644); err != nil {
		t.Fatal(err)
	}
	invalid, err := LoadMirrorConfig(invalidPath)
	if err != nil {
		t.Fatalf("invalid config should fall back to defaults: %v", err)
	}
	if invalid.CurrentName != "GitHub (Official)" {
		t.Fatalf("unexpected invalid fallback: %+v", invalid)
	}

	emptyPath := filepath.Join(tmpDir, "empty.toml")
	if err := os.WriteFile(emptyPath, []byte("current = \"\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	empty, err := LoadMirrorConfig(emptyPath)
	if err != nil {
		t.Fatalf("empty config should load: %v", err)
	}
	if empty.CurrentName != "GitHub (Official)" || len(empty.Mirrors) == 0 {
		t.Fatalf("expected defaults merged for empty config: %+v", empty)
	}

	if err := SaveMirrorConfig(string([]byte{0}), DefaultMirrorConfig()); err == nil {
		t.Fatal("expected invalid path error")
	}
}

func TestLoadCurrentMirrorFallback(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	mirror := LoadCurrentMirror()
	if mirror == nil || mirror.Name != "GitHub (Official)" {
		t.Fatalf("unexpected current mirror: %+v", mirror)
	}
}

func TestMirrorDownloadURLPattern(t *testing.T) {
	// Test that download URLs contain the expected placeholders
	mirrors := DefaultMirrors()

	for _, m := range mirrors {
		if m.DownloadURL == "" {
			t.Errorf("mirror %s has empty download URL", m.Name)
		}

		// Check for expected placeholders
		if !contains(m.DownloadURL, "{owner}") {
			t.Errorf("mirror %s download URL missing {owner} placeholder", m.Name)
		}
		if !contains(m.DownloadURL, "{repo}") {
			t.Errorf("mirror %s download URL missing {repo} placeholder", m.Name)
		}
		if !contains(m.DownloadURL, "{tag}") {
			t.Errorf("mirror %s download URL missing {tag} placeholder", m.Name)
		}
		if !contains(m.DownloadURL, "{asset}") {
			t.Errorf("mirror %s download URL missing {asset} placeholder", m.Name)
		}
	}
}

func TestLoadMirrorConfigNormalizesGhfast(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mirrors.toml")

	cfg := &MirrorConfig{
		Mirrors: []Mirror{
			{
				Name:        "GitHub Fast (China)",
				URL:         "https://ghfast.top",
				APIURL:      "https://ghfast.top",
				DownloadURL: "https://ghfast.top/{url}",
			},
		},
		CurrentName: "GitHub Fast (China)",
	}

	if err := SaveMirrorConfig(path, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadMirrorConfig(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	m := loaded.GetCurrentMirror()
	if m == nil {
		t.Fatal("expected current mirror")
	}
	if m.APIURL != "https://api.github.com" {
		t.Fatalf("expected api.github.com, got %s", m.APIURL)
	}
	if m.DownloadURL != "https://ghfast.top/https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}" {
		t.Fatalf("unexpected ghfast download url: %s", m.DownloadURL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
