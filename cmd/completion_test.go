package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carapace-sh/carapace"
)

func TestGenerateCompletionNushell(t *testing.T) {
	var buf bytes.Buffer
	if err := generateCompletion("nushell", &buf); err != nil {
		t.Fatalf("generateCompletion failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "nu-complete deca") {
		t.Fatalf("expected nushell snippet to define nu-complete function, got:\n%s", out)
	}
	if !strings.Contains(out, "_carapace nushell") {
		t.Fatalf("expected nushell snippet to invoke _carapace, got:\n%s", out)
	}
	if !strings.Contains(out, "from json") {
		t.Fatalf("expected nushell snippet to parse json output, got:\n%s", out)
	}
	if !strings.Contains(out, "length) == 1") {
		t.Fatalf("expected nushell snippet to normalize spans, got:\n%s", out)
	}
	if !strings.Contains(out, `extern "deca"`) {
		t.Fatalf("expected nushell snippet to define extern, got:\n%s", out)
	}
	if !strings.Contains(out, `...args: string@"nu-complete deca"`) {
		t.Fatalf("expected nushell snippet to bind completion to args, got:\n%s", out)
	}
}

func TestGenerateCompletionUnsupportedShell(t *testing.T) {
	var buf bytes.Buffer
	if err := generateCompletion("unknown-shell", &buf); err == nil {
		t.Fatal("expected unsupported shell error, got nil")
	}
}

func TestCarapaceConfig(t *testing.T) {
	carapace.Test(t)
}
