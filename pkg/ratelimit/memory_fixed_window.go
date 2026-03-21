package ratelimit

import (
	"sync"
	"time"
)

// MemoryFixedWindow is a simple in-memory fixed window rate limiter.
// It is suitable for demos/single-node deployments.
//
// It is safe for concurrent use.
type MemoryFixedWindow struct {
	requests int
	window   time.Duration
	ttl      time.Duration
	maxKeys  int
	cleanupInterval time.Duration
	lastCleanup time.Time

	mu      sync.Mutex
	clients map[string]*bucket
}

type bucket struct {
	remaining int
	resetAt   time.Time
	lastSeen  time.Time
}

func NewMemoryFixedWindow(requests int, window time.Duration) *MemoryFixedWindow {
	if requests <= 0 {
		requests = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	ttl := 10 * window
	if ttl < time.Minute {
		ttl = time.Minute
	}
	return &MemoryFixedWindow{
		requests: requests,
		window:   window,
		ttl:      ttl,
		maxKeys:  100000,
		cleanupInterval: window,
		lastCleanup: time.Now(),
		clients:  make(map[string]*bucket),
	}
}

// Allow returns whether a request for the given key is allowed.
func (l *MemoryFixedWindow) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.maybeCleanup(now)

	b, ok := l.clients[key]
	if !ok {
		l.clients[key] = &bucket{remaining: l.requests - 1, resetAt: now.Add(l.window), lastSeen: now}
		return true
	}
	b.lastSeen = now

	if now.After(b.resetAt) {
		b.remaining = l.requests
		b.resetAt = now.Add(l.window)
	}

	if b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}

func (l *MemoryFixedWindow) maybeCleanup(now time.Time) {
	if l.cleanupInterval <= 0 {
		l.cleanupInterval = time.Minute
	}
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.cleanupInterval {
		return
	}
	l.lastCleanup = now

	if l.ttl > 0 {
		for k, b := range l.clients {
			if now.Sub(b.lastSeen) > l.ttl {
				delete(l.clients, k)
			}
		}
	}

	if l.maxKeys > 0 {
		for len(l.clients) > l.maxKeys {
			var oldestKey string
			var oldestTime time.Time
			first := true
			for k, b := range l.clients {
				if first || b.lastSeen.Before(oldestTime) {
					oldestKey = k
					oldestTime = b.lastSeen
					first = false
				}
			}
			if oldestKey == "" {
				break
			}
			delete(l.clients, oldestKey)
		}
	}
}
