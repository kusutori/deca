package ui

import (
	"fmt"
	"strings"

	"github.com/kusutori/deca/internal/errors"
)

// PrintDecaError prints an error with optional suggestion
func PrintDecaError(err error) {
	if err == nil {
		return
	}

	Error.Println("Error:", err.Error())

	// Print suggestion if available
	suggest := errors.GetSuggest(err)
	if suggest != "" {
		Info.Println("Hint:", suggest)
	}
}

// PrintMultipleErrors prints multiple errors with their codes
func PrintMultipleErrors(errs []error) {
	if len(errs) == 0 {
		return
	}

	Error.Printf("%d error(s) occurred:\n", len(errs))
	for i, err := range errs {
		code := errors.GetCode(err)
		if code != "" {
			Error.Printf("  %d. [%s] %s\n", i+1, code, err.Error())
		} else {
			Error.Printf("  %d. %s\n", i+1, err.Error())
		}
		suggest := errors.GetSuggest(err)
		if suggest != "" {
			Info.Printf("     Hint: %s\n", suggest)
		}
	}
}

// FormatErrorForLog formats an error for logging with its code
func FormatErrorForLog(err error) string {
	code := errors.GetCode(err)
	if code != "" {
		return fmt.Sprintf("[%s] %s", code, err.Error())
	}
	return err.Error()
}

// PrintDownloadError prints a download error with helpful message
func PrintDownloadError(assetName string, err error) {
	Error.Printf("Failed to download %s\n", assetName)

	// Check for common issues
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "timeout") {
		Info.Println("Connection timed out. Check your network connection.")
	} else if strings.Contains(errStr, "connection refused") {
		Info.Println("Connection refused. Check your network or proxy settings.")
	} else if strings.Contains(errStr, "404") {
		Info.Println("Asset not found. It may have been removed from the release.")
	} else {
		PrintDecaError(err)
	}
}

// PrintInstallError prints an installation error with helpful message
func PrintInstallError(name string, err error) {
	Error.Printf("Failed to install %s\n", name)

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "permission") {
		Info.Println("Permission denied. Try running with sudo or check file permissions.")
	} else if strings.Contains(errStr, "no such file") {
		Info.Println("File not found. The download may have failed.")
	} else {
		PrintDecaError(err)
	}
}
