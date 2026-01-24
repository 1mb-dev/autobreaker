package breaker

import (
	"time"
)

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

// clearCounts resets all counters to zero and clears saturation flags.
func (cb *CircuitBreaker) clearCounts() {
	cb.requests.Store(0)
	cb.totalSuccesses.Store(0)
	cb.totalFailures.Store(0)
	cb.consecutiveSuccesses.Store(0)
	cb.consecutiveFailures.Store(0)

	// Reset saturation flags so warnings can be logged again after counts are cleared
	cb.requestsSaturated.Store(false)
	cb.totalSuccessesSaturated.Store(false)
	cb.totalFailuresSaturated.Store(false)
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
		safeIncrementCounter(&cb.totalSuccesses, &cb.totalSuccessesSaturated, "totalSuccesses", cb.name)
		// ConsecutiveSuccesses can safely overflow as it resets on failure
		cb.consecutiveSuccesses.Add(1)
		cb.consecutiveFailures.Store(0)
	} else {
		// Safe increment with saturation protection for totalFailures
		safeIncrementCounter(&cb.totalFailures, &cb.totalFailuresSaturated, "totalFailures", cb.name)
		// ConsecutiveFailures can safely overflow as it resets on success
		cb.consecutiveFailures.Add(1)
		cb.consecutiveSuccesses.Store(0)
	}
}
