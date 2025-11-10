package breaker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntegration_HTTPClient validates circuit breaker with real HTTP client.
// This is a minimal integration test - comprehensive examples in examples/http_client/
func TestIntegration_HTTPClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var requestCount atomic.Int32
	var failureCount atomic.Int32

	// Create test HTTP server that fails intermittently
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		// Fail first 3 requests to trip circuit
		if count <= 3 {
			failureCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Succeed afterwards
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "success")
	}))
	defer server.Close()

	// Create circuit breaker
	cb := New(Settings{
		Name:    "http-integration",
		Timeout: 500 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	})

	client := &http.Client{Timeout: 1 * time.Second}

	// Make HTTP request through circuit breaker
	makeRequest := func() (interface{}, error) {
		resp, err := client.Get(server.URL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return resp.StatusCode, nil
	}

	// Phase 1: Trip circuit with 3 failures
	for i := 0; i < 3; i++ {
		_, err := cb.Execute(makeRequest)
		if err == nil {
			t.Errorf("Request %d: expected error, got success", i+1)
		}
	}

	// Verify circuit is open
	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open after 3 failures, got %v", cb.State())
	}

	// Phase 2: Requests should fail fast
	_, err := cb.Execute(makeRequest)
	if err != ErrOpenState {
		t.Errorf("Expected ErrOpenState when circuit open, got %v", err)
	}

	// Phase 3: Wait for timeout and verify recovery
	time.Sleep(600 * time.Millisecond)

	// Next request should succeed and close circuit
	result, err := cb.Execute(makeRequest)
	if err != nil {
		t.Errorf("Recovery request failed: %v", err)
	}
	if result != http.StatusOK {
		t.Errorf("Expected status 200, got %v", result)
	}

	// Verify circuit recovered
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be closed after recovery, got %v", cb.State())
	}

	t.Logf("Integration test completed: %d total requests, %d failures, circuit recovered",
		requestCount.Load(), failureCount.Load())
}

// TestIntegration_HTTPClientWithContext validates ExecuteContext integration.
// This is a minimal integration test - comprehensive examples in examples/http_client/
func TestIntegration_HTTPClientWithContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create slow test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := New(Settings{Name: "http-context-integration"})
	client := &http.Client{Timeout: 5 * time.Second}

	// Test context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	makeRequest := func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return resp.StatusCode, nil
	}

	// Request should timeout due to context
	_, err := cb.ExecuteContext(ctx, makeRequest)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	// Verify context timeout didn't trip circuit
	if cb.State() != StateClosed {
		t.Errorf("Context timeout should not trip circuit, got state %v", cb.State())
	}

	counts := cb.Counts()
	if counts.TotalFailures > 0 {
		t.Errorf("Context cancellation should not count as failure, got %d failures", counts.TotalFailures)
	}
}
