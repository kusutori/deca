package cmd

import (
	"fmt"

	"github.com/deca-org/deca/internal/github"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

// SearchCmd searches for packages
var SearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for packages on GitHub",
	Long: `Search for packages/repositories on GitHub.

This command searches GitHub for repositories that have
releases with binary assets.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		ghClient := github.NewClient()
		results, err := ghClient.SearchRepositories(getContext(), query+" topic:cli")
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(results) == 0 {
			ui.PrintWarning("No results found for: " + query)
			return nil
		}

		ui.SearchTitle.Printf("Search results for '%s':\n", query)
		fmt.Println()

		for _, r := range results {
			// Repository name with color
			ui.SearchRepo.Printf("  %s\n", r.FullName)

			// Description with color
			if r.Desc != "" {
				ui.SearchDesc.Printf("    %s\n", truncate(r.Desc, 60))
			}

			// Stars and updated date with color
			ui.SearchMeta.Printf("    ")
			ui.SearchStars.Printf("★ %d", r.Stars)
			ui.SearchMeta.Printf(" | ")
			ui.SearchMeta.Printf("Updated: %s", r.UpdatedAt)
			ui.SearchMeta.Println()
			fmt.Println()
		}

		// Print add command hint
		ui.Info.Printf("  Add with: deca add <owner/repo>\n")
		fmt.Println()

		return nil
	},
}

func init() {
	RootCmd.AddCommand(SearchCmd)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
