// Package ratelimiter provides rate limiting functionality
package ratelimiter

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"

	"golang.org/x/time/rate"
)

// Strategy represents rate limiting strategy
type Strategy string

const (
	StrategyTokenBucket   Strategy = "token_bucket"
	StrategyLeakyBucket   Strategy = "leaky_bucket"
	StrategyFixedWindow   Strategy = "fixed_window"
	StrategySlidingWindow Strategy = "sliding_window"
)

// Config holds rate limiter configuration
type Config struct {
	Strategy      Strategy      `yaml:"strategy" json:"strategy"`
	Rate          float64       `yaml:"rate" json:"rate"`     // requests per second
	Burst         int           `yaml:"burst" json:"burst"`   // burst size
	Window        time.Duration `yaml:"window" json:"window"` // window duration for fixed/sliding window
	CleanupPeriod time.Duration `yaml:"cleanup_period" json:"cleanup_period"`
}

// DefaultConfig returns default rate limiter configuration
func DefaultConfig() Config {
	return Config{
		Strategy:      StrategyTokenBucket,
		Rate:          100,              // 100 requests per second
		Burst:         10,               // burst of 10 requests
		Window:        time.Minute,      // 1 minute window
		CleanupPeriod: 10 * time.Minute, // cleanup every 10 minutes
	}
}

// RateLimiter interface
type RateLimiter interface {
	Allow() bool
	AllowN(n int) bool
	AllowWithContext(ctx context.Context) (bool, error)
	AllowNWithContext(ctx context.Context, n int) (bool, error)
	Wait(ctx context.Context) error
	WaitN(ctx context.Context, n int) error
	Reserve() *Reservation
	ReserveN(n int) *Reservation
	String() string
}

// TokenBucketRateLimiter implements token bucket rate limiting
type TokenBucketRateLimiter struct {
	limiter *rate.Limiter
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter
func NewTokenBucketRateLimiter(config Config) *TokenBucketRateLimiter {
	burst := config.Burst
	if burst < 0 {
		burst = 1 // Handle negative burst by setting to 1 to allow at least one request
	}
	limiter := rate.NewLimiter(rate.Limit(config.Rate), burst)
	return &TokenBucketRateLimiter{limiter: limiter}
}

// Allow checks if a request is allowed
func (r *TokenBucketRateLimiter) Allow() bool {
	return r.limiter.Allow()
}

// AllowN checks if n requests are allowed
func (r *TokenBucketRateLimiter) AllowN(n int) bool {
	if n <= 0 {
		return n == 0 // Allow 0, deny negative
	}
	return r.limiter.AllowN(time.Now(), n)
}

// AllowWithContext checks if a request is allowed with context support
func (r *TokenBucketRateLimiter) AllowWithContext(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.Allow(), nil
	}
}

// AllowNWithContext checks if n requests are allowed with context support
func (r *TokenBucketRateLimiter) AllowNWithContext(ctx context.Context, n int) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.AllowN(n), nil
	}
}

// Wait waits until a request is allowed
func (r *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// WaitN waits until n requests are allowed
func (r *TokenBucketRateLimiter) WaitN(ctx context.Context, n int) error {
	return r.limiter.WaitN(ctx, n)
}

// Reserve reserves a request
func (r *TokenBucketRateLimiter) Reserve() *Reservation {
	reservation := r.limiter.Reserve()
	return &Reservation{
		ok:    reservation.OK(),
		delay: reservation.Delay(),
	}
}

// ReserveN reserves n requests
func (r *TokenBucketRateLimiter) ReserveN(n int) *Reservation {
	reservation := r.limiter.ReserveN(time.Now(), n)
	return &Reservation{
		ok:    reservation.OK(),
		delay: reservation.Delay(),
	}
}

// String returns string representation
func (r *TokenBucketRateLimiter) String() string {
	return fmt.Sprintf("TokenBucket(rate=%v, burst=%d)", r.limiter.Limit(), r.limiter.Burst())
}

// FixedWindowRateLimiter implements fixed window rate limiting
type FixedWindowRateLimiter struct {
	mu        sync.Mutex
	count     int
	limit     int
	window    time.Duration
	lastReset time.Time
}

// NewFixedWindowRateLimiter creates a new fixed window rate limiter
func NewFixedWindowRateLimiter(config Config) *FixedWindowRateLimiter {
	limit := int(config.Rate)
	if limit < 0 {
		limit = 0 // Handle negative rate
	}
	return &FixedWindowRateLimiter{
		limit:     limit,
		window:    config.Window,
		lastReset: time.Now(),
	}
}

// Allow checks if a request is allowed
func (r *FixedWindowRateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.lastReset) >= r.window {
		r.count = 0
		r.lastReset = now
	}

	if r.count >= r.limit {
		return false
	}

	r.count++
	return true
}

