package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

var (
	execPath    string
	icon        string
	comment     string
	terminal    bool
	categories  string
	mimeTypes   string
	name        string
	remove      bool
)

// DesktopCmd manages .desktop entry files for GUI applications
var DesktopCmd = &cobra.Command{
	Use:   "desktop <name>",
	Short: "Generate or remove a .desktop entry file for a GUI application",
	Long: `Generate or remove a .desktop entry file for a GUI application.

This creates or removes a desktop entry file in ~/.local/share/applications for
launching GUI applications from the desktop environment.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgName := args[0]

		// Handle remove mode
		if remove {
			return removeDesktopEntry(pkgName)
		}

		// Generate mode
		return generateDesktopEntry(pkgName)
	},
}

func removeDesktopEntry(pkgName string) error {
	desktopPath := config.DesktopEntryPath(pkgName)

	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		ui.Info.Printf("Desktop entry %s does not exist\n", desktopPath)
		return nil
	}

	if dryRun {
		fmt.Printf("Would remove: %s\n", desktopPath)
		return nil
	}

	if err := os.Remove(desktopPath); err != nil {
		return fmt.Errorf("failed to remove desktop entry: %w", err)
	}

	ui.Success.Printf("Removed desktop entry: %s\n", desktopPath)
	return nil
}

func generateDesktopEntry(pkgName string) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if package exists
	pkg, exists := cfg.Packages[pkgName]
	if !exists {
		return fmt.Errorf("package %q not found in config", pkgName)
	}

	// Determine executable path
	binDir := cfg.BinDir
	if binDir == "" {
		binDir = config.DefaultBinDir()
	}

	exec := execPath
	if exec == "" {
		exec = filepath.Join(binDir, pkgName)
		if runtime.GOOS == "windows" {
			exec += ".exe"
		}
	}

	// Expand $HOME and environment variables in exec path
	exec = expandPath(exec)

	// Get desktop config from package or flags
	desktopCfg := pkg.Desktop

	// Apply flag overrides
	displayName := name
	if displayName == "" {
		if desktopCfg != nil && desktopCfg.Name != "" {
			displayName = desktopCfg.Name
		} else {
			displayName = pkgName
		}
	}

	desktopComment := comment
	if desktopComment == "" && desktopCfg != nil {
		desktopComment = desktopCfg.Comment
	}

	desktopIcon := icon
	if desktopIcon == "" && desktopCfg != nil {
		desktopIcon = desktopCfg.Icon
	}

	desktopTerminal := terminal
	if !desktopTerminal && desktopCfg != nil {
		desktopTerminal = desktopCfg.Terminal
	}

	desktopCategories := categories
	if desktopCategories == "" && desktopCfg != nil {
		desktopCategories = desktopCfg.Categories
	}
	if desktopCategories == "" {
		desktopCategories = "Utilities"
	}

	desktopMimeTypes := mimeTypes
	if desktopMimeTypes == "" && desktopCfg != nil {
		desktopMimeTypes = desktopCfg.MimeTypes
	}

	// Generate .desktop content
	content := generateDesktopFile(displayName, exec, desktopComment, desktopIcon, desktopTerminal, desktopCategories, desktopMimeTypes)

	// Determine output path
	outputPath := config.DesktopEntryPath(pkgName)

	if dryRun {
		fmt.Println("=== Preview ===")
		fmt.Println(content)
		fmt.Printf("\nWould write to: %s\n", outputPath)
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .desktop file: %w", err)
	}

	ui.PrintSuccess("Desktop entry created: " + outputPath)
	return nil
}

func init() {
	DesktopCmd.Flags().StringVarP(&execPath, "exec", "e", "", "Path to executable (auto-detected by default)")
	DesktopCmd.Flags().StringVarP(&name, "name", "n", "", "Application name (defaults to package name)")
	DesktopCmd.Flags().StringVarP(&icon, "icon", "i", "", "Icon name or path")
	DesktopCmd.Flags().StringVarP(&comment, "comment", "c", "", "Short description")
	DesktopCmd.Flags().BoolVarP(&terminal, "terminal", "t", false, "Run in terminal")
	DesktopCmd.Flags().StringVarP(&categories, "categories", "C", "", "Categories (default: Utilities)")
	DesktopCmd.Flags().StringVar(&mimeTypes, "mime-types", "", "MIME types")
	DesktopCmd.Flags().BoolVarP(&remove, "remove", "r", false, "Remove the desktop entry file")

	RootCmd.AddCommand(DesktopCmd)
}

// expandPath expands $HOME and environment variables in paths
func expandPath(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		path = strings.ReplaceAll(path, "$HOME", home)
		path = strings.ReplaceAll(path, "~", home)
	}
	return path
}

// generateDesktopFile creates a .desktop entry file content
func generateDesktopFile(appName, execPath, comment, icon string, terminal bool, categories, mimeTypes string) string {
	content := "[Desktop Entry]\n"
	content += "Type=Application\n"
	content += fmt.Sprintf("Name=%s\n", appName)
	if comment != "" {
		content += fmt.Sprintf("Comment=%s\n", comment)
	}
	content += fmt.Sprintf("Exec=%s %%U\n", execPath)
	if icon != "" {
		content += fmt.Sprintf("Icon=%s\n", icon)
	}
	content += fmt.Sprintf("Terminal=%t\n", terminal)
	if categories != "" {
		content += fmt.Sprintf("Categories=%s;\n", categories)
	}
	if mimeTypes != "" {
		content += fmt.Sprintf("MimeType=%s;\n", mimeTypes)
	}
	return content
}
