package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeRedisRunner 用 Go 模拟 Lua 固定窗口语义,供单元测试。
type fakeRedisRunner struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newFakeRedisRunner() *fakeRedisRunner {
	return &fakeRedisRunner{counts: make(map[string]int64)}
}

func (f *fakeRedisRunner) Eval(_ context.Context, script string, keys []string, args ...any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[keys[0]]++
	current := f.counts[keys[0]]
	limit := args[1].(int)
	if current > int64(limit) {
		return int64(0), nil
	}
	return int64(1), nil
}

func TestRedisLimiterAllowDeny(t *testing.T) {
	runner := newFakeRedisRunner()
	l, err := NewRedisLimiter(runner, RedisLimiterConfig{Requests: 3, Window: time.Minute})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, "user-1")
		if err != nil || !ok {
			t.Fatalf("request %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	ok, err := l.Allow(ctx, "user-1")
	if err != nil {
		t.Fatalf("4th request err: %v", err)
	}
	if ok {
		t.Fatal("4th request should be denied")
	}
	// 不同 key 配额独立。
	if ok, _ := l.Allow(ctx, "user-2"); !ok {
		t.Fatal("user-2 should have its own quota")
	}
}

func TestRedisLimiterValidation(t *testing.T) {
	if _, err := NewRedisLimiter(nil, DefaultRedisLimiterConfig()); err == nil {
		t.Fatal("nil runner should be rejected")
	}
	runner := newFakeRedisRunner()
	if _, err := NewRedisLimiter(runner, RedisLimiterConfig{Requests: 0, Window: time.Minute}); err == nil {
		t.Fatal("zero requests should be rejected")
	}
	if _, err := NewRedisLimiter(runner, RedisLimiterConfig{Requests: 10, Window: 0}); err == nil {
		t.Fatal("zero window should be rejected")
	}
	if _, err := NewRedisLimiter(runner, RedisLimiterConfig{Requests: 10, Window: time.Minute}); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestRedisLimiterConcurrent(t *testing.T) {
	runner := newFakeRedisRunner()
	l, _ := NewRedisLimiter(runner, RedisLimiterConfig{Requests: 50, Window: time.Minute})
	ctx := context.Background()

	var allowed int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := l.Allow(ctx, "shared")
			if ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Fatalf("allowed = %d, want 50", allowed)
	}
}
