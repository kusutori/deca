package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kusutori/deca/internal/github"
	"github.com/ulikunitz/xz"
)

func TestFindBinary(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		search  string
		want    string
		tempDir string
	}{
		{"exact match", []string{"eza", "README.md"}, "eza", "eza", ""},
		{"with exe", []string{"app.exe"}, "app", "app.exe", ""},
		{"contains", []string{"eza-x86_64-unknown-linux-musl"}, "eza", "eza-x86_64-unknown-linux-musl", ""},
		{"case insensitive", []string{"EZA"}, "eza", "EZA", ""},
		{"not found", []string{"other"}, "eza", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindBinary(tt.files, tt.search, tt.tempDir)
			if got != tt.want {
				t.Errorf("FindBinary(%v, %q, %q) = %q, want %q", tt.files, tt.search, tt.tempDir, got, tt.want)
			}
		})
	}
}

func TestFileSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FileSize(tt.size)
			if got != tt.want {
				t.Errorf("FileSize(%d) = %q, want %q", tt.size, got, tt.want)
			}
		})
	}
}

func TestDownloadFileWithCacheFailurePaths(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "asset.bin")
	originalGet, originalCreate, originalRename := httpGetFunc, downloadCreate, downloadRename
	t.Cleanup(func() {
		httpGetFunc = originalGet
		downloadCreate = originalCreate
		downloadRename = originalRename
	})

	httpGetFunc = func(string) (*http.Response, error) { return nil, errors.New("network unavailable") }
	if err := downloadFileWithCache("unused", path, "owner/fail", "v1", "asset.bin", ""); err == nil {
		t.Fatal("expected network error")
	}

	httpGetFunc = func(string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad gateway"))}, nil
	}
	if err := downloadFileWithCache("unused", path, "owner/fail", "v1", "asset.bin", ""); err == nil {
		t.Fatal("expected HTTP error")
	}

	httpGetFunc = func(string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("payload"))}, nil
	}
	downloadCreate = func(string) (*os.File, error) { return nil, errors.New("cannot create") }
	if err := downloadFileWithCache("unused", path, "owner/fail", "v1", "asset.bin", ""); err == nil {
		t.Fatal("expected create error")
	}

	downloadCreate = os.Create
	downloadRename = func(string, string) error { return errors.New("cannot rename") }
	if err := downloadFileWithCache("unused", path, "owner/fail", "v1", "asset.bin", ""); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestExtractTarGz(t *testing.T) {
	// Create a test tar.gz file
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	extractDir := filepath.Join(tmpDir, "extracted")

	// Create test files
	files := map[string]string{
		"bin/tool":  "#!/bin/bash\necho hello",
		"bin/other": "other content",
		"README":    "readme content",
	}

	// Create tar.gz
	createTarGz(t, archivePath, files)

	// Extract
	filesList, err := extractTarGz(archivePath, extractDir)
	if err != nil {
		t.Fatalf("failed to extract tar.gz: %v", err)
	}

	// Verify files were extracted
	if len(filesList) != 3 {
		t.Errorf("expected 3 files, got %d", len(filesList))
	}

	for name, content := range files {
		path := filepath.Join(extractDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if string(data) != content {
			t.Errorf("%s content mismatch: got %q, want %q", name, string(data), content)
		}
	}
}

func TestExtractZip(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")
	extractDir := filepath.Join(tmpDir, "extracted")

	files := map[string]string{
		"bin/tool.exe": "echo hello",
		"README":       "readme content",
	}
	createZipArchive(t, archivePath, files)

	filesList, err := extractZip(archivePath, extractDir)
	if err != nil {
		t.Fatalf("failed to extract zip: %v", err)
	}
	if len(filesList) != 2 {
		t.Fatalf("expected 2 files, got %d", len(filesList))
	}
	for name, content := range files {
		data, err := os.ReadFile(filepath.Join(extractDir, name))
		if err != nil {
			t.Fatalf("failed to read extracted file %s: %v", name, err)
		}
		if string(data) != content {
			t.Fatalf("content mismatch for %s: got %q want %q", name, string(data), content)
		}
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "malicious.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZip(archivePath, filepath.Join(tmpDir, "dest")); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestExtractTarHandlesDirectoriesAndSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "entries.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gz)
	entries := []struct {
		header tar.Header
		data   string
	}{
		{header: tar.Header{Name: "bin/", Mode: 0755, Typeflag: tar.TypeDir}},
		{header: tar.Header{Name: "bin/tool", Mode: 0755, Typeflag: tar.TypeReg, Size: int64(len("binary"))}, data: "binary"},
	}
	if runtime.GOOS != "windows" {
		entries = append(entries, struct {
			header tar.Header
			data   string
		}{header: tar.Header{Name: "tool-link", Mode: 0755, Typeflag: tar.TypeSymlink, Linkname: "bin/tool"}})
	}
	for _, entry := range entries {
		if err := tarWriter.WriteHeader(&entry.header); err != nil {
			t.Fatal(err)
		}
		if entry.data != "" {
			if _, err := tarWriter.Write([]byte(entry.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpDir, "dest")
	files, err := extractTarGz(archivePath, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(entries) {
		t.Fatalf("extracted entries = %v", files)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Lstat(filepath.Join(dest, "tool-link")); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink: %v, %v", info, err)
		}
	}
}

func TestExtractTarXz(t *testing.T) {
	// Create a test tar.xz file
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.xz")
	extractDir := filepath.Join(tmpDir, "extracted")

	// Create test files
	files := map[string]string{
		"bin/tool":  "#!/bin/bash\necho hello",
		"bin/other": "other content",
		"README":    "readme content",
	}

	// Create tar.xz
	createTarXz(t, archivePath, files)

	// Extract
	filesList, err := extractTarXz(archivePath, extractDir)
	if err != nil {
		t.Fatalf("failed to extract tar.xz: %v", err)
	}

	// Verify files were extracted
	if len(filesList) != 3 {
		t.Errorf("expected 3 files, got %d", len(filesList))
	}

	for name, content := range files {
		path := filepath.Join(extractDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if string(data) != content {
			t.Errorf("%s content mismatch: got %q, want %q", name, string(data), content)
		}
	}
}

func TestExtractTarXzSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.xz")
	extractDir := filepath.Join(tmpDir, "extracted")

	// Create a malicious tar file with path traversal
	createMaliciousTarXz(t, archivePath, "../outside.txt")

	// Extract should fail
	_, err := extractTarXz(archivePath, extractDir)
	if err == nil {
		t.Error("expected error for path traversal attack")
	}
}

func TestExtractTarGzSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	extractDir := filepath.Join(tmpDir, "extracted")

	// Create a malicious tar file with path traversal
	createMaliciousTarGz(t, archivePath, "../outside.txt")

	// Extract should fail
	_, err := extractTarGz(archivePath, extractDir)
	if err == nil {
		t.Error("expected error for path traversal attack")
	}
}

func TestDownloadAndExtractWithCacheTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	archivePath := filepath.Join(tmpDir, "tool.tar.gz")
	createTarGz(t, archivePath, map[string]string{"tool": "binary"})
	payload, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	result, err := DownloadAndExtractWithCache(&github.AssetInfo{
		Name:        "tool.tar.gz",
		DownloadURL: server.URL,
	}, runtimeGOOS(), runtimeGOARCH(), "owner/repo", "v1.0.0")
	if err != nil {
		t.Fatalf("DownloadAndExtractWithCache failed: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	if !result.IsBinary || len(result.Files) != 1 || result.Files[0] != "tool" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(result.TempDir, "tool")); err != nil {
		t.Fatalf("expected extracted tool: %v", err)
	}
	// A second download must use the cached archive rather than the server.
	server.Close()
	cachedResult, err := DownloadAndExtractWithCache(&github.AssetInfo{
		Name:        "tool.tar.gz",
		DownloadURL: server.URL,
	}, runtimeGOOS(), runtimeGOARCH(), "owner/repo", "v1.0.0")
	if err != nil {
		t.Fatalf("cached DownloadAndExtractWithCache failed: %v", err)
	}
	defer os.RemoveAll(cachedResult.TempDir)
	if len(cachedResult.Files) != 1 || cachedResult.Files[0] != "tool" {
		t.Fatalf("unexpected cached result: %+v", cachedResult)
	}
}

func TestDownloadAndExtractWithCacheZipAndSingleExe(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "tool.zip")
	createZipArchive(t, zipPath, map[string]string{"tool.exe": "binary"})
	zipPayload, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zip":
			_, _ = w.Write(zipPayload)
		case "/exe":
			_, _ = w.Write([]byte("exe payload"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	zipResult, err := DownloadAndExtractWithCache(&github.AssetInfo{Name: "tool.zip", DownloadURL: server.URL + "/zip"}, "windows", "amd64", "", "")
	if err != nil {
		t.Fatalf("zip download failed: %v", err)
	}
	defer os.RemoveAll(zipResult.TempDir)
	if len(zipResult.Files) != 1 || zipResult.Files[0] != "tool.exe" {
		t.Fatalf("unexpected zip files: %+v", zipResult.Files)
	}

	exeResult, err := DownloadAndExtractWithCache(&github.AssetInfo{Name: "tool.exe", DownloadURL: server.URL + "/exe"}, "windows", "amd64", "", "")
	if err != nil {
		t.Fatalf("exe download failed: %v", err)
	}
	defer os.RemoveAll(exeResult.TempDir)
	if len(exeResult.Files) != 1 || exeResult.Files[0] != "tool.exe" {
		t.Fatalf("unexpected exe files: %+v", exeResult.Files)
	}
}

func TestCopyFileAndDownloadAndExtractNoCache(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.bin")
	dst := filepath.Join(tmpDir, "dst.bin")
	if err := os.WriteFile(src, []byte("copy payload"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "copy payload" {
		t.Fatalf("unexpected copied content: %q", data)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0750 {
		t.Fatalf("expected copied mode 0750, got %v", info.Mode().Perm())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain binary"))
	}))
	defer server.Close()

	result, err := DownloadAndExtract(&github.AssetInfo{Name: "tool.bin", DownloadURL: server.URL}, "linux", "amd64")
	if err != nil {
		t.Fatalf("DownloadAndExtract failed: %v", err)
	}
	defer os.RemoveAll(result.TempDir)
	if !result.IsBinary || len(result.Files) != 1 || result.Files[0] != "tool.bin" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestVerifyChecksumFailures(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "payload.bin")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	SetAssetDigest("sha256:" + strings.Repeat("0", 64))
	defer ClearAssetDigest()
	if err := verifyChecksumIfAvailable(path, "payload.bin"); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestParseChecksumVariantsAndLocalVerification(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	payload := filepath.Join(tmpDir, "payload.bin")
	if err := os.WriteFile(payload, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	sha256Sum, err := ComputeSHA256(payload)
	if err != nil {
		t.Fatal(err)
	}
	sha512Sum, err := ComputeSHA512(payload)
	if err != nil {
		t.Fatal(err)
	}

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	content := strings.Join([]string{
		"# comment",
		strings.Repeat("a", 32) + "  other.bin",
		sha512Sum + "  *nested/payload.bin",
		sha256Sum,
	}, "\n")
	if err := os.WriteFile(checksumPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := ParseChecksumFile(checksumPath, "payload.bin")
	if err != nil {
		t.Fatalf("ParseChecksumFile failed: %v", err)
	}
	if info.Type != ChecksumTypeSHA512 || info.Expected != sha512Sum {
		t.Fatalf("unexpected checksum info: %+v", info)
	}

	if err := verifyChecksumIfAvailable(payload, "payload.bin"); err != nil {
		t.Fatalf("local checksum verification failed: %v", err)
	}

	missingChecksumPath := filepath.Join(tmpDir, "missing-checksums.txt")
	if err := os.WriteFile(missingChecksumPath, []byte("not-a-hash other.bin\nalso-not-a-hash missing.bin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseChecksumFile(missingChecksumPath, "absent.bin"); err == nil {
		t.Fatal("expected missing checksum error")
	}
	if _, err := ParseChecksumFile(filepath.Join(tmpDir, "missing.txt"), "payload.bin"); err == nil {
		t.Fatal("expected missing checksum file error")
	}
	if err := VerifyChecksum(payload, sha256Sum, ChecksumTypeMD5); err == nil {
		t.Fatal("expected unsupported checksum error")
	}
}

func TestFindBinaryExecutableFallback(t *testing.T) {
	tmpDir := t.TempDir()
	toolPath := filepath.Join(tmpDir, "bin", "fallback")
	if err := os.MkdirAll(filepath.Dir(toolPath), 0755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		mode = 0644
	}
	if err := os.WriteFile(toolPath, []byte("binary"), mode); err != nil {
		t.Fatal(err)
	}
	files := []string{"docs/readme.txt", "bin/fallback"}
	got := FindBinary(files, "missing", tmpDir)
	if runtime.GOOS == "windows" {
		if got != "" {
			t.Fatalf("windows should not see executable mode fallback, got %q", got)
		}
		return
	}
	if got != "bin/fallback" {
		t.Fatalf("expected executable fallback, got %q", got)
	}
}

func TestVerifyChecksumWithAssetDigest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "payload.bin")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := ComputeSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	SetAssetDigest("sha256:" + sum)
	defer ClearAssetDigest()
	if err := verifyChecksumIfAvailable(path, "payload.bin"); err != nil {
		t.Fatalf("expected checksum verification success: %v", err)
	}
}

// Helper functions

func createTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content: %v", err)
		}
	}
}

func createMaliciousTarGz(t *testing.T, path string, maliciousFile string) {
	t.Helper()

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name: maliciousFile,
		Mode: 0644,
		Size: 10,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tw.Write([]byte("malicious")); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	// Skip if zip is not available
	if !hasZipCommand() {
		t.Skip("zip command not available")
	}

	// Create a temporary directory with files
	tmpDir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}
}

func createZipArchive(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry: %v", err)
		}
	}
}

func runtimeGOOS() string {
	return "linux"
}

func runtimeGOARCH() string {
	return "amd64"
}

func hasZipCommand() bool {
	_, err := os.Stat("/usr/bin/zip")
	return err == nil
}

// createTarXz creates a tar.xz archive for testing
func createTarXz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer out.Close()

	xw, err := xz.NewWriter(out)
	if err != nil {
		t.Fatalf("failed to create xz writer: %v", err)
	}
	defer xw.Close()

	tw := tar.NewWriter(xw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content: %v", err)
		}
	}
}

