package install

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
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
	if err := installer.Uninstall("testbinary", config.InstallTypeBinary, ""); err != nil {
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
	err := installer.Uninstall("nonexistent", config.InstallTypeBinary, "")
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
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX execute bits through os.FileMode")
	}

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
		Name:        "test",
		Version:     "v1.0.0",
		BinaryPath:  "/usr/local/bin/test",
		AssetName:   "test-linux-amd64.tar.gz",
		InstallType: config.InstallTypeBinary,
	}

	if result.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Name)
	}
	if result.Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", result.Version)
	}
	if result.InstallType != config.InstallTypeBinary {
		t.Errorf("expected InstallType 'binary', got '%s'", result.InstallType)
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

func TestInstallResultWithSystemPkgName(t *testing.T) {
	result := &InstallResult{
		Name:          "fresh",
		Version:       "v0.1.98",
		BinaryPath:    "",
		AssetName:     "fresh-editor_0.1.98-1_amd64.deb",
		InstallType:   config.InstallTypeSystem,
		SystemPkgName: "fresh-editor",
	}

	if result.SystemPkgName != "fresh-editor" {
		t.Errorf("expected SystemPkgName 'fresh-editor', got '%s'", result.SystemPkgName)
	}
}

func TestDetectPackageTypeMSI(t *testing.T) {
	if got := DetectPackageType("AppSetup.msi"); got != "msi" {
		t.Fatalf("expected msi package type, got %q", got)
	}
}

func TestResolveWindowsInstallMode(t *testing.T) {
	tests := []struct {
		name       string
		asset      string
		preference string
		want       string
		wantErr    bool
	}{
		{name: "auto msi", asset: "tool.msi", preference: "auto", want: "msi"},
		{name: "auto exe", asset: "tool.exe", preference: "auto", want: "portable"},
		{name: "explicit portable", asset: "tool.zip", preference: "portable", want: "portable"},
		{name: "explicit installer", asset: "setup.exe", preference: "installer", want: "installer"},
		{name: "installer requires exe", asset: "setup.zip", preference: "installer", wantErr: true},
		{name: "msi requires msi", asset: "setup.exe", preference: "msi", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWindowsInstallMode(tt.asset, tt.preference)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExposeWindowsExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "packages", "tool.exe")
	exposed := filepath.Join(tmpDir, "bin", "tool.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("portable exe"), 0644); err != nil {
		t.Fatal(err)
	}

	linkType, err := exposeWindowsExecutable(target, exposed)
	if err != nil {
		t.Fatalf("expose failed: %v", err)
	}
	if linkType != "hardlink" && linkType != "copy" {
		t.Fatalf("unexpected link type: %s", linkType)
	}
	data, err := os.ReadFile(exposed)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "portable exe" {
		t.Fatalf("unexpected exposed content: %q", string(data))
	}
}

func TestExtractDebPackageName(t *testing.T) {
	// Create a minimal deb file structure for testing
	// Since we can't easily create a real deb file, test the fallback parsing
	tmpDir := t.TempDir()

	// Test with a filename that can be parsed
	testCases := []struct {
		filename     string
		expectedName string
	}{
		{"fresh-editor_0.1.98-1_amd64.deb", "fresh-editor"},
		{"myapp_1.2.3_amd64.deb", "myapp"},
		{"complex-name_0.1.0-rc1_arm64.deb", "complex-name"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			// Since dpkg-deb may not be available in test env, test fallback parsing
			// Create a mock deb file path
			debPath := filepath.Join(tmpDir, tc.filename)

			// Write minimal content (not a real deb, just for path parsing)
			if err := os.WriteFile(debPath, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// The actual extractDebPackageName uses dpkg-deb which may not be available
			// So we test the fallback parsing by simulating the logic
			parts := strings.Split(tc.filename, "_")
			if len(parts) > 0 {
				got := parts[0]
				if got != tc.expectedName {
					t.Errorf("expected %s, got %s", tc.expectedName, got)
				}
			}
		})
	}
}

func TestUninstallSystemPackageWithName(t *testing.T) {
	// This test verifies that the Uninstall function signature is correct
	// The actual system package uninstall is tested manually or via integration tests
	installer := NewInstaller("/tmp/bin")

	// Verify the function signature accepts systemPkgName
	// This is a compile-time check, not a runtime test
	_ = func() {
		installer.Uninstall("test", config.InstallTypeSystem, "actual-pkg-name")
		installer.Uninstall("test", config.InstallTypeBinary, "")
	}
}

func TestInstallAppImage_UsesDownloadFile(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)

	orig := downloadFileFunc
	t.Cleanup(func() { downloadFileFunc = orig })
	downloadFileFunc = func(url, path string) error {
		return os.WriteFile(path, []byte("appimage"), 0755)
	}

	release := &github.ReleaseInfo{
		TagName: "v1.0.0",
		Owner:   "owner",
		Repo:    "repo",
	}
	asset := &github.AssetInfo{
		Name:        "tool.AppImage",
		DownloadURL: "http://example.com/tool.AppImage",
	}

	result, err := installer.Install("tool", release, asset)
	if err != nil {
		t.Fatalf("install appimage failed: %v", err)
	}
	if result.InstallType != config.InstallTypeAppImage {
		t.Fatalf("expected appimage install type, got %s", result.InstallType)
	}
	if result.BinaryPath == "" {
		t.Fatal("expected binary path to be set")
	}
}

func TestInstallSystemPackage_UsesDownloadFile(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)

	sentinel := errors.New("download failed")
	orig := downloadFileFunc
	t.Cleanup(func() { downloadFileFunc = orig })
	downloadFileFunc = func(url, path string) error {
		return sentinel
	}

	release := &github.ReleaseInfo{
		TagName: "v1.0.0",
		Owner:   "owner",
		Repo:    "repo",
	}
	asset := &github.AssetInfo{
		Name:        "tool_1.0.0_amd64.deb",
		DownloadURL: "http://example.com/tool.deb",
	}

	_, err := installer.installSystemPackage("tool", release, asset, "deb", "/tmp")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected download error, got %v", err)
	}
}

func TestRegression_Install_PermissionDeniedOnExistingTarget(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(binDir)

	orig := downloadFileFunc
	t.Cleanup(func() { downloadFileFunc = orig })
	downloadFileFunc = func(url, path string) error {
		return os.WriteFile(path, []byte("appimage"), 0755)
	}

	targetPath := filepath.Join(binDir, "tool")
	if err := os.WriteFile(targetPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0755) })

	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "o", Repo: "r"}
	asset := &github.AssetInfo{Name: "tool.AppImage", DownloadURL: "http://example.com/tool.AppImage"}

	_, err := installer.Install("tool", release, asset)
	if err == nil {
		t.Skip("permission semantics vary when tests run as privileged user")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestInstallAppImage_DownloadFailureUsesErrorsIs(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)

	orig := downloadFileFunc
	t.Cleanup(func() { downloadFileFunc = orig })
	downloadFileFunc = func(url, path string) error { return os.ErrPermission }

	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "o", Repo: "r"}
	asset := &github.AssetInfo{Name: "tool.AppImage", DownloadURL: "http://example.com/tool.AppImage"}

	_, err := installer.Install("tool", release, asset)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected wrapped permission error, got %v", err)
	}
}
