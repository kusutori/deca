package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenNushellCompletion(t *testing.T) {
	var buf bytes.Buffer
	if err := genNushellCompletion(&buf, RootCmd); err != nil {
		t.Fatalf("genNushellCompletion failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "nu-complete deca") {
		t.Fatalf("expected nu-complete function, got:\n%s", out)
	}
	if !strings.Contains(out, "__complete") {
		t.Fatalf("expected __complete usage, got:\n%s", out)
	}
}
