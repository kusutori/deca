package cmd

import "testing"

func TestRemoveCommand(t *testing.T) {
	tests := []commandTestCase{
		{
			name:         "missing required name",
			args:         []string{"remove"},
			wantErr:      true,
			wantContains: []string{"requires at least 1 arg"},
			wantExitCode: 1,
		},
		{
			name: "flag combination keep-installed and keep-desktop",
			args: []string{"remove", "ghost", "--keep-installed", "--keep-desktop"},
			mockSetup: func(t *testing.T) {
				setupTempConfig(t)
			},
			wantErr:      false,
			wantContains: []string{"not found in config"},
			wantExitCode: 0,
		},
		{
			name:         "help output",
			args:         []string{"remove", "--help"},
			wantErr:      false,
			wantContains: []string{"Remove a package", "--keep-installed"},
			wantExitCode: 0,
		},
		{
			name: "success path missing package still succeeds",
			args: []string{"remove", "ghost"},
			mockSetup: func(t *testing.T) {
				setupTempConfig(t)
			},
			wantErr:      false,
			wantContains: []string{"not found in config"},
			wantExitCode: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runCommandTest(t, tt) })
	}
}
