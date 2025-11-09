package autobreaker

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test helper: successful operation
func successFunc() (interface{}, error) {
	return "success", nil
}

// Test helper: failing operation
func failFunc() (interface{}, error) {
	return nil, errors.New("operation failed")
}

// Test helper: panicking operation
func panicFunc() (interface{}, error) {
	panic("test panic")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		wantName string
		wantState State
	}{
		{
			name:     "default settings",
			settings: Settings{Name: "test"},
			wantName: "test",
			wantState: StateClosed,
		},
		{
			name: "custom settings",
			settings: Settings{
				Name:        "custom",
				MaxRequests: 10,
				Timeout:     30 * time.Second,
			},
			wantName: "custom",
			wantState: StateClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.settings)

			if cb.Name() != tt.wantName {
				t.Errorf("Name() = %v, want %v", cb.Name(), tt.wantName)
			}

			if cb.State() != tt.wantState {
				t.Errorf("State() = %v, want %v", cb.State(), tt.wantState)
			}
		})
	}
}

func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		settings    Settings
		shouldPanic bool
		panicMsg    string
	}{
		{
			name: "valid adaptive settings",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.05,
				MinimumObservations:  20,
			},
			shouldPanic: false,
		},
		{
			name: "adaptive with zero threshold (uses default)",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0, // Will default to 0.05
			},
			shouldPanic: false,
		},
		{
			name: "failure rate threshold too low",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.0,
			},
			shouldPanic: false, // 0 is OK, triggers default
		},
		{
			name: "failure rate threshold negative",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: -0.1,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1)",
		},
		{
			name: "failure rate threshold equals 1",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 1.0,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1)",
		},
		{
			name: "failure rate threshold above 1",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 1.5,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1)",
		},
		{
			name: "negative interval",
			settings: Settings{
				Name:     "test",
				Interval: -1 * time.Second,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: Interval cannot be negative",
		},
		{
			name: "zero interval (valid)",
			settings: Settings{
				Name:     "test",
				Interval: 0,
			},
			shouldPanic: false,
		},
		{
			name: "non-adaptive with invalid threshold (ignored)",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    false,
				FailureRateThreshold: 5.0, // Invalid but ignored since adaptive is false
			},
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic {
					if r == nil {
						t.Errorf("Expected panic with message containing %q, but no panic occurred", tt.panicMsg)
					} else {
						panicStr := fmt.Sprint(r)
						if panicStr != tt.panicMsg {
							t.Errorf("Expected panic message %q, got %q", tt.panicMsg, panicStr)
						}
					}
				} else {
					if r != nil {
						t.Errorf("Expected no panic, but got: %v", r)
					}
				}
			}()

			_ = New(tt.settings)
		})
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultReadyToTrip(t *testing.T) {
	tests := []struct {
		name   string
		counts Counts
		want   bool
	}{
		{
			name:   "no failures",
			counts: Counts{ConsecutiveFailures: 0},
			want:   false,
		},
		{
			name:   "5 consecutive failures (not yet tripped)",
			counts: Counts{ConsecutiveFailures: 5},
			want:   false,
		},
		{
			name:   "6 consecutive failures (should trip)",
			counts: Counts{ConsecutiveFailures: 6},
			want:   true,
		},
		{
			name:   "10 consecutive failures (should trip)",
			counts: Counts{ConsecutiveFailures: 10},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultReadyToTrip(tt.counts); got != tt.want {
				t.Errorf("defaultReadyToTrip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdaptiveReadyToTrip(t *testing.T) {
	cb := New(Settings{
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10%
		MinimumObservations:  10,
	})

	tests := []struct {
		name   string
		counts Counts
		want   bool
	}{
		{
			name: "not enough observations",
			counts: Counts{
				Requests:       5,
				TotalFailures:  3,
			},
			want: false, // Below minimum observations
		},
		{
			name: "below threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  5,
			},
			want: false, // 5% failure rate < 10% threshold
		},
		{
			name: "at threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  10,
			},
			want: false, // 10% failure rate == 10% threshold (not >)
		},
		{
			name: "above threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  11,
			},
			want: true, // 11% failure rate > 10% threshold
		},
		{
			name: "high failure rate",
			counts: Counts{
				Requests:       50,
				TotalFailures:  25,
			},
			want: true, // 50% failure rate > 10% threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cb.defaultAdaptiveReadyToTrip(tt.counts); got != tt.want {
				t.Errorf("defaultAdaptiveReadyToTrip() = %v, want %v for counts %+v", got, tt.want, tt.counts)
			}
		})
	}
}

