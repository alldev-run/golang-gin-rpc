package ratelimit

import "testing"

func TestTokenBucketLimiterBurst(t *testing.T) {
	l := NewTokenBucketLimiter(60, 2)
	key := "k"
	if !l.Allow(key) {
		t.Fatalf("expected allow 1")
	}
	if !l.Allow(key) {
		t.Fatalf("expected allow 2")
	}
	// likely denied (no time has passed to refill)
	if l.Allow(key) {
		t.Fatalf("expected deny after burst exhausted")
	}
}
