package breaker

import (
	"testing"
	"time"
)
func TestCircuitBreakerDefaults(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Test default max requests
	if cb.getMaxRequests() != 1 {
		t.Errorf("default maxRequests = %v, want 1", cb.getMaxRequests())
	}

	// Test default timeout
	if cb.getTimeout() != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", cb.getTimeout())
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