// AllowN checks if n requests are allowed
func (r *FixedWindowRateLimiter) AllowN(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n <= 0 {
		return n == 0 // Allow 0, deny negative
	}

	now := time.Now()
	if now.Sub(r.lastReset) >= r.window {
		r.count = 0
		r.lastReset = now
	}

	if r.count+n > r.limit {
		return false
	}

	r.count += n
	return true
}

// AllowWithContext checks if a request is allowed with context support
func (r *FixedWindowRateLimiter) AllowWithContext(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.Allow(), nil
	}
}

// AllowNWithContext checks if n requests are allowed with context support
func (r *FixedWindowRateLimiter) AllowNWithContext(ctx context.Context, n int) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.AllowN(n), nil
	}
}

// Wait waits until a request is allowed
func (r *FixedWindowRateLimiter) Wait(ctx context.Context) error {
	return r.WaitN(ctx, 1)
}

// WaitN waits until n requests are allowed
func (r *FixedWindowRateLimiter) WaitN(ctx context.Context, n int) error {
	for {
		if r.AllowN(n) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

// Reserve reserves a request
func (r *FixedWindowRateLimiter) Reserve() *Reservation {
	return r.ReserveN(1)
}

// ReserveN reserves n requests
func (r *FixedWindowRateLimiter) ReserveN(n int) *Reservation {
	if !r.AllowN(n) {
		return &Reservation{delay: r.timeUntilNextWindow()}
	}
	return &Reservation{ok: true}
}

// timeUntilNextWindow calculates time until next window
func (r *FixedWindowRateLimiter) timeUntilNextWindow() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	nextReset := r.lastReset.Add(r.window)
	return time.Until(nextReset)
}

// String returns string representation
func (r *FixedWindowRateLimiter) String() string {
	return fmt.Sprintf("FixedWindow(limit=%d, window=%v)", r.limit, r.window)
}

// SlidingWindowRateLimiter implements sliding window rate limiting
type SlidingWindowRateLimiter struct {
	mu     sync.Mutex
	tokens []time.Time
	limit  int
	window time.Duration
}

// NewSlidingWindowRateLimiter creates a new sliding window rate limiter
func NewSlidingWindowRateLimiter(config Config) *SlidingWindowRateLimiter {
	limit := int(config.Rate)
	if limit < 0 {
		limit = 0 // Handle negative rate
	}
	return &SlidingWindowRateLimiter{
		tokens: make([]time.Time, 0),
		limit:  limit,
		window: config.Window,
	}
}

// Allow checks if a request is allowed
func (r *SlidingWindowRateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Remove old tokens outside the window
	r.cleanup(now)

	if len(r.tokens) >= r.limit {
		return false
	}

	r.tokens = append(r.tokens, now)
	return true
}

// AllowN checks if n requests are allowed
func (r *SlidingWindowRateLimiter) AllowN(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n <= 0 {
		return n == 0 // Allow 0, deny negative
	}

	now := time.Now()

	// Remove old tokens outside the window
	r.cleanup(now)

	if len(r.tokens)+n > r.limit {
		return false
	}

	for i := 0; i < n; i++ {
		r.tokens = append(r.tokens, now)
	}
	return true
}

// AllowWithContext checks if a request is allowed with context support
func (r *SlidingWindowRateLimiter) AllowWithContext(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.Allow(), nil
	}
}

// AllowNWithContext checks if n requests are allowed with context support
func (r *SlidingWindowRateLimiter) AllowNWithContext(ctx context.Context, n int) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return r.AllowN(n), nil
	}
}

// Wait waits until a request is allowed
func (r *SlidingWindowRateLimiter) Wait(ctx context.Context) error {
	return r.WaitN(ctx, 1)
}

// WaitN waits until n requests are allowed
func (r *SlidingWindowRateLimiter) WaitN(ctx context.Context, n int) error {
	for {
		if r.AllowN(n) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

// Reserve reserves a request
func (r *SlidingWindowRateLimiter) Reserve() *Reservation {
	return r.ReserveN(1)
}

// ReserveN reserves n requests
func (r *SlidingWindowRateLimiter) ReserveN(n int) *Reservation {
	if !r.AllowN(n) {
		return &Reservation{delay: r.timeUntilNextWindow()}
	}
	return &Reservation{ok: true}
}

// cleanup removes old tokens outside the window
func (r *SlidingWindowRateLimiter) cleanup(now time.Time) {
	cutoff := now.Add(-r.window)

	// Remove all tokens that are outside the window
	validTokens := make([]time.Time, 0)
	for _, token := range r.tokens {
		if token.After(cutoff) {
			validTokens = append(validTokens, token)
		}
	}
	r.tokens = validTokens
}

// timeUntilNextWindow calculates time until next available slot
func (r *SlidingWindowRateLimiter) timeUntilNextWindow() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.tokens) < r.limit {
		return 0
	}

	oldestToken := r.tokens[0]
	return time.Until(oldestToken.Add(r.window))
}

