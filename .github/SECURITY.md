# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of AutoBreaker seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please Do

- **Report privately**: Send reports to [vinay.mish@gmail.com](mailto:vinay.mish@gmail.com)
- **Include details**: Provide as much information as possible about the vulnerability
- **Allow time to respond**: Give us reasonable time to investigate and fix the issue before public disclosure
- **Be patient**: We will respond within 48 hours acknowledging receipt

### Please Don't

- **Don't open public issues**: Please do not publicly disclose the vulnerability until we've had a chance to address it
- **Don't exploit the vulnerability**: We trust you to act in good faith

### What to Include

A good security report should include:

1. **Description**: A clear description of the vulnerability
2. **Impact**: What an attacker could do with this vulnerability
3. **Reproduction steps**: How to reproduce the issue
4. **Affected versions**: Which versions are affected
5. **Suggested fix**: If you have ideas on how to fix it (optional)

### Response Timeline

- **Initial response**: Within 48 hours
- **Status update**: Within 7 days
- **Fix release**: Depends on severity, typically within 30 days

### Severity Classification

We use the following severity levels:

- **Critical**: Allows remote code execution or data theft
- **High**: Allows escalation of privileges or significant data exposure
- **Medium**: Allows limited data exposure or service disruption
- **Low**: Minor security issues with limited impact

### Security Best Practices

When using AutoBreaker in production:

#### 1. Validate Settings

Always validate circuit breaker settings at startup:

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "my-service",
    Timeout:              10 * time.Second,
    AdaptiveThreshold:    true,
    FailureRateThreshold: 0.05, // Must be in (0, 1)
    MinimumObservations:  20,   // Must be > 0
})
```

Invalid settings will panic at construction time (fail-fast).

#### 2. Use Context

Use `ExecuteContext` for proper cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := breaker.ExecuteContext(ctx, func() (interface{}, error) {
    return externalService.CallWithContext(ctx)
})
```

#### 3. Classify Errors Properly

Implement `IsSuccessful` to distinguish between different error types:

```go
breaker := autobreaker.New(autobreaker.Settings{
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }
        // Don't trip circuit on client errors (4xx)
        return isClientError(err)
    },
})
```

#### 4. Monitor Circuit State

Use `OnStateChange` callback for alerting:

```go
breaker := autobreaker.New(autobreaker.Settings{
    OnStateChange: func(name string, from, to autobreaker.State) {
        if to == autobreaker.StateOpen {
            alerter.Send("Circuit %s has opened!", name)
        }
    },
})
```

#### 5. Validate Runtime Updates

Validate settings before calling `UpdateSettings`:

```go
// Good: Validate first
newThreshold := getUserInput()
if newThreshold <= 0 || newThreshold >= 1 {
    return fmt.Errorf("invalid threshold: must be in (0, 1)")
}

err := breaker.UpdateSettings(autobreaker.SettingsUpdate{
    FailureRateThreshold: autobreaker.Float64Ptr(newThreshold),
})
```

#### 6. Avoid Denial of Service

Configure appropriate timeouts and intervals:

```go
breaker := autobreaker.New(autobreaker.Settings{
    Timeout:  30 * time.Second,  // Not too long
    Interval: 60 * time.Second,  // Periodic reset to prevent count accumulation
})
```

#### 7. Secure Callbacks

Ensure callbacks are thread-safe and don't leak sensitive information:

```go
breaker := autobreaker.New(autobreaker.Settings{
    OnStateChange: func(name string, from, to autobreaker.State) {
        // **Bad: Logging sensitive data
        log.Printf("Circuit %s changed with user data: %v", name, sensitiveData)

        // **Good: Safe logging
        log.Printf("Circuit %s: %s → %s", name, from, to)
    },
})
```

## Known Security Considerations

### 1. Goroutine Safety

AutoBreaker uses atomic operations exclusively and is safe for concurrent use without external synchronization. No known race conditions exist (verified with `-race` flag).

### 2. Panic Recovery

AutoBreaker recovers from panics in the wrapped function and re-raises them after recording the failure. This ensures:
- Panics are counted as failures
- Stack traces are preserved
- Circuit breaker state remains consistent

### 3. Memory Safety

AutoBreaker has zero external dependencies and uses only Go stdlib. All memory operations use standard Go patterns:
- No unsafe pointer arithmetic
- No C dependencies
- No reflection in hot paths

### 4. Denial of Service

Circuit breakers protect **against** DoS by failing fast, but misconfiguration could cause:
- Prematurely opening circuits (threshold too low)
- Never opening circuits (threshold too high)
- Resource exhaustion (no Interval set with high traffic)

**Mitigation**: Use adaptive thresholds and configure `Interval` for count resets.

### 5. Rate Limiting

**Important:** AutoBreaker does **not** limit the request rate to the circuit breaker itself. It protects your backend by failing fast when open, but accepts unlimited concurrent requests when closed.

The circuit breaker pattern focuses on failure detection and recovery, not request throttling.

**For request rate limiting**, use complementary libraries:
- `golang.org/x/time/rate` - Token bucket algorithm
- `github.com/ulule/limiter` - Multiple algorithms (sliding window, fixed window)

**Example - Combining Circuit Breaker + Rate Limiter:**
```go
rateLimiter := rate.NewLimiter(rate.Limit(100), 10) // 100 req/s, burst 10
breaker := autobreaker.New(autobreaker.Settings{Name: "api"})

func protectedRequest() error {
    // Rate limit first
    if err := rateLimiter.Wait(ctx); err != nil {
        return err
    }

    // Then circuit breaker
    _, err := breaker.Execute(func() (interface{}, error) {
        return apiCall()
    })
    return err
}
```

## Acknowledgments

We appreciate responsible disclosure and will acknowledge security researchers who report vulnerabilities (with permission).

---

**Last Updated**: January 2025
