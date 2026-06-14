package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
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
		prereleaseFlag, _ := cmd.Flags().GetBool("prerelease")
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

		runtimeOS := cfg.OS
		runtimeArch := cfg.Arch
		for name, pkg := range packagesToUpdate {
			ok, targetOS, targetArch, err := config.PackageMatches(&pkg, runtimeOS, runtimeArch)
			if err != nil {
				return fmt.Errorf("%s: invalid condition: %w", name, err)
			}
			if !ok {
				if verbose {
					ui.Info.Printf("%s: skipped (os/arch condition)\n", name)
				}
				continue
			}

			owner, repo, err := github.ParseRepo(pkg.Repo)
			if err != nil {
				return fmt.Errorf("%s: invalid repo: %w", name, err)
			}

			// Get desired release (latest or pinned)
			release, err := releaseForPackage(ctx, ghClient, owner, repo, &pkg, prereleaseFlag)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch release: %w", name, err)
			}

			// Find matching asset
			asset, err := github.FindMatchingAsset(release, pkg.Asset, targetOS, targetArch)
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

			// Install with rollback support
			var backupPath string
			var targetPath string
			if exists && installed.InstallType != config.InstallTypeSystem {
				targetPath = install.BinaryPath(installer.BinDir, name, installed.InstallType)
				if targetPath != "" {
					if _, err := os.Stat(targetPath); err == nil {
						backupPath, err = install.BackupFile(targetPath)
						if err != nil {
							return fmt.Errorf("%s: failed to backup: %w", name, err)
						}
					}
				}
			}

			result, err := installer.Install(name, release, asset, pkg.InstallType)
			if err != nil {
				if backupPath != "" {
					_ = install.RestoreFile(backupPath, targetPath)
				}
				return fmt.Errorf("%s: failed to install: %w", name, err)
			}
			if backupPath != "" {
				_ = install.RemoveBackup(backupPath)
			}

			// Update state
			state.SetPackage(name, installedPackageFromResult(name, &pkg, installer, release, result))

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
	UpdateCmd.Flags().Bool("prerelease", false, "Include pre-release versions when checking/updating packages")
	RootCmd.AddCommand(UpdateCmd)
}
