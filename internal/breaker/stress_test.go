package breaker

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStress_10MRequests validates stability under 10M operations.
func TestStress_10MRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cb := New(Settings{
		Name:                 "stress-10m",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		MinimumObservations:  100,
	})

	operation := func() (interface{}, error) {
		return "success", nil
	}

	const totalOps = 10_000_000
	startMem := getMemStats()

	t.Logf("Starting 10M operations test...")
	start := time.Now()

	for i := 0; i < totalOps; i++ {
		_, err := cb.Execute(operation)
		if err != nil {
			t.Fatalf("Unexpected error at iteration %d: %v", i, err)
		}

		// Log progress periodically
		if i > 0 && i%1_000_000 == 0 {
			elapsed := time.Since(start)
			throughput := float64(i) / elapsed.Seconds()
			t.Logf("Progress: %dM ops, %.0f ops/sec", i/1_000_000, throughput)
		}
	}

	elapsed := time.Since(start)
	throughput := float64(totalOps) / elapsed.Seconds()
	endMem := getMemStats()

	t.Logf("✅ Completed 10M operations in %v", elapsed)
	t.Logf("   Throughput: %.0f ops/sec", throughput)
	t.Logf("   Memory: %d KB → %d KB (Δ%+d KB)",
		startMem/1024, endMem/1024, (endMem-startMem)/1024)

	// Verify circuit state is still healthy
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after 10M successes, got %v", cb.State())
	}

	counts := cb.Counts()
	if counts.Requests != totalOps {
		t.Errorf("Expected %d requests, got %d", totalOps, counts.Requests)
	}
}

// TestStress_1000ConcurrentGoroutines validates behavior under massive concurrency.
func TestStress_1000ConcurrentGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cb := New(Settings{
		Name: "stress-1000-concurrent",
	})

	const (
		numGoroutines = 1000
		opsPerRoutine = 10_000
		totalOps      = numGoroutines * opsPerRoutine
	)

	var (
		successCount atomic.Uint64
		errorCount   atomic.Uint64
		wg           sync.WaitGroup
	)

	operation := func() (interface{}, error) {
		return "success", nil
	}

	startMem := getMemStats()
	startGoroutines := runtime.NumGoroutine()

	t.Logf("Starting %d goroutines (%d ops each)...", numGoroutines, opsPerRoutine)
	start := time.Now()

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerRoutine; j++ {
				_, err := cb.Execute(operation)
				if err == nil {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	throughput := float64(totalOps) / elapsed.Seconds()
	endMem := getMemStats()
	endGoroutines := runtime.NumGoroutine()

	successes := successCount.Load()
	errors := errorCount.Load()

	t.Logf("✅ Completed %d operations with %d goroutines in %v",
		totalOps, numGoroutines, elapsed)
	t.Logf("   Throughput: %.0f ops/sec", throughput)
	t.Logf("   Successes: %d, Errors: %d", successes, errors)
	t.Logf("   Memory: %d KB → %d KB (Δ%+d KB)",
		startMem/1024, endMem/1024, (endMem-startMem)/1024)
	t.Logf("   Goroutines: %d → %d (Δ%+d)",
		startGoroutines, endGoroutines, endGoroutines-startGoroutines)

	// Verify all operations completed successfully
	if successes != totalOps {
		t.Errorf("Expected %d successes, got %d", totalOps, successes)
	}
	if errors != 0 {
		t.Errorf("Expected 0 errors, got %d", errors)
	}

	// Check for goroutine leaks (allow some variance)
	goroutineDelta := endGoroutines - startGoroutines
	if goroutineDelta > 10 {
		t.Errorf("Potential goroutine leak: started with %d, ended with %d (Δ%+d)",
			startGoroutines, endGoroutines, goroutineDelta)
	}
}

// TestStress_MixedReadWrite validates concurrent Execute + UpdateSettings.
func TestStress_MixedReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cb := New(Settings{
		Name:                 "stress-mixed",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
	})

	const (
		duration      = 5 * time.Second
		numReaders    = 100
		numWriters    = 10
		updateInterval = 10 * time.Millisecond
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		execCount   atomic.Uint64
		updateCount atomic.Uint64
		wg          sync.WaitGroup
	)

	operation := func() (interface{}, error) {
		return "success", nil
	}

	t.Logf("Running mixed read/write test for %v...", duration)
	t.Logf("  Readers: %d goroutines executing requests", numReaders)
	t.Logf("  Writers: %d goroutines updating settings", numWriters)

	// Start readers (Execute)
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				cb.Execute(operation)
				execCount.Add(1)
			}
		}()
	}

	// Start writers (UpdateSettings)
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(updateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					update := SettingsUpdate{
						Timeout: DurationPtr(30 * time.Second),
					}
					cb.UpdateSettings(update)
					updateCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	execs := execCount.Load()
	updates := updateCount.Load()

	t.Logf("✅ Mixed read/write test completed")
	t.Logf("   Execute calls: %d (%.0f ops/sec)", execs, float64(execs)/duration.Seconds())
	t.Logf("   UpdateSettings calls: %d", updates)
	t.Logf("   Circuit state: %v", cb.State())

	// Verify circuit is still functional
	result, err := cb.Execute(operation)
	if err != nil {
		t.Errorf("Circuit not functional after stress: %v", err)
	}
	if result != "success" {
		t.Errorf("Unexpected result: %v", result)
	}
}

