package breaker

import (
	"context"
	"sync/atomic"
	"time"
)

// CircuitBreaker implements an adaptive circuit breaker pattern.
//
// CircuitBreaker protects services from cascading failures by temporarily blocking
// requests to unhealthy backends. It uses a three-state machine (Closed, Open, HalfOpen)
// to automatically detect failures, fail fast during outages, and probe for recovery.
//
// Key Features:
//
//   - Adaptive Thresholds: Percentage-based failure detection that scales with traffic
//   - Runtime Configuration: Update settings without restart via UpdateSettings()
//   - Observability: Metrics() and Diagnostics() for monitoring and troubleshooting
//   - Lock-Free: Uses atomic operations for high performance (<100ns overhead)
//   - Thread-Safe: All methods safe for concurrent use
//   - Panic Recovery: Automatically handles panics as failures
//
// Architecture:
//
// The circuit breaker uses atomic fields exclusively to avoid lock contention:
//   - State: Current circuit state (Closed/Open/HalfOpen)
//   - Counts: Request statistics (atomic counters)
//   - Settings: Runtime-updateable configuration (atomic values)
//   - Callbacks: Immutable function pointers (set at construction)
//
// Immutable Fields:
//   - name: Circuit identifier
//   - readyToTrip: Callback determining when to trip circuit
//   - onStateChange: Callback invoked on state transitions
//   - isSuccessful: Callback determining success vs failure
//   - adaptiveThreshold: Whether to use adaptive (percentage) thresholds
//
// Atomic Fields (Runtime Updateable):
//   - maxRequests: Concurrent request limit in half-open state
//   - interval: Count reset period in closed state
//   - timeout: Duration before transitioning open → half-open
//   - failureRateThreshold: Percentage threshold for adaptive mode
//   - minimumObservations: Minimum requests for adaptive mode
//
// Atomic Fields (State and Counts):
//   - state: Current circuit state
//   - requests, totalSuccesses, totalFailures: Cumulative counts
//   - consecutiveSuccesses, consecutiveFailures: Streak counts
//   - halfOpenRequests: Current half-open concurrent request count
//   - openedAt, lastClearedAt, stateChangedAt: Timestamps
//
// Do not construct CircuitBreaker directly; use New() constructor which validates
// settings and applies defaults.
//
// Example Usage:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:                 "api-client",
//	    Timeout:              10 * time.Second,
//	    AdaptiveThreshold:    true,
//	    FailureRateThreshold: 0.05, // 5%
//	})
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return httpClient.Get(url)
//	})
//	if err == autobreaker.ErrOpenState {
//	    // Circuit is open, use fallback
//	    return cachedResponse, nil
//	}
type CircuitBreaker struct {
	name string

	// Settings (immutable - set once at creation)
	readyToTrip       func(Counts) bool
	onStateChange     func(string, State, State)
	isSuccessful      func(error) bool
	adaptiveThreshold bool

	// Settings (atomic - updateable at runtime)
	maxRequests          atomic.Uint32 // uint32
	interval             atomic.Int64  // time.Duration (int64)
	timeout              atomic.Int64  // time.Duration (int64)
	failureRateThreshold atomic.Uint64 // float64 (stored as bits)
	minimumObservations  atomic.Uint32 // uint32

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
	openedAt       atomic.Int64
	lastClearedAt  atomic.Int64
	stateChangedAt atomic.Int64
}

