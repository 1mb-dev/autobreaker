// Package autobreaker provides an adaptive circuit breaker for Go.
//
// AutoBreaker automatically adjusts failure thresholds based on traffic patterns,
// eliminating the need for manual tuning across different environments.
//
// Basic usage:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "my-service",
//	})
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return externalService.Call()
//	})
package autobreaker

import (
	"errors"
	"time"
)

// State represents the circuit breaker state.
type State int32

const (
	// StateClosed allows all requests through and tracks failures.
	StateClosed State = iota

	// StateOpen rejects all requests immediately.
	StateOpen

	// StateHalfOpen allows limited requests to test recovery.
	StateHalfOpen
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Counts holds circuit breaker statistics.
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// Settings configures a circuit breaker.
type Settings struct {
	// Name is an identifier for the circuit breaker.
	Name string

	// MaxRequests is the maximum number of concurrent requests allowed in half-open state.
	// Default: 1 if set to 0.
	MaxRequests uint32

	// Interval is the period to clear counts in closed state.
	//
	// Valid range: >= 0 (negative values will panic)
	// Default: 0 (counts are cleared only on state transitions)
	// Common values: 60s for time-based windows, 0 for event-based
	Interval time.Duration

	// Timeout is the duration to wait before transitioning from open to half-open.
	//
	// Valid range: > 0 recommended
	// Default: 60 seconds if set to 0
	// Common values: 10s-120s depending on service recovery time
	Timeout time.Duration

	// ReadyToTrip is called when counts are updated in closed state.
	// If it returns true, the circuit breaker transitions to open.
	// Default: ConsecutiveFailures > 5.
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called when the circuit breaker state changes.
	// Default: nil (no callback).
	OnStateChange func(name string, from State, to State)

	// IsSuccessful determines whether an error should be counted as success or failure.
	// Default: err == nil (only nil errors are successes).
	IsSuccessful func(err error) bool

	// --- Adaptive Settings (AutoBreaker Extensions) ---

	// AdaptiveThreshold enables percentage-based failure thresholds.
	// When true, the circuit breaker uses failure rate (percentage) instead of absolute failure counts.
	// This makes the same configuration work across different traffic levels.
	//
	// Example: With 5% threshold:
	//   Production (1000 req/s): Trips at 50 failures/s
	//   Staging (10 req/s):      Trips at 0.5 failures/s
	//   Dev (1 req/min):         Trips at 0.05 failures/min
	//
	// Default: false (uses static ConsecutiveFailures > 5 for backward compatibility)
	AdaptiveThreshold bool

	// FailureRateThreshold is the failure rate (0.0-1.0) that triggers circuit open.
	// Only used when AdaptiveThreshold is true.
	//
	// Valid range: (0, 1) exclusive - values outside this range will panic
	// Default: 0.05 (5% failure rate) if set to 0
	// Recommended values:
	//   0.01 (1%)  = Strict, for critical services with low error tolerance
	//   0.05 (5%)  = Balanced, good default for most services
	//   0.10 (10%) = Lenient, for services with higher acceptable error rates
	//
	// The threshold is traffic-proportional: it works equally well at any request rate.
	FailureRateThreshold float64

	// MinimumObservations is the minimum number of requests before adaptive logic activates.
	// Prevents false positives during low traffic periods.
	// Only used when AdaptiveThreshold is true.
	//
	// Valid range: > 0 recommended (though 0 will use default)
	// Default: 20 requests if set to 0
	// Recommended values:
	//   10-20   = For high-traffic services with quick feedback
	//   20-50   = Balanced, prevents premature tripping
	//   50-100  = For services where you want high confidence before tripping
	//
	// Example: With MinimumObservations=20 and FailureRateThreshold=0.05:
	//   First 19 requests: Circuit won't trip regardless of failure rate
	//   20+ requests: Circuit trips if failure rate exceeds 5%
	MinimumObservations uint32
}

var (
	// ErrOpenState is returned when the circuit breaker is open.
	ErrOpenState = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests are attempted in half-open state.
	ErrTooManyRequests = errors.New("too many requests")
)

// defaultReadyToTrip returns true after 5 consecutive failures.
func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// defaultIsSuccessful returns true only for nil errors.
func defaultIsSuccessful(err error) bool {
	return err == nil
}
