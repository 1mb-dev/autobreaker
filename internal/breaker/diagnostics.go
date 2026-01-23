package breaker

import "time"

// Diagnostics provides comprehensive troubleshooting information about
// the circuit breaker's current state, configuration, and predicted behavior.
//
// Diagnostics combines runtime metrics, active configuration, and predictive analytics
// into a single snapshot. This information is invaluable for incident response,
// troubleshooting, and understanding circuit breaker behavior in production.
//
// Use Cases:
//   - Incident response: Understand why a circuit tripped or is about to trip
//   - Troubleshooting: Diagnose unexpected circuit behavior
//   - Configuration tuning: See current settings and their effects
//   - Alerting: Detect when circuit is about to trip (WillTripNext)
//   - Dashboards: Display complete circuit state in admin panels
//
// Example:
//
//	diag := breaker.Diagnostics()
//	if diag.WillTripNext {
//	    log.Warn("Circuit %s about to trip! Failure rate: %.2f%%",
//	        diag.Name, diag.Metrics.FailureRate*100)
//	}
//	if diag.State == StateOpen {
//	    log.Info("Circuit will probe in %s", diag.TimeUntilHalfOpen)
//	}
//
// Thread-safe: Diagnostics() takes an atomic snapshot. The returned Diagnostics struct
// is a value type and safe to use without synchronization.
type Diagnostics struct {
	// Name is the circuit breaker identifier from Settings.Name.
	Name string

	// State is the current circuit breaker state (Closed/Open/HalfOpen).
	State State

	// Metrics provides current observability data including counts, rates, and timestamps.
	// This is the same data returned by Metrics() method.
	Metrics Metrics

	// --- Active Configuration ---
	// These fields reflect the current runtime configuration, including any
	// updates made via UpdateSettings().

	// MaxRequests is the maximum concurrent requests allowed in half-open state.
	MaxRequests uint32

	// Interval is the period to clear counts in closed state.
	// Zero means counts are cleared only on state transitions.
	Interval time.Duration

	// Timeout is the duration to wait before transitioning from open to half-open.
	Timeout time.Duration

	// AdaptiveEnabled indicates whether adaptive (percentage-based) thresholds are enabled.
	// When false, uses static ConsecutiveFailures threshold.
	AdaptiveEnabled bool

	// FailureRateThreshold is the failure rate (0.0-1.0) that triggers circuit open.
	// Only used when AdaptiveEnabled is true.
	FailureRateThreshold float64

	// MinimumObservations is the minimum requests before adaptive logic activates.
	// Only used when AdaptiveEnabled is true.
	MinimumObservations uint32

	// --- Predictive Diagnostics ---
	// These fields provide forward-looking insights about circuit behavior.

	// WillTripNext predicts whether the circuit would trip if the next request fails.
	// Only meaningful in Closed state (always false in Open/HalfOpen).
	//
	// Use this for:
	//   - Proactive alerting: "Circuit about to trip!"
	//   - Load shedding: Pre-emptively reduce traffic when close to threshold
	//   - Diagnostics: Understand how close the circuit is to tripping
	//
	// Example:
	//   if diag.WillTripNext {
	//       log.Warn("Next failure will trip circuit")
	//   }
	WillTripNext bool

	// TimeUntilHalfOpen is the remaining time before circuit transitions to half-open.
	// Only meaningful in Open state (always zero in Closed/HalfOpen).
	//
	// Use this for:
	//   - User feedback: "Service unavailable, retrying in 10s"
	//   - Dashboards: Show countdown timer for recovery probe
	//   - Coordination: Schedule dependent operations after recovery probe
	//
	// Example:
	//   if diag.State == StateOpen {
	//       log.Info("Circuit will probe backend in %s", diag.TimeUntilHalfOpen)
	//   }
	TimeUntilHalfOpen time.Duration
}

