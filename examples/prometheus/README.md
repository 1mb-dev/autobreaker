# Prometheus Integration Example

This example demonstrates how to expose AutoBreaker circuit breaker metrics to Prometheus for monitoring and alerting.

## What This Example Shows

- Creating a custom Prometheus collector for circuit breaker metrics
- Exporting all circuit breaker statistics as Prometheus metrics
- Running a metrics HTTP endpoint
- Real-time metric updates as circuit state changes

## Running the Example

```bash
# Install dependencies
go mod download

# Run the example
go run main.go
```

The example will:
1. Start a circuit breaker making API calls in the background
2. Expose Prometheus metrics at http://localhost:8080/metrics
3. Show request results in console output

## Viewing Metrics

Open http://localhost:8080/metrics in your browser or use curl:

```bash
curl http://localhost:8080/metrics | grep circuit_breaker
```

## Metrics Exported

### Gauges (Current Values)

- `circuit_breaker_state` - Current state (0=closed, 1=open, 2=half-open)
- `circuit_breaker_consecutive_successes` - Current consecutive successes
- `circuit_breaker_consecutive_failures` - Current consecutive failures
- `circuit_breaker_failure_rate` - Current failure rate (0.0-1.0)
- `circuit_breaker_success_rate` - Current success rate (0.0-1.0)

### Counters (Cumulative)

- `circuit_breaker_requests_total` - Total requests attempted
- `circuit_breaker_successes_total` - Total successful requests
- `circuit_breaker_failures_total` - Total failed requests

## Example Prometheus Queries

```promql
# Current circuit breaker state
circuit_breaker_state{name="api-client"}

# Failure rate over time
circuit_breaker_failure_rate{name="api-client"}

# Rate of failures per second
rate(circuit_breaker_failures_total{name="api-client"}[1m])

# Requests per second
rate(circuit_breaker_requests_total{name="api-client"}[1m])
```

## Example Prometheus Alerts

```yaml
groups:
  - name: circuit_breaker_alerts
    rules:
      # Alert when circuit opens
      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state{name="api-client"} == 1
        for: 1m
        annotations:
          summary: "Circuit breaker {{ $labels.name }} is OPEN"
          description: "The circuit breaker has tripped due to failures"

      # Alert on high failure rate
      - alert: HighFailureRate
        expr: circuit_breaker_failure_rate{name="api-client"} > 0.10
        for: 2m
        annotations:
          summary: "High failure rate on {{ $labels.name }}"
          description: "Failure rate is {{ $value | humanizePercentage }}"
```

## Integration Pattern

The key pattern demonstrated here is the **custom Prometheus collector**:

```go
type CircuitBreakerCollector struct {
    breaker *autobreaker.CircuitBreaker
    // ... metric descriptors
}

func (c *CircuitBreakerCollector) Collect(ch chan<- prometheus.Metric) {
    metrics := c.breaker.Metrics()

    // Export each metric
    ch <- prometheus.MustNewConstMetric(
        c.stateDesc,
        prometheus.GaugeValue,
        float64(metrics.State),
    )
    // ... more metrics
}

// Register with Prometheus
prometheus.MustRegister(NewCircuitBreakerCollector(breaker))
```

## Why This Pattern?

**Advantages:**
- **No Prometheus dependency in main library
- **Users control what metrics to export
- **Flexible - can customize metric names, labels
- **Efficient - metrics computed on scrape, not continuously

**vs Built-in Integration:**
- **Would force Prometheus dependency
- **Less flexible for users
- **Can't customize to user's needs

## Production Recommendations

### Metric Labels

Add more labels for better grouping:

```go
prometheus.Labels{
    "name":        name,
    "environment": "production",
    "service":     "api-gateway",
    "version":     "v1.2.3",
}
```

### Multiple Circuit Breakers

Track multiple breakers in one collector:

```go
type MultiCircuitBreakerCollector struct {
    breakers map[string]*autobreaker.CircuitBreaker
}
```

### Histogram Metrics

Add request duration histograms:

```go
requestDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "circuit_breaker_request_duration_seconds",
    },
    []string{"name", "state"},
)
```

## Grafana Dashboard

Example Grafana panels:

1. **Circuit Breaker State** - Gauge showing current state
2. **Failure Rate** - Graph of failure rate over time
3. **Requests/sec** - Rate of requests
4. **State Changes** - Count of state transitions

## Learn More

- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Writing Exporters](https://prometheus.io/docs/instrumenting/writing_exporters/)
- [PromQL Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
