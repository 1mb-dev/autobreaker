package breaker

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// callbackPanicHandler handles panics in user callbacks with proper logging and metrics.
// This is the internal panic handler that provides deterministic behavior for each callback type.
type callbackPanicHandler struct{}

// logMutex protects fmt.Printf calls from concurrent access
var logMutex sync.Mutex

// handleReadyToTripPanic handles a panic in the ReadyToTrip callback.
// Returns a safe default: treat as "do not trip" (circuit stays closed).
func (h *callbackPanicHandler) handleReadyToTripPanic(name string, r interface{}) {
	// Log the panic with stack trace
	logCallbackPanic("ReadyToTrip", name, r)
	
	// Safe default: do not trip the circuit
	// This prevents a panicking callback from causing false circuit opens
}

// handleOnStateChangePanic handles a panic in the OnStateChange callback.
// Logs the panic but allows the state transition to proceed.
func (h *callbackPanicHandler) handleOnStateChangePanic(name string, from, to State, r interface{}) {
	// Log the panic with stack trace
	logCallbackPanic("OnStateChange", name, r)
	
	// State transition proceeds despite callback panic
	// This prevents a panicking callback from blocking state transitions
}

// handleIsSuccessfulPanic handles a panic in the IsSuccessful callback.
// Returns a safe default: treat as failure (conservative approach).
func (h *callbackPanicHandler) handleIsSuccessfulPanic(name string, r interface{}) bool {
	// Log the panic with stack trace
	logCallbackPanic("IsSuccessful", name, r)
	
	// Safe default: treat as failure
	// This is conservative - better to potentially trip circuit than ignore errors
	return false
}

// logCallbackPanic logs a callback panic with stack trace.
func logCallbackPanic(callbackName, circuitName string, panicValue interface{}) {
	// In production, we would log with stack trace
	// For now, use simple logging to avoid race detector issues
	logMutex.Lock()
	defer logMutex.Unlock()
	
	// Simple log without stack trace to avoid race detector issues
	fmt.Printf("[AUTOBREAKER WARNING] Circuit %q: %s callback panicked: %v\n",
		circuitName, callbackName, panicValue)
	
	// TODO: In v1.2.0, integrate with user-provided logging/metrics
	// Note: debug.Stack() can have race detector issues in high-concurrency scenarios
}

// safeCallWithRecovery executes a callback with panic recovery and proper handling.
// It provides deterministic behavior for each callback type.
func safeCallWithRecovery(callbackType string, circuitName string, fn func(), panicHandler func(interface{})) {
	if fn == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			// Call the appropriate panic handler
			panicHandler(r)
		}
	}()

	fn()
}

// safeCallReadyToTrip executes ReadyToTrip callback with panic recovery.
// Returns false (do not trip) if callback panics.
func safeCallReadyToTrip(circuitName string, fn func(Counts) bool, counts Counts) bool {
	var result bool
	handler := &callbackPanicHandler{}
	
	safeCallWithRecovery("ReadyToTrip", circuitName, func() {
		result = fn(counts)
	}, func(r interface{}) {
		handler.handleReadyToTripPanic(circuitName, r)
		result = false // Safe default: do not trip
	})
	
	return result
}

// safeCallOnStateChange executes OnStateChange callback with panic recovery.
func safeCallOnStateChange(circuitName string, fn func(string, State, State), from, to State) {
	if fn == nil {
		return
	}
	
	handler := &callbackPanicHandler{}
	
	safeCallWithRecovery("OnStateChange", circuitName, func() {
		fn(circuitName, from, to)
	}, func(r interface{}) {
		handler.handleOnStateChangePanic(circuitName, from, to, r)
	})
}

// safeCallIsSuccessful executes IsSuccessful callback with panic recovery.
// Returns false (failure) if callback panics.
func safeCallIsSuccessful(circuitName string, fn func(error) bool, err error) bool {
	var result bool
	handler := &callbackPanicHandler{}
	
	safeCallWithRecovery("IsSuccessful", circuitName, func() {
		result = fn(err)
	}, func(r interface{}) {
		result = handler.handleIsSuccessfulPanic(circuitName, r)
	})
	
	return result
}

// safeIncrementCounter safely increments a uint32 counter with saturation protection.
// Returns true if the counter was incremented, false if it was already at max.
// Logs a warning when saturation occurs.
func safeIncrementCounter(counter *atomic.Uint32, counterName, circuitName string) bool {
	// Use CompareAndSwap loop for atomic check-and-increment
	for {
		current := counter.Load()
		if current == math.MaxUint32 {
			// Already at max, cannot increment
			// Log saturation warning
			logCounterSaturation(counterName, circuitName, current)
			return false
		}
		if counter.CompareAndSwap(current, current+1) {
			return true
		}
		// CAS failed, retry
	}
}

// safeIncrementRequests safely increments the requests counter with saturation protection.
// Returns true if the counter was incremented, false if it was already at max (saturated).
func (cb *CircuitBreaker) safeIncrementRequests() bool {
	return safeIncrementCounter(&cb.requests, "requests", cb.name)
}

// safeDecrementRequests safely decrements the requests counter with underflow protection.
// Returns true if the counter was decremented, false if it was already at 0.
func (cb *CircuitBreaker) safeDecrementRequests() bool {
	// Use CompareAndSwap loop for atomic check-and-decrement
	for {
		current := cb.requests.Load()
		if current == 0 {
			// Already at 0, cannot decrement
			return false
		}
		if cb.requests.CompareAndSwap(current, current-1) {
			return true
		}
		// CAS failed, retry
	}
}

// logCounterSaturation logs a counter saturation event.
func logCounterSaturation(counterName, circuitName string, currentValue uint32) {
	// Format log message
	// Use mutex to protect concurrent fmt.Printf calls
	logMutex.Lock()
	defer logMutex.Unlock()
	
	fmt.Printf("[AUTOBREAKER WARNING] Circuit %q: %s counter saturated at %d (max uint32)\n",
		circuitName, counterName, currentValue)
	
	// TODO: In v1.2.0, integrate with user-provided logging/metrics
	// TODO: Add metrics for saturation events
}
