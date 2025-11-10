package breaker

import (
	"math"
	"time"
)

// Atomic accessors for updateable settings
// These ensure thread-safe access without locks

func (cb *CircuitBreaker) getMaxRequests() uint32 {
	return cb.maxRequests.Load()
}

func (cb *CircuitBreaker) setMaxRequests(val uint32) {
	cb.maxRequests.Store(val)
}

func (cb *CircuitBreaker) getInterval() time.Duration {
	return time.Duration(cb.interval.Load())
}

func (cb *CircuitBreaker) setInterval(val time.Duration) {
	cb.interval.Store(int64(val))
}

func (cb *CircuitBreaker) getTimeout() time.Duration {
	return time.Duration(cb.timeout.Load())
}

func (cb *CircuitBreaker) setTimeout(val time.Duration) {
	cb.timeout.Store(int64(val))
}

func (cb *CircuitBreaker) getFailureRateThreshold() float64 {
	bits := cb.failureRateThreshold.Load()
	return math.Float64frombits(bits)
}

func (cb *CircuitBreaker) setFailureRateThreshold(val float64) {
	bits := math.Float64bits(val)
	cb.failureRateThreshold.Store(bits)
}

func (cb *CircuitBreaker) getMinimumObservations() uint32 {
	return cb.minimumObservations.Load()
}

func (cb *CircuitBreaker) setMinimumObservations(val uint32) {
	cb.minimumObservations.Store(val)
}
