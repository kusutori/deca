package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/download"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/ui"
)

var downloadFileFunc = download.DownloadFile

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
	// Expand $HOME in BinDir first
	binDir := expandPath(i.BinDir)

	// Create bin directory if needed
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Check if this is an AppImage (self-contained executable)
	if strings.HasSuffix(strings.ToLower(asset.Name), ".appimage") {
		return i.installAppImage(name, release, asset, binDir)
	}

	// Check if this is a system package (.deb, .rpm, etc.)
	if pkgType := DetectPackageType(asset.Name); pkgType != "" {
		// System package - download and install via package manager
		return i.installSystemPackage(name, release, asset, pkgType, binDir)
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
			// No match found, look for the first executable file
			for _, f := range result.Files {
				fullPath := filepath.Join(result.TempDir, f)
				info, err := os.Stat(fullPath)
				if err != nil {
					continue
				}
				if !info.IsDir() && info.Mode()&0111 != 0 {
					binaryPath = fullPath
					break
				}
			}
			// If still not found, use first regular file
			if binaryPath == "" {
				for _, f := range result.Files {
					fullPath := filepath.Join(result.TempDir, f)
					info, err := os.Stat(fullPath)
					if err != nil {
						continue
					}
					if !info.IsDir() {
						binaryPath = fullPath
						break
					}
				}
			}
		}
	}

	// If still no binary found, fallback to downloaded path
	if binaryPath == "" {
		binaryPath = result.Path
	}

	// Get just the binary name for the final path
	finalBinaryName := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		finalBinaryName += ".exe"
	}
	finalPath := filepath.Join(binDir, finalBinaryName)

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
func (i *Installer) installAppImage(name string, release *github.ReleaseInfo, asset *github.AssetInfo, binDir string) (*InstallResult, error) {
	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "deca-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download the AppImage
	downloadPath := filepath.Join(tempDir, asset.Name)
	if err := downloadFileFunc(asset.DownloadURL, downloadPath); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", asset.Name, err)
	}

	// AppImage needs to be executable
	if err := os.Chmod(downloadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to set executable permission: %w", err)
	}

	// Copy to bin directory
	finalPath := filepath.Join(binDir, name)
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
func (i *Installer) installSystemPackage(name string, release *github.ReleaseInfo, asset *github.AssetInfo, pkgType string, binDir string) (*InstallResult, error) {
	// Download the package file
	downloadPath := filepath.Join(binDir, asset.Name)
	if err := downloadFileFunc(asset.DownloadURL, downloadPath); err != nil {
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
			ui.Info.Printf("Installing %s requires sudo privileges.\n", name)
		}
		if err := SudoRun(pkgManager, "install", "-y", downloadPath); err != nil {
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
	binDir := expandPath(i.BinDir)
	path := filepath.Join(binDir, name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}

	// Try without .exe on Windows
	if runtime.GOOS == "windows" {
		pathNoExt := filepath.Join(binDir, name)
		if _, err := os.Stat(pathNoExt); err == nil {
			return os.Remove(pathNoExt)
		}
	}

	return os.ErrNotExist
}

// uninstallAppImage removes an AppImage
func (i *Installer) uninstallAppImage(name string) error {
	binDir := expandPath(i.BinDir)
	path := filepath.Join(binDir, name)
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
	return os.MkdirAll(expandPath(i.BinDir), 0755)
}

// BinDirInPATH checks if bin_dir is in PATH
func (i *Installer) BinDirInPATH() bool {
	pathEnv := os.Getenv("PATH")
	pathSep := ":"
	if runtime.GOOS == "windows" {
		pathSep = ";"
	}

	// Expand $HOME in BinDir first
	binDir := expandPath(i.BinDir)
	absBinDir, _ := filepath.Abs(binDir)
	for _, dir := range strings.Split(pathEnv, pathSep) {
		// Also expand $HOME in PATH entries
		expandedDir := expandPath(dir)
		absDir, _ := filepath.Abs(expandedDir)
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

// expandPath expands $HOME and ~ in a path
func expandPath(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		path = strings.ReplaceAll(path, "$HOME", home)
		path = strings.ReplaceAll(path, "~", home)
	}
	return path
}
