package ui

import (
	"fmt"
	"strings"

	"github.com/deca-org/deca/internal/github"
)

// PrintAssetTable prints a simple ASCII table of assets and lets user select
func PrintAssetTable(assets []github.AssetInfo, repoName string) int {
	if len(assets) == 0 {
		Warning.Println("No assets available")
		return -1
	}

	fmt.Println()
	Primary.Printf("Available assets for %s:\n", repoName)
	fmt.Println()

	// Header
	SearchMeta.Printf("%-4s %-50s %10s\n", "#", "Name", "Size")
	SearchMeta.Println(strings.Repeat("-", 70))

	// Items
	for i, asset := range assets {
		sizeStr := formatSize(asset.Size)
		SearchMeta.Printf("%-4d %-50s %10s\n", i+1, truncate(asset.Name, 50), sizeStr)
	}

	fmt.Println()
	Info.Printf("Total: %d assets\n", len(assets))
	Info.Println("Enter a number to select (or press Enter to use default): ")

	// Read user input
	var input string
	fmt.Scanln(&input)

	// Parse input
	if input == "" {
		return 0 // Default to first asset
	}

	var selected int
	_, err := fmt.Sscanf(input, "%d", &selected)
	if err != nil || selected < 1 || selected > len(assets) {
		Warning.Printf("Invalid selection '%s', using default (1)\n", input)
		return 0
	}

	return selected - 1
}

// formatSize formats file size to human readable string
func formatSize(size int64) string {
	if size == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	unitIndex := 0
	sizeFloat := float64(size)

	for sizeFloat >= 1024 && unitIndex < len(units)-1 {
		sizeFloat /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f %s", sizeFloat, units[unitIndex])
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// InteractiveSelectAssets shows all assets and lets user select one (simplified)
func InteractiveSelectAssets(assets []github.AssetInfo, repoName string) *github.AssetInfo {
	// Check if we have a terminal
	if !IsTerminal() {
		// Not a terminal, return first asset
		if len(assets) > 0 {
			return &assets[0]
		}
		return nil
	}

	// Check if we have enough assets to need selection
	if len(assets) <= 1 {
		if len(assets) > 0 {
			return &assets[0]
		}
		return nil
	}

	// Print table and get selection
	selectedIndex := PrintAssetTable(assets, repoName)
	if selectedIndex >= 0 && selectedIndex < len(assets) {
		return &assets[selectedIndex]
	}

	return &assets[0] // Default to first
}
