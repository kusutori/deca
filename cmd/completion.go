package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/carapace-sh/carapace/pkg/uid"
	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts using carapace.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell|nushell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for supported shells.

Examples:
  deca completion bash
  deca completion zsh
  deca completion fish
  deca completion powershell
  deca completion nushell`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := strings.ToLower(args[0])
		return generateCompletion(shell, cmd.OutOrStdout())
	},
}

func generateCompletion(shell string, out io.Writer) error {
	if shell == "nushell" || shell == "nu" {
		return generateNushellCompletion(out)
	}
	snippet, err := carapace.Gen(RootCmd).Snippet(shell)
	if err != nil {
		return fmt.Errorf("unsupported shell: %w", err)
	}
	_, err = fmt.Fprint(out, snippet)
	return err
}

func generateNushellCompletion(out io.Writer) error {
	name := RootCmd.Name()
	execName := uid.Executable()
	_, err := fmt.Fprintf(out, `def "nu-complete %s" [spans: list<string>] {
  let args = if ($spans | length) == 1 {
    $spans | append ""
  } else {
    $spans
  }
  %s _carapace nushell ...$args | from json
}

# Note: Don't use 'export' to avoid module name conflict in nushell 0.110+
extern "%s" [
  ...args: string@"nu-complete %s"
]
`, name, execName, name, name)
	return err
}

func init() {
	RootCmd.AddCommand(completionCmd)
}
