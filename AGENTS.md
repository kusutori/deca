# Repository Guidelines

## Project Structure & Module Organization
Deca is a Go CLI. The entry point is `main.go`, which calls `cmd.Execute()`. Core layout:

- `cmd/`: Cobra CLI commands and flags (e.g., `cmd/root.go`).
- `internal/`: Non-exported application logic (cache, config, download, github, install, ui).
- `pkg/`: Shared utilities that may be imported by other modules.
- `deca`: Built binary output from `pixi run build` (ignored in git).
- `README.md` and `CLAUDE.md`: user and contributor guidance.

## Build, Test, and Development Commands
Use Pixi tasks for consistent builds:

- `pixi run build`: compile `deca` with version info.
- `pixi run debug`: build with debug symbols.
- `pixi run test`: run all tests.
- `pixi run static`: fully static Linux build (requires `musl-gcc`).
- `pixi run release` / `pixi run musl-release`: cross-platform release binaries.
- `pixi run info`: print build metadata.

## Coding Style & Naming Conventions
This repo follows standard Go conventions:

- Formatting: `gofmt` on all `.go` files.
- Naming: exported identifiers use `CamelCase`, packages use short lowercase names.
- Errors: wrap with context (`fmt.Errorf("pkg: %w", err)`).
- Files: tests live alongside source as `*_test.go`.

## Testing Guidelines
Tests use Go’s standard `testing` package. Run all tests with:

```bash
pixi run test
```

Prefer table-driven tests and deterministic fixtures. Name test files and functions with the Go convention (`*_test.go`, `TestXxx`).

## Commit & Pull Request Guidelines
Git history suggests conventional prefixes: `feat:`, `fix:`, `docs:` (also occasional capitalized summaries). Use one of these when possible and keep subjects short and imperative.

For PRs:

- Describe the behavior change and include a brief rationale.
- Link related issues if applicable.
- Include CLI output or screenshots for user-facing changes.
- Call out platform-specific behavior (Linux/macOS/Windows).

## Security & Configuration Tips
User config lives at `~/.config/deca/deca.toml`; state is stored in `~/.local/state/deca/state.json`. Avoid logging secrets or tokens, and keep any new config options documented in `README.md`.
