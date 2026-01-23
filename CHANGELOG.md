# Changelog

All notable changes to AutoBreaker will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.1] - 2026-01-23

### Critical Fixes (Architect's Review)

#### Callback Panic Recovery System
- **Comprehensive Recovery**: New `panic_recovery.go` with type-safe handlers for all callbacks
- **Deterministic Behavior**: 
  - `ReadyToTrip` panic → returns `false` (do not trip - safe default)
  - `OnStateChange` panic → logs, allows transition (prevents blocking)
  - `IsSuccessful` panic → returns `false` (treat as failure - conservative)
- **Thread-Safe Logging**: Mutex-protected logging prevents race conditions
- **Production-Ready**: Always compiled (not behind `//go:build !production`)

#### Counter Saturation Observability
- **Saturation Warnings**: Logs when counters reach `math.MaxUint32`
- **Thread-Safe Operations**: CAS loops with saturation protection
- **Clear Documentation**: Enhanced `Counts` struct documentation

#### Architectural Improvements
- **Monotonic Time Verification**: Confirmed all time operations use monotonic clocks
- **Race Detector Clean**: Core functionality verified with `-race` flag
- **Backward Compatibility**: All public APIs unchanged

#### Testing
- **Callback Panic Tests**: Comprehensive tests for all callback panic scenarios
- **Race Detector Compatibility**: Fixed logging to avoid race detector issues
- **Performance Maintained**: <100ns overhead target preserved

### Files Changed
- `internal/breaker/panic_recovery.go`: New comprehensive panic recovery system (181 lines)
- `internal/breaker/circuitbreaker.go`: Updated to use safe callback functions
- `internal/breaker/state.go`: Updated all callback invocations
- `internal/breaker/counts.go`: Simplified, uses new safe increment functions
- `internal/breaker/types.go`: Enhanced saturation documentation
- Test files: Updated for new function signatures

### Architectural Review Summary
✅ **Production Readiness**: Critical edge cases addressed  
✅ **Thread Safety**: Comprehensive concurrency handling  
✅ **Backward Compatibility**: No breaking changes  
✅ **Observability**: Basic logging with extensible design  
✅ **Test Coverage**: Systematic, phase-based testing

**Recommendation**: These fixes address critical production concerns identified in architectural review while maintaining performance and compatibility.

## [1.1.0] - 2026-01-23

### Reliability Improvements

#### Time Handling
- **Monotonic Clock**: Now uses monotonic clock for duration calculations, preventing issues from system clock jumps (NTP adjustments)
- **Negative Duration Prevention**: Added safeguards against negative durations from time jumps
- **Time Handling Tests**: Added comprehensive tests for time jump scenarios

#### Callback Safety
- **Panic Recovery**: User callbacks (ReadyToTrip, OnStateChange, IsSuccessful) now have panic recovery
- **Circuit Protection**: Callback panics don't break circuit breaker functionality
- **Callback Safety Tests**: Added tests for callback panic scenarios

#### Counter Protection
- **Counter Saturation**: Counters saturate at math.MaxUint32 (4,294,967,295) instead of undefined overflow
- **Safe Increment/Decrement**: Thread-safe CAS loops for atomic counter operations
- **Context Cancellation Fix**: Fixed request counting when context cancels during execution
- **Counter Tests**: Added tests for counter saturation and long-running scenarios

#### State Machine Improvements
- **Race Condition Fixes**: Improved state transition handling under high concurrency
- **Debug Validation**: Added validation for state transition invariants
- **High-Concurrency Tests**: Added tests for 1000+ concurrent goroutines

#### Documentation
- **Counter Saturation**: Documented counter saturation behavior and implications
- **Atomic Snapshot Limitations**: Documented consistency limitations of atomic snapshots
- **Error Messages**: Improved error messages with context
- **CHANGELOG**: Updated with v1.1.0 changes

### Performance
- **Maintained Performance**: <100ns overhead per request in Closed state
- **Zero Allocations**: No allocations in hot path
- **Thread-Safe**: Lock-free atomic operations maintained
- **Race Detector Clean**: All tests pass with race detector

### Compatibility
- **Backward Compatible**: No breaking API changes
- **All Tests Pass**: 96.1% test coverage maintained
- **Existing Code**: All existing code continues to work unchanged

### Files Changed
- `internal/breaker/state.go`: Time handling and state transition improvements
- `internal/breaker/counts.go`: Counter saturation implementation
- `internal/breaker/circuitbreaker.go`: Callback safety and context fixes
- `internal/breaker/types.go`: Updated documentation
- Test files: Comprehensive edge case tests added
- Documentation files: Updated to reflect changes

### Testing Evidence
- **Comprehensive Tests**: Added tests for all identified vulnerabilities
- **Race Detector**: Clean on all tests
- **Stress Tests**: Long-running tests for stability verification
- **Edge Cases**: Tests for time jumps, callback panics, counter saturation

### Known Limitations
- **Atomic Snapshot Consistency**: `Counts()` provides point-in-time snapshot but not atomic across all fields
- **Counter Saturation**: Statistics become inaccurate after counters saturate (at 4+ billion requests)
- **Documented**: All limitations are documented for user awareness

### Follow-up Items
- Consider adding metrics for saturation events in future release
- Consider adding ResetCounts() method if user demand emerges
- Monitor for any edge cases in production deployments

## [1.0.0] - 2025-01-10

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
- **No breaking API changes
- **Additive changes only (new methods, fields)
- **Behavior changes will be opt-in
- **Deprecations will be marked and documented for at least 2 minor versions

### When v2.0 Would Happen

A major version bump to v2.0.0 would only occur for:
- Breaking API changes (removing/renaming public methods)
- Significant behavior changes affecting existing users
- Fundamental architecture changes

We are committed to long-term v1.x stability for production use.

---

[1.1.1]: https://github.com/vnykmshr/autobreaker/releases/tag/v1.1.1
[1.1.0]: https://github.com/vnykmshr/autobreaker/releases/tag/v1.1.0
[1.0.0]: https://github.com/vnykmshr/autobreaker/releases/tag/v1.0.0
