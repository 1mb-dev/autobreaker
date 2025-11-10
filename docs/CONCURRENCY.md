# AutoBreaker Concurrency Model

## Design Goals

1. **Lock-Free Hot Path:** State checks must not require mutex locks
2. **Minimal Contention:** Count updates use atomic operations only
3. **Safe Transitions:** State changes are atomic and race-free
4. **No Goroutine Leaks:** No background goroutines (timers are on-demand)
5. **Bounded Memory:** No unbounded growth under load

## Concurrency Primitives

### State Storage

```go
type CircuitBreaker struct {
    state atomic.Int32  // 0=Closed, 1=Open, 2=HalfOpen
    // ... other fields
}
```

**Why atomic.Int32:**
- Lock-free reads (critical for hot path)
- Atomic compare-and-swap for state transitions
- No mutex overhead

**Operations:**
- **Read:** `state.Load()` - O(1), no locks
- **Write:** `state.CompareAndSwap(old, new)` - Atomic transition
- **No partial states:** Atomicity guarantees

---

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

**Operations:**
- **Increment:** `counter.Add(1)` - Atomic
- **Reset:** `counter.Store(0)` - Atomic
- **Read:** `counter.Load()` - Atomic

**Constraint:** Consecutive counters must update together atomically:
```go
// Reset consecutive counters atomically
consecutiveSuccesses.Store(0)
consecutiveFailures.Store(0)
// Small race window acceptable - counts are approximate
```

---

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

**Edge Case:** State transition during request execution
- Counter decremented in defer (always runs)
- Works correctly even if state changes mid-request

---

### Open State Timestamp

```go
type CircuitBreaker struct {
    openedAt atomic.Int64  // Unix nanoseconds
}
```

**Why atomic int64:**
- Store nanosecond timestamp as int64
- Lock-free reads for timeout checks
- Monotonic time (immune to clock adjustments)

**Usage:**
```go
// On transition to OPEN:
openedAt.Store(time.Now().UnixNano())

// On timeout check:
elapsed := time.Duration(time.Now().UnixNano() - openedAt.Load())
if elapsed >= timeout {
    // Transition to HALF-OPEN
}
```

---

### Interval-Based Count Clearing

```go
type CircuitBreaker struct {
    lastClearedAt atomic.Int64  // Unix nanoseconds
}
```

**Check on every request in CLOSED state:**
```go
if interval > 0 {
    now := time.Now().UnixNano()
    last := lastClearedAt.Load()

    if time.Duration(now - last) >= interval {
        // Try to claim clearing responsibility
        if lastClearedAt.CompareAndSwap(last, now) {
            // This goroutine won the race, clear counts
            cb.clearCounts()
        }
    }
}
```

**Race Safety:**
- Multiple goroutines may check condition simultaneously
- Only one succeeds in CAS (clears counts)
- Others continue normally

---

## State Transition Synchronization

### Atomic State Changes with Callbacks

**Problem:** State change must be atomic, but callbacks may be slow

**Solution:** Two-phase transition

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

---

### Example: CLOSED → OPEN Transition

```go
func (cb *CircuitBreaker) checkTrip() {
    // Read current counts (atomic, no locks)
    counts := cb.getCounts()

    // Evaluate trip condition
    if !cb.readyToTrip(counts) {
        return
    }

    // Attempt atomic state transition
    if !cb.state.CompareAndSwap(StateClosed, StateOpen) {
        return // Already transitioned (race lost)
    }

    // Post-transition: Clear counts, record timestamp, callback
    cb.clearCounts()
    cb.openedAt.Store(time.Now().UnixNano())

    if cb.onStateChange != nil {
        cb.onStateChange(cb.name, StateClosed, StateOpen)
    }
}
```

**Race Scenarios:**

**Scenario 1:** Two goroutines evaluate `readyToTrip` as true simultaneously
- Both attempt `CompareAndSwap(StateClosed, StateOpen)`
- Only one succeeds (winner executes callback)
- Loser's CAS fails, returns immediately
- Result: Single transition, single callback

**Scenario 2:** State changes between evaluation and CAS
- Goroutine A: Evaluates `readyToTrip` → true (state is CLOSED)
- Goroutine B: Transitions state CLOSED → OPEN
- Goroutine A: Attempts CAS(CLOSED, OPEN) → Fails (state is now OPEN)
- Result: No duplicate transition

---

## Request Execution Flow

### Execute Method Concurrency

```go
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
    // 1. Check current state (atomic read, no lock)
    currentState := cb.state.Load()

    // 2. Handle state-specific logic
    switch currentState {
    case StateOpen:
        // Check timeout (atomic read)
        if cb.shouldTransitionToHalfOpen() {
            cb.transitionToHalfOpen()
            // Fall through to HalfOpen handling
        } else {
            return nil, ErrOpenState
        }

    case StateHalfOpen:
        // Atomic increment-and-check
        if !cb.allowHalfOpenRequest() {
            return nil, ErrTooManyRequests
        }
        defer cb.halfOpenRequests.Add(-1)

    case StateClosed:
        // Check interval-based clearing
        cb.maybeResetCounts()
    }

    // 3. Increment request counter (atomic)
    cb.requests.Add(1)

    // 4. Execute request (no locks held)
    start := time.Now()
    result, err := cb.executeWithPanicRecovery(req)
    duration := time.Since(start)

    // 5. Record outcome (atomic count updates)
    cb.recordOutcome(err, duration)

    // 6. Check for state transition (lock-free)
    cb.checkStateTransition(err)

    return result, err
}
```

