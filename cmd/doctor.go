package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/install"
	"github.com/spf13/cobra"
)

// DoctorCmd runs health checks
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check installation health",
	Long: `Run health checks on the deca installation.

This command verifies that:
- Configuration file exists and is valid
- Binary directory exists and is in PATH
- GitHub API is accessible
- All configured packages are installed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		issues := make([]string, 0)
		checks := make([]string, 0)

		fmt.Println("Deca Doctor")
		fmt.Println(strings.Repeat("=", 40))
		fmt.Println()

		// Check Go version
		fmt.Printf("Go Version: %s\n", runtime.Version())
		checks = append(checks, fmt.Sprintf("Go version: %s", runtime.Version()))

		// Check config file
		fmt.Print("Config file: ")
		configPath := getConfigPath()
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Println("not found")
			issues = append(issues, fmt.Sprintf("Config file not found: %s", configPath))
		} else {
			cfg, err := config.Load(configPath)
			if err != nil {
				fmt.Println("error loading")
				issues = append(issues, fmt.Sprintf("Failed to load config: %v", err))
			} else {
				fmt.Println("OK")
				checks = append(checks, "Config file valid")

				// Check bin directory
				fmt.Print("Bin directory: ")
				binDir := cfg.BinDir
				if binDir == "" {
					binDir = config.DefaultBinDir()
				}

				if _, err := os.Stat(binDir); os.IsNotExist(err) {
					fmt.Println("not found")
					issues = append(issues, fmt.Sprintf("Bin directory not found: %s", binDir))
				} else {
					fmt.Println("OK")
					checks = append(checks, "Bin directory exists")

					// Check if in PATH
					installer := install.NewInstaller(binDir)
					fmt.Print("PATH entry: ")
					if installer.BinDirInPATH() {
						fmt.Println("OK")
						checks = append(checks, "Bin directory in PATH")
					} else {
						fmt.Println("missing")
						issues = append(issues, "Bin directory not in PATH")
					}
				}

				// Check state file
				fmt.Print("State file: ")
				statePath := config.DefaultStatePath()
				if _, err := os.Stat(statePath); os.IsNotExist(err) {
					fmt.Println("not found")
					issues = append(issues, fmt.Sprintf("State file not found: %s", statePath))
				} else {
					state, err := config.LoadState(statePath)
					if err != nil {
						fmt.Println("error loading")
						issues = append(issues, fmt.Sprintf("Failed to load state: %v", err))
					} else {
						fmt.Println("OK")
						fmt.Printf("  Installed packages: %d\n", len(state.Packages))
						checks = append(checks, fmt.Sprintf("State file valid (%d packages)", len(state.Packages)))
					}
				}
			}
		}

		// Check GitHub API
		fmt.Print("GitHub API: ")
		ghClient := github.NewClient()
		_, err := ghClient.GetLatestRelease(context.Background(), "github", "hub")
		if err != nil {
			fmt.Println("error")
			issues = append(issues, fmt.Sprintf("GitHub API access failed: %v", err))
		} else {
			fmt.Println("OK")
			checks = append(checks, "GitHub API accessible")
		}

		fmt.Println()
		fmt.Println(strings.Repeat("=", 40))
		fmt.Println()

		if len(issues) > 0 {
			fmt.Println("Issues found:")
			for _, issue := range issues {
				fmt.Printf("  - %s\n", issue)
			}
			fmt.Println()
		} else {
			fmt.Println("All checks passed!")
		}

		if len(checks) > 0 {
			fmt.Println("Passed checks:")
			for _, check := range checks {
				fmt.Printf("  [OK] %s\n", check)
			}
		}

		if len(issues) > 0 {
			return fmt.Errorf("%d issues found", len(issues))
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(DoctorCmd)
}
