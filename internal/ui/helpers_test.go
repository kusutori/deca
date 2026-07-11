package ui

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	decaerr "github.com/kusutori/deca/internal/errors"
	"github.com/kusutori/deca/internal/github"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
	}
	for _, tt := range tests {
		if got := formatSize(tt.size); got != tt.want {
			t.Fatalf("formatSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate short string = %q", got)
	}
	if got := truncate("abcdefghijklmnopqrstuvwxyz", 10); got != "abcdefg..." {
		t.Fatalf("truncate long string = %q", got)
	}
}

func TestFormatErrorForLog(t *testing.T) {
	errWithCode := decaerr.NewConfigNotFoundError("/tmp/deca.toml")
	got := FormatErrorForLog(errWithCode)
	if got == errWithCode.Error() {
		t.Fatalf("expected code prefix in log message, got %q", got)
	}

	plain := errors.New("plain")
	if got := FormatErrorForLog(plain); got != "plain" {
		t.Fatalf("plain error should be unchanged, got %q", got)
	}
}

func TestColorOutputHelpers(t *testing.T) {
	out := captureUIOutput(t, func() {
		DisableColors()
		PrintSuccess("done")
		PrintError("bad")
		PrintWarning("careful")
		PrintInfo("note")
		FprintfColored("plain %s", "text")
		PrintColored("line %d", 1)
		EnableColors()
	})
	for _, want := range []string{"[OK] done", "[ERROR] bad", "[WARN] careful", "[INFO] note", "plain text", "line 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if got := stripColor("abc"); got != "abc" {
		t.Fatalf("stripColor returned %q", got)
	}
}

func TestColorOutputHelpersTerminal(t *testing.T) {
	restoreTerminal := SetTerminalDetectorForTesting(func() bool { return true })
	t.Cleanup(restoreTerminal)
	out := captureUIOutput(t, func() {
		DisableColors()
		PrintSuccess("done")
		PrintError("bad")
		PrintWarning("careful")
		PrintInfo("note")
		EnableColors()
	})
	for _, want := range []string{"✓ done", "✗ bad", "⚠ careful", "ℹ note"} {
		if !strings.Contains(out, want) {
			t.Fatalf("terminal output missing %q:\n%s", want, out)
		}
	}
}

func TestErrorPrinters(t *testing.T) {
	out := captureUIOutput(t, func() {
		PrintDecaError(decaerr.NewConfigNotFoundError("missing.toml"))
		PrintDecaError(nil)
		PrintMultipleErrors([]error{
			decaerr.NewAssetNotFoundError("linux", "amd64", "*.tar.gz"),
			errors.New("plain"),
		})
		PrintMultipleErrors(nil)
		PrintDownloadError("asset.zip", errors.New("timeout while reading"))
		PrintDownloadError("asset.zip", errors.New("connection refused"))
		PrintDownloadError("asset.zip", errors.New("HTTP 404"))
		PrintDownloadError("asset.zip", errors.New("other failure"))
		PrintInstallError("tool", errors.New("permission denied"))
		PrintInstallError("tool", errors.New("no such file"))
		PrintInstallError("tool", errors.New("other failure"))
	})
	for _, want := range []string{
		"Error:",
		"Hint:",
		"2 error(s) occurred",
		"Connection timed out",
		"Connection refused",
		"Asset not found",
		"Permission denied",
		"File not found",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAssetSelectionNonTerminal(t *testing.T) {
	assets := []github.AssetInfo{{Name: "one"}, {Name: "two"}}
	if got := InteractiveSelectAssets(assets, "owner/repo"); got == nil || got.Name != "one" {
		t.Fatalf("expected first asset in non-terminal mode, got %+v", got)
	}
	if got := InteractiveSelectAssets(nil, "owner/repo"); got != nil {
		t.Fatalf("expected nil for empty asset list, got %+v", got)
	}
	out := captureUIOutput(t, func() {
		if idx := PrintAssetTable(nil, "owner/repo"); idx != -1 {
			t.Fatalf("expected -1 for empty asset table, got %d", idx)
		}
	})
	if !strings.Contains(out, "No assets available") {
		t.Fatalf("expected no assets message, got %s", out)
	}
}

func TestPrintAssetTableSelections(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	if _, err := w.WriteString("2\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	assets := []github.AssetInfo{
		{Name: "one", Size: 512},
		{Name: strings.Repeat("two", 30), Size: 2048},
	}
	out := captureUIOutput(t, func() {
		if idx := PrintAssetTable(assets, "owner/repo"); idx != 1 {
			t.Fatalf("expected selected index 1, got %d", idx)
		}
	})
	if !strings.Contains(out, "Available assets") || !strings.Contains(out, "2.0 KB") {
		t.Fatalf("unexpected asset table output:\n%s", out)
	}
}

func captureUIOutput(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	oldColorOutput := color.Output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	color.Output = w

	ch := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		ch <- string(data)
	}()

	fn()
	_ = w.Close()
	os.Stdout = oldStdout
	color.Output = oldColorOutput
	return <-ch
}
