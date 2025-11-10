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

### 4. Production-Ready Scenarios (`production_ready/`) ⭐ **Recommended**

Comprehensive example showing real-world production usage:
- Multiple realistic scenarios (normal operation, degradation, failure spikes)
- Traffic scaling from dev (5 req/s) to production (500 req/s)
- Side-by-side comparison: Adaptive vs Static thresholds
- Automatic recovery demonstration
- Production configuration recommendations

**Run:**
```bash
go run examples/production_ready/main.go
```

**Key Concepts:**
- Production configuration patterns
- Monitoring and observability
- Why adaptive beats static thresholds
- Recovery behavior
- Distributed failure detection

**Perfect for:** Understanding how to deploy AutoBreaker in production

---

### 5. Observability & Monitoring (`observability/`) ⭐ **Recommended**

Comprehensive observability example with four scenarios:
- Normal operation monitoring
- Diagnostic troubleshooting
- Real-time monitoring during failures
- Recovery process monitoring

**Run:**
```bash
go run examples/observability/main.go
```

**Key Concepts:**
- Using Metrics() API for real-time stats
- Using Diagnostics() for troubleshooting
- Predicting circuit behavior
- Health check patterns
- Structured logging examples

**Perfect for:** Production monitoring and incident response

---

### 6. Prometheus Integration (`prometheus/`)

Shows how to expose circuit breaker metrics to Prometheus:
- Custom Prometheus collector
- 8 metrics exported (state, counts, rates)
- HTTP metrics endpoint
- Example PromQL queries and alerts

**Run:**
```bash
go run examples/prometheus/main.go
# Visit http://localhost:8080/metrics
```

**Key Concepts:**
- Prometheus integration pattern
- Metric types (gauges vs counters)
- Alert rules
- Dashboard design

**Perfect for:** Prometheus users, production monitoring

---

## Quick Start

```bash
# Recommended: Start with production_ready for comprehensive overview
go run examples/production_ready/main.go

# Then explore observability
go run examples/observability/main.go

# Or run all examples
for dir in examples/*/; do
    echo "Running $(basename $dir)..."
    go run ${dir}main.go
    echo ""
done
```

## Learning Path

1. **Start here:** `production_ready/` - See everything in action
2. **Observability:** `observability/` - Learn monitoring and troubleshooting
3. **Basics:** `basic/` - Understand core concepts
4. **Adaptive:** `adaptive/` - Learn why adaptive thresholds matter
5. **Integration:** `prometheus/` - Connect to monitoring systems
6. **Customization:** `custom_errors/` - Tailor to your error types
