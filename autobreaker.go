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
	"sync/atomic"
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

// CircuitBreaker implements an adaptive circuit breaker pattern.
type CircuitBreaker struct {
	name string

	// Settings
	maxRequests          uint32
	interval             time.Duration
	timeout              time.Duration
	readyToTrip          func(Counts) bool
	onStateChange        func(string, State, State)
	isSuccessful         func(error) bool
	adaptiveThreshold    bool
	failureRateThreshold float64
	minimumObservations  uint32

	// State (atomic)
	state atomic.Int32 // State (0=Closed, 1=Open, 2=HalfOpen)

	// Counts (atomic)
	requests             atomic.Uint32
	totalSuccesses       atomic.Uint32
	totalFailures        atomic.Uint32
	consecutiveSuccesses atomic.Uint32
	consecutiveFailures  atomic.Uint32

	// Half-open limiter (atomic)
	halfOpenRequests atomic.Int32

	// Timestamps (atomic, int64 nanoseconds)
	openedAt      atomic.Int64
	lastClearedAt atomic.Int64
}

var (
	// ErrOpenState is returned when the circuit breaker is open.
	ErrOpenState = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests are attempted in half-open state.
	ErrTooManyRequests = errors.New("too many requests")
)

// New creates a new circuit breaker with the given settings.
// It panics if adaptive threshold settings are invalid.
func New(settings Settings) *CircuitBreaker {
	// Validate adaptive threshold settings
	if settings.AdaptiveThreshold {
		// FailureRateThreshold must be in (0, 1) exclusive range if explicitly set
		if settings.FailureRateThreshold != 0 {
			if settings.FailureRateThreshold <= 0 || settings.FailureRateThreshold >= 1 {
				panic("autobreaker: FailureRateThreshold must be in range (0, 1)")
			}
		}
	}

	// Validate Interval (can be 0 for no reset, but not negative)
	if settings.Interval < 0 {
		panic("autobreaker: Interval cannot be negative")
	}

	cb := &CircuitBreaker{
		name:                 settings.Name,
		maxRequests:          settings.MaxRequests,
		interval:             settings.Interval,
		timeout:              settings.Timeout,
		readyToTrip:          settings.ReadyToTrip,
		onStateChange:        settings.OnStateChange,
		isSuccessful:         settings.IsSuccessful,
		adaptiveThreshold:    settings.AdaptiveThreshold,
		failureRateThreshold: settings.FailureRateThreshold,
		minimumObservations:  settings.MinimumObservations,
	}

	// Apply defaults
	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.timeout == 0 {
		cb.timeout = 60 * time.Second
	}

	if cb.readyToTrip == nil {
		if cb.adaptiveThreshold {
			cb.readyToTrip = cb.defaultAdaptiveReadyToTrip
		} else {
			cb.readyToTrip = defaultReadyToTrip
		}
	}

	if cb.isSuccessful == nil {
		cb.isSuccessful = defaultIsSuccessful
	}

	if cb.failureRateThreshold == 0 && cb.adaptiveThreshold {
		cb.failureRateThreshold = 0.05 // 5% default
	}

	if cb.minimumObservations == 0 && cb.adaptiveThreshold {
		cb.minimumObservations = 20
	}

	// Initialize state
	cb.state.Store(int32(StateClosed))
	cb.lastClearedAt.Store(time.Now().UnixNano())

	return cb
}

// Name returns the circuit breaker name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	return State(cb.state.Load())
}

// Counts returns a snapshot of current counts.
func (cb *CircuitBreaker) Counts() Counts {
	return Counts{
		Requests:             cb.requests.Load(),
		TotalSuccesses:       cb.totalSuccesses.Load(),
		TotalFailures:        cb.totalFailures.Load(),
		ConsecutiveSuccesses: cb.consecutiveSuccesses.Load(),
		ConsecutiveFailures:  cb.consecutiveFailures.Load(),
	}
}

