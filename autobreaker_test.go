package autobreaker

import (
	"errors"
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

// Placeholder for state transition tests (Phase 1 implementation)
func TestStateTransitions(t *testing.T) {
	t.Skip("Phase 1: Implement state transition tests")
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