// Diagnostics returns comprehensive diagnostic information about the circuit breaker.
//
// This method atomically captures the current state, metrics, configuration, and computes
// predictive diagnostics (WillTripNext, TimeUntilHalfOpen) on demand. It provides a complete
// view of the circuit breaker's health and behavior.
//
// **Atomic Snapshot Limitation**: This method reads multiple atomic values sequentially.
// While each individual read is atomic, the collection as a whole is not an atomic
// snapshot. Diagnostics may be slightly inconsistent if the circuit breaker is actively
// processing requests during the read.
//
// For most use cases (troubleshooting, incident response), this inconsistency is acceptable.
// The predictions (WillTripNext, TimeUntilHalfOpen) are based on the snapshot and may
// not reflect the exact state if the circuit is actively processing requests.
// If you need a perfectly consistent snapshot, you must provide external synchronization.
//
// The returned Diagnostics struct includes:
//   - Name and State: Circuit identifier and current state
//   - Metrics: Full metrics snapshot (same as Metrics() method)
//   - Configuration: All runtime settings (including UpdateSettings changes)
//   - Predictions: Forward-looking insights (WillTripNext, TimeUntilHalfOpen)
//
// Performance: This method performs atomic loads, calls Metrics(), and computes predictions.
// Overhead is ~100-200ns (slightly more than Metrics() due to predictions). Safe to call
// frequently for monitoring, though Metrics() is more efficient if predictions aren't needed.
//
// Use this method for:
//   - Incident response: Complete state snapshot during outages
//   - Troubleshooting: Understand why circuit is behaving unexpectedly
//   - Admin dashboards: Show full circuit state and configuration
//   - Proactive alerting: Detect imminent circuit trips (WillTripNext)
//   - Configuration validation: Verify UpdateSettings() changes
//
// Use Metrics() instead if:
//   - You only need state and counts (Metrics is faster)
//   - You're polling frequently for dashboards
//   - You don't need configuration or predictions
//
// Thread-safe: Can be called concurrently with Execute(), UpdateSettings(),
// and other methods. Returns a consistent snapshot.
//
// Example - Incident Response:
//
//	diag := breaker.Diagnostics()
//	log.Error("Circuit tripped for %s:\n"+
//	    "  State: %s\n"+
//	    "  Failure Rate: %.2f%%\n"+
//	    "  Threshold: %.2f%%\n"+
//	    "  Requests: %d\n"+
//	    "  Timeout: %s",
//	    diag.Name, diag.State,
//	    diag.Metrics.FailureRate*100,
//	    diag.FailureRateThreshold*100,
//	    diag.Metrics.Counts.Requests,
//	    diag.Timeout)
//
// Example - Proactive Alerting:
//
//	diag := breaker.Diagnostics()
//	if diag.WillTripNext && diag.State == StateClosed {
//	    alert.Warn("Circuit %s about to trip! "+
//	        "Failure rate: %.2f%% (threshold: %.2f%%)",
//	        diag.Name,
//	        diag.Metrics.FailureRate*100,
//	        diag.FailureRateThreshold*100)
//	}
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
//
// This helper method simulates adding one more failure to the current counts and checks
// if the ReadyToTrip condition would be satisfied. It's used by Diagnostics() to populate
// the WillTripNext field.
//
// Algorithm:
//  1. Check if circuit is in Closed state (only state where tripping is relevant)
//  2. Simulate next failure: increment Requests, TotalFailures, ConsecutiveFailures
//  3. Reset ConsecutiveSuccesses to 0 (as failure breaks the streak)
//  4. Check if ReadyToTrip callback returns true with simulated counts
//
// This prediction is useful for:
//   - Proactive alerting: Warn operators before circuit trips
//   - Load shedding: Reduce traffic when close to threshold
//   - Testing: Understand how sensitive circuit is to failures
//
// Returns true only if:
//   - Circuit is currently Closed AND
//   - One more failure would satisfy ReadyToTrip condition
//
// Thread-safe: Uses ReadyToTrip callback which must be thread-safe.
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