func TestDefaultIsSuccessful(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error is success",
			err:  nil,
			want: true,
		},
		{
			name: "non-nil error is failure",
			err:  errors.New("test error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultIsSuccessful(tt.err); got != tt.want {
				t.Errorf("defaultIsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCircuitBreakerDefaults(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Test default max requests
	if cb.maxRequests != 1 {
		t.Errorf("default maxRequests = %v, want 1", cb.maxRequests)
	}

	// Test default timeout
	if cb.timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", cb.timeout)
	}

	// Test default state
	if cb.State() != StateClosed {
		t.Errorf("default state = %v, want Closed", cb.State())
	}

	// Test default counts
	counts := cb.Counts()
	if counts.Requests != 0 || counts.TotalFailures != 0 || counts.TotalSuccesses != 0 {
		t.Errorf("default counts = %+v, want all zeros", counts)
	}
}

func TestAdaptiveThresholdDefaults(t *testing.T) {
	cb := New(Settings{
		Name:              "test",
		AdaptiveThreshold: true,
	})

	// Test default failure rate threshold
	if cb.failureRateThreshold != 0.05 {
		t.Errorf("default failureRateThreshold = %v, want 0.05", cb.failureRateThreshold)
	}

	// Test default minimum observations
	if cb.minimumObservations != 20 {
		t.Errorf("default minimumObservations = %v, want 20", cb.minimumObservations)
	}
}

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

func TestAdaptiveReadyToTripTransition(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10%
		MinimumObservations:  20,   // Increased to 20 for clearer test
	})

	// 5 successes, 5 failures = 10 requests (should not trip - below minimum observations)
	for i := 0; i < 5; i++ {
		cb.Execute(successFunc)
		cb.Execute(failFunc)
	}

	if cb.State() != StateClosed {
		t.Errorf("At 10 requests (50%% failure): state = %v, want Closed (below minimum observations)", cb.State())
	}

	// Add more requests to reach minimum observations
	// Now at 10 success, 10 failures = 20 requests (50% >> 10% threshold - should trip)
	for i := 0; i < 5; i++ {
		cb.Execute(successFunc)
		cb.Execute(failFunc)
	}

	if cb.State() != StateOpen {
		t.Errorf("At 20 requests (50%% failure): state = %v, want Open (above threshold)", cb.State())
	}
}

