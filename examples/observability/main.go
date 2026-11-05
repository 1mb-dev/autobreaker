package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/1mb-dev/autobreaker"
)

// Demonstrate comprehensive observability using Metrics() and Diagnostics()
func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║   AutoBreaker: Observability & Monitoring Example        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create circuit breaker with state change logging
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 "payment-api",
		AdaptiveThreshold:    true,
		Interval:             10 * time.Second, // counts reset every 10s (fixed window)
		FailureRateThreshold: 0.15,             // 15% failure rate
		MinimumObservations:  20,
		Timeout:              5 * time.Second,
		OnStateChange: func(name string, from, to autobreaker.State) {
			log.Printf("🔄 STATE CHANGE [%s]: %v → %v", name, from, to)
		},
	})

	// Scenario 1: Normal operation monitoring
	fmt.Println("=== Scenario 1: Normal Operation Monitoring ===\n")
	scenario1(breaker)

	time.Sleep(1 * time.Second)

	// Scenario 2: Diagnostic troubleshooting
	fmt.Println("\n=== Scenario 2: Diagnostic Troubleshooting ===\n")
	scenario2(breaker)

	time.Sleep(1 * time.Second)

	// Scenario 3: Real-time monitoring during failures
	fmt.Println("\n=== Scenario 3: Real-Time Monitoring During Failures ===\n")
	scenario3(breaker)

	time.Sleep(1 * time.Second)

	// Scenario 4: Recovery monitoring
	fmt.Println("\n=== Scenario 4: Recovery Monitoring ===\n")
	scenario4(breaker)
}

// Scenario 1: Monitor normal operation
func scenario1(breaker *autobreaker.CircuitBreaker) {
	fmt.Println("Executing 50 requests with 5% failure rate...")
	fmt.Println()

	for i := 0; i < 50; i++ {
		_, err := breaker.Execute(func() (interface{}, error) {
			time.Sleep(5 * time.Millisecond)
			if rand.Float64() < 0.05 { // 5% failure
				return nil, fmt.Errorf("payment failed")
			}
			return "OK", nil
		})

		if err != nil && err != autobreaker.ErrOpenState {
			// Log failures (but not circuit-open rejections)
			fmt.Printf("  ❌ Request %d failed\n", i+1)
		}
	}

	// Get metrics after operation
	metrics := breaker.Metrics()

	fmt.Println()
	fmt.Println("📊 Metrics Report:")
	fmt.Printf("  State:            %v\n", metrics.State)
	fmt.Printf("  Total Requests:   %d\n", metrics.Counts.Requests)
	fmt.Printf("  Successes:        %d\n", metrics.Counts.TotalSuccesses)
	fmt.Printf("  Failures:         %d\n", metrics.Counts.TotalFailures)
	fmt.Printf("  Failure Rate:     %.1f%%\n", metrics.FailureRate*100)
	fmt.Printf("  Success Rate:     %.1f%%\n", metrics.SuccessRate*100)
	fmt.Printf("  State Changed:    %v ago\n", time.Since(metrics.StateChangedAt).Round(time.Millisecond))

	// Health check logic
	if metrics.State == autobreaker.StateClosed && metrics.FailureRate < 0.10 {
		fmt.Println()
		fmt.Println("✅ Health Status: HEALTHY")
	}
}

// Scenario 2: Use Diagnostics for troubleshooting
func scenario2(breaker *autobreaker.CircuitBreaker) {
	// Make some requests
	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			if rand.Float64() < 0.30 {
				return nil, fmt.Errorf("error")
			}
			return "OK", nil
		})
	}

	// Get comprehensive diagnostics
	diag := breaker.Diagnostics()

	fmt.Println("🔍 Diagnostics Report:")
	fmt.Printf("  Name:             %s\n", diag.Name)
	fmt.Printf("  State:            %v\n", diag.State)
	fmt.Println()

	fmt.Println("Configuration:")
	fmt.Printf("  Adaptive:         %v\n", diag.AdaptiveEnabled)
	if diag.AdaptiveEnabled {
		fmt.Printf("  Threshold:        %.1f%% failure rate\n", diag.FailureRateThreshold*100)
		fmt.Printf("  Min Observations: %d requests\n", diag.MinimumObservations)
	}
	fmt.Printf("  Timeout:          %v\n", diag.Timeout)
	fmt.Printf("  Max Requests:     %d (in half-open)\n", diag.MaxRequests)
	fmt.Println()

	fmt.Println("Current Stats:")
	fmt.Printf("  Requests:         %d\n", diag.Metrics.Counts.Requests)
	fmt.Printf("  Failure Rate:     %.1f%%\n", diag.Metrics.FailureRate*100)
	fmt.Printf("  Consecutive Fail: %d\n", diag.Metrics.Counts.ConsecutiveFailures)
	fmt.Println()

	fmt.Println("Predictions:")
	if diag.WillTripNext {
		fmt.Println("  ⚠️  WARNING: Next failure will trip the circuit!")
	} else {
		fmt.Println("  ✅ Circuit stable, won't trip on next failure")
	}

	if diag.State == autobreaker.StateOpen && diag.TimeUntilHalfOpen > 0 {
		fmt.Printf("  ⏰ Half-open in: %v\n", diag.TimeUntilHalfOpen.Round(time.Millisecond))
	}
}

