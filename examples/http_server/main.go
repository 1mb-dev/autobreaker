// Package main demonstrates circuit breaker integration with HTTP servers.
//
// This example shows how to protect HTTP endpoints with circuit breakers using
// middleware. This prevents cascading failures when downstream dependencies
// (databases, external APIs, etc.) become slow or unresponsive.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/1mb-dev/autobreaker"
)

// CircuitBreakerMiddleware wraps an HTTP handler with circuit breaker protection.
type CircuitBreakerMiddleware struct {
	breaker *autobreaker.CircuitBreaker
	handler http.Handler
}

// NewCircuitBreakerMiddleware creates middleware that protects a handler with a circuit breaker.
func NewCircuitBreakerMiddleware(breaker *autobreaker.CircuitBreaker, handler http.Handler) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		breaker: breaker,
		handler: handler,
	}
}

// ServeHTTP implements http.Handler with circuit breaker protection.
func (cb *CircuitBreakerMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Use ExecuteContext with request context
	_, err := cb.breaker.ExecuteContext(r.Context(), func() (interface{}, error) {
		// Capture the response by using a custom ResponseWriter
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		cb.handler.ServeHTTP(recorder, r)

		// Check if the response indicates a failure (5xx)
		if recorder.statusCode >= 500 {
			return nil, fmt.Errorf("server error: %d", recorder.statusCode)
		}

		return nil, nil
	})

	// If circuit is open, return 503
	if err == autobreaker.ErrOpenState {
		http.Error(w, "Service temporarily unavailable (circuit breaker open)", http.StatusServiceUnavailable)
		return
	}

	// If there was an error and we haven't written a response yet, return 500
	if err != nil && w.Header().Get("Content-Type") == "" {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// statusRecorder is a ResponseWriter that captures the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Application represents our application with its dependencies.
type Application struct {
	dbBreaker  *autobreaker.CircuitBreaker
	apiBreaker *autobreaker.CircuitBreaker
}

// NewApplication creates a new application with circuit breakers for dependencies.
func NewApplication() *Application {
	return &Application{
		dbBreaker: autobreaker.New(autobreaker.Settings{
			Name:                 "database",
			Timeout:              10 * time.Second,
			AdaptiveThreshold:    true,
			FailureRateThreshold: 0.10, // 10% failure rate
			MinimumObservations:  20,
			OnStateChange: func(name string, from, to autobreaker.State) {
				log.Printf("🔌 Circuit %s: %s → %s", name, from, to)
			},
		}),
		apiBreaker: autobreaker.New(autobreaker.Settings{
			Name:                 "external-api",
			Timeout:              15 * time.Second,
			AdaptiveThreshold:    true,
			FailureRateThreshold: 0.15, // 15% failure rate (more lenient)
			MinimumObservations:  10,
			OnStateChange: func(name string, from, to autobreaker.State) {
				log.Printf("🌐 Circuit %s: %s → %s", name, from, to)
			},
		}),
	}
}

// simulateDBQuery simulates a database query that may fail.
func (app *Application) simulateDBQuery(ctx context.Context) error {
	// Simulate occasional DB slowness/failures
	if rand.Float32() < 0.05 { // 5% failure rate
		return fmt.Errorf("database timeout")
	}
	time.Sleep(10 * time.Millisecond) // Simulate query time
	return nil
}

// simulateAPICall simulates an external API call that may fail.
func (app *Application) simulateAPICall(ctx context.Context) (string, error) {
	// Simulate occasional API failures
	if rand.Float32() < 0.10 { // 10% failure rate
		return "", fmt.Errorf("API unavailable")
	}
	time.Sleep(50 * time.Millisecond) // Simulate network latency
	return "API response", nil
}

// handleHealthCheck handles health check endpoint.
func (app *Application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	dbMetrics := app.dbBreaker.Diagnostics()
	apiMetrics := app.apiBreaker.Diagnostics()

	health := map[string]interface{}{
		"status": "healthy",
		"circuits": map[string]interface{}{
			"database": map[string]interface{}{
				"state":        dbMetrics.State.String(),
				"failure_rate": fmt.Sprintf("%.2f%%", dbMetrics.Metrics.FailureRate*100),
				"requests":     dbMetrics.Metrics.Counts.Requests,
			},
			"external_api": map[string]interface{}{
				"state":        apiMetrics.State.String(),
				"failure_rate": fmt.Sprintf("%.2f%%", apiMetrics.Metrics.FailureRate*100),
				"requests":     apiMetrics.Metrics.Counts.Requests,
			},
		},
	}

	// Overall health is degraded if any circuit is open
	if dbMetrics.State == autobreaker.StateOpen || apiMetrics.State == autobreaker.StateOpen {
		health["status"] = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleUser handles user endpoint with database circuit breaker.
func (app *Application) handleUser(w http.ResponseWriter, r *http.Request) {
	// Use database circuit breaker
	_, err := app.dbBreaker.ExecuteContext(r.Context(), func() (interface{}, error) {
		return nil, app.simulateDBQuery(r.Context())
	})

	if err == autobreaker.ErrOpenState {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Database temporarily unavailable",
		})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Database error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": 123,
		"name":    "John Doe",
	})
}

// handleData handles data endpoint with external API circuit breaker.
func (app *Application) handleData(w http.ResponseWriter, r *http.Request) {
	// Use external API circuit breaker
	result, err := app.apiBreaker.ExecuteContext(r.Context(), func() (interface{}, error) {
		return app.simulateAPICall(r.Context())
	})

	if err == autobreaker.ErrOpenState {
		// Circuit is open - return cached/fallback data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   "fallback data",
			"cached": true,
		})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "External API error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   result,
		"cached": false,
	})
}

func main() {
	app := NewApplication()

	// Create HTTP server with circuit breaker protected endpoints
	mux := http.NewServeMux()

	// Health check endpoint (no circuit breaker, always responds)
	mux.HandleFunc("/health", app.handleHealthCheck)

	// User endpoint (protected by database circuit breaker)
	mux.HandleFunc("/user", app.handleUser)

	// Data endpoint (protected by external API circuit breaker)
	mux.HandleFunc("/data", app.handleData)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("🚀 Server starting on :8080")
	log.Println()
	log.Println("Endpoints:")
	log.Println("  GET /health - Health check with circuit status")
	log.Println("  GET /user   - User endpoint (database circuit breaker)")
	log.Println("  GET /data   - Data endpoint (external API circuit breaker)")
	log.Println()
	log.Println("Try:")
	log.Println("  curl http://localhost:8080/health")
	log.Println("  curl http://localhost:8080/user")
	log.Println("  curl http://localhost:8080/data")
	log.Println()

	// Simulate some background traffic to demonstrate circuit breaker behavior
	go func() {
		time.Sleep(2 * time.Second)
		log.Println("💡 Simulating traffic to demonstrate circuit breaker...")

		for i := 0; i < 100; i++ {
			http.Get("http://localhost:8080/user")
			http.Get("http://localhost:8080/data")
			time.Sleep(100 * time.Millisecond)
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
