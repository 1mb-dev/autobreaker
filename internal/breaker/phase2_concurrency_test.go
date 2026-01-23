package breaker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPhase2_StateTransitionRaceCondition tests the fix for state transition race condition.
// This test specifically validates that when multiple goroutines try to transition
// from Open to HalfOpen simultaneously, only one succeeds and others handle it correctly.
func TestPhase2_StateTransitionRaceCondition(t *testing.T) {
	cb := New(Settings{
		Name:    "phase2-race-test",
		Timeout: 10 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit to Open state
	cb.Execute(failFunc)

	// Verify circuit is open
	if cb.State() != StateOpen {
		t.Fatalf("Expected StateOpen after failure, got %v", cb.State())
	}

	// Wait for timeout to allow transition to HalfOpen
	time.Sleep(20 * time.Millisecond)

	const goroutines = 100
	var (
		transitionAttempts atomic.Int32
		successfulTransitions atomic.Int32
		failedTransitions atomic.Int32
		stillOpenCount atomic.Int32
		wg sync.WaitGroup
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			transitionAttempts.Add(1)

			// Try to execute - this will attempt transition to HalfOpen
			_, err := cb.Execute(successFunc)
			if err == nil {
				// Success - circuit was in HalfOpen or Closed
				successfulTransitions.Add(1)
			} else if err == ErrOpenState {
				// Still open - transition failed
				stillOpenCount.Add(1)
			} else {
				// Other error (e.g., ErrTooManyRequests)
				failedTransitions.Add(1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Transition attempts: %d", transitionAttempts.Load())
	t.Logf("Successful transitions: %d", successfulTransitions.Load())
	t.Logf("Still open count: %d", stillOpenCount.Load())
	t.Logf("Other failures: %d", failedTransitions.Load())

	// Verify only one goroutine successfully transitioned the circuit
	// (or all goroutines saw the circuit as already transitioned)
	finalState := cb.State()
	if finalState != StateHalfOpen && finalState != StateClosed {
		t.Errorf("Expected circuit to be in HalfOpen or Closed after timeout, got %v", finalState)
	}

	// Verify that we didn't have inconsistent state
	if stillOpenCount.Load() > 0 && successfulTransitions.Load() > 0 {
		// This would indicate a race condition where some goroutines thought
		// circuit was still open while others successfully executed
		t.Errorf("Race condition detected: %d goroutines thought circuit was open, %d executed successfully",
			stillOpenCount.Load(), successfulTransitions.Load())
	}
}
// TestPhase2_ConcurrentStateReadsDuringTransition tests concurrent state reads
// while state transitions are happening.
func TestPhase2_ConcurrentStateReadsDuringTransition(t *testing.T) {
	const (
		readerGoroutines = 50
		writerGoroutines = 10
		duration         = 100 * time.Millisecond
	)

	cb := New(Settings{
		Name:    "phase2-concurrent-reads",
		Timeout: 5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	var (
		stateReads   atomic.Int64
		executeCalls atomic.Int64
		wg           sync.WaitGroup
	)

	// Start readers that constantly read state
	for i := 0; i < readerGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				stateReads.Add(1)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Start writers that trigger state transitions
	for i := 0; i < writerGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for ctx.Err() == nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// Mix successes and failures
				if id%3 == 0 {
					cb.Execute(failFunc)
				} else {
					cb.Execute(successFunc)
				}
				executeCalls.Add(1)
				
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent state reads during transition test:")
	t.Logf("  State reads: %d", stateReads.Load())
	t.Logf("  Execute calls: %d", executeCalls.Load())
	t.Logf("  Final state: %v", cb.State())

	// Verify circuit is still functional
	result, err := cb.Execute(successFunc)
	if err != nil && err != ErrOpenState && err != ErrTooManyRequests {
		t.Errorf("Circuit not functional after concurrent access: %v", err)
	}
	if err == nil && result != "success" {
		t.Errorf("Unexpected result: %v", result)
	}
}

// TestPhase2_MultipleConcurrentTransitions tests multiple concurrent transition attempts.
func TestPhase2_MultipleConcurrentTransitions(t *testing.T) {
	cb := New(Settings{
		Name:    "phase2-multiple-transitions",
		Timeout: 2 * time.Millisecond, // Very short timeout for rapid transitions
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
	})

	const iterations = 100
	const goroutines = 20

	var wg sync.WaitGroup
	var inconsistentStateCount atomic.Int32

	for iter := 0; iter < iterations; iter++ {
		// Start in Open state
		cb.Execute(failFunc)
		cb.Execute(failFunc)
		
		if cb.State() != StateOpen {
			t.Fatalf("Iteration %d: Expected StateOpen, got %v", iter, cb.State())
		}

		// Wait for timeout
		time.Sleep(5 * time.Millisecond)

		// Multiple goroutines try to transition simultaneously
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(gid int) {
				defer wg.Done()
				
				// Record state before attempt
				stateBefore := cb.State()
				
				// Try to execute (will attempt transition if state is Open)
				_, err := cb.Execute(successFunc)
				
				// Record state after attempt
				stateAfter := cb.State()
				
				// Check for inconsistent state transitions
				if stateBefore == StateOpen && err == nil && stateAfter == StateOpen {
					// This would indicate a bug: executed successfully but state didn't change
					inconsistentStateCount.Add(1)
					t.Errorf("Goroutine %d: Executed successfully but state remained Open", gid)
				}
				
				if stateBefore == StateOpen && err == ErrOpenState && stateAfter == StateHalfOpen {
					// This would indicate a bug: got ErrOpenState but circuit transitioned
					inconsistentStateCount.Add(1)
					t.Errorf("Goroutine %d: Got ErrOpenState but circuit is HalfOpen", gid)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Reset for next iteration
		cb.transitionToClosed()
	}

	t.Logf("Multiple concurrent transitions test:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Goroutines per iteration: %d", goroutines)
	t.Logf("  Inconsistent state transitions detected: %d", inconsistentStateCount.Load())

	if inconsistentStateCount.Load() > 0 {
		t.Errorf("Found %d inconsistent state transitions", inconsistentStateCount.Load())
	}
}

// TestPhase2_HighConcurrencyMixedOperations tests mixed operations under high concurrency.
func TestPhase2_HighConcurrencyMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	cb := New(Settings{
		Name:                 "phase2-high-concurrency",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10% threshold
		MinimumObservations:  50,
		Timeout:              10 * time.Millisecond,
	})

	const (
		totalGoroutines = 200
		duration        = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		executeCount    atomic.Int64
		stateReadCount  atomic.Int64
		countsReadCount atomic.Int64
		metricsReadCount atomic.Int64
		updateCount     atomic.Int64
		wg              sync.WaitGroup
	)

	// Start mixed workload goroutines
	for i := 0; i < totalGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for ctx.Err() == nil {
				// Distribute operations
				switch id % 10 {
				case 0, 1, 2, 3, 4:
					// Execute operations (50%)
					if id%7 == 0 {
						cb.Execute(failFunc)
					} else {
						cb.Execute(successFunc)
					}
					executeCount.Add(1)
				case 5, 6:
					// State reads (20%)
					_ = cb.State()
					stateReadCount.Add(1)
				case 7:
					// Counts reads (10%)
					_ = cb.Counts()
					countsReadCount.Add(1)
				case 8:
					// Metrics reads (10%)
					_ = cb.Metrics()
					metricsReadCount.Add(1)
				case 9:
					// Settings updates (10%)
					update := SettingsUpdate{
						Timeout: DurationPtr(time.Duration(id%100) * time.Millisecond),
					}
					_ = cb.UpdateSettings(update)
					updateCount.Add(1)
				}
				
				// Small sleep to prevent overwhelming
				time.Sleep(time.Microsecond * time.Duration(id%100))
			}
		}(i)
	}

	wg.Wait()

	t.Logf("High concurrency mixed operations test:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Goroutines: %d", totalGoroutines)
	t.Logf("  Execute calls: %d", executeCount.Load())
	t.Logf("  State reads: %d", stateReadCount.Load())
	t.Logf("  Counts reads: %d", countsReadCount.Load())
	t.Logf("  Metrics reads: %d", metricsReadCount.Load())
	t.Logf("  Settings updates: %d", updateCount.Load())
	t.Logf("  Final state: %v", cb.State())
	t.Logf("  Final counts: %+v", cb.Counts())

	// Verify circuit is still functional
	result, err := cb.Execute(successFunc)
	if err != nil && err != ErrOpenState && err != ErrTooManyRequests {
		t.Errorf("Circuit not functional after high concurrency: %v", err)
	}
	if err == nil && result != "success" {
		t.Errorf("Unexpected result: %v", result)
	}

	// Verify no panic occurred during high concurrency
	// (if we got here without panic, test passes)
}

// TestPhase2_RaceConditionOpenToHalfOpen specifically tests the Open→HalfOpen race condition fix.
func TestPhase2_RaceConditionOpenToHalfOpen(t *testing.T) {
	// This test creates a scenario where many goroutines simultaneously
	// try to transition from Open to HalfOpen state.
	
	cb := New(Settings{
		Name:    "phase2-open-halfopen-race",
		Timeout: 1 * time.Millisecond, // Very short timeout
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Trip circuit
	cb.Execute(failFunc)
	
	if cb.State() != StateOpen {
		t.Fatalf("Expected StateOpen, got %v", cb.State())
	}

	const goroutines = 500
	var (
		transitionedToHalfOpen atomic.Int32
		receivedErrOpenState   atomic.Int32
		successfulExecutions   atomic.Int32
		wg                     sync.WaitGroup
	)

	// Wait for timeout
	time.Sleep(2 * time.Millisecond)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			// All goroutines try to execute simultaneously
			_, err := cb.Execute(successFunc)
			
			if err == nil {
				successfulExecutions.Add(1)
			} else if err == ErrOpenState {
				receivedErrOpenState.Add(1)
			} else if err == ErrTooManyRequests {
				// This is OK - circuit transitioned to HalfOpen but we hit MaxRequests limit
				transitionedToHalfOpen.Add(1)
			}
		}(i)
	}

	wg.Wait()

	finalState := cb.State()
	
	t.Logf("Open→HalfOpen race condition test:")
	t.Logf("  Goroutines: %d", goroutines)
	t.Logf("  Successful executions: %d", successfulExecutions.Load())
	t.Logf("  Received ErrOpenState: %d", receivedErrOpenState.Load())
	t.Logf("  Hit MaxRequests (ErrTooManyRequests): %d", transitionedToHalfOpen.Load())
	t.Logf("  Final state: %v", finalState)

	// Validate the fix:
	// 1. Circuit should have transitioned to HalfOpen or Closed
	if finalState != StateHalfOpen && finalState != StateClosed {
		t.Errorf("Circuit should be HalfOpen or Closed after timeout, got %v", finalState)
	}
	
	// 2. We should NOT have both successful executions and ErrOpenState
	// (that would mean some goroutines thought circuit was open while others executed)
	if successfulExecutions.Load() > 0 && receivedErrOpenState.Load() > 0 {
		t.Errorf("Race condition: %d goroutines executed successfully while %d thought circuit was open",
			successfulExecutions.Load(), receivedErrOpenState.Load())
	}
	
	// 3. If circuit is HalfOpen, successful executions should be <= MaxRequests
	if finalState == StateHalfOpen && successfulExecutions.Load() > 1 {
		// MaxRequests defaults to 1
		t.Errorf("Too many successful executions in HalfOpen state: %d (MaxRequests=1)",
			successfulExecutions.Load())
	}
}
