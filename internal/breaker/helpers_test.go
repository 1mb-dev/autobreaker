package breaker

import (
	"errors"
	"testing"
	"time"
)

// Test helper: successful operation
func successFunc() (interface{}, error) {
	return "success", nil
}

// Test helper: failing operation
func failFunc() (interface{}, error) {
	return nil, errors.New("operation failed")
}

// Test helper: panicking operation
func panicFunc() (interface{}, error) {
	panic("test panic")
}

// waitForState polls until circuit breaker reaches expected state or timeout.
// Returns true if state reached, false if timeout.
// Uses 10ms polling interval with configurable timeout.
func waitForState(t *testing.T, cb *CircuitBreaker, want State, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cb.State() == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cb.State() == want
}

// requireState asserts circuit breaker is in expected state within timeout.
// Fails test if state not reached.
func requireState(t *testing.T, cb *CircuitBreaker, want State, timeout time.Duration) {
	t.Helper()
	if !waitForState(t, cb, want, timeout) {
		t.Fatalf("Expected state %v within %v, got %v", want, timeout, cb.State())
	}
}

// Test constants for failure rate calculations
const (
	// Modulo values for achieving specific failure rates
	moduloFor3Percent  = 34 // 1/34 ≈ 2.94%
	moduloFor6Percent  = 17 // 1/17 ≈ 5.88%
	moduloFor12Percent = 8  // 1/8 = 12.5%
	moduloFor20Percent = 5  // 1/5 = 20%
)
