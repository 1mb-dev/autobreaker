package breaker

import (
	"fmt"
	"testing"
	"time"
)

func TestStateTransitionClosedToOpen(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	// Verify initial state
	if cb.State() != StateClosed {
		t.Errorf("Initial state = %v, want Closed", cb.State())
	}

	// First two failures should not trip
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	if cb.State() != StateClosed {
		t.Errorf("After 2 failures: state = %v, want Closed", cb.State())
	}

	// Third failure should trip circuit
	cb.Execute(failFunc)

	if cb.State() != StateOpen {
		t.Errorf("After 3 failures: state = %v, want Open", cb.State())
	}

	// Verify counts were cleared after transition
	counts := cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("After transition: Requests = %v, want 0 (cleared)", counts.Requests)
	}
}

func TestStateTransitionWithCallback(t *testing.T) {
	var callbackFrom, callbackTo State
	var callbackName string
	var callbackCalled bool

	cb := New(Settings{
		Name: "test-callback",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
		OnStateChange: func(name string, from State, to State) {
			callbackCalled = true
			callbackName = name
			callbackFrom = from
			callbackTo = to
		},
	})

	// Trigger transition
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	if !callbackCalled {
		t.Error("OnStateChange callback was not called")
	}

	if callbackName != "test-callback" {
		t.Errorf("Callback name = %v, want 'test-callback'", callbackName)
	}

	if callbackFrom != StateClosed {
		t.Errorf("Callback from state = %v, want Closed", callbackFrom)
	}

	if callbackTo != StateOpen {
		t.Errorf("Callback to state = %v, want Open", callbackTo)
	}
}

func TestStateTransitionOpenToHalfOpen(t *testing.T) {
	timeout := 100 * time.Millisecond
	cb := New(Settings{
		Name:    "test",
		Timeout: timeout,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// Trip the circuit to Open
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	if cb.State() != StateOpen {
		t.Fatalf("Circuit not open after failures, state = %v", cb.State())
	}

	// Try request before timeout (should be rejected)
	_, err := cb.Execute(successFunc)
	if err != ErrOpenState {
		t.Errorf("Request before timeout: error = %v, want ErrOpenState", err)
	}

	// Wait for timeout with tolerance (1.5x base timeout)
	time.Sleep(timeout + 50*time.Millisecond)

	// Next request triggers transition to HalfOpen, and success closes it
	result, err := cb.Execute(successFunc)

	// Successful probe completes recovery (HalfOpen → Closed)
	requireState(t, cb, StateClosed, 200*time.Millisecond)

	if err != nil {
		t.Errorf("Request after timeout: error = %v, want nil", err)
	}

	if result != "success" {
		t.Errorf("Request after timeout: result = %v, want 'success'", result)
	}
}

func TestStateTransitionOpenToHalfOpenWithCallback(t *testing.T) {
	var transitions []struct {
		from State
		to   State
	}

	timeout := 50 * time.Millisecond
	cb := New(Settings{
		Name:    "test",
		Timeout: timeout,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
		OnStateChange: func(name string, from State, to State) {
			transitions = append(transitions, struct {
				from State
				to   State
			}{from, to})
		},
	})

	// Trip circuit (Closed → Open)
	cb.Execute(failFunc)

	if len(transitions) != 1 {
		t.Fatalf("After tripping: transitions = %d, want 1", len(transitions))
	}

	// Wait for timeout with tolerance (2x base timeout)
	time.Sleep(timeout * 2)
	cb.Execute(successFunc)

	// Successful probe triggers two transitions
	if len(transitions) != 3 {
		t.Fatalf("After successful probe: transitions = %d, want 3", len(transitions))
	}

	// Verify transition sequence
	if transitions[0].from != StateClosed || transitions[0].to != StateOpen {
		t.Errorf("First transition: %v → %v, want Closed → Open", transitions[0].from, transitions[0].to)
	}

	if transitions[1].from != StateOpen || transitions[1].to != StateHalfOpen {
		t.Errorf("Second transition: %v → %v, want Open → HalfOpen", transitions[1].from, transitions[1].to)
	}

	if transitions[2].from != StateHalfOpen || transitions[2].to != StateClosed {
		t.Errorf("Third transition: %v → %v, want HalfOpen → Closed", transitions[2].from, transitions[2].to)
	}
}

func TestStateTransitionHalfOpenToClosed(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)

	if cb.State() != StateOpen {
		t.Fatalf("Circuit not open, state = %v", cb.State())
	}

	// Wait for timeout and transition to HalfOpen
	time.Sleep(100 * time.Millisecond)
	cb.Execute(successFunc) // This should succeed and close circuit

	if cb.State() != StateClosed {
		t.Errorf("After successful probe: state = %v, want Closed", cb.State())
	}

	// Verify normal operations work
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("After recovery: error = %v, want nil", err)
	}
	if result != "success" {
		t.Errorf("After recovery: result = %v, want 'success'", result)
	}
}

func TestStateTransitionHalfOpenToOpen(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)

	if cb.State() != StateOpen {
		t.Fatalf("Circuit not open, state = %v", cb.State())
	}

	// Wait for timeout and transition to HalfOpen
	time.Sleep(100 * time.Millisecond)
	cb.Execute(failFunc) // Failed probe - should go back to Open

	if cb.State() != StateOpen {
		t.Errorf("After failed probe: state = %v, want Open", cb.State())
	}

	// Verify circuit is rejecting again
	_, err := cb.Execute(successFunc)
	if err != ErrOpenState {
		t.Errorf("After re-opening: error = %v, want ErrOpenState", err)
	}
}

func TestFullRecoveryFlow(t *testing.T) {
	var transitions []string

	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
		OnStateChange: func(name string, from State, to State) {
			transitions = append(transitions, fmt.Sprintf("%v→%v", from, to))
		},
	})

	// Normal operations
	cb.Execute(successFunc)

	// Trip circuit (Closed → Open)
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Successful probe (Open → HalfOpen → Closed)
	cb.Execute(successFunc)

	// Verify full transition sequence
	expected := []string{"closed→open", "open→half-open", "half-open→closed"}
	if len(transitions) != len(expected) {
		t.Fatalf("Transitions = %v, want %v", transitions, expected)
	}

	for i, exp := range expected {
		if transitions[i] != exp {
			t.Errorf("Transition %d = %v, want %v", i, transitions[i], exp)
		}
	}
}
