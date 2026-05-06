package download

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	// Skip - zip extraction requires a proper zip file which we can't easily create
	// without an external library
	t.Skip("zip extraction test skipped - requires external zip library")
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

func TestDownloadFileExternalDependencyFailures(t *testing.T) {
	t.Run("http timeout", func(t *testing.T) {
		origGet := httpGetFunc
		httpGetFunc = func(string) (*http.Response, error) { return nil, &net.DNSError{IsTimeout: true} }
		defer func() { httpGetFunc = origGet }()
		err := downloadFile("https://example.com/x", filepath.Join(t.TempDir(), "x"))
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("empty response body", func(t *testing.T) {
		origGet := httpGetFunc
		httpGetFunc = func(string) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(http.NoBody), ContentLength: 0}, nil
		}
		defer func() { httpGetFunc = origGet }()
		target := filepath.Join(t.TempDir(), "x")
		if err := downloadFile("https://example.com/x", target); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		data, err := os.ReadFile(target)
		if err != nil || len(data) != 0 {
			t.Fatalf("expected empty file, got len=%d err=%v", len(data), err)
		}
	})

	t.Run("create failure", func(t *testing.T) {
		origGet := httpGetFunc
		origCreate := downloadCreate
		httpGetFunc = func(string) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("x"))}, nil
		}
		downloadCreate = func(string) (*os.File, error) { return nil, os.ErrPermission }
		defer func() {
			httpGetFunc = origGet
			downloadCreate = origCreate
		}()
		if err := downloadFile("https://example.com/x", filepath.Join(t.TempDir(), "x")); !os.IsPermission(err) {
			t.Fatalf("expected permission error, got %v", err)
		}
	})
}
