package breaker

import (
	"errors"
	"time"
)

// State represents the circuit breaker state.
//
// The circuit breaker implements a state machine with three states:
// Closed → Open → HalfOpen → Closed (or back to Open).
//
// Thread-safe: State values can be safely compared and returned from State() method.
type State int32

const (
	// StateClosed indicates normal operation.
	//
	// In this state:
	//   - All requests are allowed to pass through
	//   - Failures are counted against the threshold
	//   - When failure rate exceeds threshold, transitions to StateOpen
	//   - Counts reset periodically (if Interval is configured)
	//
	// This is the initial state when a circuit breaker is created.
	StateClosed State = iota

	// StateOpen indicates the circuit has tripped due to excessive failures.
	//
	// In this state:
	//   - All requests are rejected immediately with ErrOpenState (fail-fast)
	//   - No requests reach the backend (prevents cascading failures)
	//   - After Timeout duration, transitions to StateHalfOpen
	//   - Counts are preserved but not updated
	//
	// This state protects the backend from additional load while it recovers.
	StateOpen

	// StateHalfOpen indicates the circuit is testing recovery.
	//
	// In this state:
	//   - Limited concurrent requests are allowed (controlled by MaxRequests)
	//   - Excess concurrent requests are rejected with ErrTooManyRequests
	//   - If requests succeed, transitions to StateClosed (recovery confirmed)
	//   - If requests fail, transitions back to StateOpen (still unhealthy)
	//   - Counts track consecutive successes for recovery detection
	//
	// This state probes the backend to determine if it has recovered.
	StateHalfOpen
)

const (
	stateUnknownStr = "unknown"
)

// String returns the string representation of the state.
//
// Returns "closed", "open", "half-open", or "unknown" for invalid states.
// Useful for logging and debugging.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return stateUnknownStr
	}
}

// Counts holds circuit breaker statistics.
//
// Counts are returned by the Counts() method and also embedded in Metrics.
// These counts represent the current observation window (defined by Interval setting).
//
// Counter Saturation:
// All counters use uint32 and saturate at math.MaxUint32 (4,294,967,295). Once a counter
// reaches this maximum value, it stops incrementing to prevent undefined overflow behavior.
// This is sufficient for most applications. If your service processes more than 4 billion
// requests and needs accurate statistics beyond that point, consider:
// 1. Using the Interval setting to periodically reset counts
// 2. Monitoring for saturation events
// 3. Creating a new circuit breaker instance
//
// Saturation Events:
// When a counter reaches saturation, a warning is logged to stdout:
//
//	[AUTOBREAKER WARNING] Circuit "circuit-name": requests counter saturated at 4294967295 (max uint32)
//
// This helps operators identify when counters are no longer accurate due to saturation.
//
// Thread-safe: Counts is a snapshot taken atomically.
type Counts struct {
	// Requests is the total number of requests in the current observation window.
	// Includes both successes and failures.
	// Resets when Interval expires (if configured) or on state transitions.
	Requests uint32

	// TotalSuccesses is the cumulative count of successful requests in the current window.
	// A request is successful if IsSuccessful(err) returns true.
	TotalSuccesses uint32

	// TotalFailures is the cumulative count of failed requests in the current window.
	// A request fails if IsSuccessful(err) returns false.
	TotalFailures uint32

	// ConsecutiveSuccesses is the number of consecutive successes since the last failure.
	// Resets to 0 on any failure.
	// Used in half-open state to determine when to close the circuit.
	ConsecutiveSuccesses uint32

	// ConsecutiveFailures is the number of consecutive failures since the last success.
	// Resets to 0 on any success.
	// Used by default ReadyToTrip (trips after 5 consecutive failures).
	ConsecutiveFailures uint32
}

