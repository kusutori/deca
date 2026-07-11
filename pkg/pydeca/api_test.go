package pydeca

import (
	"os"
	"path/filepath"
	"strconv"
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
	if !strings.Contains(string(data), `"$schema" = `+strconv.Quote(schemaPath)) {
		t.Fatalf("schema reference not injected: %s", string(data))
	}
}

func TestClientConvenienceMethods(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	cfgPath := filepath.Join(dir, "deca.toml")
	schemaPath := filepath.Join(dir, "schema.json")
	content := `
bin_dir = "/tmp/bin"

[packages]
eza = "eza-community/eza"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	client := NewClient(cfgPath)
	if client.ConfigPath != cfgPath {
		t.Fatalf("unexpected client config path: %s", client.ConfigPath)
	}
	if NewClient("").ConfigPath == "" {
		t.Fatal("expected default client path")
	}
	if NewDefaultClient().ConfigPath == "" {
		t.Fatal("expected default client path")
	}
	if DefaultConfigPath() == "" || DefaultSchemaPath() == "" || Version() == "" {
		t.Fatal("expected default metadata")
	}

	schemaJSON, err := client.GenerateSchemaJSON()
	if err != nil {
		t.Fatalf("client GenerateSchemaJSON failed: %v", err)
	}
	if !strings.Contains(schemaJSON, "Deca Configuration") {
		t.Fatalf("unexpected schema json: %s", schemaJSON)
	}
	if err := client.WriteSchema(schemaPath); err != nil {
		t.Fatalf("client WriteSchema failed: %v", err)
	}
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if err := client.WriteSchema(""); err != nil {
		t.Fatalf("client WriteSchema default path failed: %v", err)
	}

	cfgJSON, err := client.LoadConfigJSON()
	if err != nil {
		t.Fatalf("client LoadConfigJSON failed: %v", err)
	}
	if !strings.Contains(cfgJSON, "eza-community/eza") {
		t.Fatalf("unexpected config json: %s", cfgJSON)
	}
	names, err := client.ListPackageNames()
	if err != nil {
		t.Fatalf("client ListPackageNames failed: %v", err)
	}
	if len(names) != 1 || names[0] != "eza" {
		t.Fatalf("unexpected names: %+v", names)
	}
	if err := client.InjectSchema(schemaPath); err != nil {
		t.Fatalf("client InjectSchema failed: %v", err)
	}
}

func TestClientRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "deca.toml")
	client := NewClient(cfgPath)

	if code := client.Run([]string{"--help"}); code != 0 {
		t.Fatalf("expected help to exit 0, got %d", code)
	}
	if code := client.Run([]string{"not-a-command"}); code == 0 {
		t.Fatal("expected invalid command to fail")
	}

	noConfigClient := &Client{}
	if code := noConfigClient.Run([]string{"--version"}); code != 0 {
		t.Fatalf("expected version to exit 0, got %d", code)
	}
}

func TestConfigHelpersReturnErrorsForMissingFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	if _, err := LoadConfigJSON(missing); err == nil {
		t.Fatal("LoadConfigJSON accepted missing file")
	}
	if _, err := ListPackageNames(missing); err == nil {
		t.Fatal("ListPackageNames accepted missing file")
	}
	if err := InjectSchema(missing, "schema.json"); err == nil {
		t.Fatal("InjectSchema accepted missing config")
	}
}
