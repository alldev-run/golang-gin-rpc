// Package circuitbreaker provides circuit breaker pattern implementation for database
// connections to prevent cascading failures. It monitors failure rates and opens
// the circuit when thresholds are exceeded, preventing requests from reaching
// the failing database.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

// State represents the circuit breaker state.
type State int32

const (
	// StateClosed - circuit is closed, requests flow normally.
	StateClosed State = iota
	// StateOpen - circuit is open, requests fail fast.
	StateOpen
	// StateHalfOpen - circuit is half-open, testing if service recovered.
	StateHalfOpen
)

// String returns string representation of state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration.
type Config struct {
	// MaxFailures is the threshold to open the circuit.
	MaxFailures uint32
	// ResetTimeout is the duration to wait before entering half-open state.
	ResetTimeout time.Duration
	// HalfOpenMaxRequests is the number of allowed requests in half-open state.
	HalfOpenMaxRequests uint32
	// SuccessThreshold is the consecutive successes required to close circuit.
	SuccessThreshold uint32
	// Name identifies this circuit breaker in logs.
	Name string
}

// DefaultConfig returns default circuit breaker configuration.
func DefaultConfig() Config {
	return Config{
		MaxFailures:           5,
		ResetTimeout:          30 * time.Second,
		HalfOpenMaxRequests:   3,
		SuccessThreshold:      2,
		Name:                  "default",
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	config Config

	// atomic state
	state int32

	// atomic counters
	failures    uint32
	successes   uint32
	requests    uint32 // for half-open state

	// last failure time (atomic)
	lastFailureTime int64

	mutex sync.RWMutex
	// custom error for open circuit
	openError error
}

// ErrCircuitOpen is returned when the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// New creates a new circuit breaker.
func New(config Config) *CircuitBreaker {
	if config.MaxFailures == 0 {
		config.MaxFailures = DefaultConfig().MaxFailures
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = DefaultConfig().ResetTimeout
	}
	if config.HalfOpenMaxRequests == 0 {
		config.HalfOpenMaxRequests = DefaultConfig().HalfOpenMaxRequests
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = DefaultConfig().SuccessThreshold
	}

	return &CircuitBreaker{
		config:    config,
		state:     int32(StateClosed),
		openError: fmt.Errorf("%w: %s", ErrCircuitOpen, config.Name),
	}
}

// State returns current state (thread-safe).
func (cb *CircuitBreaker) State() State {
	return State(atomic.LoadInt32(&cb.state))
}

// Execute runs the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if we can proceed
	if err := cb.canExecute(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record result
	cb.recordResult(err)

	return err
}

// ExecuteWithResult runs the given function with circuit breaker protection and returns its result.
func (cb *CircuitBreaker) ExecuteWithResult(ctx context.Context, fn func() (any, error)) (any, error) {
	// Check if we can proceed
	if err := cb.canExecute(); err != nil {
		return nil, err
	}

	// Execute the function
	result, err := fn()

	// Record result
	cb.recordResult(err)

	return result, err
}

// canExecute checks if request can proceed (thread-safe).
func (cb *CircuitBreaker) canExecute() error {
	state := cb.State()

	switch state {
	case StateClosed:
		// Circuit closed, allow request
		return nil

	case StateOpen:
		// Circuit open, check if timeout elapsed
		lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
		timeSinceLast := time.Since(time.Unix(0, lastFailure))

		if timeSinceLast >= cb.config.ResetTimeout {
			// Try to transition to half-open
			if cb.toHalfOpen() {
				logger.Info("circuit breaker transitioned to half-open",
					zap.String("name", cb.config.Name))
				return nil
			}
			// Another goroutine transitioned, check state again
			if cb.State() == StateHalfOpen {
				return nil
			}
		}
		return cb.openError

	case StateHalfOpen:
		// In half-open, limit concurrent requests
		requests := atomic.AddUint32(&cb.requests, 1)
		if requests > cb.config.HalfOpenMaxRequests {
			atomic.AddUint32(&cb.requests, ^uint32(0)) // decrement
			return cb.openError
		}
		return nil

	default:
		return errors.New("unknown circuit breaker state")
	}
}

// recordResult records success or failure (thread-safe).
func (cb *CircuitBreaker) recordResult(err error) {
	state := cb.State()

	if err != nil {
		// Record failure
		cb.onFailure(state)
	} else {
		// Record success
		cb.onSuccess(state)
	}
}

// onFailure handles failure recording.
func (cb *CircuitBreaker) onFailure(state State) {
	switch state {
	case StateClosed:
		failures := atomic.AddUint32(&cb.failures, 1)
		if failures >= cb.config.MaxFailures {
			if cb.toOpen() {
				logger.Warn("circuit breaker opened due to failures",
					zap.String("name", cb.config.Name),
					zap.Uint32("failures", failures),
					zap.Uint32("threshold", cb.config.MaxFailures))
			}
		}

	case StateHalfOpen:
		// Failure in half-open, go back to open
		if cb.toOpen() {
			logger.Warn("circuit breaker re-opened after half-open failure",
				zap.String("name", cb.config.Name))
		}
	}
}

// onSuccess handles success recording.
func (cb *CircuitBreaker) onSuccess(state State) {
	switch state {
	case StateClosed:
		// Reset failures on success
		atomic.StoreUint32(&cb.failures, 0)

	case StateHalfOpen:
		successes := atomic.AddUint32(&cb.successes, 1)
		if successes >= cb.config.SuccessThreshold {
			if cb.toClosed() {
				logger.Info("circuit breaker closed after recovery",
					zap.String("name", cb.config.Name),
					zap.Uint32("successes", successes))
			}
		}
	}
}

// toOpen transitions to open state (CAS operation).
func (cb *CircuitBreaker) toOpen() bool {
	if atomic.CompareAndSwapInt32(&cb.state, int32(StateClosed), int32(StateOpen)) {
		atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())
		atomic.StoreUint32(&cb.failures, 0)
		return true
	}
	// Check if already transitioning from half-open
	return atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateOpen))
}

