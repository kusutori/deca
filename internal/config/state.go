package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

var (
	stateReadFile  = os.ReadFile
	stateMkdirAll  = os.MkdirAll
	stateWriteFile = os.WriteFile
)

// InstallType represents how a package was installed
type InstallType string

const (
	InstallTypeBinary   InstallType = "binary"   // Extracted from tar.gz/zip
	InstallTypeAppImage InstallType = "appimage" // AppImage executable
	InstallTypeSystem   InstallType = "system"   // System package (.deb/.rpm)
	InstallTypeSingle   InstallType = "single"   // Single binary file
)

// InstalledPackage represents the state of an installed package
type InstalledPackage struct {
	Repo                string      `json:"repo"`
	Version             string      `json:"version"`
	AssetName           string      `json:"asset_name,omitempty"`
	InstallType         InstallType `json:"install_type"`
	InstalledAt         time.Time   `json:"installed_at"`
	SystemPkgName       string      `json:"system_pkg_name,omitempty"`       // Actual package name for system packages
	VersionedBinaryPath string      `json:"versioned_binary_path,omitempty"` // Path to versioned binary (e.g. eza-v0.20.0)
}

// State represents the installation state
type State struct {
	Packages map[string]InstalledPackage `json:"packages"`
}

// LoadState reads the state file
func LoadState(path string) (*State, error) {
	data, err := stateReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Packages: make(map[string]InstalledPackage)}, nil
		}
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.Packages == nil {
		state.Packages = make(map[string]InstalledPackage)
	}

	return &state, nil
}

// SaveState writes the state to a file
func (s *State) SaveState(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := stateMkdirAll(dir, 0755); err != nil {
		return err
	}

	return stateWriteFile(path, data, 0644)
}

// DefaultStatePath returns the default state file path
func DefaultStatePath() string {
	return filepath.Join(DefaultStateDir(), "state.json")
}

// SetPackage updates or adds a package to the state
func (s *State) SetPackage(name string, pkg InstalledPackage) {
	s.Packages[name] = pkg
}

// RemovePackage removes a package from the state
func (s *State) RemovePackage(name string) {
	delete(s.Packages, name)
}

// GetPackage returns a package from the state
func (s *State) GetPackage(name string) (InstalledPackage, bool) {
	pkg, ok := s.Packages[name]
	return pkg, ok
}
