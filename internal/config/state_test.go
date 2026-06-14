package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadState(t *testing.T) {
	content := `{
  "packages": {
    "eza": {
      "repo": "eza-community/eza",
      "version": "v0.18.0",
      "asset_name": "eza-x86_64-unknown-linux-musl.tar.gz",
      "installed_at": "2024-01-15T10:30:00Z"
    }
  }
}`
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(state.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(state.Packages))
	}

	pkg, ok := state.Packages["eza"]
	if !ok {
		t.Fatal("eza package not found")
	}
	if pkg.Repo != "eza-community/eza" {
		t.Errorf("expected repo 'eza-community/eza', got '%s'", pkg.Repo)
	}
	if pkg.Version != "v0.18.0" {
		t.Errorf("expected version 'v0.18.0', got '%s'", pkg.Version)
	}
}

func TestLoadEmptyState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if state.Packages == nil {
		t.Error("expected non-nil packages map")
	}
	if len(state.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(state.Packages))
	}
}

func TestSaveState(t *testing.T) {
	state := &State{
		Packages: map[string]InstalledPackage{
			"bat": {
				Repo:        "sharkdp/bat",
				Version:     "v0.24.0",
				AssetName:   "bat-x86_64-unknown-linux-musl.tar.gz",
				InstalledAt: time.Now(),
			},
		},
	}

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	if err := state.SaveState(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Load and verify
	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load saved state: %v", err)
	}

	pkg, ok := loaded.Packages["bat"]
	if !ok {
		t.Fatal("bat package not found in loaded state")
	}
	if pkg.Repo != "sharkdp/bat" {
		t.Errorf("expected repo 'sharkdp/bat', got '%s'", pkg.Repo)
	}
}

func TestSetPackage(t *testing.T) {
	state := &State{
		Packages: make(map[string]InstalledPackage),
	}

	state.SetPackage("eza", InstalledPackage{
		Repo:    "eza-community/eza",
		Version: "v0.18.0",
	})

	if len(state.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(state.Packages))
	}

	pkg, ok := state.Packages["eza"]
	if !ok {
		t.Fatal("eza package not found")
	}
	if pkg.Repo != "eza-community/eza" {
		t.Errorf("expected repo 'eza-community/eza', got '%s'", pkg.Repo)
	}

	// Update
	state.SetPackage("eza", InstalledPackage{
		Repo:    "eza-community/eza",
		Version: "v0.19.0",
	})

	if len(state.Packages) != 1 {
		t.Errorf("expected 1 package after update, got %d", len(state.Packages))
	}

	pkg = state.Packages["eza"]
	if pkg.Version != "v0.19.0" {
		t.Errorf("expected version 'v0.19.0', got '%s'", pkg.Version)
	}
}

func TestRemovePackage(t *testing.T) {
	state := &State{
		Packages: map[string]InstalledPackage{
			"eza": {Repo: "eza-community/eza"},
			"bat": {Repo: "sharkdp/bat"},
		},
	}

	state.RemovePackage("eza")

	if len(state.Packages) != 1 {
		t.Errorf("expected 1 package after removal, got %d", len(state.Packages))
	}

	if _, ok := state.Packages["eza"]; ok {
		t.Error("eza package should have been removed")
	}

	if _, ok := state.Packages["bat"]; !ok {
		t.Error("bat package should still exist")
	}
}

func TestGetPackage(t *testing.T) {
	state := &State{
		Packages: map[string]InstalledPackage{
			"eza": {Repo: "eza-community/eza", Version: "v0.18.0"},
		},
	}

	pkg, ok := state.GetPackage("eza")
	if !ok {
		t.Error("expected eza package to exist")
	}
	if pkg.Version != "v0.18.0" {
		t.Errorf("expected version 'v0.18.0', got '%s'", pkg.Version)
	}

	_, ok = state.GetPackage("nonexistent")
	if ok {
		t.Error("expected nonexistent package to return false")
	}
}

func TestDefaultStatePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "state", "deca", "state.json")

	if path := DefaultStatePath(); path != expected {
		t.Errorf("expected '%s', got '%s'", expected, path)
	}
}

