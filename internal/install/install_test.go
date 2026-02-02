package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deca-org/deca/internal/github"
)

func TestNewInstaller(t *testing.T) {
	installer := NewInstaller("/test/bin")
	if installer.BinDir != "/test/bin" {
		t.Errorf("expected bin_dir '/test/bin', got '%s'", installer.BinDir)
	}
}

func TestEnsureBinDir(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")

	installer := NewInstaller(binDir)
	if err := installer.EnsureBinDir(); err != nil {
		t.Fatalf("failed to create bin directory: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(binDir); err != nil {
		t.Errorf("bin directory not created: %v", err)
	}
}

func TestBinDirInPATH(t *testing.T) {
	// Create a temp directory and add it to PATH
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)

	// Without PATH modification, it should be false
	if installer.BinDirInPATH() {
		t.Error("expected BinDirInPATH to return false when not in PATH")
	}

	// Note: We can't easily modify the process PATH for testing
	// This would require subprocess testing
}

func TestUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	installer := NewInstaller(binDir)

	// Create the bin directory first
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Create a test binary
	binaryPath := filepath.Join(binDir, "testbinary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	// Uninstall
	if err := installer.Uninstall("testbinary"); err != nil {
		t.Fatalf("failed to uninstall: %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Error("binary should have been removed")
	}
}

func TestUninstallNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	installer := NewInstaller(binDir)

	// Try to uninstall non-existent binary
	err := installer.Uninstall("nonexistent")
	if err != os.ErrNotExist {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	// Write source file
	content := "test content"
	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

func TestCopyFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	// Write source file with execute permissions
	if err := os.WriteFile(src, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}
	// Should have execute permission
	if info.Mode()&0111 == 0 {
		t.Error("destination should have execute permissions")
	}
}

func TestAddToPATHInstructions(t *testing.T) {
	tests := []struct {
		shell    string
		binDir   string
		wantHome bool
	}{
		{"bash", "/home/user/.local/bin", true},
		{"zsh", "/home/user/.local/bin", true},
		{"fish", "/home/user/.local/bin", true},
		{"unknown", "/custom/bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			installer := NewInstaller(tt.binDir)
			instructions := installer.AddToPATHInstructions()

			if tt.wantHome {
				if instructions == "" {
					t.Error("expected non-empty instructions")
				}
			}
		})
	}
}

func TestDetectShell(t *testing.T) {
	// Save original
	original := os.Getenv("SHELL")
	defer os.Setenv("SHELL", original)

	tests := []struct {
		shell string
		want  string
	}{
		{"/bin/bash", "bash"},
		{"/usr/bin/bash", "bash"},
		{"/bin/zsh", "zsh"},
		{"/usr/bin/zsh", "zsh"},
		{"/usr/bin/fish", "fish"},
		{"/bin/sh", "sh"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			os.Setenv("SHELL", tt.shell)
			got := detectShell()
			if got != tt.want {
				t.Errorf("detectShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallResult(t *testing.T) {
	result := &InstallResult{
		Name:       "test",
		Version:    "v1.0.0",
		BinaryPath: "/usr/local/bin/test",
		AssetName:  "test-linux-amd64.tar.gz",
	}

	if result.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Name)
	}
	if result.Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", result.Version)
	}
}

func TestInstall_CreatesBinDir(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "newbin")
	installer := NewInstaller(binDir)

	// Ensure bin dir doesn't exist yet
	if _, err := os.Stat(binDir); !os.IsNotExist(err) {
		t.Fatal("bin dir should not exist")
	}

	// Create a minimal release and asset for testing
	release := &github.ReleaseInfo{
		TagName: "v1.0.0",
		Assets: []github.AssetInfo{
			{Name: "tool", DownloadURL: "http://example.com/tool"},
		},
	}

	// This will fail due to HTTP, but bin dir should be created
	installer.Install("tool", release, &release.Assets[0])

	// Verify bin dir was created
	if _, err := os.Stat(binDir); err != nil {
		t.Errorf("bin dir should have been created: %v", err)
	}
}
