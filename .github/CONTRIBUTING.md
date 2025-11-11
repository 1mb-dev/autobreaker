# Contributing to AutoBreaker

Thank you for your interest in contributing to AutoBreaker! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, professional, and constructive. We're all here to build great software together.

## How to Contribute

### Reporting Bugs

Before creating a bug report:

1. **Search existing issues** to avoid duplicates
2. **Use the latest version** to ensure the bug hasn't been fixed
3. **Provide a minimal reproduction** if possible

A good bug report includes:

- **Clear title**: Concise description of the issue
- **Environment**: Go version, OS, AutoBreaker version
- **Steps to reproduce**: Minimal code to demonstrate the bug
- **Expected behavior**: What you expected to happen
- **Actual behavior**: What actually happened
- **Workaround**: If you've found one

Example:
```markdown
### Circuit breaker doesn't open with adaptive threshold

**Environment:**
- Go 1.23
- AutoBreaker v1.0.0
- macOS 14.0

**Reproduction:**
`​``go
breaker := autobreaker.New(autobreaker.Settings{
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.05,
    MinimumObservations:  10,
})

// Send 20 requests with 10% failure rate...
`​``

**Expected:** Circuit should open at 5% failure rate
**Actual:** Circuit remains closed at 10% failure rate
```

### Requesting Features

Before requesting a feature:

1. **Check existing issues** for similar requests
2. **Consider if it aligns** with AutoBreaker's philosophy (lean, focused, meaningful)
3. **Propose an API** if you have ideas on implementation

A good feature request includes:

- **Use case**: Why you need this feature
- **Proposed API**: How you'd like to use it
- **Alternatives**: What workarounds exist
- **Impact**: How it benefits other users

### Contributing Code

We welcome pull requests! Here's the process:

#### 1. Fork and Clone

```bash
git clone https://github.com/YOUR_USERNAME/autobreaker.git
cd autobreaker
```

#### 2. Create a Branch

```bash
git checkout -b feature/my-awesome-feature
# or
git checkout -b fix/circuit-not-opening
```

Use descriptive branch names:
- `feature/` for new features
- `fix/` for bug fixes
- `docs/` for documentation
- `test/` for test improvements
- `perf/` for performance improvements

#### 3. Make Changes

Follow these guidelines:

**Code Style:**
- Run `go fmt` on all code
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use meaningful variable names
- Add comments for complex logic

**Testing:**
- Add tests for new features
- Ensure all tests pass: `go test ./...`
- Run with race detector: `go test -race ./...`
- Maintain 98%+ code coverage

**Documentation:**
- Add godoc comments for public APIs
- Update README if needed
- Add examples for new features
- Update CHANGELOG.md

#### 4. Test Thoroughly

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. -benchmem ./internal/breaker

# Run stress tests (optional, takes time)
go test -v ./internal/breaker -run TestStress
```

#### 5. Commit

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions/changes
- `perf`: Performance improvements
- `refactor`: Code refactoring (no behavior change)
- `ci`: CI/CD changes
- `chore`: Maintenance tasks

Examples:
```
feat: add ExecuteContext for context support

Implements context-aware execution with proper cancellation handling.
Context cancellation is not counted as failure.

Closes #42
```

```
fix: circuit not opening with very low traffic

Adaptive threshold now correctly applies at low request rates.
Added test case for 1 req/sec scenario.

Fixes #123
```

#### 6. Push and Create PR

```bash
git push origin feature/my-awesome-feature
```

Create a pull request with:

- **Clear title**: Describe the change
- **Description**: Explain what and why
- **Related issues**: Link to issues it fixes
- **Testing**: Describe how you tested
- **Checklist**: Confirm all requirements met

PR Template:
```markdown
## Description
Brief description of changes

## Motivation
Why this change is needed

## Changes
- Change 1
- Change 2

## Testing
How this was tested

## Checklist
- [ ] Tests pass (`go test ./...`)
- [ ] Race detector clean (`go test -race ./...`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Examples added (if new feature)
```

### Review Process

1. **CI checks**: Must pass before review
2. **Code review**: Maintainer will review your code
3. **Feedback**: Address any comments or questions
4. **Approval**: PR will be merged after approval

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- (Optional) golangci-lint for linting

### Install Dependencies

```bash
# Clone repository
git clone https://github.com/vnykmshr/autobreaker.git
cd autobreaker

# Download dependencies
go mod download

# Run tests
go test ./...
```

### Project Structure

```
autobreaker/
├── autobreaker.go           # Public API
├── internal/
│   └── breaker/
│       ├── circuitbreaker.go    # Core implementation
│       ├── types.go             # Type definitions
│       ├── state.go             # State machine
│       ├── counts.go            # Request counting
│       ├── adaptive.go          # Adaptive threshold logic
│       ├── metrics.go           # Metrics API
│       ├── diagnostics.go       # Diagnostics API
│       ├── update.go            # Runtime configuration
│       └── *_test.go            # Tests
├── examples/                # Usage examples
├── .github/
│   └── workflows/          # CI/CD
└── docs/                   # Documentation
```

### Running CI Locally

```bash
# Run tests like CI does
go test -v -race -coverprofile=coverage.out ./...

# Run linters
golangci-lint run

# Check formatting
gofmt -l .

# Run benchmarks
go test -bench=. -benchmem -run=^$ ./internal/breaker
```

## Philosophy and Design Principles

When contributing, keep these principles in mind:

### 1. Lean
- No bloat, minimal additions
- Every line of code has a purpose
- Resist feature creep

### 2. Focused
- Production-ready quality
- No speculative features
- Solve real problems

### 3. Meaningful
- Each change adds genuine value
- Not just "nice to have"
- Benefits multiple users

### 4. Performance
- <100ns overhead in hot path
- Zero allocations where possible
- Lock-free where practical

### 5. Backward Compatible
- v1.x stability promise
- No breaking changes
- Additive only

## What We're Looking For

### High Priority

- **Bug fixes**: Correctness is paramount
- **Performance improvements**: With benchmarks proving benefit
- **Documentation**: Examples, guides, API docs
- **Test improvements**: Higher coverage, edge cases

### Medium Priority

- **New examples**: Common use cases
- **Integration guides**: Frameworks, monitoring tools
- **Tooling**: Developer experience improvements

### Low Priority (Discuss First)

- **New core features**: Must align with philosophy
- **API changes**: Very carefully considered
- **Dependencies**: We have zero, want to keep it that way

### Not Accepting

- **Breaking changes**: Not in v1.x
- **External dependencies**: Stdlib only
- **Speculative features**: Without clear use case
- **"Nice to have"**: Must be "need to have"

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Issues**: Use GitHub Issues for bugs and feature requests

## Recognition

Contributors are recognized in:
- Git commit history
- Release notes
- CHANGELOG.md (for significant contributions)

Thank you for contributing to AutoBreaker! 🚀
