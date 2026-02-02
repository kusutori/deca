package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/deca-org/deca/internal/config"
	decaerrors "github.com/deca-org/deca/internal/errors"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/install"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// ApplyCmd applies the configuration
var ApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply configuration and install packages",
	Long: `Apply the configuration file and install/update packages.

This command reads the configuration file and ensures all packages
are installed with the specified versions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return decaerrors.NewConfigNotFoundError(getConfigPath())
		}

		// Create installer
		installer := install.NewInstaller(cfg.BinDir)

		// Ensure bin directory exists
		if err := installer.EnsureBinDir(); err != nil {
			return decaerrors.NewPermissionDeniedError("create bin directory")
		}

		// Load state
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			return decaerrors.NewDecaError(decaerrors.ErrCodeConfigInvalid, "failed to load state").WithParent(err)
		}

		// Create GitHub client
		ghClient := github.NewClient()
		ctx := context.Background()

		// Track results
		results := make([]string, 0)
		var m sync.Mutex
		var errs []error

		// Process packages
		g, ctx := errgroup.WithContext(ctx)

		for name, pkg := range cfg.Packages {
			name, pkg := name, pkg
			g.Go(func() error {
				result, err := installPackage(ctx, ghClient, installer, name, &pkg, state)
				m.Lock()
				if err != nil {
					errs = append(errs, err)
				} else {
					results = append(results, result)
				}
				m.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// Save state
		if err := state.SaveState(statePath); err != nil {
			return decaerrors.NewDecaError(decaerrors.ErrCodeConfigSave, "failed to save state").WithParent(err)
		}

		// Print results
		if len(results) > 0 {
			ui.Primary.Println("Installed/Updated:")
			for _, r := range results {
				ui.PackageName.Printf("  %s\n", r)
			}
			fmt.Println()
		}

		if len(errs) > 0 {
			ui.Error.Println("Errors:")
			ui.PrintMultipleErrors(errs)
			fmt.Println()
		}

		// Check if bin dir is in PATH
		if !installer.BinDirInPATH() {
			ui.Warning.Printf("\nNote: %s is not in your PATH.\n", cfg.BinDir)
			ui.Info.Printf("Add it with: %s\n", installer.AddToPATHInstructions())
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(ApplyCmd)
}

func installPackage(ctx context.Context, ghClient *github.Client, installer *install.Installer, name string, pkg *config.Package, state *config.State) (string, error) {
	owner, repo, err := github.ParseRepo(pkg.Repo)
	if err != nil {
		return "", decaerrors.NewPackageNotFoundError(name).WithParent(err)
	}

	// Get latest release
	printStatus(fmt.Sprintf("Fetching %s...", name))
	release, err := ghClient.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", decaerrors.NewGitHubAPIError(
			fmt.Errorf("%s: failed to fetch release: %w", name, err),
		)
	}

	// Find matching asset
	asset, err := github.FindMatchingAsset(release, pkg.Asset, pkg.OS, pkg.Arch)
	if err != nil {
		return "", decaerrors.NewAssetNotFoundError(pkg.OS, pkg.Arch, pkg.Asset).WithParent(err)
	}

	// Check if already installed
	installed, exists := state.GetPackage(name)
	currentVersion := strings.TrimPrefix(installed.Version, "v")
	newVersion := strings.TrimPrefix(release.TagName, "v")

	if exists && currentVersion == newVersion {
		if verbose {
			ui.SearchMeta.Printf("%s: already installed (v%s)\n", name, currentVersion)
		}
		return fmt.Sprintf("%s v%s (up to date)", name, newVersion), nil
	}

	// Install
	printStatus(fmt.Sprintf("Installing %s v%s...", name, release.TagName))
	result, err := installer.Install(name, release, asset)
	if err != nil {
		return "", decaerrors.NewInstallError(name, err)
	}

	// Update state
	state.SetPackage(name, config.InstalledPackage{
		Repo:        pkg.Repo,
		Version:     release.TagName,
		AssetName:   asset.Name,
		InstallType: result.InstallType,
		InstalledAt: time.Now(),
	})

	// Format result message based on package type
	oldVersion := strings.TrimPrefix(installed.Version, "v")
	if oldVersion == "" {
		oldVersion = "?"
	}
	if result.BinaryPath == "" {
		// System package
		return fmt.Sprintf("%s v%s (system package)", name, strings.TrimPrefix(release.TagName, "v")), nil
	}
	return fmt.Sprintf("%s v%s -> %s", name, oldVersion, result.BinaryPath), nil
}

func loadConfig() (*config.Config, error) {
	path := getConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, decaerrors.NewConfigNotFoundError(path)
	}
	return config.Load(path)
}
