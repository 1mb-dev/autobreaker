package breaker

import (
	"testing"
	"time"
)

// TestExecuteDoesNotAllocate backs the documented claim; it excludes the log lines written
// when a callback panics or a counter saturates.
func TestExecuteDoesNotAllocate(t *testing.T) {
	ok := func() (interface{}, error) { return "result", nil }
	fail := func() (interface{}, error) { return nil, errBench }
	tripOnFirst := func(c Counts) bool { return c.ConsecutiveFailures >= 1 }

	// Each run gets a fresh breaker in the given state; building them is not measured.
	breakers := func(state State) []*CircuitBreaker {
		cbs := make([]*CircuitBreaker, 202) // AllocsPerRun makes runs+1 calls
		for i := range cbs {
			cbs[i] = New(Settings{Name: "allocs", ReadyToTrip: tripOnFirst})
			if state != StateClosed {
				_, _ = cbs[i].Execute(fail)
			}
			if state == StateHalfOpen {
				cbs[i].transitionToHalfOpen()
			}
		}
		return cbs
	}

	tests := []struct {
		name  string
		state State
		req   func() (interface{}, error)
	}{
		{"closed success", StateClosed, ok},
		{"closed failure that trips", StateClosed, fail},
		{"open rejection", StateOpen, ok},
		{"half-open probe success", StateHalfOpen, ok},
		{"half-open probe failure", StateHalfOpen, fail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cbs := breakers(tt.state)
			i := 0
			allocs := testing.AllocsPerRun(200, func() {
				_, _ = cbs[i].Execute(tt.req)
				i++
			})
			if allocs != 0 {
				t.Errorf("Execute allocates %v times per call, want 0", allocs)
			}
		})
	}

	t.Run("adaptive with interval", func(t *testing.T) {
		cb := New(Settings{Name: "allocs", AdaptiveThreshold: true, Interval: time.Hour})
		for _, req := range []func() (interface{}, error){ok, fail} {
			if allocs := testing.AllocsPerRun(10, func() { _, _ = cb.Execute(req) }); allocs != 0 {
				t.Errorf("Execute allocates %v times per call, want 0", allocs)
			}
		}
	})

	t.Run("timeout moves open to half-open inside Execute", func(t *testing.T) {
		cbs := make([]*CircuitBreaker, 202)
		for i := range cbs {
			cbs[i] = New(Settings{Name: "allocs", ReadyToTrip: tripOnFirst, Timeout: time.Nanosecond})
			_, _ = cbs[i].Execute(fail)
		}
		time.Sleep(time.Millisecond)
		i := 0
		allocs := testing.AllocsPerRun(200, func() {
			_, _ = cbs[i].Execute(ok)
			i++
		})
		if allocs != 0 {
			t.Errorf("Execute allocates %v times per call, want 0", allocs)
		}
		if got := cbs[0].State(); got != StateClosed {
			t.Errorf("state after probe = %v, want closed (Open -> HalfOpen -> Closed)", got)
		}
	})

	t.Run("closed failure, no trip", func(t *testing.T) {
		cb := New(Settings{Name: "allocs", ReadyToTrip: func(Counts) bool { return false }})
		if allocs := testing.AllocsPerRun(200, func() { _, _ = cb.Execute(fail) }); allocs != 0 {
			t.Errorf("Execute allocates %v times per call, want 0", allocs)
		}
	})
}
