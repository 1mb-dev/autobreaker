// Package autobreaker provides an adaptive circuit breaker for Go.
//
// AutoBreaker automatically adjusts failure thresholds based on traffic patterns,
// eliminating the need for manual tuning across different environments.
//
// Basic usage:
//
//	breaker := autobreaker.New(autobreaker.Settings{
//	    Name: "my-service",
//	})
//
//	result, err := breaker.Execute(func() (interface{}, error) {
//	    return externalService.Call()
//	})
package autobreaker

import "github.com/vnykmshr/autobreaker/internal/breaker"

// Re-export types
type (
	CircuitBreaker  = breaker.CircuitBreaker
	State           = breaker.State
	Counts          = breaker.Counts
	Settings        = breaker.Settings
	SettingsUpdate  = breaker.SettingsUpdate
	Metrics         = breaker.Metrics
	Diagnostics     = breaker.Diagnostics
)

// Re-export constants
const (
	StateClosed   = breaker.StateClosed
	StateOpen     = breaker.StateOpen
	StateHalfOpen = breaker.StateHalfOpen
)

// Re-export errors
var (
	ErrOpenState       = breaker.ErrOpenState
	ErrTooManyRequests = breaker.ErrTooManyRequests
)

// Re-export functions
var (
	New          = breaker.New
	Uint32Ptr    = breaker.Uint32Ptr
	DurationPtr  = breaker.DurationPtr
	Float64Ptr   = breaker.Float64Ptr
)
