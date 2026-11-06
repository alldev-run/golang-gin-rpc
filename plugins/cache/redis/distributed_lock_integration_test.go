//go:build integration

package redis

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newIntegrationRedisClient(t *testing.T) *Client {
	t.Helper()
	cfg := DefaultConfig()
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("new redis client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	return client
}

func TestDistributedLockIntegration(t *testing.T) {
	client := newIntegrationRedisClient(t)
	ctx := context.Background()

	l1 := NewDistributedLock(client, "it-lock", time.Second)
	ok, err := l1.TryAcquire(ctx)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}

	// 第二个持有者拿不到。
	l2 := NewDistributedLock(client, "it-lock", time.Second)
	ok, _ = l2.TryAcquire(ctx)
	if ok {
		t.Fatal("second acquire should fail")
	}

	// l2 释放自己的(未持有的)锁 → ErrLockNotHeld。
	if err := l2.Release(ctx); err != ErrLockNotHeld {
		t.Fatalf("expected ErrLockNotHeld, got %v", err)
	}

	// l1 续期后释放。
	if ok, err := l1.Renew(ctx); err != nil || !ok {
		t.Fatalf("renew: ok=%v err=%v", ok, err)
	}
	if err := l1.Release(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}

	// 释放后 l2 可以拿到。
	ok, err = l2.TryAcquire(ctx)
	if err != nil || !ok {
		t.Fatalf("reacquire after release: ok=%v err=%v", ok, err)
	}
	_ = l2.Release(ctx)
}

func TestDistributedLockContentionIntegration(t *testing.T) {
	client := newIntegrationRedisClient(t)
	ctx := context.Background()

	var counter atomic.Int64
	var wg sync.WaitGroup
	const workers = 8
	const loops = 20
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				l := NewDistributedLock(client, "it-counter", 2*time.Second)
				lockCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				if err := l.Acquire(lockCtx, 20*time.Millisecond); err != nil {
					cancel()
					continue
				}
				counter.Add(1)
				time.Sleep(time.Millisecond)
				_ = l.Release(context.Background())
				cancel()
			}
		}()
	}
	wg.Wait()
	if counter.Load() != workers*loops {
		t.Fatalf("counter = %d, want %d", counter.Load(), int64(workers*loops))
	}
}

func TestDistributedLockWatchdogRenewalIntegration(t *testing.T) {
	client := newIntegrationRedisClient(t)
	ctx := context.Background()

	l := NewDistributedLock(client, "it-watchdog", 400*time.Millisecond)
	l.EnableWatchdog()
	if ok, err := l.TryAcquire(ctx); err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	// 锁 TTL 只有 400ms,看门狗每 ~133ms 续期;1.2s 后锁应仍然存在。
	time.Sleep(1200 * time.Millisecond)
	n, err := client.Exists(ctx, l.Key())
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if n != 1 {
		t.Fatal("watchdog failed to keep the lock alive")
	}
	if err := l.Release(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
}
