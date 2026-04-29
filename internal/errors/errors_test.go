package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestNewDecaError(t *testing.T) {
	err := NewDecaError(ErrCodeConfigNotFound, "config not found")
	if err.Code != ErrCodeConfigNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeConfigNotFound, err.Code)
	}
	if err.Message != "config not found" {
		t.Errorf("expected message 'config not found', got %q", err.Message)
	}
}

func TestDecaErrorError(t *testing.T) {
	parent := errors.New("original error")
	err := NewDecaError(ErrCodeNetwork, "network error").WithParent(parent)

	expected := "network error: original error"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDecaErrorUnwrap(t *testing.T) {
	parent := errors.New("original error")
	err := NewDecaError(ErrCodeNetwork, "network error").WithParent(parent)

	if err.Unwrap() != parent {
		t.Error("Unwrap() should return the parent error")
	}
}

func TestDecaErrorWithSuggest(t *testing.T) {
	err := NewDecaError(ErrCodeConfigNotFound, "config not found").
		WithSuggest("Run 'deca init'")

	if err.Suggest != "Run 'deca init'" {
		t.Errorf("expected suggest 'Run deca init', got %q", err.Suggest)
	}
}

func TestNewConfigNotFoundError(t *testing.T) {
	err := NewConfigNotFoundError("/path/to/config.toml")

	if err.Code != ErrCodeConfigNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeConfigNotFound, err.Code)
	}
	if err.Suggest != "Run 'deca init' to create a default config" {
		t.Errorf("unexpected suggest: %s", err.Suggest)
	}
}

func TestNewAssetNotFoundError(t *testing.T) {
	err := NewAssetNotFoundError("linux", "amd64", "")
	if err.Code != ErrCodeAssetNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeAssetNotFound, err.Code)
	}

	err2 := NewAssetNotFoundError("", "", "*.deb")
	if err2.Code != ErrCodeAssetNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeAssetNotFound, err2.Code)
	}
}

func TestIs(t *testing.T) {
	err := NewDecaError(ErrCodeConfigNotFound, "config not found")

	if !Is(err, ErrCodeConfigNotFound) {
		t.Error("Is() should return true for matching code")
	}

	if Is(err, ErrCodeNetwork) {
		t.Error("Is() should return false for non-matching code")
	}

	// Test with wrapped error
	wrapped := &DecaError{
		Code:    ErrCodeConfigNotFound,
		Message: "config not found",
	}
	if !Is(wrapped, ErrCodeConfigNotFound) {
		t.Error("Is() should work with direct DecaError")
	}
}

func TestGetCode(t *testing.T) {
	err := NewDecaError(ErrCodeConfigNotFound, "config not found")
	if GetCode(err) != ErrCodeConfigNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeConfigNotFound, GetCode(err))
	}

	regularErr := errors.New("regular error")
	if GetCode(regularErr) != "" {
		t.Errorf("expected empty code for regular error, got %s", GetCode(regularErr))
	}
}

func TestGetSuggest(t *testing.T) {
	err := NewDecaError(ErrCodeConfigNotFound, "config not found").
		WithSuggest("Run 'deca init'")

	if GetSuggest(err) != "Run 'deca init'" {
		t.Errorf("expected suggest 'Run deca init', got %s", GetSuggest(err))
	}

	regularErr := errors.New("regular error")
	if GetSuggest(regularErr) != "" {
		t.Errorf("expected empty suggest for regular error, got %s", GetSuggest(regularErr))
	}
}

func TestErrorTypes(t *testing.T) {
	// Test all error types are defined
	tests := []struct {
		name string
		code Code
	}{
		{"ConfigNotFound", ErrCodeConfigNotFound},
		{"ConfigInvalid", ErrCodeConfigInvalid},
		{"ConfigSave", ErrCodeConfigSave},
		{"Network", ErrCodeNetwork},
		{"GitHubAPI", ErrCodeGitHubAPI},
		{"DownloadFailed", ErrCodeDownloadFailed},
		{"RateLimited", ErrCodeRateLimited},
		{"AssetNotFound", ErrCodeAssetNotFound},
		{"InstallFailed", ErrCodeInstallFailed},
		{"ExtractFailed", ErrCodeExtractFailed},
		{"UnsupportedOS", ErrCodeUnsupportedOS},
		{"UnsupportedArch", ErrCodeUnsupportedArch},
		{"PermissionDenied", ErrCodePermissionDenied},
		{"SudoRequired", ErrCodeSudoRequired},
		{"SudoFailed", ErrCodeSudoFailed},
		{"NotFound", ErrCodeNotFound},
		{"PackageNotFound", ErrCodePackageNotFound},
		{"PackageExists", ErrCodePackageExists},
		{"UpdateFailed", ErrCodeUpdateFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code == "" {
				t.Errorf("error code %s is empty", tt.name)
			}
		})
	}
}

