---
title: "Error Classification"
weight: 3
---

# Error Classification

AutoBreaker distinguishes between successful and failed requests to make protection decisions.

## Default Classification

**Rule:** Error determines success

```go
func defaultIsSuccessful(err error) bool {
    return err == nil
}
```

- `err == nil` → Success (count as success)
- `err != nil` → Failure (count as failure)

## Custom Classification

Provide your own `IsSuccessful` function:

```go
breaker := autobreaker.New(autobreaker.Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // Don't count 4xx client errors as failures
        var httpErr *HTTPError
        if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
            return true
        }

        return false
    },
})
```

### Common Use Cases

**Ignore specific errors:**
```go
IsSuccessful: func(err error) bool {
    if err == nil {
        return true
    }
    // 404 Not Found is not a failure
    if errors.Is(err, ErrNotFound) {
        return true
    }
    return false
}
```

**Only count critical errors:**
```go
IsSuccessful: func(err error) bool {
    if err == nil {
        return true
    }
    // Only count timeouts and network errors
    if errors.Is(err, context.DeadlineExceeded) {
        return false
    }
    var netErr net.Error
    if errors.As(err, &netErr) {
        return false
    }
    return true
}
```

## Special Cases

### Panic Handling

**Rule:** Panics are treated as failures

```go
defer func() {
    if r := recover(); r != nil {
        // Record as failure
        cb.recordFailure()
        // Re-panic to preserve stack trace
        panic(r)
    }
}()
```

### Context Cancellation

**Default:** Context cancellation/deadline is treated as failure

**Override if needed:**
```go
IsSuccessful: func(err error) bool {
    if err == nil {
        return true
    }
    // Don't penalize service for client cancellation
    if errors.Is(err, context.Canceled) {
        return true
    }
    return false
}
```

## Error Type Taxonomy

### HTTP Status Codes

| Status Range | Default Classification | Rationale |
|--------------|----------------|-----------|
| 2xx | Success | Normal response |
| 3xx | Success | Redirects |
| 4xx | Success* | Client error (not service issue) |
| 429 | Failure* | Rate limit (service overload signal) |
| 5xx | Failure | Server error |
| Timeout | Failure | No response received |

*Can override with `IsSuccessful`

### gRPC Status Codes

| Status | Default Classification |
|--------|----------------|
| OK | Success |
| Canceled | Success* |
| InvalidArgument | Success* |
| NotFound | Success* |
| ResourceExhausted | Failure |
| Internal | Failure |
| Unavailable | Failure |
| DeadlineExceeded | Failure |

*Highly use-case dependent

## Best Practices

### 1. Focus on Service Health
Circuit should trip when service is unhealthy, not on client errors.

### 2. Consistent Classification
Same error should always be classified the same way.

### 3. Avoid Stateful Classification
Classification should be a pure function of the error, not depend on external state.

### 4. Keep It Fast
`IsSuccessful` executes on every request - keep it <1μs.

## Testing

```go
func TestErrorClassification(t *testing.T) {
    tests := []struct {
        name     string
        err      error
        expected bool
    }{
        {"nil error is success", nil, true},
        {"generic error is failure", errors.New("fail"), false},
        {"context deadline is failure", context.DeadlineExceeded, false},
        {"not found can be success", ErrNotFound, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cb := New(Settings{
                IsSuccessful: customClassifier,
            })
            result := cb.isSuccessful(tt.err)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
            }
        })
    }
}
```

## Edge Cases

- **Nil `IsSuccessful` function:** Uses default (`err == nil`)
- **Panic in `IsSuccessful`:** Caught and treated as failure
- **Slow `IsSuccessful`:** User responsibility - keep it fast
