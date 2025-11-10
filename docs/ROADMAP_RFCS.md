# Roadmap RFCs

This document tracks feature requests that require community validation before implementation.

## Active RFCs

### RFC-001: Sliding Windows

**Status:** Awaiting community feedback
**Phase:** 4B
**GitHub Issue:** [Create issue with template below]

**Problem:**
Current implementation uses fixed-window counting (resets at interval boundaries). This can cause the "window boundary problem" where a burst of failures just before/after a window boundary might not trip the circuit.

**Proposed Solution:**
Implement sliding window algorithm that continuously tracks the last N requests or last T duration without reset boundaries.

**Questions for Community:**
1. Is the current fixed-window approach causing issues in production?
2. What use cases require sliding windows?
3. Is the added complexity worth the benefit?
4. Performance impact acceptable? (sliding windows require more memory/computation)

**Validation Criteria:**
- [ ] 10+ upvotes on GitHub issue
- [ ] 3+ users with concrete use cases
- [ ] Demonstrated problem with current fixed-window approach

**Issue Template:**
```md
**Title:** RFC: Sliding Window Implementation (Phase 4B)

**Summary:**
Proposal to add sliding window counting as an alternative to fixed-window intervals.

**Problem:**
[Describe your use case where fixed windows cause issues]

**Expected Behavior:**
[Describe how sliding windows would improve your situation]

**Current Workaround:**
[How are you handling this today?]

**Additional Context:**
- Traffic pattern: [describe]
- Interval setting: [value]
- How often does window boundary problem occur: [frequency]
```

---

### RFC-002: HTTP/gRPC Middleware Package

**Status:** Awaiting community feedback
**Phase:** 5
**GitHub Issue:** [Create issue with template below]

**Problem:**
Users must write their own middleware wrappers for HTTP/gRPC integration. Examples exist but require copy-paste.

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

// gRPC Server
grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCUnaryInterceptor(breaker)),
)
```

**Questions for Community:**
1. Are the examples sufficient, or do you need reusable middleware?
2. What frameworks need support? (http, grpc, echo, gin, chi, etc.)
3. Would you use this, or prefer to write custom wrappers?
4. What additional features would middleware need? (metrics, error classification, etc.)

**Validation Criteria:**
- [ ] 10+ upvotes on GitHub issue
- [ ] 5+ users requesting this feature
- [ ] Consensus on API design
- [ ] Identified top 3 frameworks to support

**Issue Template:**
```md
**Title:** RFC: HTTP/gRPC Middleware Package (Phase 5)

**Summary:**
Proposal to add reusable middleware for common frameworks.

**Use Case:**
[Describe your integration scenario]

**Framework:**
[http.Handler, gRPC, Echo, Gin, Chi, other]

**Current Approach:**
[Are you using examples? Writing custom? Pain points?]

**Desired API:**
[Show how you'd like to use the middleware]

**Additional Requirements:**
- [ ] Custom error classification
- [ ] Metrics integration
- [ ] Request filtering
- [ ] Other: [specify]
```

---

## Creating GitHub Issues

To create these RFCs as GitHub issues:

1. Go to: https://github.com/vnykmshr/autobreaker/issues/new
2. Copy the issue template from above
3. Fill in your specific use case
4. Tag with labels: `RFC`, `enhancement`, `community-feedback`
5. Link to this ROADMAP_RFCS.md for context

## Decision Criteria

RFCs will be approved for implementation when they meet:

1. **Community Validation:**
   - 10+ upvotes OR
   - 3+ distinct users with concrete use cases

2. **Alignment with Philosophy:**
   - Solves real, documented problems
   - Can't be easily implemented with current API
   - Adds measurable value without bloat
   - Maintains lean, focused design

3. **Technical Feasibility:**
   - Performance impact acceptable (<10% overhead)
   - Implementation complexity reasonable
   - Backwards compatible (or major version bump)

## Rejected RFCs

_(None yet)_

### Template for Rejected RFCs:
```md
**RFC-XXX:** [Name]
**Rejected:** [Date]
**Reason:** [Brief explanation]
**Alternative:** [Recommended approach]
```

---

**Note:** v1.0 may be feature-complete. Only implement new features with strong community validation.