func TestStateWithSystemPkgName(t *testing.T) {
	// Test that SystemPkgName is correctly saved and loaded
	state := &State{
		Packages: map[string]InstalledPackage{
			"fresh": {
				Repo:          "sinelaw/fresh",
				Version:       "v0.1.98",
				AssetName:     "fresh-editor_0.1.98-1_amd64.deb",
				InstallType:   InstallTypeSystem,
				SystemPkgName: "fresh-editor",
				InstalledAt:   time.Now(),
			},
		},
	}

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	if err := state.SaveState(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Load and verify
	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load saved state: %v", err)
	}

	pkg, ok := loaded.Packages["fresh"]
	if !ok {
		t.Fatal("fresh package not found in loaded state")
	}

	if pkg.SystemPkgName != "fresh-editor" {
		t.Errorf("expected SystemPkgName 'fresh-editor', got '%s'", pkg.SystemPkgName)
	}

	if pkg.InstallType != InstallTypeSystem {
		t.Errorf("expected InstallType 'system', got '%s'", pkg.InstallType)
	}
}

func TestLoadStateWithSystemPkgName(t *testing.T) {
	// Test loading state from JSON that includes system_pkg_name
	content := `{
  "packages": {
    "fresh": {
      "repo": "sinelaw/fresh",
      "version": "v0.1.98",
      "asset_name": "fresh-editor_0.1.98-1_amd64.deb",
      "install_type": "system",
      "system_pkg_name": "fresh-editor",
      "installed_at": "2024-01-15T10:30:00Z"
    }
  }
}`
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	pkg, ok := state.Packages["fresh"]
	if !ok {
		t.Fatal("fresh package not found")
	}

	if pkg.SystemPkgName != "fresh-editor" {
		t.Errorf("expected SystemPkgName 'fresh-editor', got '%s'", pkg.SystemPkgName)
	}
}

func TestStateWithWindowsInstallMetadata(t *testing.T) {
	state := &State{
		Packages: map[string]InstalledPackage{
			"tool": {
				Repo:        "owner/tool",
				Version:     "v1.2.3",
				AssetName:   "tool-windows-amd64.zip",
				InstallType: InstallTypeBinary,
				InstalledAt: time.Now(),
				InstallRoot: filepath.Join("C:", "Users", "me", "AppData", "Local", "deca", "packages", "tool", "v1.2.3"),
				ExposedPath: filepath.Join("C:", "Users", "me", "AppData", "Local", "deca", "bin", "tool.exe"),
				ProductCode: "{00000000-0000-0000-0000-000000000000}",
				LinkType:    "hardlink",
			},
		},
	}

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	if err := state.SaveState(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	pkg, ok := loaded.Packages["tool"]
	if !ok {
		t.Fatal("tool package not found")
	}
	if pkg.InstallRoot == "" || pkg.ExposedPath == "" || pkg.ProductCode == "" || pkg.LinkType != "hardlink" {
		t.Fatalf("windows metadata was not preserved: %+v", pkg)
	}
}

func TestLoadStateReadFailure(t *testing.T) {
	orig := stateReadFile
	stateReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	defer func() { stateReadFile = orig }()

	_, err := LoadState("/any")
	if !os.IsPermission(err) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestSaveStateDependencyFailures(t *testing.T) {
	state := &State{Packages: map[string]InstalledPackage{}}

	t.Run("mkdir failure", func(t *testing.T) {
		origMkdir := stateMkdirAll
		stateMkdirAll = func(string, os.FileMode) error { return os.ErrPermission }
		defer func() { stateMkdirAll = origMkdir }()
		if err := state.SaveState("/tmp/state.json"); !os.IsPermission(err) {
			t.Fatalf("expected permission error, got %v", err)
		}
	})

	t.Run("write failure", func(t *testing.T) {
		origMkdir := stateMkdirAll
		origWrite := stateWriteFile
		stateMkdirAll = func(string, os.FileMode) error { return nil }
		stateWriteFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
		defer func() {
			stateMkdirAll = origMkdir
			stateWriteFile = origWrite
		}()
		if err := state.SaveState("/tmp/state.json"); !os.IsPermission(err) {
			t.Fatalf("expected permission error, got %v", err)
		}
	})
}
