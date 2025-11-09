// Package main demonstrates adaptive threshold usage.
package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnykmshr/autobreaker"
)

func main() {
	// Create circuit breaker with adaptive thresholds
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 "adaptive-service",
		Timeout:              10 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // Trip at 10% failure rate
		MinimumObservations:  20,   // Need 20+ requests before adapting
	})

	fmt.Println("=== Adaptive Threshold Example ===")
	fmt.Println()
	fmt.Println("Circuit trips when failure rate > 10% (after 20+ requests)")
	fmt.Println()

	// Simulate low traffic scenario
	fmt.Println("1. Low traffic (50 requests, 3 failures = 6% error rate):")
	successCount := 0
	failureCount := 0

	for i := 0; i < 50; i++ {
		var err error
		if i%17 == 0 { // ~6% failure rate
			err = errors.New("failure")
			failureCount++
		}

		_, execErr := breaker.Execute(func() (interface{}, error) {
			return "result", err
		})

		if execErr == nil {
			successCount++
		}
	}

	fmt.Printf("   Results: %d successes, %d failures\n", successCount, failureCount)
	fmt.Printf("   Circuit state: %v (should be closed - below 10%% threshold)\n", breaker.State())
	fmt.Printf("   Counts: %+v\n", breaker.Counts())

	// Reset for high traffic scenario
	breaker = autobreaker.New(autobreaker.Settings{
		Name:                 "adaptive-service-2",
		Timeout:              10 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10,
		MinimumObservations:  20,
	})

	fmt.Println("\n2. High traffic (100 requests, 15 failures = 15% error rate):")
	successCount = 0
	failureCount = 0

	for i := 0; i < 100; i++ {
		var err error
		if i%7 == 0 { // ~14% failure rate
			err = errors.New("failure")
			failureCount++
		}

		_, execErr := breaker.Execute(func() (interface{}, error) {
			return "result", err
		})

		if execErr == nil {
			successCount++
		}
	}

	fmt.Printf("   Results: %d successes, %d failures\n", successCount, failureCount)
	fmt.Printf("   Circuit state: %v (should trip - above 10%% threshold)\n", breaker.State())
	fmt.Printf("   Counts: %+v\n", breaker.Counts())

	fmt.Println("\n=== Key Insight ===")
	fmt.Println("Same configuration works for both low and high traffic!")
	fmt.Println("- Low traffic: 3 failures doesn't trip (6% < 10%)")
	fmt.Println("- High traffic: 15 failures trips circuit (15% > 10%)")
	fmt.Println("\nNote: Implementation pending - Execute() not yet implemented in Phase 0")
}
