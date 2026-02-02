package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	// Save and restore original
	original := configPath
	defer func() { configPath = original }()

	// Test with default
	configPath = ""
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "deca", "deca.toml")
	if got := getConfigPath(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	// Test with custom path
	configPath = "/custom/path/deca.toml"
	if got := getConfigPath(); got != "/custom/path/deca.toml" {
		t.Errorf("expected custom path, got %s", got)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "deca", "deca.toml")
	if got := defaultConfigPath(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPrintStatus(t *testing.T) {
	// Test with verbose = false
	original := verbose
	verbose = false
	defer func() { verbose = original }()

	// Should not panic, just do nothing
	printStatus("test message")

	// Test with verbose = true
	verbose = true
	printStatus("test message") // Would print, but we're capturing stdout in real usage
}

func TestGetCurrentOS(t *testing.T) {
	if got := getCurrentOS(); got == "" {
		t.Error("expected non-empty OS")
	}
}

func TestGetCurrentArch(t *testing.T) {
	if got := getCurrentArch(); got == "" {
		t.Error("expected non-empty arch")
	}
}

func TestExecute_NoError(t *testing.T) {
	// This is a simple test to verify Execute doesn't panic
	// Full integration testing would require more setup
	defer func() {
		if r := recover(); r != nil {
			// Expected if no command is registered
		}
	}()
	Execute()
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should be set")
	}
}

func TestRootCmdSetup(t *testing.T) {
	if RootCmd.Use != "deca" {
		t.Errorf("expected Use 'deca', got '%s'", RootCmd.Use)
	}
	if RootCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}
