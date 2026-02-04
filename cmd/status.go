package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// StatusCmd checks for updates
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check for package updates",
	Long: `Check if installed packages have updates available.

This command queries GitHub to see if newer versions of
your installed packages are available.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Load state
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to load state: %w", err)
		}

		ghClient := github.NewClient()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ui.Info.Println("Checking for updates...")
		fmt.Println()

		updates := make([]string, 0)
		errors := make([]error, 0)

		g, ctx := errgroup.WithContext(ctx)
		var mu sync.Mutex

		for name, pkg := range cfg.Packages {
			name, pkg := name, pkg
			g.Go(func() error {
				hasUpdate, err := checkUpdate(ctx, ghClient, name, &pkg, state)
				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else if hasUpdate {
					updates = append(updates, name)
				}
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		if len(updates) > 0 {
			ui.UpdateAvail.Println("Updates available:")
			for _, name := range updates {
				ui.PackageName.Printf("  [→] %s\n", name)
			}
			fmt.Println()
			ui.Info.Printf("  Run 'deca update' to update all packages\n")
		} else {
			ui.Installed.Println("✓ All packages are up to date")
		}

		if len(errors) > 0 {
			fmt.Println()
			ui.Warning.Println("Errors:")
			for _, e := range errors {
				ui.Error.Printf("  - %v\n", e)
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(StatusCmd)
}

func checkUpdate(ctx context.Context, ghClient *github.Client, name string, pkg *config.Package, state *config.State) (bool, error) {
	owner, repo, err := github.ParseRepo(pkg.Repo)
	if err != nil {
		return false, fmt.Errorf("%s: invalid repo: %w", name, err)
	}

	release, err := releaseForPackage(ctx, ghClient, owner, repo, pkg)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}

	installed, exists := state.GetPackage(name)
	if !exists {
		return false, nil // Not installed yet
	}

	currentVersion := strings.TrimPrefix(installed.Version, "v")
	newVersion := strings.TrimPrefix(release.TagName, "v")

	if currentVersion != newVersion {
		return true, nil
	}

	return false, nil
}
