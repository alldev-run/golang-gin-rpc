package ratelimit

import (
	"sync"
	"time"
)

// TokenBucketLimiter is an in-memory token bucket rate limiter keyed by string.
// It is safe for concurrent use.
//
// requestsPerMinute: steady rate
// burst: maximum tokens stored
// refill happens in seconds granularity.
type TokenBucketLimiter struct {
	requestsPerMinute int
	burst            int
	refillPerSecond  int

	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate int
	lastRefill time.Time
}

func NewTokenBucketLimiter(requestsPerMinute, burst int) *TokenBucketLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burst <= 0 {
		burst = max(1, requestsPerMinute/4)
	}
	refill := requestsPerMinute / 60
	if refill <= 0 {
		refill = 1
	}
	return &TokenBucketLimiter{
		requestsPerMinute: requestsPerMinute,
		burst:            burst,
		refillPerSecond:  refill,
		buckets:          make(map[string]*tokenBucket),
	}
}

func (l *TokenBucketLimiter) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, maxTokens: l.burst, refillRate: l.refillPerSecond, lastRefill: now}
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.allow(now)
}

func (b *tokenBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := now.Sub(b.lastRefill)
	add := int(elapsed.Seconds()) * b.refillRate
	if add > 0 {
		b.tokens += add
		if b.tokens > b.maxTokens {
			b.tokens = b.maxTokens
		}
		b.lastRefill = now
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
