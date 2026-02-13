# Shell Completion Migration to Carapace

This document describes the migration from Cobra's built-in completion to Carapace.

## Changes

- Added `github.com/carapace-sh/carapace` dependency
- Created `cmd/completion_carapace.go` with custom completers for:
  - `add`: GitHub repo search completion, asset completion
  - `remove`: installed package name completion
  - `update`: installed package name completion
  - `status`: installed package name completion
- Simplified `cmd/completion.go` to use carapace-bin instead of built-in generators

## Usage

1. Install carapace-bin:
   ```bash
   go install github.com/carapace-sh/carapace-bin@latest
   ```

2. Generate completion scripts:
   ```bash
   # Bash
   source <(carapace _deca bash)

   # Zsh
   source <(carapace _deca zsh)

   # Fish
   carapace _deca fish | source
   ```

   Or save to a file:
   ```bash
   carapace _deca bash > ~/.bash_completion.d/deca
   ```

## Known Issues

### carapace-bin detection of local builds

When running `carapace _deca`, it may not detect the local deca binary even when it's in PATH. This appears to be a caching or detection issue with carapace-bin.

Workaround: Use the full path to the deca binary:
```bash
carapace /full/path/to/deca bash
```

Or ensure deca is installed to a standard location (e.g., `~/bin` or `/usr/local/bin`) and carapace cache is cleared:
```bash
carapace --clear-cache
```

### Nushell completion

The original Nushell completion implementation was removed. Nushell users should use the standard carapace approach:
```bash
carapace _deca nushell | save -f ~/.cache/carapace/completions/deca.nu
```

## Future Improvements

- Consider adding a `deca completion install` command that writes completion scripts directly
- Investigate why carapace-bin doesn't detect local builds without full path
