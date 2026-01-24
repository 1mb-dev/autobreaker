---
title: "Contributing"
weight: 7
---

# Contributing

We welcome contributions to AutoBreaker! Please follow these guidelines to ensure smooth collaboration.

## Development Workflow

1. **Fork the repository** on GitHub
2. **Create a feature branch** from `main`
3. **Make your changes** with tests
4. **Run all tests** to ensure nothing breaks
5. **Submit a pull request** with a clear description

## Code Standards

### Go Code
- Follow standard Go conventions
- Use `go fmt` for formatting
- Run `go vet` and `staticcheck` for linting
- Maintain 95%+ test coverage
- Keep the hot path allocation-free

### Documentation
- Use clear, concise language
- Include code examples where helpful
- Update README.md if API changes
- Add comments for public APIs

### Testing
- Write unit tests for new functionality
- Include edge case tests
- Run tests with race detector: `go test -race ./...`
- Ensure benchmarks don't regress

## Pull Request Process

1. **Title**: Clear, descriptive title
2. **Description**: What changes, why, and how tested
3. **Tests**: All tests pass, coverage maintained
4. **Documentation**: Updated if needed
5. **Review**: Address feedback promptly

## Release Process

Releases follow semantic versioning:

- **Major (X.0.0)**: Breaking API changes
- **Minor (1.X.0)**: New features, backward compatible
- **Patch (1.0.X)**: Bug fixes, documentation

## Getting Help

- **Issues**: Use GitHub Issues for bug reports
- **Discussions**: GitHub Discussions for questions
- **Code Review**: PR feedback from maintainers

## Code of Conduct

Be respectful and constructive. We follow the [Go Community Code of Conduct](https://golang.org/conduct).

Thank you for contributing to AutoBreaker!
