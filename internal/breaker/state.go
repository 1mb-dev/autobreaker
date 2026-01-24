package breaker

import (
	"time"
)

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

	// Check if we should trip with panic recovery
	shouldTrip := safeCallReadyToTrip(cb.name, cb.readyToTrip, counts)

	if !shouldTrip {
		return
	}

	// Attempt atomic state transition from Closed to Open
	if !cb.state.CompareAndSwap(int32(StateClosed), int32(StateOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to Open
	// Record the timestamp
	now := time.Now().UnixNano()
	cb.openedAt.Store(now)
	cb.stateChangedAt.Store(now)

	// Defensive reset: ensure halfOpenRequests is 0 when entering Open from Closed
	cb.halfOpenRequests.Store(0)

	// Clear counts
	cb.clearCounts()

	// Call state change callback if configured with panic recovery
	// Note: Callback sees zero counts (clearCounts called before callback)
	safeCallOnStateChange(cb.name, cb.onStateChange, StateClosed, StateOpen)
}

// shouldTransitionToHalfOpen checks if timeout has elapsed since circuit opened.
func (cb *CircuitBreaker) shouldTransitionToHalfOpen() bool {
	openedAt := cb.openedAt.Load()
	if openedAt == 0 {
		return false // Never opened
	}

	// Use monotonic clock for duration calculation to prevent issues from time jumps
	openedTime := time.Unix(0, openedAt)
	elapsed := time.Since(openedTime)
	return elapsed >= cb.getTimeout()
}

// transitionToHalfOpen transitions from Open to HalfOpen state.
func (cb *CircuitBreaker) transitionToHalfOpen() {
	// Attempt atomic state transition from Open to HalfOpen
	if !cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to HalfOpen
	cb.stateChangedAt.Store(time.Now().UnixNano())

	// Clear counts
	cb.clearCounts()

	// Reset half-open request counter
	cb.halfOpenRequests.Store(0)

	// Call state change callback if configured with panic recovery
	safeCallOnStateChange(cb.name, cb.onStateChange, StateOpen, StateHalfOpen)
}

// transitionToClosed transitions from HalfOpen to Closed state (recovery).
func (cb *CircuitBreaker) transitionToClosed() {
	// Attempt atomic state transition from HalfOpen to Closed
	if !cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateClosed)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned to Closed (recovery complete)
	now := time.Now().UnixNano()
	cb.stateChangedAt.Store(now)

	// Clear openedAt timestamp (circuit is no longer open)
	// This ensures clean state and prevents stale timestamp issues
	cb.openedAt.Store(0)

	// Clear counts
	cb.clearCounts()

	// Reset last cleared timestamp
	cb.lastClearedAt.Store(now)

	// Call state change callback if configured with panic recovery
	// Note: Callback sees zero counts (clearCounts called before callback)
	safeCallOnStateChange(cb.name, cb.onStateChange, StateHalfOpen, StateClosed)
}

// transitionBackToOpen transitions from HalfOpen back to Open (failed recovery).
func (cb *CircuitBreaker) transitionBackToOpen() {
	// Attempt atomic state transition from HalfOpen to Open
	if !cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateOpen)) {
		return // Lost race, another goroutine already transitioned
	}

	// Successfully transitioned back to Open
	// Record new open timestamp
	now := time.Now().UnixNano()
	cb.openedAt.Store(now)
	cb.stateChangedAt.Store(now)

	// Defensive reset: ensure halfOpenRequests is 0 when re-entering Open
	cb.halfOpenRequests.Store(0)

	// Clear counts
	cb.clearCounts()

	// Call state change callback if configured with panic recovery
	// Note: Callback sees zero counts (clearCounts called before callback)
	safeCallOnStateChange(cb.name, cb.onStateChange, StateHalfOpen, StateOpen)
}
