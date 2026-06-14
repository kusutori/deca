package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
	restoreRuntime := mockRuntime("linux", "amd64")
	defer restoreRuntime()

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
	restoreRuntime := mockRuntime("linux", "amd64")
	defer restoreRuntime()

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
			origShell := os.Getenv("SHELL")
			os.Setenv("SHELL", "/bin/"+tt.shell)
			t.Cleanup(func() { os.Setenv("SHELL", origShell) })

			installer := NewInstaller(tt.binDir)
			instructions := installer.AddToPATHInstructions()

			if tt.wantHome {
				if instructions == "" {
					t.Error("expected non-empty instructions")
				}
			} else if !strings.Contains(instructions, tt.binDir) {
				t.Fatalf("expected fallback instructions to include bin dir, got %q", instructions)
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

func TestInstall_CreateBinDirFailure(t *testing.T) {
	tmpDir := t.TempDir()
	binFile := filepath.Join(tmpDir, "bin")
	if err := os.WriteFile(binFile, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(filepath.Join(binFile, "child"))
	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	asset := &github.AssetInfo{Name: "tool.tar.gz", DownloadURL: "http://example.com/tool.tar.gz"}
	if _, err := installer.Install("tool", release, asset); err == nil {
		t.Fatal("expected bin dir creation failure")
	}
}

func TestInstallRegularBinaryArchive(t *testing.T) {
	restoreRuntime := mockRuntime("linux", "amd64")
	defer restoreRuntime()

	tests := []struct {
		name        string
		files       map[string]tarTestFile
		wantContent string
	}{
		{
			name: "finds named executable",
			files: map[string]tarTestFile{
				"tool":      {content: "named executable", mode: 0755},
				"README.md": {content: "readme", mode: 0644},
			},
			wantContent: "named executable",
		},
		{
			name: "falls back to first regular file",
			files: map[string]tarTestFile{
				"README.md": {content: "regular file", mode: 0644},
			},
			wantContent: "regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := tarGzInstallBytes(t, tt.files)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(payload)
			}))
			defer server.Close()

			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			t.Cleanup(func() { os.Setenv("HOME", oldHome) })

			installer := NewInstaller(filepath.Join(tmpDir, "bin"))
			release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
			asset := &github.AssetInfo{Name: "tool-linux-amd64.tar.gz", DownloadURL: server.URL}

			result, err := installer.Install("tool", release, asset)
			if err != nil {
				t.Fatalf("Install failed: %v", err)
			}
			if result.InstallType != config.InstallTypeBinary || result.BinaryPath == "" || result.ExposedPath != result.BinaryPath {
				t.Fatalf("unexpected result: %+v", result)
			}
			data, err := os.ReadFile(result.BinaryPath)
			if err != nil {
				t.Fatalf("failed to read installed binary: %v", err)
			}
			if string(data) != tt.wantContent {
				t.Fatalf("content mismatch: got %q want %q", string(data), tt.wantContent)
			}
		})
	}
}

