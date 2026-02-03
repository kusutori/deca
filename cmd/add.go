package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/install"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

// AddCmd adds a package
var AddCmd = &cobra.Command{
	Use:   "add <owner/repo> [--name <name>]",
	Short: "Add a package to the configuration",
	Long: `Add a package to the configuration and install it.

This command adds a new package to the configuration file and
optionally installs it immediately.

Use --interactive to see all available assets and select one.
Use --asset to specify which asset to download.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		asset, _ := cmd.Flags().GetString("asset")
		interactive, _ := cmd.Flags().GetBool("interactive")
		osFlag, _ := cmd.Flags().GetString("os")
		arch, _ := cmd.Flags().GetString("arch")
		noInstall, _ := cmd.Flags().GetBool("no-install")

		repo := args[0]

		// Validate repo format
		owner, repoName, err := github.ParseRepo(repo)
		if err != nil {
			return fmt.Errorf("invalid repo format: %w", err)
		}

		// Use repo name as package name if not specified
		if name == "" {
			name = repoName
		}

		// If interactive mode, show all assets and let user select
		if interactive {
			selectedAsset, err := interactiveSelectAsset(owner, repoName)
			if err != nil {
				return fmt.Errorf("interactive selection failed: %w", err)
			}
			if selectedAsset != nil {
				asset = selectedAsset.Name
			}
		}

		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Add package to config
		if cfg.Packages == nil {
			cfg.Packages = make(map[string]config.Package)
		}

		cfg.Packages[name] = config.Package{
			Repo:    repo,
			Asset:   asset,
			OS:      osFlag,
			Arch:    arch,
		}

		// Save config
		configPath := getConfigPath()
		if err := config.Save(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		ui.Success.Printf("Added %s -> %s\n", name, repo)
		if asset != "" {
			ui.SearchMeta.Printf("  Asset: %s\n", asset)
		}

		// Install if requested
		if !noInstall {
			pkg := cfg.Packages[name]
			return doInstall(cmd.Context(), github.NewClient(), install.NewInstaller(cfg.BinDir), name, &pkg)
		}

		return nil
	},
}

func init() {
	AddCmd.Flags().StringP("name", "n", "", "Package name (defaults to repo name)")
	AddCmd.Flags().String("asset", "", "Asset pattern to match (e.g., '*.deb', '*linux*')")
	AddCmd.Flags().BoolP("interactive", "i", false, "Interactive asset selection")
	AddCmd.Flags().String("os", runtime.GOOS, "Target OS")
	AddCmd.Flags().String("arch", runtime.GOARCH, "Target architecture")
	AddCmd.Flags().Bool("no-install", false, "Don't install immediately")
	RootCmd.AddCommand(AddCmd)
}

// interactiveSelectAsset shows all assets and lets user select one
func interactiveSelectAsset(owner, repoName string) (*github.AssetInfo, error) {
	ghClient := github.NewClient()
	ctx := getContext()

	// Get latest release
	release, err := ghClient.GetLatestRelease(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	// Print asset table
	ui.PrintAssetTable(release.Assets, owner+"/"+repoName)

	// Check if we should use interactive selection
	if !ui.IsTerminal() {
		ui.Info.Println("Non-interactive mode, using first asset")
		if len(release.Assets) > 0 {
			return &release.Assets[0], nil
		}
		return nil, nil
	}

	// Use interactive selector
	fullName := owner + "/" + repoName
	selected := ui.InteractiveSelectAssets(release.Assets, fullName)

	if selected != nil && selected.Name != "" {
		ui.Success.Printf("Selected: %s\n", selected.Name)
	}

	return selected, nil
}

func doInstall(ctx context.Context, ghClient *github.Client, installer *install.Installer, name string, pkg *config.Package) error {
	owner, repo, err := github.ParseRepo(pkg.Repo)
	if err != nil {
		return fmt.Errorf("%s: invalid repo: %w", name, err)
	}

	// Get latest release
	release, err := ghClient.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch release: %w", name, err)
	}

	// Find matching asset
	asset, err := github.FindMatchingAsset(release, pkg.Asset, pkg.OS, pkg.Arch)
	if err != nil {
		return fmt.Errorf("%s: no matching asset: %w", name, err)
	}

	// Install
	result, err := installer.Install(name, release, asset)
	if err != nil {
		return fmt.Errorf("%s: failed to install: %w", name, err)
	}

	// Update state
	statePath := config.DefaultStatePath()
	state, err := config.LoadState(statePath)
	if err == nil {
		state.SetPackage(name, config.InstalledPackage{
			Repo:          pkg.Repo,
			Version:       release.TagName,
			AssetName:     asset.Name,
			InstallType:   result.InstallType,
			InstalledAt:   time.Now(),
			SystemPkgName: result.SystemPkgName,
		})
		state.SaveState(statePath)
	}

	// Print success message
	if result.BinaryPath != "" {
		ui.Success.Printf("Installed %s v%s to %s\n", name, strings.TrimPrefix(release.TagName, "v"), result.BinaryPath)
	} else {
		ui.Success.Printf("Installed %s v%s (system package)\n", name, strings.TrimPrefix(release.TagName, "v"))
	}
	return nil
}