// Execute runs the given function if the circuit breaker allows it.
// Returns the result and error from the function, or a circuit breaker error.
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	// Get current state
	currentState := cb.State()

	// Handle based on state
	switch currentState {
	case StateOpen:
		// Check if timeout has elapsed (lazy transition to half-open)
		if cb.shouldTransitionToHalfOpen() {
			cb.transitionToHalfOpen()
			// Fall through to half-open handling below
			currentState = StateHalfOpen
		} else {
			// Circuit is still open - reject request immediately
			return nil, ErrOpenState
		}

	case StateHalfOpen:
		// Half-open state - check concurrent request limit
		current := cb.halfOpenRequests.Add(1)
		if current > int32(cb.maxRequests) {
			cb.halfOpenRequests.Add(-1)
			return nil, ErrTooManyRequests
		}
		defer cb.halfOpenRequests.Add(-1)

	case StateClosed:
		// Closed state - normal operation
		// Check if we need to clear counts based on interval
		if cb.interval > 0 {
			cb.maybeResetCounts()
		}
	}

	// Increment request counter
	cb.requests.Add(1)

	// Execute the request with panic recovery
	var result interface{}
	var err error
	var panicked bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic occurred - treat as failure
				panicked = true
				// Record panic as failure
				cb.recordOutcome(false)

				// Handle state transitions for panic (same as failure)
				cb.handleStateTransition(false, currentState)

				// Re-panic to preserve stack trace
				panic(r)
			}
		}()

		result, err = req()
	}()

	// If we got here without panic, record normal outcome
	if !panicked {
		success := cb.isSuccessful(err)
		cb.recordOutcome(success)

		// Handle state transitions based on outcome
		cb.handleStateTransition(success, currentState)
	}

	return result, err
}

// maybeResetCounts clears counts if interval has elapsed (Closed state only).
func (cb *CircuitBreaker) maybeResetCounts() {
	now := time.Now().UnixNano()
	last := cb.lastClearedAt.Load()

	// Check if interval has elapsed
	if time.Duration(now-last) >= cb.interval {
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

// handleStateTransition handles state machine transitions based on request outcome.
func (cb *CircuitBreaker) handleStateTransition(success bool, currentState State) {
	switch currentState {
	case StateClosed:
		// Only check for trip on failure (Closed → Open)
		if !success {
			cb.checkAndTripCircuit()
		}
	case StateHalfOpen:
		// Transition based on outcome (HalfOpen → Closed or Open)
		if success {
			cb.transitionToClosed()
		} else {
			cb.transitionBackToOpen()
		}
	}
}

// checkAndTripCircuit evaluates ReadyToTrip and transitions to Open if needed.
func (cb *CircuitBreaker) checkAndTripCircuit() {
	counts := cb.Counts()

	// Check if we should trip
	if !cb.readyToTrip(counts) {
		return
	}

	// Attempt atomic state transition from Closed to Open
	if !cb.state.CompareAndSwap(int32(StateClosed), int32(StateOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to Open
	// Record the timestamp
	cb.openedAt.Store(time.Now().UnixNano())

	// Clear counts
	cb.clearCounts()

	// Call state change callback if configured
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, StateClosed, StateOpen)
	}
}

// shouldTransitionToHalfOpen checks if timeout has elapsed since circuit opened.
func (cb *CircuitBreaker) shouldTransitionToHalfOpen() bool {
	openedAt := cb.openedAt.Load()
	if openedAt == 0 {
		return false // Never opened
	}

	elapsed := time.Duration(time.Now().UnixNano() - openedAt)
	return elapsed >= cb.timeout
}

// transitionToHalfOpen transitions from Open to HalfOpen state.
func (cb *CircuitBreaker) transitionToHalfOpen() {
	// Attempt atomic state transition from Open to HalfOpen
	if !cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to HalfOpen
	// Clear counts
	cb.clearCounts()

	// Reset half-open request counter
	cb.halfOpenRequests.Store(0)

	// Call state change callback if configured
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, StateOpen, StateHalfOpen)
	}
}

// transitionToClosed transitions from HalfOpen to Closed state (recovery).
func (cb *CircuitBreaker) transitionToClosed() {
	// Attempt atomic state transition from HalfOpen to Closed
	if !cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateClosed)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to Closed (recovery complete)
	// Clear counts
	cb.clearCounts()

	// Reset last cleared timestamp
	cb.lastClearedAt.Store(time.Now().UnixNano())

	// Call state change callback if configured
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, StateHalfOpen, StateClosed)
	}
}

// transitionBackToOpen transitions from HalfOpen back to Open (failed recovery).
func (cb *CircuitBreaker) transitionBackToOpen() {
	// Attempt atomic state transition from HalfOpen to Open
	if !cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned back to Open
	// Record new open timestamp
	cb.openedAt.Store(time.Now().UnixNano())

	// Clear counts
	cb.clearCounts()

	// Call state change callback if configured
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, StateHalfOpen, StateOpen)
	}
}

// Default ReadyToTrip: trip after 5 consecutive failures.
func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// Default IsSuccessful: only nil errors are successes.
func defaultIsSuccessful(err error) bool {
	return err == nil
}

// Adaptive ReadyToTrip: trip when failure rate exceeds threshold.
func (cb *CircuitBreaker) defaultAdaptiveReadyToTrip(counts Counts) bool {
	// Need minimum observations to avoid false positives
	if counts.Requests < cb.minimumObservations {
		return false
	}

	// Calculate failure rate
	failureRate := float64(counts.TotalFailures) / float64(counts.Requests)

	return failureRate > cb.failureRateThreshold
}
