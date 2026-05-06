package cache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheGetPath(t *testing.T) {
	c := &Cache{RootDir: "/tmp/test-cache"}

	path := c.GetPath("owner/repo", "v1.0.0", "file.tar.gz")
	expected := "/tmp/test-cache/owner-repo/v1.0.0/file.tar.gz"

	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCachePutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Put in cache
	cachePath, err := c.Put("owner/repo", "v1.0.0", "test.tar.gz", testFile)
	if err != nil {
		t.Fatalf("failed to put in cache: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache file does not exist: %v", err)
	}

	// Get from cache
	gotPath := c.Get("owner/repo", "v1.0.0", "test.tar.gz")
	if gotPath == "" {
		t.Error("expected to get cache path, got empty string")
	}

	// Verify content
	content, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("expected 'test content', got '%s'", string(content))
	}
}

func TestCacheExists(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Should not exist initially
	if c.Exists("owner/repo", "v1.0.0", "test.tar.gz") {
		t.Error("expected cache not to exist")
	}

	// Create test file in cache
	cacheDir := filepath.Join(tmpDir, "owner-repo", "v1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "test.tar.gz")
	if err := os.WriteFile(cacheFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	// Should exist now
	if !c.Exists("owner/repo", "v1.0.0", "test.tar.gz") {
		t.Error("expected cache to exist")
	}
}

func TestCacheRemove(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create test file in cache
	cacheDir := filepath.Join(tmpDir, "owner-repo", "v1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "test.tar.gz")
	if err := os.WriteFile(cacheFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	// Remove
	if err := c.Remove("owner/repo", "v1.0.0", "test.tar.gz"); err != nil {
		t.Errorf("failed to remove cache: %v", err)
	}

	// Should not exist
	if c.Exists("owner/repo", "v1.0.0", "test.tar.gz") {
		t.Error("cache should not exist after removal")
	}
}

func TestCacheClean(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create some files
	cacheDir1 := filepath.Join(tmpDir, "owner-repo1", "v1.0.0")
	cacheDir2 := filepath.Join(tmpDir, "owner-repo2", "v2.0.0")
	if err := os.MkdirAll(cacheDir1, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	if err := os.MkdirAll(cacheDir2, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cacheDir1, "test1.tar.gz"), []byte("test1"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir2, "test2.tar.gz"), []byte("test2"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	// Clean
	if err := c.Clean(); err != nil {
		t.Errorf("failed to clean cache: %v", err)
	}

	// Root dir should not exist anymore (Clean removes the entire directory)
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		// It's OK if root dir still exists but is empty
		entries, _ := os.ReadDir(tmpDir)
		if len(entries) != 0 {
			t.Errorf("expected 0 entries after clean, got %d", len(entries))
		}
	}
}

func TestCacheSize(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create test files with known sizes
	cacheDir := filepath.Join(tmpDir, "owner-repo", "v1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	// Create files of different sizes
	testContent := make([]byte, 1024) // 1KB
	if err := os.WriteFile(filepath.Join(cacheDir, "test1.tar.gz"), testContent, 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	testContent2 := make([]byte, 512) // 512B
	if err := os.WriteFile(filepath.Join(cacheDir, "test2.tar.gz"), testContent2, 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	size, err := c.Size()
	if err != nil {
		t.Fatalf("failed to get cache size: %v", err)
	}

	expected := int64(1536) // 1KB + 512B
	if size != expected {
		t.Errorf("expected %d, got %d", expected, size)
	}
}

func TestCacheCount(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create test files
	cacheDir := filepath.Join(tmpDir, "owner-repo", "v1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cacheDir, "test1.tar.gz"), []byte("test1"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "test2.tar.gz"), []byte("test2"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	count, err := c.Count()
	if err != nil {
		t.Fatalf("failed to get cache count: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCacheListEntries(t *testing.T) {
	tmpDir := t.TempDir()
	c := &Cache{RootDir: tmpDir}

	// Create test files
	cacheDir := filepath.Join(tmpDir, "owner-repo", "v1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cacheDir, "test.tar.gz"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	entries, err := c.ListEntries()
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Repo != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got '%s'", entries[0].Repo)
	}
	if entries[0].Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", entries[0].Version)
	}
}

func TestCacheNewCache(t *testing.T) {
	// Set a custom home for testing
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/tmp/test-home")
	defer os.Setenv("HOME", oldHome)

	// Clear XDG_CACHE_HOME
	oldXDG := os.Getenv("XDG_CACHE_HOME")
	os.Unsetenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", oldXDG)

	c := NewCache()
	expected := "/tmp/test-home/.cache/deca"

	if c.RootDir != expected {
		t.Errorf("expected %s, got %s", expected, c.RootDir)
	}
}

func TestCacheWithXDG(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CACHE_HOME")

	os.Setenv("HOME", "/tmp/test-home")
	os.Setenv("XDG_CACHE_HOME", "/custom/cache")

	defer func() {
		os.Setenv("HOME", oldHome)
		if oldXDG != "" {
			os.Setenv("XDG_CACHE_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CACHE_HOME")
		}
	}()

	c := NewCache()
	expected := "/custom/cache/deca"

	if c.RootDir != expected {
		t.Errorf("expected %s, got %s", expected, c.RootDir)
	}
}

func TestCacheNewCacheDefault(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CACHE_HOME")

	os.Setenv("HOME", "/tmp/test-home")
	os.Unsetenv("XDG_CACHE_HOME")

	defer func() {
		os.Setenv("HOME", oldHome)
		if oldXDG != "" {
			os.Setenv("XDG_CACHE_HOME", oldXDG)
		}
	}()

	c := NewCache()
	expected := "/tmp/test-home/.cache/deca"

	if c.RootDir != expected {
		t.Errorf("expected %s, got %s", expected, c.RootDir)
	}
}

func TestCachePut_SourceMissing_UsesWrappedError(t *testing.T) {
	c := &Cache{RootDir: t.TempDir()}

	_, err := c.Put("owner/repo", "v1.0.0", "asset.tar.gz", filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist wrapped, got %v", err)
	}
}

func TestCacheRemove_NotFound(t *testing.T) {
	c := &Cache{RootDir: t.TempDir()}
	err := c.Remove("owner/repo", "v1.0.0", "missing.tar.gz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist wrapped, got %v", err)
	}
}

func TestCacheGetPath_BoundaryInputs(t *testing.T) {
	c := &Cache{RootDir: t.TempDir()}
	long := strings.Repeat("a", 256)
	p := c.GetPath("", "", "../../"+long+".tar.gz")
	if filepath.Base(p) != long+".tar.gz" {
		t.Fatalf("expected sanitized basename, got %s", p)
	}
}
