package cmd

import (
	"fmt"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/install"
	"github.com/spf13/cobra"
)

// RemoveCmd removes a package
var RemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a package from the configuration",
	Long: `Remove a package from the configuration.

This command removes the specified package from the configuration
file. It does not remove the installed binary.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check if package exists
		if _, exists := cfg.Packages[name]; !exists {
			return fmt.Errorf("package %s not found in config", name)
		}

		// Remove from config
		delete(cfg.Packages, name)

		// Save config
		configPath := getConfigPath()
		if err := config.Save(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Removed %s from config\n", name)

		// Optionally remove binary
		removeBinary, _ := cmd.Flags().GetBool("remove-binary")
		if removeBinary {
			installer := install.NewInstaller(cfg.BinDir)
			if err := installer.Uninstall(name); err != nil {
				fmt.Printf("Warning: failed to remove binary: %v\n", err)
			} else {
				fmt.Printf("Removed binary %s\n", name)
			}
		}

		return nil
	},
}

func init() {
	RemoveCmd.Flags().BoolP("remove-binary", "r", false, "Also remove the installed binary")
	RootCmd.AddCommand(RemoveCmd)
}
