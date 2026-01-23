package breaker

import "time"

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
func (cb *CircuitBreaker) recordOutcome(success bool) {
	if success {
		cb.totalSuccesses.Add(1)
		cb.consecutiveSuccesses.Add(1)
		cb.consecutiveFailures.Store(0)
	} else {
		cb.totalFailures.Add(1)
		cb.consecutiveFailures.Add(1)
		cb.consecutiveSuccesses.Store(0)
	}
}