// Settings configures a circuit breaker.
//
// Settings defines the behavior and thresholds for a CircuitBreaker instance.
// Pass Settings to New() to create a configured circuit breaker.
//
// Configuration Categories:
//
//  1. Identity:
//     - Name: Circuit breaker identifier for logging/metrics
//
//  2. State Timing:
//     - Timeout: How long to stay open before probing (open → half-open)
//     - Interval: How often to reset counts in closed state (0 = never)
//
//  3. Request Limiting:
//     - MaxRequests: Concurrent request limit in half-open state
//
//  4. Failure Detection (choose one):
//     - Static: Use ReadyToTrip with ConsecutiveFailures threshold
//     - Adaptive: Use AdaptiveThreshold + FailureRateThreshold + MinimumObservations
//
//  5. Callbacks:
//     - ReadyToTrip: Custom failure detection logic
//     - OnStateChange: State transition notifications
//     - IsSuccessful: Custom success/failure determination
//
// Two Main Patterns:
//
//	A. Static Threshold (Simple, Predictable):
//	   Settings{
//	       Name:    "api-client",
//	       Timeout: 10 * time.Second,
//	       // Uses default ReadyToTrip: ConsecutiveFailures > 5
//	   }
//
//	B. Adaptive Threshold (Traffic-Proportional):
//	   Settings{
//	       Name:                 "api-client",
//	       Timeout:              10 * time.Second,
//	       AdaptiveThreshold:    true,
//	       FailureRateThreshold: 0.05,  // 5% failure rate
//	       MinimumObservations:  20,    // Need 20 requests before adapting
//	   }
//
// Defaults:
//
// If you don't specify a setting, sensible defaults are applied:
//   - MaxRequests: 1 (limit half-open probes to 1 concurrent request)
//   - Timeout: 60 seconds (wait 1 minute before probing)
//   - Interval: 0 (counts never reset, only on state transitions)
//   - ReadyToTrip: DefaultReadyToTrip or adaptive logic (based on AdaptiveThreshold)
//   - IsSuccessful: DefaultIsSuccessful (err == nil)
//   - OnStateChange: nil (no callback)
//   - AdaptiveThreshold: false (use static threshold)
//   - FailureRateThreshold: 0.05 (5%) when AdaptiveThreshold=true
//   - MinimumObservations: 20 when AdaptiveThreshold=true
//
// Validation:
//
// New() validates settings and panics on invalid configuration:
//   - FailureRateThreshold: Must be in (0, 1) exclusive when AdaptiveThreshold=true
//   - Interval: Must be >= 0 (negative values invalid)
//
// All other fields have sensible defaults and don't require validation.
//
// Thread-Safety Note:
//
// Callbacks (ReadyToTrip, OnStateChange, IsSuccessful) must be thread-safe
// as they're called concurrently without synchronization.
type Settings struct {
	// Name is an identifier for the circuit breaker.
	Name string

	// MaxRequests is the maximum number of concurrent requests allowed in half-open state.
	// Default: 1 if set to 0.
	MaxRequests uint32

	// Interval is the period to clear counts in closed state.
	//
	// Valid range: >= 0 (negative values will panic)
	// Default: 0 (counts are cleared only on state transitions)
	// Common values: 60s for time-based windows, 0 for event-based
	Interval time.Duration

	// Timeout is the duration to wait before transitioning from open to half-open.
	//
	// Valid range: > 0 recommended
	// Default: 60 seconds if set to 0
	// Common values: 10s-120s depending on service recovery time
	Timeout time.Duration

	// ReadyToTrip is called when counts are updated in Closed state after each request.
	// If it returns true, the circuit breaker transitions from Closed to Open (trips).
	//
	// This callback defines the failure detection policy. It receives current counts
	// and should return true when the failure threshold is exceeded.
	//
	// Default Implementation:
	//   - When AdaptiveThreshold=false: Uses DefaultReadyToTrip (ConsecutiveFailures > 5)
	//   - When AdaptiveThreshold=true: Uses adaptive logic (FailureRate > threshold && Requests >= MinimumObservations)
	//
	// Custom implementations can use any logic based on Counts fields:
	//   - ConsecutiveFailures: For streak-based detection
	//   - TotalFailures: For absolute count thresholds
	//   - FailureRate: For percentage-based detection (compute: TotalFailures/Requests)
	//   - Requests: For minimum observation requirements
	//
	// Thread-Safety: This callback must be thread-safe as it's called concurrently
	// from Execute() without synchronization.
	//
	// Performance: Keep this callback fast (<1μs) as it's called on every request
	// in Closed state. Avoid I/O, logging, or expensive computations.
	//
	// Example - Custom Threshold:
	//   ReadyToTrip: func(counts autobreaker.Counts) bool {
	//       // Trip after 3 consecutive failures
	//       return counts.ConsecutiveFailures >= 3
	//   }
	//
	// Example - Percentage with Minimum:
	//   ReadyToTrip: func(counts autobreaker.Counts) bool {
	//       if counts.Requests < 10 { return false }
	//       rate := float64(counts.TotalFailures) / float64(counts.Requests)
	//       return rate > 0.10 // 10% failure rate
	//   }
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the circuit breaker transitions between states.
	// It receives the circuit name, previous state, and new state.
	//
	// This callback is useful for:
	//   - Logging state transitions for debugging
	//   - Emitting metrics to monitoring systems
	//   - Triggering alerts when circuit opens
	//   - Coordinating with other systems (e.g., auto-scaling)
	//
	// Default: nil (no callback, state changes are silent)
	//
	// State Transitions:
	//   - Closed → Open: Circuit has tripped due to failures (ReadyToTrip returned true)
	//   - Open → HalfOpen: Timeout expired, circuit is probing for recovery
	//   - HalfOpen → Closed: Probe requests succeeded, circuit has recovered
	//   - HalfOpen → Open: Probe requests failed, backend still unhealthy
	//
	// Important: This callback is invoked AFTER counts are cleared. If you need
	// pre-transition counts (e.g., to log "tripped after N failures"), capture
	// them in your ReadyToTrip callback instead, which runs before the transition.
	//
	// Thread-Safety: This callback must be thread-safe. It may be called concurrently
	// from multiple goroutines during state transitions.
	//
	// Performance: Avoid blocking operations in this callback. If you need to perform
	// I/O (logging, metrics), do it asynchronously:
	//   OnStateChange: func(name string, from, to autobreaker.State) {
	//       go logStateChange(name, from, to) // Non-blocking
	//   }
	//
	// Example - Structured Logging:
	//   OnStateChange: func(name string, from, to autobreaker.State) {
	//       log.Info("circuit %s: %s → %s", name, from, to)
	//   }
	//
	// Example - Metrics:
	//   OnStateChange: func(name string, from, to autobreaker.State) {
	//       metrics.Increment("circuit_breaker.state_change",
	//           "name", name, "from", from.String(), "to", to.String())
	//   }
	//
	// Example - Alerting:
	//   OnStateChange: func(name string, from, to autobreaker.State) {
	//       if to == autobreaker.StateOpen {
	//           go alerter.Send("Circuit %s has opened!", name)
	//       }
	//   }
	OnStateChange func(name string, from State, to State)

	// IsSuccessful determines whether an error should be counted as success or failure.
	// It receives the error returned by the request function passed to Execute().
	//
	// This callback defines the success criteria. Use it to customize which errors
	// count as failures vs benign errors that shouldn't trip the circuit.
	//
	// Default: DefaultIsSuccessful (returns true only when err == nil)
	//
	// Common Patterns:
	//
	//   - Ignore Client Errors (HTTP 4xx):
	//     IsSuccessful: func(err error) bool {
	//         return err == nil || isClientError(err) // 4xx not our fault
	//     }
	//
	//   - Ignore Specific Errors:
	//     IsSuccessful: func(err error) bool {
	//         return err == nil || errors.Is(err, ErrNotFound)
	//     }
	//
	//   - Only Count Critical Errors:
	//     IsSuccessful: func(err error) bool {
	//         return !isCriticalError(err) // Only trip on critical failures
	//     }
	//
	// Thread-Safety: This callback must be thread-safe as it's called concurrently
	// from Execute() without synchronization.
	//
	// Performance: Keep this callback fast (<1μs) as it's called on every request.
	// Avoid I/O, logging, or expensive computations.
	//
	// Note: Panics are always counted as failures, regardless of this callback.
	//
	// Example - HTTP Client:
	//   IsSuccessful: func(err error) bool {
	//       if err == nil { return true }
	//       var httpErr *HTTPError
	//       if errors.As(err, &httpErr) {
	//           // Only 5xx server errors count as failures
	//           return httpErr.StatusCode < 500
	//       }
	//       return false
	//   }
	IsSuccessful func(err error) bool

	// --- Adaptive Settings (AutoBreaker Extensions) ---

	// AdaptiveThreshold enables percentage-based failure thresholds.
	// When true, the circuit breaker uses failure rate (percentage) instead of absolute failure counts.
	// This makes the same configuration work across different traffic levels.
	//
	// Example: With 5% threshold:
	//   Production (1000 req/s): Trips at 50 failures/s
	//   Staging (10 req/s):      Trips at 0.5 failures/s
	//   Dev (1 req/min):         Trips at 0.05 failures/min
	//
	// Default: false (uses static ConsecutiveFailures > 5 for backward compatibility)
	AdaptiveThreshold bool

	// FailureRateThreshold is the failure rate (0.0-1.0) that triggers circuit open.
	// Only used when AdaptiveThreshold is true.
	//
	// Valid range: (0, 1) exclusive - values outside this range will panic
	// Default: 0.05 (5% failure rate) if set to 0
	// Recommended values:
	//   0.01 (1%)  = Strict, for critical services with low error tolerance
	//   0.05 (5%)  = Balanced, good default for most services
	//   0.10 (10%) = Lenient, for services with higher acceptable error rates
	//
	// The threshold is traffic-proportional: it works equally well at any request rate.
	FailureRateThreshold float64

	// MinimumObservations is the minimum number of requests before adaptive logic activates.
	// Prevents false positives during low traffic periods.
	// Only used when AdaptiveThreshold is true.
	//
	// Valid range: > 0 recommended (though 0 will use default)
	// Default: 20 requests if set to 0
	// Recommended values:
	//   10-20   = For high-traffic services with quick feedback
	//   20-50   = Balanced, prevents premature tripping
	//   50-100  = For services where you want high confidence before tripping
	//
	// Example: With MinimumObservations=20 and FailureRateThreshold=0.05:
	//   First 19 requests: Circuit won't trip regardless of failure rate
	//   20+ requests: Circuit trips if failure rate exceeds 5%
	MinimumObservations uint32
}

