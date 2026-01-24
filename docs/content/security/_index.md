---
title: "Security"
weight: 8
---

# Security

AutoBreaker is designed with security in mind, but please follow these guidelines for secure usage.

## Security Considerations

### No External Dependencies
AutoBreaker uses only the Go standard library, reducing attack surface.

### Input Validation
- Settings validation on creation
- Runtime bounds checking
- Panic recovery for user callbacks

### Concurrency Safety
- Lock-free atomic operations
- Race detector clean
- No data races in production

## Reporting Security Issues

**DO NOT** create public GitHub issues for security vulnerabilities.

**Instead**, email security reports to: [security@example.com](mailto:security@example.com)

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if known)

We aim to respond within 48 hours and provide updates on resolution progress.

## Best Practices

### 1. Validate Settings
```go
// Use validated settings
breaker := autobreaker.New(autobreaker.Settings{
    Name:    "service-name",
    Timeout: 30 * time.Second,
})
```

### 2. Secure Callbacks
```go
// Keep callbacks fast and safe
OnStateChange: func(name string, from, to State) {
    // Async logging, no blocking I/O
    go secureLogger.Log(name, from, to)
}
```

### 3. Monitor Usage
- Track circuit breaker metrics
- Alert on abnormal patterns
- Log state changes for audit

### 4. Regular Updates
- Keep AutoBreaker updated
- Review release notes for security fixes
- Test updates in staging first

## Security Features

### Panic Recovery
User callbacks are wrapped in panic recovery to prevent crashes.

### Memory Safety
- No buffer overflows (Go memory safety)
- Bounded memory usage
- No goroutine leaks

### Thread Safety
- Atomic operations prevent race conditions
- State transitions are atomic
- Concurrent access safe

## Compliance

AutoBreaker is suitable for use in:
- **SOC 2** compliant environments
- **HIPAA** compliant applications (with proper logging)
- **GDPR** compliant systems

## Contact

For security questions or concerns:
- **Email**: [security@example.com](mailto:security@example.com)
- **PGP Key**: [Available on request]

We take security seriously and appreciate responsible disclosure.
