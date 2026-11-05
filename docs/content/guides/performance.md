---
title: "Performance"
weight: 4
---

# Performance

Measured 2026-11-05 on an Intel Core i5-8257U @ 1.40GHz (darwin/amd64) with Go 1.27.3, running the suite five times:

```bash
go test -run '^$' -bench . -benchmem ./internal/breaker/
```

Each figure is the median of the five passes; the range shows the fastest and slowest pass. On this laptop the same benchmark varied by up to 2x between passes (CPU frequency scaling), so compare rows rather than reading the numbers as exact. Run `make bench` on your own hardware.

## Execute, per state

| State | Benchmark | ns/op (median) | range | B/op | allocs/op |
|-------|-----------|------:|------:|-----:|----------:|
| Closed | `Execute_Closed` | 61.1 | 49.5–82.0 | 0 | 0 |
| Closed (`ExecuteContext`) | `ExecuteContext_Closed` | 63.1 | 55.0–81.9 | 0 | 0 |
| Closed, failing request | `ExecuteFailure` | 76.7 | 66.5–137.4 | 0 | 0 |
| Closed, 8 goroutines (`b.RunParallel`) | `ConcurrentExecute` | 164.4 | 136.5–181.8 | 0 | 0 |
| Open (request rejected) | `Execute_Open` | 112.2 | 91.6–148.7 | 0 | 0 |
| HalfOpen: trip, probe, close | `Execute_HalfOpen` | 624.3 | 521.1–958.9 | 0 | 0 |

A HalfOpen probe always changes state (one success closes the breaker), so the HalfOpen row times a full cycle on one breaker: a failing request that trips it, the move to HalfOpen, and the probe that closes it.

The function being wrapped costs 1.0 ns when called directly (`CompareBaseline`).

## Other operations

| Operation | Benchmark | ns/op (median) | range | B/op | allocs/op |
|-----------|-----------|------:|------:|-----:|----------:|
| `State()` | `State` | 0.3 | 0.3–0.4 | 0 | 0 |
| `Counts()` | `Counts` | 0.9 | 0.8–1.2 | 0 | 0 |
| `Metrics()` | `Metrics` | 18.2 | 16.0–26.5 | 0 | 0 |
| `Diagnostics()` | `Diagnostics` | 40.4 | 36.5–56.4 | 0 | 0 |
| `UpdateSettings()` | `UpdateSettings` | 12.9 | 11.8–20.2 | 0 | 0 |
| `New()` | `New` | 215.9 | 166.4–292.4 | 144 | 1 |

## Allocations

`Execute()` does not allocate in Closed, Open or HalfOpen state, including the call that changes state (trip, probe success, probe failure); `TestExecuteDoesNotAllocate` checks this with `testing.AllocsPerRun`. The exception is the log line written when a callback panics or a counter saturates. `New()` allocates once. Callbacks you supply run inside `Execute()`, so any allocation they make is added to the calls that run them.

## Lock-Free Design

```go
type CircuitBreaker struct {
    state atomic.Int32  // 0=Closed, 1=Open, 2=HalfOpen
    requests atomic.Uint32
    totalSuccesses atomic.Uint32
    totalFailures atomic.Uint32
    // ... other atomic fields
}
```

The request path uses atomic loads, adds and compare-and-swap. The only mutex serializes the log lines written when a callback panics or a counter saturates. There is no background goroutine.

## Keep Callbacks Fast

`ReadyToTrip`, `OnStateChange` and `IsSuccessful` run synchronously inside `Execute()`.

```go
// Good: Async logging
OnStateChange: func(name string, from, to State) {
    go metrics.Record(name, from, to) // Async, non-blocking
}

// Bad: Blocking I/O in callback
OnStateChange: func(name string, from, to State) {
    db.SaveStateChange(name, from, to) // Blocks!
}
```
