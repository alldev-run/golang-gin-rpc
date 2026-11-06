package websocket

import (
	"errors"
	"sync"
	"time"
)

var errRateLimited = errors.New("websocket: rate limit exceeded")

// connLimiter is a per-connection token bucket throttling inbound messages.
type connLimiter struct {
	mu     sync.Mutex
	rate   float64 // tokens per second
	burst  float64 // bucket capacity
	tokens float64
	last   time.Time
}

func newConnLimiter(cfg ConnRateLimitConfig) *connLimiter {
	burst := cfg.Burst
	if burst <= 0 {
		burst = cfg.Rate
	}
	return &connLimiter{
		rate:   float64(cfg.Rate),
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// allow consumes one token, refilling by elapsed time.
func (l *connLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
