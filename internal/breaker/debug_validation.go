//go:build debug

package breaker

import "fmt"

// validateStateMachine validates state machine invariants for debugging.
//
// This method checks internal consistency of the circuit breaker state machine.
// It's intended for debugging and testing purposes only, not for production use.
//
// Validation checks:
//   - State consistency: openedAt should be 0 if not in Open state
//   - Timestamp monotonicity: stateChangedAt should be >= openedAt
//   - Half-open request counter: should be 0 if not in HalfOpen state
//   - Count consistency: totals should equal sum of successes + failures
//
// Returns nil if all invariants hold, or an error describing the first violation found.
//
// Note: This method is not thread-safe. It should only be called when the circuit
// breaker is not actively processing requests (e.g., in tests or debugging).
func (cb *CircuitBreaker) validateStateMachine() error {
	state := cb.State()
	openedAt := cb.openedAt.Load()
	stateChangedAt := cb.stateChangedAt.Load()
	halfOpenRequests := cb.halfOpenRequests.Load()
	counts := cb.Counts()

	// Validate state consistency
	if state == StateClosed && openedAt != 0 {
		return fmt.Errorf("inconsistent: openedAt=%v but state=Closed", openedAt)
	}

	// Validate timestamp monotonicity
	if stateChangedAt < openedAt && openedAt != 0 {
		return fmt.Errorf("timestamps out of order: stateChangedAt=%v < openedAt=%v", stateChangedAt, openedAt)
	}

	// Validate half-open request counter
	if state != StateHalfOpen && halfOpenRequests != 0 {
		return fmt.Errorf("inconsistent: halfOpenRequests=%v but state=%v", halfOpenRequests, state)
	}

	// Validate count consistency
	totalCounted := counts.TotalSuccesses + counts.TotalFailures
	if counts.Requests != totalCounted {
		return fmt.Errorf("count mismatch: Requests=%v != TotalSuccesses+TotalFailures=%v",
			counts.Requests, totalCounted)
	}

	// Validate consecutive counts don't exceed totals
	if counts.ConsecutiveSuccesses > counts.TotalSuccesses {
		return fmt.Errorf("consecutive successes=%v exceeds total successes=%v",
			counts.ConsecutiveSuccesses, counts.TotalSuccesses)
	}

	if counts.ConsecutiveFailures > counts.TotalFailures {
		return fmt.Errorf("consecutive failures=%v exceeds total failures=%v",
			counts.ConsecutiveFailures, counts.TotalFailures)
	}

	return nil
}
