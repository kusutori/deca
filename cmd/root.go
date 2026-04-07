package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/deca-org/deca/internal/ui"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version, Build, and BuildTime are set at build time
var Version = "dev"
var Build = "unknown"
var BuildTime = "unknown"

// Color definitions for help output
var (
	titleColor  = color.New(color.FgCyan, color.Bold)
	headerColor = color.New(color.FgYellow, color.Bold)
	cmdColor    = color.New(color.FgGreen)
	flagColor   = color.New(color.FgMagenta)
	descColor   = color.New(color.FgWhite)
	urlColor    = color.New(color.FgBlue, color.Underline)
)

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
	RootCmd.Version = Version + " (build " + Build + ", " + BuildTime + ")"
	if err := RootCmd.Execute(); err != nil {
		ui.PrintDecaError(err)
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

	// Set custom help function
	RootCmd.SetHelpFunc(coloredHelpFunc)
}

// coloredHelpFunc provides colorful help output
func coloredHelpFunc(cmd *cobra.Command, args []string) {
	// Title
	if cmd.Long != "" {
		lines := strings.Split(cmd.Long, "\n")
		for _, line := range lines {
			if strings.Contains(line, "https://") {
				urlColor.Println(line)
			} else {
				titleColor.Println(line)
			}
		}
		fmt.Println()
	} else if cmd.Short != "" {
		titleColor.Println(cmd.Short)
		fmt.Println()
	}

	// Usage
	headerColor.Println("Usage:")
	fmt.Printf("  %s\n\n", cmd.UseLine())

	// Available Commands
	if cmd.HasAvailableSubCommands() {
		headerColor.Println("Available Commands:")
		for _, subcmd := range cmd.Commands() {
			if subcmd.IsAvailableCommand() {
				cmdColor.Printf("  %-12s", subcmd.Name())
				descColor.Printf("  %s\n", subcmd.Short)
			}
		}
		fmt.Println()
	}

	// Flags
	if cmd.HasAvailableFlags() {
		headerColor.Println("Flags:")
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			shorthand := ""
			if f.Shorthand != "" {
				shorthand = fmt.Sprintf("-%s, ", f.Shorthand)
			}
			flagColor.Printf("  %s--%s", shorthand, f.Name)
			if f.Value.Type() != "bool" {
				fmt.Printf(" %s", f.Value.Type())
			}
			descColor.Printf("\n        %s", f.Usage)
			if f.DefValue != "" && f.DefValue != "false" {
				fmt.Printf(" (default: %s)", f.DefValue)
			}
			fmt.Println()
		})
		fmt.Println()
	}

	// Footer
	descColor.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.Name())
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
