# Deca Roadmap

## Vision

Deca is a declarative GitHub Release package manager that installs and updates CLI tools through a TOML config file.

---

## Completed ✓

### Core
- [x] Declarative configuration (short and full TOML package formats)
- [x] Config parsing for `bin_dir`, `[packages]`, `[settings]`
- [x] State management via JSON
- [x] Cross-platform support (Linux, macOS, Windows)
- [x] System detection (OS/Arch/distro/package manager)

### GitHub Integration
- [x] Fetch latest release information
- [x] Asset matching by OS/Arch
- [x] Glob pattern filtering
- [x] GitHub repository search
- [x] Asset priority: native > tar.gz > .deb > AppImage > .rpm
- [x] Linux matcher optimization (avoid false matches like freebsd/openbsd/netbsd)

### Download & Install
- [x] HTTP download for release assets
- [x] Download progress bar
- [x] Download cache
- [x] Secure tar.gz extraction (path traversal protection)
- [x] `.tar.xz` extraction support
- [x] zip extraction support
- [x] Binary install (copy to `bin_dir` + permissions)
- [x] AppImage support
- [x] System package support (`.deb` via apt, `.rpm` via dnf/yum, requires sudo)
- [x] State updates after install (including install type)
- [x] Temp directory lifecycle cleanup fixes
- [x] Smart uninstall by install type
- [x] Rollback on failed update

### CLI Commands
- [x] `deca apply`
- [x] `deca add`
- [x] `deca add -i/--interactive`
- [x] `deca add --asset`
- [x] `deca remove`
- [x] `deca remove -k/--keep-installed`
- [x] `deca list`
- [x] `deca status`
- [x] `deca update`
- [x] `deca search`
- [x] `deca doctor`
- [x] `deca init`
- [x] `deca config`
- [x] `deca --version`
- [x] `deca --dry-run`
- [x] `deca -v/--verbose`

### UX
- [x] Colored output
- [x] Colored help formatting
- [x] Download progress UI
- [x] Interactive numeric asset picker
- [x] Version display via git tags
- [x] Mirror management (`deca mirror`)
- [x] Desktop entry generation (`deca desktop`)
- [x] Auto desktop entry for AppImage when `desktop` block is configured
- [x] Shell completion (`deca completion`) with Carapace

### Testing & Quality
- [x] Unit tests for core packages
- [x] Integration tests for core command flows
- [x] Security tests for tar path traversal protections
- [x] Standardized build flow with Makefile
- [x] Basic GitHub Actions CI
- [x] Coverage reporting (target >80%)

---

## Planned 📋

### Asset Handling
- [x] `.tar.xz` support
- [x] Checksum verification (SHA256/SHA512)

### User Experience
- [x] Config diff display (`deca diff`)
- [x] Better error handling (error codes + friendlier hints)

### Config Enhancements
- [x] Version pinning
- [x] Conditional matching (`os` / `arch` expressions)
- [ ] Nested config groups (`[packages.github]`)
- [x] Versioned symlink support (`versioned = true`)

### Installation Enhancements
- [ ] Post-install hooks (custom scripts)
- [ ] Symbolic links (versioned symlink workflows)

### Ecosystem Integrations
- [ ] Template marketplace
- [ ] Webhook notifications for updates
- [ ] Self-update via deca itself

### Platform Integrations
- [ ] Homebrew import/migration
- [ ] Scoop import/migration
- [ ] Plugin system for custom installers

---

## Tech Debt

### Code Quality
- [ ] Add E2E tests

### Documentation
- [ ] API docs
- [x] Contribution guide (AGENTS.md)
- [ ] Plugin development docs

---

## Version Plan

### v0.1.0 ✓ (current)
- [x] Core functionality available
- [x] Unit tests
- [x] README
- [x] System package support (.deb/.rpm)
- [x] Interactive asset selection
- [x] Download progress bar
- [x] init/config commands
- [x] Colored output and help
- [x] Linux asset match optimization

### v0.2.0 ✓ (short term)
- [x] Improved download/extraction (`.tar.xz`)
- [x] Better error handling
- [x] Config diff
- [x] Checksum verification

### v0.3.0 (mid term)
- [x] Versioned symlink
- [ ] Multi-config support
- [ ] Post-install hooks
- [ ] Community templates

### v1.0.0 (long term)
- [ ] Stable API
- [ ] Complete documentation
- [ ] Broader test coverage

---

## Next Steps (Current)

### P1 (near term)
- [ ] Post-install hooks (pre/post install)
- [ ] Multi-config support (profiles)
- [x] Versioned symlink
- [ ] Improve Windows install UX (MSI/EXE logic + paths)

### P2 (mid term)
- [ ] E2E tests (isolated real download/install scenarios)
- [ ] Self-update
- [ ] Homebrew/Scoop import
- [ ] Template marketplace/community templates

### P3 (long term)
- [ ] Plugin system

---

## Contributing

Contributions are welcome. Please check GitHub Issues for open work.

### Development Flow
1. Fork the project
2. Create a feature branch
3. Add tests
4. Ensure all tests pass
5. Open a PR

### Code Style
- Follow standard Go conventions
- Keep functions concise
- Add necessary comments
