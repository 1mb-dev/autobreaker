package breaker

import (
	"sync/atomic"
	"time"
)

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
			cb.readyToTrip = DefaultReadyToTrip
		}
	}

	if cb.isSuccessful == nil {
		cb.isSuccessful = DefaultIsSuccessful
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

// Execute runs the given request function if the circuit breaker allows it.
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	// Check if interval-based count clearing is needed (only in Closed state)
	if cb.interval > 0 && cb.State() == StateClosed {
		cb.maybeResetCounts()
	}

	// Capture current state for state machine logic
	currentState := cb.State()

	// Check state and handle accordingly
	switch currentState {
	case StateOpen:
		// Circuit is open - check if we should transition to half-open
		if cb.shouldTransitionToHalfOpen() {
			cb.transitionToHalfOpen()
			currentState = StateHalfOpen // Update local state
			// Fall through to half-open handling
		} else {
			// Reject immediately without counting as a request
			return nil, ErrOpenState
		}
	}

	// Request is allowed - increment count
	cb.requests.Add(1)

	// Handle half-open state with request limiting
	if currentState == StateHalfOpen {
		// Check if we've reached max concurrent requests in half-open
		current := cb.halfOpenRequests.Add(1)
		if current > int32(cb.maxRequests) {
			cb.halfOpenRequests.Add(-1) // Undo increment
			return nil, ErrTooManyRequests
		}
		defer cb.halfOpenRequests.Add(-1)
	}

	// Execute the request with panic recovery
	var result interface{}
	var err error
	panicked := false

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
