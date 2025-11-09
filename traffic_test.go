package autobreaker

import (
	"testing"
	"time"
)
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
