//go:build windows

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/download"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/ui"
)

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
