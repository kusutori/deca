package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/install"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

// UpdateCmd updates packages
var UpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update packages to latest versions",
	Long: `Update installed packages to their latest versions.

With no arguments, updates all installed packages.
With a name, updates only that specific package.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create installer
		installer := install.NewInstaller(cfg.BinDir)

		// Load state
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		ghClient := github.NewClient()
		ctx := getContext()

		packagesToUpdate := cfg.Packages
		specificName := ""

		if len(args) > 0 {
			specificName = args[0]
			pkg, exists := cfg.Packages[specificName]
			if !exists {
				return fmt.Errorf("package %s not found in config", specificName)
			}
			packagesToUpdate = map[string]config.Package{specificName: pkg}
		}

		ui.Primary.Println("Updating packages...")
		fmt.Println()

		updatedCount := 0
		skippedCount := 0

		for name, pkg := range packagesToUpdate {
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

			// Check if already at latest version
			installed, exists := state.GetPackage(name)
			currentVersion := strings.TrimPrefix(installed.Version, "v")
			newVersion := strings.TrimPrefix(release.TagName, "v")

			if exists && currentVersion == newVersion {
				// Already at latest version
				if verbose {
					ui.SearchMeta.Printf("%s: already at latest (v%s)\n", name, currentVersion)
				}
				skippedCount++
				continue
			}

			// Install
			result, err := installer.Install(name, release, asset)
			if err != nil {
				return fmt.Errorf("%s: failed to install: %w", name, err)
			}

			// Update state
			state.SetPackage(name, config.InstalledPackage{
				Repo:        pkg.Repo,
				Version:     release.TagName,
				AssetName:   asset.Name,
				InstalledAt: time.Now(),
			})

			ui.Success.Printf("Updated %s to v%s\n", name, release.TagName)
			if result.BinaryPath != "" {
				ui.SearchMeta.Printf("  Binary: %s\n", result.BinaryPath)
			} else {
				ui.SearchMeta.Println("  (system package)")
			}
			fmt.Println()
			updatedCount++
		}

		// Summary
		if updatedCount > 0 {
			ui.Success.Printf("Updated %d package(s)\n", updatedCount)
		}
		if skippedCount > 0 {
			ui.Installed.Printf("%d package(s) already up to date\n", skippedCount)
		}

		// Save state
		if err := state.SaveState(statePath); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(UpdateCmd)
}
