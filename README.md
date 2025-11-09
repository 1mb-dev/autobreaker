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

🚧 **In Active Development** - Phase 0 (Foundation)

Current focus: Core circuit breaker with adaptive thresholds (no ML complexity).

## Philosophy

AutoBreaker follows a lean approach:

- ✅ Solve real problems (traffic-aware thresholds)
- ✅ Simple, measurable improvements
- ✅ Production-grade reliability
- ❌ No AI bloat or unnecessary complexity
- ❌ No external dependencies
- ❌ No magic behavior

## Roadmap

- **Phase 1:** Core circuit breaker with adaptive thresholds
- **Phase 2:** Rich observability and metrics
- **Phase 3:** Production hardening and performance optimization
- **Phase 4:** Ecosystem integration (HTTP, gRPC, etc.)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please open an issue first to discuss proposed changes.