// Scenario 3: Real-time monitoring during degradation
func scenario3(breaker *autobreaker.CircuitBreaker) {
	fmt.Println("Simulating service degradation (30% failure rate)...")
	fmt.Println("Monitoring in real-time:")
	fmt.Println()

	for i := 0; i < 100; i++ {
		_, err := breaker.Execute(func() (interface{}, error) {
			time.Sleep(5 * time.Millisecond)
			if rand.Float64() < 0.30 { // 30% failure
				return nil, fmt.Errorf("timeout")
			}
			return "OK", nil
		})

		// Check metrics every 10 requests
		if (i+1)%10 == 0 {
			metrics := breaker.Metrics()
			diag := breaker.Diagnostics()

			fmt.Printf("After %3d requests: State=%10v  FailRate=%5.1f%%  ConsecFail=%d",
				i+1,
				metrics.State,
				metrics.FailureRate*100,
				metrics.Counts.ConsecutiveFailures,
			)

			if diag.WillTripNext {
				fmt.Print("  ⚠️ WILL TRIP NEXT")
			}

			fmt.Println()

			// Circuit opened, stop
			if metrics.State == autobreaker.StateOpen {
				fmt.Println()
				fmt.Println("🔴 Circuit breaker OPENED - protecting downstream service!")
				break
			}
		}

		// Handle open circuit
		if err == autobreaker.ErrOpenState {
			// Circuit is open, requests being rejected
			continue
		}
	}
}

// Scenario 4: Monitor recovery
func scenario4(breaker *autobreaker.CircuitBreaker) {
	// Ensure circuit is open first
	for i := 0; i < 30; i++ {
		breaker.Execute(func() (interface{}, error) {
			return nil, fmt.Errorf("fail")
		})
	}

	metrics := breaker.Metrics()
	if metrics.State != autobreaker.StateOpen {
		fmt.Println("⚠️  Circuit not open, skipping recovery scenario")
		return
	}

	fmt.Println("Circuit is OPEN. Monitoring recovery process...")
	fmt.Println()

	diag := breaker.Diagnostics()
	fmt.Printf("Waiting for timeout (%v)...\n", diag.TimeUntilHalfOpen.Round(time.Millisecond))

	// Poll until circuit transitions
	for {
		time.Sleep(500 * time.Millisecond)

		metrics := breaker.Metrics()
		diag := breaker.Diagnostics()

		if metrics.State == autobreaker.StateOpen {
			if diag.TimeUntilHalfOpen > 0 {
				fmt.Printf("  ⏳ Time remaining: %v\n", diag.TimeUntilHalfOpen.Round(time.Millisecond))
			} else {
				fmt.Println("  ✓ Timeout elapsed, attempting recovery...")
				// Make a probe request
				_, err := breaker.Execute(func() (interface{}, error) {
					return "OK", nil
				})
				if err == nil {
					fmt.Println()
					fmt.Println("✅ Circuit CLOSED - Service recovered!")
					break
				}
			}
		} else if metrics.State == autobreaker.StateClosed {
			fmt.Println()
			fmt.Println("✅ Circuit CLOSED - Service recovered!")
			break
		} else if metrics.State == autobreaker.StateHalfOpen {
			fmt.Println("  🟡 Circuit in HALF-OPEN state, testing...")
		}
	}

	// Final status
	metrics = breaker.Metrics()
	fmt.Println()
	fmt.Println("📊 Final Status:")
	fmt.Printf("  State:          %v\n", metrics.State)
	fmt.Printf("  Total Requests: %d\n", metrics.Counts.Requests)
	fmt.Printf("  Recovery Time:  %v\n", time.Since(metrics.StateChangedAt).Round(time.Millisecond))
}