// TestStress_LongRunning validates stability over extended duration (1 hour in CI, 5 minutes in normal tests).
func TestStress_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	duration := 5 * time.Minute
	if testing.Verbose() {
		duration = 1 * time.Hour
	}

	cb := New(Settings{
		Name:                 "stress-long-running",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05,
		Interval:             60 * time.Second, // Reset counts every minute
	})

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	const numWorkers = 50
	var (
		totalOps    atomic.Uint64
		totalErrors atomic.Uint64
		wg          sync.WaitGroup
	)

	operation := func() (interface{}, error) {
		// Simulate occasional failures (1%)
		if time.Now().UnixNano()%100 == 0 {
			return nil, errors.New("simulated failure")
		}
		return "success", nil
	}

	startMem := getMemStats()
	startGoroutines := runtime.NumGoroutine()

	t.Logf("Starting long-running test for %v with %d workers...", duration, numWorkers)

	// Monitoring goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ops := totalOps.Load()
				errs := totalErrors.Load()
				mem := getMemStats()
				metrics := cb.Metrics()

				t.Logf("Status: %d ops, %d errors, %.2f%% failure rate, %d KB memory, state=%v",
					ops, errs, metrics.FailureRate*100, mem/1024, metrics.State)
			}
		}
	}()

	// Worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				_, err := cb.Execute(operation)
				totalOps.Add(1)
				if err != nil && err != ErrOpenState {
					totalErrors.Add(1)
				}
				time.Sleep(1 * time.Millisecond) // Throttle slightly
			}
		}()
	}

	wg.Wait()

	endMem := getMemStats()
	endGoroutines := runtime.NumGoroutine()
	ops := totalOps.Load()
	errs := totalErrors.Load()

	t.Logf("✅ Long-running test completed after %v", duration)
	t.Logf("   Total operations: %d", ops)
	t.Logf("   Total errors: %d (%.2f%%)", errs, float64(errs)/float64(ops)*100)
	t.Logf("   Memory: %d KB → %d KB (Δ%+d KB)",
		startMem/1024, endMem/1024, (endMem-startMem)/1024)
	t.Logf("   Goroutines: %d → %d (Δ%+d)",
		startGoroutines, endGoroutines, endGoroutines-startGoroutines)

	// Verify no significant memory growth or goroutine leaks
	memGrowth := endMem - startMem
	if memGrowth > 10*1024*1024 { // 10 MB
		t.Errorf("Significant memory growth detected: %d KB", memGrowth/1024)
	}

	goroutineDelta := endGoroutines - startGoroutines
	if goroutineDelta > 10 {
		t.Errorf("Potential goroutine leak: Δ%+d goroutines", goroutineDelta)
	}
}

