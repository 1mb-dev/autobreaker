# Runtime Configuration Example

This example demonstrates how to update circuit breaker settings at runtime without restarting your application.

## What This Example Shows

Three methods for runtime configuration updates:

1. **Programmatic Updates** - Update settings directly in code
2. **File-Based Configuration** - Load settings from JSON files with SIGHUP reload
3. **HTTP API** - Update settings via REST API endpoints

## Running the Example

```bash
go run main.go
```

The example will:
- Create an initial circuit breaker with default settings
- Generate a config file at `/tmp/circuit_breaker_config.json`
- Start an HTTP server on port 8081
- Run through 5 scenarios demonstrating different update methods
- Keep running for interactive testing

## Scenarios Demonstrated

### Scenario 1: Normal Operation
- Simulates 20 requests with 2% failure rate
- Circuit stays closed (below 5% threshold)

### Scenario 2: Programmatic Update
- Updates failure threshold from 5% to 15% via code
- Shows direct use of `UpdateSettings()` API

### Scenario 3: Higher Failure Rate
- Simulates 30 requests with 12% failure rate
- Circuit stays closed (below new 15% threshold)
- Demonstrates effect of runtime configuration

### Scenario 4: File-Based Update
- Modifies config file to set threshold back to 5%
- Reloads configuration from file
- Shows file-based configuration pattern

### Scenario 5: Circuit Trips
- Simulates 30 requests with 8% failure rate
- Circuit trips (exceeds 5% threshold)
- Shows circuit protection with new settings

## Interactive Testing

After the demo completes, you can test runtime updates interactively:

### 1. View Current Configuration

```bash
curl http://localhost:8081/config
```

Response:
```json
{
  "max_requests": 5,
  "interval": "15s",
  "timeout": "60s",
  "failure_rate_threshold": 0.05,
  "minimum_observations": 30,
  "current_state": "open",
  "metrics": {
    "State": 1,
    "Counts": {...},
    "FailureRate": 0.08,
    ...
  }
}
```

### 2. Update Configuration via API

Update failure threshold:
```bash
curl -X POST http://localhost:8081/config/update \
  -H "Content-Type: application/json" \
  -d '{"failure_rate_threshold": 0.20}'
```

Update multiple settings:
```bash
curl -X POST http://localhost:8081/config/update \
  -H "Content-Type: application/json" \
  -d '{
    "failure_rate_threshold": 0.10,
    "timeout": "2m",
    "max_requests": 10
  }'
```

### 3. Reload from File

Modify `/tmp/circuit_breaker_config.json`:
```json
{
  "max_requests": 3,
  "interval": "30s",
  "timeout": "5m",
  "failure_rate_threshold": 0.15,
  "minimum_observations": 50
}
```

Then reload:
```bash
curl -X POST http://localhost:8081/config/reload
```

Or send SIGHUP signal:
```bash
kill -HUP <PID>
```

## Configuration File Format

JSON configuration file supports all updateable settings:

```json
{
  "max_requests": 5,
  "interval": "15s",
  "timeout": "30s",
  "failure_rate_threshold": 0.10,
  "minimum_observations": 30
}
```

**Fields:**
- `max_requests` (uint32) - Max concurrent requests in half-open state
- `interval` (duration) - Observation window for counts (e.g., "10s", "1m")
- `timeout` (duration) - How long circuit stays open (e.g., "30s", "2m")
- `failure_rate_threshold` (float64) - Failure rate to trip (0.0-1.0, e.g., 0.05 = 5%)
- `minimum_observations` (uint32) - Min requests before adaptive logic activates

**Notes:**
- All fields are optional (omitted fields keep current values)
- Durations use Go duration syntax: "10s", "5m", "1h30m"
- Changing `interval` resets counts immediately
- Changing `timeout` while circuit is open restarts the timeout

## Production Patterns

### Pattern 1: Configuration Service Integration

```go
// Load config from Consul, etcd, etc.
func loadFromConsul(breaker *autobreaker.CircuitBreaker) error {
    config := fetchFromConsul("services/api/circuit-breaker")

    update := autobreaker.SettingsUpdate{
        FailureRateThreshold: &config.Threshold,
        Timeout:              &config.Timeout,
    }

    return breaker.UpdateSettings(update)
}

// Watch for changes
consulClient.Watch("services/api/circuit-breaker", func(config Config) {
    if err := loadFromConsul(breaker); err != nil {
        log.Printf("Config update failed: %v", err)
    }
})
```

