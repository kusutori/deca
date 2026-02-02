package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

// ConfigCmd is the parent command for config operations
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage the deca configuration file.

Subcommands:
  edit   Open config in editor (default)
  show   Display current configuration
  diff   Show changes since last apply
  path   Show config file path`,
}

// ConfigEditCmd opens the configuration file in editor
var ConfigEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration in editor",
	Long: `Open the configuration file in your default editor.

This command detects your editor from the EDITOR or VISUAL
environment variables and opens the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()

		// Check if config exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			ui.Warning.Printf("Config file not found: %s\n", configPath)
			ui.Info.Println("Run 'deca init' to create a default config.")
			return nil
		}

		// Get editor
		editor := getEditor()
		if editor == "" {
			ui.Warning.Println("No editor found (EDITOR not set)")
			ui.Info.Printf("Config file: %s\n", configPath)
			ui.Info.Println("Set EDITOR environment variable or open manually.")
			return nil
		}

		ui.Info.Printf("Opening: %s\n", configPath)

		// Open editor
		var err error
		switch runtime.GOOS {
		case "windows":
			err = exec.Command("cmd", "/c", "start", editor, configPath).Start()
		case "darwin":
			err = exec.Command("open", editor, configPath).Start()
		default:
			// Linux/Unix - use the editor directly
			editorArgs := strings.Split(editor, " ")
			if len(editorArgs) > 1 {
				err = exec.Command(editorArgs[0], append(editorArgs[1:], configPath)...).Start()
			} else {
				err = exec.Command(editor, configPath).Start()
			}
		}

		if err != nil {
			return fmt.Errorf("failed to open editor: %w", err)
		}

		return nil
	},
}

// ConfigShowCmd shows the current configuration
var ConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration file content with installed status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()

		// Check if config exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			ui.Warning.Printf("Config file not found: %s\n", configPath)
			ui.Info.Println("Run 'deca init' to create a default config.")
			return nil
		}

		// Load config
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Load state
		statePath := config.DefaultStatePath()
		state, _ := config.LoadState(statePath)

		ui.Primary.Println("Configuration:")
		ui.SearchMeta.Printf("  Config File: %s\n", configPath)
		ui.SearchMeta.Printf("  Bin Directory: %s\n", cfg.BinDir)
		ui.SearchMeta.Printf("  OS: %s\n", cfg.OS)
		ui.SearchMeta.Printf("  Arch: %s\n", cfg.Arch)
		fmt.Println()

		if len(cfg.Packages) > 0 {
			ui.Primary.Println("Packages:")
			for name, pkg := range cfg.Packages {
				// Get installed state
				installed, exists := state.GetPackage(name)

				ui.PackageName.Printf("  %s\n", name)
				ui.SearchMeta.Printf("    -> %s\n", pkg.Repo)

				// Show installed version if exists
				if exists {
					if installed.Version != "" {
						ui.SearchMeta.Printf("    Installed: %s\n", installed.Version)
					}
					if installed.InstalledAt.IsZero() == false {
						ui.SearchMeta.Printf("    At: %s\n", installed.InstalledAt.Format("2006-01-02"))
					}
				} else {
					ui.Warning.Printf("    [Not installed]\n")
				}
			}
		} else {
			ui.Warning.Println("  No packages configured")
			ui.Info.Println("  Add packages with: deca add owner/repo")
		}

		return nil
	},
}

// ConfigPathCmd shows the config file path
var ConfigPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Long:  `Display the path to the configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()
		fmt.Println(configPath)
		return nil
	},
}

// ConfigDiffCmd shows the difference between config and state
var ConfigDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show changes between config and installed packages",
	Long: `Show the differences between your configuration file and
what is currently installed. This helps identify packages that:
  - Are in config but not installed
  - Are installed but not in config
  - Have updates available`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()

		// Check if config exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			ui.Warning.Printf("Config file not found: %s\n", configPath)
			ui.Info.Println("Run 'deca init' to create a default config.")
			return nil
		}

		// Load config
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Load state
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			// State file might not exist
			state = &config.State{Packages: make(map[string]config.InstalledPackage)}
		}

		ui.Primary.Println("Configuration Changes:")
		fmt.Println()

		// Track changes
		var toInstall []string
		var toRemove []string

		// Check packages in config vs state
		for name := range cfg.Packages {
			_, exists := state.GetPackage(name)
			if !exists {
				toInstall = append(toInstall, name)
			}
		}

		// Check packages in state but not in config
		for name := range state.Packages {
			if _, exists := cfg.Packages[name]; !exists {
				toRemove = append(toRemove, name)
			}
		}

		// Print summary
		if len(toInstall) > 0 {
			ui.Warning.Println("Packages to install:")
			for _, name := range toInstall {
				pkg := cfg.Packages[name]
				ui.PackageName.Printf("  + %s\n", name)
				ui.SearchMeta.Printf("    -> %s\n", pkg.Repo)
			}
			fmt.Println()
		}

		if len(toRemove) > 0 {
			ui.Warning.Println("Installed packages not in config (will be removed on next apply):")
			for _, name := range toRemove {
				installed, _ := state.GetPackage(name)
				ui.PackageName.Printf("  - %s\n", name)
				ui.SearchMeta.Printf("    Installed: %s\n", installed.Version)
			}
			fmt.Println()
		}

		// Summary
		totalChanges := len(toInstall) + len(toRemove)
		if totalChanges == 0 {
			ui.Success.Println("No changes - config matches installed packages")
		} else {
			ui.Info.Printf("Total changes: %d\n", totalChanges)
			ui.Info.Println("Run 'deca apply' to apply these changes")
		}

		return nil
	},
}

func init() {
	// Add subcommands to config
	ConfigCmd.AddCommand(ConfigEditCmd)
	ConfigCmd.AddCommand(ConfigShowCmd)
	ConfigCmd.AddCommand(ConfigPathCmd)
	ConfigCmd.AddCommand(ConfigDiffCmd)

	// Set edit as the default action when no subcommand is provided
	ConfigCmd.RunE = ConfigEditCmd.RunE

	RootCmd.AddCommand(ConfigCmd)
}

// getEditor returns the editor to use
func getEditor() string {
	// Check environment variables
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}

	visual := os.Getenv("VISUAL")
	if visual != "" {
		return visual
	}

	// Common editors
	editors := []string{
		"vim", "vi", "nvim",
		"nano", "pico",
		"code", "vscode",
		"subl", "sublime",
		"atom",
		"gedit", "kate", "mousepad",
		"emacs",
	}

	for _, e := range editors {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}

	return ""
}
