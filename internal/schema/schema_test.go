package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSchema(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema() error = %v", err)
	}

	var s JSONSchema
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("schema should be valid json: %v", err)
	}
	if s.Schema == "" || s.Title == "" {
		t.Fatalf("schema metadata should not be empty: %+v", s)
	}
	if s.Properties["packages"] == nil {
		t.Fatal("schema should include packages property")
	}
}

func TestWriteSchema(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "schemas", "deca.schema.json")
	if err := WriteSchema(out); err != nil {
		t.Fatalf("WriteSchema() error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read schema file: %v", err)
	}
	if !strings.Contains(string(data), `"Deca Configuration"`) {
		t.Fatalf("unexpected schema content: %s", string(data))
	}
}

func TestWriteSchemaEmptyPath(t *testing.T) {
	if err := WriteSchema(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestInjectSchemaReference(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "deca.toml")
	original := "\"$schema\" = \"old.json\"\nbin_dir = \"/tmp/bin\"\n"
	if err := os.WriteFile(cfg, []byte(original), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := InjectSchemaReference(cfg, "./schemas/deca.schema.json"); err != nil {
		t.Fatalf("InjectSchemaReference() error = %v", err)
	}

	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	out := string(got)
	if strings.Count(out, "\"$schema\"") != 1 {
		t.Fatalf("expected one schema line, got:\n%s", out)
	}
	if !strings.HasPrefix(out, "\"$schema\" = \"./schemas/deca.schema.json\"") {
		t.Fatalf("schema line should be first, got:\n%s", out)
	}
}

func TestInjectSchemaReferenceRejectsInvalidInputs(t *testing.T) {
	if err := InjectSchemaReference("", "schema.json"); err == nil {
		t.Fatal("expected missing config path error")
	}
	if err := InjectSchemaReference("config.toml", ""); err == nil {
		t.Fatal("expected missing schema path error")
	}
	if err := InjectSchemaReference(filepath.Join(t.TempDir(), "missing.toml"), "schema.json"); err == nil {
		t.Fatal("expected missing config file error")
	}
}
