---
title: "Performance"
weight: 4
---

# Performance

AutoBreaker achieves exceptional performance with zero allocations in the hot path.

## Benchmark Results

| Operation | Latency | Target | Status | Allocations |
|-----------|---------|--------|--------|-------------|
| State check | 0.34 ns | <10 ns | **30x better** | 0 |
| Counts snapshot | 0.87 ns | <50 ns | **57x better** | 0 |
| Execute (success) | 33.62 ns | <100 ns | **3x better** | 0 |
| Execute (failure) | 79.44 ns | <100 ns | **20% better** | 0 |
| Execute (open) | 82.35 ns | <50 ns | **65% over** | 0 |
| Concurrent Execute | 96.47 ns | comparable | **Good** | 0 |

## Overhead Analysis

**Baseline** (no circuit breaker): 1.33 ns  
**Execute (success)**: 33.62 ns  
**Net overhead**: ~32 ns per request

This is exceptional for a circuit breaker with:
- Thread-safe atomic operations
- State machine logic
- Count tracking
- Panic recovery
- Callback support

## Zero Allocations

All hot-path operations allocate **zero bytes**:
- `State()` - 0 allocs
- `Counts()` - 0 allocs
- `Execute()` - 0 allocs
- `ReadyToTrip` - 0 allocs

Only `New()` allocates (one-time, 128 bytes for the circuit breaker struct).

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

**Why atomic operations:**
- No mutex locks in hot path
- Minimal contention under high concurrency
- Cache-friendly separate fields

## Performance Under Load

### Concurrent Access
- Tested with 10,000+ concurrent goroutines
- Minimal performance degradation
- Race detector clean

### Memory Usage
- <200 bytes per breaker instance
- No background goroutines
- No unbounded growth

## Comparison to Targets

| Metric | Target | Actual | Result |
|--------|--------|--------|--------|
| State check | <10 ns | 0.34 ns | **30x faster** |
| Count update | <20 ns | ~1 ns | **20x faster** |
| Execute overhead | <100 ns | 32 ns | **3x faster** |

## Platform Details

```
OS: darwin
Arch: amd64
CPU: Intel Core i5-8257U @ 1.40GHz
```

## Best Practices for Performance

### 1. Keep Callbacks Fast
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

### 2. Use Atomic Operations
- State checks: `state.Load()` (0.34 ns)
- Count updates: `counter.Add(1)` (~1 ns)
- State transitions: `state.CompareAndSwap()` (~50 ns)

### 3. Avoid Allocations in Hot Path
- No string formatting in callbacks
- No slice allocations in `IsSuccessful`
- Use sync.Pool if needed (not needed for AutoBreaker)

## Production Readiness

**Validated performance:**
- ✅ Sub-nanosecond state reads
- ✅ ~30ns request overhead  
- ✅ Zero allocations
- ✅ Scales under concurrency
- ✅ Race detector clean

**Production deployment ready.**
