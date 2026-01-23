package breaker

import (
	"testing"
	"time"
)

func TestExactlyMinimumObservations(t *testing.T) {
	cb := New(Settings{
		Name:                 "exact-minimum",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10%
		MinimumObservations:  20,
	})

	// Send exactly 20 requests with 15% failure rate
	for i := 0; i < 20; i++ {
		if i < 3 { // 3 failures out of 20 = 15%
			cb.Execute(failFunc)
		} else {
			cb.Execute(successFunc)
		}
	}

	// Next failure should cause trip (4 failures / 21 requests = 19% > 10%)
	cb.Execute(failFunc)

	// Circuit should now be open
	if cb.State() != StateOpen {
		t.Errorf("Should trip after exceeding threshold, got state=%v", cb.State())
	}
}

// Edge case: Zero traffic for extended period
func TestZeroTrafficPeriod(t *testing.T) {
	cb := New(Settings{
		Name:                 "zero-traffic",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
	})

	// Send some requests
	for i := 0; i < 50; i++ {
		cb.Execute(successFunc)
	}

	counts := cb.Counts()
	if counts.Requests != 50 {
		t.Errorf("Initial requests = %v, want 50", counts.Requests)
	}

	// Long period with zero traffic (no requests)
	time.Sleep(200 * time.Millisecond)

	// Send more requests - should work normally
	for i := 0; i < 50; i++ {
		cb.Execute(successFunc)
	}

	counts = cb.Counts()
	if counts.Requests != 100 {
		t.Errorf("After zero traffic: requests = %v, want 100", counts.Requests)
	}

	// Should still be closed (no failures)
	if cb.State() != StateClosed {
		t.Errorf("State should be closed, got %v", cb.State())
	}
}

// Edge case: All failures in burst
func TestBurstFailures(t *testing.T) {
	cb := New(Settings{
		Name:                 "burst-failures",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
	})

	// Send 30 successful requests
	for i := 0; i < 30; i++ {
		cb.Execute(successFunc)
	}

	// Then burst of 10 failures
	for i := 0; i < 10; i++ {
		cb.Execute(failFunc)
	}

	// 10 failures out of 40 total = 25% failure rate
	// Should have tripped (25% > 5%)
	if cb.State() != StateOpen {
		t.Errorf("Should trip after burst failures, got state %v", cb.State())
	}

	counts := cb.Counts()
	expectedRequests := uint32(30 + 10) // Some might be rejected after trip
	if counts.Requests > expectedRequests {
		t.Errorf("Requests = %v, expected <= %v", counts.Requests, expectedRequests)
	}
}