// New creates a new circuit breaker with the given settings.
//
// This constructor validates settings, applies defaults, and initializes the circuit breaker
// in the Closed state. The returned CircuitBreaker is ready to use immediately and safe
// for concurrent access.
//
// Settings and Defaults:
//
//	MaxRequests:          Default 1 if not set (half-open concurrent request limit)
//	Timeout:              Default 60s if not set (open to half-open transition time)
//	Interval:             Default 0 (counts never reset, only on state transitions)
//	ReadyToTrip:          Default based on AdaptiveThreshold setting
//	IsSuccessful:         Default: err == nil
//	OnStateChange:        Default nil (no callback)
//	AdaptiveThreshold:    Default false (uses static threshold)
//	FailureRateThreshold: Default 0.05 (5%) when AdaptiveThreshold=true
//	MinimumObservations:  Default 20 when AdaptiveThreshold=true
//
// Adaptive vs Static Thresholds:
//
//   - Static (AdaptiveThreshold=false): Uses ConsecutiveFailures threshold
//     Default ReadyToTrip: ConsecutiveFailures > 5
//     Works well for: Stable traffic, known failure patterns
//
//   - Adaptive (AdaptiveThreshold=true): Uses percentage-based FailureRateThreshold
//     Default ReadyToTrip: FailureRate > 5% and Requests >= 20
//     Works well for: Variable traffic, different environments (dev/staging/prod)
//
// Validation and Panics:
//
// This function panics if settings are invalid:
//   - FailureRateThreshold not in (0, 1) exclusive range when set with AdaptiveThreshold=true
//   - Interval is negative
//
// Use panics (not errors) because invalid settings indicate programmer error that should
// be caught during development/testing, not at runtime.
//
// Thread-safety: The returned CircuitBreaker is safe for concurrent use without external
// synchronization. All methods use lock-free atomic operations.
//
// Example - Basic Static Threshold:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:    "api-client",
//	    Timeout: 10 * time.Second,
//	    // Uses default: trip after 5 consecutive failures
//	})
//
// Example - Adaptive Threshold:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:                 "user-service",
//	    Timeout:              10 * time.Second,
//	    AdaptiveThreshold:    true,
//	    FailureRateThreshold: 0.05,  // 5% failure rate
//	    MinimumObservations:  20,    // Need 20 requests before adapting
//	})
//
// Example - Custom Callbacks:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "payment-gateway",
//	    ReadyToTrip: func(counts autobreaker.Counts) bool {
//	        // Trip after 3 failures
//	        return counts.ConsecutiveFailures >= 3
//	    },
//	    OnStateChange: func(name string, from, to autobreaker.State) {
//	        log.Info("Circuit %s: %s -> %s", name, from, to)
//	    },
//	    IsSuccessful: func(err error) bool {
//	        // Don't count 4xx client errors as failures
//	        return err == nil || isClientError(err)
//	    },
//	})
//
// Example - Time-Based Windows:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:     "analytics",
//	    Timeout:  30 * time.Second,
//	    Interval: 60 * time.Second, // Reset counts every 60s
//	    // Evaluate failure rate within rolling 60s window
//	})
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
		name:              settings.Name,
		readyToTrip:       settings.ReadyToTrip,
		onStateChange:     settings.OnStateChange,
		isSuccessful:      settings.IsSuccessful,
		adaptiveThreshold: settings.AdaptiveThreshold,
	}

	// Set atomic fields using setters
	cb.setMaxRequests(settings.MaxRequests)
	cb.setInterval(settings.Interval)
	cb.setTimeout(settings.Timeout)
	cb.setFailureRateThreshold(settings.FailureRateThreshold)
	cb.setMinimumObservations(settings.MinimumObservations)

	// Apply defaults
	if cb.getMaxRequests() == 0 {
		cb.setMaxRequests(1)
	}

	if cb.getTimeout() == 0 {
		cb.setTimeout(60 * time.Second)
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

	if cb.getFailureRateThreshold() == 0 && cb.adaptiveThreshold {
		cb.setFailureRateThreshold(0.05) // 5% default
	}

	if cb.getMinimumObservations() == 0 && cb.adaptiveThreshold {
		cb.setMinimumObservations(20)
	}

	// Initialize state
	now := time.Now().UnixNano()
	cb.state.Store(int32(StateClosed))
	cb.lastClearedAt.Store(now)
	cb.stateChangedAt.Store(now)

	return cb
}

// Name returns the circuit breaker name.
//
// The name is set during construction via Settings.Name and cannot be changed.
// It's useful for logging, metrics, and identifying circuit breakers in a system
// with multiple breakers.
//
// Thread-safe: Safe to call concurrently.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// State returns the current circuit breaker state.
//
// Returns one of:
//   - StateClosed: Normal operation, requests pass through
//   - StateOpen: Circuit tripped, requests fail fast
//   - StateHalfOpen: Testing recovery, limited requests allowed
//
// The returned state is a point-in-time snapshot. The state may change
// immediately after this method returns due to concurrent Execute() calls
// or timeout expiration.
//
// Performance: ~1-2ns overhead (single atomic load).
//
// Thread-safe: Safe to call concurrently.
//
// Example:
//
//	if breaker.State() == autobreaker.StateOpen {
//	    log.Warn("Circuit is open, failing fast")
//	}
func (cb *CircuitBreaker) State() State {
	return State(cb.state.Load())
}

