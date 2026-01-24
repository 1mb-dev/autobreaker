---
title: "Roadmap"
weight: 6
---

# Roadmap

Future plans and feature requests for AutoBreaker.

## Active RFCs

### RFC-001: Sliding Windows

**Status:** Awaiting community feedback

**Problem:**
Current implementation uses fixed-window counting (resets at interval boundaries). This can cause the "window boundary problem" where a burst of failures just before/after a window boundary might not trip the circuit.

**Proposed Solution:**
Implement sliding window algorithm that continuously tracks the last N requests or last T duration without reset boundaries.

**Questions for Community:**
1. Is the current fixed-window approach causing issues in production?
2. What use cases require sliding windows?
3. Is the added complexity worth the benefit?

### RFC-002: HTTP/gRPC Middleware Package

**Status:** Awaiting community feedback

**Problem:**
Users must write their own middleware wrappers for HTTP/gRPC integration.

**Proposed Solution:**
Add optional `middleware` sub-package:

```go
import "github.com/vnykmshr/autobreaker/middleware"

// HTTP Server
handler := middleware.HTTPHandler(breaker, yourHandler)

// HTTP Client  
client := &http.Client{
    Transport: middleware.HTTPRoundTripper(breaker, http.DefaultTransport),
}
```

**Questions for Community:**
1. Are the examples sufficient, or do you need reusable middleware?
2. What frameworks need support?
3. Would you use this, or prefer custom wrappers?

## How to Contribute

### 1. Create GitHub Issues
Go to [GitHub Issues](https://github.com/vnykmshr/autobreaker/issues) and use the RFC templates.

### 2. Provide Use Cases
- Describe your specific scenario
- Show current workarounds
- Explain expected behavior

### 3. Vote on Features
Upvote issues you'd like to see implemented.

## Decision Criteria

RFCs will be approved for implementation when they meet:

1. **Community Validation:**
   - 10+ upvotes OR
   - 3+ distinct users with concrete use cases

2. **Alignment with Philosophy:**
   - Solves real, documented problems
   - Can't be easily implemented with current API
   - Adds value without bloat
   - Maintains lean, focused design

3. **Technical Feasibility:**
   - Performance impact acceptable (<10% overhead)
   - Implementation complexity reasonable
   - Backwards compatible

## Versioning

AutoBreaker follows [Semantic Versioning](https://semver.org/):

- **Major (X.0.0)**: Breaking API changes
- **Minor (1.X.0)**: New features, backward compatible  
- **Patch (1.0.X)**: Bug fixes, documentation

## Current Focus

**v1.0.x**: Bug fixes, documentation improvements
**v1.1.0**: Edge case hardening, reliability improvements
**Future**: Community-driven features based on RFCs

## Links

- **GitHub Issues**: [https://github.com/vnykmshr/autobreaker/issues](https://github.com/vnykmshr/autobreaker/issues)