// Stability: No oscillation between states
func TestNoOscillation(t *testing.T) {
	cb := New(Settings{
		Name:                 "no-oscillation",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
		Timeout:              50 * time.Millisecond,
		MaxRequests:          1,
	})

	// Trip the circuit
	for i := 0; i < 30; i++ {
		if i%10 < 2 { // 20% failure rate
			cb.Execute(failFunc)
		} else {
			cb.Execute(successFunc)
		}
	}

	if cb.State() != StateOpen {
		t.Errorf("Circuit should be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Send one successful request (should transition to HalfOpen, then Closed)
	_, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Expected successful recovery, got err=%v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("Should recover to closed, got %v", cb.State())
	}

	// Continue with low failure rate - should stay closed
	stateChanges := 0
	prevState := cb.State()

	for i := 0; i < 100; i++ {
		if i%50 == 0 { // 2% failure rate (below threshold)
			cb.Execute(failFunc)
		} else {
			cb.Execute(successFunc)
		}

		currentState := cb.State()
		if currentState != prevState {
			stateChanges++
			prevState = currentState
		}
	}

	// Should not oscillate with low failure rate
	if stateChanges > 0 {
		t.Errorf("Circuit oscillated %d times, expected 0 with 2%% failure rate", stateChanges)
	}
}

// Stability: Consistent behavior over extended period
func TestLongRunningStability(t *testing.T) {
	cb := New(Settings{
		Name:                 "long-running",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
	})

	// Run for extended period with consistent 3% failure rate (well below 5% threshold)
	totalRequests := 1000
	totalFailures := 0

	for i := 0; i < totalRequests; i++ {
		// Distribute failures evenly: moduloFor3Percent = ~2.9% failure rate
		if i > 0 && i%moduloFor3Percent == 0 { // Skip i=0 to avoid early failures
			cb.Execute(failFunc)
			totalFailures++
		} else {
			cb.Execute(successFunc)
		}
	}

	// Should remain closed throughout (2.9% < 5%)
	if cb.State() != StateClosed {
		t.Errorf("Should stay closed with consistent 2.9%% failure rate, got %v", cb.State())
	}

	actualRate := float64(totalFailures) / float64(totalRequests) * 100
	t.Logf("Completed %d requests with %d failures (%.1f%%), remained stable",
		totalRequests, totalFailures, actualRate)
}

// Edge case: Rapid state transitions
func TestRapidStateTransitions(t *testing.T) {
	cb := New(Settings{
		Name:                 "rapid-transitions",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10,
		MinimumObservations:  20,
		Timeout:              10 * time.Millisecond, // Very short timeout
		MaxRequests:          1,
	})

	// Trip the circuit with distributed failures
	for i := 0; i < 30; i++ {
		// Distribute failures throughout: moduloFor20Percent = 20% > 10% threshold
		if i > 0 && i%moduloFor20Percent == 0 {
			cb.Execute(failFunc)
		} else {
			cb.Execute(successFunc)
		}
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open with 20%% failure rate, got %v", cb.State())
	}

	// Rapid recovery attempts
	for attempt := 0; attempt < 3; attempt++ {
		time.Sleep(15 * time.Millisecond) // Wait for timeout

		// Successful recovery
		result, err := cb.Execute(successFunc)
		if err != nil {
			t.Errorf("Attempt %d: Expected success, got err=%v", attempt, err)
		}
		if result == nil {
			t.Errorf("Attempt %d: Expected result, got nil", attempt)
		}

		// Should be closed now
		if cb.State() != StateClosed {
			t.Errorf("Attempt %d: Should be closed after success, got %v", attempt, cb.State())
		}

		// Immediately trip again with distributed failures
		for i := 0; i < 30; i++ {
			// moduloFor20Percent = 20% > 10% threshold
			if i > 0 && i%moduloFor20Percent == 0 {
				cb.Execute(failFunc)
			} else {
				cb.Execute(successFunc)
			}
		}

		if cb.State() != StateOpen {
			t.Errorf("Attempt %d: Should trip again with 20%% failure rate, got %v", attempt, cb.State())
		}
	}

	t.Logf("Completed 3 rapid open→closed→open cycles successfully")
}

// Callback Safety Tests - Task 1.5: Verify panic recovery for user callbacks

// TestCallbackPanicReadyToTrip verifies that readyToTrip callback panics are recovered
func TestCallbackPanicReadyToTrip(t *testing.T) {
	panicCalled := false
	panicRecovered := false

	cb := New(Settings{
		Name: "test-ready-to-trip-panic",
		ReadyToTrip: func(counts Counts) bool {
			panicCalled = true
			panic("readyToTrip callback panic!")
		},
		OnStateChange: func(name string, from State, to State) {
			// This should still be called even if readyToTrip panics
		},
	})

	// Execute a failure - readyToTrip will be called
	// The panic should be recovered and circuit should handle it gracefully
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicRecovered = true
			}
		}()
		cb.Execute(failFunc)
	}()

	// Verify panic was called
	if !panicCalled {
		t.Error("readyToTrip callback should have been called")
	}

	// Verify panic was recovered (not propagated to user)
	if panicRecovered {
		t.Error("readyToTrip panic should have been recovered internally, not propagated")
	}

	// Circuit should still be functional
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be closed after callback panic, got %v", cb.State())
	}

	// Should be able to continue using circuit
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Should be able to execute after callback panic, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Should get success result after callback panic, got: %v", result)
	}
}