### Pattern 2: A/B Testing Different Thresholds

```go
// Feature flag controls which threshold to use
func updateForExperiment(breaker *autobreaker.CircuitBreaker, userID string) {
    threshold := 0.05 // Control group: 5%

    if featureFlags.IsInExperiment(userID, "circuit-breaker-threshold") {
        threshold = 0.10 // Experiment group: 10%
    }

    breaker.UpdateSettings(autobreaker.SettingsUpdate{
        FailureRateThreshold: autobreaker.Float64Ptr(threshold),
    })
}
```

### Pattern 3: Adaptive Tuning Based on Metrics

```go
// Auto-tune based on observed behavior
func autoTune(breaker *autobreaker.CircuitBreaker) {
    metrics := breaker.Metrics()

    // If circuit is flapping (opening/closing too often), make less sensitive
    if isFlapping(metrics) {
        currentThreshold := breaker.Diagnostics().FailureRateThreshold
        newThreshold := currentThreshold * 1.5 // Increase by 50%

        breaker.UpdateSettings(autobreaker.SettingsUpdate{
            FailureRateThreshold: autobreaker.Float64Ptr(newThreshold),
        })

        log.Printf("Circuit flapping detected, increasing threshold to %.2f%%", newThreshold*100)
    }
}
```

### Pattern 4: Environment-Specific Configuration

```go
func loadEnvironmentConfig(breaker *autobreaker.CircuitBreaker, env string) error {
    configs := map[string]autobreaker.SettingsUpdate{
        "dev": {
            FailureRateThreshold: autobreaker.Float64Ptr(0.20), // Lenient
            Timeout:              autobreaker.DurationPtr(5 * time.Second),
        },
        "staging": {
            FailureRateThreshold: autobreaker.Float64Ptr(0.10),
            Timeout:              autobreaker.DurationPtr(15 * time.Second),
        },
        "production": {
            FailureRateThreshold: autobreaker.Float64Ptr(0.05), // Strict
            Timeout:              autobreaker.DurationPtr(30 * time.Second),
        },
    }

    update, ok := configs[env]
    if !ok {
        return fmt.Errorf("unknown environment: %s", env)
    }

    return breaker.UpdateSettings(update)
}
```

## Validation and Error Handling

UpdateSettings() validates all settings before applying:

```go
err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
    FailureRateThreshold: autobreaker.Float64Ptr(1.5), // INVALID
})

if err != nil {
    log.Printf("Validation failed: %v", err)
    // Error: FailureRateThreshold must be in range (0, 1)
    // Original settings preserved - no partial updates
}
```

**Validation Rules:**
- `MaxRequests` must be > 0
- `Interval` must be >= 0
- `Timeout` must be > 0
- `FailureRateThreshold` must be in (0, 1) exclusive
- `MinimumObservations` must be > 0

If validation fails, **no settings are changed** (all-or-nothing).

## Thread Safety

UpdateSettings() is fully thread-safe:

```go
// Safe to call from multiple goroutines
go func() {
    breaker.UpdateSettings(...)  // Goroutine 1
}()

go func() {
    breaker.UpdateSettings(...)  // Goroutine 2
}()

// Safe to update while Execute() is running
go func() {
    breaker.Execute(...)  // Concurrent with updates
}()
```

All updates use atomic operations - no locks needed.

## Best Practices

### DO:
✅ Update settings during incidents to adjust circuit sensitivity
✅ Use file-based config for ops teams to tune without code changes
✅ Validate configuration before applying
✅ Log all configuration changes for audit trail
✅ Monitor metrics after updates to verify effectiveness

### DON'T:
❌ Update settings on every request (use infrequently)
❌ Change all settings at once (update incrementally)
❌ Ignore validation errors (always check return value)
❌ Update settings faster than observation window
❌ Use for application logic (settings are for tuning, not behavior)

## Learn More

- See `examples/observability/` for monitoring patterns
- See `examples/prometheus/` for metrics integration
- See main README for API documentation
