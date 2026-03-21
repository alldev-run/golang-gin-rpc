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

	mu      sync.Mutex
	clients map[string]*bucket
}

type bucket struct {
	remaining int
	resetAt   time.Time
}

func NewMemoryFixedWindow(requests int, window time.Duration) *MemoryFixedWindow {
	if requests <= 0 {
		requests = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &MemoryFixedWindow{
		requests: requests,
		window:   window,
		clients:  make(map[string]*bucket),
	}
}

// Allow returns whether a request for the given key is allowed.
func (l *MemoryFixedWindow) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.clients[key]
	if !ok {
		l.clients[key] = &bucket{remaining: l.requests - 1, resetAt: now.Add(l.window)}
		return true
	}

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