var (
	// ErrOpenState is returned when the circuit breaker is open.
	ErrOpenState = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests are attempted in half-open state.
	ErrTooManyRequests = errors.New("too many requests")
)

// DefaultReadyToTrip returns true after 5 consecutive failures.
//
// This is the default ReadyToTrip implementation when AdaptiveThreshold is false (or not set).
// It uses a simple consecutive failure threshold that's easy to understand and reason about.
//
// Logic: trips when ConsecutiveFailures > 5
//
// Characteristics:
//   - Simple and predictable
//   - Works well for stable traffic patterns
//   - Independent of request volume
//   - Good for services with known failure patterns
//
// Limitations:
//   - May be too sensitive at high traffic (6 failures might be <1% error rate)
//   - May be too slow at low traffic (6 failures might be 100% error rate)
//   - Requires tuning for different environments (dev/staging/prod)
//
// When to Use:
//   - You have stable, predictable traffic
//   - You know the acceptable failure count for your service
//   - You want simple, easy-to-understand behavior
//   - You're migrating from sony/gobreaker (compatible behavior)
//
// When to Use Adaptive Instead:
//   - Traffic varies significantly (10x+ variance)
//   - You want same config across environments
//   - You think in terms of error rates (%) not counts
//   - You want automatic scaling with traffic
//
// Example - Using Default:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "api-client",
//	    // ReadyToTrip: nil  // Uses DefaultReadyToTrip
//	    // Trips after 6 consecutive failures
//	})
//
// Example - Custom Threshold:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "critical-service",
//	    ReadyToTrip: func(counts autobreaker.Counts) bool {
//	        return counts.ConsecutiveFailures >= 3 // More sensitive
//	    },
//	})
func DefaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// DefaultIsSuccessful returns true only for nil errors.
//
// This is the default IsSuccessful implementation. It treats any non-nil error
// as a failure that counts toward tripping the circuit.
//
// Logic: returns err == nil
//
// Characteristics:
//   - Conservative: All errors count as failures
//   - Simple and predictable
//   - Safe default for most services
//   - No special error handling needed
//
// When to Use Default:
//   - All errors indicate backend problems
//   - You want conservative failure detection
//   - Your service doesn't return benign errors
//   - Simple is better than clever
//
// When to Customize:
//   - You need to distinguish client errors (4xx) from server errors (5xx)
//   - Certain errors are expected and shouldn't trip circuit (e.g., NotFound)
//   - You want to ignore transient errors (e.g., context cancellation)
//   - You have domain-specific success/failure criteria
//
// Example - Using Default:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "database",
//	    // IsSuccessful: nil  // Uses DefaultIsSuccessful
//	    // Any error counts as failure
//	})
//
// Example - Ignore Client Errors:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "http-client",
//	    IsSuccessful: func(err error) bool {
//	        if err == nil { return true }
//	        // 4xx errors are client's fault, don't count as backend failure
//	        return isClientError(err)
//	    },
//	})
//
// Example - Ignore Specific Errors:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "user-service",
//	    IsSuccessful: func(err error) bool {
//	        // NotFound is expected, not a failure
//	        return err == nil || errors.Is(err, ErrNotFound)
//	    },
//	})
func DefaultIsSuccessful(err error) bool {
	return err == nil
}