// String returns string representation
func (r *SlidingWindowRateLimiter) String() string {
	return fmt.Sprintf("SlidingWindow(limit=%d, window=%v)", r.limit, r.window)
}

// Reservation represents a rate limit reservation
type Reservation struct {
	ok    bool
	delay time.Duration
}

// OK returns true if the reservation is valid
func (r *Reservation) OK() bool {
	return r.ok
}

// Delay returns the delay before the reservation is valid
func (r *Reservation) Delay() time.Duration {
	return r.delay
}

// Cancel cancels the reservation
func (r *Reservation) Cancel() {
	// No-op for this implementation
}

// Manager manages multiple rate limiters
type Manager struct {
	limiters map[string]RateLimiter
	mutex    sync.RWMutex
	config   Config
}

// NewManager creates a new rate limiter manager
func NewManager(defaultConfig Config) *Manager {
	return &Manager{
		limiters: make(map[string]RateLimiter),
		config:   defaultConfig,
	}
}

// Add adds a rate limiter
func (m *Manager) Add(name string, limiter RateLimiter) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.limiters[name] = limiter
}

// AddConfig adds a rate limiter with configuration
func (m *Manager) AddConfig(name string, config Config) error {
	limiter, err := NewRateLimiter(config)
	if err != nil {
		return err
	}

	m.Add(name, limiter)
	return nil
}

// Get returns a rate limiter by name
func (m *Manager) Get(name string) (RateLimiter, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	limiter, exists := m.limiters[name]
	return limiter, exists
}

// Remove removes a rate limiter
func (m *Manager) Remove(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.limiters, name)
}

// List returns all rate limiter names
func (m *Manager) List() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.limiters))
	for name := range m.limiters {
		names = append(names, name)
	}
	return names
}

// Allow checks if a request is allowed for the given limiter
func (m *Manager) Allow(name string) bool {
	limiter, exists := m.Get(name)
	if !exists {
		// Use default limiter
		limiter, _ = m.Get("default")
		if limiter == nil {
			return true // No rate limiting
		}
	}

	return limiter.Allow()
}

// AllowN checks if n requests are allowed for the given limiter
func (m *Manager) AllowN(name string, n int) bool {
	limiter, exists := m.Get(name)
	if !exists {
		limiter, _ = m.Get("default")
		if limiter == nil {
			return true
		}
	}

	return limiter.AllowN(n)
}

// Wait waits until a request is allowed for the given limiter
func (m *Manager) Wait(ctx context.Context, name string) error {
	limiter, exists := m.Get(name)
	if !exists {
		limiter, _ = m.Get("default")
		if limiter == nil {
			return nil
		}
	}

	return limiter.Wait(ctx)
}

// WaitN waits until n requests are allowed for the given limiter
func (m *Manager) WaitN(ctx context.Context, name string, n int) error {
	limiter, exists := m.Get(name)
	if !exists {
		limiter, _ = m.Get("default")
		if limiter == nil {
			return nil
		}
	}

	return limiter.WaitN(ctx, n)
}

// NewRateLimiter creates a new rate limiter based on strategy
func NewRateLimiter(config Config) (RateLimiter, error) {
	switch config.Strategy {
	case StrategyTokenBucket:
		return NewTokenBucketRateLimiter(config), nil
	case StrategyFixedWindow:
		return NewFixedWindowRateLimiter(config), nil
	case StrategySlidingWindow:
		return NewSlidingWindowRateLimiter(config), nil
	default:
		// Return default token bucket limiter for invalid strategies
		return NewTokenBucketRateLimiter(config), nil
	}
}

// HTTP middleware provides rate limiting for HTTP requests
type HTTPMiddleware struct {
	manager *Manager
}

// NewHTTPMiddleware creates a new HTTP rate limiting middleware
func NewHTTPMiddleware(manager *Manager) *HTTPMiddleware {
	return &HTTPMiddleware{manager: manager}
}

// Wrap wraps an HTTP handler with rate limiting
func (m *HTTPMiddleware) Wrap(name string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.manager.Allow(name) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// WrapWithConfig wraps an HTTP handler with rate limiting using config
func (m *HTTPMiddleware) WrapWithConfig(config Config, handler http.Handler) http.Handler {
	limiter, err := NewRateLimiter(config)
	if err != nil {
		logger.Error(err)
		return handler
	}

	name := fmt.Sprintf("http_%d", time.Now().UnixNano())
	m.manager.Add(name, limiter)

	return m.Wrap(name, handler)
}
