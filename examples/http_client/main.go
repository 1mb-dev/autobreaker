// Package main demonstrates circuit breaker integration with HTTP clients.
//
// This example shows how to wrap http.Client with a circuit breaker using
// a custom RoundTripper. This protects your application from slow or failing
// HTTP services by failing fast when the service is unhealthy.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/1mb-dev/autobreaker"
)

// CircuitBreakerRoundTripper wraps an http.RoundTripper with circuit breaker protection.
type CircuitBreakerRoundTripper struct {
	breaker   *autobreaker.CircuitBreaker
	transport http.RoundTripper
}

// NewCircuitBreakerRoundTripper creates a new circuit-breaker-protected RoundTripper.
func NewCircuitBreakerRoundTripper(breaker *autobreaker.CircuitBreaker, transport http.RoundTripper) *CircuitBreakerRoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &CircuitBreakerRoundTripper{
		breaker:   breaker,
		transport: transport,
	}
}

// RoundTrip implements http.RoundTripper with circuit breaker protection.
func (cb *CircuitBreakerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Use ExecuteContext with request context for proper cancellation
	result, err := cb.breaker.ExecuteContext(req.Context(), func() (interface{}, error) {
		return cb.transport.RoundTrip(req)
	})

	if err == autobreaker.ErrOpenState {
		// Circuit is open - return a 503 Service Unavailable response
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable (Circuit Open)",
			Body:       http.NoBody,
			Request:    req,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return result.(*http.Response), nil
}

// isSuccessfulHTTPRequest determines if an HTTP response is successful.
// 4xx client errors don't indicate backend failure, only 5xx server errors do.
func isSuccessfulHTTPRequest(err error) bool {
	if err != nil {
		return false // Network errors are failures
	}
	// Note: We can't check status code here because we only have the error.
	// The actual response checking happens in the application logic.
	return true
}

// NewProtectedHTTPClient creates an http.Client with circuit breaker protection.
func NewProtectedHTTPClient(serviceName string) *http.Client {
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 serviceName,
		Timeout:              10 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // 10% failure rate
		MinimumObservations:  20,
		OnStateChange: func(name string, from, to autobreaker.State) {
			log.Printf("Circuit %s: %s → %s", name, from, to)
		},
	})

	return &http.Client{
		Transport: NewCircuitBreakerRoundTripper(breaker, nil),
		Timeout:   30 * time.Second,
	}
}

func main() {
	// Create a protected HTTP client for an external API
	client := NewProtectedHTTPClient("external-api")

	fmt.Println("HTTP Client with Circuit Breaker Example")
	fmt.Println("=========================================")
	fmt.Println()

	// Example 1: Successful request
	fmt.Println("Example 1: Making successful request...")
	resp, err := client.Get("https://httpbin.org/status/200")
	if err != nil {
		log.Printf("Request failed: %v", err)
	} else {
		fmt.Printf("✅ Success: Status %s\n", resp.Status)
		resp.Body.Close()
	}
	fmt.Println()

	// Example 2: Failed request (5xx error)
	fmt.Println("Example 2: Simulating server errors (5xx)...")
	for i := 0; i < 30; i++ {
		resp, err := client.Get("https://httpbin.org/status/500")
		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
		} else {
			if resp.StatusCode == http.StatusServiceUnavailable {
				fmt.Printf("⚡ Circuit open: Request %d rejected (fail fast)\n", i+1)
			} else {
				fmt.Printf("❌ Server error: Request %d returned %s\n", i+1, resp.Status)
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println()

	// Example 3: Context cancellation
	fmt.Println("Example 3: Request with context timeout...")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "https://httpbin.org/delay/5", nil)
	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("✅ Request cancelled: %v (not counted as failure)\n", err)
	}
	fmt.Println()

	// Example 4: Client errors (4xx) don't trip circuit
	fmt.Println("Example 4: Client errors (4xx should not trip circuit)...")
	for i := 0; i < 10; i++ {
		resp, err := client.Get("https://httpbin.org/status/404")
		if err != nil {
			log.Printf("Request failed: %v", err)
		} else {
			fmt.Printf("ℹ️  Client error: %s (doesn't trip circuit)\n", resp.Status)
			resp.Body.Close()
		}
	}
	fmt.Println()

	fmt.Println("Example complete!")
}
