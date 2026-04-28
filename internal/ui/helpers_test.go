package ui

import (
	"errors"
	"testing"

	decaerr "github.com/kusutori/deca/internal/errors"
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
