package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kusutori/deca/internal/cache"
	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/ui"
	"github.com/spf13/cobra"
)

// CacheCmd manages the download cache
var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage download cache",
	Long: `Manage the download cache for faster subsequent installations.

This command allows you to view cache status, list cached files,
and clean up the cache to free up disk space.`,
	Aliases: []string{"c"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCacheShow()
	},
}

// cacheCleanCmd cleans the cache
var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean the download cache",
	Long: `Clean the download cache.

Use --orphans to remove only cached files that are no longer
installed (based on the state file).`,
	Aliases: []string{"c"},
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		orphans, _ := cmd.Flags().GetBool("orphans")
		return doCacheClean(all, orphans)
	},
}

// cacheListCmd lists cached files
var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached files",
	Long: `List all files in the download cache with their sizes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCacheList()
	},
}

// cacheSizeCmd shows cache size
var cacheSizeCmd = &cobra.Command{
	Use:   "size",
	Short: "Show cache size",
	Long: `Show the total size of the download cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCacheSize()
	},
}

func init() {
	RootCmd.AddCommand(CacheCmd)

	// cache clean flags
	cacheCleanCmd.Flags().BoolP("all", "a", false, "Clean all cached files")
	cacheCleanCmd.Flags().Bool("orphans", false, "Clean only orphaned cached files (not in state)")
	CacheCmd.AddCommand(cacheCleanCmd)

	// cache subcommands
	CacheCmd.AddCommand(cacheListCmd)
	CacheCmd.AddCommand(cacheSizeCmd)
}

func doCacheShow() error {
	c := cache.NewCache()

	count, err := c.Count()
	if err != nil {
		return fmt.Errorf("failed to count cache entries: %w", err)
	}

	size, err := c.Size()
	if err != nil {
		return fmt.Errorf("failed to calculate cache size: %w", err)
	}

	ui.Primary.Println("Cache Status:")
	ui.SearchMeta.Printf("  Location: %s\n", c.RootDir)
	ui.SearchMeta.Printf("  Files:    %d\n", count)
	ui.SearchMeta.Printf("  Size:     %s\n", formatSize(size))

	return nil
}

func doCacheClean(all, orphans bool) error {
	c := cache.NewCache()

	if orphans {
		// Clean only orphaned files
		statePath := config.DefaultStatePath()
		state, err := config.LoadState(statePath)
		if err != nil {
			// If no state file, treat all as orphans
			state = &config.State{Packages: make(map[string]config.InstalledPackage)}
		}

		// Build a set of referenced cache keys
		referenced := make(map[string]struct{})
		for name, pkg := range state.Packages {
			key := pkg.Repo + "/" + pkg.Version + "/" + pkg.AssetName
			referenced[key] = struct{}{}
			// Also check by name
			if _, ok := state.Packages[name]; ok {
				keyByName := pkg.Repo + "/" + strings.TrimPrefix(pkg.Version, "v") + "/" + pkg.AssetName
				referenced[keyByName] = struct{}{}
			}
		}

		entries, err := c.ListEntries()
		if err != nil {
			return fmt.Errorf("failed to list cache entries: %w", err)
		}

		removed := 0
		for _, entry := range entries {
			key := entry.Repo + "/" + entry.Version + "/" + entry.AssetName
			// Also try without 'v' prefix in version
			versionNoV := strings.TrimPrefix(entry.Version, "v")
			keyNoV := entry.Repo + "/" + versionNoV + "/" + entry.AssetName

			if _, exists := referenced[key]; !exists {
				if _, exists := referenced[keyNoV]; !exists {
					if err := os.Remove(entry.Path); err == nil {
						removed++
					}
				}
			}
		}

		ui.Success.Printf("Removed %d orphaned cached file(s)\n", removed)
	} else if all {
		// Clean all
		if err := c.Clean(); err != nil {
			return fmt.Errorf("failed to clean cache: %w", err)
		}
		ui.Success.Println("Cache cleaned completely")
	} else {
		// Default: ask for confirmation
		size, _ := c.Size()
		count, _ := c.Count()

		ui.Warning.Printf("This will remove %d cached file(s) (%s).\n", count, formatSize(size))
		ui.Info.Println("Use --orphans to remove only unused files, or --all to force.")

		return nil
	}

	return nil
}

func doCacheList() error {
	c := cache.NewCache()

	entries, err := c.ListEntries()
	if err != nil {
		return fmt.Errorf("failed to list cache entries: %w", err)
	}

	if len(entries) == 0 {
		ui.Info.Println("Cache is empty")
		return nil
	}

	ui.Primary.Println("Cached Files:")
	fmt.Println()

	for _, entry := range entries {
		repoDisplay := strings.ReplaceAll(entry.Repo, "-", "/")
		ui.PackageName.Printf("  %s\n", filepath.Base(entry.Path))
		ui.SearchMeta.Printf("    %s @ %s\n", repoDisplay, entry.Version)
		ui.SearchMeta.Printf("    Size: %s\n", formatSize(entry.Size))
		ui.SearchMeta.Printf("    Path: %s\n", entry.Path)
		fmt.Println()
	}

	totalSize := formatSize(sizeFromEntries(entries))
	ui.Installed.Printf("Total: %d file(s), %s\n", len(entries), totalSize)

	return nil
}

func doCacheSize() error {
	c := cache.NewCache()

	size, err := c.Size()
	if err != nil {
		return fmt.Errorf("failed to calculate cache size: %w", err)
	}

	count, err := c.Count()
	if err != nil {
		return fmt.Errorf("failed to count cache entries: %w", err)
	}

	ui.Success.Printf("Cache size: %s (%d files)\n", formatSize(size), count)

	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func sizeFromEntries(entries []cache.Entry) int64 {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	return total
}
