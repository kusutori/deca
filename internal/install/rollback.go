package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kusutori/deca/internal/config"
)

// BinaryPath returns the installed binary path for a package.
// For system packages it returns an empty string.
func BinaryPath(binDir, name string, installType config.InstallType) string {
	if installType == config.InstallTypeSystem {
		return ""
	}

	path := filepath.Join(binDir, name)
	if installType == config.InstallTypeBinary && runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		path += ".exe"
	}
	return path
}

// BackupFile creates a backup copy next to the target file.
func BackupFile(path string) (string, error) {
	backupPath := path + ".deca.bak"
	if err := copyFile(path, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

// RestoreFile restores a backup over the target file.
func RestoreFile(backupPath, targetPath string) error {
	if backupPath == "" || targetPath == "" {
		return nil
	}
	if err := copyFile(backupPath, targetPath); err != nil {
		return err
	}
	return nil
}

// RemoveBackup deletes a backup file if present.
func RemoveBackup(backupPath string) error {
	if backupPath == "" {
		return nil
	}
	if _, err := os.Stat(backupPath); err != nil {
		return nil
	}
	return os.Remove(backupPath)
}
