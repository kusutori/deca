package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/install"
	"github.com/spf13/cobra"
)

// ListCmd lists installed packages
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Long: `List installed packages and their versions.

This command shows all packages in the configuration file
along with their installed status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Load state
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to load state: %w", err)
		}

		// Check if binaries exist
		installer := install.NewInstaller(cfg.BinDir)

		fmt.Println("Packages:")
		fmt.Println()

		if len(cfg.Packages) == 0 {
			fmt.Println("  No packages configured")
			fmt.Println()
			fmt.Println("  Add packages with: deca add owner/repo")
			return nil
		}

		for name, pkg := range cfg.Packages {
			installed, exists := state.GetPackage(name)
			status := "[ ]"
			if exists {
				status = "[*]"
			}

			version := installed.Version
			if version == "" {
				version = "(not installed)"
			}

			binaryPath := installer.BinDir + "/" + name
			if runtime.GOOS == "windows" {
				binaryPath += ".exe"
			}

			if _, err := os.Stat(binaryPath); err == nil {
				status = "[*]"
			} else if exists {
				status = "[?]"
			}

			fmt.Printf("  %s %s -> %s\n", status, name, pkg.Repo)
			fmt.Printf("      Version: %s\n", version)

			if verbose {
				if pkg.Asset != "" {
					fmt.Printf("      Asset: %s\n", pkg.Asset)
				}
				if pkg.OS != "" || pkg.Arch != "" {
					osArch := pkg.OS
					if pkg.Arch != "" {
						if osArch != "" {
							osArch += "/"
						}
						osArch += pkg.Arch
					}
					fmt.Printf("      Platform: %s\n", osArch)
				}
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(ListCmd)
}
