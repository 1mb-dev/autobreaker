# AutoBreaker Error Classification Rules

## Overview

Circuit breaker must distinguish between successful and failed requests to make protection decisions. This document defines how errors, panics, and edge cases are classified.

## Default Classification

### Rule: Error Determines Success

**Default Implementation:**
```go
func defaultIsSuccessful(err error) bool {
    return err == nil
}
```

**Logic:**
- `err == nil` → Success (count as success, increment consecutive successes)
- `err != nil` → Failure (count as failure, increment consecutive failures)

**Rationale:** Most operations use `nil` error to indicate success (Go convention)

---

## Custom Classification

### User-Provided IsSuccessful Function

**Signature:**
```go
type Settings struct {
    IsSuccessful func(err error) bool
}
```

**Use Cases:**

#### 1. Ignore Specific Errors (Don't Trip Circuit)

```go
Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // 404 Not Found is not a failure (expected response)
        if errors.Is(err, ErrNotFound) {
            return true
        }

        // Client errors (4xx) don't indicate service health issues
        var httpErr *HTTPError
        if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
            return true
        }

        return false
    },
}
```

**Result:** Circuit only trips on actual service failures (5xx, timeouts, network errors)

---

#### 2. Only Count Critical Errors

```go
Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // Only count timeouts and network errors as failures
        if errors.Is(err, context.DeadlineExceeded) {
            return false // Failure
        }

        var netErr net.Error
        if errors.As(err, &netErr) {
            return false // Failure
        }

        return true // Other errors don't count
    },
}
```

**Result:** Circuit only protects against network-level issues

---

#### 3. Business Logic Validation

```go
Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // Validation errors are client mistakes, not failures
        if errors.Is(err, ErrInvalidInput) {
            return true
        }

        // Quota exceeded is expected, not a failure
        if errors.Is(err, ErrQuotaExceeded) {
            return true
        }

        return false
    },
}
```

**Result:** Circuit ignores client-side errors, focuses on service health

---

## Special Cases

### 1. Panic Handling

**Rule:** Panics are treated as failures

**Implementation:**
```go
func (cb *CircuitBreaker) executeWithPanicRecovery(req func() (interface{}, error)) (result interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            // Record as failure
            cb.recordFailure()

            // Re-panic to preserve stack trace
            panic(r)
        }
    }()

    return req()
}
```

**Rationale:**
- Panics indicate code defects or unexpected states
- Service experiencing panics should be protected (circuit open)
- Re-panic preserves debugging information

**User Override:** Not possible - panics always count as failures

---

### 2. Context Cancellation

**Rule:** Context cancellation/deadline is treated as failure (by default)

**Default Behavior:**
```go
err := req()

// err == context.Canceled → Failure
// err == context.DeadlineExceeded → Failure
```

**Rationale:**
- `context.DeadlineExceeded` indicates timeout (service too slow)
- `context.Canceled` may indicate client abandonment, but service may still be slow

**User Override:** Can treat cancellation as success if desired

```go
Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        // Don't penalize service for client cancellation
        if errors.Is(err, context.Canceled) {
            return true
        }

        return false
    },
}
```

---

### 3. Timeout vs Slow Response

**Problem:** Request completes successfully but takes a long time

**Default:** If `err == nil`, counted as success (even if slow)

**Future Enhancement (Phase 2+):** Latency-aware classification

```go
// Potential future feature
Settings{
    LatencyThreshold: 500 * time.Millisecond,
    CountSlowAsFailure: true,
}

// Would treat slow successes as failures
if err == nil && duration > latencyThreshold {
    // Count as failure for circuit breaker purposes
}
```

**Phase 1:** Not implemented - latency tracking is separate concern

---

### 4. Nil Error with Unusual Result

**Problem:** Function returns `nil` error but result is invalid

**Example:**
```go
result, err := fetchUser(id)
// err == nil, but result is empty/corrupted
```

**Default:** Counted as success (err == nil)

**User Solution:** Validate result and return error

```go
breaker.Execute(func() (interface{}, error) {
    result, err := fetchUser(id)
    if err != nil {
        return nil, err
    }

    // Validate result
    if result == nil || result.ID == 0 {
        return nil, ErrInvalidResponse
    }

    return result, nil
})
```

**Alternative:** Custom IsSuccessful that inspects result (not recommended - breaks abstraction)

---

### 5. Partial Failures

**Problem:** Batch operation succeeds partially (e.g., 5/10 items processed)

**Default:** `err == nil` → Success

**Recommended Pattern:** Return error if any items fail

```go
breaker.Execute(func() (interface{}, error) {
    results, failures := processBatch(items)

    if len(failures) > 0 {
        return results, fmt.Errorf("partial failure: %d/%d failed", len(failures), len(items))
    }

    return results, nil
})
```

**Alternative:** Custom threshold

