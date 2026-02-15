// Package autobreaker provides an adaptive circuit breaker for Go that automatically
// adjusts failure thresholds based on traffic patterns.
//
// # Overview
//
// AutoBreaker implements the circuit breaker pattern with automatic adaptation to request
// volume. Unlike traditional circuit breakers that use static failure thresholds (e.g.,
// "trip after 10 failures"), AutoBreaker uses percentage-based thresholds that work across
// different traffic levels without manual tuning.
//
// # The Problem
//
// Traditional circuit breakers use absolute failure counts:
//   - At high traffic: 10 failures might be <1% error rate (too sensitive, false positives)
//   - At low traffic: 10 failures might be 100% error rate (too slow to protect)
//   - Different environments need different thresholds (dev/staging/prod)
//
// # The Solution
//
// AutoBreaker uses percentage-based thresholds:
//   - Same configuration works across 10x+ traffic variance
//   - Automatically adapts: at 100 RPS, 5% threshold = 50 failures/10s
//   - Automatically adapts: at 10 RPS, 5% threshold = 5 failures/10s
//   - One config for all environments
//
// # Quick Start
//
// Create a circuit breaker with adaptive thresholds:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:                 "api-client",
//	    Timeout:              10 * time.Second,
//	    AdaptiveThreshold:    true,
//	    FailureRateThreshold: 0.05, // 5% failure rate trips circuit
//	    MinimumObservations:  20,   // Require 20 requests before adapting
//	})
//
// Wrap operations with the circuit breaker:
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return externalService.Call()
//	})
//	if err == autobreaker.ErrOpenState {
//	    // Circuit is open, fail fast
//	    return nil, err
//	}
//
// # Key Features
//
//   - Adaptive Thresholds: Percentage-based thresholds adapt to traffic volume
//   - Runtime Configuration: Update settings without restart via UpdateSettings()
//   - Rich Observability: Metrics() and Diagnostics() APIs for monitoring
//   - Zero Dependencies: Only standard library
//   - High Performance: <100ns overhead per request, lock-free design
//   - Thread-Safe: All methods safe for concurrent use
//   - Drop-in Replacement: Compatible with sony/gobreaker API
//
// # Circuit States
//
// The circuit breaker operates in three states:
//
//   - Closed: Normal operation, requests pass through, failures are counted
//   - Open: Circuit has tripped, requests fail fast with ErrOpenState
//   - HalfOpen: Testing recovery, limited requests probe backend health
//
// State transitions:
//   - Closed → Open: When failure rate exceeds threshold
//   - Open → HalfOpen: After timeout duration expires
//   - HalfOpen → Closed: When probe requests succeed (recovery detected)
//   - HalfOpen → Open: When probe requests fail (still unhealthy)
//
// # Observability
//
// Monitor circuit breaker state and behavior:
//
//	// Real-time metrics
//	metrics := breaker.Metrics()
//	fmt.Printf("State: %s, Failure Rate: %.2f%%\n",
//	    metrics.State, metrics.FailureRate*100)
//
//	// Comprehensive diagnostics
//	diag := breaker.Diagnostics()
//	if diag.WillTripNext {
//	    log.Warn("Circuit about to trip!")
//	}
//
// # Runtime Configuration
//
// Update settings without restarting:
//
//	err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    FailureRateThreshold: autobreaker.Float64Ptr(0.10), // Increase to 10%
//	    Timeout:              autobreaker.DurationPtr(30 * time.Second),
//	})
//
// All updates are validated and applied atomically. Settings updates are thread-safe
// and can be called concurrently with Execute().
//
// # Thread Safety
//
// All CircuitBreaker methods are safe for concurrent use:
//   - Execute() can be called from multiple goroutines simultaneously
//   - UpdateSettings() can be called concurrently with Execute()
//   - State(), Counts(), Metrics(), Diagnostics() are thread-safe accessors
//   - No external synchronization required
//
// The implementation uses lock-free atomic operations for optimal performance.
//
// # Performance
//
//   - Execute() overhead: <100ns per request (in Closed state)
//   - Zero allocations in hot path
//   - Lock-free design, no contention
//   - Scales linearly with concurrent requests
//
// # Examples
//
// See the examples/ directory for comprehensive usage examples:
//   - examples/basic/ - Fundamental circuit breaker patterns
//   - examples/adaptive/ - Adaptive vs static threshold comparison
//   - examples/observability/ - Monitoring and diagnostics
//   - examples/runtime_config/ - Runtime configuration updates
//   - examples/prometheus/ - Prometheus integration
//   - examples/production_ready/ - Production deployment patterns
//
// # Error Handling
//
// Circuit breaker errors:
//   - ErrOpenState: Circuit is open, request rejected (fail fast)
//   - ErrTooManyRequests: Too many concurrent requests in half-open state
//
// Application errors are passed through unchanged. Use the IsSuccessful callback
// to customize which errors count as failures:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    IsSuccessful: func(err error) bool {
//	        // 4xx client errors don't indicate service failure
//	        return err == nil || isClientError(err)
//	    },
//	})
//
// # Best Practices
//
//   - Use adaptive thresholds for services with variable traffic
//   - Set MinimumObservations to prevent false positives at low traffic
//   - Monitor with Metrics() for dashboards and alerts
//   - Use Diagnostics() for troubleshooting and incident response
//   - Update settings via UpdateSettings() for dynamic tuning
//   - Set appropriate Timeout based on service recovery characteristics
//
// # Compatibility
//
// AutoBreaker is compatible with sony/gobreaker API for easy migration:
//
//	// sony/gobreaker
//	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
//	    Name: "api",
//	})
//
//	// autobreaker (drop-in replacement)
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "api",
//	})
//
// Enable adaptive thresholds by setting AdaptiveThreshold: true.
package autobreaker

