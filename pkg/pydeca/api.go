package pydeca

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"github.com/deca-org/deca/cmd"
	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/schema"
)

// Client is a Python-friendly wrapper around Deca operations.
type Client struct {
	ConfigPath string
}

// NewClient creates a client bound to configPath.
// If configPath is empty, the default Deca config path is used.
func NewClient(configPath string) *Client {
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	return &Client{ConfigPath: configPath}
}

// NewDefaultClient creates a client with default config path.
func NewDefaultClient() *Client {
	return NewClient(DefaultConfigPath())
}

// DefaultConfigPath returns the default path to deca.toml.
func DefaultConfigPath() string {
	return filepath.Join(config.DefaultConfigDir(), "deca.toml")
}

// DefaultSchemaPath returns the default path to deca.schema.json.
func DefaultSchemaPath() string {
	return filepath.Join(config.DefaultConfigDir(), "deca.schema.json")
}

// Version returns the current Deca version string.
func Version() string {
	return cmd.Version
}

// GenerateSchemaJSON returns the TOML JSON schema string.
func GenerateSchemaJSON() (string, error) {
	data, err := schema.GenerateSchema()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteSchema writes the schema JSON to path.
func WriteSchema(path string) error {
	return schema.WriteSchema(path)
}

// LoadConfigJSON loads deca.toml and returns it as JSON.
func LoadConfigJSON(path string) (string, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListPackageNames returns sorted package names in deca.toml.
func ListPackageNames(path string) ([]string, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// InjectSchema writes or updates "$schema" in configPath.
func InjectSchema(configPath, schemaPath string) error {
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	if schemaPath == "" {
		schemaPath = DefaultSchemaPath()
	}
	return schema.InjectSchemaReference(configPath, schemaPath)
}

// Run executes Deca CLI commands and returns process exit code.
// Example args: []string{"status"} or []string{"add", "owner/repo"}.
func (c *Client) Run(args []string) int {
	fullArgs := append([]string{}, args...)
	if c.ConfigPath != "" {
		fullArgs = append([]string{"--config", c.ConfigPath}, fullArgs...)
	}
	cmd.RootCmd.SetArgs(fullArgs)
	return cmd.Execute()
}

// GenerateSchemaJSON returns schema JSON through client.
func (c *Client) GenerateSchemaJSON() (string, error) {
	return GenerateSchemaJSON()
}

// WriteSchema writes schema JSON through client.
func (c *Client) WriteSchema(path string) error {
	if path == "" {
		path = DefaultSchemaPath()
	}
	return WriteSchema(path)
}

// LoadConfigJSON loads config through client.
func (c *Client) LoadConfigJSON() (string, error) {
	return LoadConfigJSON(c.ConfigPath)
}

// ListPackageNames lists package names through client.
func (c *Client) ListPackageNames() ([]string, error) {
	return ListPackageNames(c.ConfigPath)
}

// InjectSchema injects schema reference through client.
func (c *Client) InjectSchema(schemaPath string) error {
	return InjectSchema(c.ConfigPath, schemaPath)
}