// TestCallbackPanicOnStateChange verifies that onStateChange callback panics are recovered
func TestCallbackPanicOnStateChange(t *testing.T) {
	stateChangeCount := 0

	cb := New(Settings{
		Name:    "test-state-change-panic",
		Timeout: 50 * time.Millisecond, // Set short timeout for test
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
		OnStateChange: func(name string, from State, to State) {
			stateChangeCount++
			if stateChangeCount == 1 {
				panic("onStateChange callback panic!")
			}
		},
	})

	// Execute a failure - should trigger state change (Closed → Open)
	// The panic should be recovered internally by safeCall
	cb.Execute(failFunc)

	// Verify circuit transitioned to Open state despite callback panic
	if cb.State() != StateOpen {
		t.Errorf("Circuit should be Open after failure, got %v", cb.State())
	}

	// onStateChange should have been attempted once (even though it panicked)
	// Note: stateChangeCount will be 1 because the panic happens AFTER incrementing
	if stateChangeCount != 1 {
		t.Errorf("onStateChange should have been attempted once, got %d", stateChangeCount)
	}

	// Wait for timeout and try recovery
	time.Sleep(100 * time.Millisecond)

	// Execute success - should trigger state changes (Open → HalfOpen → Closed)
	// The callback will panic on first call, but subsequent calls should work
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Should be able to execute after timeout, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Should get success result, got: %v", result)
	}

	// Circuit should now be Closed
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be Closed after successful probe, got %v", cb.State())
	}

	// onStateChange should have been attempted 3 times total
	// 1st: Closed → Open (panicked)
	// 2nd: Open → HalfOpen (should work)
	// 3rd: HalfOpen → Closed (should work)
	if stateChangeCount != 3 {
		t.Errorf("onStateChange should have been attempted 3 times total, got %d", stateChangeCount)
	}
}

// TestCallbackPanicIsSuccessful verifies that isSuccessful callback panics are recovered
func TestCallbackPanicIsSuccessful(t *testing.T) {
	panicCalled := false

	cb := New(Settings{
		Name: "test-is-successful-panic",
		IsSuccessful: func(err error) bool {
			panicCalled = true
			panic("isSuccessful callback panic!")
		},
	})

	// Execute a request - isSuccessful will be called
	// The panic should be recovered and treated as failure
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isSuccessful panic should have been recovered internally, not propagated: %v", r)
			}
		}()
		cb.Execute(successFunc)
	}()

	// Verify panic was called
	if !panicCalled {
		t.Error("isSuccessful callback should have been called")
	}

	// Circuit should still be functional
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be closed after callback panic, got %v", cb.State())
	}

	// The request should be counted as failure (panic in isSuccessful)
	counts := cb.Counts()
	if counts.TotalFailures != 1 {
		t.Errorf("Request with panicking isSuccessful should count as failure, got %d failures", counts.TotalFailures)
	}

	// Should be able to continue using circuit
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Should be able to execute after callback panic, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Should get success result after callback panic, got: %v", result)
	}
}

// TestMultipleCallbackPanics verifies circuit remains functional with multiple callback panics
func TestMultipleCallbackPanics(t *testing.T) {
	callbackCallCount := 0

	cb := New(Settings{
		Name:    "test-multiple-panics",
		Timeout: 50 * time.Millisecond, // Set short timeout for test
		ReadyToTrip: func(counts Counts) bool {
			callbackCallCount++
			if callbackCallCount == 1 {
				panic("first readyToTrip panic!")
			}
			return counts.ConsecutiveFailures > 1
		},
		OnStateChange: func(name string, from State, to State) {
			callbackCallCount++
			if callbackCallCount == 3 {
				panic("onStateChange panic!")
			}
		},
		IsSuccessful: func(err error) bool {
			callbackCallCount++
			if callbackCallCount == 5 {
				panic("isSuccessful panic!")
			}
			return err == nil
		},
	})

	// Execute multiple requests with various outcomes
	// All callback panics should be recovered

	// First request - readyToTrip might panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Callback panic should have been recovered internally: %v", r)
			}
		}()
		cb.Execute(failFunc)
	}()

	// Second request - might trigger onStateChange panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Callback panic should have been recovered internally: %v", r)
			}
		}()
		cb.Execute(failFunc)
	}()

	// Circuit should be Open after 2 failures
	if cb.State() != StateOpen {
		t.Errorf("Circuit should be Open after 2 failures, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Third request - might trigger isSuccessful panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Callback panic should have been recovered internally: %v", r)
			}
		}()
		cb.Execute(successFunc)
	}()

	// Circuit should recover to Closed
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be Closed after successful probe, got %v", cb.State())
	}

	// Verify circuit is still functional
	result, err := cb.Execute(successFunc)
	if err != nil {
		t.Errorf("Circuit should still be functional after multiple callback panics, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Should get success result, got: %v", result)
	}

	t.Logf("Circuit survived %d callback calls with multiple panics, remained functional", callbackCallCount)
}
