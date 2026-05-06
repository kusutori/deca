package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddCommand(t *testing.T) {
	tests := []commandTestCase{
		{
			name:         "missing required repo",
			args:         []string{"add"},
			wantErr:      true,
			wantErrText:  "requires at least 1 arg",
			wantExitCode: 1,
		},
		{
			name: "invalid repo format",
			args: []string{"add", "badrepo", "--no-install"},
			mockSetup: func(t *testing.T) {
				setupTempConfig(t)
			},
			wantErr:      true,
			wantErrText:  "invalid repo format",
			wantExitCode: 1,
		},
		{
			name:         "help output",
			args:         []string{"add", "--help"},
			wantErr:      false,
			wantExitCode: 0,
		},
		{
			name: "success path with no install",
			args: []string{"add", "cli/cli", "--no-install", "--name", "gh"},
			mockSetup: func(t *testing.T) {
				setupTempConfig(t)
			},
			wantErr:      false,
			wantExitCode: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runCommandTest(t, tt) })
	}
}

func setupTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deca.toml")
	if err := os.WriteFile(path, []byte("bin_dir = \"~/bin\"\n[packages]\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	orig := configPath
	t.Cleanup(func() { configPath = orig })
	configPath = path
}
