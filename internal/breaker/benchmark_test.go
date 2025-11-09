package breaker

import (
	"errors"
	"testing"
)

// Benchmark helpers
var (
	benchResult interface{}
	benchError  error
)

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

// Performance targets (documented for Phase 1 implementation validation):
//
// - State check:      < 10 ns/op, 0 allocs/op
// - Counts snapshot:  < 50 ns/op, 0 allocs/op
// - Execute success:  < 100 ns/op overhead (vs baseline), 0 allocs/op
// - Execute open:     < 50 ns/op (fast rejection), 0 allocs/op
// - Adaptive check:   < 20 ns/op, 0 allocs/op
// - Default check:    < 5 ns/op, 0 allocs/op
// - Concurrent:       Linear scalability with cores
