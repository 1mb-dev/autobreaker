// Package main demonstrates custom error classification.
package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnykmshr/autobreaker"
)

// HTTPError represents an HTTP error with status code.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

var (
	ErrNotFound     = &HTTPError{StatusCode: 404, Message: "Not Found"}
	ErrBadRequest   = &HTTPError{StatusCode: 400, Message: "Bad Request"}
	ErrServerError  = &HTTPError{StatusCode: 500, Message: "Internal Server Error"}
	ErrServiceUnavailable = &HTTPError{StatusCode: 503, Message: "Service Unavailable"}
)

func main() {
	// Create circuit breaker that only trips on server errors (5xx)
	breaker := autobreaker.New(autobreaker.Settings{
		Name:    "http-service",
		Timeout: 10 * time.Second,
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}

			// 4xx errors are client mistakes, not service failures
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				// Only 5xx status codes count as failures
				return httpErr.StatusCode < 500
			}

			// Other errors (network, timeout) count as failures
			return false
		},
	})

	fmt.Println("=== Custom Error Classification Example ===")
	fmt.Println()
	fmt.Println("Circuit only trips on server errors (5xx), not client errors (4xx)")
	fmt.Println()

	// Test various error types
	testCases := []struct {
		name string
		err  error
	}{
		{"Success (200)", nil},
		{"Not Found (404)", ErrNotFound},
		{"Bad Request (400)", ErrBadRequest},
		{"Server Error (500)", ErrServerError},
		{"Service Unavailable (503)", ErrServiceUnavailable},
	}

	fmt.Println("1. Testing error classification:")
	for _, tc := range testCases {
		result, err := breaker.Execute(func() (interface{}, error) {
			return nil, tc.err
		})

		fmt.Printf("   %s: result=%v, err=%v, state=%v\n",
			tc.name, result, err, breaker.State())
	}

	fmt.Printf("\n   Final counts: %+v\n", breaker.Counts())

	// Simulate many 404s (should NOT trip circuit)
	fmt.Println("\n2. Many 404 errors (should NOT trip circuit):")
	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			return nil, ErrNotFound
		})
	}

	fmt.Printf("   Circuit state after 10x 404: %v (should be closed)\n", breaker.State())

	// Simulate server errors (should trip circuit)
	fmt.Println("\n3. Server errors (should trip circuit):")
	for i := 0; i < 10; i++ {
		result, err := breaker.Execute(func() (interface{}, error) {
			return nil, ErrServerError
		})

		if i == 5 || i == 9 {
			fmt.Printf("   After %d server errors: state=%v\n", i+1, breaker.State())
		}

		_ = result
		_ = err
	}

	fmt.Printf("\n   Final state: %v\n", breaker.State())
	fmt.Printf("   Final counts: %+v\n", breaker.Counts())

	fmt.Println("\n=== Key Insight ===")
	fmt.Println("Client errors (4xx) don't trip the circuit.")
	fmt.Println("Only server errors (5xx) indicate service health issues.")
	fmt.Println("\nCustom error classification prevents false positives!")
}
