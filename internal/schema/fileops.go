package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteSchema writes the generated schema JSON to path.
func WriteSchema(path string) error {
	if path == "" {
		return fmt.Errorf("schema path is required")
	}

	data, err := GenerateSchema()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// InjectSchemaReference adds or updates the "$schema" key at the top of a TOML file.
func InjectSchemaReference(configPath, schemaPath string) error {
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}
	if schemaPath == "" {
		return fmt.Errorf("schema path is required")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	schemaLine := fmt.Sprintf(`"$schema" = %q`, schemaPath)
	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines)+1)

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `"$schema"`) {
			continue
		}
		filtered = append(filtered, line)
	}

	out := strings.Join(append([]string{schemaLine}, filtered...), "\n")
	return os.WriteFile(configPath, []byte(out), 0644)
}
