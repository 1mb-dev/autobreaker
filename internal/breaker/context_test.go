package breaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Test context cancellation before execution
func TestExecuteContext_CancelledBeforeExecution(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	executed := false
	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		executed = true
		return "should not execute", nil
	})

	// Should return context error without executing
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
	if executed {
		t.Error("Request should not have been executed")
	}

	// Should not count as a request
	counts := cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("Expected 0 requests, got %d", counts.Requests)
	}
}

// Test context deadline exceeded before execution
func TestExecuteContext_DeadlineExceededBeforeExecution(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Create already-expired context
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	executed := false
	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		executed = true
		return "should not execute", nil
	})

	// Should return deadline exceeded error without executing
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
	if executed {
		t.Error("Request should not have been executed")
	}

	// Should not count as a request
	counts := cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("Expected 0 requests, got %d", counts.Requests)
	}
}

// Test context cancellation during execution
func TestExecuteContext_CancelledDuringExecution(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	ctx, cancel := context.WithCancel(context.Background())

	executionStarted := make(chan struct{})
	executed := false

	// Start execution in background, cancel during execution
	go func() {
		<-executionStarted
		time.Sleep(10 * time.Millisecond) // Let execution start
		cancel()
	}()

	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		executed = true
		close(executionStarted)
		time.Sleep(50 * time.Millisecond) // Simulate work
		return "result", nil
	})

	// Should return context error
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
	if !executed {
		t.Error("Request should have started execution")
	}

	// Should count as a request but NOT as success or failure
	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Expected 1 request, got %d", counts.Requests)
	}
	if counts.TotalSuccesses != 0 {
		t.Errorf("Expected 0 successes, got %d", counts.TotalSuccesses)
	}
	if counts.TotalFailures != 0 {
		t.Errorf("Expected 0 failures, got %d", counts.TotalFailures)
	}

	// Circuit should still be closed (cancellation doesn't trip circuit)
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed, got %v", cb.State())
	}
}

// Test successful execution with valid context
func TestExecuteContext_SuccessWithValidContext(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Expected 1 request, got %d", counts.Requests)
	}
	if counts.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", counts.TotalSuccesses)
	}
	if counts.TotalFailures != 0 {
		t.Errorf("Expected 0 failures, got %d", counts.TotalFailures)
	}
}

// Test failure with valid context
func TestExecuteContext_FailureWithValidContext(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	testErr := errors.New("test error")
	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		return nil, testErr
	})

	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Expected 1 request, got %d", counts.Requests)
	}
	if counts.TotalSuccesses != 0 {
		t.Errorf("Expected 0 successes, got %d", counts.TotalSuccesses)
	}
	if counts.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", counts.TotalFailures)
	}
}

// Test ExecuteContext with circuit in open state
func TestExecuteContext_CircuitOpen(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
		Timeout: 1 * time.Second,
	})

	ctx := context.Background()

	// Trip the circuit
	_, _ = cb.ExecuteContext(ctx, func() (interface{}, error) {
		return nil, errors.New("error")
	})

	// Verify circuit is open
	if cb.State() != StateOpen {
		t.Fatalf("Expected StateOpen, got %v", cb.State())
	}

	// Try to execute with context - should fail fast
	executed := false
	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		executed = true
		return "should not execute", nil
	})

	if err != ErrOpenState {
		t.Errorf("Expected ErrOpenState, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
	if executed {
		t.Error("Request should not have been executed when circuit is open")
	}
}

// Test ExecuteContext with circuit in half-open state
func TestExecuteContext_CircuitHalfOpen(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
		Timeout:     10 * time.Millisecond,
		MaxRequests: 2,
	})

	ctx := context.Background()

	// Trip the circuit
	_, _ = cb.ExecuteContext(ctx, func() (interface{}, error) {
		return nil, errors.New("error")
	})

	// Wait for timeout to transition to half-open
	time.Sleep(15 * time.Millisecond)

	// First request should succeed (half-open allows MaxRequests)
	result1, err1 := cb.ExecuteContext(ctx, func() (interface{}, error) {
		return "success", nil
	})

	if err1 != nil {
		t.Errorf("First request should succeed, got error: %v", err1)
	}
	if result1 != "success" {
		t.Errorf("Expected 'success', got %v", result1)
	}

	// Circuit should transition back to closed after success
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after successful half-open request, got %v", cb.State())
	}
}

// Test concurrent ExecuteContext calls with different contexts
func TestExecuteContext_ConcurrentContexts(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	const numGoroutines = 10
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return id, nil
			})
			errors <- err
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-errors
		if err == nil {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected %d successes, got %d", numGoroutines, successCount)
	}

	counts := cb.Counts()
	if counts.Requests != uint32(numGoroutines) {
		t.Errorf("Expected %d requests, got %d", numGoroutines, counts.Requests)
	}
}

// Test that Execute() still works (no regression)
func TestExecuteContext_ExecuteStillWorks(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Test Execute() without context
	result, err := cb.Execute(func() (interface{}, error) {
		return "execute works", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "execute works" {
		t.Errorf("Expected 'execute works', got %v", result)
	}

	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Expected 1 request, got %d", counts.Requests)
	}
	if counts.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", counts.TotalSuccesses)
	}
}

// Test panic handling with context
func TestExecuteContext_PanicHandling(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	ctx := context.Background()

	// Test that panic is recovered and re-panicked
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be re-raised")
		}
	}()

	_, _ = cb.ExecuteContext(ctx, func() (interface{}, error) {
		panic("test panic")
	})
}

// Test context timeout during long execution
func TestExecuteContext_TimeoutDuringExecution(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	result, err := cb.ExecuteContext(ctx, func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond) // Longer than timeout
		return "should timeout", nil
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Should count as request but not as success/failure
	counts := cb.Counts()
	if counts.Requests != 1 {
		t.Errorf("Expected 1 request, got %d", counts.Requests)
	}
	if counts.TotalSuccesses != 0 {
		t.Errorf("Expected 0 successes, got %d", counts.TotalSuccesses)
	}
	if counts.TotalFailures != 0 {
		t.Errorf("Expected 0 failures (timeout shouldn't count), got %d", counts.TotalFailures)
	}
}

// Test multiple context cancellations don't trip circuit
func TestExecuteContext_MultipleContextCancellationsDontTripCircuit(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	})

	// Cancel context multiple times
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cb.ExecuteContext(ctx, func() (interface{}, error) {
			return "should not execute", nil
		})
	}

	// Circuit should still be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed (cancellations shouldn't trip), got %v", cb.State())
	}

	counts := cb.Counts()
	if counts.TotalFailures != 0 {
		t.Errorf("Expected 0 failures (cancellations before execution), got %d", counts.TotalFailures)
	}
}
