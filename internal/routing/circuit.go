package routing

import (
	"sync"
	"time"
)

// CircuitState enum
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds threshold and cooldown settings
type CircuitBreakerConfig struct {
	FailureThreshold int
	CooldownDuration time.Duration
}

type targetState struct {
	failures        int
	lastFailureTime time.Time
	state           CircuitState
}

// CircuitBreaker tracks target health and trips when failures exceed threshold
type CircuitBreaker struct {
	mu      sync.RWMutex
	cfg     CircuitBreakerConfig
	targets map[string]*targetState
}

// NewCircuitBreaker creates a new CircuitBreaker instance
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 30 * time.Second
	}
	return &CircuitBreaker{
		cfg:     cfg,
		targets: make(map[string]*targetState),
	}
}

// Allow returns true if requests to target are permitted
func (cb *CircuitBreaker) Allow(target string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	ts, exists := cb.targets[target]
	if !exists {
		return true // Default CLOSED
	}

	now := time.Now()
	if ts.state == CircuitOpen {
		if now.Sub(ts.lastFailureTime) >= cb.cfg.CooldownDuration {
			ts.state = CircuitHalfOpen
			return true // Allow canary probe
		}
		return false // Still OPEN
	}

	return true // CLOSED or HALF_OPEN
}

// RecordSuccess registers a successful request and resets the circuit to CLOSED
func (cb *CircuitBreaker) RecordSuccess(target string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if ts, exists := cb.targets[target]; exists {
		ts.failures = 0
		ts.state = CircuitClosed
	}
}

// RecordFailure registers a failure and trips the circuit to OPEN if threshold reached
func (cb *CircuitBreaker) RecordFailure(target string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	ts, exists := cb.targets[target]
	if !exists {
		ts = &targetState{state: CircuitClosed}
		cb.targets[target] = ts
	}

	ts.failures++
	ts.lastFailureTime = time.Now()

	if ts.failures >= cb.cfg.FailureThreshold {
		ts.state = CircuitOpen
	}
}

// GetState returns the current circuit state of a target
func (cb *CircuitBreaker) GetState(target string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	ts, exists := cb.targets[target]
	if !exists {
		return CircuitClosed
	}

	if ts.state == CircuitOpen && time.Now().Sub(ts.lastFailureTime) >= cb.cfg.CooldownDuration {
		return CircuitHalfOpen
	}

	return ts.state
}
