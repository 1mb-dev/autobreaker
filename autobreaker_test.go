package autobreaker

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

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		wantName string
		wantState State
	}{
		{
			name:     "default settings",
			settings: Settings{Name: "test"},
			wantName: "test",
			wantState: StateClosed,
		},
		{
			name: "custom settings",
			settings: Settings{
				Name:        "custom",
				MaxRequests: 10,
				Timeout:     30 * time.Second,
			},
			wantName: "custom",
			wantState: StateClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.settings)

			if cb.Name() != tt.wantName {
				t.Errorf("Name() = %v, want %v", cb.Name(), tt.wantName)
			}

			if cb.State() != tt.wantState {
				t.Errorf("State() = %v, want %v", cb.State(), tt.wantState)
			}
		})
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultReadyToTrip(t *testing.T) {
	tests := []struct {
		name   string
		counts Counts
		want   bool
	}{
		{
			name:   "no failures",
			counts: Counts{ConsecutiveFailures: 0},
			want:   false,
		},
		{
			name:   "5 consecutive failures (not yet tripped)",
			counts: Counts{ConsecutiveFailures: 5},
			want:   false,
		},
		{
			name:   "6 consecutive failures (should trip)",
			counts: Counts{ConsecutiveFailures: 6},
			want:   true,
		},
		{
			name:   "10 consecutive failures (should trip)",
			counts: Counts{ConsecutiveFailures: 10},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultReadyToTrip(tt.counts); got != tt.want {
				t.Errorf("defaultReadyToTrip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdaptiveReadyToTrip(t *testing.T) {
	cb := New(Settings{
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10%
		MinimumObservations:  10,
	})

	tests := []struct {
		name   string
		counts Counts
		want   bool
	}{
		{
			name: "not enough observations",
			counts: Counts{
				Requests:       5,
				TotalFailures:  3,
			},
			want: false, // Below minimum observations
		},
		{
			name: "below threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  5,
			},
			want: false, // 5% failure rate < 10% threshold
		},
		{
			name: "at threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  10,
			},
			want: false, // 10% failure rate == 10% threshold (not >)
		},
		{
			name: "above threshold",
			counts: Counts{
				Requests:       100,
				TotalFailures:  11,
			},
			want: true, // 11% failure rate > 10% threshold
		},
		{
			name: "high failure rate",
			counts: Counts{
				Requests:       50,
				TotalFailures:  25,
			},
			want: true, // 50% failure rate > 10% threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cb.defaultAdaptiveReadyToTrip(tt.counts); got != tt.want {
				t.Errorf("defaultAdaptiveReadyToTrip() = %v, want %v for counts %+v", got, tt.want, tt.counts)
			}
		})
	}
}

func TestDefaultIsSuccessful(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error is success",
			err:  nil,
			want: true,
		},
		{
			name: "non-nil error is failure",
			err:  errors.New("test error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultIsSuccessful(tt.err); got != tt.want {
				t.Errorf("defaultIsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCircuitBreakerDefaults(t *testing.T) {
	cb := New(Settings{Name: "test"})

	// Test default max requests
	if cb.maxRequests != 1 {
		t.Errorf("default maxRequests = %v, want 1", cb.maxRequests)
	}

	// Test default timeout
	if cb.timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", cb.timeout)
	}

	// Test default state
	if cb.State() != StateClosed {
		t.Errorf("default state = %v, want Closed", cb.State())
	}

	// Test default counts
	counts := cb.Counts()
	if counts.Requests != 0 || counts.TotalFailures != 0 || counts.TotalSuccesses != 0 {
		t.Errorf("default counts = %+v, want all zeros", counts)
	}
}

func TestAdaptiveThresholdDefaults(t *testing.T) {
	cb := New(Settings{
		Name:              "test",
		AdaptiveThreshold: true,
	})

	// Test default failure rate threshold
	if cb.failureRateThreshold != 0.05 {
		t.Errorf("default failureRateThreshold = %v, want 0.05", cb.failureRateThreshold)
	}

	// Test default minimum observations
	if cb.minimumObservations != 20 {
		t.Errorf("default minimumObservations = %v, want 20", cb.minimumObservations)
	}
}

// Placeholder for state transition tests (Phase 1 implementation)
func TestStateTransitions(t *testing.T) {
	t.Skip("Phase 1: Implement state transition tests")
}

// Placeholder for concurrency tests (Phase 1 implementation)
func TestConcurrency(t *testing.T) {
	t.Skip("Phase 1: Implement concurrency tests with race detector")
}

// Placeholder for execute tests (Phase 1 implementation)
func TestExecute(t *testing.T) {
	t.Skip("Phase 1: Implement Execute() tests")
}
