package breaker

import (
	"context"
	"errors"
	"testing"
)

// Benchmark helpers
var (
	benchResult interface{}
	benchError  error
)

// testContext returns a background context for benchmarks.
func testContext(b *testing.B) context.Context {
	return context.Background()
}

// BenchmarkStateCheck measures the overhead of checking circuit breaker state.
func BenchmarkStateCheck(b *testing.B) {
	cb := New(Settings{Name: "bench"})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.State()
	}
}

// BenchmarkCountsSnapshot measures the overhead of getting counts snapshot.
func BenchmarkCountsSnapshot(b *testing.B) {
	cb := New(Settings{Name: "bench"})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.Counts()
	}
}

// BenchmarkExecuteSuccess measures overhead of successful operations.
func BenchmarkExecuteSuccess(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkExecuteFailure measures overhead when operations fail.
func BenchmarkExecuteFailure(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return nil, errors.New("failure")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkExecuteOpenState measures overhead when circuit is open (fast rejection).
func BenchmarkExecuteOpenState(b *testing.B) {
	cb := New(Settings{
		Name: "bench",
		ReadyToTrip: func(counts Counts) bool {
			return true // Always trip immediately
		},
	})

	// Trip the circuit by causing a failure
	operation := func() (interface{}, error) {
		return nil, errors.New("failure")
	}
	cb.Execute(operation) // Should open the circuit

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkAdaptiveReadyToTrip measures overhead of adaptive threshold calculation.
func BenchmarkAdaptiveReadyToTrip(b *testing.B) {
	cb := New(Settings{
		Name:                 "bench",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  20,
	})

	counts := Counts{
		Requests:      100,
		TotalFailures: 10,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.defaultAdaptiveReadyToTrip(counts)
	}
}

// BenchmarkDefaultReadyToTrip measures overhead of default threshold check.
func BenchmarkDefaultReadyToTrip(b *testing.B) {
	counts := Counts{
		ConsecutiveFailures: 6,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = DefaultReadyToTrip(counts)
	}
}

// BenchmarkConcurrentExecute measures performance under concurrent load.
func BenchmarkConcurrentExecute(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(operation)
		}
	})
}

// BenchmarkNew measures circuit breaker creation overhead.
func BenchmarkNew(b *testing.B) {
	settings := Settings{
		Name:    "bench",
		Timeout: 60,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = New(settings)
	}
}

// Comparison benchmarks (to be compared against sony/gobreaker in Phase 1)

// BenchmarkCompareBaseline provides baseline for operation without circuit breaker.
func BenchmarkCompareBaseline(b *testing.B) {
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = operation()
	}
}

// BenchmarkExecute_Closed measures Execute() performance in closed state (hot path).
func BenchmarkExecute_Closed(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkExecute_Open measures Execute() performance when circuit is open (fast-fail).
func BenchmarkExecute_Open(b *testing.B) {
	cb := New(Settings{
		Name: "bench",
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	})

	// Trip the circuit
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("error")
	})

	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkExecute_HalfOpen measures Execute() performance in half-open state.
func BenchmarkExecute_HalfOpen(b *testing.B) {
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create new breaker for each iteration to keep it in half-open
		cb := New(Settings{
			Name: "bench",
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
			Timeout: 0,
		})

		// Trip and immediately transition to half-open
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("error")
		})
		cb.transitionToHalfOpen()

		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkExecuteContext_Closed measures ExecuteContext() performance in closed state.
func BenchmarkExecuteContext_Closed(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchResult, benchError = cb.ExecuteContext(testContext(b), operation)
	}
}

// BenchmarkState measures State() accessor performance.
func BenchmarkState(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	var state State

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		state = cb.State()
	}
	_ = state
}

// BenchmarkCounts measures Counts() snapshot performance.
func BenchmarkCounts(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	var counts Counts

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		counts = cb.Counts()
	}
	_ = counts
}

// BenchmarkMetrics measures Metrics() collection performance.
func BenchmarkMetrics(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	var metrics Metrics

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		metrics = cb.Metrics()
	}
	_ = metrics
}

// BenchmarkDiagnostics measures Diagnostics() full diagnostic performance.
func BenchmarkDiagnostics(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	var diag Diagnostics

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		diag = cb.Diagnostics()
	}
	_ = diag
}

// BenchmarkUpdateSettings measures UpdateSettings() performance.
func BenchmarkUpdateSettings(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	update := SettingsUpdate{
		Timeout: DurationPtr(30),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.UpdateSettings(update)
	}
}

// BenchmarkUpdateSettings_Concurrent measures UpdateSettings() under concurrent load.
func BenchmarkUpdateSettings_Concurrent(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	update := SettingsUpdate{
		Timeout: DurationPtr(30),
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.UpdateSettings(update)
		}
	})
}

// BenchmarkHighThroughput measures performance with 1M operations.
func BenchmarkHighThroughput(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < 1000000; i++ {
		benchResult, benchError = cb.Execute(operation)
	}
}

// BenchmarkConcurrent_100Goroutines measures performance with 100 concurrent goroutines.
func BenchmarkConcurrent_100Goroutines(b *testing.B) {
	cb := New(Settings{Name: "bench"})
	operation := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(100)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(operation)
		}
	})
}

// BenchmarkStateTransitions measures performance with mixed state transitions.
func BenchmarkStateTransitions(b *testing.B) {
	operation := func() (interface{}, error) {
		return "result", nil
	}
	failOp := func() (interface{}, error) {
		return nil, errors.New("error")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cb := New(Settings{
			Name: "bench",
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
			Timeout: 0,
		})

		// Closed -> Open
		cb.Execute(failOp)
		cb.Execute(failOp)
		cb.Execute(failOp)

		// Open -> HalfOpen
		cb.transitionToHalfOpen()

		// HalfOpen -> Closed
		benchResult, benchError = cb.Execute(operation)
	}
}

// Performance targets (documented for v1.0.0 validation):
//
// - State():              < 5 ns/op, 0 allocs/op
// - Counts():             < 10 ns/op, 0 allocs/op
// - Metrics():            < 20 ns/op, 0 allocs/op
// - Diagnostics():        < 200 ns/op, 0 allocs/op
// - Execute (closed):     < 100 ns/op, 0 allocs/op
// - Execute (open):       < 50 ns/op, 0 allocs/op
// - ExecuteContext:       < 100 ns/op, 0 allocs/op
// - UpdateSettings:       < 100 ns/op, 0 allocs/op
// - Concurrent scaling:   Linear with cores
// - Zero allocations:     All hot paths
