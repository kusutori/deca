package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
	"github.com/spf13/cobra"
)

// InitCmd initializes the configuration
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize deca configuration",
	Long: `Initialize deca configuration and detect system information.

This command creates a default configuration file and detects
your system type for optimal package downloads.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Detect system information
		systemInfo := detectSystemInfo()

		ui.Primary.Println("Deca Initialization")
		ui.Secondary.Println(strings.Repeat("=", 40))
		fmt.Println()

		// Display detected information
		ui.Info.Println("Detected System Information:")
		ui.SearchMeta.Printf("  OS: %s\n", systemInfo.OS)
		ui.SearchMeta.Printf("  Arch: %s\n", systemInfo.Arch)
		ui.SearchMeta.Printf("  Package Manager: %s\n", systemInfo.PackageManager)
		ui.SearchMeta.Printf("  Bin Directory: %s\n", systemInfo.BinDir)
		fmt.Println()

		// Check if config already exists
		configPath := getConfigPath()
		if _, err := os.Stat(configPath); err == nil {
			ui.Warning.Printf("Config already exists: %s\n", configPath)
			ui.Info.Println("Use 'deca config' to edit it.")
			return nil
		}

		// Create default configuration
		cfg := &config.Config{
			BinDir:     config.DefaultBinDir(),
			OS:         systemInfo.OS,
			Arch:       systemInfo.Arch,
			Packages:   make(map[string]config.Package),
			SystemInfo: &config.SystemInfo{
				OS:            systemInfo.OS,
				Arch:          systemInfo.Arch,
				Distribution:  systemInfo.Distribution,
				PackageManager: systemInfo.PackageManager,
				BinDir:        systemInfo.BinDir,
			},
		}

		// Ensure config directory exists
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Save configuration
		if err := config.Save(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		ui.Success.Printf("Created config: %s\n", configPath)
		fmt.Println()

		// Print next steps
		ui.Info.Println("Next steps:")
		ui.SearchMeta.Println("  1. Edit config: deca config")
		ui.SearchMeta.Println("  2. Add packages: deca add owner/repo")
		ui.SearchMeta.Println("  3. Install: deca apply")
		fmt.Println()

		// Check if bin dir is in PATH
		installer := install.NewInstaller(cfg.BinDir)
		if !installer.BinDirInPATH() {
			ui.Warning.Printf("Note: %s is not in your PATH.\n", cfg.BinDir)
			ui.Info.Printf("Add it with: %s\n", installer.AddToPATHInstructions())
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(InitCmd)
}

// SystemInfo contains detected system information
type SystemInfo struct {
	OS            string // linux, darwin, windows
	Arch          string // amd64, arm64, etc.
	Distribution  string // ubuntu, debian, fedora, etc.
	PackageManager string // apt, dnf, yum, brew
	BinDir        string
}

// detectSystemInfo detects the current system information
func detectSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		BinDir: config.DefaultBinDir(),
	}

	// Detect Linux distribution
	if runtime.GOOS == "linux" {
		info.Distribution = detectLinuxDistribution()
		info.PackageManager = detectPackageManager(info.Distribution)
	} else if runtime.GOOS == "darwin" {
		info.Distribution = "macos"
		info.PackageManager = "brew"
	} else if runtime.GOOS == "windows" {
		info.Distribution = "windows"
		info.PackageManager = "choco"
	}

	return info
}

// detectLinuxDistribution detects the Linux distribution
func detectLinuxDistribution() string {
	// Try /etc/os-release first
	etcRelease := "/etc/os-release"
	if _, err := os.Stat(etcRelease); err == nil {
		data, err := os.ReadFile(etcRelease)
		if err == nil {
			content := string(data)
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "ID=") {
					id := strings.TrimPrefix(line, "ID=")
					id = strings.Trim(id, `"`)
					// Normalize common distributions
					switch id {
					case "ubuntu", "debian", "linuxmint", "pop":
						return "ubuntu"
					case "fedora", "rhel", "centos", "rocky", "alma":
						return "fedora"
					case "arch", "manjaro", "endeavouros":
						return "arch"
					case "opensuse", "opensuse-tumbleweed":
						return "opensuse"
					case "alpine":
						return "alpine"
					default:
						return id
					}
				}
			}
		}
	}

	// Try other common files
	distFiles := []string{
		"/etc/lsb-release",
		"/etc/redhat-release",
		"/etc/centos-release",
		"/etc/fedora-release",
	}

	for _, file := range distFiles {
		if _, err := os.Stat(file); err == nil {
			return "linux" // Generic Linux
		}
	}

	return "linux"
}

// detectPackageManager detects the package manager based on distribution
func detectPackageManager(distribution string) string {
	switch distribution {
	case "ubuntu", "debian", "linuxmint", "pop":
		return "apt"
	case "fedora", "rhel", "centos", "rocky", "alma":
		if _, err := exec.LookPath("dnf"); err == nil {
			return "dnf"
		}
		return "yum"
	case "arch", "manjaro", "endeavouros":
		return "pacman"
	case "opensuse", "opensuse-tumbleweed":
		return "zypper"
	case "alpine":
		return "apk"
	default:
		// Check for common package managers
		if _, err := exec.LookPath("apt"); err == nil {
			return "apt"
		}
		if _, err := exec.LookPath("dnf"); err == nil {
			return "dnf"
		}
		if _, err := exec.LookPath("brew"); err == nil {
			return "brew"
		}
		return "unknown"
	}
}
