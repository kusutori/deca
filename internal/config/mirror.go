package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Mirror represents a GitHub mirror source
type Mirror struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	// APIURL is the GitHub API URL (e.g., https://api.github.com)
	// For most mirrors, this is the same as URL with /api/v3
	APIURL string `toml:"api_url"`
	// DownloadURL is the asset download URL pattern
	// {owner}, {repo}, {asset} will be replaced
	DownloadURL string `toml:"download_url"`
}

// DefaultMirrors returns the list of default mirrors
func DefaultMirrors() []Mirror {
	return []Mirror{
		{
			Name:        "GitHub (Official)",
			URL:         "https://github.com",
			APIURL:      "https://api.github.com",
			DownloadURL: "https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}",
		},
		{
			Name:        "GitHub Fast (China)",
			URL:         "https://ghfast.top",
			APIURL:      "https://api.github.com",
			DownloadURL: "https://ghfast.top/https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}",
		},
		{
			Name:        "GitHub Proxy (China)",
			URL:         "https://github.moeyy.xyz",
			APIURL:      "https://github.moeyy.xyz",
			DownloadURL: "https://github.moeyy.xyz/{owner}/{repo}/releases/download/{tag}/{asset}",
		},
		{
			Name:        "Jihulab (China)",
			URL:         "https://jihulab.com",
			APIURL:      "https://jihulab.com/api/v4",
			DownloadURL: "https://jihulab.com/{owner}/{repo}/-/releases/{tag}/downloads/{asset}",
		},
		{
			Name:        "FastGit (China)",
			URL:         "https://fastgit.org",
			APIURL:      "https://api.fastgit.org",
			DownloadURL: "https://download.fastgit.org/{owner}/{repo}/releases/download/{tag}/{asset}",
		},
	}
}

// GetMirrorByName returns a mirror by name
func GetMirrorByName(name string, mirrors []Mirror) *Mirror {
	for i := range mirrors {
		if mirrors[i].Name == name {
			return &mirrors[i]
		}
	}
	return nil
}

// MirrorConfig represents the mirror configuration
type MirrorConfig struct {
	Mirrors     []Mirror `toml:"mirrors"`
	CurrentName string   `toml:"current"`
}

// LoadMirrorConfig loads the mirror configuration
func LoadMirrorConfig(path string) (*MirrorConfig, error) {
	// Try to load from file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultMirrorConfig(), nil
		}
		return nil, fmt.Errorf("failed to read mirror config: %w", err)
	}

	var config MirrorConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		// If parsing fails, return default config
		return DefaultMirrorConfig(), nil
	}

	// Ensure we have mirrors (merge with defaults if empty)
	if len(config.Mirrors) == 0 {
		config.Mirrors = DefaultMirrors()
	}

	// Validate current mirror exists
	if config.CurrentName == "" {
		config.CurrentName = "GitHub (Official)"
	}

	return &config, nil
}

// DefaultMirrorConfig returns the default mirror configuration
func DefaultMirrorConfig() *MirrorConfig {
	return &MirrorConfig{
		Mirrors:     DefaultMirrors(),
		CurrentName: "GitHub (Official)",
	}
}

// LoadCurrentMirror loads the current mirror from config file, falling back to defaults.
func LoadCurrentMirror() *Mirror {
	cfg, err := LoadMirrorConfig(GetMirrorPath())
	if err != nil {
		return DefaultMirrorConfig().GetCurrentMirror()
	}
	current := cfg.GetCurrentMirror()
	if current == nil {
		return DefaultMirrorConfig().GetCurrentMirror()
	}
	return current
}

// GetCurrentMirror returns the currently selected mirror
func (c *MirrorConfig) GetCurrentMirror() *Mirror {
	return GetMirrorByName(c.CurrentName, c.Mirrors)
}

// GetMirrorPath returns the path to the mirror config file
func GetMirrorPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".config", "deca", "mirrors.toml")
}

// SaveMirrorConfig saves the mirror configuration
func SaveMirrorConfig(path string, config *MirrorConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to TOML
	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal mirror config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
