//go:build !windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kusutori/deca/internal/github"
)

func (i *Installer) installWindows(name string, release *github.ReleaseInfo, asset *github.AssetInfo, binDir, preference string) (*InstallResult, error) {
	return nil, fmt.Errorf("windows install mode is only supported on Windows")
}

func (i *Installer) uninstallWindowsPortable(name string, meta UninstallMetadata) error {
	return fmt.Errorf("windows portable uninstall is only supported on Windows")
}

func (i *Installer) uninstallWindowsMSI(name, productCode string) error {
	return fmt.Errorf("windows MSI uninstall is only supported on Windows")
}

func (i *Installer) uninstallWindowsInstaller(name string) error {
	return fmt.Errorf("windows installer uninstall is only supported on Windows")
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