// SettingsUpdate holds optional configuration updates for a circuit breaker.
//
// Used with UpdateSettings() to modify circuit breaker configuration at runtime
// without restarting the application.
//
// Pointer Semantics:
//   - nil fields: Setting will not be updated (keeps current value)
//   - non-nil fields: Setting will be updated to the pointed value
//
// This allows partial updates where only specific settings change:
//
//	err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    FailureRateThreshold: autobreaker.Float64Ptr(0.10), // Update this
//	    // All other fields are nil, so they keep their current values
//	})
//
// All updates are validated before being applied. If validation fails, no settings
// are changed (all-or-nothing semantics).
//
// Smart Reset Behavior:
//   - Changing Interval in Closed state resets counts (old counts invalid with new window)
//   - Changing Timeout while Open restarts timeout from now (new timeout applies fully)
//   - Other changes preserve current state and counts
//
// Thread-safe: UpdateSettings() can be called concurrently with Execute().
type SettingsUpdate struct {
	// MaxRequests updates the maximum number of concurrent requests allowed in half-open state.
	// Valid range: > 0 (will be validated)
	MaxRequests *uint32

	// Interval updates the period to clear counts in closed state.
	// Valid range: >= 0
	// Note: Changing interval will reset counts immediately to maintain accuracy.
	Interval *time.Duration

	// Timeout updates the duration to wait before transitioning from open to half-open.
	// Valid range: > 0 (will be validated)
	// Note: Changing timeout while circuit is open will restart the timeout from now.
	Timeout *time.Duration

	// FailureRateThreshold updates the failure rate (0.0-1.0) that triggers circuit open.
	// Only applies when adaptive threshold is enabled.
	// Valid range: (0, 1) exclusive
	FailureRateThreshold *float64

	// MinimumObservations updates the minimum number of requests before adaptive logic activates.
	// Only applies when adaptive threshold is enabled.
	// Valid range: > 0 (will be validated)
	MinimumObservations *uint32
}

// Uint32Ptr returns a pointer to the given uint32 value.
// Helper function for constructing SettingsUpdate.
func Uint32Ptr(v uint32) *uint32 {
	return &v
}

// DurationPtr returns a pointer to the given time.Duration value.
// Helper function for constructing SettingsUpdate.
func DurationPtr(v time.Duration) *time.Duration {
	return &v
}

// Float64Ptr returns a pointer to the given float64 value.
// Helper function for constructing SettingsUpdate.
func Float64Ptr(v float64) *float64 {
	return &v
}
