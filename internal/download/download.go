package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deca-org/deca/internal/cache"
	"github.com/deca-org/deca/internal/github"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
	"github.com/ulikunitz/xz"
)

var isTerminal = isatty.IsTerminal
var progressWriter io.Writer = os.Stderr

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Chmod(srcInfo.Mode())
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Asset    *github.AssetInfo
	Path     string
	Files    []string
	TempDir  string // Caller must clean up this directory after use
	IsBinary bool
}

// DownloadAndExtract downloads an asset and extracts it
func DownloadAndExtract(asset *github.AssetInfo, targetOS, targetArch string) (*DownloadResult, error) {
	return DownloadAndExtractWithCache(asset, targetOS, targetArch, "", "")
}

// DownloadAndExtractWithCache downloads an asset with optional caching
func DownloadAndExtractWithCache(asset *github.AssetInfo, targetOS, targetArch string, repo, version string) (*DownloadResult, error) {
	// Create temp directory - caller must clean up via result.TempDir
	tempDir, err := os.MkdirTemp("", "deca-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Set asset digest for verification
	SetAssetDigest(asset.Digest)
	defer ClearAssetDigest()

	// Download file
	downloadPath := filepath.Join(tempDir, filepath.Base(asset.Name))
	var usedCache bool

	// Try to use cache if repo and version are provided
	if repo != "" && version != "" {
		c := cache.NewCache()
		cachedPath := c.Get(repo, version, asset.Name)
		if cachedPath != "" {
			// Copy from cache to temp dir
			if err := copyFile(cachedPath, downloadPath); err == nil {
				usedCache = true
			}
		}

		if !usedCache {
			// Download and cache
			if err := downloadFileWithCache(asset.DownloadURL, downloadPath, repo, version, asset.Name, asset.Digest); err != nil {
				return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
			}
		}
	} else {
		if err := downloadFile(asset.DownloadURL, downloadPath); err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
		}
	}

	// Extract based on file type
	result := &DownloadResult{
		Asset:   asset,
		Path:    downloadPath,
		TempDir: tempDir,
	}

	switch {
	case strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".tgz"):
		files, err := extractTarGz(downloadPath, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract tar.gz: %w", err)
		}
		result.Files = files
		result.IsBinary = true
	case strings.HasSuffix(asset.Name, ".tar.xz") || strings.HasSuffix(asset.Name, ".txz"):
		files, err := extractTarXz(downloadPath, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract tar.xz: %w", err)
		}
		result.Files = files
		result.IsBinary = true
	case strings.HasSuffix(asset.Name, ".zip"):
		files, err := extractZip(downloadPath, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract zip: %w", err)
		}
		result.Files = files
		result.IsBinary = true
	case strings.HasSuffix(asset.Name, ".exe"):
		// Windows executable - just copy
		result.Files = []string{downloadPath}
		result.IsBinary = true
	default:
		// Assume it's a single binary
		result.Files = []string{downloadPath}
		result.IsBinary = true
	}

	return result, nil
}

// DownloadFile downloads a file from a URL with progress when available.
func DownloadFile(url, path string) error {
	return downloadFile(url, path)
}

// downloadFile downloads a file from a URL
func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get content length for progress bar
	contentLength := resp.ContentLength
	filename := filepath.Base(path)

	// Use progress bar if we know the size and output is to a terminal
	bar := progressBarFor(contentLength, filename)

	// Copy with progress - use MultiWriter to write to both file and progress bar
	if bar != nil {
		_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	// Close progress bar if used
	if bar != nil {
		bar.Finish()
	}

	return err
}

// downloadFileWithCache downloads a file and caches it
func downloadFileWithCache(url, path, repo, version, assetName, digest string) error {
	// Set the digest for verification
	SetAssetDigest(digest)
	defer ClearAssetDigest()

	// Create temp file for download
	tmpPath := path + ".tmp"

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get content length for progress bar
	contentLength := resp.ContentLength
	filename := filepath.Base(path)

	// Use progress bar if we know the size and output is to a terminal
	bar := progressBarFor(contentLength, filename)

	// Copy with progress
	if bar != nil {
		_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	// Close progress bar if used
	if bar != nil {
		bar.Finish()
	}

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Rename to final path
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Verify checksum if available
	if err := verifyChecksumIfAvailable(path, assetName); err != nil {
		return err
	}

	// Cache the file
	c := cache.NewCache()
	c.EnsureDir()
	c.Put(repo, version, assetName, path)

	return nil
}

func progressBarFor(contentLength int64, filename string) *progressbar.ProgressBar {
	if contentLength <= 0 {
		return nil
	}
	if progressWriter == nil {
		return nil
	}
	if !isTerminal(os.Stderr.Fd()) {
		return nil
	}

	return progressbar.NewOptions64(contentLength,
		progressbar.OptionSetDescription("Downloading "+filename),
		progressbar.OptionSetWriter(progressWriter),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[cyan]=[reset]",
			SaucerHead:    "[cyan]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(progressWriter, "\n")
		}),
	)
}