import "github.com/1mb-dev/autobreaker/internal/breaker"

// Core Types
//
// These types form the public API of the circuit breaker.

// CircuitBreaker is the main type that implements the circuit breaker pattern with
// adaptive thresholds. See the internal/breaker package for implementation details.
//
// All methods are thread-safe and can be called concurrently.
type CircuitBreaker = breaker.CircuitBreaker

// State represents the current state of the circuit breaker.
// Valid states are StateClosed, StateOpen, and StateHalfOpen.
type State = breaker.State

// Counts holds statistics about requests processed by the circuit breaker.
// Returned by the Counts() method.
type Counts = breaker.Counts

// Settings configures a circuit breaker instance. Passed to New() to create
// a circuit breaker.
//
// See internal/breaker.Settings for detailed field documentation.
type Settings = breaker.Settings

// SettingsUpdate specifies runtime configuration updates. Used with UpdateSettings()
// to modify circuit breaker settings without restarting.
//
// Fields set to nil will not be updated. Non-nil fields will update the corresponding setting.
// See internal/breaker.SettingsUpdate for detailed field documentation.
type SettingsUpdate = breaker.SettingsUpdate

// Metrics provides real-time metrics about the circuit breaker state and behavior.
// Returned by the Metrics() method. Useful for monitoring and dashboards.
//
// See internal/breaker.Metrics for detailed field documentation.
type Metrics = breaker.Metrics

// Diagnostics provides comprehensive diagnostic information about the circuit breaker.
// Returned by the Diagnostics() method. Useful for troubleshooting and debugging.
//
// See internal/breaker.Diagnostics for detailed field documentation.
type Diagnostics = breaker.Diagnostics

// State Constants
//
// These constants represent the three possible circuit breaker states.

const (
	// StateClosed indicates the circuit is closed (normal operation).
	// Requests pass through and failures are counted. If the failure rate
	// exceeds the threshold, the circuit transitions to Open.
	StateClosed = breaker.StateClosed

	// StateOpen indicates the circuit is open (failed state).
	// All requests are rejected immediately with ErrOpenState to prevent
	// cascading failures. After the timeout period, the circuit transitions
	// to HalfOpen to test recovery.
	StateOpen = breaker.StateOpen

	// StateHalfOpen indicates the circuit is testing recovery.
	// A limited number of requests (MaxRequests) are allowed to probe the
	// backend. If they succeed, the circuit closes. If they fail, the circuit
	// reopens.
	StateHalfOpen = breaker.StateHalfOpen
)

// Errors
//
// These errors are returned by the circuit breaker to indicate its state.

var (
	// ErrOpenState is returned when Execute() is called but the circuit breaker
	// is in the Open state. This indicates fail-fast behavior to prevent
	// cascading failures. The application should handle this error gracefully,
	// typically by returning a cached response or degraded service.
	ErrOpenState = breaker.ErrOpenState

	// ErrTooManyRequests is returned when too many concurrent requests are
	// attempted in the HalfOpen state. The circuit breaker limits concurrent
	// requests during recovery testing (controlled by MaxRequests setting).
	// This error indicates the circuit is testing recovery and additional
	// concurrent requests should wait or fail fast.
	ErrTooManyRequests = breaker.ErrTooManyRequests
)

// Constructor and Helper Functions
//
// These functions create and configure circuit breakers.
//
// Implementation Note: Package Variable Pattern
//
// We expose internal/breaker functions via package variables (var New = breaker.New)
// rather than wrapper functions. This provides a cleaner import path for users
// (autobreaker.New vs breaker.New) while avoiding wrapper function overhead.
//
// Trade-offs:
//   - Pros: Cleaner API, zero wrapper overhead, simpler imports
//   - Cons: Less conventional, harder to mock in tests
//
// This pattern is intentional and appropriate for a library facade.
// Consider function wrappers in v2.0 if testability issues arise.

// New creates a new CircuitBreaker with the given settings.
//
// Settings are validated at creation time. Invalid settings will cause a panic.
// See Settings type for configuration options and defaults.
//
// Example:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name:                 "api-client",
//	    Timeout:              10 * time.Second,
//	    AdaptiveThreshold:    true,
//	    FailureRateThreshold: 0.05,
//	    MinimumObservations:  20,
//	})
//
// The returned CircuitBreaker is ready to use and thread-safe.
var New = breaker.New

// Uint32Ptr returns a pointer to the given uint32 value.
// Helper function for constructing SettingsUpdate with explicit values.
//
// Example:
//
//	breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    MaxRequests: autobreaker.Uint32Ptr(10),
//	})
var Uint32Ptr = breaker.Uint32Ptr

// DurationPtr returns a pointer to the given time.Duration value.
// Helper function for constructing SettingsUpdate with explicit values.
//
// Example:
//
//	breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    Timeout: autobreaker.DurationPtr(30 * time.Second),
//	})
var DurationPtr = breaker.DurationPtr

// Float64Ptr returns a pointer to the given float64 value.
// Helper function for constructing SettingsUpdate with explicit values.
//
// Example:
//
//	breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    FailureRateThreshold: autobreaker.Float64Ptr(0.10),
//	})
var Float64Ptr = breaker.Float64Ptr
