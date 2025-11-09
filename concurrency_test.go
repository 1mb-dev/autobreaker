package autobreaker

import (
	"sync"
	"testing"
	"time"
)
func TestConcurrentExecute(t *testing.T) {
	cb := New(Settings{
		Name: "concurrent-test",
		// Never trip during this test
		ReadyToTrip: func(counts Counts) bool {
			return false
		},
	})

	const goroutines = 100
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				// Mix successes and failures
				if (id+j)%3 == 0 {
					cb.Execute(failFunc)
				} else {
					cb.Execute(successFunc)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify counts are consistent
	counts := cb.Counts()
	expectedRequests := uint32(goroutines * requestsPerGoroutine)

	if counts.Requests != expectedRequests {
		t.Errorf("Concurrent requests: got %d, want %d", counts.Requests, expectedRequests)
	}

	// Total should equal sum of successes and failures
	total := counts.TotalSuccesses + counts.TotalFailures
	if total != expectedRequests {
		t.Errorf("Sum of successes+failures = %d, want %d", total, expectedRequests)
	}
}

func TestConcurrentStateTransitions(t *testing.T) {
	cb := New(Settings{
		Name:    "concurrent-transitions",
		Timeout: 50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrently trigger failures to trip circuit
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.Execute(failFunc)
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Circuit should be open after many concurrent failures
	if cb.State() != StateOpen {
		t.Errorf("After concurrent failures: state = %v, want Open", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Concurrent recovery attempts
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cb.Execute(successFunc)
		}()
	}

	wg.Wait()

	// Should have recovered to closed
	if cb.State() != StateClosed {
		t.Errorf("After concurrent recovery: state = %v, want Closed", cb.State())
	}
}

func TestConcurrentHalfOpenLimiting(t *testing.T) {
	cb := New(Settings{
		Name:        "concurrent-halfopen",
		MaxRequests: 3,
		Timeout:     50 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Launch many concurrent requests - only MaxRequests should execute
	const goroutines = 20
	results := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Use slow function to ensure concurrency
	slowSuccess := func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return "slow-success", nil
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := cb.Execute(slowSuccess)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Count how many were rejected
	rejectedCount := 0
	for err := range results {
		if err == ErrTooManyRequests {
			rejectedCount++
		}
	}

	// Most should be rejected (only MaxRequests allowed)
	if rejectedCount < goroutines-int(cb.maxRequests)-2 {
		t.Errorf("Too few rejections: got %d, want at least %d", rejectedCount, goroutines-int(cb.maxRequests)-2)
	}
}

func TestConcurrentCountClearing(t *testing.T) {
	cb := New(Settings{
		Name:     "concurrent-clearing",
		Interval: 100 * time.Millisecond,
	})

	const goroutines = 50
	var wg sync.WaitGroup

	// Phase 1: Concurrent requests
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.Execute(successFunc)
			}
		}()
	}
	wg.Wait()

	initialCounts := cb.Counts()
	if initialCounts.Requests == 0 {
		t.Fatal("Expected some requests to be counted")
	}

	// Wait for interval
	time.Sleep(150 * time.Millisecond)

	// Phase 2: More concurrent requests (should trigger clearing)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cb.Execute(successFunc)
		}()
	}
	wg.Wait()

	// Counts should be reset (only recent requests counted)
	finalCounts := cb.Counts()
	if finalCounts.Requests > uint32(goroutines*2) {
		t.Errorf("Counts not cleared properly: got %d requests", finalCounts.Requests)
	}
}

func TestRaceConditions(t *testing.T) {
	// This test is specifically designed to catch races with -race flag
	cb := New(Settings{
		Name:    "race-test",
		Timeout: 10 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	const goroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Mix of operations
				switch (id + j) % 5 {
				case 0:
					cb.Execute(successFunc)
				case 1:
					cb.Execute(failFunc)
				case 2:
					_ = cb.State()
				case 3:
					_ = cb.Counts()
				case 4:
					_ = cb.Name()
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without race detector errors, we're good
}

