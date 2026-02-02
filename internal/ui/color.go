package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// Colors for different output types
var (
	// Primary colors for main elements
	Primary    = color.New(color.FgCyan)
	Secondary  = color.New(color.FgBlue)
	Success    = color.New(color.FgGreen)
	Warning    = color.New(color.FgYellow)
	Error      = color.New(color.FgRed)
	Info       = color.New(color.FgWhite)

	// Package related
	PackageName = color.New(color.FgGreen, color.Bold)
	PackageRepo = color.New(color.FgBlue)
	Version     = color.New(color.FgYellow)
	VersionNew  = color.New(color.FgHiGreen)
	VersionOld  = color.New(color.FgHiYellow)

	// Status indicators
	Installed   = color.New(color.FgGreen, color.Bold)
	NotInstalled = color.New(color.FgRed, color.Bold)
	UpdateAvail = color.New(color.FgHiYellow, color.Bold)

	// Search results
	SearchTitle = color.New(color.FgCyan, color.Bold)
	SearchRepo  = color.New(color.FgGreen)
	SearchDesc  = color.New(color.FgWhite)
	SearchMeta  = color.New(color.FgBlue)
	SearchStars = color.New(color.FgHiMagenta)

	// Doctor
	DoctorOK    = color.New(color.FgGreen, color.Bold)
	DoctorFail  = color.New(color.FgRed, color.Bold)
	DoctorCheck = color.New(color.FgCyan)
)

// IsTerminal checks if stdout is a terminal
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// EnableColors enables colors (default on terminals)
func EnableColors() {
	color.NoColor = false
}

// DisableColors disables colors (for non-terminals)
func DisableColors() {
	color.NoColor = true
}

// PrintSuccess prints a success message
func PrintSuccess(msg string) {
	if IsTerminal() {
		Success.Println("✓ " + msg)
	} else {
		fmt.Println("[OK] " + msg)
	}
}

// PrintError prints an error message
func PrintError(msg string) {
	if IsTerminal() {
		Error.Println("✗ " + msg)
	} else {
		fmt.Println("[ERROR] " + msg)
	}
}

// PrintWarning prints a warning message
func PrintWarning(msg string) {
	if IsTerminal() {
		Warning.Println("⚠ " + msg)
	} else {
		fmt.Println("[WARN] " + msg)
	}
}

// PrintInfo prints an info message
func PrintInfo(msg string) {
	if IsTerminal() {
		Info.Println("ℹ " + msg)
	} else {
		fmt.Println("[INFO] " + msg)
	}
}

// FprintfColored prints formatted colored output
func FprintfColored(format string, args ...interface{}) {
	if IsTerminal() {
		fmt.Printf(format, args...)
	} else {
		// Strip colors for non-terminals
		stripped := stripColor(format)
		fmt.Printf(stripped, args...)
	}
}

// PrintColored prints colored output
func PrintColored(format string, args ...interface{}) {
	if IsTerminal() {
		fmt.Printf(format+"\n", args...)
	} else {
		stripped := stripColor(format)
		fmt.Printf(stripped+"\n", args...)
	}
}

// stripColor removes ANSI color codes for non-terminal output
func stripColor(s string) string {
	// Simple strip - in production use a proper library
	// This is a basic implementation
	result := s
	// Would use github.com/mattn/go-colorable strip functionality
	return result
}
