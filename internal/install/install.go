package install

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/download"
	"github.com/deca-org/deca/internal/github"
)

// Installer handles installation of binaries
type Installer struct {
	BinDir string
}

// NewInstaller creates a new installer
func NewInstaller(binDir string) *Installer {
	return &Installer{BinDir: binDir}
}

// InstallResult contains the result of an installation
type InstallResult struct {
	Name          string
	Version       string
	BinaryPath    string
	AssetName     string
	InstallType   config.InstallType
	SystemPkgName string // Actual package name for system packages (e.g., "fresh-editor" from "fresh_0.1.98.deb")
}

// Install installs a package from a release
func (i *Installer) Install(name string, release *github.ReleaseInfo, asset *github.AssetInfo) (*InstallResult, error) {
	// Create bin directory if needed
	if err := os.MkdirAll(i.BinDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Check if this is an AppImage (self-contained executable)
	if strings.HasSuffix(strings.ToLower(asset.Name), ".appimage") {
		return i.installAppImage(name, release, asset)
	}

	// Check if this is a system package (.deb, .rpm, etc.)
	if pkgType := DetectPackageType(asset.Name); pkgType != "" {
		// System package - download and install via package manager
		return i.installSystemPackage(name, release, asset, pkgType)
	}

	// Regular binary package - download and extract with caching
	result, err := download.DownloadAndExtractWithCache(asset, runtime.GOOS, runtime.GOARCH, release.Owner+"/"+release.Repo, release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	// Clean up temp directory after we're done
	defer os.RemoveAll(result.TempDir)

	// Find the binary from extracted files
	var binaryPath string
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// For archives, result.Files contains the extracted file paths (relative to tempDir)
	// For single binaries, result.Files contains the download path
	if len(result.Files) > 0 {
		// Try to find the binary by name first
		found := download.FindBinary(result.Files, binaryName)
		if found != "" {
			binaryPath = filepath.Join(result.TempDir, found)
		} else {
			// Use first file if no match found
			binaryPath = filepath.Join(result.TempDir, result.Files[0])
		}
	} else {
		// Fallback to downloaded path (shouldn't happen normally)
		binaryPath = result.Path
	}

	// Get just the binary name for the final path
	finalBinaryName := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		finalBinaryName += ".exe"
	}
	finalPath := filepath.Join(i.BinDir, finalBinaryName)

	// Copy binary to bin directory
	if err := copyFile(binaryPath, finalPath); err != nil {
		return nil, fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(finalPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to set permissions: %w", err)
	}

	return &InstallResult{
		Name:        name,
		Version:     release.TagName,
		BinaryPath:  finalPath,
		AssetName:   asset.Name,
		InstallType: config.InstallTypeBinary,
	}, nil
}

// installAppImage handles AppImage installation (self-contained executable)
func (i *Installer) installAppImage(name string, release *github.ReleaseInfo, asset *github.AssetInfo) (*InstallResult, error) {
	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "deca-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download the AppImage
	downloadPath := filepath.Join(tempDir, asset.Name)
	if err := downloadFile(asset.DownloadURL, downloadPath); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
	}

	// AppImage needs to be executable
	if err := os.Chmod(downloadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to set executable permission: %w", err)
	}

	// Copy to bin directory
	finalPath := filepath.Join(i.BinDir, name)
	if err := copyFile(downloadPath, finalPath); err != nil {
		return nil, fmt.Errorf("failed to copy AppImage: %w", err)
	}

	return &InstallResult{
		Name:        name,
		Version:     release.TagName,
		BinaryPath:  finalPath,
		AssetName:   asset.Name,
		InstallType: config.InstallTypeAppImage,
	}, nil
}

// installSystemPackage handles system package installation (.deb, .rpm)
func (i *Installer) installSystemPackage(name string, release *github.ReleaseInfo, asset *github.AssetInfo, pkgType string) (*InstallResult, error) {
	// Download the package file
	downloadPath := filepath.Join(i.BinDir, asset.Name)
	if err := downloadFile(asset.DownloadURL, downloadPath); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
	}

	// Extract actual package name from .deb file
	var systemPkgName string
	if pkgType == "deb" {
		systemPkgName = extractDebPackageName(downloadPath)
		if systemPkgName == "" {
			// Fallback to using the asset name without version info
			systemPkgName = name
		}
	} else {
		systemPkgName = name
	}

	// Install via system package manager
	var pkgManager string

	switch pkgType {
	case "deb":
		pkgManager = "apt"
	case "rpm":
		if _, err := exec.LookPath("dnf"); err == nil {
			pkgManager = "dnf"
		} else {
			pkgManager = "yum"
		}
	default:
		return nil, fmt.Errorf("unsupported package type: %s", pkgType)
	}

	// Install the package with sudo
	if syscall.Getuid() != 0 {
		// Need sudo, check if cached
		if !IsSudoCached() {
			fmt.Printf("Installing %s requires sudo privileges.\n", name)
		}
		if err := SudoRun(pkgManager, append([]string{"install", "-y"}, downloadPath)...); err != nil {
			// Clean up failed download
			os.Remove(downloadPath)
			return nil, fmt.Errorf("failed to install %s: %w", name, err)
		}
	} else {
		// Running as root
		cmd := exec.Command(pkgManager, "install", "-y", downloadPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			os.Remove(downloadPath)
			return nil, fmt.Errorf("failed to install %s: %w", name, err)
		}
	}

	// Remove the .deb/.rpm file after successful installation
	os.Remove(downloadPath)

	return &InstallResult{
		Name:          name,
		Version:       release.TagName,
		BinaryPath:    "", // System packages don't have a simple binary path
		AssetName:     asset.Name,
		InstallType:   config.InstallTypeSystem,
		SystemPkgName: systemPkgName,
	}, nil
}

