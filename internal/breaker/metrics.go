package breaker

import "time"

// Metrics provides enhanced observability into circuit breaker behavior.
// It includes current state, counts, derived statistics, and timestamps.
type Metrics struct {
	// State is the current circuit breaker state.
	State State

	// Counts contains request and failure statistics.
	Counts Counts

	// FailureRate is the current failure rate (TotalFailures / Requests).
	// Returns 0 if no requests have been made.
	// Range: [0.0, 1.0]
	FailureRate float64

	// SuccessRate is the current success rate (TotalSuccesses / Requests).
	// Returns 0 if no requests have been made.
	// Range: [0.0, 1.0]
	SuccessRate float64

	// StateChangedAt is the timestamp of the last state transition.
	// Zero value if no state change has occurred yet.
	StateChangedAt time.Time

	// CountsLastClearedAt is the timestamp when counts were last reset.
	// This happens on state transitions or interval-based clearing.
	CountsLastClearedAt time.Time
}

// Metrics returns a snapshot of current circuit breaker metrics.
// This method is safe to call concurrently and computes derived
// statistics on demand.
func (cb *CircuitBreaker) Metrics() Metrics {
	counts := cb.Counts()
	state := cb.State()

	// Calculate derived metrics
	var failureRate, successRate float64
	if counts.Requests > 0 {
		failureRate = float64(counts.TotalFailures) / float64(counts.Requests)
		successRate = float64(counts.TotalSuccesses) / float64(counts.Requests)
	}

	// Get timestamps
	var stateChangedAt time.Time
	if ts := cb.stateChangedAt.Load(); ts > 0 {
		stateChangedAt = time.Unix(0, ts)
	}

	var countsLastClearedAt time.Time
	if ts := cb.lastClearedAt.Load(); ts > 0 {
		countsLastClearedAt = time.Unix(0, ts)
	}

	return Metrics{
		State:               state,
		Counts:              counts,
		FailureRate:         failureRate,
		SuccessRate:         successRate,
		StateChangedAt:      stateChangedAt,
		CountsLastClearedAt: countsLastClearedAt,
	}
}
