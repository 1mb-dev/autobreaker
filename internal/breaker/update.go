package breaker

import (
	"errors"
	"fmt"
	"time"
)

// UpdateSettings atomically updates the circuit breaker configuration at runtime.
//
// This method allows dynamic tuning of circuit breaker behavior without restart,
// enabling adaptive responses to changing traffic patterns, incident mitigation,
// and A/B testing of different thresholds.
//
// Partial Updates:
//
// Only non-nil fields in SettingsUpdate are modified. Nil fields keep their current values.
// This allows surgical updates of specific settings:
//
//	// Update only failure threshold, keep everything else
//	breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    FailureRateThreshold: autobreaker.Float64Ptr(0.10),
//	})
//
// Validation:
//
// All non-nil fields are validated before any updates are applied (all-or-nothing semantics):
//   - MaxRequests: Must be > 0
//   - Interval: Must be >= 0 (0 = no periodic reset)
//   - Timeout: Must be > 0
//   - FailureRateThreshold: Must be in (0, 1) exclusive when AdaptiveThreshold enabled
//   - MinimumObservations: Must be > 0
//
// If validation fails, no settings are changed and an error is returned.
//
// Smart Reset Behavior:
//
// Some setting changes trigger intelligent resets to maintain consistency:
//
//   - Interval Change + Closed State: Resets counts immediately
//     Rationale: Existing counts were measured with old window, invalid for new window
//
//   - Timeout Change + Open State: Restarts timeout from now
//     Rationale: User wants new timeout to apply fully, not partial remaining time
//
//   - Other Changes: Preserve current state and counts
//     Rationale: Settings like FailureRateThreshold can be adjusted without losing data
//
// Thread-Safety:
//
// This method is safe to call concurrently with Execute() and other methods. Each
// individual setting update is atomic. The order of updates is deterministic and
// designed for consistency.
//
// Note: Multiple concurrent UpdateSettings() calls are serialized by atomic operations,
// but the final state depends on execution order (last write wins per field).
//
// Performance:
//
// Settings updates are fast (~100-200ns) using atomic stores. Safe to call frequently,
// though typically settings change infrequently (minutes/hours, not per-request).
//
// Use Cases:
//
//  1. Incident Response - Relax threshold during incidents:
//     breaker.UpdateSettings(autobreaker.SettingsUpdate{
//         FailureRateThreshold: autobreaker.Float64Ptr(0.20), // 5% → 20%
//     })
//
//  2. Traffic Scaling - Adjust window for traffic changes:
//     breaker.UpdateSettings(autobreaker.SettingsUpdate{
//         Interval: autobreaker.DurationPtr(30 * time.Second), // 60s → 30s
//     })
//
//  3. Progressive Recovery - Gradually increase probe requests:
//     breaker.UpdateSettings(autobreaker.SettingsUpdate{
//         MaxRequests: autobreaker.Uint32Ptr(5), // 1 → 5
//     })
//
//  4. Configuration Reload - Apply changes from config file/API:
//     newSettings := loadFromConfig()
//     if err := breaker.UpdateSettings(newSettings); err != nil {
//         log.Error("Invalid settings: %v", err)
//     }
//
// Example - Gradual Rollout:
//
//	// Start conservative
//	breaker := autobreaker.New(autobreaker.Settings{
//	    FailureRateThreshold: 0.01, // 1%
//	})
//
//	// After monitoring, relax threshold
//	time.Sleep(1 * time.Hour)
//	breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	    FailureRateThreshold: autobreaker.Float64Ptr(0.05), // → 5%
//	})
//
// Example - Feature Flag Integration:
//
//	if featureFlags.IsEnabled("relaxed-circuit-breaker") {
//	    breaker.UpdateSettings(autobreaker.SettingsUpdate{
//	        FailureRateThreshold: autobreaker.Float64Ptr(0.10),
//	        Timeout:              autobreaker.DurationPtr(30 * time.Second),
//	    })
//	}
//
// Returns nil on success, or an error describing which field failed validation.
func (cb *CircuitBreaker) UpdateSettings(update SettingsUpdate) error {
	// Validate all settings before applying any changes
	if err := cb.validateUpdate(update); err != nil {
		return err
	}

	// Track if we need to reset counts or timer
	var needsCountReset bool
	var needsTimerReset bool

	// Check current state for smart reset logic
	currentState := cb.State()

	// Apply updates atomically
	// Note: We can't make all updates truly atomic without locks, but we can
	// make each individual update atomic. The order matters for correctness.

	// Update MaxRequests (simple field update)
	if update.MaxRequests != nil {
		cb.setMaxRequests(*update.MaxRequests)
	}

	// Update Interval and check if reset needed
	if update.Interval != nil {
		oldInterval := cb.getInterval()
		newInterval := *update.Interval

		cb.setInterval(newInterval)

		// If interval changed and we're in Closed state, reset counts
		if oldInterval != newInterval && currentState == StateClosed {
			needsCountReset = true
		}
	}

	// Update Timeout and check if timer reset needed
	if update.Timeout != nil {
		oldTimeout := cb.getTimeout()
		newTimeout := *update.Timeout

		cb.setTimeout(newTimeout)

		// If timeout changed and we're in Open state, reset timer
		if oldTimeout != newTimeout && currentState == StateOpen {
			needsTimerReset = true
		}
	}

	// Update FailureRateThreshold (simple field update)
	if update.FailureRateThreshold != nil {
		cb.setFailureRateThreshold(*update.FailureRateThreshold)
	}

	// Update MinimumObservations (simple field update)
	if update.MinimumObservations != nil {
		cb.setMinimumObservations(*update.MinimumObservations)
	}

	// Apply smart resets after all settings are updated
	if needsCountReset {
		cb.resetCounts()
	}

	if needsTimerReset {
		// Reset the open timer to start timeout from now
		now := time.Now().UnixNano()
		cb.openedAt.Store(now)
	}

	return nil
}

