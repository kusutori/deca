package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// InstalledPackage represents the state of an installed package
type InstalledPackage struct {
	Repo        string    `json:"repo"`
	Version     string    `json:"version"`
	AssetName   string    `json:"asset_name,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
}

// State represents the installation state
type State struct {
	Packages map[string]InstalledPackage `json:"packages"`
}

// LoadState reads the state file
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
