---
title: "Migration"
weight: 7
---

# Migration from sony/gobreaker

AutoBreaker is a drop-in replacement for [sony/gobreaker](https://github.com/sony/gobreaker) with enhanced features.

## Quick Migration

Replace the import and constructor:

```go
// Before (sony/gobreaker)
import "github.com/sony/gobreaker"

breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name: "service-name",
})

// After (AutoBreaker)
import "github.com/vnykmshr/autobreaker"

breaker := autobreaker.New(autobreaker.Settings{
    Name: "service-name",
})
```

## API Compatibility

### Identical APIs
- `Execute(func() (interface{}, error))`
- `State()` returns same enum values (0=Closed, 1=Open, 2=HalfOpen)
- `ErrOpenState`, `ErrTooManyRequests` error types
- `Counts()` struct with same fields
- `Settings` struct with same field names/types

### Enhanced Features
- **Adaptive thresholds** (percentage-based, enabled by default)
- **Runtime updates** via `UpdateSettings()`
- **Better observability** with `Metrics()` and `Diagnostics()`
- **Callback safety** with panic recovery

## Settings Mapping

| sony/gobreaker | AutoBreaker | Notes |
|----------------|-------------|-------|
| `Name` | `Name` | Same |
| `MaxRequests` | `MaxRequests` | Same |
| `Interval` | `Interval` | Same |
| `Timeout` | `Timeout` | Same |
| `ReadyToTrip` | `ReadyToTrip` | Enhanced with adaptive logic |
| `OnStateChange` | `OnStateChange` | Same, plus panic recovery |
| - | `AdaptiveThreshold` | New: Enable percentage-based thresholds |
| - | `FailureRateThreshold` | New: Trip at >X% error rate |
| - | `MinimumObservations` | New: Minimum requests before evaluating |
| - | `IsSuccessful` | New: Custom error classification |

## Behavior Differences

### 1. Adaptive Thresholds (Default)
sony/gobreaker uses absolute counts: `ConsecutiveFailures > 5`

AutoBreaker uses percentage-based: `(TotalFailures / Requests) > 0.05`

**To match sony/gobreaker behavior:**
```go
breaker := autobreaker.New(autobreaker.Settings{
    Name: "service-name",
    AdaptiveThreshold: false, // Disable adaptive thresholds
})
```

### 2. Callback Safety
AutoBreaker wraps callbacks in panic recovery. If your callback panics, AutoBreaker continues working.

### 3. Runtime Updates
AutoBreaker supports updating settings without recreating the breaker.

## Migration Checklist

1. **Update imports**
2. **Update constructor** (`NewCircuitBreaker` → `New`)
3. **Review settings** (enable/disable adaptive thresholds as needed)
4. **Test thoroughly** (behavior may differ with adaptive thresholds)
5. **Consider enhancements** (runtime updates, better observability)

## Example Migration

```go
// Before: sony/gobreaker
import "github.com/sony/gobreaker"

settings := gobreaker.Settings{
    Name:          "api-client",
    MaxRequests:   3,
    Interval:      60 * time.Second,
    Timeout:       30 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
    OnStateChange: func(name string, from, to gobreaker.State) {
        log.Printf("%s: %v → %v", name, from, to)
    },
}

breaker := gobreaker.NewCircuitBreaker(settings)

// After: AutoBreaker
import "github.com/vnykmshr/autobreaker"

settings := autobreaker.Settings{
    Name:          "api-client",
    MaxRequests:   3,
    Interval:      60 * time.Second,
    Timeout:       30 * time.Second,
    AdaptiveThreshold: false, // Match sony/gobreaker behavior
    ReadyToTrip: func(counts autobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
    OnStateChange: func(name string, from, to autobreaker.State) {
        log.Printf("%s: %v → %v", name, from, to)
    },
}

breaker := autobreaker.New(settings)
```

## Benefits of Migration

1. **Adaptive thresholds** - Works correctly at any traffic level
2. **Runtime updates** - Change settings without restart
3. **Better observability** - More metrics and diagnostics
4. **Callback safety** - Panics won't break the circuit
5. **Active maintenance** - Regular updates and improvements

## Need Help?

- **GitHub Issues**: [https://github.com/vnykmshr/autobreaker/issues](https://github.com/vnykmshr/autobreaker/issues)
- **Examples**: See [examples/](https://github.com/vnykmshr/autobreaker/tree/main/examples)
