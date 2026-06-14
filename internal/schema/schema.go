package schema

import "encoding/json"

// JSONSchema represents a JSON Schema object.
type JSONSchema struct {
	Schema               string                 `json:"$schema,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Type                 any                    `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	AdditionalProperties *JSONSchema            `json:"additionalProperties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
	Default              any                    `json:"default,omitempty"`
	Required             []string               `json:"required,omitempty"`
}

// GenerateSchema returns the JSON Schema (draft-07) for deca.toml as indented JSON bytes.
func GenerateSchema() ([]byte, error) {
	s := buildSchema()
	return json.MarshalIndent(s, "", "  ")
}

func buildSchema() *JSONSchema {
	return &JSONSchema{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Title:       "Deca Configuration",
		Description: "Configuration file for the Deca GitHub Release package manager.",
		Type:        "object",
		Properties: map[string]*JSONSchema{
			"$schema": {
				Type:        "string",
				Description: "Path or URL to the JSON Schema for this config file (used by TOML LSPs).",
			},
			"bin_dir": {
				Type:        "string",
				Description: "Directory where binaries are installed. Defaults to ~/.local/bin on Linux/macOS.",
			},
			"os": {
				Type:        "string",
				Description: "Target operating system. Defaults to the current OS.",
				Enum:        []string{"linux", "darwin", "windows"},
			},
			"arch": {
				Type:        "string",
				Description: "Target CPU architecture. Defaults to the current arch.",
				Enum:        []string{"amd64", "arm64", "386", "arm"},
			},
			"packages": {
				Type:        "object",
				Description: "Map of package names to their configuration. Each value is either a 'owner/repo' string or a full package table.",
				AdditionalProperties: &JSONSchema{
					OneOf: []*JSONSchema{
						{
							Type:        "string",
							Description: "Simple format: 'owner/repo'.",
						},
						buildPackageSchema(),
					},
				},
			},
			"settings": buildSettingsSchema(),
			"system_info": {
				Type:        "object",
				Description: "Auto-detected system information written by 'deca init'. Do not edit manually.",
				Properties: map[string]*JSONSchema{
					"os":              {Type: "string", Description: "Detected operating system."},
					"arch":            {Type: "string", Description: "Detected CPU architecture."},
					"distribution":    {Type: "string", Description: "Linux distribution name (e.g. ubuntu, arch)."},
					"package_manager": {Type: "string", Description: "Detected system package manager (apt, dnf, brew, …)."},
					"bin_dir":         {Type: "string", Description: "Resolved binary directory."},
				},
			},
		},
	}
}

func buildPackageSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Full package configuration table.",
		Properties: map[string]*JSONSchema{
			"repo": {
				Type:        "string",
				Description: "GitHub repository in 'owner/repo' format. Required for the table format.",
			},
			"asset": {
				Type:        "string",
				Description: "Glob pattern to match the release asset filename (e.g. '*.tar.gz', '*linux_amd64*').",
			},
			"version": {
				Type:        "string",
				Description: "Pin a specific release tag (e.g. 'v1.2.3'). Omit to always use the latest release.",
			},
			"os": {
				Type:        "string",
				Description: "Override OS for this package.",
				Enum:        []string{"linux", "darwin", "windows"},
			},
			"arch": {
				Type:        "string",
				Description: "Override architecture for this package.",
				Enum:        []string{"amd64", "arm64", "386", "arm"},
			},
			"install_type": {
				Type:        "string",
				Description: "Install strategy. On Windows, use 'portable' for direct executables, 'msi' for Windows Installer packages, or 'installer' for interactive GUI installers. Defaults to 'auto'.",
				Enum:        []string{"auto", "portable", "msi", "installer"},
				Default:     "auto",
			},
			"versioned": {
				Type:        "boolean",
				Description: "Keep versioned binaries on disk and create a symlink to the active version.",
				Default:     false,
			},
			"prerelease": {
				Type:        "boolean",
				Description: "Allow pre-release versions when selecting the latest release (ignored when version is pinned).",
				Default:     false,
			},
			"desktop": buildDesktopSchema(),
		},
		Required: []string{"repo"},
	}
}

func buildDesktopSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Generate a .desktop entry for this package (Linux only).",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Application display name shown in launchers. Defaults to the package name.",
			},
			"comment": {
				Type:        "string",
				Description: "Short description shown in application launchers.",
			},
			"icon": {
				Type:        "string",
				Description: "Icon name (from the icon theme) or absolute path to an icon file.",
			},
			"terminal": {
				Type:        "boolean",
				Description: "Whether to run the application in a terminal emulator.",
				Default:     false,
			},
			"categories": {
				Type:        "string",
				Description: "Semicolon-separated XDG categories (e.g. 'Utility;TextEditor'). Defaults to 'Utilities'.",
			},
			"mime_types": {
				Type:        "string",
				Description: "Semicolon-separated MIME types the application can handle.",
			},
		},
	}
}

func buildSettingsSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Optional global settings.",
		Properties: map[string]*JSONSchema{
			"auto_update": {
				Type:        "boolean",
				Description: "Automatically check for and apply updates.",
				Default:     false,
			},
			"check_interval": {
				Type:        "string",
				Description: "How often to check for updates (e.g. '24h', '7d'). Only used when auto_update is true.",
			},
		},
	}
}