```go
Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }

        var partialErr *PartialFailureError
        if errors.As(err, &partialErr) {
            // Allow up to 20% failure rate in batch
            failureRate := float64(partialErr.Failed) / float64(partialErr.Total)
            return failureRate < 0.20
        }

        return false
    },
}
```

---

## Error Type Taxonomy

### Network Errors (Usually Failures)

```go
var netErr net.Error
if errors.As(err, &netErr) {
    // Connection refused, DNS failure, etc.
    // → Failure (service unreachable)
}
```

### Timeout Errors (Usually Failures)

```go
if errors.Is(err, context.DeadlineExceeded) {
    // Request took too long
    // → Failure (service too slow)
}
```

### HTTP Status Codes

| Status Range | Classification | Rationale |
|--------------|----------------|-----------|
| 2xx | Success | Normal response |
| 3xx | Success* | Redirects (client follows) |
| 4xx | Success* | Client error (not service issue) |
| 429 | Failure* | Rate limit (service overload signal) |
| 5xx | Failure | Server error |
| Timeout | Failure | No response received |

*Default - user may override

### gRPC Status Codes

| Status | Classification | Rationale |
|--------|----------------|-----------|
| OK | Success | Successful RPC |
| Canceled | Success* | Client cancellation |
| InvalidArgument | Success* | Client error |
| NotFound | Success* | Expected response |
| AlreadyExists | Success* | Expected response |
| PermissionDenied | Success* | Auth issue (client) |
| Unauthenticated | Success* | Auth issue (client) |
| ResourceExhausted | Failure | Service overload |
| FailedPrecondition | Success* | Business logic |
| Aborted | Failure | Transaction conflict |
| OutOfRange | Success* | Client error |
| Unimplemented | Success* | API mismatch |
| Internal | Failure | Server error |
| Unavailable | Failure | Service down |
| DataLoss | Failure | Critical error |
| DeadlineExceeded | Failure | Timeout |
| Unknown | Failure | Unexpected error |

*Default - highly use-case dependent

---

## Classification Best Practices

### 1. Focus on Service Health

**Good:** Circuit trips when service is unhealthy
```go
// Trip on 5xx, timeouts, network errors
IsSuccessful: func(err error) bool {
    return err == nil || isClientError(err)
}
```

**Bad:** Circuit trips on client errors
```go
// Trips on 4xx errors (not service issue)
IsSuccessful: func(err error) bool {
    return err == nil
}
```

---

### 2. Consistent Classification

**Good:** Same error always classified the same way
```go
IsSuccessful: func(err error) bool {
    // Deterministic - based on error type only
    if errors.Is(err, ErrNotFound) {
        return true
    }
    return err == nil
}
```

**Bad:** Non-deterministic classification
```go
IsSuccessful: func(err error) bool {
    // Bad: Random classification
    return rand.Float64() > 0.5
}
```

---

### 3. Avoid Stateful Classification

**Good:** Pure function of error
```go
IsSuccessful: func(err error) bool {
    return err == nil || isExpectedError(err)
}
```

**Bad:** Depends on external state
```go
var errorCount int
IsSuccessful: func(err error) bool {
    errorCount++
    // Bad: Classification changes based on history
    return errorCount < 10
}
```

---

## Testing Error Classification

### Test Cases

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
        {"context canceled is failure", context.Canceled, false},
        {"not found can be success", ErrNotFound, true}, // If custom IsSuccessful
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

---

## Edge Cases

### 1. Nil Error Function

**Behavior:** Treat as default (err == nil)

```go
if cb.isSuccessful == nil {
    cb.isSuccessful = defaultIsSuccessful
}
```

---

### 2. IsSuccessful Panics

**Behavior:** Catch panic, treat as failure

```go
func (cb *CircuitBreaker) isSuccessful(err error) bool {
    defer func() {
        if r := recover(); r != nil {
            // Log panic in IsSuccessful, treat as failure
            return false
        }
    }()

    return cb.settings.IsSuccessful(err)
}
```

**Rationale:** Don't crash circuit breaker due to user code

---

### 3. Slow IsSuccessful Function

**Problem:** User-provided function takes a long time

**Mitigation:** None in Phase 1 - user responsibility

**Future Enhancement:** Timeout on classification

---

## Summary

**Error Classification Principles:**
- ✅ Default: `err == nil` → Success
- ✅ User override via `IsSuccessful` function
- ✅ Panics always count as failures
- ✅ Context cancellation counted as failure (by default)
- ✅ Focus on service health, not client errors
- ✅ Consistent, stateless classification
- ✅ Protect against user function panics

**Implementation Checklist:**
- [ ] Default classifier: `err == nil`
- [ ] User override support
- [ ] Panic recovery and re-panic
- [ ] Context error handling
- [ ] Classifier panic protection
- [ ] Test coverage for common error types

All implementations must follow these classification rules.
