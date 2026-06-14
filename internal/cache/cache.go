package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache represents the download cache
type Cache struct {
	RootDir string
}

// NewCache creates a new cache manager
func NewCache() *Cache {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	// Use XDG cache dir if available, otherwise ~/.cache/deca
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache", "deca")
	} else {
		cacheDir = filepath.Join(cacheDir, "deca")
	}

	return &Cache{RootDir: cacheDir}
}

// GetPath returns the cache path for a given repo, version, and asset
func (c *Cache) GetPath(repo, version, assetName string) string {
	// Normalize repo name (replace / with -)
	repoName := strings.ReplaceAll(repo, "/", "-")
	// Sanitize asset name (keep only safe characters)
	safeAsset := filepath.Base(assetName)
	return filepath.Join(c.RootDir, repoName, version, safeAsset)
}

// GetCachePath returns the cache path for a download URL
// URL format is used to derive the cache key
func (c *Cache) GetCachePath(repo, version, assetName string) (string, error) {
	path := c.GetPath(repo, version, assetName)
	return path, nil
}

// Get returns the cached file path if it exists, empty string otherwise
func (c *Cache) Get(repo, version, assetName string) string {
	path, err := c.GetCachePath(repo, version, assetName)
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return ""
	}
	return path
}

// Put stores a file in the cache
func (c *Cache) Put(repo, version, assetName, sourcePath string) (string, error) {
	cachePath, err := c.GetCachePath(repo, version, assetName)
	if err != nil {
		return "", err
	}

	// Create directory structure
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Copy file to cache
	if err := copyFile(sourcePath, cachePath); err != nil {
		return "", fmt.Errorf("failed to copy to cache: %w", err)
	}

	return cachePath, nil
}

// Exists checks if a file exists in the cache
func (c *Cache) Exists(repo, version, assetName string) bool {
	path := c.Get(repo, version, assetName)
	return path != ""
}

// Remove removes a file from the cache
func (c *Cache) Remove(repo, version, assetName string) error {
	path, err := c.GetCachePath(repo, version, assetName)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// Clean removes all cached files
func (c *Cache) Clean() error {
	return os.RemoveAll(c.RootDir)
}

// CleanOrphans removes cached files that are no longer in the state
func (c *Cache) CleanOrphans(statePackages map[string]struct{}) error {
	entries, err := c.ListEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Check if this entry is still referenced
		key := entry.Repo + "/" + entry.Version + "/" + entry.AssetName
		if _, exists := statePackages[key]; !exists {
			if err := os.Remove(entry.Path); err != nil {
				continue
			}
		}
	}

	return nil
}

// Entry represents a cache entry
type Entry struct {
	Repo      string
	Version   string
	AssetName string
	Path      string
	Size      int64
}

// ListEntries returns all cache entries
func (c *Cache) ListEntries() ([]Entry, error) {
	var entries []Entry

	err := filepath.Walk(c.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Parse path: root/repo/version/assetName
		rel, err := filepath.Rel(c.RootDir, path)
		if err != nil {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) >= 3 {
			repo := strings.ReplaceAll(parts[0], "-", "/")
			version := parts[1]
			assetName := filepath.Join(parts[2:]...)
			entries = append(entries, Entry{
				Repo:      repo,
				Version:   version,
				AssetName: assetName,
				Path:      path,
				Size:      info.Size(),
			})
		}

		return nil
	})

	return entries, err
}

// Size returns the total size of the cache in bytes
func (c *Cache) Size() (int64, error) {
	var total int64

	err := filepath.Walk(c.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})

	return total, err
}

// Count returns the number of cached files
func (c *Cache) Count() (int, error) {
	entries, err := c.ListEntries()
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// EnsureDir creates the cache directory if it doesn't exist
func (c *Cache) EnsureDir() error {
	return os.MkdirAll(c.RootDir, 0755)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := srcFile.Seek(0, 0); err != nil {
		return err
	}

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	_, err = dstFile.ReadFrom(srcFile)
	if err != nil {
		return err
	}

	return dstFile.Chmod(info.Mode())
}
