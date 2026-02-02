package errors

import (
	"errors"
	"fmt"
)

// Error codes for different error categories
type Code string

const (
	// Config errors (1xx)
	ErrCodeConfigNotFound Code = "CONFIG_NOT_FOUND"
	ErrCodeConfigInvalid  Code = "CONFIG_INVALID"
	ErrCodeConfigSave     Code = "CONFIG_SAVE_FAILED"

	// Network errors (2xx)
	ErrCodeNetwork          Code = "NETWORK_ERROR"
	ErrCodeGitHubAPI        Code = "GITHUB_API_ERROR"
	ErrCodeDownloadFailed   Code = "DOWNLOAD_FAILED"
	ErrCodeRateLimited      Code = "RATE_LIMITED"

	// Installation errors (3xx)
	ErrCodeAssetNotFound    Code = "ASSET_NOT_FOUND"
	ErrCodeInstallFailed    Code = "INSTALL_FAILED"
	ErrCodeExtractFailed    Code = "EXTRACT_FAILED"
	ErrCodeUnsupportedOS    Code = "UNSUPPORTED_OS"
	ErrCodeUnsupportedArch  Code = "UNSUPPORTED_ARCH"

	// System errors (4xx)
	ErrCodePermissionDenied Code = "PERMISSION_DENIED"
	ErrCodeSudoRequired     Code = "SUDO_REQUIRED"
	ErrCodeSudoFailed       Code = "SUDO_FAILED"
	ErrCodeNotFound         Code = "NOT_FOUND"

	// Package errors (5xx)
	ErrCodePackageNotFound  Code = "PACKAGE_NOT_FOUND"
	ErrCodePackageExists    Code = "PACKAGE_EXISTS"
	ErrCodeUpdateFailed     Code = "UPDATE_FAILED"
)

// DecaError is the main error type for Deca
type DecaError struct {
	Code     Code
	Message  string
	Suggest  string
	Parent   error
}

// Error returns the error message
func (e *DecaError) Error() string {
	if e.Parent != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Parent.Error())
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *DecaError) Unwrap() error {
	return e.Parent
}

// WithParent wraps an existing error
func (e *DecaError) WithParent(parent error) *DecaError {
	e.Parent = parent
	return e
}

// WithSuggest adds a suggestion to the error
func (e *DecaError) WithSuggest(suggest string) *DecaError {
	e.Suggest = suggest
	return e
}

// NewDecaError creates a new DecaError
func NewDecaError(code Code, message string) *DecaError {
	return &DecaError{
		Code:    code,
		Message: message,
	}
}

// NewConfigNotFoundError creates an error for missing config file
func NewConfigNotFoundError(path string) *DecaError {
	return NewDecaError(ErrCodeConfigNotFound, fmt.Sprintf("config file not found: %s", path)).
		WithSuggest("Run 'deca init' to create a default config")
}

// NewConfigInvalidError creates an error for invalid config
func NewConfigInvalidError(reason string) *DecaError {
	return NewDecaError(ErrCodeConfigInvalid, fmt.Sprintf("invalid config: %s", reason)).
		WithSuggest("Check your config file syntax")
}

// NewNetworkError creates a network error
func NewNetworkError(err error) *DecaError {
	return NewDecaError(ErrCodeNetwork, "network error").
		WithParent(err).
		WithSuggest("Check your internet connection")
}

// NewGitHubAPIError creates a GitHub API error
func NewGitHubAPIError(err error) *DecaError {
	return NewDecaError(ErrCodeGitHubAPI, "GitHub API error").
		WithParent(err).
		WithSuggest("Check your GitHub token or try again later")
}

// NewDownloadError creates a download error
func NewDownloadError(assetName string, err error) *DecaError {
	return NewDecaError(ErrCodeDownloadFailed, fmt.Sprintf("failed to download %s", assetName)).
		WithParent(err)
}

// NewAssetNotFoundError creates an error when no asset matches
func NewAssetNotFoundError(os, arch, pattern string) *DecaError {
	msg := fmt.Sprintf("no matching asset found for os=%s arch=%s", os, arch)
	if pattern != "" {
		msg = fmt.Sprintf("no matching asset found for pattern: %s", pattern)
	}
	return NewDecaError(ErrCodeAssetNotFound, msg).
		WithSuggest("Use 'deca add <repo> --interactive' to select an asset manually")
}

// NewInstallError creates an installation error
func NewInstallError(name string, err error) *DecaError {
	return NewDecaError(ErrCodeInstallFailed, fmt.Sprintf("failed to install %s", name)).
		WithParent(err)
}

// NewExtractError creates an extraction error
func NewExtractError(format string, err error) *DecaError {
	return NewDecaError(ErrCodeExtractFailed, fmt.Sprintf("failed to extract %s", format)).
		WithParent(err)
}

// NewPackageNotFoundError creates an error for missing package
func NewPackageNotFoundError(name string) *DecaError {
	return NewDecaError(ErrCodePackageNotFound, fmt.Sprintf("package %s not found", name)).
		WithSuggest("Use 'deca add <owner/repo>' to add a new package")
}

// NewSudoRequiredError creates an error when sudo is needed
func NewSudoRequiredError(operation string) *DecaError {
	return NewDecaError(ErrCodeSudoRequired, fmt.Sprintf("%s requires sudo privileges", operation)).
		WithSuggest("Make sure sudo is configured or run with appropriate permissions")
}

// NewSudoFailedError creates an error when sudo fails
func NewSudoFailedError(err error) *DecaError {
	return NewDecaError(ErrCodeSudoFailed, "sudo operation failed").
		WithParent(err).
		WithSuggest("Check your sudo configuration and try again")
}

// NewPermissionDeniedError creates a permission denied error
func NewPermissionDeniedError(operation string) *DecaError {
	return NewDecaError(ErrCodePermissionDenied, fmt.Sprintf("permission denied: %s", operation))
}

// NewRateLimitedError creates a rate limit error
func NewRateLimitedError() *DecaError {
	return NewDecaError(ErrCodeRateLimited, "GitHub API rate limit exceeded").
		WithSuggest("Set GITHUB_TOKEN environment variable to increase rate limit")
}

// Is checks if the error matches a specific code
func Is(err error, code Code) bool {
	var decaErr *DecaError
	if errors.As(err, &decaErr) {
		return decaErr.Code == code
	}
	return false
}

// GetCode returns the error code
func GetCode(err error) Code {
	var decaErr *DecaError
	if errors.As(err, &decaErr) {
		return decaErr.Code
	}
	return ""
}

// GetSuggest returns the suggestion for the error
func GetSuggest(err error) string {
	var decaErr *DecaError
	if errors.As(err, &decaErr) {
		return decaErr.Suggest
	}
	return ""
}