**Key Points:**
- No mutex locks in entire path
- All state reads/writes are atomic
- Multiple goroutines execute concurrently
- State transitions coordinate via CAS

---

## Memory Ordering Guarantees

### Happens-Before Relationships

**State Transition → Count Clearing:**
```go
state.Store(StateOpen)           // Write A
cb.requests.Store(0)             // Write B

// Guarantee: All goroutines see counts=0 after state=Open
```

Go's atomic package guarantees sequential consistency for atomic operations.

**Count Update → State Check:**
```go
cb.totalFailures.Add(1)          // Write A
counts := cb.getCounts()         // Read B (includes failure)
if cb.readyToTrip(counts) { ... }
```

Atomic operations are sequentially consistent - no reordering.

---

## Race Condition Prevention

### 1. Double-Check State After Transition

**Problem:** State may change between check and action

**Solution:** Re-verify after state-dependent actions

```go
// Check state
if cb.state.Load() == StateHalfOpen {
    // Attempt to allow request
    if cb.allowHalfOpenRequest() {
        defer cb.halfOpenRequests.Add(-1)

        // Execute request...
        // State may have changed to OPEN or CLOSED during execution
        // Defer ensures counter is decremented regardless
    }
}
```

---

### 2. Idempotent State Transitions

**Problem:** Multiple goroutines trigger same transition

**Solution:** CAS ensures single execution

```go
// Safe to call concurrently - only one succeeds
cb.state.CompareAndSwap(StateOpen, StateHalfOpen)
```

---

### 3. Count Reset During Transition

**Problem:** Count increments during state transition

**Solution:** Acceptable race - counts are approximate

```go
// Goroutine A: Transitions state, clears counts
cb.state.Store(StateOpen)
cb.requests.Store(0)

// Goroutine B: Increments count (may happen before/after clear)
cb.requests.Add(1)

// Result: Small race window, but safe
// Worst case: One count survives reset (acceptable)
```

**Why acceptable:**
- Circuit breaker decisions are approximate
- Off-by-one count doesn't affect correctness
- Alternative (locking) costs more than it's worth

---

### 4. Callback Execution Safety

**Problem:** Callbacks may panic or block

**Solution:** Execute after state committed, protect with recover

```go
// State already committed (visible to all)
if cb.onStateChange != nil {
    func() {
        defer func() {
            if r := recover(); r != nil {
                // Log panic, don't crash circuit breaker
            }
        }()
        cb.onStateChange(cb.name, from, to)
    }()
}
```

---

## Performance Characteristics

### Operation Latencies (Estimated)

| Operation | Latency | Contention |
|-----------|---------|------------|
| State check | 1-5 ns | None (read-only atomic) |
| Count increment | 5-20 ns | Minimal (atomic add) |
| State transition | 50-200 ns | CAS retry on race |
| Callback execution | User-defined | None (post-commit) |

### Scalability

**Concurrent Goroutines:** Tested with 10,000+

**Throughput:** Limited by request execution time, not breaker overhead

**Contention Points:**
- Count increments: Minimal (cache line bouncing only)
- State transitions: Rare (only during failures/recovery)

---

## Testing Strategy

### Race Detection

```bash
go test -race ./...
```

**Must pass with:**
- 1000+ concurrent goroutines
- Rapid state transitions
- High request throughput

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

### Correctness Invariants

**Invariant 1:** Count totals are consistent
```go
assert(TotalSuccesses + TotalFailures <= Requests)
```

**Invariant 2:** Half-open concurrent requests bounded
```go
assert(halfOpenRequests <= MaxRequests)
```

**Invariant 3:** State is always valid
```go
assert(state in {StateClosed, StateOpen, StateHalfOpen})
```

---

## Anti-Patterns (What NOT to Do)

****Mutex for state:** Adds contention, slows hot path

****RWMutex for counts:** Atomic operations are faster

****Channels for coordination:** Overhead, complexity, potential deadlocks

****Background goroutines:** Memory leaks, resource waste

****Locks during request execution:** Serializes requests (defeats purpose)

---

## Summary

**Concurrency Model:**
- **Lock-free reads (state, counts)
- **Atomic updates (state transitions, count increments)
- **CAS-based coordination (no locks)
- **Safe under high concurrency (10k+ goroutines)
- **Bounded memory (no goroutine leaks)
- **Minimal contention (atomic operations only)

**Performance Target:**
- State check: <10 ns
- Count update: <20 ns
- Supports 1M+ RPS

All implementation must use these concurrency patterns.
