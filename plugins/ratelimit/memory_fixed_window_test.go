package ratelimit

import (
	"testing"
	"time"
)

func TestMemoryFixedWindow(t *testing.T) {
	l := NewMemoryFixedWindow(2, time.Minute)
	key := "client"
	if !l.Allow(key) {
		t.Fatalf("expected first allow")
	}
	if !l.Allow(key) {
		t.Fatalf("expected second allow")
	}
	if l.Allow(key) {
		t.Fatalf("expected third denied")
	}
}
