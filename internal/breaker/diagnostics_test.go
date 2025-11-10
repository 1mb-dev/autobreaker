package breaker

import (
	"testing"
	"time"
)

func TestDiagnostics(t *testing.T) {
	cb := New(Settings{
		Name:                 "test-diagnostics",
		MaxRequests:          5,
		Interval:             60 * time.Second,
		Timeout:              30 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10,
		MinimumObservations:  20,
	})

	diag := cb.Diagnostics()

	// Verify name
	if diag.Name != "test-diagnostics" {
		t.Errorf("Name = %v, want 'test-diagnostics'", diag.Name)
	}

	// Verify state
	if diag.State != StateClosed {
		t.Errorf("State = %v, want Closed", diag.State)
	}

	// Verify configuration
	if diag.MaxRequests != 5 {
		t.Errorf("MaxRequests = %v, want 5", diag.MaxRequests)
	}

	if diag.Interval != 60*time.Second {
		t.Errorf("Interval = %v, want 60s", diag.Interval)
	}

	if diag.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", diag.Timeout)
	}

	if !diag.AdaptiveEnabled {
		t.Error("AdaptiveEnabled = false, want true")
	}

	if diag.FailureRateThreshold != 0.10 {
		t.Errorf("FailureRateThreshold = %v, want 0.10", diag.FailureRateThreshold)
	}

	if diag.MinimumObservations != 20 {
		t.Errorf("MinimumObservations = %v, want 20", diag.MinimumObservations)
	}

	// Verify metrics included
	if diag.Metrics.State != StateClosed {
		t.Errorf("Metrics.State = %v, want Closed", diag.Metrics.State)
	}
}

func TestDiagnosticsWillTripNext(t *testing.T) {
	tests := []struct {
		name            string
		settings        Settings
		requests        []func() (interface{}, error)
		expectWillTrip  bool
	}{
		{
			name: "static - one more failure will trip",
			settings: Settings{
				Name: "test",
				ReadyToTrip: func(counts Counts) bool {
					return counts.ConsecutiveFailures > 2
				},
			},
			requests: []func() (interface{}, error){
				failFunc, // 1 failure
				failFunc, // 2 failures
			},
			expectWillTrip: true, // Next failure would be 3, which triggers > 2
		},
		{
			name: "static - won't trip yet",
			settings: Settings{
				Name: "test",
				ReadyToTrip: func(counts Counts) bool {
					return counts.ConsecutiveFailures > 5
				},
			},
			requests: []func() (interface{}, error){
				failFunc, // 1 failure
				failFunc, // 2 failures
			},
			expectWillTrip: false, // Next failure would be 3, needs > 5
		},
		{
			name: "adaptive - one more failure will trip",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.10, // 10%
				MinimumObservations:  10,
			},
			requests: func() []func() (interface{}, error) {
				// 9 successes, 2 failures = 11 total, 18% failure rate
				reqs := make([]func() (interface{}, error), 11)
				reqs[0] = failFunc
				reqs[1] = failFunc
				for i := 2; i < 11; i++ {
					reqs[i] = successFunc
				}
				return reqs
			}(),
			expectWillTrip: true, // Next failure: 3/12 = 25% > 10%
		},
		{
			name: "adaptive - won't trip yet",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.30, // 30%
				MinimumObservations:  10,
			},
			requests: func() []func() (interface{}, error) {
				// 9 successes, 2 failures = 11 total, 18% failure rate
				reqs := make([]func() (interface{}, error), 11)
				reqs[0] = failFunc
				reqs[1] = failFunc
				for i := 2; i < 11; i++ {
					reqs[i] = successFunc
				}
				return reqs
			}(),
			expectWillTrip: false, // Next failure: 3/12 = 25% < 30% threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.settings)

			// Execute requests
			for _, req := range tt.requests {
				cb.Execute(req)
			}

			// Get diagnostics
			diag := cb.Diagnostics()

			if diag.WillTripNext != tt.expectWillTrip {
				t.Errorf("WillTripNext = %v, want %v (after %d requests, state=%v)",
					diag.WillTripNext, tt.expectWillTrip, len(tt.requests), diag.State)
			}
		})
	}
}

