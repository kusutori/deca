package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts.
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
		out := cmd.OutOrStdout()

		switch shell {
		case "bash":
			return RootCmd.GenBashCompletionV2(out, true)
		case "zsh":
			return RootCmd.GenZshCompletion(out)
		case "fish":
			return RootCmd.GenFishCompletion(out, true)
		case "powershell", "pwsh":
			return RootCmd.GenPowerShellCompletionWithDesc(out)
		case "nushell", "nu":
			return genNushellCompletion(out, RootCmd)
		default:
			return fmt.Errorf("unsupported shell: %s", shell)
		}
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}

func genNushellCompletion(w io.Writer, root *cobra.Command) error {
	name := root.Name()
	_, err := fmt.Fprintf(w, `# Nushell completion for %s
def "nu-complete %s" [line: string, pos: int] {
  let cmd = ($line | str substring ..$pos)
  let completions = (^%s __complete $cmd | lines)
  $completions
    | where {|it| not ($it | str starts-with ":")}
    | each {|it|
        let parts = ($it | split column "\t")
        { value: $parts.column1, description: ($parts.column2 | default "") }
      }
}

export extern "%s" [
  ...args
] | complete "nu-complete %s"
`, name, name, name, name, name)
	return err
}
