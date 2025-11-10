package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vnykmshr/autobreaker"
)

// Config represents the circuit breaker configuration that can be loaded from files
type Config struct {
	MaxRequests          *uint32        `json:"max_requests,omitempty"`
	Interval             *time.Duration `json:"interval,omitempty"`
	Timeout              *time.Duration `json:"timeout,omitempty"`
	FailureRateThreshold *float64       `json:"failure_rate_threshold,omitempty"`
	MinimumObservations  *uint32        `json:"minimum_observations,omitempty"`
}

// ConfigManager handles runtime configuration updates
type ConfigManager struct {
	breaker    *autobreaker.CircuitBreaker
	configFile string
	mu         sync.RWMutex
	lastConfig Config
}

func NewConfigManager(breaker *autobreaker.CircuitBreaker, configFile string) *ConfigManager {
	return &ConfigManager{
		breaker:    breaker,
		configFile: configFile,
	}
}

// LoadAndApply loads configuration from file and applies it to the circuit breaker
func (cm *ConfigManager) LoadAndApply() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Read config file
	data, err := os.ReadFile(cm.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Convert to SettingsUpdate
	update := autobreaker.SettingsUpdate{
		MaxRequests:          config.MaxRequests,
		Interval:             config.Interval,
		Timeout:              config.Timeout,
		FailureRateThreshold: config.FailureRateThreshold,
		MinimumObservations:  config.MinimumObservations,
	}

	// Apply update
	if err := cm.breaker.UpdateSettings(update); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	cm.lastConfig = config
	log.Printf("Configuration updated successfully from %s", cm.configFile)
	cm.logCurrentConfig()

	return nil
}

func (cm *ConfigManager) logCurrentConfig() {
	diag := cm.breaker.Diagnostics()
	log.Printf("Current configuration:")
	log.Printf("  MaxRequests: %d", diag.MaxRequests)
	log.Printf("  Interval: %v", diag.Interval)
	log.Printf("  Timeout: %v", diag.Timeout)
	log.Printf("  FailureRateThreshold: %.2f%%", diag.FailureRateThreshold*100)
	log.Printf("  MinimumObservations: %d", diag.MinimumObservations)
}

// WatchForSignals sets up signal handler for config reload on SIGHUP
func (cm *ConfigManager) WatchForSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		for range sigChan {
			log.Println("Received SIGHUP, reloading configuration...")
			if err := cm.LoadAndApply(); err != nil {
				log.Printf("Error reloading config: %v", err)
			}
		}
	}()
}

// HTTPHandler returns an http.Handler for runtime config updates via API
func (cm *ConfigManager) HTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// GET /config - Show current configuration
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		diag := cm.breaker.Diagnostics()
		response := map[string]interface{}{
			"max_requests":           diag.MaxRequests,
			"interval":               diag.Interval.String(),
			"timeout":                diag.Timeout.String(),
			"failure_rate_threshold": diag.FailureRateThreshold,
			"minimum_observations":   diag.MinimumObservations,
			"current_state":          diag.State.String(),
			"metrics":                diag.Metrics,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// POST /config - Update configuration
	mux.HandleFunc("/config/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		update := autobreaker.SettingsUpdate{
			MaxRequests:          config.MaxRequests,
			Interval:             config.Interval,
			Timeout:              config.Timeout,
			FailureRateThreshold: config.FailureRateThreshold,
			MinimumObservations:  config.MinimumObservations,
		}

		if err := cm.breaker.UpdateSettings(update); err != nil {
			http.Error(w, fmt.Sprintf("Update failed: %v", err), http.StatusBadRequest)
			return
		}

		log.Println("Configuration updated via HTTP API")
		cm.logCurrentConfig()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Configuration updated successfully",
		})
	})

	// POST /config/reload - Reload from file
	mux.HandleFunc("/config/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := cm.LoadAndApply(); err != nil {
			http.Error(w, fmt.Sprintf("Reload failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Configuration reloaded from file",
		})
	})

	return mux
}

