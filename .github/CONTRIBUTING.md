# Contributing to AutoBreaker

AutoBreaker is a small, stable v1 library in maintenance: bug and security fixes, and additions that change no existing behavior. Open an issue before writing code, so we can agree on the change first.

To report a vulnerability, see the [security policy](https://github.com/1mb-dev/autobreaker/security/policy); do not open a public issue.

## Setup

Requires Go 1.26 or later.

```bash
make setup        # once per clone: installs the pre-commit hook
make check        # fmt-check, vet, golangci-lint
make test-short   # fast loop
make test-race    # full suite with the race detector; run before a PR
```

## Rules

- **No dependencies.** The root module imports the standard library only.
- **No breaking changes.** Exported signatures and default behavior are fixed within v1.
- **Tests with the change.** CI requires 95% coverage on `internal/...`; `Execute` must not allocate (`TestExecuteDoesNotAllocate`).
- **Docs with the change.** A fact stated in the README, godoc and `docs/content` changes in all of them. The README Quick Start must match `Example` in `example_test.go`.

## Commits and pull requests

Commit messages follow `<type>(<scope>): <subject>` with types `feat`, `fix`, `docs`, `test`, `perf`, `refactor` and `chore`. One concern per commit, one concern per pull request. CI must be green before review.

## Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md).
