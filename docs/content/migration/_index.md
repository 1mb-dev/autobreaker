---
title: "Migration"
weight: 7
---

# Migration from sony/gobreaker

AutoBreaker follows the [sony/gobreaker](https://github.com/sony/gobreaker) v1 API shape. Most code moves over by changing the import and the constructor; the differences are listed below.

## Quick Migration

Replace the import and constructor:

```go
// Before (sony/gobreaker)
import "github.com/sony/gobreaker"

breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name: "service-name",
})

// After (AutoBreaker)
import "github.com/1mb-dev/autobreaker"

breaker := autobreaker.New(autobreaker.Settings{
    Name: "service-name",
})
```

## API Compatibility

### Same as gobreaker v1
- `Execute(func() (interface{}, error))`, `Name()`, `State()`, `Counts()`
- `ErrOpenState`, `ErrTooManyRequests`
- `Counts` fields and types
- `Settings` field names and types (`MaxRequests` behaves differently, below)
- State constants `StateClosed`, `StateOpen`, `StateHalfOpen`

### Different
- **Constructor:** `New` instead of `NewCircuitBreaker`
- **State values:** `StateOpen` is 1 and `StateHalfOpen` is 2; gobreaker numbers them the other way round. Compare against the constants, never stored integers
- **`MaxRequests`:** caps concurrent HalfOpen probes, and the first successful probe closes the breaker. In gobreaker it caps total HalfOpen requests and the breaker closes after `MaxRequests` consecutive successes, so a breaker with `MaxRequests: 3` recovers sooner here
- **No `TwoStepCircuitBreaker`**

### Enhanced Features
- **Adaptive thresholds** (percentage-based, opt-in with `AdaptiveThreshold: true`)
- **Runtime updates** via `UpdateSettings()`
- **Better observability** with `Metrics()` and `Diagnostics()`
- **Callback safety** with panic recovery

## Settings Mapping

| sony/gobreaker | AutoBreaker | Notes |
|----------------|-------------|-------|
| `Name` | `Name` | Same |
| `MaxRequests` | `MaxRequests` | Different: concurrent probe limit; first success closes |
| `Interval` | `Interval` | Same |
| `Timeout` | `Timeout` | Same |
| `ReadyToTrip` | `ReadyToTrip` | Enhanced with adaptive logic |
| `OnStateChange` | `OnStateChange` | Same, plus panic recovery |
| `IsSuccessful` | `IsSuccessful` | Same |
| - | `AdaptiveThreshold` | New: Enable percentage-based thresholds |
| - | `FailureRateThreshold` | New: Trip at >X% error rate |
| - | `MinimumObservations` | New: Minimum requests before evaluating |

## Behavior Differences

### 1. Adaptive Thresholds (Opt-in)
By default both libraries trip on `ConsecutiveFailures > 5`.

With `AdaptiveThreshold: true`, AutoBreaker trips on `(TotalFailures / Requests) > FailureRateThreshold` (default 0.05) once `Requests >= MinimumObservations` (default 20):
```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:              "service-name",
    AdaptiveThreshold: true,
    Interval:          10 * time.Second, // counts reset every 10s
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
4. **Test thoroughly** (behavior differs once adaptive thresholds are on)
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
import "github.com/1mb-dev/autobreaker"

settings := autobreaker.Settings{
    Name:          "api-client",
    MaxRequests:   3,
    Interval:      60 * time.Second,
    Timeout:       30 * time.Second,
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

1. **Adaptive thresholds** - One threshold across traffic levels (each window needs `MinimumObservations` requests)
2. **Runtime updates** - Change settings without restart
3. **Better observability** - More metrics and diagnostics
4. **Callback safety** - Panics won't break the circuit
5. **Active maintenance** - Regular updates and improvements

## Need Help?

- **GitHub Issues**: [https://github.com/1mb-dev/autobreaker/issues](https://github.com/1mb-dev/autobreaker/issues)
- **Examples**: See [examples/](https://github.com/1mb-dev/autobreaker/tree/main/examples)
