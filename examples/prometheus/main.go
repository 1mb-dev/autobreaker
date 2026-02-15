package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/1mb-dev/autobreaker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CircuitBreakerCollector collects circuit breaker metrics for Prometheus.
type CircuitBreakerCollector struct {
	breaker *autobreaker.CircuitBreaker

	// Metric descriptors
	stateDesc          *prometheus.Desc
	requestsDesc       *prometheus.Desc
	successesDesc      *prometheus.Desc
	failuresDesc       *prometheus.Desc
	consecSuccessDesc  *prometheus.Desc
	consecFailuresDesc *prometheus.Desc
	failureRateDesc    *prometheus.Desc
	successRateDesc    *prometheus.Desc
}

// NewCircuitBreakerCollector creates a Prometheus collector for a circuit breaker.
func NewCircuitBreakerCollector(breaker *autobreaker.CircuitBreaker) *CircuitBreakerCollector {
	// Get breaker name for labels
	name := breaker.Name()

	return &CircuitBreakerCollector{
		breaker: breaker,

		// Define metric descriptors
		stateDesc: prometheus.NewDesc(
			"circuit_breaker_state",
			"Current circuit breaker state (0=closed, 1=open, 2=half-open)",
			nil,
			prometheus.Labels{"name": name},
		),
		requestsDesc: prometheus.NewDesc(
			"circuit_breaker_requests_total",
			"Total number of requests",
			nil,
			prometheus.Labels{"name": name},
		),
		successesDesc: prometheus.NewDesc(
			"circuit_breaker_successes_total",
			"Total number of successful requests",
			nil,
			prometheus.Labels{"name": name},
		),
		failuresDesc: prometheus.NewDesc(
			"circuit_breaker_failures_total",
			"Total number of failed requests",
			nil,
			prometheus.Labels{"name": name},
		),
		consecSuccessDesc: prometheus.NewDesc(
			"circuit_breaker_consecutive_successes",
			"Current consecutive successes",
			nil,
			prometheus.Labels{"name": name},
		),
		consecFailuresDesc: prometheus.NewDesc(
			"circuit_breaker_consecutive_failures",
			"Current consecutive failures",
			nil,
			prometheus.Labels{"name": name},
		),
		failureRateDesc: prometheus.NewDesc(
			"circuit_breaker_failure_rate",
			"Current failure rate (failures/requests)",
			nil,
			prometheus.Labels{"name": name},
		),
		successRateDesc: prometheus.NewDesc(
			"circuit_breaker_success_rate",
			"Current success rate (successes/requests)",
			nil,
			prometheus.Labels{"name": name},
		),
	}
}

// Describe implements prometheus.Collector.
func (c *CircuitBreakerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.stateDesc
	ch <- c.requestsDesc
	ch <- c.successesDesc
	ch <- c.failuresDesc
	ch <- c.consecSuccessDesc
	ch <- c.consecFailuresDesc
	ch <- c.failureRateDesc
	ch <- c.successRateDesc
}

// Collect implements prometheus.Collector.
func (c *CircuitBreakerCollector) Collect(ch chan<- prometheus.Metric) {
	metrics := c.breaker.Metrics()

	// Export state as gauge (0=closed, 1=open, 2=half-open)
	ch <- prometheus.MustNewConstMetric(
		c.stateDesc,
		prometheus.GaugeValue,
		float64(metrics.State),
	)

	// Export counts as counters
	ch <- prometheus.MustNewConstMetric(
		c.requestsDesc,
		prometheus.CounterValue,
		float64(metrics.Counts.Requests),
	)

	ch <- prometheus.MustNewConstMetric(
		c.successesDesc,
		prometheus.CounterValue,
		float64(metrics.Counts.TotalSuccesses),
	)

	ch <- prometheus.MustNewConstMetric(
		c.failuresDesc,
		prometheus.CounterValue,
		float64(metrics.Counts.TotalFailures),
	)

	// Export consecutive counts as gauges
	ch <- prometheus.MustNewConstMetric(
		c.consecSuccessDesc,
		prometheus.GaugeValue,
		float64(metrics.Counts.ConsecutiveSuccesses),
	)

	ch <- prometheus.MustNewConstMetric(
		c.consecFailuresDesc,
		prometheus.GaugeValue,
		float64(metrics.Counts.ConsecutiveFailures),
	)

	// Export rates as gauges
	ch <- prometheus.MustNewConstMetric(
		c.failureRateDesc,
		prometheus.GaugeValue,
		metrics.FailureRate,
	)

	ch <- prometheus.MustNewConstMetric(
		c.successRateDesc,
		prometheus.GaugeValue,
		metrics.SuccessRate,
	)
}

// Simulate API calls with varying success rates
func simulateAPICall() error {
	time.Sleep(10 * time.Millisecond) // Simulate latency

	// 20% failure rate
	if rand.Float64() < 0.20 {
		return fmt.Errorf("API error")
	}
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Create circuit breaker with adaptive thresholds
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 "api-client",
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.10, // Trip at 10% failure rate
		MinimumObservations:  20,
		Timeout:              5 * time.Second,
	})

	// Create and register Prometheus collector
	collector := NewCircuitBreakerCollector(breaker)
	prometheus.MustRegister(collector)

	// Start background worker to make requests
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			_, err := breaker.Execute(func() (interface{}, error) {
				return nil, simulateAPICall()
			})

			if err != nil {
				if err == autobreaker.ErrOpenState {
					fmt.Println("⚠️  Circuit is OPEN - request rejected")
				} else {
					fmt.Printf("❌ Request failed: %v\n", err)
				}
			} else {
				fmt.Println("✓ Request succeeded")
			}
		}
	}()

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     AutoBreaker + Prometheus Integration Example              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Prometheus metrics available at: http://localhost:8080/metrics")
	fmt.Println()
	fmt.Println("Example queries:")
	fmt.Println("  - circuit_breaker_state")
	fmt.Println("  - circuit_breaker_failure_rate")
	fmt.Println("  - circuit_breaker_failures_total")
	fmt.Println()
	fmt.Println("Circuit breaker making requests in background...")
	fmt.Println("Watch for circuit state changes!")
	fmt.Println()

	log.Fatal(http.ListenAndServe(":8080", nil))
}
