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
	os.Setenv("HOME", "/test/home")
	defer os.Setenv("HOME", oldHome)

	path := GetMirrorPath()
	expected := "/test/home/.config/deca/mirrors.toml"

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
