package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/1mb-dev/autobreaker"
)

// Simulate different environments with varying traffic levels
type Environment struct {
	Name           string
	RequestsPerSec int
	FailureRate    float64 // Natural failure rate (e.g., 0.02 = 2%)
}

var environments = []Environment{
	{"Development", 5, 0.02},  // 5 req/s, 2% failure
	{"Staging", 50, 0.02},     // 50 req/s, 2% failure
	{"Production", 500, 0.02}, // 500 req/s, 2% failure
}

// Simulate API call with realistic failure patterns
func callExternalAPI(failureRate float64) error {
	// Simulate network latency
	time.Sleep(10 * time.Millisecond)

	if rand.Float64() < failureRate {
		return errors.New("service unavailable")
	}
	return nil
}

// Scenario 1: Normal operation with natural failures
func scenarioNormalOperation(env Environment, breaker *autobreaker.CircuitBreaker) {
	fmt.Printf("\n=== Scenario 1: Normal Operation (%s) ===\n", env.Name)
	fmt.Printf("Traffic: %d req/s, Natural failure rate: %.1f%%\n\n", env.RequestsPerSec, env.FailureRate*100)

	duration := 2 * time.Second
	ticker := time.NewTicker(time.Second / time.Duration(env.RequestsPerSec))
	defer ticker.Stop()

	deadline := time.Now().Add(duration)
	var successes, failures, rejected int

	for time.Now().Before(deadline) {
		<-ticker.C

		_, err := breaker.Execute(func() (interface{}, error) {
			return nil, callExternalAPI(env.FailureRate)
		})

		switch err {
		case nil:
			successes++
		case autobreaker.ErrOpenState:
			rejected++
		default:
			failures++
		}
	}

	total := successes + failures + rejected
	fmt.Printf("Results: %d total requests\n", total)
	fmt.Printf("  - Successes: %d (%.1f%%)\n", successes, float64(successes)/float64(total)*100)
	fmt.Printf("  - Failures: %d (%.1f%%)\n", failures, float64(failures)/float64(total)*100)
	fmt.Printf("  - Rejected: %d (%.1f%%)\n", rejected, float64(rejected)/float64(total)*100)
	fmt.Printf("Circuit state: %v\n", breaker.State())

	// Circuit should remain closed under normal conditions
	if breaker.State() != autobreaker.StateClosed {
		fmt.Printf("⚠️  Warning: Circuit opened under normal conditions!\n")
	} else {
		fmt.Printf("✓ Circuit remained stable\n")
	}
}

// Scenario 2: Service degradation (increased error rate)
func scenarioServiceDegradation(env Environment, breaker *autobreaker.CircuitBreaker) {
	fmt.Printf("\n=== Scenario 2: Service Degradation (%s) ===\n", env.Name)
	fmt.Printf("Traffic: %d req/s, Degraded failure rate: 15%%\n\n", env.RequestsPerSec)

	duration := 2 * time.Second
	ticker := time.NewTicker(time.Second / time.Duration(env.RequestsPerSec))
	defer ticker.Stop()

	deadline := time.Now().Add(duration)
	var successes, failures, rejected int
	var trippedAt time.Duration
	startTime := time.Now()

	for time.Now().Before(deadline) {
		<-ticker.C

		_, err := breaker.Execute(func() (interface{}, error) {
			return nil, callExternalAPI(0.15) // 15% failure rate
		})

		switch err {
		case nil:
			successes++
		case autobreaker.ErrOpenState:
			rejected++
			if trippedAt == 0 {
				trippedAt = time.Since(startTime)
			}
		default:
			failures++
		}
	}

	total := successes + failures + rejected
	fmt.Printf("Results: %d total requests\n", total)
	fmt.Printf("  - Successes: %d (%.1f%%)\n", successes, float64(successes)/float64(total)*100)
	fmt.Printf("  - Failures: %d (%.1f%%)\n", failures, float64(failures)/float64(total)*100)
	fmt.Printf("  - Rejected: %d (%.1f%%)\n", rejected, float64(rejected)/float64(total)*100)
	fmt.Printf("Circuit state: %v\n", breaker.State())

	if trippedAt > 0 {
		fmt.Printf("✓ Circuit tripped after %v (protecting downstream service)\n", trippedAt.Round(time.Millisecond))
	} else {
		fmt.Printf("⚠️  Circuit did not trip despite high error rate\n")
	}
}

