package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// Version and Build are set at build time
var Version = "dev"
var Build = "unknown"

// RootCmd is the root command
var RootCmd = &cobra.Command{
	Use:   "deca",
	Short: "GitHub Release package manager",
	Long: `Deca is a GitHub Release package manager that downloads and manages
binaries from GitHub Releases.

Find more information at: https://github.com/deca-org/deca`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// Execute runs the root command
func Execute() int {
	RootCmd.Version = Version + " (build " + Build + ")"
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

var (
	configPath string
	dryRun     bool
	verbose    bool
)

func init() {
	RootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: ~/.config/deca/deca.toml)")
	RootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	if configPath != "" {
		return configPath
	}
	return defaultConfigPath()
}

// defaultConfigPath returns the default config path
func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return fmt.Sprintf("%s/.config/deca/deca.toml", home)
}

// getContext returns a context with cancellation
func getContext() context.Context {
	return context.Background()
}

// printStatus prints a status message
func printStatus(msg string) {
	if verbose {
		fmt.Println(msg)
	}
}

// getCurrentOS returns the current operating system
func getCurrentOS() string {
	return runtime.GOOS
}

// getCurrentArch returns the current architecture
func getCurrentArch() string {
	return runtime.GOARCH
}
