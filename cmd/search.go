package cmd

import (
	"fmt"

	"github.com/deca-org/deca/internal/github"
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
			fmt.Println("No results found")
			return nil
		}

		fmt.Printf("Search results for '%s':\n", query)
		fmt.Println()

		for _, r := range results {
			fmt.Printf("  %s\n", r.FullName)
			if r.Desc != "" {
				fmt.Printf("    %s\n", truncate(r.Desc, 60))
			}
			fmt.Printf("    Stars: %d | Updated: %s\n", r.Stars, r.UpdatedAt)
			fmt.Println()
		}

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
