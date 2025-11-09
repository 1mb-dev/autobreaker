# AutoBreaker Examples

This directory contains example programs demonstrating various AutoBreaker features.

## Examples

### 1. Basic Usage (`basic/`)

Demonstrates fundamental circuit breaker behavior:
- Creating a circuit breaker with default settings
- Successful request execution
- Circuit tripping on failures
- Request rejection when circuit is open

**Run:**
```bash
go run examples/basic/main.go
```

**Key Concepts:**
- Default threshold: 6 consecutive failures
- State transitions: Closed → Open
- Fast-fail behavior when open

---

### 2. Adaptive Thresholds (`adaptive/`)

Shows how adaptive thresholds work across different traffic volumes:
- Low traffic scenario (50 requests)
- High traffic scenario (100 requests)
- Same configuration works for both

**Run:**
```bash
go run examples/adaptive/main.go
```

**Key Concepts:**
- Percentage-based thresholds vs absolute counts
- Minimum observations before adaptation
- Traffic-aware protection

---

### 3. Custom Error Classification (`custom_errors/`)

Demonstrates customizing which errors count as failures:
- HTTP status code handling
- Treating 4xx as success (client errors)
- Treating 5xx as failure (server errors)

**Run:**
```bash
go run examples/custom_errors/main.go
```

**Key Concepts:**
- `IsSuccessful` callback
- Service health vs client errors
- Preventing false positives

---

## Running All Examples

```bash
# Run all examples
for dir in examples/*/; do
    echo "Running $(basename $dir)..."
    go run ${dir}main.go
    echo ""
done
```

## Note

These examples are part of Phase 0 (Foundation). The `Execute()` method is not yet implemented.

Full implementation will be completed in Phase 1.

## Next Steps

After Phase 1 implementation:
- [ ] Verify all examples work correctly
- [ ] Add state transition examples
- [ ] Add observability examples
- [ ] Add integration examples (HTTP, gRPC)
