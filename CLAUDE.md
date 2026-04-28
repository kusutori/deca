# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Deca** is a GitHub Release package manager written in Go. It downloads and manages binaries from GitHub Releases using a declarative TOML configuration file.

- **Module**: `github.com/kusutori/deca`
- **Entry**: `main.go` calls `cmd.Execute()`

## Build Commands

```bash
pixi run build      # Build with version info (outputs 'deca')
pixi run debug      # Build with debug symbols
pixi run test       # Run all tests
pixi run release    # Cross-platform build (linux/darwin/windows, amd64/arm64)
go test ./...       # Run all tests
```

## CLI Architecture

**Framework**: Cobra (`github.com/spf13/cobra`)

### Persistent Flags (root)
- `--config` - Config file path (default: `~/.config/deca/deca.toml`)
- `-v, --verbose` - Verbose output
- `--dry-run` - Preview without changes

### Commands

| Command | Purpose |
|---------|---------|
| `deca apply` | Install/update all packages from config |
| `deca add <repo>` | Add package (supports `--asset`, `-i` for interactive selection) |
| `deca remove <name>` | Remove package |
| `deca list` | List packages with status |
| `deca status` | Check for updates |
| `deca update [name]` | Update packages |
| `deca search <query>` | Search GitHub |
| `deca doctor` | Health check |
| `deca config` | View/edit config |
| `deca init` | Initialize config |

## Configuration

**Config**: `~/.config/deca/deca.toml`
**State**: `~/.local/state/deca/state.json` (JSON, tracks installed versions)
**Binaries**: `~/.local/bin` (Linux/macOS)

```toml
bin_dir = "$HOME/.local/bin"

[packages]
# Simple format
eza = "eza-community/eza"

# Full format
zellij = { repo = "zellij-org/zellij", asset = "*.deb", os = "linux", arch = "amd64" }
```

## Key Abstractions

### Download & Install Flow
```
cmd/add.go -> doInstall() -> installer.Install()
                              -> download.DownloadAndExtract()
                              -> github.FindMatchingAsset()
```

### Asset Priority
Native binary > tar.gz > .deb > AppImage > .rpm (system packages use apt/dnf)

### Special Handling
- **AppImage**: Copied directly, set executable bit
- **.deb/.rpm**: Installed via system package manager (requires sudo)
- **Progress bars**: Uses `schollz/progressbar/v3`

## Important Patterns

- **Parallel processing**: Uses `golang.org/x/sync/errgroup`
- **Error handling**: `fmt.Errorf("package: %w", err)` for context
- **Path traversal protection**: Validates extraction paths
- **Terminal detection**: `isatty.IsTerminal()` for color/progress

## Git Integration

Version info set at build time:
```bash
ldflags = "-X github.com/kusutori/deca/cmd.Version=$(git describe --tags --always)"
```

## Technical Debt & Known Issues

### 1. go-github library missing 'digest' field (HIGH PRIORITY)
**File**: `internal/github/api.go:60-67`

GitHub added SHA256 digest support for release assets in June 2025. The `go-github/v60` library (v60.0.0) doesn't yet include the `digest` field in `ReleaseAsset`.

**Current workaround**: Custom `GetAssetDigest()` function makes direct HTTP call to GitHub API.

**Proper fix needed**: Update `go-github` to a version that includes the `digest` field and use `asset.GetDigest()` instead of the custom API call.

**Tracking**: https://github.com/google/go-github/issues/XXXX

### 2. Windows support incomplete
- No native MSI/.exe silent install support
- No Scoop/Chocolatey integration
- Default bin_dir doesn't account for Windows paths

### 3. Binary size optimization
Static musl builds are ~15MB. Consider:
- UPX compression for releases
- Build with `-s -w` flags to strip debug info
