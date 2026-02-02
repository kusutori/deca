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
		if runtime.GOOS == "windows" {
			err = exec.Command("cmd", "/c", "start", editor, configPath).Start()
		} else if runtime.GOOS == "darwin" {
			err = exec.Command("open", editor, configPath).Start()
		} else {
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
	Long: `Display the current configuration file content.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()

		// Check if config exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			ui.Warning.Printf("Config file not found: %s\n", configPath)
			ui.Info.Println("Run 'deca init' to create a default config.")
			return nil
		}

		// Load and display config
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ui.Primary.Println("Configuration:")
		ui.SearchMeta.Printf("  Config File: %s\n", configPath)
		ui.SearchMeta.Printf("  Bin Directory: %s\n", cfg.BinDir)
		ui.SearchMeta.Printf("  OS: %s\n", cfg.OS)
		ui.SearchMeta.Printf("  Arch: %s\n", cfg.Arch)
		fmt.Println()

		if len(cfg.Packages) > 0 {
			ui.Primary.Println("Packages:")
			for name, pkg := range cfg.Packages {
				ui.PackageName.Printf("  %s\n", name)
				ui.SearchMeta.Printf("    -> %s\n", pkg.Repo)
			}
		} else {
			ui.Warning.Println("  No packages configured")
		}

		return nil
	},
}

// ConfigPathCmd shows the config file path
var ConfigPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Long: `Display the path to the configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := getConfigPath()
		fmt.Println(configPath)
		return nil
	},
}

func init() {
	// Add subcommands to config
	ConfigCmd.AddCommand(ConfigEditCmd)
	ConfigCmd.AddCommand(ConfigShowCmd)
	ConfigCmd.AddCommand(ConfigPathCmd)

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
