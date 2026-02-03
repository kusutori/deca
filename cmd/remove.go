package cmd

import (
	"fmt"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/install"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

// RemoveCmd removes a package
var RemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a package from configuration and uninstall",
	Long: `Remove a package from the configuration and uninstall it.

This command removes the specified package from the configuration
file and uninstalls the installed binary or system package.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check if package exists in config
		if _, exists := cfg.Packages[name]; !exists {
			ui.Warning.Printf("Package %s not found in config, checking installed packages...\n", name)
		}

		// Load state to get install type
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		installedPkg, installed := state.GetPackage(name)

		// Remove from config if exists
		if _, exists := cfg.Packages[name]; exists {
			delete(cfg.Packages, name)
			configPath := getConfigPath()
			if err := config.Save(cfg, configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			ui.Success.Printf("Removed %s from config\n", name)
		}

		// Uninstall if installed
		keepInstalled, _ := cmd.Flags().GetBool("keep-installed")
		if !keepInstalled && installed {
			installer := install.NewInstaller(cfg.BinDir)
			err := installer.Uninstall(name, installedPkg.InstallType, installedPkg.SystemPkgName)
			if err != nil {
				ui.Warning.Printf("Warning: failed to uninstall %s: %v\n", name, err)
				ui.Info.Println("You may need to manually remove the package")
			} else {
				switch installedPkg.InstallType {
				case config.InstallTypeSystem:
					ui.Success.Printf("Removed system package %s\n", name)
				case config.InstallTypeAppImage:
					ui.Success.Printf("Removed AppImage %s\n", name)
				default:
					ui.Success.Printf("Removed binary %s\n", name)
				}
			}
		}

		// Remove from state
		if installed {
			state.RemovePackage(name)
			if err := state.SaveState(statePath); err != nil {
				ui.Warning.Printf("Warning: failed to save state: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	RemoveCmd.Flags().BoolP("keep-installed", "k", false, "Keep the installed package, only remove from config")
	RootCmd.AddCommand(RemoveCmd)
}