func TestDiagnosticsWillTripNextAfterSuccess(t *testing.T) {
	cb := New(Settings{
		Name: "test",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	// 2 failures
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	diag1 := cb.Diagnostics()
	if !diag1.WillTripNext {
		t.Error("Should predict trip after 2 consecutive failures")
	}

	// 1 success (resets consecutive)
	cb.Execute(successFunc)

	diag2 := cb.Diagnostics()
	if diag2.WillTripNext {
		t.Error("Should not predict trip after success reset consecutive failures")
	}
}

func TestDiagnosticsTimeUntilHalfOpen(t *testing.T) {
	timeout := 100 * time.Millisecond

	cb := New(Settings{
		Name:    "test",
		Timeout: timeout,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// Initially closed, no TimeUntilHalfOpen
	diagClosed := cb.Diagnostics()
	if diagClosed.TimeUntilHalfOpen != 0 {
		t.Errorf("Closed state: TimeUntilHalfOpen = %v, want 0", diagClosed.TimeUntilHalfOpen)
	}

	// Trip circuit
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	// Immediately after opening
	diagOpen := cb.Diagnostics()
	if diagOpen.State != StateOpen {
		t.Fatalf("State = %v, want Open", diagOpen.State)
	}

	if diagOpen.TimeUntilHalfOpen <= 0 {
		t.Error("Open state: TimeUntilHalfOpen should be > 0 immediately after opening")
	}

	if diagOpen.TimeUntilHalfOpen > timeout {
		t.Errorf("TimeUntilHalfOpen = %v, should be <= timeout %v", diagOpen.TimeUntilHalfOpen, timeout)
	}

	// Wait partway
	time.Sleep(50 * time.Millisecond)

	diagMidway := cb.Diagnostics()
	if diagMidway.TimeUntilHalfOpen <= 0 || diagMidway.TimeUntilHalfOpen > timeout {
		t.Errorf("Midway: TimeUntilHalfOpen = %v, expected between 0 and %v", diagMidway.TimeUntilHalfOpen, timeout)
	}

	// Should decrease over time
	if diagMidway.TimeUntilHalfOpen >= diagOpen.TimeUntilHalfOpen {
		t.Error("TimeUntilHalfOpen should decrease as time passes")
	}

	// Wait for timeout to expire
	time.Sleep(100 * time.Millisecond)

	// After timeout, TimeUntilHalfOpen should be 0
	diagExpired := cb.Diagnostics()
	if diagExpired.TimeUntilHalfOpen != 0 {
		t.Errorf("After timeout: TimeUntilHalfOpen = %v, want 0", diagExpired.TimeUntilHalfOpen)
	}
}

func TestDiagnosticsInHalfOpenState(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// Trip and wait for half-open
	cb.Execute(failFunc)
	cb.Execute(failFunc)
	time.Sleep(100 * time.Millisecond)

	// Start a request to trigger half-open (but don't complete it yet)
	// Actually, we need to execute to trigger transition
	cb.Execute(successFunc) // This transitions to HalfOpen then Closed

	// Circuit is now Closed again due to successful probe
	diag := cb.Diagnostics()

	// WillTripNext should be false in Closed state with clean counts
	if diag.WillTripNext {
		t.Errorf("After recovery: WillTripNext = %v, want false", diag.WillTripNext)
	}

	// TimeUntilHalfOpen should be 0 (not Open)
	if diag.TimeUntilHalfOpen != 0 {
		t.Errorf("After recovery: TimeUntilHalfOpen = %v, want 0", diag.TimeUntilHalfOpen)
	}
}

func TestDiagnosticsDefaultConfiguration(t *testing.T) {
	// Test with all defaults
	cb := New(Settings{
		Name: "test-defaults",
	})

	diag := cb.Diagnostics()

	// Default MaxRequests
	if diag.MaxRequests != 1 {
		t.Errorf("Default MaxRequests = %v, want 1", diag.MaxRequests)
	}

	// Default Timeout
	if diag.Timeout != 60*time.Second {
		t.Errorf("Default Timeout = %v, want 60s", diag.Timeout)
	}

	// Default Adaptive disabled
	if diag.AdaptiveEnabled {
		t.Error("Default AdaptiveEnabled = true, want false")
	}

	// No adaptive settings when disabled
	if diag.FailureRateThreshold != 0 {
		t.Errorf("Default FailureRateThreshold = %v, want 0 (adaptive disabled)", diag.FailureRateThreshold)
	}
}

func TestDiagnosticsWillTripNextInNonClosedState(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	// Trip circuit
	cb.Execute(failFunc)
	cb.Execute(failFunc)

	// In Open state
	diagOpen := cb.Diagnostics()
	if diagOpen.State != StateOpen {
		t.Fatalf("State = %v, want Open", diagOpen.State)
	}

	// WillTripNext only relevant in Closed state
	if diagOpen.WillTripNext {
		t.Error("Open state: WillTripNext should be false (only relevant in Closed)")
	}
}
