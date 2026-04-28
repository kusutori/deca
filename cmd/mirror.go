package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/ui"
	"github.com/spf13/cobra"
)

// MirrorCmd manages GitHub mirror sources
var MirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Manage GitHub mirror sources",
	Long: `Manage GitHub mirror sources for faster downloads.

This command allows you to list available mirrors, add custom mirrors,
and select which mirror to use for downloads.`,
	Aliases: []string{"m"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return doMirrorShow()
	},
}

// mirrorAddCmd adds a new mirror
var mirrorAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Add a new mirror source",
	Long: `Add a new GitHub mirror source.

Examples:
  deca mirror add "My Mirror" https://mirror.example.com
  deca mirror add "China Mirror" https://ghfast.top`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		url := args[1]
		return doMirrorAdd(name, url)
	},
}

// mirrorListCmd lists available mirrors
var mirrorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available mirrors",
	Long:  `List all available mirror sources with their status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doMirrorList()
	},
}

// mirrorSelectCmd interactively selects a mirror
var mirrorSelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Interactively select a mirror",
	Long: `Interactively select which mirror source to use.

This will show a list of available mirrors and let you choose one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doMirrorSelect()
	},
}

// mirrorRemoveCmd removes a mirror
var mirrorRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a mirror",
	Long: `Remove a custom mirror source.

You cannot remove the default "GitHub (Official)" mirror.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return doMirrorRemove(name)
	},
}

// mirrorTestCmd tests connectivity for current mirror
var mirrorTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test mirror connectivity",
	Long: `Test the connectivity of the current mirror by probing API and download URLs.

This will print the exact URL being tested.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doMirrorTest()
	},
}

func init() {
	RootCmd.AddCommand(MirrorCmd)

	// Add subcommands
	MirrorCmd.AddCommand(mirrorAddCmd)
	MirrorCmd.AddCommand(mirrorListCmd)
	MirrorCmd.AddCommand(mirrorSelectCmd)
	MirrorCmd.AddCommand(mirrorRemoveCmd)
	MirrorCmd.AddCommand(mirrorTestCmd)
}

func doMirrorShow() error {
	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}
	current := cfg.GetCurrentMirror()

	ui.Primary.Println("Mirror Status:")
	fmt.Println()

	if current != nil {
		ui.SearchMeta.Printf("  Current:  %s\n", current.Name)
		ui.SearchMeta.Printf("  URL:      %s\n", current.URL)
		ui.SearchMeta.Printf("  API URL:  %s\n", current.APIURL)
	} else {
		ui.Warning.Println("  No mirror selected")
	}

	fmt.Println()
	ui.Info.Println("Use 'deca mirror list' to see all mirrors")
	ui.Info.Println("Use 'deca mirror select' to change mirror")

	return nil
}

func doMirrorList() error {
	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}
	current := cfg.GetCurrentMirror()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	ui.Primary.Println("Available Mirrors:")
	fmt.Println()

	for i, m := range cfg.Mirrors {
		if m.Name == currentName {
			ui.Success.Printf("  [%d] %s (current)\n", i+1, m.Name)
		} else {
			ui.PackageName.Printf("  [%d] %s\n", i+1, m.Name)
		}
		ui.SearchMeta.Printf("      URL:      %s\n", m.URL)
		ui.SearchMeta.Printf("      API:      %s\n", m.APIURL)
		fmt.Println()
	}

	return nil
}

func doMirrorSelect() error {
	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}
	current := cfg.GetCurrentMirror()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	ui.Primary.Println("Select a mirror:")
	fmt.Println()

	for i, m := range cfg.Mirrors {
		if m.Name == currentName {
			ui.Success.Printf("  [%d] %s (current)\n", i+1, m.Name)
		} else {
			ui.PackageName.Printf("  [%d] %s\n", i+1, m.Name)
		}
		ui.SearchMeta.Printf("      %s\n", m.URL)
		fmt.Println()
	}

	if !ui.IsTerminal() {
		ui.Info.Println("Non-interactive mode. Use 'deca mirror list' to see mirrors.")
		return nil
	}

	fmt.Print("Enter a number to select: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var selected int
	if _, err := fmt.Sscanf(input, "%d", &selected); err != nil || selected < 1 || selected > len(cfg.Mirrors) {
		ui.Warning.Printf("Invalid selection '%s'\n", input)
		return nil
	}

	selectedMirror := cfg.Mirrors[selected-1]
	cfg.CurrentName = selectedMirror.Name

	// Save to config
	path := config.GetMirrorPath()
	if err := config.SaveMirrorConfig(path, cfg); err != nil {
		return fmt.Errorf("failed to save mirror config: %w", err)
	}

	ui.Success.Printf("Selected: %s\n", selectedMirror.Name)
	ui.SearchMeta.Printf("  URL: %s\n", selectedMirror.URL)

	return nil
}

