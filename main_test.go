package main

import (
	"testing"

	"github.com/kusutori/deca/cmd"
)

func TestMainInvokesCommand(t *testing.T) {
	cmd.RootCmd.SetArgs([]string{"--help"})
	originalExit := exit
	exitCode := -1
	exit = func(code int) { exitCode = code }
	t.Cleanup(func() {
		cmd.RootCmd.SetArgs(nil)
		exit = originalExit
	})
	main()
	if exitCode != 0 {
		t.Fatalf("main exited with %d, want 0", exitCode)
	}
}
