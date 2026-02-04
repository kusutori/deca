package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/deca-org/deca/internal/config"
)

func TestBinaryPath(t *testing.T) {
	binDir := "/tmp/bin"
	path := BinaryPath(binDir, "tool", config.InstallTypeSystem)
	if path != "" {
		t.Fatalf("expected empty path for system packages")
	}

	path = BinaryPath(binDir, "tool", config.InstallTypeAppImage)
	if filepath.Base(path) != "tool" {
		t.Fatalf("unexpected path: %s", path)
	}

	path = BinaryPath(binDir, "tool", config.InstallTypeBinary)
	if runtime.GOOS == "windows" {
		if filepath.Base(path) != "tool.exe" {
			t.Fatalf("expected .exe on windows, got %s", path)
		}
	} else if filepath.Base(path) != "tool" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestBackupRestoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "tool")
	if err := os.WriteFile(target, []byte("v1"), 0755); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}

	backup, err := BackupFile(target)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if err := os.WriteFile(target, []byte("v2"), 0755); err != nil {
		t.Fatalf("failed to update target: %v", err)
	}

	if err := RestoreFile(backup, target); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "v1" {
		t.Fatalf("unexpected content after restore: %q", string(data))
	}
}
