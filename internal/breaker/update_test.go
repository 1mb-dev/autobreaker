package breaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestUpdateSettings_MaxRequests(t *testing.T) {
	cb := New(Settings{
		Name:        "test",
		MaxRequests: 1,
	})

	// Update MaxRequests
	err := cb.UpdateSettings(SettingsUpdate{
		MaxRequests: Uint32Ptr(10),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getMaxRequests() != 10 {
		t.Errorf("Expected MaxRequests to be 10, got %d", cb.getMaxRequests())
	}
}

func TestUpdateSettings_Interval(t *testing.T) {
	cb := New(Settings{
		Name:     "test",
		Interval: 10 * time.Second,
	})

	// Update Interval
	newInterval := 30 * time.Second
	err := cb.UpdateSettings(SettingsUpdate{
		Interval: DurationPtr(newInterval),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getInterval() != newInterval {
		t.Errorf("Expected Interval to be %v, got %v", newInterval, cb.getInterval())
	}
}

func TestUpdateSettings_Timeout(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 10 * time.Second,
	})

	// Update Timeout
	newTimeout := 60 * time.Second
	err := cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(newTimeout),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getTimeout() != newTimeout {
		t.Errorf("Expected Timeout to be %v, got %v", newTimeout, cb.getTimeout())
	}
}

func TestUpdateSettings_FailureRateThreshold(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	// Update FailureRateThreshold
	newThreshold := 0.10
	err := cb.UpdateSettings(SettingsUpdate{
		FailureRateThreshold: Float64Ptr(newThreshold),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getFailureRateThreshold() != newThreshold {
		t.Errorf("Expected FailureRateThreshold to be %f, got %f", newThreshold, cb.getFailureRateThreshold())
	}
}

func TestUpdateSettings_MinimumObservations(t *testing.T) {
	cb := New(Settings{
		Name:                "test",
		AdaptiveThreshold:   true,
		MinimumObservations: 20,
	})

	// Update MinimumObservations
	newMinObs := uint32(50)
	err := cb.UpdateSettings(SettingsUpdate{
		MinimumObservations: Uint32Ptr(newMinObs),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getMinimumObservations() != newMinObs {
		t.Errorf("Expected MinimumObservations to be %d, got %d", newMinObs, cb.getMinimumObservations())
	}
}

func TestUpdateSettings_MultipleSettings(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		MaxRequests:          1,
		Timeout:              10 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	// Update multiple settings at once
	err := cb.UpdateSettings(SettingsUpdate{
		MaxRequests:          Uint32Ptr(5),
		Timeout:              DurationPtr(30 * time.Second),
		FailureRateThreshold: Float64Ptr(0.08),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if cb.getMaxRequests() != 5 {
		t.Errorf("Expected MaxRequests to be 5, got %d", cb.getMaxRequests())
	}
	if cb.getTimeout() != 30*time.Second {
		t.Errorf("Expected Timeout to be 30s, got %v", cb.getTimeout())
	}
	if cb.getFailureRateThreshold() != 0.08 {
		t.Errorf("Expected FailureRateThreshold to be 0.08, got %f", cb.getFailureRateThreshold())
	}
}

func TestUpdateSettings_NilFieldsNoChange(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		MaxRequests:          5,
		Timeout:              30 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	// Update only FailureRateThreshold, leave others unchanged
	err := cb.UpdateSettings(SettingsUpdate{
		FailureRateThreshold: Float64Ptr(0.10),
		// All other fields are nil, should not change
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Check that unchanged fields are still the same
	if cb.getMaxRequests() != 5 {
		t.Errorf("Expected MaxRequests to remain 5, got %d", cb.getMaxRequests())
	}
	if cb.getTimeout() != 30*time.Second {
		t.Errorf("Expected Timeout to remain 30s, got %v", cb.getTimeout())
	}
	// Check that changed field is updated
	if cb.getFailureRateThreshold() != 0.10 {
		t.Errorf("Expected FailureRateThreshold to be 0.10, got %f", cb.getFailureRateThreshold())
	}
}

func TestUpdateSettings_ValidationMaxRequests(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Try to set MaxRequests to 0 (invalid)
	err := cb.UpdateSettings(SettingsUpdate{
		MaxRequests: Uint32Ptr(0),
	})
	if err == nil {
		t.Fatal("Expected error for MaxRequests = 0, got nil")
	}

	// Verify settings were not changed
	if cb.getMaxRequests() == 0 {
		t.Error("MaxRequests should not have been updated")
	}
}

func TestUpdateSettings_ValidationInterval(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Try to set Interval to negative (invalid)
	err := cb.UpdateSettings(SettingsUpdate{
		Interval: DurationPtr(-1 * time.Second),
	})
	if err == nil {
		t.Fatal("Expected error for negative Interval, got nil")
	}
}

func TestUpdateSettings_ValidationTimeout(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Try to set Timeout to 0 (invalid)
	err := cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(0),
	})
	if err == nil {
		t.Fatal("Expected error for Timeout = 0, got nil")
	}

	// Try to set Timeout to negative (invalid)
	err = cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(-1 * time.Second),
	})
	if err == nil {
		t.Fatal("Expected error for negative Timeout, got nil")
	}
}

func TestUpdateSettings_ValidationFailureRateThreshold(t *testing.T) {
	cb := New(Settings{
		Name:              "test",
		AdaptiveThreshold: true,
	})

	tests := []struct {
		name      string
		threshold float64
		expectErr bool
	}{
		{"zero threshold", 0.0, true},
		{"negative threshold", -0.1, true},
		{"threshold = 1.0", 1.0, true},
		{"threshold > 1.0", 1.5, true},
		{"valid threshold 0.01", 0.01, false},
		{"valid threshold 0.5", 0.5, false},
		{"valid threshold 0.99", 0.99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cb.UpdateSettings(SettingsUpdate{
				FailureRateThreshold: Float64Ptr(tt.threshold),
			})

			if tt.expectErr && err == nil {
				t.Errorf("Expected error for threshold %f, got nil", tt.threshold)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for threshold %f, got: %v", tt.threshold, err)
			}
		})
	}
}

func TestUpdateSettings_ValidationMinimumObservations(t *testing.T) {
	cb := New(Settings{
		Name:              "test",
		AdaptiveThreshold: true,
	})

	// Try to set MinimumObservations to 0 (invalid)
	err := cb.UpdateSettings(SettingsUpdate{
		MinimumObservations: Uint32Ptr(0),
	})
	if err == nil {
		t.Fatal("Expected error for MinimumObservations = 0, got nil")
	}
}

func TestUpdateSettings_IntervalResetsCounts(t *testing.T) {
	cb := New(Settings{
		Name:     "test",
		Interval: 10 * time.Second,
	})

	// Generate some requests to populate counts
	for i := 0; i < 10; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, nil // success
		})
	}

	counts := cb.Counts()
	if counts.Requests != 10 {
		t.Fatalf("Expected 10 requests before update, got %d", counts.Requests)
	}

	// Update Interval - should reset counts in Closed state
	err := cb.UpdateSettings(SettingsUpdate{
		Interval: DurationPtr(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Verify counts were reset
	counts = cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("Expected counts to be reset after Interval change, got %d requests", counts.Requests)
	}
	if counts.TotalSuccesses != 0 {
		t.Errorf("Expected TotalSuccesses to be reset, got %d", counts.TotalSuccesses)
	}
}

func TestUpdateSettings_IntervalNoResetInOpenState(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		Interval:             10 * time.Second,
		Timeout:              100 * time.Millisecond,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.01, // Very sensitive
		MinimumObservations:  2,
	})

	// Trip the circuit
	for i := 0; i < 10; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected circuit to be Open, got %s", cb.State())
	}

	countsBefore := cb.Counts()

	// Update Interval while Open - should NOT reset counts
	err := cb.UpdateSettings(SettingsUpdate{
		Interval: DurationPtr(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	countsAfter := cb.Counts()
	if countsAfter.Requests != countsBefore.Requests {
		t.Errorf("Expected counts to remain unchanged in Open state, requests changed from %d to %d",
			countsBefore.Requests, countsAfter.Requests)
	}
}

func TestUpdateSettings_TimeoutResetsTimer(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		Timeout:              1 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.01, // Very sensitive
		MinimumObservations:  2,
	})

	// Trip the circuit
	for i := 0; i < 10; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected circuit to be Open, got %s", cb.State())
	}

	// Record when circuit opened
	openedAtBefore := cb.openedAt.Load()

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Update Timeout - should reset the timer
	err := cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Check that openedAt was updated (timer reset)
	openedAtAfter := cb.openedAt.Load()
	if openedAtAfter <= openedAtBefore {
		t.Errorf("Expected openedAt to be reset, but it was not updated")
	}

	// Verify timeout was changed
	if cb.getTimeout() != 2*time.Second {
		t.Errorf("Expected timeout to be 2s, got %v", cb.getTimeout())
	}
}

func TestUpdateSettings_TimeoutNoResetInClosedState(t *testing.T) {
	cb := New(Settings{
		Name:    "test",
		Timeout: 1 * time.Second,
	})

	// Circuit is Closed
	if cb.State() != StateClosed {
		t.Fatalf("Expected circuit to be Closed, got %s", cb.State())
	}

	openedAtBefore := cb.openedAt.Load()

	// Update Timeout while Closed - should NOT reset timer
	err := cb.UpdateSettings(SettingsUpdate{
		Timeout: DurationPtr(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	openedAtAfter := cb.openedAt.Load()
	if openedAtAfter != openedAtBefore {
		t.Errorf("Expected openedAt to remain unchanged in Closed state")
	}
}

func TestUpdateSettings_ConcurrentUpdates(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	// Run concurrent updates
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			threshold := 0.05 + float64(i%10)*0.01
			err := cb.UpdateSettings(SettingsUpdate{
				FailureRateThreshold: Float64Ptr(threshold),
			})
			if err != nil {
				t.Errorf("Concurrent update failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is valid
	threshold := cb.getFailureRateThreshold()
	if threshold <= 0 || threshold >= 1 {
		t.Errorf("Invalid final threshold: %f", threshold)
	}
}

func TestUpdateSettings_PreservesStateOnValidationError(t *testing.T) {
	cb := New(Settings{
		Name:                 "test",
		MaxRequests:          5,
		Timeout:              30 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	// Try to update with some valid and some invalid settings
	err := cb.UpdateSettings(SettingsUpdate{
		MaxRequests:          Uint32Ptr(10),                 // valid
		FailureRateThreshold: Float64Ptr(1.5),               // INVALID
		Timeout:              DurationPtr(60 * time.Second), // valid
	})

	// Should return error
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	// Original settings should be preserved (no partial updates)
	if cb.getMaxRequests() != 5 {
		t.Errorf("Expected MaxRequests to remain 5 after failed update, got %d", cb.getMaxRequests())
	}
	if cb.getTimeout() != 30*time.Second {
		t.Errorf("Expected Timeout to remain 30s after failed update, got %v", cb.getTimeout())
	}
	if cb.getFailureRateThreshold() != 0.05 {
		t.Errorf("Expected FailureRateThreshold to remain 0.05 after failed update, got %f", cb.getFailureRateThreshold())
	}
}

func TestUpdateSettings_CanSetZeroInterval(t *testing.T) {
	cb := New(Settings{
		Name:     "test",
		Interval: 10 * time.Second,
	})

	// Set Interval to 0 (valid - means no automatic reset)
	err := cb.UpdateSettings(SettingsUpdate{
		Interval: DurationPtr(0),
	})
	if err != nil {
		t.Fatalf("Expected no error for Interval = 0, got: %v", err)
	}

	if cb.getInterval() != 0 {
		t.Errorf("Expected Interval to be 0, got %v", cb.getInterval())
	}
}
