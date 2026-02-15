// Package main demonstrates basic usage of AutoBreaker.
package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/1mb-dev/autobreaker"
)

func main() {
	// Create circuit breaker with default settings
	breaker := autobreaker.New(autobreaker.Settings{
		Name:    "example-service",
		Timeout: 10 * time.Second,
	})

	fmt.Println("=== Basic Circuit Breaker Example ===")
	fmt.Println()

	// Simulate successful operations
	fmt.Println("1. Successful operations:")
	for i := 0; i < 3; i++ {
		result, err := breaker.Execute(func() (interface{}, error) {
			return "success", nil
		})

		fmt.Printf("   Attempt %d: result=%v, err=%v, state=%v\n", i+1, result, err, breaker.State())
	}

	// Simulate failures (will trip circuit after 6 consecutive failures)
	fmt.Println("\n2. Failing operations (will trip circuit):")
	for i := 0; i < 8; i++ {
		result, err := breaker.Execute(func() (interface{}, error) {
			return nil, errors.New("service unavailable")
		})

		fmt.Printf("   Attempt %d: result=%v, err=%v, state=%v\n", i+1, result, err, breaker.State())
	}

	// Circuit should now be open
	fmt.Printf("\n   Circuit state: %v\n", breaker.State())
	fmt.Printf("   Counts: %+v\n", breaker.Counts())

	// Requests are rejected while circuit is open
	fmt.Println("\n3. Requests while circuit is open (will be rejected):")
	for i := 0; i < 3; i++ {
		result, err := breaker.Execute(func() (interface{}, error) {
			return "success", nil
		})

		fmt.Printf("   Attempt %d: result=%v, err=%v\n", i+1, result, err)
	}

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("\nCircuit breaker is fully functional!")
}
