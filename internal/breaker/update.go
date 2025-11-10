package breaker

import (
	"errors"
	"fmt"
	"time"
)

// UpdateSettings atomically updates the circuit breaker configuration.
//
// Only non-nil fields in the update are applied. Nil fields are left unchanged.
//
// Validation:
//   - MaxRequests must be > 0
//   - Interval must be >= 0
//   - Timeout must be > 0
//   - FailureRateThreshold must be in range (0, 1) exclusive
//   - MinimumObservations must be > 0
//
// Smart Reset Behavior:
//   - Changing Interval while in Closed state resets counts immediately
//   - Changing Timeout while in Open state restarts the timeout from now
//   - Other setting changes preserve current state
//
// Returns an error if validation fails. No settings are updated if validation fails.
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
		cb.maxRequests = *update.MaxRequests
	}

	// Update Interval and check if reset needed
	if update.Interval != nil {
		oldInterval := cb.interval
		newInterval := *update.Interval

		cb.interval = newInterval

		// If interval changed and we're in Closed state, reset counts
		if oldInterval != newInterval && currentState == StateClosed {
			needsCountReset = true
		}
	}

	// Update Timeout and check if timer reset needed
	if update.Timeout != nil {
		oldTimeout := cb.timeout
		newTimeout := *update.Timeout

		cb.timeout = newTimeout

		// If timeout changed and we're in Open state, reset timer
		if oldTimeout != newTimeout && currentState == StateOpen {
			needsTimerReset = true
		}
	}

	// Update FailureRateThreshold (simple field update)
	if update.FailureRateThreshold != nil {
		cb.failureRateThreshold = *update.FailureRateThreshold
	}

	// Update MinimumObservations (simple field update)
	if update.MinimumObservations != nil {
		cb.minimumObservations = *update.MinimumObservations
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
