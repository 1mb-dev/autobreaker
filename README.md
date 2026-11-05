# AutoBreaker

Circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

[![CI](https://github.com/1mb-dev/autobreaker/workflows/CI/badge.svg)](https://github.com/1mb-dev/autobreaker/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/1mb-dev/autobreaker.svg)](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
[![Go Report Card](https://goreportcard.com/badge/github.com/1mb-dev/autobreaker)](https://goreportcard.com/report/github.com/1mb-dev/autobreaker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-1mb-dev.github.io/autobreaker-blue)](https://1mb-dev.github.io/autobreaker/)

## Overview

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems: at high traffic, 10 failures may represent <1% error rate (too sensitive), while at low traffic, 10 failures may be 100% error rate (too slow to protect).

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. One threshold works across traffic levels, as long as each `Interval` window holds at least `MinimumObservations` requests. See [How It Works](#how-it-works).

**Features:**
- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - State and counts are atomic; all methods are safe for concurrent use

## When to Use This

Use AutoBreaker when your services handle variable traffic and you don't want to tune circuit breaker thresholds per environment.

**Choose AutoBreaker over [sony/gobreaker](https://github.com/sony/gobreaker) when:**
- Your traffic varies 10x+ and you want failure-rate tripping built in, with a minimum-observations guard, rather than a hand-written `ReadyToTrip`
- You need to change thresholds at runtime without building a new breaker (`UpdateSettings`)
- You want built-in observability without wiring up custom callbacks (`Metrics`/`Diagnostics`)
- You want callbacks that cannot take the breaker down (panic recovery) and a context-aware `ExecuteContext`

**Choose sony/gobreaker instead when:**
- Static failure counts work for your traffic pattern (consistent, predictable load)
- You need the widest possible community adoption and ecosystem integrations
- You want a rolling window (`BucketPeriod`), generics, or two-step and distributed breakers (gobreaker v2)

AutoBreaker follows the sony/gobreaker v1 API shape (`Execute`, `Settings`, `Counts`, state names); the constructor is `New`. Set `AdaptiveThreshold: true` to get percentage-based detection on top of the familiar interface.

## Installation

```bash
go get github.com/1mb-dev/autobreaker
```

Requires Go 1.26 or later.

## Quick Start

```go
package main

import (
    "errors"
    "fmt"
    "net/http"
    "time"

    "github.com/1mb-dev/autobreaker"
)

func main() {
    client := &http.Client{Timeout: 5 * time.Second}

    breaker := autobreaker.New(autobreaker.Settings{
        Name:              "api-client",
        Timeout:           10 * time.Second, // Open -> HalfOpen after 10s
        AdaptiveThreshold: true,             // trip on failure rate instead of 6 consecutive failures
        Interval:          10 * time.Second, // rate measured over fixed 10s windows
    })

    result, err := breaker.Execute(func() (interface{}, error) {
        resp, err := client.Get("https://api.example.com/data")
        if err != nil {
            return nil, err
        }
        if resp.StatusCode >= 500 {
            _ = resp.Body.Close()
            return nil, fmt.Errorf("server error: %s", resp.Status)
        }
        return resp, nil
    })

    switch {
    case errors.Is(err, autobreaker.ErrOpenState), errors.Is(err, autobreaker.ErrTooManyRequests):
        fmt.Println("circuit open, using fallback")
        return
    case err != nil:
        fmt.Println("request failed:", err)
        return
    }

    resp := result.(*http.Response)
    defer func() { _ = resp.Body.Close() }()
    fmt.Println("status:", resp.Status)
}
```

This code is compiled by the test suite as the package [`Example`](https://pkg.go.dev/github.com/1mb-dev/autobreaker#example-package). Adaptive mode needs 20 requests in a window before it can trip; for low-traffic services see [How It Works](#how-it-works).

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

With these settings the breaker trips on the 6th consecutive failure (static threshold, the default). For percentage-based tripping, set `AdaptiveThreshold: true` and an `Interval`, as in the Quick Start; the rate then defaults to 5% over at least 20 requests.

For advanced configuration, runtime updates, and complete settings reference, see the [Configuration Guide](https://1mb-dev.github.io/autobreaker/getting-started/#configuration).

## How It Works

With `AdaptiveThreshold: true`, AutoBreaker trips when the failure rate in the current window exceeds the threshold. Counts reset on every state change, and every `Interval` if one is set: a fixed window, not a sliding one.

```
AdaptiveThreshold: true, Interval: 10s, threshold 5% (default)

At 100 RPS (1000 successes in the window) → trips on the 53rd failure
At 10 RPS  (100 successes in the window)  → trips on the 6th failure
```

With `Interval: 0`, counts reset only on state changes, so a long healthy run dilutes later failures: after 100,000 successes it takes 5,264 failures to trip.

The rate is checked only once a window holds `MinimumObservations` requests (default 20), with no consecutive-failure fallback. At `Interval: 10s`, a service under 2 RPS never trips, even if every request fails; lengthen `Interval` or lower `MinimumObservations` so one window sees enough requests.

The implementation uses a three-state machine (Closed → Open → HalfOpen → Closed) with lock-free atomic operations for minimal overhead.

## Performance

Per-state latency and allocations (Closed, Open, HalfOpen), with the CPU and Go version they were measured on, are in the [Performance Guide](https://1mb-dev.github.io/autobreaker/guides/performance/).

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

**Callback Performance:** Callbacks run synchronously inside `Execute()`: `IsSuccessful` on each completed request, `ReadyToTrip` after each failed request in Closed state, `OnStateChange` on every state change. Keep them <1μs. Use goroutines for slow operations:

```go
OnStateChange: func(name string, from, to State) {
    go metrics.Record(name, from, to) // Async, non-blocking
}
```

**Compatibility:** Follows the sony/gobreaker v1 API shape; the [Migration Guide](https://1mb-dev.github.io/autobreaker/migration/) lists what differs.

## Status

**Production-ready.**

- Zero dependencies
- Requires Go 1.26+

See the [Roadmap](https://1mb-dev.github.io/autobreaker/guides/roadmap/) for the next release theme.

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please open an issue to discuss changes before submitting PRs.

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for guidelines.
