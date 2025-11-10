# HTTP Client with Circuit Breaker

This example demonstrates how to integrate AutoBreaker with HTTP clients to protect your application from slow or failing external services.

## Overview

The example implements a custom `http.RoundTripper` that wraps requests with circuit breaker protection. This ensures:

- **Fail Fast**: When a service is unhealthy, requests fail immediately instead of waiting for timeouts
- **Automatic Recovery**: The circuit automatically probes for recovery after the timeout period
- **Context Support**: Proper cancellation and timeout handling using `ExecuteContext`
- **Smart Error Classification**: Distinguishes between client errors (4xx) and server errors (5xx)

## Key Components

### CircuitBreakerRoundTripper

```go
type CircuitBreakerRoundTripper struct {
    breaker   *autobreaker.CircuitBreaker
    transport http.RoundTripper
}
```

Implements `http.RoundTripper` interface with circuit breaker protection:

- Wraps each HTTP request with `ExecuteContext`
- Returns 503 Service Unavailable when circuit is open
- Preserves request context for proper cancellation
- Works with any `http.Transport` (or uses default)

### Error Classification

The example shows how to differentiate between error types:

- **5xx Server Errors**: Count as failures, can trip the circuit
- **4xx Client Errors**: Don't indicate backend problems, shouldn't trip circuit
- **Network Errors**: Count as failures
- **Context Cancellation**: Not counted as failures (client-initiated)

## Running the Example

```bash
cd examples/http_client
go run main.go
```

## What It Demonstrates

### 1. Successful Requests

```go
client := NewProtectedHTTPClient("external-api")
resp, err := client.Get("https://httpbin.org/status/200")
// **Success: Status 200 OK
```

### 2. Circuit Opens on Failures

After repeated 5xx errors:
```
**Server error: Request 1 returned 500 Internal Server Error
**Server error: Request 2 returned 500 Internal Server Error
...
⚡ Circuit open: Request 15 rejected (fail fast)
```

### 3. Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
defer cancel()

req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
resp, err = client.Do(req)
// **Request cancelled: context deadline exceeded (not counted as failure)
```

### 4. Client Errors Don't Trip Circuit

```go
resp, err := client.Get("https://httpbin.org/status/404")
// Note: Client error: 404 Not Found (doesn't trip circuit)
```

## Integration Pattern

```go
// 1. Create circuit breaker with adaptive thresholds
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "my-api",
    Timeout:              10 * time.Second,
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.10, // 10% failure rate
    MinimumObservations:  20,
    OnStateChange: func(name string, from, to autobreaker.State) {
        log.Printf("Circuit %s: %s → %s", name, from, to)
    },
})

// 2. Wrap http.Client with circuit breaker
client := &http.Client{
    Transport: NewCircuitBreakerRoundTripper(breaker, nil),
    Timeout:   30 * time.Second,
}

// 3. Use client normally
resp, err := client.Get("https://api.example.com/data")
```

## Advanced: Custom Error Classification

For more sophisticated error handling:

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name: "api-client",
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // Don't count 4xx as failures
        var httpErr *HTTPError
        if errors.As(err, &httpErr) {
            return httpErr.StatusCode < 500
        }

        // Don't count context cancellation as failure
        if errors.Is(err, context.Canceled) {
            return true
        }

        return false
    },
})
```

## Benefits

****Fail Fast**: Reject requests immediately when service is down
****Automatic Recovery**: Test recovery automatically after timeout
****Context Aware**: Proper cancellation and deadline support
****Adaptive**: Same config works at any traffic level
****Observable**: State change callbacks for monitoring
****Drop-in**: Works with existing `http.Client` code

## See Also

- [HTTP Server Example](../http_server/) - Middleware for protecting server endpoints
- [Basic Example](../basic/) - Circuit breaker fundamentals
- [Adaptive Example](../adaptive/) - Adaptive vs static thresholds
