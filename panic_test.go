package autobreaker

import (
	"testing"
	"time"
)
func TestPanicRecovery(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Execute should recover from panic and re-panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be re-thrown")
		} else if r != "test panic" {
			t.Errorf("Panic value = %v, want 'test panic'", r)
		}
	}()

	cb.Execute(panicFunc)
	t.Error("Should not reach here - panic should have been thrown")
}

func TestPanicCountsAsFailure(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Catch the re-panic
	func() {
		defer func() {
			recover() // Swallow the panic
		}()
		cb.Execute(panicFunc)
	}()

	// Verify panic was counted as failure
	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("After panic: Requests = %v, want 1", counts.Requests)
	}
	if counts.TotalFailures != 1 {
		t.Errorf("After panic: TotalFailures = %v, want 1 (panic counts as failure)", counts.TotalFailures)
	}
	if counts.ConsecutiveFailures != 1 {
		t.Errorf("After panic: ConsecutiveFailures = %v, want 1", counts.ConsecutiveFailures)
	}
}

func TestPanicTripsCircuit(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// First panic
	func() {
		defer func() {
			recover() // Swallow the panic
		}()
		cb.Execute(panicFunc)
	}()

	if cb.State() != StateClosed {
		t.Errorf("After 1 panic: state = %v, want Closed", cb.State())
	}

	// Second panic should trip circuit
	func() {
		defer func() {
			recover() // Swallow the panic
		}()
		cb.Execute(panicFunc)
	}()

	if cb.State() != StateOpen {
		t.Errorf("After 2 panics: state = %v, want Open (circuit tripped)", cb.State())
	}
}

func TestPanicInHalfOpenReopensCircuit(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Panic during probe should reopen circuit
	func() {
		defer func() {
			recover() // Swallow the panic
		}()
		cb.Execute(panicFunc)
	}()

	if cb.State() != StateOpen {
		t.Errorf("After panic in half-open: state = %v, want Open (re-opened)", cb.State())
	}
}

