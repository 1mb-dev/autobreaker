---
title: "AutoBreaker"
weight: 1
---

# AutoBreaker

Circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

[![Go Reference](https://pkg.go.dev/badge/github.com/1mb-dev/autobreaker.svg)](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
[![CI](https://github.com/1mb-dev/autobreaker/workflows/CI/badge.svg)](https://github.com/1mb-dev/autobreaker/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/1mb-dev/autobreaker)](https://goreportcard.com/report/github.com/1mb-dev/autobreaker)

See [Why AutoBreaker](https://github.com/1mb-dev/autobreaker#why-autobreaker) in the README.

## Quick Start

```bash
go get github.com/1mb-dev/autobreaker
```

The Quick Start is in the [README](https://github.com/1mb-dev/autobreaker#quick-start); the same code is compiled as the package [`Example`](https://pkg.go.dev/github.com/1mb-dev/autobreaker#example-package).

## Documentation

### Getting Started
- [Installation and basic usage](/getting-started/)
- [Configuration options](/getting-started/#configuration)

### Guides
- [State Machine](/guides/state-machine/) - State transition specification
- [Concurrency](/guides/concurrency/) - Lock-free implementation details
- [Error Classification](/guides/error-classification/) - Custom error handling
- [Performance](/guides/performance/) - Benchmarks and optimization
- [Decision Guide](/guides/decision-guide/) - When to use AutoBreaker
- [Migration](/migration/) - From sony/gobreaker

### Reference
- [API Reference](https://pkg.go.dev/github.com/1mb-dev/autobreaker) - Complete API documentation
- [Examples](https://github.com/1mb-dev/autobreaker/tree/main/examples) - Runnable examples
- [Contributing](https://github.com/1mb-dev/autobreaker/blob/main/.github/CONTRIBUTING.md) - How to contribute
- [Security](https://github.com/1mb-dev/autobreaker/security/policy) - Reporting a vulnerability
- [Changelog](/changelog/) - Release history

## Performance

Per-state latency and allocations, with the CPU and Go version they were measured on, are in the [Performance Guide](/guides/performance/).

## Examples

Examples in the [`examples/`](https://github.com/1mb-dev/autobreaker/tree/main/examples) directory:

- **production_ready** - HTTP client integration, recommended starting point
- **runtime_config** - Dynamic configuration updates (file, API, signals)
- **observability** - Monitoring, metrics, and diagnostics
- **adaptive** - Adaptive vs static threshold comparison
- **custom_errors** - Custom error classification

## License

MIT License - see [LICENSE](https://github.com/1mb-dev/autobreaker/blob/main/LICENSE) file.

## Contributing

Open an issue before submitting a change; see the [Contributing Guide](https://github.com/1mb-dev/autobreaker/blob/main/.github/CONTRIBUTING.md).
