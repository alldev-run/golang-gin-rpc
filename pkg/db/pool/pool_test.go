package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang-gin-rpc/pkg/db"
)

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxSize != 10 {
		t.Errorf("MaxSize = %d, want 10", cfg.MaxSize)
	}
	if cfg.InitialSize != 2 {
		t.Errorf("InitialSize = %d, want 2", cfg.InitialSize)
	}
	if cfg.MaxIdleTime != 30*time.Minute {
		t.Errorf("MaxIdleTime = %v, want 30m", cfg.MaxIdleTime)
	}
	if cfg.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s", cfg.HealthCheckPeriod)
	}
	if cfg.MaxFailures != 3 {
		t.Errorf("MaxFailures = %d, want 3", cfg.MaxFailures)
	}
	if cfg.AcquireTimeout != 5*time.Second {
		t.Errorf("AcquireTimeout = %v, want 5s", cfg.AcquireTimeout)
	}
	if cfg.RetryDelay != 1*time.Second {
		t.Errorf("RetryDelay = %v, want 1s", cfg.RetryDelay)
	}
}

func TestNewPool(t *testing.T) {
	factory := db.NewFactory()
	pool := New(DefaultConfig(), factory)

	if pool == nil {
		t.Fatal("New() returned nil")
	}
	if pool.factory == nil {
		t.Error("Pool factory is nil")
	}
	if pool.clients == nil {
		t.Error("Pool clients map is nil")
	}
	if pool.healthStop == nil {
		t.Error("Pool healthStop channel is nil")
	}

	// Clean up
	_ = pool.Close()
}

func TestPooledClientIsConnected(t *testing.T) {
	pc := &PooledClient{
		id:    "test",
		state: int32(StateConnected),
	}

	if !pc.IsConnected() {
		t.Error("IsConnected() should return true for StateConnected")
	}

	// Test disconnected state
	pc.state = int32(StateDisconnected)
	if pc.IsConnected() {
		t.Error("IsConnected() should return false for StateDisconnected")
	}

	// Test failed state
	pc.state = int32(StateFailed)
	if pc.IsConnected() {
		t.Error("IsConnected() should return false for StateFailed")
	}
}

func TestPooledClientMarkUsed(t *testing.T) {
	pc := &PooledClient{
		id:        "test",
		state:     int32(StateConnected),
		useCount:  0,
		failCount: 5, // Some failures
	}

	pc.markUsed()

	if atomic.LoadInt64(&pc.useCount) != 1 {
		t.Errorf("useCount = %d, want 1", atomic.LoadInt64(&pc.useCount))
	}

	if atomic.LoadInt32(&pc.failCount) != 0 {
		t.Errorf("failCount should be reset to 0, got %d", atomic.LoadInt32(&pc.failCount))
	}

	if atomic.LoadInt64(&pc.lastUsed) == 0 {
		t.Error("lastUsed should be set")
	}
}

func TestPooledClientMarkFailed(t *testing.T) {
	pc := &PooledClient{
		id:        "test",
		failCount: 0,
	}

	pc.markFailed()
	if atomic.LoadInt32(&pc.failCount) != 1 {
		t.Errorf("failCount = %d, want 1", atomic.LoadInt32(&pc.failCount))
	}

	pc.markFailed()
	if atomic.LoadInt32(&pc.failCount) != 2 {
		t.Errorf("failCount = %d, want 2", atomic.LoadInt32(&pc.failCount))
	}
}

func TestPooledClientGetStats(t *testing.T) {
	now := time.Now().Unix()
	pc := &PooledClient{
		id:        "test-client",
		state:     int32(StateConnected),
		lastUsed:  now,
		createdAt: now,
		useCount:  10,
		failCount: 2,
	}

	stats := pc.GetStats()

	if stats.ID != "test-client" {
		t.Errorf("ID = %s, want test-client", stats.ID)
	}
	if stats.State != StateConnected {
		t.Errorf("State = %v, want StateConnected", stats.State)
	}
	if stats.UseCount != 10 {
		t.Errorf("UseCount = %d, want 10", stats.UseCount)
	}
	if stats.FailCount != 2 {
		t.Errorf("FailCount = %d, want 2", stats.FailCount)
	}
}

func TestPoolSize(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	if pool.Size() != 0 {
		t.Errorf("Initial size = %d, want 0", pool.Size())
	}
}

func TestPoolGetConnectionNames(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	names := pool.GetConnectionNames()
	if len(names) != 0 {
		t.Errorf("Expected 0 names, got %d", len(names))
	}
}

func TestPoolGetStats(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	stats := pool.GetStats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 stats, got %d", len(stats))
	}
}

// TestConcurrentAccess tests thread safety of the pool
func TestConcurrentAccess(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Try to acquire (will fail without registered connections, but tests concurrency)
			_, _ = pool.Acquire(context.Background(), "test")

			// Get stats
			_ = pool.GetStats()

			// Get names
			_ = pool.GetConnectionNames()
		}(i)
	}

	wg.Wait()
	// If we get here without panic, concurrency is working
	t.Log("Concurrent access test passed")
}

// TestRegisterDuplicate tests that duplicate registration fails
func TestRegisterDuplicate(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	// First registration would require a valid config and working DB
	// So we just test the internal state
	pool.mutex.Lock()
	pool.clients["test"] = &PooledClient{
		id:    "test",
		state: int32(StateConnected),
	}
	pool.mutex.Unlock()

	// Try to register again
	err := pool.Register("test", db.Config{Type: db.TypeMySQL})
	if err == nil {
		t.Error("Register() should fail for duplicate name")
	}
}

// TestUnregisterNotFound tests that unregistering non-existent connection fails
func TestUnregisterNotFound(t *testing.T) {
	pool := New(DefaultConfig(), db.NewFactory())
	defer pool.Close()

	err := pool.Unregister("nonexistent")
	if err == nil {
		t.Error("Unregister() should fail for non-existent connection")
	}
}

// TestStateConstants tests that state values are correct
func TestStateConstants(t *testing.T) {
	if StateDisconnected != 0 {
		t.Errorf("StateDisconnected = %d, want 0", StateDisconnected)
	}
	if StateConnecting != 1 {
		t.Errorf("StateConnecting = %d, want 1", StateConnecting)
	}
	if StateConnected != 2 {
		t.Errorf("StateConnected = %d, want 2", StateConnected)
	}
	if StateFailed != 3 {
		t.Errorf("StateFailed = %d, want 3", StateFailed)
	}
}