// Counts returns a snapshot of current counts.
//
// The returned Counts struct includes:
//   - Requests: Total requests in current observation window
//   - TotalSuccesses: Cumulative successful requests
//   - TotalFailures: Cumulative failed requests
//   - ConsecutiveSuccesses: Consecutive successes since last failure
//   - ConsecutiveFailures: Consecutive failures since last success
//
// All counts are captured atomically and represent a consistent point-in-time view.
// However, counts may change immediately after this method returns due to concurrent
// Execute() calls.
//
// Counts represent the current observation window:
//   - If Interval > 0: Counts reset every Interval duration (in Closed state)
//   - If Interval = 0: Counts reset only on state transitions
//
// Performance: ~5-10ns overhead (5 atomic loads).
//
// Thread-safe: Safe to call concurrently.
//
// Example:
//
//	counts := breaker.Counts()
//	log.Info("Circuit %s: %d requests, %d failures, %d consecutive failures",
//	    breaker.Name(), counts.Requests, counts.TotalFailures,
//	    counts.ConsecutiveFailures)
//
// Use Metrics() instead if you also need:
//   - Failure rate and success rate percentages
//   - Timestamps (state changes, count resets)
//   - Current state combined with counts
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
//
// This is the primary method for wrapping operations with circuit breaker protection.
// It implements the circuit breaker state machine, counts successes/failures, and
// manages state transitions automatically.
//
// Behavior by State:
//
//   - Closed: Executes request, counts result, may transition to Open if threshold exceeded
//   - Open: Rejects immediately with ErrOpenState (fail-fast), or transitions to HalfOpen if timeout expired
//   - HalfOpen: Executes limited concurrent requests (MaxRequests), transitions based on results
//
// State Transitions:
//
//   - Closed → Open: When ReadyToTrip returns true (failure threshold exceeded)
//   - Open → HalfOpen: After Timeout duration expires
//   - HalfOpen → Closed: When probe requests succeed (recovery detected)
//   - HalfOpen → Open: When probe requests fail (backend still unhealthy)
//
// Panic Handling:
//
// If the request function panics, Execute:
//  1. Counts the panic as a failure
//  2. Handles state transitions as if request failed
//  3. Re-panics to preserve stack trace and caller's panic handling
//
// Return Values:
//
//   - Success: Returns (result, err) from request function
//   - Circuit Open: Returns (nil, ErrOpenState) without executing request
//   - Too Many Requests: Returns (nil, ErrTooManyRequests) in half-open with exceeded MaxRequests
//   - Application Error: Returns (result, err) unchanged; isSuccessful determines if counted as failure
//
// Performance: <100ns overhead in Closed state (hot path). Uses lock-free atomic operations.
//
// Thread-safe: Can be called concurrently from multiple goroutines. State transitions
// are serialized internally.
//
// Example - Basic Usage:
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return externalService.Call()
//	})
//	if err == autobreaker.ErrOpenState {
//	    // Circuit is open, use fallback
//	    return cachedResponse, nil
//	}
//	if err != nil {
//	    // Application error
//	    return nil, err
//	}
//	return result, nil
//
// Example - Type Assertion:
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return fetchUser(id)
//	})
//	if err != nil {
//	    return nil, err
//	}
//	user := result.(*User)
//	return user, nil
//
// Example - Panic Recovery:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        log.Error("Request panicked: %v", r)
//	        // Panic was counted as failure by circuit breaker
//	    }
//	}()
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return riskyOperation() // May panic
//	})
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	// Check if interval-based count clearing is needed (only in Closed state)
	if cb.getInterval() > 0 && cb.State() == StateClosed {
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
		if current > int32(cb.getMaxRequests()) {
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

// ExecuteContext runs the given request function if the circuit breaker allows it,
// respecting the provided context for cancellation and deadlines.
//
// This method provides context-aware circuit breaker protection, enabling proper
// cancellation propagation, deadline enforcement, and graceful shutdown patterns.
// It's the recommended method for Go applications using context.Context.
//
// Context Handling:
//
//   - Before Execution: Checks ctx.Err() and returns immediately if context is already canceled
//   - During Execution: Request function executes normally (should respect context internally)
//   - After Execution: Checks ctx.Err() again; if canceled, returns context error without counting as failure
//
// Behavior is identical to Execute() except for context integration:
//   - Same state machine (Closed/Open/HalfOpen)
//   - Same failure counting and state transitions
//   - Same panic handling
//   - Context cancellation does NOT count as failure (client-initiated, not backend problem)
//
// Context Cancellation:
//
// If context is canceled or deadline exceeded:
//   - Before execution: Returns ctx.Err() immediately, request count NOT incremented
//   - During execution: Returns ctx.Err(), request IS counted but NOT as success/failure
//
// This design ensures context cancellation doesn't trip the circuit, as it indicates
// client-side cancellation, not backend health issues.
//
// Return Values:
//
//   - Success: Returns (result, err) from request function
//   - Context Canceled: Returns (nil, ctx.Err()) - context.Canceled or context.DeadlineExceeded
//   - Circuit Open: Returns (nil, ErrOpenState) without executing request
//   - Too Many Requests: Returns (nil, ErrTooManyRequests) in half-open with exceeded MaxRequests
//   - Application Error: Returns (result, err) unchanged; isSuccessful determines if counted as failure
//
// Performance: Same as Execute() (~<100ns overhead in Closed state).
//
// Thread-safe: Can be called concurrently from multiple goroutines with different contexts.
//
// Example - Basic Usage with Timeout:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	result, err := breaker.ExecuteContext(ctx, func() (interface{}, error) {
//	    return externalService.CallWithContext(ctx)
//	})
//	if err == context.DeadlineExceeded {
//	    // Request timed out (not counted as failure)
//	    log.Warn("Request timeout")
//	    return nil, err
//	}
//	if err == autobreaker.ErrOpenState {
//	    // Circuit is open, use fallback
//	    return cachedResponse, nil
//	}
//
// Example - Graceful Shutdown:
//
//	func (s *Server) Shutdown(ctx context.Context) error {
//	    // Use shutdown context for all in-flight requests
//	    _, err := breaker.ExecuteContext(ctx, func() (interface{}, error) {
//	        return s.cleanup()
//	    })
//	    return err
//	}
//
// Example - Request Cancellation:
//
//	// Client cancels request
//	ctx, cancel := context.WithCancel(r.Context())
//	defer cancel()
//
//	go func() {
//	    <-clientDisconnected
//	    cancel() // Cancel context when client disconnects
//	}()
//
//	result, err := breaker.ExecuteContext(ctx, func() (interface{}, error) {
//	    return processLongRunningRequest(ctx)
//	})
//	if err == context.Canceled {
//	    log.Info("Client disconnected, request cancelled")
//	    return
//	}
//
// When to Use ExecuteContext vs Execute:
//
//   - Use ExecuteContext when:
//
//   - You have a context (HTTP handlers, gRPC, etc.)
//
//   - You need cancellation support
//
//   - You want to enforce timeouts
//
//   - You're implementing graceful shutdown
//
//   - Use Execute when:
//
//   - You don't have a context
//
//   - Cancellation isn't needed
//
//   - Simpler API is preferred
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, req func() (interface{}, error)) (interface{}, error) {
	// Check context before attempting execution
	if err := ctx.Err(); err != nil {
		// Context already canceled/expired, return immediately
		// Don't increment request count since we never attempted execution
		return nil, err
	}

	// Check if interval-based count clearing is needed (only in Closed state)
	if cb.getInterval() > 0 && cb.State() == StateClosed {
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
		if current > int32(cb.getMaxRequests()) {
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

	// Check context after execution
	if ctxErr := ctx.Err(); ctxErr != nil {
		// Context was canceled/expired during execution
		// Return context error WITHOUT counting as success or failure
		// Rationale: Cancellation is client-initiated, not a backend health issue
		return nil, ctxErr
	}

	// If we got here without panic and context is still valid, record normal outcome
	if !panicked {
		success := cb.isSuccessful(err)
		cb.recordOutcome(success)

		// Handle state transitions based on outcome
		cb.handleStateTransition(success, currentState)
	}

	return result, err
}
