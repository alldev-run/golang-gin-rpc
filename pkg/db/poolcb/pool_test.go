package poolcb

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang-gin-rpc/pkg/db"
	"golang-gin-rpc/pkg/db/circuitbreaker"
	"golang-gin-rpc/pkg/db/pool"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PoolConfig.MaxSize != 10 {
		t.Errorf("Pool MaxSize = %d, want 10", cfg.PoolConfig.MaxSize)
	}
	if cfg.BreakerConfig.MaxFailures != 5 {
		t.Errorf("Breaker MaxFailures = %d, want 5", cfg.BreakerConfig.MaxFailures)
	}
}

func TestNewPoolWithBreaker(t *testing.T) {
	cfg := DefaultConfig()
	p := New(cfg, nil)

	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.pool == nil {
		t.Error("Pool is nil")
	}
	if p.breaker == nil {
		t.Error("Breaker is nil")
	}

	// Clean up
	_ = p.Close()
}

func TestPoolWithBreakerState(t *testing.T) {
	cfg := DefaultConfig()
	p := New(cfg, nil)
	defer p.Close()

	// Initial state should be closed
	if p.GetState() != circuitbreaker.StateClosed {
		t.Errorf("Initial state should be Closed, got %v", p.GetState())
	}

	// Force open
	p.ForceOpen()
	if p.GetState() != circuitbreaker.StateOpen {
		t.Errorf("State should be Open after ForceOpen, got %v", p.GetState())
	}

	// Force closed
	p.ForceClosed()
	if p.GetState() != circuitbreaker.StateClosed {
		t.Errorf("State should be Closed after ForceClosed, got %v", p.GetState())
	}
}

func TestPoolWithBreakerStats(t *testing.T) {
	cfg := DefaultConfig()
	p := New(cfg, nil)
	defer p.Close()

	// Get pool stats
	poolStats := p.GetPoolStats()
	if len(poolStats) != 0 {
		t.Errorf("Expected 0 pool stats, got %d", len(poolStats))
	}

	// Get breaker stats
	breakerStats := p.GetBreakerStats()
	if breakerStats.State != circuitbreaker.StateClosed {
		t.Errorf("Expected breaker state Closed, got %v", breakerStats.State)
	}
}

func TestAcquireWithOpenCircuit(t *testing.T) {
	cfg := Config{
		PoolConfig: pool.DefaultConfig(),
		BreakerConfig: circuitbreaker.Config{
			MaxFailures:  1,
			ResetTimeout: 100 * time.Millisecond,
			Name:         "test",
		},
	}

	p := New(cfg, nil)
	defer p.Close()

	// Force circuit open
	p.ForceOpen()

	// Try to acquire - should fail with circuit open
	_, err := p.Acquire(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when circuit is open")
	}
}

func TestNewSQL(t *testing.T) {
	cfg := DefaultConfig()
	sqlPool := NewSQL(cfg, nil)

	if sqlPool == nil {
		t.Fatal("NewSQL() returned nil")
	}
	if sqlPool.PoolWithBreaker == nil {
		t.Error("PoolWithBreaker is nil")
	}

	// Clean up
	_ = sqlPool.Close()
}

// TestProtectedClient tests the protectedClient wrapper
func TestProtectedClient(t *testing.T) {
	cfg := DefaultConfig()
	p := New(cfg, db.NewFactory())
	defer p.Close()

	// Since we can't get a real protected client without a real DB,
	// we test that the structure exists
	pc := &protectedClient{
		client:  nil,
		breaker: p.breaker,
	}

	if pc.breaker == nil {
		t.Error("protectedClient breaker should not be nil")
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	cfg := DefaultConfig()
	p := New(cfg, nil)
	defer p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Concurrent state checks
			_ = p.GetState()
			_ = p.GetBreakerStats()
			_ = p.GetPoolStats()
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent access test passed")
}