// toHalfOpen transitions to half-open state (CAS operation).
func (cb *CircuitBreaker) toHalfOpen() bool {
	if atomic.CompareAndSwapInt32(&cb.state, int32(StateOpen), int32(StateHalfOpen)) {
		atomic.StoreUint32(&cb.successes, 0)
		atomic.StoreUint32(&cb.requests, 0)
		return true
	}
	return false
}

// toClosed transitions to closed state (CAS operation).
func (cb *CircuitBreaker) toClosed() bool {
	if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateClosed)) {
		atomic.StoreUint32(&cb.failures, 0)
		atomic.StoreUint32(&cb.successes, 0)
		atomic.StoreUint32(&cb.requests, 0)
		return true
	}
	return false
}

// ForceOpen manually opens the circuit.
func (cb *CircuitBreaker) ForceOpen() {
	atomic.StoreInt32(&cb.state, int32(StateOpen))
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())
	logger.Warn("circuit breaker manually opened",
		zap.String("name", cb.config.Name))
}

// ForceClosed manually closes the circuit.
func (cb *CircuitBreaker) ForceClosed() {
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreUint32(&cb.failures, 0)
	atomic.StoreUint32(&cb.successes, 0)
	atomic.StoreUint32(&cb.requests, 0)
	logger.Info("circuit breaker manually closed",
		zap.String("name", cb.config.Name))
}

// GetStats returns circuit breaker statistics.
func (cb *CircuitBreaker) GetStats() Stats {
	return Stats{
		State:       cb.State(),
		Failures:    atomic.LoadUint32(&cb.failures),
		Successes:   atomic.LoadUint32(&cb.successes),
		Requests:    atomic.LoadUint32(&cb.requests),
		LastFailure: time.Unix(0, atomic.LoadInt64(&cb.lastFailureTime)),
	}
}

// Stats holds circuit breaker statistics.
type Stats struct {
	State       State     `json:"state"`
	Failures    uint32    `json:"failures"`
	Successes   uint32    `json:"successes"`
	Requests    uint32    `json:"requests"`
	LastFailure time.Time `json:"last_failure"`
}
