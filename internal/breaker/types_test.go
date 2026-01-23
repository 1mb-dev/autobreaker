package breaker

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		settings  Settings
		wantName  string
		wantState State
	}{
		{
			name:      "default settings",
			settings:  Settings{Name: "test"},
			wantName:  "test",
			wantState: StateClosed,
		},
		{
			name: "custom settings",
			settings: Settings{
				Name:        "custom",
				MaxRequests: 10,
				Timeout:     30 * time.Second,
			},
			wantName:  "custom",
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

func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		settings    Settings
		shouldPanic bool
		panicMsg    string
	}{
		{
			name: "valid adaptive settings",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.05,
				MinimumObservations:  20,
			},
			shouldPanic: false,
		},
		{
			name: "adaptive with zero threshold (uses default)",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0, // Will default to 0.05
			},
			shouldPanic: false,
		},
		{
			name: "failure rate threshold too low",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 0.0,
			},
			shouldPanic: false, // 0 is OK, triggers default
		},
		{
			name: "failure rate threshold negative",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: -0.1,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1), got -0.1",
		},
		{
			name: "failure rate threshold equals 1",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 1.0,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1), got 1",
		},
		{
			name: "failure rate threshold above 1",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    true,
				FailureRateThreshold: 1.5,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: FailureRateThreshold must be in range (0, 1), got 1.5",
		},
		{
			name: "negative interval",
			settings: Settings{
				Name:     "test",
				Interval: -1 * time.Second,
			},
			shouldPanic: true,
			panicMsg:    "autobreaker: Interval cannot be negative, got -1s",
		},
		{
			name: "zero interval (valid)",
			settings: Settings{
				Name:     "test",
				Interval: 0,
			},
			shouldPanic: false,
		},
		{
			name: "non-adaptive with invalid threshold (ignored)",
			settings: Settings{
				Name:                 "test",
				AdaptiveThreshold:    false,
				FailureRateThreshold: 5.0, // Invalid but ignored since adaptive is false
			},
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic {
					if r == nil {
						t.Errorf("Expected panic with message containing %q, but no panic occurred", tt.panicMsg)
					} else {
						panicStr := fmt.Sprint(r)
						if panicStr != tt.panicMsg {
							t.Errorf("Expected panic message %q, got %q", tt.panicMsg, panicStr)
						}
					}
				} else {
					if r != nil {
						t.Errorf("Expected no panic, but got: %v", r)
					}
				}
			}()

			_ = New(tt.settings)
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
			if got := DefaultReadyToTrip(tt.counts); got != tt.want {
				t.Errorf("DefaultReadyToTrip() = %v, want %v", got, tt.want)
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
			if got := DefaultIsSuccessful(tt.err); got != tt.want {
				t.Errorf("DefaultIsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}
