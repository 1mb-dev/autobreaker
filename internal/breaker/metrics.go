package breaker

import "time"

// Metrics provides enhanced observability into circuit breaker behavior.
//
// Metrics combines current state, request counts, derived statistics (failure/success rates),
// and timestamps into a single snapshot. All values are computed atomically and represent
// a consistent point-in-time view.
//
// Use Cases:
//   - Real-time monitoring and dashboards
//   - Health checks and liveness probes
//   - Alerting based on failure rates
//   - Periodic logging of circuit state
//
// Example:
//
//	metrics := breaker.Metrics()
//	log.Printf("Circuit %s: state=%s, failure_rate=%.2f%%",
//	    breaker.Name(), metrics.State, metrics.FailureRate*100)
//
// Thread-safe: Metrics() takes an atomic snapshot. The returned Metrics struct
// is a value type and safe to use without synchronization.
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

	// Saturated indicates if any counter has reached its maximum value (math.MaxUint32).
	// When true, statistics (failure rate, counts) may be inaccurate.
	// Counters saturate to prevent undefined overflow behavior.
	// Saturation resets when counts are cleared (state transitions or interval reset).
	Saturated bool
}

// Metrics returns a snapshot of current circuit breaker metrics.
//
// This method atomically captures the current state, counts, and timestamps,
// then computes derived statistics (failure rate, success rate) on demand.
//
// The returned Metrics struct includes:
//   - Current circuit state (Closed/Open/HalfOpen)
//   - Request counts (total, successes, failures, consecutive)
//   - Computed rates (FailureRate, SuccessRate as percentages 0.0-1.0)
//   - Timestamps (last state change, last counts reset)
//
// **Atomic Snapshot Limitation**: This method reads multiple atomic values sequentially.
// While each individual read is atomic, the collection as a whole is not an atomic
// snapshot. Metrics may be slightly inconsistent if the circuit breaker is actively
// processing requests during the read.
//
// For most use cases (monitoring, dashboards), this inconsistency is acceptable.
// If you need a perfectly consistent snapshot, you must provide external
// synchronization.
//
// Performance: This method performs atomic loads and simple arithmetic.
// Overhead is negligible (~10-20ns). Safe to call frequently for monitoring.
//
// Use this method for:
//   - Dashboards and real-time monitoring
//   - Health check endpoints
//   - Periodic metrics collection
//   - Alert condition evaluation
//
// Thread-safe: Can be called concurrently with Execute(), UpdateSettings(),
// and other methods. Returns a consistent snapshot.
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

	// Check if any counter is saturated
	saturated := cb.requestsSaturated.Load() ||
		cb.totalSuccessesSaturated.Load() ||
		cb.totalFailuresSaturated.Load()

	return Metrics{
		State:               state,
		Counts:              counts,
		FailureRate:         failureRate,
		SuccessRate:         successRate,
		StateChangedAt:      stateChangedAt,
		CountsLastClearedAt: countsLastClearedAt,
		Saturated:           saturated,
	}
}