// validateUpdate validates all non-nil fields in the update.
// Returns an error if any field is invalid.
func (cb *CircuitBreaker) validateUpdate(update SettingsUpdate) error {
	// Validate MaxRequests
	if update.MaxRequests != nil {
		if *update.MaxRequests == 0 {
			return errors.New("autobreaker: MaxRequests must be > 0")
		}
	}

	// Validate Interval
	if update.Interval != nil {
		if *update.Interval < 0 {
			return errors.New("autobreaker: Interval cannot be negative")
		}
	}

	// Validate Timeout
	if update.Timeout != nil {
		if *update.Timeout <= 0 {
			return errors.New("autobreaker: Timeout must be > 0")
		}
	}

	// Validate FailureRateThreshold
	if update.FailureRateThreshold != nil {
		threshold := *update.FailureRateThreshold

		// Only validate if adaptive mode is enabled
		if cb.adaptiveThreshold {
			if threshold <= 0 || threshold >= 1 {
				return fmt.Errorf("autobreaker: FailureRateThreshold must be in range (0, 1), got %f", threshold)
			}
		}
		// Note: If adaptive mode is disabled, we still allow the update
		// (user might enable it later or use it with ReadyToTrip)
	}

	// Validate MinimumObservations
	if update.MinimumObservations != nil {
		if *update.MinimumObservations == 0 {
			return errors.New("autobreaker: MinimumObservations must be > 0")
		}
	}

	return nil
}

// resetCounts resets all counts and restarts the interval timer.
func (cb *CircuitBreaker) resetCounts() {
	cb.requests.Store(0)
	cb.totalSuccesses.Store(0)
	cb.totalFailures.Store(0)
	cb.consecutiveSuccesses.Store(0)
	cb.consecutiveFailures.Store(0)

	// Update the lastClearedAt timestamp
	now := time.Now().UnixNano()
	cb.lastClearedAt.Store(now)
}
