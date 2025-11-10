package breaker

import (
	"testing"
)

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
				Requests:      5,
				TotalFailures: 3,
			},
			want: false, // Below minimum observations
		},
		{
			name: "below threshold",
			counts: Counts{
				Requests:      100,
				TotalFailures: 5,
			},
			want: false, // 5% failure rate < 10% threshold
		},
		{
			name: "at threshold",
			counts: Counts{
				Requests:      100,
				TotalFailures: 10,
			},
			want: false, // 10% failure rate == 10% threshold (not >)
		},
		{
			name: "above threshold",
			counts: Counts{
				Requests:      100,
				TotalFailures: 11,
			},
			want: true, // 11% failure rate > 10% threshold
		},
		{
			name: "high failure rate",
			counts: Counts{
				Requests:      50,
				TotalFailures: 25,
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

func TestAdaptiveThresholdDefaults(t *testing.T) {
	cb := New(Settings{
		Name:              "test",
		AdaptiveThreshold: true,
	})

	// Test default failure rate threshold
	if cb.getFailureRateThreshold() != 0.05 {
		t.Errorf("default failureRateThreshold = %v, want 0.05", cb.getFailureRateThreshold())
	}

	// Test default minimum observations
	if cb.getMinimumObservations() != 20 {
		t.Errorf("default minimumObservations = %v, want 20", cb.getMinimumObservations())
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
