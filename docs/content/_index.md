---
title: "AutoBreaker"
weight: 1
---

# AutoBreaker

Circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

[![Go Reference](https://pkg.go.dev/badge/github.com/1mb-dev/autobreaker.svg)](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
[![CI](https://github.com/1mb-dev/autobreaker/workflows/CI/badge.svg)](https://github.com/1mb-dev/autobreaker/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/1mb-dev/autobreaker)](https://goreportcard.com/report/github.com/1mb-dev/autobreaker)

## Why AutoBreaker?

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems: at high traffic, 10 failures may represent <1% error rate (too sensitive), while at low traffic, 10 failures may be 100% error rate (too slow to protect).

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. One threshold works across traffic levels, as long as each `Interval` window holds at least `MinimumObservations` requests.

## Quick Start

```bash
go get github.com/1mb-dev/autobreaker
```

The Quick Start is in the [README](https://github.com/1mb-dev/autobreaker#quick-start); the same code is compiled as the package [`Example`](https://pkg.go.dev/github.com/1mb-dev/autobreaker#example-package).

## Key Features

- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - State and counts are atomic; all methods are safe for concurrent use

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
- [Examples](https://github.com/1mb-dev/autobreaker/tree/main/examples) - Production-ready examples
- [Contributing](https://github.com/1mb-dev/autobreaker/blob/main/.github/CONTRIBUTING.md) - How to contribute
- [Security](https://github.com/1mb-dev/autobreaker/security/policy) - Reporting a vulnerability
- [Changelog](/changelog/) - Release history

## Performance

Per-state latency and allocations, with the CPU and Go version they were measured on, are in the [Performance Guide](/guides/performance/).

## Examples

Production-ready examples in the [`examples/`](https://github.com/1mb-dev/autobreaker/tree/main/examples) directory:

- **production_ready** - HTTP client integration, recommended starting point
- **runtime_config** - Dynamic configuration updates (file, API, signals)
- **observability** - Monitoring, metrics, and diagnostics
- **adaptive** - Adaptive vs static threshold comparison
- **custom_errors** - Custom error classification

## License

MIT License - see [LICENSE](https://github.com/1mb-dev/autobreaker/blob/main/LICENSE) file.

## Contributing

Open an issue before submitting a change; see the [Contributing Guide](https://github.com/1mb-dev/autobreaker/blob/main/.github/CONTRIBUTING.md).
