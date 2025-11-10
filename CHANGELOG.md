# Changelog

All notable changes to AutoBreaker will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-01-XX

### Added

#### Core Features
- **Adaptive Circuit Breaker**: Percentage-based failure thresholds that scale with traffic
- **Context Support**: `ExecuteContext()` method for proper cancellation and timeout handling
- **Runtime Configuration**: `UpdateSettings()` for dynamic configuration without restart
- **Rich Observability**: `Metrics()` and `Diagnostics()` APIs for monitoring
- **Thread-Safe**: Lock-free atomic operations for all methods
- **High Performance**: <100ns overhead, zero allocations in hot path

#### API

- `New(Settings)`: Create circuit breaker with validation and defaults
- `Execute(func() (interface{}, error))`: Execute operation with circuit breaker protection
- `ExecuteContext(context.Context, func())`: Context-aware execution (v1.0.0)
- `State()`: Get current circuit state (Closed/Open/HalfOpen)
- `Counts()`: Get request statistics snapshot
- `Metrics()`: Get comprehensive metrics with computed rates
- `Diagnostics()`: Get full diagnostic information with predictions
- `UpdateSettings(SettingsUpdate)`: Update configuration at runtime
- `Name()`: Get circuit breaker identifier

#### Settings

- **Basic Settings**:
  - `Name`: Circuit breaker identifier
  - `MaxRequests`: Concurrent request limit in half-open state
  - `Interval`: Count reset period in closed state
  - `Timeout`: Duration before transitioning open → half-open

- **Adaptive Settings**:
  - `AdaptiveThreshold`: Enable percentage-based thresholds
  - `FailureRateThreshold`: Failure rate (0.0-1.0) that triggers open
  - `MinimumObservations`: Minimum requests before adaptive logic activates

- **Callbacks**:
  - `ReadyToTrip`: Custom failure detection logic
  - `OnStateChange`: State transition notifications
  - `IsSuccessful`: Custom success/failure determination

#### Documentation

- **Comprehensive godoc**: 2000+ lines covering entire API
- **9 Complete Examples**:
  - Basic usage patterns
  - Adaptive vs static thresholds
  - Custom error handling
  - Observability and monitoring
  - Runtime configuration
  - Prometheus integration
  - Production-ready patterns
  - HTTP client integration (custom RoundTripper)
  - HTTP server integration (middleware pattern)
- **README**: Complete usage guide with quick start
- **CONTRIBUTING**: Guidelines for contributors
- **SECURITY**: Vulnerability reporting policy

#### Testing

- **Unit Tests**: 80+ tests with 98.3% coverage
- **Context Tests**: 12 tests for ExecuteContext scenarios
- **Stress Tests**: 7 comprehensive tests:
  - 10M operations (27M ops/sec)
  - 1000 concurrent goroutines (11.4M ops/sec)
  - Mixed read/write (Execute + UpdateSettings)
  - Long-running stability (5 min - 1 hour)
  - Rapid state transitions (10K iterations)
  - Very high request rate (1000 workers)
  - Very low request rate (1 req/sec)
- **Race Detector**: All tests pass with -race flag

#### Benchmarks

- **Core Benchmarks**:
  - Execute (closed): 34.23 ns/op, 0 allocs
  - Execute (open): 78.80 ns/op, 0 allocs
  - ExecuteContext: 35.22 ns/op, 0 allocs
  - State(): 0.34 ns/op, 0 allocs
  - Counts(): 0.88 ns/op, 0 allocs
  - Metrics(): 18.52 ns/op, 0 allocs
  - Diagnostics(): 39.44 ns/op, 0 allocs
  - UpdateSettings(): 11.59 ns/op, 0 allocs

- **Realistic Scenarios**:
  - High throughput (1M ops)
  - Concurrent execution (100 goroutines)
  - State transitions (mixed states)

#### CI/CD

- **GitHub Actions Workflows**:
  - **Tests**: Go 1.21/1.22/1.23 × Linux/macOS/Windows with race detector
  - **Lint**: golangci-lint, go fmt, go vet, staticcheck
  - **Benchmarks**: Performance validation on every push/PR
  - **Coverage**: 98%+ threshold enforcement

- **Quality Gates**:
  - Zero allocations verification in hot paths
  - Performance target validation
  - Cross-platform compatibility
  - Race condition detection

### Performance

- **Throughput**: 27M+ ops/sec (single-threaded), 11M+ ops/sec (1000 concurrent)
- **Latency**: <100ns overhead in closed state, <80ns in open state
- **Memory**: Zero allocations in hot path, stable under load
- **Concurrency**: Linear scaling with cores, no contention

### Compatibility

- **Go Versions**: 1.21, 1.22, 1.23+
- **Platforms**: Linux, macOS, Windows
- **Drop-in**: Compatible with sony/gobreaker API
- **Dependencies**: Zero external dependencies (stdlib only)

## Development History

### Phase 4A: Runtime Configuration
*Completed: [Date]*

- Implemented `UpdateSettings()` method for dynamic configuration
- Added `SettingsUpdate` type with pointer semantics for partial updates
- Made all updateable settings atomic for thread-safety
- Smart reset behavior (interval changes reset counts, timeout changes restart timer)
- Comprehensive tests for concurrent updates
- Runtime configuration example

### Phase 3A: Observability
*Completed: [Date]*

- Implemented `Metrics()` API with computed failure/success rates
- Implemented `Diagnostics()` API with predictive insights
- Added `WillTripNext` prediction for proactive alerting
- Added `TimeUntilHalfOpen` for recovery timing
- Observability and monitoring examples
- Prometheus integration example

### Phase 2A: Adaptive Thresholds
*Completed: [Date]*

- Implemented adaptive threshold algorithm (percentage-based)
- Added `AdaptiveThreshold`, `FailureRateThreshold`, `MinimumObservations` settings
- Traffic-proportional failure detection
- Comprehensive tests for adaptive logic
- Adaptive vs static threshold comparison example

### Phase 1: Core Circuit Breaker
*Completed: [Date]*

- Three-state machine (Closed, Open, HalfOpen)
- State transitions with automatic recovery probing
- Request counting and statistics
- Panic handling and recovery
- Thread-safe atomic operations
- Basic settings and callbacks
- Comprehensive test suite (98%+ coverage)
- Basic usage examples

## [Unreleased]

No unreleased changes.

---

## Versioning Strategy

AutoBreaker follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version (v2.0.0): Incompatible API changes
- **MINOR** version (v1.1.0): Backward-compatible new features
- **PATCH** version (v1.0.1): Backward-compatible bug fixes

### v1.x Compatibility Promise

Within v1.x releases:
- ✅ No breaking API changes
- ✅ Additive changes only (new methods, fields)
- ✅ Behavior changes will be opt-in
- ✅ Deprecations will be marked and documented for at least 2 minor versions

### When v2.0 Would Happen

A major version bump to v2.0.0 would only occur for:
- Breaking API changes (removing/renaming public methods)
- Significant behavior changes affecting existing users
- Fundamental architecture changes

We are committed to long-term v1.x stability for production use.

---

[1.0.0]: https://github.com/vnykmshr/autobreaker/releases/tag/v1.0.0
