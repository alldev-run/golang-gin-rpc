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
	ttl              time.Duration
	maxKeys          int
	cleanupInterval  time.Duration
	lastCleanup      time.Time

	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate int
	lastRefill time.Time
	lastSeen   time.Time
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
	ttl := 10 * time.Minute
	if ttl < time.Minute {
		ttl = time.Minute
	}
	return &TokenBucketLimiter{
		requestsPerMinute: requestsPerMinute,
		burst:            burst,
		refillPerSecond:  refill,
		ttl:              ttl,
		maxKeys:          100000,
		cleanupInterval:  time.Minute,
		lastCleanup:      time.Now(),
		buckets:          make(map[string]*tokenBucket),
	}
}

func (l *TokenBucketLimiter) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	l.maybeCleanup(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, maxTokens: l.burst, refillRate: l.refillPerSecond, lastRefill: now, lastSeen: now}
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.allow(now)
}

func (l *TokenBucketLimiter) maybeCleanup(now time.Time) {
	if l.cleanupInterval <= 0 {
		l.cleanupInterval = time.Minute
	}
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.cleanupInterval {
		return
	}
	l.lastCleanup = now

	if l.ttl > 0 {
		for k, b := range l.buckets {
			b.mu.Lock()
			lastSeen := b.lastSeen
			b.mu.Unlock()
			if now.Sub(lastSeen) > l.ttl {
				delete(l.buckets, k)
			}
		}
	}

	if l.maxKeys > 0 {
		for len(l.buckets) > l.maxKeys {
			var oldestKey string
			var oldestTime time.Time
			first := true
			for k, b := range l.buckets {
				b.mu.Lock()
				lastSeen := b.lastSeen
				b.mu.Unlock()
				if first || lastSeen.Before(oldestTime) {
					oldestKey = k
					oldestTime = lastSeen
					first = false
				}
			}
			if oldestKey == "" {
				break
			}
			delete(l.buckets, oldestKey)
		}
	}
}

func (b *tokenBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastSeen = now

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
