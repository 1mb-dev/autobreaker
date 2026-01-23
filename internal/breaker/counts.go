package breaker

import (
	"math"
	"sync/atomic"
	"time"
)

// safeIncrementCounter safely increments a uint32 counter with saturation protection.
// Returns true if the counter was incremented, false if it was already at max.
func safeIncrementCounter(counter *atomic.Uint32) bool {
	// Use CompareAndSwap loop for atomic check-and-increment
	for {
		current := counter.Load()
		if current == math.MaxUint32 {
			// Already at max, cannot increment
			return false
		}
		if counter.CompareAndSwap(current, current+1) {
			return true
		}
		// CAS failed, retry
	}
}

// safeIncrementRequests safely increments the requests counter with saturation protection.
// Returns true if the counter was incremented, false if it was already at max (saturated).
func (cb *CircuitBreaker) safeIncrementRequests() bool {
	return safeIncrementCounter(&cb.requests)
}

// safeDecrementRequests safely decrements the requests counter with underflow protection.
// Returns true if the counter was decremented, false if it was already at 0.
func (cb *CircuitBreaker) safeDecrementRequests() bool {
	// Use CompareAndSwap loop for atomic check-and-decrement
	for {
		current := cb.requests.Load()
		if current == 0 {
			// Already at 0, cannot decrement
			return false
		}
		if cb.requests.CompareAndSwap(current, current-1) {
			return true
		}
		// CAS failed, retry
	}
}

// maybeResetCounts clears counts if interval has elapsed (Closed state only).
func (cb *CircuitBreaker) maybeResetCounts() {
	now := time.Now().UnixNano()
	last := cb.lastClearedAt.Load()

	// Use monotonic clock for duration calculation to prevent issues from time jumps
	lastTime := time.Unix(0, last)
	elapsed := time.Since(lastTime)
	if elapsed >= cb.getInterval() {
		// Try to claim clearing responsibility
		if cb.lastClearedAt.CompareAndSwap(last, now) {
			// We won the race, clear counts
			cb.clearCounts()
		}
	}
}

// clearCounts resets all counters to zero.
func (cb *CircuitBreaker) clearCounts() {
	cb.requests.Store(0)
	cb.totalSuccesses.Store(0)
	cb.totalFailures.Store(0)
	cb.consecutiveSuccesses.Store(0)
	cb.consecutiveFailures.Store(0)
}

// recordOutcome updates counts based on request outcome.
//
// Counters saturate at math.MaxUint32 (4,294,967,295) to prevent undefined overflow behavior.
// Once a counter reaches saturation, it stops incrementing. This ensures predictable
// behavior for long-running services while maintaining thread safety.
//
// Saturation behavior:
// - Counters stop incrementing at math.MaxUint32
// - Statistics (failure rate) become inaccurate after saturation
// - The circuit breaker continues functioning for protection
// - State transitions and interval resets will reset counters to 0
func (cb *CircuitBreaker) recordOutcome(success bool) {
	if success {
		// Safe increment with saturation protection for totalSuccesses
		safeIncrementCounter(&cb.totalSuccesses)
		// ConsecutiveSuccesses can safely overflow as it resets on failure
		cb.consecutiveSuccesses.Add(1)
		cb.consecutiveFailures.Store(0)
	} else {
		// Safe increment with saturation protection for totalFailures
		safeIncrementCounter(&cb.totalFailures)
		// ConsecutiveFailures can safely overflow as it resets on success
		cb.consecutiveFailures.Add(1)
		cb.consecutiveSuccesses.Store(0)
	}
}