func TestStateTransitionOpenToHalfOpen(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 100 * time.Millisecond,
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

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request triggers transition to HalfOpen, and success closes it
	result, err := cb.Execute(successFunc)

	// Successful probe completes recovery (HalfOpen → Closed)
	if cb.State() != StateClosed {
		t.Errorf("After successful probe: state = %v, want Closed (full recovery)", cb.State())
	}

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

	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
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

	// Wait for timeout and trigger transition (Open → HalfOpen → Closed)
	time.Sleep(100 * time.Millisecond)
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

func TestConcurrentExecute(t *testing.T) {
	cb := New(Settings{
		Name: "concurrent-test",
		// Never trip during this test
		ReadyToTrip: func(counts Counts) bool {
			return false
		},
	})

	const goroutines = 100
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				// Mix successes and failures
				if (id+j)%3 == 0 {
					cb.Execute(failFunc)
				} else {
					cb.Execute(successFunc)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify counts are consistent
	counts := cb.Counts()
	expectedRequests := uint32(goroutines * requestsPerGoroutine)

	if counts.Requests != expectedRequests {
		t.Errorf("Concurrent requests: got %d, want %d", counts.Requests, expectedRequests)
	}

	// Total should equal sum of successes and failures
	total := counts.TotalSuccesses + counts.TotalFailures
	if total != expectedRequests {
		t.Errorf("Sum of successes+failures = %d, want %d", total, expectedRequests)
	}
}

func TestConcurrentStateTransitions(t *testing.T) {
	cb := New(Settings{
		Name:    "concurrent-transitions",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrently trigger failures to trip circuit
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.Execute(failFunc)
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Circuit should be open after many concurrent failures
	if cb.State() != StateOpen {
		t.Errorf("After concurrent failures: state = %v, want Open", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Concurrent recovery attempts
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cb.Execute(successFunc)
		}()
	}

	wg.Wait()

	// Should have recovered to closed
	if cb.State() != StateClosed {
		t.Errorf("After concurrent recovery: state = %v, want Closed", cb.State())
	}
}

func TestConcurrentHalfOpenLimiting(t *testing.T) {
	cb := New(Settings{
		Name:        "concurrent-halfopen",
		MaxRequests: 3,
		Timeout:     50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Launch many concurrent requests - only MaxRequests should execute
	const goroutines = 20
	results := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Use slow function to ensure concurrency
	slowSuccess := func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return "slow-success", nil
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := cb.Execute(slowSuccess)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Count how many were rejected
	rejectedCount := 0
	for err := range results {
		if err == ErrTooManyRequests {
			rejectedCount++
		}
	}

	// Most should be rejected (only MaxRequests allowed)
	if rejectedCount < goroutines-int(cb.maxRequests)-2 {
		t.Errorf("Too few rejections: got %d, want at least %d", rejectedCount, goroutines-int(cb.maxRequests)-2)
	}
}

func TestConcurrentCountClearing(t *testing.T) {
	cb := New(Settings{
		Name:     "concurrent-clearing",
		Interval: 100 * time.Millisecond,
	})

	const goroutines = 50
	var wg sync.WaitGroup

	// Phase 1: Concurrent requests
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.Execute(successFunc)
			}
		}()
	}
	wg.Wait()

	initialCounts := cb.Counts()
	if initialCounts.Requests == 0 {
		t.Fatal("Expected some requests to be counted")
	}

	// Wait for interval
	time.Sleep(150 * time.Millisecond)

	// Phase 2: More concurrent requests (should trigger clearing)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cb.Execute(successFunc)
		}()
	}
	wg.Wait()

	// Counts should be reset (only recent requests counted)
	finalCounts := cb.Counts()
	if finalCounts.Requests > uint32(goroutines*2) {
		t.Errorf("Counts not cleared properly: got %d requests", finalCounts.Requests)
	}
}

func TestRaceConditions(t *testing.T) {
	// This test is specifically designed to catch races with -race flag
	cb := New(Settings{
		Name:    "race-test",
		Timeout: 10 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	const goroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Mix of operations
				switch (id + j) % 5 {
				case 0:
					cb.Execute(successFunc)
				case 1:
					cb.Execute(failFunc)
				case 2:
					_ = cb.State()
				case 3:
					_ = cb.Counts()
				case 4:
					_ = cb.Name()
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without race detector errors, we're good
}

func TestExecuteBasic(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Test successful execution
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if result != "success" {
		t.Errorf("Execute() result = %v, want 'success'", result)
	}

	// Verify counts
	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Requests = %v, want 1", counts.Requests)
	}
	if counts.TotalSuccesses != 1 {
		t.Errorf("TotalSuccesses = %v, want 1", counts.TotalSuccesses)
	}
	if counts.ConsecutiveSuccesses != 1 {
		t.Errorf("ConsecutiveSuccesses = %v, want 1", counts.ConsecutiveSuccesses)
	}
}

func TestExecuteCountTracking(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Execute 3 successes
	for i := 0; i < 3; i++ {
		cb.Execute(successFunc)
	}

	counts := cb.Counts()
	if counts.Requests != 3 {
		t.Errorf("After 3 successes: Requests = %v, want 3", counts.Requests)
	}
	if counts.TotalSuccesses != 3 {
		t.Errorf("After 3 successes: TotalSuccesses = %v, want 3", counts.TotalSuccesses)
	}
	if counts.ConsecutiveSuccesses != 3 {
		t.Errorf("After 3 successes: ConsecutiveSuccesses = %v, want 3", counts.ConsecutiveSuccesses)
	}

	// Execute 2 failures
	for i := 0; i < 2; i++ {
		cb.Execute(failFunc)
	}

	counts = cb.Counts()
	if counts.Requests != 5 {
		t.Errorf("After 2 failures: Requests = %v, want 5", counts.Requests)
	}
	if counts.TotalFailures != 2 {
		t.Errorf("After 2 failures: TotalFailures = %v, want 2", counts.TotalFailures)
	}
	if counts.ConsecutiveFailures != 2 {
		t.Errorf("After 2 failures: ConsecutiveFailures = %v, want 2", counts.ConsecutiveFailures)
	}
	if counts.ConsecutiveSuccesses != 0 {
		t.Errorf("After 2 failures: ConsecutiveSuccesses = %v, want 0 (streak broken)", counts.ConsecutiveSuccesses)
	}

	// Execute 1 success (breaks failure streak)
	cb.Execute(successFunc)

	counts = cb.Counts()
	if counts.ConsecutiveFailures != 0 {
		t.Errorf("After success: ConsecutiveFailures = %v, want 0 (streak broken)", counts.ConsecutiveFailures)
	}
	if counts.ConsecutiveSuccesses != 1 {
		t.Errorf("After success: ConsecutiveSuccesses = %v, want 1", counts.ConsecutiveSuccesses)
	}
}

func TestExecuteOpenState(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Manually set state to Open
	cb.state.Store(int32(StateOpen))

	// Attempt execution
	result, err := cb.Execute(successFunc)

	if err != ErrOpenState {
		t.Errorf("Execute() in open state: error = %v, want ErrOpenState", err)
	}
	if result != nil {
		t.Errorf("Execute() in open state: result = %v, want nil", result)
	}

	// Verify counts not incremented (request was rejected)
	counts := cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("Open state rejection: Requests = %v, want 0", counts.Requests)
	}
}

func TestExecuteHalfOpenMaxRequests(t *testing.T) {
	cb := New(Settings{
		Name:        "test",
		MaxRequests: 2,
	})

	// Set state to HalfOpen
	cb.state.Store(int32(StateHalfOpen))

	// Use a slow function to keep requests in-flight
	slowFunc := func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return "slow", nil
	}

	// Start first two requests concurrently (should succeed)
	type result struct {
		val interface{}
		err error
	}
	results := make(chan result, 3)

	// Request 1
	go func() {
		val, err := cb.Execute(slowFunc)
		results <- result{val, err}
	}()

	// Request 2
	go func() {
		val, err := cb.Execute(slowFunc)
		results <- result{val, err}
	}()

	// Give them time to start
	time.Sleep(10 * time.Millisecond)

	// Request 3 (should be rejected - exceeds MaxRequests)
	go func() {
		val, err := cb.Execute(slowFunc)
		results <- result{val, err}
	}()

	// Collect results
	var errors []error
	for i := 0; i < 3; i++ {
		r := <-results
		if r.err != nil {
			errors = append(errors, r.err)
		}
	}

	// Exactly one should be TooManyRequests error
	tooManyCount := 0
	for _, err := range errors {
		if err == ErrTooManyRequests {
			tooManyCount++
		}
	}

	if tooManyCount != 1 {
		t.Errorf("Expected exactly 1 TooManyRequests error, got %d. Errors: %v", tooManyCount, errors)
	}
}

func TestIntervalBasedCountClearing(t *testing.T) {
	cb := New(Settings{
		Name:     "test",
		Interval: 100 * time.Millisecond,
	})

	// Execute some requests
	cb.Execute(successFunc)
	cb.Execute(failFunc)

	counts := cb.Counts()
	if counts.Requests != 2 {
		t.Errorf("Before interval: Requests = %v, want 2", counts.Requests)
	}

	// Wait for interval to elapse
	time.Sleep(150 * time.Millisecond)

	// Execute another request (should trigger count clearing)
	cb.Execute(successFunc)

	counts = cb.Counts()
	// After clearing, only the new request should be counted
	if counts.Requests != 1 {
		t.Errorf("After interval: Requests = %v, want 1 (counts should be cleared)", counts.Requests)
	}
	if counts.TotalSuccesses != 1 {
		t.Errorf("After interval: TotalSuccesses = %v, want 1", counts.TotalSuccesses)
	}
}

// Test adaptive thresholds work across different traffic levels (core value proposition)
func TestAdaptiveVsStaticLowTraffic(t *testing.T) {
	// Low traffic scenario: 100 requests total
	const totalRequests = 100
	const failureRate = 0.06 // 6% failures

	// Adaptive breaker: should trip at 5% failure rate
	adaptive := New(Settings{
		Name:                 "adaptive-low-traffic",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // 5%
		MinimumObservations:  20,
	})

	// Static breaker: needs 6 consecutive failures
	static := New(Settings{
		Name: "static-low-traffic",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	// Simulate requests with 6% failure rate
	adaptiveTripped := false
	staticTripped := false

	for i := 0; i < totalRequests; i++ {
		// Create request that fails 6% of the time
		var req func() (interface{}, error)
		if i%17 == 0 { // ~6% failure rate
			req = failFunc
		} else {
			req = successFunc
		}

		// Execute on adaptive breaker
		if !adaptiveTripped {
			_, err := adaptive.Execute(req)
			if err == ErrOpenState {
				adaptiveTripped = true
			}
		}

		// Execute on static breaker
		if !staticTripped {
			_, err := static.Execute(req)
			if err == ErrOpenState {
				staticTripped = true
			}
		}
	}

	// Adaptive should have tripped (6% > 5% threshold)
	if !adaptiveTripped {
		t.Error("Adaptive breaker should have tripped at 6% failure rate in low traffic")
	}

	// Static might not have tripped (depends on failure distribution)
	// This demonstrates the problem with absolute count thresholds
	t.Logf("Low traffic (100 req): Adaptive tripped=%v, Static tripped=%v", adaptiveTripped, staticTripped)
}

func TestAdaptiveVsStaticHighTraffic(t *testing.T) {
	// High traffic scenario: 10,000 requests total
	const totalRequests = 10000
	const failureRate = 0.06 // 6% failures

	// Adaptive breaker: should trip at 5% failure rate
	adaptive := New(Settings{
		Name:                 "adaptive-high-traffic",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // 5%
		MinimumObservations:  20,
	})

	// Static breaker: needs 6 consecutive failures
	static := New(Settings{
		Name: "static-high-traffic",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	// Simulate requests with 6% failure rate
	adaptiveTripped := false
	staticTripped := false

	for i := 0; i < totalRequests; i++ {
		// Create request that fails 6% of the time
		var req func() (interface{}, error)
		if i%17 == 0 { // ~6% failure rate
			req = failFunc
		} else {
			req = successFunc
		}

		// Execute on adaptive breaker
		if !adaptiveTripped {
			_, err := adaptive.Execute(req)
			if err == ErrOpenState {
				adaptiveTripped = true
			}
		}

		// Execute on static breaker
		if !staticTripped {
			_, err := static.Execute(req)
			if err == ErrOpenState {
				staticTripped = true
			}
		}
	}

	// Adaptive should have tripped (6% > 5% threshold)
	if !adaptiveTripped {
		t.Error("Adaptive breaker should have tripped at 6% failure rate in high traffic")
	}

	// Static might or might not trip depending on failure distribution
	// (needs 6 consecutive failures, but our failures are spread out)
	// This demonstrates adaptive is more reliable across traffic patterns
	t.Logf("High traffic (10000 req): Adaptive tripped=%v, Static tripped=%v", adaptiveTripped, staticTripped)
}

func TestAdaptiveSameConfigDifferentTraffic(t *testing.T) {
	// Test that same adaptive config works across different traffic levels
	configs := []struct {
		name          string
		totalRequests int
		description   string
	}{
		{"low-traffic", 50, "dev environment"},
		{"medium-traffic", 500, "staging environment"},
		{"high-traffic", 5000, "production environment"},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			// Same configuration for all traffic levels
			cb := New(Settings{
				Name:                 tc.name,
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.10, // 10%
				MinimumObservations:  20,
			})

			tripped := false
			requestsBeforeTrip := 0

			// Simulate requests with 12% failure rate (above 10% threshold)
			for i := 0; i < tc.totalRequests; i++ {
				var req func() (interface{}, error)
				if i%8 == 0 { // ~12.5% failure rate
					req = failFunc
				} else {
					req = successFunc
				}

				if !tripped {
					_, err := cb.Execute(req)
					if err == ErrOpenState {
						tripped = true
						requestsBeforeTrip = i
					}
				}
			}

			// Should trip in all traffic levels (12% > 10%)
			if !tripped {
				t.Errorf("%s: Circuit should have tripped at 12%% failure rate", tc.description)
			}

			// Should trip after minimum observations
			if tripped && requestsBeforeTrip < 20 {
				t.Errorf("%s: Circuit tripped too early (%d requests, expected >= 20)",
					tc.description, requestsBeforeTrip)
			}

			t.Logf("%s: Tripped after %d requests (expected >= 20)", tc.description, requestsBeforeTrip)
		})
	}
}

func TestTrafficSpike(t *testing.T) {
	// Test behavior during traffic spike: low → high → low
	cb := New(Settings{
		Name:                 "traffic-spike",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
		Interval:             100 * time.Millisecond,
	})

	// Phase 1: Low traffic (10 req/s equivalent)
	for i := 0; i < 10; i++ {
		cb.Execute(successFunc)
	}

	counts := cb.Counts()
	if counts.Requests != 10 {
		t.Errorf("Phase 1: Requests = %v, want 10", counts.Requests)
	}

	// Phase 2: Traffic spike (1000 req/s equivalent)
	for i := 0; i < 1000; i++ {
		// 3% failure rate (below threshold)
		if i%33 == 0 {
			cb.Execute(failFunc)
		} else {
			cb.Execute(successFunc)
		}
	}

	counts = cb.Counts()
	if counts.Requests != 1010 {
		t.Errorf("Phase 2: Requests = %v, want 1010", counts.Requests)
	}

	// Should NOT have tripped (3% < 5% threshold)
	if cb.State() != StateClosed {
		t.Errorf("Should remain closed with 3%% failure rate, got state %v", cb.State())
	}

	// Wait for interval to clear counts
	time.Sleep(150 * time.Millisecond)
	cb.Execute(successFunc) // Trigger count clearing

	// Phase 3: Back to low traffic
	for i := 0; i < 10; i++ {
		cb.Execute(successFunc)
	}

	counts = cb.Counts()
	// Should have cleared after interval
	if counts.Requests > 15 { // Allow some buffer for clearing timing
		t.Errorf("Phase 3: Requests should be cleared, got %v", counts.Requests)
	}
}

func TestGradualTrafficIncrease(t *testing.T) {
	// Test behavior during gradual traffic increase
	cb := New(Settings{
		Name:                 "gradual-increase",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
	})

	// Gradually increase traffic while maintaining 3% failure rate
	trafficLevels := []int{10, 50, 100, 500, 1000}

	totalExecuted := 0
	for _, level := range trafficLevels {
		for i := 0; i < level; i++ {
			// 3% failure rate (below 5% threshold)
			// Use totalExecuted to avoid clustering failures
			if totalExecuted%34 == 0 && totalExecuted > 0 { // ~3% failure rate, skip first request
				cb.Execute(failFunc)
			} else {
				cb.Execute(successFunc)
			}
			totalExecuted++
		}
	}

	// Should NOT have tripped at any level (3% < 5%)
	if cb.State() != StateClosed {
		t.Errorf("Should remain closed with 3%% failure rate during gradual increase, got state %v", cb.State())
	}

	totalRequests := 0
	for _, level := range trafficLevels {
		totalRequests += level
	}

	counts := cb.Counts()
	if counts.Requests != uint32(totalRequests) {
		t.Errorf("Total requests = %v, want %v", counts.Requests, totalRequests)
	}

	t.Logf("Handled gradual increase from 10 to 1000 req without tripping")
}

func TestLowTrafficBehavior(t *testing.T) {
	// Test that minimum observations prevents premature tripping in very low traffic
	cb := New(Settings{
		Name:                 "very-low-traffic",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // 5%
		MinimumObservations:  20,
	})

	// Send only 10 requests with 100% failure rate
	for i := 0; i < 10; i++ {
		cb.Execute(failFunc)
	}

	// Should NOT trip (below minimum observations)
	if cb.State() != StateClosed {
		t.Errorf("Should not trip with <MinimumObservations, got state %v", cb.State())
	}

	counts := cb.Counts()
	if counts.Requests != 10 {
		t.Errorf("Requests = %v, want 10", counts.Requests)
	}
	if counts.TotalFailures != 10 {
		t.Errorf("TotalFailures = %v, want 10", counts.TotalFailures)
	}

	// Send 10 more successful requests (now at MinimumObservations)
	for i := 0; i < 10; i++ {
		cb.Execute(successFunc)
	}

	// Now at 20 requests: 10 failures / 20 total = 50% failure rate
	// Should trip (50% >> 5%)
	cb.Execute(failFunc) // This should trigger the trip

	if cb.State() != StateOpen {
		t.Errorf("Should trip after reaching MinimumObservations with high failure rate, got state %v", cb.State())
	}
}
