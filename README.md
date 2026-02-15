# AutoBreaker

Adaptive circuit breaker for Go with percentage-based thresholds that automatically adjust to traffic patterns.

[![CI](https://github.com/1mb-dev/autobreaker/workflows/CI/badge.svg)](https://github.com/1mb-dev/autobreaker/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/1mb-dev/autobreaker.svg)](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
[![Go Report Card](https://goreportcard.com/badge/github.com/1mb-dev/autobreaker)](https://goreportcard.com/report/github.com/1mb-dev/autobreaker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-1mb-dev.github.io/autobreaker-blue)](https://1mb-dev.github.io/autobreaker/)

## Overview

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems: at high traffic, 10 failures may represent <1% error rate (too sensitive), while at low traffic, 10 failures may be 100% error rate (too slow to protect).

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. Configure once, works correctly across all traffic levels and environments.

**Features:**
- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **High Performance** - <100ns overhead per request, zero allocations
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - Lock-free atomic operations throughout

## Installation

```bash
go get github.com/1mb-dev/autobreaker
```

Requires Go 1.21 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/1mb-dev/autobreaker"
)

func main() {
    breaker := autobreaker.New(autobreaker.Settings{
        Name:    "api-client",
        Timeout: 10 * time.Second,
    })

    result, err := breaker.Execute(func() (interface{}, error) {
        return httpClient.Get("https://api.example.com/data")
    })

    if err == autobreaker.ErrOpenState {
        fmt.Println("Circuit open, using fallback")
        return
    }

    fmt.Printf("Result: %v\n", result)
}
```

## Documentation

**📚 Complete documentation available at: [1mb-dev.github.io/autobreaker/](https://1mb-dev.github.io/autobreaker/)**

The documentation includes:
- **Getting Started** - Installation, basic usage, configuration
- **Guides** - Architecture, state machine, concurrency, error classification, performance, decision guide
- **Migration** - From sony/gobreaker
- **API Reference** - Complete API documentation
- **Examples** - Production-ready code examples

### Quick Links
- [Getting Started Guide](https://1mb-dev.github.io/autobreaker/getting-started/)
- [Configuration Guide](https://1mb-dev.github.io/autobreaker/getting-started/#configuration)
- [API Reference](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
- [Examples](https://github.com/1mb-dev/autobreaker/tree/main/examples)

## Basic Configuration

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:    "service-name",
    Timeout: 30 * time.Second, // Open → HalfOpen transition time
})
```

Default behavior uses adaptive thresholds (5% failure rate, minimum 20 observations).

For advanced configuration, runtime updates, and complete settings reference, see the [Configuration Guide](https://1mb-dev.github.io/autobreaker/getting-started/#configuration).

## How It Works

AutoBreaker calculates failure rate as a percentage of recent requests:

```
Adaptive: "Trip when error rate > 5%"

At 100 RPS → trips at 50 failures (5% of 1000 req/10s)
At 10 RPS  → trips at 5 failures  (5% of 100 req/10s)

Same config, correct behavior at any traffic level.
```

The implementation uses a three-state machine (Closed → Open → HalfOpen → Closed) with lock-free atomic operations for minimal overhead.

## Performance

Benchmarks (Go 1.21, Apple M1):

| Operation          | Latency | Allocations |
|--------------------|---------|-------------|
| Execute (Closed)   | 78.5 ns | 0 allocs    |
| Execute (Open)     | 0.34 ns | 0 allocs    |
| UpdateSettings()   | 89.2 ns | 0 allocs    |

See [Performance Guide](https://1mb-dev.github.io/autobreaker/guides/performance/) for detailed analysis.

## Examples

Production-ready examples in [`examples/`](examples/):

- [**production_ready**](examples/production_ready/) - HTTP client integration, recommended starting point
- [**runtime_config**](examples/runtime_config/) - Dynamic configuration updates (file, API, signals)
- [**observability**](examples/observability/) - Monitoring, metrics, and diagnostics
- [**prometheus**](examples/prometheus/) - Prometheus collector integration
- [**adaptive**](examples/adaptive/) - Adaptive vs static threshold comparison
- [**custom_errors**](examples/custom_errors/) - Custom error classification

Run examples:
```bash
go run examples/production_ready/main.go
```

## Important Notes

**Callback Performance:** `ReadyToTrip`, `OnStateChange`, and `IsSuccessful` callbacks execute synchronously on every request. Keep them <1μs. Use goroutines for slow operations:

```go
OnStateChange: func(name string, from, to State) {
    go metrics.Record(name, from, to) // Async, non-blocking
}
```

**Compatibility:** Drop-in replacement for sony/gobreaker API. Enable adaptive thresholds with `AdaptiveThreshold: true`.

## Status

**Production-Ready** - v1.1.1

- 102 tests, 97.1% coverage, race detector clean
- Zero dependencies, zero allocations in hot path
- Compatible with Go 1.21+

See [Roadmap](https://1mb-dev.github.io/autobreaker/guides/roadmap/) for future plans.

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please open an issue to discuss changes before submitting PRs.

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for guidelines.
