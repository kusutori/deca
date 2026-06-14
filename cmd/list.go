package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"
	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
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

		ui.Primary.Println("Packages:")
		fmt.Println()

		if len(cfg.Packages) == 0 {
			ui.Warning.Println("  No packages configured")
			fmt.Println()
			ui.Info.Println("  Add packages with: deca add owner/repo")
			return nil
		}

		for name, pkg := range cfg.Packages {
			installed, exists := state.GetPackage(name)

			// Determine status indicator and color
			var statusStr string
			statusStr = "[ ]"

			binaryPath := installer.BinDir + "/" + name
			if runtime.GOOS == "windows" {
				binaryPath += ".exe"
			}
			if installed.ExposedPath != "" {
				binaryPath = installed.ExposedPath
			}

			var statusColor interface{}
			if _, err := os.Stat(binaryPath); err == nil {
				// Binary exists
				if exists {
					statusStr = "[✓]"
					statusColor = ui.Installed
				} else {
					statusStr = "[?]"
					statusColor = ui.Warning
				}
			} else if exists {
				// In state but binary missing
				statusStr = "[!]"
				statusColor = ui.Warning
			} else {
				// Not installed
				statusStr = "[ ]"
				statusColor = ui.NotInstalled
			}

			// Print with colors
			switch c := statusColor.(type) {
			case *color.Color:
				c.Print("  " + statusStr + " ")
			}
			ui.PackageName.Println(name)
			ui.PackageRepo.Printf("    -> %s\n", pkg.Repo)

			version := installed.Version
			if version == "" {
				ui.SearchMeta.Println("    Version: (not installed)")
			} else {
				ui.SearchMeta.Printf("    Version: %s\n", version)
			}

			if verbose {
				if pkg.Asset != "" {
					ui.SearchMeta.Printf("    Asset: %s\n", pkg.Asset)
				}
				if pkg.OS != "" || pkg.Arch != "" {
					osArch := pkg.OS
					if pkg.Arch != "" {
						if osArch != "" {
							osArch += "/"
						}
						osArch += pkg.Arch
					}
					ui.SearchMeta.Printf("    Platform: %s\n", osArch)
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
