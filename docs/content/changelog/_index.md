---
title: "Changelog"
weight: 9
---

# Changelog

All notable changes to AutoBreaker are documented in this file.

## [Unreleased]

### Added
- Hugo-based documentation site
- GitHub Pages deployment
- Mermaid diagram support

## [1.0.0] - 2023-11-10

### Added
- Initial release
- Adaptive percentage-based thresholds
- Runtime configuration updates
- Zero-dependency implementation
- High performance (<100ns overhead)
- Complete test suite (97.1% coverage)

### Features
- Three-state circuit breaker (Closed, Open, HalfOpen)
- Lock-free atomic operations
- Custom error classification
- Observability APIs (Metrics, Diagnostics)
- Production-ready examples

## [0.1.0] - 2023-10-15

### Added
- Initial beta release
- Basic circuit breaker functionality
- Compatibility with sony/gobreaker API
- Adaptive threshold prototype

## Versioning

AutoBreaker follows [Semantic Versioning](https://semver.org/):

- **Major (X.0.0)**: Breaking API changes
- **Minor (1.X.0)**: New features, backward compatible  
- **Patch (1.0.X)**: Bug fixes, documentation

## Upgrading

### From 0.x to 1.0
- No breaking changes
- API fully stable
- Ready for production use

### From sony/gobreaker
See [Migration Guide](/migration/) for details.

## Links

- **GitHub Releases**: [https://github.com/1mb-dev/autobreaker/releases](https://github.com/1mb-dev/autobreaker/releases)
- **Full CHANGELOG**: [CHANGELOG.md](https://github.com/1mb-dev/autobreaker/blob/main/CHANGELOG.md)
