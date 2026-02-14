# Shell Completion Migration to Carapace

This document describes the migration from Cobra's built-in completion to Carapace.

## Changes

- Added `github.com/carapace-sh/carapace` dependency
- Created `cmd/completion_carapace.go` with custom completers for:
  - `add`: GitHub repo search completion, asset completion
  - `remove`: installed package name completion
  - `update`: installed package name completion
  - `status`: installed package name completion
- Updated `cmd/completion.go` to emit Carapace snippets instead of Cobra's built-in generators
- Fixed Nushell completion to remove `export` keyword (avoids module name conflict in Nushell 0.110+)

## Usage

### Bash/Zsh/Fish

Generate completion scripts with the built-in command:

```bash
# Bash
deca completion bash > ~/.bash_completion.d/deca
source ~/.bash_completion.d/deca

# Zsh
deca completion zsh > ~/.zsh/completion/_deca
# Add ~/.zsh/completion to fpath in .zshrc

# Fish
deca completion fish > ~/.config/fish/completions/deca.fish
```

Or source directly:
```bash
source <(deca completion bash)
```

### Nushell (Recommended Approach)

**Don't use separate completion files.** Instead, configure the external completer in `~/.config/nushell/config.nu`:

```nushell
$env.config.completions = {
    external: {
        enable: true
        max_results: 100
        completer: {|spans|
            let cmd = $spans.0
            if ($cmd == "deca") {
                try {
                    deca _carapace nushell ...$spans | from json
                } catch {
                    null
                }
            } else {
                # Use carapace for other commands if installed
                try {
                    carapace $cmd nushell ...$spans | from json
                } catch {
                    null
                }
            }
        }
    }
}
```

This approach:
- Works with Nushell 0.110+ (no module name conflicts)
- Integrates with carapace ecosystem
- No separate completion files needed
- Supports both deca and other carapace-enabled tools

For detailed troubleshooting and explanation, see [NUSHELL_COMPLETION_GUIDE.md](./NUSHELL_COMPLETION_GUIDE.md).

### PowerShell

```powershell
deca completion powershell | Out-String | Invoke-Expression
```

## Direct Carapace Access

You can also call the hidden carapace subcommand directly:

```bash
deca _carapace bash
deca _carapace nushell deca ""
```

## Implementation Details

### Key Fixes

1. **Nushell 0.110+ Compatibility**: Removed `export` keyword from generated completion to avoid module name conflicts
2. **Correct Context Usage**: Use `c.Value` instead of `c.Args[0]` in carapace callbacks to get current input
3. **GitHub Search Filters**: Added `" has:releases sort:stars"` to search queries for better results

### Completion Features

- **Dynamic GitHub search**: Type 2+ characters to search repositories
- **Asset completion**: Tab-complete available release assets with `--asset` flag
- **Package name completion**: Tab-complete installed package names for `remove`, `update`, `status`
- **Interactive mode**: Use `-i` flag with `add` command for interactive asset selection

## Testing

```bash
# Test in bash
bash -c 'source <(deca completion bash) && complete -p deca'

# Test in nushell
nu -c 'deca _carapace nushell deca "" | from json'

# Test GitHub search
deca _carapace nushell deca add "vim" | from json
```

## Future Improvements

- Consider adding a `deca completion install` command that automatically configures shell completions
- Add support for more completion contexts (e.g., OS/arch values)

