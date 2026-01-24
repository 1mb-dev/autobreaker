---
title: "Configuration"
weight: 2
---

# Configuration

AutoBreaker provides flexible configuration options for different use cases.

## Basic Settings

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:    "service-name",
    Timeout: 30 * time.Second, // Open → HalfOpen transition time
})
```

Default behavior uses adaptive thresholds (5% failure rate, minimum 20 observations).

## Advanced Configuration

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "service-name",
    Timeout:              30 * time.Second,
    MaxRequests:          3,     // Concurrent requests in half-open state
    Interval:             60 * time.Second, // Stats reset interval

    // Adaptive threshold settings
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.05,  // Trip at 5% error rate
    MinimumObservations:  20,    // Minimum requests before evaluating

    // Callbacks (must be fast: <1μs)
    OnStateChange: func(name string, from, to autobreaker.State) {
        go log.Printf("%s: %v → %v", name, from, to) // Async
    },

    IsSuccessful: func(err error) bool {
        // Don't count 4xx client errors as failures
        if httpErr, ok := err.(*HTTPError); ok {
            return httpErr.Code >= 400 && httpErr.Code < 500
        }
        return err == nil
    },
})
```

## Runtime Updates

Update settings without restarting:

```go
err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
    FailureRateThreshold: autobreaker.Float64Ptr(0.10),
    Timeout:              autobreaker.DurationPtr(15 * time.Second),
})
```

## Settings Reference

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `Name` | `string` | Required | Circuit breaker identifier |
| `Timeout` | `time.Duration` | `60 * time.Second` | Open → HalfOpen transition time |
| `MaxRequests` | `uint32` | `1` | Concurrent requests in half-open state |
| `Interval` | `time.Duration` | `0` (disabled) | Statistics reset interval |
| `AdaptiveThreshold` | `bool` | `true` | Enable percentage-based thresholds |
| `FailureRateThreshold` | `float64` | `0.05` | Trip at >5% error rate |
| `MinimumObservations` | `uint32` | `20` | Minimum requests before evaluating |
| `OnStateChange` | `func(string, State, State)` | `nil` | State change callback |
| `IsSuccessful` | `func(error) bool` | `nil` | Custom error classification |

## Performance Considerations

**Callback Performance**: `ReadyToTrip`, `OnStateChange`, and `IsSuccessful` callbacks execute synchronously on every request. Keep them <1μs.

```go
// Good: Async logging
OnStateChange: func(name string, from, to State) {
    go metrics.Record(name, from, to) // Async, non-blocking
}

// Bad: Blocking I/O in callback
OnStateChange: func(name string, from, to State) {
    db.SaveStateChange(name, from, to) // Blocks!
}
```

## Best Practices

1. **Use descriptive names** - Helps with monitoring and debugging
2. **Set reasonable timeouts** - Match your service's typical recovery time
3. **Monitor failure rates** - Adjust thresholds based on observed behavior
4. **Use async callbacks** - Keep the hot path fast
5. **Test with examples** - See [examples](https://github.com/vnykmshr/autobreaker/tree/main/examples) for production patterns
