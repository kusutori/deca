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

	"github.com/deca-org/deca/internal/github"
)

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Asset    *github.AssetInfo
	Path     string
	Files    []string
	IsBinary bool
}

// DownloadAndExtract downloads an asset and extracts it
func DownloadAndExtract(asset *github.AssetInfo, targetOS, targetArch string) (*DownloadResult, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "deca-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download file
	downloadPath := filepath.Join(tempDir, filepath.Base(asset.Name))
	if err := downloadFile(asset.DownloadURL, downloadPath); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
	}

	// Extract based on file type
	result := &DownloadResult{
		Asset: asset,
		Path:  downloadPath,
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

	_, err = io.Copy(out, resp.Body)
	return err
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
	// Use external xz command if available
	// Otherwise, return an error
	return nil, fmt.Errorf("tar.xz extraction is not yet supported; please use tar.gz assets")
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
func FindBinary(files []string, name string) string {
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

	// Look for binary in PATH-like names
	for _, f := range files {
		base := filepath.Base(f)
		if base == name ||
			strings.Contains(base, name) ||
			strings.EqualFold(base, name) {
			return f
		}
	}

	// Return first executable file if no match
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return f
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