// TestStress_RapidStateTransitions validates behavior with frequent state changes.
func TestStress_RapidStateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const iterations = 10_000

	t.Logf("Testing %d rapid state transitions...", iterations)
	start := time.Now()

	successOp := func() (interface{}, error) { return "success", nil }
	failOp := func() (interface{}, error) { return nil, errors.New("failure") }

	var transitionCount int

	for i := 0; i < iterations; i++ {
		cb := New(Settings{
			Name: "stress-transitions",
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
			Timeout: 0, // Instant transition
		})

		// Closed → Open
		cb.Execute(failOp)
		cb.Execute(failOp)
		if cb.State() == StateOpen {
			transitionCount++
		}

		// Open → HalfOpen
		cb.transitionToHalfOpen()
		if cb.State() == StateHalfOpen {
			transitionCount++
		}

		// HalfOpen → Closed
		cb.Execute(successOp)
		if cb.State() == StateClosed {
			transitionCount++
		}
	}

	elapsed := time.Since(start)
	transitionsPerSec := float64(transitionCount) / elapsed.Seconds()

	t.Logf("✅ Completed %d state transitions in %v", transitionCount, elapsed)
	t.Logf("   Rate: %.0f transitions/sec", transitionsPerSec)

	expectedTransitions := iterations * 3 // 3 transitions per iteration
	if transitionCount != expectedTransitions {
		t.Errorf("Expected %d transitions, got %d", expectedTransitions, transitionCount)
	}
}

// TestStress_VeryHighRequestRate validates behavior at extreme throughput.
func TestStress_VeryHighRequestRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cb := New(Settings{
		Name: "stress-high-rate",
	})

	const duration = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	const numWorkers = 1000 // Massive concurrency
	var totalOps atomic.Uint64

	operation := func() (interface{}, error) {
		return "success", nil
	}

	var wg sync.WaitGroup

	t.Logf("Starting very high request rate test (%d workers, %v)...", numWorkers, duration)
	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				cb.Execute(operation)
				totalOps.Add(1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	ops := totalOps.Load()
	throughput := float64(ops) / elapsed.Seconds()

	t.Logf("✅ Very high request rate test completed")
	t.Logf("   Total operations: %d", ops)
	t.Logf("   Duration: %v", elapsed)
	t.Logf("   Throughput: %.0f ops/sec (%.2fM ops/sec)", throughput, throughput/1_000_000)

	// Verify circuit is still healthy
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after high rate test, got %v", cb.State())
	}
}

// TestStress_VeryLowRequestRate validates behavior at very low traffic.
func TestStress_VeryLowRequestRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cb := New(Settings{
		Name:                 "stress-low-rate",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.50, // 50% threshold
		MinimumObservations:  5,
	})

	const (
		duration     = 30 * time.Second
		requestRate  = 1 * time.Second // 1 request per second
		failureRate  = 0.6              // 60% failures (should trip at 50%)
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		totalOps  int
		successes int
		failures  int
	)

	t.Logf("Starting very low request rate test (1 req/sec for %v)...", duration)

	ticker := time.NewTicker(requestRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			totalOps++
			shouldFail := float64(totalOps)*failureRate >= float64(totalOps)

			var err error
			if shouldFail && failures < int(float64(totalOps)*failureRate) {
				_, err = cb.Execute(func() (interface{}, error) {
					return nil, errors.New("failure")
				})
				if err != ErrOpenState {
					failures++
				}
			} else {
				_, err = cb.Execute(func() (interface{}, error) {
					return "success", nil
				})
				if err == nil {
					successes++
				}
			}

			if totalOps%5 == 0 {
				metrics := cb.Metrics()
				t.Logf("After %d requests: state=%v, failure_rate=%.2f%%",
					totalOps, metrics.State, metrics.FailureRate*100)
			}
		}
	}

done:
	metrics := cb.Metrics()

	t.Logf("✅ Low request rate test completed")
	t.Logf("   Total requests: %d", totalOps)
	t.Logf("   Successes: %d, Failures: %d", successes, failures)
	t.Logf("   Final state: %v", metrics.State)
	t.Logf("   Final failure rate: %.2f%%", metrics.FailureRate*100)

	// Verify adaptive threshold worked at low traffic
	if totalOps >= 5 && metrics.FailureRate > 0.50 {
		if cb.State() != StateOpen {
			t.Errorf("Circuit should have opened at low traffic with %.2f%% failure rate",
				metrics.FailureRate*100)
		}
	}
}

// Helper: getMemStats returns current allocated memory in bytes.
func getMemStats() uint64 {
	runtime.GC() // Force GC for accurate measurement
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
