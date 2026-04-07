package pydeca

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSchemaJSON(t *testing.T) {
	data, err := GenerateSchemaJSON()
	if err != nil {
		t.Fatalf("GenerateSchemaJSON failed: %v", err)
	}
	if !strings.Contains(data, `"title": "Deca Configuration"`) {
		t.Fatalf("schema output missing expected title: %s", data)
	}
}

func TestLoadConfigJSONAndListPackageNames(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "deca.toml")
	content := `
bin_dir = "/tmp/bin"

[packages]
eza = "eza-community/eza"
bat = { repo = "sharkdp/bat", asset = "*.tar.gz" }
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfgJSON, err := LoadConfigJSON(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfigJSON failed: %v", err)
	}
	if !strings.Contains(cfgJSON, `"BinDir":"/tmp/bin"`) {
		t.Fatalf("unexpected config JSON: %s", cfgJSON)
	}

	names, err := ListPackageNames(cfgPath)
	if err != nil {
		t.Fatalf("ListPackageNames failed: %v", err)
	}
	if len(names) != 2 || names[0] != "bat" || names[1] != "eza" {
		t.Fatalf("unexpected package names: %#v", names)
	}
}

func TestInjectSchema(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "deca.toml")
	schemaPath := filepath.Join(dir, "deca.schema.json")

	if err := os.WriteFile(cfgPath, []byte("[packages]\neza = \"eza-community/eza\"\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := WriteSchema(schemaPath); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}
	if err := InjectSchema(cfgPath, schemaPath); err != nil {
		t.Fatalf("InjectSchema failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), `"$schema" = "`+schemaPath+`"`) {
		t.Fatalf("schema reference not injected: %s", string(data))
	}
}