// extractDebPackageName extracts the actual package name from a .deb file
func extractDebPackageName(debPath string) string {
	// Use dpkg-deb to query the package name
	cmd := exec.Command("dpkg-deb", "-f", debPath, "Package")
	output, err := cmd.Output()
	if err != nil {
		// Fallback: try to parse from filename
		// Format: package_version_architecture.deb
		base := filepath.Base(debPath)
		parts := strings.Split(base, "_")
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	}
	return strings.TrimSpace(string(output))
}

// downloadFile downloads a file from a URL (for system packages)
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

	// Preserve permissions
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return err
	}

	return dstFile.Sync()
}

// Uninstall removes a package based on its install type
func (i *Installer) Uninstall(name string, installType config.InstallType, systemPkgName string) error {
	switch installType {
	case config.InstallTypeSystem:
		return i.uninstallSystemPackage(name, systemPkgName)
	case config.InstallTypeAppImage:
		return i.uninstallAppImage(name)
	default:
		return i.uninstallBinary(name)
	}
}

// uninstallBinary removes a binary installed from archive
func (i *Installer) uninstallBinary(name string) error {
	path := filepath.Join(i.BinDir, name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}

	// Try without .exe on Windows
	if runtime.GOOS == "windows" {
		pathNoExt := filepath.Join(i.BinDir, name)
		if _, err := os.Stat(pathNoExt); err == nil {
			return os.Remove(pathNoExt)
		}
	}

	return os.ErrNotExist
}

// uninstallAppImage removes an AppImage
func (i *Installer) uninstallAppImage(name string) error {
	path := filepath.Join(i.BinDir, name)
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return os.ErrNotExist
}

// uninstallSystemPackage removes a system package
func (i *Installer) uninstallSystemPackage(name, systemPkgName string) error {
	// Use the actual system package name if available, otherwise fallback to name
	pkgName := systemPkgName
	if pkgName == "" {
		pkgName = name
	}

	// Determine which package manager to use
	var pkgManager string
	if _, err := exec.LookPath("dnf"); err == nil {
		pkgManager = "dnf"
	} else if _, err := exec.LookPath("yum"); err == nil {
		pkgManager = "yum"
	} else if _, err := exec.LookPath("apt"); err == nil {
		pkgManager = "apt"
	} else {
		return fmt.Errorf("no supported package manager found")
	}

	// Remove via system package manager
	if syscall.Getuid() != 0 {
		if !IsSudoCached() {
			fmt.Printf("Removing %s (system package: %s) requires sudo privileges.\n", name, pkgName)
		}
		return SudoRun(pkgManager, "remove", "-y", pkgName)
	}

	// Running as root
	var cmd *exec.Cmd
	switch pkgManager {
	case "dnf":
		cmd = exec.Command("dnf", "remove", "-y", pkgName)
	case "yum":
		cmd = exec.Command("yum", "remove", "-y", pkgName)
	case "apt":
		cmd = exec.Command("apt", "remove", "-y", pkgName)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// EnsureBinDir creates the binary directory if it doesn't exist
func (i *Installer) EnsureBinDir() error {
	return os.MkdirAll(i.BinDir, 0755)
}

// BinDirInPATH checks if bin_dir is in PATH
func (i *Installer) BinDirInPATH() bool {
	pathEnv := os.Getenv("PATH")
	pathSep := ":"
	if runtime.GOOS == "windows" {
		pathSep = ";"
	}

	absBinDir, _ := filepath.Abs(i.BinDir)
	for _, dir := range strings.Split(pathEnv, pathSep) {
		absDir, _ := filepath.Abs(dir)
		if absDir == absBinDir {
			return true
		}
	}

	return false
}

// AddToPATHInstructions returns instructions for adding bin_dir to PATH
func (i *Installer) AddToPATHInstructions() string {
	shell := detectShell()
	switch shell {
	case "bash":
		return `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc`
	case "zsh":
		return `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc`
	case "fish":
		return fmt.Sprintf(`fish_add_path %s`, i.BinDir)
	default:
		return fmt.Sprintf(`Add "%s" to your PATH`, i.BinDir)
	}
}

// detectShell attempts to detect the current shell
func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	base := filepath.Base(shell)
	if strings.HasPrefix(base, "bash") {
		return "bash"
	}
	if strings.HasPrefix(base, "zsh") {
		return "zsh"
	}
	if strings.HasPrefix(base, "fish") {
		return "fish"
	}
	return base
}
