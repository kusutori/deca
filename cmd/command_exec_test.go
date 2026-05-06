package cmd

import (
	"bytes"
	"testing"
)

type commandTestCase struct {
	name         string
	args         []string
	mockSetup    func(t *testing.T)
	wantErr      bool
	wantContains []string
	wantExitCode int
}

func runCommandTest(t *testing.T, tt commandTestCase) {
	t.Helper()
	if tt.mockSetup != nil {
		tt.mockSetup(t)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	RootCmd.SetOut(out)
	RootCmd.SetErr(errOut)
	RootCmd.SilenceUsage = true
	RootCmd.SilenceErrors = true
	RootCmd.SetArgs(tt.args)

	err := RootCmd.Execute()
	exitCode := 0
	if err != nil {
		exitCode = 1
	}

	if tt.wantErr != (err != nil) {
		t.Fatalf("wantErr=%v got err=%v", tt.wantErr, err)
	}
	if tt.wantExitCode != exitCode {
		t.Fatalf("wantExitCode=%d got=%d err=%v", tt.wantExitCode, exitCode, err)
	}

	combined := out.String() + "\n" + errOut.String()
	for _, want := range tt.wantContains {
		if !bytes.Contains([]byte(combined), []byte(want)) {
			t.Fatalf("output missing %q\noutput:\n%s", want, combined)
		}
	}
}
