## Summary

- [ ] Describe the behavior change.
- [ ] Add rationale and related issue(s).

## Coverage

- Before (base branch total): <!-- e.g. 78.20% -->
- After (this PR total): <!-- e.g. 80.10% -->
- Affected packages: <!-- e.g. internal/config, internal/install -->

## Checklist

- [ ] `go test ./... -coverprofile=coverage.out`
- [ ] `go tool cover -func=coverage.out`
- [ ] Coverage does not drop compared with base branch.
- [ ] For critical packages, coverage is >= 85% (`internal/config`, `internal/install`).
