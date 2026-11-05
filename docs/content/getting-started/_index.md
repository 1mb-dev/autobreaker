---
title: "Getting Started"
weight: 1
---

# Getting Started with AutoBreaker

AutoBreaker is a circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

See [Why AutoBreaker](https://github.com/1mb-dev/autobreaker#why-autobreaker) in the README.

## Installation

```bash
go get github.com/1mb-dev/autobreaker
```

Requires Go 1.26 or later.

## Quick Start

The Quick Start is in the [README](https://github.com/1mb-dev/autobreaker#quick-start); the same code is compiled as the package [`Example`](https://pkg.go.dev/github.com/1mb-dev/autobreaker#example-package).

## How It Works

The README explains the window, `Interval: 0` and low-traffic behavior: see [How It Works](https://github.com/1mb-dev/autobreaker#how-it-works).

## Key Features

- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - State and counts are atomic; all methods are safe for concurrent use

## Next Steps

- [Configuration](#configuration) - Detailed settings and options
- [Examples](#examples) - Production-ready code examples
- [API Reference](https://pkg.go.dev/github.com/1mb-dev/autobreaker) - Complete API documentation
- [Migration Guide](/migration/) - Moving from sony/gobreaker

## Configuration

For detailed configuration options and advanced settings, see the [Configuration Guide](/getting-started/configuration/).

## Examples

Production-ready code examples are available in the [examples directory](https://github.com/1mb-dev/autobreaker/tree/main/examples) on GitHub.
