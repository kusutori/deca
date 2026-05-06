package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
	"github.com/spf13/cobra"
)

// AddCmd adds a package
var AddCmd = &cobra.Command{
	Use:   "add <owner/repo> ... [--name <name>]",
	Short: "Add a package to the configuration",
	Long: `Add a package to the configuration and install it.

This command adds a new package to the configuration file and
optionally installs it immediately.

Multiple packages can be added at once by specifying multiple owner/repo arguments.
When adding multiple packages, --name flag is ignored and repo name is used.

Use --interactive to see all available assets and select one.
Use --asset to specify which asset to download.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		asset, _ := cmd.Flags().GetString("asset")
		interactive, _ := cmd.Flags().GetBool("interactive")
		osFlag, _ := cmd.Flags().GetString("os")
		arch, _ := cmd.Flags().GetString("arch")
		noInstall, _ := cmd.Flags().GetBool("no-install")
		prereleaseFlag, _ := cmd.Flags().GetBool("prerelease")

		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.Packages == nil {
			cfg.Packages = make(map[string]config.Package)
		}

		ghClient := github.NewClient()
		installer := install.NewInstaller(cfg.BinDir)

		for _, repo := range args {
			owner, repoName, err := github.ParseRepo(repo)
			if err != nil {
				return fmt.Errorf("invalid repo format %q: %w", repo, err)
			}

			pkgName := name
			if len(args) > 1 || pkgName == "" {
				pkgName = repoName
			}

			pkgAsset := asset
			if interactive {
				selectedAsset, err := interactiveSelectAsset(owner, repoName, prereleaseFlag)
				if err != nil {
					return fmt.Errorf("interactive selection failed for %s: %w", repo, err)
				}
				if selectedAsset != nil {
					pkgAsset = selectedAsset.Name
				}
			}

			cfg.Packages[pkgName] = config.Package{
				Repo:       repo,
				Asset:      pkgAsset,
				OS:         osFlag,
				Arch:       arch,
				Prerelease: prereleaseFlag,
			}

			configPath := getConfigPath()
			if err := config.Save(cfg, configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			ui.Success.Printf("Added %s -> %s\n", pkgName, repo)
			if pkgAsset != "" {
				ui.SearchMeta.Printf("  Asset: %s\n", pkgAsset)
			}

			if !noInstall {
				pkg := cfg.Packages[pkgName]
				if err := doInstall(cmd.Context(), ghClient, installer, pkgName, &pkg, prereleaseFlag); err != nil {
					return err
				}
			}
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
	AddCmd.Flags().Bool("prerelease", false, "Include pre-release versions when selecting the latest release")
	RootCmd.AddCommand(AddCmd)
}

// interactiveSelectAsset shows all assets and lets user select one
func interactiveSelectAsset(owner, repoName string, includePrerelease bool) (*github.AssetInfo, error) {
	ghClient := github.NewClient()
	ctx := getContext()

	// Get latest release
	release, err := ghClient.GetLatestReleaseWithOptions(ctx, owner, repoName, includePrerelease)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	// Check if we should use interactive selection
	if !ui.IsTerminal() {
		ui.Info.Println("Non-interactive mode, using first asset")
		if len(release.Assets) > 0 {
			return &release.Assets[0], nil
		}
		return nil, nil
	}

	// Use interactive selector (which prints the table)
	fullName := owner + "/" + repoName
	selected := ui.InteractiveSelectAssets(release.Assets, fullName)

	if selected != nil && selected.Name != "" {
		ui.Success.Printf("Selected: %s\n", selected.Name)
	}

	return selected, nil
}

func doInstall(ctx context.Context, ghClient *github.Client, installer *install.Installer, name string, pkg *config.Package, forcePrerelease bool) error {
	ok, targetOS, targetArch, err := config.PackageMatches(pkg, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("%s: invalid condition: %w", name, err)
	}
	if !ok {
		ui.Warning.Printf("%s skipped: os/arch condition not met\n", name)
		return nil
	}

	owner, repo, err := github.ParseRepo(pkg.Repo)
	if err != nil {
		return fmt.Errorf("%s: invalid repo: %w", name, err)
	}

	// Get latest release
	release, err := ghClient.GetLatestReleaseWithOptions(ctx, owner, repo, pkg.Prerelease || forcePrerelease)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch release: %w", name, err)
	}

	// Find matching asset
	asset, err := github.FindMatchingAsset(release, pkg.Asset, targetOS, targetArch)
	if err != nil {
		return fmt.Errorf("%s: no matching asset: %w", name, err)
	}

	// Install
	result, err := installer.Install(name, release, asset)
	if err != nil {
		return fmt.Errorf("%s: failed to install: %w", name, err)
	}

	// Create versioned symlink if requested and binary was installed
	if pkg.Versioned && result.BinaryPath != "" {
		versionedPath, symlinkErr := install.CreateVersionedSymlink(installer.BinDir, name, release.TagName, result.BinaryPath)
		if symlinkErr != nil {
			ui.Warning.Printf("Warning: failed to create versioned symlink: %v\n", symlinkErr)
		} else {
			result.VersionedBinaryPath = versionedPath
		}
	}

	// Update state
	statePath := config.DefaultStatePath()
	state, err := config.LoadState(statePath)
	if err == nil {
		state.SetPackage(name, config.InstalledPackage{
			Repo:                pkg.Repo,
			Version:             release.TagName,
			AssetName:           asset.Name,
			InstallType:         result.InstallType,
			InstalledAt:         time.Now(),
			SystemPkgName:       result.SystemPkgName,
			VersionedBinaryPath: result.VersionedBinaryPath,
		})
		state.SaveState(statePath)
	}

	// Auto-create desktop entry for AppImage if desktop config is present
	if result.InstallType == config.InstallTypeAppImage && pkg.Desktop != nil {
		if desktopErr := generateDesktopEntry(name); desktopErr != nil {
			ui.Warning.Printf("Warning: failed to create desktop entry: %v\n", desktopErr)
		}
	}

	// Print success message
	if result.BinaryPath != "" {
		ui.Success.Printf("Installed %s v%s to %s\n", name, strings.TrimPrefix(release.TagName, "v"), result.BinaryPath)
	} else {
		ui.Success.Printf("Installed %s v%s (system package)\n", name, strings.TrimPrefix(release.TagName, "v"))
	}
	return nil
}
