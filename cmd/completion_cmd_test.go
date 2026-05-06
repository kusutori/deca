package cmd

import "testing"

func TestCompletionCommand(t *testing.T) {
	tests := []commandTestCase{
		{
			name:         "missing required shell",
			args:         []string{"completion"},
			wantErr:      true,
			wantErrText:  "accepts 1 arg(s), received 0",
			wantExitCode: 1,
		},
		{
			name:         "invalid shell format",
			args:         []string{"completion", "unknown-shell"},
			wantErr:      true,
			wantErrText:  "unsupported shell",
			wantExitCode: 1,
		},
		{
			name:         "help output",
			args:         []string{"completion", "--help"},
			wantErr:      false,
			wantExitCode: 0,
		},
		{
			name:         "success path bash",
			args:         []string{"completion", "bash"},
			wantErr:      false,
			wantExitCode: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runCommandTest(t, tt) })
	}
}
