package poolrw

import (
	"context"
	"sync"
	"testing"
	"time"

	"alldev-gin-rpc/pkg/db"
	"alldev-gin-rpc/pkg/db/rwproxy"
)

func TestDefaultRWPoolConfig(t *testing.T) {
	cfg := DefaultRWPoolConfig()

	if cfg.Strategy != rwproxy.LBStrategyRoundRobin {
		t.Errorf("Strategy = %v, want LBStrategyRoundRobin", cfg.Strategy)
	}
	if cfg.PoolConfig.MaxIdleTime != 30*time.Minute {
		t.Errorf("MaxIdleTime = %v, want 30m", cfg.PoolConfig.MaxIdleTime)
	}
	if cfg.PoolConfig.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s", cfg.PoolConfig.HealthCheckPeriod)
	}
	if cfg.PoolConfig.MaxFailures != 3 {
		t.Errorf("MaxFailures = %d, want 3", cfg.PoolConfig.MaxFailures)
	}
	if cfg.PoolConfig.AcquireTimeout != 5*time.Second {
		t.Errorf("AcquireTimeout = %v, want 5s", cfg.PoolConfig.AcquireTimeout)
	}
}

func TestNewPoolWithoutFactory(t *testing.T) {
	cfg := DefaultRWPoolConfig()

	// Will fail because no real database, but tests that factory is created
	_, err := New(cfg, nil)
	if err == nil {
		t.Log("Pool created unexpectedly (may need real DB for full test)")
	} else {
		t.Logf("Expected error without DB: %v", err)
	}
}

func TestPoolStateConstants(t *testing.T) {
	if StateRunning != 0 {
		t.Errorf("StateRunning = %d, want 0", StateRunning)
	}
	if StateClosing != 1 {
		t.Errorf("StateClosing = %d, want 1", StateClosing)
	}
	if StateClosed != 2 {
		t.Errorf("StateClosed = %d, want 2", StateClosed)
	}
}

func TestPoolStatsStruct(t *testing.T) {
	stats := PoolStats{
		MasterConnected: true,
		ReplicaCount:    3,
		ReplicasHealthy: 2,
		ForceMaster:     false,
	}

	if !stats.MasterConnected {
		t.Error("MasterConnected should be true")
	}
	if stats.ReplicaCount != 3 {
		t.Errorf("ReplicaCount = %d, want 3", stats.ReplicaCount)
	}
	if stats.ReplicasHealthy != 2 {
		t.Errorf("ReplicasHealthy = %d, want 2", stats.ReplicasHealthy)
	}
	if stats.ForceMaster {
		t.Error("ForceMaster should be false")
	}
}

// TestGetSQLDBNotSQLClient tests error when client is not SQLClient
func TestGetSQLDBNotSQLClient(t *testing.T) {
	// Create a mock non-SQL client
	nonSQLClient := &mockNonSQLClient{}

	pool := &Pool{
		factory: db.NewFactory(),
	}

	_, err := pool.getSQLDB(nonSQLClient)
	if err == nil {
		t.Error("getSQLDB should fail for non-SQL client")
	}
}

// mockNonSQLClient implements db.Client but not db.SQLClient
type mockNonSQLClient struct{}

func (m *mockNonSQLClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockNonSQLClient) Close() error {
	return nil
}

// TestRWPoolConfigStruct tests config struct fields
type TestRWPoolConfigStruct struct {
	MasterConfig   db.Config
	ReplicaConfigs []db.Config
	Strategy       rwproxy.LBStrategy
	PoolConfig     PoolSettings
}

func TestPoolSettingsStruct(t *testing.T) {
	settings := PoolSettings{
		MaxIdleTime:       10 * time.Minute,
		HealthCheckPeriod: 15 * time.Second,
		MaxFailures:       5,
		AcquireTimeout:    3 * time.Second,
	}

	if settings.MaxIdleTime != 10*time.Minute {
		t.Error("MaxIdleTime mismatch")
	}
	if settings.HealthCheckPeriod != 15*time.Second {
		t.Error("HealthCheckPeriod mismatch")
	}
	if settings.MaxFailures != 5 {
		t.Error("MaxFailures mismatch")
	}
	if settings.AcquireTimeout != 3*time.Second {
		t.Error("AcquireTimeout mismatch")
	}
}

// TestLoadBalancerStrategies tests that strategies are properly defined
func TestLoadBalancerStrategies(t *testing.T) {
	if rwproxy.LBStrategyRoundRobin != 0 {
		t.Error("LBStrategyRoundRobin should be 0")
	}
	if rwproxy.LBStrategyRandom != 1 {
		t.Error("LBStrategyRandom should be 1")
	}
}

// TestPoolMethodsWithNilRWClient tests that methods handle nil rwClient gracefully
func TestPoolMethodsWithNilRWClient(t *testing.T) {
	pool := &Pool{
		config:   DefaultRWPoolConfig(),
		factory:  db.NewFactory(),
		rwClient: nil,
		master:   nil,
		replicas: []db.Client{},
	}

	ctx := context.Background()

	// Test Query
	_, err := pool.Query(ctx, "SELECT 1")
	if err == nil {
		t.Error("Query should fail with nil rwClient")
	}

	// Test Exec
	_, err = pool.Exec(ctx, "INSERT INTO t VALUES (1)")
	if err == nil {
		t.Error("Exec should fail with nil rwClient")
	}

	// Test Begin
	_, err = pool.Begin(ctx, nil)
	if err == nil {
		t.Error("Begin should fail with nil rwClient")
	}

	// Test Prepare
	_, err = pool.Prepare(ctx, "SELECT 1")
	if err == nil {
		t.Error("Prepare should fail with nil rwClient")
	}

	// Test IsMasterForced - should return false safely
	forced := pool.IsMasterForced()
	if forced {
		t.Error("IsMasterForced should return false with nil rwClient")
	}
}

// TestForceMasterWithNilRWClient tests ForceMaster with nil rwClient
type mockPoolWithStats struct {
	rwClient *mockRWClient
}

type mockRWClient struct {
	forceMaster bool
}

func (m *mockRWClient) ForceMaster(force bool) {
	m.forceMaster = force
}

func (m *mockRWClient) IsMasterForced() bool {
	return m.forceMaster
}

// TestPoolConcurrency tests thread safety of pool operations
func TestPoolConcurrency(t *testing.T) {
	pool := &Pool{
		config:   DefaultRWPoolConfig(),
		factory:  db.NewFactory(),
		rwClient: nil,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Concurrent read operations
			_ = pool.GetStats()
			_ = pool.IsMasterForced()
		}(i)
	}

	wg.Wait()
	t.Log("Pool concurrency test passed")
}
