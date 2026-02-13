package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts using carapace.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for supported shells.

This command uses carapace-bin to generate completion scripts.
Make sure carapace-bin is installed:
  go install github.com/carapace-sh/carapace-bin@latest

Then generate completions:
  carapace _deca bash > ~/.bash_completion.d/deca
  carapace _deca zsh  > ~/.zsh/completion/_deca
  carapace _deca fish > ~/.config/fish/completions/deca.fish

Examples:
  deca completion bash
  deca completion zsh
  deca completion fish`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if carapace-bin is available
		if _, err := exec.LookPath("carapace"); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "carapace-bin not found. Install it with:")
			fmt.Fprintln(cmd.OutOrStdout(), "  go install github.com/carapace-sh/carapace-bin@latest")
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Then generate completions:")
			fmt.Fprintln(cmd.OutOrStdout(), "  carapace _deca bash > ~/.bash_completion.d/deca")
			fmt.Fprintln(cmd.OutOrStdout(), "  carapace _deca zsh  > ~/.zsh/completion/_deca")
			fmt.Fprintln(cmd.OutOrStdout(), "  carapace _deca fish > ~/.config/fish/completions/deca.fish")
			return nil
		}

		shell := strings.ToLower(args[0])
		return generateCompletion(shell, cmd.OutOrStdout())
	},
}

func generateCompletion(shell string, out io.Writer) error {
	switch shell {
	case "bash":
		_, err := fmt.Fprintln(out, "Run: carapace _deca bash > ~/.bash_completion.d/deca")
		return err
	case "zsh":
		_, err := fmt.Fprintln(out, "Run: carapace _deca zsh > ~/.zsh/completion/_deca")
		return err
	case "fish":
		_, err := fmt.Fprintln(out, "Run: carapace _deca fish > ~/.config/fish/completions/deca.fish")
		return err
	case "powershell", "pwsh":
		_, err := fmt.Fprintln(out, "Run: carapace _deca powershell > ~/.config/powershell/Modules/deca/deca.ps1")
		return err
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
	}
}

func init() {
	RootCmd.AddCommand(completionCmd)
}
