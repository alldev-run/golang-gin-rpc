package cache

import (
	"context"
	"testing"
	"time"
)

func TestCacheAdaptersHandleBackendFailuresAndMemoryOperations(t *testing.T) {
	ctx := context.Background()

	_, err := NewRedisCache(RedisConfig{
		Host: "127.0.0.1", Port: 1, MaxRetries: -1,
		DialTimeout: 20 * time.Millisecond, ReadTimeout: 20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond, PoolTimeout: 20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("NewRedisCache() succeeded without a reachable Redis server")
	}

	memcacheCache, err := NewMemcacheCache(MemcacheConfig{
		Hosts: []string{"127.0.0.1:1"}, Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewMemcacheCache() error = %v", err)
	}
	_, _ = memcacheCache.Get(ctx, "key")
	_ = memcacheCache.Set(ctx, "key", "value", time.Second)
	_ = memcacheCache.Set(ctx, "key", struct{}{}, time.Second)
	_ = memcacheCache.Delete(ctx, "key")
	_, _ = memcacheCache.Exists(ctx, "key")
	_ = memcacheCache.Clear(ctx)
	if stats := memcacheCache.GetStats(); stats.Errors == 0 {
		t.Fatal("Memcache adapter did not record backend errors")
	}
	if err := memcacheCache.Close(); err != nil {
		t.Fatal(err)
	}

	failoverCache, err := NewFailoverCache(FailoverConfig{
		StorageType:   "memory",
		FallbackOrder: []string{"memory"},
	})
	if err != nil {
		t.Fatalf("NewFailoverCache() error = %v", err)
	}
	if value, err := failoverCache.Get(ctx, "missing"); err != nil || value != nil {
		t.Fatalf("Get() for missing key = %v, %v", value, err)
	}
	if err := failoverCache.Set(ctx, "key", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if value, err := failoverCache.GetWithLock(ctx, "key"); err != nil || value != "value" {
		t.Fatalf("GetWithLock() = %v, %v", value, err)
	}
	if err := failoverCache.SetWithRandomTTL(ctx, "random", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if exists, err := failoverCache.Exists(ctx, "key"); err != nil || !exists {
		t.Fatalf("Exists() = %v, %v", exists, err)
	}
	if err := failoverCache.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if err := failoverCache.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if stats := failoverCache.GetStats(); stats.Sets == 0 || stats.Deletes == 0 {
		t.Fatalf("Failover adapter stats = %+v", stats)
	}
	if err := failoverCache.Close(); err != nil {
		t.Fatal(err)
	}
}
