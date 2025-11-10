package breaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestEmptyName validates circuit breaker works with empty name.
func TestEmptyName(t *testing.T) {
	cb := New(Settings{Name: ""})

	if cb.Name() != "" {
		t.Errorf("Expected empty name, got %q", cb.Name())
	}

	// Should work normally even with empty name
	result, err := cb.Execute(successFunc)
	if err != nil || result != "success" {
		t.Errorf("Circuit with empty name should work: result=%v, err=%v", result, err)
	}
}

// TestConcurrentOnStateChangeCallback validates callback thread-safety.
// This tests that OnStateChange can be safely called concurrently.
func TestConcurrentOnStateChangeCallback(t *testing.T) {
	var callbackCount atomic.Int32
	var mu sync.Mutex
	var transitions []struct {
		from State
		to   State
	}

	cb := New(Settings{
		Name:    "concurrent-callback",
		Timeout: 10 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(name string, from State, to State) {
			callbackCount.Add(1)

			// Simulate some work in callback
			time.Sleep(5 * time.Millisecond)

			// Thread-safe append
			mu.Lock()
			transitions = append(transitions, struct {
				from State
				to   State
			}{from, to})
			mu.Unlock()
		},
	})

	// Trigger rapid state transitions concurrently
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			// Trip circuit
			cb.Execute(failFunc)
			cb.Execute(failFunc)

			// Wait for recovery
			time.Sleep(20 * time.Millisecond)

			// Trigger recovery
			cb.Execute(successFunc)
		}()
	}

	wg.Wait()

	// Callback should have been called multiple times
	count := callbackCount.Load()
	if count == 0 {
		t.Error("OnStateChange callback was never called")
	}

	t.Logf("Callback invoked %d times from concurrent goroutines", count)
}

// TestMetricsZeroRequests validates metrics with zero requests.
func TestMetricsZeroRequests(t *testing.T) {
	cb := New(Settings{Name: "zero-requests"})

	metrics := cb.Metrics()

	if metrics.FailureRate != 0 {
		t.Errorf("FailureRate with 0 requests should be 0, got %v", metrics.FailureRate)
	}

	if metrics.SuccessRate != 0 {
		t.Errorf("SuccessRate with 0 requests should be 0, got %v", metrics.SuccessRate)
	}

	if metrics.Counts.Requests != 0 {
		t.Errorf("Request count should be 0, got %v", metrics.Counts.Requests)
	}
}

// TestDiagnosticsZeroRequests validates diagnostics with zero requests.
func TestDiagnosticsZeroRequests(t *testing.T) {
	cb := New(Settings{
		Name:              "zero-requests-diag",
		AdaptiveThreshold: true,
	})

	diag := cb.Diagnostics()

	if diag.WillTripNext {
		t.Error("WillTripNext should be false with 0 requests")
	}

	if diag.TimeUntilHalfOpen != 0 {
		t.Errorf("TimeUntilHalfOpen should be 0 when not open, got %v", diag.TimeUntilHalfOpen)
	}
}

// TestUpdateSettingsPreservesCallbacks validates immutable callback preservation.
func TestUpdateSettingsPreservesCallbacks(t *testing.T) {
	callbackCalled := false
	readyToTripCalled := false

	cb := New(Settings{
		Name: "preserve-callbacks",
		OnStateChange: func(name string, from, to State) {
			callbackCalled = true
		},
		ReadyToTrip: func(counts Counts) bool {
			readyToTripCalled = true
			return counts.ConsecutiveFailures > 1
		},
	})

	// Update settings (should not affect callbacks)
	err := cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Trigger callbacks
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	if !callbackCalled {
		t.Error("OnStateChange callback was not preserved after UpdateSettings")
	}

	if !readyToTripCalled {
		t.Error("ReadyToTrip callback was not preserved after UpdateSettings")
	}
}

// TestAdaptiveThresholdBoundary validates exact boundary conditions.
func TestAdaptiveThresholdBoundary(t *testing.T) {
	tests := []struct {
		name            string
		threshold       float64
		failures        uint32
		requests        uint32
		minObservations uint32
		shouldTrip      bool
	}{
		{
			name:            "exactly at threshold",
			threshold:       0.10,
			failures:        10,
			requests:        100,
			minObservations: 20,
			shouldTrip:      false, // 10% == 10%, not > 10%
		},
		{
			name:            "just above threshold",
			threshold:       0.10,
			failures:        11,
			requests:        100,
			minObservations: 20,
			shouldTrip:      true, // 11% > 10%
		},
		{
			name:            "below minimum observations",
			threshold:       0.10,
			failures:        50, // 50% failure rate
			requests:        10, // But only 10 requests
			minObservations: 20,
			shouldTrip:      false, // Below minObservations
		},
		{
			name:            "at minimum observations",
			threshold:       0.10,
			failures:        3,
			requests:        20, // Exactly minObservations
			minObservations: 20,
			shouldTrip:      true, // 15% > 10%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(Settings{
				Name:                 tt.name,
				AdaptiveThreshold:    true,
				FailureRateThreshold: tt.threshold,
				MinimumObservations:  tt.minObservations,
			})

			counts := Counts{
				Requests:      tt.requests,
				TotalFailures: tt.failures,
			}

			result := cb.defaultAdaptiveReadyToTrip(counts)
			if result != tt.shouldTrip {
				failureRate := float64(tt.failures) / float64(tt.requests)
				t.Errorf("Expected shouldTrip=%v for %.1f%% failure rate (%d/%d) with threshold %.1f%% and minObs=%d, got %v",
					tt.shouldTrip, failureRate*100, tt.failures, tt.requests,
					tt.threshold*100, tt.minObservations, result)
			}
		})
	}
}

// TestStateStringForInvalidStates validates State.String() for invalid states.
func TestStateStringForInvalidStates(t *testing.T) {
	invalidStates := []State{
		State(-1),
		State(3),
		State(999),
	}

	for _, state := range invalidStates {
		result := state.String()
		if result != "unknown" {
			t.Errorf("Invalid state %d should return 'unknown', got %q", state, result)
		}
	}
}
