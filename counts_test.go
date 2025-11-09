package autobreaker

import (
	"testing"
	"time"
)
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
