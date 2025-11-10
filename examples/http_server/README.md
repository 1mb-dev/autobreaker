# HTTP Server with Circuit Breaker Middleware

This example demonstrates how to protect HTTP server endpoints with circuit breakers to prevent cascading failures when downstream dependencies become slow or unresponsive.

## Overview

The example shows:

- **Per-Dependency Circuit Breakers**: Separate breakers for database, external APIs, etc.
- **Middleware Pattern**: Wrapping handlers with circuit breaker protection
- **Graceful Degradation**: Returning cached/fallback data when circuits are open
- **Health Checks**: Exposing circuit breaker status via health endpoint
- **Context Integration**: Using `ExecuteContext` with request context

## Key Components

### Multiple Circuit Breakers

```go
type Application struct {
    dbBreaker  *autobreaker.CircuitBreaker  // Database circuit
    apiBreaker *autobreaker.CircuitBreaker  // External API circuit
}
```

Each dependency gets its own circuit breaker with tailored settings:

- **Database**: 10% failure threshold, 10s timeout
- **External API**: 15% failure threshold (more lenient), 15s timeout

### Middleware Pattern

```go
type CircuitBreakerMiddleware struct {
    breaker *autobreaker.CircuitBreaker
    handler http.Handler
}
```

Wraps handlers to provide circuit breaker protection at the HTTP layer.

### Graceful Degradation

```go
if err == autobreaker.ErrOpenState {
    // Circuit is open - return cached/fallback data
    return cachedData, nil
}
```

## Running the Example

```bash
cd examples/http_server
go run main.go
```

The server starts on `:8080` with these endpoints:

- `GET /health` - Health check with circuit status
- `GET /user` - User endpoint (database circuit breaker)
- `GET /data` - Data endpoint (external API circuit breaker)

## Endpoints

### Health Check

```bash
curl http://localhost:8080/health
```

Returns:
```json
{
  "status": "healthy",
  "circuits": {
    "database": {
      "state": "closed",
      "failure_rate": "2.50%",
      "requests": 40
    },
    "external_api": {
      "state": "open",
      "failure_rate": "15.30%",
      "requests": 150
    }
  }
}
```

Status codes:
- `200 OK`: All circuits healthy
- `503 Service Unavailable`: One or more circuits open

### User Endpoint (Database)

```bash
curl http://localhost:8080/user
```

Protected by database circuit breaker:

**Success (Circuit Closed)**:
```json
{
  "user_id": 123,
  "name": "John Doe"
}
```

**Circuit Open**:
```json
{
  "error": "Database temporarily unavailable"
}
```

### Data Endpoint (External API)

```bash
curl http://localhost:8080/data
```

Protected by external API circuit breaker with fallback:

**Success (Circuit Closed)**:
```json
{
  "data": "API response",
  "cached": false
}
```

**Circuit Open (Fallback)**:
```json
{
  "data": "fallback data",
  "cached": true
}
```

## What It Demonstrates

### 1. Per-Dependency Protection

Different dependencies have different circuit breakers:

```go
// Database circuit
app.dbBreaker.ExecuteContext(ctx, func() (interface{}, error) {
    return nil, app.queryDatabase(ctx)
})

// API circuit
app.apiBreaker.ExecuteContext(ctx, func() (interface{}, error) {
    return app.callExternalAPI(ctx)
})
```

### 2. Automatic State Transitions

Watch the logs for circuit state changes:

```
🔌 Circuit database: closed → open
🌐 Circuit external-api: open → half-open
🌐 Circuit external-api: half-open → closed
```

### 3. Traffic Simulation

The example includes background traffic to demonstrate circuit behavior:

```go
go func() {
    for i := 0; i < 100; i++ {
        http.Get("http://localhost:8080/user")
        http.Get("http://localhost:8080/data")
        time.Sleep(100 * time.Millisecond)
    }
}()
```

## Integration Patterns

### Pattern 1: Per-Endpoint Circuit Breakers

```go
mux.Handle("/api/users",
    NewCircuitBreakerMiddleware(userBreaker, userHandler))
mux.Handle("/api/orders",
    NewCircuitBreakerMiddleware(orderBreaker, orderHandler))
```

### Pattern 2: Per-Dependency Circuit Breakers

```go
type App struct {
    db  *autobreaker.CircuitBreaker
    api *autobreaker.CircuitBreaker
}

func (app *App) handleRequest(w http.ResponseWriter, r *http.Request) {
    // Use database circuit
    dbResult, _ := app.db.ExecuteContext(r.Context(), ...)

    // Use API circuit
    apiResult, _ := app.api.ExecuteContext(r.Context(), ...)
}
```

### Pattern 3: Global Circuit Breaker

```go
breaker := autobreaker.New(...)
handler := NewCircuitBreakerMiddleware(breaker, mux)
http.ListenAndServe(":8080", handler)
```

## Advanced: Dynamic Circuit Configuration

Update circuit settings at runtime without restart:

```go
// Increase threshold during maintenance
app.dbBreaker.UpdateSettings(autobreaker.SettingsUpdate{
    FailureRateThreshold: autobreaker.Float64Ptr(0.25), // 10% → 25%
})

// Adjust timeout based on load
app.apiBreaker.UpdateSettings(autobreaker.SettingsUpdate{
    Timeout: autobreaker.DurationPtr(30 * time.Second), // 15s → 30s
})
```

## Monitoring Integration

Export circuit breaker metrics:

```go
func (app *Application) handleMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := app.dbBreaker.Metrics()

    // Export to Prometheus, Datadog, etc.
    circuitStateGauge.Set(float64(metrics.State))
    failureRateGauge.Set(metrics.FailureRate)
    requestCounter.Add(float64(metrics.Counts.Requests))
}
```

## Best Practices

✅ **Separate Circuits**: One circuit per dependency (database, API, etc.)
✅ **Graceful Degradation**: Return cached/fallback data when circuit is open
✅ **Health Checks**: Expose circuit status for monitoring
✅ **Context Usage**: Use `ExecuteContext` with request context
✅ **Error Classification**: Distinguish temporary vs permanent failures
✅ **Observability**: Log state transitions for debugging
✅ **Adaptive Thresholds**: Use percentage-based thresholds for variable traffic

## Benefits

✅ **Prevent Cascading Failures**: Isolate failing dependencies
✅ **Fast Recovery**: Automatic probing for dependency recovery
✅ **Graceful Degradation**: Serve degraded responses instead of errors
✅ **Observable**: Health endpoint shows circuit status
✅ **Configurable**: Per-dependency settings for optimal protection
✅ **Production Ready**: Tested under load with stress tests

## See Also

- [HTTP Client Example](../http_client/) - Circuit breaker for HTTP clients
- [Basic Example](../basic/) - Circuit breaker fundamentals
- [Observability Example](../observability/) - Monitoring and diagnostics
