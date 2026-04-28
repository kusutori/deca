# Deca - GitHub Release Package Manager

Deca is a declarative package manager for command-line tools. It downloads binaries from GitHub Releases and manages install/update state through a TOML config file.

## Features

- Declarative config in TOML
- Multiple asset formats: binaries, tar.gz/tar.xz/zip, .deb/.rpm, AppImage
- Interactive asset selection
- State tracking (installed version and timestamp)
- Download cache to avoid repeated downloads
- GitHub mirror support
- Cross-platform: Linux, macOS, Windows
- Colored output and help text

## Installation

```bash
# Build from source
git clone https://github.com/kusutori/deca.git
cd deca
make build
./deca init

# Or move to a global bin dir
sudo mv deca /usr/local/bin/
```

## Quick Start

```bash
# Initialize config (auto-detect system info)
deca init

# Add and install packages
deca add eza-community/eza
deca add sharkdp/bat

# Interactive asset selection
deca add sourcegit-scm/sourcegit --interactive

# Select specific asset pattern
deca add sourcegit-scm/sourcegit --asset "*.deb"

# Show installed/configured packages
deca list
deca config show

# Check updates
deca status

# Update all packages
deca update

# Search GitHub repositories
deca search fd

# Mirror management
deca mirror list
deca mirror select

# Cache management
deca cache
deca cache list
deca cache clean --orphans
```

## Configuration

Create `~/.config/deca/deca.toml`:

```toml
bin_dir = "$HOME/.local/bin"

[packages]
# Short format
eza = "eza-community/eza"
bat = "sharkdp/bat"

# Full format
zellij = { repo = "zellij-org/zellij", asset = "*.deb" }
neovim = { repo = "neovim/neovim", version = "0.10.0" }

[settings]
auto_update = true
check_interval = "24h"
```

## Commands

| Command | Description |
|------|------|
| `deca init` | Initialize config and detect system info |
| `deca apply` | Apply config and install/update all packages |
| `deca add <owner/repo>` | Add and install a package |
| `deca add <owner/repo> -i` | Interactive asset selection |
| `deca add <owner/repo> --asset "*.deb"` | Use asset pattern |
| `deca add <owner/repo> --no-install` | Add config only |
| `deca remove <name>` | Uninstall and remove from config |
| `deca remove <name> -k` | Remove from config only |
| `deca list` | List packages and status |
| `deca config show` | Show config and install state |
| `deca config diff` | Show config differences |
| `deca config` | Edit config file |
| `deca status` | Check updates |
| `deca update [name]` | Update one/all packages |
| `deca search <query>` | Search GitHub |
| `deca doctor` | Health checks |
| `deca cache` | Show cache status |
| `deca cache list` | List cached files |
| `deca cache size` | Show cache size |
| `deca cache clean` | Clean cache |
| `deca schema` | Generate JSON Schema for `deca.toml` |
| `deca mirror` | Show active mirror |
| `deca mirror list` | List mirrors |
| `deca mirror select` | Select mirror interactively |
| `deca mirror add <name> <url>` | Add custom mirror |
| `deca mirror remove <name>` | Remove custom mirror |
| `deca --version` | Show version |
| `deca --dry-run` | Preview without executing |
| `deca -v/--verbose` | Verbose output |

## Shell Completion

Generate completion scripts with `deca completion <shell>` (Carapace based):

### Bash
```bash
deca completion bash > /etc/bash_completion.d/deca
```

### Zsh
```bash
deca completion zsh > "${fpath[1]}/_deca"
```

### Fish
```bash
deca completion fish > ~/.config/fish/completions/deca.fish
```

### PowerShell
```powershell
deca completion powershell | Out-String | Invoke-Expression
```

### Nushell
```nu
deca completion nushell | save -f ~/.config/nushell/completions/deca.nu
```
Add this to `config.nu`:
```nu
source ~/.config/nushell/completions/deca.nu
```

## Supported Package Formats

| Format | Handling |
|------|------|
| tar.gz/tar.xz/zip | Auto extract binary |
| .deb | Install via apt (sudo required) |
| .rpm | Install via dnf/yum (sudo required) |
| AppImage | Copy directly and mark executable |
| Single binary | Copy directly |

## Download Cache

Downloaded files are cached at `~/.cache/deca`.

```bash
deca cache
deca cache list
deca cache size
deca cache clean --orphans
deca cache clean --all
```

## Mirrors

Useful for faster downloads in some regions:

```bash
deca mirror list
deca mirror select
deca mirror
deca mirror add "My Mirror" https://mirror.example.com
```

## State Management

Install state is stored at `~/.local/state/deca/state.json`:

```json
{
  "packages": {
    "eza": {
      "repo": "eza-community/eza",
      "version": "v0.18.0",
      "asset_name": "eza-x86_64-unknown-linux-musl.tar.gz",
      "installed_at": "2024-01-15T10:30:00Z"
    }
  }
}
```

## Paths

| Type | Linux/macOS | Windows |
|------|-------------|---------|
| Config | `~/.config/deca/deca.toml` | `%APPDATA%\\deca\\deca.toml` |
| State | `~/.local/state/deca/state.json` | `%LOCALAPPDATA%\\deca\\state.json` |
| Cache | `~/.cache/deca` | `%LOCALAPPDATA%\\deca\\cache` |
| Binary dir | `~/.local/bin` | `%LOCALAPPDATA%\\deca\\bin` |
| Mirror config | `~/.config/deca/mirrors.toml` | `%APPDATA%\\deca\\mirrors.toml` |

## Global Options

| Option | Description |
|------|------|
| `--config <path>` | Specify config file |
| `--dry-run` | Preview without executing |
| `-v, --verbose` | Verbose output |

## GitHub Token

For private repositories or higher GitHub API limits:

```bash
export GITHUB_TOKEN="your_token_here"
```

## Python Bindings (gopy)

Go package exposed for gopy: `github.com/kusutori/deca/pkg/pydeca`.

### 1) Install generator tools

```bash
just setup-python-bindings
```

### 2) Generate Python package

```bash
just python-bindings
python3 -m pip install -e bindings/python
```

### 3) Use from Python

```python
from deca_py import pydeca

schema_json = pydeca.GenerateSchemaJSON()
print(schema_json[:80])

c = pydeca.NewDefaultClient()
code = c.Run(["status"])
print("exit:", code)
```

## Build

```bash
make build
make test
make release
make static
make musl-release
make clean
```

### glibc Compatibility

Default builds require glibc 2.32+, which may fail on older systems (e.g., Ubuntu 18.04, CentOS 7).

Recommended: static build with musl.

```bash
sudo apt install musl-tools
make static
```

## Project Structure

```text
deca/
├── cmd/              # CLI commands
├── internal/         # Internal modules
│   ├── config/
│   ├── github/
│   ├── download/
│   ├── install/
│   ├── cache/
│   └── ui/
├── Makefile
└── README.md
```

## License

MIT
