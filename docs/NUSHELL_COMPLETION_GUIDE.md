# Nushell Completion Integration Guide

This document explains how to properly integrate carapace-based completions with Nushell, based on our experience implementing completions for deca.

## The Problem

When implementing shell completions using carapace, the integration with Nushell requires specific configuration that isn't immediately obvious. Simply having a `_carapace` subcommand and generating completion scripts isn't enough.

## How Nushell Completions Work

### 1. Extern Declarations (Not Sufficient Alone)

The `extern` keyword in Nushell declares an external command's signature:

```nushell
extern "deca" [
  ...args: string@"nu-complete deca"
]
```

**Important**: This declaration alone does NOT trigger completions. It only tells Nushell about the command's signature.

### 2. External Completer (Required)

Nushell uses an **external completer** configured in `$env.config.completions.external.completer`. This is a closure that:
- Takes the current command line as input (`spans`)
- Returns completion suggestions as JSON
- Runs when no built-in Nushell completions are found

## Integration Steps

### Step 1: Implement `_carapace` Subcommand

Your CLI tool should support the `_carapace` subcommand that outputs completions in carapace's JSON format:

```bash
$ deca _carapace nushell deca ""
[{"value":"add ","display":"add","description":"Add a package"}]
```

### Step 2: Configure External Completer

In your `~/.config/nushell/config.nu`, configure the external completer:

```nushell
$env.config.completions = {
    case_sensitive: false
    quick: true
    partial: true
    algorithm: "fuzzy"
    external: {
        enable: true
        max_results: 100
        completer: {|spans|
            let cmd = $spans.0
            # For commands with built-in carapace support
            if ($cmd == "deca") {
                try {
                    deca _carapace nushell ...$spans | from json
                } catch {
                    null
                }
            } else {
                # For other commands, use carapace if installed
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

### Step 3: No Need for Separate Completion Files

With the external completer configured, you **don't need** to source separate completion files like `deca.nu`. The external completer handles everything.

## Common Pitfalls

### 1. Module Name Conflict (Nushell 0.110+)

**Problem**: Using `export extern "deca"` in a file named `deca.nu` causes a module name conflict.

**Solution**: Don't use `export` keyword, or don't use separate completion files at all (use external completer instead).

### 2. Using `c.Args` Instead of `c.Value` in Carapace

**Problem**: In carapace's `ActionCallback`, `c.Args` contains completed arguments, not the current input.

**Solution**: Use `c.Value` to get the current input being typed:

```go
carapace.ActionCallback(func(c carapace.Context) carapace.Action {
    query := c.Value  // Current input, not c.Args[0]
    // ...
})
```

### 3. Missing Search Filters

**Problem**: GitHub API searches without filters return empty results.

**Solution**: Add appropriate filters to your search queries:

```go
repos, err := ghClient.SearchRepositories(ctx, query+" has:releases sort:stars")
```

### 4. Empty External Completer

**Problem**: Having `external.completer: null` or commenting it out disables all external completions.

**Solution**: Always configure a proper external completer closure, even if it just returns `null` for unsupported commands.

## Testing Completions

### Test in Bash (Quick Verification)

```bash
bash -c 'source <(deca completion bash) && export COMP_LINE="deca " COMP_POINT=5 && deca _carapace bash deca ""'
```

### Test in Nushell

```nushell
# Reload config
source ~/.config/nushell/config.nu

# Test completions
deca <TAB>          # Should show subcommands
deca add vi<TAB>    # Should search GitHub (after 2+ chars)
deca remove <TAB>   # Should show installed packages
```

### Debug External Completer

```nushell
# Check if external completer is configured
$env.config.completions.external.completer

# Test carapace directly
deca _carapace nushell deca "" | from json
```

## Architecture Summary

```
User presses TAB
    ↓
Nushell checks for built-in completions
    ↓
No built-in found → calls external completer
    ↓
External completer checks command name
    ↓
If "deca" → calls: deca _carapace nushell ...
If other → calls: carapace <cmd> nushell ...
    ↓
Returns JSON completion data
    ↓
Nushell displays completions
```

## Benefits of This Approach

1. **No separate completion files needed** - Everything is handled by the external completer
2. **Works with carapace ecosystem** - Other carapace-enabled tools work automatically
3. **Fallback support** - Commands without carapace support fall back to file completion
4. **Single source of truth** - Completion logic lives in your CLI tool, not in shell-specific files

## References

- [Nushell Custom Completions](https://www.nushell.sh/book/custom_completions.html)
- [Carapace Documentation](https://carapace-sh.github.io/carapace/)
- [Nushell External Completers](https://www.nushell.sh/cookbook/external_completers.html)