func main() {
	fmt.Println("=== Runtime Configuration Example ===\n")

	// Create initial circuit breaker with default settings
	breaker := autobreaker.New(autobreaker.Settings{
		Name:                 "api-client",
		Timeout:              10 * time.Second,
		Interval:             10 * time.Second,
		AdaptiveThreshold:    true,
		FailureRateThreshold: 0.05, // 5%
		MinimumObservations:  20,
		OnStateChange: func(name string, from, to autobreaker.State) {
			log.Printf("Circuit breaker '%s' state changed: %s → %s", name, from, to)
		},
	})

	// Create config file with example configuration
	configFile := "/tmp/circuit_breaker_config.json"
	initialConfig := Config{
		MaxRequests:          autobreaker.Uint32Ptr(5),
		Interval:             autobreaker.DurationPtr(15 * time.Second),
		Timeout:              autobreaker.DurationPtr(30 * time.Second),
		FailureRateThreshold: autobreaker.Float64Ptr(0.10), // 10%
		MinimumObservations:  autobreaker.Uint32Ptr(30),
	}

	configData, _ := json.MarshalIndent(initialConfig, "", "  ")
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}
	fmt.Printf("Created example config file: %s\n\n", configFile)

	// Setup configuration manager
	configMgr := NewConfigManager(breaker, configFile)

	// Load initial configuration from file
	log.Println("Loading initial configuration from file...")
	if err := configMgr.LoadAndApply(); err != nil {
		log.Fatalf("Failed to load initial config: %v", err)
	}
	fmt.Println()

	// Setup signal handler for SIGHUP (reload config)
	configMgr.WatchForSignals()
	log.Println("Signal handler installed: send SIGHUP to reload config")
	fmt.Println()

	// Start HTTP server for runtime updates
	go func() {
		log.Println("Starting HTTP server on :8081")
		log.Println("  GET  /config        - View current configuration")
		log.Println("  POST /config/update - Update configuration")
		log.Println("  POST /config/reload - Reload from file")
		fmt.Println()

		if err := http.ListenAndServe(":8081", configMgr.HTTPHandler()); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Simulate some API calls
	fmt.Println("=== Scenario 1: Normal Operation ===")
	simulateRequests(breaker, 20, 0.02) // 2% failure rate
	time.Sleep(1 * time.Second)

	fmt.Println("\n=== Scenario 2: Update Configuration via Code ===")
	log.Println("Updating threshold to 15% (less sensitive)...")
	err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
		FailureRateThreshold: autobreaker.Float64Ptr(0.15),
	})
	if err != nil {
		log.Printf("Update failed: %v", err)
	} else {
		log.Println("Configuration updated successfully")
		configMgr.logCurrentConfig()
	}
	fmt.Println()

	fmt.Println("=== Scenario 3: Increased Failure Rate ===")
	simulateRequests(breaker, 30, 0.12) // 12% failure rate - won't trip (threshold is 15%)
	time.Sleep(1 * time.Second)

	fmt.Println("\n=== Scenario 4: Update via File ===")
	log.Println("Modifying config file to make circuit more sensitive...")
	sensitiveConfig := Config{
		FailureRateThreshold: autobreaker.Float64Ptr(0.05), // Back to 5%
		Timeout:              autobreaker.DurationPtr(60 * time.Second),
	}
	configData, _ = json.MarshalIndent(sensitiveConfig, "", "  ")
	os.WriteFile(configFile, configData, 0644)

	log.Println("Reloading configuration from file...")
	if err := configMgr.LoadAndApply(); err != nil {
		log.Printf("Reload failed: %v", err)
	}
	fmt.Println()

	fmt.Println("=== Scenario 5: Circuit Trips with New Config ===")
	simulateRequests(breaker, 30, 0.08) // 8% failure rate - will trip now (threshold is 5%)
	time.Sleep(1 * time.Second)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nYou can now:")
	fmt.Println("  1. Send SIGHUP to reload config:  kill -HUP", os.Getpid())
	fmt.Println("  2. View config:  curl http://localhost:8081/config")
	fmt.Println("  3. Update config:  curl -X POST http://localhost:8081/config/update -d '{\"failure_rate_threshold\":0.20}'")
	fmt.Println("  4. Reload file:  curl -X POST http://localhost:8081/config/reload")
	fmt.Println("\nPress Ctrl+C to exit")

	// Keep running for interactive testing
	select {}
}

func simulateRequests(breaker *autobreaker.CircuitBreaker, count int, failureRate float64) {
	successCount := 0
	failureCount := 0
	rejectedCount := 0

	for i := 0; i < count; i++ {
		_, err := breaker.Execute(func() (interface{}, error) {
			// Simulate failure based on failure rate
			if float64(i%100) < failureRate*100 {
				return nil, fmt.Errorf("simulated failure")
			}
			return "success", nil
		})

		if err == autobreaker.ErrOpenState {
			rejectedCount++
		} else if err != nil {
			failureCount++
		} else {
			successCount++
		}

		time.Sleep(10 * time.Millisecond)
	}

	metrics := breaker.Metrics()
	log.Printf("Completed %d requests: %d success, %d failures, %d rejected (circuit %s, failure rate: %.1f%%)",
		count, successCount, failureCount, rejectedCount, metrics.State, metrics.FailureRate*100)
}
