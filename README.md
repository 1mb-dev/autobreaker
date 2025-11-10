# AutoBreaker

Adaptive circuit breaker for Go with percentage-based thresholds that automatically adjust to traffic patterns.

[![CI](https://github.com/vnykmshr/autobreaker/workflows/CI/badge.svg)](https://github.com/vnykmshr/autobreaker/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/autobreaker.svg)](https://pkg.go.dev/github.com/vnykmshr/autobreaker)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/autobreaker)](https://goreportcard.com/report/github.com/vnykmshr/autobreaker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Overview

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems: at high traffic, 10 failures may represent <1% error rate (too sensitive), while at low traffic, 10 failures may be 100% error rate (too slow to protect).

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. Configure once, works correctly across all traffic levels and environments.

**Features:**
- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **High Performance** - <100ns overhead per request, zero allocations
- **Rich Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - Lock-free atomic operations throughout

## Installation

```bash
go get github.com/vnykmshr/autobreaker
```

Requires Go 1.21 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/vnykmshr/autobreaker"
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

## Configuration

### Basic Settings

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:    "service-name",
    Timeout: 30 * time.Second, // Open → HalfOpen transition time
})
```

Default behavior uses adaptive thresholds (5% failure rate, minimum 20 observations).

### Advanced Configuration

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "service-name",
    Timeout:              30 * time.Second,
    MaxRequests:          3,     // Concurrent requests in half-open state
    Interval:             60 * time.Second, // Stats reset interval

    // Adaptive threshold settings
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.05,  // Trip at 5% error rate
    MinimumObservations:  20,    // Minimum requests before evaluating

    // Callbacks (must be fast: <1μs)
    OnStateChange: func(name string, from, to autobreaker.State) {
        go log.Printf("%s: %v → %v", name, from, to) // Async
    },

    IsSuccessful: func(err error) bool {
        // Don't count 4xx client errors as failures
        if httpErr, ok := err.(*HTTPError); ok {
            return httpErr.Code >= 400 && httpErr.Code < 500
        }
        return err == nil
    },
})
```

### Runtime Updates

```go
err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
    FailureRateThreshold: autobreaker.Float64Ptr(0.10),
    Timeout:              autobreaker.DurationPtr(15 * time.Second),
})
```

## Observability

### Metrics

```go
metrics := breaker.Metrics()
fmt.Printf("State: %v, Requests: %d, Failure Rate: %.2f%%\n",
    metrics.State, metrics.Requests, metrics.FailureRate*100)
```

### Diagnostics

```go
diag := breaker.Diagnostics()
if diag.WillTripNext {
    log.Warn("Circuit about to trip on next failure")
}
```

See [examples/observability](examples/observability/) for Prometheus integration and monitoring patterns.

## How It Works

AutoBreaker calculates failure rate as a percentage of recent requests:

```
Adaptive: "Trip when error rate > 5%"

At 100 RPS → trips at 50 failures (5% of 1000 req/10s)
At 10 RPS  → trips at 5 failures  (5% of 100 req/10s)

Same config, correct behavior at any traffic level.
```

The implementation uses a three-state machine (Closed → Open → HalfOpen → Closed) with lock-free atomic operations for minimal overhead.

**Architecture Details:** See [docs/TECHNICAL_BLOG.md](docs/TECHNICAL_BLOG.md) for comprehensive design explanation with diagrams.

## Performance

Benchmarks (Go 1.21, Apple M1):

| Operation          | Latency | Allocations |
|--------------------|---------|-------------|
| Execute (Closed)   | 78.5 ns | 0 allocs    |
| Execute (Open)     | 0.34 ns | 0 allocs    |
| UpdateSettings()   | 89.2 ns | 0 allocs    |

See [docs/BENCHMARKS.md](docs/BENCHMARKS.md) for detailed performance analysis.

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

## Documentation

- [Technical Blog](docs/TECHNICAL_BLOG.md) - Architecture deep-dive with diagrams
- [State Machine](docs/STATE_MACHINE.md) - State transition specification
- [Concurrency](docs/CONCURRENCY.md) - Lock-free implementation details
- [Error Classification](docs/ERROR_CLASSIFICATION.md) - Custom error handling
- [API Reference](https://pkg.go.dev/github.com/vnykmshr/autobreaker) - Full API documentation

## Important Notes

**Callback Performance:** `ReadyToTrip`, `OnStateChange`, and `IsSuccessful` callbacks execute synchronously on every request. Keep them <1μs. Use goroutines for slow operations:

```go
OnStateChange: func(name string, from, to State) {
    go metrics.Record(name, from, to) // Async, non-blocking
}
```

**Compatibility:** Drop-in replacement for sony/gobreaker API. Enable adaptive thresholds with `AdaptiveThreshold: true`.

## Status

**Production-Ready** - v1.0.0

- 102 tests, 97.1% coverage, race detector clean
- Zero dependencies, zero allocations in hot path
- Compatible with Go 1.21+

See [docs/ROADMAP_RFCS.md](docs/ROADMAP_RFCS.md) for future plans.

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please open an issue to discuss changes before submitting PRs.

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for guidelines.
