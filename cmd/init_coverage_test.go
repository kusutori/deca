package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectLinuxDistribution(t *testing.T) {
	tmp := t.TempDir()
	origOSReleasePath, origReleaseFiles := osReleasePath, linuxReleaseFiles
	t.Cleanup(func() {
		osReleasePath = origOSReleasePath
		linuxReleaseFiles = origReleaseFiles
	})

	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{"debian family", "ubuntu", "ubuntu"},
		{"fedora family", "rocky", "fedora"},
		{"arch family", "manjaro", "arch"},
		{"suse family", "opensuse-tumbleweed", "opensuse"},
		{"alpine", "alpine", "alpine"},
		{"unknown", "nixos", "nixos"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tmp, tc.name)
			if err := os.WriteFile(path, []byte("NAME=Test\nID=\""+tc.id+"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			osReleasePath = path
			linuxReleaseFiles = nil
			if got := detectLinuxDistribution(); got != tc.want {
				t.Fatalf("detectLinuxDistribution() = %q, want %q", got, tc.want)
			}
		})
	}

	osReleasePath = filepath.Join(tmp, "missing")
	fallback := filepath.Join(tmp, "lsb-release")
	if err := os.WriteFile(fallback, []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
	linuxReleaseFiles = []string{fallback}
	if got := detectLinuxDistribution(); got != "linux" {
		t.Fatalf("fallback distribution = %q, want linux", got)
	}
}

func TestDetectPackageManager(t *testing.T) {
	for _, tc := range []struct {
		distribution string
		want         string
	}{
		{"ubuntu", "apt"},
		{"debian", "apt"},
		{"arch", "pacman"},
		{"opensuse", "zypper"},
		{"alpine", "apk"},
	} {
		if got := detectPackageManager(tc.distribution); got != tc.want {
			t.Errorf("detectPackageManager(%q) = %q, want %q", tc.distribution, got, tc.want)
		}
	}

	pathDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	managerName := func(name string) string {
		if runtime.GOOS == "windows" {
			return name + ".exe"
		}
		return name
	}
	setManagers := func(names ...string) {
		for _, name := range []string{"apt", "dnf", "brew"} {
			_ = os.Remove(filepath.Join(pathDir, managerName(name)))
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(pathDir, managerName(name)), []byte("test"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Setenv("PATH", pathDir); err != nil {
			t.Fatal(err)
		}
	}
	setManagers("dnf")
	if got := detectPackageManager("fedora"); got != "dnf" {
		t.Errorf("fedora with dnf = %q, want dnf", got)
	}
	setManagers()
	if got := detectPackageManager("fedora"); got != "yum" {
		t.Errorf("fedora without dnf = %q, want yum", got)
	}
	for _, tc := range []struct {
		available string
		want      string
	}{{"apt", "apt"}, {"dnf", "dnf"}, {"brew", "brew"}, {"", "unknown"}} {
		if tc.available == "" {
			setManagers()
		} else {
			setManagers(tc.available)
		}
		if got := detectPackageManager("other"); got != tc.want {
			t.Errorf("fallback with %q = %q, want %q", tc.available, got, tc.want)
		}
	}
}

func TestDetectSystemInfo(t *testing.T) {
	info := detectSystemInfo()
	if info.OS != runtime.GOOS || info.Arch != runtime.GOARCH || info.BinDir == "" {
		t.Fatalf("unexpected system info: %+v", info)
	}
}
