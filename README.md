# AutoBreaker

Adaptive circuit breaker for Go that automatically adjusts to your traffic patterns.

## Why AutoBreaker?

Traditional circuit breakers use static thresholds (e.g., "trip after 10 failures"). This causes problems:

- **At high traffic:** 10 failures might be <1% error rate (too sensitive, false positives)
- **At low traffic:** 10 failures might be 100% error rate (too slow to protect)
- **Configuration burden:** Different thresholds needed for dev/staging/prod

AutoBreaker solves this by using **percentage-based thresholds** that adapt to request volume automatically.

## Features

- **Adaptive Thresholds:** Same config works across different traffic volumes
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

✅ **Production-Ready with Observability** - Phase 3A Complete

- ✅ Full circuit breaker implementation with adaptive thresholds
- ✅ Comprehensive observability (Metrics() + Diagnostics() APIs)
- ✅ Comprehensive test suite (50 tests, 98.2% coverage)
- ✅ Race-detector clean
- ✅ 6 production-ready examples (including Prometheus integration)
- ⏭️ Next: Advanced features (Phase 4)

## Examples

See comprehensive examples in the [`examples/`](examples/) directory:

- **[production_ready/](examples/production_ready/)** ⭐ - Realistic production scenarios, recommended starting point
- **[observability/](examples/observability/)** ⭐ - Monitoring, metrics, and diagnostics patterns
- **[prometheus/](examples/prometheus/)** - Prometheus integration (custom collector)
- **[basic/](examples/basic/)** - Fundamental circuit breaker patterns
- **[adaptive/](examples/adaptive/)** - Adaptive vs static threshold comparison
- **[custom_errors/](examples/custom_errors/)** - Custom error classification

Run any example:
```bash
go run examples/production_ready/main.go
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

- ✅ **Phase 1:** Core circuit breaker implementation
- ✅ **Phase 2A:** Adaptive thresholds, validation, comprehensive tests
- ✅ **Phase 3A:** Observability & metrics (Metrics API, Diagnostics API, examples)
- ⏭️ **Phase 4:** Advanced features (sliding windows, health checks, middleware)
- 🔮 **Phase 5:** Ecosystem integration (HTTP middleware, gRPC interceptors)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please open an issue first to discuss proposed changes.
