---
title: "Concurrency"
weight: 2
---

# Concurrency

AutoBreaker uses lock-free atomic operations for maximum performance under high concurrency.

## Design Goals

1. **Lock-Free Hot Path:** State checks require no mutex locks
2. **Minimal Contention:** Count updates use atomic operations only
3. **Safe Transitions:** State changes are atomic and race-free
4. **No Goroutine Leaks:** No background goroutines
5. **Bounded Memory:** No unbounded growth under load

## Concurrency Primitives

### State Storage

```go
type CircuitBreaker struct {
    state atomic.Int32  // 0=Closed, 1=Open, 2=HalfOpen
}
```

**Why atomic.Int32:**
- Lock-free reads (critical for hot path)
- Atomic compare-and-swap for state transitions
- No mutex overhead

### Counts Storage

```go
type CircuitBreaker struct {
    requests             atomic.Uint32
    totalSuccesses       atomic.Uint32
    totalFailures        atomic.Uint32
    consecutiveSuccesses atomic.Uint32
    consecutiveFailures  atomic.Uint32
}
```

**Why separate atomic fields:**
- Independent updates without locking
- No struct-level mutex for simple counters
- Cache-friendly (separate cache lines)

### Half-Open Request Limiter

```go
type CircuitBreaker struct {
    halfOpenRequests atomic.Int32  // Concurrent request count in half-open
}
```

**Enforcement:**
```go
// Increment and check atomically
current := cb.halfOpenRequests.Add(1)
if current > cb.maxRequests {
    cb.halfOpenRequests.Add(-1)  // Undo increment
    return nil, ErrTooManyRequests
}

// On completion (defer):
cb.halfOpenRequests.Add(-1)
```

## State Transition Synchronization

### Atomic State Changes with Callbacks

**Two-phase transition pattern:**

```go
// Phase 1: Atomic state change (fast, lock-free)
if !state.CompareAndSwap(oldState, newState) {
    return // Lost race, another goroutine transitioned
}

// Phase 2: Post-transition actions (may be slow, but state already changed)
cb.clearCounts()
if cb.onStateChange != nil {
    cb.onStateChange(cb.name, oldState, newState)
}
```

**Guarantees:**
- State visible immediately to all goroutines
- Callbacks execute after state is committed
- No locks held during callbacks

## Request Execution Flow

```go
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
    // 1. Check current state (atomic read, no lock)
    currentState := cb.state.Load()

    // 2. Handle state-specific logic
    switch currentState {
    case StateOpen:
        if cb.shouldTransitionToHalfOpen() {
            cb.transitionToHalfOpen()
        } else {
            return nil, ErrOpenState
        }

    case StateHalfOpen:
        if !cb.allowHalfOpenRequest() {
            return nil, ErrTooManyRequests
        }
        defer cb.halfOpenRequests.Add(-1)

    case StateClosed:
        cb.maybeResetCounts()
    }

    // 3. Increment request counter (atomic)
    cb.requests.Add(1)

    // 4. Execute request (no locks held)
    result, err := cb.executeWithPanicRecovery(req)

    // 5. Record outcome (atomic count updates)
    cb.recordOutcome(err)

    // 6. Check for state transition (lock-free)
    cb.checkStateTransition(err)

    return result, err
}
```

## Race Condition Prevention

### 1. Double-Check State After Transition

```go
// Check state
if cb.state.Load() == StateHalfOpen {
    // Attempt to allow request
    if cb.allowHalfOpenRequest() {
        defer cb.halfOpenRequests.Add(-1)
        // Execute request...
        // State may have changed during execution
        // Defer ensures counter is decremented regardless
    }
}
```

### 2. Idempotent State Transitions

```go
// Safe to call concurrently - only one succeeds
cb.state.CompareAndSwap(StateOpen, StateHalfOpen)
```

### 3. Count Reset During Transition

Small race window acceptable - counts are approximate. Off-by-one count doesn't affect correctness.

## Performance Characteristics

| Operation | Latency | Contention |
|-----------|---------|------------|
| State check | 1-5 ns | None (read-only atomic) |
| Count increment | 5-20 ns | Minimal (atomic add) |
| State transition | 50-200 ns | CAS retry on race |
| Callback execution | User-defined | None (post-commit) |

## Scalability

**Concurrent Goroutines:** Tested with 10,000+

**Throughput:** Limited by request execution time, not breaker overhead

**Contention Points:**
- Count increments: Minimal (cache line bouncing only)
- State transitions: Rare (only during failures/recovery)

## Testing

### Race Detection
```bash
go test -race ./...
```

### Stress Testing
```go
func TestConcurrentAccess(t *testing.T) {
    cb := New(Settings{})
    const goroutines = 1000
    const requestsPerGoroutine = 1000
    
    var wg sync.WaitGroup
    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < requestsPerGoroutine; j++ {
                cb.Execute(func() (interface{}, error) {
                    return nil, nil
                })
            }
        }()
    }
    wg.Wait()
}
```

## Anti-Patterns

**❌ Mutex for state:** Adds contention, slows hot path  
**❌ RWMutex for counts:** Atomic operations are faster  
**❌ Channels for coordination:** Overhead, complexity  
**❌ Background goroutines:** Memory leaks, resource waste  
**❌ Locks during request execution:** Serializes requests
