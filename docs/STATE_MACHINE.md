# AutoBreaker State Machine Specification

## Overview

AutoBreaker implements a three-state circuit breaker pattern with adaptive thresholds. This document defines exact state transition conditions, edge cases, and behavior guarantees.

## States

```
┌─────────┐
│ CLOSED  │ ─┐ Normal operation, requests flow through
└─────────┘  │
     │       │
     │ Trip  │ Success after interval
     ▼       │
┌─────────┐  │
│  OPEN   │ ─┘ Rejecting requests, waiting for timeout
└─────────┘
     │
     │ Timeout expires
     ▼
┌───────────┐
│ HALF-OPEN │   Testing recovery with limited requests
└───────────┘
     │     │
     │     │ Success → CLOSED
     │     │ Failure → OPEN
```

### State Definitions

**CLOSED (0)**
- Normal operation mode
- All requests are allowed through
- Counts track success/failure statistics
- Transitions to OPEN when `ReadyToTrip` condition is met

**OPEN (1)**
- Protection mode - service assumed unhealthy
- All requests are immediately rejected with `ErrOpenState`
- No operations are executed
- After `Timeout` duration, transitions to HALF-OPEN

**HALF-OPEN (2)**
- Recovery testing mode
- Limited requests allowed (up to `MaxRequests` concurrent)
- First failure → immediate transition to OPEN
- First success → immediate transition to CLOSED
- Excess concurrent requests rejected with `ErrTooManyRequests`

## State Transitions

### CLOSED → OPEN

**Trigger:** `ReadyToTrip(counts) == true`

**When Evaluated:** After each request completes (success or failure)

**Default Condition:** `ConsecutiveFailures > 5`

**Adaptive Enhancement:**
```go
// Traditional (absolute)
ConsecutiveFailures > threshold

// Adaptive (percentage-based)
(TotalFailures / Requests) > failureRateThreshold
```

**Actions on Transition:**
1. Set state to OPEN
2. Record transition timestamp
3. Clear all counts
4. Call `OnStateChange(name, CLOSED, OPEN)` if configured
5. Start timeout timer

**Thread Safety:** Must be atomic - no partial state during transition

---

### OPEN → HALF-OPEN

**Trigger:** `time.Since(openStateTimestamp) >= Timeout`

**When Evaluated:** On next request arrival after timeout expires

**Default Timeout:** 60 seconds (if Settings.Timeout == 0)

**Actions on Transition:**
1. Set state to HALF-OPEN
2. Clear all counts
3. Reset concurrent request counter to 0
4. Call `OnStateChange(name, OPEN, HALF-OPEN)` if configured

**Note:** Transition is lazy - happens on first request after timeout, not automatically

---

### HALF-OPEN → CLOSED

**Trigger:** First successful request completion in HALF-OPEN state

**Actions on Transition:**
1. Set state to CLOSED
2. Clear all counts
3. Call `OnStateChange(name, HALF-OPEN, CLOSED)` if configured

**Important:** Single success is sufficient (conservative recovery)

---

### HALF-OPEN → OPEN

**Trigger:** First failed request in HALF-OPEN state

**Actions on Transition:**
1. Set state to OPEN
2. Record new open timestamp
3. Clear all counts
4. Call `OnStateChange(name, HALF-OPEN, OPEN)` if configured
5. Restart timeout timer

**Important:** Single failure causes immediate re-opening (fail-fast recovery)

---

## Counts Management

### Counts Structure
```go
type Counts struct {
    Requests             uint32  // Total requests in current state
    TotalSuccesses       uint32  // Total successful requests
    TotalFailures        uint32  // Total failed requests
    ConsecutiveSuccesses uint32  // Streak of successes
    ConsecutiveFailures  uint32  // Streak of failures
}
```

### Count Update Rules

**On Request Start:**
- `Requests++` (in CLOSED and HALF-OPEN states)

**On Successful Completion:**
- `TotalSuccesses++`
- `ConsecutiveSuccesses++`
- `ConsecutiveFailures = 0`

**On Failed Completion:**
- `TotalFailures++`
- `ConsecutiveFailures++`
- `ConsecutiveSuccesses = 0`

**On State Transition:**
- All counts reset to 0

**On Interval Expiry (CLOSED state only):**
- If `Interval > 0` and time since last clear > Interval
- Reset all counts to 0
- Does NOT trigger state change

### Thread Safety Requirements
- All count updates must be atomic
- Use `sync/atomic` package for lock-free updates
- No torn reads/writes of multi-field updates

---

## Concurrency Guarantees

### MaxRequests Enforcement (HALF-OPEN)

**Rule:** Maximum `MaxRequests` concurrent requests in HALF-OPEN state

**Implementation:**
```go
// Atomic counter for concurrent requests
var halfOpenRequests atomic.Int32

// On request entry in HALF-OPEN:
if halfOpenRequests.Add(1) > MaxRequests {
    halfOpenRequests.Add(-1)
    return ErrTooManyRequests
}

// On request completion (success or failure):
halfOpenRequests.Add(-1)
```

**Edge Case:** State transition during request execution
- Requests started in HALF-OPEN complete normally even if state changes
- Counter decremented regardless of final state

---

## Error Classification

### Success vs Failure Determination

**Default Rule:** `IsSuccessful(err) == (err == nil)`

**Custom Classification:** User-provided `IsSuccessful func(err error) bool`