func TestInstallRegularSingleFile(t *testing.T) {
	restoreRuntime := mockRuntime("linux", "amd64")
	defer restoreRuntime()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("single binary"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	installer := NewInstaller(filepath.Join(tmpDir, "bin"))
	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	asset := &github.AssetInfo{Name: "tool", DownloadURL: server.URL}
	result, err := installer.Install("tool", release, asset)
	if err != nil {
		t.Fatalf("Install single file failed: %v", err)
	}
	data, err := os.ReadFile(result.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "single binary" {
		t.Fatalf("unexpected installed content: %q", string(data))
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

func TestWindowsInstallRootAndHelpers(t *testing.T) {
	oldLocalAppData := os.Getenv("LOCALAPPDATA")
	tmpDir := t.TempDir()
	os.Setenv("LOCALAPPDATA", tmpDir)
	t.Cleanup(func() { os.Setenv("LOCALAPPDATA", oldLocalAppData) })

	root := WindowsInstallRoot(`bad/name:tool`, `v1.0.0`)
	want := filepath.Join(tmpDir, "deca", "packages", "bad_name_tool", "v1.0.0")
	if root != want {
		t.Fatalf("unexpected install root: got %s want %s", root, want)
	}

	files := []string{"bin/tool.exe", "README.md"}
	srcRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "bin", "tool.exe"), []byte("exe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}
	dstRoot := t.TempDir()
	if err := copyExtractedFiles(srcRoot, dstRoot, files); err != nil {
		t.Fatalf("copyExtractedFiles failed: %v", err)
	}
	if got := firstWindowsExecutable(files); got != "bin/tool.exe" {
		t.Fatalf("expected first exe, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "bin", "tool.exe")); err != nil {
		t.Fatalf("copied exe missing: %v", err)
	}
	if err := copyExtractedFiles(srcRoot, dstRoot, []string{"../escape.exe"}); err == nil {
		t.Fatal("expected unsafe archive path error")
	}
}

func TestUninstallPortableAndAppImage(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(binDir)

	appImagePath := filepath.Join(binDir, "app")
	if err := os.WriteFile(appImagePath, []byte("appimage"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installer.Uninstall("app", config.InstallTypeAppImage, ""); err != nil {
		t.Fatalf("uninstall appimage failed: %v", err)
	}
	if _, err := os.Stat(appImagePath); !os.IsNotExist(err) {
		t.Fatal("expected appimage removed")
	}

	installRoot := filepath.Join(tmpDir, "packages", "tool", "v1")
	exposedPath := filepath.Join(binDir, "tool.exe")
	if err := os.MkdirAll(installRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exposedPath, []byte("exe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installer.uninstallWindowsPortable("tool", UninstallMetadata{
		InstallRoot: installRoot,
		ExposedPath: exposedPath,
	}); err != nil {
		t.Fatalf("uninstall windows portable failed: %v", err)
	}
	if _, err := os.Stat(exposedPath); !os.IsNotExist(err) {
		t.Fatal("expected exposed exe removed")
	}
	if _, err := os.Stat(installRoot); !os.IsNotExist(err) {
		t.Fatal("expected install root removed")
	}
}

func TestVersionedSymlinkAndRollbackHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(binDir, "tool")
	if err := os.WriteFile(current, []byte("v1"), 0755); err != nil {
		t.Fatal(err)
	}
	versioned, err := CreateVersionedSymlink(binDir, "tool", "v1.0.0", current)
	if err != nil {
		t.Fatalf("CreateVersionedSymlink failed: %v", err)
	}
	if _, err := os.Stat(versioned); err != nil {
		t.Fatalf("versioned binary missing: %v", err)
	}
	if err := UninstallVersioned(current, versioned); err != nil {
		t.Fatalf("UninstallVersioned failed: %v", err)
	}
	if _, err := os.Stat(versioned); !os.IsNotExist(err) {
		t.Fatal("expected versioned binary removed")
	}

	backup := filepath.Join(tmpDir, "missing.bak")
	if err := RestoreFile("", current); err != nil {
		t.Fatalf("empty restore should be no-op: %v", err)
	}
	if err := RemoveBackup(backup); err != nil {
		t.Fatalf("missing backup removal should be no-op: %v", err)
	}
	if err := os.WriteFile(backup, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveBackup(backup); err != nil {
		t.Fatalf("RemoveBackup failed: %v", err)
	}

	if _, err := CreateVersionedSymlink(binDir, "missing", "v1.0.0", filepath.Join(binDir, "missing")); err == nil {
		t.Fatal("expected missing current binary error")
	}

	badVersioned := filepath.Join(tmpDir, "dir-versioned")
	if err := os.MkdirAll(filepath.Join(badVersioned, "child"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := UninstallVersioned("", badVersioned); err == nil {
		t.Fatal("expected remove non-empty directory error")
	}
}

func TestDetectPackageTypeAllKnownTypes(t *testing.T) {
	tests := map[string]string{
		"tool.deb": "deb",
		"tool.rpm": "rpm",
		"tool.msi": "msi",
		"tool.apk": "apk",
		"tool.dmg": "dmg",
		"tool.zip": "",
	}
	for name, want := range tests {
		if got := DetectPackageType(name); got != want {
			t.Fatalf("DetectPackageType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestInstallWindowsPortableSingleExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows portable install path is only active on Windows")
	}

	tmpDir := t.TempDir()
	oldLocalAppData := os.Getenv("LOCALAPPDATA")
	oldHome := os.Getenv("HOME")
	os.Setenv("LOCALAPPDATA", tmpDir)
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("LOCALAPPDATA", oldLocalAppData)
		os.Setenv("HOME", oldHome)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("exe payload"))
	}))
	defer server.Close()

	installer := NewInstaller(filepath.Join(tmpDir, "bin"))
	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	asset := &github.AssetInfo{Name: "tool.exe", DownloadURL: server.URL}

	result, err := installer.Install("tool", release, asset)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if result.InstallRoot == "" || result.ExposedPath == "" || result.VersionedBinaryPath == "" {
		t.Fatalf("expected windows install metadata: %+v", result)
	}
	if _, err := os.Stat(result.ExposedPath); err != nil {
		t.Fatalf("expected exposed executable: %v", err)
	}
	if _, err := os.Stat(result.VersionedBinaryPath); err != nil {
		t.Fatalf("expected versioned executable: %v", err)
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

			restore := mockInstallExec(t)
			// Force dpkg-deb failure so extractDebPackageName exercises filename fallback.
			execCommandFunc = func(string, ...string) *exec.Cmd {
				return fakeInstallCommand("fail")
			}
			got := extractDebPackageName(debPath)
			restore()
			if got != tc.expectedName {
				t.Errorf("expected %s, got %s", tc.expectedName, got)
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

func TestInstallAppImageCopyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	binFile := filepath.Join(tmpDir, "bin-file")
	if err := os.WriteFile(binFile, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(binFile)

	orig := downloadFileFunc
	t.Cleanup(func() { downloadFileFunc = orig })
	downloadFileFunc = func(url, path string) error {
		return os.WriteFile(path, []byte("appimage"), 0755)
	}

	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "o", Repo: "r"}
	asset := &github.AssetInfo{Name: "tool.AppImage", DownloadURL: "http://example.com/tool.AppImage"}
	if _, err := installer.installAppImage("tool", release, asset, binFile); err == nil {
		t.Fatal("expected AppImage copy failure")
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

func TestInstallSystemPackageSuccessAndUnsupported(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	restore := mockInstallExec(t)
	defer restore()

	origDownload := downloadFileFunc
	downloadFileFunc = func(url, path string) error {
		return os.WriteFile(path, []byte("package"), 0644)
	}
	t.Cleanup(func() { downloadFileFunc = origDownload })

	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	deb := &github.AssetInfo{Name: "tool_1.0.0_amd64.deb", DownloadURL: "http://example.com/tool.deb"}
	result, err := installer.installSystemPackage("tool", release, deb, "deb", tmpDir)
	if err != nil {
		t.Fatalf("installSystemPackage deb failed: %v", err)
	}
	if result.InstallType != config.InstallTypeSystem || result.SystemPkgName != "pkg-from-dpkg" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, deb.Name)); !os.IsNotExist(err) {
		t.Fatalf("expected package file cleaned up, got %v", err)
	}

	rpm := &github.AssetInfo{Name: "tool.rpm", DownloadURL: "http://example.com/tool.rpm"}
	result, err = installer.installSystemPackage("tool", release, rpm, "rpm", tmpDir)
	if err != nil {
		t.Fatalf("installSystemPackage rpm failed: %v", err)
	}
	if result.SystemPkgName != "tool" {
		t.Fatalf("expected rpm package name fallback, got %+v", result)
	}

	if _, err := installer.installSystemPackage("tool", release, deb, "apk", tmpDir); err == nil {
		t.Fatal("expected unsupported package type error")
	}
}

func TestInstallSystemPackageNonRootAndYumFallback(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	restore := mockInstallExec(t)
	defer restore()

	getuidFunc = func() int { return 1000 }
	execLookPathFunc = func(name string) (string, error) {
		switch name {
		case "dnf":
			return "", os.ErrNotExist
		case "yum", "sudo":
			return name, nil
		default:
			return "", os.ErrNotExist
		}
	}
	origDownload := downloadFileFunc
	downloadFileFunc = func(url, path string) error {
		return os.WriteFile(path, []byte("package"), 0644)
	}
	t.Cleanup(func() { downloadFileFunc = origDownload })

	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	asset := &github.AssetInfo{Name: "tool.rpm", DownloadURL: "http://example.com/tool.rpm"}
	if _, err := installer.installSystemPackage("tool", release, asset, "rpm", tmpDir); err != nil {
		t.Fatalf("non-root rpm install failed: %v", err)
	}
}

func TestUninstallSystemPackageBranches(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	restore := mockInstallExec(t)
	defer restore()

	if err := installer.uninstallSystemPackage("tool", "system-tool"); err != nil {
		t.Fatalf("uninstall system package failed: %v", err)
	}

	getuidFunc = func() int { return 1000 }
	if err := installer.uninstallSystemPackage("tool", "system-tool"); err != nil {
		t.Fatalf("non-root uninstall system package failed: %v", err)
	}
	getuidFunc = func() int { return 0 }

	origLookPath := execLookPathFunc
	execLookPathFunc = func(string) (string, error) { return "", os.ErrNotExist }
	if err := installer.uninstallSystemPackage("tool", ""); err == nil {
		t.Fatal("expected no package manager error")
	}
	execLookPathFunc = origLookPath
}

func TestUninstallDispatchForWindowsTypes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows uninstall dispatch is only active on Windows")
	}
	installer := NewInstaller(t.TempDir())
	restore := mockInstallExec(t)
	defer restore()

	if err := installer.Uninstall("tool", config.InstallTypeWindowsMSI, "", UninstallMetadata{ProductCode: "{PRODUCT-CODE}"}); err != nil {
		t.Fatalf("windows MSI uninstall dispatch failed: %v", err)
	}
	if err := installer.Uninstall("tool", config.InstallTypeWindowsInstaller, ""); err != nil {
		t.Fatalf("windows installer uninstall dispatch failed: %v", err)
	}
}

func TestSudoRunRootAndCached(t *testing.T) {
	restore := mockInstallExec(t)
	defer restore()

	if err := SudoRun("echo", "ok"); err != nil {
		t.Fatalf("root SudoRun failed: %v", err)
	}

	getuidFunc = func() int { return 1000 }
	if err := SudoRun("echo", "ok"); err != nil {
		t.Fatalf("cached SudoRun failed: %v", err)
	}

	execLookPathFunc = func(string) (string, error) { return "", os.ErrNotExist }
	if err := SudoRun("echo", "ok"); err == nil {
		t.Fatal("expected sudo missing error")
	}
}

func TestSudoRunPasswordPromptBranch(t *testing.T) {
	restore := mockInstallExec(t)
	defer restore()

	getuidFunc = func() int { return 1000 }
	execCommandFunc = func(command string, args ...string) *exec.Cmd {
		if command == "sudo" && len(args) >= 2 && args[0] == "-n" && args[1] == "true" {
			return fakeInstallCommand("fail")
		}
		return fakeInstallCommand(command, args...)
	}
	origReadPassword := termReadPasswordFunc
	termReadPasswordFunc = func(int) ([]byte, error) { return []byte("password"), nil }
	defer func() { termReadPasswordFunc = origReadPassword }()

	if err := SudoRun("echo", "ok"); err != nil {
		t.Fatalf("password prompt SudoRun failed: %v", err)
	}
}

func TestInstallWindowsMSIAndInstaller(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows installer paths are only active on Windows")
	}
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldLocalAppData := os.Getenv("LOCALAPPDATA")
	os.Setenv("HOME", tmpDir)
	os.Setenv("LOCALAPPDATA", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("LOCALAPPDATA", oldLocalAppData)
	})
	restore := mockInstallExec(t)
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("installer"))
	}))
	defer server.Close()

	installer := NewInstaller(filepath.Join(tmpDir, "bin"))
	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}
	msiResult, err := installer.Install("tool", release, &github.AssetInfo{Name: "tool.msi", DownloadURL: server.URL})
	if err != nil {
		t.Fatalf("MSI install failed: %v", err)
	}
	if msiResult.InstallType != config.InstallTypeWindowsMSI || msiResult.ProductCode == "" {
		t.Fatalf("unexpected MSI result: %+v", msiResult)
	}

	installerResult, err := installer.Install("tool", release, &github.AssetInfo{Name: "setup.exe", DownloadURL: server.URL}, "installer")
	if err != nil {
		t.Fatalf("interactive installer failed: %v", err)
	}
	if installerResult.InstallType != config.InstallTypeWindowsInstaller {
		t.Fatalf("unexpected installer result: %+v", installerResult)
	}
	if err := installer.uninstallWindowsMSI("tool", "{PRODUCT-CODE}"); err != nil {
		t.Fatalf("MSI uninstall failed: %v", err)
	}
	if err := installer.uninstallWindowsInstaller("tool"); err != nil {
		t.Fatalf("installer uninstall state cleanup failed: %v", err)
	}
	if err := installer.uninstallWindowsMSI("tool", ""); err == nil {
		t.Fatal("expected missing product code error")
	}
}

