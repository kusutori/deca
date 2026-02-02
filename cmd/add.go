package cmd

import (
	"context"
	"fmt"
	"runtime"

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
optionally installs it immediately.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		asset, _ := cmd.Flags().GetString("asset")
		osFlag, _ := cmd.Flags().GetString("os")
		arch, _ := cmd.Flags().GetString("arch")
		noInstall, _ := cmd.Flags().GetBool("no-install")

		repo := args[0]

		// Validate repo format
		_, repoName, err := github.ParseRepo(repo)
		if err != nil {
			return fmt.Errorf("invalid repo format: %w", err)
		}

		// Use repo name as package name if not specified
		if name == "" {
			name = repoName
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
	AddCmd.Flags().String("asset", "", "Asset pattern to match")
	AddCmd.Flags().String("os", runtime.GOOS, "Target OS")
	AddCmd.Flags().String("arch", runtime.GOARCH, "Target architecture")
	AddCmd.Flags().Bool("no-install", false, "Don't install immediately")
	RootCmd.AddCommand(AddCmd)
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

	ui.Success.Printf("Installed %s v%s to %s\n", name, release.TagName, result.BinaryPath)
	return nil
}
