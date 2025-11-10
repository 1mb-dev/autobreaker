# Observability & Monitoring Example

This example demonstrates how to use AutoBreaker's `Metrics()` and `Diagnostics()` APIs for comprehensive observability, monitoring, and troubleshooting.

## What This Example Shows

Four real-world scenarios demonstrating:

1. **Normal Operation Monitoring** - Track metrics during healthy operation
2. **Diagnostic Troubleshooting** - Use Diagnostics API to debug circuit behavior
3. **Real-Time Monitoring** - Watch circuit state during service degradation
4. **Recovery Monitoring** - Observe circuit recovery process

## Running the Example

```bash
go run main.go
```

## Scenarios Explained

### Scenario 1: Normal Operation Monitoring

Shows how to:
- Collect metrics after a batch of requests
- Calculate success/failure rates
- Determine system health
- Log metrics for monitoring

**Use Case**: Periodic health checks, dashboards

### Scenario 2: Diagnostic Troubleshooting

Demonstrates:
- Using `Diagnostics()` to see full circuit state
- Viewing current configuration
- Predicting circuit behavior ("Will trip next?")
- Understanding why circuit might trip

**Use Case**: Debugging production issues, understanding circuit sensitivity

### Scenario 3: Real-Time Monitoring

Shows:
- Monitoring metrics during active degradation
- Detecting when circuit will trip
- Watching failure rate increase
- Seeing circuit protection kick in

**Use Case**: Active incident response, real-time dashboards

### Scenario 4: Recovery Monitoring

Demonstrates:
- Monitoring circuit during Open state
- Tracking time until half-open
- Observing recovery process
- Measuring recovery time

**Use Case**: Incident resolution, SLA tracking

## Key APIs Used

### Metrics() API

```go
metrics := breaker.Metrics()

// Access current state
fmt.Printf("State: %v\n", metrics.State)

// Get computed rates
fmt.Printf("Failure Rate: %.1f%%\n", metrics.FailureRate*100)

// Access counts
fmt.Printf("Requests: %d\n", metrics.Counts.Requests)

// Check timestamps
fmt.Printf("State changed: %v ago\n", time.Since(metrics.StateChangedAt))
```

**What it provides:**
- Current state and counts
- Calculated failure/success rates
- Timestamps for state changes and count resets

### Diagnostics() API

```go
diag := breaker.Diagnostics()

// View configuration
fmt.Printf("Threshold: %.1f%%\n", diag.FailureRateThreshold*100)

// Predict behavior
if diag.WillTripNext {
    fmt.Println("WARNING: Next failure will trip!")
}

// Check recovery time
if diag.State == autobreaker.StateOpen {
    fmt.Printf("Half-open in: %v\n", diag.TimeUntilHalfOpen)
}
```

**What it provides:**
- All Metrics data
- Full configuration
- Predictive analytics (WillTripNext)
- Time until half-open

## Integration Patterns

### Health Check Endpoint

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    metrics := breaker.Metrics()

    health := map[string]interface{}{
        "status":       "healthy",
        "circuit_state": metrics.State.String(),
        "failure_rate":  metrics.FailureRate,
    }

    if metrics.State != autobreaker.StateClosed {
        health["status"] = "degraded"
    }

    json.NewEncoder(w).Encode(health)
}
```

### Structured Logging

```go
logger.Info("circuit_breaker_metrics",
    "state", metrics.State.String(),
    "requests", metrics.Counts.Requests,
    "failures", metrics.Counts.TotalFailures,
    "failure_rate", metrics.FailureRate,
    "state_changed_at", metrics.StateChangedAt,
)
```

### Alerting Logic

```go
metrics := breaker.Metrics()
diag := breaker.Diagnostics()

// Alert on circuit open
if metrics.State == autobreaker.StateOpen {
    alert.Send("Circuit breaker opened for " + diag.Name)
}

// Alert on approaching threshold
if diag.WillTripNext {
    alert.Send("Circuit breaker about to trip")
}

// Alert on high failure rate
if metrics.FailureRate > 0.10 {
    alert.Send(fmt.Sprintf("High failure rate: %.1f%%", metrics.FailureRate*100))
}
```

### Dashboard Metrics

```go
// Periodic metrics collection for dashboard
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    metrics := breaker.Metrics()

    // Send to metrics system
    statsd.Gauge("circuit.state", int(metrics.State))
    statsd.Gauge("circuit.failure_rate", metrics.FailureRate)
    statsd.Count("circuit.requests", int64(metrics.Counts.Requests))
    statsd.Count("circuit.failures", int64(metrics.Counts.TotalFailures))
}
```

## Production Recommendations

### What to Monitor

**Critical Metrics:**
- Circuit state (alert on Open)
- Failure rate (alert on threshold approach)
- State change frequency (alert on flapping)

**Helpful Metrics:**
- Request throughput
- Success/failure counts
- Time in each state

### Logging Best Practices

```go
// Log state changes (already built-in via OnStateChange)
OnStateChange: func(name string, from, to autobreaker.State) {
    logger.Warn("circuit_state_change",
        "breaker", name,
        "from", from.String(),
        "to", to.String(),
    )
}

// Log metrics periodically
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        metrics := breaker.Metrics()
        logger.Info("circuit_metrics",
            "state", metrics.State.String(),
            "failure_rate", metrics.FailureRate,
        )
    }
}()
```

### When to Use Each API

**Use Metrics():**
- Real-time monitoring
- Health checks
- Dashboards
- Periodic logging

**Use Diagnostics():**
- Troubleshooting production issues
- Understanding circuit configuration
- Predicting circuit behavior
- Debugging unexpected trips

## Learn More

- See `examples/prometheus/` for Prometheus integration
- See `examples/production_ready/` for comprehensive scenarios
- See main README for API documentation