func TestWindowsInstallerErrorBranches(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows installer paths are only active on Windows")
	}
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldLocalAppData := os.Getenv("LOCALAPPDATA")
	os.Setenv("HOME", tmpDir)
	os.Setenv("LOCALAPPDATA", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("LOCALAPPDATA", oldLocalAppData)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	installer := NewInstaller(filepath.Join(tmpDir, "bin"))
	release := &github.ReleaseInfo{TagName: "v1.0.0", Owner: "owner", Repo: "repo"}

	restore := mockInstallExec(t)
	execCommandFunc = func(command string, args ...string) *exec.Cmd {
		if command == "powershell.exe" {
			return fakeInstallCommand("fail")
		}
		return fakeInstallCommand(command, args...)
	}
	if _, err := installer.Install("tool", release, &github.AssetInfo{Name: "tool.msi", DownloadURL: server.URL}); err == nil {
		t.Fatal("expected MSI product code read failure")
	}
	restore()

	restore = mockInstallExec(t)
	execCommandFunc = func(command string, args ...string) *exec.Cmd {
		if strings.HasSuffix(strings.ToLower(command), ".exe") {
			return fakeInstallCommand("fail")
		}
		return fakeInstallCommand(command, args...)
	}
	if _, err := installer.Install("tool", release, &github.AssetInfo{Name: "setup.exe", DownloadURL: server.URL}, "installer"); err == nil {
		t.Fatal("expected interactive installer failure")
	}
	restore()

	if _, err := installer.Install("tool", release, &github.AssetInfo{Name: "readme.txt", DownloadURL: server.URL}); err == nil {
		t.Fatal("expected no executable found error")
	}
}

func TestCopyFileErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()
	if err := copyFile(filepath.Join(tmpDir, "missing"), filepath.Join(tmpDir, "dst")); err == nil {
		t.Fatal("expected missing source error")
	}

	src := filepath.Join(tmpDir, "src")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, tmpDir); err == nil {
		t.Fatal("expected create destination error")
	}
}

func TestWindowsHelperEdgeBranches(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows helper branch coverage is only meaningful on Windows")
	}
	oldLocalAppData := os.Getenv("LOCALAPPDATA")
	os.Unsetenv("LOCALAPPDATA")
	t.Cleanup(func() { os.Setenv("LOCALAPPDATA", oldLocalAppData) })
	if root := WindowsInstallRoot("", ""); !strings.Contains(root, filepath.Join("deca", "packages", "_", "_")) {
		t.Fatalf("unexpected fallback root: %s", root)
	}
	if got := firstWindowsExecutable([]string{"README.md"}); got != "" {
		t.Fatalf("expected no executable, got %q", got)
	}

	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := copyExtractedFiles(srcRoot, dstRoot, []string{"docs"}); err != nil {
		t.Fatalf("copy directory entry failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "docs")); err != nil {
		t.Fatalf("expected copied directory: %v", err)
	}

	installer := NewInstaller(t.TempDir())
	if err := installer.uninstallWindowsPortable("missing", UninstallMetadata{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not exist for missing portable uninstall, got %v", err)
	}

	if _, err := exposeWindowsExecutable(filepath.Join(t.TempDir(), "missing.exe"), filepath.Join(t.TempDir(), "bin", "tool.exe")); err == nil {
		t.Fatal("expected expose missing target error")
	}
}

func TestUninstallBinaryWindowsNoExtensionFallback(t *testing.T) {
	restoreRuntime := mockRuntime("windows", "amd64")
	defer restoreRuntime()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	pathNoExt := filepath.Join(tmpDir, "tool")
	if err := os.WriteFile(pathNoExt, []byte("exe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installer.uninstallBinary("tool"); err != nil {
		t.Fatalf("expected no-extension fallback removal: %v", err)
	}
	if _, err := os.Stat(pathNoExt); !os.IsNotExist(err) {
		t.Fatalf("expected no-extension binary removed, got %v", err)
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

func mockInstallExec(t *testing.T) func() {
	t.Helper()
	origCommand := execCommandFunc
	origLookPath := execLookPathFunc
	origGetuid := getuidFunc
	execCommandFunc = fakeInstallCommand
	execLookPathFunc = func(name string) (string, error) {
		switch name {
		case "dnf", "yum", "apt", "sudo":
			return name, nil
		default:
			return "", os.ErrNotExist
		}
	}
	getuidFunc = func() int { return 0 }
	return func() {
		execCommandFunc = origCommand
		execLookPathFunc = origLookPath
		getuidFunc = origGetuid
	}
}

func fakeInstallCommand(command string, args ...string) *exec.Cmd {
	allArgs := append([]string{"-test.run=TestInstallHelperProcess", "--", command}, args...)
	cmd := exec.Command(os.Args[0], allArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_INSTALL_HELPER_PROCESS=1")
	return cmd
}

func TestInstallHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_INSTALL_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	sep := 0
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == 0 || sep+1 >= len(args) {
		os.Exit(2)
	}
	command := args[sep+1]
	switch command {
	case "fail":
		os.Exit(1)
	case "dpkg-deb":
		_, _ = os.Stdout.WriteString("pkg-from-dpkg\n")
	case "sudo", "apt", "dnf", "yum", "msiexec", "echo":
	case "powershell.exe":
		_, _ = os.Stdout.WriteString("{PRODUCT-CODE}\n")
	default:
		// Treat downloaded interactive installer paths as successful commands.
	}
	os.Exit(0)
}

func mockRuntime(goos, goarch string) func() {
	origGOOS := runtimeGOOS
	origGOARCH := runtimeGOARCH
	runtimeGOOS = goos
	runtimeGOARCH = goarch
	return func() {
		runtimeGOOS = origGOOS
		runtimeGOARCH = origGOARCH
	}
}

type tarTestFile struct {
	content string
	mode    int64
}

func tarGzInstallBytes(t *testing.T, files map[string]tarTestFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, file := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: file.mode,
			Size: int64(len(file.content)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(file.content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}