// verifyChecksumIfAvailable checks for checksum and verifies the download
// Priority: GitHub API digest > local checksum file
func verifyChecksumIfAvailable(filePath, assetName string) error {
	// Try GitHub API digest first (new feature, preferred)
	if assetDigest != "" {
		fmt.Fprintf(os.Stderr, "[cyan]Verifying checksum (GitHub API)... [reset]")
		if err := VerifyChecksum(filePath, assetDigest, ChecksumTypeSHA256); err != nil {
			fmt.Fprintf(os.Stderr, "\n")
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[green]OK[reset]\n")
		return nil
	}

	// Fallback to local checksum file
	checksumFile := FindChecksumFile(assetName)
	if checksumFile == "" {
		return nil // No checksum found, skip verification
	}

	// Parse the checksum file
	info, err := ParseChecksumFile(checksumFile, assetName)
	if err != nil {
		return nil // Skip verification if checksum not found for this file
	}

	// Verify the checksum
	fmt.Fprintf(os.Stderr, "[cyan]Verifying checksum... [reset]")
	if err := VerifyChecksum(filePath, info.Expected, info.Type); err != nil {
		fmt.Fprintf(os.Stderr, "\n")
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[green]OK[reset]\n")

	return nil
}

// AssetDigest holds the SHA256 digest from GitHub API (set during download)
var assetDigest string

// SetAssetDigest sets the digest for the current download
func SetAssetDigest(digest string) {
	assetDigest = digest
}

// ClearAssetDigest clears the digest after verification
func ClearAssetDigest() {
	assetDigest = ""
}

// extractTarGz extracts a tar.gz archive
func extractTarGz(path, dest string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	return extractTar(gzReader, dest)
}

// extractTarXz extracts a tar.xz archive
func extractTarXz(path, dest string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return nil, err
	}

	return extractTar(xzReader, dest)
}

// extractZip extracts a zip archive
func extractZip(path, dest string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range reader.File {
		outPath := filepath.Join(dest, f.Name)

		// Security: ensure path is within destination
		absDest, _ := filepath.Abs(dest)
		absOut, _ := filepath.Abs(outPath)
		if !strings.HasPrefix(absOut, absDest) {
			return nil, fmt.Errorf("zip entry outside of target dir: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0755)
			continue
		}

		files = append(files, f.Name)

		src, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer src.Close()

		os.MkdirAll(filepath.Dir(outPath), 0755)
		dst, err := os.Create(outPath)
		if err != nil {
			return nil, err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// extractTar extracts a tar archive
func extractTar(reader io.Reader, dest string) ([]string, error) {
	tarReader := tar.NewReader(reader)

	var files []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		outPath := filepath.Join(dest, header.Name)

		// Security: ensure path is within destination
		absDest, _ := filepath.Abs(dest)
		absOut, _ := filepath.Abs(outPath)
		if !strings.HasPrefix(absOut, absDest) {
			return nil, fmt.Errorf("tar entry outside of target dir: %s", header.Name)
		}

		files = append(files, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(outPath, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(outPath), 0755)
			outFile, err := os.Create(outPath)
			if err != nil {
				return nil, err
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return nil, err
			}
			os.Chmod(outPath, header.FileInfo().Mode())
		case tar.TypeSymlink:
			// Handle symlinks
			linkTarget := header.Linkname
			if !filepath.IsAbs(linkTarget) {
				// Relative symlinks need special handling
				linkTarget = filepath.Join(filepath.Dir(outPath), linkTarget)
			}
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.Symlink(linkTarget, outPath)
		}
	}

	return files, nil
}

// FindBinary finds the binary in extracted files
func FindBinary(files []string, name string, tempDir string) string {
	// Look for exact match first
	for _, f := range files {
		if filepath.Base(f) == name {
			return f
		}
	}

	// Try with common extensions
	extensions := []string{"", ".exe"}
	for _, ext := range extensions {
		for _, f := range files {
			if filepath.Base(f) == name+ext {
				return f
			}
		}
	}

	// Look for binary in PATH-like names (skip directories if file exists)
	for _, f := range files {
		fullPath := filepath.Join(tempDir, f)
		info, err := os.Stat(fullPath)
		// Only skip if file exists AND is a directory
		if err == nil && info.IsDir() {
			continue
		}
		base := filepath.Base(f)
		if base == name ||
			strings.Contains(base, name) ||
			strings.EqualFold(base, name) {
			return f
		}
	}

	// Return first executable file if no match (only if tempDir is provided)
	if tempDir != "" {
		for _, f := range files {
			fullPath := filepath.Join(tempDir, f)
			info, err := os.Stat(fullPath)
			if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return f
			}
		}
	}

	return ""
}

// FileSize returns a human-readable file size
func FileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
