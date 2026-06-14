package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/fatih/color"
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
}

func runCommandTest(t *testing.T, tt commandTestCase) {
	t.Helper()
	resetCommandFlags(RootCmd)
	if tt.mockSetup != nil {
		tt.mockSetup(t)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	oldCmdOut := RootCmd.OutOrStdout()
	oldCmdErr := RootCmd.ErrOrStderr()
	oldSilenceUsage := RootCmd.SilenceUsage
	oldSilenceErrors := RootCmd.SilenceErrors
	t.Cleanup(func() {
		RootCmd.SetOut(oldCmdOut)
		RootCmd.SetErr(oldCmdErr)
		RootCmd.SilenceUsage = oldSilenceUsage
		RootCmd.SilenceErrors = oldSilenceErrors
		RootCmd.SetArgs(nil)
		resetCommandFlags(RootCmd)
	})
	RootCmd.SetOut(out)
	RootCmd.SetErr(errOut)
	RootCmd.SilenceUsage = true
	RootCmd.SilenceErrors = true
	RootCmd.SetArgs(tt.args)

	stdout, stderr, err := captureProcessOutput(t, func() error {
		return RootCmd.Execute()
	})
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

	errText := ""
	if err != nil {
		errText = err.Error()
	}
	combined := out.String() + "\n" + errOut.String() + "\n" + stdout + "\n" + stderr + "\n" + errText
	for _, want := range tt.wantContains {
		if !bytes.Contains([]byte(combined), []byte(want)) {
			t.Fatalf("output missing %q\noutput:\n%s", want, combined)
		}
	}
}

func resetCommandFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(resetFlag)
	cmd.PersistentFlags().VisitAll(resetFlag)
	cmd.InheritedFlags().VisitAll(resetFlag)
	for _, child := range cmd.Commands() {
		resetCommandFlags(child)
	}
}

func resetFlag(f *pflag.Flag) {
	_ = f.Value.Set(f.DefValue)
	f.Changed = false
}

func captureProcessOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldColorOutput := color.Output
	oldColorError := color.Error

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr
	color.Output = wOut
	color.Error = wErr

	outCh := make(chan string)
	errCh := make(chan string)
	go func() {
		data, _ := io.ReadAll(rOut)
		outCh <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(rErr)
		errCh <- string(data)
	}()

	fnErr := fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	color.Output = oldColorOutput
	color.Error = oldColorError

	return <-outCh, <-errCh, fnErr
}
