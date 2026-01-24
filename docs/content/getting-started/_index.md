---
title: "Getting Started"
weight: 1
---

# Getting Started with AutoBreaker

AutoBreaker is an adaptive circuit breaker for Go that uses percentage-based thresholds to automatically adjust to traffic patterns.

## Why AutoBreaker?

Traditional circuit breakers use static failure thresholds (e.g., "trip after 10 failures"). This creates problems:

- **High traffic**: 10 failures may represent <1% error rate (too sensitive)
- **Low traffic**: 10 failures may be 100% error rate (too slow to protect)

AutoBreaker uses **percentage-based thresholds** that adapt to request volume automatically. Configure once, works correctly across all traffic levels.

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

## How It Works

AutoBreaker calculates failure rate as a percentage of recent requests:

```
Adaptive: "Trip when error rate > 5%"

At 100 RPS → trips at 50 failures (5% of 1000 req/10s)
At 10 RPS  → trips at 5 failures  (5% of 100 req/10s)

Same config, correct behavior at any traffic level.
```

## Key Features

- **Adaptive Thresholds** - Percentage-based failure detection scales with traffic
- **Runtime Configuration** - Update settings without restart
- **Zero Dependencies** - Standard library only
- **High Performance** - <100ns overhead per request, zero allocations
- **Observability** - Metrics() and Diagnostics() APIs built-in
- **Thread-Safe** - Lock-free atomic operations throughout

## Next Steps

- [Configuration](#configuration) - Detailed settings and options
- [Examples](#examples) - Production-ready code examples
- [API Reference](https://pkg.go.dev/github.com/vnykmshr/autobreaker) - Complete API documentation
- [Migration Guide](/migration/) - Moving from sony/gobreaker

## Configuration

For detailed configuration options and advanced settings, see the [Configuration Guide](/getting-started/configuration/).

## Examples

Production-ready code examples are available in the [examples directory](https://github.com/vnykmshr/autobreaker/tree/main/examples) on GitHub.
