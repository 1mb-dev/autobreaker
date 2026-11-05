# AutoBreaker

Circuit breaker for Go with percentage-based (adaptive) thresholds, runtime settings updates and built-in metrics.

[![CI](https://github.com/1mb-dev/autobreaker/workflows/CI/badge.svg)](https://github.com/1mb-dev/autobreaker/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/1mb-dev/autobreaker.svg)](https://pkg.go.dev/github.com/1mb-dev/autobreaker)
[![Go Report Card](https://goreportcard.com/badge/github.com/1mb-dev/autobreaker)](https://goreportcard.com/report/github.com/1mb-dev/autobreaker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-1mb-dev.github.io/autobreaker-blue)](https://1mb-dev.github.io/autobreaker/)

## Why AutoBreaker

A fixed failure count means different things at different traffic levels: ten failures is noise at 1,000 requests per second and a full outage at one. AutoBreaker can trip on the failure rate instead, over a window you choose, behind the familiar sony/gobreaker v1 API. One rate holds across traffic levels as long as each window sees at least `MinimumObservations` requests; see [How It Works](#how-it-works).

It stays small on purpose: no dependencies, no allocations in `Execute`, settings you can change at runtime with `UpdateSettings`, and `Metrics()`/`Diagnostics()` snapshots for monitoring. All methods are safe for concurrent use. The default is the classic static rule; percentage-based tripping is one setting away.

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
- You want a two-step breaker, or gobreaker v2's rolling window (`BucketPeriod`), generics or distributed breaker

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

## Stability

- **API:** v1 is stable. No breaking changes and no change to default behavior within v1; additions only when they change no existing behavior.
- **Maintenance:** bug and security fixes, dependency and CI upkeep; CI runs daily. Responses are best effort, and there is no release cadence.
- **Go versions:** requires Go 1.26 or later.
- **Dependencies:** none outside the standard library.

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please open an issue to discuss changes before submitting PRs.

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for guidelines.
