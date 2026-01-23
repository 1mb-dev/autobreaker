//go:build debug

package breaker

import (
	"testing"
	"time"
)

func TestValidateStateMachine(t *testing.T) {
	t.Run("ValidClosedState", func(t *testing.T) {
		cb := New(Settings{
			Name: "test-validate-closed",
		})

		if err := cb.validateStateMachine(); err != nil {
			t.Errorf("Valid closed state should pass validation: %v", err)
		}
	})

	t.Run("ValidOpenState", func(t *testing.T) {
		cb := New(Settings{
			Name:    "test-validate-open",
			Timeout: 10 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures > 0
			},
		})

		// Trip circuit to open state
		cb.Execute(failFunc)

		if err := cb.validateStateMachine(); err != nil {
			t.Errorf("Valid open state should pass validation: %v", err)
		}
	})

	t.Run("ValidHalfOpenState", func(t *testing.T) {
		cb := New(Settings{
			Name:    "test-validate-halfopen",
			Timeout: 1 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures > 0
			},
		})

		// Trip circuit
		cb.Execute(failFunc)

		// Wait for timeout
		time.Sleep(2 * time.Millisecond)

		// Transition to half-open
		cb.transitionToHalfOpen()

		if err := cb.validateStateMachine(); err != nil {
			t.Errorf("Valid half-open state should pass validation: %v", err)
		}
	})

	t.Run("InvalidOpenedAtInClosedState", func(t *testing.T) {
		cb := New(Settings{
			Name: "test-invalid-openedat",
		})

		// Manually set openedAt while in closed state (simulating bug)
		cb.openedAt.Store(time.Now().UnixNano())

		if err := cb.validateStateMachine(); err == nil {
			t.Error("Should detect inconsistency: openedAt set but state is not Open")
		}
	})

	t.Run("InvalidHalfOpenRequestsInClosedState", func(t *testing.T) {
		cb := New(Settings{
			Name: "test-invalid-halfopen-requests",
		})

		// Manually set halfOpenRequests while in closed state
		cb.halfOpenRequests.Store(5)

		if err := cb.validateStateMachine(); err == nil {
			t.Error("Should detect inconsistency: halfOpenRequests > 0 but state is not HalfOpen")
		}
	})

	t.Run("CountConsistency", func(t *testing.T) {
		cb := New(Settings{
			Name: "test-count-consistency",
		})

		// Execute some requests
		for i := 0; i < 10; i++ {
			if i%3 == 0 {
				cb.Execute(failFunc)
			} else {
				cb.Execute(successFunc)
			}
		}

		if err := cb.validateStateMachine(); err != nil {
			t.Errorf("Counts should be consistent after normal operations: %v", err)
		}
	})

	t.Run("TimestampMonotonicity", func(t *testing.T) {
		cb := New(Settings{
			Name:    "test-timestamp-monotonicity",
			Timeout: 10 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures > 0
			},
		})

		// Trip circuit
		cb.Execute(failFunc)

		// Manually set stateChangedAt to be earlier than openedAt (simulating bug)
		cb.stateChangedAt.Store(cb.openedAt.Load() - 1)

		if err := cb.validateStateMachine(); err == nil {
			t.Error("Should detect timestamp monotonicity violation")
		}
	})
}