func doMirrorAdd(name, url string) error {
	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}

	// Check if mirror with same name exists
	for _, m := range cfg.Mirrors {
		if m.Name == name {
			return fmt.Errorf("mirror '%s' already exists", name)
		}
	}

	// Build API and download URLs
	// Try to detect if it's a known mirror type
	apiURL := url
	downloadURL := url + "/{owner}/{repo}/releases/download/{tag}/{asset}"

	// Adjust for specific mirror patterns
	if strings.Contains(url, "jihulab") {
		apiURL = url + "/api/v4"
		downloadURL = url + "/{owner}/{repo}/-/releases/{tag}/downloads/{asset}"
	} else if strings.Contains(url, "fastgit") {
		downloadURL = "https://download.fastgit.org/{owner}/{repo}/releases/download/{tag}/{asset}"
	} else if strings.Contains(url, "ghfast") {
		apiURL = "https://api.github.com"
		downloadURL = url + "/https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}"
	} else if strings.Contains(url, "ghproxy") {
		downloadURL = url + "/https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}"
	}

	// Add new mirror
	newMirror := config.Mirror{
		Name:        name,
		URL:         url,
		APIURL:      apiURL,
		DownloadURL: downloadURL,
	}

	cfg.Mirrors = append(cfg.Mirrors, newMirror)

	// Save to config
	path := config.GetMirrorPath()
	if err := config.SaveMirrorConfig(path, cfg); err != nil {
		return fmt.Errorf("failed to save mirror config: %w", err)
	}

	ui.Success.Printf("Added mirror: %s\n", name)
	ui.SearchMeta.Printf("  URL:      %s\n", url)
	ui.SearchMeta.Printf("  API:      %s\n", apiURL)

	return nil
}

func doMirrorRemove(name string) error {
	// Cannot remove GitHub (Official)
	if name == "GitHub (Official)" {
		return fmt.Errorf("cannot remove the default GitHub (Official) mirror")
	}

	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}

	// Find and remove
	found := false
	newMirrors := []config.Mirror{}
	for _, m := range cfg.Mirrors {
		if m.Name == name {
			found = true
		} else {
			newMirrors = append(newMirrors, m)
		}
	}

	if !found {
		return fmt.Errorf("mirror '%s' not found", name)
	}

	// If current mirror is being removed, switch to GitHub (Official)
	if cfg.CurrentName == name {
		cfg.CurrentName = "GitHub (Official)"
	}

	cfg.Mirrors = newMirrors

	// Save to config
	path := config.GetMirrorPath()
	if err := config.SaveMirrorConfig(path, cfg); err != nil {
		return fmt.Errorf("failed to save mirror config: %w", err)
	}

	ui.Success.Printf("Removed mirror: %s\n", name)

	return nil
}

func doMirrorTest() error {
	cfg, err := config.LoadMirrorConfig(config.GetMirrorPath())
	if err != nil {
		return err
	}
	current := cfg.GetCurrentMirror()
	if current == nil {
		ui.Warning.Println("No mirror selected")
		return nil
	}

	ui.Primary.Println("Mirror Connectivity Test:")
	fmt.Println()
	ui.SearchMeta.Printf("  Current: %s\n", current.Name)
	fmt.Println()

	client := &http.Client{Timeout: 6 * time.Second}

	apiBase := current.APIURL
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	testAPIURL := strings.TrimRight(apiBase, "/") + "/rate_limit"
	testDownloadURL := buildDownloadTestURL(current)

	runMirrorProbe(client, "API", testAPIURL, "GET")
	fmt.Println()
	runMirrorProbe(client, "Download", testDownloadURL, "HEAD")

	return nil
}

func buildDownloadTestURL(mirror *config.Mirror) string {
	url := mirror.DownloadURL
	url = strings.ReplaceAll(url, "{owner}", "cli")
	url = strings.ReplaceAll(url, "{repo}", "cli")
	url = strings.ReplaceAll(url, "{tag}", "v2.0.0")
	url = strings.ReplaceAll(url, "{asset}", "gh_2.0.0_linux_amd64.tar.gz")
	if strings.Contains(url, "{") {
		ui.Warning.Println("Download URL template is invalid; please re-select or re-add the mirror.")
	}
	return url
}

func runMirrorProbe(client *http.Client, label, targetURL, method string) {
	ui.Info.Printf("Testing %s URL:\n", label)
	ui.SearchMeta.Printf("  %s\n", targetURL)

	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		ui.Error.Printf("  Error: %v\n", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		ui.Error.Printf("  Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		ui.Success.Printf("  OK (%d)\n", resp.StatusCode)
	} else {
		ui.Warning.Printf("  Status: %d\n", resp.StatusCode)
	}
}

// GetCurrentMirror returns the currently selected mirror
func GetCurrentMirror() *config.Mirror {
	return config.LoadCurrentMirror()
}

// GetMirrorURLs returns the API and download URLs for the current mirror
func GetMirrorURLs() (apiURL, downloadURL string) {
	mirror := GetCurrentMirror()
	if mirror == nil {
		return "https://api.github.com", "https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}"
	}
	return mirror.APIURL, mirror.DownloadURL
}

// IsUsingMirror checks if a non-default mirror is selected
func IsUsingMirror() bool {
	mirror := GetCurrentMirror()
	if mirror == nil {
		return false
	}
	return mirror.Name != "GitHub (Official)"
}
