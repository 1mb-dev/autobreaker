---
title: "Getting Started"
weight: 1
---

# Getting Started with AutoBreaker

AutoBreaker is a circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

## Why AutoBreaker?

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems:

- **High traffic**: 10 failures may represent <1% error rate (too sensitive)
- **Low traffic**: 10 failures may be 100% error rate (too slow to protect)

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. One threshold works across traffic levels, as long as each `Interval` window holds at least `MinimumObservations` requests.

## Installation

```bash
go get github.com/1mb-dev/autobreaker
```

Requires Go 1.26 or later.

## Quick Start

The Quick Start is in the [README](https://github.com/1mb-dev/autobreaker#quick-start); the same code is compiled as the package [`Example`](https://pkg.go.dev/github.com/1mb-dev/autobreaker#example-package).

## How It Works

With `AdaptiveThreshold: true`, AutoBreaker trips when the failure rate in the current window exceeds the threshold. Counts reset on every state change, and every `Interval` if one is set: a fixed window, not a sliding one.

```
AdaptiveThreshold: true, Interval: 10s, threshold 5% (default)

At 100 RPS (1000 successes in the window) → trips on the 53rd failure
At 10 RPS  (100 successes in the window)  → trips on the 6th failure
```

With `Interval: 0`, counts reset only on state changes, so a long healthy run dilutes later failures: after 100,000 successes it takes 5,264 failures to trip.

The rate is checked only once a window holds `MinimumObservations` requests (default 20), with no consecutive-failure fallback. At `Interval: 10s`, a service under 2 RPS never trips, even if every request fails; lengthen `Interval` or lower `MinimumObservations` so one window sees enough requests.

## Key Features

- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - Lock-free atomic operations throughout

## Next Steps

- [Configuration](#configuration) - Detailed settings and options
- [Examples](#examples) - Production-ready code examples
- [API Reference](https://pkg.go.dev/github.com/1mb-dev/autobreaker) - Complete API documentation
- [Migration Guide](/migration/) - Moving from sony/gobreaker

## Configuration

For detailed configuration options and advanced settings, see the [Configuration Guide](/getting-started/configuration/).

## Examples

Production-ready code examples are available in the [examples directory](https://github.com/1mb-dev/autobreaker/tree/main/examples) on GitHub.