// createMaliciousTarXz creates a malicious tar.xz archive with path traversal
func createMaliciousTarXz(t *testing.T, path string, maliciousFile string) {
	t.Helper()

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer out.Close()

	xw, err := xz.NewWriter(out)
	if err != nil {
		t.Fatalf("failed to create xz writer: %v", err)
	}
	defer xw.Close()

	tw := tar.NewWriter(xw)
	defer tw.Close()

	hdr := &tar.Header{
		Name: maliciousFile,
		Mode: 0644,
		Size: 10,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tw.Write([]byte("malicious")); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
}

// Fuzz-like test for archive extraction
func TestExtractTarInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "invalid.tar.gz")
	extractDir := filepath.Join(tmpDir, "extracted")

	// Write invalid data
	if err := os.WriteFile(archivePath, []byte("not a valid gzip"), 0644); err != nil {
		t.Fatalf("failed to write invalid archive: %v", err)
	}

	// Extract should fail
	_, err := extractTarGz(archivePath, extractDir)
	if err == nil {
		t.Error("expected error for invalid archive")
	}
}

// Mock downloadFile for testing (we don't test actual HTTP in unit tests)
func TestDownloadFile_InvalidURL(t *testing.T) {
	err := downloadFile("http://invalid.example.com/nonexistent", "/tmp/test")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestProgressBarFor(t *testing.T) {
	origIsTerminal := isTerminal
	origWriter := progressWriter
	t.Cleanup(func() {
		isTerminal = origIsTerminal
		progressWriter = origWriter
	})

	isTerminal = func(uintptr) bool { return true }
	progressWriter = io.Discard

	if bar := progressBarFor(128, "test.bin"); bar == nil {
		t.Error("expected progress bar to be created")
	}

	isTerminal = func(uintptr) bool { return false }
	if bar := progressBarFor(128, "test.bin"); bar != nil {
		t.Error("expected no progress bar when not a terminal")
	}

	if bar := progressBarFor(0, "test.bin"); bar != nil {
		t.Error("expected no progress bar for zero content length")
	}
}

func TestDownloadFile_WithProgress(t *testing.T) {
	origIsTerminal := isTerminal
	origWriter := progressWriter
	t.Cleanup(func() {
		isTerminal = origIsTerminal
		progressWriter = origWriter
	})

	isTerminal = func(uintptr) bool { return true }
	progressWriter = io.Discard

	payload := []byte("download payload")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "16")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping download test; cannot open listener: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.Start()
	defer server.Close()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "payload.bin")
	if err := DownloadFile(server.URL, target); err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded data mismatch: got %q, want %q", string(data), string(payload))
	}
}

func TestDownloadFile_HTTPNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	err := DownloadFile(srv.URL, filepath.Join(t.TempDir(), "x"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadFile_InvalidURL_Regression(t *testing.T) {
	err := DownloadFile("://bad-url", filepath.Join(t.TempDir(), "x"))
	if err == nil {
		t.Fatal("expected error")
	}
}
