package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
	"github.com/spf13/cobra"
)

// RemoveCmd removes a package
var RemoveCmd = &cobra.Command{
	Use:   "remove <name> ...",
	Short: "Remove a package from configuration and uninstall",
	Long: `Remove a package from the configuration and uninstall it.

This command removes the specified package from the configuration
file and uninstalls the installed binary or system package.

Multiple packages can be removed at once by specifying multiple names.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		keepInstalled, _ := cmd.Flags().GetBool("keep-installed")
		keepDesktop, _ := cmd.Flags().GetBool("keep-desktop")

		for _, name := range args {
			if _, exists := cfg.Packages[name]; !exists {
				ui.Warning.Printf("Package %s not found in config, checking installed packages...\n", name)
			}

			installedPkg, installed := state.GetPackage(name)

			if _, exists := cfg.Packages[name]; exists {
				delete(cfg.Packages, name)
				configPath := getConfigPath()
				if err := config.Save(cfg, configPath); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
				ui.Success.Printf("Removed %s from config\n", name)
			}

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
				if installedPkg.VersionedBinaryPath != "" {
					symlinkPath := filepath.Join(expandPath(cfg.BinDir), name)
					if removeErr := install.UninstallVersioned(symlinkPath, installedPkg.VersionedBinaryPath); removeErr != nil {
						ui.Warning.Printf("Warning: failed to remove versioned binary: %v\n", removeErr)
					}
				}
			}

			desktopPath := config.DesktopEntryPath(name)
			if _, err := os.Stat(desktopPath); err == nil {
				if !keepDesktop {
					if err := os.Remove(desktopPath); err != nil {
						ui.Warning.Printf("Warning: failed to remove desktop entry: %v\n", err)
					} else {
						ui.Success.Printf("Removed desktop entry: %s\n", desktopPath)
					}
				}
			}

			if installed {
				state.RemovePackage(name)
			}
		}

		if err := state.SaveState(statePath); err != nil {
			ui.Warning.Printf("Warning: failed to save state: %v\n", err)
		}

		return nil
	},
}

func init() {
	RemoveCmd.Flags().BoolP("keep-installed", "k", false, "Keep the installed package, only remove from config")
	RemoveCmd.Flags().Bool("keep-desktop", false, "Keep the desktop entry file when removing")
	RootCmd.AddCommand(RemoveCmd)
}
