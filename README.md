# AutoBreaker

[![CI](https://github.com/vnykmshr/autobreaker/workflows/CI/badge.svg)](https://github.com/vnykmshr/autobreaker/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/autobreaker.svg)](https://pkg.go.dev/github.com/vnykmshr/autobreaker)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/autobreaker)](https://goreportcard.com/report/github.com/vnykmshr/autobreaker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Adaptive circuit breaker for Go that automatically adjusts to your traffic patterns.

## Why AutoBreaker?

Traditional circuit breakers use static thresholds (e.g., "trip after 10 failures"). This causes problems:

- **At high traffic:** 10 failures might be <1% error rate (too sensitive, false positives)
- **At low traffic:** 10 failures might be 100% error rate (too slow to protect)
- **Configuration burden:** Different thresholds needed for dev/staging/prod

AutoBreaker solves this by using **percentage-based thresholds** that adapt to request volume automatically.

## Features

- **Adaptive Thresholds:** Same config works across different traffic volumes
- **Runtime Configuration:** Update settings without restart
- **Drop-in Replacement:** Compatible with sony/gobreaker API
- **Zero Dependencies:** Only standard library
- **High Performance:** <100ns overhead per request
- **Rich Observability:** Detailed metrics and insights built-in

## Quick Start

```go
package main

import (
    "github.com/vnykmshr/autobreaker"
    "fmt"
    "time"
)

func main() {
    // Create adaptive breaker with sensible defaults
    breaker := autobreaker.New(autobreaker.Settings{
        Name: "api-client",
        Timeout: 10 * time.Second,
    })

    // Wrap your operation
    result, err := breaker.Execute(func() (interface{}, error) {
        return callExternalAPI()
    })

    if err != nil {
        fmt.Printf("Circuit breaker: %v\n", err)
        return
    }

    fmt.Printf("Result: %v\n", result)
}
```

## ⚠️ Performance Warning: User Callbacks

**Critical:** `ReadyToTrip`, `OnStateChange`, and `IsSuccessful` callbacks run **synchronously** on every request. Slow callbacks will block all traffic.

**Requirements:**
- Keep callbacks <1μs (microsecond)
- No I/O, network calls, or `time.Sleep()`
- For slow work, spawn goroutines:
  ```go
  OnStateChange: func(name string, from, to State) {
      go metrics.Record(name, from, to) // Async, non-blocking
  }
  ```

**Why:** Callbacks execute on the hot path. A 10ms callback = 10ms added to every request.

## How It Works

AutoBreaker adapts failure thresholds as a **percentage of recent requests** instead of absolute counts:

```
Static:   "Trip after 10 failures in 10 seconds"
Problem:  At 100 RPS → 10 failures = 1% (too sensitive)
          At 10 RPS → 10 failures = 100% (too slow)

Adaptive: "Trip when error rate > 5% over 10 second window"
Behavior: At 100 RPS → trips at 50 failures
          At 10 RPS → trips at 5 failures
          ✓ Same config, right behavior
```

## Installation

```bash
go get github.com/vnykmshr/autobreaker
```

## Status

✅ **Production-Ready** - v1.0.0

- Adaptive thresholds with runtime configuration
- 102 tests, 97.1% coverage, race-detector clean
- <100ns overhead, zero allocations, zero dependencies
- 9 production examples (HTTP, Prometheus, runtime config)
- Full observability (Metrics, Diagnostics)

## Examples

See comprehensive examples in the [`examples/`](examples/) directory:

- **[production_ready/](examples/production_ready/)** ⭐ - Realistic production scenarios, recommended starting point
- **[runtime_config/](examples/runtime_config/)** ⭐ - Runtime configuration updates (file, API, SIGHUP)
- **[observability/](examples/observability/)** ⭐ - Monitoring, metrics, and diagnostics patterns
- **[prometheus/](examples/prometheus/)** - Prometheus integration (custom collector)
- **[basic/](examples/basic/)** - Fundamental circuit breaker patterns
- **[adaptive/](examples/adaptive/)** - Adaptive vs static threshold comparison
- **[custom_errors/](examples/custom_errors/)** - Custom error classification

Run any example:
```bash
go run examples/production_ready/main.go
go run examples/runtime_config/main.go
go run examples/observability/main.go
```

## Philosophy

AutoBreaker follows a lean approach:

- ✅ Solve real problems (traffic-aware thresholds)
- ✅ Simple, measurable improvements
- ✅ Production-grade reliability
- ❌ No AI bloat or unnecessary complexity
- ❌ No external dependencies
- ❌ No magic behavior

## Roadmap

- ✅ **Phase 1:** Core circuit breaker
- ✅ **Phase 2A:** Adaptive thresholds
- ✅ **Phase 3A:** Observability & metrics
- ✅ **Phase 4A:** Runtime configuration
- 🔍 **Phase 4B/5:** Feature requests - [RFC: Sliding Windows](../../issues), [RFC: Middleware](../../issues)

**Note:** v1.0 may be feature-complete. Future phases depend on validated community demand.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please open an issue first to discuss proposed changes.
