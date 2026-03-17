// Package circuitbreaker provides circuit breaker pattern implementation
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"alldev-gin-rpc/pkg/logger"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

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

// Config holds circuit breaker configuration
type Config struct {
	MaxRequests        uint32        // Maximum requests in half-open state
	Interval           time.Duration // Time to collect metrics in closed state
	Timeout            time.Duration // Time to wait in open state
	ReadyToTrip        func(counts Counts) bool
	OnStateChange      func(name string, from, to State)
	IsSuccessful       func(err error) bool
	Fallback          func(err error) error
}

// DefaultConfig returns default circuit breaker configuration
func DefaultConfig() Config {
	return Config{
		MaxRequests: 1,
		Interval:    time.Minute,
		Timeout:     time.Minute,
		ReadyToTrip:  func(counts Counts) bool { return counts.ConsecutiveFailures > 5 },
		OnStateChange: func(name string, from, to State) {
			logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
		IsSuccessful: func(err error) bool { return err == nil },
	}
}

// Counts holds circuit breaker counts
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	onStateChange func(name string, from, to State)
	isSuccessful  func(err error) bool
	fallback     func(err error) error

	mutex      sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:          name,
		maxRequests:   config.MaxRequests,
		interval:      config.Interval,
		timeout:       config.Timeout,
		readyToTrip:   config.ReadyToTrip,
		onStateChange: config.OnStateChange,
		isSuccessful:  config.IsSuccessful,
		fallback:     config.Fallback,
	}

	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.interval <= 0 {
		cb.interval = time.Minute
	}

	if cb.timeout <= 0 {
		cb.timeout = time.Minute
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// Execute executes the given function if the circuit breaker is closed or half-open
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		// Circuit breaker is open, try fallback
		if cb.fallback != nil {
			return nil, cb.fallback(err)
		}
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := fn()
	success := cb.isSuccessful(err)
	cb.afterRequest(generation, success)

	if err != nil && cb.fallback != nil {
		return nil, cb.fallback(err)
	}

	return result, err
}

// ExecuteContext executes the given function with context
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		// Circuit breaker is open, try fallback
		if cb.fallback != nil {
			return nil, cb.fallback(err)
		}
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := fn(ctx)
	success := cb.isSuccessful(err)
	cb.afterRequest(generation, success)

	if err != nil && cb.fallback != nil {
		return nil, cb.fallback(err)
	}

	return result, err
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.state
}

// Counts returns the current circuit breaker counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

// beforeRequest checks if the request can proceed
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()

	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitBreakerOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

// afterRequest updates the circuit breaker state after a request
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess handles successful request
func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	switch state {
	case StateClosed:
		// Nothing to do
	case StateHalfOpen:
		if cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

// onFailure handles failed request
func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	switch state {
	case StateClosed:
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState returns the current state and generation
func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState sets the circuit breaker state
func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

// toNewGeneration creates a new generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default:
		cb.expiry = zero
	}
}

// Errors
var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrTooManyRequests   = errors.New("too many requests in half-open state")
)

// CircuitBreakerGroup manages multiple circuit breakers
type CircuitBreakerGroup struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewCircuitBreakerGroup creates a new circuit breaker group
func NewCircuitBreakerGroup() *CircuitBreakerGroup {
	return &CircuitBreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Add adds a circuit breaker to the group
func (g *CircuitBreakerGroup) Add(name string, config Config) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.breakers[name] = NewCircuitBreaker(name, config)
}

// Get returns a circuit breaker by name
func (g *CircuitBreakerGroup) Get(name string) (*CircuitBreaker, bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	cb, exists := g.breakers[name]
	return cb, exists
}

// Remove removes a circuit breaker from the group
func (g *CircuitBreakerGroup) Remove(name string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	delete(g.breakers, name)
}

// List returns all circuit breaker names
func (g *CircuitBreakerGroup) List() []string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	names := make([]string, 0, len(g.breakers))
	for name := range g.breakers {
		names = append(names, name)
	}
	return names
}

// States returns the state of all circuit breakers
func (g *CircuitBreakerGroup) States() map[string]State {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	states := make(map[string]State)
	for name, cb := range g.breakers {
		states[name] = cb.State()
	}
	return states
}

// Execute executes a function with circuit breaker protection
func (g *CircuitBreakerGroup) Execute(name string, fn func() (interface{}, error)) (interface{}, error) {
	cb, exists := g.Get(name)
	if !exists {
		// If no circuit breaker exists, execute directly
		return fn()
	}

	return cb.Execute(fn)
}

// ExecuteContext executes a function with circuit breaker protection and context
func (g *CircuitBreakerGroup) ExecuteContext(name string, ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	cb, exists := g.Get(name)
	if !exists {
		// If no circuit breaker exists, execute directly
		return fn(ctx)
	}

	return cb.ExecuteContext(ctx, fn)
}

// HealthChecker provides health checking for circuit breakers
type HealthChecker struct {
	group *CircuitBreakerGroup
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(group *CircuitBreakerGroup) *HealthChecker {
	return &HealthChecker{group: group}
}

// CheckHealth checks the health of all circuit breakers
func (h *HealthChecker) CheckHealth() error {
	states := h.group.States()
	
	for name, state := range states {
		if state == StateOpen {
			return fmt.Errorf("circuit breaker %s is open", name)
		}
	}
	
	return nil
}

// GetHealthStatus returns detailed health status
func (h *HealthChecker) GetHealthStatus() map[string]interface{} {
	states := h.group.States()
	
	status := make(map[string]interface{})
	for name, state := range states {
		cb, _ := h.group.Get(name)
		
		status[name] = map[string]interface{}{
			"state":  state.String(),
			"counts": cb.Counts(),
		}
	}
	
	return status
}

// Middleware provides circuit breaker middleware for HTTP
type Middleware struct {
	group *CircuitBreakerGroup
}

// NewMiddleware creates a new circuit breaker middleware
func NewMiddleware(group *CircuitBreakerGroup) *Middleware {
	return &Middleware{group: group}
}

// Wrap wraps an HTTP handler with circuit breaker protection
func (m *Middleware) Wrap(name string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := m.group.Execute(name, func() (interface{}, error) {
			handler.ServeHTTP(w, r)
			return nil, nil
		})
		
		if err != nil {
			if err == ErrCircuitBreakerOpen {
				http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			} else if err == ErrTooManyRequests {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
	})
}
