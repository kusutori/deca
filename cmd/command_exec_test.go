package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandTestCase struct {
	name         string
	args         []string
	mockSetup    func(t *testing.T)
	wantErr      bool
	wantContains []string
	wantExitCode int
	wantErrText  string
}

func runCommandTest(t *testing.T, tt commandTestCase) {
	t.Helper()
	resetCommandFlags(RootCmd)
	if tt.mockSetup != nil {
		tt.mockSetup(t)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()
	RootCmd.SetOut(out)
	RootCmd.SetErr(errOut)
	oldSilenceUsage := RootCmd.SilenceUsage
	oldSilenceErrors := RootCmd.SilenceErrors
	RootCmd.SilenceUsage = true
	RootCmd.SilenceErrors = true
	defer func() {
		RootCmd.SilenceUsage = oldSilenceUsage
		RootCmd.SilenceErrors = oldSilenceErrors
	}()
	RootCmd.SetArgs(tt.args)

	err := RootCmd.Execute()
	_ = wOut.Close()
	_ = wErr.Close()
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
	if tt.wantErrText != "" {
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte(tt.wantErrText)) {
			t.Fatalf("error missing %q, got %v", tt.wantErrText, err)
		}
	}

	stdoutBytes, _ := io.ReadAll(rOut)
	stderrBytes, _ := io.ReadAll(rErr)
	combined := out.String() + "\n" + errOut.String() + "\n" + string(stdoutBytes) + "\n" + string(stderrBytes)
	for _, want := range tt.wantContains {
		if !bytes.Contains([]byte(combined), []byte(want)) {
			t.Fatalf("output missing %q\noutput:\n%s", want, combined)
		}
	}
}

func resetCommandFlags(cmd *cobra.Command) {
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	for _, c := range cmd.Commands() {
		resetCommandFlags(c)
	}
}

func resetFlagSet(fs *pflag.FlagSet) {
	fs.VisitAll(func(f *pflag.Flag) {
		_ = fs.Set(f.Name, f.DefValue)
	})
}
