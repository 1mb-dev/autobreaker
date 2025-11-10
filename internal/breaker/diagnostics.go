package breaker

import "time"

// Diagnostics provides comprehensive troubleshooting information about
// the circuit breaker's current state, configuration, and predicted behavior.
type Diagnostics struct {
	// Name is the circuit breaker identifier.
	Name string

	// State is the current circuit breaker state.
	State State

	// Metrics provides current observability data.
	Metrics Metrics

	// Configuration settings
	MaxRequests          uint32
	Interval             time.Duration
	Timeout              time.Duration
	AdaptiveEnabled      bool
	FailureRateThreshold float64
	MinimumObservations  uint32

	// Diagnostic predictions
	WillTripNext      bool          // Would circuit trip on next failure?
	TimeUntilHalfOpen time.Duration // Time remaining before half-open (if open)
}

// Diagnostics returns comprehensive diagnostic information about the circuit breaker.
// This is useful for troubleshooting, debugging, and understanding circuit behavior.
func (cb *CircuitBreaker) Diagnostics() Diagnostics {
	metrics := cb.Metrics()
	state := metrics.State

	// Calculate diagnostic predictions
	willTripNext := cb.wouldTripOnNextFailure(metrics.Counts)

	var timeUntilHalfOpen time.Duration
	if state == StateOpen {
		openedAt := cb.openedAt.Load()
		if openedAt > 0 {
			elapsed := time.Since(time.Unix(0, openedAt))
			remaining := cb.getTimeout() - elapsed
			if remaining > 0 {
				timeUntilHalfOpen = remaining
			}
		}
	}

	return Diagnostics{
		Name:    cb.name,
		State:   state,
		Metrics: metrics,

		// Configuration
		MaxRequests:          cb.getMaxRequests(),
		Interval:             cb.getInterval(),
		Timeout:              cb.getTimeout(),
		AdaptiveEnabled:      cb.adaptiveThreshold,
		FailureRateThreshold: cb.getFailureRateThreshold(),
		MinimumObservations:  cb.getMinimumObservations(),

		// Predictions
		WillTripNext:      willTripNext,
		TimeUntilHalfOpen: timeUntilHalfOpen,
	}
}

// wouldTripOnNextFailure predicts if the circuit would trip if the next request fails.
// This is useful for diagnostics and understanding circuit sensitivity.
func (cb *CircuitBreaker) wouldTripOnNextFailure(counts Counts) bool {
	// Only relevant in Closed state
	if cb.State() != StateClosed {
		return false
	}

	// Simulate what counts would be after one more failure
	simulatedCounts := Counts{
		Requests:             counts.Requests + 1,
		TotalSuccesses:       counts.TotalSuccesses,
		TotalFailures:        counts.TotalFailures + 1,
		ConsecutiveSuccesses: 0, // Reset on failure
		ConsecutiveFailures:  counts.ConsecutiveFailures + 1,
	}

	// Check if readyToTrip would trigger
	return cb.readyToTrip(simulatedCounts)
}
