---
title: "AutoBreaker"
weight: 1
---

# AutoBreaker

Adaptive circuit breaker for Go with percentage-based thresholds that automatically adjust to traffic patterns.

[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/autobreaker.svg)](https://pkg.go.dev/github.com/vnykmshr/autobreaker)
[![CI](https://github.com/vnykmshr/autobreaker/workflows/CI/badge.svg)](https://github.com/vnykmshr/autobreaker/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/autobreaker)](https://goreportcard.com/report/github.com/vnykmshr/autobreaker)

## Why AutoBreaker?

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems: at high traffic, 10 failures may represent <1% error rate (too sensitive), while at low traffic, 10 failures may be 100% error rate (too slow to protect).

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. Configure once, works correctly across all traffic levels and environments.

## Quick Start

```bash
go get github.com/vnykmshr/autobreaker
```

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

## Key Features

- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **High Performance** - <100ns overhead per request, zero allocations
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - Lock-free atomic operations throughout

## Documentation

### Getting Started
- [Installation and basic usage](/getting-started/)
- [Configuration options](/getting-started/#configuration)

### Guides
- [Building AutoBreaker](/guides/building-autobreaker/) - Architecture and design decisions
- [State Machine](/guides/state-machine/) - State transition specification
- [Concurrency](/guides/concurrency/) - Lock-free implementation details
- [Error Classification](/guides/error-classification/) - Custom error handling
- [Performance](/guides/performance/) - Benchmarks and optimization
- [Decision Guide](/guides/decision-guide/) - When to use AutoBreaker
- [Migration](/migration/) - From sony/gobreaker

### Reference
- [API Reference](https://pkg.go.dev/github.com/vnykmshr/autobreaker) - Complete API documentation
- [Examples](https://github.com/vnykmshr/autobreaker/tree/main/examples) - Production-ready examples
- [Contributing](/contributing/) - How to contribute
- [Security](/security/) - Security policy
- [Changelog](/changelog/) - Release history

## Performance

Benchmarks (Go 1.21+, Apple M1):

| Operation | Latency | Allocations |
|-----------|---------|-------------|
| Execute (Closed) | 78.5 ns | 0 allocs |
| Execute (Open) | 0.34 ns | 0 allocs |
| UpdateSettings() | 89.2 ns | 0 allocs |

See [Performance Guide](/guides/performance/) for detailed analysis.

## Examples

Production-ready examples in the [`examples/`](https://github.com/vnykmshr/autobreaker/tree/main/examples) directory:

- **production_ready** - HTTP client integration, recommended starting point
- **runtime_config** - Dynamic configuration updates (file, API, signals)
- **observability** - Monitoring, metrics, and diagnostics
- **prometheus** - Prometheus collector integration
- **adaptive** - Adaptive vs static threshold comparison
- **custom_errors** - Custom error classification

## License

MIT License - see [LICENSE](https://github.com/vnykmshr/autobreaker/blob/main/LICENSE) file.

## Contributing

Contributions welcome! Please see [Contributing Guide](/contributing/) for guidelines.