func TestDecaErrorErrorWithoutParent(t *testing.T) {
	err := NewDecaError(ErrCodeNotFound, "not found")
	if got := err.Error(); got != "not found" {
		t.Fatalf("expected plain message, got %q", got)
	}
}

func TestFactoryFunctions(t *testing.T) {
	base := errors.New("boom")
	tests := []struct {
		name            string
		err             *DecaError
		wantCode        Code
		wantMsgContains string
		wantSuggest     string
		wantParent      bool
	}{
		{
			name:            "config invalid",
			err:             NewConfigInvalidError("bad field"),
			wantCode:        ErrCodeConfigInvalid,
			wantMsgContains: "invalid config: bad field",
			wantSuggest:     "Check your config file syntax",
		},
		{
			name:            "network",
			err:             NewNetworkError(base),
			wantCode:        ErrCodeNetwork,
			wantMsgContains: "network error",
			wantSuggest:     "Check your internet connection",
			wantParent:      true,
		},
		{
			name:            "github",
			err:             NewGitHubAPIError(base),
			wantCode:        ErrCodeGitHubAPI,
			wantMsgContains: "GitHub API error",
			wantSuggest:     "Check your GitHub token or try again later",
			wantParent:      true,
		},
		{
			name:            "download",
			err:             NewDownloadError("tool.tar.gz", base),
			wantCode:        ErrCodeDownloadFailed,
			wantMsgContains: "failed to download tool.tar.gz",
			wantParent:      true,
		},
		{
			name:            "install",
			err:             NewInstallError("tool", base),
			wantCode:        ErrCodeInstallFailed,
			wantMsgContains: "failed to install tool",
			wantParent:      true,
		},
		{
			name:            "extract",
			err:             NewExtractError("tar.gz", base),
			wantCode:        ErrCodeExtractFailed,
			wantMsgContains: "failed to extract tar.gz",
			wantParent:      true,
		},
		{
			name:            "package missing",
			err:             NewPackageNotFoundError("tool"),
			wantCode:        ErrCodePackageNotFound,
			wantMsgContains: "package tool not found",
			wantSuggest:     "Use 'deca add <owner/repo>' to add a new package",
		},
		{
			name:            "sudo required",
			err:             NewSudoRequiredError("install"),
			wantCode:        ErrCodeSudoRequired,
			wantMsgContains: "install requires sudo privileges",
			wantSuggest:     "Make sure sudo is configured or run with appropriate permissions",
		},
		{
			name:            "sudo failed",
			err:             NewSudoFailedError(base),
			wantCode:        ErrCodeSudoFailed,
			wantMsgContains: "sudo operation failed",
			wantSuggest:     "Check your sudo configuration and try again",
			wantParent:      true,
		},
		{
			name:            "permission denied",
			err:             NewPermissionDeniedError("write file"),
			wantCode:        ErrCodePermissionDenied,
			wantMsgContains: "permission denied: write file",
		},
		{
			name:            "rate limited",
			err:             NewRateLimitedError(),
			wantCode:        ErrCodeRateLimited,
			wantMsgContains: "GitHub API rate limit exceeded",
			wantSuggest:     "Set GITHUB_TOKEN environment variable to increase rate limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Fatalf("expected code %s, got %s", tt.wantCode, tt.err.Code)
			}
			if !strings.Contains(tt.err.Message, tt.wantMsgContains) {
				t.Fatalf("expected message %q to contain %q", tt.err.Message, tt.wantMsgContains)
			}
			if tt.wantSuggest != "" && tt.err.Suggest != tt.wantSuggest {
				t.Fatalf("expected suggest %q, got %q", tt.wantSuggest, tt.err.Suggest)
			}
			if tt.wantParent && !errors.Is(tt.err, base) {
				t.Fatalf("expected wrapped parent error")
			}
		})
	}
}