**Examples:**
```go
// Treat specific errors as success (e.g., 404 not found)
IsSuccessful: func(err error) bool {
    if err == nil {
        return true
    }
    if errors.Is(err, ErrNotFound) {
        return true // Don't count 404 as failure
    }
    return false
}

// Only count 5xx errors as failures
IsSuccessful: func(err error) bool {
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.StatusCode < 500
    }
    return err == nil
}
```

### Panic Handling

**Behavior:** Panics are treated as failures

**Implementation:**
```go
defer func() {
    if r := recover(); r != nil {
        // Record as failure
        recordFailure()
        // Re-panic to preserve stack trace
        panic(r)
    }
}()
```

**Rationale:** Panics indicate code defects or unexpected states, should trip breaker

---

## Edge Cases & Special Scenarios

### 1. Time Jumps (NTP sync, clock adjustments)

**Problem:** System clock moves backward during timeout period

**Solution:** Use monotonic time (`time.Now()` includes monotonic clock in Go 1.9+)

**Validation:**
```go
// Safe against time jumps
elapsed := time.Since(startTime) // Uses monotonic clock
```

---

### 2. Concurrent State Transitions

**Problem:** Multiple goroutines trigger state transition simultaneously

**Solution:** Use atomic compare-and-swap (CAS) for state changes

**Implementation:**
```go
// Only one goroutine succeeds in changing state
if atomic.CompareAndSwapInt32(&breaker.state, CLOSED, OPEN) {
    // This goroutine won the race, perform transition actions
    breaker.onTransition(CLOSED, OPEN)
}
```

---

### 3. Context Cancellation

**Problem:** Request context cancelled during execution

**Behavior:** Treat as failure (request didn't succeed)

**Rationale:** Context cancellation indicates timeout or client abandonment

**Exception:** User can override via `IsSuccessful` if desired

---

### 4. Zero/Negative Durations

**Timeout = 0:** Use default (60 seconds)

**Timeout < 0:** Invalid, treat as 0 (use default)

**Interval = 0:** No periodic count clearing (counts persist until state change)

**Interval < 0:** Invalid, treat as 0 (no clearing)

---

### 5. MaxRequests Edge Cases

**MaxRequests = 0:** Treat as 1 (default)

**MaxRequests = 1:** Only one probe request at a time (conservative)

**MaxRequests >> traffic:** Effectively no limit in HALF-OPEN (aggressive recovery)

---

### 6. Rapid State Oscillation (Flapping)

**Problem:** HALF-OPEN → OPEN → HALF-OPEN → OPEN (rapid cycling)

**Current Behavior:** Allowed (fail-fast recovery)

**Future Enhancement (Phase 2+):**
- Track state change frequency
- Increase timeout on repeated failures
- Exponential backoff

---

## Adaptive Threshold Algorithm (Phase 2)

### Basic Adaptive Logic

**Traditional ReadyToTrip:**
```go
func(counts Counts) bool {
    return counts.ConsecutiveFailures > 5
}
```

**Adaptive ReadyToTrip:**
```go
func(counts Counts) bool {
    if counts.Requests < minObservations {
        return false // Not enough data yet
    }

    failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
    return failureRate > failureRateThreshold // e.g., 0.05 = 5%
}
```

**Key Differences:**
- Absolute count (5 failures) → Percentage (5% failure rate)
- Works across different traffic volumes
- Requires minimum observations to avoid false positives on low traffic

### Adaptive Threshold Bounds

**Minimum Observations:** Prevent false positives on low traffic
```go
minObservations := 20 // Don't trip on <20 requests
```

**Maximum Failure Rate:** Cap for safety
```go
maxFailureRate := 0.50 // Never allow >50% errors
```

**Minimum Failure Rate:** Floor for sensitivity
```go
minFailureRate := 0.01 // Always trip at >1% errors if enough volume
```

---

## Performance Requirements

**State Check Latency:** <50ns (lock-free atomic read)

**Transition Latency:** <1μs (atomic CAS + callback)

**Count Update Latency:** <20ns (atomic increment)

**Memory Overhead:** <200 bytes per breaker instance

**Concurrency:** Must handle 1M+ RPS on modern hardware

---

## Testing Requirements

### State Transition Tests
- [ ] CLOSED → OPEN on ReadyToTrip
- [ ] OPEN → HALF-OPEN after timeout
- [ ] HALF-OPEN → CLOSED on success
- [ ] HALF-OPEN → OPEN on failure

### Concurrency Tests
- [ ] Race detector passes with 1000+ concurrent goroutines
- [ ] MaxRequests enforced correctly under load
- [ ] No lost counts under concurrent updates

### Edge Case Tests
- [ ] Panic recovery and re-panic
- [ ] Context cancellation handling
- [ ] Zero/negative duration handling
- [ ] Time jump resilience
- [ ] State transition during request execution

### Adaptive Threshold Tests (Phase 2)
- [ ] Low traffic: 10 RPS, 5% error rate triggers
- [ ] High traffic: 1000 RPS, 5% error rate triggers
- [ ] Minimum observation threshold enforced

---

## Compatibility Notes

**sony/gobreaker Parity:**
- State values: 0=Closed, 1=HalfOpen, 2=Open
- Error types: `ErrOpenState`, `ErrTooManyRequests`
- Settings struct field names and types
- Counts struct fields
- Callback signatures

**Deviations (Enhancements):**
- Adaptive thresholds (opt-in via Settings)
- Additional metrics exposure (Phase 3)

---

## Summary

This state machine provides:
- **Clear, deterministic behavior
- **Thread-safe state transitions
- **Backward compatible with sony/gobreaker
- **Foundation for adaptive enhancements
- **Edge case handling
- **Performance guarantees

All implementation must adhere to these specifications.
