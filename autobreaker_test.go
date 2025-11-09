package autobreaker

import (
	"errors"
	"fmt"
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

// Placeholder for concurrency tests (Phase 1 implementation)
func TestConcurrency(t *testing.T) {
	t.Skip("Phase 1: Implement concurrency tests with race detector")
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
