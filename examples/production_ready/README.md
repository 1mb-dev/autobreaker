# Production-Ready Adaptive Circuit Breaker Example

This comprehensive example demonstrates how to use AutoBreaker's adaptive thresholds in production scenarios.

## What This Example Shows

### Real-World Scenarios

1. **Normal Operation**: Circuit remains stable under normal 2% failure rate
2. **Service Degradation**: Circuit trips when failure rate exceeds threshold (5%)
3. **Failure Spike**: Circuit handles bursts of failures and recovers automatically
4. **Adaptive vs Static**: Side-by-side comparison showing adaptive advantages

### Key Features Demonstrated

- **Traffic-Proportional Protection**: Same config works from dev (5 req/s) to production (500 req/s)
- **Distributed Failure Detection**: Catches failures that aren't consecutive
- **Automatic Recovery**: Circuit closes when service health improves
- **Observable Behavior**: Clear metrics and state transitions

## Running the Example

```bash
go run main.go
```

## Configuration Explained

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "api-client",
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.05,  // Trip at 5% failure rate
    MinimumObservations:  20,    // Need 20+ requests before evaluating
    Timeout:              1 * time.Second,
})
```

### Why These Values?

- **5% threshold**: Balanced sensitivity - catches problems without false positives
- **20 minimum observations**: Prevents premature tripping during low traffic
- **1 second timeout**: Quick recovery attempts (adjust based on your service)

## Expected Output

The example demonstrates:

1. ****Stability** under normal conditions (2% failure rate)
2. ****Protection** during degradation (15% failure rate triggers circuit)
3. ****Recovery** when service improves
4. ****Superiority** over static thresholds (catches distributed failures)

## Production Recommendations

### Typical Threshold Values

```go
// Critical services - low tolerance
FailureRateThreshold: 0.01  // 1%

// Standard services - balanced
FailureRateThreshold: 0.05  // 5%

// Resilient services - high tolerance
FailureRateThreshold: 0.10  // 10%
```

### Minimum Observations

```go
// High-traffic services (>100 req/s)
MinimumObservations: 10-20

// Medium-traffic services (10-100 req/s)
MinimumObservations: 20-50

// Low-traffic services (<10 req/s)
MinimumObservations: 50-100
```

### Timeout Configuration

```go
// Fast-recovering services (APIs, databases)
Timeout: 5 * time.Second

// Slow-recovering services (batch jobs, external APIs)
Timeout: 30 * time.Second

// Very slow services (cold starts, deploys)
Timeout: 60 * time.Second
```

## Comparison: Adaptive vs Static

### Adaptive Advantages

- **Traffic-agnostic**: Works at any request rate
- **Catches distributed failures**: Doesn't require consecutive failures
- **No tuning needed**: Same config across environments
- **Percentage-based**: Intuitive threshold (5% = 5 failures per 100 requests)

### Static Limitations

- **Traffic-dependent**: Must tune for each environment
- **Misses patterns**: Only detects consecutive failures
- **Requires updates**: Need to adjust when traffic changes
- **Absolute counts**: "5 failures" means different things at different scales

## Integration Example

```go
type APIClient struct {
    baseURL string
    breaker *autobreaker.CircuitBreaker
}

func NewAPIClient(baseURL string) *APIClient {
    return &APIClient{
        baseURL: baseURL,
        breaker: autobreaker.New(autobreaker.Settings{
            Name:                 "api-client",
            AdaptiveThreshold:    true,
            FailureRateThreshold: 0.05,
            MinimumObservations:  20,
            Timeout:              10 * time.Second,
            OnStateChange: func(name string, from, to autobreaker.State) {
                log.Printf("Circuit %s: %v -> %v", name, from, to)
            },
        }),
    }
}

func (c *APIClient) GetUser(id string) (*User, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.doHTTPRequest("GET", "/users/"+id)
    })

    if err != nil {
        return nil, err
    }

    return result.(*User), nil
}
```

## Monitoring

Track these metrics in production:

```go
counts := breaker.Counts()
state := breaker.State()

// Emit metrics
metrics.Gauge("circuit.requests", counts.Requests)
metrics.Gauge("circuit.failures", counts.TotalFailures)
metrics.Gauge("circuit.failure_rate",
    float64(counts.TotalFailures) / float64(counts.Requests))
metrics.Gauge("circuit.state", int(state))
```

## Learn More

- See `examples/adaptive/` for simpler adaptive threshold demo
- See `examples/basic/` for fundamental circuit breaker patterns
- See `examples/custom_errors/` for error classification strategies
