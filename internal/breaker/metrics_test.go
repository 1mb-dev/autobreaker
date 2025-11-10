package breaker

import (
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Get initial metrics
	metrics := cb.Metrics()

	// Verify initial state
	if metrics.State != StateClosed {
		t.Errorf("Initial state = %v, want Closed", metrics.State)
	}

	if metrics.Counts.Requests != 0 {
		t.Errorf("Initial requests = %v, want 0", metrics.Counts.Requests)
	}

	if metrics.FailureRate != 0 {
		t.Errorf("Initial failure rate = %v, want 0", metrics.FailureRate)
	}

	if metrics.SuccessRate != 0 {
		t.Errorf("Initial success rate = %v, want 0", metrics.SuccessRate)
	}

	// StateChangedAt should be set (initialized in New())
	if metrics.StateChangedAt.IsZero() {
		t.Error("StateChangedAt should be set on initialization")
	}

	// CountsLastClearedAt should be set
	if metrics.CountsLastClearedAt.IsZero() {
		t.Error("CountsLastClearedAt should be set on initialization")
	}
}

func TestMetricsFailureRate(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return false // Never trip for this test
		},
	})

	// Execute 10 requests: 6 successes, 4 failures
	// i=0,3,6,9 → fail (4 failures)
	// i=1,2,4,5,7,8 → success (6 successes)
	for i := 0; i < 10; i++ {
		var req func() (interface{}, error)
		if i%3 == 0 {
			req = failFunc
		} else {
			req = successFunc
		}
		cb.Execute(req)
	}

	metrics := cb.Metrics()

	// Verify counts
	if metrics.Counts.Requests != 10 {
		t.Errorf("Requests = %v, want 10", metrics.Counts.Requests)
	}

	if metrics.Counts.TotalSuccesses != 6 {
		t.Errorf("TotalSuccesses = %v, want 6", metrics.Counts.TotalSuccesses)
	}

	if metrics.Counts.TotalFailures != 4 {
		t.Errorf("TotalFailures = %v, want 4", metrics.Counts.TotalFailures)
	}

	// Verify failure rate (4/10 = 0.4)
	expectedFailureRate := 0.4
	if metrics.FailureRate != expectedFailureRate {
		t.Errorf("FailureRate = %v, want %v", metrics.FailureRate, expectedFailureRate)
	}

	// Verify success rate (6/10 = 0.6)
	expectedSuccessRate := 0.6
	if metrics.SuccessRate != expectedSuccessRate {
		t.Errorf("SuccessRate = %v, want %v", metrics.SuccessRate, expectedSuccessRate)
	}
}

func TestMetricsStateChangeTimestamp(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	// Get initial timestamp
	initialMetrics := cb.Metrics()
	initialStateChangedAt := initialMetrics.StateChangedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Trip the circuit (Closed → Open)
	cb.Execute(failFunc)
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	// Get metrics after state change
	openMetrics := cb.Metrics()

	if openMetrics.State != StateOpen {
		t.Fatalf("State = %v, want Open", openMetrics.State)
	}

	// StateChangedAt should have been updated
	if openMetrics.StateChangedAt.Equal(initialStateChangedAt) {
		t.Error("StateChangedAt should be updated after state transition")
	}

	if !openMetrics.StateChangedAt.After(initialStateChangedAt) {
		t.Error("StateChangedAt should be after initial timestamp")
	}
}

func TestMetricsCountsClearedTimestamp(t *testing.T) {
	cb := New(Settings{
		Name:     "test",
		Interval: 100 * time.Millisecond,
	})

	// Get initial timestamp
	initialMetrics := cb.Metrics()
	initialClearedAt := initialMetrics.CountsLastClearedAt

	// Make some requests
	cb.Execute(successFunc)
	cb.Execute(successFunc)

	// Wait for interval to pass
	time.Sleep(150 * time.Millisecond)

	// Trigger interval-based clearing
	cb.Execute(successFunc)

	// Get metrics
	metrics := cb.Metrics()

	// CountsLastClearedAt should have been updated
	if metrics.CountsLastClearedAt.Equal(initialClearedAt) {
		t.Error("CountsLastClearedAt should be updated after interval clearing")
	}

	if !metrics.CountsLastClearedAt.After(initialClearedAt) {
		t.Error("CountsLastClearedAt should be after initial timestamp")
	}

	// Counts should be reset
	if metrics.Counts.Requests != 1 {
		t.Errorf("After clearing: Requests = %v, want 1 (just the current request)", metrics.Counts.Requests)
	}
}

func TestMetricsAfterStateTransitions(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// Trip circuit (Closed → Open)
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	openMetrics := cb.Metrics()
	if openMetrics.State != StateOpen {
		t.Fatalf("State = %v, want Open", openMetrics.State)
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Trigger transition to HalfOpen and then Closed
	cb.Execute(successFunc)

	closedMetrics := cb.Metrics()
	if closedMetrics.State != StateClosed {
		t.Fatalf("State = %v, want Closed", closedMetrics.State)
	}

	// StateChangedAt should be more recent after recovery
	if !closedMetrics.StateChangedAt.After(openMetrics.StateChangedAt) {
		t.Error("StateChangedAt should be updated after recovery")
	}
}

func TestMetricsZeroDivision(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Get metrics without any requests
	metrics := cb.Metrics()

	// Should not panic, should return 0
	if metrics.FailureRate != 0 {
		t.Errorf("FailureRate with no requests = %v, want 0", metrics.FailureRate)
	}

	if metrics.SuccessRate != 0 {
		t.Errorf("SuccessRate with no requests = %v, want 0", metrics.SuccessRate)
	}
}

func TestMetricsConcurrent(t *testing.T) {
	cb := New(Settings{
		Name: "test",
	})

	// Concurrently call Metrics() while executing requests
	done := make(chan bool)

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = cb.Metrics()
			}
			done <- true
		}()
	}

	// Writer goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cb.Execute(successFunc)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or race
	finalMetrics := cb.Metrics()
	if finalMetrics.Counts.Requests == 0 {
		t.Error("Expected some requests to have been recorded")
	}
}
