package breaker

// defaultAdaptiveReadyToTrip implements percentage-based threshold logic.
func (cb *CircuitBreaker) defaultAdaptiveReadyToTrip(counts Counts) bool {
	// Need minimum observations before evaluating
	if counts.Requests < cb.getMinimumObservations() {
		return false
	}

	// Calculate failure rate
	if counts.Requests == 0 {
		return false
	}

	failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
	return failureRate > cb.getFailureRateThreshold()
}
