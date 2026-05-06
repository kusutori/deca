package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

// DefaultConfigDir returns the default configuration directory
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".config", "deca")
}

// DefaultStateDir returns the default state directory
func DefaultStateDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".local", "state", "deca")
}

// DefaultDesktopDir returns the default desktop entry directory
func DefaultDesktopDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".local", "share", "applications")
}

// DesktopEntryPath returns the path for a .desktop entry file
func DesktopEntryPath(name string) string {
	return filepath.Join(DefaultDesktopDir(), name+".desktop")
}

// DefaultBinDir returns the default binary directory
func DefaultBinDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "deca", "bin")
	default:
		return filepath.Join(home, ".local", "bin")
	}
}

// Config represents the main configuration file
type Config struct {
	BinDir     string             `toml:"bin_dir"`
	OS         string             `toml:"os"`
	Arch       string             `toml:"arch"`
	Packages   map[string]Package `toml:"packages"`
	Settings   Settings           `toml:"settings"`
	SystemInfo *SystemInfo        `toml:"system_info"`
}

// SystemInfo stores detected system information
type SystemInfo struct {
	OS             string `toml:"os"`
	Arch           string `toml:"arch"`
	Distribution   string `toml:"distribution"`
	PackageManager string `toml:"package_manager"`
	BinDir         string `toml:"bin_dir"`
}

// DesktopConfig represents .desktop file configuration
type DesktopConfig struct {
	Name       string `toml:"name"`        // Application name (defaults to package name)
	Comment    string `toml:"comment"`     // Short description
	Icon       string `toml:"icon"`        // Icon name or path
	Terminal   bool   `toml:"terminal"`    // Whether to run in terminal
	Categories string `toml:"categories"`  // Categories (default: Utilities)
	MimeTypes  string `toml:"mime_types"`  // MIME types
}

// Package represents a single package configuration
type Package struct {
	Repo       string         `toml:"repo"`
	Asset      string         `toml:"asset"`
	Version    string         `toml:"version"`
	OS         string         `toml:"os"`
	Arch       string         `toml:"arch"`
	Desktop    *DesktopConfig `toml:"desktop"`
	Versioned  bool           `toml:"versioned"`  // Keep versioned binaries with symlink
	Prerelease bool           `toml:"prerelease"` // Allow selecting pre-release versions when version is not pinned
}

// Settings represents optional settings
type Settings struct {
	AutoUpdate    bool   `toml:"auto_update"`
	CheckInterval string `toml:"check_interval"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First, try unmarshaling into a generic map to handle mixed formats
	var generic map[string]interface{}
	if err := toml.Unmarshal(data, &generic); err != nil {
		return nil, err
	}

	cfg := &Config{
		Packages: make(map[string]Package),
	}

	// Extract bin_dir
	if binDir, ok := generic["bin_dir"].(string); ok {
		cfg.BinDir = binDir
	}

	// Extract OS and Arch
	if os, ok := generic["os"].(string); ok {
		cfg.OS = os
	}
	if arch, ok := generic["arch"].(string); ok {
		cfg.Arch = arch
	}

	// Set defaults
	if cfg.BinDir == "" {
		cfg.BinDir = DefaultBinDir()
	}
	if cfg.OS == "" {
		cfg.OS = runtime.GOOS
	}
	if cfg.Arch == "" {
		cfg.Arch = runtime.GOARCH
	}

	// Parse packages - handle both string and table formats
	if packages, ok := generic["packages"].(map[string]interface{}); ok {
		for name, value := range packages {
			pkg := Package{}
			switch v := value.(type) {
			case string:
				// Simple format: eza = "owner/repo"
				pkg.Repo = v
			case map[string]interface{}:
				// Full format: zellij = { repo = "...", asset = "..." }
				if repo, ok := v["repo"].(string); ok {
					pkg.Repo = repo
				}
				if asset, ok := v["asset"].(string); ok {
					pkg.Asset = asset
				}
				if version, ok := v["version"].(string); ok {
					pkg.Version = version
				}
				if os, ok := v["os"].(string); ok {
					pkg.OS = os
				}
				if arch, ok := v["arch"].(string); ok {
					pkg.Arch = arch
				}
				// Parse desktop config
				if desktop, ok := v["desktop"].(map[string]interface{}); ok {
					pkg.Desktop = &DesktopConfig{
						Name:       getString(desktop, "name"),
						Comment:    getString(desktop, "comment"),
						Icon:       getString(desktop, "icon"),
						Terminal:   getBool(desktop, "terminal"),
						Categories: getString(desktop, "categories"),
						MimeTypes:  getString(desktop, "mime_types"),
					}
				}
				// Parse versioned flag
				pkg.Versioned = getBool(v, "versioned")
				pkg.Prerelease = getBool(v, "prerelease")
			}
			cfg.Packages[name] = pkg
		}
	}

	// Parse settings
	if settings, ok := generic["settings"].(map[string]interface{}); ok {
		cfg.Settings.AutoUpdate = getBool(settings, "auto_update")
		if interval, ok := settings["check_interval"].(string); ok {
			cfg.Settings.CheckInterval = interval
		}
	}

	// Parse system_info
	if systemInfo, ok := generic["system_info"].(map[string]interface{}); ok {
		cfg.SystemInfo = &SystemInfo{
			OS:             getString(systemInfo, "os"),
			Arch:           getString(systemInfo, "arch"),
			Distribution:   getString(systemInfo, "distribution"),
			PackageManager: getString(systemInfo, "package_manager"),
			BinDir:         getString(systemInfo, "bin_dir"),
		}
	}

	return cfg, nil
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// LoadDefault loads the default configuration file
func LoadDefault() (*Config, error) {
	configPath := filepath.Join(DefaultConfigDir(), "deca.toml")
	return Load(configPath)
}

// Save writes the configuration to a file
func Save(cfg *Config, path string) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// PackageRef returns a reference string for the package (for display)
func (p *Package) PackageRef() string {
	if p.Repo != "" {
		return p.Repo
	}
	return ""
}
