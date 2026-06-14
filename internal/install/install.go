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

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/download"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/ui"
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
	Name                string
	Version             string
	BinaryPath          string
	AssetName           string
	InstallType         config.InstallType
	SystemPkgName       string // Actual package name for system packages (e.g., "fresh-editor" from "fresh_0.1.98.deb")
	VersionedBinaryPath string // Path to versioned binary, set when versioned symlink was created
	InstallRoot         string
	ExposedPath         string
	ProductCode         string
	LinkType            string
}

// Install installs a package from a release
func (i *Installer) Install(name string, release *github.ReleaseInfo, asset *github.AssetInfo, installTypePreference ...string) (*InstallResult, error) {
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

	if runtime.GOOS == "windows" {
		preference := "auto"
		if len(installTypePreference) > 0 && strings.TrimSpace(installTypePreference[0]) != "" {
			preference = installTypePreference[0]
		}
		return i.installWindows(name, release, asset, binDir, preference)
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
		found := download.FindBinary(result.Files, binaryName, result.TempDir)
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
		ExposedPath: finalPath,
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
		ExposedPath: finalPath,
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

func (i *Installer) installWindows(name string, release *github.ReleaseInfo, asset *github.AssetInfo, binDir, preference string) (*InstallResult, error) {
	mode, err := resolveWindowsInstallMode(asset.Name, preference)
	if err != nil {
		return nil, err
	}

	switch mode {
	case "msi":
		return i.installWindowsMSI(name, release, asset)
	case "installer":
		return i.installWindowsInstaller(name, release, asset)
	default:
		return i.installWindowsPortable(name, release, asset, binDir)
	}
}

func resolveWindowsInstallMode(assetName, preference string) (string, error) {
	preference = strings.ToLower(strings.TrimSpace(preference))
	if preference == "" {
		preference = "auto"
	}
	assetName = strings.ToLower(assetName)

	switch preference {
	case "auto":
		if strings.HasSuffix(assetName, ".msi") {
			return "msi", nil
		}
		return "portable", nil
	case "portable":
		return preference, nil
	case "msi":
		if !strings.HasSuffix(assetName, ".msi") {
			return "", fmt.Errorf("install_type %q requires an .msi asset", preference)
		}
		return preference, nil
	case "installer":
		if !strings.HasSuffix(assetName, ".exe") {
			return "", fmt.Errorf("install_type %q requires an .exe asset", preference)
		}
		return preference, nil
	default:
		return "", fmt.Errorf("unsupported install_type %q", preference)
	}
}

func (i *Installer) installWindowsPortable(name string, release *github.ReleaseInfo, asset *github.AssetInfo, binDir string) (*InstallResult, error) {
	result, err := download.DownloadAndExtractWithCache(asset, runtime.GOOS, runtime.GOARCH, release.Owner+"/"+release.Repo, release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer os.RemoveAll(result.TempDir)

	installRoot := WindowsInstallRoot(name, release.TagName)
	if err := os.RemoveAll(installRoot); err != nil {
		return nil, fmt.Errorf("failed to prepare install root: %w", err)
	}
	if err := os.MkdirAll(installRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install root: %w", err)
	}

	if len(result.Files) == 0 {
		result.Files = []string{filepath.Base(result.Path)}
	}
	if err := copyExtractedFiles(result.TempDir, installRoot, result.Files); err != nil {
		return nil, fmt.Errorf("failed to install files: %w", err)
	}

	binaryName := name
	if !strings.HasSuffix(strings.ToLower(binaryName), ".exe") {
		binaryName += ".exe"
	}
	binaryRel := download.FindBinary(result.Files, binaryName, result.TempDir)
	if binaryRel == "" {
		binaryRel = download.FindBinary(result.Files, name, result.TempDir)
	}
	if binaryRel == "" {
		binaryRel = firstWindowsExecutable(result.Files)
	}
	if binaryRel == "" {
		return nil, fmt.Errorf("no executable found in %s", asset.Name)
	}

	installedBinaryPath := filepath.Join(installRoot, binaryRel)
	exposedPath := filepath.Join(binDir, binaryName)
	linkType, err := exposeWindowsExecutable(installedBinaryPath, exposedPath)
	if err != nil {
		return nil, err
	}

	return &InstallResult{
		Name:                name,
		Version:             release.TagName,
		BinaryPath:          exposedPath,
		AssetName:           asset.Name,
		InstallType:         config.InstallTypeBinary,
		VersionedBinaryPath: installedBinaryPath,
		InstallRoot:         installRoot,
		ExposedPath:         exposedPath,
		LinkType:            linkType,
	}, nil
}

func (i *Installer) installWindowsMSI(name string, release *github.ReleaseInfo, asset *github.AssetInfo) (*InstallResult, error) {
	result, err := download.DownloadAndExtractWithCache(asset, runtime.GOOS, runtime.GOARCH, release.Owner+"/"+release.Repo, release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer os.RemoveAll(result.TempDir)

	productCode, err := readMSIProductCode(result.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read MSI product code: %w", err)
	}

	cmd := exec.Command("msiexec", "/i", result.Path, "/qn", "/norestart")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to install MSI: %w", err)
	}

	return &InstallResult{
		Name:        name,
		Version:     release.TagName,
		AssetName:   asset.Name,
		InstallType: config.InstallTypeWindowsMSI,
		ProductCode: productCode,
	}, nil
}

func (i *Installer) installWindowsInstaller(name string, release *github.ReleaseInfo, asset *github.AssetInfo) (*InstallResult, error) {
	result, err := download.DownloadAndExtractWithCache(asset, runtime.GOOS, runtime.GOARCH, release.Owner+"/"+release.Repo, release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer os.RemoveAll(result.TempDir)

	cmd := exec.Command(result.Path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("installer exited with error: %w", err)
	}

	return &InstallResult{
		Name:        name,
		Version:     release.TagName,
		AssetName:   asset.Name,
		InstallType: config.InstallTypeWindowsInstaller,
	}, nil
}

// WindowsInstallRoot returns the versioned install root used by portable Windows installs.
func WindowsInstallRoot(name, version string) string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "deca", "packages", sanitizePathSegment(name), sanitizePathSegment(version))
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(value)
}

func copyExtractedFiles(srcRoot, dstRoot string, files []string) error {
	for _, rel := range files {
		cleanRel := filepath.Clean(rel)
		if cleanRel == "." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) || filepath.IsAbs(cleanRel) {
			return fmt.Errorf("unsafe archive path: %s", rel)
		}

		src := filepath.Join(srcRoot, cleanRel)
		dst := filepath.Join(dstRoot, cleanRel)
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.MkdirAll(dst, info.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func firstWindowsExecutable(files []string) string {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(filepath.Base(f)), ".exe") {
			return f
		}
	}
	return ""
}

func exposeWindowsExecutable(targetPath, exposedPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(exposedPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}
	if err := os.Remove(exposedPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to replace exposed executable: %w", err)
	}
	if err := os.Link(targetPath, exposedPath); err == nil {
		return "hardlink", nil
	}
	if err := copyFile(targetPath, exposedPath); err != nil {
		return "", fmt.Errorf("failed to expose executable: %w", err)
	}
	return "copy", nil
}

func readMSIProductCode(msiPath string) (string, error) {
	script := `$msiPath = $args[0]
$installer = New-Object -ComObject WindowsInstaller.Installer
$database = $installer.GetType().InvokeMember('OpenDatabase', 'InvokeMethod', $null, $installer, @($msiPath, 0))
$view = $database.GetType().InvokeMember('OpenView', 'InvokeMethod', $null, $database, @("SELECT Value FROM Property WHERE Property = 'ProductCode'"))
$view.GetType().InvokeMember('Execute', 'InvokeMethod', $null, $view, $null) | Out-Null
$record = $view.GetType().InvokeMember('Fetch', 'InvokeMethod', $null, $view, $null)
if ($null -eq $record) { exit 2 }
$record.GetType().InvokeMember('StringData', 'GetProperty', $null, $record, 1)`
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script, msiPath).Output()
	if err != nil {
		return "", err
	}
	productCode := strings.TrimSpace(string(output))
	if productCode == "" {
		return "", fmt.Errorf("MSI product code is empty")
	}
	return productCode, nil
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

// UninstallMetadata contains optional state needed to remove platform-specific installs.
type UninstallMetadata struct {
	InstallRoot string
	ExposedPath string
	ProductCode string
}

// Uninstall removes a package based on its install type
func (i *Installer) Uninstall(name string, installType config.InstallType, systemPkgName string, metadata ...UninstallMetadata) error {
	var meta UninstallMetadata
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	switch installType {
	case config.InstallTypeSystem:
		return i.uninstallSystemPackage(name, systemPkgName)
	case config.InstallTypeAppImage:
		return i.uninstallAppImage(name)
	case config.InstallTypeWindowsMSI:
		return i.uninstallWindowsMSI(name, meta.ProductCode)
	case config.InstallTypeWindowsInstaller:
		return i.uninstallWindowsInstaller(name)
	default:
		if runtime.GOOS == "windows" {
			return i.uninstallWindowsPortable(name, meta)
		}
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

func (i *Installer) uninstallWindowsPortable(name string, meta UninstallMetadata) error {
	binDir := expandPath(i.BinDir)
	exposedPath := meta.ExposedPath
	if exposedPath == "" {
		exposedPath = filepath.Join(binDir, name)
		if !strings.HasSuffix(strings.ToLower(exposedPath), ".exe") {
			exposedPath += ".exe"
		}
	}

	var removed bool
	if err := os.Remove(exposedPath); err == nil {
		removed = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if !removed && strings.HasSuffix(strings.ToLower(exposedPath), ".exe") {
		pathNoExt := strings.TrimSuffix(exposedPath, filepath.Ext(exposedPath))
		if err := os.Remove(pathNoExt); err == nil {
			removed = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	if meta.InstallRoot != "" {
		if err := os.RemoveAll(meta.InstallRoot); err != nil {
			return err
		}
		removed = true
	}

	if removed {
		return nil
	}
	return os.ErrNotExist
}

func (i *Installer) uninstallWindowsMSI(name, productCode string) error {
	if productCode == "" {
		return fmt.Errorf("cannot uninstall %s: missing MSI product code", name)
	}
	cmd := exec.Command("msiexec", "/x", productCode, "/qn", "/norestart")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (i *Installer) uninstallWindowsInstaller(name string) error {
	ui.Info.Printf("%s was installed by an interactive Windows installer. Remove it from Windows Installed Apps if needed.\n", name)
	return nil
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

// expandPath expands $HOME and ~ in a path, and removes quotes
func expandPath(path string) string {
	// Remove leading/trailing single and double quotes
	path = strings.Trim(path, `'"`)

	home, _ := os.UserHomeDir()
	if home != "" {
		path = strings.ReplaceAll(path, "$HOME", home)
		path = strings.ReplaceAll(path, "~", home)
	}
	return path
}

// CreateVersionedSymlink creates a versioned copy of a binary and a symlink pointing to it.
// The versioned binary is named "<name>-<version>" and the symlink "<name>" points to it.
// Returns the path to the versioned binary.
// On Windows, symlinks are not created; the versioned binary is still copied.
func CreateVersionedSymlink(binDir, name, version, currentBinaryPath string) (string, error) {
	binDir = expandPath(binDir)
	// Normalize version: strip leading "v" for the file name suffix
	versionedName := name + "-" + version
	if runtime.GOOS == "windows" {
		versionedName += ".exe"
	}
	versionedPath := filepath.Join(binDir, versionedName)

	// Copy the current binary to the versioned path
	if err := copyFile(currentBinaryPath, versionedPath); err != nil {
		return "", fmt.Errorf("failed to copy versioned binary: %w", err)
	}
	if err := os.Chmod(versionedPath, 0755); err != nil {
		return "", fmt.Errorf("failed to set permissions on versioned binary: %w", err)
	}

	// On non-Windows, update symlink to point to the versioned binary
	if runtime.GOOS != "windows" {
		// Remove existing symlink or binary at the target path
		_ = os.Remove(currentBinaryPath)
		if err := os.Symlink(versionedPath, currentBinaryPath); err != nil {
			// Restore the binary if symlink creation fails
			_ = copyFile(versionedPath, currentBinaryPath)
			return versionedPath, fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	return versionedPath, nil
}

// UninstallVersioned removes the versioned binary and the symlink.
func UninstallVersioned(symlinkPath, versionedBinaryPath string) error {
	// Remove the symlink
	if symlinkPath != "" {
		_ = os.Remove(symlinkPath)
	}
	// Remove the versioned binary
	if versionedBinaryPath != "" {
		if err := os.Remove(versionedBinaryPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