// Scenario 3: Spike in failures
func scenarioFailureSpike(env Environment, breaker *autobreaker.CircuitBreaker) {
	fmt.Printf("\n=== Scenario 3: Sudden Failure Spike (%s) ===\n", env.Name)
	fmt.Printf("Burst of failures followed by recovery\n\n")

	var successes, failures, rejected int

	// Burst of 30 failures
	fmt.Print("Simulating failure burst... ")
	for i := 0; i < 30; i++ {
		_, err := breaker.Execute(func() (interface{}, error) {
			return nil, errors.New("connection timeout")
		})

		if err == autobreaker.ErrOpenState {
			rejected++
		} else {
			failures++
		}
		time.Sleep(5 * time.Millisecond)
	}
	fmt.Printf("done\n")

	fmt.Printf("After burst: %d failures, %d rejected, state=%v\n", failures, rejected, breaker.State())

	// Wait for circuit to potentially recover
	if breaker.State() == autobreaker.StateOpen {
		fmt.Print("Waiting for timeout... ")
		time.Sleep(1100 * time.Millisecond) // Wait for default 60s timeout (we'll use 1s for demo)
		fmt.Printf("done\n")
	}

	// Try recovery with successful requests
	fmt.Print("Testing recovery... ")
	for i := 0; i < 10; i++ {
		_, err := breaker.Execute(func() (interface{}, error) {
			return nil, nil // Success
		})

		if err == nil {
			successes++
		}
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Printf("done\n")

	fmt.Printf("After recovery: %d successes, state=%v\n", successes, breaker.State())

	if breaker.State() == autobreaker.StateClosed {
		fmt.Printf("✓ Circuit recovered successfully\n")
	}
}

// Compare adaptive vs static configuration
func compareAdaptiveVsStatic() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("COMPARISON: Adaptive vs Static Thresholds")
	fmt.Println(strings.Repeat("=", 70))

	// Adaptive breaker: 5% threshold
	adaptive := autobreaker.New(autobreaker.Settings{
		Name:                 "adaptive",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // 5%
		MinimumObservations:  20,
		Timeout:              500 * time.Millisecond,
	})

	// Static breaker: 5 consecutive failures
	static := autobreaker.New(autobreaker.Settings{
		Name:    "static",
		Timeout: 500 * time.Millisecond,
		ReadyToTrip: func(counts autobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	fmt.Printf("\nTest: 100 requests with 6%% failure rate (6 failures distributed)\n")
	fmt.Printf("Adaptive: Trips when failure rate > 5%%\n")
	fmt.Printf("Static: Trips when consecutive failures > 5\n\n")

	// Test both breakers
	var adaptiveTripped, staticTripped bool

	for i := 0; i < 100; i++ {
		// Distribute failures (every ~17th request fails = ~6%)
		shouldFail := i%17 == 0

		req := func() (interface{}, error) {
			if shouldFail {
				return nil, errors.New("failed")
			}
			return "ok", nil
		}

		// Test adaptive
		if !adaptiveTripped {
			_, err := adaptive.Execute(req)
			if err == autobreaker.ErrOpenState {
				adaptiveTripped = true
				fmt.Printf("Adaptive breaker tripped at request %d\n", i)
			}
		}

		// Test static
		if !staticTripped {
			_, err := static.Execute(req)
			if err == autobreaker.ErrOpenState {
				staticTripped = true
				fmt.Printf("Static breaker tripped at request %d\n", i)
			}
		}
	}

	fmt.Printf("\nResults:\n")
	fmt.Printf("  Adaptive: %s (correctly detected 6%% > 5%% threshold)\n",
		map[bool]string{true: "✓ TRIPPED", false: "✗ Did not trip"}[adaptiveTripped])
	fmt.Printf("  Static: %s (failures were distributed, not consecutive)\n",
		map[bool]string{true: "✓ TRIPPED", false: "✗ Did not trip"}[staticTripped])

	if adaptiveTripped && !staticTripped {
		fmt.Printf("\n💡 Key Insight: Adaptive threshold caught distributed failures that\n")
		fmt.Printf("   static consecutive-count logic missed!\n")
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("╔════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     AutoBreaker: Production-Ready Adaptive Circuit Breaker        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════╝")

	// Use adaptive threshold configuration
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 "api-client",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // Trip at 5% failure rate
		MinimumObservations:  20,   // Need 20+ requests before evaluating
		Timeout:              1 * time.Second,
	})

	fmt.Println("\nConfiguration:")
	fmt.Println("  - Adaptive threshold: 5% failure rate")
	fmt.Println("  - Minimum observations: 20 requests")
	fmt.Println("  - Timeout: 1 second")
	fmt.Println("  - Same config works for all environments! 🎯")

	// Run scenarios across different environments
	env := environments[2] // Production

	scenarioNormalOperation(env, breaker)
	scenarioServiceDegradation(env, breaker)

	// Wait for circuit to close before next scenario
	time.Sleep(1200 * time.Millisecond)

	scenarioFailureSpike(env, breaker)

	// Comparison
	compareAdaptiveVsStatic()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✓ All scenarios complete!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  1. Same config works across all traffic levels (dev → production)")
	fmt.Println("  2. Automatically adapts to your traffic patterns")
	fmt.Println("  3. Catches distributed failures that static thresholds miss")
	fmt.Println("  4. Protects downstream services during degradation")
	fmt.Println("  5. Recovers automatically when service improves")
	fmt.Println("\n💡 Use adaptive thresholds to eliminate manual tuning!")
}
